package api

import (
	"context"
	"net/http"

	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

// CreateMinecraftPodHandler creates a new Minecraft pod with a dedicated PVC in the "minecharts" namespace.
func CreateMinecraftPodHandler(c *gin.Context) {
	var req struct {
		PodName string `json:"podName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Construct the pod and PVC names
	podName := "minecraft-server-" + req.PodName
	pvcName := podName + "-pvc" // Unique PVC name per pod

	// Define the PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: "minecharts",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: strPtr("rook-ceph-block"),
		},
	}

	// Create the PVC
	_, err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims("minecharts").Create(context.Background(), pvc, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create PVC"})
		return
	}

	// Define the Minecraft Pod with the dynamically created PVC
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

	// Create the pod
	_, err = kubernetes.Clientset.CoreV1().Pods("minecharts").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		// If pod creation fails, rollback and delete the PVC
		_ = kubernetes.Clientset.CoreV1().PersistentVolumeClaims("minecharts").Delete(context.Background(), pvcName, metav1.DeleteOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pod"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Minecraft pod created with persistent storage", "podName": podName, "pvcName": pvcName})
}

// strPtr is a helper function to return a pointer to a string
func strPtr(s string) *string {
	return &s
}

// DeleteMinecraftPodHandler deletes a Minecraft pod and its associated PVC.
func DeleteMinecraftPodHandler(c *gin.Context) {
	// Construct the pod and PVC names
	podName := "minecraft-server-" + c.Param("podName")
	pvcName := podName + "-pvc"

	// Retrieve the pod
	pod, err := kubernetes.Clientset.CoreV1().Pods("minecharts").Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	if pod.Labels["created-by"] != "minecharts-api" {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not allowed to delete this pod"})
		return
	}

	// Delete the pod
	err = kubernetes.Clientset.CoreV1().Pods("minecharts").Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod"})
		return
	}

	// Delete the PVC
	err = kubernetes.Clientset.CoreV1().PersistentVolumeClaims("minecharts").Delete(context.Background(), pvcName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pod deleted but failed to delete PVC"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod and PVC deleted", "podName": podName, "pvcName": pvcName})
}

// StartMinecraftPodHandler creates a new pod using an existing PVC (or creates a new one if missing).
func StartMinecraftPodHandler(c *gin.Context) {
	podName := "minecraft-server-" + c.Param("podName")
	pvcName := podName + "-pvc"

	// Check if the PVC exists
	_, err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims("minecharts").Get(context.Background(), pvcName, metav1.GetOptions{})
	if err != nil {
		// If PVC does not exist, create it
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: "minecharts",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
				StorageClassName: strPtr("rook-ceph-block"),
			},
		}

		_, err := kubernetes.Clientset.CoreV1().PersistentVolumeClaims("minecharts").Create(context.Background(), pvc, metav1.CreateOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create PVC"})
			return
		}
	}

	// Define the Minecraft Pod
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

	// Create the pod
	_, err = kubernetes.Clientset.CoreV1().Pods("minecharts").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pod"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Minecraft server started", "podName": podName, "pvcName": pvcName})
}

// StopMinecraftPodHandler deletes a Minecraft pod but keeps its PVC.
func StopMinecraftPodHandler(c *gin.Context) {
	podName := "minecraft-server-" + c.Param("podName")

	// Check if the pod exists
	_, err := kubernetes.Clientset.CoreV1().Pods("minecharts").Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	// Delete the pod only (keep the PVC)
	err = kubernetes.Clientset.CoreV1().Pods("minecharts").Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pod"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod stopped, PVC retained", "podName": podName})
}
