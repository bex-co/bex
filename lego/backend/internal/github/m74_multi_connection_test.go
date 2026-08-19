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

// multiConnSvc builds a service whose default workspace already holds two
// connections — an org (octo, installation 7) and a personal account (personal,
// installation 9) — the ADR075 N-per-workspace shape.
func multiConnSvc(t *testing.T, fc *fakeClient) *Service {
	t.Helper()
	st := newFakeStore()
	st.conns = append(st.conns,
		store.GitConnection{WorkspaceID: core.DefaultTenant, InstallationID: 7, AccountLogin: "octo"},
		store.GitConnection{WorkspaceID: core.DefaultTenant, InstallationID: 9, AccountLogin: "personal"},
	)
	return &Service{Base: &core.Base{Namespace: "default"}, GitHub: fc, Store: st}
}

// TestListReposAggregatesAcrossConnections: ListRepos unions every connection's
// repos and stamps each with the account + installation it came from (ADR075 §4).
func TestListReposAggregatesAcrossConnections(t *testing.T) {
	fc := &fakeClient{reposByInst: map[int64][]Repo{
		7: {{ID: 1, FullName: "octo/app"}},
		9: {{ID: 2, FullName: "personal/site"}, {ID: 3, FullName: "personal/blog"}},
	}}
	svc := multiConnSvc(t, fc)

	repos, err := svc.ListRepos(context.Background(), "")
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("got %d repos, want 3: %+v", len(repos), repos)
	}
	byAccount := map[string]int64{}
	for _, r := range repos {
		if r.AccountLogin == "" || r.InstallationID == 0 {
			t.Errorf("repo %q not annotated with account/installation: %+v", r.FullName, r)
		}
		byAccount[r.FullName] = r.InstallationID
	}
	if byAccount["octo/app"] != 7 || byAccount["personal/site"] != 9 {
		t.Errorf("repos annotated with wrong installation: %+v", byAccount)
	}
}

// TestListReposDegradesOneFailedConnection: a dead installation must not blank
// out the other account's repos — ListRepos logs it and serves the rest (ADR075).
func TestListReposDegradesOneFailedConnection(t *testing.T) {
	fc := &fakeClient{
		reposByInst:  map[int64][]Repo{9: {{ID: 2, FullName: "personal/site"}}},
		listReposErr: map[int64]error{7: &APIError{Status: 500, Body: "down"}},
	}
	svc := multiConnSvc(t, fc)

	repos, err := svc.ListRepos(context.Background(), "")
	if err != nil {
		t.Fatalf("one failed connection must not fail the whole list: %v", err)
	}
	if len(repos) != 1 || repos[0].FullName != "personal/site" {
		t.Fatalf("want only the surviving account's repo, got %+v", repos)
	}
}

// TestListReposSurfacesWhenEveryConnectionFails keeps a workspace whose ONLY
// connection errors byte-identical to the pre-ADR075 surface (an error, not a
// misleading empty list).
func TestListReposSurfacesWhenEveryConnectionFails(t *testing.T) {
	fc := &fakeClient{listReposErr: map[int64]error{
		7: &APIError{Status: 503}, 9: &APIError{Status: 503},
	}}
	svc := multiConnSvc(t, fc)
	if _, err := svc.ListRepos(context.Background(), ""); err == nil {
		t.Error("every connection failing must surface an error, not an empty list")
	}
}

// TestCloneTokenPicksOwnersConnection: a repo owned by account B mints from B's
// installation, never A's (ADR075 §4) — the account-A-token-for-account-B-repo leak.
func TestCloneTokenPicksOwnersConnection(t *testing.T) {
	fc := &fakeClient{
		repoOK:      true,
		tokenByInst: map[int64]string{7: "tok-octo", 9: "tok-personal"},
	}
	svc := multiConnSvc(t, fc)

	tok, ok, err := svc.cloneToken(context.Background(), core.DefaultTenant, "https://github.com/personal/site")
	if err != nil || !ok {
		t.Fatalf("cloneToken(personal) ok=%v err=%v", ok, err)
	}
	if tok != "tok-personal" || fc.gotMintInst != 9 {
		t.Errorf("minted from installation %d (token %q), want 9/tok-personal", fc.gotMintInst, tok)
	}

	tok, ok, err = svc.cloneToken(context.Background(), core.DefaultTenant, "https://github.com/octo/app")
	if err != nil || !ok {
		t.Fatalf("cloneToken(octo) ok=%v err=%v", ok, err)
	}
	if tok != "tok-octo" || fc.gotMintInst != 7 {
		t.Errorf("minted from installation %d (token %q), want 7/tok-octo", fc.gotMintInst, tok)
	}
}

