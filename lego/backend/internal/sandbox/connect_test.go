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

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
)

// execContract is the shared golden the pinned Render CLI is driven against on
// the CLI side (lego/cli/sandbox_contract_test.go). The tests here prove the
// REAL mint route, redeem route, and gateway produce exactly its bytes, so the
// two modules — which cannot import each other (Go 1.26 vs 1.27) — agree on
// one file. Change it together with the CLI test when the pin's transport
// changes.
type execContract struct {
	Workspace string `json:"workspace"`
	SandboxID string `json:"sandboxId"`
	Command   string `json:"command"`
	Connect   struct {
		Path        string   `json:"path"`
		Query       string   `json:"query"`
		Status      int      `json:"status"`
		Fields      []string `json:"fields"`
		RespMethod  string   `json:"responseMethod"`
		URIPath     string   `json:"uriPath"`
		RequestBody struct {
			Command string `json:"command"`
		} `json:"requestBody"`
	} `json:"connect"`
	Stream struct {
		Headers     map[string]string `json:"headers"`
		Status      int               `json:"status"`
		ContentType string            `json:"contentType"`
	} `json:"stream"`
	Transcripts struct {
		Exit7      string `json:"exit7"`
		Terminated string `json:"terminated"`
	} `json:"transcripts"`
}

func loadExecContract(t *testing.T) execContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "sandbox-exec-contract.json"))
	if err != nil {
		t.Fatalf("read shared contract: %v", err)
	}
	var c execContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode shared contract: %v", err)
	}
	return c
}

// contractExecutor is the gateway's pods/exec seam: it writes the contract's
// stdout/stderr and returns exit 7, or the terminated-target error.
type contractExecutor struct {
	calls      atomic.Int32
	terminated bool
}

func (e *contractExecutor) Execute(_ context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	e.calls.Add(1)
	if e.terminated {
		return 0, sshgateway.ErrTargetTerminated
	}
	_, _ = stdout.Write([]byte("out\n"))
	_, _ = stderr.Write([]byte("err\n"))
	return 7, nil
}

// connectFixture wires the REAL pieces end to end: the gated REST mux (with
// the auth gate replaced by a fixed identity), the outside-gate redeem route,
// bex-api's exec proxy, and the real sandboxsse gateway over a fake executor.
type connectFixture struct {
	svc      *Service
	api      *httptest.Server
	executor *contractExecutor
	clock    *time.Time
	gone     *atomic.Bool // when set, the OpenSandbox upstream reports the sandbox absent
}

func newConnectFixture(t *testing.T, owner string, authz core.Checker) *connectFixture {
	t.Helper()
	secret := []byte("exec-secret")
	executor := &contractExecutor{}
	gateway := httptest.NewServer((&sandboxsse.Server{
		Executor: executor, Secret: secret,
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:  sshgateway.NewSessionLimiter(100, 5), SessionTimeout: time.Minute,
	}).Handler())
	t.Cleanup(gateway.Close)

	gone := &atomic.Bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sandboxes/os-1" || gone.Load() {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(osSandbox{ID: "os-1", Metadata: map[string]string{
			metadataOwner: owner, metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
			metadataNetworkPolicy: string(NetworkPolicyDenyAll),
		}})
	}))
	t.Cleanup(upstream.Close)

	now := time.Now().UTC().Truncate(time.Second) // the real gateway checks ticket bounds against wall time
	clock := &now
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a", "id-admin": "tea-a"}, Authz: authz, Clock: func() time.Time { return *clock }},
		Client: NewClient(upstream.URL),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client(), TTL: 60 * time.Second},
	}
	gated := http.NewServeMux()
	svc.RegisterREST(gated)
	root := http.NewServeMux()
	root.Handle("/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gated.ServeHTTP(w, r.WithContext(identityCtx("id-a")))
	}))
	root.Handle(ConnectStreamPattern, svc.ConnectStreamHandler())
	api := httptest.NewServer(root)
	t.Cleanup(api.Close)
	return &connectFixture{svc: svc, api: api, executor: executor, clock: clock, gone: gone}
}

