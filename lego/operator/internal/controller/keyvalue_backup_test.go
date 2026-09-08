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
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

func assertKeyValueBackupSchedule(t *testing.T, spec *batchv1.CronJob) {
	t.Helper()
	var minute, hour int
	if _, err := fmt.Sscanf(spec.Spec.Schedule, "%d %d * * *", &minute, &hour); err != nil || hour != 3 || minute < 20 || minute > 39 {
		t.Fatalf("schedule = %q, want a stable 03:20-03:39 UTC slot", spec.Spec.Schedule)
	}
	if spec.Spec.TimeZone == nil || *spec.Spec.TimeZone != "Etc/UTC" || spec.Spec.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Fatalf("cron timing contract = zone %v concurrency %q", spec.Spec.TimeZone, spec.Spec.ConcurrencyPolicy)
	}
	if spec.Spec.Suspend == nil || *spec.Spec.Suspend {
		t.Fatal("running paid KeyValue backup CronJob is unexpectedly suspended")
	}
}

func assertKeyValueBackupPod(t *testing.T, pod corev1.PodSpec) {
	t.Helper()
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Fatal("backup pod must not mount a ServiceAccount token")
	}
	if len(pod.InitContainers) != 2 || len(pod.Containers) != 1 {
		t.Fatalf("backup stages = %d init + %d main, want snapshot/compress/upload", len(pod.InitContainers), len(pod.Containers))
	}
	assertKeyValueSnapshot(t, pod.InitContainers[0])
	assertKeyValueUpload(t, pod.Containers[0])
	for _, container := range append(append([]corev1.Container{}, pod.InitContainers...), pod.Containers...) {
		security := container.SecurityContext
		if security == nil || security.AllowPrivilegeEscalation == nil || *security.AllowPrivilegeEscalation ||
			security.Capabilities == nil || len(security.Capabilities.Drop) == 0 || security.Capabilities.Drop[0] != "ALL" ||
			security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
			t.Errorf("container %s lost the platform hardening baseline: %#v", container.Name, security)
		}
	}
}

func assertKeyValueSnapshot(t *testing.T, snapshot corev1.Container) {
	t.Helper()
	if !strings.Contains(snapshot.Args[0], "valkey-cli") || !strings.Contains(snapshot.Args[0], "--rdb /backup/dump.rdb") {
		t.Fatalf("snapshot command does not request a coherent remote RDB: %q", snapshot.Args[0])
	}
	if strings.Contains(snapshot.Args[0], " -a ") || len(snapshot.Env) != 2 || snapshot.Env[1].Name != "REDISCLI_AUTH" ||
		snapshot.Env[1].ValueFrom.SecretKeyRef.Name != "red-paid-kv-auth" {
		t.Fatalf("snapshot authentication must use REDISCLI_AUTH from the authority Secret: %#v", snapshot.Env)
	}
}

func assertKeyValueUpload(t *testing.T, upload corev1.Container) {
	t.Helper()
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
}

// testBackupHelperImage stands in for what selfimage.Resolve reads off the
// manager Pod in a cluster: the operator's own digest-pinned image, whose
// /backup-encrypt entrypoint is the encrypt stage (w7/m85).
const testBackupHelperImage = "ghcr.io/bex-co/bex-operator@sha256:" +
	"5efd2d8c754176992ec59cb688ae8aa19b8dc2d71bff542a1c91c76603c9a76e"

