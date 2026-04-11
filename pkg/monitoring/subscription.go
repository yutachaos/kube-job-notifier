package monitoring

import "os"

type JobInfo struct {
	Name        string
	CronJobName string
	Namespace   string
	Annotations map[string]string
}

func (j JobInfo) getJobName() string {
	if j.CronJobName != "" {
		return j.CronJobName
	}
	return j.Name
}

type Subscription interface {
	SuccessEvent(jobInfo JobInfo) (err error)
	FailEvent(jobInfo JobInfo) (err error)
}

// NewSubscription Support for returning multiple event notifications in one
func NewSubscription() map[string]Subscription {
	res := make(map[string]Subscription)
	if os.Getenv("DATADOG_ENABLE") == "true" {
		res["datadog"] = newDatadog()
	}
	return res
}
