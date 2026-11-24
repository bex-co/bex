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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeConnections struct{ row store.GitConnection }

// GetGitConnectionByOwner returns the seeded row only when the repo's owner
// matches its account login (ADR078) — a mismatch is ErrNotFound, which Mint maps
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

// noEnvironment is a workspace that never saved its agent environment.
var noEnvironment = fakeEnvironments{err: store.ErrNotFound}

type fakeSessionGitHub struct {
	calls         int
	readCalls     int
	apiCalls      int
	apiWriteCalls int
	installation  int64
	repository    string
}

func (f *fakeSessionGitHub) MintRepositoryAPIWriteToken(_ context.Context, installation int64, repository string) (github.InstallationToken, error) {
	f.apiWriteCalls++
	f.installation, f.repository = installation, repository
	return github.InstallationToken{Token: "ghs_api_write", ExpiresAt: time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)}, nil
}

func (f *fakeSessionGitHub) MintRepositoryAPIReadToken(_ context.Context, installation int64, repository string) (github.InstallationToken, error) {
	f.apiCalls++
	f.installation, f.repository = installation, repository
	return github.InstallationToken{Token: "ghs_api_read", ExpiresAt: time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)}, nil
}

func (f *fakeSessionGitHub) MintRepositoryReadToken(_ context.Context, installation int64, repository string) (github.InstallationToken, error) {
	f.readCalls++
	f.installation, f.repository = installation, repository
	return github.InstallationToken{Token: "ghs_read", ExpiresAt: time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)}, nil
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

// githubAPIMints are the gh route's two verbs (w10/m513 reads, w10/m522
// writes): the Minter method, the token the fake mints, its audit verb and
// which fake counter that mint advances.
var githubAPIMints = []struct {
	name   string
	mint   func(*Minter, context.Context, MintRequest) (MintResponse, error)
	token  string
	verb   string
	minted func(*fakeSessionGitHub) int
}{
	{"read", (*Minter).MintGitHubRead, "ghs_api_read", AuditVerbMintGitHubReadCredential, func(f *fakeSessionGitHub) int { return f.apiCalls }},
	{"write", (*Minter).MintGitHubWrite, "ghs_api_write", AuditVerbMintGitHubWriteCredential, func(f *fakeSessionGitHub) int { return f.apiWriteCalls }},
}

