# aws-chatbot-mention

`aws-chatbot-mention` allows to insert an additional mention for notifications sent to AWS Chatbot.

## How it works

`aws-chatbot-mention` is a lambda function that sits between AWS Chatbot and its SNS topic.

```
[CloudWatch] -> [SNS] -> [aws-chatbot-mention] -> [SNS] -> [AWS Chatbot]
```

To insert an additional mention to the message, `aws-chatbot-mention` will detect a notification type and alter the SNS message.

**CloudWatch Alarm**: When `aws-chatbot-mention` finds special keyboards in the Alarm Description, it will copy text to `NewStateReason` so that Chatbot will show the text in the notification.

Example:
```
mention-OK:<!here>
mention-ALARM:<!channel>
```

If `NewStateValue` is `ALARM`, it will copy the text after `mention-ALARM:` string to the Alarm Description. If the state is `OK`, then it will copy the text after `mention-OK:`.

**CloudWatch Events**: `aws-chatbot-mention` will check a tag named `SLACK_GROUP` in the SNS topic. If a value exists, it will decode it by base64 and inject it into the text in the notification. The available characters in a tag is very restrictive so we decided to encoded a value in base64.
