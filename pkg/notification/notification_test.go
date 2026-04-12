package notification

import (
	"os"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewNotifications(t *testing.T) {
	t.Run("returns empty map when no env vars set", func(t *testing.T) {
		os.Unsetenv("SLACK_ENABLED")
		os.Unsetenv("MSTEAMSV2_ENABLED")
		notifications, err := NewNotifications()
		assert.NoError(t, err)
		assert.Empty(t, notifications)
	})

	t.Run("returns error when SLACK_ENABLED=true but SLACK_TOKEN missing", func(t *testing.T) {
		t.Setenv("SLACK_ENABLED", "true")
		os.Unsetenv("SLACK_TOKEN")
		_, err := NewNotifications()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "slack")
	})

	t.Run("includes slack when SLACK_ENABLED=true and SLACK_TOKEN set", func(t *testing.T) {
		t.Setenv("SLACK_ENABLED", "true")
		t.Setenv("SLACK_TOKEN", "xoxb-test-token")
		os.Unsetenv("MSTEAMSV2_ENABLED")
		notifications, err := NewNotifications()
		assert.NoError(t, err)
		assert.Contains(t, notifications, "slack")
		assert.NotContains(t, notifications, "msteamsv2")
	})

	t.Run("returns error when MSTEAMSV2_ENABLED=true but webhook URL missing", func(t *testing.T) {
		os.Unsetenv("SLACK_ENABLED")
		t.Setenv("MSTEAMSV2_ENABLED", "true")
		os.Unsetenv("MSTEAMSV2_WEBHOOK_URL")
		_, err := NewNotifications()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "msteamsv2")
	})

	t.Run("includes msteamsv2 when MSTEAMSV2_ENABLED=true and webhook URL set", func(t *testing.T) {
		os.Unsetenv("SLACK_ENABLED")
		t.Setenv("MSTEAMSV2_ENABLED", "true")
		t.Setenv("MSTEAMSV2_WEBHOOK_URL", "https://example.com/webhook")
		notifications, err := NewNotifications()
		assert.NoError(t, err)
		assert.Contains(t, notifications, "msteamsv2")
		assert.NotContains(t, notifications, "slack")
	})
}

func TestSetExecutionTime(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	restore := flextime.Set(mockTime)
	defer restore()
	startTime := &metav1.Time{Time: mockTime}
	completionTime := &metav1.Time{Time: startTime.Add(1 * time.Minute)}
	actual := MessageTemplateParam{
		JobName:        "Job",
		CronJobName:    "CronJob",
		Namespace:      "namespace",
		StartTime:      startTime,
		CompletionTime: completionTime,
		ExecutionTime:  0,
		Log:            "",
	}

	actual.CompletionTime, actual.ExecutionTime = actual.calculateExecutionTime()

	assert.Equal(t, startTime, actual.StartTime)
	assert.Equal(t, completionTime, actual.CompletionTime)
	assert.Equal(t, completionTime.Sub(startTime.Time), actual.ExecutionTime)

	mockTime = time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC).
		Add(1*time.Hour + 30*time.Minute)
	restore = flextime.Set(mockTime)
	defer restore()
	completionTime = &metav1.Time{Time: mockTime}

	actual = MessageTemplateParam{
		JobName:        "Job",
		CronJobName:    "CronJob",
		Namespace:      "namespace",
		StartTime:      startTime,
		CompletionTime: nil,
		ExecutionTime:  0,
		Log:            "",
	}

	actual.CompletionTime, actual.ExecutionTime = actual.calculateExecutionTime()

	assert.Equal(t, startTime, actual.StartTime)
	assert.Equal(t, completionTime.Truncate(time.Second), actual.CompletionTime.Truncate(time.Second))
	assert.NotEmpty(t, actual.ExecutionTime)
}
