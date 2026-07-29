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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

// echoExecutor writes to stdout/stderr and returns a fixed exit code, recording
// the target it was asked to exec into.
type echoExecutor struct {
	stdout, stderr string
	code           int
	gotTarget      apps.SSHInstanceTarget
	gotCommand     []string
}

func (e *echoExecutor) Execute(_ context.Context, target apps.SSHInstanceTarget, command []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	e.gotTarget = target
	e.gotCommand = command
	if e.stdout != "" && stdout != nil {
		_, _ = stdout.Write([]byte(e.stdout))
	}
	if e.stderr != "" && stderr != nil {
		_, _ = stderr.Write([]byte(e.stderr))
	}
	return e.code, nil
}

func newSandboxExecGateway(exec Executor, secret []byte) *Server {
	return &Server{
		Executor: exec, Metrics: NewMetrics(prometheus.NewRegistry()),
		SandboxExecSecret: secret, MaxSessions: 100, MaxPerIdentity: 5,
		SessionTimeout: time.Minute,
	}
}

func TestSandboxExecStreamsSSE(t *testing.T) {
	secret := []byte("exec-secret")
	exec := &echoExecutor{stdout: "hello\n", code: 0}
	srv := httptest.NewServer(newSandboxExecGateway(exec, secret).SandboxExecHandler())
	defer srv.Close()

	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"/bin/sh", "-c", "echo hello"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status=%d ct=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	// Render CLI SSE contract: `event: output` + {stream,data}, `event: exit` + {exitCode}.
	if !strings.Contains(got, "event: output") || !strings.Contains(got, `"stream":"stdout"`) || !strings.Contains(got, `"data":"hello\n"`) {
		t.Errorf("missing/incorrect output event in:\n%s", got)
	}
	if !strings.Contains(got, "event: exit") || !strings.Contains(got, `"exitCode":0`) {
		t.Errorf("missing exit event in:\n%s", got)
	}
	// The gateway targeted the sandbox pod in the ticket's namespace.
	if exec.gotTarget.PodName != "os-1-0" || exec.gotTarget.Namespace != "tea-a-sandbox" || exec.gotTarget.Container != "sandbox" {
		t.Errorf("target = %+v", exec.gotTarget)
	}
}

func TestSandboxExecRejectsBadTicketAndReplay(t *testing.T) {
	secret := []byte("exec-secret")
	gw := newSandboxExecGateway(&echoExecutor{stdout: "x", code: 0}, secret)
	srv := httptest.NewServer(gw.SandboxExecHandler())
	defer srv.Close()

	do := func(ticket string) int {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
		if ticket != "" {
			req.Header.Set(sandboxexec.TicketHeader, ticket)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := do(""); code != http.StatusUnauthorized {
		t.Errorf("missing ticket = %d, want 401", code)
	}
	if code := do("garbage.sig"); code != http.StatusUnauthorized {
		t.Errorf("bad ticket = %d, want 401", code)
	}
	// Valid ticket works once, then a replay is rejected (single-use nonce).
	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"sh"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	if code := do(tok); code != http.StatusOK {
		t.Errorf("first use = %d, want 200", code)
	}
	if code := do(tok); code != http.StatusUnauthorized {
		t.Errorf("replay = %d, want 401", code)
	}
}

func TestSandboxExecDisabledWhenNoSecret(t *testing.T) {
	srv := httptest.NewServer(newSandboxExecGateway(&echoExecutor{}, nil).SandboxExecHandler())
	defer srv.Close()
	resp, _ := http.Post(srv.URL, "", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("disabled = %d, want 503", resp.StatusCode)
	}
	resp.Body.Close()
}
