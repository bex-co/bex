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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m117: deploy A (generation 1) is in flight when deploy B (generation 2)
// is triggered into the pending slot and canceled. The stamp names A as the
// release the cancel leaves running, with the commit A's row records, and every
// later cancel restates it: one that also ends A removes it, so neither a
// suspend nor a second Cancel leaves the operator carrying a canceled release.
func TestCancelOfThePendingSlotNamesTheReleaseItLeavesRunning(t *testing.T) {
	ctx := context.Background()
	patches := 0
	cl := cancelTestClient(t, &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web",
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
		Spec: appv1alpha1.AppSpec{Repo: "https://example.invalid/acme/web.git", BuildCommit: "bbb"},
	}, interceptor.Funcs{Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
		patches++
		return c.Patch(ctx, obj, patch, opts...)
	}})
	app := func() *appv1alpha1.App {
		return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"}}
	}
	expect := func(step, canceled, survivor, commit string) {
		t.Helper()
		a := app()
		if err := cl.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
			t.Fatal(err)
		}
		if c, s, b := a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration],
			a.Annotations[appv1alpha1.AnnotationSurvivingReleaseGeneration], a.Spec.BuildCommit; c != canceled || s != survivor || b != commit {
			t.Fatalf("%s: canceled=%q survivor=%q commit=%q, want %q %q %q", step, c, s, b, canceled, survivor, commit)
		}
	}
	a := &Deploy{Generation: 1, Commit: "aaa"}

	if err := CancelRelease(ctx, cl, app(), 2, a, ""); err != nil {
		t.Fatal(err)
	}
	expect("cancel B", "2", "1", "aaa")
	before := patches
	if err := CancelRelease(ctx, cl, app(), 2, a, ""); err != nil || patches != before {
		t.Fatalf("a repeated cancel = %v after %d more patches, want a no-op", err, patches-before)
	}

	// A suspend, or Cancel of A itself: the stamp cannot move back to A, so
	// it is the survivor's name that goes.
	if err := CancelRelease(ctx, cl, app(), 1, nil, ""); err != nil {
		t.Fatal(err)
	}
	expect("cancel A after B", "2", "", "aaa")
}

// A deploy C opened between Cancel choosing A and the stamp landing is newer
// than both: the cancel of B names no survivor and leaves C's commit alone.
func TestCancelRacingANewerDeployNamesNoSurvivor(t *testing.T) {
	ctx := context.Background()
	cl := cancelTestClient(t, &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web",
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "3"}},
		Spec: appv1alpha1.AppSpec{Repo: "https://example.invalid/acme/web.git", BuildCommit: "ccc"},
	}, interceptor.Funcs{})
	a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"}}
	if err := CancelRelease(ctx, cl, a, 2, &Deploy{Generation: 1, Commit: "aaa"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
		t.Fatal(err)
	}
	if survivor, ok := a.Annotations[appv1alpha1.AnnotationSurvivingReleaseGeneration]; ok || a.Spec.BuildCommit != "ccc" {
		t.Fatalf("survivor=%q commit=%q, want none and C's commit", survivor, a.Spec.BuildCommit)
	}
}

// A's row is open when B's cancel lands. Whether it closes depends on whether
// A's release is still the one in flight; when the cancel settled below it,
// the row closes now — not at its gate timeout — naming the canceled deploy.
func TestCancelOfANewerReleaseClosesAnOlderRowOnlyOnceSettledBelowIt(t *testing.T) {
	ctx := context.Background()
	cr := func(reported int64) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2, Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:         "2",
				appv1alpha1.AnnotationCanceledReleaseGeneration: "2",
			}},
			Spec:   appv1alpha1.AppSpec{Repo: "https://example.invalid/acme/web.git"},
			Status: appv1alpha1.AppStatus{ReleaseGeneration: reported, Phase: appv1alpha1.PhaseBuilding},
		}
	}
	older := Deploy{Generation: 1, Status: DeployQueued}

	if got, settled := supersededDeployStatus(older, cr(1), false); settled {
		t.Errorf("A in flight = (%q, settled); the cancel of B must leave it running", got)
	}
	if got, settled := supersededDeployStatus(Deploy{Generation: 3, Status: DeployQueued, OverlapPending: true}, cr(1), false); settled && got == DeployCanceled {
		t.Errorf("a row newer than the cancel closed %q", got)
	}

	st := newMemStore()
	tenant, err := st.CreateTenant(ctx, "cancel-along", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Repo: "https://example.invalid/acme/web.git", Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := st.DeployIDByGeneration(ctx, row.ID, 1)
	if err != nil || firstID == "" {
		t.Fatalf("the create deploy: %q, %v", firstID, err)
	}
	second, err := st.CreateDeploy(ctx, row.ID, TriggerConfigChange, "", 2, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if won, err := st.CloseDeploy(ctx, second.ID, DeployCanceled, ""); err != nil || !won {
		t.Fatalf("cancel B: won=%v err=%v", won, err)
	}
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	settled := cr(0)
	rec := NewReconciler(fake.NewClientBuilder().WithScheme(scheme).WithObjects(settled.DeepCopy()).Build(), st)
	open, err := st.GetDeploy(ctx, row.ID, firstID)
	if err != nil {
		t.Fatal(err)
	}
	rec.recordDeploy(ctx, DesiredApp{App: row}, open, settled)
	got, err := st.GetDeploy(ctx, row.ID, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != DeployCanceled || got.CancelReason != "Canceled along with "+second.ID {
		t.Fatalf("A after the cancel settled below it = %s %q, want canceled along with %s", got.Status, got.CancelReason, second.ID)
	}
}

// A saved-image service has no build to pin, so B's request replaced A's
// release before B was canceled: the reported release is B's (or the settled
// one below both) until the operator rolls A again. A's row waits for that,
// within its gate timeout, instead of closing as superseded or canceled along
// (w8/m117, the 2026-10-28 hosted replay).
func TestASavedImageSurvivorsRowWaitsForItsRelease(t *testing.T) {
	cr := func(reported int64) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2, Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:          "2",
				appv1alpha1.AnnotationCanceledReleaseGeneration:  "2",
				appv1alpha1.AnnotationSurvivingReleaseGeneration: "1",
			}},
			Spec:   appv1alpha1.AppSpec{Image: "python:3.12-alpine"},
			Status: appv1alpha1.AppStatus{ReleaseGeneration: reported, Phase: appv1alpha1.PhaseDeploying},
		}
	}
	survivor := Deploy{Generation: 1, Status: DeployCreated}
	for _, reported := range []int64{2, 0} {
		if got := observedDeployStatus(survivor, cr(reported), false); got != "" {
			t.Errorf("reported release %d: A = %q, want it left open for the redispatch", reported, got)
		}
	}
	if got := observedDeployStatus(survivor, cr(2), true); got == "" {
		t.Error("past its gate timeout A must still close")
	}
	if got := observedDeployStatus(Deploy{Generation: 1, Status: DeployCreated}, cr(1), false); got == DeployCanceled {
		t.Error("once A is the reported release its own evidence decides, not the cancel")
	}
	newer := cr(3)
	newer.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
	if got := observedDeployStatus(survivor, newer, false); got != DeployCanceled {
		t.Errorf("a deploy requested after the cancel replaces A too: A = %q, want canceled", got)
	}
}
