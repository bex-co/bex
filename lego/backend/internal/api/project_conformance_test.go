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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/projects"
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
	p := store.Project{ID: "prj-created", TenantID: tenantID, Name: name, CreatedAt: conformEpoch}
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
	s.projects[id] = p
	return nil
}

func (s *conformProjectStore) DeleteProject(_ context.Context, id string) error {
	delete(s.projects, id)
	return nil
}

func (s *conformProjectStore) SetProjectServices(context.Context, string, string, []string) ([]core.ServicePlacementChange, error) {
	return nil, nil
}

func (s *conformProjectStore) ListProjectServices(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *conformProjectStore) ListEnvironments(_ context.Context, projectID string) ([]store.Environment, error) {
	return s.envs[projectID], nil
}

// projectConformanceMux wires the real projects REST fragment over the stateful
// fake, allow-all authz. Databases/KeyValues use the empty sweep double so the
// link PUTs resolve rather than 503.
func projectConformanceMux() (*http.ServeMux, context.Context) {
	base := &core.Base{Authz: &fakeChecker{allow: true}}
	st := newConformProjectStore(store.Project{ID: "prj-1", TenantID: "tea-1", Name: "platform", CreatedAt: conformEpoch})
	st.envs["prj-1"] = []store.Environment{{ID: "env-1", ProjectID: "prj-1", TenantID: "tea-1", Name: "production"}}
	svc := &projects.Service{
		Base:      base,
		Store:     st,
		Databases: sweepProjectResources{},
		KeyValues: sweepProjectResources{},
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
