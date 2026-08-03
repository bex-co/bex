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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/usage"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// readside_ownerid_e2e_test.go is w6/m18's end-to-end proof (t005) — the
// read/bind-side mirror of w6/m14's TestMultiWorkspaceTargetingE2E (t007).
// m14 closed the gap for WRITES (an explicit ownerId lands a create in the
// named, membership-checked workspace); the three surfaces here — usage,
// api-keys, and GitHub connections — were the noted residual: they already
// honored core.WithWorkspace's override once set (they read through
// core.Base.Tenant), but no REST/GraphQL/MCP adapter ever SET it, so a
// multi-workspace caller could only ever see their default workspace's
// answer. This proves a caller targeting workspace B explicitly gets B's
// data, and targeting nothing gets A's (their default) — against REAL
// infrastructure (a live Postgres + a live OpenFGA), same harness and skip
// condition as multiworkspace_e2e_test.go:
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:5433/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api/ -run TestReadSideOwnerIDTargetingE2E -v
func TestReadSideOwnerIDTargetingE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		t.Skip("BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	// Additive, never truncates — see multiworkspace_e2e_test.go's comment on
	// why every name here is unique-per-run.
	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	dana, erin := "dana-"+run, "erin-"+run
	ws := func(kind string) string { return kind + "-" + run }

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(workspaces.WorkspaceGranter)
	roles := checker.(store.MembershipGranter)

	// dana's two workspaces, in the order she got them: alpha first (her
	// default), bravo second.
	wsA, err := st.CreateWorkspace(ctx, ws("alpha"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	wsB, err := st.CreateWorkspace(ctx, ws("bravo"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create bravo: %v", err)
	}
	for _, w := range []store.Tenant{wsA, wsB} {
		if err := granter.GrantWorkspaceAdmin(ctx, w.ID, "user:"+dana); err != nil {
			t.Fatalf("grant admin on %s: %v", w.Name, err)
		}
	}

	// --- seed distinct per-workspace data for the three read surfaces ---

	// Usage: one instance_seconds row per workspace, same hour, different
	// quantities — the two summaries must not just BOTH be present, they must
	// be A's and B's respectively.
	hour := time.Now().UTC().Truncate(time.Hour)
	if err := st.UpsertUsageHourly(ctx, store.HourlyRow{
		WorkspaceID: wsA.ID, ServiceID: "svc-alpha", ResourceKind: store.ResourceKindService,
		Kind: store.UsageKindInstanceSeconds, Tier: "starter", WindowStart: hour, Quantity: 3600,
	}); err != nil {
		t.Fatalf("seed alpha usage: %v", err)
	}
	if err := st.UpsertUsageHourly(ctx, store.HourlyRow{
		WorkspaceID: wsB.ID, ServiceID: "svc-bravo", ResourceKind: store.ResourceKindService,
		Kind: store.UsageKindInstanceSeconds, Tier: "pro", WindowStart: hour, Quantity: 7200,
	}); err != nil {
		t.Fatalf("seed bravo usage: %v", err)
	}

	// GitHub: one connection per workspace, distinct account logins.
	if _, err := st.UpsertGitConnection(ctx, store.GitConnection{
		WorkspaceID: wsA.ID, InstallationID: 111, AccountLogin: "alpha-org",
	}); err != nil {
		t.Fatalf("seed alpha git connection: %v", err)
	}
	if _, err := st.UpsertGitConnection(ctx, store.GitConnection{
		WorkspaceID: wsB.ID, InstallationID: 222, AccountLogin: "bravo-org",
	}); err != nil {
		t.Fatalf("seed bravo git connection: %v", err)
	}

	// The REAL resolver, shared by Base.Workspace (auth) and apikeys' Binding
	// (bind/list/revoke scoping) — same object, so a request's workspace
	// resolution and its key-tenant resolution can never drift.
	tenantSvc := NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz:     checker,
		Workspace: tenantSvc,
	}
	usageSvc := usage.NewService(base, st, "", nil) // PromBase empty: read-only, no metering loop
	keyStore := newFakeKeyStore()
	fakeGH := &fakeGitHubAPIClient{}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, WorkspaceGranter: granter,
		Usage:        usageSvc,
		APIKeys:      keyStore,
		KeyBinder:    tenantSvc,
		GitHubClient: fakeGH,
		GitHubStore:  st,
	})
	mux := srv.restHandler()
	call := func(subject, method, path, body string) *http.Response {
		return e2eCall(mux, ctx, subject, method, path, body).Result()
	}

	// --- (1) usage: explicit ownerId=bravo answers bravo, no ownerId answers
	// dana's default (alpha) — the exact shape the m11 regression fixed for
	// writes, now proven for this read. ---

	gotUsage := decodeJSON[workspaceIDResponse](t, call(dana, "GET", "/v1/usage", ""))
	if gotUsage.WorkspaceID != wsA.ID {
		t.Errorf("GET /v1/usage (no ownerId) workspaceId = %q, want alpha %q", gotUsage.WorkspaceID, wsA.ID)
	}

	gotUsage = decodeJSON[workspaceIDResponse](t, call(dana, "GET", "/v1/usage?ownerId="+wsB.ID, ""))
	if gotUsage.WorkspaceID != wsB.ID {
		t.Errorf("GET /v1/usage?ownerId=bravo workspaceId = %q, want bravo %q", gotUsage.WorkspaceID, wsB.ID)
	}

	// erin, a member of neither workspace, cannot target bravo's usage.
	if resp := call(erin, "GET", "/v1/usage?ownerId="+wsB.ID, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("erin GET /v1/usage?ownerId=bravo: %d, want 403", resp.StatusCode)
	}

	// --- (2) GitHub connections: same shape. ---

	gotConn := decodeJSON[accountLoginResponse](t, call(dana, "GET", "/v1/git/connection", ""))
	if gotConn.AccountLogin != "alpha-org" {
		t.Errorf("GET /v1/git/connection (no ownerId) accountLogin = %q, want alpha-org", gotConn.AccountLogin)
	}

	gotConn = decodeJSON[accountLoginResponse](t, call(dana, "GET", "/v1/git/connection?ownerId="+wsB.ID, ""))
	if gotConn.AccountLogin != "bravo-org" {
		t.Errorf("GET /v1/git/connection?ownerId=bravo accountLogin = %q, want bravo-org", gotConn.AccountLogin)
	}

	if resp := call(erin, "GET", "/v1/git/connection?ownerId="+wsB.ID, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("erin GET /v1/git/connection?ownerId=bravo: %d, want 403", resp.StatusCode)
	}

	// --- (3) api-keys: a key MINTED in bravo (explicit ownerId) binds there —
	// the "which workspace does a new key bind to" gap — then LIST is scoped
	// per-workspace (the "list scoping" gap: before this milestone ListAPIKeys
	// had no tenant filter at all), and REVOKE refuses to act cross-workspace. ---

	createResp := call(dana, "POST", "/v1/api-keys", `{"name":"agent-alpha"}`) // no ownerId => her default, alpha
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent-alpha: %d", createResp.StatusCode)
	}
	keyA := decodeJSON[apikeys.APIKey](t, createResp)

	createResp = call(dana, "POST", "/v1/api-keys", `{"name":"agent-bravo","ownerId":"`+wsB.ID+`"}`)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent-bravo: %d", createResp.StatusCode)
	}
	keyB := decodeJSON[apikeys.APIKey](t, createResp)

	// List with no ownerId (default alpha) sees only agent-alpha.
	alphaKeys := decodeJSON[[]apikeys.APIKey](t, call(dana, "GET", "/v1/api-keys", ""))
	if !onlyContains(alphaKeys, keyA.ID) {
		t.Errorf("GET /v1/api-keys (no ownerId) = %+v, want exactly [%s]", idsOf(alphaKeys), keyA.ID)
	}

	// List with ownerId=bravo sees only agent-bravo.
	bravoKeys := decodeJSON[[]apikeys.APIKey](t, call(dana, "GET", "/v1/api-keys?ownerId="+wsB.ID, ""))
	if !onlyContains(bravoKeys, keyB.ID) {
		t.Errorf("GET /v1/api-keys?ownerId=bravo = %+v, want exactly [%s]", idsOf(bravoKeys), keyB.ID)
	}

	// Revoking agent-alpha (bound to alpha) while TARGETING bravo is refused —
	// a caller who can manage bravo's keys may not reach into alpha's, even
	// though dana herself administers both (the same cross-workspace gate
	// w6/m14 gave Apps/Databases/KeyValues).
	if resp := call(dana, "DELETE", "/v1/api-keys/"+keyA.ID+"?ownerId="+wsB.ID, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("DELETE agent-alpha targeting bravo: %d, want 403", resp.StatusCode)
	}

	// Revoking it against its OWN workspace (the default, no ownerId) succeeds.
	if resp := call(dana, "DELETE", "/v1/api-keys/"+keyA.ID, ""); resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE agent-alpha (default): %d, want 204", resp.StatusCode)
	}

	// erin cannot even list bravo's keys.
	if resp := call(erin, "GET", "/v1/api-keys?ownerId="+wsB.ID, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("erin GET /v1/api-keys?ownerId=bravo: %d, want 403", resp.StatusCode)
	}
}

