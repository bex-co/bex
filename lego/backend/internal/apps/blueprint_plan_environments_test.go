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
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// declaringEnvironment is manifest (a groupedResources file) with block,
// render.yaml lines indented as an environment's fields, declared on
// project's environment.
func declaringEnvironment(t *testing.T, manifest, project, environment, block string) string {
	t.Helper()
	start := strings.Index(manifest, "  - name: "+project+"\n")
	head := "      - name: " + environment + "\n"
	at := strings.Index(manifest[max(start, 0):], head)
	if start < 0 || at < 0 {
		t.Fatalf("no environment %s/%s in %q", project, environment, manifest)
	}
	at += start + len(head)
	return manifest[:at] + block + manifest[at:]
}

func protectionBlock(value string) string {
	return "        permissions:\n          protection: " + value + "\n"
}

func isolationBlock(value string) string {
	return "        networking:\n          isolation: " + value + "\n"
}

// storedEnvironment is the environment st holds for project/environment.
func storedEnvironment(t *testing.T, st *blueprintGroupingTestStore, project, environment string) store.Environment {
	t.Helper()
	for _, p := range st.projects {
		for _, e := range st.environments {
			if p.Name == project && e.ProjectID == p.ID && e.Name == environment {
				return e
			}
		}
	}
	t.Fatalf("no environment %s/%s", project, environment)
	return store.Environment{}
}

// controlRows are the audit rows of the environment-setting writes a sync
// made, each with its caller.
func controlRows(sink *captureAuditSink) []string {
	var rows []string
	for _, ev := range sink.events {
		if ev.Verb == "apps.BlueprintGrouping.environment_controls_updated" {
			rows = append(rows, ev.TargetName+" by "+ev.Caller)
		}
	}
	return rows
}

// w9/095: in a Blueprint whose syncs wait for approval, a render.yaml change
// whose only effect is an environment's own setting plans that environment as
// an update listing the setting's path, as the plan lists a resource's changed
// fields: permissions.protection, networking.isolation, or both. Planning
// writes nothing; approving the plan applies the setting once, by the
// approver. The same file, or one that omits the setting (which the sync
// preserves), then plans no_changes. A new environment with nothing in it is a
// create, which the approval makes.
func TestApprovalModeBlueprintPlansAndAppliesAnEnvironmentSettingAlone(t *testing.T) {
	svc := newBlueprintService(newFakeBlueprintStore(), fakeWorkspace{"user-a": "tea-a", "approver": "tea-a"})
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc.Store, svc.BlueprintGroups, svc.Environments = st, st, st
	base := groupedImageService("shop", "staging")
	svc.GitFetcher = fakeBlueprintFetcher{contents: base}
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
	settings := func(project, environment string) string {
		e := storedEnvironment(t, st, project, environment)
		return fmt.Sprintf("%q isolated=%t", e.ProtectedStatus, e.NetworkIsolationEnabled)
	}
	withQA := strings.Replace(base, "  - name: blog\n", "      - name: qa\n  - name: blog\n", 1)

	for _, step := range []struct {
		name                 string
		manifest             string
		plan                 string // "" plans no_changes
		project, environment string // the environment the step checks
		create               bool   // the approval creates it
		want                 string // its settings after the step
	}{
		{"protecting an environment", declaringEnvironment(t, base, "shop", "production", protectionBlock("enabled")),
			"update environment production: permissions.protection", "shop", "production", false, `"protected" isolated=false`},
		{"the same file", declaringEnvironment(t, base, "shop", "production", protectionBlock("enabled")),
			"", "shop", "production", false, `"protected" isolated=false`},
		{"a file that omits the protection keeps it", base, "", "shop", "production", false, `"protected" isolated=false`},
		{"unprotecting it", declaringEnvironment(t, base, "shop", "production", protectionBlock("disabled")),
			"update environment production: permissions.protection", "shop", "production", false, `"unprotected" isolated=false`},
		{"isolating an environment that holds no service", declaringEnvironment(t, base, "blog", "production", isolationBlock("enabled")),
			"update environment production: networking.isolation", "blog", "production", false, `"" isolated=true`},
		{"both settings at once", declaringEnvironment(t, base, "blog", "production", protectionBlock("enabled")+isolationBlock("disabled")),
			"update environment production: networking.isolation, permissions.protection", "blog", "production", false, `"protected" isolated=false`},
		{"a new environment with nothing in it", withQA, "create environment qa", "shop", "qa", true, `"" isolated=false`},
		{"the same file again", withQA, "", "shop", "qa", false, `"" isolated=false`},
	} {
		svc.GitFetcher = fakeBlueprintFetcher{contents: step.manifest}
		sink.events = nil
		envs := len(st.environments)
		var before string
		if !step.create {
			before = settings(step.project, step.environment)
		}
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
			if got := settings(step.project, step.environment); got != step.want {
				t.Fatalf("%s left the environment %s, want %s", step.name, got, step.want)
			}
			continue
		}
		if plan.PlanningOutcome != PlanningPlanned {
			t.Fatalf("%s: plan = %s, want planned", step.name, plan.PlanningOutcome)
		}
		if len(st.environments) != envs || (!step.create && settings(step.project, step.environment) != before) || len(controlRows(sink)) != 0 {
			t.Fatalf("%s: planning wrote the environments (%d -> %d rows, %v)", step.name, envs, len(st.environments), controlRows(sink))
		}
		if _, err := svc.ApproveBlueprint(approver, bpID, "tea-a", nil); err != nil {
			t.Fatalf("%s: approve: %v", step.name, err)
		}
		if got := settings(step.project, step.environment); got != step.want {
			t.Fatalf("%s: approval left the environment %s, want %s", step.name, got, step.want)
		}
		var rows []string
		if !step.create {
			rows = []string{step.environment + " by approver"}
		}
		if got := controlRows(sink); !slices.Equal(got, rows) {
			t.Fatalf("%s: approval recorded %v, want %v", step.name, got, rows)
		}
	}
}

