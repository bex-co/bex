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

package controller

import (
	"context"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A held release's view takes what it builds and runs from its adopted spec
// and its operational settings from the saved one (w8/m121).
func TestReleaseViewSplitsReleaseFromOperationalFields(t *testing.T) {
	adopted := appv1alpha1.AppSpec{
		Repo: "https://github.com/bex-co/hello", Branch: "main", BuildCommit: "1111111111111111111111111111111111111111",
		Builder: "dockerfile", Port: 3000, StartCommand: "serve A", Replicas: 1,
		Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 10},
	}
	saved := appv1alpha1.AppSpec{
		Image: "ghcr.io/example/prebuilt:v1", Port: 3000, StartCommand: "serve B", Replicas: 3,
		Suspended: true, CloneSecret: "clone-new",
		Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 20},
	}
	view := releaseView(saved, adopted)

	if view.Repo != adopted.Repo || view.Image != "" || view.Branch != "main" || view.BuildCommit != adopted.BuildCommit || view.StartCommand != "serve A" {
		t.Fatalf("view must build and run the adopted release, got repo=%q image=%q branch=%q commit=%q start=%q",
			view.Repo, view.Image, view.Branch, view.BuildCommit, view.StartCommand)
	}
	if view.Replicas != 3 || !view.Suspended || view.CloneSecret != "clone-new" {
		t.Fatalf("view must keep saved operational settings, got replicas=%d suspended=%v clone=%q", view.Replicas, view.Suspended, view.CloneSecret)
	}
	if view.Disk == nil || view.Disk.SizeGB != 20 {
		t.Fatalf("view must keep the saved disk, got %+v", view.Disk)
	}
	if got, want := desiredAppReleaseIdentity(view).release, desiredAppReleaseIdentity(adopted).release; got != want {
		t.Fatalf("view release identity = %s, want the adopted release's %s", got, want)
	}

	remounted := saved
	remounted.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 20}
	if desiredAppReleaseIdentity(releaseView(remounted, adopted)).release == desiredAppReleaseIdentity(adopted).release {
		t.Fatal("a disk remounted since adoption must fail the view's identity check, so the view is not used")
	}
}

// The view swap must survive the status writes a pass makes: each decodes the
// stored, saved spec back into the App.
func TestKeepReleaseViewRestoresTheViewedSpec(t *testing.T) {
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "web", "default"
	app.Status.ReleaseGeneration = 2
	saved := appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/hello", Branch: "next"}
	view := appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/hello", Branch: "main"}
	ctx := withViewedRelease(context.Background(), app, view)
	if app.Spec.Branch != "main" || !viewedRelease(ctx, app) {
		t.Fatalf("withViewedRelease must run the pass on the view, got branch %q", app.Spec.Branch)
	}

	app.Spec = saved
	keepReleaseView(ctx, app)
	if app.Spec.Branch != "main" {
		t.Fatalf("keepReleaseView must re-apply the view after a write decoded the saved spec, got %q", app.Spec.Branch)
	}

	other := app.DeepCopy()
	other.Name, other.Spec = "other", saved
	keepReleaseView(ctx, other)
	if other.Spec.Branch != "next" || viewedRelease(ctx, other) {
		t.Fatal("a view applies only to its own App")
	}
}

// A saved Rollback or Restart selects a release: it overrides the release in
// flight, as before views, so no view holds that release behind it.
func TestHeldReleaseViewYieldsToASelectedRelease(t *testing.T) {
	app := overlapApp(appv1alpha1.PhaseBuilding, "rev-1")
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 3, SourceGeneration: 1, Image: "registry.example/app:gen-1"}
	if app.ActiveReleaseConfig() == nil {
		t.Fatal("precondition: the saved spec selects generation 1's release")
	}
	// No client: a selected release must be decided before any record read.
	view, err := (&AppReconciler{}).heldReleaseView(context.Background(), app)
	if err != nil || view != nil {
		t.Fatalf("heldReleaseView = %v, %v; want no view under a selected release", view, err)
	}
}
