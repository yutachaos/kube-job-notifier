package notification

import (
	"github.com/Songmu/flextime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type MessageTemplateParam struct {
	JobName        string
	CronJobName    string
	Namespace      string
	StartTime      *metav1.Time
	CompletionTime *metav1.Time
	ExecutionTime  time.Duration
	Log            string
}

func (m MessageTemplateParam) calculateExecutionTime() (completionTime *metav1.Time, executionTime time.Duration) {
	completionTime = m.CompletionTime
	if m.StartTime != nil {
		if completionTime == nil {
			completionTime = &metav1.Time{Time: flextime.Now()}
		}
		executionTime = completionTime.Sub(m.StartTime.Time)
	}
	return completionTime, executionTime.Truncate(time.Second)
}

type Notification interface {
	NotifyStart(messageParam MessageTemplateParam, slackChannel string) (err error)
	NotifySuccess(messageParam MessageTemplateParam, slackChannel string) (err error)
	NotifyFailed(messageParam MessageTemplateParam, slackChannel string) (err error)
}

func NewNotifications() map[string]Notification {
	res := make(map[string]Notification)
	// default notification
	res["slack"] = newSlack()
	return res
}
