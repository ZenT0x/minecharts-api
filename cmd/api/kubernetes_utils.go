package api

import (
	"context"
	"minecharts/cmd/kubernetes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// checkDeploymentExists checks if a deployment exists and returns an HTTP error if it does not
func checkDeploymentExists(c *gin.Context, namespace, deploymentName string) (*appsv1.Deployment, bool) {
	deployment, err := kubernetes.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return nil, false
	}
	return deployment, true
}

// getServerInfo returns the deployment and PVC names from a Gin context.
func getServerInfo(c *gin.Context) (deploymentName, pvcName string) {
	// Extract the server name from the URL parameter
	serverName := c.Param("serverName")

	// Build the full deployment and PVC names
	deploymentName = DeploymentPrefix + serverName
	pvcName = deploymentName + PVCSuffix
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

// createDeployment creates a Minecraft deployment using the specified PVC, with the provided environment variables.
func createDeployment(namespace, deploymentName, pvcName string, envVars []corev1.EnvVar) error {
	replicas := int32(DefaultReplicas)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				"created-by": "minecharts-api",
				"app":        deploymentName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentName,
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
			},
		},
	}

	_, err := kubernetes.Clientset.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	return err
}

// restartDeployment restarts a deployment by updating an annotation
func restartDeployment(namespace, deploymentName string) error {
	deployment, err := kubernetes.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}

	// Add or update a restart timestamp annotation
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = kubernetes.Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	return err
}

// updateDeployment updates a deployment with new environment variables
func updateDeployment(namespace, deploymentName string, envVars []corev1.EnvVar) error {
	deployment, err := kubernetes.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Update environment variables for the minecraft-server container
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == "minecraft-server" {
			deployment.Spec.Template.Spec.Containers[i].Env = envVars
			break
		}
	}

	_, err = kubernetes.Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	return err
}

// getMinecraftPod gets the first pod associated with a deployment
func getMinecraftPod(namespace, deploymentName string) (*corev1.Pod, error) {
	labelSelector := "app=" + deploymentName

	podList, err := kubernetes.Clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, nil // No pods found
	}

	return &podList.Items[0], nil
}
