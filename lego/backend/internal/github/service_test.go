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

package github

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fakes ---

// fakeStore models git_connections keyed by installation id (ADR078): a
// workspace may hold several connections, one per GitHub account. conns is a flat
// slice so len(conns) still reads as "how many connections exist in total".
type fakeStore struct {
	mu    sync.Mutex
	conns []store.GitConnection
	// txns is the subject-bound connect-transaction table (w1/m67 F3), keyed by
	// nonce. Consumption deletes, mirroring the store's single-statement claim.
	txns map[string]store.GitHubConnectTransaction
}

func (f *fakeStore) BindGitConnection(_ context.Context, c store.GitConnection, maxConnections int) (store.GitConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.conns {
		if f.conns[i].InstallationID != c.InstallationID {
			continue
		}
		if f.conns[i].WorkspaceID != c.WorkspaceID {
			return store.GitConnection{}, store.ErrConflict
		}
		f.conns[i] = c
		return c, nil
	}
	count := 0
	for _, existing := range f.conns {
		if existing.WorkspaceID == c.WorkspaceID {
			count++
		}
	}
	if maxConnections > 0 && count >= maxConnections {
		return store.GitConnection{}, &store.GitConnectionLimitError{Count: count, Limit: maxConnections}
	}
	f.conns = append(f.conns, c)
	return c, nil
}

// firstFor returns a workspace's oldest connection (insertion order), the
// singular-alias semantics; ok=false when it holds none.
func (f *fakeStore) firstFor(workspaceID string) (store.GitConnection, bool) {
	for _, c := range f.conns {
		if c.WorkspaceID == workspaceID {
			return c, true
		}
	}
	return store.GitConnection{}, false
}

// testCallerSubject is the bex identity every test flow starts from; the
// callback must present the same one (w1/m67 F3).
const testCallerSubject = "identity-1"

// testCallerCtx carries that identity, as the auth gate would.
func testCallerCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: testCallerSubject, Method: "session"})
}

// seedConnectTxn records a connect attempt as StartConnect would and returns its
// nonce, so a test can exercise the callback directly (w1/m67 F3).
func seedConnectTxn(t *testing.T, svc *Service, workspaceID, subject string) string {
	t.Helper()
	f, ok := svc.Store.(*fakeStore)
	if !ok {
		t.Fatalf("seedConnectTxn needs the fake store, got %T", svc.Store)
	}
	nonce := "nonce-" + workspaceID + "-" + subject
	f.txns[nonce] = store.GitHubConnectTransaction{
		Nonce: nonce, TenantID: workspaceID, Subject: subject,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	return nonce
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		txns: map[string]store.GitHubConnectTransaction{},
	}
}

func (f *fakeStore) CreateGitHubConnectTransaction(_ context.Context, t store.GitHubConnectTransaction) error {
	f.txns[t.Nonce] = t
	return nil
}

func (f *fakeStore) ConsumeGitHubConnectTransaction(_ context.Context, nonce string) (store.GitHubConnectTransaction, error) {
	t, ok := f.txns[nonce]
	if !ok {
		return store.GitHubConnectTransaction{}, store.ErrNotFound
	}
	delete(f.txns, nonce) // single-use by construction, like the DELETE … RETURNING
	return t, nil
}

func (f *fakeStore) UpsertGitConnection(_ context.Context, c store.GitConnection) (store.GitConnection, error) {
	for i := range f.conns { // keyed by installation id
		if f.conns[i].InstallationID == c.InstallationID {
			f.conns[i] = c
			return c, nil
		}
	}
	f.conns = append(f.conns, c)
	return c, nil
}

func (f *fakeStore) GetGitConnection(_ context.Context, workspaceID string) (store.GitConnection, error) {
	if c, ok := f.firstFor(workspaceID); ok {
		return c, nil
	}
	return store.GitConnection{}, store.ErrNotFound
}

