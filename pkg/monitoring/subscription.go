package monitoring

type JobInfo struct {
	Name      string
	Namespace string
}

type Subscription interface {
	SuccessEvent(jobInfo JobInfo) (err error)
	FailEvent(jobInfo JobInfo) (err error)
}

// Support for returning multiple event notifications in one
func NewSubscription() map[string]Subscription {
	res := make(map[string]Subscription)
	// default notification
	res["datadog"] = newDatadog()
	return res
}
