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
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

func dialLiveWebSocket(baseURL, ticket string) (*websocket.Conn, *http.Response, error) {
	dialer := websocket.Dialer{
		Subprotocols: []string{wsShellSubprotocol, wsTicketPrefix + ticket},
	}
	return dialer.Dial(baseURL, nil)
}

// TestGatewayRealKubernetesWebShell is the opt-in live counterpart to the
// in-process WebSocket tests. It redeems a real bex-api ticket through the
// deployed gateway, drives a TTY inside a Ready tenant pod, observes resize,
// output, and exit propagation, then proves the ticket cannot be replayed.
// Ordinary go test remains cluster-free because both inputs are required.
func TestGatewayRealKubernetesWebShell(t *testing.T) {
	baseURL := os.Getenv("BEX_TEST_SHELL_WS_URL")
	ticket := os.Getenv("BEX_TEST_SHELL_TICKET")
	if baseURL == "" || ticket == "" {
		t.Skip("set BEX_TEST_SHELL_WS_URL and BEX_TEST_SHELL_TICKET for the real-cluster web-shell smoke")
	}

	conn, resp, err := dialLiveWebSocket(baseURL, ticket)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial deployed web shell: status=%d err=%v", status, err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":101,"rows":37}`)); err != nil {
		t.Fatalf("send resize: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("printf 'bex-web-shell-live\n'; exit 23\n")); err != nil {
		t.Fatalf("send terminal input: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var output strings.Builder
	exitCode := -1
	for {
		messageType, data, readErr := conn.ReadMessage()
		if readErr != nil {
			break
		}
		switch messageType {
		case websocket.BinaryMessage:
			output.Write(data)
		case websocket.TextMessage:
			var control serverControl
			if json.Unmarshal(data, &control) == nil {
				switch control.Type {
				case "error":
					t.Fatalf("gateway returned an error control frame: %s", control.Message)
				case "exit":
					exitCode = control.Code
				}
			}
		}
	}
	if !strings.Contains(output.String(), "bex-web-shell-live") {
		t.Fatal("terminal output did not contain the live marker")
	}
	if exitCode != 23 {
		t.Fatalf("terminal exit code = %d, want 23", exitCode)
	}

	_, replayResp, replayErr := dialLiveWebSocket(baseURL, ticket)
	if replayErr == nil {
		t.Fatal("replayed ticket unexpectedly opened a second shell")
	}
	if replayResp == nil || replayResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replayed ticket status = %v, want 401", replayResp)
	}
}

// TestGatewayRealKubernetesWebShellTimeout proves the deployed gateway closes
// an attached Kubernetes exec stream when its configured session deadline
// expires. Set the gateway's BEX_SSH_SESSION_TIMEOUT to a short value before
// running this opt-in acceptance test.
func TestGatewayRealKubernetesWebShellTimeout(t *testing.T) {
	baseURL := os.Getenv("BEX_TEST_SHELL_WS_URL")
	ticket := os.Getenv("BEX_TEST_SHELL_TICKET")
	if baseURL == "" || ticket == "" || os.Getenv("BEX_TEST_SHELL_EXPECT_TIMEOUT") != "1" {
		t.Skip("set the live Web Shell inputs and BEX_TEST_SHELL_EXPECT_TIMEOUT=1")
	}

	conn, resp, err := dialLiveWebSocket(baseURL, ticket)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial deployed web shell: status=%d err=%v", status, err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("sleep 30\n")); err != nil {
		t.Fatalf("start long-running command: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read timeout close: %v", err)
		}
		if messageType != websocket.TextMessage {
			continue
		}
		var control serverControl
		if json.Unmarshal(data, &control) == nil && control.Type == "error" {
			if control.Message != "session closed" {
				t.Fatalf("timeout message = %q, want session closed", control.Message)
			}
			return
		}
	}
}

// TestGatewayRealKubernetesWebShellPodDeletion attaches first, then deletes the
// disposable target pod and proves the browser stream closes. It is opt-in
// because it mutates the named live-test workload; its Deployment is expected
// to replace the pod.
func TestGatewayRealKubernetesWebShellPodDeletion(t *testing.T) {
	baseURL := os.Getenv("BEX_TEST_SHELL_WS_URL")
	ticket := os.Getenv("BEX_TEST_SHELL_TICKET")
	kubeconfig := os.Getenv("BEX_TEST_SHELL_KUBECONFIG")
	podName := os.Getenv("BEX_TEST_SHELL_POD")
	if baseURL == "" || ticket == "" || kubeconfig == "" || podName == "" {
		t.Skip("set the live Web Shell inputs, kubeconfig, and disposable pod name")
	}
	namespace := os.Getenv("BEX_TEST_SHELL_NAMESPACE")
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
	conn, resp, err := dialLiveWebSocket(baseURL, ticket)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial deployed web shell: status=%d err=%v", status, err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("printf 'bex-web-shell-attached\n'; sleep 30\n")); err != nil {
		t.Fatalf("start attached command: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	attached := false
	for !attached {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("wait for attached marker: %v", err)
		}
		if messageType == websocket.BinaryMessage && strings.Contains(string(data), "bex-web-shell-attached") {
			attached = true
		}
	}
	zero := int64(0)
	if err := clientset.CoreV1().Pods(namespace).Delete(
		context.Background(), podName, metav1.DeleteOptions{GracePeriodSeconds: &zero},
	); err != nil {
		t.Fatalf("delete disposable shell pod: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		var control serverControl
		if json.Unmarshal(data, &control) == nil && (control.Type == "error" || control.Type == "exit") {
			return
		}
	}
}

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
	store := &fakeGatewayStore{}
	resolver := &fakeResolver{target: apps.SSHInstanceTarget{
		ID: "srv-abcdeabcdeabcdeabcde-smoke", ServiceID: "srv-abcdeabcdeabcdeabcde",
		OwnerID: "tea-smoke", Namespace: namespace, PodName: podName, Container: core.AppContainer,
	}}
	addr, stop := startGateway(t, store, resolver, &KubeExecutor{Config: config, Client: clientset}, clientSigner)
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
	resolver := &fakeResolver{target: apps.SSHInstanceTarget{
		ID: "srv-abcdeabcdeabcdeabcde-noshell", ServiceID: "srv-abcdeabcdeabcdeabcde",
		OwnerID: "tea-smoke", Namespace: namespace, PodName: podName, Container: core.AppContainer,
	}}
	addr, stop := startGateway(t, &fakeGatewayStore{}, resolver, &KubeExecutor{Config: config, Client: clientset}, clientSigner)
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
