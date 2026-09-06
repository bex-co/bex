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

package projects

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// renderProjectKeys is Render's exact `project` object key set — six required
// properties and no others (docs/render-artifacts project schema). Every project
// handler, read AND write, must emit exactly this (w6/m126).
var renderProjectKeys = []string{"createdAt", "environmentIds", "id", "name", "owner", "updatedAt"}

// internalOnlyProjectKeys are the ProjectView fields Render's project object has
// no place for; their presence on any response is the w6/m126 defect (the write
// paths used to emit the internal view instead of Render's shape).
var internalOnlyProjectKeys = []string{"databaseIds", "keyValueIds", "ownerId", "serviceIds"}

// envListerStore is fakeProjectStore plus the optional ListEnvironments
// capability renderProject reads to populate Render's required environmentIds —
// so these tests prove the WRITE paths resolve environment membership (the
// ctx+error step core's context-free view hooks could not express), not merely
// that they emit an empty array.
type envListerStore struct {
	*fakeProjectStore
	envs map[string][]store.Environment
}

func (e envListerStore) ListEnvironments(_ context.Context, projectID string) ([]store.Environment, error) {
	return e.envs[projectID], nil
}

// projectResponseKeys serves one request against mux and returns the sorted
// top-level JSON object keys of the (2xx) response body.
func projectResponseKeys(t *testing.T, mux *http.ServeMux, method, path, body string) []string {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s = %d, want 2xx: %s", method, path, rec.Code, rec.Body.String())
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
		t.Fatalf("%s %s: decode object: %v (body %s)", method, path, err, rec.Body.String())
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// TestProjectWritePathsEmitRenderShape is w6/m126's core guard: every project
// handler that returns a project — the two reads AND the five writes (POST,
// PATCH, and the three link PUTs) — emits Render's `project` shape, identical
// key set, containing none of the internal ProjectView fields. Reverting any one
// write handler to return the raw ProjectView turns this red.
func TestProjectWritePathsEmitRenderShape(t *testing.T) {
	fake := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-1", Name: "platform"})
	st := envListerStore{fakeProjectStore: fake, envs: map[string][]store.Environment{
		"prj-1": {{ID: "env-1", ProjectID: "prj-1", TenantID: "tea-1", Name: "production"}},
	}}
	svc := &Service{
		Base:      &core.Base{Authz: allowChecker{}},
		Store:     st,
		Databases: newFakeResourceIndex("tea-1"),
		KeyValues: newFakeResourceIndex("tea-1"),
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// The read shape is the reference every write must match, and it is Render's.
	readKeys := projectResponseKeys(t, mux, http.MethodGet, "/v1/projects/prj-1", "")
	if !slices.Equal(readKeys, renderProjectKeys) {
		t.Fatalf("GET /v1/projects/{id} keys = %v, want Render's %v", readKeys, renderProjectKeys)
	}

	writes := []struct{ name, method, path, body string }{
		{"create", http.MethodPost, "/v1/projects", `{"name":"api","ownerId":"tea-1"}`},
		{"rename", http.MethodPatch, "/v1/projects/prj-1", `{"name":"renamed"}`},
		{"service-links", http.MethodPut, "/v1/projects/prj-1/service-links", `{"serviceIds":[]}`},
		{"database-links", http.MethodPut, "/v1/projects/prj-1/database-links", `{"databaseIds":[]}`},
		{"keyvalue-links", http.MethodPut, "/v1/projects/prj-1/keyvalue-links", `{"keyValueIds":[]}`},
	}
	for _, w := range writes {
		t.Run(w.name, func(t *testing.T) {
			keys := projectResponseKeys(t, mux, w.method, w.path, w.body)
			if !slices.Equal(keys, readKeys) {
				t.Fatalf("%s keys = %v, want read's %v — a write path is not emitting Render's project shape", w.name, keys, readKeys)
			}
			for _, forbidden := range internalOnlyProjectKeys {
				if slices.Contains(keys, forbidden) {
					t.Fatalf("%s response carries internal-view key %q — Render's project object has no such field", w.name, forbidden)
				}
			}
		})
	}
}

// TestWritePathsResolveEnvironmentMembership pins the reason the fix could not
// reuse core's context-free view hooks: renderProject reads environment
// membership, so a write handler must resolve it too. A rename of a project that
// owns env-1 must return environmentIds:["env-1"], exactly as a read would.
func TestWritePathsResolveEnvironmentMembership(t *testing.T) {
	fake := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-1", Name: "platform"})
	st := envListerStore{fakeProjectStore: fake, envs: map[string][]store.Environment{
		"prj-1": {{ID: "env-1", ProjectID: "prj-1", TenantID: "tea-1", Name: "production"}},
	}}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/projects/prj-1", strings.NewReader(`{"name":"renamed"}`))
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body.String())
	}
	var got renderProject
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !slices.Equal(got.EnvironmentIDs, []string{"env-1"}) {
		t.Fatalf("rename environmentIds = %v, want [env-1] — the write path must resolve environment membership like a read", got.EnvironmentIDs)
	}
	if got.Owner.ID != "tea-1" || got.Owner.Type != "team" || got.Name != "renamed" {
		t.Fatalf("rename response = %+v, want Render shape with owner tea-1 and new name", got)
	}
}

