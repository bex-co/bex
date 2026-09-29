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

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// release_config_snapshot_test.go pins w1/m152 t002: a cancel over a release that
// served reverts CONFIGURATION with the image. Before it, settleCanceledRelease
// dispatched status.image against the current spec, so a canceled config_change
// still shipped the canceled values.

// servedAppWithGroup is a service that served generation 3 and has since started
// release 5 (the one being canceled), linked to one env group, with the release-5
// snapshot currently projected.
func servedAppWithGroup(ns string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: ns, Generation: 5},
		Spec: appv1alpha1.AppSpec{
			Image:          "ghcr.io/bex-co/api@sha256:v5",
			Port:           3000,
			EnvFromSecrets: []string{"evg-shared-env"},
			EnvFromSecret:  "api-env",
		},
		Status: appv1alpha1.AppStatus{
			Image:                    "ghcr.io/bex-co/api@sha256:v3",
			ActiveRevision:           "rev-3",
			ReleaseGeneration:        5,
			ConfigSnapshotGeneration: 5,
			ObservedGeneration:       5,
		},
	}
}

// snapshotTestNS is the one namespace every fixture here lives in.
const snapshotTestNS = "tea-m152"

func secret(name, val string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: snapshotTestNS},
		Data:       map[string][]byte{"MESSAGE": []byte(val)},
	}
}

func envNames(app *appv1alpha1.App) []string {
	sources := envFromSources(app)
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		out = append(out, s.SecretRef.Name)
	}
	return out
}

func TestSettleConfigSnapshotRevertsToServedRelease(t *testing.T) {
	ctx := context.Background()
	ns := snapshotTestNS
	app := servedAppWithGroup(ns)
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app,
		// the served release's copies, taken when generation 3 dispatched
		secret("api-evg-shared-env-r3", "group-v3"),
		secret("api-env-r3", "OK"),
		// the canceled release's copies
		secret("api-evg-shared-env-r5", "group-v5"),
		secret("api-env-r5", "should-not-ship"),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if got := envNames(app); got[1] != "api-env-r5" {
		t.Fatalf("precondition: projecting %v, want the canceled release's copies", got)
	}

	moved, err := r.settleConfigSnapshotTo(ctx, app, successfulReleaseGeneration(app))
	if err != nil || !moved {
		t.Fatalf("settleConfigSnapshotTo = (%v, %v), want it to move onto generation 3", moved, err)
	}
	got := envNames(app)
	want := []string{"api-evg-shared-env-r3", "api-env-r3"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("after cancel the template reads %v, want %v — config must revert with the image", got, want)
	}

	// The saved spec is left alone: the canceled change stays saved, and a later
	// deploy ships it (t002 step 2).
	if app.Spec.EnvFromSecret != "api-env" || app.Spec.Image != "ghcr.io/bex-co/api@sha256:v5" {
		t.Fatalf("settle rewrote the saved spec: %+v", app.Spec)
	}
}

// A served release with no snapshots — it predates t001, or GC reclaimed it beyond
// the retained window — leaves the projection where it is. Restoring the image
// only is the honest outcome, and pointing at copies that do not exist would give
// pods an unresolvable optional Secret: an environment silently dropped.
func TestSettleConfigSnapshotLeavesProjectionWhenTargetHasNoCopy(t *testing.T) {
	ctx := context.Background()
	ns := snapshotTestNS
	app := servedAppWithGroup(ns)
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app,
		secret("api-evg-shared-env-r5", "group-v5"),
		secret("api-env-r5", "should-not-ship"),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	moved, err := r.settleConfigSnapshotTo(ctx, app, 3)
	if err != nil || moved {
		t.Fatalf("settleConfigSnapshotTo = (%v, %v), want no move", moved, err)
	}
	if app.Status.ConfigSnapshotGeneration != 5 {
		t.Fatalf("projection moved to %d with no copy to point at", app.Status.ConfigSnapshotGeneration)
	}
}

// All-or-nothing: if even one source of the target release is missing, the
// projection does not move. Pointing some sources at generation 3 and leaving
// others on 5 would make precedence depend on which copy survived.
func TestSettleConfigSnapshotIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	ns := snapshotTestNS
	app := servedAppWithGroup(ns)
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app,
		secret("api-env-r3", "OK"), // the group's r3 copy is gone
		secret("api-evg-shared-env-r5", "group-v5"),
		secret("api-env-r5", "should-not-ship"),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if moved, err := r.settleConfigSnapshotTo(ctx, app, 3); err != nil || moved {
		t.Fatalf("settleConfigSnapshotTo = (%v, %v), want no move on a partial set", moved, err)
	}
	for _, n := range envNames(app) {
		if n == "api-env-r3" {
			t.Fatalf("projection mixed generations: %v", envNames(app))
		}
	}
}

