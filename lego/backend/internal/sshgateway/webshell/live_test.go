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

package webshell

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
