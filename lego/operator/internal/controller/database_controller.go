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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cnpgClusterGVK is the CloudNativePG Cluster type. We project onto it via
// unstructured so the operator needn't vendor CNPG's Go API (and stays decoupled
// from its version).
var cnpgClusterGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster"}

// cnpgScheduledBackupGVK / cnpgPoolerGVK are the CloudNativePG resources the
// Database projects for durability (a daily base backup) and connection pooling
// (a PgBouncer instance). Projected via unstructured like the Cluster.
var cnpgScheduledBackupGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "ScheduledBackup"}
var cnpgBackupGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Backup"}
var cnpgPoolerGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Pooler"}

// barmanCloudObjectStoreGVK is the Barman Cloud plugin's ObjectStore — a NAMESPACED
// object the Cluster references by name. Projected via unstructured like the
// CNPG resources above, so the operator needn't vendor the plugin's Go API.
// (Named barmanCloud* rather than barman* to match barmanCloudPlugin above AND
// to stay clear of the retired in-tree CNPG field name that
// scripts/platform-deprecations-validate.sh greps for — this is the plugin's
// ObjectStore CRD, an entirely different thing that merely reads alike.)
var barmanCloudObjectStoreGVK = schema.GroupVersionKind{Group: "barmancloud.cnpg.io", Version: "v1", Kind: "ObjectStore"}

const (
	dbStorageClass = "hcloud-volumes"
	// pgPort is the Postgres listen + Service port.
	pgPort = 5432
	// hibernationAnnotation / restartAnnotation are the CNPG cluster annotations
	// the lifecycle verbs map onto: declarative hibernation (suspend/resume) and
	// a manual rolling restart (verb-as-timestamp).
	hibernationAnnotation = "cnpg.io/hibernation"
	restartAnnotation     = "kubectl.kubernetes.io/restartedAt"
	// recoverySource is the externalClusters entry name a restore-to-new Database
	// references from its bootstrap.recovery.
	recoverySource = "source"
	// backupSchedule is the daily base-backup cron (CNPG's 6-field form:
	// sec min hour dom mon dow) — 03:00 UTC.
	backupSchedule = "0 0 3 * * *"
	// barmanCloudPluginName is CNPG's stable plugin identifier. The namespaced
	// ObjectStore is installed by GitOps before the operator starts projecting
	// plugin-backed tenant databases.
	barmanCloudPluginName       = "barman-cloud.cloudnative-pg.io"
	tenantBackupObjectStoreName = "bex-tenant-postgres"
	// logicalExportRetention matches Render's documented logical-backup window.
	logicalExportRetention = 7 * 24 * time.Hour
	// logicalExportClientVersion is used when Database.spec.version is empty.
	// A newer pg_dump supports every server version bex currently accepts.
	logicalExportClientVersion = "18"
	logicalExportLabel         = "app.bex.co/export"
	logicalExportDBLabel       = "app.bex.co/database"
	logicalExportPollInterval  = 5 * time.Second
	// dbFinalizer is the finalizer stamped on every Database so the controller can
	// purge barman object-store backups after the CNPG Cluster is gone (w7/m12).
	// Decision: delete S3 backups on Database deletion (30d retention via CNPG's own
	// policy would not run once the Cluster is gone; explicit purge avoids unbounded
	// storage accumulation and prevents inadvertent restore from a deleted tenant's data).
	dbFinalizer                = "app.bex.co/db-finalizer"
	annotDBBackupPurgeComplete = "app.bex.co/backup-purge-complete"
)

// BackupStore is the non-secret object-store contract shared by the GitOps
// Barman Cloud ObjectStore, logical exports, and finalizer purge jobs. The
// physical Cluster projection references the namespaced ObjectStore by name;
// exports and purge jobs still need these coordinates directly. All three
// fields must be set for the controller to enable either path.
type BackupStore struct {
	// DestinationPath is the S3 URL prefix, e.g. "s3://bex-tfstate/postgres".
	// CNPG namespaces each cluster under it by serverName.
	DestinationPath string
	// EndpointURL is the S3-compatible endpoint, e.g.
	// "https://s3.eu-central-2.wasabisys.com".
	EndpointURL string
	// S3Secret is a Secret (Database's namespace) with AWS_ACCESS_KEY_ID and
	// AWS_SECRET_ACCESS_KEY keys.
	S3Secret string
	// AgePublicKey, when non-empty, enables ADR050 Tier A client-side age
	// encryption of the snapshot before upload. It is the recipient public key
	// (safe to inline — the private half stays out-of-band in .env/CI); empty ⇒
	// plain upload, byte-identical to pre-ADR050. Only the KeyValue backup path
	// consumes it today; the Barman-backed Postgres archives use provider SSE
	// (ADR050 Tier B) instead, so this field does not gate configured().
	AgePublicKey string
}

// configured reports whether every field needed to project backups is present.
func (b BackupStore) configured() bool {
	return b.DestinationPath != "" && b.EndpointURL != "" && b.S3Secret != ""
}

// resolvePlan returns the plan (defaulting per the shared catalog — free)
// and the effective storage size in GB (never below the plan floor — storage
// only grows). The ladder is lego/types/tiers' postgres family, the Database
// sibling of the compute ladder resourcesForTier reads.
func resolvePlan(spec appv1alpha1.DatabaseSpec) (tiers.PostgresTier, int32) {
	plan, ok := tiers.Postgres.ByID(spec.Plan)
	if !ok {
		plan = tiers.Postgres.Default()
	}
	return plan, growOnlyStorage(spec.StorageGB, plan.StorageGB)
}

func cnpgStorageGB(cluster *unstructured.Unstructured) int32 {
	size, found, err := unstructured.NestedString(cluster.Object, "spec", "storage", "size")
	if err != nil || !found {
		return 0
	}
	return storageQuantityGB(size)
}

func (r *DatabaseReconciler) databaseStorageIntent(
	ctx context.Context,
	db *appv1alpha1.Database,
	cluster *unstructured.Unstructured,
	desiredGB int32,
) (int32, ctrl.Result, bool, error) {
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil && !apierrors.IsNotFound(err) {
		result, failErr := r.dbFail(ctx, db, "ClusterReadFailed", err)
		return 0, result, true, failErr
	}
	currentGB := max(db.Status.AllocatedStorageGB, cnpgStorageGB(cluster))
	intentChanged := db.Status.AllocatedStorageGB > 0 && db.Status.ObservedStorageGB != db.Spec.StorageGB
	if intentChanged && db.Spec.StorageGB > 0 && currentGB > db.Spec.StorageGB {
		return 0, ctrl.Result{}, true, r.rejectDatabaseStorageShrink(ctx, db, currentGB, db.Spec.StorageGB)
	}
	return max(desiredGB, currentGB), ctrl.Result{}, false, nil
}

