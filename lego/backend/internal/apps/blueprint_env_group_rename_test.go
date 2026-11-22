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
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/envgroups/envgroupstest"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/382, production 2026-11-21 on examples/blueprint-refs: after the env
// group a Blueprint created was renamed in the dashboard, the next plan read
// `create` for the declared name, and approving it minted a second group and
// linked the service to both. A Blueprint now follows the group it applied by
// id and its sync renames it back, as the next sync overwrites a change made
// outside the file on Render (render.com/docs/infrastructure-as-code).

const renamedGroupManifest = `envVarGroups:
  - name: shared
    envVars:
      - {key: LOG_LEVEL, value: info}
services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - fromGroup: shared
`

// renamedGroupBlueprint is an approved Blueprint of renamedGroupManifest over
// the real env-groups service, wired as the composition root wires it.
func renamedGroupBlueprint(t *testing.T) (*Service, *envgroups.Service, context.Context, string) {
	t.Helper()
	svc, _, ctx := newOnboardingService(t, renamedGroupManifest)
	groups := &envgroups.Service{Base: svc.Base, Store: newMemKV(), Groups: envgroupstest.New()}
	svc.EnvGroups = groups
	svc.EnvGroupWritesOwed = envgroups.GroupWritesOwed(groups)
	svc.EnvGroupRename = envgroups.GroupRename(groups)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	return svc, groups, ctx, res.Blueprint.ID
}

// onlyGroup is the workspace's one env group.
func onlyGroup(t *testing.T, groups *envgroups.Service, ctx context.Context) envgroups.EnvGroupView {
	t.Helper()
	list, err := groups.ListEnvGroups(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		names := make([]string, 0, len(list))
		for _, g := range list {
			names = append(names, g.ID+"="+g.Name)
		}
		t.Fatalf("env groups = %v, want exactly one", names)
	}
	return list[0]
}

// webGroupSecrets is the env group Secrets web links.
func webGroupSecrets(t *testing.T, svc *Service, ctx context.Context) []string {
	t.Helper()
	web := &appv1alpha1.App{}
	if err := svc.Client.Get(ctx, appKey(t, svc, "web"), web); err != nil {
		t.Fatal(err)
	}
	return slices.DeleteFunc(slices.Clone(web.Spec.EnvFromSecrets), func(s string) bool { return !strings.HasPrefix(s, "evg-") })
}

// syncActions is a plan's sync's actions.
func syncActions(t *testing.T, plan BlueprintPlanResult) []BlueprintPlanAction {
	t.Helper()
	if plan.Sync == nil {
		t.Fatalf("plan = %+v, want a planned sync", plan)
	}
	return plan.Sync.Actions
}

func TestASyncRenamesItsEnvGroupBackInsteadOfMintingACopy(t *testing.T) {
	svc, groups, ctx, bpID := renamedGroupBlueprint(t)
	original := onlyGroup(t, groups, ctx)
	if got := webGroupSecrets(t, svc, ctx); len(got) != 1 || got[0] != original.ID+"-env" {
		t.Fatalf("web links %v, want only %s", got, original.ID)
	}
	if _, err := groups.RenameEnvGroup(ctx, original.ID, "renamed-outside"); err != nil {
		t.Fatal(err)
	}

	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	action := planAction(t, syncActions(t, plan), "shared")
	if action.Operation != BlueprintPlanUpdate || action.ResourceID != original.ID ||
		len(action.ChangedFields) != 1 || action.ChangedFields[0].Path != "name" {
		t.Fatalf("renamed group planned %+v, want an update of name alone on %s", action, original.ID)
	}
	// While renamed, the Blueprint still lists the group as its own.
	view, err := svc.GetBlueprintByID(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(view.Resources, func(r BlueprintResource) bool { return r.ID == original.ID }) {
		t.Errorf("resources %+v do not list the renamed group %s", view.Resources, original.ID)
	}

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := onlyGroup(t, groups, ctx); got.ID != original.ID || got.Name != "shared" {
		t.Fatalf("after the sync the group is %s=%q, want %s=shared", got.ID, got.Name, original.ID)
	}
	if got := webGroupSecrets(t, svc, ctx); len(got) != 1 || got[0] != original.ID+"-env" {
		t.Fatalf("after the sync web links %v, want only %s", got, original.ID)
	}
	if again, err := svc.PlanBlueprint(ctx, bpID, "tea-a"); err != nil || again.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("re-plan = %+v err=%v, want no_changes", again, err)
	}
}