func (f *fakeStore) ListGitConnections(_ context.Context, workspaceID string) ([]store.GitConnection, error) {
	out := []store.GitConnection{}
	for _, c := range f.conns {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) GetGitConnectionByOwner(_ context.Context, workspaceID, accountLogin string) (store.GitConnection, error) {
	for _, c := range f.conns {
		if c.WorkspaceID == workspaceID && strings.EqualFold(c.AccountLogin, accountLogin) {
			return c, nil
		}
	}
	return store.GitConnection{}, store.ErrNotFound
}

func (f *fakeStore) CountGitConnections(_ context.Context, workspaceID string) (int, error) {
	n := 0
	for _, c := range f.conns {
		if c.WorkspaceID == workspaceID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) GitConnectionByInstallation(_ context.Context, installationID int64) (store.GitConnection, error) {
	for _, c := range f.conns {
		if c.InstallationID == installationID {
			return c, nil
		}
	}
	return store.GitConnection{}, store.ErrNotFound
}

func (f *fakeStore) DeleteGitConnection(_ context.Context, workspaceID string, installationID int64) error {
	for i, c := range f.conns {
		if c.WorkspaceID == workspaceID && c.InstallationID == installationID {
			f.conns = append(f.conns[:i], f.conns[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

type fakeClient struct {
	installErr    error
	login         string
	repos         []Repo
	reposErr      error
	branches      []string
	branchesErr   error
	gotBranchRepo []string // (owner, repo) ListBranches was called with
	tree          []RepoTreeEntry
	treeErr       error
	gotTree       []string // (token, owner, repo, path, ref)
	token         string
	tokenErr      error
	repoOK        bool // RepoAccessible result
	repoErr       error
	publicRepoOK  bool
	repoTokens    []string
	commit        Commit // GetCommit result (w9/001)
	commitErr     error
	// gotCommitRef records the (token, owner, repo, ref) GetCommit was called
	// with, for assertions.
	gotCommitRef []string
	// Multi-connection (ADR078) test seams: reposByInst returns per-installation
	// repos (falls back to repos when nil); tokenByInst maps an installation to
	// the token it mints; gotMintInst records the installation MintInstallationToken
	// was last called with.
	reposByInst  map[int64][]Repo
	tokenByInst  map[int64]string
	gotMintInst  int64
	listReposErr map[int64]error // per-installation ListRepos error (ADR078 degrade test)
}

func (c *fakeClient) InstallURL() string { return "https://github.com/apps/bex/installations/new" }

func (c *fakeClient) GetInstallation(_ context.Context, id int64) (Installation, error) {
	if c.installErr != nil {
		return Installation{}, c.installErr
	}
	return Installation{ID: id, AccountLogin: c.login}, nil
}

func (c *fakeClient) ListRepos(_ context.Context, installationID int64) ([]Repo, error) {
	if c.listReposErr != nil {
		if err := c.listReposErr[installationID]; err != nil {
			return nil, err
		}
	}
	if c.reposByInst != nil {
		return c.reposByInst[installationID], c.reposErr
	}
	return c.repos, c.reposErr
}

func (c *fakeClient) ListBranches(_ context.Context, _ int64, owner, repo string) ([]string, error) {
	c.gotBranchRepo = []string{owner, repo}
	return c.branches, c.branchesErr
}

func (c *fakeClient) ListRepoTree(_ context.Context, token, owner, repo, path, ref string) ([]RepoTreeEntry, error) {
	c.gotTree = []string{token, owner, repo, path, ref}
	return c.tree, c.treeErr
}

func (c *fakeClient) MintInstallationToken(_ context.Context, installationID int64) (InstallationToken, error) {
	c.gotMintInst = installationID
	if c.tokenErr != nil {
		return InstallationToken{}, c.tokenErr
	}
	if c.tokenByInst != nil {
		return InstallationToken{Token: c.tokenByInst[installationID]}, nil
	}
	return InstallationToken{Token: c.token}, nil
}

func (c *fakeClient) RepoAccessible(_ context.Context, token, _, _ string) (bool, error) {
	c.repoTokens = append(c.repoTokens, token)
	if token == "" && c.publicRepoOK {
		return true, c.repoErr
	}
	return c.repoOK, c.repoErr
}

func (c *fakeClient) GetCommit(_ context.Context, token, owner, repo, ref string) (Commit, error) {
	c.gotCommitRef = []string{token, owner, repo, ref}
	if c.commitErr != nil {
		return Commit{}, c.commitErr
	}
	return c.commit, nil
}

func (c *fakeClient) GetFileContents(_ context.Context, _, _, _, _, _ string) (FileContents, error) {
	return FileContents{}, nil
}

func (c *fakeClient) GetRepoCommitSHA(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}
func (c *fakeClient) OpenDraftPullRequest(_ context.Context, _ int64, _, _, _, _, _, _ string) (PullRequest, error) {
	return PullRequest{}, nil
}

// allowChecker allows exactly the relations in its set.
type allowChecker map[string]bool

func (a allowChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return a[relation], nil
}

func withIdentity(ctx context.Context) context.Context {
	return core.WithIdentity(ctx, core.Identity{Subject: "alice", Method: "session"})
}

// --- tests ---

func TestConnectRoundTrip(t *testing.T) {
	svc := &Service{
		Base:     &core.Base{Namespace: "default"},
		GitHub:   &fakeClient{login: "octo", repos: []Repo{{ID: 1, FullName: "octo/app", Private: true}}},
		Store:    newFakeStore(),
		Verifier: &fakeVerifier{ok: true},
	}
	ctx := context.Background()

	conn, err := svc.connectFromCallback(ctx, seedConnectTxn(t, svc, core.DefaultTenant, testCallerSubject), testCallerSubject, 42, "oauth-code")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !conn.Connected || conn.AccountLogin != "octo" || conn.InstallationID != 42 {
		t.Fatalf("connect view = %+v", conn)
	}
	if conn.InstallURL == "" {
		t.Error("install url should always be set")
	}

	got, err := svc.GetConnection(ctx, "")
	if err != nil || !got.Connected || got.AccountLogin != "octo" {
		t.Fatalf("get after connect = %+v err=%v", got, err)
	}

	repos, err := svc.ListRepos(ctx, "")
	if err != nil || len(repos) != 1 || !repos[0].Private {
		t.Fatalf("list repos = %+v err=%v", repos, err)
	}

	if err := svc.Disconnect(ctx, "", 0); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	// After disconnect: not connected, repos empty (not an error).
	got, err = svc.GetConnection(ctx, "")
	if err != nil || got.Connected {
		t.Fatalf("get after disconnect = %+v err=%v", got, err)
	}
	repos, err = svc.ListRepos(ctx, "")
	if err != nil || len(repos) != 0 {
		t.Fatalf("repos after disconnect = %+v err=%v", repos, err)
	}

	// Disconnect is idempotent.
	if err := svc.Disconnect(ctx, "", 0); err != nil {
		t.Fatalf("second disconnect not idempotent: %v", err)
	}
}

func TestVerbs503WhenUnconfigured(t *testing.T) {
	ctx := context.Background()
	cases := map[string]*Service{
		"no github": {Base: &core.Base{Namespace: "default"}, Store: newFakeStore()},
		"no store":  {Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{}},
		"neither":   {Base: &core.Base{Namespace: "default"}},
	}
	for name, svc := range cases {
		t.Run(name, func(t *testing.T) {
			// Unconfigured short-circuits before any transaction lookup, so the
			// nonce here is deliberately arbitrary (some cases have no store at all).
			if _, err := svc.connectFromCallback(ctx, "unused-nonce", testCallerSubject, 1, "oauth-code"); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("connect err = %v", err)
			}
			if _, err := svc.GetConnection(ctx, ""); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("get err = %v", err)
			}
			if _, err := svc.ListRepos(ctx, ""); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("list err = %v", err)
			}
			if err := svc.Disconnect(ctx, "", 0); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("disconnect err = %v", err)
			}
		})
	}
}

func TestConnectForgedInstallationRejected(t *testing.T) {
	svc := &Service{
		Base:     &core.Base{Namespace: "default"},
		GitHub:   &fakeClient{installErr: &APIError{Status: 404, Body: "Not Found"}},
		Store:    newFakeStore(),
		Verifier: &fakeVerifier{ok: true},
	}
	_, err := svc.connectFromCallback(context.Background(), seedConnectTxn(t, svc, core.DefaultTenant, testCallerSubject), testCallerSubject, 999, "oauth-code")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("forged installation err = %v, want ErrBadRequest", err)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Error("forged installation must not persist a connection")
	}
}

func TestConnectRejectsNonPositiveID(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{}, Store: newFakeStore(), Verifier: &fakeVerifier{ok: true}}
	if _, err := svc.connectFromCallback(context.Background(), seedConnectTxn(t, svc, core.DefaultTenant, testCallerSubject), testCallerSubject, 0, "oauth-code"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("id 0 err = %v, want ErrBadRequest", err)
	}
}

func TestListReposGitHubErrorSurfacesClean(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})

	// A 5xx from GitHub surfaces as an error (never a silent empty list).
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{reposErr: &APIError{Status: 503, Body: "down"}}, Store: st}
	if _, err := svc.ListRepos(context.Background(), ""); err == nil {
		t.Error("5xx GitHub error should surface")
	}
	// A 4xx maps to a clean bad-request.
	svc.GitHub = &fakeClient{reposErr: &APIError{Status: 403, Body: "forbidden"}}
	if _, err := svc.ListRepos(context.Background(), ""); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("4xx GitHub error = %v, want ErrBadRequest", err)
	}
}

