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
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	kubeexec "k8s.io/client-go/util/exec"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

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
	return 126, err
}
