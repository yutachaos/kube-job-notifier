# kube-job-notifier

A tool for monitoring Kubernetes job execution status and sending notifications to Slack and Datadog.

## Features

- Notifications for Kubernetes job start, success, and failure
- Slack notifications with log attachments
- Datadog service check notifications
- Support for multiple container log collection
- Per-job notification customization
- Easy deployment with Helm charts

## Installation

### Installation with Helm (Recommended)

```bash
helm repo add kube-job-notifier https://yutachaos.github.io/kube-job-notifier/
helm install kube-job-notifier/kube-job-notifier --generate-name
```

### Installation with Manifests

1. Apply the manifests:
```bash
kubectl apply -f manifests/sample/
```

2. Verify the deployment:
```bash
kubectl get po
```

## Configuration

### Slack Notification Settings

Configure using environment variables:

```bash
export MSTEAMSV2_ENABLED=true
export MSTEAMSV2_WEBHOOK_URL=YOUR_WEBHOOK_URL
export SLACK_ENABLED=true
export SLACK_TOKEN=YOUR_SLACK_TOKEN
export SLACK_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID
export SLACK_STARTED_NOTIFY=true      # Optional, default: true
export SLACK_SUCCEEDED_NOTIFY=true    # Optional, default: true
export SLACK_FAILED_NOTIFY=true       # Optional, default: true
export SLACK_USERNAME=YOUR_NOTIFICATION_USERNAME  # Optional
export SLACK_SUCCEED_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID  # Optional
export SLACK_FAILED_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID   # Optional
export DATADOG_ENABLED=true               # Optional, default: false
export NAMESPACE=KUBERNETES_NAMESPACE     # Optional
export CRONJOB_REGEX=REGEX                # Optional, if empty all cronjobs will be included
```

### Job Annotation Configuration

You can customize notification channels per job:

```
kube-job-notifier/default-channel    - Default notification channel
kube-job-notifier/success-channel    - Channel for successful job notifications
kube-job-notifier/started-channel    - Channel for job start notifications
kube-job-notifier/failed-channel     - Channel for failed job notifications
```

You can also suppress notifications:

```
kube-job-notifier/suppress-success-notification  - Suppress success notifications
kube-job-notifier/suppress-started-notification  - Suppress start notifications
kube-job-notifier/suppress-failed-notification   - Suppress failure notifications
```

### Slack Permission Requirements

The following permissions are required:
- `chat:write`
- `files:write`

### Datadog Integration

- Sends Datadog service checks when jobs succeed or fail
- For more details: https://docs.datadoghq.com/developers/service_checks/dogstatsd_service_checks_submission/

### Multiple Container Log Collection

You can specify log collection mode using the `kube-job-notifier/log-mode` annotation:

- `ownerContainer` - Get logs only from container with the same name as the job (default)
- `podOnly` - Get logs from the pod, works perfectly with single container pods
- `podContainers` - Get logs from all pod containers and concatenate them

## Running Locally

```bash
go run *.go -kubeconfig {YOUR_KUBECONFIG_PATH}
```

## Documentation

For detailed documentation, visit: https://yutachaos.github.io/kube-job-notifier/

