package notification

import (
	"github.com/remmercier/kube-job-notifier/pkg/httpclient"
)

const (
	colorRed   = "Attention"
	colorGreen = "Good"
	colorGrey  = "Warning"
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
	return MsTeamsV2{}
}

// NotifyFailed implements Notification.
func (m MsTeamsV2) NotifyFailed(messageParam MessageTemplateParam) (err error) {
	panic("unimplemented")
}

// NotifyStart implements Notification.
func (m MsTeamsV2) NotifyStart(messageParam MessageTemplateParam) (err error) {
	panic("unimplemented")
}

// NotifySuccess implements Notification.
func (m MsTeamsV2) NotifySuccess(messageParam MessageTemplateParam) (err error) {
	panic("unimplemented")
}

func (m MsTeamsV2) GetTeamsMessage(title string, text string, color string) TeamsMessage {
	return TeamsMessage{
		Type: "message",
		Attachments: []Attachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: Content{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: "1.2",
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
							Text: text,
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
