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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastore_secret_plane_test.go guards ADR043 D8.2 (w7/m77/t012).
//
// A Database/KeyValue now lives in its workspace's own `<ws>` namespace, but the
// manager's Secret informer covers exactly one namespace
// (NamespacedSecretCacheOptions) and CANNOT be widened: the operator's
// ClusterRole deliberately omits cluster-wide Secrets (w7/m7), so a cluster-wide
// Secret list would fail and stop the whole shared cache — App controller
// included — from starting. Datastore Secret access therefore has to run on the
// uncached client, the same escape hatch the App path uses.
//
// The regression this pins is silent and only reproduces in production: with a
// cached client the reads simply do not see a tenant namespace. So the test
// hands the reconciler two DISTINCT clients and checks which one the Secrets
// landed in — a call site reverted to r.Client fails here immediately.

func datastoreSecretScheme(t *testing.T) *runtime.Scheme {
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

func TestKeyValueCredentialsUseTheUncachedSecretClient(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	// A tenant namespace, i.e. NOT the one the manager's Secret informer covers.
	const ns = "tea-secretplane"

	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-plane", Namespace: ns},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free"},
	}
	cached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	uncached := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &KeyValueReconciler{Client: cached, Scheme: scheme, SecretClient: uncached}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "red-plane", Namespace: ns}
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	for _, name := range []string{"red-plane", "red-plane-auth"} {
		var onUncached corev1.Secret
		if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &onUncached); err != nil {
			t.Errorf("Secret %q was not written through SecretClient: %v — a tenant-namespace Secret written on the cached client is invisible to the operator in production", name, err)
		}
		var onCached corev1.Secret
		if err := cached.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &onCached); !apierrors.IsNotFound(err) {
			t.Errorf("Secret %q reached the CACHED client (err=%v); the Secret plane must not depend on the manager cache", name, err)
		}
	}
}

func TestPoolerSecretUsesTheUncachedSecretClient(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	const ns = "tea-secretplane"

	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-plane", Namespace: ns},
		Status:     appv1alpha1.DatabaseStatus{PoolerHost: "dpg-plane-pooler." + ns + ".svc"},
	}
	// CNPG's application Secret — the source the pooler Secret projects from. It
	// lives beside the Database, so it too is only reachable uncached.
	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-plane-app", Namespace: ns},
		Data: map[string][]byte{
			"username": []byte("app"),
			"password": []byte("pw"),
			"dbname":   []byte("appdb"),
		},
	}
	cached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	uncached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(source).Build()

	r := &DatabaseReconciler{Client: cached, Scheme: scheme, SecretClient: uncached}
	ctx := context.Background()

	ok, err := r.reconcilePoolerConnectionSecret(ctx, db)
	if err != nil || !ok {
		t.Fatalf("reconcilePoolerConnectionSecret: ok=%v err=%v — the CNPG source Secret was not found on the uncached client", ok, err)
	}

	var projected corev1.Secret
	if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "dpg-plane-pooler-app"}, &projected); err != nil {
		t.Fatalf("pooler Secret was not written through SecretClient: %v", err)
	}
	if len(projected.Data["uri"]) == 0 {
		t.Error("pooler Secret carries no uri")
	}
	var onCached corev1.Secret
	if err := cached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "dpg-plane-pooler-app"}, &onCached); !apierrors.IsNotFound(err) {
		t.Errorf("pooler Secret reached the CACHED client (err=%v)", err)
	}

	// Delete follows the same plane, or a workspace teardown would leave the
	// projected credential behind in a namespace the operator cannot see.
	if err := r.deletePoolerConnectionSecret(ctx, db); err != nil {
		t.Fatalf("deletePoolerConnectionSecret: %v", err)
	}
	if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "dpg-plane-pooler-app"}, &projected); !apierrors.IsNotFound(err) {
		t.Errorf("pooler Secret survived delete on the uncached client (err=%v)", err)
	}
}

// TestSecretClientFallsBackToTheCachedClient pins the nil-SecretClient contract
// the existing tests and embedders rely on: without one wired, behavior is
// exactly as before. Otherwise every test constructing a bare reconciler would
// silently start writing Secrets nowhere.
func TestSecretClientFallsBackToTheCachedClient(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	if got := (&KeyValueReconciler{Client: cl}).secretClient(); got != cl {
		t.Error("KeyValueReconciler.secretClient() did not fall back to the cached client")
	}
	if got := (&DatabaseReconciler{Client: cl}).secretClient(); got != cl {
		t.Error("DatabaseReconciler.secretClient() did not fall back to the cached client")
	}
}

// --- ADR043 D8.4: per-tenant backup transport (w7/m77/t004) ---

