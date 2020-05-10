package monitoring

import (
	"github.com/DataDog/datadog-go/statsd"
	"k8s.io/klog"
	"os"
)

const serviceCheckName = "kube_job_notifier.cronjob.status"

type datadog struct {
	client *statsd.Client
}

type JobInfo struct {
	Name      string
	Namespace string
}

type Datadog interface {
	SuccessEvent(jobInfo JobInfo) (err error)
	FailEvent(jobInfo JobInfo) (err error)
}

func NewDatadog() Datadog {
	client, err := statsd.New("127.0.0.1:8125")
	if err != nil {
		klog.Errorf("Failed create statsd client. error: %v", err)
	}

	tags := []string{os.Getenv("DD_TAGS")}

	if len(tags) != 0 {
		client.Tags = tags
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
	err = d.client.ServiceCheck(
		&statsd.ServiceCheck{
			Name:    serviceCheckName,
			Status:  statsd.Ok,
			Message: "Cronjob succeed",
			Tags: []string{
				"job_name:" + jobInfo.Name,
				"namespace:" + jobInfo.Namespace,
			},
		})
	if err != nil {
		klog.Errorf("Failed subscribe custom event. error: %v", err)
		return err
	}
	klog.Infof("Event subscribe successfully %s", jobInfo.Name)
	return nil
}

func (d datadog) FailEvent(jobInfo JobInfo) (err error) {
	err = d.client.ServiceCheck(
		&statsd.ServiceCheck{
			Name:    serviceCheckName,
			Status:  statsd.Critical,
			Message: "Cronjob failed",
			Tags: []string{
				"job_name:" + jobInfo.Name,
				"namespace:" + jobInfo.Namespace,
			},
		})
	if err != nil {
		klog.Errorf("Failed subscribe custom event. error: %v", err)
		return err
	}
	klog.Infof("Event subscribe successfully %s", jobInfo.Name)
	return nil
}
