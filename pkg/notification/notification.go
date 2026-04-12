package notification

import (
	"fmt"
	"os"
	"time"

	"github.com/Songmu/flextime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MessageTemplateParam struct {
	JobName        string
	CronJobName    string
	Namespace      string
	StartTime      *metav1.Time
	CompletionTime *metav1.Time
	ExecutionTime  time.Duration
	Log            string
	Annotations    map[string]string
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
	NotifyStart(messageParam MessageTemplateParam) (err error)
	NotifySuccess(messageParam MessageTemplateParam) (err error)
	NotifyFailed(messageParam MessageTemplateParam) (err error)
}

func NewNotifications() (map[string]Notification, error) {
	res := make(map[string]Notification)
	if os.Getenv("SLACK_ENABLED") == "true" {
		s, err := newSlack()
		if err != nil {
			return nil, fmt.Errorf("failed to create slack notification: %w", err)
		}
		res["slack"] = s
	}
	if os.Getenv("MSTEAMSV2_ENABLED") == "true" {
		m, err := newMsTeamsV2()
		if err != nil {
			return nil, fmt.Errorf("failed to create msteamsv2 notification: %w", err)
		}
		res["msteamsv2"] = m
	}
	return res, nil
}
