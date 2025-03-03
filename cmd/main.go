package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var configPath string

	// Try to load kubeconfig from flag or default location
	if home := os.Getenv("HOME"); home != "" {
		configPath = filepath.Join(home, ".kube", "config")
	}
	flag.StringVar(&configPath, "kubeconfig", configPath, "absolute path to the kubeconfig file")
	flag.Parse()

	// Use kubeconfig if available, otherwise use in-cluster config
	var config *rest.Config
	var err error
	if _, err := os.Stat(configPath); err == nil {
		// kubeconfig exists, use it
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			panic(err.Error())
		}
	} else {
		// kubeconfig doesn't exist, assume in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	// Create the Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Define the pod specification
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "minecraft-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "minecraft-server",
					Image: "itzg/minecraft-server",
					Env: []corev1.EnvVar{
						{
							Name:  "EULA",
							Value: "TRUE",
						},
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 25565,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}

	// Create the pod in the default namespace
	podClient := clientset.CoreV1().Pods("default")
	result, err := podClient.Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Pod %q created.\n", result.Name)
}
