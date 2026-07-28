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
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestDatabaseDisplayNameDoesNotAffectDataPlaneIdentity is the rename safety
// invariant. spec.name is API/display metadata only; every CNPG object,
// credential, route, backup path, and connection host remains derived from
// immutable metadata.name, while the database/user pair comes from immutable
// create-time fields (or their metadata-name defaults). Reconcile therefore
// updates no data-plane identity when only spec.name changes.
func TestDatabaseDisplayNameDoesNotAffectDataPlaneIdentity(t *testing.T) {
	before := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-c185th5c2rvvnhbfiltg", Namespace: "tenant-a"},
		Spec: appv1alpha1.DatabaseSpec{
			Name:             "orders-old",
			DatabaseName:     "orders_data",
			DatabaseUser:     "orders_owner",
			Plan:             "basic-1gb",
			Public:           true,
			Pooler:           true,
			HighAvailability: true,
		},
	}
	after := before.DeepCopy()
	after.Spec.Name = "orders-new"

	identity := func(db *appv1alpha1.Database) map[string]any {
		plan, storageGB := resolvePlan(db.Spec)
		dbname := db.Spec.EffectiveDatabaseName(db.Name)
		owner := db.Spec.EffectiveDatabaseUser(db.Name)
		return map[string]any{
			"cluster":         db.Name,
			"clusterSpec":     cnpgClusterSpec(clusterParams{plan: plan, storageGB: storageGB, dbname: dbname, owner: owner, highAvailability: db.Spec.HighAvailability}),
			"database":        dbname,
			"owner":           owner,
			"host":            db.Name + "-rw." + db.Namespace + ".svc",
			"secret":          db.Name + "-app",
			"scheduledBackup": db.Name + "-backup",
			"pooler":          db.Name + "-pooler",
			"publicRoute":     db.Name + "-pg",
			"poolRoute":       db.Name + "-pool",
			"backupServer":    db.Name,
		}
	}

	if !reflect.DeepEqual(identity(before), identity(after)) {
		t.Fatalf("spec.name changed a data-plane identity:\nbefore=%v\nafter=%v", identity(before), identity(after))
	}
}

var testStore = BackupStore{
	DestinationPath: "s3://bex-tfstate/postgres",
	EndpointURL:     "https://s3.eu-central-2.wasabisys.com",
	S3Secret:        "pg-backup-s3",
}

func TestResolvePlan(t *testing.T) {
	// known plan (from the shared lego/types/tiers postgres family)
	if p, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}); p.Memory != "1Gi" || gb != 5 {
		t.Errorf("basic-1gb => mem %q storage %d, want 1Gi/5", p.Memory, gb)
	}
	// unknown plan falls back to free
	if p, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "nonsense"}); p.Memory != "256Mi" {
		t.Errorf("unknown plan should default to free (256Mi), got %q", p.Memory)
	}
	// storage only grows: a request below the plan floor is raised to the floor
	if _, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 2}); gb != 5 {
		t.Errorf("storage below plan floor should be raised to 5, got %d", gb)
	}
	// a larger request is honored
	if _, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 10}); gb != 10 {
		t.Errorf("larger storage request should be honored, got %d", gb)
	}
}

// TestDatabasePlanChangeProducesNewResources confirms that updating spec.plan
// causes the reconciler to project a CNPG Cluster spec with different resource
// requests — the "plan change resizes pods on next reconcile" contract. The
// reconciler calls cnpgClusterSpec on every reconcile loop (CreateOrUpdate
// always mutates cluster.Object["spec"]), so a plan diff in spec flows through
// in one reconcile without any special handling.
func TestDatabasePlanChangeProducesNewResources(t *testing.T) {
	freePlan, freeGB := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	bigPlan, bigGB := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"})

	freeSpec := cnpgClusterSpec(clusterParams{plan: freePlan, storageGB: freeGB, dbname: "d", owner: "d_user"})
	bigSpec := cnpgClusterSpec(clusterParams{plan: bigPlan, storageGB: bigGB, dbname: "d", owner: "d_user"})

	freeReq := freeSpec["resources"].(map[string]any)["requests"].(map[string]any)
	bigReq := bigSpec["resources"].(map[string]any)["requests"].(map[string]any)

	if freeReq["memory"] == bigReq["memory"] {
		t.Errorf("free and basic-1gb plans must produce different memory requests; both = %v", freeReq["memory"])
	}
	if bigReq["memory"] != "1Gi" {
		t.Errorf("basic-1gb memory request = %v, want 1Gi", bigReq["memory"])
	}
	if freeReq["memory"] != "256Mi" {
		t.Errorf("free memory request = %v, want 256Mi", freeReq["memory"])
	}
}

