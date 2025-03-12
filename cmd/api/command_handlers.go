package api

import (
	"bytes"
	"context"
	"minecharts/cmd/kubernetes"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// executeCommandInPod executes a command in the specified pod and returns the output.
// This is a utility function to avoid code duplication across handlers.
func executeCommandInPod(podName, namespace, containerName, command string) (stdout, stderr string, err error) {
	// Prepare the execution request in the pod.
	execReq := kubernetes.Clientset.CoreV1().RESTClient().Post().
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
	exec, err := remotecommand.NewSPDYExecutor(kubernetes.Config, "POST", execReq.URL())
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