// GC counts generations behind the current release, keeps the retained window and
// the current generation, and leaves another App's snapshots alone.
func TestGCReleaseConfigSnapshotsKeepsWindowAndOwnership(t *testing.T) {
	ctx := context.Background()
	ns := snapshotTestNS
	app := servedAppWithGroup(ns)
	app.UID = "app-uid"
	app.Status.ReleaseGeneration = 30
	other := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: ns, UID: "other-uid"}}

	owned := func(name string, gen string, owner *appv1alpha1.App) *corev1.Secret {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			Labels: map[string]string{snapshotGenerationLabel: gen, snapshotOfLabel: "api-env"},
		}}
		truth := true
		s.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "app.bex.co/v1alpha1", Kind: "App", Name: owner.Name, UID: owner.UID, Controller: &truth,
		}}
		return s
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, other,
		owned("api-env-r9", "9", app),   // 21 behind: reclaimed
		owned("api-env-r10", "10", app), // exactly at the cutoff: reclaimed
		owned("api-env-r11", "11", app), // inside the window: kept
		owned("api-env-r30", "30", app), // current: kept
		owned("other-env-r1", "1", other),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if err := r.gcReleaseConfigSnapshots(ctx, app); err != nil {
		t.Fatalf("gc: %v", err)
	}
	exists := func(name string) bool {
		return cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &corev1.Secret{}) == nil
	}
	for name, want := range map[string]bool{
		"api-env-r9": false, "api-env-r10": false, "api-env-r11": true, "api-env-r30": true,
		"other-env-r1": true,
	} {
		if exists(name) != want {
			t.Errorf("%s exists=%v, want %v", name, !want, want)
		}
	}
}

// The rollout invariant (t003 step 1): only a template change made while settling a
// cancel is metered, and a reprojection — the fallback that can ship canceled
// changes — is distinguishable from a restore.
func TestAfterServingDeploymentMetersCancelTemplateChanges(t *testing.T) {
	ctx := context.Background()
	settling := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: "api", Namespace: snapshotTestNS, Generation: 2,
				Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "2"},
			},
			Status: appv1alpha1.AppStatus{Image: "img@sha256:v1", ActiveRevision: "rev-1", ReleaseGeneration: 1},
		}
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(settling()).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
	count := func(kind string) float64 { return testutil.ToFloat64(cancelTemplateChangesTotal.WithLabelValues(kind)) }

	restore0, reproject0 := count(cancelTemplateRestore), count(cancelTemplateReproject)
	app := settling()
	r.afterServingDeployment(ctx, app, corev1.PodTemplateSpec{}, true, true)
	r.afterServingDeployment(ctx, app, corev1.PodTemplateSpec{}, true, false)
	r.afterServingDeployment(ctx, app, corev1.PodTemplateSpec{}, false, false) // unchanged: not a rollout
	if got := count(cancelTemplateRestore) - restore0; got != 1 {
		t.Errorf("restore increments = %v, want 1", got)
	}
	if got := count(cancelTemplateReproject) - reproject0; got != 1 {
		t.Errorf("reproject increments = %v, want 1", got)
	}
	if !app.Status.UndeployedChanges {
		t.Error("settling a cancel over a served release must report undeployed changes")
	}

	normal := settling()
	normal.Annotations = nil
	before := count(cancelTemplateReproject) + count(cancelTemplateRestore)
	normal.Status.UndeployedChanges = true
	r.afterServingDeployment(ctx, normal, corev1.PodTemplateSpec{}, true, false)
	if normal.Status.UndeployedChanges {
		t.Error("a normal dispatch must clear undeployed changes")
	}
	if after := count(cancelTemplateReproject) + count(cancelTemplateRestore); after != before {
		t.Errorf("a normal rollout was metered as a cancel template change (%v -> %v)", before, after)
	}
}