// TestKeyValueBackupFixedImagesAreDigestPinned (round-14 #5): every backup
// stage whose image is a reference BEX ITSELF chooses — busybox compress, the
// first-party encrypt step, the AWS CLI uploader, and the default-version
// valkey snapshot — must carry an immutable sha256 digest. These containers
// share the plaintext backup volume and (snapshot/uploader) the datastore and
// S3 credentials, so a retagged mutable tag must not be able to become their
// code.
//
// w1/m73 removed the one exemption this test used to carry. A snapshot stage
// whose image came from an EXPLICIT tenant-chosen spec.version was allowed to
// be unpinned, on the reasoning that bex "cannot pre-resolve a digest for a
// major it has not seen" — but KeyValueSpec.Version is a closed CRD enum, so it
// never sees one. Every version now resolves to a pinned image
// (kvVersionImages), and every stage below is asserted pinned.
func TestKeyValueBackupFixedImagesAreDigestPinned(t *testing.T) {
	withAge := BackupStore{DestinationPath: "s3://b", EndpointURL: "https://s3.example.test", S3Secret: "s", AgePublicKey: "age1sentinel"}
	for _, tc := range []struct {
		name  string
		store BackupStore
	}{
		{"plain", testKeyValueBackupStore},
		{"age-encrypted", withAge},
	} {
		for _, vc := range []struct {
			name           string
			version        string
			snapshotPinned bool
		}{
			// Both paths are pinned since w1/m73 — the default bex owns end to
			// end, and each major the CRD enum permits.
			{"default-version", "", true},
			{"explicit-version-8", "8", true},
			{"explicit-version-7", "7", true},
		} {
			t.Run(tc.name+"/"+vc.name, func(t *testing.T) {
				kv := &appv1alpha1.KeyValue{
					ObjectMeta: metav1.ObjectMeta{Name: "red-paid-kv", Namespace: "default", UID: "paid-kv-uid"},
					Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: vc.version},
				}
				r := &KeyValueReconciler{Backup: tc.store, BackupHelperImage: testBackupHelperImage}
				spec := r.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "red-paid-kv-auth")
				pod := spec.JobTemplate.Spec.Template.Spec
				sawSnapshot := false
				for _, c := range append(append([]corev1.Container{}, pod.InitContainers...), pod.Containers...) {
					pinned := strings.Contains(c.Image, "@sha256:")
					if c.Name == "snapshot" {
						sawSnapshot = true
						if pinned != vc.snapshotPinned {
							t.Errorf("snapshot image %q digest-pinned=%v, want %v", c.Image, pinned, vc.snapshotPinned)
						}
						continue
					}
					if !pinned {
						t.Errorf("backup stage %q image %q is not digest-pinned — a mutable tag in the credential-bearing pipeline", c.Name, c.Image)
					}
				}
				if !sawSnapshot {
					t.Fatal("no snapshot stage in the rendered backup job")
				}
			})
		}
	}
}

func TestKeyValueBackupCronJobSpec(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-paid-kv", Namespace: "default", UID: "paid-kv-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
	}
	r := &KeyValueReconciler{Backup: testKeyValueBackupStore}
	spec := r.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "red-paid-kv-auth")

	cron := &batchv1.CronJob{Spec: spec}
	assertKeyValueBackupSchedule(t, cron)
	assertKeyValueBackupPod(t, cron.Spec.JobTemplate.Spec.Template.Spec)

	kv.Spec.Suspended = true
	suspended := r.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "red-paid-kv-auth")
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

func TestKeyValueBackupEncryptStepDisabledByDefault(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-plain-kv", Namespace: "default", UID: "plain-kv-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
	}
	// testKeyValueBackupStore has no AgePublicKey ⇒ byte-identical pre-ADR050 shape.
	r := &KeyValueReconciler{Backup: testKeyValueBackupStore}
	pod := r.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "red-plain-kv-auth").JobTemplate.Spec.Template.Spec
	if len(pod.InitContainers) != 2 {
		t.Fatalf("encryption off must keep snapshot+compress only, got %d init containers", len(pod.InitContainers))
	}
	for _, c := range pod.InitContainers {
		if c.Name == "encrypt" {
			t.Fatal("encrypt step present with no AgePublicKey configured")
		}
	}
	upload := pod.Containers[0].Args[0]
	if strings.Contains(upload, ".age") {
		t.Fatalf("plain upload must not reference .age objects: %s", upload)
	}
	if !strings.Contains(upload, `s3 cp /backup/dump.rdb.gz "${prefix}${timestamp}.rdb.gz"`) {
		t.Fatalf("plain upload lost its .rdb.gz object naming: %s", upload)
	}
}