func TestCnpgClusterSpec(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "my_db", owner: "my_db_user"})

	if spec["instances"] != int64(1) {
		t.Errorf("free instances = %v, want 1", spec["instances"])
	}
	storage := spec["storage"].(map[string]any)
	if storage["size"] != "1Gi" || storage["storageClass"] != dbStorageClass {
		t.Errorf("storage = %v", storage)
	}
	req := spec["resources"].(map[string]any)["requests"].(map[string]any)
	lim := spec["resources"].(map[string]any)["limits"].(map[string]any)
	if req["cpu"] != "100m" || req["memory"] != "256Mi" || lim["memory"] != "256Mi" {
		t.Errorf("resources requests=%v limits=%v (want Guaranteed 100m/256Mi)", req, lim)
	}
	initdb := spec["bootstrap"].(map[string]any)["initdb"].(map[string]any)
	if initdb["database"] != "my_db" || initdb["owner"] != "my_db_user" {
		t.Errorf("initdb = %v, want database=my_db owner=my_db_user", initdb)
	}
	if _, hasImage := spec["imageName"]; hasImage {
		t.Errorf("no version => no imageName override, got %v", spec["imageName"])
	}

	// version pins the image
	withVer := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, version: "16", dbname: "d", owner: "d_user"})
	if withVer["imageName"] != "ghcr.io/cloudnative-pg/postgresql:16" {
		t.Errorf("version image = %v", withVer["imageName"])
	}
}

func TestCnpgClusterSpecBackup(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"})

	// Backups off => no plugin or legacy backup block.
	off := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user"})
	if _, has := off["backup"]; has {
		t.Error("no store => no legacy backup block")
	}
	if _, has := off["plugins"]; has {
		t.Error("no store => no backup plugin")
	}

	// Backups on => the Barman Cloud plugin is the sole WAL archiver. Transport,
	// credentials, and retention stay on the GitOps-managed ObjectStore.
	on := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", store: &testStore, backupServerName: "d-pg16"})
	if _, has := on["backup"]; has {
		t.Fatalf("plugin-backed cluster must not project spec.backup: %v", on["backup"])
	}
	plugins := on["plugins"].([]any)
	if len(plugins) != 1 {
		t.Fatalf("plugins = %v, want exactly one", plugins)
	}
	plugin := plugins[0].(map[string]any)
	if plugin["name"] != barmanCloudPluginName || plugin["isWALArchiver"] != true {
		t.Errorf("backup plugin = %v", plugin)
	}
	parameters := plugin["parameters"].(map[string]any)
	if parameters["barmanObjectName"] != tenantBackupObjectStoreName || parameters["serverName"] != "d-pg16" {
		t.Errorf("backup plugin parameters = %v", parameters)
	}
	// initdb bootstrap when not recovering.
	if _, has := on["bootstrap"].(map[string]any)["initdb"]; !has {
		t.Error("non-recovery cluster should bootstrap via initdb")
	}

	// A configured store must not change a plan whose durability bit is off.
	free, freeGB := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	freeWithoutStore := cnpgClusterSpec(clusterParams{plan: free, storageGB: freeGB, dbname: "d", owner: "d_user"})
	freeWithStore := cnpgClusterSpec(clusterParams{plan: free, storageGB: freeGB, dbname: "d", owner: "d_user", store: &testStore})
	if !reflect.DeepEqual(freeWithoutStore, freeWithStore) {
		t.Fatalf("backup-disabled plan changed when store configured:\nwithout=%v\nwith=%v", freeWithoutStore, freeWithStore)
	}
}