// clusterParams bundles the inputs the CNPG Cluster projection needs beyond the
// plan — kept as a struct so the growing set (backup, recovery, users, HA) stays
// readable and the projection remains a pure, unit-testable function.
type clusterParams struct {
	plan      tiers.PostgresTier
	storageGB int32
	version   string
	dbname    string
	owner     string
	// store is the object-store contract when the controller has one (nil => not
	// configured). It gates both this cluster's own plugin and a recovery read.
	// A cluster loads the WAL-archiver plugin when the plan opts in (plan.Backup)
	// and store is set — derived, not passed.
	store *BackupStore
	// backupServerName isolates physical archives by PostgreSQL major. CNPG
	// resets the system ID/timeline during pg_upgrade, so reusing one archive
	// prefix across majors can corrupt PITR history.
	backupServerName string
	// recovery, when non-nil (with store set), bootstraps the cluster by restoring
	// a source Database's backups instead of a fresh initdb.
	recovery *appv1alpha1.DatabaseRecovery
	// users are additional managed login roles (spec.managed.roles).
	users []appv1alpha1.DatabaseUser
	// deletedUsers are removed roles projected as ensure:absent so CNPG drops
	// them from PostgreSQL (codex #8).
	deletedUsers []string
	// highAvailability, when true, provisions a replicated cluster (≥2 instances,
	// primary + standby) with pod anti-affinity. Render's enableHighAvailability.
	highAvailability bool
	// parameters are additional postgresql.conf key-value overrides from
	// Database.spec.parameters. Merged on top of the built-in defaults;
	// shared_preload_libraries is always forced to include pg_stat_statements.
	parameters map[string]string
}

func versionedBackupServerName(name, version string) string {
	if version == "" {
		return name
	}
	return fmt.Sprintf("%s-pg%s", name, version)
}

func databaseBackupServerNames(db *appv1alpha1.Database) (current, target string) {
	current = db.Status.BackupServerName
	if current == "" {
		current = db.Name
	}
	target = current
	if db.Spec.Version != "" && db.Status.CurrentVersion != "" && db.Spec.Version != db.Status.CurrentVersion {
		target = versionedBackupServerName(db.Name, db.Spec.Version)
	}
	return current, target
}

// barmanCloudPlugin builds the CNPG-I reference used for both a Cluster's WAL
// archiver and a recovery externalCluster. serverName keeps each database and
// PostgreSQL-major generation in its existing isolated archive prefix.
func barmanCloudPlugin(serverName string, walArchiver bool) map[string]any {
	parameters := map[string]any{"barmanObjectName": tenantBackupObjectStoreName}
	if serverName != "" {
		parameters["serverName"] = serverName
	}
	plugin := map[string]any{
		"name":       barmanCloudPluginName,
		"parameters": parameters,
	}
	if walArchiver {
		plugin["isWALArchiver"] = true
	}
	return plugin
}

// canonicalNamespace reports whether obj lives in a namespace the reconciler is
// allowed to act on: the shared/bootstrap apps namespace ("default") or its own
// per-workspace namespace (namespace == workspace label). Any other namespace is
// a confused-deputy / cross-tenant write (codex #11). Shared by the Database and
// KeyValue reconcilers.
func canonicalNamespace(meta *metav1.ObjectMeta) bool {
	if meta.Namespace == "default" {
		return true
	}
	ws := meta.Labels[labelWorkspace]
	return ws != "" && meta.Namespace == ws
}

// managedRoles projects additional Database users onto CNPG spec.managed.roles —
// login roles ensured present, each with its password read from the referenced
// Secret (created by bex-api). DeletedUsers are projected as ensure:absent so CNPG
// drops the live PostgreSQL role (codex #8). Returns nil for no roles (so
// spec.managed is omitted).
func managedRoles(users []appv1alpha1.DatabaseUser, deletedUsers []string) []any {
	if len(users) == 0 && len(deletedUsers) == 0 {
		return nil
	}
	roles := make([]any, 0, len(users)+len(deletedUsers))
	for _, u := range users {
		role := map[string]any{"name": u.Name, "ensure": "present", "login": true}
		if u.SecretName != "" {
			role["passwordSecret"] = map[string]any{"name": u.SecretName}
		}
		roles = append(roles, role)
	}
	// ensure:absent tombstones so CNPG drops the role from PostgreSQL (codex #8).
	for _, name := range deletedUsers {
		roles = append(roles, map[string]any{"name": name, "ensure": "absent"})
	}
	return roles
}

// cnpgClusterSpec builds the CloudNativePG Cluster .spec for a Database. Pure
// (no client) so the plan->Cluster projection is unit-testable. When p.recovery
// is set (with a backup store), the cluster bootstraps through the Barman Cloud
// plugin from a source Database's backups — a NEW instance, never in place.
// When p.highAvailability is true, instances is raised to at least 2 and pod
// anti-affinity is set to spread the primary and standby across nodes.
func cnpgClusterSpec(p clusterParams) map[string]any {
	instances := int64(p.plan.Instances)
	if p.highAvailability && instances < 2 {
		instances = 2
	}
	spec := map[string]any{
		"instances": instances,
		"storage": map[string]any{
			"size":         fmt.Sprintf("%dGi", p.storageGB),
			"storageClass": dbStorageClass,
		},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": p.plan.CPU, "memory": p.plan.Memory},
			"limits":   map[string]any{"cpu": p.plan.CPU, "memory": p.plan.Memory},
		},
	}
	// Pod anti-affinity: spread primary and standbys across nodes so a single
	// node failure doesn't take out all instances.
	if p.highAvailability {
		spec["affinity"] = map[string]any{
			"enablePodAntiAffinity": true,
			"topologyKey":           "kubernetes.io/hostname",
		}
	}
	if p.version != "" {
		spec["imageName"] = "ghcr.io/cloudnative-pg/postgresql:" + p.version
	}
	// Bootstrap: restore-to-new (PITR) when recovery is requested and a store is
	// available; otherwise a fresh, empty database.
	if p.recovery != nil && p.store != nil {
		rec := map[string]any{"source": recoverySource}
		if p.recovery.TargetTime != "" {
			rec["recoveryTarget"] = map[string]any{"targetTime": p.recovery.TargetTime}
		}
		spec["bootstrap"] = map[string]any{"recovery": rec}
		spec["externalClusters"] = []any{map[string]any{
			"name":   recoverySource,
			"plugin": barmanCloudPlugin(p.recovery.SourceBackupServerName, false),
		}}
	} else {
		spec["bootstrap"] = map[string]any{
			"initdb": map[string]any{
				"database": p.dbname,
				"owner":    p.owner,
				// Create pg_stat_statements so top-queries introspection works
				// without a manual CREATE EXTENSION after provisioning.
				"postInitSQL": []any{"CREATE EXTENSION IF NOT EXISTS pg_stat_statements"},
			},
		}
	}
	// Always load pg_stat_statements so the insights endpoints have it available.
	// Merge user-supplied parameters on top; shared_preload_libraries is forced
	// to include pg_stat_statements regardless of any user value. It rides CNPG's
	// dedicated spec.postgresql.shared_preload_libraries field — inside
	// `parameters` it is a "fixed configuration parameter" the admission webhook
	// rejects outright.
	pgParams := map[string]any{
		"pg_stat_statements.track": "all",
	}
	for k, v := range p.parameters {
		if k != "shared_preload_libraries" {
			pgParams[k] = v
		}
	}
	spec["postgresql"] = map[string]any{
		"parameters":               pgParams,
		"shared_preload_libraries": []any{"pg_stat_statements"},
	}
	// Durability: load the Barman Cloud plugin as the sole WAL archiver when the
	// plan opts in and the object-store contract is configured. Retention and S3
	// transport live on the namespaced ObjectStore; ScheduledBackup pins daily
	// base backups below.
	if p.plan.Backup && p.store != nil {
		spec["plugins"] = []any{barmanCloudPlugin(p.backupServerName, true)}
	}
	if roles := managedRoles(p.users, p.deletedUsers); roles != nil {
		spec["managed"] = map[string]any{"roles": roles}
	}
	return spec
}