func TestKeyValueBackupEncryptStepWhenKeyConfigured(t *testing.T) {
	const pubKey = "age1qqqexamplepublicrecipientkeyfortestonly000000000000000000"
	store := testKeyValueBackupStore
	store.AgePublicKey = pubKey
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-enc-kv", Namespace: "default", UID: "enc-kv-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
	}
	r := &KeyValueReconciler{Backup: store, BackupHelperImage: testBackupHelperImage}
	pod := r.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "red-enc-kv-auth").JobTemplate.Spec.Template.Spec

	// The encrypt step slots between compress and upload (snapshot, compress, encrypt).
	if len(pod.InitContainers) != 3 {
		t.Fatalf("encryption on must add an encrypt step, got %d init containers", len(pod.InitContainers))
	}
	if pod.InitContainers[1].Name != "compress" || pod.InitContainers[2].Name != "encrypt" {
		t.Fatalf("encrypt must follow compress, got order %s,%s,%s",
			pod.InitContainers[0].Name, pod.InitContainers[1].Name, pod.InitContainers[2].Name)
	}
	encrypt := pod.InitContainers[2]
	// w7/m85 (ADR067 #8 → ADR068 #9): the stage that reads the PLAINTEXT RDB
	// resolves NOTHING at run time — no package index, and no release tarball
	// downloaded next to the unencrypted backup either. It execs a first-party
	// entrypoint out of the operator's own image with plain file arguments, so
	// there is no shell to inject into and nothing to fetch.
	if len(encrypt.Command) != 1 || encrypt.Command[0] != "/backup-encrypt" {
		t.Fatalf("encrypt must exec the first-party helper, got command %#v", encrypt.Command)
	}
	joined := strings.Join(append(append([]string{}, encrypt.Command...), encrypt.Args...), " ")
	for _, forbidden := range []string{"apk", "wget", "curl", "http://", "https://", "sh -c", "/bin/sh"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("encrypt stage resolves %q at run time: %q", forbidden, joined)
		}
	}
	if len(encrypt.Args) != 2 ||
		encrypt.Args[0] != "/backup/dump.rdb.gz" || encrypt.Args[1] != "/backup/dump.rdb.gz.age" {
		t.Fatalf("encrypt args = %#v, want plaintext in + ciphertext out", encrypt.Args)
	}
	env := map[string]string{}
	for _, e := range encrypt.Env {
		env[e.Name] = e.Value
	}
	if len(env) != 1 || env["AGE_PUBLIC_KEY"] != pubKey {
		t.Fatalf("encrypt env = %#v, want exactly the recipient", encrypt.Env)
	}
	if encrypt.Image != testBackupHelperImage {
		t.Fatalf("encrypt image %q is not the operator's own resolved image", encrypt.Image)
	}
	// The encrypt step keeps the platform hardening baseline.
	sec := encrypt.SecurityContext
	if sec == nil || sec.AllowPrivilegeEscalation == nil || *sec.AllowPrivilegeEscalation ||
		sec.Capabilities == nil || len(sec.Capabilities.Drop) == 0 || sec.Capabilities.Drop[0] != "ALL" ||
		sec.SeccompProfile == nil || sec.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("encrypt container lost the hardening baseline: %#v", sec)
	}

	// The upload now ships the .age object and prunes across both suffixes.
	upload := pod.Containers[0].Args[0]
	if !strings.Contains(upload, `s3 cp /backup/dump.rdb.gz.age "${prefix}${timestamp}.rdb.gz.age"`) {
		t.Fatalf("upload must ship the encrypted object: %s", upload)
	}
	if !strings.Contains(upload, `[.]rdb[.]gz([.]age)?$`) {
		t.Fatalf("retention must match both plain and .age objects across the transition: %s", upload)
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

// TestKeyValueDeletionSurfacesStalledConditionOnOverrun is w8/012: a KeyValue
// finalization past its bound stamps DeletionStalled and backs off requeue.
func TestKeyValueDeletionSurfacesStalledConditionOnOverrun(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	now := metav1.Now()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "stalled-kv", Namespace: "default", UID: "uid-stalled-kv",
		Finalizers: []string{kvFinalizer}, DeletionTimestamp: &now,
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}, &batchv1.Job{}).Build()
	r := &KeyValueReconciler{
		Client: cl, Scheme: scheme, FinalizerOverrunAfter: time.Nanosecond,
		Backup: testKeyValueBackupStore,
	}
	nn := types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}
	result, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != childHealthRequeue {
		t.Fatalf("stalled requeue = %v, want childHealthRequeue %v", result.RequeueAfter, childHealthRequeue)
	}
	var current appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(&current, kvFinalizer) {
		t.Fatal("finalizer must be retained while stalled")
	}
	cond := meta.FindStatusCondition(current.Status.Conditions, conditionDeletionStalled)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("DeletionStalled condition = %+v, want present and True", cond)
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

