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
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var testKeyValueBackupStore = BackupStore{
	DestinationPath: "s3://backups/keyvalue",
	EndpointURL:     "https://s3.example.test",
	S3Secret:        "kv-backup-creds",
}

func keyValueBackupTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(certManagerCertificateGVK, &unstructured.Unstructured{})
	return scheme
}

func TestKeyValueBackupCronJobSpec(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-paid-kv", Namespace: "default", UID: "paid-kv-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
	}
	r := &KeyValueReconciler{Backup: testKeyValueBackupStore}
	spec := r.keyValueBackupCronJobSpec(kv, "red-paid-kv-auth")

	var minute, hour int
	if _, err := fmt.Sscanf(spec.Schedule, "%d %d * * *", &minute, &hour); err != nil || hour != 3 || minute < 20 || minute > 39 {
		t.Fatalf("schedule = %q, want a stable 03:20-03:39 UTC slot", spec.Schedule)
	}
	if spec.TimeZone == nil || *spec.TimeZone != "Etc/UTC" || spec.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Fatalf("cron timing contract = zone %v concurrency %q", spec.TimeZone, spec.ConcurrencyPolicy)
	}
	if spec.Suspend == nil || *spec.Suspend {
		t.Fatal("running paid KeyValue backup CronJob is unexpectedly suspended")
	}
	pod := spec.JobTemplate.Spec.Template.Spec
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Fatal("backup pod must not mount a ServiceAccount token")
	}
	if len(pod.InitContainers) != 2 || len(pod.Containers) != 1 {
		t.Fatalf("backup stages = %d init + %d main, want snapshot/compress/upload", len(pod.InitContainers), len(pod.Containers))
	}
	snapshot := pod.InitContainers[0]
	if !strings.Contains(snapshot.Args[0], "valkey-cli") || !strings.Contains(snapshot.Args[0], "--rdb /backup/dump.rdb") {
		t.Fatalf("snapshot command does not request a coherent remote RDB: %q", snapshot.Args[0])
	}
	if strings.Contains(snapshot.Args[0], " -a ") || len(snapshot.Env) != 2 || snapshot.Env[1].Name != "REDISCLI_AUTH" ||
		snapshot.Env[1].ValueFrom.SecretKeyRef.Name != "red-paid-kv-auth" {
		t.Fatalf("snapshot authentication must use REDISCLI_AUTH from the authority Secret: %#v", snapshot.Env)
	}
	upload := pod.Containers[0]
	for _, required := range []string{"retain=7", "rdb.gz", `head -n "-${retain}"`, "aws --endpoint-url"} {
		if !strings.Contains(upload.Args[0], required) {
			t.Errorf("upload command lacks %q: %s", required, upload.Args[0])
		}
	}
	if len(upload.EnvFrom) != 1 || upload.EnvFrom[0].SecretRef.Name != testKeyValueBackupStore.S3Secret {
		t.Fatalf("uploader credential source = %#v", upload.EnvFrom)
	}
	if len(upload.Env) < 2 || upload.Env[0].Name != "HOME" || upload.Env[0].Value != "/tmp" ||
		upload.Env[1].Name != "AWS_EC2_METADATA_DISABLED" || upload.Env[1].Value != "true" {
		t.Fatalf("uploader writable AWS config contract = %#v", upload.Env)
	}
	for _, container := range append(append([]corev1.Container{}, pod.InitContainers...), pod.Containers...) {
		security := container.SecurityContext
		if security == nil || security.AllowPrivilegeEscalation == nil || *security.AllowPrivilegeEscalation ||
			security.Capabilities == nil || len(security.Capabilities.Drop) == 0 || security.Capabilities.Drop[0] != "ALL" ||
			security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
			t.Errorf("container %s lost the platform hardening baseline: %#v", container.Name, security)
		}
	}

	kv.Spec.Suspended = true
	suspended := r.keyValueBackupCronJobSpec(kv, "red-paid-kv-auth")
	if suspended.Suspend == nil || !*suspended.Suspend {
		t.Fatal("suspended KeyValue must suspend its backup CronJob")
	}
	if got := keyValueBackupSchedule(kv.Name); got != spec.Schedule {
		t.Fatalf("schedule drifted for the same resource: %q != %q", got, spec.Schedule)
	}
	if got := keyValueBackupName(strings.Repeat("a", 80)); len(got) > 63 {
		t.Fatalf("bounded CronJob name is %d bytes: %q", len(got), got)
	}
	longKV := kv.DeepCopy()
	longKV.Name = strings.Repeat("a", 63)
	if got := r.keyValueBackupPurgeJob(longKV).Name; len(got) > 63 {
		t.Fatalf("bounded purge Job name is %d bytes: %q", len(got), got)
	}
}

