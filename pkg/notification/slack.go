package notification

import (
	"bytes"
	"html/template"
	"os"

	slackapi "github.com/nlopes/slack"
	"k8s.io/klog"
)

const (
	START                = "start"
	SUCCESS              = "success"
	FAILED               = "failed"
	SlackMessageTemplate = `

*JobName*: {{.JobName}}
*Namespace*: {{.Namespace}}

{{if .Log }}*Log*: {{.Log}}{{end}}

`
)

var slackColors = map[string]string{
	"Normal":  "good",
	"Warning": "warning",
	"Danger":  "danger",
}

type slack struct {
	token    string
	channel  string
	username string
}

type MessageTemplateParam struct {
	JobName   string
	Namespace string
	Log       string
}

type Slack interface {
	NotifyStart(messageParam MessageTemplateParam) (err error)
	NotifySuccess(messageParam MessageTemplateParam) (err error)
	NotifyFailed(messageParam MessageTemplateParam) (err error)
	notify(attachment slackapi.Attachment) (err error)
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

	username := os.Getenv("SLACK_USERNAME")

	return slack{token: token, channel: channel, username: username}
}

func (s slack) NotifyStart(messageParam MessageTemplateParam) (err error) {

	slackMessage, err := getSlackMessage(messageParam)
	if err != nil {
		klog.Errorf("Template execute failed %s\n", err)
		return err
	}
	attachment := slackapi.Attachment{

		Color: slackColors["Normal"],
		Title: "Job Start",
		Text:  slackMessage,
	}
	err = s.notify(attachment)
	if err != nil {
		return err
	}
	return nil
}

func getSlackMessage(messageParam MessageTemplateParam) (slackMessage string, err error) {
	var b bytes.Buffer
	tpl, err := template.New("slack").Parse(SlackMessageTemplate)
	if err != nil {
		return "", err
	}
	err = tpl.Execute(&b, messageParam)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func (s slack) NotifySuccess(messageParam MessageTemplateParam) (err error) {
	slackMessage, err := getSlackMessage(messageParam)
	if err != nil {
		klog.Errorf("Template execute failed %s\n", err)
		return err
	}
	attachment := slackapi.Attachment{
		Color: slackColors["Normal"],
		Title: "Job Success",
		Text:  slackMessage,
	}
	err = s.notify(attachment)
	if err != nil {
		return err
	}
	return nil
}

func (s slack) NotifyFailed(messageParam MessageTemplateParam) (err error) {
	slackMessage, err := getSlackMessage(messageParam)
	if err != nil {
		klog.Errorf("Template execute failed %s\n", err)
		return err
	}
	attachment := slackapi.Attachment{
		Color: slackColors["Danger"],
		Title: "Job Failed",
		Text:  slackMessage,
	}
	err = s.notify(attachment)
	if err != nil {
		return err
	}
	return nil
}

func (s slack) notify(attachment slackapi.Attachment) (err error) {
	api := slackapi.New(s.token)

	channelID, timestamp, err := api.PostMessage(
		s.channel,
		slackapi.MsgOptionText("", true),
		slackapi.MsgOptionAttachments(attachment),
		slackapi.MsgOptionUsername(s.username),
	)

	if err != nil {
		klog.Errorf("Send messageParam failed %s\n", err)
		return
	}

	klog.Infof("Message successfully sent to channel %s at %s", channelID, timestamp)
	return err
}
