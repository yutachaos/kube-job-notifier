package notification

import (
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

type Notification interface {
	NotifyStart(messageParam MessageTemplateParam) (err error)
	NotifySuccess(messageParam MessageTemplateParam) (err error)
	NotifyFailed(messageParam MessageTemplateParam) (err error)
}

func NewNotifications() map[string]Notification {
	res := make(map[string]Notification)
	// default notification
	res["slack"] = newSlack()
	return res
}
