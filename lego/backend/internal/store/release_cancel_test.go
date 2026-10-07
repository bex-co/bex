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
	"errors"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m114: once the cancel stamp lands the release is ended, so a build delete
// that fails must not keep the row open. The operator's settle rewinds the
// release generation, and no later pass would read the row as suspend-ended
// again: it would sit "in progress" until the gate timeout. A second pass sends
// no second patch for a stamp already in place.
func TestSuspendCloseSurvivesAFailedBuildDelete(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	tenant, err := st.CreateTenant(ctx, "suspend-build", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Repo: "https://example.invalid/acme/web.git", Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	open, err := st.CreateDeploy(ctx, row.ID, TriggerNewCommit, "", 7, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.TransitionDeploy(ctx, open.ID, DeployBuildInProgress, "", "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-suspend", Generation: 8},
		Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/acme/web.git", Suspended: true},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseHibernated, Image: "registry.example/web@sha256:six",
			ReleaseGeneration: 7, ActiveRevision: "rev-6", ObservedGeneration: 8,
		},
	}
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	patches, deletes := 0, 0
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			patches++
			return c.Patch(ctx, obj, patch, opts...)
		},
		Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
			deletes++
			if obj.GetNamespace() != "bex-build" {
				t.Errorf("build delete in %q, want BEX_BUILD_NAMESPACE", obj.GetNamespace())
			}
			return errors.New("apiserver unavailable")
		},
	}).Build()
	rec := NewReconciler(cl, st)
	rec.BuildNamespace = "bex-build"

	current, err := st.GetDeploy(ctx, row.ID, open.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec.recordDeploy(ctx, DesiredApp{App: row}, current, app)
	got, err := st.GetDeploy(ctx, row.ID, open.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != DeployCanceled || got.CancelReason != suspendCancelReason {
		t.Fatalf("deploy = %s %q, want canceled by the suspend even though the build delete failed", got.Status, got.CancelReason)
	}
	if deletes == 0 {
		t.Fatal("the build delete was never attempted")
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] != "7" || patches != 1 {
		t.Fatalf("stamp = %q after %d patches, want 7 after one", stored.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration], patches)
	}
	if err := markReleaseCanceled(ctx, cl, &stored, 7); err != nil || patches != 1 {
		t.Fatalf("restamp = %v after %d patches, want a no-op", err, patches)
	}
}

func cancelTestClient(t *testing.T, app *appv1alpha1.App, funcs interceptor.Funcs, builds ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(builds, app)...).WithInterceptorFuncs(funcs).Build()
}

// w5/136: a buildpack release's kpack Image is named by a hash of the App's UID
// and revision, so a cancel that addressed it by the build Job's name never
// stopped it. The cancel selects it by the labels the operator stamps instead,
// using the UID it re-reads rather than the caller's copy: the canceled
// release's build goes, whichever shape it took, while a recreated namesake
// App's build of the same revision and the App's later release stay. bex-api
// may list kpack Images only in the build namespace. Nothing retries a cancel
// once its stamp lands, so a failed Job delete must not spare the Image, and a
// cluster without kpack has no Image to stop.
func TestCancelReleaseStopsItsBuild(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"},
		Spec: appv1alpha1.AppSpec{Repo: "https://example.invalid/acme/web.git"}}
	image := func(name string, uid types.UID, generation int64) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(kpackImageGVK)
		u.SetNamespace("bex-build")
		u.SetName(name)
		u.SetLabels(appv1alpha1.ReleaseBuildLabels("web", string(uid), appv1alpha1.BuildRevision(generation)))
		return u
	}
	gone := func(cl client.Client, obj client.Object) bool {
		t.Helper()
		err := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
		return err != nil
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(7)), Namespace: "bex-build"}}
	canceled := image("bld-web-gen-7-0123456789ab", app.UID, 7)
	kept := []client.Object{image("bld-web-gen-7-ba9876543210", "uid-namesake", 7), image("bld-web-gen-8-00112233aabb", app.UID, 8)}
	cl := cancelTestClient(t, app.DeepCopy(), interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if ns := (&client.ListOptions{}).ApplyOptions(opts).Namespace; ns != "bex-build" {
				t.Errorf("kpack Images listed in %q, want only the build namespace", ns)
			}
			return c.List(ctx, list, opts...)
		},
	}, append(kept, job, canceled)...)
	caller := app.DeepCopy()
	caller.UID = ""
	if err := CancelRelease(ctx, cl, caller, 7, "bex-build"); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []client.Object{job, canceled} {
		if !gone(cl, obj) {
			t.Errorf("canceled release's %s survived the cancel", obj.GetName())
		}
	}
	for _, obj := range kept {
		if gone(cl, obj) {
			t.Errorf("%s was not the canceled release's, yet the cancel deleted it", obj.GetName())
		}
	}

	jobDeleteFails := cancelTestClient(t, app.DeepCopy(), interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*batchv1.Job); ok {
				return errors.New("apiserver unavailable")
			}
			return c.Delete(ctx, obj, opts...)
		},
	}, canceled.DeepCopy())
	if err := deleteReleaseBuild(ctx, jobDeleteFails, app, 7, "bex-build"); err == nil || !gone(jobDeleteFails, canceled) {
		t.Errorf("cancel whose Job delete fails = %v, Image stopped = %t; want the failure reported and the Image stopped anyway", err, gone(jobDeleteFails, canceled))
	}

	withoutKpack := cancelTestClient(t, app.DeepCopy(), interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return &meta.NoKindMatchError{GroupKind: kpackImageGVK.GroupKind()}
		},
	})
	if err := deleteReleaseBuild(ctx, withoutKpack, app, 7, "bex-build"); err != nil {
		t.Fatalf("cancel where kpack is not installed = %v, want nothing to report", err)
	}
}