// TestGithubOwnerRepo covers the repo-URL parser feeding ListBranches (w5/m54):
// github.com URLs split into owner/repo (https, .git, and scp forms), and
// anything else (other host, missing repo) degrades to ok=false.
func TestGithubOwnerRepo(t *testing.T) {
	for _, c := range []struct {
		url, owner, repo string
		ok               bool
	}{
		{"https://github.com/acme/app", "acme", "app", true},
		{"https://github.com/acme/app.git", "acme", "app", true},
		{"git@github.com:acme/app.git", "acme", "app", true},
		{"https://gitlab.com/acme/app", "", "", false},
		{"https://github.com/acme", "", "", false},
		{"", "", "", false},
		// Origin-spoofing forms that must NOT canonicalize to github.com:
		{"https://evil.example/@github.com/acme/app", "", "", false},     // @ in the PATH, not userinfo
		{"https://evil.example/@github.com/acme/app.git", "", "", false}, // .git form of the same
		{"https://github.com@evil.example/acme/app", "", "", false},      // real userinfo, host=evil.example
		{"https://user@github.com/acme/app", "", "", false},              // userinfo rejected outright
		{"https://github.com.evil.example/acme/app", "", "", false},      // github.com as a subdomain label
		{"https://github.com:8443/acme/app", "", "", false},              // non-default port
		{"ssh://git@github.com/acme/app", "", "", false},                 // ssh never mints an http token
	} {
		owner, repo, ok := githubOwnerRepo(c.url)
		if ok != c.ok || owner != c.owner || repo != c.repo {
			t.Errorf("githubOwnerRepo(%q) = %q,%q,%v; want %q,%q,%v", c.url, owner, repo, ok, c.owner, c.repo, c.ok)
		}
	}
}

