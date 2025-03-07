package api

import (
	"context"
	"minecharts/cmd/kubernetes"
	"net/http"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StartMinecraftPodHandler creates the PVC (if it doesn't exist) and starts the Minecraft pod.
// The JSON body must contain "podName" and optionally "env" (map[string]string).
func StartMinecraftPodHandler(c *gin.Context) {
	var req struct {
		PodName string            `json:"podName"`
		Env     map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseName := req.PodName
	podName := PodPrefix + baseName
	pvcName := podName + PVCSuffix

	// Creates the PVC if it doesn't already exist.
	if err := ensurePVC(DefaultNamespace, pvcName); err != nil {
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

	// Creates the pod with the existing PVC (created if necessary).
	if err := createPod(DefaultNamespace, podName, pvcName, envVars); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pod: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Minecraft server started", "podName": podName, "pvcName": pvcName})
}

// RestartMinecraftPodHandler saves the world, stops the pod, and restarts it.
func RestartMinecraftPodHandler(c *gin.Context) {
	podName, pvcName := getPodInfo(c)

	// Check if the pod exists
	pod, ok := checkPodExists(c, DefaultNamespace, podName)
	if !ok {
		return
	}

	// Gather the existing environment variables
	var envVars []corev1.EnvVar
	if len(pod.Spec.Containers) > 0 {
		for _, container := range pod.Spec.Containers {
			if container.Name == "minecraft-server" {
				envVars = container.Env
				break
			}
		}
	}

	// Ensure the CREATE_CONSOLE_IN_PIPE environment variable is set
	consoleInPipeExists := false
	for i := range envVars {
		if envVars[i].Name == "CREATE_CONSOLE_IN_PIPE" {
			envVars[i].Value = "true"
			consoleInPipeExists = true
			break
		}
	}
	if !consoleInPipeExists {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "CREATE_CONSOLE_IN_PIPE",
			Value: "true",
		})
	}

	// Save the world
	stdout, stderr, err := saveWorld(podName, DefaultNamespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save world: " + err.Error(),
			"podName": podName,
		})
		return
	}

	// Restart the pod
	if err := restartPod(podName, pvcName, envVars); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to restart pod: " + err.Error(),
			"podName": podName,
		})
		return
	}

	response := gin.H{
		"message": "Minecraft server restarted",
		"podName": podName,
		"pvcName": pvcName,
	}

	if stdout != "" || stderr != "" {
		response["save_stdout"] = stdout
		response["save_stderr"] = stderr
	}

	c.JSON(http.StatusOK, response)
}

// StopMinecraftPodHandler only deletes the pod, keeping the PVC.
func StopMinecraftPodHandler(c *gin.Context) {
	podName, _ := getPodInfo(c)

	// Check if the pod exists
	_, ok := checkPodExists(c, DefaultNamespace, podName)
	if !ok {
		return
	}

	// Save the world
	// We save the world before stopping because when the pod is stopped
	// server is stuck in saving chunks. This way we ensure that the world is saved.
	stdout, stderr, err := saveWorld(podName, DefaultNamespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save world: " + err.Error(),
			"podName": podName,
		})
		return
	}

	// Delete the pod
	err = kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod: " + err.Error()})
		return
	}

	response := gin.H{"message": "Pod stopped, PVC retained", "podName": podName}
	if stdout != "" || stderr != "" {
		response["save_stdout"] = stdout
		response["save_stderr"] = stderr
	}
	c.JSON(http.StatusOK, response)
}

// DeleteMinecraftPodHandler deletes the pod if it exists, then deletes its associated PVC.
func DeleteMinecraftPodHandler(c *gin.Context) {
	podName, pvcName := getPodInfo(c)

	// Delete the pod if it exists
	_ = kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})

	// Delete the PVC
	err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims(DefaultNamespace).Delete(context.Background(), pvcName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete PVC: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod and PVC deleted", "podName": podName, "pvcName": pvcName})
}

// ExecCommandHandler executes a Minecraft command in the pod.
func ExecCommandHandler(c *gin.Context) {
	podName, _ := getPodInfo(c)

	// Check if the pod exists
	if _, ok := checkPodExists(c, DefaultNamespace, podName); !ok {
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
	stdout, stderr, err := executeCommandInPod(podName, DefaultNamespace, "minecraft-server", execCommand)
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
