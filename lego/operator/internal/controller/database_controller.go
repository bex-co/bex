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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cnpgClusterGVK is the CloudNativePG Cluster type. We project onto it via
// unstructured so the operator needn't vendor CNPG's Go API (and stays decoupled
// from its version).
var cnpgClusterGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster"}

// traefikIngressRouteTCPGVK is Traefik's TCP router CRD (v3). Used for the
// external SNI endpoint.
var traefikIngressRouteTCPGVK = schema.GroupVersionKind{Group: "traefik.io", Version: "v1alpha1", Kind: "IngressRouteTCP"}

// traefikMiddlewareTCPGVK is Traefik's TCP middleware CRD (v3). Used to gate the
// external SNI endpoint with an ipAllowList (source-range) middleware.
var traefikMiddlewareTCPGVK = schema.GroupVersionKind{Group: "traefik.io", Version: "v1alpha1", Kind: "MiddlewareTCP"}

// cnpgScheduledBackupGVK / cnpgPoolerGVK are the CloudNativePG resources the
// Database projects for durability (a daily base backup) and connection pooling
// (a PgBouncer instance). Projected via unstructured like the Cluster.
var cnpgScheduledBackupGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "ScheduledBackup"}
var cnpgPoolerGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Pooler"}

const (
	dbStorageClass = "hcloud-volumes"
	// pgEntryPoint is the Traefik TCP entrypoint bound to :5432 (traefik values).
	pgEntryPoint = "postgres"
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
	// backupRetention is how long CNPG keeps base backups + WAL in object storage.
	backupRetention = "30d"
	// backupSchedule is the daily base-backup cron (CNPG's 6-field form:
	// sec min hour dom mon dow) — 03:00 UTC.
	backupSchedule = "0 0 3 * * *"
)

// BackupStore is the object-store target CNPG's barmanObjectStore writes to
// (Wasabi/S3-compatible), reusing the etcd/OpenBao backup credential pattern
// (docs/ADR011-etcd-backup-restore.md). All three fields must be set for the controller
// to project backups; otherwise no plan gets them (recovery/PITR unavailable).
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
}

// configured reports whether every field needed to project backups is present.
func (b BackupStore) configured() bool {
	return b.DestinationPath != "" && b.EndpointURL != "" && b.S3Secret != ""
}

// normalizeIdent turns an App/DB name into a valid unquoted PostgreSQL
// identifier: lowercase, hyphens -> underscores. So a client never has to
// double-quote the db/role name (docs/ADR009-postgresql-management.md §4).
func normalizeIdent(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "-", "_"))
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

// clusterParams bundles the inputs the CNPG Cluster projection needs beyond the
// plan — kept as a struct so the growing set (backup, recovery, users) stays
// readable and the projection remains a pure, unit-testable function.
type clusterParams struct {
	plan      tiers.PostgresTier
	storageGB int32
	version   string
	dbname    string
	owner     string
	// store is the object-store config when the controller has one (nil => not
	// configured). It backs both this cluster's own backups and a recovery read.
	// A cluster gets its own backup stanza when the plan opts in (plan.Backup) and
	// store is set — derived, not passed.
	store *BackupStore
	// recovery, when non-nil (with store set), bootstraps the cluster by restoring
	// a source Database's backups instead of a fresh initdb.
	recovery *appv1alpha1.DatabaseRecovery
	// users are additional managed login roles (spec.managed.roles).
	users []appv1alpha1.DatabaseUser
}

// barmanObjectStore builds a CNPG barmanObjectStore config pointing at the S3
// target. serverName scopes the backup path under DestinationPath — empty lets
// CNPG default it to the cluster name (for a cluster's own backups); a restore's
// externalCluster passes the source Database name so it reads the right path.
func barmanObjectStore(store BackupStore, serverName string) map[string]any {
	m := map[string]any{
		"destinationPath": store.DestinationPath,
		"endpointURL":     store.EndpointURL,
		"s3Credentials": map[string]any{
			"accessKeyId":     map[string]any{"name": store.S3Secret, "key": "AWS_ACCESS_KEY_ID"},
			"secretAccessKey": map[string]any{"name": store.S3Secret, "key": "AWS_SECRET_ACCESS_KEY"},
		},
		"wal":  map[string]any{"compression": "gzip"},
		"data": map[string]any{"compression": "gzip"},
	}
	if serverName != "" {
		m["serverName"] = serverName
	}
	return m
}

// managedRoles projects additional Database users onto CNPG spec.managed.roles —
// login roles ensured present, each with its password read from the referenced
// Secret (created by bex-api). Returns nil for no users (so spec.managed is omitted).
func managedRoles(users []appv1alpha1.DatabaseUser) []any {
	if len(users) == 0 {
		return nil
	}
	roles := make([]any, 0, len(users))
	for _, u := range users {
		role := map[string]any{"name": u.Name, "ensure": "present", "login": true}
		if u.SecretName != "" {
			role["passwordSecret"] = map[string]any{"name": u.SecretName}
		}
		roles = append(roles, role)
	}
	return roles
}

