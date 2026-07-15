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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fakes ---

type fakeStore struct {
	conns map[string]store.GitConnection
}

func newFakeStore() *fakeStore { return &fakeStore{conns: map[string]store.GitConnection{}} }

func (f *fakeStore) UpsertGitConnection(_ context.Context, c store.GitConnection) (store.GitConnection, error) {
	f.conns[c.WorkspaceID] = c
	return c, nil
}

func (f *fakeStore) GetGitConnection(_ context.Context, workspaceID string) (store.GitConnection, error) {
	c, ok := f.conns[workspaceID]
	if !ok {
		return store.GitConnection{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeStore) DeleteGitConnection(_ context.Context, workspaceID string) error {
	if _, ok := f.conns[workspaceID]; !ok {
		return store.ErrNotFound
	}
	delete(f.conns, workspaceID)
	return nil
}

type fakeClient struct {
	installErr error
	login      string
	repos      []Repo
	reposErr   error
	token      string
	tokenErr   error
	repoOK     bool // RepoAccessible result
	repoErr    error
	commit     Commit // GetCommit result (w9/001)
	commitErr  error
	// gotCommitRef records the (token, owner, repo, ref) GetCommit was called
	// with, for assertions.
	gotCommitRef []string
}

func (c *fakeClient) InstallURL() string { return "https://github.com/apps/bex/installations/new" }

func (c *fakeClient) GetInstallation(_ context.Context, id int64) (Installation, error) {
	if c.installErr != nil {
		return Installation{}, c.installErr
	}
	return Installation{ID: id, AccountLogin: c.login}, nil
}

func (c *fakeClient) ListRepos(_ context.Context, _ int64) ([]Repo, error) {
	return c.repos, c.reposErr
}

func (c *fakeClient) MintInstallationToken(_ context.Context, _ int64) (InstallationToken, error) {
	if c.tokenErr != nil {
		return InstallationToken{}, c.tokenErr
	}
	return InstallationToken{Token: c.token}, nil
}

func (c *fakeClient) RepoAccessible(_ context.Context, _, _, _ string) (bool, error) {
	return c.repoOK, c.repoErr
}

func (c *fakeClient) GetCommit(_ context.Context, token, owner, repo, ref string) (Commit, error) {
	c.gotCommitRef = []string{token, owner, repo, ref}
	if c.commitErr != nil {
		return Commit{}, c.commitErr
	}
	return c.commit, nil
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
		Base:   &core.Base{Namespace: "default"},
		GitHub: &fakeClient{login: "octo", repos: []Repo{{ID: 1, FullName: "octo/app", Private: true}}},
		Store:  newFakeStore(),
	}
	ctx := context.Background()

	conn, err := svc.Connect(ctx, 42)
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

	if err := svc.Disconnect(ctx, ""); err != nil {
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
	if err := svc.Disconnect(ctx, ""); err != nil {
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
			if _, err := svc.Connect(ctx, 1); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("connect err = %v", err)
			}
			if _, err := svc.GetConnection(ctx, ""); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("get err = %v", err)
			}
			if _, err := svc.ListRepos(ctx, ""); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("list err = %v", err)
			}
			if err := svc.Disconnect(ctx, ""); !errors.Is(err, core.ErrGitHubUnavailable) {
				t.Errorf("disconnect err = %v", err)
			}
		})
	}
}

func TestConnectForgedInstallationRejected(t *testing.T) {
	svc := &Service{
		Base:   &core.Base{Namespace: "default"},
		GitHub: &fakeClient{installErr: &APIError{Status: 404, Body: "Not Found"}},
		Store:  newFakeStore(),
	}
	_, err := svc.Connect(context.Background(), 999)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("forged installation err = %v, want ErrBadRequest", err)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Error("forged installation must not persist a connection")
	}
}

func TestConnectRejectsNonPositiveID(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{}, Store: newFakeStore()}
	if _, err := svc.Connect(context.Background(), 0); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("id 0 err = %v, want ErrBadRequest", err)
	}
}

func TestListReposGitHubErrorSurfacesClean(t *testing.T) {
	st := newFakeStore()
	st.conns["default"] = store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"}

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

func TestAuthzGatesWritesButAllowsMemberReads(t *testing.T) {
	// A viewer may read the connection/repos but not connect/disconnect.
	viewer := allowChecker{core.RelCanView: true, core.RelCanManage: false}
	st := newFakeStore()
	st.conns["default"] = store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"}
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
	if _, err := svc.Connect(ctx, 1); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("viewer Connect = %v, want Forbidden", err)
	}
	if err := svc.Disconnect(ctx, ""); !errors.Is(err, core.ErrForbidden) {
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
	st.conns["default"] = store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"}
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

// TestResolveCommit covers w9/001: the deploy path's commit resolution —
// connected repo => the ref's resolved SHA+message; every "nothing to
// resolve" shape (github off, no connection, unparseable repo URL, unknown
// ref/out-of-grant repo per 404/422) => (false, nil), never an error a
// caller might mistake for a deploy-blocking failure.
func TestResolveCommit(t *testing.T) {
	st := newFakeStore()
	st.conns["default"] = store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"}
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
	if _, err := svc.Connect(ctx, 1); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("connect = %v", err)
	}
	if _, err := svc.GetConnection(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("get = %v", err)
	}
	if _, err := svc.ListRepos(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("list = %v", err)
	}
	if err := svc.Disconnect(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("disconnect = %v", err)
	}
}
