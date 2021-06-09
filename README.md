# aws-chatbot-mention

`aws-chatbot-mention` allows to add `@mention` specified in the Alarm Description.

## How it works

AWS Chatbot uses `NewStateReason` for the body of Slack notification. If it finds a special keyboard in the Alarm Description, it will add a text to `NewStateReason`.

**Keywords**: You can put the following keyboards.
```
mention-OK:<!here>
mention-ALARM:<!channel>
```

If `NewStateValue` is `ALARM`, it will copy the text after `mention-ALARM:` string to the Alarm Description. If the state is `OK`, then it will copy the text after `mention-OK:`.

Actually, you can specify not only Slack mention but any text you want.