// w10/m513, w10/m522: each gh route credential goes through the same Pod,
// lifecycle and owner checks as the Git mint, mints only its own token of the
// one repository, is audited under its own verb allowed or denied, and is
// never minted for a build.
func TestMintGitHubAPICredentialsAreScopedAndAudited(t *testing.T) {
	for _, kind := range githubAPIMints {
		gh := &fakeSessionGitHub{}
		audit := &auditRecorder{}
		m := &Minter{
			GitHub: gh, Connections: fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}},
			Sessions: fakeSessions{session: activeSession()}, Environments: noEnvironment, Audit: audit,
		}
		got, err := kind.mint(m, context.Background(), validMintRequest())
		if err != nil {
			t.Fatal(err)
		}
		if got.Token != kind.token || kind.minted(gh) != 1 || gh.apiCalls+gh.apiWriteCalls != 1 || gh.calls != 0 || gh.readCalls != 0 || gh.installation != 42 || gh.repository != "repo" {
			t.Fatalf("%s: response=%+v fake=%+v", kind.name, got, gh)
		}
		if len(audit.events) != 1 || audit.events[0].Verb != kind.verb || audit.events[0].Outcome != core.AuditAllowed || audit.events[0].TargetName != "octo/repo" || audit.events[0].Target != "agent-session:ags-one" {
			t.Fatalf("%s audit = %+v", kind.name, audit.events)
		}

		for name, sessions := range map[string]fakeSessions{
			"completed": {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "completed"}},
			"stale":     {session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-2", Phase: "running"}},
			"missing":   {err: store.ErrNotFound},
		} {
			gh, audit := &fakeSessionGitHub{}, &auditRecorder{}
			m := &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Sessions: sessions, Audit: audit}
			if _, err := kind.mint(m, context.Background(), validMintRequest()); !errors.Is(err, ErrForbidden) || gh.apiCalls+gh.apiWriteCalls != 0 {
				t.Fatalf("%s, %s session: err=%v fake=%+v", kind.name, name, err, gh)
			}
			if len(audit.events) != 1 || audit.events[0].Verb != kind.verb || audit.events[0].Outcome != core.AuditDenied || audit.events[0].TargetName != "octo/repo" {
				t.Fatalf("%s, %s session audit = %+v", kind.name, name, audit.events)
			}
		}

		// Another Pod than the session's sandbox, another branch, another
		// account's repository.
		for name, mutate := range map[string]func(*MintRequest){
			"pod":     func(r *MintRequest) { r.PodName = "sbx-9-0" },
			"branch":  func(r *MintRequest) { r.Branch = "main" },
			"account": func(r *MintRequest) { r.Repository = "other/repo" },
		} {
			gh := &fakeSessionGitHub{}
			m := &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Sessions: fakeSessions{session: activeSession()}, Environments: noEnvironment, Audit: &auditRecorder{}}
			req := validMintRequest()
			mutate(&req)
			if _, err := kind.mint(m, context.Background(), req); err == nil || gh.apiCalls+gh.apiWriteCalls != 0 {
				t.Fatalf("%s, another %s: err=%v fake=%+v", kind.name, name, err, gh)
			}
		}

		// A running build's own builder, which Mint would give its clone token.
		gh = &fakeSessionGitHub{}
		audit = &auditRecorder{}
		m = &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Builds: fakeBuilds{build: runningBuild()}, Audit: audit}
		if _, err := kind.mint(m, context.Background(), buildMintRequest()); !errors.Is(err, ErrForbidden) || gh.apiCalls+gh.apiWriteCalls+gh.readCalls != 0 {
			t.Fatalf("%s, build principal: err=%v fake=%+v", kind.name, err, gh)
		}
		if len(audit.events) != 1 || audit.events[0].Verb != kind.verb || audit.events[0].Outcome != core.AuditDenied {
			t.Fatalf("%s, build principal audit = %+v", kind.name, audit.events)
		}
	}
}

// Each gh route verb answers on its own path with its own token, and the Git
// mint verb never hands one of them out.
func TestGitHubAPIHandlersMintTheirOwnTokens(t *testing.T) {
	secret := []byte("secret")
	gh := &fakeSessionGitHub{}
	minter := &Minter{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Sessions: fakeSessions{session: activeSession()}, Environments: noEnvironment}
	for _, verb := range []struct {
		handler http.Handler
		token   string
	}{
		{&GitHubReadHandler{Secret: secret, Minter: minter}, "ghs_api_read"},
		{&GitHubWriteHandler{Secret: secret, Minter: minter}, "ghs_api_write"},
		{&Handler{Secret: secret, Minter: minter}, "ghs_scoped"},
	} {
		server := httptest.NewServer(verb.handler)
		got, err := (&Client{URL: server.URL, Secret: secret, HTTP: server.Client()}).Mint(context.Background(), validMintRequest())
		if err != nil || got.Token != verb.token {
			t.Fatalf("%T = %+v %v, want %s", verb.handler, got, err, verb.token)
		}
		if _, err := (&Client{URL: server.URL, Secret: []byte("other"), HTTP: server.Client()}).Mint(context.Background(), validMintRequest()); err == nil {
			t.Fatalf("%T accepted a request signed with another secret", verb.handler)
		}
		server.Close()
	}
	if gh.calls != 1 || gh.apiCalls != 1 || gh.apiWriteCalls != 1 {
		t.Fatalf("mints = %+v, want one of each", gh)
	}
}

