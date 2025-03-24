package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// createService creates a Kubernetes Service to expose a Minecraft server deployment
func CreateService(namespace, deploymentName string, serviceType corev1.ServiceType, port int32, annotations map[string]string) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName + "-svc",
			Labels: map[string]string{
				"created-by": "minecharts-api",
				"app":        deploymentName,
			},
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Ports: []corev1.ServicePort{
				{
					Name:       "minecraft",
					Port:       port,
					TargetPort: intstr.FromInt32(25565),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
		},
	}

	return Clientset.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
}

// deleteService removes a service if it exists
func DeleteService(namespace, serviceName string) error {
	return Clientset.CoreV1().Services(namespace).Delete(context.Background(), serviceName, metav1.DeleteOptions{})
}

// getServiceDetails retrieves information about an existing service
func GetServiceDetails(namespace, serviceName string) (*corev1.Service, error) {
	return Clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
}
