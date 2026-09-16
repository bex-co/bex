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

// project_conformance_test.go closes the w6/m126 conformance gap: before it,
// TestRenderConformance covered only reads and NO create/update response of any
// resource, so the five project write handlers that emitted the internal
// ProjectView instead of Render's `project` shape were simply never examined.
//
// This drives every project handler — the two reads and the five writes (POST,
// PATCH, and the three link PUTs) — through the real REST fragment and asserts
// two things:
//   - the create/update/read responses validate against the SAME pinned Render
//     OpenAPI schema the request gate uses (catches a handler that regresses to
//     the internal view: owner/environmentIds/updatedAt would go missing); and
//   - every write response has the identical key set to a read and carries none
//     of the internal-only fields (catches the link PUTs, which are bex-native
//     extensions absent from Render's spec, and any extra-key drift the schema's
//     open additionalProperties would otherwise permit).
//
// Reverting any one project write handler to return the raw ProjectView turns
// this file red — the demonstration the milestone's "drift fails the build"
// requirement calls for.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/projects"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// conformProjectStore is a stateful projects.ProjectStore for the conformance
// fixtures. It also carries ListEnvironments (the optional
// projectEnvironmentLister renderProject reads) so environmentIds is populated
// from real membership, not a coincidental empty array.
type conformProjectStore struct {
	projects map[string]store.Project
	envs     map[string][]store.Environment
}

func newConformProjectStore(seed ...store.Project) *conformProjectStore {
	s := &conformProjectStore{projects: map[string]store.Project{}, envs: map[string][]store.Environment{}}
	for _, p := range seed {
		s.projects[p.ID] = p
	}
	return s
}

func (s *conformProjectStore) CreateProject(_ context.Context, tenantID, name string) (store.Project, error) {
	p := store.Project{ID: "prj-created", TenantID: tenantID, Name: name, CreatedAt: conformEpoch, UpdatedAt: conformEpoch}
	s.projects[p.ID] = p
	return p, nil
}

func (s *conformProjectStore) GetProject(_ context.Context, id string) (store.Project, error) {
	p, ok := s.projects[id]
	if !ok {
		return store.Project{}, core.ErrNotFound
	}
	return p, nil
}

func (s *conformProjectStore) ListProjects(_ context.Context, tenantID string) ([]store.Project, error) {
	var out []store.Project
	for _, p := range s.projects {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *conformProjectStore) RenameProject(_ context.Context, id, name string) error {
	p, ok := s.projects[id]
	if !ok {
		return core.ErrNotFound
	}
	p.Name = name
	// Advance past createdAt so value-level conformance can see a real move.
	p.UpdatedAt = conformEpoch.Add(time.Minute)
	s.projects[id] = p
	return nil
}

func (s *conformProjectStore) TouchProject(_ context.Context, id string) error {
	p, ok := s.projects[id]
	if !ok {
		return core.ErrNotFound
	}
	p.UpdatedAt = conformEpoch.Add(2 * time.Minute)
	s.projects[id] = p
	return nil
}

func (s *conformProjectStore) DeleteProject(_ context.Context, id string) error {
	delete(s.projects, id)
	return nil
}

func (s *conformProjectStore) SetProjectServices(_ context.Context, id, _ string, _ []string) ([]core.ServicePlacementChange, error) {
	if err := s.TouchProject(context.Background(), id); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *conformProjectStore) ListProjectServices(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *conformProjectStore) ListEnvironments(_ context.Context, projectID string) ([]store.Environment, error) {
	return s.envs[projectID], nil
}

// projectConformanceOwner is the resolvable workspace identity the value-level
// conformance fixtures seed — reverting the owner-resolution fix turns these
// assertions red (w4/m109/t003).
var projectConformanceOwner = resourcemeta.Owner{
	ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team",
}

type fixedProjectOwnerResolver map[string]resourcemeta.Owner

func (f fixedProjectOwnerResolver) ResolveResourceOwners(_ context.Context, ids []string) map[string]resourcemeta.Owner {
	out := map[string]resourcemeta.Owner{}
	for _, id := range ids {
		if o, ok := f[id]; ok {
			out[id] = o
		}
	}
	return out
}

// projectConformanceMux wires the real projects REST fragment over the stateful
// fake, allow-all authz. Databases/KeyValues use the empty sweep double so the
// link PUTs resolve rather than 503. Owners is wired so owner.name/email are
// populated — key-set-only conformance used to green-light blank owners.
func projectConformanceMux() (*http.ServeMux, context.Context) {
	base := &core.Base{Authz: &fakeChecker{allow: true}}
	st := newConformProjectStore(store.Project{
		ID: "prj-1", TenantID: "tea-1", Name: "platform",
		CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	})
	st.envs["prj-1"] = []store.Environment{{ID: "env-1", ProjectID: "prj-1", TenantID: "tea-1", Name: "production"}}
	svc := &projects.Service{
		Base:      base,
		Store:     st,
		Databases: sweepProjectResources{},
		KeyValues: sweepProjectResources{},
		Owners:    fixedProjectOwnerResolver{"tea-1": projectConformanceOwner},
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "qa", Method: "session"})
	return mux, ctx
}

