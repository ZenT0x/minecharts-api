package kubernetes

import (
	"bytes"
	"context"
	"time"

	"minecharts/cmd/logging"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// getMinecraftPod gets the first pod associated with a deployment
func GetMinecraftPod(namespace, deploymentName string) (*corev1.Pod, error) {
	labelSelector := "app=" + deploymentName

	logging.WithFields(
		logging.F("namespace", namespace),
		logging.F("deployment_name", deploymentName),
		logging.F("label_selector", labelSelector),
	).Debug("Looking for Minecraft pod with label selector")

	podList, err := Clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		logging.WithFields(
			logging.F("namespace", namespace),
			logging.F("deployment_name", deploymentName),
			logging.F("error", err.Error()),
		).Error("Failed to list pods")
		return nil, err
	}

	if len(podList.Items) == 0 {
		logging.WithFields(
			logging.F("namespace", namespace),
			logging.F("deployment_name", deploymentName),
		).Warn("No pods found for deployment")
		return nil, nil // No pods found
	}

	pod := &podList.Items[0]
	logging.WithFields(
		logging.F("namespace", namespace),
		logging.F("deployment_name", deploymentName),
		logging.F("pod_name", pod.Name),
		logging.F("pod_status", pod.Status.Phase),
	).Debug("Found Minecraft pod")
	return pod, nil
}

// executeCommandInPod executes a command in the specified pod and returns the output.
// This is a utility function to avoid code duplication across handlers.
func ExecuteCommandInPod(podName, namespace, containerName, command string) (stdout, stderr string, err error) {
	logging.WithFields(
		logging.F("namespace", namespace),
		logging.F("pod_name", podName),
		logging.F("container_name", containerName),
		logging.F("command", command),
	).Debug("Executing command in pod")

	// Prepare the execution request in the pod.
	execReq := Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	execReq.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   []string{"/bin/bash", "-c", command},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create buffers to capture the command output.
	var stdoutBuf, stderrBuf bytes.Buffer

	// Execute the command in the pod.
	exec, err := remotecommand.NewSPDYExecutor(Config, "POST", execReq.URL())
	if err != nil {
		logging.WithFields(
			logging.F("namespace", namespace),
			logging.F("pod_name", podName),
			logging.F("container_name", containerName),
			logging.F("error", err.Error()),
		).Error("Failed to create SPDY executor")
		return "", "", err
	}

	// Set a timeout context for the command execution.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stream the command output to our buffers.
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	})

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		logging.WithFields(
			logging.F("namespace", namespace),
			logging.F("pod_name", podName),
			logging.F("container_name", containerName),
			logging.F("command", command),
			logging.F("stdout", stdout),
			logging.F("stderr", stderr),
			logging.F("error", err.Error()),
		).Error("Command execution failed")
	} else {
		logging.WithFields(
			logging.F("namespace", namespace),
			logging.F("pod_name", podName),
			logging.F("container_name", containerName),
			logging.F("command", command),
			logging.F("stdout_length", len(stdout)),
			logging.F("stderr_length", len(stderr)),
		).Debug("Command executed successfully")
	}

	// Return the command output even if there was an error.
	return stdout, stderr, err
}
