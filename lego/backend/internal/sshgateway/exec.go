/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sshgateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	kubeexec "k8s.io/client-go/util/exec"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

// ErrTargetTerminated is the stable gateway-internal signal that the exact Pod
// or workload container bound into an exec ticket is already terminal. The SSE
// transport maps it to a code; bex-api never has direct Pod access.
var ErrTargetTerminated = errors.New("sandbox target terminated")

// TargetTerminated reports whether the exact Pod/container can no longer serve
// exec or attach traffic. Both transports use this predicate so they project
// the same Kubernetes terminal states.
func TargetTerminated(pod *corev1.Pod, container string) bool {
	if pod == nil || pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		return true
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container && status.State.Terminated != nil {
			return true
		}
	}
	return false
}

func (e *KubeExecutor) checkTarget(ctx context.Context, target apps.SSHInstanceTarget) error {
	pod, err := e.Client.CoreV1().Pods(target.Namespace).Get(ctx, target.PodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("%w: pod no longer exists", ErrTargetTerminated)
	}
	if err != nil {
		return err
	}
	if TargetTerminated(pod, target.Container) {
		return fmt.Errorf("%w: pod or container is terminal", ErrTargetTerminated)
	}
	return nil
}

// Executor is the single privileged pods/exec seam shared by every transport
// (nativessh, webshell, sandboxsse). KubeExecutor is the production
// implementation and the only place pods/exec is actually issued.
type Executor interface {
	Execute(context.Context, apps.SSHInstanceTarget, []string, bool, remotecommand.TerminalSizeQueue, io.Reader, io.Writer, io.Writer) (int, error)
}

type KubeExecutor struct {
	Config *rest.Config
	Client kubernetes.Interface
}

func (e *KubeExecutor) Execute(ctx context.Context, target apps.SSHInstanceTarget, command []string, tty bool, sizes remotecommand.TerminalSizeQueue, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	req := e.Client.CoreV1().RESTClient().Post().
		Resource("pods").Name(target.PodName).Namespace(target.Namespace).
		SubResource("exec").VersionedParams(&corev1.PodExecOptions{
		Container: target.Container, Command: command,
		Stdin: stdin != nil, Stdout: stdout != nil, Stderr: stderr != nil, TTY: tty,
	}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(e.Config, http.MethodPost, req.URL())
	if err != nil {
		return 126, err
	}
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin: stdin, Stdout: stdout, Stderr: stderr, Tty: tty, TerminalSizeQueue: sizes,
	})
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(kubeexec.ExitError); ok {
		return exitErr.ExitStatus(), nil
	}
	// Classify a failed exec against fresh Pod state. A dead target must converge
	// as terminal instead of becoming a generic retrying "container not found"
	// error forever; healthy execs avoid an extra Kubernetes GET.
	if targetErr := e.checkTarget(ctx, target); errors.Is(targetErr, ErrTargetTerminated) {
		return 126, targetErr
	}
	return 126, err
}
