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
// credential, route, backup path, database/user identifier, and connection host
// remains derived from immutable metadata.name. Reconcile therefore updates no
// data-plane identity when only spec.name changes.
func TestDatabaseDisplayNameDoesNotAffectDataPlaneIdentity(t *testing.T) {
	before := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-c185th5c2rvvnhbfiltg", Namespace: "tenant-a"},
		Spec: appv1alpha1.DatabaseSpec{
			Name:             "orders-old",
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
		dbname := normalizeIdent(db.Name)
		return map[string]any{
			"cluster":         db.Name,
			"clusterSpec":     cnpgClusterSpec(clusterParams{plan: plan, storageGB: storageGB, dbname: dbname, owner: dbname + "_user", highAvailability: db.Spec.HighAvailability}),
			"database":        dbname,
			"owner":           dbname + "_user",
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

func TestNormalizeIdent(t *testing.T) {
	cases := map[string]string{
		"bex-mvp-smoketest": "bex_mvp_smoketest", // hyphens -> underscores (valid unquoted identifier)
		"MyDB":              "mydb",              // lowercased
		"plain":             "plain",
	}
	for in, want := range cases {
		if got := normalizeIdent(in); got != want {
			t.Errorf("normalizeIdent(%q) = %q, want %q", in, got, want)
		}
	}
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

func TestIngressRouteTCPSpec(t *testing.T) {
	spec := ingressRouteTCPSpec(pgEntryPoint, "smoke-db.db.bex.co", "smoke-db-rw", 5432, nil)

	if ep := spec["entryPoints"].([]any); len(ep) != 1 || ep[0] != pgEntryPoint {
		t.Errorf("entryPoints = %v, want [%s]", ep, pgEntryPoint)
	}
	// TLS passthrough: Postgres terminates its own TLS.
	if pt := spec["tls"].(map[string]any)["passthrough"]; pt != true {
		t.Errorf("tls.passthrough = %v, want true", pt)
	}
	route := spec["routes"].([]any)[0].(map[string]any)
	if route["match"] != "HostSNI(`smoke-db.db.bex.co`)" {
		t.Errorf("match = %v", route["match"])
	}
	svc := route["services"].([]any)[0].(map[string]any)
	if svc["name"] != "smoke-db-rw" || svc["port"] != int64(5432) {
		t.Errorf("service = %v, want smoke-db-rw:5432", svc)
	}
}

func TestCnpgClusterSpecBackup(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"})

	// Backups off => no backup block.
	off := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user"})
	if _, has := off["backup"]; has {
		t.Error("no store => no backup block")
	}

	// Backups on => barmanObjectStore + retention, credentials from the Secret.
	on := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", store: &testStore, backupServerName: "d-pg16"})
	backup := on["backup"].(map[string]any)
	if backup["retentionPolicy"] != backupRetention {
		t.Errorf("retentionPolicy = %v", backup["retentionPolicy"])
	}
	bos := backup["barmanObjectStore"].(map[string]any)
	if bos["destinationPath"] != testStore.DestinationPath || bos["endpointURL"] != testStore.EndpointURL {
		t.Errorf("barmanObjectStore target = %v", bos)
	}
	creds := bos["s3Credentials"].(map[string]any)["accessKeyId"].(map[string]any)
	if creds["name"] != testStore.S3Secret || creds["key"] != "AWS_ACCESS_KEY_ID" {
		t.Errorf("s3 credentials ref = %v", creds)
	}
	// Major versions use distinct archive namespaces because pg_upgrade resets
	// the PostgreSQL system ID/timeline.
	if bos["serverName"] != "d-pg16" {
		t.Errorf("own backup serverName = %v, want d-pg16", bos["serverName"])
	}
	// initdb bootstrap when not recovering.
	if _, has := on["bootstrap"].(map[string]any)["initdb"]; !has {
		t.Error("non-recovery cluster should bootstrap via initdb")
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
	// externalClusters reads the SOURCE's serverName from the shared store.
	ext := spec["externalClusters"].([]any)[0].(map[string]any)
	if ext["name"] != recoverySource {
		t.Errorf("externalCluster name = %v", ext["name"])
	}
	if ext["barmanObjectStore"].(map[string]any)["serverName"] != "src-pg16" {
		t.Errorf("externalCluster serverName should be the source db, got %v", ext["barmanObjectStore"])
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
	if sb["schedule"] != backupSchedule || sb["method"] != "barmanObjectStore" {
		t.Errorf("scheduledBackup = %v", sb)
	}
	if sb["cluster"].(map[string]any)["name"] != "mydb" {
		t.Errorf("scheduledBackup cluster = %v", sb["cluster"])
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

func TestIPAllowListMiddlewareSpec(t *testing.T) {
	mw := ipAllowListMiddlewareSpec([]appv1alpha1.IPAllowEntry{
		{CIDR: "203.0.113.0/24", Description: "office"},
		{CIDR: "10.0.0.0/8"},
	})
	ranges := mw["ipAllowList"].(map[string]any)["sourceRange"].([]any)
	if len(ranges) != 2 || ranges[0] != "203.0.113.0/24" {
		t.Errorf("sourceRange = %v", ranges)
	}
	// Descriptions are operator-facing metadata: the rendered middleware must be
	// byte-identical to a description-free list (enforcement reads CIDRs only).
	bare := ipAllowListMiddlewareSpec([]appv1alpha1.IPAllowEntry{
		{CIDR: "203.0.113.0/24"},
		{CIDR: "10.0.0.0/8"},
	})
	if !reflect.DeepEqual(mw, bare) {
		t.Errorf("description changed the rendered middleware: %v != %v", mw, bare)
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
		traefikIngressRouteTCPGVK, traefikMiddlewareTCPGVK,
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
