package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
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

		em, err = h.detectEventType(er.SNS.Message)
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

func (h *handler) detectEventType(msg string) (eventMessageMarshaler, error) {
	if cwa := h.detectCloudWatchAlarmEvent(msg); cwa != nil {
		// we found a CloudWatch Alarm
		return cwa, nil
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
