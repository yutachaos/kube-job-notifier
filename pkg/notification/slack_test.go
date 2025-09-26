package notification

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	slackapi "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	assert.Equal(t, expected.channel, actual.channel)
	assert.Equal(t, expected.username, actual.username)

	os.Unsetenv("SLACK_CHANNEL")
	os.Unsetenv("SLACK_USERNAME")

	actual = newSlack()
	expected = slack{
		client:   slackapi.New("slack_token"),
		channel:  "",
		username: "",
	}
	assert.Equal(t, expected.channel, actual.channel)
	assert.Equal(t, expected.username, actual.username)
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

func TestNotifyStart(t *testing.T) {
	defaultChannel := "default_channel"
	tests := []struct {
		Name                    string
		startedNotifyChannelEnv string
		succeededChannelEnv     string
		annotations             map[string]string

		expectedChannel string
		notifyCalled    bool
	}{
		{
			"Notify turned off",
			"false",
			"",
			map[string]string{},

			defaultChannel,
			false,
		},
		{
			"Notify suppressed in annotations",
			"true",
			"",
			map[string]string{
				"kube-job-notifier/suppress-started-notification": "true",
			},

			defaultChannel,
			false,
		},
		{
			"Succeeded channel not specifed in environment",
			"true",
			"",
			map[string]string{},

			defaultChannel,
			true,
		},
		{
			"Succeeded channel from environment",
			"true",
			"started-channel",
			map[string]string{},

			"started-channel",
			true,
		},
		{
			"Succeeded channel overwritten in annotations",
			"true",
			"",
			map[string]string{
				"kube-job-notifier/started-channel": "from-annotations",
			},

			"from-annotations",
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			os.Setenv("SLACK_STARTED_NOTIFY", test.startedNotifyChannelEnv)
			os.Setenv("SLACK_SUCCEED_CHANNEL", test.succeededChannelEnv)

			u := "job_notifier"

			mc := &MockSlackClient{}
			if test.notifyCalled {
				mc.On("PostMessage", test.expectedChannel, mock.AnythingOfType("[]slack.MsgOption")).
					Return(test.expectedChannel, "timestamp", nil)
			}

			slack := slack{client: mc, channel: defaultChannel, username: u}

			err := slack.NotifyStart(MessageTemplateParam{
				JobName:     "the-job",
				Annotations: test.annotations,
			})

			assert.NoError(t, err)
			mc.AssertExpectations(t)

			os.Unsetenv("SLACK_STARTED_NOTIFY")
			os.Unsetenv("SLACK_SUCCEED_CHANNEL")
		})
	}
}

func TestNotifySuccess(t *testing.T) {
	defaultChannel := "default_channel"
	tests := []struct {
		Name                     string
		succededNotifyChannelEnv string
		succeededChannelEnv      string
		annotations              map[string]string

		expectedChannel string
		notifyCalled    bool
	}{
		{
			"Notify turned off",
			"false",
			"",
			map[string]string{},

			defaultChannel,
			false,
		},
		{
			"Notify suppressed in annotations",
			"true",
			"",
			map[string]string{
				"kube-job-notifier/suppress-success-notification": "true",
			},

			defaultChannel,
			false,
		},
		{
			"Succeeded channel not specifed in environment",
			"true",
			"",
			map[string]string{},

			defaultChannel,
			true,
		},
		{
			"Succeeded channel from environment",
			"true",
			"succeded-channel",
			map[string]string{},

			"succeded-channel",
			true,
		},
		{
			"Succeeded channel overwritten in annotations",
			"true",
			"",
			map[string]string{
				"kube-job-notifier/success-channel": "from-annotations",
			},

			"from-annotations",
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			os.Setenv("SLACK_SUCCEEDED_NOTIFY", test.succededNotifyChannelEnv)
			os.Setenv("SLACK_SUCCEED_CHANNEL", test.succeededChannelEnv)

			u := "job_notifier"

			mc := &MockSlackClient{}
			if test.notifyCalled {
				mc.On("PostMessage", test.expectedChannel, mock.AnythingOfType("[]slack.MsgOption")).
					Return(test.expectedChannel, "timestamp", nil)
			}

			slack := slack{client: mc, channel: defaultChannel, username: u}

			err := slack.NotifySuccess(MessageTemplateParam{
				JobName:     "the-job",
				Annotations: test.annotations,
			})

			assert.NoError(t, err)
			mc.AssertExpectations(t)

			os.Unsetenv("SLACK_SUCCEEDED_NOTIFY")
			os.Unsetenv("SLACK_SUCCEED_CHANNEL")
		})
	}
}