// TestKeyValueBackupNetworkPolicyRestoresJobEgress pins the w5/039 fix: the
// platform-wide Cilium node/metadata deny selects every app.bex.co/workspace pod
// (including the backup/purge Jobs) into egress default-deny, so without this
// operator-managed allow the snapshot step can't even resolve the Valkey Service
// (DNS "Try again") and no KeyValue backup ever uploads.
func TestKeyValueBackupNetworkPolicyRestoresJobEgress(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "red-egress-kv", Namespace: "default", UID: "egress-kv-uid",
		Labels: map[string]string{labelWorkspace: "tea-egresstest"},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, Backup: testKeyValueBackupStore}
	if err := r.reconcileKeyValueBackupNetworkPolicy(context.Background(), kv); err != nil {
		t.Fatal(err)
	}

	np := &networkingv1.NetworkPolicy{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: kv.Name + "-backup-egress", Namespace: kv.Namespace}, np); err != nil {
		t.Fatalf("backup egress NetworkPolicy not created: %v", err)
	}
	// Egress-only: the Valkey server's ingress (client connections) must stay
	// governed elsewhere, never restricted by this Job-scoped allow.
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != networkingv1.PolicyTypeEgress {
		t.Fatalf("policy must be egress-only, got %v", np.Spec.PolicyTypes)
	}
	// Scoped to the Job pods (which carry app.bex.co/component), not the Valkey
	// server pods (which never dial out and keep their default-deny egress).
	sel := np.Spec.PodSelector
	if sel.MatchLabels[labelKeyValue] != kv.Name || len(sel.MatchExpressions) != 1 ||
		sel.MatchExpressions[0].Key != "app.bex.co/component" ||
		sel.MatchExpressions[0].Operator != metav1.LabelSelectorOpIn {
		t.Fatalf("selector must scope to this KeyValue's backup/purge Jobs: %#v", sel)
	}

	var dns, valkey, internet bool
	for _, rule := range np.Spec.Egress {
		switch {
		case len(rule.Ports) == 2 && rule.Ports[0].Port.IntValue() == 53:
			// DNS to kube-system CoreDNS — the exact hop the 039 failure lost.
			if len(rule.To) != 1 || rule.To[0].NamespaceSelector == nil ||
				rule.To[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] != "kube-system" {
				t.Fatalf("DNS egress must target kube-system, got %#v", rule.To)
			}
			dns = true
		case len(rule.Ports) == 2 && rule.Ports[0].Port.IntValue() == kvPort:
			// Reach the Valkey Service pods (a private IP the internet rule excludes).
			if len(rule.To) != 1 || rule.To[0].PodSelector == nil ||
				rule.To[0].PodSelector.MatchLabels[labelKeyValue] != kv.Name {
				t.Fatalf("Valkey egress must target this KeyValue's pods, got %#v", rule.To)
			}
			valkey = true
		case len(rule.To) == 1 && rule.To[0].IPBlock != nil && rule.To[0].IPBlock.CIDR == "0.0.0.0/0":
			// Object-store upload, minus in-cluster/private + metadata ranges.
			for _, private := range []string{"10.0.0.0/8", "169.254.0.0/16"} {
				if !slices.Contains(rule.To[0].IPBlock.Except, private) {
					t.Fatalf("internet egress must still except %s (metadata/private): %#v", private, rule.To[0].IPBlock.Except)
				}
			}
			internet = true
		}
	}
	if !dns || !valkey || !internet {
		t.Fatalf("egress allow incomplete: dns=%v valkey=%v internet=%v", dns, valkey, internet)
	}

	if len(np.OwnerReferences) != 1 || np.OwnerReferences[0].Name != kv.Name {
		t.Fatalf("policy must be owned by its KeyValue for GC, got %#v", np.OwnerReferences)
	}
}

// TestKeyValueBackupEncryptionFailsClosedWithoutHelperImage (w7/m85): the
// encrypt stage runs the operator's OWN image, resolved off the manager Pod at
// startup. If that resolution fails — no POD_NAME, no RBAC, no override — the
// reconcile must error rather than converge a CronJob, because the only two
// alternatives are a CronJob the API server rejects for an empty image and, if
// the stage were quietly dropped instead, a nightly PLAINTEXT upload to a
// bucket the operator was explicitly told to encrypt into.
//
// Unencrypted backups do not depend on the helper image and must be unaffected.
func TestKeyValueBackupEncryptionFailsClosedWithoutHelperImage(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	ctx := context.Background()
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-failclosed-kv", Namespace: "default", UID: "failclosed-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "failclosed", Plan: "starter"},
	}
	encrypted := testKeyValueBackupStore
	encrypted.AgePublicKey = "age1sentinelrecipient"

	cronKey := types.NamespacedName{Name: keyValueBackupName(kv.Name), Namespace: kv.Namespace}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv.DeepCopy()).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, Backup: encrypted}
	err := r.reconcileKeyValueBackup(ctx, kv, starterValkeyTier(), "red-failclosed-kv-auth")
	if err == nil {
		t.Fatal("encryption configured with no helper image must fail the reconcile, not converge a CronJob")
	}
	if !strings.Contains(err.Error(), "BEX_BACKUP_HELPER_IMAGE") || !strings.Contains(err.Error(), "POD_NAME") {
		t.Fatalf("fail-closed error must name both remedies, got %v", err)
	}
	if getErr := cl.Get(ctx, cronKey, &batchv1.CronJob{}); !apierrors.IsNotFound(getErr) {
		t.Fatalf("no CronJob may exist after the fail-closed reconcile, got %v", getErr)
	}

	// Same operator, same missing helper image, encryption OFF: unchanged.
	plain := &KeyValueReconciler{Client: cl, Scheme: scheme, Backup: testKeyValueBackupStore}
	if err := plain.reconcileKeyValueBackup(ctx, kv, starterValkeyTier(), "red-failclosed-kv-auth"); err != nil {
		t.Fatalf("unencrypted backups must not depend on the helper image: %v", err)
	}
	cron := &batchv1.CronJob{}
	if err := cl.Get(ctx, cronKey, cron); err != nil {
		t.Fatalf("unencrypted backup CronJob was not created: %v", err)
	}
	if len(cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers) != 2 {
		t.Fatal("unencrypted pipeline must stay snapshot+compress")
	}
}
