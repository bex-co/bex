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
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// TestSandboxGroupCredentialsE2E exercises all 7 Render sandbox group
// credential operations end-to-end against real Postgres, real OpenBao, and
// real OpenFGA, validating EVERY response against Render's pinned OpenAPI
// schema with zero allowlist entries (w2/m532 t006).
func TestSandboxGroupCredentialsE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	baoURL, baoToken := os.Getenv("BEX_TEST_OPENBAO_KV_URL"), os.Getenv("BEX_TEST_OPENBAO_KV_TOKEN")
	if dbURI == "" || fgaURL == "" || baoURL == "" || baoToken == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI, BEX_TEST_OPENFGA_URL, BEX_TEST_OPENBAO_KV_URL and BEX_TEST_OPENBAO_KV_TOKEN not all set")
	}
	ctx := context.Background()
	spec := loadRenderSpec(t)

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	baoStore, removeMount, err := secrets.NewOpenBaoTestStore(ctx, baoURL, baoToken)
	if err != nil {
		t.Fatalf("OpenBao test store: %v", err)
	}
	t.Cleanup(removeMount)

	run := uniqueRunSuffix()
	admin, dev, viewer, outsider := "sbcred-admin-"+run, "sbcred-dev-"+run, "sbcred-viewer-"+run, "sbcred-out-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)

	ws, err := st.CreateWorkspace(ctx, "sbcred-ws-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	otherWS, err := st.CreateWorkspace(ctx, "sbcred-other-"+run, store.PlanPro, outsider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), otherWS.ID) })

	seat := func(wsID, subject, role string) {
		t.Helper()
		if err := roles.GrantWorkspaceRole(ctx, wsID, "user:"+subject, role); err != nil {
			t.Fatalf("grant %s %s on %s: %v", subject, role, wsID, err)
		}
	}
	seat(ws.ID, admin, "admin")
	seat(otherWS.ID, outsider, "admin")
	for _, m := range []struct{ subject, role string }{{dev, "developer"}, {viewer, "viewer"}} {
		if err := st.AddMember(ctx, m.subject, ws.ID, m.role); err != nil {
			t.Fatal(err)
		}
		seat(ws.ID, m.subject, m.role)
	}

	group, err := st.CreateSandboxGroup(ctx, ws.ID, "cred-primary-"+run, store.SandboxGroupRegion, "", 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	secondGroup, err := st.CreateSandboxGroup(ctx, ws.ID, "cred-deletable-"+run, store.SandboxGroupRegion, "", 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	foreignGroup, err := st.CreateSandboxGroup(ctx, otherWS.ID, "cred-foreign-"+run, store.SandboxGroupRegion, "", 100, 10)
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(upstream.Close)

	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	resolver := tenancy.NewTenantService(st, roles)
	auditSink := &fakeAuditSink{}
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: checker, Workspace: resolver, KeyCreators: keys, KeyGrants: resolver,
		Audit: auditSink,
	}
	vault := sandbox.NewCredentialVault(baoStore)
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		APIKeys: keys, KeyBinder: resolver,
		Secrets:            baoStore,
		SandboxClient:      sandbox.NewClient(upstream.URL),
		SandboxGroups:      st,
		SandboxCredentials: st,
		SandboxExecutions:  st,
		WorkspacePreCascadePurgers: []workspaces.WorkspacePurger{
			&sandbox.WorkspacePurger{Credentials: st, CredentialVault: vault},
		},
	})
	mux := srv.restHandler()
	validated, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatalf("render request validator: %v", err)
	}
	gated := srv.withScopeClassREST(mux, validated)

	session := func(subject string) core.Identity {
		return core.Identity{Subject: subject, Method: core.MethodSession, Human: true}
	}
	keyOf := func(id string) core.Identity {
		return core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2}
	}
	call := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	assertConform := func(t *testing.T, opID string, status int, body []byte) {
		t.Helper()
		if errs := rendertest.Unallowed(opID, spec.validateStatus(opID, status, body)); len(errs) > 0 {
			t.Errorf("%s (%d) schema violation(s):\n  %s\nbody: %s", opID, status, strings.Join(errs, "\n  "), string(body))
		}
	}

	basePath := "/v1/sandbox-groups/" + group.ID + "/credentials"

	// 1. Empty list conforms to Render schema and returns [].
	rec := call(session(viewer), http.MethodGet, basePath, "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list = %d %s", rec.Code, rec.Body.String())
	}
	assertConform(t, "list-sandbox-group-credentials", rec.Code, rec.Body.Bytes())

	// 2. Developer cannot create (403); foreign workspace group answers 404.
	secret1 := "sk-live-secret-v1-" + run
	createBody := fmt.Sprintf(`{"id":"openai-key","domain":"API.OpenAI.com","value":%q,"methods":["post"],"paths":["/v1/**"],"injection":{"type":"header","name":"Authorization","template":"Bearer ${SECRET}"}}`, secret1)
	rec = call(session(dev), http.MethodPost, basePath, createBody)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer create = %d %s, want 403", rec.Code, rec.Body.String())
	}
	assertConform(t, "create-sandbox-group-credential", rec.Code, rec.Body.Bytes())

	rec = call(session(admin), http.MethodPost, "/v1/sandbox-groups/"+foreignGroup.ID+"/credentials", createBody)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign group create = %d %s, want 404", rec.Code, rec.Body.String())
	}

	// 3. Admin creates credential (201), conforms to Render schema, never returns secret.
	rec = call(session(admin), http.MethodPost, basePath, createBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create = %d %s, want 201", rec.Code, rec.Body.String())
	}
	assertConform(t, "create-sandbox-group-credential", rec.Code, rec.Body.Bytes())
	if strings.Contains(rec.Body.String(), secret1) {
		t.Fatalf("create response leaked secret: %s", rec.Body.String())
	}
	var credV1 sandbox.GroupCredential
	if err := json.Unmarshal(rec.Body.Bytes(), &credV1); err != nil {
		t.Fatal(err)
	}
	if credV1.ID != "openai-key" || credV1.Domain != "api.openai.com" || credV1.Version.ID != 1 {
		t.Fatalf("unexpected v1: %+v", credV1)
	}

	// Verify OpenBao holds v1 secret and Postgres does NOT.
	storedV1, err := st.GetSandboxGroupCredential(ctx, ws.ID, group.ID, "openai-key")
	if err != nil {
		t.Fatal(err)
	}
	if gotSec, ok, err := vault.Get(ctx, ws.ID, group.ID, "openai-key", storedV1.CurrentSecretRef); err != nil || !ok || gotSec != secret1 {
		t.Fatalf("OpenBao v1 = (%q, %v, %v), want %q", gotSec, ok, err, secret1)
	}
	var pgLeaks int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM sandbox_group_credential_versions
		WHERE workspace_id = $1 AND (config_json::text LIKE $2 OR fingerprint LIKE $2 OR secret_ref LIKE $2)`,
		ws.ID, "%"+secret1+"%").Scan(&pgLeaks); err != nil || pgLeaks != 0 {
		t.Fatalf("Postgres leaked secret (%d rows, err=%v)", pgLeaks, err)
	}

	// 4. List & Retrieve conform to Render schema.
	rec = call(session(viewer), http.MethodGet, basePath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	assertConform(t, "list-sandbox-group-credentials", rec.Code, rec.Body.Bytes())

	credPath := basePath + "/openai-key"
	rec = call(session(viewer), http.MethodGet, credPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("retrieve = %d %s", rec.Code, rec.Body.String())
	}
	assertConform(t, "retrieve-sandbox-group-credential", rec.Code, rec.Body.Bytes())

	// 5. Split RBAC PATCH: Developer can update methods/enabled/injection; cannot update domain or value (403).
	rec = call(session(dev), http.MethodPatch, credPath, `{"enabled":false,"methods":[],"paths":["/v1/*"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("developer allowed PATCH = %d %s, want 200", rec.Code, rec.Body.String())
	}
	assertConform(t, "update-sandbox-group-credential", rec.Code, rec.Body.Bytes())

	rec = call(session(dev), http.MethodPatch, credPath, `{"domain":"api.anthropic.com"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer PATCH domain = %d %s, want 403", rec.Code, rec.Body.String())
	}
	assertConform(t, "update-sandbox-group-credential", rec.Code, rec.Body.Bytes())

	rec = call(session(dev), http.MethodPatch, credPath, `{"value":"sk-dev-attempt"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer PATCH value = %d %s, want 403", rec.Code, rec.Body.String())
	}

	// 6. Push 3 more distinct versions (v3, v4, v5) with distinct secrets so v1 is evicted from retention AND purged from OpenBao!
	secretV1Ref := storedV1.CurrentSecretRef
	// Note: v1 and v2 shared secret1 (v2 only changed enabled/methods/paths). Push v3..v6 with distinct secrets so BOTH v1 and v2 are evicted!
	for i := 3; i <= 6; i++ {
		sec := fmt.Sprintf("sk-live-secret-v%d-%s", i, run)
		rec = call(session(admin), http.MethodPatch, credPath, fmt.Sprintf(`{"value":%q,"enabled":true}`, sec))
		if rec.Code != http.StatusOK {
			t.Fatalf("admin PATCH v%d = %d %s", i, rec.Code, rec.Body.String())
		}
		assertConform(t, "update-sandbox-group-credential", rec.Code, rec.Body.Bytes())
	}
	// After v6: retained distinct configs are v6 (current) + v5, v4, v3 (3 prior). v1 & v2 (secret1) are evicted!
	if _, ok, err := vault.Get(ctx, ws.ID, group.ID, "openai-key", secretV1Ref); err != nil || ok {
		t.Fatalf("evicted secret1 still in OpenBao: ok=%v err=%v", ok, err)
	}

	// List versions conforms and returns [6, 5, 4, 3].
	rec = call(session(viewer), http.MethodGet, credPath+"/versions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list versions = %d %s", rec.Code, rec.Body.String())
	}
	assertConform(t, "list-sandbox-group-credential-versions", rec.Code, rec.Body.Bytes())
	var vers []sandbox.GroupCredentialVersion
	if err := json.Unmarshal(rec.Body.Bytes(), &vers); err != nil {
		t.Fatal(err)
	}
	gotVIDs := make([]int64, len(vers))
	for i, v := range vers {
		gotVIDs[i] = v.ID
	}
	if !slices.Equal(gotVIDs, []int64{6, 5, 4, 3}) {
		t.Fatalf("retained version IDs = %v, want [6 5 4 3]", gotVIDs)
	}

	// 7. Restore evicted v1 -> 404; restore retained v3 -> 200 with restoredFromVersionId=3 & matching fingerprint!
	rec = call(session(admin), http.MethodPost, credPath+"/restore", `{"versionId":1}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("restore evicted v1 = %d %s, want 404", rec.Code, rec.Body.String())
	}
	assertConform(t, "restore-sandbox-group-credential", rec.Code, rec.Body.Bytes())

	rec = call(session(admin), http.MethodPost, credPath+"/restore", `{"versionId":3}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore retained v3 = %d %s, want 200", rec.Code, rec.Body.String())
	}
	assertConform(t, "restore-sandbox-group-credential", rec.Code, rec.Body.Bytes())
	var restored sandbox.GroupCredential
	if err := json.Unmarshal(rec.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Version.ID != 7 || restored.Version.RestoredFromVersionID == nil || *restored.Version.RestoredFromVersionID != 3 {
		t.Fatalf("restored = %+v, want v7 restoredFrom=3", restored.Version)
	}
	if restored.Version.Fingerprint != vers[len(vers)-1].Fingerprint {
		t.Fatalf("restored fingerprint %s != v3 fingerprint %s", restored.Version.Fingerprint, vers[len(vers)-1].Fingerprint)
	}

	// 8. Scoped API key enforcement (KeySandboxesHistory for reads, KeySandboxesCredentials for writes + creator ceiling).
	mintKey := func(creator, name, permsJSON string) core.Identity {
		t.Helper()
		rec := call(session(creator), http.MethodPost, "/v1/api-keys",
			fmt.Sprintf(`{"name":%q,"ownerId":%q,"permissions":%s}`, name, ws.ID, permsJSON))
		if rec.Code != http.StatusCreated {
			t.Fatalf("mint key %s = %d %s", name, rec.Code, rec.Body.String())
		}
		var k apikeys.APIKey
		if err := json.Unmarshal(rec.Body.Bytes(), &k); err != nil {
			t.Fatal(err)
		}
		return keyOf(k.ID)
	}
	historyKey := mintKey(admin, "hist-"+run, `{"grants":[{"permission":"sandboxes","level":"read"}]}`)
	adminCredKey := mintKey(admin, "admcred-"+run, `{"grants":[{"permission":"sandboxes","level":"read"},{"permission":"sandbox_credentials","level":"write"}]}`)
	devCredKey := mintKey(dev, "devcred-"+run, `{"grants":[{"permission":"sandboxes","level":"read"},{"permission":"sandbox_credentials","level":"write"}]}`)

	// History-only key can read but is refused (403 insufficient_token_scope) on write.
	if rec := call(historyKey, http.MethodGet, basePath, ""); rec.Code != http.StatusOK {
		t.Fatalf("historyKey GET = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := call(historyKey, http.MethodPatch, credPath, `{"enabled":false}`); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), core.KeyPermissionDeniedCode) {
		t.Fatalf("historyKey PATCH = %d %s, want 403 %s", rec.Code, rec.Body.String(), core.KeyPermissionDeniedCode)
	}
	// Developer key with KeySandboxesCredentials can PATCH enabled, but creator ceiling still blocks domain/value/delete (403).
	if rec := call(devCredKey, http.MethodPatch, credPath, `{"enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("devCredKey allowed PATCH = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := call(devCredKey, http.MethodPatch, credPath, `{"domain":"api.github.com"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("devCredKey PATCH domain = %d %s, want 403", rec.Code, rec.Body.String())
	}
	if rec := call(devCredKey, http.MethodDelete, credPath, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("devCredKey DELETE = %d %s, want 403", rec.Code, rec.Body.String())
	}
	// Admin key with KeySandboxesCredentials deletes credential -> 202 empty body, OpenBao purged, repeat 202!
	rec = call(adminCredKey, http.MethodDelete, credPath, "")
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("adminCredKey DELETE = %d %q, want 202 empty", rec.Code, rec.Body.String())
	}
	assertConform(t, "delete-sandbox-group-credential", rec.Code, rec.Body.Bytes())
	rec = call(adminCredKey, http.MethodDelete, credPath, "")
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("repeat DELETE = %d %q, want 202 empty", rec.Code, rec.Body.String())
	}
	// Confirm OpenBao has zero entries left under group/openai-key.
	if keysLeft, err := baoStore.List(secrets.WithTenant(ctx, ws.ID), "sandbox-groups/"+group.ID+"/credentials"); err != nil || len(keysLeft) != 0 {
		t.Fatalf("OpenBao still holds credentials after delete: keys=%v err=%v", keysLeft, err)
	}

	// 8b. Real-DB + OpenBao + OpenFGA cross-surface parity on GraphQL and MCP
	// (including ownerId / per-call workspaceId, split RBAC, scoped API keys,
	// version history + restore, and write-only secret redaction — w2/m533 t003).
	gqlSecret1 := "sk-gql-secret-v1-" + run
	gqlSecret2 := "sk-gql-secret-v2-" + run
	gqlOut := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation {
		createSandboxGroupCredential(
			sandboxGroupId: %q,
			ownerId: %q,
			input: {
				id: "gql-cred",
				domain: "API.Stripe.com",
				value: %q,
				methods: ["post"],
				paths: ["/v1/charges"],
				injection: { type: "header", name: "Authorization", template: "Bearer ${SECRET}" }
			}
		) { id domain enabled methods paths injection { type name template } version { id updatedBy fingerprint restoredFromVersionId } }
	}`, group.ID, ws.ID, gqlSecret1))
	if code := graphqlCode(gqlOut); code != "" {
		t.Fatalf("GraphQL admin create failed: %v", gqlOut)
	}
	if raw, _ := json.Marshal(gqlOut); strings.Contains(string(raw), gqlSecret1) {
		t.Fatalf("GraphQL create leaked secret: %s", raw)
	}
	// GraphQL scoped API keys: historyKey reads, refused on write; devCredKey updates enabled, refused on domain/value.
	gqlList := graphqlDo(t, srv, historyKey, fmt.Sprintf(`{ sandboxGroupCredentials(sandboxGroupId: %q, ownerId: %q) { id domain enabled version { id } } }`, group.ID, ws.ID))
	if errs, _ := gqlList["errors"].([]any); len(errs) != 0 {
		t.Fatalf("GraphQL historyKey list failed: %v", gqlList)
	}
	gqlHistoryWrite := graphqlDo(t, srv, historyKey, fmt.Sprintf(`mutation { updateSandboxGroupCredential(sandboxGroupId: %q, id: "gql-cred", ownerId: %q, input: { enabled: false }) { id } }`, group.ID, ws.ID))
	if got := graphqlCode(gqlHistoryWrite); got != core.KeyPermissionDeniedCode {
		t.Fatalf("GraphQL historyKey write code = %q (%v), want %s", got, gqlHistoryWrite, core.KeyPermissionDeniedCode)
	}
	gqlDevUpdate := graphqlDo(t, srv, devCredKey, fmt.Sprintf(`mutation { updateSandboxGroupCredential(sandboxGroupId: %q, id: "gql-cred", ownerId: %q, input: { enabled: false, methods: [] }) { id enabled methods version { id } } }`, group.ID, ws.ID))
	if code := graphqlCode(gqlDevUpdate); code != "" {
		t.Fatalf("GraphQL devCredKey allowed update failed: %v", gqlDevUpdate)
	}
	gqlDevDomain := graphqlDo(t, srv, devCredKey, fmt.Sprintf(`mutation { updateSandboxGroupCredential(sandboxGroupId: %q, id: "gql-cred", ownerId: %q, input: { domain: "api.github.com" }) { id } }`, group.ID, ws.ID))
	if errs, _ := gqlDevDomain["errors"].([]any); len(errs) == 0 {
		t.Fatalf("GraphQL devCredKey domain update succeeded, want forbidden: %v", gqlDevDomain)
	}

	// MCP surface with explicit workspaceId + scoped API keys + version restore + delete.
	mcpSecret := "sk-mcp-secret-v1-" + run
	mcpRes := mcpCall(t, srv, historyKey, "create_sandbox_group_credential", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"id":             "mcp-cred",
		"domain":         "api.openai.com",
		"value":          mcpSecret,
		"injection":      map[string]any{"type": "header", "name": "Authorization"},
	})
	if !mcpRes.IsError || !strings.Contains(mcpText(mcpRes), core.KeyPermissionDeniedCode) {
		t.Fatalf("MCP historyKey create = error=%v %q, want %s", mcpRes.IsError, mcpText(mcpRes), core.KeyPermissionDeniedCode)
	}
	mcpRes = mcpCall(t, srv, adminCredKey, "create_sandbox_group_credential", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"id":             "mcp-cred",
		"domain":         "api.openai.com",
		"value":          mcpSecret,
		"injection":      map[string]any{"type": "header", "name": "Authorization", "template": "Bearer ${SECRET}"},
	})
	if mcpRes.IsError || strings.Contains(mcpText(mcpRes), mcpSecret) {
		t.Fatalf("MCP admin create = error=%v %q", mcpRes.IsError, mcpText(mcpRes))
	}
	// MCP devCredKey can update enabled/paths, cannot rotate value; admin rotates then restores v1.
	mcpRes = mcpCall(t, srv, devCredKey, "update_sandbox_group_credential", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"credentialId":   "mcp-cred",
		"enabled":        false,
		"paths":          []string{"/v1/*"},
	})
	if mcpRes.IsError {
		t.Fatalf("MCP devCredKey allowed update = %q", mcpText(mcpRes))
	}
	mcpRes = mcpCall(t, srv, devCredKey, "update_sandbox_group_credential", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"credentialId":   "mcp-cred",
		"value":          gqlSecret2,
	})
	if !mcpRes.IsError {
		t.Fatalf("MCP devCredKey value update succeeded, want forbidden: %q", mcpText(mcpRes))
	}
	mcpRes = mcpCall(t, srv, adminCredKey, "restore_sandbox_group_credential", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"credentialId":   "mcp-cred",
		"versionId":      1,
	})
	if mcpRes.IsError || !strings.Contains(mcpText(mcpRes), `"restoredFromVersionId":1`) || strings.Contains(mcpText(mcpRes), mcpSecret) {
		t.Fatalf("MCP restore = error=%v %q", mcpRes.IsError, mcpText(mcpRes))
	}
	mcpVers := mcpCall(t, srv, historyKey, "list_sandbox_group_credential_versions", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"credentialId":   "mcp-cred",
	})
	if mcpVers.IsError || strings.Contains(mcpText(mcpVers), mcpSecret) {
		t.Fatalf("MCP versions = error=%v %q", mcpVers.IsError, mcpText(mcpVers))
	}
	// Clean up both credentials via GraphQL & MCP delete and confirm OpenBao is empty.
	gqlDel := graphqlDo(t, srv, adminCredKey, fmt.Sprintf(`mutation { deleteSandboxGroupCredential(sandboxGroupId: %q, id: "gql-cred", ownerId: %q) }`, group.ID, ws.ID))
	if code := graphqlCode(gqlDel); code != "" {
		t.Fatalf("GraphQL delete failed: %v", gqlDel)
	}
	mcpDel := mcpCall(t, srv, adminCredKey, "delete_sandbox_group_credential", map[string]any{
		"workspaceId":    ws.ID,
		"sandboxGroupId": group.ID,
		"credentialId":   "mcp-cred",
	})
	if mcpDel.IsError {
		t.Fatalf("MCP delete failed: %q", mcpText(mcpDel))
	}
	if keysLeft, err := baoStore.List(secrets.WithTenant(ctx, ws.ID), "sandbox-groups/"+group.ID+"/credentials"); err != nil || len(keysLeft) != 0 {
		t.Fatalf("OpenBao still holds credentials after GQL/MCP delete: keys=%v err=%v", keysLeft, err)
	}

	// 8c. Demoted Admin & narrowed/revoked API key enforcement (ADR123 attack class 8).
	admin2 := "sbcred-admin2-" + run
	if err := st.AddMember(ctx, admin2, ws.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	seat(ws.ID, admin2, "admin")
	admin2KeyRec := call(session(admin2), http.MethodPost, "/v1/api-keys",
		fmt.Sprintf(`{"name":"adm2cred-%s","ownerId":%q,"permissions":{"grants":[{"permission":"sandboxes","level":"read"},{"permission":"sandbox_credentials","level":"write"}]}}`, run, ws.ID))
	if admin2KeyRec.Code != http.StatusCreated {
		t.Fatalf("mint admin2 key = %d %s", admin2KeyRec.Code, admin2KeyRec.Body.String())
	}
	var admin2KeyMeta apikeys.APIKey
	if err := json.Unmarshal(admin2KeyRec.Body.Bytes(), &admin2KeyMeta); err != nil {
		t.Fatal(err)
	}
	admin2Key := keyOf(admin2KeyMeta.ID)
	demoteCredPath := basePath + "/demote-check"
	if rec := call(admin2Key, http.MethodPost, basePath,
		`{"id":"demote-check","domain":"api.openai.com","value":"sk-demote-v1","injection":{"type":"header","name":"Authorization"}}`); rec.Code != http.StatusCreated {
		t.Fatalf("admin2Key create before demotion = %d %s, want 201", rec.Code, rec.Body.String())
	}
	// Narrow key permissions to sandboxes:read only -> write refused with KeyPermissionDeniedCode.
	if rec := call(session(admin2), http.MethodPatch, "/v1/api-keys/"+admin2KeyMeta.ID+"?ownerId="+ws.ID,
		`{"permissions":{"grants":[{"permission":"sandboxes","level":"read"}]}}`); rec.Code != http.StatusOK {
		t.Fatalf("narrow admin2 key permissions = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := call(admin2Key, http.MethodPatch, demoteCredPath, `{"enabled":false}`); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), core.KeyPermissionDeniedCode) {
		t.Fatalf("narrowed admin2Key PATCH = %d %s, want 403 %s", rec.Code, rec.Body.String(), core.KeyPermissionDeniedCode)
	}
	// Restore write permission, then demote creator admin2 -> developer in OpenFGA.
	if rec := call(session(admin2), http.MethodPatch, "/v1/api-keys/"+admin2KeyMeta.ID+"?ownerId="+ws.ID,
		`{"permissions":{"grants":[{"permission":"sandboxes","level":"read"},{"permission":"sandbox_credentials","level":"write"}]}}`); rec.Code != http.StatusOK {
		t.Fatalf("restore admin2 key permissions = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if err := checker.(members.RoleRevoker).RevokeWorkspaceMember(ctx, ws.ID, "user:"+admin2, "admin"); err != nil {
		t.Fatalf("revoke admin2 admin role: %v", err)
	}
	seat(ws.ID, admin2, "developer")
	// Creator ceiling immediately blocks Admin-only verbs (create, delete, restore, domain/value PATCH)
	// while still permitting Developer PATCH (enabled).
	if rec := call(admin2Key, http.MethodPatch, demoteCredPath, `{"enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("demoted admin2Key enabled PATCH = %d %s, want 200", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		name, method, path, body string
	}{
		{"create", http.MethodPost, basePath, `{"id":"demote-blocked","domain":"api.openai.com","value":"sk-x","injection":{"type":"header","name":"Authorization"}}`},
		{"patch-domain", http.MethodPatch, demoteCredPath, `{"domain":"api.github.com"}`},
		{"patch-value", http.MethodPatch, demoteCredPath, `{"value":"sk-demote-v2"}`},
		{"restore", http.MethodPost, demoteCredPath + "/restore", `{"versionId":1}`},
		{"delete", http.MethodDelete, demoteCredPath, ""},
	} {
		if rec := call(admin2Key, tc.method, tc.path, tc.body); rec.Code != http.StatusForbidden {
			t.Errorf("demoted admin2Key %s = %d %s, want 403", tc.name, rec.Code, rec.Body.String())
		}
	}
	if rec := call(session(admin), http.MethodDelete, demoteCredPath, ""); rec.Code != http.StatusAccepted {
		t.Fatalf("cleanup demote-check = %d %s, want 202", rec.Code, rec.Body.String())
	}

	// 9. Sandbox group delete purges credential secrets from OpenBao.
	rec = call(session(admin), http.MethodPost, "/v1/sandbox-groups/"+secondGroup.ID+"/credentials",
		`{"id":"group-del-key","domain":"api.stripe.com","value":"sk-group-del","injection":{"type":"header","name":"Authorization"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create in secondGroup = %d %s", rec.Code, rec.Body.String())
	}
	if err := srv.Sandbox.DeleteGroup(core.WithIdentity(ctx, session(admin)), secondGroup.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if keysLeft, err := baoStore.List(secrets.WithTenant(ctx, ws.ID), "sandbox-groups/"+secondGroup.ID+"/credentials"); err != nil || len(keysLeft) != 0 {
		t.Fatalf("OpenBao still holds secrets after group delete: keys=%v err=%v", keysLeft, err)
	}

	// 10. Workspace teardown purges remaining credential secrets from OpenBao.
	rec = call(session(admin), http.MethodPost, basePath,
		`{"id":"ws-del-key","domain":"api.github.com","value":"ghp-ws-del","injection":{"type":"header","name":"Authorization"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create for workspace delete = %d %s", rec.Code, rec.Body.String())
	}
	purger := &sandbox.WorkspacePurger{Credentials: st, CredentialVault: vault}
	if err := purger.PurgeWorkspace(ctx, ws.ID); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}
	if keysLeft, err := baoStore.List(secrets.WithTenant(ctx, ws.ID), "sandbox-groups"); err != nil || len(keysLeft) != 0 {
		t.Fatalf("OpenBao still holds secrets after workspace purge: keys=%v err=%v", keysLeft, err)
	}

	// 11. Audit events never contain any secret value.
	auditBytes, _ := json.Marshal(auditSink.events)
	for _, sec := range []string{secret1, gqlSecret1, gqlSecret2, mcpSecret, "sk-group-del", "ghp-ws-del", "sk-dev-attempt", "sk-demote-v1", "sk-demote-v2"} {
		if strings.Contains(string(auditBytes), sec) {
			t.Fatalf("audit log leaked secret %q", sec)
		}
	}

	// 12. Internal broker lookup route (w2/m534) is NEVER mounted on the public :8090 API.
	rec = call(session(admin), http.MethodPost, sandbox.InternalBrokerLookupPath, `{}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public :8090 POST %s = %d, want 404", sandbox.InternalBrokerLookupPath, rec.Code)
	}
}
