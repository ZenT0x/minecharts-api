package handlers

import (
	"net/http"

	"minecharts/cmd/config"
	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"

	corev1 "k8s.io/api/core/v1"
)

// StartMinecraftServerHandler creates the PVC (if it doesn't exist) and starts the Minecraft deployment.
// The JSON body must contain "serverName" and optionally "env" (map[string]string).
func StartMinecraftServerHandler(c *gin.Context) {
	var req struct {
		ServerName string            `json:"serverName"`
		Env        map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseName := req.ServerName
	deploymentName := config.DeploymentPrefix + baseName
	pvcName := deploymentName + config.PVCSuffix

	// Creates the PVC if it doesn't already exist.
	if err := kubernetes.EnsurePVC(config.DefaultNamespace, pvcName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure PVC: " + err.Error()})
		return
	}

	// Prepares default environment variables.
	envVars := []corev1.EnvVar{
		{
			Name:  "EULA",
			Value: "TRUE",
		},
		{
			Name:  "CREATE_CONSOLE_IN_PIPE",
			Value: "true",
		},
	}
	// Adds additional environment variables provided in the request.
	for key, value := range req.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Creates the deployment with the existing PVC (created if necessary).
	if err := kubernetes.CreateDeployment(config.DefaultNamespace, deploymentName, pvcName, envVars); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deployment: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Minecraft server started", "deploymentName": deploymentName, "pvcName": pvcName})
}

// RestartMinecraftServerHandler saves the world and then restarts the deployment.
func RestartMinecraftServerHandler(c *gin.Context) {
	deploymentName, _ := kubernetes.GetServerInfo(c)

	// Check if the deployment exists
	_, ok := kubernetes.CheckDeploymentExists(c, config.DefaultNamespace, deploymentName)
	if !ok {
		return
	}

	// Get the pod associated with this deployment to run the save command
	pod, err := kubernetes.GetMinecraftPod(config.DefaultNamespace, deploymentName)
	if err != nil || pod == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to find pod for deployment: " + deploymentName,
		})
		return
	}

	// Save the world
	stdout, stderr, err := kubernetes.SaveWorld(pod.Name, config.DeploymentPrefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "Failed to save world: " + err.Error(),
			"deploymentName": deploymentName,
		})
		return
	}

	// Wait a moment for the save to complete
	// time.Sleep(10 * time.Second)

	// Restart the deployment
	if err := kubernetes.RestartDeployment(config.DefaultNamespace, deploymentName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "Failed to restart deployment: " + err.Error(),
			"deploymentName": deploymentName,
		})
		return
	}

	response := gin.H{
		"message":        "Minecraft server restarting",
		"deploymentName": deploymentName,
	}

	if stdout != "" || stderr != "" {
		response["save_stdout"] = stdout
		response["save_stderr"] = stderr
	}

	c.JSON(http.StatusOK, response)
}

// StopMinecraftServerHandler scales the deployment to 0 replicas.
func StopMinecraftServerHandler(c *gin.Context) {
	deploymentName, _ := kubernetes.GetServerInfo(c)

	// Check if the deployment exists
	_, ok := kubernetes.CheckDeploymentExists(c, config.DefaultNamespace, deploymentName)
	if !ok {
		return
	}

	// Get the pod associated with this deployment to run the save command
	pod, err := kubernetes.GetMinecraftPod(config.DefaultNamespace, deploymentName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to find pod for deployment: " + deploymentName,
		})
		return
	}

	if pod != nil {
		// Save the world before scaling down
		_, _, err := kubernetes.ExecuteCommandInPod(pod.Name, config.DefaultNamespace, "minecraft-server", "mc-send-to-console save-all")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":          "Failed to save world: " + err.Error(),
				"deploymentName": deploymentName,
			})
			return
		}
	}

	// Scale deployment to 0
	if err := kubernetes.SetDeploymentReplicas(config.DefaultNamespace, deploymentName, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "Failed to scale deployment: " + err.Error(),
			"deploymentName": deploymentName,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Server stopped (deployment scaled to 0), data retained",
		"deploymentName": deploymentName,
	})
}

// StartStoppedServerHandler scales a stopped deployment back to 1 replica.
func StartStoppedServerHandler(c *gin.Context) {
	deploymentName, _ := kubernetes.GetServerInfo(c)

	// Check if the deployment exists
	_, ok := kubernetes.CheckDeploymentExists(c, config.DefaultNamespace, deploymentName)
	if !ok {
		return
	}

	// Scale deployment to 1
	if err := kubernetes.SetDeploymentReplicas(config.DefaultNamespace, deploymentName, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "Failed to start deployment: " + err.Error(),
			"deploymentName": deploymentName,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Server starting (deployment scaled to 1)",
		"deploymentName": deploymentName,
	})
}

func DeleteMinecraftServerHandler(c *gin.Context) {
	deploymentName, pvcName := kubernetes.GetServerInfo(c)

	// Delete the deployment if it exists
	_ = kubernetes.DeleteDeployment(config.DefaultNamespace, deploymentName)

	// Delete the PVC
	_ = kubernetes.DeletePVC(config.DefaultNamespace, pvcName)

	// Clean up network resources
	serviceName := deploymentName + "-svc"
	_ = kubernetes.DeleteService(config.DefaultNamespace, serviceName)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Deployment, PVC and network resources deleted",
		"deploymentName": deploymentName,
		"pvcName":        pvcName,
	})
}

// ExecCommandHandler executes a Minecraft command in the first pod of the deployment.
func ExecCommandHandler(c *gin.Context) {
	// Extract the server name from the URL parameter
	serverName := c.Param("serverName")
	deploymentName := config.DeploymentPrefix + serverName

	// Check if the deployment exists
	_, ok := kubernetes.CheckDeploymentExists(c, config.DefaultNamespace, deploymentName)
	if !ok {
		return
	}

	// Get the pod associated with this deployment
	pod, err := kubernetes.GetMinecraftPod(config.DefaultNamespace, deploymentName)
	if err != nil || pod == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to find running pod for deployment: " + deploymentName,
		})
		return
	}

	// Parse the command from the JSON body
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Prepare the command to send to the console
	execCommand := "mc-send-to-console " + req.Command

	// Execute the command in the pod
	stdout, stderr, err := kubernetes.ExecuteCommandInPod(pod.Name, config.DeploymentPrefix, "minecraft-server", execCommand)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to execute command: " + err.Error(),
			"stderr":  stderr,
			"command": req.Command,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stdout":  stdout,
		"stderr":  stderr,
		"command": req.Command,
	})
}
