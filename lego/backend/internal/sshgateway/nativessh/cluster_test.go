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

package nativessh

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// TestGatewayRealKubernetesExec is an opt-in smoke for the protocol-to-SPDY
// boundary that unit fakes cannot prove. The caller supplies a disposable,
// Running pod whose app container has /bin/sh and BEX_SSH_SMOKE_VALUE set.
// BEX_TEST_SSH_DELETE_POD=1 also proves pod deletion closes an attached stream.
func TestGatewayRealKubernetesExec(t *testing.T) {
	kubeconfig := os.Getenv("BEX_TEST_SSH_KUBECONFIG")
	podName := os.Getenv("BEX_TEST_SSH_POD")
	if kubeconfig == "" || podName == "" {
		t.Skip("set BEX_TEST_SSH_KUBECONFIG and BEX_TEST_SSH_POD for the real-cluster SSH smoke")
	}
	namespace := os.Getenv("BEX_TEST_SSH_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pod.Status.Phase != "Running" {
		t.Fatalf("smoke pod phase = %s, want Running", pod.Status.Phase)
	}
	containerFound := false
	for _, container := range pod.Spec.Containers {
		if container.Name == core.AppContainer {
			containerFound = true
		}
	}
	if !containerFound {
		t.Fatalf("smoke pod has no %q container", core.AppContainer)
	}

	clientSigner := signer(t)
	store := &gatewaytest.FakeStore{}
	resolver := &gatewaytest.FakeResolver{Target: apps.SSHInstanceTarget{
		ID: "srv-abcdeabcdeabcdeabcde-smoke", ServiceID: "srv-abcdeabcdeabcdeabcde",
		OwnerID: "tea-smoke", Namespace: namespace, PodName: podName, Container: core.AppContainer,
	}}
	addr, stop := startGateway(t, store, resolver, &sshgateway.KubeExecutor{Config: config, Client: clientset}, clientSigner)
	defer stop()

	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde-smoke", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.Output(`printf '%s' "$BEX_SSH_SMOKE_VALUE"`)
	_ = client.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "real-kube-exec" {
		t.Fatalf("runtime environment output = %q", output)
	}

	client, err = dialGateway(addr, "srv-abcdeabcdeabcdeabcde-smoke", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	session, err = client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	err = session.Run("exit 37")
	_ = client.Close()
	var exitErr *ssh.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 37 {
		t.Fatalf("real Kubernetes exit status = %v, want 37", err)
	}

	if os.Getenv("BEX_TEST_SSH_DELETE_POD") != "1" {
		return
	}
	client, err = dialGateway(addr, "srv-abcdeabcdeabcdeabcde-smoke", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	session, err = client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Start("while :; do sleep 1; done"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := clientset.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- session.Wait() }()
	select {
	case <-wait:
	case <-time.After(75 * time.Second):
		t.Fatal("pod deletion did not close the attached SSH stream")
	}
	_ = client.Close()
}

func TestGatewayRealKubernetesShelllessImage(t *testing.T) {
	kubeconfig := os.Getenv("BEX_TEST_SSH_KUBECONFIG")
	podName := os.Getenv("BEX_TEST_SSH_SHELLLESS_POD")
	if kubeconfig == "" || podName == "" {
		t.Skip("set BEX_TEST_SSH_KUBECONFIG and BEX_TEST_SSH_SHELLLESS_POD for the real-cluster shellless smoke")
	}
	namespace := os.Getenv("BEX_TEST_SSH_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	clientSigner := signer(t)
	resolver := &gatewaytest.FakeResolver{Target: apps.SSHInstanceTarget{
		ID: "srv-abcdeabcdeabcdeabcde-noshell", ServiceID: "srv-abcdeabcdeabcdeabcde",
		OwnerID: "tea-smoke", Namespace: namespace, PodName: podName, Container: core.AppContainer,
	}}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, resolver, &sshgateway.KubeExecutor{Config: config, Client: clientset}, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde-noshell", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.CombinedOutput("true")
	var exitErr *ssh.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 126 {
		t.Fatalf("shellless Kubernetes exec = %v, want SSH exit 126", err)
	}
	if string(output) != "unable to start /bin/sh in this image\n" {
		t.Fatalf("shellless output = %q", output)
	}
}
