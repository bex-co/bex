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
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// groupedImageService is imageService declared in project's environment, or
// in the legacy ungrouped: list for an empty project, beside the shop
// (staging, production) and blog (production) groupings the regroup tests
// move it between, which every manifest declares.
func groupedImageService(project, environment string) string {
	return groupedResources(imageService, project, environment)
}

// groupedResources is groupedImageService for any resource lists. A grouping
// the fixture lacks is declared as well, for the apply to create.
func groupedResources(resources, project, environment string) string {
	indent := func(s, prefix string) string {
		lines := strings.SplitAfter(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = prefix + line
			}
		}
		return strings.Join(lines, "")
	}
	type grouping struct {
		name         string
		environments []string
	}
	groupings := []grouping{{"shop", []string{"staging", "production"}}, {"blog", []string{"production"}}}
	if project != "" {
		i := slices.IndexFunc(groupings, func(g grouping) bool { return g.name == project })
		if i < 0 {
			groupings, i = append(groupings, grouping{name: project}), len(groupings)
		}
		if !slices.Contains(groupings[i].environments, environment) {
			groupings[i].environments = append(groupings[i].environments, environment)
		}
	}
	var b strings.Builder
	b.WriteString("projects:\n")
	for _, p := range groupings {
		fmt.Fprintf(&b, "  - name: %s\n    environments:\n", p.name)
		for _, e := range p.environments {
			fmt.Fprintf(&b, "      - name: %s\n", e)
			if p.name == project && e == environment {
				b.WriteString(indent(resources, "        "))
			}
		}
	}
	if project == "" {
		b.WriteString("ungrouped:\n" + indent(resources, "  "))
	}
	return b.String()
}

// groupingAt is a grouping st holds, spelled as serviceRows spells a move's
// side: its project and environment ids.
func groupingAt(t *testing.T, st *blueprintGroupingTestStore, project, environment string) string {
	t.Helper()
	for _, p := range st.projects {
		for _, e := range st.environments {
			if p.Name == project && e.ProjectID == p.ID && e.Name == environment {
				return p.ID + "/" + e.ID
			}
		}
	}
	t.Fatalf("no grouping %s/%s", project, environment)
	return ""
}

// labelsAt is a's placement as its labels hold it, spelled like groupingAt.
func labelsAt(a *appv1alpha1.App) string {
	return a.Labels[core.LabelProject] + "/" + a.Labels[core.LabelEnvironment]
}

// regroupRows are serviceRows less the rows that create the groupings.
func regroupRows(sink *captureAuditSink) []string {
	return slices.DeleteFunc(serviceRows(sink), func(row string) bool { return strings.HasPrefix(row, "apps.BlueprintGrouping.") })
}

// groupingStackService is managedStackService whose store holds the
// Blueprint's projects and environments and the service rows' placements.
func groupingStackService(t *testing.T) (*Service, client.Client, *captureAuditSink, context.Context, *blueprintGroupingTestStore) {
	t.Helper()
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc, base, sink, ctx := managedStackService(t, st)
	svc.BlueprintGroups, svc.Environments = st, st
	return svc, base, sink, ctx, st
}

// refusingAppPatches is base with every App patch refused.
func refusingAppPatches(base client.Client) client.Client {
	return interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*appv1alpha1.App); ok {
				return errors.New("the API server refused the patch")
			}
			return cl.Patch(ctx, obj, patch, opts...)
		},
	})
}