// cnpgClusterSpec builds the CloudNativePG Cluster .spec for a Database. Pure
// (no client) so the plan->Cluster projection is unit-testable. When p.recovery
// is set (with a backup store), the cluster bootstraps by restoring a source
// Database's object-store backups — a NEW instance, never in place.
func cnpgClusterSpec(p clusterParams) map[string]any {
	spec := map[string]any{
		"instances": int64(p.plan.Instances),
		"storage": map[string]any{
			"size":         fmt.Sprintf("%dGi", p.storageGB),
			"storageClass": dbStorageClass,
		},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": p.plan.CPU, "memory": p.plan.Memory},
			"limits":   map[string]any{"cpu": p.plan.CPU, "memory": p.plan.Memory},
		},
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
			"name":              recoverySource,
			"barmanObjectStore": barmanObjectStore(*p.store, p.recovery.SourceDatabase),
		}}
	} else {
		spec["bootstrap"] = map[string]any{
			"initdb": map[string]any{"database": p.dbname, "owner": p.owner},
		}
	}
	// Durability: continuous WAL archiving + base backups to object storage —
	// when the plan opts in and a store is configured.
	if p.plan.Backup && p.store != nil {
		spec["backup"] = map[string]any{
			"barmanObjectStore": barmanObjectStore(*p.store, ""),
			"retentionPolicy":   backupRetention,
		}
	}
	if roles := managedRoles(p.users); roles != nil {
		spec["managed"] = map[string]any{"roles": roles}
	}
	return spec
}

