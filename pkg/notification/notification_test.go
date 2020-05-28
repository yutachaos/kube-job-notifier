package notification

import (
	"github.com/Songmu/flextime"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

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

	mockTime = time.Date(2020, 11, 28, 1, 2, 3, 123456000, time.UTC).Add(1 * time.Hour)
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
	assert.Equal(t, completionTime, actual.CompletionTime)
	assert.NotEmpty(t, actual.ExecutionTime)
}