func serveProject(t *testing.T, mux *http.ServeMux, ctx context.Context, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func projectBodyKeys(t *testing.T, body []byte) []string {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("decode project object: %v (body %s)", err, body)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// renderProjectForbiddenKeys are the internal ProjectView fields Render's
// project object has no place for; their presence on any response is the
// w6/m126 defect.
var renderProjectForbiddenKeys = []string{"databaseIds", "keyValueIds", "ownerId", "serviceIds"}

// TestProjectResponsesConformToRenderSchema validates the project read AND write
// responses against the pinned Render OpenAPI schema — the reads that always
// conformed and the create/update that were never under test.
func TestProjectResponsesConformToRenderSchema(t *testing.T) {
	spec := loadRenderSpec(t)
	mux, ctx := projectConformanceMux()

	assertConforms := func(t *testing.T, operationID string, status int, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("%s => %d (want %d): %s", operationID, rec.Code, status, rec.Body.String())
		}
		if errs := spec.validateStatus(operationID, status, rec.Body.Bytes()); len(errs) > 0 {
			t.Errorf("Render schema violation(s) for %s:\n  %s", operationID, strings.Join(errs, "\n  "))
		}
	}

	t.Run("list-projects", func(t *testing.T) {
		assertConforms(t, "list-projects", http.StatusOK,
			serveProject(t, mux, ctx, http.MethodGet, "/v1/projects?ownerId=tea-1", ""))
	})
	t.Run("retrieve-project", func(t *testing.T) {
		assertConforms(t, "retrieve-project", http.StatusOK,
			serveProject(t, mux, ctx, http.MethodGet, "/v1/projects/prj-1", ""))
	})
	t.Run("create-project", func(t *testing.T) {
		assertConforms(t, "create-project", http.StatusCreated,
			serveProject(t, mux, ctx, http.MethodPost, "/v1/projects", `{"name":"api","ownerId":"tea-1"}`))
	})
	t.Run("update-project", func(t *testing.T) {
		assertConforms(t, "update-project", http.StatusOK,
			serveProject(t, mux, ctx, http.MethodPatch, "/v1/projects/prj-1", `{"name":"renamed"}`))
	})
}

// TestProjectWriteResponsesMatchReadShape is the drift guard for the whole
// resource: every write handler returns the identical key set a read does and
// none of the internal-only fields. It covers the three link PUTs too — they are
// bex-native extensions with no Render response schema, so conformance validation
// alone cannot see them, yet they are three of the five handlers that used to
// leak the internal view.
func TestProjectWriteResponsesMatchReadShape(t *testing.T) {
	mux, ctx := projectConformanceMux()

	read := serveProject(t, mux, ctx, http.MethodGet, "/v1/projects/prj-1", "")
	if read.Code != http.StatusOK {
		t.Fatalf("GET /v1/projects/prj-1 => %d: %s", read.Code, read.Body.String())
	}
	readKeys := projectBodyKeys(t, read.Body.Bytes())
	// Render's `project` object: exactly these six, nothing else.
	if want := []string{"createdAt", "environmentIds", "id", "name", "owner", "updatedAt"}; !slices.Equal(readKeys, want) {
		t.Fatalf("read key set = %v, want Render's %v", readKeys, want)
	}

	writes := []struct {
		name, method, path, body string
		status                   int
	}{
		{"create", http.MethodPost, "/v1/projects", `{"name":"api","ownerId":"tea-1"}`, http.StatusCreated},
		{"rename", http.MethodPatch, "/v1/projects/prj-1", `{"name":"renamed"}`, http.StatusOK},
		{"service-links", http.MethodPut, "/v1/projects/prj-1/service-links", `{"serviceIds":[]}`, http.StatusOK},
		{"database-links", http.MethodPut, "/v1/projects/prj-1/database-links", `{"databaseIds":[]}`, http.StatusOK},
		{"keyvalue-links", http.MethodPut, "/v1/projects/prj-1/keyvalue-links", `{"keyValueIds":[]}`, http.StatusOK},
	}
	for _, w := range writes {
		t.Run(w.name, func(t *testing.T) {
			rec := serveProject(t, mux, ctx, w.method, w.path, w.body)
			if rec.Code != w.status {
				t.Fatalf("%s => %d (want %d): %s", w.name, rec.Code, w.status, rec.Body.String())
			}
			keys := projectBodyKeys(t, rec.Body.Bytes())
			if !slices.Equal(keys, readKeys) {
				t.Fatalf("%s key set = %v, want read's %v — this write path is not emitting Render's project shape", w.name, keys, readKeys)
			}
			for _, forbidden := range renderProjectForbiddenKeys {
				if slices.Contains(keys, forbidden) {
					t.Fatalf("%s response carries internal-view key %q", w.name, forbidden)
				}
			}
		})
	}
}

// assertPopulatedOwner fails when owner is missing, blank, or disagrees with
// the fixture — the value-level guard w6/m126's key-set checks could not
// provide (w4/m109/t003). Sibling families (services/postgres/key-value) are
// covered by resource_metadata_test.go's assertResourceMetadata.
func assertPopulatedOwner(t *testing.T, body []byte, want resourcemeta.Owner) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("decode: %v", err)
	}
	owner, ok := obj["owner"].(map[string]any)
	if !ok {
		t.Fatalf("owner missing or wrong shape in %s", body)
	}
	if owner["id"] != want.ID || owner["name"] != want.Name || owner["email"] != want.Email || owner["type"] != want.Type {
		t.Fatalf("owner = %#v, want {id:%s name:%s email:%s type:%s}", owner, want.ID, want.Name, want.Email, want.Type)
	}
	if owner["name"] == "" || owner["email"] == "" {
		t.Fatalf("owner name/email must be non-empty (blank values used to ship green under key-set-only conformance)")
	}
}