// w9/093: a Blueprint that moves an existing service into another environment
// or project, or out of every one, records service_moved as the REST move
// does: environments.MoveService with the project and environment before and
// after, once, by the member who deployed it. Its create, and a re-deploy that
// keeps the grouping, record none.
func TestManagedBlueprintRegroupRecordsServiceMoved(t *testing.T) {
	svc, base, sink, ctx, st := groupingStackService(t)
	deploy := func(project, environment string) {
		t.Helper()
		if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedImageService(project, environment)}); err != nil {
			t.Fatalf("deploy into %q/%q: %v", project, environment, err)
		}
	}
	deploy("shop", "staging")
	if got := regroupRows(sink); len(got) != 0 {
		t.Fatalf("the Blueprint's create of the service recorded %v, want nothing", got)
	}
	app := getTenantApp(t, base, "tea-a", "web")
	if svc.blueprintIntentRow(app) == "" {
		t.Fatal("want a managed App")
	}
	staging, production, blog := groupingAt(t, st, "shop", "staging"), groupingAt(t, st, "shop", "production"), groupingAt(t, st, "blog", "production")
	for _, step := range []struct {
		name, project, environment string
		want                       []string
	}{
		{"another environment", "shop", "production", []string{"environments.MoveService " + staging + "->" + production + " by id-a"}},
		{"the same grouping", "shop", "production", nil},
		{"another project", "blog", "production", []string{"environments.MoveService " + production + "->" + blog + " by id-a"}},
		{"ungrouped", "", "", []string{"environments.MoveService " + blog + "->none by id-a"}},
		{"still ungrouped", "", "", nil},
		{"back into an environment", "shop", "staging", []string{"environments.MoveService none->" + staging + " by id-a"}},
	} {
		sink.events = nil
		deploy(step.project, step.environment)
		if got := regroupRows(sink); !slices.Equal(got, step.want) {
			t.Fatalf("%s recorded %v, want %v", step.name, got, step.want)
		}
		requireServiceTargets(t, sink, app)
		want := "/"
		if step.project != "" {
			want = groupingAt(t, st, step.project, step.environment)
		}
		if at := labelsAt(getTenantApp(t, base, "tea-a", "web")); at != want {
			t.Fatalf("%s left the App in %q, want %q", step.name, at, want)
		}
	}
}

// A managed App's move is recorded when its row moves, so an apply whose
// patch fails and its retry record it once, whether or not the projector
// brought the App's labels to the row's placement in between.
func TestManagedBlueprintRegroupIsRecordedOnceAcrossARetry(t *testing.T) {
	for _, resynced := range []bool{false, true} {
		t.Run(fmt.Sprintf("resynced=%t", resynced), func(t *testing.T) {
			svc, base, sink, ctx, st := groupingStackService(t)
			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedImageService("shop", "staging")}); err != nil {
				t.Fatalf("create: %v", err)
			}
			sink.events = nil
			staging, production := groupingAt(t, st, "shop", "staging"), groupingAt(t, st, "shop", "production")
			moved := groupedImageService("shop", "production")
			svc.Client = refusingAppPatches(base)
			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: moved}); err == nil {
				t.Fatal("want the deploy to fail at its patch")
			}
			want := []string{"environments.MoveService " + staging + "->" + production + " by id-a"}
			if got := regroupRows(sink); !slices.Equal(got, want) {
				t.Fatalf("the failed apply recorded %v, want the row's move %v", got, want)
			}
			app := getTenantApp(t, base, "tea-a", "web")
			if at := labelsAt(app); at != staging {
				t.Fatalf("the failed patch left the App in %q, want %q", at, staging)
			}
			if resynced { // the projector copies the row's placement onto the App
				project, environment, _ := strings.Cut(production, "/")
				app.Labels[core.LabelProject], app.Labels[core.LabelEnvironment] = project, environment
				if err := base.Update(ctx, app); err != nil {
					t.Fatal(err)
				}
			}

			svc.Client = base
			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: moved}); err != nil {
				t.Fatalf("retry: %v", err)
			}
			if got := regroupRows(sink); !slices.Equal(got, want) {
				t.Fatalf("the failed apply and its retry recorded %v, want %v", got, want)
			}
			if at := labelsAt(getTenantApp(t, base, "tea-a", "web")); at != production {
				t.Fatalf("the retry left the App in %q, want %q", at, production)
			}
		})
	}
}