// scheduledBackupSpec builds a CNPG ScheduledBackup .spec: a daily base backup
// of the cluster to object storage (WAL archiving from the Cluster's plugin is
// what actually enables PITR; this pins recovery windows down).
func scheduledBackupSpec(clusterName string) map[string]any {
	return map[string]any{
		"schedule":             backupSchedule,
		"backupOwnerReference": "self",
		"immediate":            true,
		"cluster":              map[string]any{"name": clusterName},
		"method":               "plugin",
		"pluginConfiguration":  map[string]any{"name": barmanCloudPluginName},
	}
}

func onDemandBackupSpec(clusterName string) map[string]any {
	return map[string]any{
		"cluster":             map[string]any{"name": clusterName},
		"method":              "plugin",
		"pluginConfiguration": map[string]any{"name": barmanCloudPluginName},
	}
}

// poolerSpec builds a CNPG Pooler .spec: a single-instance PgBouncer in
// transaction mode fronting the cluster's read-write endpoint. CNPG creates a
// Service named after the Pooler ("<db>-pooler") that the pooled URLs target.
func poolerSpec(clusterName string) map[string]any {
	return map[string]any{
		"cluster":   map[string]any{"name": clusterName},
		"instances": int64(1),
		"type":      "rw",
		"pgbouncer": map[string]any{"poolMode": "transaction"},
	}
}

// cidrMiddlewareSpec is the flat-CIDR variant shared by the environment layer
// (spec.environmentIPAllowList, w4/m28) on App HTTP ingress.
func cidrMiddlewareSpec(cidrs []string) map[string]any {
	ranges := make([]any, len(cidrs))
	for i, c := range cidrs {
		ranges[i] = c
	}
	return map[string]any{"ipAllowList": map[string]any{"sourceRange": ranges}}
}

// deleteOptionalObject best-effort deletes an optional owned object (a Traefik
// route/middleware, a CNPG ScheduledBackup/Pooler), treating NotFound and
// NoMatch (the CRD not installed — e.g. envtest or a cluster without that
// operator) as "nothing to delete". Shared by the Database and KeyValue cleanup.
func deleteOptionalObject(ctx context.Context, c client.Client, o *unstructured.Unstructured) error {
	if err := c.Delete(ctx, o); err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		return err
	}
	return nil
}

// DatabaseReconciler projects a Database into a CloudNativePG Cluster and
// surfaces the connection info. Optionally exposes an external SNI endpoint via
// Traefik, off-cluster backups (Barman Cloud plugin + ScheduledBackup), a
// PgBouncer Pooler, and an IP-allowlist middleware. It is a thin executor —
// CNPG does the actual Postgres lifecycle.
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Recorder emits a Kubernetes Event for every automatic storage increase.
	// Nil is allowed in tests and disables Event emission only.
	Recorder record.EventRecorder
	// DiskUsageReader reads the fullest CNPG PVC's kubelet usage sample. Nil
	// leaves the field round-trippable but disables the automatic control loop.
	DiskUsageReader DatabaseDiskUsageReader
	// DBDomain is the wildcard base for external endpoints, e.g. "db.bex.co".
	// Empty => Public databases get no external route.
	DBDomain string
	// Backup is the object-store target for backups + PITR. Unset (any field
	// empty) => backups are disabled for every plan (recovery unavailable).
	Backup BackupStore
	// ExportRetention overrides the seven-day logical-export retention in tests.
	// Zero uses logicalExportRetention.
	ExportRetention time.Duration
	// Now is a test clock. Nil uses time.Now.
	Now func() time.Time
	// BackupSourceNamespace holds the GitOps-installed ObjectStore + S3
	// credential that tenant namespaces are projected from (ADR043 D8.4). It is
	// the shared apps namespace (BEX_APPS_NAMESPACE). Empty disables projection,
	// which is correct for a single-namespace deployment: the Database is
	// already beside the originals.
	BackupSourceNamespace string
	// SecretClient reads and writes this Database's derived Secrets. It must be
	// an UNCACHED client — see KeyValueReconciler.SecretClient for why the
	// manager's Secret informer cannot simply be widened. Nil falls back to the
	// cached client.
	SecretClient client.Client
}

// secretClient returns the uncached client for tenant-namespace Secret access —
// the same escape hatch the App path uses (AppReconciler.buildPlaneClient).
func (r *DatabaseReconciler) secretClient() client.Client {
	if r.SecretClient != nil {
		return r.SecretClient
	}
	return r.Client
}

// +kubebuilder:rbac:groups=app.bex.co,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters;backups;scheduledbackups;poolers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters/status,verbs=patch
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutetcps;middlewaretcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch
// Secrets access is namespace-scoped to the apps namespace via deploy/gitops/base/operator-apps-rbac.yaml.

// upsertOwned creates-or-updates an owned unstructured object (gvk/name in the
// owner's namespace), applying spec and stamping an owner reference so the
// object is garbage-collected with its owner. Shared by the Database and
// KeyValue reconcilers.
func upsertOwned(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, gvk schema.GroupVersionKind, name string, spec map[string]any) error {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(gvk)
	o.SetName(name)
	o.SetNamespace(owner.GetNamespace())
	_, err := controllerutil.CreateOrUpdate(ctx, c, o, func() error {
		o.Object["spec"] = spec
		return controllerutil.SetControllerReference(owner, o, scheme)
	})
	return err
}

