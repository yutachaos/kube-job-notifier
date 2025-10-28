package notification

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"k8s.io/klog"
)

const (
	colorRed   = "Attention"
	colorGreen = "Good"
	colorGrey  = "Warning"

	TeamsMessageTemplate = `
{{if .CronJobName}}**CronJobName**: {{.CronJobName}}{{end}}
**JobName**: {{.JobName}}
{{if .Namespace}}**Namespace**: {{.Namespace}}{{end}}
{{if .StartTime }}**StartTime**: {{.StartTime.Format "2006-01-02 15:04:05 -07:00"}}{{end}}
{{if .CompletionTime }}**CompletionTime**: {{.CompletionTime.Format "2006-01-02 15:04:05 -07:00"}}{{end}}
{{if .ExecutionTime }}**ExecutionTime**: {{.ExecutionTime}}{{end}}`
)

// https://learn.microsoft.com/en-us/connectors/teams/?tabs=text1#adaptivecarditemschema
type Content struct {
	Schema  string  `json:"$schema"`
	Type    string  `json:"type"`
	Version string  `json:"version"`
	Body    []Body  `json:"body"`
	Msteams Msteams `json:"msteams,omitempty"`
}

type Body struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Weight string `json:"weight,omitempty"`
	Size   string `json:"size,omitempty"`
	Wrap   bool   `json:"wrap,omitempty"`
	Style  string `json:"style,omitempty"`
	Color  string `json:"color,omitempty"`
}

type Msteams struct {
	Width string `json:"width"`
}

type Attachment struct {
	ContentType string  `json:"contentType"`
	ContentURL  *string `json:"contentUrl"` // Use a pointer to handle null values
	Content     Content `json:"content"`
}

type TeamsMessage struct {
	Type        string       `json:"type"`
	Attachments []Attachment `json:"attachments"`
}

type MsTeamsV2 struct {
	webhookURL string
}

func newMsTeamsV2() MsTeamsV2 {
	webhookURL := os.Getenv("MSTEAMSV2_WEBHOOK_URL")
	if webhookURL == "" {
		panic("please set webhook URL for MSTeamsV2")
	}
	return MsTeamsV2{
		webhookURL: webhookURL,
	}
}

func getTeamsMessage(messageParam MessageTemplateParam) (slackMessage string, err error) {
	var b bytes.Buffer
	tpl, err := template.New("teams").Parse(TeamsMessageTemplate)
	if err != nil {
		return "", err
	}
	err = tpl.Execute(&b, messageParam)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

// NotifyStart implements Notification.
func (m MsTeamsV2) NotifyStart(messageParam MessageTemplateParam) (err error) {

	return m.SendNotification("Job Start", messageParam, colorGrey)
}

// NotifySuccess implements Notification.
func (m MsTeamsV2) NotifySuccess(messageParam MessageTemplateParam) (err error) {
	messageParam.CompletionTime, messageParam.ExecutionTime = messageParam.calculateExecutionTime()

	return m.SendNotification("Job Succeeded", messageParam, colorGreen)
}

// NotifyFailed implements Notification.
func (m MsTeamsV2) NotifyFailed(messageParam MessageTemplateParam) (err error) {
	messageParam.CompletionTime, messageParam.ExecutionTime = messageParam.calculateExecutionTime()

	return m.SendNotification("Job Failed", messageParam, colorRed)
}

func (m MsTeamsV2) SendNotification(title string, messageParam MessageTemplateParam, color string) (err error) {
	message, err := getTeamsMessage(messageParam)
	if err != nil {
		klog.Errorf("Template execute failed %s\n", err)
		return err
	}

	var body io.Reader
	t := m.GetTeamsPayload(title, message, color)
	var payload bytes.Buffer
	if err = json.NewEncoder(&payload).Encode(t); err != nil {
		return err
	}

	body = &payload

	resp, err := http.Post(m.webhookURL, "application/json", body)
	if resp != nil {
		klog.Infof("HTTP Response Status: %s", resp.Status)
	}
	if err != nil {
		return err
	}
	return nil
}

func (m MsTeamsV2) GetTeamsPayload(title string, text string, color string) TeamsMessage {
	// Replace single line breaks with double for Adaptive Cards
	formattedText := strings.ReplaceAll(text, "\n", "\n\n")

	return TeamsMessage{
		Type: "message",
		Attachments: []Attachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: Content{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: "1.4",
					Body: []Body{
						{
							Type:   "TextBlock",
							Text:   title,
							Weight: "Bolder",
							Size:   "Medium",
							Wrap:   true,
							Style:  "heading",
							Color:  color,
						},
						{
							Type: "TextBlock",
							Text: formattedText,
							Wrap: true,
						},
					},
					Msteams: Msteams{
						Width: "full",
					},
				},
			},
		},
	}
}
