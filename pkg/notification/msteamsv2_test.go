package notification

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewMsTeamsV2(t *testing.T) {
	t.Run("should create MsTeamsV2 with webhook URL", func(t *testing.T) {
		t.Setenv("MSTEAMSV2_WEBHOOK_URL", "https://example.com/webhook")

		msTeams := newMsTeamsV2()

		assert.Equal(t, "https://example.com/webhook", msTeams.webhookURL)
	})

	t.Run("should panic when webhook URL is not set", func(t *testing.T) {
		t.Setenv("MSTEAMSV2_WEBHOOK_URL", "")

		assert.Panics(t, func() {
			newMsTeamsV2()
		})
	})
}

func TestGetTeamsMessage(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	restore := flextime.Set(mockTime)
	defer restore()

	startTime := &metav1.Time{Time: mockTime}
	completionTime := &metav1.Time{Time: startTime.Add(1 * time.Minute)}

	messageParam := MessageTemplateParam{
		JobName:        "test-job",
		CronJobName:    "test-cronjob",
		Namespace:      "default",
		StartTime:      startTime,
		CompletionTime: completionTime,
		ExecutionTime:  1 * time.Minute,
		Log:            "",
	}

	message, err := getTeamsMessage(messageParam)

	assert.NoError(t, err)
	assert.Contains(t, message, "**CronJobName**: test-cronjob")
	assert.Contains(t, message, "**JobName**: test-job")
	assert.Contains(t, message, "**Namespace**: default")
	assert.Contains(t, message, "**StartTime**:")
	assert.Contains(t, message, "**CompletionTime**:")
	assert.Contains(t, message, "**ExecutionTime**: 1m0s")
}

func TestGetTeamsMessageWithoutCronJob(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	startTime := &metav1.Time{Time: mockTime}

	messageParam := MessageTemplateParam{
		JobName:   "test-job",
		Namespace: "default",
		StartTime: startTime,
	}

	message, err := getTeamsMessage(messageParam)

	assert.NoError(t, err)
	assert.NotContains(t, message, "**CronJobName**:")
	assert.Contains(t, message, "**JobName**: test-job")
}

func TestMsTeamsV2_GetTeamsPayload(t *testing.T) {
	msTeams := MsTeamsV2{webhookURL: "https://example.com/webhook"}

	t.Run("should create payload with correct structure", func(t *testing.T) {
		payload := msTeams.GetTeamsPayload("Test Title", "Test Message", colorGreen)

		assert.Equal(t, "message", payload.Type)
		assert.Len(t, payload.Attachments, 1)
		assert.Equal(t, "application/vnd.microsoft.card.adaptive", payload.Attachments[0].ContentType)
		assert.Nil(t, payload.Attachments[0].ContentURL)

		content := payload.Attachments[0].Content
		assert.Equal(t, "http://adaptivecards.io/schemas/adaptive-card.json", content.Schema)
		assert.Equal(t, "AdaptiveCard", content.Type)
		assert.Equal(t, "1.4", content.Version)
		assert.Len(t, content.Body, 2)

		// Check title
		assert.Equal(t, "TextBlock", content.Body[0].Type)
		assert.Equal(t, "Test Title", content.Body[0].Text)
		assert.Equal(t, "Bolder", content.Body[0].Weight)
		assert.Equal(t, "Medium", content.Body[0].Size)
		assert.Equal(t, colorGreen, content.Body[0].Color)

		// Check message
		assert.Equal(t, "TextBlock", content.Body[1].Type)
		assert.Equal(t, "Test Message", content.Body[1].Text)
		assert.True(t, content.Body[1].Wrap)
	})

	t.Run("should format text with double line breaks", func(t *testing.T) {
		payload := msTeams.GetTeamsPayload("Title", "Line1\nLine2\nLine3", colorRed)

		assert.Equal(t, "Line1\n\nLine2\n\nLine3", payload.Attachments[0].Content.Body[1].Text)
	})

	t.Run("should use correct colors", func(t *testing.T) {
		tests := []struct {
			name  string
			color string
		}{
			{"red color", colorRed},
			{"green color", colorGreen},
			{"grey color", colorGrey},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				payload := msTeams.GetTeamsPayload("Title", "Message", tt.color)
				assert.Equal(t, tt.color, payload.Attachments[0].Content.Body[0].Color)
			})
		}
	})
}

