package main

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
)

func TestFindMentionLineValue(t *testing.T) {
	assert := assert.New(t)

	alarmDescription := `foo
bar
mention-OK:<!here>

yes
mention-ALARM:<!channel>
`

	for state, expected := range map[types.StateValue]string{
		types.StateValueOk:               "<!here>",
		types.StateValueAlarm:            "<!channel>",
		types.StateValueInsufficientData: "",
	} {
		actual := FindMentionLineValue(alarmDescription, state)
		assert.Equal(expected, actual)
	}
}
