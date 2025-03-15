package api

import (
	"context"
	"minecharts/cmd/kubernetes"
	"net/http"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
)

// createService creates a Kubernetes Service to expose a Minecraft server deployment
func createService(namespace, deploymentName string, serviceType corev1.ServiceType, port int32) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName + "-svc",
			Labels: map[string]string{
				"created-by": "minecharts-api",
				"app":        deploymentName,
			},
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

// createIngress creates a standard Kubernetes Ingress for Minecraft server
func createIngress(namespace, deploymentName, serviceName string, port int32, host string) (*networkingv1.Ingress, error) {
	pathType := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName + "-ingress",
			Labels: map[string]string{
				"created-by": "minecharts-api",
				"app":        deploymentName,
			},
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
				// Additional annotations may be needed for TCP proxying
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return kubernetes.Clientset.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
}

// createIngressRoute creates a Traefik IngressRouteTCP CRD for Minecraft server
func createIngressRoute(namespace, deploymentName, serviceName string, port int32, host string) error {
	// Define IngressRouteTCP as unstructured object since it's a CRD
	ingressRoute := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute", // Changed from IngressRouteTCP to standard IngressRoute
			"metadata": map[string]interface{}{
				"name":      deploymentName + "-ingressroute",
				"namespace": namespace,
				"labels": map[string]interface{}{
					"created-by": "minecharts-api",
					"app":        deploymentName,
				},
			},
			"spec": map[string]interface{}{
				"entryPoints": []string{"web"},
				"routes": []map[string]interface{}{
					{
						"match": "Host(`" + host + "`)", // Using Host matcher instead of HostSNI
						"kind":  "Rule",
						"services": []map[string]interface{}{
							{
								"name": serviceName,
								"port": port,
							},
						},
					},
				},
			},
		},
	}

	// Update the GVR to match the standard IngressRoute
	ingressRouteGVR := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes", // Changed from ingressroutetcps to ingressroutes
	}

	// Get dynamic client for custom resources
	dynamicClient, err := dynamic.NewForConfig(kubernetes.Config)
	if err != nil {
		return err
	}

	// Create the resource
	_, err = dynamicClient.Resource(ingressRouteGVR).Namespace(namespace).Create(
		context.Background(), ingressRoute, metav1.CreateOptions{})
	return err
}

// getServiceDetails retrieves information about an existing service
func getServiceDetails(namespace, serviceName string) (*corev1.Service, error) {
	return kubernetes.Clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
}

// deleteService removes a service if it exists
func deleteService(namespace, serviceName string) error {
	return kubernetes.Clientset.CoreV1().Services(namespace).Delete(context.Background(), serviceName, metav1.DeleteOptions{})
}

// deleteIngress removes an ingress if it exists
func deleteIngress(namespace, ingressName string) error {
	return kubernetes.Clientset.NetworkingV1().Ingresses(namespace).Delete(context.Background(), ingressName, metav1.DeleteOptions{})
}

// deleteIngressRoute removes a Traefik IngressRoute if it exists
func deleteIngressRoute(namespace, ingressRouteName string) error {
	dynamicClient, err := dynamic.NewForConfig(kubernetes.Config)
	if err != nil {
		return err
	}

	ingressRouteGVR := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes", // Changed from ingressroutetcps to ingressroutes
	}

	return dynamicClient.Resource(ingressRouteGVR).Namespace(namespace).Delete(
		context.Background(), ingressRouteName, metav1.DeleteOptions{})
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
		req.ExposureType != "Ingress" &&
		req.ExposureType != "IngressRoute" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid exposureType. Must be one of: ClusterIP, NodePort, LoadBalancer, Ingress, IngressRoute",
		})
		return
	}

	// Domain is required for Ingress/IngressRoute
	if (req.ExposureType == "Ingress" || req.ExposureType == "IngressRoute") && req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Domain is required for Ingress/IngressRoute exposure type",
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
	_ = deleteIngress(DefaultNamespace, deploymentName+"-ingress")
	_ = deleteIngressRoute(DefaultNamespace, deploymentName+"-ingressroute")

	// Create appropriate service based on exposure type
	var serviceType corev1.ServiceType

	switch req.ExposureType {
	case "NodePort":
		serviceType = corev1.ServiceTypeNodePort
	case "LoadBalancer":
		serviceType = corev1.ServiceTypeLoadBalancer
	default:
		serviceType = corev1.ServiceTypeClusterIP
	}

	// Create the service
	service, err := createService(DefaultNamespace, deploymentName, serviceType, req.Port)
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
	switch serviceType {
	case corev1.ServiceTypeNodePort:
		if len(service.Spec.Ports) > 0 && service.Spec.Ports[0].NodePort > 0 {
			response["nodePort"] = service.Spec.Ports[0].NodePort
		}
	case corev1.ServiceTypeLoadBalancer:
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
	}

	// For Ingress or IngressRoute types, create additional resources
	if req.ExposureType == "Ingress" {
		ingress, err := createIngress(DefaultNamespace, deploymentName, serviceName, req.Port, req.Domain)
		if err != nil {
			// Don't fail the request - the service is still usable
			response["ingressError"] = err.Error()
		} else {
			response["ingress"] = ingress.Name
			response["domain"] = req.Domain
			response["note"] = "Ingress created, but may not work for Minecraft without TCP proxy support"
		}
	} else if req.ExposureType == "IngressRoute" {
		err := createIngressRoute(DefaultNamespace, deploymentName, serviceName, req.Port, req.Domain)
		if err != nil {
			// Don't fail the request - the service is still usable
			response["ingressRouteError"] = err.Error()
		} else {
			response["ingressRoute"] = deploymentName + "-ingressroute"
			response["domain"] = req.Domain
			response["note"] = "IngressRoute created, requires Traefik with a 'minecraft' entrypoint configured"
		}
	}

	c.JSON(http.StatusOK, response)
}
