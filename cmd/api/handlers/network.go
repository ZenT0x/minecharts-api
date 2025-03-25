package handlers

import (
	"net/http"

	"minecharts/cmd/config"
	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"

	corev1 "k8s.io/api/core/v1"
)

// ExposeServerRequest represents the request to expose a Minecraft server.
type ExposeServerRequest struct {
	ExposureType string `json:"exposureType" binding:"required" example:"NodePort"`
	Domain       string `json:"domain" example:"mc.example.com"`
	Port         int32  `json:"port" example:"25565"`
}

// ExposeMinecraftServerHandler exposes a Minecraft server using the specified method.
//
// @Summary      Expose Minecraft server
// @Description  Creates a Kubernetes service to expose the Minecraft server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     APIKeyAuth
// @Param        serverName  path      string              true  "Server name"
// @Param        request     body      ExposeServerRequest  true  "Exposure configuration"
// @Success      200         {object}  map[string]interface{}  "Service created"
// @Failure      400         {object}  map[string]string       "Invalid request"
// @Failure      401         {object}  map[string]string       "Authentication required"
// @Failure      403         {object}  map[string]string       "Permission denied"
// @Failure      404         {object}  map[string]string       "Server not found"
// @Failure      500         {object}  map[string]string       "Server error"
// @Router       /servers/{serverName}/expose [post]
func ExposeMinecraftServerHandler(c *gin.Context) {
	// Get server info from URL parameter
	serverName := c.Param("serverName")
	deploymentName := config.DeploymentPrefix + serverName

	// Check if the deployment exists
	_, ok := kubernetes.CheckDeploymentExists(c, config.DefaultNamespace, deploymentName)
	if !ok {
		return
	}

	// Parse request body
	var req ExposeServerRequest

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
	_ = kubernetes.DeleteService(config.DefaultNamespace, serviceName)

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
	service, err := kubernetes.CreateService(config.DefaultNamespace, deploymentName, serviceType, req.Port, annotations)
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
