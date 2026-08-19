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

package sandboxsse

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// echoExecutor writes to stdout/stderr and returns a fixed exit code, recording
// the target it was asked to exec into.
type echoExecutor struct {
	stdout, stderr string
	code           int
	err            error
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
	return e.code, e.err
}

func TestSandboxExecCodesTerminalTarget(t *testing.T) {
	secret := []byte("exec-secret")
	srv := httptest.NewServer(newSandboxExecGateway(&echoExecutor{err: sshgateway.ErrTargetTerminated}, secret).Handler())
	defer srv.Close()
	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"true"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"code":"`+sandboxexec.ErrorCodeTargetTerminated+`"`) {
		t.Fatalf("terminal SSE = %s", body)
	}
}

func newSandboxExecGateway(exec sshgateway.Executor, secret []byte) *Server {
	return &Server{
		Executor: exec, Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Secret: secret, Limits: sshgateway.NewSessionLimiter(100, 5),
		SessionTimeout: time.Minute,
	}
}

func TestSandboxExecStreamsSSE(t *testing.T) {
	secret := []byte("exec-secret")
	exec := &echoExecutor{stdout: "hello\n", code: 0}
	srv := httptest.NewServer(newSandboxExecGateway(exec, secret).Handler())
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
	if exec.gotTarget.PodName != "os-1-0" || exec.gotTarget.Namespace != "tea-a-sandbox" || exec.gotTarget.Container != sandboxexec.SandboxContainer {
		t.Errorf("target = %+v", exec.gotTarget)
	}
}

func TestSandboxExecRejectsBadTicketAndReplay(t *testing.T) {
	secret := []byte("exec-secret")
	gw := newSandboxExecGateway(&echoExecutor{stdout: "x", code: 0}, secret)
	srv := httptest.NewServer(gw.Handler())
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

// gateExecutor signals when the exec stream is live and holds it open until
// released, so a test can observe the gauge while the limiter slot is held.
type gateExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (e *gateExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, _, _ io.Writer) (int, error) {
	close(e.started)
	select {
	case <-e.release:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// w1/m76/t005: sandboxsse holds a shared-limiter slot, so it must report the
// slot to bex_ssh_gateway_active_sessions exactly as nativessh/webshell do —
// incremented while the exec stream holds the slot, decremented on release.
func TestSandboxExecSessionGaugeBracketsSharedLimiterSlot(t *testing.T) {
	secret := []byte("exec-secret")
	registry := prometheus.NewRegistry()
	exec := &gateExecutor{started: make(chan struct{}), release: make(chan struct{})}
	gw := &Server{
		Secret: secret, Executor: exec,
		Metrics:        sshgateway.NewMetrics(registry),
		Limits:         sshgateway.NewSessionLimiter(100, 5),
		SessionTimeout: time.Minute,
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"sleep"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			respCh <- nil
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		respCh <- resp
	}()

	select {
	case <-exec.started:
	case <-time.After(2 * time.Second):
		t.Fatal("exec stream never started")
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_active_sessions", nil); got != 1 {
		t.Fatalf("active_sessions while slot held = %v, want 1", got)
	}
	close(exec.release)
	if resp := <-respCh; resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("exec response = %+v, want 200", resp)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_active_sessions", nil); got != 0 {
		t.Fatalf("active_sessions after release = %v, want 0", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_sessions_total", map[string]string{"result": "completed"}); got != 1 {
		t.Fatalf("sessions_total{completed} = %v, want 1", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": "accepted"}); got != 1 {
		t.Fatalf("authentications_total{accepted} = %v, want exactly 1", got)
	}
}

// A refused acquire never touched the gauge or the sessions counter: the gauge
// mirrors the limiter's occupancy, and a shed exchange holds no slot.
func TestSandboxExecLimitRejectionLeavesGaugeUntouched(t *testing.T) {
	secret := []byte("exec-secret")
	registry := prometheus.NewRegistry()
	limits := sshgateway.NewSessionLimiter(100, 1)
	if ok, _ := limits.Acquire("id-a"); !ok {
		t.Fatal("failed to seed limiter")
	}
	defer limits.Release("id-a")
	gw := &Server{
		Secret: secret, Executor: &echoExecutor{},
		Metrics:        sshgateway.NewMetrics(registry),
		Limits:         limits,
		SessionTimeout: time.Minute,
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"true"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_active_sessions", nil); got != 0 {
		t.Fatalf("active_sessions after shed = %v, want 0", got)
	}
	for _, result := range []string{"closed", "completed", "failed", "revoked"} {
		if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_sessions_total", map[string]string{"result": result}); got != 0 {
			t.Fatalf("sessions_total{%s} = %v, want 0 (no slot was held)", result, got)
		}
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": "identity"}); got != 1 {
		t.Fatalf("limit_rejections_total{identity} = %v, want 1", got)
	}
}

// w1/m76/t005: one exchange increments authentications_total exactly once —
// the mid-stream revocation is reported as sessions_total{revoked} (like the
// other transports), never as a second authentications_total sample.
func TestSandboxExecRevokedStreamCountsOnce(t *testing.T) {
	secret := []byte("exec-secret")
	registry := prometheus.NewRegistry()
	var checks atomic.Int64
	flip := &flipRevalidator{deny: func() bool { return checks.Add(1) > 1 }}
	exec := &blockingExecutor{}
	gw := &Server{
		Secret: secret, Executor: exec,
		Metrics:            sshgateway.NewMetrics(registry),
		Limits:             sshgateway.NewSessionLimiter(100, 5),
		Nonces:             &sshgateway.NonceGuard{},
		Revalidator:        flip,
		SessionTimeout:     time.Minute,
		RevalidateInterval: 20 * time.Millisecond,
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox", Workspace: "tea-a",
		Command: []string{"/bin/sh", "-c", "sleep 300"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "revoked") {
		t.Fatalf("stream body missing revocation event:\n%s", body)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": "accepted"}); got != 1 {
		t.Fatalf("authentications_total{accepted} = %v, want exactly 1", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": "revoked"}); got != 0 {
		t.Fatalf("authentications_total{revoked} = %v, want 0 (the pre-t005 double count)", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_sessions_total", map[string]string{"result": "revoked"}); got != 1 {
		t.Fatalf("sessions_total{revoked} = %v, want 1", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_active_sessions", nil); got != 0 {
		t.Fatalf("active_sessions after revocation = %v, want 0", got)
	}
}

func TestSandboxExecDisabledWhenNoSecret(t *testing.T) {
	srv := httptest.NewServer(newSandboxExecGateway(&echoExecutor{}, nil).Handler())
	defer srv.Close()
	resp, _ := http.Post(srv.URL, "", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("disabled = %d, want 503", resp.StatusCode)
	}
	resp.Body.Close()
}
