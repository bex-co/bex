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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeConnections struct{ row store.GitConnection }

func (f fakeConnections) GetGitConnection(context.Context, string) (store.GitConnection, error) {
	return f.row, nil
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
		Branch: "bex-agent/task-1", PodName: "sandbox-one", PodUID: "uid-one",
	}
}

func TestMinterScopesRepoWritesAndAudits(t *testing.T) {
	gh := &fakeSessionGitHub{}
	audit := &auditRecorder{}
	m := &Minter{
		GitHub: gh, Connections: fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "Octo"}},
		Audit: audit, Now: func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) },
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
		m := &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Audit: audit}
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

func TestMinterAuditsMalformedRepositoryWithoutRawInput(t *testing.T) {
	gh := &fakeSessionGitHub{}
	audit := &auditRecorder{}
	m := &Minter{GitHub: gh, Connections: fakeConnections{}, Audit: audit}
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
