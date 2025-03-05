package api

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// Global configuration variables (modifiable via environment variables)
var (
	DefaultNamespace = getEnv("MINECHARTS_NAMESPACE", "minecharts")
	PodPrefix        = getEnv("MINECHARTS_POD_PREFIX", "minecraft-server-")
	PVCSuffix        = getEnv("MINECHARTS_PVC_SUFFIX", "-pvc")
	StorageSize      = getEnv("MINECHARTS_STORAGE_SIZE", "10Gi")
	StorageClass     = getEnv("MINECHARTS_STORAGE_CLASS", "rook-ceph-block")
	TerminationGrace = getEnvAsInt64("MINECHARTS_TERMINATION_GRACE", 310)
	StopDuration     = getEnv("MINECHARTS_STOP_DURATION", "300")
)

// getEnv returns the value of the environment variable if set, otherwise returns fallback.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsInt64 returns the environment variable as int64 if set, otherwise returns fallback.
func getEnvAsInt64(key string, fallback int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

// ensurePVC checks if a PVC exists in the given namespace; if not, it creates one using the latest API types.
func ensurePVC(namespace, pvcName string) error {
	_, err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
	if err == nil {
		return nil // PVC exists.
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			// Utilisation de VolumeResourceRequirements pour la dernière version de l'API
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: ptr.To(StorageClass),
		},
	}

	_, err = kubernetes.Clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
	return err
}

// / createPod creates a Minecraft pod using the specified PVC.
// La section Volumes utilise maintenant VolumeSource avec PersistentVolumeClaim.
func createPod(namespace, podName, pvcName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"created-by": "minecharts-api",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: ptr.To[int64](TerminationGrace),
			Containers: []corev1.Container{
				{
					Name:  "minecraft-server",
					Image: "itzg/minecraft-server",
					Env: []corev1.EnvVar{
						{
							Name:  "EULA",
							Value: "TRUE",
						},
						{
							Name:  "STOP_DURATION",
							Value: StopDuration,
						},
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 25565,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "minecraft-storage",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "minecraft-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	_, err := kubernetes.Clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	return err
}

// StartMinecraftPodHandler creates a new Minecraft pod using a dedicated PVC.
// It expects a JSON body with "podName" and an optional "env" map for additional environment variables.
func StartMinecraftPodHandler(c *gin.Context) {
	// Define the expected request structure.
	var req struct {
		PodName string            `json:"podName"`
		Env     map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Centralized configuration variables.
	baseName := req.PodName
	podName := PodPrefix + baseName
	pvcName := podName + PVCSuffix

	// Ensure the PVC exists (create it if necessary).
	if err := ensurePVC(DefaultNamespace, pvcName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure PVC: " + err.Error()})
		return
	}

	// Prepare default environment variables.
	envVars := []corev1.EnvVar{
		{
			Name:  "EULA",
			Value: "TRUE",
		},
		{
			Name:  "STOP_DURATION",
			Value: StopDuration,
		},
	}
	// Append additional environment variables provided in the request.
	for key, value := range req.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Define the Minecraft pod with the PVC mounted.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"created-by": "minecharts-api",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: ptr.To[int64](TerminationGrace),
			Containers: []corev1.Container{
				{
					Name:  "minecraft-server",
					Image: "itzg/minecraft-server",
					Env:   envVars,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 25565,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "minecraft-storage",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "minecraft-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	// Create the pod in the configured namespace.
	if _, err := kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pod: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Minecraft server started", "podName": podName, "pvcName": pvcName})
}

// StopMinecraftPodHandler deletes the pod without deleting its associated PVC.
func StopMinecraftPodHandler(c *gin.Context) {
	baseName := c.Param("podName")
	podName := PodPrefix + baseName

	// Verify the pod exists
	_, err := kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	// Delete the pod while retaining the PVC
	err = kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod stopped, PVC retained", "podName": podName})
}
