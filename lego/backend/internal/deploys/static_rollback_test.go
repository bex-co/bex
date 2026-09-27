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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// staticSite is a no-build static site: published straight from its repo, so
// no deploy ever carries a ResolvedImage (ADR029).
func staticSite() *appv1alpha1.App {
	a := sampleApp("site", "srv-site")
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

// w4/m141: a no-build static site's deploys were never rollback targets (no
// ResolvedImage), so Rollback was refused and the dashboard's dispatch recheck
// silently dropped it. Rolling back now re-publishes the target's commit.
func TestRollbackRepublishesANoBuildStaticSitesEarlierCommit(t *testing.T) {
	ds := newFakeStore()
	older, _ := liveStaticDeploys(t, ds)
	svc, cl := newService(ds, staticSite())

	rolled, err := svc.Rollback(context.Background(), "site", older.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Trigger != store.TriggerRollback || rolled.RollbackOf != older.ID || rolled.CommitID != "80b423bc" {
		t.Fatalf("rollback deploy = %+v, want a rollback of %s at 80b423bc", rolled, older.ID)
	}
	got := getApp(t, cl, "site")
	if got.Spec.BuildCommit != "80b423bc" || got.Spec.Image != "" || got.Spec.RestartedAt == "" {
		t.Fatalf("app spec after rollback = buildCommit %q image %q restartedAt %q; want the earlier commit re-published",
			got.Spec.BuildCommit, got.Spec.Image, got.Spec.RestartedAt)
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
