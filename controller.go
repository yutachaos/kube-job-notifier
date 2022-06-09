package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const (
	controllerAgentName = "cronjob-controller"
	intTrue             = 1
	searchLabel         = "controller-uid"
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
		AddFunc: func(new interface{}) {
			newJob := new.(*batchv1.Job)
			klog.Infof("Job Added: %v", newJob.Status)

			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
				return
			}

			if notifiedJobs[newJob.Name] == true {
				return
			}

			klog.Infof("Job started: %v", newJob.Status)

			cronJob, err := getCronJobNameFromOwnerReferences(kubeclientset, newJob)

			if err != nil {
				klog.Errorf("Get cronjob failed: %v", err)
			}
			messageParam := notification.MessageTemplateParam{
				JobName:     newJob.Name,
				CronJobName: cronJob,
				Namespace:   newJob.Namespace,
				StartTime:   newJob.Status.StartTime,
			}
			for name, n := range notifications {
				err := n.NotifyStart(messageParam)
				if err != nil {
					klog.Errorf("Failed %s notification: %v", name, err)
				}
			}

		},
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)

			klog.Infof("oldJob.Status:%v", oldJob.Status)
			klog.Infof("newJob.Status:%v", newJob.Status)
			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
				return
			}

			if notifiedJobs[newJob.Name] == true {
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
				jobLogStr := getPodLogs(kubeclientset, jobPod, cronJobName)

				messageParam := notification.MessageTemplateParam{
					JobName:        newJob.Name,
					CronJobName:    cronJobName,
					Namespace:      newJob.Namespace,
					StartTime:      newJob.Status.StartTime,
					CompletionTime: newJob.Status.CompletionTime,
					Log:            jobLogStr,
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
						})
					if err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
				klog.V(4).Infof("Job succeeded log: %v", jobLogStr)

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

				jobLogStr := getPodLogs(kubeclientset, jobPod, cronJobName)

				messageParam := notification.MessageTemplateParam{
					JobName:        newJob.Name,
					CronJobName:    cronJobName,
					Namespace:      newJob.Namespace,
					StartTime:      newJob.Status.StartTime,
					CompletionTime: newJob.Status.CompletionTime,
					Log:            jobLogStr,
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
						})
					if err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
			}
			notifiedJobs[newJob.Name] = isCompletedJob(kubeclientset, newJob)
		}, DeleteFunc: func(obj interface{}) {
			deletedJob := obj.(*batchv1.Job)
			delete(notifiedJobs, deletedJob.Name)
		},
	})

	return controller
}

// Run is Kubernetes Controller execute method
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
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
		Limit:         int64(*job.Spec.BackoffLimit),
	})

	if err != nil {
		return true
	}
	if jobPodList.Size() < int(*job.Spec.BackoffLimit) {
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

func getPodLogs(clientset kubernetes.Interface, pod corev1.Pod, cronJobName string) string {
	var req *rest.Request
	// OwnerReferenceがCronJobではない場合cronJobNameが空になる
	if cronJobName == "" {
		req = clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	} else {
		req = clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: cronJobName})
	}

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
