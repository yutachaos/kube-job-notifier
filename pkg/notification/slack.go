package notification

import (
	"bytes"
	slackapi "github.com/slack-go/slack"
	"html/template"
	"k8s.io/klog"
	"os"
)

const (
	START                = "start"
	SUCCESS              = "success"
	FAILED               = "failed"
	SlackMessageTemplate = `
{{if .CronJobName}} *CronJobName*: {{.CronJobName}}{{end}}
 *JobName*: {{.JobName}}
{{if .Namespace}} *Namespace*: {{.Namespace}}{{end}}
{{if .StartTime }} *StartTime*: {{.StartTime.Format "2006/1/2 15:04:05 UTC"}}{{end}}
{{if .CompletionTime }} *CompletionTime*: {{.CompletionTime.Format "2006/1/2 15:04:05 UTC"}}{{end}}
{{if .ExecutionTime }} *ExecutionTime*: {{.ExecutionTime}}{{end}}
{{if .Log }} *Loglink*: {{.Log}}{{end}}`

	defaultAnnotationName         = "kube-job-notifier/default-channel"
	successAnnotationName         = "kube-job-notifier/success-channel"
	startedAnnotationName         = "kube-job-notifier/started-channel"
	failedAnnotationName          = "kube-job-notifier/failed-channel"
	suppressSuccessAnnotationName = "kube-job-notifier/suppress-success-notification"
	suppressStartedAnnotationName = "kube-job-notifier/suppress-started-notification"
	suppressFailedAnnotationName  = "kube-job-notifier/suppress-failed-notification"
)

var slackColors = map[string]string{
	"Normal":  "good",
	"Warning": "warning",
	"Danger":  "danger",
}

type slack struct {
	client   *slackapi.Client
	channel  string
	username string
}

func newSlack() slack {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		panic("please set slack client")
	}

	client := slackapi.New(token)

	channel := os.Getenv("SLACK_CHANNEL")

	username := os.Getenv("SLACK_USERNAME")

	return slack{
		client:   client,
		channel:  channel,
		username: username,
	}

}

func (s slack) NotifyStart(messageParam MessageTemplateParam) (err error) {

	if !isNotifyFromEnv("SLACK_STARTED_NOTIFY") {
		return nil
	}

	if isNotificationSuppressed(messageParam.Annotations, suppressStartedAnnotationName) {
		klog.Infof("Notification for %s is suppressed", messageParam.JobName)
		return nil
	}

	succeedChannel := os.Getenv("SLACK_SUCCEED_CHANNEL")
	if succeedChannel != "" {
		s.channel = succeedChannel
	}
	slackChannel := getSlackChannel(messageParam.Annotations, startedAnnotationName)
	if slackChannel != "" {
		s.channel = slackChannel
	}

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

	if !isNotifyFromEnv("SLACK_SUCCEEDED_NOTIFY") {
		return nil
	}

	if isNotificationSuppressed(messageParam.Annotations, suppressSuccessAnnotationName) {
		klog.Infof("Notification for %s is suppressed", messageParam.JobName)
		return nil
	}

	succeedChannel := os.Getenv("SLACK_SUCCEED_CHANNEL")
	if succeedChannel != "" {
		s.channel = succeedChannel
	}
	slackChannel := getSlackChannel(messageParam.Annotations, successAnnotationName)
	if slackChannel != "" {
		s.channel = slackChannel
	}
	if messageParam.Log != "" {
		file, err := s.uploadLog(messageParam)
		if err != nil {
			klog.Errorf("Template execute failed %s\n", err)
			return err
		}
		messageParam.Log = file.Permalink
	}

	messageParam.CompletionTime, messageParam.ExecutionTime = messageParam.calculateExecutionTime()

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

	if !isNotifyFromEnv("SLACK_FAILED_NOTIFY") {
		return nil
	}

	if isNotificationSuppressed(messageParam.Annotations, suppressFailedAnnotationName) {
		klog.Infof("Notification for %s is suppressed", messageParam.JobName)
		return nil
	}

	failedChannel := os.Getenv("SLACK_FAILED_CHANNEL")
	if failedChannel != "" {
		s.channel = failedChannel
	}
	slackChannel := getSlackChannel(messageParam.Annotations, failedAnnotationName)
	if slackChannel != "" {
		s.channel = slackChannel
	}
	if messageParam.Log != "" {
		file, err := s.uploadLog(messageParam)
		if err != nil {
			klog.Errorf("Template execute failed %s\n", err)
			return err
		}
		messageParam.Log = file.Permalink
	}

	messageParam.CompletionTime, messageParam.ExecutionTime = messageParam.calculateExecutionTime()

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

func getSlackChannel(annotations map[string]string, annotationName string) string {
	slackChannel, ok := annotations[annotationName]
	if !ok {
		return annotations[defaultAnnotationName]
	}
	return slackChannel
}

func isNotificationSuppressed(annotations map[string]string, annotationName string) bool {
	a, ok := annotations[annotationName]
	if !ok {
		return false
	}
	return a == "true"
}

func (s slack) notify(attachment slackapi.Attachment) (err error) {

	channelID, timestamp, err := s.client.PostMessage(
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

func (s slack) uploadLog(param MessageTemplateParam) (file *slackapi.File, err error) {
	file, err = s.client.UploadFile(
		slackapi.FileUploadParameters{
			Title:    param.Namespace + "_" + param.JobName,
			Content:  param.Log,
			Filetype: "txt",
			Channels: []string{s.channel},
		})
	if err != nil {
		klog.Errorf("File uploadLog failed %s\n", err)
		return
	}

	klog.Infof("File uploadLog successfully %s", file.Name)
	return
}

func isNotifyFromEnv(key string) bool {
	value := os.Getenv(key)
	if value == "false" {
		return false
	}
	return true
}
