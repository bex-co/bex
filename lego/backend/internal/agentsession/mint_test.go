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

package agentsession

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeConnections struct{ row store.GitConnection }

// GetGitConnectionByOwner returns the seeded row only when the repo's owner
// matches its account login (ADR075) — a mismatch is ErrNotFound, which Mint maps
// to ErrForbidden exactly as the old owner-equality check did.
func (f fakeConnections) GetGitConnectionByOwner(_ context.Context, _ string, accountLogin string) (store.GitConnection, error) {
	if f.row.AccountLogin == "" || !strings.EqualFold(f.row.AccountLogin, accountLogin) {
		return store.GitConnection{}, store.ErrNotFound
	}
	return f.row, nil
}

type fakeSessions struct {
	session store.AgentSession
	err     error
}

func (f fakeSessions) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	if f.err != nil {
		return store.AgentSession{}, f.err
	}
	if f.session.ID != "" && f.session.ID != id {
		return store.AgentSession{}, store.ErrNotFound
	}
	return f.session, nil
}

// activeSession is the durable record for the valid mint request: still running,
// in tea-a, holding a live sandbox.
func activeSession() store.AgentSession {
	return store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running"}
}

type fakeSessionGitHub struct {
	calls        int
	installation int64
	repository   string
}

func (f *fakeSessionGitHub) MintSessionInstallationToken(_ context.Context, installation int64, repository string) (github.InstallationToken, error) {
	f.calls++
	f.installation, f.repository = installation, repository
	return github.InstallationToken{Token: "ghs_scoped", ExpiresAt: time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)}, nil
}

type auditRecorder struct{ events []core.AuditEvent }

func (r *auditRecorder) Record(_ context.Context, event core.AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

func validMintRequest() MintRequest {
	return MintRequest{
		SessionID: "ags-one", Workspace: "tea-a", Repository: "octo/repo",
		Branch: "bex-agent/task-1", PodName: "sbx-1-0", PodUID: "uid-one",
	}
}

func TestMinterScopesRepoWritesAndAudits(t *testing.T) {
	gh := &fakeSessionGitHub{}
	audit := &auditRecorder{}
	m := &Minter{
		GitHub: gh, Connections: fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "Octo"}},
		Sessions: fakeSessions{session: activeSession()},
		Audit:    audit, Now: func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) },
	}
	got, err := m.Mint(context.Background(), validMintRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "ghs_scoped" || got.Username != "x-access-token" || gh.calls != 1 || gh.installation != 42 || gh.repository != "repo" {
		t.Fatalf("response=%+v github calls=%d installation=%d repo=%q", got, gh.calls, gh.installation, gh.repository)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(audit.events))
	}
	ev := audit.events[0]
	if ev.Verb != AuditVerbMintCredential || ev.Resource != "workspace:tea-a" || ev.Target != "agent-session:ags-one" || ev.TargetName != "octo/repo" || ev.Outcome != core.AuditAllowed {
		t.Fatalf("audit event = %+v", ev)
	}
}

func TestMinterRefusesWrongBranchOrInstallationOwner(t *testing.T) {
	for _, mutate := range []func(*MintRequest){
		func(r *MintRequest) { r.Branch = "main" },
		func(r *MintRequest) { r.Repository = "other/repo" },
	} {
		gh := &fakeSessionGitHub{}
		audit := &auditRecorder{}
		m := &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Sessions: fakeSessions{session: activeSession()}, Audit: audit}
		req := validMintRequest()
		mutate(&req)
		_, err := m.Mint(context.Background(), req)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("Mint error = %v, want forbidden", err)
		}
		if gh.calls != 0 {
			t.Fatal("refused mint reached GitHub")
		}
		if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied {
			t.Fatalf("denied audit = %+v", audit.events)
		}
	}
}

func TestMinterRefusesTerminalOrForeignSession(t *testing.T) {
	// codex F12: a retained terminal sandbox pod (or a foreign/absent session)
	// must not mint a fresh repository write token, even though its pod identity
	// is otherwise intact.
	cases := map[string]fakeSessions{
		"completed": {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "completed"}},
		"failed":    {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "failed"}},
		"canceled":  {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "canceled"}},
		// round-5 finding 10: Cancel persists "canceling" before external teardown
		// and leaves it on a teardown failure, so a still-live sandbox must not mint.
		"canceling":         {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "canceling"}},
		"hibernating":       {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "hibernating"}},
		"hibernated":        {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "hibernated"}},
		"unknown phase":     {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "future-phase"}},
		"sandbox cleared":   {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "", Phase: "running"}},
		"foreign workspace": {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-b", SandboxID: "sbx-1", Phase: "running"}},
		"stale generation":  {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-2", Phase: "running"}},
		"absent session":    {err: store.ErrNotFound},
	}
	for name, sessions := range cases {
		t.Run(name, func(t *testing.T) {
			gh := &fakeSessionGitHub{}
			audit := &auditRecorder{}
			m := &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}}, Sessions: sessions, Audit: audit}
			_, err := m.Mint(context.Background(), validMintRequest())
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("Mint error = %v, want forbidden", err)
			}
			if gh.calls != 0 {
				t.Fatal("refused mint reached GitHub")
			}
			if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied {
				t.Fatalf("denied audit = %+v", audit.events)
			}
		})
	}
}

func TestMinterAuditsMalformedRepositoryWithoutRawInput(t *testing.T) {
	gh := &fakeSessionGitHub{}
	audit := &auditRecorder{}
	m := &Minter{GitHub: gh, Connections: fakeConnections{}, Sessions: fakeSessions{session: activeSession()}, Audit: audit}
	req := validMintRequest()
	req.Repository = "octo/repo\nforged-audit-line"
	_, err := m.Mint(context.Background(), req)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Mint error = %v, want invalid request", err)
	}
	if gh.calls != 0 {
		t.Fatal("malformed mint reached GitHub")
	}
	if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied || audit.events[0].TargetName != "" {
		t.Fatalf("malformed audit = %+v", audit.events)
	}
}
