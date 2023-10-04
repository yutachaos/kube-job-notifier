# kube-job-notifier
For kubernetes job notification tool 

## Description
- Notification Kubernetes Job start, success, and failure.

## Usage

### Notification setting(Slack only)
- Please set environment variable

```
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

It will take SLACK_CHANNEL as default channel which may be overwritten by SLACK_SUCCEED_CHANNEL, SLACK_FAILED_CHANNEL environment variables.

Another way of overriding behaviour is using job annotations in k8s. Available job annotations to override are: 

```
- kube-job-notifier/default-channel - will be used as channel for a notification if similar success channel annotation is empty 
- kube-job-notifier/success-channel - will be used as channel for a success job notification 
- kube-job-notifier/started-channel - will be used as channel for a started job notification 
- kube-job-notifier/failed-channel - will be used as channel for a failed job notification 
```

Also it's possible to suppress notification per job: 

```
- kube-job-notifier/suppress-success-notification - suppress notification for succesfully finished job even if SLACK_SUCCEEDED_NOTIFY environment variable set to true
- kube-job-notifier/suppress-started-notification - suppress notification when job is started even if SLACK_STARTED_NOTIFY environment variable set to true 
- kube-job-notifier/suppress-failed-notification - suppress notification when job is failed even if SLACK_FAILED_NOTIFY environment variable set to true 
```
#### slack permissions
- Required permission above.
```
chat:write
files:write
```

### Event subscription setting(Current Datadog support only)
- Datadog service checks are sent when the Job succeeds or fails.
- More information https://docs.datadoghq.com/developers/service_checks/dogstatsd_service_checks_submission/

### Job with multiple containers logging

By default for cron jobs logs are attached from container with the same name as a cron job. This can be overwritten by adding *kube-job-notifier/log-mode* annotation. 

- *ownerContainer* - get logs only from container with the same name as cron job (default behaviour if annotation is not presented);
- *podOnly* - get logs from the pod, works perfectly with pod with single container;
- *podContainers* - get logs from all pod containers and concatenate them. 

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

#### Install with Helm
`helm repo add kube-job-notifier https://yutachaos.github.io/kube-job-notifier/`
`helm install kube-job-notifier/kube-job-notifier --generate-name`

#### Document

https://yutachaos.github.io/kube-job-notifier/

