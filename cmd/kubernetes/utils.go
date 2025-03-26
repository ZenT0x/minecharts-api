package kubernetes

import (
	"time"

	"minecharts/cmd/config"
	"minecharts/cmd/logging"

	"github.com/gin-gonic/gin"
)

// getServerInfo returns the deployment and PVC names from a Gin context.
func GetServerInfo(c *gin.Context) (deploymentName, pvcName string) {
	// Extract the server name from the URL parameter
	serverName := c.Param("serverName")

	// Build the full deployment and PVC names
	deploymentName = config.DeploymentPrefix + serverName
	pvcName = deploymentName + config.PVCSuffix

	logging.WithFields(
		logging.F("server_name", serverName),
		logging.F("deployment_name", deploymentName),
		logging.F("pvc_name", pvcName),
		logging.F("remote_ip", c.ClientIP()),
	).Debug("Retrieved server info from request")

	return
}

// saveWorld sends a "save-all" command to the Minecraft server pod to save the world data.
// This is a utility function to avoid code duplication across handlers.
func SaveWorld(podName, namespace string) (stdout, stderr string, err error) {
	logging.WithFields(
		logging.F("pod_name", podName),
		logging.F("namespace", namespace),
	).Debug("Saving world data for Minecraft server")

	stdout, stderr, err = ExecuteCommandInPod(podName, namespace, "minecraft-server", "mc-send-to-console save-all")
	if err != nil {
		logging.WithFields(
			logging.F("pod_name", podName),
			logging.F("namespace", namespace),
			logging.F("error", err.Error()),
		).Error("Failed to save world data")
		return stdout, stderr, err
	}

	// Wait for the save to complete
	logging.WithFields(
		logging.F("pod_name", podName),
		logging.F("namespace", namespace),
	).Debug("World save command sent, waiting for completion")

	time.Sleep(10 * time.Second)

	logging.WithFields(
		logging.F("pod_name", podName),
		logging.F("namespace", namespace),
	).Info("World data save completed")

	return stdout, stderr, err
}
