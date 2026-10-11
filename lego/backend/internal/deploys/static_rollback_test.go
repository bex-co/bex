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

package deploys

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// staticSite is a no-build static site: published straight from its repo, so
// no deploy ever carries a ResolvedImage (ADR029).
func staticSite() *appv1alpha1.App {
	a := sampleApp("site", "srv-site")
	a.UID = "site-uid"
	a.Spec.PublishPath = "dist"
	a.Spec.Image = ""
	a.Spec.Type = appv1alpha1.TypeStaticSite
	a.Spec.Repo = "https://github.com/bex-co/bex"
	a.Spec.BuildCommit = "28aa2ab7"
	return a
}

// liveStaticDeploys records the pass-178 shape: an earlier publish of 80b423bc,
// now deactivated, and the live publish of 28aa2ab7. Neither has an image.
func liveStaticDeploys(t *testing.T, ds *fakeStore) (older, live store.Deploy) {
	t.Helper()
	ctx := context.Background()
	older, _ = ds.CreateDeploy(ctx, "srv-site", "create", "", 1, store.CommitInfo{Hash: "80b423bc", Message: "first"}, "")
	if won, err := ds.CloseDeploy(ctx, older.ID, store.DeployLive, ""); err != nil || !won {
		t.Fatalf("close older live: %v", err)
	}
	live, _ = ds.CreateDeploy(ctx, "srv-site", "api", "", 2, store.CommitInfo{Hash: "28aa2ab7", Message: "second"}, "")
	if won, err := ds.CloseDeploy(ctx, live.ID, store.DeployLive, ""); err != nil || !won {
		t.Fatalf("close live: %v", err)
	}
	if won, err := ds.CloseDeploy(ctx, older.ID, store.DeployDeactivated, ""); err != nil && won {
		t.Fatalf("deactivate older: %v", err)
	}
	older, _ = ds.GetDeploy(ctx, "srv-site", older.ID)
	live, _ = ds.GetDeploy(ctx, "srv-site", live.ID)
	return older, live
}

// staticRecord is the operator's record of where generation published
// (w4/m197), controlled by app as the operator writes it.
func staticRecord(app *appv1alpha1.App, generation int64, prefix string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: appv1alpha1.StaticPublicationRecordName(app.Name, generation), Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: app.Name, UID: app.UID, Controller: new(true),
			}},
		},
		Data: map[string][]byte{appv1alpha1.StaticPublicationPrefixKey: []byte(prefix)},
	}
}

