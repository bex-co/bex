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
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/m149 F2: the pinned CLI resolves `--project <name>` with an unfiltered
// GET /v1/projects?name=…, which on Render spans every workspace the caller
// belongs to. It used to answer from the default workspace only, so a project
// in the CLI's active (non-default) workspace was "not found".
func TestUnfilteredProjectListSpansTheCallersWorkspaces(t *testing.T) {
	st := newFakeProjectStore(
		store.Project{ID: "prj-a", TenantID: "tea-a", Name: "alpha"},
		store.Project{ID: "prj-b", TenantID: "tea-b", Name: "bravo"},
		// Same name in both workspaces: both come back, and the pinned CLI
		// reports the name as ambiguous rather than picking one.
		store.Project{ID: "prj-c", TenantID: "tea-a", Name: "shared"},
		store.Project{ID: "prj-d", TenantID: "tea-b", Name: "shared"},
	)
	svc := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: coretest.Workspaces{"tea-a", "tea-b"},
		MemberWorkspaceIDs: func(context.Context, core.Identity) ([]string, error) { return []string{"tea-a", "tea-b"}, nil }}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	list := func(query string) []string {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/projects"+query, nil).WithContext(ctx))
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/projects%s = %d %s", query, w.Code, w.Body)
		}
		var rows []struct {
			Project struct {
				ID string `json:"id"`
			} `json:"project"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, r := range rows {
			ids = append(ids, r.Project.ID)
		}
		slices.Sort(ids)
		return ids
	}
	for query, want := range map[string][]string{
		"":                          {"prj-a", "prj-b", "prj-c", "prj-d"},
		"?name=shared":              {"prj-c", "prj-d"},
		"?name=bravo":               {"prj-b"},
		"?ownerId=tea-a&name=bravo": nil,
		"?ownerId=tea-b":            {"prj-b", "prj-d"},
	} {
		if got := list(query); !slices.Equal(got, want) {
			t.Errorf("GET /v1/projects%s = %v, want %v", query, got, want)
		}
	}
}