// TestCreateAcceptsRenderEnvironmentsInputUnderStrictDecode verifies real persisted environments and ACLs.
func TestCreateAcceptsRenderEnvironmentsInputUnderStrictDecode(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := ctxAs("user-a")
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	workspace, err := st.CreateWorkspace(ctx, "project-inline", store.PlanHobby, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", workspace.ID) }()
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"api","ownerId":"` + workspace.ID + `","environments":[` +
		`{"name":"production","protectedStatus":"protected","networkIsolationEnabled":true,` +
		`"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"office"}]},{"name":"staging"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(body))
	// The composed server sets this on every Render-contract route; setting it
	// here exercises the exact strict decoder that route runs, without the full
	// request gate.
	mux.ServeHTTP(rec, req.WithContext(core.WithStrictJSONDecoding(ctxAs("user-a"))))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST with Render environments input = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var got renderProject
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.EnvironmentIDs) != 2 {
		t.Fatalf("environmentIds = %v, want two persisted environments", got.EnvironmentIDs)
	}
	envs, err := st.ListEnvironments(ctx, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 {
		t.Fatalf("persisted environments = %+v", envs)
	}
	byName := map[string]store.Environment{}
	for _, e := range envs {
		if !slices.Contains(got.EnvironmentIDs, e.ID) {
			t.Fatalf("response omitted environment %s", e.ID)
		}
		byName[e.Name] = e
	}
	production := byName["production"]
	if production.ProtectedStatus != core.ProtectedStatusProtected || !production.NetworkIsolationEnabled || !slices.Equal(production.IPAllowList, []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}}) {
		t.Fatalf("production ACL = %+v", production)
	}
	staging := byName["staging"]
	if staging.ID == "" || staging.ProtectedStatus != core.ProtectedStatusUnprotected || !slices.Equal(staging.IPAllowList, core.DefaultEnvironmentAllowList()) {
		t.Fatalf("staging defaults = %+v", staging)
	}
	// A duplicate name fails after the project and first environment insert.
	// The transaction must roll all of them back.
	_, err = svc.CreateWithEnvironments(ctx, workspace.ID, "rollback", []EnvironmentInput{{Name: "same"}, {Name: "same"}})
	if err == nil {
		t.Fatal("duplicate environment names succeeded")
	}
	projects, err := st.ListProjects(ctx, workspace.ID)
	if err != nil || len(projects) != 1 {
		t.Fatalf("failed create leaked project: %+v, %v", projects, err)
	}
	svc.MaxGroupings = 2
	_, err = svc.CreateWithEnvironments(ctx, workspace.ID, "overquota", []EnvironmentInput{{Name: "a"}, {Name: "b"}})
	if err == nil {
		t.Fatal("environment quota was not enforced")
	}
	projects, err = st.ListProjects(ctx, workspace.ID)
	if err != nil || len(projects) != 1 {
		t.Fatalf("quota rejection leaked project: %+v, %v", projects, err)
	}

	// The strict decoder must still be active: a genuinely unknown field is
	// rejected. t002 widened the accepted schema deliberately, it did not disable
	// strictness.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(`{"name":"api2","ownerId":"tea-1","bogus":true}`))
	mux.ServeHTTP(rec2, req2.WithContext(core.WithStrictJSONDecoding(ctxAs("user-a"))))
	if !strings.Contains(rec2.Body.String(), "bogus") {
		t.Fatalf("unknown field not named: %s", rec2.Body.String())
	}
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("POST with unknown field = %d, want 400 — strict decoding must stay active", rec2.Code)
	}
}