// An App with no store row is placed by its labels alone, so its move is
// recorded once its patch lands: a refused patch records none, and the retry
// that lands records the move.
func TestUnmanagedBlueprintRegroupRecordsServiceMovedOnceItsPatchLands(t *testing.T) {
	svc, base, sink, ctx, st := groupingStackService(t)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedImageService("shop", "staging")}); err != nil {
		t.Fatalf("create: %v", err)
	}
	app := getTenantApp(t, base, "tea-a", "web")
	delete(app.Labels, store.LabelManagedBy)
	delete(app.Labels, store.LabelAppID)
	if err := base.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if svc.blueprintIntentRow(app) != "" {
		t.Fatal("want an App with no row")
	}
	sink.events = nil
	moved := groupedImageService("shop", "production")
	svc.Client = refusingAppPatches(base)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: moved}); err == nil {
		t.Fatal("want the deploy to fail at its patch")
	}
	if got := regroupRows(sink); len(got) != 0 {
		t.Fatalf("the refused patch recorded %v, want nothing", got)
	}
	svc.Client = base
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: moved}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	want := []string{"environments.MoveService " + groupingAt(t, st, "shop", "staging") + "->" + groupingAt(t, st, "shop", "production") + " by id-a"}
	if got := regroupRows(sink); !slices.Equal(got, want) {
		t.Fatalf("the landed retry recorded %v, want %v", got, want)
	}
	requireServiceTargets(t, sink, app)
}

// An automatic sync applies a push nobody ran by hand, so the move it makes
// names no caller, as its setting changes do.
func TestAutomaticBlueprintRegroupNamesNoCaller(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc.Store, svc.BlueprintGroups, svc.Environments = st, st, st
	git.byCommit[shaA] = groupedImageService("shop", "staging")
	git.byCommit[shaB] = groupedImageService("shop", "production")
	sink := &captureAuditSink{}
	svc.Audit = sink
	w := &BlueprintAutoSyncWorker{Svc: svc}
	git.headSHA = shaA
	w.process(context.Background(), claimOrderingIntent(t, fs, b, shaA, "create"))
	if got := regroupRows(sink); len(got) != 0 {
		t.Fatalf("the automatic create recorded %v, want nothing", got)
	}

	sink.events = nil
	git.headSHA = shaB
	w.process(context.Background(), claimOrderingIntent(t, fs, b, shaB, "regroup"))
	want := []string{"environments.MoveService " + groupingAt(t, st, "shop", "staging") + "->" + groupingAt(t, st, "shop", "production") + " by no caller"}
	if got := regroupRows(sink); !slices.Equal(got, want) {
		t.Fatalf("automatic sync recorded %v, want %v", got, want)
	}
}

// The approver of a planned sync applies it, so the move it makes, beside a
// setting change, names the approver; planning it records nothing. The plan
// lists both (w9/094).
func TestBlueprintPlanApprovalRegroupNamesTheApprover(t *testing.T) {
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

	svc.GitFetcher = fakeBlueprintFetcher{contents: strings.Replace(groupedImageService("shop", "production"), "plan: starter", "plan: standard", 1)}
	plan, err := svc.PlanBlueprint(creator, bpID, "tea-a")
	if err != nil || plan.PlanningOutcome != PlanningPlanned {
		t.Fatalf("plan = %+v err=%v, want a planned change", plan, err)
	}
	if got := plannedUpdates(plan.Sync.Actions); got != "update service web: environment, plan" {
		t.Fatalf("planned %q, want the move and the plan change", got)
	}
	if got := regroupRows(sink); len(got) != 0 {
		t.Fatalf("creating and planning recorded %v, want nothing", got)
	}
	if _, err := svc.ApproveBlueprint(approver, bpID, "tea-a", nil); err != nil {
		t.Fatalf("approve the change: %v", err)
	}
	want := []string{
		"apps.SetPlan starter->standard by approver",
		"environments.MoveService " + groupingAt(t, st, "shop", "staging") + "->" + groupingAt(t, st, "shop", "production") + " by approver",
	}
	if got := regroupRows(sink); !slices.Equal(got, want) {
		t.Fatalf("approval recorded %v, want %v", got, want)
	}
}
