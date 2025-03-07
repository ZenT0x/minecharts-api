package api

import (
	"bytes"
	"context"
	"minecharts/cmd/kubernetes"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
)

// Global configuration variables, configurable via environment variables.
var (
	DefaultNamespace = getEnv("MINECHARTS_NAMESPACE", "minecharts")
	PodPrefix        = getEnv("MINECHARTS_POD_PREFIX", "minecraft-server-")
	PVCSuffix        = getEnv("MINECHARTS_PVC_SUFFIX", "-pvc")
	StorageSize      = getEnv("MINECHARTS_STORAGE_SIZE", "10Gi")
	StorageClass     = getEnv("MINECHARTS_STORAGE_CLASS", "rook-ceph-block")
	TerminationGrace = getEnvAsInt64("MINECHARTS_TERMINATION_GRACE", 310)
	StopDuration     = getEnv("MINECHARTS_STOP_DURATION", "300")
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt64(key string, fallback int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

// ensurePVC checks if a PVC exists in the given namespace; if not, it creates it.
func ensurePVC(namespace, pvcName string) error {
	_, err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
	if err == nil {
		return nil // PVC already exists.
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
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(StorageSize),
				},
			},
			StorageClassName: ptr.To(StorageClass),
		},
	}
	_, err = kubernetes.Clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
	return err
}

// createPod creates a Minecraft pod using the specified PVC, with the provided environment variables.
func createPod(namespace, podName, pvcName string, envVars []corev1.EnvVar) error {
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
	_, err := kubernetes.Clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	return err
}

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
			Name:  "STOP_DURATION",
			Value: StopDuration,
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

// StopMinecraftPodHandler only deletes the pod, keeping the PVC.
func StopMinecraftPodHandler(c *gin.Context) {
	baseName := c.Param("podName")
	podName := PodPrefix + baseName

	// Verifies that the pod exists.
	_, err := kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	// Deletes the pod while keeping the PVC.
	err = kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod stopped, PVC retained", "podName": podName})
}

// DeleteMinecraftPodHandler deletes the pod if it exists, then deletes its associated PVC.
func DeleteMinecraftPodHandler(c *gin.Context) {
	baseName := c.Param("podName")
	podName := PodPrefix + baseName
	pvcName := podName + PVCSuffix

	// Deletes the pod (if it exists).
	_ = kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})

	// Deletes the PVC.
	err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims(DefaultNamespace).Delete(context.Background(), pvcName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete PVC: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod and PVC deleted", "podName": podName, "pvcName": pvcName})
}

// ExecCommandHandler executes an arbitrary shell command in the Minecraft pod.
func ExecCommandHandler(c *gin.Context) {
	// Retrieve the base name of the pod from the URL and form the full name.
	baseName := c.Param("podName")
	podName := PodPrefix + baseName

	// Expected structure in the body to get the command.
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Prepare the execution request in the pod.
	execRequest := kubernetes.Clientset.CoreV1().RESTClient().
		Post().
		Namespace(DefaultNamespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command:   []string{"sh", "-c", req.Command},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
			Container: "minecraft-server", // Must match the container name in the pod
		}, metav1.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(kubernetes.Config, "POST", execRequest.URL())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create executor: " + err.Error()})
		return
	}

	// Capture the command output
	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to execute command: " + err.Error(),
			"stderr":  stderr.String(),
			"command": req.Command,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stdout": stdout.String(), "stderr": stderr.String()})
}
