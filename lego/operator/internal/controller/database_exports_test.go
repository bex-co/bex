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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func newExportReconciler(t *testing.T, now *time.Time, db *appv1alpha1.Database) *DatabaseReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&batchv1.Job{}).
		WithObjects(db).
		Build()
	return &DatabaseReconciler{
		Client:          cl,
		Scheme:          scheme,
		Backup:          testStore,
		ExportRetention: time.Hour,
		Now:             func() time.Time { return *now },
	}
}

func exportTestDatabase(now time.Time) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-db", Namespace: "default", UID: "db-uid"},
		Spec: appv1alpha1.DatabaseSpec{
			Plan:    "basic-1gb",
			Version: "16",
			Exports: []appv1alpha1.DatabaseExportRequest{{
				ID:          "exp-c185th5c2rvvnhbfiltg",
				RequestedAt: now.Format(time.RFC3339),
			}},
		},
		Status: appv1alpha1.DatabaseStatus{SecretName: "tenant-db-app", BackupsEnabled: true},
	}
}

func TestLogicalExportJobShape(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 34, 0, 0, time.UTC)
	db := exportTestDatabase(now)
	job := exportJob(db, db.Spec.Exports[0], testStore)

	if job.Name != "postgres-export-"+db.Spec.Exports[0].ID {
		t.Fatalf("job name = %q", job.Name)
	}
	if len(job.Spec.Template.Spec.InitContainers) != 1 || len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("want sequential pg_dump init + upload main containers: %+v", job.Spec.Template.Spec)
	}
	dump := job.Spec.Template.Spec.InitContainers[0]
	wantImage := cnpgExportImage("16")
	if dump.Image != wantImage || !strings.Contains(dump.Image, "@sha256:") ||
		!strings.Contains(dump.Args[0], "pg_dump --format=directory") || !strings.Contains(dump.Args[0], "tar -C /work -czf") {
		t.Fatalf("dump container does not produce a directory-format tarball from a pinned image: %+v", dump)
	}
	for _, env := range dump.Env {
		if env.Name == "PGPASSWORD" && env.Value != "" {
			t.Fatal("database password must be a Secret ref, never a literal")
		}
	}
	upload := job.Spec.Template.Spec.Containers[0]
	if upload.Image != publishDefaultAWSCLIImageForTest() || !strings.Contains(upload.Args[0], "aws s3 cp") {
		t.Fatalf("upload container = %+v", upload)
	}
	if automount := job.Spec.Template.Spec.AutomountServiceAccountToken; automount == nil || *automount {
		t.Fatalf("automountServiceAccountToken = %v, want an explicit false", automount)
	}
	objectKey := envValue(upload.Env, "OBJECT_KEY")
	if !strings.Contains(objectKey, "/logical-exports/tenant-db/"+db.Spec.Exports[0].ID+"/") || !strings.HasSuffix(objectKey, ".dir.tar.gz") {
		t.Fatalf("object key = %q, want per-export Render-format artifact", objectKey)
	}
	if upload.Resources.Limits.Cpu().IsZero() || upload.Resources.Limits.Memory().IsZero() {
		t.Fatal("export upload container must be resource-limited")
	}
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != int32((8*24*time.Hour).Seconds()) {
		t.Fatalf("completed export Job TTL = %v, want eight days", job.Spec.TTLSecondsAfterFinished)
	}
}

// Keep the test independent of publish's package export while still pinning
// the exact platform uploader image selected by exportJob.
func publishDefaultAWSCLIImageForTest() string {
	return "amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2"
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}

func TestLogicalExportLifecycleAndRetention(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	db := exportTestDatabase(now)
	r := newExportReconciler(t, &now, db)

	next, err := r.reconcileExports(ctx, db, true)
	if err != nil || next != logicalExportPollInterval || db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportCreated {
		t.Fatalf("created reconcile: next=%v err=%v status=%+v", next, err, db.Status.Exports)
	}
	var job batchv1.Job
	if err := r.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: exportJobName(db.Spec.Exports[0].ID)}, &job); err != nil {
		t.Fatalf("export Job was not created: %v", err)
	}

	start := metav1.NewTime(now.Add(time.Minute))
	job.Status.Active = 1
	job.Status.StartTime = &start
	if err := r.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	if _, err := r.reconcileExports(ctx, db, true); err != nil || db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportRunning {
		t.Fatalf("running reconcile: err=%v status=%+v", err, db.Status.Exports[0])
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(&job), &job); err != nil {
		t.Fatal(err)
	}
	finished := metav1.NewTime(now.Add(2 * time.Minute))
	job.Status.Active = 0
	job.Status.Succeeded = 1
	job.Status.CompletionTime = &finished
	if err := r.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	if next, err = r.reconcileExports(ctx, db, true); err != nil || next != time.Hour || db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportAvailable {
		t.Fatalf("available reconcile: next=%v err=%v status=%+v", next, err, db.Status.Exports[0])
	}

	// A shortened one-hour window drives a real object-delete Job. The record is
	// not marked expired until that Job succeeds.
	now = now.Add(2 * time.Hour)
	if next, err = r.reconcileExports(ctx, db, true); err != nil || next != logicalExportPollInterval || db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportExpiring {
		t.Fatalf("expiring reconcile: next=%v err=%v status=%+v", next, err, db.Status.Exports[0])
	}
	if _, err = r.reconcileExports(ctx, db, true); err != nil {
		t.Fatal(err)
	}
	var cleanup batchv1.Job
	cleanupKey := client.ObjectKey{Namespace: db.Namespace, Name: exportCleanupJobName(db.Spec.Exports[0].ID)}
	if err := r.Get(ctx, cleanupKey, &cleanup); err != nil {
		t.Fatalf("cleanup Job was not created: %v", err)
	}
	cleanup.Status.Succeeded = 1
	cleanup.Status.CompletionTime = &metav1.Time{Time: now}
	if err := r.Status().Update(ctx, &cleanup); err != nil {
		t.Fatal(err)
	}
	if next, err = r.reconcileExports(ctx, db, true); err != nil || next != 0 || db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportExpired {
		t.Fatalf("expired reconcile: next=%v err=%v status=%+v", next, err, db.Status.Exports[0])
	}
}

func TestLogicalExportFailureIsTerminalAndHonest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	db := exportTestDatabase(now)
	r := newExportReconciler(t, &now, db)
	if _, err := r.reconcileExports(ctx, db, true); err != nil {
		t.Fatal(err)
	}
	var job batchv1.Job
	if err := r.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: exportJobName(db.Spec.Exports[0].ID)}, &job); err != nil {
		t.Fatal(err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{
		Type:               batchv1.JobFailed,
		Status:             corev1.ConditionTrue,
		Reason:             "BackoffLimitExceeded",
		Message:            "S3 credentials were rejected",
		LastTransitionTime: metav1.NewTime(now.Add(time.Minute)),
	}}
	if err := r.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	next, err := r.reconcileExports(ctx, db, true)
	status := db.Status.Exports[0]
	if err != nil || next != 0 || status.Phase != appv1alpha1.DatabaseExportFailed || !strings.Contains(status.FailureReason, "credentials") {
		t.Fatalf("failed reconcile: next=%v err=%v status=%+v", next, err, status)
	}
}
