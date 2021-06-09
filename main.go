package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func FindMentionLineValue(ad string, newState string) string {
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

type handler struct {
	snsSvc   *sns.Client
	topicArn string
}

func (h *handler) handle(e events.SNSEvent) error {
	fmt.Printf("%+v\n", e)

	for _, ev := range e.Records {
		cwa := &events.CloudWatchAlarmSNSPayload{}
		err := json.Unmarshal([]byte(ev.SNS.Message), cwa)

		newMessage := ev.SNS.Message

		if err == nil && cwa.AlarmName != "" {
			txt := FindMentionLineValue(cwa.AlarmDescription, cwa.NewStateValue)
			if txt != "" {
				cwa.NewStateReason = txt + "\n" + cwa.NewStateReason
			}

			b, err := json.Marshal(cwa)
			if err == nil {
				newMessage = string(b)
			}
		}

		// relay to the Chatbot SNS topic with the new message
		fmt.Printf("NEW MESSAGE: %+v\n", newMessage)
		_, err = h.snsSvc.Publish(context.TODO(), &sns.PublishInput{
			Message:  &newMessage,
			TopicArn: &h.topicArn,
		})
		if err != nil {
			return err
		}

		log.Print("SNS message has been published")
	}

	return nil
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