// stampFinalizer adds the finalizer so deletion goes through the reconciler's
// teardown path, reporting whether the caller must return now. The finalizer
// update doesn't bump generation, so the requeue re-runs this reconcile against
// the stamped object. Shared by the App, Database, and KeyValue reconcilers.
func stampFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) (ctrl.Result, bool, error) {
	if !controllerutil.AddFinalizer(obj, finalizer) {
		return ctrl.Result{}, false, nil
	}
	if err := c.Update(ctx, obj); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{Requeue: true}, true, nil
}

// deleteOwned best-effort removes an owned optional object by gvk/name in the
// owner's namespace.
func deleteOwned(ctx context.Context, c client.Client, owner client.Object, gvk schema.GroupVersionKind, name string) error {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(gvk)
	o.SetName(name)
	o.SetNamespace(owner.GetNamespace())
	return deleteOptionalObject(ctx, c, o)
}

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var db appv1alpha1.Database
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deletion: the CNPG Cluster, ScheduledBackup, and Pooler are owned by the
	// Database and garbage-collected via owner refs.
	// The finalizer waits for the Cluster to be fully gone before purging S3
	// backups so we don't race with in-flight WAL archiving (w7/m12).
	if !db.DeletionTimestamp.IsZero() {
		return r.handleDBDeletion(ctx, req, &db)
	}
	// codex #11: refuse a Database outside its canonical tenant namespace before
	// adding a finalizer or creating dependent resources. A compromised bex-api
	// ServiceAccount has cluster-wide Database CRUD; without this guard it could
	// place a Database in a platform or foreign-tenant namespace and drive the
	// operator as a confused deputy (the same threat the App admission policy closes).
	if !canonicalNamespace(&db.ObjectMeta) {
		logf.FromContext(ctx).Info("refusing Database outside a canonical tenant namespace (codex #11)",
			"namespace", db.Namespace, "name", db.Name)
		return ctrl.Result{}, nil
	}
	if res, done, err := stampFinalizer(ctx, r.Client, &db, dbFinalizer); done {
		return res, err
	}

	diskRequeue, err := r.applyDiskAutoscaling(ctx, &db)
	if err != nil {
		return r.dbFail(ctx, &db, "DiskAutoscalingFailed", err)
	}

	plan, storageGB := resolvePlan(db.Spec)
	dbname := db.Spec.EffectiveDatabaseName(db.Name)
	owner := db.Spec.EffectiveDatabaseUser(db.Name)

	backups, reason, err := r.prepareBackups(ctx, &db, plan)
	if err != nil {
		return r.dbFail(ctx, &db, reason, err)
	}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(db.Name)
	cluster.SetNamespace(db.Namespace)
	storageGB, result, done, err := r.databaseStorageIntent(ctx, &db, cluster, storageGB)
	if done || err != nil {
		return result, err
	}
	if err := r.reconcileCluster(ctx, &db, cluster, clusterParams{
		plan: plan, storageGB: storageGB, version: db.Spec.Version,
		dbname: dbname, owner: owner, store: backups.store,
		backupServerName: backups.targetServerName,
		recovery:         db.Spec.Recovery, users: db.Spec.Users,
		deletedUsers:     db.Spec.DeletedUsers,
		highAvailability: db.Spec.HighAvailability,
		parameters:       db.Spec.Parameters,
	}); err != nil {
		return r.dbFail(ctx, &db, "ClusterFailed", err)
	}

	r.stampConnectionStatus(&db, storageGB, backups)

	if reason, err := r.reconcileScheduledBackup(ctx, &db, backups.enabled); err != nil {
		return r.dbFail(ctx, &db, reason, err)
	}

	// --- logical exports: pg_dump directory archive -> object store ---
	exportRequeue, err := r.reconcileExports(ctx, &db, backups.storeConfigured)
	if err != nil {
		return r.dbFail(ctx, &db, "ExportFailed", err)
	}

	// --- PgBouncer Pooler when requested ---
	if poolerRequeue, err := r.reconcilePooler(ctx, &db); err != nil {
		return r.dbFail(ctx, &db, "PoolerFailed", err)
	} else if poolerRequeue {
		// CNPG creates source credentials after accepting its bootstrap spec.
		// Keep retrying until the derived Secret exists; consumers can never
		// observe a plaintext pooler URL in a Blueprint.
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// --- external SNI endpoint status (rw + pooler + read replicas) ---
	r.updateExternalAddressStatus(&db)

	if db.Spec.Suspended {
		return r.settleSuspended(ctx, &db, exportRequeue)
	}

	// Desired cluster size: the plan default, raised to 2 for HA so the ready-gate
	// waits for the standby to come up before reporting the cluster as Ready.
	desiredInstances := int64(plan.Instances)
	if db.Spec.HighAvailability && desiredInstances < 2 {
		desiredInstances = 2
	}

	return r.reconcileDatabaseReadiness(ctx, &db, cluster, desiredInstances, backups.enabled, backups.targetServerName, soonerRequeue(exportRequeue, diskRequeue))
}

// databaseBackupIntent is what one reconcile resolved about this Database's
// backups: whether a store exists at all, whether this cluster gets its own
// backups, and the Barman server names it reads from and writes to.
type databaseBackupIntent struct {
	store             *BackupStore
	storeConfigured   bool
	enabled           bool
	currentServerName string
	targetServerName  string
}

// prepareBackups resolves the Database's backup intent and projects the Barman
// ObjectStore into its namespace when backups are on. A non-empty reason
// accompanies an error and names the status the caller should fail with.
//
// Backups (and so recovery/PITR) require the store to be configured; whether
// this cluster gets its OWN backups additionally needs the plan to opt in.
// Restoring to a new instance without a store is a hard error (nothing to
// restore from) — fail loudly rather than silently init an empty db.
func (r *DatabaseReconciler) prepareBackups(ctx context.Context, db *appv1alpha1.Database, plan tiers.PostgresTier) (databaseBackupIntent, string, error) {
	intent := databaseBackupIntent{storeConfigured: r.Backup.configured()}
	if intent.storeConfigured {
		intent.store = &r.Backup
	}
	intent.enabled = plan.Backup && intent.storeConfigured
	intent.currentServerName, intent.targetServerName = databaseBackupServerNames(db)
	if db.Spec.Recovery != nil && !intent.storeConfigured {
		return intent, "RecoveryUnavailable", fmt.Errorf("recovery requested but no backup store is configured")
	}
	// The Barman ObjectStore and its S3 credential are namespaced, and GitOps
	// installs exactly one of each in the shared apps namespace. A Database in a
	// tenant namespace (ADR043 D8.4) therefore needs both projected there before
	// the Cluster below can reference them by name. Failing loudly beats a
	// Cluster that comes up with a dangling barmanObjectName and silently
	// archives nothing.
	if intent.enabled {
		if err := r.reconcileTenantBackupStore(ctx, db); err != nil {
			return intent, "BackupStoreUnavailable", err
		}
	}
	return intent, "", nil
}

