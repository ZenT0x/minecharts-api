package kubernetes

import (
	"time"

	"minecharts/cmd/config"

	"github.com/gin-gonic/gin"
)

// getServerInfo returns the deployment and PVC names from a Gin context.
func GetServerInfo(c *gin.Context) (deploymentName, pvcName string) {
	// Extract the server name from the URL parameter
	serverName := c.Param("serverName")

	// Build the full deployment and PVC names
	deploymentName = config.DeploymentPrefix + serverName
	pvcName = deploymentName + config.PVCSuffix
	return
}

// saveWorld sends a "save-all" command to the Minecraft server pod to save the world data.
// This is a utility function to avoid code duplication across handlers.
func saveWorld(podName, namespace string) (stdout, stderr string, err error) {
	stdout, stderr, err = ExecuteCommandInPod(podName, namespace, "minecraft-server", "mc-send-to-console save-all")
	if err == nil {
		// Wait for the save to complete
		time.Sleep(10 * time.Second)
	}
	return stdout, stderr, err
}
