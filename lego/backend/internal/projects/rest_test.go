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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fixedOwnerResolver is a minimal resourcemeta.OwnerResolver a test can seed
// without pulling in workspaces.Service.
type fixedOwnerResolver map[string]resourcemeta.Owner

func (f fixedOwnerResolver) ResolveResourceOwners(_ context.Context, ids []string) map[string]resourcemeta.Owner {
	out := map[string]resourcemeta.Owner{}
	for _, id := range ids {
		if o, ok := f[id]; ok {
			out[id] = o
		}
	}
	return out
}

type recordingOwnerResolver struct {
	fixedOwnerResolver
	calls int
}

func (r *recordingOwnerResolver) ResolveResourceOwners(ctx context.Context, ids []string) map[string]resourcemeta.Owner {
	r.calls++
	return r.fixedOwnerResolver.ResolveResourceOwners(ctx, ids)
}

func TestToRenderProjectIncludesEnvironmentMembership(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	owner := resourcemeta.Owner{ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team"}
	got := toRenderProject(ProjectView{
		ID: "prj-1", Name: "platform", OwnerID: "tea-1",
		CreatedAt: created, UpdatedAt: updated,
	}, []string{"env-1"}, owner)
	if got.ID != "prj-1" || got.Owner == nil || got.Owner.ID != "tea-1" || got.Owner.Name != "Acme" || got.Owner.Type != "team" || len(got.EnvironmentIDs) != 1 || got.EnvironmentIDs[0] != "env-1" {
		t.Fatalf("Render project = %+v", got)
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Fatalf("updatedAt = %v, want %v", got.UpdatedAt, updated)
	}
}

func TestToRenderProjectOmitsUnresolvedOwner(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	got := toRenderProject(ProjectView{ID: "prj-1", Name: "platform", OwnerID: "tea-1", CreatedAt: created}, nil, resourcemeta.Owner{ID: "tea-1"})
	if got.Owner != nil {
		t.Fatalf("owner = %+v, want omitted (Available requires name)", got.Owner)
	}
	if !got.UpdatedAt.Equal(created) {
		t.Fatalf("zero UpdatedAt should fall back to createdAt, got %v", got.UpdatedAt)
	}
}

func TestProjectsRESTListNamesMissingOwnerID(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore()}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/projects without ownerId = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	// The opaque "bad request" the bug reported must be replaced by a body that
	// names ownerId, matching the validator-gated siblings (w4/038).
	if !strings.Contains(body.Error, "ownerId") {
		t.Fatalf("error body %q does not name the missing ownerId parameter", body.Error)
	}
	if body.Message != body.Error {
		t.Fatalf("message %q and error %q disagree", body.Message, body.Error)
	}
}

func TestProjectsRESTPaginationWalkTerminatesWithoutDuplicates(t *testing.T) {
	const total = 2*core.DefaultPageLimit + 1
	seeded := make([]store.Project, 0, total)
	for i := 1; i <= total; i++ {
		seeded = append(seeded, store.Project{
			ID:       fmt.Sprintf("prj-%03d", i),
			TenantID: "tea-1",
			Name:     fmt.Sprintf("project-%03d", i),
		})
	}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore(seeded...)}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	requestPage := func(cursor string, includeLimit bool) []renderProjectWithCursor {
		t.Helper()
		query := url.Values{"ownerId": {"tea-1"}}
		if includeLimit {
			query.Set("limit", fmt.Sprint(core.DefaultPageLimit))
		}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects?"+query.Encode(), nil)
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET projects = %d: %s", rec.Code, rec.Body.String())
		}
		var page []renderProjectWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode page: %v", err)
		}
		return page
	}

	if first := requestPage("", false); len(first) != core.DefaultPageLimit {
		t.Fatalf("absent limit returned %d projects, want default %d", len(first), core.DefaultPageLimit)
	}

	seen := make(map[string]struct{}, total)
	cursor := ""
	pages := 0
	for {
		page := requestPage(cursor, true)
		pages++
		if len(page) == 0 {
			break
		}
		for _, item := range page {
			if _, duplicate := seen[item.Project.ID]; duplicate {
				t.Fatalf("duplicate project %q after cursor %q", item.Project.ID, cursor)
			}
			seen[item.Project.ID] = struct{}{}
		}
		cursor = page[len(page)-1].Cursor
		if len(page) < core.DefaultPageLimit {
			if tail := requestPage(cursor, true); len(tail) != 0 {
				t.Fatalf("tail after final cursor = %d projects, want empty", len(tail))
			}
			break
		}
		if pages > 4 {
			t.Fatal("pagination did not terminate")
		}
	}
	if pages != 3 || len(seen) != total {
		t.Fatalf("walk = %d pages, %d unique projects; want 3 pages, %d projects", pages, len(seen), total)
	}
}