// TestCloneTokenNoConnectionForOwner: a repo whose owner is not one of the
// workspace's connected accounts mints nothing (public-clone fallback, ADR075 §4).
func TestCloneTokenNoConnectionForOwner(t *testing.T) {
	svc := multiConnSvc(t, &fakeClient{repoOK: true, token: "tok"})
	_, ok, err := svc.cloneToken(context.Background(), core.DefaultTenant, "https://github.com/stranger/repo")
	if err != nil || ok {
		t.Fatalf("unconnected owner should mint nothing: ok=%v err=%v", ok, err)
	}
}

// TestConnectionQuotaRefusesBeyondCap: a NEW connection past MaxConnections is
// refused with the coded GIT_CONNECTION_LIMIT; an idempotent re-connect is exempt.
func TestConnectionQuotaRefusesBeyondCap(t *testing.T) {
	svc := multiConnSvc(t, &fakeClient{login: "third"})
	svc.MaxConnections = 2 // the workspace already holds 2

	// A third, different installation is over the cap.
	_, err := svc.connectWithWorkspace(context.Background(), core.DefaultTenant, 11)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "GIT_CONNECTION_LIMIT" {
		t.Fatalf("over-cap connect err = %v, want GIT_CONNECTION_LIMIT", err)
	}

	// Re-connecting an EXISTING installation (7) is idempotent and quota-exempt.
	svc.GitHub = &fakeClient{login: "octo"}
	if _, err := svc.connectWithWorkspace(context.Background(), core.DefaultTenant, 7); err != nil {
		t.Fatalf("idempotent re-connect must be quota-exempt: %v", err)
	}
}

// TestConnectAddsSecondInstallation: a second, different installation is ADDED to
// the workspace's set rather than replacing the first (the core ADR075 change).
func TestConnectAddsSecondInstallation(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: core.DefaultTenant, InstallationID: 7, AccountLogin: "octo"})
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "personal"}, Store: st}

	if _, err := svc.connectWithWorkspace(context.Background(), core.DefaultTenant, 9); err != nil {
		t.Fatalf("add second connection: %v", err)
	}
	got, _ := st.ListGitConnections(context.Background(), core.DefaultTenant)
	if len(got) != 2 {
		t.Fatalf("want 2 connections after adding a second, got %d: %+v", len(got), got)
	}
}

// TestDisconnectByInstallationRemovesExactlyOne: per-installation disconnect
// removes only its row; the singular alias (id 0) refuses when several exist.
func TestDisconnectByInstallation(t *testing.T) {
	fc := &fakeClient{}
	svc := multiConnSvc(t, fc)
	svc.Base.Authz = allowChecker{core.RelCanManage: true}
	ctx := testCallerCtx()

	// Ambiguous singular disconnect (id 0, two connections) is a conflict.
	if err := svc.Disconnect(ctx, "", 0); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("ambiguous singular disconnect err = %v, want ErrConflict", err)
	}

	// Targeted disconnect removes exactly installation 7.
	if err := svc.Disconnect(ctx, "", 7); err != nil {
		t.Fatalf("targeted disconnect: %v", err)
	}
	got, _ := svc.Store.ListGitConnections(ctx, core.DefaultTenant)
	if len(got) != 1 || got[0].InstallationID != 9 {
		t.Fatalf("after disconnecting 7, want only 9 left, got %+v", got)
	}

	// Now that one remains, the singular alias disconnects it.
	if err := svc.Disconnect(ctx, "", 0); err != nil {
		t.Fatalf("singular disconnect of sole connection: %v", err)
	}
	if got, _ := svc.Store.ListGitConnections(ctx, core.DefaultTenant); len(got) != 0 {
		t.Fatalf("want no connections left, got %+v", got)
	}
}

// TestListConnectionsReturnsAll: the plural surface returns every connection,
// each a connected view.
func TestListConnectionsReturnsAll(t *testing.T) {
	svc := multiConnSvc(t, &fakeClient{})
	svc.Base.Authz = allowChecker{core.RelCanView: true}

	conns, err := svc.ListConnections(testCallerCtx(), "")
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(conns) != 2 {
		t.Fatalf("want 2 connections, got %d", len(conns))
	}
	for _, c := range conns {
		if !c.Connected || c.AccountLogin == "" {
			t.Errorf("connection view not fully populated: %+v", c)
		}
	}
}

// TestGetConnectionNotConnectedHasNoInstallURL: the singular not-connected view
// no longer advertises the bare (stateless) install URL (ADR075 §3).
func TestGetConnectionNotConnectedHasNoInstallURL(t *testing.T) {
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Authz: allowChecker{core.RelCanView: true}},
		GitHub: &fakeClient{},
		Store:  newFakeStore(),
	}
	conn, err := svc.GetConnection(testCallerCtx(), "")
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if conn.Connected {
		t.Fatal("empty workspace should be not-connected")
	}
	if conn.InstallURL != "" {
		t.Errorf("not-connected view must not carry a bare install URL, got %q", conn.InstallURL)
	}
}