func (f *connectFixture) mint(t *testing.T, path, command string) (*http.Response, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"command": command})
	resp, err := http.Post(f.api.URL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func (f *connectFixture) redeem(t *testing.T, uri, bearer, command string) (*http.Response, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"command": command})
	req, _ := http.NewRequest(http.MethodPost, uri, bytes.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func TestConnectRunMintsRenderConnectResponse(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	resp, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	if resp.StatusCode != c.Connect.Status {
		t.Fatalf("mint status = %d, want %d: %v", resp.StatusCode, c.Connect.Status, out)
	}
	for _, field := range c.Connect.Fields {
		if v, ok := out[field]; !ok || v == "" {
			t.Errorf("connect response missing %q: %v", field, out)
		}
	}
	if out["method"] != c.Connect.RespMethod {
		t.Errorf("method = %v, want %s", out["method"], c.Connect.RespMethod)
	}
	exe, _ := out["executionId"].(string)
	if kind, ok := id.KindOf(exe); !ok || kind != id.SandboxExecution {
		t.Errorf("executionId %q is not a minted exe- id", exe)
	}
	wantURI := f.api.URL + strings.ReplaceAll(c.Connect.URIPath, "{executionId}", exe)
	if out["uri"] != wantURI {
		t.Errorf("uri = %v, want %s", out["uri"], wantURI)
	}
	expires, err := time.Parse(time.RFC3339, out["expiresAt"].(string))
	if err != nil {
		t.Fatalf("expiresAt %v is not RFC3339: %v", out["expiresAt"], err)
	}
	if got := expires.Sub(*f.clock); got <= 0 || got > connectTokenMaxTTL {
		t.Errorf("expiresAt is %s from now, want within (0, %s]", got, connectTokenMaxTTL)
	}
	if f.executor.calls.Load() != 0 {
		t.Error("minting a connect token must not exec anything")
	}
}

func TestConnectRunRefusalsMintNothing(t *testing.T) {
	c := loadExecContract(t)
	cases := []struct {
		name       string
		fixture    *connectFixture
		path       string
		command    string
		wantStatus int
		wantCode   string
	}{
		{"unsupported operation", newConnectFixture(t, "id-a", nil), "/v1/sandboxes/os-1/runs/attach/token?ownerId=tea-a", c.Command, http.StatusBadRequest, ""},
		{"missing command", newConnectFixture(t, "id-a", nil), c.Connect.Path + "?ownerId=tea-a", "", http.StatusBadRequest, ""},
		{"unknown sandbox", newConnectFixture(t, "id-a", nil), "/v1/sandboxes/os-404/runs/stream/token?ownerId=tea-a", c.Command, http.StatusNotFound, "SANDBOX_NOT_FOUND"},
		{"foreign sandbox is the same not-found", newConnectFixture(t, "id-b", nil), c.Connect.Path + "?ownerId=tea-a", c.Command, http.StatusNotFound, "SANDBOX_NOT_FOUND"},
		{"no can_create", newConnectFixture(t, "id-a", denyChecker{}), c.Connect.Path + "?ownerId=tea-a", c.Command, http.StatusForbidden, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, out := tc.fixture.mint(t, tc.path, tc.command)
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %v", resp.StatusCode, tc.wantStatus, out)
			}
			if _, leaked := out["token"]; leaked {
				t.Fatalf("a refused mint returned a token: %v", out)
			}
			if tc.wantCode != "" && out["code"] != tc.wantCode {
				t.Errorf("code = %v, want %s", out["code"], tc.wantCode)
			}
			if msg, _ := out["message"].(string); msg == "" {
				t.Errorf("refusal carries no message: %v", out)
			}
		})
	}
}