// reconcileCluster projects the Database onto its CNPG Cluster (same name, same
// namespace). The caller owns cluster because the readiness gate reads it back.
func (r *DatabaseReconciler) reconcileCluster(ctx context.Context, db *appv1alpha1.Database, cluster *unstructured.Unstructured, params clusterParams) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cluster, func() error {
		spec := cnpgClusterSpec(params)
		// Mark CNPG-managed pods as tenant databases so the node log shipper can
		// exclude platform/auth CNPG clusters. Propagate workspace identity through
		// the same bounded inheritedMetadata label map for NetworkPolicy selection.
		inheritedLabels := map[string]any{"app.bex.co/component": "database"}
		if ws := db.Labels[labelWorkspace]; ws != "" {
			inheritedLabels[labelWorkspace] = ws
		}
		spec["inheritedMetadata"] = map[string]any{"labels": inheritedLabels}
		cluster.Object["spec"] = spec
		setLifecycleAnnotations(cluster, db.Spec.Suspended, db.Spec.RestartedAt)
		return controllerutil.SetControllerReference(db, cluster, r.Scheme)
	})
	return err
}

// stampConnectionStatus records how consumers reach this Database and what its
// backups resolved to. CNPG generates Secret "<cluster>-app" (username/password/
// dbname/host/port/uri) and Service "<cluster>-rw" — the internal Database URL
// is that Secret's "uri".
func (r *DatabaseReconciler) stampConnectionStatus(db *appv1alpha1.Database, storageGB int32, backups databaseBackupIntent) {
	db.Status.Host = fmt.Sprintf("%s-rw.%s.svc", db.Name, db.Namespace)
	db.Status.Port = pgPort
	db.Status.SecretName = db.Name + "-app"
	db.Status.AllocatedStorageGB = storageGB
	db.Status.ObservedStorageGB = db.Spec.StorageGB
	db.Status.ObservedGeneration = db.Generation
	db.Status.BackupsEnabled = backups.enabled
	if backups.enabled && db.Status.BackupServerName == "" {
		db.Status.BackupServerName = backups.currentServerName
	}
	if backups.storeConfigured {
		db.Status.BackupEndpointURL = r.Backup.EndpointURL
		db.Status.BackupS3SecretName = r.Backup.S3Secret
	} else {
		db.Status.BackupEndpointURL = ""
		db.Status.BackupS3SecretName = ""
	}
}

// reconcileScheduledBackup converges the daily base backup on the plan's
// intent. A non-empty reason accompanies an error.
func (r *DatabaseReconciler) reconcileScheduledBackup(ctx context.Context, db *appv1alpha1.Database, backupEnabled bool) (string, error) {
	if backupEnabled {
		if err := upsertOwned(ctx, r.Client, r.Scheme, db, cnpgScheduledBackupGVK, db.Name+"-backup", scheduledBackupSpec(db.Name)); err != nil {
			return "ScheduledBackupFailed", err
		}
		return "", nil
	}
	if err := deleteOwned(ctx, r.Client, db, cnpgScheduledBackupGVK, db.Name+"-backup"); err != nil {
		return "ScheduledBackupCleanupFailed", err
	}
	return "", nil
}

// settleSuspended reports a hibernated Database Ready immediately — CNPG stops
// compute, so waiting on readyInstances would never converge (mirrors the
// KeyValue suspend path).
func (r *DatabaseReconciler) settleSuspended(ctx context.Context, db *appv1alpha1.Database, exportRequeue time.Duration) (ctrl.Result, error) {
	db.Status.Phase = appv1alpha1.DBPhaseReady
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Suspended",
		Message: "postgres suspended (hibernated; PVC and config kept)", ObservedGeneration: db.Generation,
	})
	if err := r.Status().Update(ctx, db); err != nil {
		return ctrl.Result{}, err
	}
	if exportRequeue > 0 {
		return ctrl.Result{RequeueAfter: exportRequeue}, nil
	}
	return ctrl.Result{}, nil
}

// reconcilePooler creates/removes the owned CNPG Pooler and its derived
// connection Secret. Its boolean result means the source Secret is not ready
// yet and reconciliation should retry.
func (r *DatabaseReconciler) reconcilePooler(ctx context.Context, db *appv1alpha1.Database) (bool, error) {
	if !db.Spec.Pooler {
		if err := deleteOwned(ctx, r.Client, db, cnpgPoolerGVK, db.Name+"-pooler"); err != nil {
			return false, fmt.Errorf("remove pooler: %w", err)
		}
		if err := r.deletePoolerConnectionSecret(ctx, db); err != nil {
			return false, fmt.Errorf("remove pooler credentials: %w", err)
		}
		db.Status.PoolerHost = ""
		return false, nil
	}
	if err := upsertOwned(ctx, r.Client, r.Scheme, db, cnpgPoolerGVK, db.Name+"-pooler", poolerSpec(db.Name)); err != nil {
		return false, fmt.Errorf("create pooler: %w", err)
	}
	db.Status.PoolerHost = fmt.Sprintf("%s-pooler.%s.svc", db.Name, db.Namespace)
	ready, err := r.reconcilePoolerConnectionSecret(ctx, db)
	if err != nil {
		return false, fmt.Errorf("project pooler credentials: %w", err)
	}
	return !ready, nil
}

// reconcilePoolerConnectionSecret projects CNPG's rotating application
// credentials into a second Secret whose URI targets the owned PgBouncer
// Service. A Blueprint fromDatabase.connectionPoolString can therefore remain
// a normal SecretKeyRef rather than injecting a password-bearing URL into an
// App spec. The source Secret is never mutated.
func (r *DatabaseReconciler) reconcilePoolerConnectionSecret(ctx context.Context, db *appv1alpha1.Database) (bool, error) {
	source := &corev1.Secret{}
	if err := r.secretClient().Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: db.Name + "-app"}, source); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	username, password, databaseName := source.Data["username"], source.Data["password"], source.Data["dbname"]
	if len(username) == 0 || len(password) == 0 || len(databaseName) == 0 {
		return false, fmt.Errorf("CNPG application Secret %q lacks username, password, or dbname", source.Name)
	}
	target := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: db.Name + "-pooler-app", Namespace: db.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.secretClient(), target, func() error {
		target.Type = corev1.SecretTypeOpaque
		target.Data = map[string][]byte{
			"uri": fmt.Appendf(nil, "postgresql://%s:%s@%s:5432/%s", username, password, db.Status.PoolerHost, databaseName),
		}
		return controllerutil.SetControllerReference(db, target, r.Scheme)
	})
	return err == nil, err
}