func TestKeyValueBackupPlanAndConfigurationGating(t *testing.T) {
	free, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
	paid, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "starter"})
	if keyValueBackupsEnabled(free, testKeyValueBackupStore) {
		t.Fatal("Free plan unexpectedly enabled backups")
	}
	if !keyValueBackupsEnabled(paid, testKeyValueBackupStore) {
		t.Fatal("paid plan with a complete store did not enable backups")
	}
	for _, incomplete := range []BackupStore{
		{},
		{DestinationPath: testKeyValueBackupStore.DestinationPath},
		{DestinationPath: testKeyValueBackupStore.DestinationPath, EndpointURL: testKeyValueBackupStore.EndpointURL},
	} {
		if keyValueBackupsEnabled(paid, incomplete) {
			t.Fatalf("incomplete store unexpectedly enabled backups: %+v", incomplete)
		}
	}
}

func TestKeyValueBackupReconcileGatesAndTracksPlanChanges(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	ctx := context.Background()
	nn := types.NamespacedName{Name: "red-kv-gating", Namespace: "default"}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace, UID: "kv-gating-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "gating", Plan: "starter"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}, &appsv1.StatefulSet{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, Backup: testKeyValueBackupStore}
	req := reconcile.Request{NamespacedName: nn}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(kv, kvFinalizer) {
		t.Fatal("paid backed-up KeyValue did not receive the purge finalizer")
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	cronKey := types.NamespacedName{Name: keyValueBackupName(kv.Name), Namespace: kv.Namespace}
	cron := &batchv1.CronJob{}
	if err := cl.Get(ctx, cronKey, cron); err != nil {
		t.Fatalf("paid plan did not get a backup CronJob: %v", err)
	}
	if !metav1.IsControlledBy(cron, kv) {
		t.Fatal("backup CronJob is not controller-owned by the KeyValue")
	}

	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatal(err)
	}
	kv.Spec.Plan = "free"
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, cronKey, &batchv1.CronJob{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Free downgrade retained the backup CronJob: %v", err)
	}
	if err := cl.Get(ctx, nn, kv); err != nil || !controllerutil.ContainsFinalizer(kv, kvFinalizer) {
		t.Fatalf("downgrade lost the historical-prefix purge finalizer: err=%v finalizers=%v", err, kv.Finalizers)
	}

	localKV := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-kv-disabled", Namespace: "default", UID: "kv-disabled-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "disabled", Plan: "starter"},
	}
	localClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(localKV).
		WithStatusSubresource(&appv1alpha1.KeyValue{}, &appsv1.StatefulSet{}).Build()
	local := &KeyValueReconciler{Client: localClient, Scheme: scheme}
	localReq := reconcile.Request{NamespacedName: types.NamespacedName{Name: localKV.Name, Namespace: localKV.Namespace}}
	if _, err := local.Reconcile(ctx, localReq); err != nil {
		t.Fatal(err)
	}
	if err := localClient.Get(ctx, localReq.NamespacedName, localKV); err != nil {
		t.Fatal(err)
	}
	if controllerutil.ContainsFinalizer(localKV, kvFinalizer) {
		t.Fatal("fully disabled backup config changed the KeyValue finalizer set")
	}
	if err := localClient.Get(ctx, types.NamespacedName{Name: keyValueBackupName(localKV.Name), Namespace: localKV.Namespace}, &batchv1.CronJob{}); !apierrors.IsNotFound(err) {
		t.Fatalf("fully disabled config created a CronJob: %v", err)
	}
}

