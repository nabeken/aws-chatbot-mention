package main

import (
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		actual := findMentionLineValue(alarmDescription, string(state))
		assert.Equal(expected, actual)
	}
}

func TestDetectEventType(t *testing.T) {
	require := require.New(t)

	b, err := os.ReadFile("_testdata/cloudwatch_alarm_OK.json")
	require.NoError(err)

	h := &handler{}
	em, err := h.detectEventType(string(b))
	require.NoError(err)

	cwa, ok := em.(*cloudWatchAlarmEvent)
	require.True(ok, "must be *cloudWatchAlarmEvent")

	cwa.update()
	require.True(strings.HasPrefix(cwa.cwa.NewStateReason, "<!here>"), "must start with <!here>")
}
