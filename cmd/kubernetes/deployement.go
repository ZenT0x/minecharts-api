package kubernetes

import (
	"context"
	"net/http"
	"time"

	"minecharts/cmd/config"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkDeploymentExists checks if a deployment exists and returns an HTTP error if it does not
func CheckDeploymentExists(c *gin.Context, namespace, deploymentName string) (*appsv1.Deployment, bool) {
	deployment, err := Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return nil, false
	}
	return deployment, true
}

// createDeployment creates a Minecraft deployment using the specified PVC, with the provided environment variables.
func CreateDeployment(namespace, deploymentName, pvcName string, envVars []corev1.EnvVar) error {
	replicas := int32(config.DefaultReplicas)

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
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
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
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh", "-c",
											"mc-send-to-console save-all stop && sleep 5",
										},
									},
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

	_, err := Clientset.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	return err
}

// restartDeployment restarts a deployment by updating an annotation to trigger a rollout
func RestartDeployment(namespace, deploymentName string) error {
	deployment, err := Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}

	// Add or update a restart timestamp annotation
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	return err
}

// UpdateDeployment updates a deployment with new environment variables
func UpdateDeployment(namespace, deploymentName string, envVars []corev1.EnvVar) error {
	deployment, err := Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
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

	_, err = Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	return err
}

// SetDeploymentReplicas updates the number of replicas for a deployment
func SetDeploymentReplicas(namespace, deploymentName string, replicas int32) error {
	deployment, err := Clientset.AppsV1().Deployments(namespace).Get(
		context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	_, err = Clientset.AppsV1().Deployments(namespace).Update(
		context.Background(), deployment, metav1.UpdateOptions{})
	return err
}