// scheduledBackupSpec builds a CNPG ScheduledBackup .spec: a daily base backup
// of the cluster to object storage (WAL archiving from the Cluster's backup
// stanza is what actually enables PITR; this pins recovery windows down).
func scheduledBackupSpec(clusterName string) map[string]any {
	return map[string]any{
		"schedule":             backupSchedule,
		"backupOwnerReference": "self",
		"immediate":            true,
		"cluster":              map[string]any{"name": clusterName},
		"method":               "barmanObjectStore",
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

// ipAllowListMiddlewareSpec builds a Traefik MiddlewareTCP .spec restricting the
// source IP range — attached to the external SNI route so only listed CIDRs
// reach the public Postgres endpoint (the internal -rw path is never gated).
func ipAllowListMiddlewareSpec(cidrs []string) map[string]any {
	ranges := make([]any, len(cidrs))
	for i, c := range cidrs {
		ranges[i] = c
	}
	return map[string]any{"ipAllowList": map[string]any{"sourceRange": ranges}}
}

// ingressRouteTCPSpec builds a Traefik IngressRouteTCP .spec routing an SNI
// hostname to a backend Service with TLS passthrough (the backend terminates its
// own TLS), optionally gated by TCP middlewares (an ipAllowList). Shared by the
// Database (postgres :5432) and KeyValue (valkey :6379) public routes; pass nil
// middlewares for an ungated route. Pure so the projection is unit-testable.
func ingressRouteTCPSpec(entryPoint, host, serviceName string, port int64, middlewares []any) map[string]any {
	route := map[string]any{
		"match": fmt.Sprintf("HostSNI(`%s`)", host),
		"services": []any{map[string]any{
			"name": serviceName,
			"port": port,
		}},
	}
	if len(middlewares) > 0 {
		route["middlewares"] = middlewares
	}
	return map[string]any{
		"entryPoints": []any{entryPoint},
		"routes":      []any{route},
		"tls":         map[string]any{"passthrough": true},
	}
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
// Traefik, off-cluster backups (barmanObjectStore + ScheduledBackup), a
// PgBouncer Pooler, and an IP-allowlist middleware. It is a thin executor —
// CNPG does the actual Postgres lifecycle.
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// DBDomain is the wildcard base for external endpoints, e.g. "db.bex.co".
	// Empty => Public databases get no external route.
	DBDomain string
	// Backup is the object-store target for backups + PITR. Unset (any field
	// empty) => backups are disabled for every plan (recovery unavailable).
	Backup BackupStore
}

// +kubebuilder:rbac:groups=app.bex.co,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters;scheduledbackups;poolers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutetcps;middlewaretcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// upsertOwned creates-or-updates an owned unstructured object (gvk/name in the
// Database's namespace), applying mutate to its .spec and stamping an owner
// reference so the object is garbage-collected with the Database.
func (r *DatabaseReconciler) upsertOwned(ctx context.Context, db *appv1alpha1.Database, gvk schema.GroupVersionKind, name string, spec map[string]any) error {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(gvk)
	o.SetName(name)
	o.SetNamespace(db.Namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, o, func() error {
		o.Object["spec"] = spec
		return controllerutil.SetControllerReference(db, o, r.Scheme)
	})
	return err
}

// deleteOwned best-effort removes an owned optional object by gvk/name.
func (r *DatabaseReconciler) deleteOwned(ctx context.Context, db *appv1alpha1.Database, gvk schema.GroupVersionKind, name string) error {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(gvk)
	o.SetName(name)
	o.SetNamespace(db.Namespace)
	return deleteOptionalObject(ctx, r.Client, o)
}

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var db appv1alpha1.Database
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Deletion: the CNPG Cluster (and the ScheduledBackup, Pooler, routes and
	// middleware) is owned by the Database, so it is garbage-collected via owner
	// refs automatically.
	if !db.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	plan, storageGB := resolvePlan(db.Spec)
	dbname := normalizeIdent(db.Name)
	owner := dbname + "_user"

	// Backups (and so recovery/PITR) require the store to be configured; whether
	// this cluster gets its OWN backups additionally needs the plan to opt in.
	// Restoring to a new instance without a store is a hard error (nothing to
	// restore from) — fail loudly rather than silently init an empty db.
	storeConfigured := r.Backup.configured()
	var store *BackupStore
	if storeConfigured {
		store = &r.Backup
	}
	backupEnabled := plan.Backup && storeConfigured
	if db.Spec.Recovery != nil && !storeConfigured {
		return r.dbFail(ctx, &db, "RecoveryUnavailable",
			fmt.Errorf("recovery requested but no backup store is configured"))
	}

	// --- project onto a CNPG Cluster (same name, same namespace) ---
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(db.Name)
	cluster.SetNamespace(db.Namespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cluster, func() error {
		spec := cnpgClusterSpec(clusterParams{
			plan: plan, storageGB: storageGB, version: db.Spec.Version,
			dbname: dbname, owner: owner, store: store,
			recovery: db.Spec.Recovery, users: db.Spec.Users,
		})
		// Propagate the workspace label to CNPG-managed pods via inheritedMetadata
		// so same-workspace NetworkPolicy selectors can reach the database.
		if ws := db.Labels[labelWorkspace]; ws != "" {
			spec["inheritedMetadata"] = map[string]any{
				"labels": map[string]any{labelWorkspace: ws},
			}
		}
		cluster.Object["spec"] = spec
		setLifecycleAnnotations(cluster, db.Spec.Suspended, db.Spec.RestartedAt)
		return controllerutil.SetControllerReference(&db, cluster, r.Scheme)
	}); err != nil {
		return r.dbFail(ctx, &db, "ClusterFailed", err)
	}

	// CNPG generates Secret "<cluster>-app" (username/password/dbname/host/port/uri)
	// and Service "<cluster>-rw". The internal Database URL is that Secret's "uri".
	db.Status.Host = fmt.Sprintf("%s-rw.%s.svc", db.Name, db.Namespace)
	db.Status.Port = pgPort
	db.Status.SecretName = db.Name + "-app"
	db.Status.ObservedGeneration = db.Generation
	db.Status.BackupsEnabled = backupEnabled

	// --- daily base backup (ScheduledBackup) when backups are enabled ---
	if backupEnabled {
		if err := r.upsertOwned(ctx, &db, cnpgScheduledBackupGVK, db.Name+"-backup", scheduledBackupSpec(db.Name)); err != nil {
			return r.dbFail(ctx, &db, "ScheduledBackupFailed", err)
		}
	} else if err := r.deleteOwned(ctx, &db, cnpgScheduledBackupGVK, db.Name+"-backup"); err != nil {
		return r.dbFail(ctx, &db, "ScheduledBackupCleanupFailed", err)
	}

	// --- PgBouncer Pooler when requested ---
	if db.Spec.Pooler {
		if err := r.upsertOwned(ctx, &db, cnpgPoolerGVK, db.Name+"-pooler", poolerSpec(db.Name)); err != nil {
			return r.dbFail(ctx, &db, "PoolerFailed", err)
		}
		db.Status.PoolerHost = fmt.Sprintf("%s-pooler.%s.svc", db.Name, db.Namespace)
	} else {
		if err := r.deleteOwned(ctx, &db, cnpgPoolerGVK, db.Name+"-pooler"); err != nil {
			return r.dbFail(ctx, &db, "PoolerCleanupFailed", err)
		}
		db.Status.PoolerHost = ""
	}

	// --- external SNI endpoints (rw + pooler) + IP-allowlist middleware ---
	if err := r.reconcileExternalRoutes(ctx, &db); err != nil {
		return r.dbFail(ctx, &db, "RouteFailed", err)
	}

	// Suspended (hibernated) settles immediately — CNPG stops compute, so waiting
	// on readyInstances would never converge (mirrors the KeyValue suspend path).
	if db.Spec.Suspended {
		db.Status.Phase = appv1alpha1.DBPhaseReady
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "postgres suspended (hibernated; PVC and config kept)", ObservedGeneration: db.Generation,
		})
		if err := r.Status().Update(ctx, &db); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ready when CNPG reports enough ready instances.
	ready := int64(0)
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err == nil {
		ready, _, _ = unstructured.NestedInt64(cluster.Object, "status", "readyInstances")
	}
	if ready >= int64(plan.Instances) {
		db.Status.Phase = appv1alpha1.DBPhaseReady
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionTrue, Reason: "Provisioned",
			Message: "postgres ready", ObservedGeneration: db.Generation,
		})
		if err := r.Status().Update(ctx, &db); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("database ready", "name", db.Name, "host", db.Status.Host)
		return ctrl.Result{}, nil
	}

	db.Status.Phase = appv1alpha1.DBPhaseProvisioning
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: "Provisioning",
		Message: "waiting for CloudNativePG", ObservedGeneration: db.Generation,
	})
	if err := r.Status().Update(ctx, &db); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
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

