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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDiskDeletionRejectsForgedBootstrapTenantAliasBeforeAnyMutation(t *testing.T) {
	for _, labels := range []map[string]string{
		{"bex.co/tenant": "other-workspace"},
		{labelWorkspace: "other-workspace"},
	} {
		app := deletionApp("forged-snapshot-owner")
		app.Labels = labels
		app.Annotations = map[string]string{annotDiskSnapshots: string(app.UID)}
		writer := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "forged-source-job", Namespace: app.Namespace, UID: "writer", Labels: diskBackupLabels(app)}}
		mutations := 0
		cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, writer).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					mutations++
					return c.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					mutations++
					return c.Update(ctx, obj, opts...)
				},
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					mutations++
					return c.Patch(ctx, obj, patch, opts...)
				},
				Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					mutations++
					return c.Delete(ctx, obj, opts...)
				},
			}).Build()
		r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
		if done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app); done || err == nil || !strings.Contains(err.Error(), "scope") {
			t.Fatalf("forged scope accepted: done=%v error=%v", done, err)
		}
		if mutations != 0 {
			t.Fatalf("forged scope triggered %d mutations", mutations)
		}
	}
}

func diskDeletionFixture(t *testing.T) (*AppReconciler, *appv1alpha1.App) {
	t.Helper()
	app := deletionApp("disk-delete")
	app.Spec.Disk = &appv1alpha1.DiskSpec{SizeGB: 1, MountPath: "/data"}
	app.Annotations = map[string]string{annotDiskProvisioned: diskProvisionedMarker}
	credential := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "disk-s3", Namespace: app.Namespace},
		Data: map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("test"), "AWS_SECRET_ACCESS_KEY": []byte("test-secret")}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, credential).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	return &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes,
		BackupHelperImage: "test-operator", DiskSnapshots: DiskSnapshotStore{
			Endpoint: "https://s3.example.invalid", Bucket: "snapshots", S3Secret: credential.Name, AgePublicKey: "test-recipient",
		}}, app
}

func TestDiskDeletionWaitsForWritersPurgeAndPodAbsence(t *testing.T) {
	r, app := diskDeletionFixture(t)
	ctx := t.Context()
	labels := diskBackupLabels(app)
	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: diskBackupName(app.Name), Namespace: app.Namespace, UID: "cron", Labels: labels}}
	if err := controllerutil.SetControllerReference(app, cron, r.Scheme); err != nil {
		t.Fatal(err)
	}
	writer := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "snapshot-writer", Namespace: app.Namespace, UID: "writer", Labels: labels}}
	writerPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "snapshot-writer-pod", Namespace: app.Namespace, UID: "writer-pod", Labels: labels, Finalizers: []string{"test/hold"}}}
	restore := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: diskRestoreName(app.Name), Namespace: app.Namespace, UID: "restore", Labels: labels}}
	for _, obj := range []client.Object{cron, writer, writerPod, restore} {
		if err := r.Create(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(app)}
	run := func(wantError bool) {
		t.Helper()
		_, err := r.Reconcile(ctx, request)
		if (err != nil) != wantError {
			t.Fatalf("reconcile error = %v, want error %v", err, wantError)
		}
	}
	assertHeld := func() {
		t.Helper()
		var current appv1alpha1.App
		if err := r.Get(ctx, request.NamespacedName, &current); err != nil || !controllerutil.ContainsFinalizer(&current, finalizer) {
			t.Fatalf("finalizer released early: app=%+v error=%v", current.ObjectMeta, err)
		}
	}
	run(false)
	assertHeld()
	for _, obj := range []client.Object{cron, writer, restore} {
		if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); !apierrors.IsNotFound(err) {
			t.Fatalf("writer %s remains: %v", obj.GetName(), err)
		}
	}
	var job batchv1.Job
	jobKey := client.ObjectKey{Namespace: app.Namespace, Name: diskPurgeName(app)}
	if err := r.Get(ctx, jobKey, &job); !apierrors.IsNotFound(err) {
		t.Fatalf("purge dispatched before writer Pod was gone: %v", err)
	}
	run(false)
	assertHeld()
	if err := r.Get(ctx, client.ObjectKeyFromObject(writerPod), writerPod); err != nil {
		t.Fatal(err)
	}
	writerPod.Finalizers = nil
	if err := r.Update(ctx, writerPod); err != nil {
		t.Fatal(err)
	}
	run(false)
	assertHeld()
	if err := r.Get(ctx, jobKey, &job); err != nil {
		t.Fatal(err)
	}
	if job.Labels[execution.LabelAppUID] != string(app.UID) || job.Labels[execution.LabelComponent] != diskPurgeComponent {
		t.Fatalf("purge lifetime labels = %v", job.Labels)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "S3 unavailable"}}
	if err := r.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	run(true)
	assertHeld()
	run(false)
	if err := r.Get(ctx, jobKey, &job); err != nil {
		t.Fatal(err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := r.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	purgePod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "purge-pod", Namespace: app.Namespace, UID: "purge-pod", Labels: job.Spec.Template.Labels, Finalizers: []string{"test/hold"}}}
	if err := r.Create(ctx, purgePod); err != nil {
		t.Fatal(err)
	}
	for range 4 {
		run(false)
		assertHeld()
	}
	var current appv1alpha1.App
	if err := r.Get(ctx, request.NamespacedName, &current); err != nil {
		t.Fatal(err)
	}
	if current.Annotations[annotDiskPurgeComplete] != string(app.UID) {
		t.Fatal("successful purge was not persisted for this App UID")
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(purgePod), purgePod); err != nil {
		t.Fatal(err)
	}
	purgePod.Finalizers = nil
	if err := r.Update(ctx, purgePod); err != nil {
		t.Fatal(err)
	}
	run(false)
	if err := r.Get(ctx, request.NamespacedName, &current); !apierrors.IsNotFound(err) {
		t.Fatalf("completed cleanup retained App: %v", err)
	}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: r.DiskSnapshots.S3Secret}, &corev1.Secret{}); err != nil {
		t.Fatalf("shared backup credential was removed: %v", err)
	}
}

