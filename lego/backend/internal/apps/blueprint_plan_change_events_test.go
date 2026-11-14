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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// planManifest is ownershipManifest's web service on plan.
func planManifest(plan string) string {
	return ownershipManifest + "    plan: " + plan + "\n"
}

// saveCachingProfile saves a caching profile on a tenant's App directly: a
// Blueprint declares none, and a Free service refuses SetCacheProfile.
func saveCachingProfile(t *testing.T, cl client.Client, tenantID, name string) {
	t.Helper()
	app := getTenantApp(t, cl, tenantID, name)
	app.Spec.CacheProfile = appv1alpha1.CacheProfileStatic
	if err := cl.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
}

// planChangeRows names the plan_changed and edge-cache rows sink recorded, in
// order, with the caller each names.
func planChangeRows(sink *captureAuditSink) []string {
	var out []string
	for _, ev := range sink.events {
		by := ev.Caller
		if by == "" {
			by = "no caller"
		}
		switch ev.Verb {
		case core.AuditVerbSetPlan:
			out = append(out, "plan "+ptr.Deref(ev.PlanFrom, "")+"->"+ptr.Deref(ev.PlanTo, "")+" by "+by)
		case core.AuditVerbSetPlanEdgeCache, core.AuditVerbSetCacheProfile:
			out = append(out, ev.Verb+" caching "+cacheToggle(ev.EdgeCacheEnabledTo)+" by "+by)
		}
	}
	return out
}

// w9/091: a Blueprint sync that changes a service's plan records
// plan_changed, as SetPlan does, and the system edge-cache toggle when the
// change crosses Free for a service with a caching profile, both naming the
// member who ran the sync. A sync that keeps the plan records neither, and the
// Blueprint's create of the service is no plan change.
func TestBlueprintSyncPlanChangeRecordsPlanChanged(t *testing.T) {
	const toggle = core.AuditVerbSetPlanEdgeCache
	for _, tc := range []struct {
		name     string
		from     string
		cached   bool
		manifest string
		want     []string
	}{
		{"a cached service moved from paid to Free", "standard", true, planManifest("free"),
			[]string{"plan standard->free by id-a", toggle + " caching off by id-a"}},
		{"a cached service moved from Free to paid", "free", true, planManifest("starter"),
			[]string{"plan free->starter by id-a", toggle + " caching on by id-a"}},
		{"a cached service moved between paid plans", "standard", true, planManifest("pro"),
			[]string{"plan standard->pro by id-a"}},
		{"an uncached service moved to Free", "standard", false, planManifest("free"),
			[]string{"plan standard->free by id-a"}},
		{"a cached service whose sync keeps its plan", "standard", true,
			strings.Replace(planManifest("standard"), "nginx:1", "nginx:2", 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := ownershipService(t)
			sink := &captureAuditSink{}
			svc.Audit = sink
			ctx := ownershipCtx()
			svc.GitFetcher = fakeBlueprintFetcher{contents: planManifest(tc.from), sha: "abc1234"}
			bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if got := planChangeRows(sink); len(got) != 0 {
				t.Fatalf("the Blueprint's create of the service recorded %v, want no plan change", got)
			}
			if tc.cached {
				saveCachingProfile(t, svc.Client, "tea-a", "web")
			}

			sink.events = nil
			svc.GitFetcher = fakeBlueprintFetcher{contents: tc.manifest, sha: "def5678"}
			if _, err := svc.SyncBlueprint(ctx, bp.ID, "tea-a", "", "", nil); err != nil {
				t.Fatalf("sync: %v", err)
			}
			if got := planChangeRows(sink); !slices.Equal(got, tc.want) {
				t.Fatalf("sync recorded %v, want %v", got, tc.want)
			}
			app := getTenantApp(t, svc.Client, "tea-a", "web")
			if tc.cached && app.Spec.CacheProfile != appv1alpha1.CacheProfileStatic {
				t.Fatalf("saved profile = %q, want it kept across the sync", app.Spec.CacheProfile)
			}
			for _, ev := range sink.events {
				if (ev.Verb == core.AuditVerbSetPlan || ev.Verb == toggle) && ev.Target != core.ServiceTarget(core.AppPublicID(app)) {
					t.Fatalf("%s row targets %q, want the service", ev.Verb, ev.Target)
				}
			}
		})
	}
}

// An automatic sync applies a push nobody ran by hand: its plan change and
// toggle name no caller, as its deploy row names no user.
func TestAutomaticBlueprintSyncPlanChangeNamesNoCaller(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	git.byCommit[shaA], git.byCommit[shaB] = planManifest("standard"), planManifest("free")
	sink := &captureAuditSink{}
	svc.Audit = sink
	w := &BlueprintAutoSyncWorker{Svc: svc}
	git.headSHA = shaA
	w.process(context.Background(), claimOrderingIntent(t, fs, b, shaA, "create"))
	if got := planChangeRows(sink); len(got) != 0 {
		t.Fatalf("the automatic create recorded %v, want no plan change", got)
	}
	saveCachingProfile(t, svc.Client, b.TenantID, "web")

	sink.events = nil
	git.headSHA = shaB
	w.process(context.Background(), claimOrderingIntent(t, fs, b, shaB, "to-free"))
	want := []string{"plan standard->free by no caller", core.AuditVerbSetPlanEdgeCache + " caching off by no caller"}
	if got := planChangeRows(sink); !slices.Equal(got, want) {
		t.Fatalf("automatic sync recorded %v, want %v", got, want)
	}
	if tier := getTenantApp(t, svc.Client, b.TenantID, "web").Spec.Tier; tier != "free" {
		t.Fatalf("tier = %q, want the synced free", tier)
	}
}

