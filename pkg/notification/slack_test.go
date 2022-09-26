package notification

import (
	"github.com/Songmu/flextime"
	slackapi "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
	"time"
)

func TestNewSlack(t *testing.T) {
	os.Setenv("SLACK_TOKEN", "slack_token")
	os.Setenv("SLACK_CHANNEL", "slack_channel")
	os.Setenv("SLACK_USERNAME", "slack_username")

	expected := slack{
		client:   slackapi.New("slack_token"),
		channel:  "slack_channel",
		username: "slack_username",
	}
	actual := newSlack()
	assert.Equal(t, expected, actual)

	os.Unsetenv("SLACK_CHANNEL")
	os.Unsetenv("SLACK_USERNAME")

	actual = newSlack()
	expected = slack{
		client:   slackapi.New("slack_token"),
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

func TestGetSlackMessage(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	restore := flextime.Set(mockTime)
	defer restore()
	startTime := &metav1.Time{Time: flextime.Now()}
	completionTime := &metav1.Time{Time: startTime.Add(1 * time.Minute)}
	input := MessageTemplateParam{
		JobName:        "Job",
		CronJobName:    "CronJob",
		Namespace:      "namespace",
		StartTime:      startTime,
		CompletionTime: completionTime,
		ExecutionTime:  0,
		Log:            "Log",
	}

	input.CompletionTime, input.ExecutionTime = input.calculateExecutionTime()

	actual, err := getSlackMessage(input)

	assert.Empty(t, err)
	expect := `
 *CronJobName*: CronJob
 *JobName*: Job
 *Namespace*: namespace
 *StartTime*: 2020/11/28 01:02:03 UTC
 *CompletionTime*: 2020/11/28 01:03:03 UTC
 *ExecutionTime*: 1m0s
 *Loglink*: Log`
	assert.Equal(t, expect, actual)

	mockTime = time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC).
		Add(1*time.Hour + 30*time.Minute + 1*time.Millisecond)

	restore = flextime.Set(mockTime)
	defer restore()

	input = MessageTemplateParam{
		JobName:        "Job",
		CronJobName:    "CronJob",
		Namespace:      "namespace",
		StartTime:      startTime,
		CompletionTime: nil,
		ExecutionTime:  0,
		Log:            "Log",
	}

	input.CompletionTime, input.ExecutionTime = input.calculateExecutionTime()

	actual, err = getSlackMessage(input)
	assert.Empty(t, err)
	expect = `
 *CronJobName*: CronJob
 *JobName*: Job
 *Namespace*: namespace
 *StartTime*: 2020/11/28 01:02:03 UTC
 *CompletionTime*: 2020/11/28 02:32:03 UTC
 *ExecutionTime*: 1h30m0s
 *Loglink*: Log`
	assert.Equal(t, expect, actual)
}

func TestGetSlackChannel(t *testing.T) {
	tests := []struct {
		Name              string
		annotations       map[string]string
		channelAnnotation string
		expected          string
	}{
		{
			"No annotations",
			map[string]string{
				"kube-job-notifier/foo": "bar",
			},
			"kube-job-notifier/success-channel",
			"",
		},
		{
			"Default channel",
			map[string]string{
				"kube-job-notifier/default-channel": "job-alerts",
			},
			"kube-job-notifier/success-channel",
			"job-alerts",
		},
		{
			"Success channel",
			map[string]string{
				"kube-job-notifier/default-channel": "job-alerts",
				"kube-job-notifier/success-channel": "job-alerts-success",
			},
			"kube-job-notifier/success-channel",
			"job-alerts-success",
		},
		{
			"Nil annotation not break",
			nil,
			"kube-job-notifier/suppress-success-notification",
			"",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			result := getSlackChannel(test.annotations, test.channelAnnotation)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestIsNotificationSuppressed(t *testing.T) {
	tests := []struct {
		Name                   string
		annotations            map[string]string
		suppressAnnotationName string
		expected               bool
	}{
		{
			"No annotations",
			map[string]string{
				"kube-job-notifier/foo": "bar",
			},
			"kube-job-notifier/suppress-success-notification",
			false,
		},
		{
			"Annotation not true",
			map[string]string{
				"kube-job-notifier/suppress-success-notification": "false",
			},
			"kube-job-notifier/suppress-success-notification",
			false,
		},
		{
			"Annotation true",
			map[string]string{
				"kube-job-notifier/suppress-success-notification": "true",
			},
			"kube-job-notifier/suppress-success-notification",
			true,
		},
		{
			"Nil annotation not break",
			nil,
			"kube-job-notifier/suppress-success-notification",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			result := isNotificationSuppressed(test.annotations, test.suppressAnnotationName)

			assert.Equal(t, test.expected, result)
		})
	}
}
