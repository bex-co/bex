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

package apps

import (
	"context"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestBlueprintProtectedPlacementTruthTable pins which Blueprint placements
// need an Admin (w3/m281): a create in, or a move into or out of, a protected
// environment — never a resource the apply leaves where it is.
func TestBlueprintProtectedPlacementTruthTable(t *testing.T) {
	placements := blueprintPlacements{
		protected: map[string]bool{"env-prod": true, "env-staging": false},
		targets: map[string]groupingTarget{
			"prod":     {environmentID: "env-prod", protected: true},
			"staging":  {environmentID: "env-staging"},
			"new-prod": {protected: true}, // the apply creates it, protected
			"new-dev":  {},
		},
	}
	unknown := placementOf{unknown: true}
	for _, tc := range []struct {
		name      string
		grouping  string
		ungrouped bool
		current   placementOf
		wantEnv   string
		refused   bool
	}{
		{"create in protected", "prod", false, placementOf{}, "env-prod", true},
		{"create in a new protected environment", "new-prod", false, placementOf{}, "", true},
		{"create in unprotected", "staging", false, placementOf{}, "", false},
		{"create ungrouped", "", true, placementOf{}, "", false},
		{"create with no placement", "", false, placementOf{}, "", false},
		{"stays in protected", "prod", false, placedIn(map[string]string{core.LabelEnvironment: "env-prod"}), "", false},
		{"moves into protected", "prod", false, placementOf{exists: true, environmentID: "env-staging"}, "env-prod", true},
		{"moves from unassigned into protected", "prod", false, placementOf{exists: true}, "env-prod", true},
		{"moves out of protected", "staging", false, placementOf{exists: true, environmentID: "env-prod"}, "env-prod", true},
		{"moves out of protected to unassigned", "", true, placementOf{exists: true, environmentID: "env-prod"}, "env-prod", true},
		{"moves out of protected into a new environment", "new-dev", false, placementOf{exists: true, environmentID: "env-prod"}, "env-prod", true},
		{"moves between unprotected", "new-dev", false, placementOf{exists: true, environmentID: "env-staging"}, "", false},
		{"stays unassigned", "", true, placementOf{exists: true}, "", false},
		{"declared without placement stays in protected", "", false, placementOf{exists: true, environmentID: "env-prod"}, "", false},
		{"an unread placement into protected counts as a move", "prod", false, unknown, "env-prod", true},
		{"an unread placement into unprotected is open", "staging", false, unknown, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, refused := placements.check(tc.grouping, tc.ungrouped, tc.current)
			if refused != tc.refused || env != tc.wantEnv {
				t.Fatalf("check(%q, ungrouped=%v, %+v) = %q, %v; want %q, %v", tc.grouping, tc.ungrouped, tc.current, env, refused, tc.wantEnv, tc.refused)
			}
		})
	}
}

// workspaceEnvironmentsStore offers the workspace-wide listing and counts the
// per-project one, which the placement read must then not use.
type workspaceEnvironmentsStore struct {
	*blueprintGroupingTestStore
	perProject int
}

func (s *workspaceEnvironmentsStore) ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error) {
	s.perProject++
	return s.blueprintGroupingTestStore.ListEnvironments(ctx, projectID)
}

func (s *workspaceEnvironmentsStore) ListWorkspaceEnvironments(_ context.Context, tenantID string) ([]store.Environment, error) {
	var out []store.Environment
	for _, e := range s.environments {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}

// w3/m292: the placement read lists the workspace's environments in one query
// when the store offers it, and resolves the groupings exactly as the
// per-project fallback does.
func TestBlueprintEnvironmentPlacementsUseOneWorkspaceListing(t *testing.T) {
	base := &blueprintGroupingTestStore{recordingStore: &recordingStore{},
		projects: []store.Project{{ID: "prj-a", TenantID: "tea-a", Name: "shop"}, {ID: "prj-b", TenantID: "tea-a", Name: "blog"}},
		environments: []store.Environment{
			{ID: "env-prod", ProjectID: "prj-a", TenantID: "tea-a", Name: "production", ProtectedStatus: core.ProtectedStatusProtected},
			{ID: "env-stage", ProjectID: "prj-a", TenantID: "tea-a", Name: "staging"},
			{ID: "env-blog", ProjectID: "prj-b", TenantID: "tea-a", Name: "production"},
		}}
	groupings := []parsedGrouping{{projectName: "shop", environmentName: "production"}, {projectName: "blog", environmentName: "production"}, {projectName: "shop", environmentName: "new"}}
	fallback := &Service{BlueprintGroups: base}
	want, err := fallback.blueprintEnvironmentPlacements(context.Background(), &workspaceSnapshot{s: fallback, tenantID: "tea-a"}, groupings)
	if err != nil {
		t.Fatal(err)
	}
	wide := &workspaceEnvironmentsStore{blueprintGroupingTestStore: base}
	wideService := &Service{BlueprintGroups: wide}
	snap := &workspaceSnapshot{s: wideService, tenantID: "tea-a"}
	got, err := wideService.blueprintEnvironmentPlacements(context.Background(), snap, groupings)
	if err != nil {
		t.Fatal(err)
	}
	if wide.perProject != 0 {
		t.Fatalf("per-project listings = %d, want 0", wide.perProject)
	}
	// w9/094: the snapshot lists the workspace's groupings once for the
	// protected-placement check and the plan alike, until it forgets them.
	base.projects = append(base.projects, store.Project{ID: "prj-c", TenantID: "tea-a", Name: "late"})
	if again, err := wideService.blueprintEnvironmentPlacements(context.Background(), snap, groupings); err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("a second read = %+v err=%v, want the snapshot's %+v", again, err, got)
	}
	snap.forget()
	if rows, err := snap.groupings(context.Background()); err != nil || len(rows.projects) != 3 {
		t.Fatalf("after forget the snapshot lists %+v err=%v, want the late project too", rows, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("placements = %+v, want %+v", got, want)
	}
	if !got.targets[blueprintGroupingKey("shop", "production")].protected || got.targets[blueprintGroupingKey("blog", "production")].protected {
		t.Fatalf("targets = %+v, want only shop/production protected", got.targets)
	}
}