// w10/m524: while the workspace has its GitHub CLI setting off, both gh route
// mints are refused with ErrGitHubCLIOff (still an ErrForbidden) before any
// GitHub token is minted, and audited as denied; the Git mint of the same
// session still mints, so pushes keep working. A saved row with gh on mints,
// and an unreadable or unwired setting refuses instead of guessing.
func TestMintGitHubAPICredentialsAreRefusedWhileGitHubCLIIsOff(t *testing.T) {
	off := fakeEnvironments{row: store.AgentEnvironment{WorkspaceID: "tea-a", GitHubCLIDisabled: true}}
	connections := fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}}
	for _, kind := range githubAPIMints {
		gh, audit := &fakeSessionGitHub{}, &auditRecorder{}
		m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{session: activeSession()}, Environments: off, Audit: audit}
		_, err := kind.mint(m, context.Background(), validMintRequest())
		if !errors.Is(err, ErrGitHubCLIOff) || !errors.Is(err, ErrForbidden) || kind.minted(gh) != 0 {
			t.Fatalf("%s with gh off: err=%v fake=%+v, want ErrGitHubCLIOff and no token", kind.name, err, gh)
		}
		if len(audit.events) != 1 || audit.events[0].Verb != kind.verb || audit.events[0].Outcome != core.AuditDenied || audit.events[0].TargetName != "octo/repo" || audit.events[0].Target != "agent-session:ags-one" {
			t.Fatalf("%s with gh off audit = %+v", kind.name, audit.events)
		}
		if got, err := m.Mint(context.Background(), validMintRequest()); err != nil || got.Token != "ghs_scoped" || gh.calls != 1 {
			t.Fatalf("%s: the Git mint with gh off = %+v %v, want the push token", kind.name, got, err)
		}

		for name, environments := range map[string]EnvironmentStore{
			"no saved row":           noEnvironment,
			"a saved row with gh on": fakeEnvironments{row: store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: "full"}},
		} {
			gh := &fakeSessionGitHub{}
			m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{session: activeSession()}, Environments: environments}
			if got, err := kind.mint(m, context.Background(), validMintRequest()); err != nil || got.Token != kind.token {
				t.Fatalf("%s, %s: %+v %v, want the token", kind.name, name, got, err)
			}
		}
		for name, environments := range map[string]EnvironmentStore{
			"an unreadable setting": fakeEnvironments{err: errors.New("connection reset")},
			"no setting store":      nil,
		} {
			gh := &fakeSessionGitHub{}
			m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{session: activeSession()}, Environments: environments}
			if _, err := kind.mint(m, context.Background(), validMintRequest()); err == nil || errors.Is(err, ErrForbidden) || kind.minted(gh) != 0 {
				t.Fatalf("%s, %s: err=%v fake=%+v, want an unavailable error and no token", kind.name, name, err, gh)
			}
		}
	}
}

