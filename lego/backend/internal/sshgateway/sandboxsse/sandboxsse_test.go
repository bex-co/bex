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
	"encoding/json"
	"fmt"
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
	// The pinned CLI decodes {status, message} and prints
	// "sandbox exec stream error status 404: sandbox is no longer running";
	// bex-api's buffered readers key on the internal code (w7/m147).
	if !strings.Contains(string(body), `"status":404,"message":"sandbox is no longer running"`) ||
		!strings.Contains(string(body), `"code":"`+sandboxexec.ErrorCodeTargetTerminated+`"`) {
		t.Fatalf("terminal SSE = %s", body)
	}
}

// execEvents runs one exec through the gateway over exec and returns the SSE
// body.
func execEvents(t *testing.T, exec sshgateway.Executor) string {
	t.Helper()
	secret := []byte("exec-secret")
	srv := httptest.NewServer(newSandboxExecGateway(exec, secret).Handler())
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
	return string(body)
}

// A Pod on a NotReady node answers its own code, not the generic
// "exec failed to start", so bex-api never reads a node stall as a broken or
// unconfigured sandbox.
func TestSandboxExecCodesUnavailableTarget(t *testing.T) {
	body := execEvents(t, &echoExecutor{code: 126, err: fmt.Errorf("%w: pod is not Ready", sshgateway.ErrTargetNotReady)})
	if !strings.Contains(body, `"status":503`) ||
		!strings.Contains(body, `"code":"`+sandboxexec.ErrorCodeTargetUnavailable+`"`) ||
		strings.Contains(body, "exec failed to start") {
		t.Fatalf("unavailable SSE = %s", body)
	}
}

// A terminal target event carries how its container ended.
func TestSandboxExecTerminalTargetCarriesItsExit(t *testing.T) {
	code := 137
	body := execEvents(t, &echoExecutor{code: 126, err: &sshgateway.TargetTerminatedError{Detail: "pod or container is terminal", Reason: "OOMKilled", ExitCode: &code}})
	if !strings.Contains(body, `"code":"`+sandboxexec.ErrorCodeTargetTerminated+`"`) ||
		!strings.Contains(body, `"terminatedReason":"OOMKilled","terminatedExitCode":137`) {
		t.Fatalf("terminal SSE = %s", body)
	}
	if bare := execEvents(t, &echoExecutor{code: 126, err: sshgateway.ErrTargetTerminated}); strings.Contains(bare, "terminatedReason") || strings.Contains(bare, "terminatedExitCode") {
		t.Fatalf("a termination nothing recorded was invented: %s", bare)
	}
}