// TestListBranches: a connected GitHub repo returns its real branches (parsed to
// owner/repo); a non-GitHub repo or a missing connection degrades to an empty
// list without touching the client — the dashboard then uses free-text (w5/m54).
func TestListBranches(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})
	fc := &fakeClient{branches: []string{"main", "dev", "release/1.0"}}
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: fc, Store: st}
	ctx := context.Background()

	got, err := svc.ListBranches(ctx, "", "https://github.com/octo/app")
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(got) != 3 || got[0] != "main" {
		t.Fatalf("branches = %v", got)
	}
	if len(fc.gotBranchRepo) != 2 || fc.gotBranchRepo[0] != "octo" || fc.gotBranchRepo[1] != "app" {
		t.Errorf("ListBranches called with %v, want [octo app]", fc.gotBranchRepo)
	}

	// A non-GitHub repo degrades to empty without calling the client.
	fc.gotBranchRepo = nil
	if got, err := svc.ListBranches(ctx, "", "https://gitlab.com/octo/app"); err != nil || len(got) != 0 {
		t.Fatalf("non-github = %v err=%v; want empty", got, err)
	}
	if fc.gotBranchRepo != nil {
		t.Error("a non-GitHub repo must not call the GitHub client")
	}

	// No connection => empty, not an error.
	svc.Store = newFakeStore()
	if got, err := svc.ListBranches(ctx, "", "https://github.com/octo/app"); err != nil || len(got) != 0 {
		t.Fatalf("no connection = %v err=%v; want empty", got, err)
	}
}

