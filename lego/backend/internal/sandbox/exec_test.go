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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

func execSandboxClient(t *testing.T, owner string) *Client {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sandboxes/os-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(osSandbox{
			ID: "os-1",
			Metadata: map[string]string{
				metadataOwner:         owner,
				metadataWorkspace:     "tea-a",
				metadataRegime:        metadataSandboxRegime,
				metadataNetworkPolicy: string(NetworkPolicyDenyAll),
			},
		})
	}))
	t.Cleanup(upstream.Close)
	return NewClient(upstream.URL)
}

func TestStreamExecAuthorizesMintsTicketAndRelaysSSE(t *testing.T) {
	secret := []byte("exec-secret")
	var gotNamespace, gotSandbox string
	var gotCommand []string
	// Stub gateway: verify the ticket bex-api minted, then emit the CLI's SSE.
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		gotNamespace, gotSandbox, gotCommand = claims.Namespace, claims.SandboxID, claims.Command
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"hi\\n\"}\n\nevent: exit\ndata: {\"exitCode\":0}\n\n"))
	}))
	defer gw.Close()

	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: execSandboxClient(t, "id-a"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}
	rr := httptest.NewRecorder()
	err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"}, rr, rr.Flush)
	if err != nil {
		t.Fatalf("StreamExec: %v", err)
	}
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("resp code=%d ct=%q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if body := rr.Body.String(); !strings.Contains(body, `"stream":"stdout"`) || !strings.Contains(body, `"exitCode":0`) {
		t.Errorf("relayed body missing events:\n%s", body)
	}
	// bex-api derived the sandbox namespace from the RESOLVED workspace and signed
	// the exact command — the scoping + integrity guarantees.
	if gotNamespace != "tea-a-sandbox" || gotSandbox != "os-1" {
		t.Errorf("ticket namespace/sandbox = %q/%q", gotNamespace, gotSandbox)
	}
	if len(gotCommand) != 3 || gotCommand[0] != "/bin/sh" || gotCommand[2] != "echo hi" {
		t.Errorf("signed command = %v", gotCommand)
	}
}

func TestStreamExecUnavailableWhenUnconfigured(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}}
	rr := httptest.NewRecorder()
	err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"}, rr, rr.Flush)
	if !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Errorf("unconfigured err = %v, want ErrSandboxesUnavailable", err)
	}
}

func TestStreamExecAuthzRefusalNeverReachesGateway(t *testing.T) {
	gw := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("authz-refused exec must not reach the gateway")
	}))
	defer gw.Close()
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}, Authz: denyChecker{}},
		Client: NewClient("http://unused"),
		Exec:   &ExecConfig{Secret: []byte("s"), GatewayURL: gw.URL, Client: gw.Client()},
	}
	rr := httptest.NewRecorder()
	if err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"}, rr, rr.Flush); err == nil {
		t.Fatal("expected authz refusal")
	}
}

func TestExecBufferedParsesSSE(t *testing.T) {
	secret := []byte("exec-secret")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"hi\\n\"}\n\n" +
			"event: output\ndata: {\"stream\":\"stderr\",\"data\":\"oops\\n\"}\n\n" +
			"event: exit\ndata: {\"exitCode\":7}\n\n"))
	}))
	defer gw.Close()
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: execSandboxClient(t, "id-a"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}
	res, err := svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stdout != "hi\n" || res.Stderr != "oops\n" || res.ExitCode != 7 {
		t.Errorf("buffered result = %+v", res)
	}
}

func TestExecBufferedFailsClosedWithoutExitEvent(t *testing.T) {
	secret := []byte("exec-secret")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"partial\"}\n\n"))
	}))
	t.Cleanup(gw.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: execSandboxClient(t, "id-a"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}
	_, err := svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"})
	if !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Fatalf("truncated buffered exec = %v, want ErrSandboxesUnavailable", err)
	}
}