// A group's Secret lives in the workspace namespace and is read by every linked
// service, while generations are per App. Two services at the same generation
// must each get their own copy of the group, taken when THEY dispatched (t004).
func TestLinkedServicesSnapshotASharedGroupSeparately(t *testing.T) {
	ctx := context.Background()
	linked := func(name string) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: snapshotTestNS, Generation: 3},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, EnvFromSecrets: []string{"evg-shared-env"}},
			Status:     appv1alpha1.AppStatus{ReleaseGeneration: 3},
		}
	}
	a, b := linked("svc-a"), linked("svc-b")
	group := secret("evg-shared-env", "v1")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(a, b, group).WithStatusSubresource(a, b).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if err := r.ensureReleaseConfigSnapshot(ctx, a); err != nil {
		t.Fatal(err)
	}
	// The group is saved between the two services' dispatches.
	group.Data["MESSAGE"] = []byte("v2")
	if err := cl.Update(ctx, group); err != nil {
		t.Fatal(err)
	}
	if err := r.ensureReleaseConfigSnapshot(ctx, b); err != nil {
		t.Fatal(err)
	}

	for app, want := range map[*appv1alpha1.App]string{a: "v1", b: "v2"} {
		ref := envNames(app)[0]
		var snap corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: ref}, &snap); err != nil {
			t.Fatalf("%s reads %s: %v", app.Name, ref, err)
		}
		if got := string(snap.Data["MESSAGE"]); got != want {
			t.Errorf("%s reads group value %q from %s, want %q — another service's copy", app.Name, got, ref, want)
		}
		if !metav1.IsControlledBy(&snap, app) {
			t.Errorf("%s's group copy %s is owned by another service, whose GC or deletion would drop it", app.Name, ref)
		}
	}
}

// An App already projecting the first build's unscoped group copy keeps
// projecting it — switching names would roll the service with no deploy — and
// co-owns the copy, so the service that created it cannot delete it from under
// this one. Its next release moves to scoped names.
func TestAdoptUnscopedSnapshotsKeepsTheTemplateAndCoOwns(t *testing.T) {
	ctx := context.Background()
	app := servedAppWithGroup(snapshotTestNS)
	app.UID = "uid-api"
	other := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: snapshotTestNS, UID: "uid-worker"}}
	legacy := secret("evg-shared-env-r5", "running")
	legacy.Labels = map[string]string{snapshotOfLabel: "evg-shared-env", snapshotGenerationLabel: "5"}
	sch := deletionScheme(t)
	if err := controllerutil.SetControllerReference(other, legacy, sch); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(app, other, legacy,
		secret("evg-shared-env", "saved-since"),
		secret("api-env-r5", "OK"),
	).WithStatusSubresource(app).Build()
	r := &AppReconciler{Client: cl, Scheme: sch}

	before := envNames(app)
	if before[0] != "api-evg-shared-env-r5" {
		t.Fatalf("precondition: without the marker the projection is scoped, got %v", before)
	}
	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
		t.Fatal(err)
	}
	if got := envNames(app); got[0] != "evg-shared-env-r5" || got[1] != "api-env-r5" {
		t.Fatalf("projection = %v, want the unscoped copy the pods read today", got)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.UnscopedSnapshotGeneration != 5 {
		t.Fatalf("marker not persisted: %d", stored.Status.UnscopedSnapshotGeneration)
	}
	var copyOf corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: "evg-shared-env-r5"}, &copyOf); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(&copyOf, other) || len(copyOf.OwnerReferences) != 2 {
		t.Fatalf("owners = %+v, want worker controlling and api co-owning", copyOf.OwnerReferences)
	}

	// Expiry: the controller keeps a copy a co-owner still reads; the co-owner
	// drops its reference; then the controller deletes it.
	other.Status.ReleaseGeneration = 5 + releaseSnapshotRetention
	app.Status.ReleaseGeneration = 5 + releaseSnapshotRetention
	if err := r.gcReleaseConfigSnapshots(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: "evg-shared-env-r5"}, &copyOf); err != nil {
		t.Fatalf("the controller deleted a copy its co-owner still holds: %v", err)
	}
	if err := r.gcReleaseConfigSnapshots(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: "evg-shared-env-r5"}, &copyOf); err != nil {
		t.Fatalf("the co-owner deleted a copy it does not control: %v", err)
	}
	if len(copyOf.OwnerReferences) != 1 {
		t.Fatalf("co-owner kept its reference past expiry: %+v", copyOf.OwnerReferences)
	}
	if err := r.gcReleaseConfigSnapshots(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: "evg-shared-env-r5"}, &copyOf); err == nil {
		t.Fatal("the expired copy survived once nobody held it")
	}
}