func TestConnectStreamRedeemsOnceAndStreamsPinnedShapes(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	uri, token := out["uri"].(string), out["token"].(string)

	resp, body := f.redeem(t, uri, token, c.Command)
	if resp.StatusCode != c.Stream.Status || resp.Header.Get("Content-Type") != c.Stream.ContentType {
		t.Fatalf("redeem status=%d ct=%q body=%s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	if body != c.Transcripts.Exit7 {
		t.Fatalf("stream bytes drifted from the shared contract:\n got: %q\nwant: %q", body, c.Transcripts.Exit7)
	}
	if f.executor.calls.Load() != 1 {
		t.Fatalf("executor calls = %d, want 1", f.executor.calls.Load())
	}

	// Replay: same token, same everything — refused, and nothing runs again.
	resp, body = f.redeem(t, uri, token, c.Command)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(body, "already used") {
		t.Fatalf("replay status=%d body=%s, want 409 already used", resp.StatusCode, body)
	}
	if f.executor.calls.Load() != 1 {
		t.Fatalf("a replayed token ran the command again (%d calls)", f.executor.calls.Load())
	}
}

func TestConnectStreamRefusesMisuseBeforeAnyExec(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	uri, token := out["uri"].(string), out["token"].(string)
	exe := out["executionId"].(string)
	gatewayTicket, err := sandboxexec.Mint(f.svc.Exec.Secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox", Workspace: "tea-a",
		Command: []string{"/bin/sh", "-c", c.Command}, IssuedAt: f.clock.Unix(), ExpiresAt: f.clock.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		uri        string
		bearer     string
		command    string
		wantStatus int
	}{
		{"missing bearer", uri, "", c.Command, http.StatusUnauthorized},
		{"oauth access token is not a connect token", uri, "ory_at_not_a_connect_token", c.Command, http.StatusUnauthorized},
		{"gateway exec ticket is not a connect token", uri, gatewayTicket, c.Command, http.StatusUnauthorized},
		{"tampered token", uri, token[:len(token)-2] + "xx", c.Command, http.StatusUnauthorized},
		{"other sandbox", strings.Replace(uri, "/sandboxes/os-1/", "/sandboxes/os-2/", 1), token, c.Command, http.StatusForbidden},
		{"other execution", strings.Replace(uri, exe, "exe-00000000000000000000", 1), token, c.Command, http.StatusForbidden},
		{"altered command", uri, token, c.Command + "; id", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := f.redeem(t, tc.uri, tc.bearer, tc.command)
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, tc.wantStatus, body)
			}
			var env map[string]any
			if json.Unmarshal([]byte(body), &env) != nil || env["message"] == "" {
				t.Fatalf("refusal is not a JSON {message} body: %s", body)
			}
		})
	}
	if f.executor.calls.Load() != 0 {
		t.Fatalf("a refused redeem reached the executor (%d calls)", f.executor.calls.Load())
	}
	// The legitimate redemption still works after every refusal above: misuse
	// did not consume the nonce.
	if resp, body := f.redeem(t, uri, token, c.Command); resp.StatusCode != http.StatusOK || body != c.Transcripts.Exit7 {
		t.Fatalf("legitimate redeem after misuse: status=%d body=%q", resp.StatusCode, body)
	}
}

func TestConnectStreamExpiredTokenIsGone(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	*f.clock = f.clock.Add(connectTokenMaxTTL + 2*time.Minute)
	resp, body := f.redeem(t, out["uri"].(string), out["token"].(string), c.Command)
	if resp.StatusCode != http.StatusGone || !strings.Contains(body, "expired") {
		t.Fatalf("expired redeem status=%d body=%s, want 410 expired", resp.StatusCode, body)
	}
	if f.executor.calls.Load() != 0 {
		t.Fatal("an expired token reached the executor")
	}
}

func TestConnectStreamRechecksSandboxAtRedeem(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	f.gone.Store(true) // terminated between mint and redeem
	resp, body := f.redeem(t, out["uri"].(string), out["token"].(string), c.Command)
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(body, "SANDBOX_NOT_FOUND") {
		t.Fatalf("redeem on a vanished sandbox status=%d body=%s, want 404 SANDBOX_NOT_FOUND", resp.StatusCode, body)
	}
	if f.executor.calls.Load() != 0 {
		t.Fatal("a vanished sandbox reached the executor")
	}
}