// workspaceIDResponse/accountLoginResponse decode just the one field each
// assertion below needs from GET /v1/usage / GET /v1/git/connection.
type workspaceIDResponse struct {
	WorkspaceID string `json:"workspaceId"`
}

type accountLoginResponse struct {
	AccountLogin string `json:"accountLogin"`
}

// decodeJSON decodes resp's body as T, failing the test on a decode error —
// shared by every REST response this file reads.
func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func onlyContains(keys []apikeys.APIKey, id string) bool {
	if len(keys) != 1 {
		return false
	}
	return keys[0].ID == id
}

func idsOf(keys []apikeys.APIKey) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.ID
	}
	return out
}

// fakeGitHubAPIClient is the minimal github.APIClient this test needs: only
// InstallURL is read (GetConnection's install-CTA field); the rest are unused
// by the read verbs under test.
type fakeGitHubAPIClient struct{}

func (f *fakeGitHubAPIClient) InstallURL() string {
	return "https://github.com/apps/bex/installations/new"
}
func (f *fakeGitHubAPIClient) GetInstallation(context.Context, int64) (github.Installation, error) {
	return github.Installation{}, nil
}
func (f *fakeGitHubAPIClient) ListRepos(context.Context, int64) ([]github.Repo, error) {
	return nil, nil
}
func (f *fakeGitHubAPIClient) ListBranches(context.Context, int64, string, string) ([]string, error) {
	return nil, nil
}
func (f *fakeGitHubAPIClient) MintInstallationToken(context.Context, int64) (github.InstallationToken, error) {
	return github.InstallationToken{}, nil
}
func (f *fakeGitHubAPIClient) RepoAccessible(context.Context, string, string, string) (bool, error) {
	return false, nil
}
func (f *fakeGitHubAPIClient) GetCommit(context.Context, string, string, string, string) (github.Commit, error) {
	return github.Commit{}, nil
}

func (f *fakeGitHubAPIClient) GetFileContents(context.Context, string, string, string, string, string) (github.FileContents, error) {
	return github.FileContents{}, nil
}

func (f *fakeGitHubAPIClient) GetRepoCommitSHA(context.Context, string, string, string, string) (string, error) {
	return "", nil
}
func (f *fakeGitHubAPIClient) OpenDraftPullRequest(context.Context, int64, string, string, string, string, string, string) (github.PullRequest, error) {
	return github.PullRequest{}, nil
}
