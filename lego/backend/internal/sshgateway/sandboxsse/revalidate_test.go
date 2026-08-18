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
)

// relChecker allows exactly the recorded (subject, relation) pairs on both the
// cached and fresh paths.
type relChecker struct{ allow map[string]bool }

func (c relChecker) key(subject, relation string) string { return subject + "|" + relation }

func (c relChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	return c.allow[c.key(subject, relation)], nil
}

// freshRevokedChecker answers like relChecker on the cached path but denies
// everything fresh — the member revoked moments ago whose positive is still
// warm on another replica (the round-5 finding-4 model the revalidator must
// not be fooled by).
type freshRevokedChecker struct{ relChecker }

func (c freshRevokedChecker) CheckFresh(_ context.Context, subject, relation, _ string) (bool, error) {
	_ = c.key(subject, relation)
	return false, nil
}

func TestExecRevalidatorRelations(t *testing.T) {
	base := func(checker core.Checker) *ExecRevalidator {
		return &ExecRevalidator{Base: &core.Base{Authz: checker}}
	}
	caller := sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Workspace: "tea-a", Command: []string{"sh"},
	}

	t.Run("system subject exempt", func(t *testing.T) {
		claims := caller
		claims.Subject = sandboxexec.SystemSubject
		if err := base(relChecker{}).RevalidateExec(context.Background(), claims); err != nil {
			t.Fatalf("system ticket revalidated against roles: %v", err)
		}
	})

	t.Run("caller ticket needs workspace can_operate", func(t *testing.T) {
		allow := relChecker{allow: map[string]bool{"user:id-a|" + core.RelCanOperate: true}}
		if err := base(allow).RevalidateExec(context.Background(), caller); err != nil {
			t.Fatalf("authorized caller: %v", err)
		}
		if err := base(relChecker{}).RevalidateExec(context.Background(), caller); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("no-relation caller = %v, want ErrForbidden", err)
		}
	})

	t.Run("agent session needs can_view_sensitive", func(t *testing.T) {
		claims := caller
		claims.AgentSessionID = "ags-one"
		contributor := relChecker{allow: map[string]bool{"user:id-a|" + core.RelCanOperate: true}}
		if err := base(contributor).RevalidateExec(context.Background(), claims); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("contributor on agent sandbox = %v, want ErrForbidden", err)
		}
		developer := relChecker{allow: map[string]bool{
			"user:id-a|" + core.RelCanOperate:        true,
			"user:id-a|" + core.RelCanViewSensitive: true,
		}}
		if err := base(developer).RevalidateExec(context.Background(), claims); err != nil {
			t.Fatalf("developer on agent sandbox: %v", err)
		}
	})

	t.Run("fresh decision required", func(t *testing.T) {
		stale := freshRevokedChecker{relChecker: relChecker{allow: map[string]bool{
			"user:id-a|" + core.RelCanOperate: true,
		}}}
		if err := base(stale).RevalidateExec(context.Background(), caller); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("stale-positive caller = %v, want ErrForbidden", err)
		}
	})

	t.Run("workspace-less caller ticket refused", func(t *testing.T) {
		claims := caller
		claims.Workspace = ""
		if err := base(relChecker{allow: map[string]bool{}}).RevalidateExec(context.Background(), claims); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("workspace-less ticket = %v, want ErrForbidden", err)
		}
	})
}

// TestSandboxExecRedemptionRecheck (round-13 #3): a ticket whose subject lost
// the relation between bex-api's mint and the gateway redemption is refused
// with 403 before any pods/exec — the agentattach redemption pattern extended
// to the exec transport.
func TestSandboxExecRedemptionRecheck(t *testing.T) {
	secret := []byte("exec-secret")
	exec := &countingExecutor{}
	srv := &Server{
		Secret: secret,
		Executor:  exec,
		Metrics:   sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:    sshgateway.NewSessionLimiter(100, 5),
		Nonces:    &sshgateway.NonceGuard{},
		Revalidator: &ExecRevalidator{Base: &core.Base{Authz: relChecker{}}},
		SessionTimeout: time.Minute,
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox", Workspace: "tea-a",
		Command: []string{"/bin/sh", "-c", "id"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, httpSrv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("revoked-subject redemption = %d, want 403", resp.StatusCode)
	}
	if exec.calls.Load() != 0 {
		t.Fatalf("refused redemption still exec'd %d time(s)", exec.calls.Load())
	}
}

// TestSandboxExecRevocationEndsLiveStream (round-13 #3): an ESTABLISHED exec
// stream is re-validated while it runs; a revocation mid-command cancels the
// exec context and ends the stream with a revoked error event instead of
// running to the 4h cap (the round-9 #6 watchdog, extended to this transport).
func TestSandboxExecRevocationEndsLiveStream(t *testing.T) {
	secret := []byte("exec-secret")
	// The revalidator allows the first check (redemption) and denies afterwards.
	var checks atomic.Int64
	flip := &flipRevalidator{deny: func() bool { return checks.Add(1) > 1 }}
	exec := &blockingExecutor{}
	srv := &Server{
		Secret: secret,
		Executor:  exec,
		Metrics:   sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:    sshgateway.NewSessionLimiter(100, 5),
		Nonces:    &sshgateway.NonceGuard{},
		Revalidator: flip,
		SessionTimeout:     time.Minute,
		RevalidateInterval: 20 * time.Millisecond,
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	tok, _ := sandboxexec.Mint(secret, sandboxexec.Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox", Workspace: "tea-a",
		Command: []string{"/bin/sh", "-c", "sleep 300"}, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req, _ := http.NewRequest(http.MethodPost, httpSrv.URL, nil)
	req.Header.Set(sandboxexec.TicketHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initially-authorized stream = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "revoked") {
		t.Fatalf("stream body missing revocation event:\n%s", body)
	}
	if !exec.canceled.Load() {
		t.Fatal("exec context was never canceled — the watchdog did not reach the executor")
	}
}

type flipRevalidator struct {
	deny func() bool
}

func (f *flipRevalidator) RevalidateExec(context.Context, sandboxexec.Claims) error {
	if f.deny() {
		return core.ErrForbidden
	}
	return nil
}

type countingExecutor struct{ calls atomic.Int64 }

func (e *countingExecutor) Execute(context.Context, apps.SSHInstanceTarget, []string, bool, remotecommand.TerminalSizeQueue, io.Reader, io.Writer, io.Writer) (int, error) {
	e.calls.Add(1)
	return 0, nil
}

// blockingExecutor simulates a long-running command: it returns only when its
// context is canceled, recording that the cancellation happened.
type blockingExecutor struct{ canceled atomic.Bool }

func (e *blockingExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, _, _ io.Writer) (int, error) {
	<-ctx.Done()
	e.canceled.Store(true)
	return 0, ctx.Err()
}
