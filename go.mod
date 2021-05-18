module github.com/yutachaos/kube-job-notifier

go 1.15

require (
	github.com/DataDog/datadog-go v3.7.0+incompatible
	github.com/Songmu/flextime v0.1.0
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/ghodss/yaml v0.0.0-20150909031657-73d445a93680 // indirect
	github.com/slack-go/slack v0.9.1
	github.com/stretchr/testify v1.7.0
	github.com/thoas/go-funk v0.8.0
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/klog v1.0.0
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)