func TestConnectStreamTerminatedTargetReportsStatusAndMessage(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	f.executor.terminated = true
	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	resp, body := f.redeem(t, out["uri"].(string), out["token"].(string), c.Command)
	if resp.StatusCode != http.StatusOK || body != c.Transcripts.Terminated {
		t.Fatalf("terminated stream status=%d\n got: %q\nwant: %q", resp.StatusCode, body, c.Transcripts.Terminated)
	}
}

// TestConnectRunHonoursTheSharedExecGate pins that the mint applies exactly the
// exec gate: a workspace admin may mint for a sandbox another member owns, the
// same override the direct exec verb grants.
func TestConnectRunHonoursTheSharedExecGate(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-b", adminChecker{})
	if resp, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("member mint for a foreign sandbox status=%d, want 404: %v", resp.StatusCode, out)
	}
	res, err := f.svc.ConnectRun(identityCtx("id-admin"), ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: ConnectOperationStream, Command: c.Command}, f.api.URL)
	if err != nil || res.Token == "" {
		t.Fatalf("admin mint: %v (%+v)", err, res)
	}
	// Redeem runs under the token's minter (id-admin), so the admin override
	// still resolves the sandbox at redeem time.
	if resp, body := f.redeem(t, res.URI, res.Token, c.Command); resp.StatusCode != http.StatusOK || body != c.Transcripts.Exit7 {
		t.Fatalf("admin redeem status=%d body=%q", resp.StatusCode, body)
	}
}

// TestBufferExecReadsPinnedAndTransitionalShapes covers bex-api's own buffered
// readers (MCP sandbox_exec, the agent-session status/hibernate reads, the
// scrub-before-suspend exec) over both the pinned CLI keys the gateway now
// emits and the pre-m147 keys a not-yet-rolled gateway still sends.
func TestBufferExecReadsPinnedAndTransitionalShapes(t *testing.T) {
	cases := []struct {
		name     string
		sse      string
		wantExit int
		wantErr  error
		wantMsg  string
	}{
		{"pinned exit_code", "event: exit\ndata: {\"exit_code\":7}\n\n", 7, nil, ""},
		{"both keys prefer pinned", "event: exit\ndata: {\"exit_code\":7,\"exitCode\":7}\n\n", 7, nil, ""},
		{"legacy exitCode", "event: exit\ndata: {\"exitCode\":9}\n\n", 9, nil, ""},
		{"pinned terminated by status", "event: error\ndata: {\"status\":404,\"message\":\"sandbox is no longer running\"}\n\n", 0, core.ErrNotFound, ""},
		{"pinned terminated by code", "event: error\ndata: {\"status\":404,\"message\":\"gone\",\"error\":\"gone\",\"code\":\"sandbox_terminated\"}\n\n", 0, core.ErrNotFound, ""},
		{"legacy terminated", "event: error\ndata: {\"error\":\"sandbox is no longer running\",\"code\":\"sandbox_terminated\"}\n\n", 0, core.ErrNotFound, ""},
		{"pinned unavailable keeps message", "event: error\ndata: {\"status\":503,\"message\":\"exec failed to start in this sandbox\"}\n\n", 0, core.ErrSandboxesUnavailable, "exec failed to start in this sandbox"},
		{"legacy unavailable keeps message", "event: error\ndata: {\"error\":\"access was revoked during this exec\"}\n\n", 0, core.ErrSandboxesUnavailable, "access was revoked during this exec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := bufferExec(&http.Response{Body: io.NopCloser(strings.NewReader(tc.sse))})
			if tc.wantErr == nil {
				if err != nil || res.ExitCode != tc.wantExit {
					t.Fatalf("exit=%d err=%v, want exit %d", res.ExitCode, err, tc.wantExit)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err=%q lost the gateway message %q", err, tc.wantMsg)
			}
		})
	}
}