func TestAuthzGatesWritesButAllowsMemberReads(t *testing.T) {
	// A viewer may read the connection/repos but not connect/disconnect.
	viewer := allowChecker{core.RelCanView: true, core.RelCanManage: false}
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Authz: viewer},
		GitHub: &fakeClient{login: "octo", repos: []Repo{{ID: 1, FullName: "octo/app"}}},
		Store:  st,
	}
	ctx := withIdentity(context.Background())

	if _, err := svc.GetConnection(ctx, ""); err != nil {
		t.Errorf("viewer GetConnection = %v, want ok", err)
	}
	if _, err := svc.ListRepos(ctx, ""); err != nil {
		t.Errorf("viewer ListRepos = %v, want ok", err)
	}
	if _, err := svc.StartConnect(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("viewer StartConnect = %v, want Forbidden", err)
	}
	if err := svc.Disconnect(ctx, "", 0); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("viewer Disconnect = %v, want Forbidden", err)
	}
}

func TestOwnerRepo(t *testing.T) {
	for _, url := range []string{
		"https://github.com/octo/app",
		"https://github.com/octo/app.git",
		"git@github.com:octo/app.git",
		"ssh://git@github.com/octo/app",
	} {
		owner, repo, ok := ownerRepo(url)
		if !ok || owner != "octo" || repo != "app" {
			t.Errorf("ownerRepo(%q) = %q,%q,%v", url, owner, repo, ok)
		}
	}
	for _, bad := range []string{"", "not-a-url", "github.com"} {
		if _, _, ok := ownerRepo(bad); ok {
			t.Errorf("ownerRepo(%q) should be !ok", bad)
		}
	}
}

func TestCloneToken(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})
	ctx := context.Background()

	// Repo in the grant (RepoAccessible ok) => a fresh token, across URL forms.
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "ghs_fresh", repoOK: true}, Store: st}
	for _, url := range []string{"https://github.com/octo/app", "https://github.com/octo/app.git", "git@github.com:octo/app.git"} {
		tok, ok, err := svc.cloneToken(ctx, "default", url)
		if err != nil || !ok || tok != "ghs_fresh" {
			t.Errorf("cloneToken(%q) = %q,%v,%v", url, tok, ok, err)
		}
	}

	// Repo NOT in the grant (RepoAccessible false) => no token, no error.
	notGranted := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", repoOK: false}, Store: st}
	if tok, ok, err := notGranted.cloneToken(ctx, "default", "https://github.com/octo/app"); ok || tok != "" || err != nil {
		t.Errorf("unconnected repo = %q,%v,%v", tok, ok, err)
	}

	// A URL with no owner/repo => no token, no error (public-clone path).
	if _, ok, err := svc.cloneToken(ctx, "default", "not-a-url"); ok || err != nil {
		t.Errorf("unparseable repo = ok=%v err=%v", ok, err)
	}

	// GitHub off => no token, no error.
	off := &Service{Base: &core.Base{Namespace: "default"}}
	if _, ok, err := off.cloneToken(ctx, "default", "https://github.com/octo/app"); ok || err != nil {
		t.Errorf("github off should be (false,nil), got ok=%v err=%v", ok, err)
	}

	// No connection => no token, no error.
	noConn := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", repoOK: true}, Store: newFakeStore()}
	if _, ok, err := noConn.cloneToken(ctx, "default", "https://github.com/octo/app"); ok || err != nil {
		t.Errorf("no connection should be (false,nil), got ok=%v err=%v", ok, err)
	}

	// Mint failure on a connected repo => error (never a silent public clone).
	failMint := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{tokenErr: &APIError{Status: 500}, repoOK: true}, Store: st}
	if _, _, err := failMint.cloneToken(ctx, "default", "https://github.com/octo/app"); err == nil {
		t.Error("mint failure on a connected repo must surface an error")
	}

	// Grant-check failure (5xx) => error, not a silent public clone.
	failCheck := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", repoErr: &APIError{Status: 500}}, Store: st}
	if _, _, err := failCheck.cloneToken(ctx, "default", "https://github.com/octo/app"); err == nil {
		t.Error("repo-access-check failure must surface an error")
	}
}

