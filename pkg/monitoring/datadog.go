package monitoring

import (
	"github.com/DataDog/datadog-go/statsd"
	"k8s.io/klog"
	"os"
)

const (
	defaultStatsAddrUDS           = "unix:///var/run/datadog/dsd.socket"
	hostName                      = "kube-job-notifier"
	serviceCheckName              = "kube_job_notifier.job.status"
	suppressSuccessAnnotationName = "kube-job-notifier/suppress-success-datadog-subscription"
	suppressFailedAnnotationName  = "kube-job-notifier/suppress-failed-datadog-subscription"
)

type datadog struct {
	client *statsd.Client
}

func newDatadog() datadog {
	client, err := statsd.New(defaultStatsAddrUDS)
	if err != nil {
		klog.Errorf("Failed create statsd client. error: %v", err)
	}

	tags := os.Getenv("DD_TAGS")

	if tags != "" {
		client.Tags = []string{tags}
	}

	namespace := os.Getenv("DD_NAMESPACE")

	if namespace != "" {
		client.Namespace = namespace
	}

	return datadog{
		client: client,
	}
}

func (d datadog) SuccessEvent(jobInfo JobInfo) (err error) {
	if isSubscriptionSuppressed(jobInfo.Annotations, suppressSuccessAnnotationName) {
		klog.Infof("Notification for %s is suppressed", jobInfo.Name)
		return nil
	}
	sc := &statsd.ServiceCheck{
		Name:     serviceCheckName,
		Status:   statsd.Ok,
		Message:  "Job succeed",
		Hostname: hostName,
		Tags: []string{
			"job_name:" + jobInfo.getJobName(),
			"namespace:" + jobInfo.Namespace,
		},
	}
	err = d.client.ServiceCheck(sc)
	if err != nil {
		klog.Errorf("Failed subscribe custom event. error: %v", err)
		return err
	}
	klog.Infof("Event subscribe successfully %s", jobInfo.Name)
	return nil
}

func (d datadog) FailEvent(jobInfo JobInfo) (err error) {
	if isSubscriptionSuppressed(jobInfo.Annotations, suppressFailedAnnotationName) {
		klog.Infof("Notification for %s is suppressed", jobInfo.Name)
		return nil
	}
	sc := &statsd.ServiceCheck{
		Name:     serviceCheckName,
		Status:   statsd.Critical,
		Message:  "Job failed",
		Hostname: hostName,
		Tags: []string{
			"job_name:" + jobInfo.getJobName(),
			"namespace:" + jobInfo.Namespace,
		},
	}
	err = d.client.ServiceCheck(sc)
	if err != nil {
		klog.Errorf("Failed subscribe custom event. error: %v", err)
		return err
	}
	klog.Infof("Event subscribe successfully %s", jobInfo.getJobName())
	return nil
}

func isSubscriptionSuppressed(annotations map[string]string, annotationName string) bool {
	a, ok := annotations[annotationName]
	if !ok {
		return false
	}
	return a == "true"
}
