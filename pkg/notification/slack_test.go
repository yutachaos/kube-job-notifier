package notification

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewSlack(t *testing.T) {
	os.Setenv("SLACK_TOKEN", "slack_token")
	os.Setenv("SLACK_CHANNEL", "slack_channel")
	os.Setenv("SLACK_USERNAME", "slack_username")

	expected := slack{
		client:   "slack_token",
		channel:  "slack_channel",
		username: "slack_username",
	}
	actual := newSlack()
	assert.Equal(t, expected, actual)

	os.Unsetenv("SLACK_CHANNEL")
	os.Unsetenv("SLACK_USERNAME")

	actual = newSlack()
	expected = slack{
		client:   "slack_token",
		channel:  "",
		username: "",
	}
	assert.Equal(t, expected, actual)

	// For panic test
	defer func() {
		err := recover()
		if err != "please set slack client" {
			t.Errorf("got %v\nwant %v", err, "please set slack client")
		}
	}()
	os.Unsetenv("SLACK_TOKEN")
	actual = newSlack()
}
