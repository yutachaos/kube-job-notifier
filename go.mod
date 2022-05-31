module github.com/yutachaos/kube-job-notifier

go 1.16

require (
	github.com/DataDog/datadog-go v4.8.3+incompatible
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/Songmu/flextime v0.1.0
	github.com/slack-go/slack v0.10.2
	github.com/stretchr/testify v1.7.1
	github.com/thoas/go-funk v0.9.2
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	k8s.io/klog v1.0.0
)