// A stamp read a pass ago must not overwrite a newer release's cancel that
// landed since: the newer marker is what keeps that release from rolling.
func TestCancelReleaseNeverOverwritesANewerCancel(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a",
		Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "9"}}}
	stale := app.DeepCopy()
	stale.Annotations = nil // the copy a pass listed before the newer cancel landed
	cl := cancelTestClient(t, app, interceptor.Funcs{})
	if err := CancelRelease(ctx, cl, stale, 7, ""); err != nil {
		t.Fatal(err)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if got := stored.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration]; got != "9" {
		t.Fatalf("canceled release generation = %q, want the newer cancel 9 kept", got)
	}
}

// Once the stamp lands the release is ended, so CancelRelease succeeds even
// when its build cannot be deleted — including where kpack's Image kind is
// not served at all.
func TestCancelReleaseSucceedsOnceStamped(t *testing.T) {
	ctx := context.Background()
	for name, deleteErr := range map[string]error{
		"apiserver error": errors.New("apiserver unavailable"),
		"kind not served": &meta.NoKindMatchError{GroupKind: kpackImageGVK.GroupKind()},
	} {
		t.Run(name, func(t *testing.T) {
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
				Spec: appv1alpha1.AppSpec{Repo: "https://example.invalid/acme/web.git"}}
			cl := cancelTestClient(t, app.DeepCopy(), interceptor.Funcs{
				Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error { return deleteErr },
			})
			if err := CancelRelease(ctx, cl, app, 7, ""); err != nil {
				t.Fatalf("CancelRelease = %v, want success once the stamp landed", err)
			}
			if got := app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration]; got != "7" {
				t.Fatalf("canceled release generation = %q, want 7", got)
			}
		})
	}
	if err := deleteBuildArtifact(ctx, cancelTestClient(t, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}, interceptor.Funcs{
		Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
			return &meta.NoKindMatchError{GroupKind: kpackImageGVK.GroupKind()}
		},
	}), &appv1alpha1.App{}); err != nil {
		t.Fatalf("deleting a kind the cluster does not serve = %v, want it read as already gone", err)
	}
}

// A cancel whose stamp landed but whose row never closed — deploys.Cancel
// failing after CancelRelease — closes on the next pass, even after the
// operator settled and rewound the release generation, which hides the row
// from every other close rule.
func TestStampedReleaseClosesItsStrandedRow(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	tenant, err := st.CreateTenant(ctx, "stranded", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "registry.example/web:v7", Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	open, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, row.Image, 7, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.TransitionDeploy(ctx, open.ID, DeployUpdateInProgress, "", "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-stranded", Generation: 7,
			Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "7"}},
		Spec: appv1alpha1.AppSpec{Image: row.Image},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, Image: "registry.example/web:v6",
			ReleaseGeneration: 6, ActiveRevision: "rev-6", ObservedGeneration: 7},
	}
	if err := cl.Create(ctx, app.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	current, err := st.GetDeploy(ctx, row.ID, open.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec.recordDeploy(ctx, DesiredApp{App: row}, current, app)
	got, err := st.GetDeploy(ctx, row.ID, open.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != DeployCanceled || got.CancelReason != "" {
		t.Fatalf("stranded row = %s %q, want canceled with a user cancel's empty reason", got.Status, got.CancelReason)
	}
}
