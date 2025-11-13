package notification

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"

	slackapi "github.com/slack-go/slack"
	"k8s.io/klog"
)

const (
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

type slackClient interface {
	PostMessage(channelID string, options ...slackapi.MsgOption) (string, string, error)
	UploadFileV2Context(ctx context.Context, params slackapi.UploadFileV2Parameters) (file *slackapi.FileSummary, err error)
	GetFileInfoContext(ctx context.Context, fileID string, count, page int) (*slackapi.File, []slackapi.Comment, *slackapi.Paging, error)
	GetConversationsContext(ctx context.Context, params *slackapi.GetConversationsParameters) (channels []slackapi.Channel, nextCursor string, err error)
}

type slack struct {
	client    slackClient
	channel   string
	channelID string
	username  string
}

func newSlack() slack {
	ctx := context.Background()
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		panic("please set slack client")
	}

	newSlack := slackapi.New(token)

	client := slack{
		client: newSlack,
	}
	channel := os.Getenv("SLACK_CHANNEL")

	channelID := client.getChannelID(ctx, channel)
	if channelID == "" {
		channelID = channel
	}
	client.channelID = channelID

	username := os.Getenv("SLACK_USERNAME")

	client.username = username
	client.channel = channel

	return client
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
	ctx := context.Background()
	content := param.Log
	filename := param.Namespace + "_" + param.JobName + ".txt"

	fileSize := len([]byte(content))
	if fileSize == 0 {
		return nil, fmt.Errorf("file size cannot be 0")
	}

	if filename == "" {
		return nil, fmt.Errorf("filename cannot be empty")
	}

	if s.channel == "" {
		return nil, fmt.Errorf("channel cannot be empty")
	}

	// Use filename if title is empty
	title := param.Namespace + "_" + param.JobName
	if title == "" || title == "_" {
		title = filename
	}

	params := slackapi.UploadFileV2Parameters{
		Title:    title,
		Content:  content,
		FileSize: fileSize,
		Filename: filename,
		Channel:  s.channelID,
	}

	klog.V(4).Infof("Uploading file: title=%s, filename=%s, fileSize=%d, channel=%s, channelID=%s)", title, filename, fileSize, s.channel, s.channelID)

	fileSummary, err := s.client.UploadFileV2Context(ctx, params)
	if err != nil {
		klog.Errorf("File uploadLog failed: %v (title=%s, filename=%s, fileSize=%d, channel=%s, channelID=%s, contentLength=%d)\n",
			err, title, filename, fileSize, s.channel, s.channelID, len(content))
		return
	}

	// Get complete File information from FileSummary to get Permalink
	fileInfo, _, _, err := s.client.GetFileInfoContext(ctx, fileSummary.ID, 0, 0)
	if err != nil {
		klog.Errorf("Get file info failed %s\n", err)
		return
	}

	klog.Infof("File uploadLog successfully %s", fileInfo.Name)
	return fileInfo, nil
}

func isNotifyFromEnv(key string) bool {
	value := os.Getenv(key)
	return value != "false"
}

// getChannelID converts a channel name (e.g., "#channel-name") to a channel ID (e.g., "C1234567890")
// If the input is already a channel ID or lookup fails, it returns the original value
func (s slack) getChannelID(ctx context.Context, channel string) string {
	// If channel starts with 'C', 'G', or 'D', it's likely already a channel ID
	if len(channel) > 0 && (channel[0] == 'C' || channel[0] == 'G' || channel[0] == 'D') {
		return channel
	}

	// Remove '#' prefix if present
	channelName := channel
	if len(channelName) > 0 && channelName[0] == '#' {
		channelName = channelName[1:]
	}

	// Try to find the channel by name
	params := &slackapi.GetConversationsParameters{
		Types:  []string{"public_channel", "private_channel"},
		Limit:  1000,
		Cursor: "",
	}

	for {
		channels, nextCursor, err := s.client.GetConversationsContext(ctx, params)
		if err != nil {
			klog.V(4).Infof("Failed to get conversations: %v", err)
			return ""
		}

		for _, ch := range channels {
			if ch.Name == channelName {
				klog.V(4).Infof("Found channel ID %s for channel name %s", ch.ID, channelName)
				return ch.ID
			}
		}

		if nextCursor == "" {
			break
		}
		params.Cursor = nextCursor
	}

	klog.V(4).Infof("Channel ID not found for channel name %s", channelName)
	return ""
}