// w9/095: the plan lists each environment the apply creates or whose own
// settings it changes, before the resources placed in them and in the order
// the apply writes them: an existing one by its public id, a new one as a
// create, each located at its projects[].environments[] entry. An unchanged
// environment is not listed, so an unchanged file plans as it did before
// w9/095, and a file without projects lists none.
func TestBlueprintPlanListsEachDeclaredEnvironment(t *testing.T) {
	svc, _, _, ctx, st := groupingStackService(t)
	base := groupedImageService("shop", "staging")
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: base}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	web := slices.IndexFunc(st.appCreates, func(a store.App) bool { return a.Name == "web" })
	if web < 0 {
		t.Fatalf("the deploy stored no web row: %+v", st.appCreates)
	}
	unchangedWeb := "noop service web #/projects/0/environments/0/services/0 " + st.appCreates[web].ID
	planned := func(manifest string) (*BlueprintValidationPlan, []string) {
		t.Helper()
		v, err := svc.ValidateBlueprint(ctx, "tea-a", manifest, "")
		if err != nil || !v.Valid || v.Plan == nil || v.Plan.Mode != "current_state" {
			t.Fatalf("validate = %+v err=%v, want a current-state plan", v, err)
		}
		var rows []string
		for _, a := range v.Plan.Actions {
			row := fmt.Sprintf("%s %s %s %s %s", a.Operation, a.Kind, a.Name, a.SourcePath, a.ResourceID)
			if len(a.ChangedFields) > 0 {
				row += " " + strings.Join(fieldPaths(a.ChangedFields), " ")
			}
			rows = append(rows, row)
		}
		return v.Plan, rows
	}

	plan, got := planned(strings.Replace(declaringEnvironment(t, base, "shop", "production", protectionBlock("enabled")),
		"  - name: blog\n", "      - name: qa\n  - name: blog\n", 1))
	production := ids.EnvironmentPublicID(storedEnvironment(t, st, "shop", "production").ID)
	want := []string{
		"update environment production #/projects/0/environments/1 " + production + " permissions.protection",
		"create environment qa #/projects/0/environments/2 ",
		unchangedWeb,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("plan =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if id := plan.Actions[0].ResourceID; !strings.HasPrefix(id, "evm-") {
		t.Fatalf("the environment is listed by %q, want its public evm- id", id)
	}
	if plan.TotalActions != 3 {
		t.Fatalf("totalActions = %d, want 3", plan.TotalActions)
	}

	plan, got = planned(base)
	if want := []string{unchangedWeb}; !slices.Equal(got, want) || plan.TotalActions != 1 {
		t.Fatalf("the unchanged file planned %v (totalActions %d), want %v (1)", got, plan.TotalActions, want)
	}

	plan, _ = planned(imageService)
	if i := slices.IndexFunc(plan.Actions, func(a BlueprintPlanAction) bool { return a.Kind == BlueprintResourceEnvironment }); i >= 0 {
		t.Fatalf("a file without projects listed %+v", plan.Actions[i])
	}
}
