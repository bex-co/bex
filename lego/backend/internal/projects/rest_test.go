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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestToRenderProjectIncludesEnvironmentMembership(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	got := toRenderProject(ProjectView{ID: "prj-1", Name: "platform", OwnerID: "tea-1", CreatedAt: created}, []string{"env-1"})
	if got.ID != "prj-1" || got.Owner.ID != "tea-1" || got.Owner.Type != "team" || len(got.EnvironmentIDs) != 1 || got.EnvironmentIDs[0] != "env-1" {
		t.Fatalf("Render project = %+v", got)
	}
	if !got.UpdatedAt.Equal(created) {
		t.Fatalf("updatedAt = %v, want %v", got.UpdatedAt, created)
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
