# aws-chatbot-mention

`aws-chatbot-mention` allows to insert an additional mention for notifications sent to AWS Chatbot.

## How it works

`aws-chatbot-mention` is a lambda function that sits between AWS Chatbot and its SNS topic.

```
[CloudWatch] -> [SNS] -> [aws-chatbot-mention] -> [SNS] -> [AWS Chatbot]
```

To insert an additional mention to the message, `aws-chatbot-mention` will detect a notification type and alter the SNS message.

**CloudWatch Alarm**: If `aws-chatbot-mention` finds special keyboards in the Alarm Description, it will copy text to `NewStateReason` so that Chatbot will show the text in the notification.

Example:
```
mention-OK:<!here>
mention-ALARM:<!channel>
```

If `NewStateValue` is `ALARM`, it will copy the text after `mention-ALARM:` string to the Alarm Description. If the state is `OK`, then it will copy the text after `mention-OK:`.

**CloudWatch Events**: TBD
