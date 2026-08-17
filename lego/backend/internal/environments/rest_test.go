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

package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestToRenderEnvironmentUsesOfficialCLIFields(t *testing.T) {
	got := toRenderEnvironment(EnvironmentView{
		ID: "env-1", ProjectID: "prj-1", Name: "staging", ServiceIDs: []string{"web"},
		DatabaseIDs: []string{"db"}, KeyValueIDs: []string{"kv"}, IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}},
	})
	if got.ID != "env-1" || len(got.ServiceIDs) != 1 || len(got.DatabasesIDs) != 1 || len(got.RedisIDs) != 1 {
		t.Fatalf("Render environment = %+v", got)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "10.0.0.0/8" {
		t.Fatalf("Render ipAllowList = %+v", got.IPAllowList)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{"ownerId", "createdAt", "updatedAt"} {
		if _, ok := fields[extension]; ok {
			t.Errorf("Render environment unexpectedly exposes bex extension %q: %s", extension, raw)
		}
	}
	if environmentGQLType.Fields()["ownerId"] == nil || environmentGQLType.Fields()["createdAt"] == nil {
		t.Fatal("GraphQL bex extensions ownerId/createdAt disappeared")
	}
}

// restHarness stands up a Service (fake store, fake k8s client) and its REST
// mux, pre-seeded with project prj-1 — the w4/017 create/update tests below
// all start from here.
func restHarness(t *testing.T) (*Service, *http.ServeMux) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return svc, mux
}

// newMCPClient stands up svc's MCP server over an in-memory transport and
// returns a connected client session, closed automatically at test cleanup —
// the connect-server/connect-client/register-cleanup boilerplate every MCP
// subtest below otherwise repeats.
func newMCPClient(t *testing.T, ctx context.Context, svc *Service) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func doREST(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	return rec
}

