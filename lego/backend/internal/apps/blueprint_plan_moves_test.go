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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// plannedUpdates is the actions of a plan that change something, each with
// the paths it lists: "update service web: environment, project", "create
// environment qa".
func plannedUpdates(actions []BlueprintPlanAction) string {
	var parts []string
	for _, a := range actions {
		if a.Operation == BlueprintPlanNoop {
			continue
		}
		part := string(a.Operation) + " " + string(a.Kind) + " " + a.Name
		if len(a.ChangedFields) > 0 {
			part += ": " + strings.Join(fieldPaths(a.ChangedFields), ", ")
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

// movedSide is a labelsAt placement as a move row spells it.
func movedSide(at string) string {
	if at == "/" {
		return "none"
	}
	return at
}

// w9/094: which placements a plan lists for a resource that already exists,
// as the apply compares them: the environment and project its labels hold
// against the ones its declaration resolves to, and a service's
// network-isolation label against the environment's isolation.
func TestBlueprintPlacementMovesTruthTable(t *testing.T) {
	placements := blueprintPlacements{targets: map[string]groupingTarget{
		"staging":     {environmentID: "env-staging", projectID: "prj-shop"},
		"production":  {environmentID: "env-prod", projectID: "prj-shop", isolated: true},
		"blog":        {environmentID: "env-blog", projectID: "prj-blog"},
		"new-env":     {projectID: "prj-shop"},                 // the apply creates the environment in shop
		"new-project": {},                                      // the apply creates both
		"new-iso":     {projectID: "prj-shop", isolated: true}, // a new isolated environment
	}}
	in := func(project, environment string) map[string]string {
		return map[string]string{core.LabelProject: project, core.LabelEnvironment: environment}
	}
	isolatedIn := func(project, environment string) map[string]string {
		labels := in(project, environment)
		labels[core.LabelNetworkIsolation] = environment
		return labels
	}
	for _, tc := range []struct {
		name      string
		grouping  string
		ungrouped bool
		labels    map[string]string
		want      string // for a service; a datastore lists no networkIsolation
	}{
		{"stays", "staging", false, in("prj-shop", "env-staging"), ""},
		{"another environment", "production", false, in("prj-shop", "env-staging"), "environment"},
		{"another project", "blog", false, in("prj-shop", "env-staging"), "environment,project"},
		{"into a grouping from none", "staging", false, nil, "environment,project"},
		{"a new environment of its project", "new-env", false, in("prj-shop", "env-staging"), "environment"},
		{"a new isolated environment", "new-iso", false, in("prj-shop", "env-staging"), "environment"},
		{"a new project", "new-project", false, in("prj-shop", "env-staging"), "environment,project"},
		{"a new project from none", "new-project", false, nil, "environment,project"},
		{"out of every grouping", "", true, in("prj-shop", "env-staging"), "environment,project"},
		{"stays ungrouped", "", true, nil, ""},
		{"declared without a placement", "", false, in("prj-shop", "env-staging"), ""},
		{"stays in an isolated environment", "production", false, isolatedIn("prj-shop", "env-prod"), ""},
		{"its environment turns isolation on", "production", false, in("prj-shop", "env-prod"), "networkIsolation"},
		{"its environment turns isolation off", "staging", false, isolatedIn("prj-shop", "env-staging"), "networkIsolation"},
		{"out of an isolated environment", "staging", false, isolatedIn("prj-shop", "env-prod"), "environment"},
		{"a stale isolation label while ungrouped", "", true, map[string]string{core.LabelNetworkIsolation: "env-prod"}, "networkIsolation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(placements.moves(tc.grouping, tc.ungrouped, true, tc.labels), ","); got != tc.want {
				t.Fatalf("service moves(%q, ungrouped=%v, %v) = %q, want %q", tc.grouping, tc.ungrouped, tc.labels, got, tc.want)
			}
			want := strings.TrimSuffix(strings.TrimSuffix(tc.want, "networkIsolation"), ",")
			if got := strings.Join(placements.moves(tc.grouping, tc.ungrouped, false, tc.labels), ","); got != want {
				t.Fatalf("datastore moves(%q, ungrouped=%v, %v) = %q, want %q", tc.grouping, tc.ungrouped, tc.labels, got, want)
			}
		})
	}
}

// isolatingShopProduction is manifest with the shop project's production
// environment network-isolated.
func isolatingShopProduction(manifest string) string {
	return strings.Replace(manifest, "      - name: production\n", "      - name: production\n        networking:\n          isolation: enabled\n", 1)
}

// w9/094: in a Blueprint whose syncs wait for approval, a render.yaml change
// whose only effect is to move the service plans the move, as the plan lists
// other changed fields: "environment", and "project" when the project changes
// too. Approving that plan applies the move and records one service_moved, by
// the approver; planning records nothing. Turning on the network isolation of
// the service's environment plans "networkIsolation" and moves nothing (the
// environment lists its own "networking.isolation", w9/095). A plan that keeps
// the placement, or declares the service at the top level (which leaves it
// where it is), finds no changes.
func TestApprovalModeBlueprintPlansAndAppliesAMoveAlone(t *testing.T) {
	svc := newBlueprintService(newFakeBlueprintStore(), fakeWorkspace{"user-a": "tea-a", "approver": "tea-a"})
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc.Store, svc.BlueprintGroups, svc.Environments = st, st, st
	svc.GitFetcher = fakeBlueprintFetcher{contents: groupedImageService("shop", "staging")}
	sink := &captureAuditSink{}
	svc.Audit = sink
	creator := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	approver := core.WithIdentity(context.Background(), core.Identity{Subject: "approver", Method: "session"})
	res, err := svc.CreatePlannedBlueprint(creator, RenderCreateBlueprintRequest{
		Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"},
	})
	if err != nil {
		t.Fatal(err)
	}
	bpID := res.Blueprint.ID
	if _, err := svc.ApproveBlueprint(creator, bpID, "tea-a", nil); err != nil {
		t.Fatalf("approve the create: %v", err)
	}
	placement := func() string { return labelsAt(getTenantApp(t, svc.Client, "tea-a", "web")) }

	for _, step := range []struct {
		name, project, environment string
		isolate                    bool   // the file isolates shop/production
		isolated                   bool   // the service is isolated after the approval
		plan                       string // "" plans no_changes
	}{
		{"another environment", "shop", "production", false, false, "update service web: environment"},
		{"the same environment", "shop", "production", false, false, ""},
		{"its environment turns network isolation on", "shop", "production", true, true, "update environment production: networking.isolation; update service web: networkIsolation"},
		{"a file that omits the isolation keeps it", "shop", "production", false, true, ""},
		{"another project", "blog", "production", false, false, "update service web: environment, project"},
		{"out of every project", "", "", false, false, "update service web: environment, project"},
		{"still ungrouped", "", "", false, false, ""},
		{"back into an isolated environment", "shop", "production", false, true, "update service web: environment, project"},
		{"a new environment the approval creates", "shop", "qa", false, false, "create environment qa; update service web: environment"},
	} {
		manifest := groupedImageService(step.project, step.environment)
		if step.isolate {
			manifest = isolatingShopProduction(manifest)
		}
		svc.GitFetcher = fakeBlueprintFetcher{contents: manifest}
		sink.events = nil
		before := placement()
		plan, err := svc.PlanBlueprint(creator, bpID, "tea-a")
		if err != nil {
			t.Fatalf("%s: plan: %v", step.name, err)
		}
		var actions []BlueprintPlanAction
		if plan.Sync != nil {
			actions = plan.Sync.Actions
		}
		if got := plannedUpdates(actions); got != step.plan {
			t.Fatalf("%s planned %q, want %q", step.name, got, step.plan)
		}
		if step.plan == "" {
			if plan.PlanningOutcome != PlanningNoChanges || plan.Sync != nil {
				t.Fatalf("%s: plan = %s with sync %v, want no_changes", step.name, plan.PlanningOutcome, plan.Sync)
			}
			if at := placement(); at != before {
				t.Fatalf("%s moved the service from %q to %q", step.name, before, at)
			}
			continue
		}
		if plan.PlanningOutcome != PlanningPlanned {
			t.Fatalf("%s: plan = %s, want planned", step.name, plan.PlanningOutcome)
		}
		if got := regroupRows(sink); len(got) != 0 || placement() != before {
			t.Fatalf("%s: planning recorded %v and left the service in %q, want nothing recorded or moved", step.name, got, placement())
		}
		if _, err := svc.ApproveBlueprint(approver, bpID, "tea-a", nil); err != nil {
			t.Fatalf("%s: approve: %v", step.name, err)
		}
		want := "/"
		if step.project != "" {
			want = groupingAt(t, st, step.project, step.environment)
		}
		if at := placement(); at != want {
			t.Fatalf("%s: approval left the service in %q, want %q", step.name, at, want)
		}
		wantIsolation := ""
		if step.isolated {
			_, wantIsolation, _ = strings.Cut(want, "/")
		}
		if got := getTenantApp(t, svc.Client, "tea-a", "web").Labels[core.LabelNetworkIsolation]; got != wantIsolation {
			t.Fatalf("%s: approval left the isolation label %q, want %q", step.name, got, wantIsolation)
		}
		var rows []string
		if before != want {
			rows = []string{"environments.MoveService " + movedSide(before) + "->" + movedSide(want) + " by approver"}
		}
		if got := regroupRows(sink); !slices.Equal(got, rows) {
			t.Fatalf("%s: approval recorded %v, want %v", step.name, got, rows)
		}
	}

	svc.GitFetcher = fakeBlueprintFetcher{contents: imageService}
	if plan, err := svc.PlanBlueprint(creator, bpID, "tea-a"); err != nil || plan.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("a top-level declaration planned %+v err=%v, want no_changes", plan, err)
	}
}