// TestExecBufferedCapsCumulativeOutput pins w1/m65 F8: a command producing more
// than maxExecOutputBytes of combined output is truncated at the cap (with
// Truncated=true) instead of buffering unbounded output that could exhaust the
// shared API pod. Each SSE line stays under the per-line scanner cap; it's the
// NUMBER of lines that the fix bounds.
func TestExecBufferedCapsCumulativeOutput(t *testing.T) {
	secret := []byte("exec-secret")
	chunk := strings.Repeat("x", 700_000) // < 1 MiB per SSE line; several exceed the 2 MiB cumulative cap
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		for i := 0; i < 6; i++ { // 6 * 700 KB = 4.2 MB of output, well over the 2 MiB cap
			_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"" + chunk + "\"}\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		_, _ = w.Write([]byte("event: exit\ndata: {\"exitCode\":0}\n\n"))
	}))
	defer gw.Close()
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: execSandboxClient(t, "id-a"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}
	res, err := svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "yes"})
	if err != nil {
		t.Fatalf("ExecBuffered: %v", err)
	}
	if !res.Truncated {
		t.Error("over-cap output must report Truncated=true")
	}
	if total := len(res.Stdout) + len(res.Stderr); total > maxExecOutputBytes {
		t.Errorf("buffered output = %d bytes, exceeds cap %d", total, maxExecOutputBytes)
	} else if total < maxExecOutputBytes/2 {
		t.Errorf("buffered output = %d bytes, suspiciously small (cap not exercised)", total)
	}
}

func TestExecEnforcesOwnerAndWorkspaceAdminOverride(t *testing.T) {
	secret := []byte("exec-secret")
	gatewayCalls := 0
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gatewayCalls++
		if _, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: exit\ndata: {\"exitCode\":0}\n\n"))
	}))
	t.Cleanup(gw.Close)

	svc := &Service{
		Base: &core.Base{
			Namespace: "default",
			Workspace: fakeWorkspace{"id-a": "tea-a", "id-admin": "tea-a"},
			Authz:     adminChecker{},
		},
		Client: execSandboxClient(t, "id-b"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}

	rr := httptest.NewRecorder()
	err := svc.StreamExec(identityCtx("id-a"), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "id"}, rr, rr.Flush)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cross-owner exec = %v, want non-enumerating not found", err)
	}
	if gatewayCalls != 0 {
		t.Fatalf("cross-owner exec reached gateway %d time(s)", gatewayCalls)
	}

	rr = httptest.NewRecorder()
	if err := svc.StreamExec(identityCtx("id-admin"), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "id"}, rr, rr.Flush); err != nil {
		t.Fatalf("workspace-admin exec: %v", err)
	}
	if gatewayCalls != 1 {
		t.Fatalf("workspace-admin exec reached gateway %d time(s), want 1", gatewayCalls)
	}
}

// The Completer reads a session's driver status file through the gateway exec
// boundary from a system loop with NO caller identity. The gateway rejects an
// empty-subject ticket as malformed (sandboxexec.Verify), so mintAndDial must
// fall back to a stable system subject — otherwise every status read 401s and
// the session strands in `running` forever (the w3/m43 live-E2E failure).
func TestReadSessionStatusMintsSystemSubjectWithoutIdentity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-agent" {
			sb := osSandbox{ID: "os-agent", Metadata: map[string]string{
				metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
				metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-one",
			}}
			sb.Status.State = "running" // a live sandbox: the status read must proceed
			_ = json.NewEncoder(w).Encode(sb)
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	secret := []byte("exec-secret")
	var gotSubject string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
		if err != nil {
			// This is exactly what the pre-fix empty subject produced: reject as the
			// gateway does, so the test fails loudly on regression.
			http.Error(w, "invalid ticket", http.StatusUnauthorized)
			return
		}
		gotSubject = claims.Subject
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"{\\\"state\\\":\\\"succeeded\\\"}\"}\n\n"))
		_, _ = w.Write([]byte("event: exit\ndata: {\"exitCode\":0}\n\n"))
	}))
	defer gateway.Close()

	service := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(upstream.URL),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client()},
	}
	lifecycle := &AgentSessionLifecycle{service: service}

	// context.Background() carries NO identity — the Completer's exact situation.
	raw, err := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
	if err != nil {
		t.Fatalf("ReadSessionStatus without identity: %v", err)
	}
	if !strings.Contains(raw, `"state":"succeeded"`) {
		t.Fatalf("status content = %q, want the succeeded status", raw)
	}
	if gotSubject != systemExecSubject {
		t.Fatalf("ticket subject = %q, want the system sentinel %q", gotSubject, systemExecSubject)
	}
}

