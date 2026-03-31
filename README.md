# kube-job-notifier

A tool for monitoring Kubernetes job execution status and sending notifications to Slack, Microsoft Teams, and Datadog.

## Features

- Notifications for Kubernetes job start, success, and failure
- Slack notifications with log attachments
- Microsoft Teams V2 notifications via Adaptive Cards
- Datadog service check notifications
- Support for multiple container log collection
- Per-job notification customization via Kubernetes annotations
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

### General Settings

| Environment Variable | Required | Default | Description |
|---|---|---|---|
| `NAMESPACE` | No | (all namespaces) | Kubernetes namespace to watch |
| `CRONJOB_REGEX` | No | (all CronJobs) | Regex to filter CronJobs by name; if empty, all CronJobs are included |

### Slack Notification Settings

Set `SLACK_ENABLED=true` to enable Slack notifications.

| Environment Variable | Required | Default | Description |
|---|---|---|---|
| `SLACK_ENABLED` | No | `false` | Enable Slack notifications |
| `SLACK_TOKEN` | Yes (if enabled) | — | Slack Bot OAuth token |
| `SLACK_CHANNEL` | Yes (if enabled) | — | Default notification channel name or ID |
| `SLACK_STARTED_NOTIFY` | No | `true` | Send notification when a job starts |
| `SLACK_SUCCEEDED_NOTIFY` | No | `true` | Send notification when a job succeeds |
| `SLACK_FAILED_NOTIFY` | No | `true` | Send notification when a job fails |
| `SLACK_USERNAME` | No | — | Override the bot display name |
| `SLACK_SUCCEED_CHANNEL` | No | — | Override the channel for success notifications |
| `SLACK_FAILED_CHANNEL` | No | — | Override the channel for failure notifications |

#### Slack Permission Requirements

The Slack bot token must have the following OAuth scopes:
- `chat:write`
- `files:write`

### Microsoft Teams V2 Notification Settings

Set `MSTEAMSV2_ENABLED=true` to enable Microsoft Teams notifications via Incoming Webhook.

| Environment Variable | Required | Default | Description |
|---|---|---|---|
| `MSTEAMSV2_ENABLED` | No | `false` | Enable Microsoft Teams V2 notifications |
| `MSTEAMSV2_WEBHOOK_URL` | Yes (if enabled) | — | Incoming Webhook URL for the Teams channel |

Messages are sent as Adaptive Cards with color-coded headings:
- Job Start — grey
- Job Succeeded — green
- Job Failed — red

To obtain a webhook URL, follow the [Microsoft Teams Incoming Webhook documentation](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook).

### Datadog Integration

Set `DATADOG_ENABLED=true` to enable Datadog service check reporting.

| Environment Variable | Required | Default | Description |
|---|---|---|---|
| `DATADOG_ENABLED` | No | `false` | Enable Datadog service checks |
| `DD_TAGS` | No | — | Tags to attach to all service checks (comma-separated) |
| `DD_NAMESPACE` | No | — | Prefix namespace for metric names |

Service checks are submitted via DogStatsD over the Unix socket at `/var/run/datadog/dsd.socket` using the service check name `kube_job_notifier.job.status`.

For more details: https://docs.datadoghq.com/developers/service_checks/dogstatsd_service_checks_submission/

### Job Annotation Configuration

Annotations are set on Kubernetes Job or CronJob resources.

#### Channel Routing (Slack only)

| Annotation | Description |
|---|---|
| `kube-job-notifier/default-channel` | Default fallback channel for all Slack notification types |
| `kube-job-notifier/started-channel` | Override channel for job start notifications |
| `kube-job-notifier/success-channel` | Override channel for job success notifications |
| `kube-job-notifier/failed-channel` | Override channel for job failure notifications |

#### Notification Suppression (Slack)

| Annotation | Value | Description |
|---|---|---|
| `kube-job-notifier/suppress-started-notification` | `"true"` | Suppress Slack start notification for this job |
| `kube-job-notifier/suppress-success-notification` | `"true"` | Suppress Slack success notification for this job |
| `kube-job-notifier/suppress-failed-notification` | `"true"` | Suppress Slack failure notification for this job |

#### Notification Suppression (Datadog)

| Annotation | Value | Description |
|---|---|---|
| `kube-job-notifier/suppress-success-datadog-subscription` | `"true"` | Suppress Datadog service check on job success |
| `kube-job-notifier/suppress-failed-datadog-subscription` | `"true"` | Suppress Datadog service check on job failure |

### Multiple Container Log Collection

Set via the `kube-job-notifier/log-mode` annotation on the Job or CronJob resource.

| Value | Description |
|---|---|
| `ownerContainer` | (default) Collect logs only from the container whose name matches the job name |
| `podOnly` | Collect logs from the pod as a whole; works well for single-container pods |
| `podContainers` | Collect logs from all containers in the pod and concatenate them |

## Running Locally

```bash
go run *.go -kubeconfig {YOUR_KUBECONFIG_PATH}
```

## Documentation

For detailed documentation, visit: https://yutachaos.github.io/kube-job-notifier/