// reconcileTenantBackupStore projects the backup transport into a Database's own
// namespace: the S3 credential Secret and the Barman Cloud ObjectStore the
// Cluster references by name (ADR043 D8.4).
//
// Both are namespaced, and GitOps installs exactly one of each — it cannot
// enumerate namespaces created at workspace-create time. The operator owns this
// because it already owns the whole backup contract (the BEX_DB_BACKUP_* env is
// its own); giving it to the namespace reconciler would split one contract
// across two processes with two config sources.
//
// The ObjectStore spec is COPIED from the GitOps original rather than rebuilt
// from env, so destination, retention, compression, and SSE stay single-sourced
// in Git. Rebuilding it here would create a second place for the backup policy
// to drift, and the drift would only be discovered at restore time.
//
// A no-op when the Database is already in the source namespace (the pre-D8
// topology and single-namespace deployments), so those stay byte-identical.
func (r *DatabaseReconciler) reconcileTenantBackupStore(ctx context.Context, db *appv1alpha1.Database) error {
	src := r.BackupSourceNamespace
	if src == "" || src == db.Namespace {
		return nil
	}

	// The credential. Tenant workloads cannot read it: they have no Kubernetes
	// API access and run with automountServiceAccountToken disabled (w7/m2), so
	// a namespace-local Secret is not reachable from tenant code.
	if err := projectBackupCredential(ctx, r.secretClient(), src, db.Namespace, r.Backup.S3Secret); err != nil {
		return err
	}

	// The ObjectStore, spec copied verbatim from the GitOps original.
	source := &unstructured.Unstructured{}
	source.SetGroupVersionKind(barmanCloudObjectStoreGVK)
	if err := r.Get(ctx, client.ObjectKey{Namespace: src, Name: tenantBackupObjectStoreName}, source); err != nil {
		return fmt.Errorf("reading ObjectStore %s/%s: %w", src, tenantBackupObjectStoreName, err)
	}
	spec, found, err := unstructured.NestedMap(source.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("ObjectStore %s/%s has no usable spec", src, tenantBackupObjectStoreName)
	}
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(barmanCloudObjectStoreGVK)
	target.SetName(tenantBackupObjectStoreName)
	target.SetNamespace(db.Namespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, target, func() error {
		// Namespace-scoped and shared by every Database in this namespace, so it
		// is deliberately NOT owned by any one of them — an ownerReference would
		// delete the store the moment its first Database went away and silently
		// break every sibling's archiving.
		return unstructured.SetNestedMap(target.Object, spec, "spec")
	}); err != nil {
		return fmt.Errorf("projecting ObjectStore into %s: %w", db.Namespace, err)
	}
	return nil
}

// projectBackupCredential copies the platform's S3 backup credential from the
// source namespace into a datastore's own namespace (ADR043 D8.4). Shared by the
// Database and KeyValue reconcilers, which need the identical copy for different
// consumers (the Barman ObjectStore vs. the RDB CronJob).
func projectBackupCredential(ctx context.Context, cl client.Client, srcNS, dstNS, name string) error {
	var credential corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: srcNS, Name: name}, &credential); err != nil {
		return fmt.Errorf("reading backup credential %s/%s: %w", srcNS, name, err)
	}
	projected := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dstNS}}
	if _, err := controllerutil.CreateOrUpdate(ctx, cl, projected, func() error {
		projected.Type = credential.Type
		projected.Data = credential.Data
		// codex F1: mark the shared backup credential so the App reconcile refuses
		// to wire it into any co-located tenant pod (EnvFrom/volume/env by name).
		if projected.Labels == nil {
			projected.Labels = map[string]string{}
		}
		projected.Labels[execution.LabelProtectedFromTenantMount] = execution.ProtectedFromTenantMount
		return nil
	}); err != nil {
		return fmt.Errorf("projecting backup credential into %s: %w", dstNS, err)
	}
	return nil
}

func (r *DatabaseReconciler) deletePoolerConnectionSecret(ctx context.Context, db *appv1alpha1.Database) error {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: db.Name + "-pooler-app", Namespace: db.Namespace}}
	if err := r.secretClient().Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// reconcileDatabaseReadiness maps CNPG's status-only lifecycle onto the public
// Database phase and performs the one-time post-upgrade backup transition.
func (r *DatabaseReconciler) reconcileDatabaseReadiness(
	ctx context.Context,
	db *appv1alpha1.Database,
	cluster *unstructured.Unstructured,
	desiredInstances int64,
	backupEnabled bool,
	targetBackupServerName string,
	exportRequeue time.Duration,
) (ctrl.Result, error) {
	previousVersion := db.Status.CurrentVersion
	clusterState := r.readClusterStatus(ctx, db, cluster)
	if clusterState.currentVersion != "" {
		db.Status.CurrentVersion = clusterState.currentVersion
	}

	// CNPG's offline pg_upgrade deliberately takes every instance down. Surface
	// its own phase before consulting readyInstances so an upgrade is never
	// mislabeled as ordinary provisioning (or, worse, left Ready from the prior
	// generation). A failed upgrade is likewise an honest terminal status with
	// CNPG's phase/condition message retained for operators and API clients.
	if clusterState.majorUpgradeFailed() {
		// CNPG leaves the failed target image in desired state and requires an
		// explicit revert. Return Database.spec.version to the observed source so
		// the next reconcile deletes the failed job and restarts the original
		// server instead of retrying forever.
		if db.Status.CurrentVersion != "" && db.Spec.Version != db.Status.CurrentVersion {
			before := db.DeepCopy()
			db.Spec.Version = db.Status.CurrentVersion
			if err := r.Patch(ctx, db, client.MergeFrom(before)); err != nil {
				return ctrl.Result{}, err
			}
		}
		db.Status.Phase = appv1alpha1.DBPhaseFailed
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "MajorVersionUpgradeFailed",
			Message: clusterState.message, ObservedGeneration: db.Generation,
		})
		if err := r.Status().Update(ctx, db); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if clusterState.majorUpgradeRunning() {
		db.Status.Phase = appv1alpha1.DBPhaseUpgrading
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "MajorVersionUpgrade",
			Message: clusterState.message, ObservedGeneration: db.Generation,
		})
		if err := r.Status().Update(ctx, db); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if clusterState.ready >= desiredInstances {
		// A major upgrade creates a new PostgreSQL system ID/timeline; pre-upgrade
		// backups cannot provide PITR into the new major. Start a fresh base backup
		// as soon as CNPG reports the upgraded cluster healthy.
		if backupEnabled && previousVersion != "" && clusterState.currentVersion != "" && previousVersion != clusterState.currentVersion {
			name := fmt.Sprintf("%s-post-upgrade-pg%s", db.Name, clusterState.currentVersion)
			if err := upsertOwned(ctx, r.Client, r.Scheme, db, cnpgBackupGVK, name, onDemandBackupSpec(db.Name)); err != nil {
				return r.dbFail(ctx, db, "PostUpgradeBackupFailed", err)
			}
			db.Status.BackupServerName = targetBackupServerName
		}
		db.Status.Phase = appv1alpha1.DBPhaseReady
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Provisioned",
			Message: "postgres ready", ObservedGeneration: db.Generation,
		})
		if err := r.Status().Update(ctx, db); err != nil {
			return ctrl.Result{}, err
		}
		logf.FromContext(ctx).Info("database ready", "name", db.Name, "host", db.Status.Host)
		if exportRequeue > 0 {
			return ctrl.Result{RequeueAfter: exportRequeue}, nil
		}
		return ctrl.Result{}, nil
	}

	db.Status.Phase = appv1alpha1.DBPhaseProvisioning
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Provisioning",
		Message: "waiting for CloudNativePG", ObservedGeneration: db.Generation,
	})
	if err := r.Status().Update(ctx, db); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: soonerRequeue(10*time.Second, exportRequeue)}, nil
}

