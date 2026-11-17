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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/cliauth"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/oauthgrant"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// oauthAccessRows are a member's grant to an agent and its revocation, as
// internal/oauthgrant records them (w4/m454).
func oauthAccessRows(workspace string, at time.Time) []store.AuditRow {
	return []store.AuditRow{
		{ID: "aud-revoke", WorkspaceID: workspace, Caller: "human-1", CallerMethod: "session", Verb: core.AuditVerbRevokeOAuthAccess,
			Resource: core.WorkspaceObject(workspace), Target: core.OAuthClientTarget("dcr-claude"), TargetName: "Claude Code",
			Outcome: string(core.AuditAllowed), At: at},
		{ID: "aud-grant", WorkspaceID: workspace, Caller: "human-1", CallerMethod: "session", Verb: core.AuditVerbGrantOAuthAccess,
			Resource: core.WorkspaceObject(workspace), Target: core.OAuthClientTarget("dcr-claude"), TargetName: "Claude Code",
			OAuthGrantScopes: []string{"openid", "offline_access", "bex.read"}, Outcome: string(core.AuditAllowed), At: at.Add(-time.Minute)},
	}
}

type restAuditLog struct {
	AuditLog struct {
		ID       string `json:"id"`
		Event    string `json:"event"`
		Actor    struct{ Type, ID string }
		Metadata map[string]string `json:"metadata"`
	} `json:"auditLog"`
}

// readOAuthAccessSurfaces reads workspace's audit log over REST and GraphQL.
func readOAuthAccessSurfaces(t *testing.T, h http.Handler, workspace string) ([]restAuditLog, []any) {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/v1/owners/"+workspace+"/audit-logs", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("REST audit-logs: %d %s", rec.Code, rec.Body.String())
	}
	var rest []restAuditLog
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	data := gql(t, h, `{ auditLogs(ownerId: "`+workspace+`") { id action actorMethod status resource targetName oauthGrantScopes oauthClientId } }`)
	rows, _ := data["auditLogs"].([]any)
	return rest, rows
}

// assertOAuthAccessSurfaces checks one grant and one revocation, newest first,
// agree on REST and GraphQL and name the client and the granted scopes.
func assertOAuthAccessSurfaces(t *testing.T, rest []restAuditLog, gqlRows []any) {
	t.Helper()
	if len(rest) != 2 || len(gqlRows) != 2 {
		t.Fatalf("REST %d rows, GraphQL %d rows; want the revocation and the grant", len(rest), len(gqlRows))
	}
	wantREST := []struct {
		event    string
		metadata map[string]string
	}{
		{"RevokeOAuthAccessEvent", map[string]string{"client_id": "dcr-claude", "client_name": "Claude Code"}},
		{"GrantOAuthAccessEvent", map[string]string{"client_id": "dcr-claude", "client_name": "Claude Code", "scopes": "openid offline_access bex.read"}},
	}
	for i, want := range wantREST {
		got := rest[i].AuditLog
		if got.Event != want.event || got.Actor.Type != "user" || !reflect.DeepEqual(got.Metadata, want.metadata) {
			t.Errorf("REST row %d = %+v, want %s by the member with %v", i, got, want.event, want.metadata)
		}
		row := gqlRows[i].(map[string]any)
		if row["id"] != got.ID || row["action"] != want.event || row["actorMethod"] != "session" || row["status"] != "success" ||
			row["resource"] != "oauth-client:dcr-claude" || row["targetName"] != "Claude Code" || row["oauthClientId"] != nil {
			t.Errorf("GraphQL row %d = %v, diverges from REST %+v", i, row, got)
		}
	}
	if scopes := gqlRows[0].(map[string]any)["oauthGrantScopes"]; scopes != nil {
		t.Errorf("the revocation's oauthGrantScopes = %v, want null", scopes)
	}
	if scopes := gqlRows[1].(map[string]any)["oauthGrantScopes"]; !reflect.DeepEqual(scopes, []any{"openid", "offline_access", "bex.read"}) {
		t.Errorf("the grant's oauthGrantScopes = %v", scopes)
	}
}

