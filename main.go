package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type handler struct {
	snsSvc   *sns.Client
	topicArn string
}

type eventMessageMarshaler interface {
	MarshalSNSJSONString() (string, error)
}

type unknownEventMessage string

func (m unknownEventMessage) MarshalSNSJSONString() (string, error) {
	return string(m), nil
}

func (h *handler) handle(e events.SNSEvent) error {
	log.Printf("%+v\n", e)
	for _, er := range e.Records {
		var (
			em  eventMessageMarshaler
			err error
		)

		em, err = h.detectEventType(er.SNS)
		if err != nil {
			log.Printf("using the message as-is: %s", err)
			em = unknownEventMessage(er.SNS.Message)
		}

		newMessage, err := em.MarshalSNSJSONString()
		if err != nil {
			return err
		}

		log.Printf("NEW MESSAGE: %+v\n", newMessage)
		_, err = h.snsSvc.Publish(context.TODO(), &sns.PublishInput{
			Message:  &newMessage,
			TopicArn: &h.topicArn,
		})
		if err != nil {
			return fmt.Errorf("unable to publish SNS message: %w", err)
		}
	}

	return nil
}

func (h *handler) detectEventType(snse events.SNSEntity) (eventMessageMarshaler, error) {
	// try with CloudWatch Alarm
	if cwa := h.detectCloudWatchAlarmEvent(snse.Message); cwa != nil {
		return cwa, nil
	}

	// try with CloudWatch Events
	if cwe := h.detectCloudWatchEvent(snse); cwe != nil {
		return cwe, nil
	}

	return nil, errors.New("no event type detected")
}

func (h *handler) detectCloudWatchAlarmEvent(msg string) *cloudWatchAlarmEvent {
	cwa := &events.CloudWatchAlarmSNSPayload{}
	err := json.Unmarshal([]byte(msg), cwa)
	if err != nil || cwa.AlarmName == "" {
		log.Printf("unable to unmarshal into *events.CloudWatchAlarmSNSPayload. skipping...: %s", err)
		return nil
	}

	return (&cloudWatchAlarmEvent{cwa: cwa}).update()
}

func (h *handler) detectCloudWatchEvent(snse events.SNSEntity) *cloudWatchEvent {
	cwe := &events.CloudWatchEvent{}
	err := json.Unmarshal([]byte(snse.Message), cwe)
	if err != nil || cwe.DetailType == "" {
		log.Printf("unable to unmarshal into *events.CloudWatchEvent. skipping...: %s", err)
		return nil
	}

	return (&cloudWatchEvent{snsSvc: h.snsSvc, cwe: cwe, snsEvent: snse}).update()
}

type cloudWatchEvent struct {
	snsSvc   *sns.Client
	cwe      *events.CloudWatchEvent
	snsEvent events.SNSEntity
}

func (e *cloudWatchEvent) MarshalSNSJSONString() (string, error) {
	b, err := json.Marshal(e.cwe)
	if err != nil {
		return "", fmt.Errorf("unable to marshal into JSON: %w", err)
	}
	return string(b), nil
}

func (e cloudWatchEvent) getMentionToByTag() (string, error) {
	req := &sns.ListTagsForResourceInput{
		ResourceArn: aws.String(e.snsEvent.TopicArn),
	}

	resp, err := e.snsSvc.ListTagsForResource(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("unable to get the tag from the resource: %w", err)
	}

	tag, err := findTagByKey(resp.Tags, "SLACK_GROUP")
	if err != nil {
		return "", err
	}

	b, err := base64.StdEncoding.DecodeString(*tag.Value)
	if err != nil {
		return "", fmt.Errorf("unable to decode the value in base64: %w", err)
	}

	return string(b), nil
}

func findTagByKey(tags []types.Tag, key string) (types.Tag, error) {
	for _, t := range tags {
		if *t.Key == key {
			return t, nil
		}
	}

	return types.Tag{}, fmt.Errorf("tag not found for '%s'", key)
}

func (e *cloudWatchEvent) update() *cloudWatchEvent {
	const keyToinject = "userAgent"

	mentionTo, err := e.getMentionToByTag()
	if err != nil {
		log.Printf("unable to get the mention-to. skipping...: %s", err)
		return e
	}

	detail := make(map[string]interface{})
	if err := json.Unmarshal(e.cwe.Detail, &detail); err != nil {
		// nothing to do
		log.Printf("unable to unmarshal the event detail. skipping...: %s", err)
		return e
	}

	keyValue, ok := detail[keyToinject]
	if !ok {
		log.Print("no key to inject in the event. skipping.")
		return e
	}

	// will inject a Slack mention into the key to inject
	if keyValueStr, ok := keyValue.(string); ok && keyValueStr != "" {
		detail[keyToinject] = keyValueStr + "\n" + mentionTo + "\n"
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		log.Printf("unable to re-marshal the event: %w", err)
		return e
	}

	e.cwe.Detail = detailJSON
	return e
}

type cloudWatchAlarmEvent struct {
	cwa *events.CloudWatchAlarmSNSPayload
}

func (e *cloudWatchAlarmEvent) MarshalSNSJSONString() (string, error) {
	b, err := json.Marshal(e.cwa)
	if err != nil {
		return "", fmt.Errorf("unable to marshal into JSON: %w", err)
	}
	return string(b), nil
}

func (e *cloudWatchAlarmEvent) update() *cloudWatchAlarmEvent {
	txt := findMentionLineValue(e.cwa.AlarmDescription, e.cwa.NewStateValue)
	if txt != "" {
		e.cwa.NewStateReason = txt + "\n" + e.cwa.NewStateReason
	}
	return e
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := sns.NewFromConfig(cfg)
	h := &handler{
		snsSvc:   svc,
		topicArn: os.Getenv("CHATBOT_SNS_TOPIC_ARN"),
	}
	lambda.Start(h.handle)
}

func findMentionLineValue(ad string, newState string) string {
	prefix := fmt.Sprintf("mention-%s:", newState)

	scanner := bufio.NewScanner(strings.NewReader(ad))
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.HasPrefix(txt, prefix) {
			return txt[len(prefix):]
		}
	}

	return ""
}