// TestCloneTokenRequiresGitHubOrigin pins w1/m65 F1: a repo URL on a NON-github
// host whose path suffix matches a granted owner/repo must never mint a token
// (which the operator's build credential helper would then send to that
// attacker host). The token is bound to a verified github.com origin.
func TestCloneTokenRequiresGitHubOrigin(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})
	// repoOK:true => if the host were honored as github.com, octo/app IS in the
	// grant, so a token WOULD be minted. The host binding is the only thing
	// stopping it.
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "ghs_leak", repoOK: true}, Store: st}
	ctx := context.Background()

	for _, url := range []string{
		"https://evil.example/octo/app",             // attacker host, granted path suffix
		"https://evil.example/octo/app.git",         // .git form
		"https://evil.example/@github.com/octo/app", // @github.com in the PATH — the F1-bypass this test now pins
		"https://evil.example/@github.com/octo/app.git",
		"https://github.com.evil.example/octo/app", // github.com as a subdomain prefix
		"https://github.com@evil.example/octo/app", // userinfo trick
		"git@evil.example:octo/app.git",            // scp form, attacker host
		"https://github.com:8443/octo/app",         // non-default port
	} {
		tok, ok, err := svc.cloneToken(ctx, "default", url)
		if ok || tok != "" || err != nil {
			t.Errorf("cloneToken(%q) = %q,%v,%v; want no token minted for a non-github.com origin", url, tok, ok, err)
		}
	}

	// Control: the legitimate github.com URL still mints (the fix doesn't break
	// the real path).
	if tok, ok, err := svc.cloneToken(ctx, "default", "https://github.com/octo/app"); !ok || tok != "ghs_leak" || err != nil {
		t.Errorf("github.com clone token = %q,%v,%v; want the minted token", tok, ok, err)
	}
}

// TestConnectRejectsForeignInstallation pins w1/m65 F2: an installation already
// bound to one workspace cannot be claimed by a DIFFERENT workspace (the App JWT
// can look up every installation, so GetInstallation success is not ownership
// proof). The second claim is refused with ErrConflict and does not mutate the
// second workspace's connection.
func TestConnectRejectsForeignInstallation(t *testing.T) {
	svc := &Service{
		Base:   &core.Base{Namespace: "default"},
		GitHub: &fakeClient{login: "octo"},
		Store:  newFakeStore(),
	}
	ctx := context.Background()

	// Workspace A connects installation 42.
	if _, err := svc.connectWithWorkspace(ctx, "tea-a", 42); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	// Workspace B tries to claim the same installation — refused.
	if _, err := svc.connectWithWorkspace(ctx, "tea-b", 42); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("foreign claim err = %v, want ErrConflict", err)
	}
	fs := svc.Store.(*fakeStore)
	if _, ok := fs.firstFor("tea-b"); ok {
		t.Error("foreign installation claim must not persist a connection for workspace B")
	}
	if a, _ := fs.firstFor("tea-a"); a.InstallationID != 42 {
		t.Errorf("workspace A connection disturbed: %+v", a)
	}
	// Re-connecting the SAME workspace to the SAME installation stays idempotent.
	if _, err := svc.connectWithWorkspace(ctx, "tea-a", 42); err != nil {
		t.Fatalf("idempotent re-connect: %v", err)
	}
}

func TestConcurrentConnectsCannotExceedWorkspaceQuota(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "tea-a", InstallationID: 1, AccountLogin: "first"})
	svc := &Service{
		Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "next"},
		Store: st, MaxConnections: 2,
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, installationID := range []int64{2, 3} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.connectWithWorkspace(context.Background(), "tea-a", installationID)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes, limits := 0, 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		var coded *core.CodedError
		if errors.As(err, &coded) && coded.Code == "GIT_CONNECTION_LIMIT" {
			limits++
			continue
		}
		t.Fatalf("unexpected connect result: %v", err)
	}
	if successes != 1 || limits != 1 {
		t.Fatalf("concurrent results: successes=%d limits=%d", successes, limits)
	}
	if count, _ := st.CountGitConnections(context.Background(), "tea-a"); count != 2 {
		t.Fatalf("workspace connection count = %d, want hard limit 2", count)
	}
}