// w4/m141 made a no-build static site's deploys rollback targets; w4/m197
// restores them from the retained publication instead of re-cloning the
// target's commit through today's saved rootDir/publishPath. Saved settings,
// including the build commit, stay as saved.
func TestRollbackRestoresANoBuildStaticSitesRetainedPublication(t *testing.T) {
	ds := newFakeStore()
	older, _ := liveStaticDeploys(t, ds)
	app := staticSite()
	app.Spec.PublishPath = "missing-output"
	svc, cl := newService(ds, app, staticRecord(app, 1, "tea-1/site/rev-1/"))

	rolled, err := svc.Rollback(context.Background(), "site", older.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Trigger != store.TriggerRollback || rolled.RollbackOf != older.ID || rolled.CommitID != "80b423bc" {
		t.Fatalf("rollback deploy = %+v, want a rollback of %s at 80b423bc", rolled, older.ID)
	}
	got := getApp(t, cl, "site")
	row, _ := ds.GetDeploy(context.Background(), "srv-site", rolled.ID)
	sel := got.Spec.ReleaseConfig
	if sel == nil || sel.StaticPrefix != "tea-1/site/rev-1/" || sel.Image != "" || sel.Generation != row.Generation {
		t.Fatalf("selection = %+v, want the retained rev-1 publication bound to the new release", sel)
	}
	if got.ActiveReleaseConfig() == nil {
		t.Fatal("an image-less static selection must still be the active release selection")
	}
	if got.Spec.BuildCommit != "28aa2ab7" || got.Spec.PublishPath != "missing-output" || got.Spec.Image != "" || got.Spec.RestartedAt == "" {
		t.Fatalf("saved settings after rollback = buildCommit %q publishPath %q image %q restartedAt %q; want them unchanged",
			got.Spec.BuildCommit, got.Spec.PublishPath, got.Spec.Image, got.Spec.RestartedAt)
	}
}

// The QA a79 journey: a built static site's v1 target restores its retained
// publication while today's saved publish directory does not exist and the
// saved build command writes v2.
func TestRollbackRestoresABuiltStaticSitesRetainedPublication(t *testing.T) {
	ctx := context.Background()
	ds := newFakeStore()
	v1, _ := ds.CreateDeploy(ctx, "srv-site", "create", "", 1, store.CommitInfo{Hash: "7b95b46"}, "")
	ds.CloseDeploy(ctx, v1.ID, store.DeployLive, "zot/site:gen-1@sha256:aa")
	v2, _ := ds.CreateDeploy(ctx, "srv-site", "config_change", "", 3, store.CommitInfo{Hash: "7b95b46"}, "")
	ds.CloseDeploy(ctx, v2.ID, store.DeployLive, "zot/site:gen-3@sha256:bb")
	ds.CloseDeploy(ctx, v1.ID, store.DeployDeactivated, "")
	app := staticSite()
	app.Spec.BuildCommit = ""
	app.Spec.BuildCommand = "mkdir -p dist && printf 'v2' > dist/index.html"
	app.Spec.PublishPath = "qa-a79-missing-output"
	app.Spec.AutoDeploy = false
	svc, cl := newService(ds, app, staticRecord(app, 1, "tea-1/site/rev-1/"), staticRecord(app, 3, "tea-1/site/rev-3/"))

	rolled, err := svc.Rollback(ctx, "site", v1.ID, RollbackOptions{DisableAutoDeploy: true})
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	got := getApp(t, cl, "site")
	sel := got.Spec.ReleaseConfig
	if sel == nil || sel.StaticPrefix != "tea-1/site/rev-1/" || sel.Image != "zot/site:gen-1@sha256:aa" || sel.SourceGeneration != 0 {
		t.Fatalf("selection = %+v, want v1's retained publication (image as provenance only)", sel)
	}
	if rolled.RollbackOf != v1.ID || got.Spec.PublishPath != "qa-a79-missing-output" || got.Spec.BuildCommand != app.Spec.BuildCommand || got.Spec.AutoDeploy {
		t.Fatalf("rollback %+v / saved %+v: want the exact target and saved settings unchanged", rolled, got.Spec)
	}
}

// A target whose publication was never recorded (published before w4/m197, or
// GC'd past the retained window), or whose record belongs to a same-name
// predecessor, is refused with a named reason before any row is created — and
// the capability projection agrees.
func TestStaticRollbackRefusesAnUnretainedPublicationBeforeDispatch(t *testing.T) {
	foreign := staticSite()
	foreign.UID = "a-deleted-predecessor"
	for name, objs := range map[string][]client.Object{
		"no record":      nil,
		"foreign record": {staticRecord(foreign, 1, "tea-1/site/rev-1/")},
		"bad prefix":     {staticRecord(staticSite(), 1, "../other/rev-1/")},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ds := newFakeStore()
			older, _ := liveStaticDeploys(t, ds)
			app := staticSite()
			svc, cl := newService(ds, append([]client.Object{app}, objs...)...)
			before, _ := ds.ListDeploys(ctx, "srv-site", store.DeployFilter{})

			_, err := svc.Rollback(ctx, "site", older.ID)
			if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), "no retained static publication") {
				t.Fatalf("Rollback = %v, want a named not-retained conflict", err)
			}
			after, _ := ds.ListDeploys(ctx, "srv-site", store.DeployFilter{})
			if len(after) != len(before) || getApp(t, cl, "site").Spec.ReleaseConfig != nil {
				t.Fatal("a refused rollback must not open a row or select anything")
			}
			_, _, pre := svc.deployPreconditions(ctx, app)
			if pre != core.PrecondNoEligibleRollbackTarget {
				t.Fatalf("projection = %q, want no_eligible_rollback_target like the verb", pre)
			}
		})
	}
}

func TestRollbackRefusesTheLiveStaticCommit(t *testing.T) {
	ds := newFakeStore()
	_, live := liveStaticDeploys(t, ds)
	svc, _ := newService(ds, staticSite())

	if _, err := svc.Rollback(context.Background(), "site", live.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("rollback to the live, already-published commit: err = %v, want ErrConflict", err)
	}
}

// The capability projection answers what the verb will do: a static site with
// an earlier published commit has a rollback target.
func TestStaticSiteRollbackCapabilityMatchesTheVerb(t *testing.T) {
	site := staticSite()
	older := store.Deploy{Status: store.DeployDeactivated, Commit: "80b423bc"}
	live := store.Deploy{Status: store.DeployLive, Commit: "28aa2ab7"}
	if !RollbackActionable(site, older) {
		t.Error("an earlier published static commit must be a rollback target")
	}
	if RollbackActionable(site, live) {
		t.Error("the live static commit is not a rollback target")
	}
	if RollbackEligible(site, store.Deploy{Status: store.DeployDeactivated}) {
		t.Error("a static deploy with no recorded commit has nothing to re-publish")
	}
	web := sampleApp("web", "srv-1")
	if RollbackEligible(web, store.Deploy{Status: store.DeployDeactivated, Commit: "80b423bc"}) {
		t.Error("a web service without an image stays ineligible: only static sites re-publish")
	}
}

func TestStaticRollbackHonorsDashboardAutoDeployOption(t *testing.T) {
	for _, dashboard := range []bool{false, true} {
		ds := newFakeStore()
		older, _ := liveStaticDeploys(t, ds)
		app := staticSite()
		app.Spec.AutoDeploy = true
		svc, cl := newService(ds, app, staticRecord(app, 1, "site/rev-1/"))
		if _, err := svc.Rollback(context.Background(), "site", older.ID, RollbackOptions{DisableAutoDeploy: dashboard}); err != nil {
			t.Fatal(err)
		}
		if got := getApp(t, cl, "site").Spec.AutoDeploy; got == dashboard {
			t.Fatalf("dashboard=%v: autoDeploy=%v", dashboard, got)
		}
	}
}