// TestOAuthAccessEventsAgreeOnEverySurface is TestAuditSurfaceParity's
// sibling for w4/m454's two events: REST's Render metadata keys and GraphQL's
// fields describe the same grant and revocation.
func TestOAuthAccessEventsAgreeOnEverySurface(t *testing.T) {
	fakeStore := &fakeAuditStore{rows: oauthAccessRows("tea-a", time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: fakeStore}})
	rest, rows := readOAuthAccessSurfaces(t, h, "tea-a")
	assertOAuthAccessSurfaces(t, rest, rows)
}

// consentHydra is Hydra's admin API as internal/oauthgrant reads it: one
// subject's consent sessions and one client's registration.
func consentHydra(t *testing.T, subject string, handledAt time.Time) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/admin/oauth2/auth/sessions/consent" && r.URL.Query().Get("subject") == subject:
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"grant_scope": []string{"openid", "offline_access", "bex.read"},
				"handled_at":  handledAt.Format(time.RFC3339Nano),
				"consent_request": map[string]any{"client": map[string]any{
					"client_id": "dcr-claude", "client_name": "Claude Code",
				}},
			}})
		case r.URL.Path == "/admin/oauth2/auth/sessions/consent":
			_, _ = w.Write([]byte("[]"))
		case r.URL.Path == "/admin/clients/dcr-claude":
			_, _ = w.Write([]byte(`{"client_id":"dcr-claude","client_name":"Claude Code"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestOAuthAccessIsAuditedInEachOfTheMembersWorkspaces pins w4/m454's
// definition of done against the real store: a member's approval, reported by
// the dashboard (twice, as a retry would), and the revocation under Connected
// agents leave exactly one GrantOAuthAccessEvent and one RevokeOAuthAccessEvent
// in each workspace the member belongs to and none anywhere else; a forged
// report for a client the member never approved records nothing; and REST and
// GraphQL read both rows with the client and the granted scopes.
func TestOAuthAccessIsAuditedInEachOfTheMembersWorkspaces(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	run := uniqueRunSuffix()
	member, stranger := "human-oauth-"+run, "human-other-"+run
	var workspaces []string
	for i, owner := range []string{member, member, stranger} {
		tenant, err := st.CreateWorkspace(ctx, fmt.Sprintf("oauth-access-%s-%d", run, i), store.PlanPro, owner)
		if err != nil {
			t.Fatal(err)
		}
		workspaces = append(workspaces, tenant.ID)
	}
	t.Cleanup(func() {
		for _, ws := range workspaces {
			_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, ws)
			_ = st.DeleteTenant(context.Background(), ws)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM oauth_revocations WHERE subject = $1`, member)
	})

	now := time.Now()
	svc := cliauth.New("", nil, nil)
	svc.Revocations = st
	svc.Access = &oauthgrant.Access{HydraAdminURL: consentHydra(t, member, now.Add(-time.Minute)).URL, Workspaces: st, Audit: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	report := func(path, clientID string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"clientId":"`+clientID+`"}`))
		req = req.WithContext(core.WithIdentity(ctx, core.Identity{Subject: member, Method: "session", Human: true}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	for range 2 {
		if code := report("/v1/oauth/grants", "dcr-claude"); code != http.StatusNoContent {
			t.Fatalf("grant report => %d", code)
		}
	}
	if code := report("/v1/oauth/grants", "never-approved"); code != http.StatusNotFound {
		t.Fatalf("forged grant report => %d, want 404", code)
	}
	if code := report("/v1/oauth/revocations", "dcr-claude"); code != http.StatusNoContent {
		t.Fatalf("revocation => %d", code)
	}

	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: st}})
	for _, ws := range workspaces[:2] {
		rest, rows := readOAuthAccessSurfaces(t, h, ws)
		assertOAuthAccessSurfaces(t, rest, rows)
	}
	if rows, err := st.ListAuditEvents(ctx, workspaces[2], store.AuditFilter{}); err != nil || len(rows) != 0 {
		t.Fatalf("a workspace the member is not in holds %d rows (%v), want none", len(rows), err)
	}
}
