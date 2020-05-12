package monitoring

import (
	"github.com/DataDog/datadog-go/statsd"
	"k8s.io/klog"
	"os"
)

const (
	hostName         = "kube-job-notifier"
	serviceCheckName = "kube_job_notifier.job.status"
)

type datadog struct {
	client *statsd.Client
}

func newDatadog() datadog {
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

	klog.Infof("Debug service check: %v", sc)
	if err != nil {
		klog.Errorf("Failed subscribe custom event. error: %v", err)
		return err
	}
	klog.Infof("Event subscribe successfully %s", jobInfo.Name)
	return nil
}

func (d datadog) FailEvent(jobInfo JobInfo) (err error) {
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
	klog.Infof("Debug service check: %v", sc)
	if err != nil {
		klog.Errorf("Failed subscribe custom event. error: %v", err)
		return err
	}
	klog.Infof("Event subscribe successfully %s", jobInfo.getJobName())
	return nil
}
