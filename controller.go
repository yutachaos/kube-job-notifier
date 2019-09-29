package main

import (
	"bytes"
	"fmt"
	"io"
	batchv1 "k8s.io/api/batch/v1"
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
	"time"
)

const controllerAgentName = "cronjob-controller"

const (
	INT_TRUE     = 1
	SEARCH_LABEL = "job-name"
)

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

	klog.Info("Setting event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			newJob := new.(*batchv1.Job)
			klog.Infof("Job started: %v", newJob.Status)
			controller.handleObject(new)
		},
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)

			klog.Infof("oldJob.Status:%v", oldJob.Status)
			klog.Infof("newJob.Status:%v", newJob.Status)
			if newJob.Status.Succeeded == INT_TRUE {
				klog.Infof("Job succeeded: %v", newJob.Status)
				jobPod, err := getPodFromJobName(newJob, kubeclientset)
				if err != nil {
					klog.Errorf("Get pods failed: %v", err)
				}
				jobLogStr := getPodLogs(kubeclientset, jobPod)

				klog.Infof("Job succeeded log: %v", jobLogStr)

			} else if newJob.Status.Failed == INT_TRUE {
				klog.Infof("Job failed: %v", newJob.Status)
				jobPod, err := getPodFromJobName(newJob, kubeclientset)
				if err != nil {
					klog.Errorf("Get pods failed: %v", err)
				}
				jobLogStr := getPodLogs(kubeclientset, jobPod)

				klog.Infof("Job failed log: %v", jobLogStr)
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

func getPodFromJobName(job *batchv1.Job, kubeclientset kubernetes.Interface) (corev1.Pod, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{SEARCH_LABEL: job.Name}}
	jobPodList, err := kubeclientset.CoreV1().Pods(job.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         1,
	})
	if err != nil {
		return corev1.Pod{}, err
	}
	jobPod := jobPodList.Items[0]
	return jobPod, err
}

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
		klog.Infof("Successfully synced '%s'", key)
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
	klog.Infof("name: %s", name)
	klog.Infof("namespace: %s", namespace)
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
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}

	klog.Infof("Processing object: %v", object.GetName())

	if job, ok := obj.(*batchv1.Job); !ok && job.Status.Succeeded == INT_TRUE {
		return
	}
	c.enqueueJob(object)
	return
}

func getPodLogs(clientset kubernetes.Interface, pod corev1.Pod) string {
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	podLogs, err := req.Stream()
	defer podLogs.Close()
	if err != nil {
		return "error in open log stream"
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "error in copy information from log to buffer"
	}
	str := buf.String()

	return str
}