func soonerRequeue(current, candidate time.Duration) time.Duration {
	if candidate <= 0 || (current > 0 && current <= candidate) {
		return current
	}
	return candidate
}

// setLifecycleAnnotations maps the Database lifecycle intent onto CNPG's cluster
// annotations: declarative hibernation (suspend/resume) and a manual rolling
// restart (verb-as-timestamp). Preserves any other annotations already present.
func setLifecycleAnnotations(cluster *unstructured.Unstructured, suspended bool, restartedAt string) {
	anns := cluster.GetAnnotations()
	if anns == nil {
		anns = map[string]string{}
	}
	if suspended {
		anns[hibernationAnnotation] = "on"
	} else {
		delete(anns, hibernationAnnotation)
	}
	if restartedAt != "" {
		anns[restartAnnotation] = restartedAt
	}
	cluster.SetAnnotations(anns)
}

// readClusterStatus reads the CNPG cluster's status, updates HA-related Database
// status fields (CurrentPrimary, HighAvailabilityEnabled, LastFailoverAt) and
// triggers a switchover if spec.failoverAt has advanced. Returns readyInstances.
type cnpgClusterState struct {
	ready          int64
	phase          string
	message        string
	currentVersion string
}

func (s cnpgClusterState) majorUpgradeRunning() bool {
	phase := strings.ToLower(s.phase)
	return strings.Contains(phase, "upgrad") && strings.Contains(phase, "postgres") && !strings.Contains(phase, "fail")
}

func (s cnpgClusterState) majorUpgradeFailed() bool {
	phase := strings.ToLower(s.phase)
	return strings.Contains(phase, "upgrad") && strings.Contains(phase, "postgres") && strings.Contains(phase, "fail")
}

// cnpgConditionMessage returns the most useful condition message CNPG exposes,
// falling back to its phase. This keeps a failed pg_upgrade's reason visible on
// Database.status without coupling the operator to CNPG's Go API.
func cnpgConditionMessage(cluster *unstructured.Unstructured, fallback string) string {
	conditions, found, _ := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	if found {
		for _, raw := range conditions {
			condition, ok := raw.(map[string]any)
			if !ok || condition["status"] != "False" {
				continue
			}
			if message, ok := condition["message"].(string); ok && message != "" {
				return message
			}
		}
	}
	return fallback
}

func (r *DatabaseReconciler) readClusterStatus(
	ctx context.Context,
	db *appv1alpha1.Database,
	cluster *unstructured.Unstructured,
) cnpgClusterState {
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
		db.Status.HighAvailabilityEnabled = false
		return cnpgClusterState{}
	}
	ready, _, _ := unstructured.NestedInt64(cluster.Object, "status", "readyInstances")
	phase, _, _ := unstructured.NestedString(cluster.Object, "status", "phase")
	major, _, _ := unstructured.NestedInt64(cluster.Object, "status", "pgDataImageInfo", "majorVersion")
	state := cnpgClusterState{ready: ready, phase: phase, message: cnpgConditionMessage(cluster, phase)}
	if major > 0 {
		state.currentVersion = fmt.Sprintf("%d", major)
	}
	if cp, found, _ := unstructured.NestedString(cluster.Object, "status", "currentPrimary"); found {
		db.Status.CurrentPrimary = cp
	}
	// Trigger a CNPG switchover when spec.failoverAt has advanced and HA is on.
	// Patches cluster.status.targetPrimary to a non-primary instance so CNPG
	// promotes it (the planned-switchover path — docs/render-artifacts/postgres-ha.md).
	if db.Spec.HighAvailability && db.Spec.FailoverAt != "" && db.Spec.FailoverAt != db.Status.LastFailoverAt {
		r.triggerCNPGSwitchover(ctx, cluster)
		db.Status.LastFailoverAt = db.Spec.FailoverAt
	}
	// HighAvailabilityEnabled: HA is on in spec AND the cluster has ≥2 ready instances.
	db.Status.HighAvailabilityEnabled = db.Spec.HighAvailability && ready >= 2
	return state
}

// triggerCNPGSwitchover patches cluster.status.targetPrimary to a non-primary
// instance to initiate a CNPG planned switchover. A no-op when no ready
// standby exists; failures are logged but do not block the reconcile loop.
func (r *DatabaseReconciler) triggerCNPGSwitchover(ctx context.Context, cluster *unstructured.Unstructured) {
	log := logf.FromContext(ctx)
	currentPrimary, _, _ := unstructured.NestedString(cluster.Object, "status", "currentPrimary")
	instanceNames, _, _ := unstructured.NestedStringSlice(cluster.Object, "status", "instanceNames")
	var target string
	for _, inst := range instanceNames {
		if inst != currentPrimary {
			target = inst
			break
		}
	}
	if target == "" {
		return
	}
	clusterPatch := cluster.DeepCopy()
	if setErr := unstructured.SetNestedField(clusterPatch.Object, target, "status", "targetPrimary"); setErr == nil {
		if patchErr := r.Status().Patch(ctx, clusterPatch, client.MergeFrom(cluster)); patchErr != nil {
			log.Info("failover: status patch failed (may not have a ready replica yet)", "err", patchErr)
		}
	}
}

