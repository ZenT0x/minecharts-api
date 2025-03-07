package api

import (
	"context"
	"minecharts/cmd/kubernetes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// checkPodExists vérifie si un pod existe et renvoie une erreur HTTP s'il n'existe pas
func checkPodExists(c *gin.Context, namespace, podName string) (*corev1.Pod, bool) {
	pod, err := kubernetes.Clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return nil, false
	}
	return pod, true
}

// getPodInfo returns the pod and PVC names from a Gin context.
func getPodInfo(c *gin.Context) (podName, pvcName string) {
	// Extract the server name from the URL parameter
	serverName := c.Param("podName")

	// Build the full pod and PVC names
	podName = PodPrefix + serverName
	pvcName = podName + PVCSuffix
	return
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

// restartPod restarts a pod using the same configuration
func restartPod(podName, pvcName string, envVars []corev1.EnvVar) error {
	// Delete the pod
	err := kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// Wait for the pod to be completely deleted
	for range 30 {
		_, err := kubernetes.Clientset.CoreV1().Pods(DefaultNamespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			// Pod successfully deleted
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Create a new pod with the same configuration
	return createPod(DefaultNamespace, podName, pvcName, envVars)
}