func TestNotifyFailed(t *testing.T) {
	defaultChannel := "default_channel"
	tests := []struct {
		Name                   string
		failedNotifyChannelEnv string
		failedChannelEnv       string
		annotations            map[string]string

		expectedChannel string
		notifyCalled    bool
	}{
		{
			"Notify turned off",
			"false",
			"",
			map[string]string{},

			defaultChannel,
			false,
		},
		{
			"Notify suppressed in annotations",
			"true",
			"",
			map[string]string{
				"kube-job-notifier/suppress-failed-notification": "true",
			},

			defaultChannel,
			false,
		},
		{
			"Failed channel not specifed in environment",
			"true",
			"",
			map[string]string{},

			defaultChannel,
			true,
		},
		{
			"Failed channel from environment",
			"true",
			"failed-channel",
			map[string]string{},

			"failed-channel",
			true,
		},
		{
			"Failed channel overwritten in annotations",
			"true",
			"",
			map[string]string{
				"kube-job-notifier/failed-channel": "from-annotations",
			},

			"from-annotations",
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			os.Setenv("SLACK_FAILED_NOTIFY", test.failedNotifyChannelEnv)
			os.Setenv("SLACK_FAILED_CHANNEL", test.failedChannelEnv)

			u := "job_notifier"

			mc := &MockSlackClient{}
			if test.notifyCalled {
				mc.On("PostMessage", test.expectedChannel, mock.AnythingOfType("[]slack.MsgOption")).
					Return(test.expectedChannel, "timestamp", nil)
			}

			slack := slack{client: mc, channel: defaultChannel, username: u}

			err := slack.NotifyFailed(MessageTemplateParam{
				JobName:     "the-job",
				Annotations: test.annotations,
			})

			assert.NoError(t, err)
			mc.AssertExpectations(t)

			os.Unsetenv("SLACK_FAILED_NOTIFY")
			os.Unsetenv("SLACK_FAILED_CHANNEL")
		})
	}
}

type MockSlackClient struct {
	mock.Mock
}

func (c *MockSlackClient) PostMessage(channelID string, options ...slackapi.MsgOption) (string, string, error) {
	args := c.Called(channelID, options)
	return args.String(0), args.String(1), args.Error(2)
}

func (c *MockSlackClient) UploadFile(params slackapi.FileUploadParameters) (file *slackapi.File, err error) {
	args := c.Called(params)
	return args.Get(0).(*slackapi.File), args.Error(1)
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

func TestUploadLog_Success(t *testing.T) {
	// stub HTTP
	original := httpDo
	defer func() { httpDo = original }()

	httpDo = func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "slack.com" && req.URL.Path == "/api/files.getUploadURLExternal" && req.Method == http.MethodPost {
			body := []byte(`{"ok":true,"upload_url":"https://uploads.slack.com/abc123","file_id":"F123"}`)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
		if req.URL.Host == "uploads.slack.com" && req.Method == http.MethodPut {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		}
		if req.URL.Host == "slack.com" && req.URL.Path == "/api/files.completeUploadExternal" && req.Method == http.MethodPost {
			body := []byte(`{"ok":true,"files":[{"id":"F123","name":"ns_job.txt","permalink":"https://slack-files.com/TX/F123"}]}`)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}

	os.Setenv("SLACK_TOKEN", "xoxb-test")
	defer os.Unsetenv("SLACK_TOKEN")

	s := slack{client: &MockSlackClient{}, channel: "C123", username: "bot"}
	file, err := s.uploadLog(MessageTemplateParam{Namespace: "ns", JobName: "job", Log: "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "ns_job.txt", file.Name)
	assert.Equal(t, "https://slack-files.com/TX/F123", file.Permalink)
}

func TestUploadLog_TokenMissing(t *testing.T) {
	original := httpDo
	defer func() { httpDo = original }()
	httpDo = func(req *http.Request) (*http.Response, error) {
		t.Fatalf("httpDo should not be called when token is missing")
		return nil, nil
	}

	os.Unsetenv("SLACK_TOKEN")
	s := slack{client: &MockSlackClient{}, channel: "C123", username: "bot"}
	_, err := s.uploadLog(MessageTemplateParam{Namespace: "ns", JobName: "job", Log: "hello"})
	assert.Error(t, err)
}