func newSandboxExecGateway(exec sshgateway.Executor, secret []byte) *Server {
	return &Server{
		Executor: exec, Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Secret: secret, SessionTimeout: time.Minute,
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
	// Render CLI SSE contract (pkg/sandbox/sse.go at every pin): `event: output`
	// + {stream,data}, `event: exit` + {exit_code}. The legacy `exitCode` rides
	// along for one release (w7/m147).
	if !strings.Contains(got, "event: output") || !strings.Contains(got, `"stream":"stdout"`) || !strings.Contains(got, `"data":"hello\n"`) {
		t.Errorf("missing/incorrect output event in:\n%s", got)
	}
	if !strings.Contains(got, "event: exit\ndata: {\"exit_code\":0,\"exitCode\":0}\n\n") {
		t.Errorf("exit event is not the pinned CLI shape in:\n%s", got)
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

// A stream's slot is reported on the sandbox gauge for its pool, held while
// the exec runs and freed when it ends, never on active_sessions: that gauge
// says how close native SSH, the web shell and agent attach are to
// BEX_SSH_MAX_SESSIONS, which sandbox streams no longer spend (w22/m623).
func TestSandboxExecGaugeBracketsItsStreamSlot(t *testing.T) {
	secret := []byte("exec-secret")
	registry := prometheus.NewRegistry()
	exec := &gateExecutor{started: make(chan struct{}), release: make(chan struct{})}
	gw := &Server{
		Secret: secret, Executor: exec,
		Metrics:        sshgateway.NewMetrics(registry),
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
	if got := sandboxStreams(t, registry, "caller"); got != 1 {
		t.Fatalf("active_sandbox_streams{caller} while slot held = %v, want 1", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_active_sessions", nil); got != 0 {
		t.Fatalf("active_sessions while a sandbox stream runs = %v, want 0", got)
	}
	close(exec.release)
	if resp := <-respCh; resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("exec response = %+v, want 200", resp)
	}
	if got := sandboxStreams(t, registry, "caller"); got != 0 {
		t.Fatalf("active_sandbox_streams{caller} after release = %v, want 0", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_sessions_total", map[string]string{"result": "completed"}); got != 1 {
		t.Fatalf("sessions_total{completed} = %v, want 1", got)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": "accepted"}); got != 1 {
		t.Fatalf("authentications_total{accepted} = %v, want exactly 1", got)
	}
}

func sandboxStreams(t *testing.T, registry *prometheus.Registry, pool string) float64 {
	t.Helper()
	return gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_active_sandbox_streams", map[string]string{"pool": pool})
}

// A refused stream never touched the gauge or the sessions counter: the gauge
// mirrors the pool's occupancy, and a shed exchange holds no slot. It is a
// 429, which bex-api answers with its own 429 and Retry-After.
func TestSandboxExecLimitRejectionLeavesGaugeUntouched(t *testing.T) {
	secret := []byte("exec-secret")
	registry := prometheus.NewRegistry()
	limits := sshgateway.NewSandboxStreamLimits(1, 0, 0, 0)
	end, ok := limits.Admit(nil, "id-b", false)
	if !ok {
		t.Fatal("failed to seed the pool")
	}
	defer end("closed")
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
	if got := sandboxStreams(t, registry, "caller"); got != 0 {
		t.Fatalf("active_sandbox_streams{caller} after shed = %v, want 0", got)
	}
	for _, result := range []string{"closed", "completed", "failed", "revoked"} {
		if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_sessions_total", map[string]string{"result": result}); got != 0 {
			t.Fatalf("sessions_total{%s} = %v, want 0 (no slot was held)", result, got)
		}
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": "sandbox_global"}); got != 1 {
		t.Fatalf("limit_rejections_total{sandbox_global} = %v, want 1", got)
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
	if got := sandboxStreams(t, registry, "caller"); got != 0 {
		t.Fatalf("active_sandbox_streams{caller} after revocation = %v, want 0", got)
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

// quietExecutor prints nothing for a while, then exits: the shape a Render SDK
// client reads with a 45 s read timeout that relies on the server's keep-alive.
type quietExecutor struct{ quiet time.Duration }

func (q quietExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, _, _ io.Writer) (int, error) {
	select {
	case <-time.After(q.quiet):
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func TestSandboxExecKeepsAQuietStreamAlive(t *testing.T) {
	secret := []byte("exec-secret")
	gw := newSandboxExecGateway(quietExecutor{quiet: 300 * time.Millisecond}, secret)
	gw.KeepAlive = 50 * time.Millisecond
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"sleep", "1"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	exit := strings.Index(got, "event: exit")
	if exit < 0 || strings.Count(got[:exit], ": keep-alive\n\n") < 2 {
		t.Fatalf("a quiet command got no keep-alive before its exit:\n%s", got)
	}
	if strings.Contains(got[exit:], "keep-alive") {
		t.Fatalf("a keep-alive followed the terminal event:\n%s", got)
	}
}

// splitExecutor writes each of its outputs in two writes split at one byte
// offset, stdout and stderr interleaved, as a pipe's writes can fall anywhere.
type splitExecutor struct {
	stdout, stderr string
	at             int
}

func (e splitExecutor) Execute(_ context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	so, se := min(e.at, len(e.stdout)), min(e.at, len(e.stderr))
	_, _ = stdout.Write([]byte(e.stdout[:so]))
	_, _ = stderr.Write([]byte(e.stderr[:se]))
	_, _ = stdout.Write([]byte(e.stdout[so:]))
	_, _ = stderr.Write([]byte(e.stderr[se:]))
	return 0, nil
}

// outputs reassembles each stream's data from an SSE body.
func outputs(t *testing.T, body string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, event := range strings.Split(body, "\n\n") {
		data, ok := strings.CutPrefix(event, "event: output\ndata: ")
		if !ok {
			continue
		}
		var out outputEvent
		if err := json.Unmarshal([]byte(data), &out); err != nil {
			t.Fatalf("output event %q: %v", data, err)
		}
		got[out.Stream] += out.Data
	}
	return got
}

// w32/021: a character a write boundary splits arrives whole, whichever byte
// the boundary falls after, on both streams at once. A byte invalid in the
// output itself, or a character still incomplete when the output ends, is
// replaced, as a JSON string must.
func TestSandboxExecKeepsCharactersSplitAcrossWrites(t *testing.T) {
	const text = "日志 ab é𝄞z\n"
	for at := 0; at <= len(text); at++ {
		body := execEvents(t, splitExecutor{stdout: text, stderr: "错误" + text, at: at})
		if got := outputs(t, body); got["stdout"] != text || got["stderr"] != "错误"+text {
			t.Fatalf("split after byte %d: stdout %q, stderr %q", at, got["stdout"], got["stderr"])
		}
		if !strings.Contains(body, "event: exit") {
			t.Fatalf("split after byte %d: no exit event in %s", at, body)
		}
	}
	got := outputs(t, execEvents(t, splitExecutor{stdout: "ok\xff then \xe6\x97", at: 4}))
	// encoding/json replaces each byte of an invalid sequence, as before.
	if got["stdout"] != "ok\uFFFD then \uFFFD\uFFFD" {
		t.Fatalf("invalid bytes = %q, want each byte replaced", got["stdout"])
	}
}

// holdExecutor holds every exec open until release is closed.
type holdExecutor struct{ release chan struct{} }

func (e *holdExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, _, _ io.Writer) (int, error) {
	select {
	case <-e.release:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// openExec redeems a fresh ticket for subject and returns once the gateway
// answered: on 200 the stream holds its slot until the executor lets go.
func openExec(t *testing.T, url string, secret []byte, subject string) *http.Response {
	t.Helper()
	tok, err := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: subject, SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"sleep", "60"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// w22/m623: one caller's concurrent commands are held only to the sandbox
// pool, never to the SSH session limiter's 5 per identity, so a client that
// runs Render's 1,000 execs a minute in parallel meets Render's rate first.
// At the pool's edge the gateway answers 429, and bex-api's own streams keep
// a pool of their own that a full caller pool does not refuse.
func TestOneCallerHoldsMoreThanFiveExecStreams(t *testing.T) {
	secret := []byte("exec-secret")
	registry := prometheus.NewRegistry()
	exec := &holdExecutor{release: make(chan struct{})}
	gw := &Server{
		Secret: secret, Executor: exec,
		Metrics:        sshgateway.NewMetrics(registry),
		Limits:         sshgateway.NewSandboxStreamLimits(8, 0, 2, 0),
		SessionTimeout: time.Minute,
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	defer close(exec.release)

	for i := range 8 {
		if resp := openExec(t, srv.URL, secret, "id-a"); resp.StatusCode != http.StatusOK {
			t.Fatalf("stream %d of one caller = %d, want 200", i+1, resp.StatusCode)
		}
	}
	if got := sandboxStreams(t, registry, "caller"); got != 8 {
		t.Fatalf("active_sandbox_streams{caller} = %v, want 8", got)
	}
	if resp := openExec(t, srv.URL, secret, "id-a"); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("a stream past the pool = %d, want 429", resp.StatusCode)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": "sandbox_global"}); got != 1 {
		t.Fatalf("limit_rejections_total{sandbox_global} = %v, want 1", got)
	}
	for _, subject := range []string{sandboxexec.SystemSubject, sandboxexec.ProbeSubjectPrefix + "tea-a"} {
		if resp := openExec(t, srv.URL, secret, subject); resp.StatusCode != http.StatusOK {
			t.Fatalf("bex-api's own stream (%s) behind a full caller pool = %d, want 200", subject, resp.StatusCode)
		}
	}
	if got := sandboxStreams(t, registry, "platform"); got != 2 {
		t.Fatalf("active_sandbox_streams{platform} = %v, want 2", got)
	}
	if resp := openExec(t, srv.URL, secret, sandboxexec.SystemSubject); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("a platform stream past its pool = %d, want 429", resp.StatusCode)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": "sandbox_platform_global"}); got != 1 {
		t.Fatalf("limit_rejections_total{sandbox_platform_global} = %v, want 1", got)
	}
}