// reconcileExternalRoutes projects the public SNI endpoints when the Database is
// Public and a DBDomain is set: the read-write route (<name>.<domain>) and, when
// pooling is on, a pooled route (<name>-pool.<domain>), both gated by an
// ipAllowList middleware when spec.ipAllowList is non-empty. Everything is
// removed when the Database is private (or no domain). Sets the external status
// hosts. The internal -rw path is never gated.
func (r *DatabaseReconciler) reconcileExternalRoutes(ctx context.Context, db *appv1alpha1.Database) error {
	mwName := db.Name + "-allow"
	public := db.Spec.Public && r.DBDomain != ""

	if !public {
		db.Status.ExternalHost = ""
		db.Status.PoolerExternalHost = ""
		for _, name := range []string{db.Name + "-pg", db.Name + "-pool"} {
			if err := r.deleteOwned(ctx, db, traefikIngressRouteTCPGVK, name); err != nil {
				return err
			}
		}
		return r.deleteOwned(ctx, db, traefikMiddlewareTCPGVK, mwName)
	}

	// IP allowlist: a middleware referenced by both routes when CIDRs are set.
	var middlewares []any
	if len(db.Spec.IPAllowList) > 0 {
		if err := r.upsertOwned(ctx, db, traefikMiddlewareTCPGVK, mwName, ipAllowListMiddlewareSpec(db.Spec.IPAllowList)); err != nil {
			return err
		}
		middlewares = []any{map[string]any{"name": mwName, "namespace": db.Namespace}}
	} else if err := r.deleteOwned(ctx, db, traefikMiddlewareTCPGVK, mwName); err != nil {
		return err
	}

	routeSpec := func(host, service string) map[string]any {
		return ingressRouteTCPSpec(pgEntryPoint, host, service, pgPort, middlewares)
	}

	rwHost := fmt.Sprintf("%s.%s", db.Name, r.DBDomain)
	if err := r.upsertOwned(ctx, db, traefikIngressRouteTCPGVK, db.Name+"-pg", routeSpec(rwHost, db.Name+"-rw")); err != nil {
		return err
	}
	db.Status.ExternalHost = rwHost

	// Pooled external endpoint (only when pooling is on).
	if db.Spec.Pooler {
		poolHost := fmt.Sprintf("%s-pool.%s", db.Name, r.DBDomain)
		if err := r.upsertOwned(ctx, db, traefikIngressRouteTCPGVK, db.Name+"-pool", routeSpec(poolHost, db.Name+"-pooler")); err != nil {
			return err
		}
		db.Status.PoolerExternalHost = poolHost
	} else {
		if err := r.deleteOwned(ctx, db, traefikIngressRouteTCPGVK, db.Name+"-pool"); err != nil {
			return err
		}
		db.Status.PoolerExternalHost = ""
	}
	return nil
}

func (r *DatabaseReconciler) dbFail(ctx context.Context, db *appv1alpha1.Database, reason string, err error) (ctrl.Result, error) {
	db.Status.Phase = appv1alpha1.DBPhaseFailed
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: err.Error(),
		ObservedGeneration: db.Generation,
	})
	_ = r.Status().Update(ctx, db)
	return ctrl.Result{}, err
}

// SetupWithManager wires the controller. It owns the CNPG Cluster (unstructured)
// so deletes cascade and Cluster changes re-trigger reconcile.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	owned := &unstructured.Unstructured{}
	owned.SetGroupVersionKind(cnpgClusterGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.Database{}).
		Owns(owned).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("database").
		Complete(r)
}