// The refusal crosses the internal hop by name: the gateway's client gets
// ErrGitHubCLIOff back from either gh verb, so it can answer with the
// setting, and a caller that only tests ErrForbidden is unchanged.
func TestGitHubCLIOffRefusalSurvivesTheInternalHop(t *testing.T) {
	secret := []byte("secret")
	gh := &fakeSessionGitHub{}
	minter := &Minter{
		GitHub: gh, Connections: fakeConnections{row: store.GitConnection{InstallationID: 42, AccountLogin: "octo"}}, Sessions: fakeSessions{session: activeSession()},
		Environments: fakeEnvironments{row: store.AgentEnvironment{WorkspaceID: "tea-a", GitHubCLIDisabled: true}},
	}
	for _, handler := range []http.Handler{&GitHubReadHandler{Secret: secret, Minter: minter}, &GitHubWriteHandler{Secret: secret, Minter: minter}} {
		server := httptest.NewServer(handler)
		_, err := (&Client{URL: server.URL, Secret: secret, HTTP: server.Client()}).Mint(context.Background(), validMintRequest())
		server.Close()
		if !errors.Is(err, ErrGitHubCLIOff) || !errors.Is(err, ErrForbidden) {
			t.Fatalf("%T with gh off: err=%v, want ErrGitHubCLIOff", handler, err)
		}
	}
	server := httptest.NewServer(&Handler{Secret: secret, Minter: minter})
	defer server.Close()
	if got, err := (&Client{URL: server.URL, Secret: secret, HTTP: server.Client()}).Mint(context.Background(), validMintRequest()); err != nil || got.Token != "ghs_scoped" {
		t.Fatalf("the Git verb with gh off = %+v %v, want the push token", got, err)
	}
	if gh.apiCalls+gh.apiWriteCalls != 0 || gh.calls != 1 {
		t.Fatalf("mints = %+v, want only the push token", gh)
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

// editorSession is a finished session whose sandbox resume started for a
// person's editor (ADR054 D8): the sandbox validMintRequest's Pod names.
func editorSession(phase string) store.AgentSession {
	return store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", EditorSandboxID: "sbx-1", Phase: phase}
}

// w6/168: once an editor's start settles the session is finished again, and
// its sandbox still pushes the session branch through the Git mint. Nothing
// else widens: the gh route's verbs, a finished turn's own sandbox, a
// superseded editor sandbox, an archived or canceled session, another Pod or
// another branch are refused as before.
func TestMinterAdmitsGitOnlyToAFinishedSessionsEditorSandbox(t *testing.T) {
	minter := func(session store.AgentSession) (*Minter, *fakeSessionGitHub, *auditRecorder) {
		gh, audit := &fakeSessionGitHub{}, &auditRecorder{}
		return &Minter{
			GitHub: gh, Connections: fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}},
			Sessions: fakeSessions{session: session}, Environments: noEnvironment, Audit: audit,
		}, gh, audit
	}
	for _, phase := range []string{"completed", "failed"} {
		m, gh, audit := minter(editorSession(phase))
		got, err := m.Mint(context.Background(), validMintRequest())
		if err != nil || got.Token != "ghs_scoped" || gh.calls != 1 || gh.repository != "repo" {
			t.Fatalf("%s editor sandbox: response=%+v err=%v fake=%+v", phase, got, err, gh)
		}
		if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditAllowed || audit.events[0].Verb != AuditVerbMintCredential {
			t.Fatalf("%s editor sandbox audit = %+v", phase, audit.events)
		}
		for _, kind := range githubAPIMints {
			m, gh, _ := minter(editorSession(phase))
			if _, err := kind.mint(m, context.Background(), validMintRequest()); !errors.Is(err, ErrForbidden) || gh.apiCalls+gh.apiWriteCalls != 0 {
				t.Fatalf("%s editor sandbox, gh %s: err=%v fake=%+v", phase, kind.name, err, gh)
			}
		}
	}

	archived := editorSession("completed")
	archived.ArchivedAt = new(time.Now())
	for name, c := range map[string]struct {
		session store.AgentSession
		req     func(*MintRequest)
	}{
		"a finished turn's own sandbox": {store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "completed"}, nil},
		"a superseded editor sandbox":   {store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", EditorSandboxID: "sbx-0", Phase: "completed"}, nil},
		"the editor's sandbox ended":    {store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", EditorSandboxID: "sbx-1", Phase: "completed"}, nil},
		"archived":                      {archived, nil},
		"canceling":                     {editorSession("canceling"), nil},
		"canceled":                      {editorSession("canceled"), nil},
		"unknown phase":                 {editorSession("future-phase"), nil},
		"foreign workspace":             {func() store.AgentSession { s := editorSession("completed"); s.WorkspaceID = "tea-b"; return s }(), nil},
		"another Pod":                   {editorSession("completed"), func(r *MintRequest) { r.PodName = "sbx-9-0" }},
		"no Pod UID":                    {editorSession("completed"), func(r *MintRequest) { r.PodUID = "" }},
		"another branch":                {editorSession("completed"), func(r *MintRequest) { r.Branch = "main" }},
	} {
		m, gh, audit := minter(c.session)
		req := validMintRequest()
		if c.req != nil {
			c.req(&req)
		}
		if _, err := m.Mint(context.Background(), req); err == nil || gh.calls != 0 {
			t.Fatalf("%s: err=%v fake=%+v", name, err, gh)
		}
		if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied {
			t.Fatalf("%s audit = %+v", name, audit.events)
		}
	}
}

// fakeLogins answers whether a person's SSH login into a session is open,
// and records what it was asked.
type fakeLogins struct {
	open       bool
	err        error
	calls      int
	session    string
	freshSince time.Time
}

func (f *fakeLogins) AgentSessionSSHActivity(_ context.Context, id string, freshSince time.Time) (bool, *time.Time, error) {
	f.calls++
	f.session, f.freshSince = id, freshSince
	return f.open, nil, f.err
}

// w6/175: a finished turn's own sandbox pushes the session branch while a
// person's SSH login into the session is open, as a Devin machine's terminal
// pushes whenever the machine is awake. With no open login, on the gh
// route's mints, for an archived, canceled or foreign session, another Pod
// or another branch, it is refused as before, and a login read that fails
// mints nothing.
func TestMinterAdmitsGitToAFinishedTurnsSandboxWhileAnSSHLoginIsOpen(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	turn := func(phase string) store.AgentSession {
		return store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: phase}
	}
	minter := func(session store.AgentSession, logins *fakeLogins) (*Minter, *fakeSessionGitHub, *auditRecorder) {
		gh, audit := &fakeSessionGitHub{}, &auditRecorder{}
		m := &Minter{
			GitHub: gh, Connections: fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}},
			Sessions: fakeSessions{session: session}, Environments: noEnvironment, Audit: audit, Now: func() time.Time { return now },
		}
		if logins != nil {
			m.Logins = logins
		}
		return m, gh, audit
	}
	for _, phase := range []string{"completed", "failed"} {
		logins := &fakeLogins{open: true}
		m, gh, audit := minter(turn(phase), logins)
		got, err := m.Mint(context.Background(), validMintRequest())
		if err != nil || got.Token != "ghs_scoped" || gh.calls != 1 || gh.repository != "repo" {
			t.Fatalf("%s turn's sandbox with a login open: response=%+v err=%v fake=%+v", phase, got, err, gh)
		}
		if logins.session != "ags-one" || !logins.freshSince.Equal(now.Add(-SSHLoginCap)) {
			t.Fatalf("%s: the login read asked for %q since %s, want ags-one since %s", phase, logins.session, logins.freshSince, now.Add(-SSHLoginCap))
		}
		if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditAllowed || audit.events[0].Verb != AuditVerbMintCredential {
			t.Fatalf("%s audit = %+v", phase, audit.events)
		}
		for _, kind := range githubAPIMints {
			logins := &fakeLogins{open: true}
			m, gh, _ := minter(turn(phase), logins)
			if _, err := kind.mint(m, context.Background(), validMintRequest()); !errors.Is(err, ErrForbidden) || gh.apiCalls+gh.apiWriteCalls != 0 || logins.calls != 0 {
				t.Fatalf("%s turn's sandbox with a login open, gh %s: err=%v fake=%+v logins=%+v", phase, kind.name, err, gh, logins)
			}
		}
	}

	// A turn that runs mints as always, without reading logins.
	logins := &fakeLogins{}
	m, _, _ := minter(turn("running"), logins)
	if _, err := m.Mint(context.Background(), validMintRequest()); err != nil || logins.calls != 0 {
		t.Fatalf("running turn: err=%v login reads=%d", err, logins.calls)
	}

	archived := turn("completed")
	archived.ArchivedAt = new(now)
	for name, c := range map[string]struct {
		session store.AgentSession
		logins  *fakeLogins
		req     func(*MintRequest)
	}{
		"no login open":         {turn("completed"), &fakeLogins{}, nil},
		"logins unwired":        {turn("completed"), nil, nil},
		"archived":              {archived, &fakeLogins{open: true}, nil},
		"canceling":             {turn("canceling"), &fakeLogins{open: true}, nil},
		"canceled":              {turn("canceled"), &fakeLogins{open: true}, nil},
		"unknown phase":         {turn("future-phase"), &fakeLogins{open: true}, nil},
		"the sandbox ended":     {store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", Phase: "completed"}, &fakeLogins{open: true}, nil},
		"foreign workspace":     {func() store.AgentSession { s := turn("completed"); s.WorkspaceID = "tea-b"; return s }(), &fakeLogins{open: true}, nil},
		"another Pod":           {turn("completed"), &fakeLogins{open: true}, func(r *MintRequest) { r.PodName = "sbx-9-0" }},
		"no Pod UID":            {turn("completed"), &fakeLogins{open: true}, func(r *MintRequest) { r.PodUID = "" }},
		"another branch":        {turn("completed"), &fakeLogins{open: true}, func(r *MintRequest) { r.Branch = "main" }},
		"the login read failed": {turn("completed"), &fakeLogins{open: true, err: errors.New("database unavailable")}, nil},
	} {
		m, gh, audit := minter(c.session, c.logins)
		req := validMintRequest()
		if c.req != nil {
			c.req(&req)
		}
		if _, err := m.Mint(context.Background(), req); err == nil || gh.calls != 0 {
			t.Fatalf("%s: err=%v fake=%+v", name, err, gh)
		}
		if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied {
			t.Fatalf("%s audit = %+v", name, audit.events)
		}
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