// updateExternalAddressStatus publishes status hostnames for the metered Postgres
// SNI proxy. The proxy watches the same Database CR and owns exact
// endpoint/allowlist routing; internal CNPG Services are never gated.
// InternalHost for replicas is always set.
func (r *DatabaseReconciler) updateExternalAddressStatus(db *appv1alpha1.Database) {
	public := db.Spec.Public && r.DBDomain != ""

	roInternal := fmt.Sprintf("%s-ro.%s.svc", db.Name, db.Namespace)
	newStatuses := make([]appv1alpha1.DatabaseReadReplicaStatus, 0, len(db.Spec.ReadReplicas))
	if !public {
		db.Status.ExternalHost = ""
		db.Status.PoolerExternalHost = ""
		for _, rep := range db.Spec.ReadReplicas {
			newStatuses = append(newStatuses, appv1alpha1.DatabaseReadReplicaStatus{
				Name: rep.Name, InternalHost: roInternal,
			})
		}
		db.Status.ReadReplicaStatuses = newStatuses
		return
	}

	rwHost := fmt.Sprintf("%s.%s", db.Name, r.DBDomain)
	db.Status.ExternalHost = rwHost

	if db.Spec.Pooler {
		db.Status.PoolerExternalHost = fmt.Sprintf("%s-pool.%s", db.Name, r.DBDomain)
	} else {
		db.Status.PoolerExternalHost = ""
	}

	for _, rep := range db.Spec.ReadReplicas {
		roHost := fmt.Sprintf("%s-ro-%s.%s", db.Name, rep.Name, r.DBDomain)
		newStatuses = append(newStatuses, appv1alpha1.DatabaseReadReplicaStatus{
			Name: rep.Name, InternalHost: roInternal, ExternalHost: roHost,
		})
	}
	db.Status.ReadReplicaStatuses = newStatuses
}

func (r *DatabaseReconciler) dbFail(ctx context.Context, db *appv1alpha1.Database, reason string, err error) (ctrl.Result, error) {
	db.Status.Phase = appv1alpha1.DBPhaseFailed
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: err.Error(),
		ObservedGeneration: db.Generation,
	})
	_ = r.Status().Update(ctx, db)
	return ctrl.Result{}, err
}

func (r *DatabaseReconciler) rejectDatabaseStorageShrink(ctx context.Context, db *appv1alpha1.Database, current, requested int32) error {
	db.Status.Phase = appv1alpha1.DBPhaseFailed
	db.Status.AllocatedStorageGB = current
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "StorageShrinkRejected",
		Message:            fmt.Sprintf("Postgres storage is grow-only: requested %d GB is below the allocated %d GB", requested, current),
		ObservedGeneration: db.Generation,
	})
	return r.Status().Update(ctx, db)
}

// SetupWithManager wires the controller. It owns the CNPG Cluster (unstructured)
// so deletes cascade and Cluster changes re-trigger reconcile.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	owned := &unstructured.Unstructured{}
	owned.SetGroupVersionKind(cnpgClusterGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.Database{}, builder.WithPredicates(generationDeletionOrFinalizerPredicate{})).
		// CNPG reports readiness and major-upgrade progress through Cluster status
		// updates, which do not advance metadata.generation. Watch resourceVersion
		// so Database.status promptly reflects Upgrading, Failed, and Ready.
		Owns(owned, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		// Job status updates do not change metadata.generation, so use resource
		// version here; otherwise completed/failed exports would never settle.
		Owns(&batchv1.Job{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Named("database").
		Complete(r)
}

// handleDBDeletion runs the finalizer teardown for a Database being deleted:
// waits for the CNPG Cluster to be fully gone (preventing a race with in-flight
// WAL archiving), then optionally dispatches an S3 backup purge Job before
// removing the finalizer. Extracted from Reconcile to keep its cyclomatic
// complexity in check.
func (r *DatabaseReconciler) handleDBDeletion(ctx context.Context, req ctrl.Request, db *appv1alpha1.Database) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(db, dbFinalizer) {
		// Wait for the CNPG Cluster to be gone before purging its S3 backups.
		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(cnpgClusterGVK)
		if err := r.Get(ctx, req.NamespacedName, cluster); err == nil {
			// Owner-ref garbage collection only removes the Cluster AFTER the
			// Database object is gone from storage, while this finalizer keeps
			// the Database around until the Cluster is gone — waiting here
			// without acting deadlocks the deletion. Delete the Cluster
			// explicitly (idempotent) and requeue until it is fully gone.
			if cluster.GetDeletionTimestamp() == nil {
				if err := r.Delete(ctx, cluster); err != nil && !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		} else if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		// Cluster is gone: run and observe the persisted barman S3 purge Job.
		if db.Status.BackupsEnabled && r.Backup.configured() {
			job := r.dbBackupPurgeJob(db)
			done, err := reconcileCleanupJob(ctx, r.Client, db, job, annotDBBackupPurgeComplete)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !done {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			}
		} else if db.Status.BackupsEnabled {
			return ctrl.Result{}, fmt.Errorf("database backup purge configuration is unavailable")
		}
		controllerutil.RemoveFinalizer(db, dbFinalizer)
		if err := r.Update(ctx, db); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// dbBackupPurgeJob constructs the durable terminal Job that recursively deletes
// the database's legacy and per-major barman object-store prefixes.
func (r *DatabaseReconciler) dbBackupPurgeJob(db *appv1alpha1.Database) *batchv1.Job {
	deadline := int64((15 * time.Minute) / time.Second)
	backoff := int32(3)
	base := strings.TrimRight(r.Backup.DestinationPath, "/")
	jobName := cleanupJobName("purge-db-", db.Name, db.UID)
	labels := map[string]string{
		"app.bex.co/database":      db.Name,
		execution.LabelDatabaseUID: string(db.UID),
		execution.LabelComponent:   "database-backup-purge",
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: db.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoff,
			ActiveDeadlineSeconds: &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					AutomountServiceAccountToken: ptr(false),
					Containers: []corev1.Container{{
						Name:    "purge",
						Image:   publish.DefaultAWSCLIImage,
						Command: []string{"/bin/sh", "-ec"},
						Args: []string{
							`for suffix in '' -pg13 -pg14 -pg15 -pg16 -pg17 -pg18; do
  aws s3 rm "${DESTINATION}/${DATABASE}${suffix}/" --recursive --endpoint-url "${ENDPOINT}"
done`,
						},
						Env: []corev1.EnvVar{
							{Name: "DESTINATION", Value: base},
							{Name: "DATABASE", Value: db.Name},
							{Name: "ENDPOINT", Value: r.Backup.EndpointURL},
						},
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: r.Backup.S3Secret},
							},
						}},
					}},
				},
			},
		},
	}
	return job
}
