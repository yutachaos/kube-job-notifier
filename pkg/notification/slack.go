package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	slackapi "github.com/slack-go/slack"
	"html/template"
	"io"
	"k8s.io/klog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// httpDo is a package-level variable to allow tests to stub HTTP requests easily.
var httpDo = func(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

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

type slackClient interface {
	PostMessage(channelID string, options ...slackapi.MsgOption) (string, string, error)
	// Deprecated in Slack API: kept for backward compatibility in tests, but no longer used
	UploadFile(params slackapi.FileUploadParameters) (file *slackapi.File, err error)
}

type slack struct {
	client   slackClient
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
	// External upload flow per Slack deprecation of files.upload
	// 1) Call files.getUploadURLExternal to obtain an upload URL and file ID
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("SLACK_TOKEN is not set")
	}

	title := param.Namespace + "_" + param.JobName
	content := param.Log
	contentBytes := []byte(content)
	length := len(contentBytes)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: get upload URL
	form := url.Values{}
	form.Set("filename", title+".txt")
	form.Set("length", strconv.Itoa(length))
	getURLReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/files.getUploadURLExternal", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	getURLReq.Header.Set("Authorization", "Bearer "+token)
	getURLReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpDo(getURLReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("files.getUploadURLExternal failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var getURLRes struct {
		OK        bool   `json:"ok"`
		UploadURL string `json:"upload_url"`
		FileID    string `json:"file_id"`
		Error     string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&getURLRes); err != nil {
		return nil, err
	}
	if !getURLRes.OK {
		return nil, fmt.Errorf("files.getUploadURLExternal error: %s", getURLRes.Error)
	}

	// 2) Upload bytes to the pre-signed URL with PUT
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, getURLRes.UploadURL, bytes.NewReader(contentBytes))
	if err != nil {
		return nil, err
	}
	putReq.Header.Set("Content-Type", "text/plain")
	putReq.Header.Set("Content-Length", strconv.Itoa(length))
	putResp, err := httpDo(putReq)
	if err != nil {
		return nil, err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		b, _ := io.ReadAll(putResp.Body)
		return nil, fmt.Errorf("upload to upload_url failed: status=%d body=%s", putResp.StatusCode, string(b))
	}

	// 3) Complete upload to share into the channel
	completeBody := struct {
		ChannelID string `json:"channel_id"`
		Files     []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"files"`
	}{
		ChannelID: s.channel,
		Files: []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}{{ID: getURLRes.FileID, Title: title + ".txt"}},
	}

	bodyBytes, err := json.Marshal(completeBody)
	if err != nil {
		return nil, err
	}

	completeReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/files.completeUploadExternal", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	completeReq.Header.Set("Authorization", "Bearer "+token)
	completeReq.Header.Set("Content-Type", "application/json")

	completeResp, err := httpDo(completeReq)
	if err != nil {
		return nil, err
	}
	defer completeResp.Body.Close()
	if completeResp.StatusCode < 200 || completeResp.StatusCode >= 300 {
		b, _ := io.ReadAll(completeResp.Body)
		return nil, fmt.Errorf("files.completeUploadExternal failed: status=%d body=%s", completeResp.StatusCode, string(b))
	}

	var completeRes struct {
		OK    bool            `json:"ok"`
		Files []slackapi.File `json:"files"`
		Error string          `json:"error"`
	}
	if err := json.NewDecoder(completeResp.Body).Decode(&completeRes); err != nil {
		return nil, err
	}
	if !completeRes.OK {
		return nil, fmt.Errorf("files.completeUploadExternal error: %s", completeRes.Error)
	}
	if len(completeRes.Files) == 0 {
		return nil, fmt.Errorf("files.completeUploadExternal returned no files")
	}

	klog.Infof("File uploadLog successfully %s", completeRes.Files[0].Name)
	return &completeRes.Files[0], nil
}

func isNotifyFromEnv(key string) bool {
	value := os.Getenv(key)
	if value == "false" {
		return false
	}
	return true
}