const groupedDatastores = `services:
  - type: keyvalue
    name: cache
    plan: starter
    ipAllowList: []
databases:
  - name: data
    plan: basic-256mb
`

// w9/094: Postgres and Key Value are placed as services are, so a plan lists
// their moves as well, and once a deploy has made a move the same file plans
// no change.
func TestBlueprintPlanListsADatastoreMove(t *testing.T) {
	svc, _, _, ctx, _ := groupingStackService(t)
	deploy := func(manifest string) {
		t.Helper()
		if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
			t.Fatalf("deploy: %v", err)
		}
	}
	planned := func(manifest string) string {
		t.Helper()
		v, err := svc.ValidateBlueprint(ctx, "tea-a", manifest, "")
		if err != nil || !v.Valid || v.Plan == nil || v.Plan.Mode != "current_state" {
			t.Fatalf("validate = %+v err=%v, want a current-state plan", v, err)
		}
		return plannedUpdates(v.Plan.Actions)
	}
	deploy(groupedResources(groupedDatastores, "shop", "staging"))
	for _, step := range []struct{ project, environment, want string }{
		{"shop", "production", "update postgres data: environment; update key_value cache: environment"},
		{"", "", "update postgres data: environment, project; update key_value cache: environment, project"},
		{"blog", "production", "update postgres data: environment, project; update key_value cache: environment, project"},
	} {
		manifest := groupedResources(groupedDatastores, step.project, step.environment)
		if got := planned(manifest); got != step.want {
			t.Fatalf("moving into %q/%q planned %q, want %q", step.project, step.environment, got, step.want)
		}
		deploy(manifest)
		if got := planned(manifest); got != "" {
			t.Fatalf("after the move into %q/%q the same file planned %q, want nothing", step.project, step.environment, got)
		}
	}
}
