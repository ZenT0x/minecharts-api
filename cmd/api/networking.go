package api

import (
	"context"
	"minecharts/cmd/kubernetes"
	"net/http"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// createService creates a Kubernetes Service to expose a Minecraft server deployment
func createService(namespace, deploymentName string, serviceType corev1.ServiceType, port int32, annotations map[string]string) (*corev1.Service, error) {
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

	return kubernetes.Clientset.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
}

// getServiceDetails retrieves information about an existing service
func getServiceDetails(namespace, serviceName string) (*corev1.Service, error) {
	return kubernetes.Clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
}

// deleteService removes a service if it exists
func deleteService(namespace, serviceName string) error {
	return kubernetes.Clientset.CoreV1().Services(namespace).Delete(context.Background(), serviceName, metav1.DeleteOptions{})
}

// ExposeMinecraftServerHandler exposes a Minecraft server using the specified method
func ExposeMinecraftServerHandler(c *gin.Context) {
	// Get server info from URL parameter
	deploymentName, _ := getServerInfo(c)

	// Check if the deployment exists
	_, ok := checkDeploymentExists(c, DefaultNamespace, deploymentName)
	if !ok {
		return
	}

	// Parse request body
	var req struct {
		ExposureType string `json:"exposureType" binding:"required"`
		Domain       string `json:"domain"`
		Port         int32  `json:"port"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate exposure type
	if req.ExposureType != "ClusterIP" &&
		req.ExposureType != "NodePort" &&
		req.ExposureType != "LoadBalancer" &&
		req.ExposureType != "MCRouter" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid exposureType. Must be one of: ClusterIP, NodePort, LoadBalancer, MCRouter",
		})
		return
	}

	// Domain is required for MCRouter
	if req.ExposureType == "MCRouter" && req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Domain is required for MCRouter exposure type",
		})
		return
	}

	// Use default Minecraft port if not provided
	if req.Port <= 0 {
		req.Port = 25565
	}

	// Service name will be consistent
	serviceName := deploymentName + "-svc"

	// Clean up any existing services for this deployment
	// Ignore errors in case the resources don't exist yet
	_ = deleteService(DefaultNamespace, serviceName)

	// Create appropriate service based on exposure type
	var serviceType corev1.ServiceType
	annotations := make(map[string]string)

	switch req.ExposureType {
	case "NodePort":
		serviceType = corev1.ServiceTypeNodePort
	case "LoadBalancer":
		serviceType = corev1.ServiceTypeLoadBalancer
	case "MCRouter":
		serviceType = corev1.ServiceTypeClusterIP
		annotations["mc-router.itzg.me/externalServerName"] = req.Domain
	default:
		serviceType = corev1.ServiceTypeClusterIP
	}

	// Create the service
	service, err := createService(DefaultNamespace, deploymentName, serviceType, req.Port, annotations)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create service: " + err.Error(),
		})
		return
	}

	response := gin.H{
		"message":      "Service created",
		"serviceName":  service.Name,
		"exposureType": req.ExposureType,
		"serviceType":  string(serviceType),
	}

	// Add service-specific information to response
	switch req.ExposureType {
	case "NodePort":
		if len(service.Spec.Ports) > 0 && service.Spec.Ports[0].NodePort > 0 {
			response["nodePort"] = service.Spec.Ports[0].NodePort
		}
	case "LoadBalancer":
		// External IP might not be assigned immediately
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			ip := service.Status.LoadBalancer.Ingress[0].IP
			if ip != "" {
				response["externalIP"] = ip
			} else {
				response["externalIP"] = service.Status.LoadBalancer.Ingress[0].Hostname
			}
		} else {
			response["externalIP"] = "pending"
			response["note"] = "LoadBalancer external IP is being provisioned and may take a few minutes"
		}
	case "MCRouter":
		response["domain"] = req.Domain
		response["note"] = "MCRouter configuration created. Make sure mc-router is deployed in your cluster."
	}

	c.JSON(http.StatusOK, response)
}