// TestTenantBackupStoreProjectsCredentialAndObjectStore pins that a Database in
// a tenant namespace gets both halves of the backup transport there.
//
// Both are namespaced and GitOps installs exactly one of each, so without this
// projection the Cluster comes up referencing a barmanObjectName that does not
// resolve — and CNPG reports that as "backups not configured" rather than as an
// error, i.e. a Postgres that looks healthy and archives nothing.
func TestTenantBackupStoreProjectsCredentialAndObjectStore(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	scheme.AddKnownTypeWithName(barmanCloudObjectStoreGVK, &unstructured.Unstructured{})
	const src, tenant = "default", "tea-backup"

	sourceStore := &unstructured.Unstructured{}
	sourceStore.SetGroupVersionKind(barmanCloudObjectStoreGVK)
	sourceStore.SetName(tenantBackupObjectStoreName)
	sourceStore.SetNamespace(src)
	if err := unstructured.SetNestedMap(sourceStore.Object, map[string]any{
		"configuration":   map[string]any{"destinationPath": "s3://bex-tfstate/postgres"},
		"retentionPolicy": "30d",
	}, "spec"); err != nil {
		t.Fatal(err)
	}
	credential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bex-db-backup-s3", Namespace: src},
		Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("k"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sourceStore).Build()
	secretCl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credential).Build()
	r := &DatabaseReconciler{
		Client: cl, Scheme: scheme, SecretClient: secretCl, BackupSourceNamespace: src,
		Backup: BackupStore{
			DestinationPath: "s3://bex-tfstate/postgres",
			EndpointURL:     "https://s3.example.test",
			S3Secret:        "bex-db-backup-s3",
		},
	}
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-bk", Namespace: tenant}}
	ctx := context.Background()
	if err := r.reconcileTenantBackupStore(ctx, db); err != nil {
		t.Fatalf("reconcileTenantBackupStore: %v", err)
	}

	var projectedSecret corev1.Secret
	if err := secretCl.Get(ctx, client.ObjectKey{Namespace: tenant, Name: "bex-db-backup-s3"}, &projectedSecret); err != nil {
		t.Fatalf("credential not projected into the tenant namespace: %v", err)
	}
	if string(projectedSecret.Data["AWS_ACCESS_KEY_ID"]) != "k" {
		t.Error("projected credential lost its data")
	}

	projectedStore := &unstructured.Unstructured{}
	projectedStore.SetGroupVersionKind(barmanCloudObjectStoreGVK)
	if err := cl.Get(ctx, client.ObjectKey{Namespace: tenant, Name: tenantBackupObjectStoreName}, projectedStore); err != nil {
		t.Fatalf("ObjectStore not projected into the tenant namespace: %v", err)
	}
	// The spec is copied, not rebuilt from env — retention and destination stay
	// single-sourced in GitOps. A rebuilt spec would be a second place for the
	// backup policy to drift, discovered only at restore time.
	retention, _, _ := unstructured.NestedString(projectedStore.Object, "spec", "retentionPolicy")
	if retention != "30d" {
		t.Errorf("projected ObjectStore retentionPolicy = %q, want the source's 30d", retention)
	}
	// No ownerReference: the store is shared by every Database in the namespace,
	// so binding it to one would delete it when that one goes away and silently
	// break every sibling's archiving.
	if len(projectedStore.GetOwnerReferences()) != 0 {
		t.Errorf("projected ObjectStore carries ownerReferences %+v; it is namespace-shared and must outlive any single Database",
			projectedStore.GetOwnerReferences())
	}
}

// TestTenantBackupStoreIsNoOpInTheSourceNamespace pins the byte-identical
// pre-D8 path: a Database already beside the originals needs no projection, and
// a single-namespace deployment (BackupSourceNamespace empty) must not start
// writing objects it never wrote before.
func TestTenantBackupStoreIsNoOpInTheSourceNamespace(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	scheme.AddKnownTypeWithName(barmanCloudObjectStoreGVK, &unstructured.Unstructured{})
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &DatabaseReconciler{
		Client: cl, Scheme: scheme, SecretClient: cl,
		Backup: BackupStore{DestinationPath: "s3://x", EndpointURL: "https://e", S3Secret: "sec"},
	}
	ctx := context.Background()

	// Source namespace unset (single-namespace deployment).
	if err := r.reconcileTenantBackupStore(ctx, &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-a", Namespace: "default"},
	}); err != nil {
		t.Fatalf("unset BackupSourceNamespace must be a no-op, got: %v", err)
	}
	// Source namespace equal to the Database's own.
	r.BackupSourceNamespace = "default"
	if err := r.reconcileTenantBackupStore(ctx, &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-b", Namespace: "default"},
	}); err != nil {
		t.Fatalf("same-namespace Database must be a no-op, got: %v", err)
	}
	var secrets corev1.SecretList
	if err := cl.List(ctx, &secrets); err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 0 {
		t.Errorf("no-op path wrote %d Secrets", len(secrets.Items))
	}
}