func TestMsTeamsV2_NotifyStart(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	restore := flextime.Set(mockTime)
	defer restore()

	var receivedPayload TeamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	msTeams := MsTeamsV2{webhookURL: server.URL}
	startTime := &metav1.Time{Time: mockTime}

	messageParam := MessageTemplateParam{
		JobName:   "test-job",
		Namespace: "default",
		StartTime: startTime,
	}

	err := msTeams.NotifyStart(messageParam)

	assert.NoError(t, err)
	assert.Equal(t, "Job Start", receivedPayload.Attachments[0].Content.Body[0].Text)
	assert.Equal(t, colorGrey, receivedPayload.Attachments[0].Content.Body[0].Color)
}

func TestMsTeamsV2_NotifySuccess(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	restore := flextime.Set(mockTime)
	defer restore()

	var receivedPayload TeamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	msTeams := MsTeamsV2{webhookURL: server.URL}
	startTime := &metav1.Time{Time: mockTime}
	completionTime := &metav1.Time{Time: startTime.Add(1 * time.Minute)}

	messageParam := MessageTemplateParam{
		JobName:        "test-job",
		Namespace:      "default",
		StartTime:      startTime,
		CompletionTime: completionTime,
	}

	err := msTeams.NotifySuccess(messageParam)

	assert.NoError(t, err)
	assert.Equal(t, "Job Succeeded", receivedPayload.Attachments[0].Content.Body[0].Text)
	assert.Equal(t, colorGreen, receivedPayload.Attachments[0].Content.Body[0].Color)
	assert.Contains(t, receivedPayload.Attachments[0].Content.Body[1].Text, "**ExecutionTime**:")
}

func TestMsTeamsV2_NotifyFailed(t *testing.T) {
	mockTime := time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC)
	restore := flextime.Set(mockTime)
	defer restore()

	var receivedPayload TeamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	msTeams := MsTeamsV2{webhookURL: server.URL}
	startTime := &metav1.Time{Time: mockTime}
	completionTime := &metav1.Time{Time: startTime.Add(1 * time.Minute)}

	messageParam := MessageTemplateParam{
		JobName:        "test-job",
		Namespace:      "default",
		StartTime:      startTime,
		CompletionTime: completionTime,
	}

	err := msTeams.NotifyFailed(messageParam)

	assert.NoError(t, err)
	assert.Equal(t, "Job Failed", receivedPayload.Attachments[0].Content.Body[0].Text)
	assert.Equal(t, colorRed, receivedPayload.Attachments[0].Content.Body[0].Color)
	assert.Contains(t, receivedPayload.Attachments[0].Content.Body[1].Text, "**ExecutionTime**:")
}

func TestMsTeamsV2_SendNotificationError(t *testing.T) {
	msTeams := MsTeamsV2{webhookURL: "http://invalid-url-that-does-not-exist.local"}

	messageParam := MessageTemplateParam{
		JobName:   "test-job",
		Namespace: "default",
	}

	err := msTeams.SendNotification("Test", messageParam, colorGreen)

	assert.Error(t, err)
}

func TestMsTeamsV2_SendNotificationWithHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	msTeams := MsTeamsV2{webhookURL: server.URL}

	messageParam := MessageTemplateParam{
		JobName:   "test-job",
		Namespace: "default",
	}

	err := msTeams.SendNotification("Test", messageParam, colorGreen)

	assert.NoError(t, err) // The function doesn't check HTTP status codes
}
