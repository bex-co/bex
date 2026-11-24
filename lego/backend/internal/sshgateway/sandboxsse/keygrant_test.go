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
	"errors"
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
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w9/125: an exec a bex API key started ("Run commands in sandboxes", what
// its ticket's mint needed) is held to the key's grant at redemption and on
// every revalidation tick, over bex-api's real grant verb.

const keyExecTick = 10 * time.Millisecond

// heldExecutor runs one command until its context ends.
type heldExecutor struct {
	started  chan struct{}
	canceled atomic.Bool
	calls    atomic.Int64
}

func (e *heldExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, _, _ io.Writer) (int, error) {
	e.calls.Add(1)
	select {
	case e.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	e.canceled.Store(true)
	return 0, ctx.Err()
}

type grantExec struct {
	exec *heldExecutor
	body chan string
	code int
}

// startGrantExec redeems one exec ticket for subject naming keyAction ("" for
// a person's), with the subject holding every relation, and waits until the
// command runs (or the redemption is refused).
func startGrantExec(t *testing.T, grants *gatewaytest.KeyGrants, subject, keyAction string) grantExec {
	t.Helper()
	secret := []byte("exec-secret")
	exec := &heldExecutor{started: make(chan struct{}, 1)}
	srv := &Server{
		Secret:   secret,
		Executor: exec,
		Metrics:  sshgateway.NewMetrics(prometheus.NewRegistry()),
		Nonces:   &sshgateway.NonceGuard{},
		Revalidator: &sshgateway.ExecRevalidator{
			Base:      &core.Base{Authz: relChecker{allow: map[string]bool{"user:" + subject + "|" + core.RelCanCreate: true}}},
			KeyGrants: &sshgateway.KeyGrantGate{Asker: grants.Client(t)},
		},
		SessionTimeout:     time.Minute,
		RevalidateInterval: keyExecTick,
	}
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)
	tok, err := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: subject, SandboxID: "os-1", Namespace: "tea-a-sandbox", Workspace: "tea-a",
		Command: []string{"/bin/sh", "-c", "sleep 300"}, KeyAction: keyAction,
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, httpSrv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	out := grantExec{exec: exec, body: make(chan string, 1), code: resp.StatusCode}
	go func() {
		b, _ := io.ReadAll(resp.Body)
		out.body <- string(b)
	}()
	if resp.StatusCode == http.StatusOK {
		select {
		case <-exec.started:
		case <-time.After(2 * time.Second):
			t.Fatal("the command never started")
		}
	}
	return out
}

func (g grantExec) ended(t *testing.T) string {
	t.Helper()
	select {
	case body := <-g.body:
		return body
	case <-time.After(3 * time.Second):
		t.Fatal("the exec stream did not end")
		return ""
	}
}

func (g grantExec) wantLive(t *testing.T, after string) {
	t.Helper()
	if g.exec.canceled.Load() {
		t.Fatalf("the exec ended after %s", after)
	}
}

func TestSandboxExecKeyStreamEndsAtTheFirstTickAfterItsGrantNarrows(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute", "sandboxes:read")
	run := startGrantExec(t, grants, "key-1", string(core.KeySandboxesExec))
	if run.code != http.StatusOK {
		t.Fatalf("a covered key's exec = %d, want 200", run.code)
	}
	mark := grants.Set("key-1", "sandboxes:read")
	if body := run.ended(t); !strings.Contains(body, "revoked") {
		t.Fatalf("stream body missing the revocation event:\n%s", body)
	}
	if n := grants.Reads() - mark; n != 1 {
		t.Fatalf("the exec ended after %d grant reads since the narrowing, want 1 (the first tick)", n)
	}
	if !run.exec.canceled.Load() {
		t.Fatal("the command's context was never canceled")
	}
}

func TestSandboxExecKeyStreamSurvivesAWideningAndAnUnrelatedChange(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute")
	run := startGrantExec(t, grants, "key-1", string(core.KeySandboxesExec))
	for _, change := range []struct {
		name string
		set  func() int
	}{
		{"no change", grants.Reads},
		{"a widening", func() int { return grants.Set("key-1", "sandbox_execution:execute", "sandboxes:read_write") }},
		{"an unrelated change", func() int { return grants.Set("key-1", "sandbox_execution:execute", "services:read") }},
		{"a widening to All", func() int { return grants.SetAll("key-1") }},
	} {
		mark := change.set()
		grants.WaitReads(t, mark+3)
		run.wantLive(t, change.name)
	}
	grants.Set("key-1", "services:read")
	if body := run.ended(t); !strings.Contains(body, "revoked") {
		t.Fatalf("a narrowing after the changes did not end the exec:\n%s", body)
	}
}

func TestSandboxExecKeyTicketIsRefusedAtRedemptionOnceItsGrantNarrowed(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandboxes:read_write")
	run := startGrantExec(t, grants, "key-1", string(core.KeySandboxesExec))
	if run.code != http.StatusForbidden {
		t.Fatalf("an uncovered key's redemption = %d, want 403", run.code)
	}
	if n := run.exec.calls.Load(); n != 0 {
		t.Fatalf("a refused redemption exec'd %d time(s)", n)
	}
}

func TestSandboxExecKeyStreamEndsWhenItsGrantCannotBeRead(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute")
	run := startGrantExec(t, grants, "key-1", string(core.KeySandboxesExec))
	grants.Fail(errors.New("postgres down"))
	if body := run.ended(t); !strings.Contains(body, "revoked") {
		t.Fatalf("an unanswerable grant did not end the exec:\n%s", body)
	}
}

func TestSandboxExecPersonStreamNeverAsksForAGrant(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Fail(errors.New("bex-api unreachable"))
	run := startGrantExec(t, grants, "user-1", "")
	if run.code != http.StatusOK {
		t.Fatalf("a person's exec = %d, want 200", run.code)
	}
	time.Sleep(10 * keyExecTick)
	run.wantLive(t, "ten ticks with the verb down")
	if n := grants.Reads(); n != 0 {
		t.Fatalf("a person's exec asked the grant verb %d times", n)
	}
}