// A declared name another group took after the rename cannot be given back,
// and adopting that group would link web to both: the plan and the sync
// refuse, naming both groups, and nothing is written.
// A renamed group whose declared variables also changed plans one update
// naming name and envVars together: the sync renames the group back and
// writes the changed value, and the same file then plans no change. Until
// w8/382 the plan read `create`; until w8/376 it named every declared field.
func TestASyncRenamesItsEnvGroupBackAndWritesItsChangedVars(t *testing.T) {
	svc, groups, ctx, bpID := renamedGroupBlueprint(t)
	original := onlyGroup(t, groups, ctx)
	if _, err := groups.RenameEnvGroup(ctx, original.ID, "renamed-outside"); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(renamedGroupManifest, "value: info", "value: debug", 1)
	svc.GitFetcher = fakeBlueprintFetcher{contents: changed}

	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	action := planAction(t, syncActions(t, plan), "shared")
	if action.Operation != BlueprintPlanUpdate || action.ResourceID != original.ID || action.Message != "" {
		t.Fatalf("renamed group with changed vars planned %+v, want an update on %s with no message", action, original.ID)
	}
	var paths []string
	for _, c := range action.ChangedFields {
		paths = append(paths, c.Path)
	}
	if !slices.Equal(paths, []string{"envVars", "name"}) {
		t.Fatalf("renamed group with changed vars planned fields %q, want [envVars name]", paths)
	}

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := onlyGroup(t, groups, ctx); got.ID != original.ID || got.Name != "shared" {
		t.Fatalf("after the sync the group is %s=%q, want %s=shared", got.ID, got.Name, original.ID)
	}
	if got := webGroupSecrets(t, svc, ctx); len(got) != 1 || got[0] != original.ID+"-env" {
		t.Fatalf("after the sync web links %v, want only %s", got, original.ID)
	}
	if again, err := svc.PlanBlueprint(ctx, bpID, "tea-a"); err != nil || again.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("re-plan = %+v err=%v, want no_changes", again, err)
	}
}

func TestASyncRefusesWhenItsEnvGroupsNameWasTakenMeanwhile(t *testing.T) {
	svc, groups, ctx, bpID := renamedGroupBlueprint(t)
	original := onlyGroup(t, groups, ctx)
	if _, err := groups.RenameEnvGroup(ctx, original.ID, "renamed-outside"); err != nil {
		t.Fatal(err)
	}
	other, err := groups.CreateEnvGroup(ctx, envgroups.CreateEnvGroupRequest{Name: "shared", OwnerID: "tea-a"})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanningOutcome != PlanningFailed || len(plan.Notices) == 0 ||
		!strings.Contains(plan.Notices[0].Message, original.ID) || !strings.Contains(plan.Notices[0].Message, other.ID) {
		t.Fatalf("plan = %+v, want a failed plan naming %s and %s", plan, original.ID, other.ID)
	}
	if _, err := svc.SyncBlueprint(ctx, bpID, "tea-a", "", "", nil); err == nil || !strings.Contains(err.Error(), other.ID) {
		t.Fatalf("sync err = %v, want the refusal naming %s", err, other.ID)
	}
	list, err := groups.ListEnvGroups(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("env groups after the refusal = %+v, want the two there were", list)
	}
	if got := webGroupSecrets(t, svc, ctx); len(got) != 1 || got[0] != original.ID+"-env" {
		t.Fatalf("after the refusal web links %v, want only %s", got, original.ID)
	}
}

// A group deleted since is recreated under its declared name, as Render
// recreates a deleted Blueprint resource, and web links only the new one.
func TestASyncRecreatesADeletedEnvGroupOnce(t *testing.T) {
	svc, groups, ctx, bpID := renamedGroupBlueprint(t)
	original := onlyGroup(t, groups, ctx)
	if err := groups.DeleteEnvGroup(ctx, original.ID); err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if action := planAction(t, syncActions(t, plan), "shared"); action.Operation != BlueprintPlanCreate {
		t.Fatalf("deleted group planned %+v, want a create", action)
	}
	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	recreated := onlyGroup(t, groups, ctx)
	if recreated.ID == original.ID || recreated.Name != "shared" {
		t.Fatalf("recreated group %s=%q, want a new group named shared", recreated.ID, recreated.Name)
	}
	if got := webGroupSecrets(t, svc, ctx); len(got) != 1 || got[0] != recreated.ID+"-env" {
		t.Fatalf("web links %v, want only %s", got, recreated.ID)
	}
}

