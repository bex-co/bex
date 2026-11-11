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

// rollback_live_noop_test.go is w8/261. A rollback to the deploy that is
// already live must answer 409. It answered 201, restarted the instance and
// added a redundant deploy, because "already running" compared the live
// deploy's digest-pinned reference with the caller's spelling of the image.
// The fixtures here have production's shapes; the older tests closed a deploy
// with the same string spec.image held, which hid it.

// liveDeploy records one deploy of appID that went live, as the reconciler
// closes it: created from the saved image, resolved to a digest reference.
func liveDeploy(t *testing.T, ds *fakeStore, appID, image string, generation int64, resolved string) store.Deploy {
	t.Helper()
	ctx := context.Background()
	d, err := ds.CreateDeploy(ctx, appID, "create", image, generation, store.CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if won, err := ds.CloseDeploy(ctx, d.ID, store.DeployLive, resolved); err != nil || !won {
		t.Fatalf("close %s live: won=%v err=%v", d.ID, won, err)
	}
	return d
}

func wantAlreadyLive(t *testing.T, svc *Service, ds *fakeStore, appName, appID, deployID string) {
	t.Helper()
	before, _ := ds.ListDeploys(context.Background(), appID, store.DeployFilter{Limit: 50})
	_, err := svc.Rollback(context.Background(), appName, deployID)
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("rollback to the live deploy = %v, want the already-live conflict", err)
	}
	if after, _ := ds.ListDeploys(context.Background(), appID, store.DeployFilter{Limit: 50}); len(after) != len(before) {
		t.Fatalf("the refused rollback added a deploy row: %d, was %d", len(after), len(before))
	}
}

func TestRollbackToTheLiveDeployIsRefusedHoweverItsImageIsSpelled(t *testing.T) {
	t.Run("an image service's only deploy: a tag saved, a digest run", func(t *testing.T) {
		ds := newFakeStore()
		app := sampleApp("web", "srv-1")
		app.Spec.Image = "docker.io/traefik/whoami:v1.10.1"
		d := liveDeploy(t, ds, "srv-1", app.Spec.Image, 1, "docker.io/traefik/whoami@sha256:6956a30c")
		svc, _ := newService(ds, app)
		if pre := rollbackPrecondition(t, svc, "web"); pre != core.PrecondNoEligibleRollbackTarget {
			t.Errorf("deployActions rollback precondition = %q, want no_eligible_rollback_target", pre)
		}
		wantAlreadyLive(t, svc, ds, "web", "srv-1", d.ID)
	})

	t.Run("a repository service: built images, no saved image", func(t *testing.T) {
		ds := newFakeStore()
		app := sampleApp("web", "srv-1")
		app.Spec.Image, app.Spec.Repo = "", "https://github.com/a/app"
		first := liveDeploy(t, ds, "srv-1", "", 1, "zot.bex-registry.svc:5000/ws/web:gen-1@sha256:aaaa")
		second := liveDeploy(t, ds, "srv-1", "", 2, "zot.bex-registry.svc:5000/ws/web:gen-2@sha256:bbbb")
		svc, _ := newService(ds, app)
		wantAlreadyLive(t, svc, ds, "web", "srv-1", second.ID)
		// The previous build is still a target, so the action stays offered.
		if !RollbackActionable(app, mustDeploy(t, ds, "srv-1", first.ID)) {
			t.Error("the previous live deploy is no longer a rollback target")
		}
		if pre := rollbackPrecondition(t, svc, "web"); pre != "" {
			t.Errorf("deployActions rollback precondition = %q, want none", pre)
		}
	})

	t.Run("a live deploy made with an image tag override", func(t *testing.T) {
		ds := newFakeStore()
		app := sampleApp("web", "srv-1")
		app.Spec.Image = "docker.io/traefik/whoami:v1.10.1"
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 2, Image: "docker.io/traefik/whoami:v1.10.4"}
		app.Status.ReleaseGeneration = 2
		liveDeploy(t, ds, "srv-1", app.Spec.Image, 1, "docker.io/traefik/whoami@sha256:6956a30c")
		override := liveDeploy(t, ds, "srv-1", "docker.io/traefik/whoami:v1.10.4", 2, "docker.io/traefik/whoami@sha256:1699d99c")
		svc, _ := newService(ds, app)
		if app.ActiveReleaseConfig() == nil {
			t.Fatal("the fixture's selection is not active")
		}
		wantAlreadyLive(t, svc, ds, "web", "srv-1", override.ID)
	})
}

// The recovery arms stay: a live deploy is a target again once something
// newer was asked for and did not go live.
func TestRollbackToTheLiveDeployStaysARecovery(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "docker.io/traefik/whoami:v1.10.1"
	d := mustDeploy(t, ds, "srv-1", liveDeploy(t, ds, "srv-1", app.Spec.Image, 1, "docker.io/traefik/whoami@sha256:6956a30c").ID)

	// A newer release was requested (a failed configuration-only release).
	requested := app.DeepCopy()
	requested.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}
	if !RollbackActionable(requested, d) {
		t.Error("with a newer release requested, the live deploy must be a recovery target")
	}
	// The saved image moved off the one the deploy was created from.
	drifted := app.DeepCopy()
	drifted.Spec.Image = "docker.io/traefik/whoami:bad"
	if !RollbackActionable(drifted, d) {
		t.Error("with the saved image drifted, the live deploy must be a recovery target")
	}
	// Neither: it is what is running.
	if RollbackActionable(app, d) {
		t.Error("the live deploy of an unchanged service is not a target")
	}
	// Nor when the saved image is spelled as the reference that ran, whatever
	// the row was created from.
	pinned := app.DeepCopy()
	pinned.Spec.Image = d.ResolvedImage
	if RollbackActionable(pinned, d) {
		t.Error("a saved image equal to the reference that ran is not drift")
	}
	// A row from before deploys recorded their release keeps the old rule.
	legacy := d
	legacy.Generation = 0
	if !RollbackActionable(requested, legacy) || !RollbackActionable(app, legacy) {
		t.Error("a generation-0 row whose reference differs from the saved image must stay a target")
	}
	if RollbackActionable(pinned, legacy) {
		t.Error("a generation-0 row running the saved reference is not a target")
	}
}

func rollbackPrecondition(t *testing.T, svc *Service, appName string) string {
	t.Helper()
	acts, err := svc.ActionCapabilities(context.Background(), appName)
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	return actionByID(t, acts, core.ActionRollback).Precondition
}

func mustDeploy(t *testing.T, ds *fakeStore, appID, id string) store.Deploy {
	t.Helper()
	d, err := ds.GetDeploy(context.Background(), appID, id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
