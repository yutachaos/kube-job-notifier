package main

import (
	"flag"
	"os"
	"os/user"
	"path/filepath"

	"github.com/yutachaos/kube-job-notifier/pkg/signals"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	masterURL  string
	kubeconfig string
	kubeClient kubernetes.Interface
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	stopCh := signals.SetupSignalHandler()

	if _, err := rest.InClusterConfig(); err != nil {
		cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
		if err != nil {
			klog.Fatalf("Error building kubeclient: %s", err.Error())
		}
		kubeClient, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			klog.Fatalf("Error building kubeclient: %s", err.Error())
		}
	} else {
		cfg, err := rest.InClusterConfig()
		kubeClient, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			klog.Fatalf("Error building in cluster kubeclient: %s", err.Error())
		}
	}

	// Specified namespace
	namespace := os.Getenv("NAMESPACE")
	var kubeInformerFactory kubeinformers.SharedInformerFactory
	// Sync event only
	if namespace == "" {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	} else {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))
	}

	controller := NewController(kubeClient, kubeInformerFactory.Batch().V1().Jobs())

	kubeInformerFactory.Start(stopCh)

	if err := controller.Run(stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	u, _ := user.Current()
	defaultPath := filepath.Join(u.HomeDir, ".kube", "config")
	// set kubeconfig flag
	flag.StringVar(&kubeconfig, "kubeconfig", defaultPath, "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