func TestDiskDeletionFailsClosedAfterSnapshotStateButAllowsNeverConfiguredDisks(t *testing.T) {
	for _, configuredBefore := range []bool{false, true} {
		t.Run(map[bool]string{false: "never-configured", true: "configuration-lost"}[configuredBefore], func(t *testing.T) {
			r, app := diskDeletionFixture(t)
			r.DiskSnapshots = DiskSnapshotStore{}
			if configuredBefore {
				if err := r.markDiskSnapshots(t.Context(), app); err != nil {
					t.Fatal(err)
				}
			}
			done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app)
			if configuredBefore && (done || err == nil) {
				t.Fatalf("lost credentials acknowledged: done=%v error=%v", done, err)
			}
			if !configuredBefore && (!done || err != nil) {
				t.Fatalf("never-configured disk blocked: done=%v error=%v", done, err)
			}
		})
	}
	for _, missing := range []string{"Secret", "AWS_SECRET_ACCESS_KEY"} {
		t.Run("missing-"+missing, func(t *testing.T) {
			r, app := diskDeletionFixture(t)
			if err := r.markDiskSnapshots(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			secret := &corev1.Secret{}
			if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: r.DiskSnapshots.S3Secret}, secret); err != nil {
				t.Fatal(err)
			}
			if missing == "Secret" {
				if err := r.Delete(t.Context(), secret); err != nil {
					t.Fatal(err)
				}
			} else {
				delete(secret.Data, missing)
				if err := r.Update(t.Context(), secret); err != nil {
					t.Fatal(err)
				}
			}
			if done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app); done || err == nil {
				t.Fatalf("missing credential acknowledged: done=%v error=%v", done, err)
			}
		})
	}
}

func TestDiskDeletionDiscoversLegacySnapshotStateBeforeStoppingWriters(t *testing.T) {
	r, app := diskDeletionFixture(t)
	r.DiskSnapshots = DiskSnapshotStore{}
	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: diskBackupName(app.Name), Namespace: app.Namespace, UID: "legacy-cron"}}
	if err := controllerutil.SetControllerReference(app, cron, r.Scheme); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(t.Context(), cron); err != nil {
		t.Fatal(err)
	}
	if done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app); done || err != nil {
		t.Fatalf("legacy writer cleanup = done %v error %v", done, err)
	}
	app = &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	if app.Annotations[annotDiskSnapshots] != string(app.UID) {
		t.Fatal("legacy state was not persisted before writer deletion")
	}
	if done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app); done || err == nil {
		t.Fatalf("restart lost legacy cleanup authority: done=%v error=%v", done, err)
	}
}