func TestCnpgClusterSpecRecovery(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	rec := &appv1alpha1.DatabaseRecovery{SourceDatabase: "src", SourceBackupServerName: "src-pg16", TargetTime: "2026-07-09T10:00:00Z"}
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", store: &testStore, recovery: rec})

	boot := spec["bootstrap"].(map[string]any)
	if _, has := boot["initdb"]; has {
		t.Error("recovery cluster must not initdb")
	}
	recovery := boot["recovery"].(map[string]any)
	if recovery["source"] != recoverySource {
		t.Errorf("recovery.source = %v", recovery["source"])
	}
	if recovery["recoveryTarget"].(map[string]any)["targetTime"] != rec.TargetTime {
		t.Errorf("recoveryTarget = %v", recovery["recoveryTarget"])
	}
	// externalClusters reads the source's exact archive generation through the
	// same shared namespaced ObjectStore.
	ext := spec["externalClusters"].([]any)[0].(map[string]any)
	if ext["name"] != recoverySource {
		t.Errorf("externalCluster name = %v", ext["name"])
	}
	plugin := ext["plugin"].(map[string]any)
	if plugin["name"] != barmanCloudPluginName {
		t.Errorf("recovery plugin = %v", plugin)
	}
	parameters := plugin["parameters"].(map[string]any)
	if parameters["barmanObjectName"] != tenantBackupObjectStoreName || parameters["serverName"] != "src-pg16" {
		t.Errorf("recovery plugin parameters = %v", parameters)
	}
	if _, has := plugin["isWALArchiver"]; has {
		t.Errorf("external recovery plugin must not be marked WAL archiver: %v", plugin)
	}
	withoutGeneration := barmanCloudPlugin("", false)["parameters"].(map[string]any)
	if _, has := withoutGeneration["serverName"]; has {
		t.Errorf("empty legacy recovery serverName must use the plugin default: %v", withoutGeneration)
	}
}

func TestCnpgClusterSpecManagedRoles(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	users := []appv1alpha1.DatabaseUser{{Name: "reporting", SecretName: "d-user-reporting"}}
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", users: users})

	roles := spec["managed"].(map[string]any)["roles"].([]any)
	role := roles[0].(map[string]any)
	if role["name"] != "reporting" || role["ensure"] != "present" || role["login"] != true {
		t.Errorf("managed role = %v", role)
	}
	if role["passwordSecret"].(map[string]any)["name"] != "d-user-reporting" {
		t.Errorf("passwordSecret = %v", role["passwordSecret"])
	}
	// No users => no managed block.
	bare := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user"})
	if _, has := bare["managed"]; has {
		t.Error("no users => no managed block")
	}
}

func TestScheduledBackupSpec(t *testing.T) {
	sb := scheduledBackupSpec("mydb")
	if sb["schedule"] != backupSchedule || sb["method"] != "plugin" {
		t.Errorf("scheduledBackup = %v", sb)
	}
	if sb["cluster"].(map[string]any)["name"] != "mydb" {
		t.Errorf("scheduledBackup cluster = %v", sb["cluster"])
	}
	if sb["pluginConfiguration"].(map[string]any)["name"] != barmanCloudPluginName {
		t.Errorf("scheduledBackup plugin = %v", sb["pluginConfiguration"])
	}
	onDemand := onDemandBackupSpec("mydb")
	if onDemand["method"] != "plugin" || onDemand["pluginConfiguration"].(map[string]any)["name"] != barmanCloudPluginName {
		t.Errorf("on-demand backup = %v", onDemand)
	}
}

func TestDatabaseRecoveryRequiresCompleteBackupStore(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "restore-db", Namespace: "default", Finalizers: []string{dbFinalizer}},
		Spec: appv1alpha1.DatabaseSpec{
			Plan: "free",
			Recovery: &appv1alpha1.DatabaseRecovery{
				SourceDatabase:         "source-db",
				SourceBackupServerName: "source-db-pg16",
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	r := &DatabaseReconciler{
		Client: cl,
		Scheme: scheme,
		Backup: BackupStore{DestinationPath: testStore.DestinationPath, EndpointURL: testStore.EndpointURL},
	}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}
	if _, err := r.Reconcile(context.Background(), req); err == nil || !strings.Contains(err.Error(), "no backup store is configured") {
		t.Fatalf("incomplete backup store recovery error = %v", err)
	}
	if err := cl.Get(context.Background(), req.NamespacedName, db); err != nil {
		t.Fatal(err)
	}
	if db.Status.Phase != appv1alpha1.DBPhaseFailed || len(db.Status.Conditions) == 0 || db.Status.Conditions[0].Reason != "RecoveryUnavailable" {
		t.Fatalf("recovery failure status = phase %q conditions %v", db.Status.Phase, db.Status.Conditions)
	}
}

