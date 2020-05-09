package notification

type MessageTemplateParam struct {
	JobName   string
	Namespace string
	Log       string
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