// TestKeyValueTenantBackupCredentialProjected is the KeyValue half: its backup
// CronJob mounts the credential by name from its own namespace, so without the
// projection every nightly run fails at mount time — a backup that is scheduled
// forever and never produces a snapshot.
func TestKeyValueTenantBackupCredentialProjected(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	const src, tenant = "default", "tea-backup"

	credential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bex-kv-backup-s3", Namespace: src},
		Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("k")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	secretCl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credential).Build()
	r := &KeyValueReconciler{
		Client: cl, Scheme: scheme, SecretClient: secretCl, BackupSourceNamespace: src,
		Backup: BackupStore{DestinationPath: "s3://x", EndpointURL: "https://e", S3Secret: "bex-kv-backup-s3"},
	}
	ctx := context.Background()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-bk", Namespace: tenant}}
	if err := r.reconcileTenantBackupCredential(ctx, kv); err != nil {
		t.Fatalf("reconcileTenantBackupCredential: %v", err)
	}
	var projected corev1.Secret
	if err := secretCl.Get(ctx, client.ObjectKey{Namespace: tenant, Name: "bex-kv-backup-s3"}, &projected); err != nil {
		t.Fatalf("KeyValue backup credential not projected: %v", err)
	}
}

// --- ADR043 D8: linked datastore Secrets reach the pre-deploy Job (w7/m77/t005) ---

// TestPreDeployMirrorsLinkedDatastoreSecrets pins that a pre-deploy command can
// resolve the datastore link its App declares.
//
// A pre-deploy step is most often a database migration, so DATABASE_URL is
// exactly what it needs — but the Job runs in BEX_BUILD_NAMESPACE, and
// mirrorPreDeploySecrets historically copied only the whole-Secret sources
// (envFromSecrets / filesFromSecrets / the service's own env Secret). The
// per-variable secretKeyRefs that carry a Blueprint's fromDatabase/fromService
// link were never mirrored, so the migration pod failed at container-config time
// with `secret "<dpg-id>-app" not found` — a failed deploy with no reason
// attached. Pre-existing, but squarely part of "the link actually works".
func TestPreDeployMirrorsLinkedDatastoreSecrets(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	const appNS, buildNS = "tea-predeploy", "bex-build"

	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "forum", Namespace: appNS, UID: "uid-1"},
		Spec: appv1alpha1.AppSpec{
			EnvFromSecrets: []string{"evg-shared-env"},
			Env: []appv1alpha1.EnvVar{
				{Name: "PLAIN", Value: "literal"},
				{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{
					SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-forum-app", Key: "uri"},
				}},
				{Name: "DATABASE_HOST", ValueFrom: &appv1alpha1.EnvVarSource{
					// Same Secret as above — the mirror must not copy it twice.
					SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-forum-app", Key: "host"},
				}},
				{Name: "REDIS_URL", ValueFrom: &appv1alpha1.EnvVarSource{
					SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "red-forum", Key: "uri"},
				}},
			},
		},
	}
	secretNames := []string{"evg-shared-env", "dpg-forum-app", "red-forum"}
	objs := make([]client.Object, 0, len(secretNames)+1)
	objs = append(objs, app)
	for _, name := range secretNames {
		objs = append(objs, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: appNS},
			Data:       map[string][]byte{"k": []byte(name)},
		})
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()

	if err := r.mirrorPreDeploySecrets(ctx, app, buildNS); err != nil {
		t.Fatalf("mirrorPreDeploySecrets: %v", err)
	}
	for _, name := range secretNames {
		var got corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: name}, &got); err != nil {
			t.Errorf("Secret %q not mirrored into the pre-deploy namespace: %v — a migration referencing it fails with CreateContainerConfigError", name, err)
		}
	}
}

// TestPreDeployMirrorIsNoOpWhenCoLocated pins the unchanged path: with no
// separate build namespace the Job already runs beside the Secrets, and the
// mirror must not create copies that would then need cleaning up.
func TestPreDeployMirrorIsNoOpWhenCoLocated(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "forum", Namespace: "tea-x"},
		Spec: appv1alpha1.AppSpec{Env: []appv1alpha1.EnvVar{
			{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{
				SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-x-app", Key: "uri"},
			}},
		}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	if err := r.mirrorPreDeploySecrets(context.Background(), app, "tea-x"); err != nil {
		t.Fatalf("co-located mirror must be a no-op, got: %v", err)
	}
	var secrets corev1.SecretList
	if err := cl.List(context.Background(), &secrets); err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 0 {
		t.Errorf("co-located mirror wrote %d Secrets", len(secrets.Items))
	}
}