func decodeProjectTimes(t *testing.T, body []byte) (created, updated time.Time) {
	t.Helper()
	var obj struct {
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("decode timestamps: %v", err)
	}
	return obj.CreatedAt, obj.UpdatedAt
}

// TestProjectOwnerAndUpdatedAtValuesConform is the drift guard for the two
// value bugs key-set conformance missed: blank owner.name/email, and
// updatedAt frozen as a copy of createdAt.
func TestProjectOwnerAndUpdatedAtValuesConform(t *testing.T) {
	mux, ctx := projectConformanceMux()

	read := serveProject(t, mux, ctx, http.MethodGet, "/v1/projects/prj-1", "")
	if read.Code != http.StatusOK {
		t.Fatalf("GET => %d: %s", read.Code, read.Body.String())
	}
	assertPopulatedOwner(t, read.Body.Bytes(), projectConformanceOwner)
	created, before := decodeProjectTimes(t, read.Body.Bytes())
	if !before.Equal(created) {
		t.Fatalf("seeded project updatedAt=%v createdAt=%v, want equal before mutation", before, created)
	}

	rename := serveProject(t, mux, ctx, http.MethodPatch, "/v1/projects/prj-1", `{"name":"renamed"}`)
	if rename.Code != http.StatusOK {
		t.Fatalf("PATCH => %d: %s", rename.Code, rename.Body.String())
	}
	assertPopulatedOwner(t, rename.Body.Bytes(), projectConformanceOwner)
	_, after := decodeProjectTimes(t, rename.Body.Bytes())
	if !after.After(before) {
		t.Fatalf("rename updatedAt=%v did not advance past pre-rename %v — UpdatedAt:p.CreatedAt hardcode would fail here", after, before)
	}
	if !after.After(created) {
		t.Fatalf("rename updatedAt=%v not after createdAt=%v", after, created)
	}

	create := serveProject(t, mux, ctx, http.MethodPost, "/v1/projects", `{"name":"api","ownerId":"tea-1"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("POST => %d: %s", create.Code, create.Body.String())
	}
	assertPopulatedOwner(t, create.Body.Bytes(), projectConformanceOwner)
	cCreated, cUpdated := decodeProjectTimes(t, create.Body.Bytes())
	if !cCreated.Equal(cUpdated) {
		t.Fatalf("new project createdAt=%v updatedAt=%v, want equal", cCreated, cUpdated)
	}

	// Link PUTs must also advance updatedAt (project membership is a project
	// modification).
	link := serveProject(t, mux, ctx, http.MethodPut, "/v1/projects/prj-1/service-links", `{"serviceIds":[]}`)
	if link.Code != http.StatusOK {
		t.Fatalf("PUT service-links => %d: %s", link.Code, link.Body.String())
	}
	assertPopulatedOwner(t, link.Body.Bytes(), projectConformanceOwner)
	_, linked := decodeProjectTimes(t, link.Body.Bytes())
	if !linked.After(after) {
		t.Fatalf("service-links updatedAt=%v did not advance past rename %v", linked, after)
	}
}

// TestProjectOwnerOmittedWhenUnresolved pins the resourcemeta contract: an
// unavailable owner is omitted entirely, never blanked to "".
func TestProjectOwnerOmittedWhenUnresolved(t *testing.T) {
	base := &core.Base{Authz: &fakeChecker{allow: true}}
	st := newConformProjectStore(store.Project{
		ID: "prj-1", TenantID: "tea-1", Name: "platform",
		CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	})
	svc := &projects.Service{
		Base:  base,
		Store: st,
		// Empty resolver map ⇒ owner unavailable.
		Owners: fixedProjectOwnerResolver{},
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "qa", Method: "session"})

	rec := serveProject(t, mux, ctx, http.MethodGet, "/v1/projects/prj-1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET => %d: %s", rec.Code, rec.Body.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v, ok := obj["owner"]; ok {
		t.Fatalf("unresolvable owner must be omitted, got %#v", v)
	}
}
