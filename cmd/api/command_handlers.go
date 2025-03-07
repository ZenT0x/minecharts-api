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
		Command:   []string{"sh", "-c", command},
		Container: containerName,
		Stdout:    true,
		Stderr:    true,
		Stdin:     false,
		TTY:       false,
	}, scheme.ParameterCodec)

	// Create executor
	executor, err := remotecommand.NewSPDYExecutor(kubernetes.Config, "POST", execReq.URL())
	if err != nil {
		return "", "", err
	}

	// Capture the command output
	var stdoutBuf, stderrBuf bytes.Buffer
	err = executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Stdin:  nil,
	})

	return stdoutBuf.String(), stderrBuf.String(), err
}

// saveWorld sends a "save-all" command to the Minecraft server pod to save the world data.
// This is a utility function to avoid code duplication across handlers.
func saveWorld(podName, namespace string) (stdout, stderr string, err error) {
	stdout, stderr, err = executeCommandInPod(podName, namespace, "minecraft-server", "mc-send-to-console save-all")
	if err == nil {
		// Wait for the save to complete
		time.Sleep(1 * time.Second)
	}
	return stdout, stderr, err
}