func TestConcurrentReconnectRemainsQuotaExempt(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "tea-a", InstallationID: 1, AccountLogin: "first"})
	svc := &Service{
		Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "refreshed"},
		Store: st, MaxConnections: 1,
	}
	start := make(chan struct{})
	results := make(chan struct {
		id  int64
		err error
	}, 2)
	for _, installationID := range []int64{1, 2} {
		go func() {
			<-start
			_, err := svc.connectWithWorkspace(context.Background(), "tea-a", installationID)
			results <- struct {
				id  int64
				err error
			}{installationID, err}
		}()
	}
	close(start)
	for range 2 {
		result := <-results
		if result.id == 1 && result.err != nil {
			t.Fatalf("same-workspace reconnect was not quota-exempt: %v", result.err)
		}
		if result.id == 2 {
			var coded *core.CodedError
			if !errors.As(result.err, &coded) || coded.Code != "GIT_CONNECTION_LIMIT" {
				t.Fatalf("new binding result = %v, want GIT_CONNECTION_LIMIT", result.err)
			}
		}
	}
}

// fakeVerifier drives the installation-admin proof in tests.
type fakeVerifier struct {
	ok      bool
	err     error
	gotCode string
	gotID   int64
	// Claim seams (ADR078 §3a): the admin-filtered installations the fake
	// resolves from a code, and the code ClaimableInstallations saw.
	claimable    []Installation
	claimErr     error
	gotClaimCode string
}

func (f *fakeVerifier) VerifyInstallationAdmin(_ context.Context, code string, id int64) (bool, error) {
	f.gotCode, f.gotID = code, id
	return f.ok, f.err
}

func (f *fakeVerifier) AuthorizeURL() string {
	return "https://github.example/login/oauth/authorize?client_id=test-client"
}

func (f *fakeVerifier) ClaimableInstallations(_ context.Context, code string) ([]Installation, error) {
	f.gotClaimCode = code
	return f.claimable, f.claimErr
}

// TestConnectFromCallbackEnforcesInstallationAdmin pins w1/m65 F2's principal
// proof: when the OAuth verifier is wired, the browser callback records a
// connection only if the user proves they administer the installation. A failed
// proof is refused (ErrForbidden) and persists nothing.
func TestConnectFromCallbackEnforcesInstallationAdmin(t *testing.T) {
	ctx := context.Background()

	// Verifier rejects (the user does NOT administer installation 42).
	reject := &Service{
		Base:     &core.Base{Namespace: "default"},
		GitHub:   &fakeClient{login: "octo"},
		Store:    newFakeStore(),
		Verifier: &fakeVerifier{ok: false},
	}
	if _, err := reject.connectFromCallback(ctx, seedConnectTxn(t, reject, "tea-a", testCallerSubject), testCallerSubject, 42, "usercode"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unproven installation err = %v, want ErrForbidden", err)
	}
	if len(reject.Store.(*fakeStore).conns) != 0 {
		t.Error("a failed installation-admin proof must not persist a connection")
	}

	// Verifier accepts (the user administers installation 42) — records it and
	// forwards the exact code + installation id to the verifier.
	fv := &fakeVerifier{ok: true}
	accept := &Service{
		Base:     &core.Base{Namespace: "default"},
		GitHub:   &fakeClient{login: "octo"},
		Store:    newFakeStore(),
		Verifier: fv,
	}
	if _, err := accept.connectFromCallback(ctx, seedConnectTxn(t, accept, "tea-a", testCallerSubject), testCallerSubject, 42, "usercode"); err != nil {
		t.Fatalf("proven installation connect: %v", err)
	}
	if a, _ := accept.Store.(*fakeStore).firstFor("tea-a"); a.InstallationID != 42 {
		t.Error("proven installation must persist the connection")
	}
	if fv.gotCode != "usercode" || fv.gotID != 42 {
		t.Errorf("verifier saw code=%q id=%d, want usercode/42", fv.gotCode, fv.gotID)
	}
	if _, err := accept.connectFromCallback(ctx, seedConnectTxn(t, accept, "tea-a", testCallerSubject), testCallerSubject, 42, ""); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("missing OAuth code err = %v, want ErrBadRequest", err)
	}

	// No verifier wired => binding fails closed even if GitHub can look up the
	// installation. App-level visibility is not proof of the user's authority.
	noVerify := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "octo"}, Store: newFakeStore()}
	if _, err := noVerify.connectFromCallback(ctx, seedConnectTxn(t, noVerify, "tea-a", testCallerSubject), testCallerSubject, 42, "usercode"); !errors.Is(err, core.ErrGitHubUnavailable) {
		t.Fatalf("verifier-unwired callback err = %v, want ErrGitHubUnavailable", err)
	}
	if len(noVerify.Store.(*fakeStore).conns) != 0 {
		t.Error("verifier-unwired callback must not persist a connection")
	}
}