func TestKeyValueDeletionWaitsForBackupJobsAndPurge(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	now := metav1.Now()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "red-kv-delete", Namespace: "default", UID: "kv-delete-uid",
		Finalizers: []string{kvFinalizer}, DeletionTimestamp: &now,
	}}
	labels := keyValueBackupLabels(kv, keyValueBackupComponent)
	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: keyValueBackupName(kv.Name), Namespace: kv.Namespace}}
	backupJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "kv-backup-running", Namespace: kv.Namespace, Labels: labels}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, cron, backupJob).
		WithStatusSubresource(&appv1alpha1.KeyValue{}, &batchv1.Job{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, Backup: testKeyValueBackupStore}
	ctx := context.Background()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(cron), &batchv1.CronJob{}); !apierrors.IsNotFound(err) {
		t.Fatalf("backup CronJob survived teardown: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(backupJob), &batchv1.Job{}); err != nil {
		t.Fatalf("backup Job was removed before the CronJob absence was observed: %v", err)
	}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(backupJob), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("backup Job survived teardown: %v", err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	purge := r.keyValueBackupPurgeJob(kv)
	if err := cl.Get(ctx, client.ObjectKeyFromObject(purge), purge); err != nil {
		t.Fatalf("S3 purge Job was not persisted: %v", err)
	}
	purge.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(ctx, purge); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	if err := cl.Get(ctx, req.NamespacedName, &appv1alpha1.KeyValue{}); !apierrors.IsNotFound(err) {
		t.Fatalf("KeyValue survived proven S3 purge: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(purge), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("terminal purge Job survived finalization: %v", err)
	}
}

func TestKeyValueDeletionDoesNotWedgeWhenBackupsAreDisabled(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	now := metav1.Now()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "red-kv-disabled-delete", Namespace: "default", UID: "kv-disabled-delete-uid",
		Finalizers: []string{kvFinalizer}, DeletionTimestamp: &now,
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), req.NamespacedName, &appv1alpha1.KeyValue{}); !apierrors.IsNotFound(err) {
		t.Fatalf("disabled backup config wedged deletion: %v", err)
	}
}

func TestKeyValueDeletionRetainsFinalizerWhenPurgeFails(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	now := metav1.Now()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "red-kv-purge-fails", Namespace: "default", UID: "kv-purge-fails-uid",
		Finalizers: []string{kvFinalizer}, DeletionTimestamp: &now,
	}}
	r := &KeyValueReconciler{Scheme: scheme, Backup: testKeyValueBackupStore}
	purge := r.keyValueBackupPurgeJob(kv)
	purge.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "UploadDenied", Message: "object store denied delete",
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, purge).
		WithStatusSubresource(&appv1alpha1.KeyValue{}, &batchv1.Job{}).Build()
	r.Client = cl
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}}
	if _, err := r.Reconcile(context.Background(), req); err == nil || !strings.Contains(err.Error(), "object store denied delete") {
		t.Fatalf("failed purge error = %v", err)
	}
	current := &appv1alpha1.KeyValue{}
	if err := cl.Get(context.Background(), req.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(current, kvFinalizer) {
		t.Fatal("failed purge released the KeyValue finalizer")
	}
}

func TestKeyValueBackupPurgeUsesWritableAWSConfigHome(t *testing.T) {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-kv-purge-home", Namespace: "default", UID: "kv-purge-home-uid"}}
	r := &KeyValueReconciler{Backup: testKeyValueBackupStore}
	env := r.keyValueBackupPurgeJob(kv).Spec.Template.Spec.Containers[0].Env
	if len(env) < 2 || env[0].Name != "HOME" || env[0].Value != "/tmp" ||
		env[1].Name != "AWS_EC2_METADATA_DISABLED" || env[1].Value != "true" {
		t.Fatalf("purge writable AWS config contract = %#v", env)
	}
}