func TestVersionedBackupServerName(t *testing.T) {
	if got := versionedBackupServerName("orders", "17"); got != "orders-pg17" {
		t.Errorf("versioned serverName = %q", got)
	}
	if got := versionedBackupServerName("orders", ""); got != "orders" {
		t.Errorf("legacy serverName = %q", got)
	}
}

func TestPoolerSpec(t *testing.T) {
	p := poolerSpec("mydb")
	if p["cluster"].(map[string]any)["name"] != "mydb" || p["type"] != "rw" {
		t.Errorf("pooler = %v", p)
	}
	if p["pgbouncer"].(map[string]any)["poolMode"] != "transaction" {
		t.Errorf("poolMode = %v", p["pgbouncer"])
	}
}

func TestCnpgClusterSpecHA(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})

	// HA off => instances stays at plan default (1 for free).
	noHA := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", highAvailability: false})
	if noHA["instances"] != int64(1) {
		t.Errorf("HA off: instances = %v, want 1", noHA["instances"])
	}
	if _, has := noHA["affinity"]; has {
		t.Error("HA off: affinity must not be set")
	}

	// HA on => instances raised to 2 and anti-affinity set.
	ha := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", highAvailability: true})
	if ha["instances"] != int64(2) {
		t.Errorf("HA on: instances = %v, want 2", ha["instances"])
	}
	aff := ha["affinity"].(map[string]any)
	if aff["enablePodAntiAffinity"] != true || aff["topologyKey"] != "kubernetes.io/hostname" {
		t.Errorf("HA on: affinity = %v, want enablePodAntiAffinity=true topologyKey=kubernetes.io/hostname", aff)
	}

	// HA on a plan that already has >1 instances keeps the higher count.
	bigPlan, bigGB := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"})
	bigHA := cnpgClusterSpec(clusterParams{plan: bigPlan, storageGB: bigGB, dbname: "d", owner: "d_user", highAvailability: true})
	if bigHA["instances"].(int64) < 2 {
		t.Errorf("HA on basic-1gb: instances = %v, want >= 2", bigHA["instances"])
	}
}

func TestSetLifecycleAnnotations(t *testing.T) {
	// Suspend + restart set both annotations.
	c := &unstructured.Unstructured{}
	setLifecycleAnnotations(c, true, "2026-07-09T10:00:00Z")
	anns := c.GetAnnotations()
	if anns[hibernationAnnotation] != "on" || anns[restartAnnotation] != "2026-07-09T10:00:00Z" {
		t.Errorf("annotations = %v", anns)
	}
	// Resume removes the hibernation annotation, keeps a prior restart stamp.
	setLifecycleAnnotations(c, false, "2026-07-09T10:00:00Z")
	anns = c.GetAnnotations()
	if _, has := anns[hibernationAnnotation]; has {
		t.Error("resume must remove the hibernation annotation")
	}
}

