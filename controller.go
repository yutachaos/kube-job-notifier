package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/thoas/go-funk"
	"github.com/yutachaos/kube-job-notifier/pkg/monitoring"
	"github.com/yutachaos/kube-job-notifier/pkg/notification"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	batchesinformers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	batcheslisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const (
	controllerAgentName = "cronjob-controller"
	intTrue             = 1
	searchLabel         = "controller-uid"

	logModeAnnotationName = "kube-job-notifier/log-mode"
)

type logMode int

const (
	ownerContainer logMode = iota
	podOnly
	podContainers
)

var serverStartTime time.Time

// Controller is Kubernetes Controller struct
type Controller struct {
	jobsLister batcheslisters.JobLister
	jobsSynced cache.InformerSynced
	recorder   record.EventRecorder
}

// NewController returns a new controller
func NewController(
	kubeclientset kubernetes.Interface,
	jobInformer batchesinformers.JobInformer) *Controller {

	utilruntime.Must(scheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		jobsLister: jobInformer.Lister(),
		jobsSynced: jobInformer.Informer().HasSynced,
		recorder:   recorder,
	}
	serverStartTime = time.Now().Local()
	var notifiedJobs sync.Map

	notifications, err := notification.NewNotifications()
	if err != nil {
		klog.Fatalf("Error creating notifications: %s", err)
	}
	subscriptions := monitoring.NewSubscription()
	datadogSubscription := subscriptions["datadog"]

	regexEnv := os.Getenv("CRONJOB_REGEX")
	var regex *regexp.Regexp
	if regexEnv != "" {
		regex = regexp.MustCompile(regexEnv)
	}

	klog.Info("Setting event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new any) {
			newJob := new.(*batchv1.Job)
			klog.Infof("Job added: %v", newJob.Status)

			cronJob, err := getCronJobNameFromOwnerReferences(kubeclientset, newJob)
			if err != nil {
				klog.Errorf("Get cronjob failed: %v", err)
			}

			if regex != nil && !regex.MatchString(cronJob) {
				return
			}

			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
				return
			}

			if v, ok := notifiedJobs.Load(newJob.Name); ok && v.(bool) {
				return
			}

			klog.Infof("Job started: %v", newJob.Status)

			jobPod, err := getPodFromControllerUID(kubeclientset, newJob)
			if err != nil {
				klog.Errorf("Get pods failed: %v", err)
				return
			}

			err = waitForPodRunning(kubeclientset, jobPod)
			if err != nil {
				klog.Errorf("Error waiting for pod to become running: %v", jobPod)
				return
			}

			klog.Infof("Job started: %v", newJob.Status)
			messageParam := notification.MessageTemplateParam{
				JobName:     newJob.Name,
				CronJobName: cronJob,
				Namespace:   newJob.Namespace,
				StartTime:   newJob.Status.StartTime,
				Annotations: newJob.Spec.Template.Annotations,
			}
			for name, n := range notifications {
				if err := n.NotifyStart(messageParam); err != nil {
					klog.Errorf("Failed %s notification: %v", name, err)
				}
			}

			klog.V(4).Infof("Job %s: Start notification sent, waiting for completion", newJob.Name)
		},
		UpdateFunc: func(old, new any) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)

			klog.Infof("oldJob.Status:%v", oldJob.Status)
			klog.Infof("newJob.Status:%v", newJob.Status)

			cronJobName, err := getCronJobNameFromOwnerReferences(kubeclientset, newJob)
			if err != nil {
				klog.Errorf("Get cronjob failed: %v", err)
			}

			if regex != nil && !regex.MatchString(cronJobName) {
				return
			}

			klog.Infof("Job %s: CreationTime=%v, ServerStartTime=%v, TimeDiff=%.2f seconds",
				newJob.Name, newJob.CreationTimestamp, serverStartTime,
				newJob.CreationTimestamp.Sub(serverStartTime).Seconds())

			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
				klog.Infof("Job %s: Skipping notification - Job created before server start", newJob.Name)
				return
			}

			oldSucceeded := oldJob.Status.Succeeded == intTrue
			newSucceeded := newJob.Status.Succeeded == intTrue
			oldFailed := oldJob.Status.Failed == intTrue
			newFailed := newJob.Status.Failed == intTrue

			if oldSucceeded == newSucceeded && oldFailed == newFailed {
				klog.V(4).Infof("Job %s: Status unchanged, skipping notification", newJob.Name)
				return
			}

			if v, ok := notifiedJobs.Load(newJob.Name); ok && v.(bool) {
				klog.Infof("Job %s: Skipping notification - Already notified", newJob.Name)
				return
			}

			// Only proceed for newly succeeded or newly failed
			if !((newSucceeded && !oldSucceeded) || (newFailed && !oldFailed)) {
				return
			}

			jobPod, err := getPodFromControllerUID(kubeclientset, newJob)
			if err != nil {
				klog.Errorf("Get pods failed: %v", err)
				return
			}

			if err = waitForPodRunning(kubeclientset, jobPod); err != nil {
				klog.Errorf("Error waiting for pod to become running: %v", err)
				return
			}

			annotations := newJob.Spec.Template.Annotations
			lm := getLogMode(annotations, logModeAnnotationName)
			jobLogStr := getJobLogs(kubeclientset, jobPod, cronJobName, lm)

			messageParam := notification.MessageTemplateParam{
				JobName:        newJob.Name,
				CronJobName:    cronJobName,
				Namespace:      newJob.Namespace,
				StartTime:      newJob.Status.StartTime,
				CompletionTime: newJob.Status.CompletionTime,
				Log:            jobLogStr,
				Annotations:    annotations,
			}

			jobInfo := monitoring.JobInfo{
				CronJobName: cronJobName,
				Name:        newJob.Name,
				Namespace:   newJob.Namespace,
				Annotations: newJob.Spec.Template.Annotations,
			}

			if newSucceeded && !oldSucceeded {
				klog.Infof("Job succeeded: Name: %s: Status: %v", newJob.Name, newJob.Status)
				for name, n := range notifications {
					if err := n.NotifySuccess(messageParam); err != nil {
						klog.Errorf("Failed %s notification for job %s: %v", name, newJob.Name, err)
					}
				}
				if os.Getenv("DATADOG_ENABLE") == "true" {
					if err := datadogSubscription.SuccessEvent(jobInfo); err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
			} else {
				klog.Infof("Job failed: Name: %s: Status: %v", newJob.Name, newJob.Status)
				for name, n := range notifications {
					if err := n.NotifyFailed(messageParam); err != nil {
						klog.Errorf("Failed %s failure notification for job %s: %v", name, newJob.Name, err)
					}
				}
				if os.Getenv("DATADOG_ENABLE") == "true" {
					if err := datadogSubscription.FailEvent(jobInfo); err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
			}

			isCompleted := isCompletedJob(kubeclientset, newJob)
			klog.Infof("Job %s: Setting notified flag to %t", newJob.Name, isCompleted)
			notifiedJobs.Store(newJob.Name, isCompleted)
		},
		DeleteFunc: func(obj any) {
			deletedJob := obj.(*batchv1.Job)
			notifiedJobs.Delete(deletedJob.Name)
		},
	})

	return controller
}

