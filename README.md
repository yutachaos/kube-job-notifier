# kube-job-notifier
For kubernetes job notification tool 

## Description
- Custom controller that notifies Kubernetes Job start, success, and failure.

## Usage

### Notification setting(Slack only)
- Please set environment variable
```bash
export SLACK_TOKEN=YOUR_SLACK_TOKEN
export SLACK_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID
```

### Run
- go run *.go -kubeconfig {YOUR_KUBECONFIG_PATH}
 