func decodeRenderProject(t *testing.T, body []byte) renderProject {
	t.Helper()
	var got renderProject
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	return got
}

func projectRESTFixture(t *testing.T, owners resourcemeta.OwnerResolver) (*Service, *http.ServeMux, *fakeProjectStore) {
	t.Helper()
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	st := newFakeProjectStore(store.Project{
		ID: "prj-1", TenantID: "tea-1", Name: "platform",
		CreatedAt: created, UpdatedAt: created,
	})
	svc := &Service{
		Base:      &core.Base{Authz: allowChecker{}},
		Store:     st,
		Databases: newFakeResourceIndex("tea-1"),
		KeyValues: newFakeResourceIndex("tea-1"),
		Owners:    owners,
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return svc, mux, st
}

func TestProjectMutationsAdvanceUpdatedAt(t *testing.T) {
	owners := fixedOwnerResolver{"tea-1": {ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team"}}
	_, mux, _ := projectRESTFixture(t, owners)

	get := func() renderProject {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj-1", nil)
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String())
		}
		return decodeRenderProject(t, rec.Body.Bytes())
	}

	before := get()
	if !before.UpdatedAt.Equal(before.CreatedAt) {
		t.Fatalf("seed updatedAt=%v createdAt=%v", before.UpdatedAt, before.CreatedAt)
	}

	mutations := []struct {
		name, method, path, body string
	}{
		{"rename", http.MethodPatch, "/v1/projects/prj-1", `{"name":"renamed"}`},
		{"service-links", http.MethodPut, "/v1/projects/prj-1/service-links", `{"serviceIds":[]}`},
		{"database-links", http.MethodPut, "/v1/projects/prj-1/database-links", `{"databaseIds":[]}`},
		{"keyvalue-links", http.MethodPut, "/v1/projects/prj-1/keyvalue-links", `{"keyValueIds":[]}`},
	}
	prev := before.UpdatedAt
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(m.method, m.path, strings.NewReader(m.body))
			mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s = %d: %s", m.name, rec.Code, rec.Body.String())
			}
			got := decodeRenderProject(t, rec.Body.Bytes())
			if !got.UpdatedAt.After(prev) {
				t.Fatalf("%s updatedAt=%v did not advance past %v", m.name, got.UpdatedAt, prev)
			}
			prev = got.UpdatedAt
			stable := get()
			if !stable.UpdatedAt.Equal(got.UpdatedAt) {
				t.Fatalf("read after %s moved updatedAt: %v -> %v", m.name, got.UpdatedAt, stable.UpdatedAt)
			}
		})
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(`{"name":"fresh","ownerId":"tea-1"}`))
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeRenderProject(t, rec.Body.Bytes())
	if !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("POST createdAt=%v updatedAt=%v, want equal", created.CreatedAt, created.UpdatedAt)
	}
}

func TestProjectOwnerResolutionBatchAndOmit(t *testing.T) {
	recorder := &recordingOwnerResolver{fixedOwnerResolver: fixedOwnerResolver{
		"tea-1": {ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team"},
	}}
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	st := newFakeProjectStore(
		store.Project{ID: "prj-1", TenantID: "tea-1", Name: "a", CreatedAt: created, UpdatedAt: created},
		store.Project{ID: "prj-2", TenantID: "tea-1", Name: "b", CreatedAt: created, UpdatedAt: created},
	)
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Owners: recorder}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects?ownerId=tea-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body.String())
	}
	var page []renderProjectWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d, want 2", len(page))
	}
	if recorder.calls != 1 {
		t.Fatalf("owner resolution called %d times for a page, want 1 (N+1 guard)", recorder.calls)
	}
	for _, item := range page {
		if item.Project.Owner == nil || item.Project.Owner.Name != "Acme" {
			t.Fatalf("project %s owner = %+v, want populated Acme", item.Project.ID, item.Project.Owner)
		}
	}

	// Unresolvable: empty map → omit.
	svc.Owners = fixedOwnerResolver{}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/projects/prj-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	got := decodeRenderProject(t, rec.Body.Bytes())
	if got.Owner != nil {
		t.Fatalf("unresolvable owner = %+v, want omitted", got.Owner)
	}

	// Nil Owners (control-plane unavailable) still renders the project.
	svc.Owners = nil
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/projects/prj-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil Owners GET = %d: %s", rec.Code, rec.Body.String())
	}
	got = decodeRenderProject(t, rec.Body.Bytes())
	if got.ID != "prj-1" || got.Owner != nil {
		t.Fatalf("nil Owners response = %+v", got)
	}
}
