package kubernetes

import (
	"bytes"
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// getMinecraftPod gets the first pod associated with a deployment
func GetMinecraftPod(namespace, deploymentName string) (*corev1.Pod, error) {
	labelSelector := "app=" + deploymentName

	podList, err := Clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
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

// executeCommandInPod executes a command in the specified pod and returns the output.
// This is a utility function to avoid code duplication across handlers.
func ExecuteCommandInPod(podName, namespace, containerName, command string) (stdout, stderr string, err error) {
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

	// Return the command output even if there was an error.
	return stdoutBuf.String(), stderrBuf.String(), err
}
