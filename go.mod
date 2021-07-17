module github.com/yutachaos/kube-job-notifier

go 1.15

require (
	github.com/DataDog/datadog-go v3.7.0+incompatible
	github.com/Songmu/flextime v0.1.0
	github.com/slack-go/slack v0.9.3
	github.com/stretchr/testify v1.7.0
	github.com/thoas/go-funk v0.8.0
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	k8s.io/api v0.0.0-20191003035645-10e821c09743
	k8s.io/apimachinery v0.0.0-20191003035458-c930edf45883
	k8s.io/client-go v0.0.0-20191003035859-a746c2f219b7
	k8s.io/klog v1.0.0
)
