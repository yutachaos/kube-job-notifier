module github.com/yutachaos/kube-job-notifier

go 1.14

require (
	github.com/DataDog/datadog-go v3.7.0+incompatible
	github.com/Songmu/flextime v0.0.6
	github.com/slack-go/slack v0.6.3
	github.com/stretchr/testify v1.4.0
	github.com/thoas/go-funk v0.6.0
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
	k8s.io/api v0.0.0-20191003035645-10e821c09743
	k8s.io/apimachinery v0.0.0-20191003035458-c930edf45883
	k8s.io/client-go v0.0.0-20191003035859-a746c2f219b7
	k8s.io/klog v1.0.0
)
