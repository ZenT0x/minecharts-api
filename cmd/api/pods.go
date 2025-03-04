package api

import (
	"context"
	"net/http"

	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListPodsHandler lists all pods in the "minecharts" namespace.
func ListPodsHandler(c *gin.Context) {
	pods, err := kubernetes.Clientset.CoreV1().Pods("minecharts").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pods)
}

// CreateMinecraftPodHandler creates a new Minecraft pod in the "minecharts" namespace.
// It expects a JSON body with a "podName" field.
func CreateMinecraftPodHandler(c *gin.Context) {
	var req struct {
		PodName string `json:"podName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	podName := "minecraft-server-" + req.PodName

	// Define the pod with the itzg/minecraft-server image and accept the EULA.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
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

	newPod, err := kubernetes.Clientset.CoreV1().Pods("minecharts").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, newPod)
}

// DeleteMinecraftPodHandler deletes a Minecraft pod in the "minecharts" namespace.
// It expects the pod name to be provided as a URL parameter.
func DeleteMinecraftPodHandler(c *gin.Context) {

	podName := "minecraft-server-" + c.Param("podName")

	// Delete the pod using the global Kubernetes clientset
	err := kubernetes.Clientset.CoreV1().Pods("minecharts").Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted", "podName": podName})
}