// The approver of a planned sync applies it, so its plan change names the
// approver, not the member who connected the Blueprint. Planning the change
// records nothing.
func TestBlueprintPlanApprovalPlanChangeNamesTheApprover(t *testing.T) {
	svc := newBlueprintService(newFakeBlueprintStore(), fakeWorkspace{"user-a": "tea-a", "approver": "tea-a"})
	svc.GitFetcher = fakeBlueprintFetcher{contents: planManifest("standard")}
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
	if _, err := svc.ApproveBlueprint(creator, bpID, "tea-a", "", nil); err != nil {
		t.Fatalf("approve the create: %v", err)
	}
	saveCachingProfile(t, svc.Client, "tea-a", "web")

	svc.GitFetcher = fakeBlueprintFetcher{contents: planManifest("free")}
	if plan, err := svc.PlanBlueprint(creator, bpID, "tea-a"); err != nil || plan.PlanningOutcome != PlanningPlanned {
		t.Fatalf("plan = %+v err=%v, want a planned change", plan, err)
	}
	if got := planChangeRows(sink); len(got) != 0 {
		t.Fatalf("creating and planning recorded %v, want no plan change", got)
	}
	if _, err := svc.ApproveBlueprint(approver, bpID, "tea-a", "", nil); err != nil {
		t.Fatalf("approve the plan change: %v", err)
	}
	want := []string{"plan standard->free by approver", core.AuditVerbSetPlanEdgeCache + " caching off by approver"}
	if got := planChangeRows(sink); !slices.Equal(got, want) {
		t.Fatalf("approval recorded %v, want %v", got, want)
	}
}

// A direct deploy (POST /v1/blueprints/deploy, MCP deploy) runs the same stack
// apply, so a store-less App's plan change records the same rows once its
// patch lands, by the member who deployed.
func TestDeployStackPlanChangeRecordsPlanChanged(t *testing.T) {
	svc, cl := newService(nil)
	sink := &captureAuditSink{}
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "deployer", Method: "oauth2"})
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planManifest("standard")}); err != nil {
		t.Fatalf("create: %v", err)
	}
	app := getApp(t, cl, "web")
	if svc.blueprintIntentRow(app) != "" {
		t.Fatal("want a store-less App")
	}
	app.Spec.CacheProfile = appv1alpha1.CacheProfileStatic
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if got := planChangeRows(sink); len(got) != 0 {
		t.Fatalf("the deploy's create recorded %v, want no plan change", got)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planManifest("free")}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	want := []string{"plan standard->free by deployer", core.AuditVerbSetPlanEdgeCache + " caching off by deployer"}
	if got := planChangeRows(sink); !slices.Equal(got, want) {
		t.Fatalf("deploy recorded %v, want %v", got, want)
	}
}

// A managed App's row is its plan's source of truth: once the row takes the
// new plan, a resync copies it onto the CR whatever step of the apply fails
// next, and a retry finds nothing to change. So the change is recorded when
// the row lands, exactly once across the failed apply and its retry.
func TestManagedStackPlanChangeIsRecordedOnceTheRowLands(t *testing.T) {
	rec := &recordingStore{}
	svc, base := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	svc.DomainTraffic = allowDomainTraffic{}
	sink := &captureAuditSink{}
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planManifest("standard")}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if svc.blueprintIntentRow(getTenantApp(t, base, "tea-a", "web")) == "" {
		t.Fatal("want a managed App")
	}

	svc.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*appv1alpha1.App); ok {
				return errors.New("the API server refused the patch")
			}
			return cl.Patch(ctx, obj, patch, opts...)
		},
	})
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planManifest("free")}); err == nil {
		t.Fatal("want the deploy to fail at its patch")
	}
	if n := len(rec.tierCalls); n == 0 || rec.tierCalls[n-1].tier != "free" {
		t.Fatalf("tier rows = %+v, want the row to have taken free", rec.tierCalls)
	}
	// The projector's resync restores the CR from the row.
	app := getTenantApp(t, base, "tea-a", "web")
	if app.Spec.Tier != "standard" {
		t.Fatalf("CR tier = %q, want the failed patch to have left standard", app.Spec.Tier)
	}
	app.Spec.Tier = "free"
	if err := base.Update(ctx, app); err != nil {
		t.Fatal(err)
	}

	svc.Client = base
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planManifest("free")}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	want := []string{"plan standard->free by id-a"}
	if got := planChangeRows(sink); !slices.Equal(got, want) {
		t.Fatalf("the failed apply and its retry recorded %v, want %v", got, want)
	}
}
