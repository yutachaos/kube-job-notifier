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
export SLACK_USERNAME=YOUR_NOTIFICATION_USERNAME # OPTIONAL
ecport SLACK_SUCCEED_CHANNE=YOUR_NOTIFICATION_CHANNEL_ID # OPTIONAL
ecport SLACK_FAILED_CHANNEL=YOUR_NOTIFICATION_CHANNEL_ID # OPTIONAL
ecport NAMESPACE=KUBERNETES_NAMESPACE # OPTIONAL
```

### Run
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