// groupedEnvGroupResources is shared linked from web: the two move together
// between environments, as Render's scoping rule links an environment's group
// only to that environment's services (w8/179).
const groupedEnvGroupResources = `envVarGroups:
  - name: shared
    envVars:
      - {key: LOG_LEVEL, value: info}
services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - fromGroup: shared
`

// groupedBlueprint is an approved Blueprint of groupedEnvGroupResources in
// shop/staging over the real env-groups service plus the grouping store, as
// the composition root wires them: claims, renames, placements and moves all
// resolve, so a plan and a sync can move the group between environments.
func groupedBlueprint(t *testing.T, resources string) (*Service, *envgroups.Service, *blueprintGroupingTestStore, context.Context, string) {
	t.Helper()
	gst := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc, _, ctx := newOnboardingService(t, resources)
	svc.BlueprintGroups, svc.Environments = gst, gst
	groups := &envgroups.Service{Base: svc.Base, Store: newMemKV(), Groups: envgroupstest.New()}
	groups.Environments = gst
	svc.EnvGroups = groups
	svc.EnvGroupWritesOwed = envgroups.GroupWritesOwed(groups)
	svc.EnvGroupRename = envgroups.GroupRename(groups)
	svc.EnvGroupPlacements = envgroups.GroupPlacements(groups)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	return svc, groups, gst, ctx, res.Blueprint.ID
}

// groupEnvironment is the public id of shop's environment in the grouping
// store.
func groupEnvironment(t *testing.T, gst *blueprintGroupingTestStore, name string) string {
	t.Helper()
	var shopID string
	for _, p := range gst.projects {
		if p.Name == "shop" {
			shopID = p.ID
		}
	}
	for _, e := range gst.environments {
		if e.Name == name && e.ProjectID == shopID {
			return ids.EnvironmentPublicID(e.ID)
		}
	}
	t.Fatalf("no shop/%s environment in %+v", name, gst.environments)
	return ""
}

// changedPaths is the action's changed-field paths in order.
func changedPaths(action BlueprintPlanAction) []string {
	var paths []string
	for _, c := range action.ChangedFields {
		paths = append(paths, c.Path)
	}
	return paths
}

// A group the file moves with its linked service: the plan names the
// environment move and the sync moves both, so the same file then plans no
// change. Until the group's move waited for the services, the apply moved
// the group first and Render's scoping rule refused it while the service
// still sat elsewhere — the sync failed although the plan promised the move.
func TestASyncMovesAnEnvGroupWithItsService(t *testing.T) {
	svc, groups, gst, ctx, bpID := groupedBlueprint(t, groupedResources(groupedEnvGroupResources, "shop", "staging"))
	original := onlyGroup(t, groups, ctx)
	svc.GitFetcher = fakeBlueprintFetcher{contents: groupedResources(groupedEnvGroupResources, "shop", "production")}

	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	action := planAction(t, syncActions(t, plan), "shared")
	if action.Operation != BlueprintPlanUpdate || action.ResourceID != original.ID {
		t.Fatalf("moved group planned %+v, want an update on %s", action, original.ID)
	}
	if paths := changedPaths(action); !slices.Equal(paths, []string{"environment"}) {
		t.Fatalf("moved group planned fields %q, want [environment]", paths)
	}

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	if moved := onlyGroup(t, groups, ctx); moved.ID != original.ID || moved.EnvironmentID != groupEnvironment(t, gst, "production") {
		t.Fatalf("after the sync the group is %s in %q, want %s in shop/production", moved.ID, moved.EnvironmentID, original.ID)
	}
	if again, err := svc.PlanBlueprint(ctx, bpID, "tea-a"); err != nil || again.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("re-plan = %+v err=%v, want no_changes", again, err)
	}
}

