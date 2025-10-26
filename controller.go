package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
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
	kubeclientset kubernetes.Interface
	jobsLister    batcheslisters.JobLister
	jobsSynced    cache.InformerSynced
	recorder      record.EventRecorder
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
	notifiedJobs := make(map[string]bool)

	notifications := notification.NewNotifications()
	subscriptions := monitoring.NewSubscription()
	datadogSubscription := subscriptions["datadog"]

	klog.Info("Setting event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new any) {
			newJob := new.(*batchv1.Job)
			klog.Infof("Job added: %v", newJob.Status)

			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
				return
			}

			if notifiedJobs[newJob.Name] {
				return
			}

			klog.Infof("Job started: %v", newJob.Status)

			jobPod, err := getPodFromControllerUID(kubeclientset, newJob)
			err = waitForPodRunning(kubeclientset, jobPod)

			if err != nil {
				klog.Errorf("Error waiting for pod to become running: %v", jobPod)
				return
			}

			cronJob, err := getCronJobNameFromOwnerReferences(kubeclientset, newJob)

			if err != nil {
				klog.Errorf("Get cronjob failed: %v", err)
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
				err := n.NotifyStart(messageParam)
				if err != nil {
					klog.Errorf("Failed %s notification: %v", name, err)
				}
			}

		},
		UpdateFunc: func(old, new any) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)

			klog.Infof("oldJob.Status:%v", oldJob.Status)
			klog.Infof("newJob.Status:%v", newJob.Status)
			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
				return
			}

			if notifiedJobs[newJob.Name] {
				return
			}

			jobPod, err := getPodFromControllerUID(kubeclientset, newJob)
			err = waitForPodRunning(kubeclientset, jobPod)

			if err != nil {
				klog.Errorf("Error waiting for pod to become running: %v", err)
				return
			}

			if newJob.Status.Succeeded == intTrue {
				klog.Infof("Job succeeded: Name: %s: Status: %v", newJob.Name, newJob.Status)
				jobPod, err := getPodFromControllerUID(kubeclientset, newJob)
				if err != nil {
					klog.Errorf("Get pods failed: %v", err)
					return
				}

				cronJobName, err := getCronJobNameFromOwnerReferences(kubeclientset, newJob)

				if err != nil {
					klog.Errorf("Get cronjob failed: %v", err)
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

				for name, n := range notifications {
					err = n.NotifySuccess(messageParam)
					if err != nil {
						klog.Errorf("Failed %s n: %v", name, err)
					}
				}

				if os.Getenv("DATADOG_ENABLE") == "true" {

					err = datadogSubscription.SuccessEvent(
						monitoring.JobInfo{
							CronJobName: cronJobName,
							Name:        newJob.Name,
							Namespace:   newJob.Namespace,
							Annotations: newJob.Spec.Template.Annotations,
						})
					if err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
				klog.V(4).Infof("Job succeeded log: %v", jobLogStr)
				notifiedJobs[newJob.Name] = isCompletedJob(kubeclientset, newJob)
			} else if newJob.Status.Failed == intTrue {
				klog.Infof("Job failed: Name: %s: Status: %v", newJob.Name, newJob.Status)
				jobPod, err := getPodFromControllerUID(kubeclientset, newJob)
				if err != nil {
					klog.Errorf("Get pods failed: %v", err)
					return
				}

				cronJobName, err := getCronJobNameFromOwnerReferences(kubeclientset, newJob)
				if err != nil {
					klog.Errorf("Get cronjob failed: %v", err)
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
				for name, n := range notifications {
					err := n.NotifyFailed(messageParam)
					if err != nil {
						klog.Errorf("Failed %s notification: %v", name, err)
					}
				}
				if os.Getenv("DATADOG_ENABLE") == "true" {
					err = datadogSubscription.FailEvent(
						monitoring.JobInfo{
							CronJobName: cronJobName,
							Name:        newJob.Name,
							Namespace:   newJob.Namespace,
							Annotations: newJob.Spec.Template.Annotations,
						})
					if err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
				notifiedJobs[newJob.Name] = isCompletedJob(kubeclientset, newJob)
			}
		},
		DeleteFunc: func(obj any) {
			deletedJob := obj.(*batchv1.Job)
			delete(notifiedJobs, deletedJob.Name)
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

	if ownerReferences, ok := funk.Filter(job.OwnerReferences,
		func(ownerReference metav1.OwnerReference) bool {
			return ownerReference.Kind == "CronJob"
		}).([]metav1.OwnerReference); ok &&
		len(ownerReferences) > 0 {
		cronJobBeta, err := kubeclientset.BatchV1beta1().CronJobs(job.Namespace).Get(context.TODO(),
			ownerReferences[0].Name,
			metav1.GetOptions{
				TypeMeta: metav1.TypeMeta{
					Kind:       ownerReferences[0].Kind,
					APIVersion: ownerReferences[0].APIVersion,
				},
			})

		if err == nil {
			return cronJobBeta.Name, err
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
			return cronJobV1.Name, err
		}
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