// TestResolveCommit covers w9/001: the deploy path's commit resolution —
// connected repo => the ref's resolved SHA+message; every "nothing to
// resolve" shape (github off, no connection, unparseable repo URL, unknown
// ref/out-of-grant repo per 404/422) => (false, nil), never an error a
// caller might mistake for a deploy-blocking failure.
func TestResolveCommit(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})
	ctx := context.Background()

	cl := &fakeClient{token: "ghs_fresh", commit: Commit{SHA: "abc1234def", Message: "fix: header"}}
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: cl, Store: st}
	c, ok, err := svc.DeployCommitSource().ResolveCommit(ctx, "default", "https://github.com/octo/app", "main")
	if err != nil || !ok || c.Hash != "abc1234def" || c.Message != "fix: header" {
		t.Fatalf("resolveCommit = %+v,%v,%v, want the resolved commit", c, ok, err)
	}
	// The lookup authenticates with the freshly minted installation token.
	if len(cl.gotCommitRef) != 4 || cl.gotCommitRef[0] != "ghs_fresh" || cl.gotCommitRef[3] != "main" {
		t.Errorf("GetCommit called with %v, want the minted token + the ref", cl.gotCommitRef)
	}

	// Unknown ref / repo out of the grant (GitHub 422/404) => (false, nil).
	for _, status := range []int{404, 422} {
		bad := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", commitErr: &APIError{Status: status}}, Store: st}
		if _, ok, err := bad.resolveCommit(ctx, "default", "https://github.com/octo/app", "gone"); ok || err != nil {
			t.Errorf("status %d: want (false, nil), got ok=%v err=%v", status, ok, err)
		}
	}

	// A real GitHub failure surfaces as an error (the caller decides it's
	// best-effort, not this seam).
	down := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", commitErr: &APIError{Status: 500}}, Store: st}
	if _, _, err := down.resolveCommit(ctx, "default", "https://github.com/octo/app", "main"); err == nil {
		t.Error("a 500 from GitHub must surface an error")
	}

	// Nothing to resolve: github off, no connection, unparseable URL, empty ref.
	off := &Service{Base: &core.Base{Namespace: "default"}}
	if _, ok, err := off.resolveCommit(ctx, "default", "https://github.com/octo/app", "main"); ok || err != nil {
		t.Errorf("github off = ok=%v err=%v, want (false, nil)", ok, err)
	}
	noConn := &Service{Base: &core.Base{Namespace: "default"}, GitHub: cl, Store: newFakeStore()}
	if _, ok, err := noConn.resolveCommit(ctx, "default", "https://github.com/octo/app", "main"); ok || err != nil {
		t.Errorf("no connection = ok=%v err=%v, want (false, nil)", ok, err)
	}
	if _, ok, err := svc.resolveCommit(ctx, "default", "not-a-url", "main"); ok || err != nil {
		t.Errorf("unparseable repo = ok=%v err=%v, want (false, nil)", ok, err)
	}
	if _, ok, err := svc.resolveCommit(ctx, "default", "https://github.com/octo/app", ""); ok || err != nil {
		t.Errorf("empty ref = ok=%v err=%v, want (false, nil)", ok, err)
	}
}

func TestAuthzDenyAllForbidsEveryVerb(t *testing.T) {
	deny := allowChecker{}
	svc := &Service{Base: &core.Base{Namespace: "default", Authz: deny}, GitHub: &fakeClient{}, Store: newFakeStore()}
	ctx := withIdentity(context.Background())
	if _, err := svc.StartConnect(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("start connect = %v", err)
	}
	if _, err := svc.GetConnection(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("get = %v", err)
	}
	if _, err := svc.ListRepos(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("list = %v", err)
	}
	if err := svc.Disconnect(ctx, "", 0); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("disconnect = %v", err)
	}
}