// A renamed group that also moves environment: the plan probes the placement
// by the live name and names environment beside name, the sync renames the
// group back before it moves it, and the same file then plans no change.
func TestASyncRenamesItsEnvGroupBackAndMovesIt(t *testing.T) {
	svc, groups, gst, ctx, bpID := groupedBlueprint(t, groupedResources(groupedEnvGroupResources, "shop", "staging"))
	original := onlyGroup(t, groups, ctx)
	if _, err := groups.RenameEnvGroup(ctx, original.ID, "renamed-outside"); err != nil {
		t.Fatal(err)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: groupedResources(groupedEnvGroupResources, "shop", "production")}

	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	action := planAction(t, syncActions(t, plan), "shared")
	if action.Operation != BlueprintPlanUpdate || action.ResourceID != original.ID {
		t.Fatalf("renamed group that moved planned %+v, want an update on %s", action, original.ID)
	}
	if paths := changedPaths(action); !slices.Equal(paths, []string{"environment", "name"}) {
		t.Fatalf("renamed group that moved planned fields %q, want [environment name]", paths)
	}

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	moved := onlyGroup(t, groups, ctx)
	if moved.ID != original.ID || moved.Name != "shared" {
		t.Fatalf("after the sync the group is %s=%q, want %s=shared", moved.ID, moved.Name, original.ID)
	}
	if moved.EnvironmentID != groupEnvironment(t, gst, "production") {
		t.Fatalf("after the sync the group is in %q, want shop/production", moved.EnvironmentID)
	}
	if got := webGroupSecrets(t, svc, ctx); len(got) != 1 || got[0] != original.ID+"-env" {
		t.Fatalf("after the sync web links %v, want only %s", got, original.ID)
	}
	if again, err := svc.PlanBlueprint(ctx, bpID, "tea-a"); err != nil || again.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("re-plan = %+v err=%v, want no_changes", again, err)
	}
}

// groupedEnvGroupLeftBehind is shared moved to production while web stays in
// staging linked to it: the move strands the link, so only the placement gate
// can stop it before the apply's own refusal.
const groupedEnvGroupLeftBehind = `projects:
  - name: shop
    environments:
      - name: staging
        services:
          - name: web
            type: web
            runtime: image
            image: {url: nginx:1}
            envVars:
              - fromGroup: shared
      - name: production
        envVarGroups:
          - name: shared
            envVars:
              - {key: LOG_LEVEL, value: info}
`

// A renamed group moved out of a protected environment is refused like one
// that kept its name: the gate reads the placement by the live name, so the
// move is not mistaken for a create in the unprotected target. Until the gate
// knew the live name, a non-Admin's plan promised the move and the sync sailed
// past the gate, failing later at the apply's own refusal instead.
func TestASyncRefusesARenamedEnvGroupLeavingAProtectedEnvironment(t *testing.T) {
	svc, groups, gst, ctx, bpID := groupedBlueprint(t, groupedResources(groupedEnvGroupResources, "shop", "staging"))
	for i := range gst.environments {
		if gst.environments[i].Name == "staging" {
			gst.environments[i].ProtectedStatus = core.ProtectedStatusProtected
		}
	}
	original := onlyGroup(t, groups, ctx)
	if _, err := groups.RenameEnvGroup(ctx, original.ID, "renamed-outside"); err != nil {
		t.Fatal(err)
	}
	asNonAdmin(svc)
	svc.GitFetcher = fakeBlueprintFetcher{contents: groupedEnvGroupLeftBehind}

	plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanningOutcome != PlanningFailed || len(plan.Notices) == 0 ||
		!strings.Contains(plan.Notices[0].Message, "Admins") {
		t.Fatalf("plan = %+v err=%v, want the protected-placement refusal", plan, err)
	}
	if v, err := svc.ValidateBlueprint(ctx, "tea-a", groupedEnvGroupLeftBehind, bpID); err != nil || v.Valid {
		t.Fatalf("validate = %+v err=%v, want the protected-placement refusal", v, err)
	}

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil); err == nil {
		t.Fatalf("sync of a refused plan succeeded, want the protected-placement refusal")
	}
	if got := onlyGroup(t, groups, ctx); got.ID != original.ID || got.Name != "renamed-outside" {
		t.Fatalf("after the refusal the group is %s=%q, want %s=renamed-outside", got.ID, got.Name, original.ID)
	}
}

// Services were never matched by their label: a dashboard rename leaves the
// Blueprint name (immutableName) the file matches, so the plan is unchanged
// and no second service appears (ADR049 § Service display labels).
func TestASyncAfterAServiceRenameKeepsTheService(t *testing.T) {
	svc, _, ctx, bpID := renamedGroupBlueprint(t)
	if _, err := svc.SetDisplayName(ctx, "web", "renamed-web"); err != nil {
		t.Fatal(err)
	}
	if plan, err := svc.PlanBlueprint(ctx, bpID, "tea-a"); err != nil || plan.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("plan after a service rename = %+v err=%v, want no_changes", plan, err)
	}
	var apps appv1alpha1.AppList
	if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 1 {
		t.Fatalf("%d services after the rename (err %v), want 1", len(apps.Items), err)
	}
}