func TestDiskDeletionPreservesForeignLifetimesAndChecksCompletedJobOwnership(t *testing.T) {
	r, app := diskDeletionFixture(t)
	ctx := t.Context()
	foreign := app.DeepCopy()
	foreign.UID = types.UID("old-lifetime")
	oldWriter := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "old-writer", Namespace: app.Namespace, UID: "foreign-job", Labels: diskBackupLabels(foreign)}}
	oldPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old-writer-pod", Namespace: app.Namespace, UID: "foreign-pod", Labels: diskBackupLabels(foreign)}}
	otherNamespace := oldWriter.DeepCopy()
	otherNamespace.Namespace = "other-tenant"
	otherNamespace.Labels = diskBackupLabels(app)
	for _, obj := range []client.Object{oldWriter, oldPod, otherNamespace} {
		if err := r.Create(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	if done, err := r.reconcileDiskSnapshotCleanup(ctx, app); done || err == nil {
		t.Fatalf("foreign writer did not block same-prefix purge: done=%v error=%v", done, err)
	}
	for _, obj := range []client.Object{oldWriter, oldPod, otherNamespace} {
		if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil || !obj.GetDeletionTimestamp().IsZero() {
			t.Fatalf("foreign writer changed: %s/%s error=%v", obj.GetNamespace(), obj.GetName(), err)
		}
	}
	if err := r.Delete(ctx, oldWriter); err != nil {
		t.Fatal(err)
	}
	if done, err := r.reconcileDiskSnapshotCleanup(ctx, app); done || err == nil {
		t.Fatalf("foreign Pod did not block same-prefix purge: done=%v error=%v", done, err)
	}
	if err := r.Delete(ctx, oldPod); err != nil {
		t.Fatal(err)
	}
	if _, err := r.reconcileDiskSnapshotCleanup(ctx, app); err != nil {
		t.Fatal(err)
	}
	var ownJob batchv1.Job
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskPurgeName(app)}, &ownJob); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, &ownJob); err != nil {
		t.Fatal(err)
	}
	if err := r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotDiskPurgeComplete, string(app.UID))
	}); err != nil {
		t.Fatal(err)
	}
	foreignPurge := ownJob.DeepCopy()
	foreignPurge.ResourceVersion = ""
	foreignPurge.UID = "foreign-purge"
	foreignPurge.Labels[execution.LabelAppUID] = string(foreign.UID)
	if err := r.Create(ctx, foreignPurge); err != nil {
		t.Fatal(err)
	}
	if done, err := r.reconcileDiskSnapshotCleanup(ctx, app); done || err == nil || !strings.Contains(err.Error(), "different resource lifetime") {
		t.Fatalf("foreign Job accepted after completion: done=%v error=%v", done, err)
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(foreignPurge), foreignPurge); err != nil {
		t.Fatalf("foreign purge Job deleted: %v", err)
	}
	foreignPurge.Labels[execution.LabelAppUID] = string(app.UID)
	foreignPurge.OwnerReferences = nil
	if err := controllerutil.SetControllerReference(foreign, foreignPurge, r.Scheme); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(ctx, foreignPurge); err != nil {
		t.Fatal(err)
	}
	if done, err := r.reconcileDiskSnapshotCleanup(ctx, app); done || err == nil {
		t.Fatalf("matching label overrode foreign purge owner: done=%v error=%v", done, err)
	}
}

func TestDiskDeletionRefusesForeignSnapshotScheduleNameCollision(t *testing.T) {
	r, app := diskDeletionFixture(t)
	foreign := app.DeepCopy()
	foreign.UID = "old-lifetime"
	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: diskBackupName(app.Name), Namespace: app.Namespace,
		UID: "foreign-cron", Labels: diskBackupLabels(foreign)}}
	if err := controllerutil.SetControllerReference(foreign, cron, r.Scheme); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(t.Context(), cron); err != nil {
		t.Fatal(err)
	}
	if done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app); done || err == nil {
		t.Fatalf("foreign CronJob did not block same-prefix purge: done=%v error=%v", done, err)
	}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(cron), cron); err != nil || !cron.DeletionTimestamp.IsZero() {
		t.Fatalf("foreign CronJob was changed: %v", err)
	}
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: diskPurgeName(app)}, &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("foreign schedule caused a purge dispatch: %v", err)
	}
	cron.Labels[execution.LabelAppUID] = string(app.UID)
	if err := r.Update(t.Context(), cron); err != nil {
		t.Fatal(err)
	}
	if done, err := r.reconcileDiskSnapshotCleanup(t.Context(), app); done || err == nil {
		t.Fatalf("matching label overrode foreign schedule owner: done=%v error=%v", done, err)
	}
}
