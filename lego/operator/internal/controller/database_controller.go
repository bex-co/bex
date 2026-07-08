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

const (
	dbStorageClass = "hcloud-volumes"
	// pgEntryPoint is the Traefik TCP entrypoint bound to :5432 (traefik values).
	pgEntryPoint = "postgres"
)

// normalizeIdent turns an App/DB name into a valid unquoted PostgreSQL
// identifier: lowercase, hyphens -> underscores. So a client never has to
// double-quote the db/role name (docs/postgresql-management.md §4).
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
	storageGB := spec.StorageGB
	if storageGB < plan.StorageGB {
		storageGB = plan.StorageGB
	}
	return plan, storageGB
}

// cnpgClusterSpec builds the CloudNativePG Cluster .spec for a Database. Pure
// (no client) so the plan->Cluster projection is unit-testable.
func cnpgClusterSpec(plan tiers.PostgresTier, storageGB int32, version, dbname, owner string) map[string]any {
	spec := map[string]any{
		"instances": int64(plan.Instances),
		"storage": map[string]any{
			"size":         fmt.Sprintf("%dGi", storageGB),
			"storageClass": dbStorageClass,
		},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": plan.CPU, "memory": plan.Memory},
			"limits":   map[string]any{"cpu": plan.CPU, "memory": plan.Memory},
		},
		"bootstrap": map[string]any{
			"initdb": map[string]any{"database": dbname, "owner": owner},
		},
	}
	if version != "" {
		spec["imageName"] = "ghcr.io/cloudnative-pg/postgresql:" + version
	}
	return spec
}

// ingressRouteTCPSpec builds a Traefik IngressRouteTCP .spec routing the SNI
// hostname to the CNPG read-write Service, TLS-passthrough (Postgres speaks its
// own TLS). Pure so the external-route projection is unit-testable.
func ingressRouteTCPSpec(host, serviceName string) map[string]any {
	return map[string]any{
		"entryPoints": []any{pgEntryPoint},
		"routes": []any{map[string]any{
			"match": fmt.Sprintf("HostSNI(`%s`)", host),
			"services": []any{map[string]any{
				"name": serviceName,
				"port": int64(5432),
			}},
		}},
		"tls": map[string]any{"passthrough": true},
	}
}

// DatabaseReconciler projects a Database into a CloudNativePG Cluster and
// surfaces the connection info. Optionally exposes an external SNI endpoint via
// Traefik. It is a thin executor — CNPG does the actual Postgres lifecycle.
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// DBDomain is the wildcard base for external endpoints, e.g. "db.bex.co".
	// Empty => Public databases get no external route.
	DBDomain string
}

// +kubebuilder:rbac:groups=app.bex.co,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutetcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var db appv1alpha1.Database
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Deletion: the CNPG Cluster is owned by the Database, so it (and its pods,
	// Service, PVC, generated Secret) is garbage-collected automatically.
	if !db.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	plan, storageGB := resolvePlan(db.Spec)
	dbname := normalizeIdent(db.Name)
	owner := dbname + "_user"

	// --- project onto a CNPG Cluster (same name, same namespace) ---
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(db.Name)
	cluster.SetNamespace(db.Namespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cluster, func() error {
		cluster.Object["spec"] = cnpgClusterSpec(plan, storageGB, db.Spec.Version, dbname, owner)
		return controllerutil.SetControllerReference(&db, cluster, r.Scheme)
	}); err != nil {
		return r.dbFail(ctx, &db, "ClusterFailed", err)
	}

	// CNPG generates Secret "<cluster>-app" (username/password/dbname/host/port/uri)
	// and Service "<cluster>-rw". The internal Database URL is that Secret's "uri".
	db.Status.Host = fmt.Sprintf("%s-rw.%s.svc", db.Name, db.Namespace)
	db.Status.Port = 5432
	db.Status.SecretName = db.Name + "-app"
	db.Status.ObservedGeneration = db.Generation

	// --- optional external SNI endpoint via Traefik ---
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(traefikIngressRouteTCPGVK)
	route.SetName(db.Name + "-pg")
	route.SetNamespace(db.Namespace)
	if db.Spec.Public && r.DBDomain != "" {
		externalHost := fmt.Sprintf("%s.%s", db.Name, r.DBDomain)
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
			route.Object["spec"] = ingressRouteTCPSpec(externalHost, db.Name+"-rw")
			return controllerutil.SetControllerReference(&db, route, r.Scheme)
		}); err != nil {
			return r.dbFail(ctx, &db, "RouteFailed", err)
		}
		db.Status.ExternalHost = externalHost
	} else {
		// Not public (or no base domain): remove any route we previously made.
		if err := r.Delete(ctx, route); err != nil && !apierrors.IsNotFound(err) {
			return r.dbFail(ctx, &db, "RouteCleanupFailed", err)
		}
		db.Status.ExternalHost = ""
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
