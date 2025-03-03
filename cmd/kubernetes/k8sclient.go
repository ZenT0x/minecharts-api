package kubernetes

import (
	"flag"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Clientset is a global Kubernetes clientset instance.
var Clientset *kubernetes.Clientset

// Init initializes the global Kubernetes clientset.
// It uses the local kubeconfig if available; otherwise, it falls back to in-cluster config.
func Init() error {
	var config *rest.Config
	var err error
	var kubeconfig string

	// Set default kubeconfig path from HOME if available.
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "absolute path to the kubeconfig file")
	flag.Parse()

	// Use kubeconfig if available; otherwise, use in-cluster config.
	if _, err := os.Stat(kubeconfig); err == nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return err
		}
	}

	Clientset, err = kubernetes.NewForConfig(config)
	return err
}
