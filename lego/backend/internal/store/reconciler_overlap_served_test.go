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

package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// overlapFixture is the w8/m119 shape: generation 1 live, generation 2
// building, generation 3 saved behind it. It returns the three rows.
func overlapFixture(t *testing.T, ctx context.Context, rec *Reconciler, st *memStore, cl client.Client) (string, Deploy, Deploy, Deploy) {
	t.Helper()
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	row, _ := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: "https://example.com/acme/web.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("create projection: %v", err)
	}
	setStatus := func(mutate func(*appv1alpha1.App)) {
		t.Helper()
		app := getApp(t, cl)
		mutate(app)
		if err := cl.Status().Update(ctx, app); err != nil {
			t.Fatal(err)
		}
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ReleaseGeneration, a.Status.ActiveRevision = appv1alpha1.PhaseRunning, 1, "rev-1"
		a.Status.Image, a.Status.Conditions = "registry.example/acme/web:gen-1", nil
	})
	history, _ := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if len(history) != 1 {
		t.Fatalf("history = %+v, want the first deploy", history)
	}
	first := history[0]
	executing, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", 2, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ReleaseGeneration = appv1alpha1.PhaseBuilding, 2
		a.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: "Building", ObservedGeneration: a.Generation,
		}}
	})
	pending, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", 3, CommitInfo{}, "")
	if err != nil || pending.Status != DeployQueued {
		t.Fatalf("pending generation 3 = %+v (err %v), want queued", pending, err)
	}
	return row.ID, first, executing, pending
}

// The operator holds generation 3 until generation 2 serves, then adopts it
// and keeps status.activeRevision on generation 2 while 3 builds (w8/m119).
// The deploy reconciler may first look only after that adoption; the served
// record, not the Ready instant it missed, closes generation 2's row live.
// Before the fix it read the newer release generation and closed the row
// canceled while its release served.
func TestReconcileOverlapRecordsServedReleaseAfterTheNextIsAdopted(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	appID, first, executing, pending := overlapFixture(t, ctx, rec, st, cl)

	app := getApp(t, cl)
	app.Status.Phase, app.Status.ReleaseGeneration, app.Status.ActiveRevision = appv1alpha1.PhaseBuilding, 3, "rev-2"
	app.Status.Image = "registry.example/acme/web:gen-2"
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: "Building", ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	first, _ = st.GetDeploy(ctx, appID, first.ID)
	executing, _ = st.GetDeploy(ctx, appID, executing.ID)
	if executing.Status != DeployLive || first.Status != DeployDeactivated {
		t.Fatalf("after generation 2 served: first=%q executing=%q, want deactivated/live", first.Status, executing.Status)
	}
	// Restart and Rollback need the image the row's release ran; while the
	// newer release builds, status.image is still that image.
	if executing.ResolvedImage != "registry.example/acme/web:gen-2" {
		t.Errorf("resolved image = %q, want generation 2's", executing.ResolvedImage)
	}

	app = getApp(t, cl)
	app.Status.Phase, app.Status.ActiveRevision = appv1alpha1.PhaseRunning, "rev-3"
	app.Status.Image, app.Status.Conditions = "registry.example/acme/web:gen-3", nil
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	executing, _ = st.GetDeploy(ctx, appID, executing.ID)
	pending, _ = st.GetDeploy(ctx, appID, pending.ID)
	if executing.Status != DeployDeactivated || pending.Status != DeployLive {
		t.Fatalf("after generation 3 served: executing=%q pending=%q, want deactivated/live", executing.Status, pending.Status)
	}
}

