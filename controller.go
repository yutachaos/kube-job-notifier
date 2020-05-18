package main

import (
	"bytes"
	"fmt"
	"github.com/thoas/go-funk"
	"github.com/yutachaos/kube-job-notifier/pkg/monitoring"
	"github.com/yutachaos/kube-job-notifier/pkg/notification"
	"golang.org/x/xerrors"
	"io"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	batchesinformers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	batcheslisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"os"
	"time"
)

const (
	controllerAgentName = "cronjob-controller"
	intTrue             = 1
	searchLabel         = "job-name"
)

var serverStartTime time.Time

// Controller is Kubernetes Controller struct
type Controller struct {
	kubeclientset kubernetes.Interface
	jobsLister    batcheslisters.JobLister
	jobsSynced    cache.InformerSynced
	workqueue     workqueue.RateLimitingInterface
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
		workqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Kjobs"),
		recorder:   recorder,
	}
	serverStartTime = time.Now().Local()

	notifications := notification.NewNotifications()
	subscriptions := monitoring.NewSubscription()
	datadogSubscription := subscriptions["datadog"]

	klog.Info("Setting event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			newJob := new.(*batchv1.Job)
			klog.Infof("Job Added: %v", newJob.Status)
			if newJob.CreationTimestamp.Sub(serverStartTime).Seconds() > 0 {
				klog.Infof("Job started: %v", newJob.Status)

				cronJob, err := getCronJobFromOwnerReferences(kubeclientset, newJob)

				if err != nil {
					klog.Errorf("Get cronjob failed: %v", err)
				}
				messageParam := notification.MessageTemplateParam{
					CronJobName: cronJob.Name,
					JobName:     newJob.Name,
					Namespace:   newJob.Namespace,
				}
				for name, n := range notifications {
					err := n.NotifyStart(messageParam)
					if err != nil {
						klog.Errorf("Failed %s notification: %v", name, err)
					}
				}

			}
			controller.handleObject(new)
		},
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)

			klog.Infof("oldJob.Status:%v", oldJob.Status)
			klog.Infof("newJob.Status:%v", newJob.Status)
			if newJob.Status.Succeeded == intTrue {
				klog.Infof("Job succeeded: %v", newJob.Status)
				jobPod, err := getPodFromJobName(kubeclientset, newJob)
				if err != nil {
					klog.Errorf("Get pods failed: %v", err)
				}
				cronJob, err := getCronJobFromOwnerReferences(kubeclientset, newJob)

				if err != nil {
					klog.Errorf("Get cronjob failed: %v", err)
				}

				jobLogStr, err := getPodLogs(kubeclientset, jobPod)
				if err != nil {
					klog.Errorf("Get pods log failed: %v", err)
				}

				messageParam := notification.MessageTemplateParam{
					CronJobName: cronJob.Name,
					JobName:     newJob.Name,
					Log:         jobLogStr,
					Namespace:   newJob.Namespace,
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
							CronJobName: cronJob.Name,
							Name:        newJob.Name,
							Namespace:   newJob.Namespace,
						})
					if err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
				klog.V(4).Infof("Job succeeded log: %v", jobLogStr)

			} else if newJob.Status.Failed == intTrue {
				klog.Infof("Job failed: %v", newJob.Status)
				jobPod, err := getPodFromJobName(kubeclientset, newJob)
				if err != nil {
					klog.Errorf("Get pods failed: %v", err)
				}
				cronJob, err := getCronJobFromOwnerReferences(kubeclientset, newJob)
				klog.Infof("Debug cronjob name: %s", cronJob.Name)
				if err != nil {
					klog.Errorf("Get cronjob failed: %v", err)
				}

				jobLogStr, err := getPodLogs(kubeclientset, jobPod)
				if err != nil {
					klog.Errorf("Get pods log failed: %v", err)
				}

				messageParam := notification.MessageTemplateParam{
					CronJobName: cronJob.Name,
					JobName:     newJob.Name,
					Log:         jobLogStr,
					Namespace:   newJob.Namespace,
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
							CronJobName: cronJob.Name,
							Name:        newJob.Name,
							Namespace:   newJob.Namespace,
						})
					if err != nil {
						klog.Errorf("Fail event subscribe.: %v", err)
					}
				}
				klog.V(4).Infof("Job failed log: %v", jobLogStr)
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run is Kubernetes Controller execute method
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting kubernetes job notify controller")

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.jobsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {

		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(obj)
		klog.V(4).Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	klog.V(4).Infof("name: %s", name)
	klog.V(4).Infof("namespace: %s", namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	_, err = c.jobsLister.Jobs(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("job '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	return nil
}

func (c *Controller) enqueueJob(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) handleObject(obj interface{}) {
	var job *batchv1.Job
	var ok bool

	if job, ok = obj.(*batchv1.Job); ok {
		klog.V(4).Infof("Processing job: %v", job.GetName())
		if job.Status.StartTime != nil {
			c.enqueueJob(job)
			return
		}
	}
	return
}

func getPodFromJobName(kubeclientset kubernetes.Interface, job *batchv1.Job) (corev1.Pod, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{searchLabel: job.Name}}
	jobPodList, err := kubeclientset.CoreV1().Pods(job.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         1,
	})
	if err != nil {
		return corev1.Pod{}, err
	}
	if jobPodList.Size() == 0 {
		return corev1.Pod{}, err
	}
	jobPod := jobPodList.Items[0]
	return jobPod, nil
}

func getCronJobFromOwnerReferences(kubeclientset kubernetes.Interface, job *batchv1.Job) (v1beta1.CronJob, error) {

	if ownerReferences, ok := funk.Filter(job.OwnerReferences,
		func(ownerReference metav1.OwnerReference) bool {
			return ownerReference.Kind == "CronJob"
		}).([]metav1.OwnerReference); ok &&
		len(ownerReferences) > 0 {
		cronJob, err := kubeclientset.BatchV1beta1().CronJobs(job.Namespace).Get(
			ownerReferences[0].Name,
			metav1.GetOptions{
				TypeMeta: metav1.TypeMeta{
					Kind:       ownerReferences[0].Kind,
					APIVersion: ownerReferences[0].APIVersion,
				},
			})

		if err != nil {
			return v1beta1.CronJob{}, err
		}
		return *cronJob, err

	}
	return v1beta1.CronJob{}, nil
}

func getPodLogs(clientset kubernetes.Interface, pod corev1.Pod) (string, error) {
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	podLogs, err := req.Stream()
	if err != nil {
		return "", xerrors.Errorf("error in open log stream: %v", err)
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", xerrors.Errorf("error in copy information from log to buffer: %v", err)
	}
	str := buf.String()
	err = podLogs.Close()

	if err != nil {
		return "", xerrors.Errorf("error in close log stream: %v", err)
	}
	return str, nil
}
