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
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// organization_api_keys_pg_test.go drives an organization's audit-log key
// through the composed server over the real control-plane store (w1/m657,
// ADR115 D11): an Org Owner mints it on GraphQL, and the key alone, sent as
// a bearer, reads the org's audit log and the log of every workspace the org
// holds when it asks, and nothing else on any surface; a revoke and the org's
// end stop it.

// ownerOnly is OpenFGA for this test: the one person is allowed everything,
// and nobody else anything, so an org key is never let in by a tuple.
type ownerOnly string

func (o ownerOnly) Check(_ context.Context, subject, _, _ string) (bool, error) {
	return subject == "user:"+string(o), nil
}

func TestOrganizationAuditLogKeyPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	const owner = "identity-1" // fakeKratos's session
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	workspace := func(name string) store.Tenant {
		t.Helper()
		w, err := pg.CreateWorkspace(ctx, name+"-"+stamp, store.PlanPro, owner)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), w.ID) })
		return w
	}
	organization := func(name string) (store.Tenant, store.Organization) {
		t.Helper()
		primary := workspace(name)
		_, org, _, err := pg.UpgradeToOrganization(ctx, primary.ID, store.PlanScale, []string{owner})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM organization_api_keys WHERE organization_id = $1`, org.ID)
		})
		return primary, org
	}
	record := func(resource, id string) {
		t.Helper()
		if err := pg.Record(ctx, core.AuditEvent{ID: id + "-" + stamp, Verb: "apps.Create", Resource: resource, Outcome: core.AuditAllowed, At: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}

	primary, org := organization("orgkey-primary")
	secondary, err := pg.CreateOrganizationWorkspace(ctx, org.ID, "orgkey-second-"+stamp, store.PrivacyInviteOnly, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), secondary.ID) })
	outsider := workspace("orgkey-outsider")
	_, otherOrg := organization("orgkey-other")
	for _, r := range []struct{ resource, id string }{
		{core.OrganizationObject(org.ID), "aud-org"}, {core.WorkspaceObject(primary.ID), "aud-primary"},
		{core.WorkspaceObject(secondary.ID), "aud-secondary"}, {core.WorkspaceObject(outsider.ID), "aud-outsider"},
		{core.OrganizationObject(otherOrg.ID), "aud-other-org"},
	} {
		record(r.resource, r.id)
	}

	hydra := newKeyHydra(t)
	kratos := fakeKratos(t)
	keys := &hydraKeys{hydra: hydra, next: []string{keyClientA, keyClientB}}
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: ownerOnly(owner), Plans: pg, Audit: pg,
		KeyGrants: pg, OrganizationKeys: pg,
	}
	srv := NewServer(base, Deps{
		WorkspaceStore:   pg,
		APIKeys:          keys,
		OrganizationKeys: pg,
		Audit:            &audit.Service{Base: base, Store: pg},
	})
	srv.HydraAdminURL, srv.OAuthIssuer, srv.KratosURL = hydra.URL, hydra.URL, kratos.URL
	h := buildHandler(t, srv)
	session := map[string]string{"Cookie": "ory_kratos_session=live", "Content-Type": "application/json"}
	graphQL := func(query string) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"query": query})
		w := request(h, http.MethodPost, "/graphql", string(body), session)
		var out struct {
			Data   map[string]any `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Errors) > 0 {
			t.Fatalf("%s = %d %s", query, w.Code, w.Body.String())
		}
		return out.Data
	}
	mint := func(label string) string {
		t.Helper()
		data := graphQL(`mutation { provisionOrganizationAuditLogToken(organizationId: "` + org.ID + `", label: "` + label + `") { token } }`)
		token, _ := data["provisionOrganizationAuditLogToken"].(map[string]any)["token"].(string)
		if !strings.HasPrefix(token, "bex_") {
			t.Fatalf("provisionOrganizationAuditLogToken answered %v", data)
		}
		return token
	}
	listed := func() []any {
		t.Helper()
		data := graphQL(`{ organization(organizationId: "` + org.ID + `") { organizationAPITokens { id label createdAt lastUsedAt expiresAt type } } }`)
		return data["organization"].(map[string]any)["organizationAPITokens"].([]any)
	}
	logIDs := func(bearer, path string) (int, []string) {
		t.Helper()
		w := request(h, http.MethodGet, path, "", map[string]string{"Authorization": "Bearer " + bearer})
		if w.Code != http.StatusOK {
			return w.Code, nil
		}
		var page []struct {
			AuditLog struct {
				ID string `json:"id"`
			} `json:"auditLog"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, row := range page {
			ids = append(ids, strings.TrimSuffix(row.AuditLog.ID, "-"+stamp))
		}
		return w.Code, ids
	}
	orgLog := "/v1/organizations/" + org.ID + "/audit-logs"
	ownerLog := func(ws string) string { return "/v1/owners/" + ws + "/audit-logs" }

	// The Owner mints a key; it is shown once and listed without it.
	key := mint("siem")
	tokens := listed()
	if len(tokens) != 1 {
		t.Fatalf("organizationAPITokens = %v, want the one key", tokens)
	}
	token := tokens[0].(map[string]any)
	if token["id"] != keyClientA || token["label"] != "siem" || token["type"] != "AUDIT_LOG" || token["expiresAt"] != nil || token["createdAt"] == "" {
		t.Fatalf("listed key = %v", token)
	}
	if raw, _ := json.Marshal(tokens); strings.Contains(string(raw), key) {
		t.Fatal("the list carries the key")
	}

	// The key reads the org's log and each org workspace's, only their rows.
	for path, want := range map[string]string{
		orgLog: "aud-org", ownerLog(primary.ID): "aud-primary", ownerLog(secondary.ID): "aud-secondary",
	} {
		if code, ids := logIDs(key, path); code != http.StatusOK || !slices.Contains(ids, want) || slices.Contains(ids, "aud-outsider") || slices.Contains(ids, "aud-other-org") {
			t.Errorf("GET %s with the key = %d %v, want 200 with %s", path, code, ids, want)
		}
	}
	// A workspace outside the org and another org's log refuse it.
	for _, path := range []string{ownerLog(outsider.ID), "/v1/organizations/" + otherOrg.ID + "/audit-logs"} {
		if code, _ := logIDs(key, path); code != http.StatusForbidden {
			t.Errorf("GET %s with the key = %d, want 403", path, code)
		}
	}
	// Every other surface refuses it: REST, GraphQL (the workspace log's own
	// GraphQL field included) and MCP.
	for surface, w := range surfaceCalls(h, key) {
		if w.Code == http.StatusOK && !strings.Contains(w.Body.String(), `"errors"`) && !strings.Contains(w.Body.String(), `"isError":true`) {
			t.Errorf("%s with the key = %d %s, want a refusal", surface, w.Code, w.Body.String())
		}
	}
	bearer := map[string]string{"Authorization": "Bearer " + key, "Content-Type": "application/json"}
	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/users", ""},
		{http.MethodGet, "/v1/owners", ""},
		{http.MethodPost, "/graphql", `{"query":"{ auditLogs(ownerId: \"` + primary.ID + `\") { id } }"}`},
		{http.MethodPost, "/graphql", `{"query":"{ organizations { id } }"}`},
		{http.MethodPost, "/graphql", `{"query":"mutation { provisionOrganizationAuditLogToken(organizationId: \"` + org.ID + `\", label: \"x\") { token } }"}`},
	} {
		w := request(h, call.method, call.path, call.body, bearer)
		if w.Code == http.StatusOK && !strings.Contains(w.Body.String(), `"errors"`) {
			t.Errorf("%s %s %s with the key = %d %s, want a refusal", call.method, call.path, call.body, w.Code, w.Body.String())
		}
	}

	// A workspace the org gains after the mint is read; one it loses is not.
	later, err := pg.CreateOrganizationWorkspace(ctx, org.ID, "orgkey-later-"+stamp, store.PrivacyPublic, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), later.ID) })
	record(core.WorkspaceObject(later.ID), "aud-later")
	if code, ids := logIDs(key, ownerLog(later.ID)); code != http.StatusOK || !slices.Contains(ids, "aud-later") {
		t.Errorf("a workspace created after the key = %d %v, want 200", code, ids)
	}
	if _, err := pool.Exec(ctx, `UPDATE tenants SET organization_id = NULL WHERE id = $1`, later.ID); err != nil {
		t.Fatal(err)
	}
	if code, _ := logIDs(key, ownerLog(later.ID)); code != http.StatusForbidden {
		t.Errorf("a workspace that left the org = %d, want 403", code)
	}

	// Revoked, it stops at once; a second revoke is not found.
	revoke := `mutation { revokeOrganizationAPIToken(organizationId: "` + org.ID + `", tokenId: "` + keyClientA + `") }`
	if data := graphQL(revoke); data["revokeOrganizationAPIToken"] != true {
		t.Fatalf("revoke = %v", data)
	}
	if code, _ := logIDs(key, orgLog); code == http.StatusOK {
		t.Error("the revoked key still reads the org's log")
	}
	body, _ := json.Marshal(map[string]string{"query": revoke})
	if w := request(h, http.MethodPost, "/graphql", string(body), session); !strings.Contains(w.Body.String(), "not found") {
		t.Errorf("a second revoke = %s, want not found", w.Body.String())
	}
	if tokens := listed(); len(tokens) != 0 {
		t.Errorf("after the revoke organizationAPITokens = %v", tokens)
	}

	// The org's end leaves its keys reading nothing, before and after the
	// purge that retires them.
	second := mint("archive")
	if code, _ := logIDs(second, ownerLog(primary.ID)); code != http.StatusOK {
		t.Fatalf("the second key = %d, want 200", code)
	}
	for _, ws := range []string{secondary.ID, later.ID} {
		if err := pg.DeleteTenant(ctx, ws); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pg.DissolveOrganization(ctx, org.ID, store.PlanPro); err != nil {
		t.Fatal(err)
	}
	if code, _ := logIDs(second, ownerLog(primary.ID)); code != http.StatusForbidden {
		t.Errorf("a key of a dissolved org = %d, want 403", code)
	}
	if err := (apikeys.OrganizationPurger{Store: keys, Keys: pg}).PurgeOrganization(ctx, org.ID); err != nil {
		t.Fatal(err)
	}
	if live, _ := pg.ListOrganizationAPIKeys(ctx, org.ID); len(live) != 0 {
		t.Errorf("after the purge the org still has live keys %v", live)
	}
	if code, _ := logIDs(second, ownerLog(primary.ID)); code == http.StatusOK {
		t.Error("a purged key still reads")
	}
}
