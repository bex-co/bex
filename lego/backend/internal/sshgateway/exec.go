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
	"strings"

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

// ErrTargetNotReady is the gateway-internal signal that the exact Pod still
// exists and is not terminal but cannot serve exec right now: it is not Ready,
// which is what a Pod on a NotReady or unreachable node reads while its phase
// still says Running. The transports map it to a 503 of its own, which bex-api
// treats as transient, never as a dead sandbox.
var ErrTargetNotReady = errors.New("sandbox target not ready")

// TargetTerminatedError is ErrTargetTerminated with what the target's container
// last reported when it ended: the kubelet's terminated reason (OOMKilled,
// Error, …) and exit code, or the Pod's own reason (Evicted) when no container
// recorded one. Both are empty for a Pod that no longer exists.
type TargetTerminatedError struct {
	Detail   string
	Reason   string
	ExitCode *int
}

func (e *TargetTerminatedError) Error() string {
	return ErrTargetTerminated.Error() + ": " + e.Detail
}

func (e *TargetTerminatedError) Unwrap() error { return ErrTargetTerminated }

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

// targetNotReady reports a Pod that is not Ready. The node lifecycle
// controller marks every Pod on a NotReady or unreachable (Unknown) node
// Ready=False, so the Pod's own Ready condition carries its node's state; the
// gateway does not read Nodes, a cluster-scoped grant it deliberately does not
// hold (scripts/gitops-validate.sh). A Pod with no Ready condition at all is
// not known to be Ready either.
func targetNotReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status != corev1.ConditionTrue
		}
	}
	return true
}

// ClassifyTarget reads one exec target's Pod after an exec into it failed: a
// *TargetTerminatedError for a terminal Pod or container, ErrTargetNotReady
// for a live Pod that is not Ready, and nil for a Ready one (the exec failed
// for another reason).
func ClassifyTarget(pod *corev1.Pod, container string) error {
	if TargetTerminated(pod, container) {
		reason, exitCode := targetTermination(pod, container)
		return &TargetTerminatedError{Detail: "pod or container is terminal", Reason: reason, ExitCode: exitCode}
	}
	if targetNotReady(pod) {
		return fmt.Errorf("%w: its node may be NotReady or unreachable", ErrTargetNotReady)
	}
	return nil
}

// targetTermination is the container's current terminated state, else its
// last one (a restarted container), else the Pod's eviction (by its ephemeral
// storage limit, or another cause), else why it is being deleted
// (DisruptionTarget: a taint eviction from a NotReady node, a preemption).
func targetTermination(pod *corev1.Pod, container string) (string, *int) {
	if pod == nil {
		return "", nil
	}
	eviction := evictionReason(pod)
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != container {
			continue
		}
		terminated := status.State.Terminated
		if terminated == nil {
			terminated = status.LastTerminationState.Terminated
		}
		// After an eviction the kubelet records a synthetic
		// ContainerStatusUnknown exit 137, which would hide the cause.
		if terminated != nil && (eviction == "" || terminated.Reason != "ContainerStatusUnknown") {
			code := int(terminated.ExitCode)
			return terminated.Reason, &code
		}
	}
	if eviction != "" {
		return eviction, nil
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason, nil
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.DisruptionTarget && condition.Status == corev1.ConditionTrue {
			return condition.Reason, nil
		}
	}
	return "", nil
}

// evictionReason is "EvictedEphemeralStorage" for a Pod the kubelet evicted
// for exceeding its ephemeral storage limit (recognised by the kubelet's fixed
// wording, so no node text is kept), "Evicted" for any other eviction, else "".
func evictionReason(pod *corev1.Pod) string {
	if pod.Status.Phase != corev1.PodFailed || pod.Status.Reason != "Evicted" {
		return ""
	}
	message := pod.Status.Message
	if strings.Contains(message, "ephemeral local storage usage exceeds") || strings.Contains(message, "exceeded its local ephemeral storage limit") {
		return "EvictedEphemeralStorage"
	}
	return "Evicted"
}

func (e *KubeExecutor) checkTarget(ctx context.Context, target apps.SSHInstanceTarget) error {
	pod, err := e.Client.CoreV1().Pods(target.Namespace).Get(ctx, target.PodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return &TargetTerminatedError{Detail: "pod no longer exists"}
	}
	if err != nil {
		return err
	}
	return ClassifyTarget(pod, target.Container)
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
	// error forever, and a Pod on a stalled node as not ready rather than as a
	// broken exec; healthy execs avoid an extra Kubernetes GET.
	if targetErr := e.checkTarget(ctx, target); errors.Is(targetErr, ErrTargetTerminated) || errors.Is(targetErr, ErrTargetNotReady) {
		return 126, targetErr
	}
	return 126, err
}