// TestREST_CreateAcceptsRenderACLObjects is w4/017: Render's create body
// carries the ACL triple with ipAllowList as [{cidrBlock, description}]
// objects — bex accepts the object form on the standard POST (description
// discarded — the apps/postgres/keyvalue convention), not just string CIDRs
// through the bex-only /acl route.
func TestREST_CreateAcceptsRenderACLObjects(t *testing.T) {
	_, mux := restHarness(t)
	rec := doREST(t, mux, "POST", "/v1/environments", `{
		"name": "staging", "projectId": "prj-1",
		"protectedStatus": "protected",
		"networkIsolationEnabled": true,
		"ipAllowList": [{"cidrBlock": "10.0.0.0/8", "description": "office"}]
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var got renderEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ProtectedStatus != "protected" || !got.NetworkIsolationEnabled {
		t.Errorf("created ACL = %q/%v, want protected/true", got.ProtectedStatus, got.NetworkIsolationEnabled)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "10.0.0.0/8" {
		t.Errorf("created ipAllowList = %+v, want the object-form CIDR echoed", got.IPAllowList)
	}
}

// TestREST_CreateRejectsBadACLWithoutOrphan: a bad CIDR (or protectedStatus)
// in the create body is a clean 400 and the environment must NOT have been
// created — the ACL is validated before the row exists.
func TestREST_CreateRejectsBadACLWithoutOrphan(t *testing.T) {
	svc, mux := restHarness(t)
	rec := doREST(t, mux, "POST", "/v1/environments", `{
		"name": "staging", "projectId": "prj-1",
		"ipAllowList": [{"cidrBlock": "not-a-cidr", "description": ""}]
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST with bad CIDR = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	list, err := svc.List(ctxAs("user-a"), "prj-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("environment created despite the 400: %+v", list)
	}
}

func TestCreateWithACLAcrossRESTGraphQLAndMCP(t *testing.T) {
	assertACL := func(t *testing.T, got EnvironmentView) {
		t.Helper()
		if got.ProtectedStatus != ProtectedStatusProtected || !got.NetworkIsolationEnabled ||
			len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "10.0.0.0/8" || got.IPAllowList[0].Description != "office" {
			t.Fatalf("created ACL = %+v", got)
		}
	}

	t.Run("REST", func(t *testing.T) {
		_, mux := restHarness(t)
		rec := doREST(t, mux, http.MethodPost, "/v1/environments", `{
			"name":"rest","projectId":"prj-1","protectedStatus":"protected",
			"networkIsolationEnabled":true,
			"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"office"}]
		}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("REST create = %d: %s", rec.Code, rec.Body.String())
		}
		var wire renderEnvironment
		if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}
		assertACL(t, EnvironmentView{ProtectedStatus: wire.ProtectedStatus, NetworkIsolationEnabled: wire.NetworkIsolationEnabled, IPAllowList: wire.IPAllowList})
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, _ := restHarness(t)
		field := svc.GraphQLMutation()["createEnvironment"]
		out, err := field.Resolve(graphql.ResolveParams{Context: ctxAs("user-a"), Args: map[string]any{
			"name": "graphql", "projectId": "prj-1", "protectedStatus": "protected",
			"networkIsolationEnabled": true,
			"ipAllowList":             []any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "office"}},
		}})
		if err != nil {
			t.Fatalf("GraphQL create: %v", err)
		}
		assertACL(t, out.(EnvironmentView))
	})

	t.Run("MCP", func(t *testing.T) {
		svc, _ := restHarness(t)
		ctx := ctxAs("user-a")
		client := newMCPClient(t, ctx, svc)
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "create_environment", Arguments: map[string]any{
			"name": "mcp", "projectId": "prj-1", "protectedStatus": "protected",
			"networkIsolationEnabled": true,
			"ipAllowList":             []any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "office"}},
		}})
		if err != nil || res.IsError {
			t.Fatalf("MCP create: err=%v result=%+v", err, res)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var got EnvironmentView
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		assertACL(t, got)
	})
}

func TestCreateWithACLInvalidCIDRLeavesNoOrphanAcrossMachineSurfaces(t *testing.T) {
	t.Run("GraphQL", func(t *testing.T) {
		svc, _ := restHarness(t)
		field := svc.GraphQLMutation()["createEnvironment"]
		_, err := field.Resolve(graphql.ResolveParams{Context: ctxAs("user-a"), Args: map[string]any{
			"name": "bad", "projectId": "prj-1",
			"ipAllowList": []any{map[string]any{"cidrBlock": "not-a-cidr"}},
		}})
		if err == nil {
			t.Fatal("GraphQL invalid CIDR succeeded")
		}
		if got, listErr := svc.List(ctxAs("user-a"), "prj-1"); listErr != nil || len(got) != 0 {
			t.Fatalf("GraphQL invalid create left orphan: %+v err=%v", got, listErr)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		svc, _ := restHarness(t)
		ctx := ctxAs("user-a")
		client := newMCPClient(t, ctx, svc)
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "create_environment", Arguments: map[string]any{
			"name": "bad", "projectId": "prj-1",
			"ipAllowList": []any{map[string]any{"cidrBlock": "not-a-cidr"}},
		}})
		if err == nil && !res.IsError {
			t.Fatal("MCP invalid CIDR succeeded")
		}
		if got, listErr := svc.List(ctxAs("user-a"), "prj-1"); listErr != nil || len(got) != 0 {
			t.Fatalf("MCP invalid create left orphan: %+v err=%v", got, listErr)
		}
	})
}

func TestREST_ListEnvironmentDocumentedFiltersHonoredOrRejected(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "alpha-project"})
	st.addProject(store.Project{ID: "prj-2", TenantID: "tea-b", Name: "bravo-project"})
	base := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	for _, environment := range []store.Environment{
		{ID: "env-alpha", ProjectID: "prj-1", TenantID: "tea-a", Name: "alpha", CreatedAt: base.Add(-2 * time.Hour), ProtectedStatus: ProtectedStatusUnprotected},
		{ID: "env-bravo", ProjectID: "prj-1", TenantID: "tea-a", Name: "bravo", CreatedAt: base.Add(-time.Hour), ProtectedStatus: ProtectedStatusUnprotected},
		{ID: "env-charlie", ProjectID: "prj-2", TenantID: "tea-b", Name: "charlie", CreatedAt: base, ProtectedStatus: ProtectedStatusUnprotected},
	} {
		st.envs[environment.ID] = environment
	}
	svc, _ := newServiceWithClient(st)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	tests := []struct {
		name      string
		query     string
		wantNames []string
		wantCode  int
		wantError string
	}{
		{name: "projectId", query: "projectId=prj-1", wantNames: []string{"alpha", "bravo"}},
		// codex-security round 12, finding 5: duplicate ids dedupe to one list
		// run (repeat query keys must not multiply the fan-out).
		{name: "duplicate projectIds dedupe", query: "projectId=prj-1,prj-1&projectId=prj-1", wantNames: []string{"alpha", "bravo"}},
		{name: "name", query: "projectId=prj-1,prj-2&name=bravo", wantNames: []string{"bravo"}},
		{name: "repeated names", query: "projectId=prj-1,prj-2&name=alpha&name=charlie", wantNames: []string{"alpha", "charlie"}},
		{name: "environmentId", query: "projectId=prj-1,prj-2&environmentId=env-charlie", wantNames: []string{"charlie"}},
		{name: "ownerId", query: "projectId=prj-1,prj-2&ownerId=tea-b", wantNames: []string{"charlie"}},
		{name: "createdBefore", query: "projectId=prj-1,prj-2&createdBefore=2026-07-15T11:30:00Z", wantNames: []string{"alpha", "bravo"}},
		{name: "createdAfter", query: "projectId=prj-1,prj-2&createdAfter=2026-07-15T10:30:00Z", wantNames: []string{"bravo", "charlie"}},
		{name: "updatedBefore", query: "projectId=prj-1&updatedBefore=2026-07-15T12:00:00Z", wantCode: http.StatusBadRequest, wantError: "updatedBefore"},
		{name: "updatedAfter", query: "projectId=prj-1&updatedAfter=2026-07-15T12:00:00Z", wantCode: http.StatusBadRequest, wantError: "updatedAfter"},
		{name: "invalid created timestamp", query: "projectId=prj-1&createdBefore=yesterday", wantCode: http.StatusBadRequest, wantError: "createdBefore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doREST(t, mux, http.MethodGet, "/v1/environments?limit=100&"+tt.query, "")
			wantCode := tt.wantCode
			if wantCode == 0 {
				wantCode = http.StatusOK
			}
			if rec.Code != wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, wantCode, rec.Body.String())
			}
			if wantCode != http.StatusOK {
				if !strings.Contains(rec.Body.String(), tt.wantError) {
					t.Fatalf("error %q does not name %q", rec.Body.String(), tt.wantError)
				}
				return
			}
			var page []environmentWithCursor
			if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(page))
			for i := range page {
				got[i] = page[i].Environment.Name
			}
			if len(got) != len(tt.wantNames) {
				t.Fatalf("names = %v, want %v", got, tt.wantNames)
			}
			for _, name := range tt.wantNames {
				if !slices.Contains(got, name) {
					t.Fatalf("names = %v, missing %q", got, name)
				}
			}
		})
	}
}

// TestREST_ListEnvironmentProjectIDFanOutCap pins codex-security round 12,
// finding 5: one list request may fan out into at most maxListProjectIDs
// distinct projectId values (each runs a full list + enrichment pass); past the
// cap it is a clean 400 before the first list runs.
func TestREST_ListEnvironmentProjectIDFanOutCap(t *testing.T) {
	_, mux := restHarness(t)
	ids := make([]string, maxListProjectIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("prj-%02d", i)
	}
	rec := doREST(t, mux, http.MethodGet, "/v1/environments?projectId="+strings.Join(ids, ","), "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "projectId") {
		t.Fatalf("over-cap fan-out = %d %s, want 400 naming projectId", rec.Code, rec.Body.String())
	}
	atCap := strings.Join(ids[:maxListProjectIDs], ",")
	rec = doREST(t, mux, http.MethodGet, "/v1/environments?projectId="+atCap, "")
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("at-cap fan-out must not be a 400: %s", rec.Body.String())
	}
}

// TestREST_PatchMergesRenderACLFields: Render's PATCH is per-field partial —
// naming only ipAllowList must keep the current protectedStatus/
// networkIsolationEnabled (SetACL itself is full-replace, so the handler
// merges); naming only name must not touch the ACL; an empty body is 400.
func TestREST_PatchMergesRenderACLFields(t *testing.T) {
	svc, mux := restHarness(t)
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusProtected, true, []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	// Only ipAllowList: protectedStatus/networkIsolationEnabled survive.
	rec := doREST(t, mux, "PATCH", "/v1/environments/"+e.ID,
		`{"ipAllowList": [{"cidrBlock": "192.168.0.0/16", "description": "vpn"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH ipAllowList = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var got renderEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ProtectedStatus != ProtectedStatusProtected || !got.NetworkIsolationEnabled {
		t.Errorf("ACL siblings not preserved: %q/%v, want protected/true", got.ProtectedStatus, got.NetworkIsolationEnabled)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "192.168.0.0/16" {
		t.Errorf("ipAllowList = %+v, want the replaced CIDR", got.IPAllowList)
	}

	// networkIsolationEnabled:false must be appliable (absent != false).
	rec = doREST(t, mux, "PATCH", "/v1/environments/"+e.ID, `{"networkIsolationEnabled": false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH networkIsolationEnabled=false = %d (body: %s)", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.NetworkIsolationEnabled || got.ProtectedStatus != ProtectedStatusProtected || len(got.IPAllowList) != 1 {
		t.Errorf("after isolation-off PATCH: %+v, want only that field changed", got)
	}

	// Only name: rename, ACL untouched.
	rec = doREST(t, mux, "PATCH", "/v1/environments/"+e.ID, `{"name": "production"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH name = %d (body: %s)", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "production" || got.ProtectedStatus != ProtectedStatusProtected {
		t.Errorf("rename-only PATCH: %+v, want name changed and ACL untouched", got)
	}

	// Empty body names nothing: 400, same as before w4/017.
	if rec := doREST(t, mux, "PATCH", "/v1/environments/"+e.ID, `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH {} = %d, want 400", rec.Code)
	}
}

// TestUpdateAcrossRESTGraphQLAndMCPProduceIdenticalMerges is w4/m30/t006: REST
// PATCH, GraphQL updateEnvironment, and MCP update_environment all ride the
// one core Update verb, so the same patch sequence — an ACL-only merge, an
// explicit zero-value bool (networkIsolationEnabled:false, absent != false),
// then a name-only rename — must land on byte-identical final state across
// all three, and the pre-migration empty-protectedStatus default (w4/017)
// must resolve the same way everywhere too.
// newProtectedBaselineEnv creates an environment with a protected/isolated
// ACL baseline (protectedStatus, networkIsolationEnabled, and one CIDR all
// set) — the starting point every subtest of
// TestUpdateAcrossRESTGraphQLAndMCPProduceIdenticalMerges patches from, to
// prove the later merges preserve untouched siblings rather than resetting
// them.
func newProtectedBaselineEnv(t *testing.T) (*Service, *http.ServeMux, string) {
	t.Helper()
	svc, mux := restHarness(t)
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusProtected, true, []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	return svc, mux, e.ID
}

func TestUpdateAcrossRESTGraphQLAndMCPProduceIdenticalMerges(t *testing.T) {
	assertMerged := func(t *testing.T, got EnvironmentView) {
		t.Helper()
		if got.Name != "production" {
			t.Errorf("Name = %q, want production", got.Name)
		}
		if got.ProtectedStatus != ProtectedStatusProtected {
			t.Errorf("ProtectedStatus = %q, want protected (untouched by the later patches)", got.ProtectedStatus)
		}
		if got.NetworkIsolationEnabled {
			t.Errorf("NetworkIsolationEnabled = true, want false (the explicit zero-value patch)")
		}
		if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "192.168.0.0/16" || got.IPAllowList[0].Description != "vpn" {
			t.Errorf("IPAllowList = %+v, want the merged replacement CIDR", got.IPAllowList)
		}
	}

	t.Run("REST", func(t *testing.T) {
		_, mux, id := newProtectedBaselineEnv(t)
		var got renderEnvironment
		for _, body := range []string{
			`{"ipAllowList":[{"cidrBlock":"192.168.0.0/16","description":"vpn"}]}`,
			`{"networkIsolationEnabled":false}`,
			`{"name":"production"}`,
		} {
			rec := doREST(t, mux, "PATCH", "/v1/environments/"+id, body)
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH %s = %d: %s", body, rec.Code, rec.Body.String())
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
		}
		assertMerged(t, EnvironmentView{Name: got.Name, ProtectedStatus: got.ProtectedStatus, NetworkIsolationEnabled: got.NetworkIsolationEnabled, IPAllowList: got.IPAllowList})
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, _, id := newProtectedBaselineEnv(t)
		field := svc.GraphQLMutation()["updateEnvironment"]
		var got EnvironmentView
		for _, args := range []map[string]any{
			{"id": id, "ipAllowListEntries": []any{map[string]any{"cidrBlock": "192.168.0.0/16", "description": "vpn"}}},
			{"id": id, "networkIsolationEnabled": false},
			{"id": id, "name": "production"},
		} {
			out, err := field.Resolve(graphql.ResolveParams{Context: ctxAs("user-a"), Args: args})
			if err != nil {
				t.Fatalf("updateEnvironment(%+v): %v", args, err)
			}
			got = out.(EnvironmentView)
		}
		assertMerged(t, got)
	})

	t.Run("MCP", func(t *testing.T) {
		svc, _, id := newProtectedBaselineEnv(t)
		ctx := ctxAs("user-a")
		client := newMCPClient(t, ctx, svc)
		var got EnvironmentView
		for _, args := range []map[string]any{
			{"id": id, "ipAllowList": []any{map[string]any{"cidrBlock": "192.168.0.0/16", "description": "vpn"}}},
			{"id": id, "networkIsolationEnabled": false},
			{"id": id, "name": "production"},
		} {
			res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_environment", Arguments: args})
			if err != nil || res.IsError {
				t.Fatalf("MCP update_environment(%+v): err=%v result=%+v", args, err, res)
			}
			raw, _ := json.Marshal(res.StructuredContent)
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
		}
		assertMerged(t, got)
	})
}

// TestUpdatePreMigrationEmptyProtectedStatusDefaultsAcrossSurfaces is w4/017's
// pre-ACL-migration case (a row created before the ACL columns existed
// surfaces protectedStatus ""): an ACL-only patch through Update must default
// it to unprotected rather than persisting the empty string, identically on
// every surface — mirroring REST's former inline check, now owned once by
// core Update.
func TestUpdatePreMigrationEmptyProtectedStatusDefaultsAcrossSurfaces(t *testing.T) {
	newPreMigrationEnv := func(t *testing.T) (*Service, *http.ServeMux, store.Environment) {
		t.Helper()
		st := newFakeStore()
		st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
		svc, _ := newServiceWithClient(st)
		// fakeStore.CreateEnvironment seeds ProtectedStatus: unprotected (the
		// post-migration default) — a real pre-ACL-migration row is inserted
		// directly, ProtectedStatus intentionally left "" (empty column).
		row := store.Environment{ID: id.New(id.Environment), ProjectID: "prj-1", TenantID: "tea-a", Name: "staging"}
		st.envs[row.ID] = row
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		return svc, mux, row
	}

	t.Run("REST", func(t *testing.T) {
		_, mux, row := newPreMigrationEnv(t)
		rec := doREST(t, mux, "PATCH", "/v1/environments/"+row.ID, `{"networkIsolationEnabled":true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body.String())
		}
		var got renderEnvironment
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ProtectedStatus != ProtectedStatusUnprotected {
			t.Errorf("ProtectedStatus = %q, want the unprotected default", got.ProtectedStatus)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, _, row := newPreMigrationEnv(t)
		field := svc.GraphQLMutation()["updateEnvironment"]
		out, err := field.Resolve(graphql.ResolveParams{Context: ctxAs("user-a"), Args: map[string]any{
			"id": row.ID, "networkIsolationEnabled": true,
		}})
		if err != nil {
			t.Fatalf("updateEnvironment: %v", err)
		}
		if got := out.(EnvironmentView); got.ProtectedStatus != ProtectedStatusUnprotected {
			t.Errorf("ProtectedStatus = %q, want the unprotected default", got.ProtectedStatus)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		svc, _, row := newPreMigrationEnv(t)
		ctx := ctxAs("user-a")
		client := newMCPClient(t, ctx, svc)
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_environment", Arguments: map[string]any{
			"id": row.ID, "networkIsolationEnabled": true,
		}})
		if err != nil || res.IsError {
			t.Fatalf("MCP update_environment: err=%v result=%+v", err, res)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var got EnvironmentView
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.ProtectedStatus != ProtectedStatusUnprotected {
			t.Errorf("ProtectedStatus = %q, want the unprotected default", got.ProtectedStatus)
		}
	})
}
