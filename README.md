# kube-job-notifier
For kubernetes job notification tool 

## Description
- Notification Kubernetes Job start, success, and failure.

## Usage

### Notification setting(Slack only)
- Please set environment variable
```bash
export SLACK_TOKEN=YOUR_SLACK_TOKEN
export SLACK_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID
export SLACK_STARTED_NOTIFY=true # OPTIONAL DEFAULT true
export SLACK_SUCCEEDED_NOTIFY=true # OPTIONAL DEFAULT true
export SLACK_FAILED_NOTIFY=true # OPTIONAL DEFAULT true
export SLACK_USERNAME=YOUR_NOTIFICATION_USERNAME # OPTIONAL
export SLACK_SUCCEED_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID # OPTIONAL
export SLACK_FAILED_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID # OPTIONAL
export DATADOG_ENABLED=true # OPTIONAL DEFAULT false

export NAMESPACE=KUBERNETES_NAMESPACE # OPTIONAL
```

### Event subscription setting(Current Datadog support only)
- Datadog service checks are sent when the Job succeeds or fails.
- More information https://docs.datadoghq.com/developers/service_checks/dogstatsd_service_checks_submission/

### Run

#### Local

`go run *.go -kubeconfig {YOUR_KUBECONFIG_PATH}`
 
#### Kubernetes
- Run your kubernetes cluster.(Note default namespace is `default`). If change apply namespace, please edit manifest.
- Setting SLACK_TOKEN and SLACK_CHANNEL in manifest/sample/deployment.yaml.
- Apply manifest
`kubectl apply -f manifests/sample/`
- Check running
```
kubectl get po
NAME                                            READY   STATUS    RESTARTS   AGE
kube-job-notifier-deployment-698fbc8b54-ffk2q   1/1     Running   0          8m12s
```