// Run is Kubernetes Controller execute method
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	klog.Info("Starting kubernetes job notify controller")

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.jobsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func isCompletedJob(kubeclientset kubernetes.Interface, job *batchv1.Job) bool {

	if job.Status.Succeeded == intTrue {
		return true
	}

	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{searchLabel: string(job.UID)}}

	jobPodList, err := kubeclientset.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         int64(*job.Spec.BackoffLimit + 1),
	})

	if err != nil {
		return true
	}

	if jobPodList.Items != nil && len(jobPodList.Items) != int(*job.Spec.BackoffLimit)+1 {
		return false
	}
	return true
}

func getPodFromControllerUID(kubeclientset kubernetes.Interface, job *batchv1.Job) (corev1.Pod, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{searchLabel: string(job.UID)}}
	jobPodList, err := kubeclientset.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         int64(*job.Spec.BackoffLimit),
	})
	if err != nil {
		return corev1.Pod{}, err
	}
	if jobPodList.Size() == 0 {
		return corev1.Pod{}, fmt.Errorf("failed get pod list JobPodListSize: %v", jobPodList.Size())
	}
	if len(jobPodList.Items) == 0 {
		return corev1.Pod{}, fmt.Errorf("failed get pod list jobPodList.Items): %v", jobPodList.Items)
	}
	jobPod := jobPodList.Items[len(jobPodList.Items)-1]
	return jobPod, nil
}

func getCronJobNameFromOwnerReferences(kubeclientset kubernetes.Interface, job *batchv1.Job) (cronJobName string, err error) {
	ownerReferences, ok := funk.Filter(job.OwnerReferences,
		func(ownerReference metav1.OwnerReference) bool {
			return ownerReference.Kind == "CronJob"
		}).([]metav1.OwnerReference)
	if !ok || len(ownerReferences) == 0 {
		return cronJobName, err
	}

	cronJobV1, err := kubeclientset.BatchV1().CronJobs(job.Namespace).Get(context.TODO(),
		ownerReferences[0].Name,
		metav1.GetOptions{
			TypeMeta: metav1.TypeMeta{
				Kind:       ownerReferences[0].Kind,
				APIVersion: ownerReferences[0].APIVersion,
			},
		})
	if err == nil {
		return cronJobV1.Name, nil
	}
	return cronJobName, err
}

func getLogMode(annotations map[string]string, annotationName string) logMode {
	a, ok := annotations[annotationName]
	if !ok {
		return ownerContainer
	}
	switch a {
	case "OwnerContainer":
		return ownerContainer
	case "PodOnly":
		return podOnly
	case "PodContainers":
		return podContainers
	default:
		return ownerContainer
	}
}

func getJobLogs(clientset kubernetes.Interface, pod corev1.Pod, cronJobName string, mode logMode) string {
	switch mode {
	case podContainers:
		if len(pod.Spec.Containers) == 1 {
			return getPodLogs(clientset, pod, pod.Spec.Containers[0].Name)
		}

		b := strings.Builder{}
		for _, c := range pod.Spec.Containers {
			b.WriteString(fmt.Sprintf("Container %s logs:\r\n%s\r\n", c.Name, getPodLogs(clientset, pod, c.Name)))
		}
		return b.String()
	case podOnly:
		return getPodLogs(clientset, pod, "")
	default:
		return getPodLogs(clientset, pod, cronJobName)
	}
}

func getPodLogs(clientset kubernetes.Interface, pod corev1.Pod, containerName string) string {
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: containerName})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return err.Error()
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return err.Error()
	}
	str := buf.String()
	err = podLogs.Close()

	if err != nil {
		return err.Error()
	}
	return str
}

func waitForPodRunning(clientset kubernetes.Interface, pod corev1.Pod) error {
	pollInterval := 10 * time.Second

	timeout := 20 * time.Minute
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for pod to become running")
		}

		pod, err := clientset.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error getting pod: %v", err)
		}

		if pod.Status.Phase != corev1.PodPending {
			return nil
		}

		time.Sleep(pollInterval)
	}
}