// RecordTranscript (ADR051) mints an agent-session ticket signed with the exec
// HMAC (binding session/pod/turn under the system subject) and POSTs the
// gateway's recorder endpoint — the trusted Completer path, no caller identity.
func TestRecordTranscriptMintsExecSignedTicket(t *testing.T) {
	secret := []byte("exec-secret")
	var got agentsessionticket.Claims
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("record method = %s, want POST", r.Method)
		}
		c, err := agentsessionticket.Verify(secret, r.Header.Get(agentsessionticket.TicketHeader), time.Now())
		if err != nil {
			http.Error(w, "bad ticket", http.StatusUnauthorized)
			return
		}
		got = c
		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	service := &Service{
		Base:   &core.Base{Namespace: "default"},
		Client: NewClient("http://unused"),
		Exec:   &ExecConfig{Secret: secret, RecordURL: gateway.URL, Client: gateway.Client()},
	}
	lifecycle := &AgentSessionLifecycle{service: service}
	if err := lifecycle.RecordTranscript(context.Background(), "tea-a", "ags-one", "sandbox-1", 3); err != nil {
		t.Fatalf("RecordTranscript: %v", err)
	}
	if got.SessionID != "ags-one" || got.Turn != 3 || got.Pod != "sandbox-1-0" ||
		got.Namespace != "tea-a-sandbox" || got.Subject != systemExecSubject {
		t.Fatalf("record ticket claims = %+v", got)
	}
}

// With no recorder URL configured, recording is a best-effort no-op (never an
// error) so the Completer's teardown is unaffected on dev/unwired deployments.
func TestRecordTranscriptNoOpWhenUnconfigured(t *testing.T) {
	service := &Service{
		Base:   &core.Base{Namespace: "default"},
		Client: NewClient("http://unused"),
		Exec:   &ExecConfig{Secret: []byte("s")}, // no RecordURL
	}
	lifecycle := &AgentSessionLifecycle{service: service}
	if err := lifecycle.RecordTranscript(context.Background(), "tea-a", "ags-one", "sandbox-1", 1); err != nil {
		t.Fatalf("unconfigured recorder must be a no-op, got %v", err)
	}
}

// A sandbox whose OpenSandbox state is terminal/errored (its pod exited) can
// never report a success status; ReadSessionStatus must surface NotFound so the
// Completer finalizes the session as failed instead of exec-ing into a dead pod
// forever (w3/m43 crash-leg stranding).
func TestReadSessionStatusFailsClosedOnTerminalSandbox(t *testing.T) {
	for _, state := range []string{"terminated", "Failed", "Deleted", ""} {
		t.Run("state="+state, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-agent" {
					sb := osSandbox{ID: "os-agent", Metadata: map[string]string{
						metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
						metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-one",
					}}
					sb.Status.State = state
					_ = json.NewEncoder(w).Encode(sb)
					return
				}
				http.NotFound(w, r)
			}))
			defer upstream.Close()
			gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("a terminal sandbox must never be exec'd")
			}))
			defer gateway.Close()

			service := &Service{
				Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
				Client: NewClient(upstream.URL),
				Exec:   &ExecConfig{Secret: []byte("s"), GatewayURL: gateway.URL, Client: gateway.Client()},
			}
			lifecycle := &AgentSessionLifecycle{service: service}
			_, err := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
			if !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("terminal-state read err = %v, want ErrNotFound", err)
			}
		})
	}
}