// A rollout that failed over the prior release, and the adoption of the
// pending release right after it, leave the row's own failure record; it
// closes update_failed with that diagnosis, never canceled (w8/m119).
func TestReconcileOverlapKeepsTheFailedRolloutVerdictAfterTheNextIsAdopted(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	appID, first, executing, _ := overlapFixture(t, ctx, rec, st, cl)

	app := getApp(t, cl)
	app.Status.Phase, app.Status.ReleaseGeneration, app.Status.ActiveRevision = appv1alpha1.PhaseBuilding, 3, "rev-1"
	app.Status.Conditions = []metav1.Condition{
		{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Building", ObservedGeneration: app.Generation},
		{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
			Message: "rollout did not become healthy within the progress deadline", ObservedGeneration: 2},
	}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	first, _ = st.GetDeploy(ctx, appID, first.ID)
	executing, _ = st.GetDeploy(ctx, appID, executing.ID)
	if executing.Status != DeployUpdateFailed || first.Status != DeployLive {
		t.Fatalf("after generation 2's rollout failed: first=%q executing=%q, want live/update_failed", first.Status, executing.Status)
	}
	if !strings.Contains(executing.FailureReason, "progress deadline") {
		t.Errorf("failure reason = %q, want the rollout diagnosis", executing.FailureReason)
	}
}

// A Restart releases the running configuration over a deploy still in flight
// and ends it (Override, as on Render's pre-2025-07-14 workspaces). The ended
// row names the restart and says its saved changes were not deployed, instead
// of a bare "Superseded by", so the change is not silently lost (w8/m119).
func TestReconcileNamesTheRestartThatEndedAnInFlightDeploy(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	appID, _, executing, pending := overlapFixture(t, ctx, rec, st, cl)
	restart, err := st.CreateRestartDeploy(ctx, appID, "registry.example/acme/web:gen-1", 4, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}

	app := getApp(t, cl)
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{
		Generation: 4, SourceGeneration: 1, Image: "registry.example/acme/web:gen-1", PreserveGroupValues: true,
	}
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	app = getApp(t, cl)
	app.Status.Phase, app.Status.ReleaseGeneration, app.Status.ActiveRevision = appv1alpha1.PhaseDeploying, 4, "rev-1"
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for _, d := range []Deploy{executing, pending} {
		got, _ := st.GetDeploy(ctx, appID, d.ID)
		if got.Status != DeployCanceled || !strings.Contains(got.CancelReason, "Canceled by restart "+restart.ID) ||
			!strings.Contains(got.CancelReason, "was not deployed") {
			t.Errorf("deploy %d = %q %q, want canceled naming restart %s and the undeployed change", d.Generation, got.Status, got.CancelReason, restart.ID)
		}
	}
}

// When the release adopted next needs no build (a configuration-only change),
// it rolls out at once and takes status.image over before this pass looks. The
// served row's image comes from its own release record, so Restart and
// Rollback still select what it ran (w8/m119).
func TestReconcileOverlapReadsTheServedImageFromItsReleaseRecord(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	appID, _, executing, _ := overlapFixture(t, ctx, rec, st, cl)

	app := getApp(t, cl)
	tmpl, err := json.Marshal(corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "metrics-sidecar", Image: "registry.example/sidecar:1"},
		{Name: appv1alpha1.ReleaseRecordAppContainer, Image: "registry.example/acme/web:gen-2"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: app.Namespace, Name: appv1alpha1.ReleaseRecordName(app.Name, 2)},
		Data:       map[string][]byte{appv1alpha1.ReleaseRecordPodTemplateKey: tmpl},
	}); err != nil {
		t.Fatal(err)
	}
	app.Status.Phase, app.Status.ReleaseGeneration, app.Status.ActiveRevision = appv1alpha1.PhaseDeploying, 3, "rev-2"
	app.Status.Image = "registry.example/acme/web:gen-3"
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: "Deploying", ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	executing, _ = st.GetDeploy(ctx, appID, executing.ID)
	if executing.Status != DeployLive || executing.ResolvedImage != "registry.example/acme/web:gen-2" {
		t.Fatalf("served row = %q image %q, want live with generation 2's recorded image", executing.Status, executing.ResolvedImage)
	}
}