// TestCNPGWorkspaceLabelPropagated verifies that the workspace label is carried
// into CNPG pod templates via inheritedMetadata so same-workspace NetworkPolicy
// selectors reach the database. This is the invariant the w7/m33 policy split
// depends on: CNPG pods must keep the workspace label (for intra-workspace
// allow) but be excluded from the node/metadata egress deny by cnpg.io/cluster
// (which is set by CNPG itself on every managed pod, not by the operator).
func TestCNPGWorkspaceLabelPropagated(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, gvk := range []schema.GroupVersionKind{
		cnpgClusterGVK, cnpgScheduledBackupGVK, cnpgBackupGVK, cnpgPoolerGVK,
	} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}

	const ws = "tea-testworkspace0001"
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dpg-wstest", Namespace: "default",
			Labels: map[string]string{labelWorkspace: ws},
		},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(db).WithStatusSubresource(&appv1alpha1.Database{}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}

	ctx := context.Background()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}
	// First reconcile stamps the finalizer and requeues.
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	// Second reconcile reaches the CNPG Cluster CreateOrUpdate.
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(ctx, req.NamespacedName, cluster); err != nil {
		t.Fatalf("CNPG Cluster not found: %v", err)
	}
	meta, _, _ := unstructured.NestedMap(cluster.Object, "spec", "inheritedMetadata", "labels")
	if got := meta[labelWorkspace]; got != ws {
		t.Errorf("inheritedMetadata.labels[%q] = %q, want %q — workspace label must be propagated to CNPG pods so same-workspace NetworkPolicy selectors work", labelWorkspace, got, ws)
	}
	if got := meta["app.bex.co/component"]; got != "database" {
		t.Errorf("inheritedMetadata.labels[app.bex.co/component] = %q, want database — the log shipper must distinguish tenant databases from platform CNPG clusters", got)
	}
}

// TestLegacyDatabaseGainsPgStatStatements proves a pre-insights CNPG Cluster
// converges through an ordinary Database reconcile. CloudNativePG treats a
// pg_stat_statements.* parameter as the managed-extension switch: it adds the
// preload library, rolls PostgreSQL when required, and runs CREATE EXTENSION
// IF NOT EXISTS in every connectable database. A second reconcile must retain
// the same spec so the backfill cannot cause restart churn.
func TestLegacyDatabaseGainsPgStatStatements(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	for _, gvk := range []schema.GroupVersionKind{
		cnpgClusterGVK, cnpgScheduledBackupGVK, cnpgBackupGVK, cnpgPoolerGVK,
	} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}

	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-insights", Namespace: "default", Finalizers: []string{dbFinalizer}},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free"},
	}
	legacy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name": "legacy-insights", "namespace": "default",
		},
		"spec": map[string]any{
			"instances": int64(1),
			"postgresql": map[string]any{
				"parameters": map[string]any{"work_mem": "4MB"},
			},
		},
	}}
	legacy.SetGroupVersionKind(cnpgClusterGVK)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, legacy).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	params, _, _ := unstructured.NestedStringMap(got.Object, "spec", "postgresql", "parameters")
	preload, _, _ := unstructured.NestedStringSlice(got.Object, "spec", "postgresql", "shared_preload_libraries")
	if params["pg_stat_statements.track"] != "all" || !reflect.DeepEqual(preload, []string{"pg_stat_statements"}) {
		t.Fatalf("legacy cluster did not gain managed pg_stat_statements config: params=%v preload=%v", params, preload)
	}
	firstSpec, _, _ := unstructured.NestedMap(got.Object, "spec")

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	again := &unstructured.Unstructured{}
	again.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(context.Background(), req.NamespacedName, again); err != nil {
		t.Fatal(err)
	}
	secondSpec, _, _ := unstructured.NestedMap(again.Object, "spec")
	if !reflect.DeepEqual(firstSpec, secondSpec) {
		t.Fatalf("pg_stat_statements backfill is not idempotent:\nfirst=%v\nsecond=%v", firstSpec, secondSpec)
	}
}

func TestDatabasePublicFrontDoorStatus(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Public: true, Pooler: true,
			ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "analytics"}},
		},
		Status: appv1alpha1.DatabaseStatus{
			ReadReplicaStatuses: []appv1alpha1.DatabaseReadReplicaStatus{{Name: "retired"}},
		},
	}
	r := &DatabaseReconciler{DBDomain: "db.example.test"}
	r.updateExternalAddressStatus(db)
	if db.Status.ExternalHost != "orders.db.example.test" || db.Status.PoolerExternalHost != "orders-pool.db.example.test" {
		t.Fatalf("public status hosts = %q / %q", db.Status.ExternalHost, db.Status.PoolerExternalHost)
	}
	if got := db.Status.ReadReplicaStatuses; len(got) != 1 || got[0].ExternalHost != "orders-ro-analytics.db.example.test" {
		t.Fatalf("read replica status = %#v", got)
	}
}
