package notification

import (
	slackapi "github.com/nlopes/slack"
	"k8s.io/klog"
	"os"
)

const (
	START   = "start"
	SUCCESS = "success"
	FAILED  = "failed"
)

var slackColors = map[string]string{
	"Normal":  "good",
	"Warning": "warning",
	"Danger":  "danger",
}

type slack struct {
	token   string
	channel string
}

type Slack interface {
	NotifyStart(message string) (err error)
	NotifySuccess(message string, log string) (err error)
	NotifyFailed(message string, log string) (err error)
	notify(attachment slackapi.Attachment, log string) (err error)
}

func NewSlack() Slack {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		panic("please set slack token")
	}
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" {
		panic("please set slack channel")
	}

	return slack{token: token, channel: channel}
}

func (s slack) NotifyStart(message string) (err error) {
	attachment := slackapi.Attachment{
		Color: slackColors["Normal"],
		Title: "Job Start",
		Text:  message,
	}
	err = s.notify(attachment, "")
	if err != nil {
		return err
	}
	return nil
}

func (s slack) NotifySuccess(message string, log string) (err error) {
	attachment := slackapi.Attachment{
		Color: slackColors["Normal"],
		Title: "Job Success",
		Text:  message,
	}
	err = s.notify(attachment, log)
	if err != nil {
		return err
	}
	return nil
}

func (s slack) NotifyFailed(message string, log string) (err error) {
	attachment := slackapi.Attachment{
		Color: slackColors["Danger"],
		Title: "Job Failed",
		Text:  message,
	}
	err = s.notify(attachment, log)
	if err != nil {
		return err
	}
	return nil
}

func (s slack) notify(attachment slackapi.Attachment, log string) (err error) {
	api := slackapi.New(s.token)
	if log != "" {
		attachment.Fields = []slackapi.AttachmentField{
			{
				Title: "log",
				Value: log,
				Short: true,
			},
		}
	}
	channelID, timestamp, err := api.PostMessage(
		s.channel,
		slackapi.MsgOptionText("", true),
		slackapi.MsgOptionAttachments(attachment),
	)

	if err != nil {
		klog.Errorf("Send message failed %s\n", err)
		return
	}

	klog.Infof("Message successfully sent to channel %s at %s", channelID, timestamp)
	return err
}
