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
	"slices"
	"strings"
	"testing"

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

// TestCreateAcceptsRenderEnvironmentsInputUnderStrictDecode is w6/m126 t002:
// Render's projectPOSTInput requires an `environments` array, and the Render
// intersection routes strict-decode (reject unknown fields). The POST body must
// therefore ACCEPT the full Render create shape — including the nested
// environment objects and their ACL triple — so a client that mints its types
// from Render's schema (the official CLI) is not 400'd. bex records the
// divergence: it does not provision the environments, so the response's
// environmentIds truthfully reports [] rather than faking the requested one.
func TestCreateAcceptsRenderEnvironmentsInputUnderStrictDecode(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore()}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"api","ownerId":"tea-1","environments":[` +
		`{"name":"production","protectedStatus":"unprotected","networkIsolationEnabled":false,` +
		`"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"office"}]}]}`
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
	if len(got.EnvironmentIDs) != 0 {
		t.Fatalf("environmentIds = %v, want [] — bex does not provision POST environments (t002 divergence)", got.EnvironmentIDs)
	}

	// The strict decoder must still be active: a genuinely unknown field is
	// rejected. t002 widened the accepted schema deliberately, it did not disable
	// strictness.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(`{"name":"api2","ownerId":"tea-1","bogus":true}`))
	mux.ServeHTTP(rec2, req2.WithContext(core.WithStrictJSONDecoding(ctxAs("user-a"))))
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("POST with unknown field = %d, want 400 — strict decoding must stay active", rec2.Code)
	}
}
