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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rollback_build_stored_test.go is w8/m156 t003. The in-cluster registry keeps
// a bounded history, so the build an old deploy ran on can be gone. A rollback
// to it answered 201 and failed at the pull minutes later. The operator
// publishes which manifests the registry holds; a target that is not among
// them is refused at once and is not offered.

const (
	clusterRepo = "zot.bex-registry.svc:5000/ws/web"
	digestOld   = "sha256:" + "1111111111111111111111111111111111111111111111111111111111111111"
	digestPrev  = "sha256:" + "2222222222222222222222222222222222222222222222222222222222222222"
	digestLive  = "sha256:" + "3333333333333333333333333333333333333333333333333333333333333333"
)

// storedBuildFixture is a repository service with three builds live in turn.
// The registry holds the two newest.
func storedBuildFixture(t *testing.T) (svc *Service, ds *fakeStore, app *appv1alpha1.App, old, prev, live store.Deploy) {
	t.Helper()
	ds = newFakeStore()
	app = sampleApp("web", "srv-1")
	app.Spec.Image, app.Spec.Repo = "", "https://github.com/a/app"
	app.Status.Image = clusterRepo + ":gen-3@" + digestLive
	app.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: 3, Digest: digestLive}
	app.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: 3, Digests: []string{digestPrev, digestLive}}
	old = liveDeploy(t, ds, "srv-1", "", 1, clusterRepo+":gen-1@"+digestOld)
	prev = liveDeploy(t, ds, "srv-1", "", 2, clusterRepo+":gen-2@"+digestPrev)
	live = liveDeploy(t, ds, "srv-1", "", 3, clusterRepo+":gen-3@"+digestLive)
	svc, _ = newService(ds, app)
	return svc, ds, app, mustDeploy(t, ds, "srv-1", old.ID), mustDeploy(t, ds, "srv-1", prev.ID), mustDeploy(t, ds, "srv-1", live.ID)
}

func TestRollbackToABuildThatIsNoLongerStoredIsRefused(t *testing.T) {
	ctx := context.Background()
	svc, ds, _, old, prev, _ := storedBuildFixture(t)
	before, _ := ds.ListDeploys(ctx, "srv-1", store.DeployFilter{Limit: 50})

	_, err := svc.Rollback(ctx, "web", old.ID)
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("rollback to a missing build = %v, want a conflict", err)
	}
	// It names the deploy and the build in the tenant's terms, and what to do.
	for _, want := range []string{old.ID, "ws/web:gen-1", "no longer stored", "Deploy its commit again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "zot.bex-registry.svc") {
		t.Errorf("refusal prints the registry's cluster-internal host: %q", err)
	}
	if after, _ := ds.ListDeploys(ctx, "srv-1", store.DeployFilter{Limit: 50}); len(after) != len(before) {
		t.Fatalf("the refused rollback added a deploy row: %d, was %d", len(after), len(before))
	}

	// A stored build still rolls back.
	if _, err := svc.Rollback(ctx, "web", prev.ID); err != nil {
		t.Fatalf("rollback to a stored build: %v", err)
	}
}

func TestDeployListMarksTheBuildsThatAreGone(t *testing.T) {
	svc, _, _, old, prev, live := storedBuildFixture(t)
	rows, err := svc.List(context.Background(), "web", ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	gone := map[string]bool{}
	for _, r := range rows {
		gone[r.ID] = r.BuildGone
	}
	if !gone[old.ID] || gone[prev.ID] || gone[live.ID] {
		t.Errorf("BuildGone = %v, want only the oldest deploy's build gone", gone)
	}
	one, err := svc.Get(context.Background(), "web", old.ID)
	if err != nil || !one.BuildGone {
		t.Errorf("Get(old) BuildGone = %v (err %v), want true", one.BuildGone, err)
	}
}

// With every earlier build gone, the action is not offered at all.
func TestDeployActionsOfferNoRollbackWhenEveryEarlierBuildIsGone(t *testing.T) {
	svc, _, app, _, _, _ := storedBuildFixture(t)
	if pre := rollbackPrecondition(t, svc, "web"); pre != "" {
		t.Fatalf("with the previous build stored, rollback precondition = %q, want none", pre)
	}
	app.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: 3, Digests: []string{digestLive}}
	svc, _ = newService(svc.Store.(*fakeStore), app)
	if pre := rollbackPrecondition(t, svc, "web"); pre != core.PrecondNoEligibleRollbackTarget {
		t.Errorf("rollback precondition = %q, want no_eligible_rollback_target", pre)
	}
}

// Nothing is refused on a guess: where the registry's holdings do not cover a
// deploy's image, the build counts as stored.
func TestBuildStoredOnlyJudgesWhatTheListingCovers(t *testing.T) {
	_, _, app, old, prev, live := storedBuildFixture(t)
	if BuildStored(app, old) {
		t.Fatal("the fixture's oldest build must read as gone")
	}
	noListing := app.DeepCopy()
	noListing.Status.StoredImages = nil
	// A stale reading: taken at generation 1, before the two later builds
	// existed. It may say the first build is gone; it knows nothing of them.
	stale := app.DeepCopy()
	stale.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: 1, Digests: []string{digestOld}}
	// A reading that somehow lacks the build that is serving.
	lacksServing := app.DeepCopy()
	lacksServing.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: 3, Digests: []string{digestPrev}}
	external := old
	external.ResolvedImage = "docker.io/traefik/whoami@" + digestOld
	otherRepo := old
	otherRepo.ResolvedImage = "zot.bex-registry.svc:5000/ws/build-source:gen-1@" + digestOld
	noImage := old
	noImage.ResolvedImage = ""
	tagOnly := old
	tagOnly.ResolvedImage = clusterRepo + ":gen-1"
	noGeneration := old
	noGeneration.Generation = 0
	for name, stored := range map[string]bool{
		"no listing published yet":           BuildStored(noListing, old),
		"a build made after the reading":     BuildStored(stale, prev) && BuildStored(stale, live),
		"the serving build, unlisted":        BuildStored(lacksServing, live),
		"an external image":                  BuildStored(app, external),
		"another repository's build":         BuildStored(app, otherRepo),
		"a deploy with no image":             BuildStored(app, noImage),
		"a reference by tag alone":           BuildStored(app, tagOnly),
		"a deploy with no generation":        BuildStored(app, noGeneration),
		"a stale reading still knows a gone": !BuildStored(lacksServing, old),
	} {
		if !stored {
			t.Errorf("%s: wrong verdict", name)
		}
	}
}
