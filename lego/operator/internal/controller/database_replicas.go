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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Read replicas (w6/m467, ADR009 § Per-replica readers and pools).
//
// Each named replica is a one-instance CNPG replica cluster
// (appv1alpha1.ReadReplicaClusterName) that streams from the primary's -rw
// Service, with a CNPG-managed Service "<db>-ro-<name>" on its designated
// primary and, when the database is pooled, a CNPG Pooler "<db>-rp-<name>"
// sized like the primary's. So each replica has its own instance, URLs and
// pool of max_connections - 10, as each Render replica does, and none is ever
// the primary's failover target: the primary Cluster holds only the HA
// standby. Adding or removing a replica creates or deletes its own objects
// and touches neither the primary Cluster nor another replica.
const (
	// labelReadReplicaOf marks a replica cluster and its Pooler with the
	// Database they serve, so a removed replica's objects are found and
	// deleted.
	labelReadReplicaOf = "app.bex.co/read-replica-of"
	// annotationReplicaMajor records the PostgreSQL major a replica cluster
	// was cloned at. A physical standby cannot cross a major, so a replica
	// left behind by the primary's upgrade is cloned again.
	annotationReplicaMajor = "app.bex.co/postgres-major"
	// readReplicaSourceName names the primary in a replica cluster's
	// externalClusters, bootstrap and replica stanzas.
	readReplicaSourceName = "primary"
)

// readReplicasProvisioned reports whether a Database's declared replicas get
// instances: a plan with the CPU for them, a count within its limit, and
// storage within the replica bounds. A legacy declaration outside that policy
// keeps its names but provisions nothing.
func readReplicasProvisioned(plan tiers.PostgresTier, storageGB int32, replicas int) bool {
	return replicas > 0 && replicas <= plan.MaxReadReplicas() &&
		storageGB >= tiers.PostgresReadReplicaMinStorageGB &&
		storageGB <= tiers.Postgres.MaxStorageGB(plan.ID)
}

// readReplicaState is what one reconcile resolved about a Database's replicas.
type readReplicaState struct {
	// provisioned: the declared replicas get instances.
	provisioned bool
	// ready: every declared replica that should have an instance has a ready
	// one (true when none should).
	ready bool
}

// readReplicaSource is what a replica cluster copies from its primary.
type readReplicaSource struct {
	// settled: the primary serves (a ready instance) and runs the major its
	// spec names, so a replica cloned from it now starts. A primary that is
	// provisioning, hibernated or mid-upgrade leaves its replicas as they are.
	settled bool
	// major is the PostgreSQL major the primary's data runs.
	major string
	// spec is the primary Cluster's live spec.
	spec map[string]any
}

func readReplicaSourceOf(db *appv1alpha1.Database, primary *unstructured.Unstructured) readReplicaSource {
	spec, _, _ := unstructured.NestedMap(primary.Object, "spec")
	ready, _, _ := unstructured.NestedInt64(primary.Object, "status", "readyInstances")
	major, _, _ := unstructured.NestedInt64(primary.Object, "status", "pgDataImageInfo", "majorVersion")
	phase, _, _ := unstructured.NestedString(primary.Object, "status", "phase")
	src := readReplicaSource{spec: spec}
	if major > 0 {
		src.major = strconv.FormatInt(major, 10)
	}
	src.settled = ready > 0 && src.major != "" && !db.Spec.Suspended &&
		!(cnpgClusterState{phase: phase}).majorUpgradeRunning() &&
		!majorUpgradePending(db.Spec.Version, src.major)
	return src
}

// readReplicaPublicNames are the public SNI names a replica serves: its
// direct host and, when pooled, its PgBouncer's.
func readReplicaPublicNames(db *appv1alpha1.Database, domain, replica string) []string {
	if !db.Spec.Public || domain == "" {
		return nil
	}
	names := []string{appv1alpha1.ReadReplicaServiceName(db.Name, replica) + "." + domain}
	if db.Spec.Pooler {
		names = append(names, appv1alpha1.ReadReplicaPoolerName(db.Name, replica)+"."+domain)
	}
	return names
}

// readReplicaClusterSpec is the CNPG Cluster .spec of one replica: a single
// instance with the primary's image, plan, storage, PostgreSQL settings and
// labels, cloned with pg_basebackup and streaming from the primary's -rw
// Service as streaming_replica, with the primary's replication client
// certificate, verifying the primary's server CA (CNPG's own replication
// uses verify-ca: database_public_tls.go).
//
// Its server certificate chains to the root clients already trust for the
// primary: a public database's issued certificate, which names every replica
// (databaseServerAltDNSNames), or else one CNPG signs with the primary's own
// CA, the one bex-api serves (serverCACertificate), naming the replica's
// public hosts. Its direct Service is CNPG's, on the designated primary.
func readReplicaClusterSpec(db *appv1alpha1.Database, replica string, primary map[string]any, publicNames []string) map[string]any {
	serverCA := db.Name + "-ca"
	certificates := map[string]any{"serverCASecret": serverCA}
	if tlsSecret, _, _ := unstructured.NestedString(primary, "certificates", "serverTLSSecret"); tlsSecret != "" {
		serverCA, _, _ = unstructured.NestedString(primary, "certificates", "serverCASecret")
		certificates = map[string]any{"serverTLSSecret": tlsSecret, "serverCASecret": serverCA}
	} else if len(publicNames) > 0 {
		certificates["serverAltDNSNames"] = stringsAsAny(publicNames)
	}
	replication := db.Name + "-replication"
	spec := map[string]any{
		"instances": int64(1),
		// One instance: no disruption budget, and a readiness check load
		// cannot time out, as a single-instance primary (cnpgClusterSpec).
		"enablePDB":         false,
		"probes":            map[string]any{"readiness": singleInstanceReadiness()},
		"priorityClassName": appv1alpha1.DatastorePriorityClassName,
		"certificates":      certificates,
		"bootstrap":         map[string]any{"pg_basebackup": map[string]any{"source": readReplicaSourceName}},
		"replica":           map[string]any{"enabled": true, "source": readReplicaSourceName},
		"externalClusters": []any{map[string]any{
			"name": readReplicaSourceName,
			"connectionParameters": map[string]any{
				"host":    fmt.Sprintf("%s-rw.%s.svc", db.Name, db.Namespace),
				"user":    "streaming_replica",
				"dbname":  "postgres",
				"sslmode": "verify-ca",
			},
			"sslKey":      map[string]any{"name": replication, "key": corev1.TLSPrivateKeyKey},
			"sslCert":     map[string]any{"name": replication, "key": corev1.TLSCertKey},
			"sslRootCert": map[string]any{"name": serverCA, "key": "ca.crt"},
		}},
		// Away from the primary's node where there is room, so a replica's
		// reads do not take the primary's CPU.
		"affinity": map[string]any{"additionalPodAntiAffinity": map[string]any{
			"preferredDuringSchedulingIgnoredDuringExecution": []any{map[string]any{
				"weight": int64(100),
				"podAffinityTerm": map[string]any{
					"labelSelector": map[string]any{"matchLabels": map[string]any{cnpgClusterLabel: db.Name}},
					"topologyKey":   corev1.LabelHostname,
				},
			}},
		}},
		"managed": map[string]any{"services": map[string]any{"additional": []any{map[string]any{
			"selectorType": cnpgReadWrite,
			// CNPG's defaults, spelled out: the list is replaced whole on every
			// projection, so a defaulted field would be a write each pass.
			"updateStrategy": "patch",
			"serviceTemplate": map[string]any{
				"metadata": map[string]any{"name": appv1alpha1.ReadReplicaServiceName(db.Name, replica)},
				"spec":     map[string]any{},
			},
		}}}},
	}
	for _, key := range []string{"imageName", "storage", "resources", "inheritedMetadata"} {
		if value, ok := primary[key]; ok {
			spec[key] = runtime.DeepCopyJSONValue(value)
		}
	}
	// A hot standby needs the primary's settings: max_connections and its
	// siblings no lower, and the same preloaded libraries for the same image.
	postgresql := map[string]any{}
	for _, key := range []string{"parameters", "shared_preload_libraries"} {
		if value, ok, _ := unstructured.NestedFieldCopy(primary, "postgresql", key); ok {
			postgresql[key] = value
		}
	}
	if len(postgresql) > 0 {
		spec["postgresql"] = postgresql
	}
	return spec
}

// reconcileReadReplicas creates, updates and deletes the Database's replica
// clusters and their Poolers. primary is the live primary Cluster.
func (r *DatabaseReconciler) reconcileReadReplicas(ctx context.Context, db *appv1alpha1.Database, primary *unstructured.Unstructured, plan tiers.PostgresTier, storageGB int32) (readReplicaState, error) {
	state := readReplicaState{provisioned: readReplicasProvisioned(plan, storageGB, len(db.Spec.ReadReplicas)), ready: true}
	clusters, poolers := map[string]bool{}, map[string]bool{}
	if state.provisioned {
		src := readReplicaSourceOf(db, primary)
		// A public database's replicas serve the primary's one certificate.
		digests := map[string]string{}
		for _, replica := range db.Spec.ReadReplicas {
			cluster, ready, err := r.reconcileReadReplicaCluster(ctx, db, replica.Name, src)
			if err != nil {
				return state, fmt.Errorf("read replica %q: %w", replica.Name, err)
			}
			clusters[cluster.GetName()] = true
			state.ready = state.ready && ready
			if !db.Spec.Pooler {
				continue
			}
			name := appv1alpha1.ReadReplicaPoolerName(db.Name, replica.Name)
			poolers[name] = true
			if cluster.GetResourceVersion() == "" {
				continue // not cloned yet: its PgBouncer follows it
			}
			served, _, _ := unstructured.NestedString(cluster.Object, "spec", "certificates", "serverTLSSecret")
			digest, read := digests[served]
			if !read {
				if digest, err = r.servedCertificateDigest(ctx, db.Namespace, cluster); err != nil {
					return state, fmt.Errorf("read replica %q: read served certificate: %w", replica.Name, err)
				}
				digests[served] = digest
			}
			if err := r.upsertPooler(ctx, db, cluster, name, plan, digest, map[string]string{labelReadReplicaOf: db.Name}); err != nil {
				return state, fmt.Errorf("read replica %q: pooler: %w", replica.Name, err)
			}
		}
	}
	if !readReplicaStatusesStale(db, state.provisioned) {
		return state, nil
	}
	// Remove what no declared replica owns any more.
	if err := r.pruneReadReplicaObjects(ctx, db, cnpgPoolerGVK, poolers); err != nil {
		return state, fmt.Errorf("remove read replica pooler: %w", err)
	}
	if err := r.pruneReadReplicaObjects(ctx, db, cnpgClusterGVK, clusters); err != nil {
		return state, fmt.Errorf("remove read replica: %w", err)
	}
	return state, nil
}

// readReplicaStatusesStale reports whether the replica statuses the last pass
// saved differ from what this spec asks for: another set of names, pooling or
// provisioning. Only then can a replica's cluster or pooler be left over to
// remove, so a database whose replicas are unchanged lists nothing. The
// statuses are rewritten only after the removal succeeds
// (updateExternalAddressStatus), so a failed one is retried.
func readReplicaStatusesStale(db *appv1alpha1.Database, provisioned bool) bool {
	if len(db.Status.ReadReplicaStatuses) != len(db.Spec.ReadReplicas) {
		return true
	}
	for i, status := range db.Status.ReadReplicaStatuses {
		if status.Name != db.Spec.ReadReplicas[i].Name || (status.InternalHost != "") != provisioned ||
			(status.PoolerHost != "") != (provisioned && db.Spec.Pooler) {
			return true
		}
	}
	return false
}

// reconcileReadReplicaCluster converges one replica cluster and reports it
// and whether its instance is ready. Its spec follows the primary only while
// the primary is settled, so a new replica is cloned from a primary that is
// up, and an upgrade in flight never reaches the replicas. Once the primary
// runs a major the replica was not cloned at, the replica is deleted and
// cloned again on a later pass. Hibernation and restarts follow the
// Database's at all times.
func (r *DatabaseReconciler) reconcileReadReplicaCluster(ctx context.Context, db *appv1alpha1.Database, replica string, src readReplicaSource) (*unstructured.Unstructured, bool, error) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(appv1alpha1.ReadReplicaClusterName(db.Name, replica))
	cluster.SetNamespace(db.Namespace)
	exists := true
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); apierrors.IsNotFound(err) {
		exists = false
	} else if err != nil {
		return cluster, false, err
	}
	switch {
	case exists && !cluster.GetDeletionTimestamp().IsZero():
		return cluster, false, nil
	case exists && src.settled && cluster.GetAnnotations()[annotationReplicaMajor] != src.major:
		return cluster, false, deleteTolerantly(ctx, r.Client, cluster)
	case !exists && !src.settled:
		return cluster, false, nil
	}
	// The cluster is loaded already, so write it directly rather than through
	// CreateOrUpdate, which would read it again.
	live := cluster.DeepCopy()
	if src.settled {
		spec := readReplicaClusterSpec(db, replica, src.spec, readReplicaPublicNames(db, r.DBDomain, replica))
		if err := projectUnstructuredSpec(cluster, spec); err != nil {
			return cluster, false, err
		}
		cluster.SetAnnotations(withEntry(cluster.GetAnnotations(), annotationReplicaMajor, src.major))
	}
	setLifecycleAnnotations(cluster, db.Spec.Suspended, db.Spec.RestartedAt)
	cluster.SetLabels(withEntry(cluster.GetLabels(), labelReadReplicaOf, db.Name))
	if err := controllerutil.SetControllerReference(db, cluster, r.Scheme); err != nil {
		return cluster, false, err
	}
	var err error
	switch {
	case !exists:
		err = r.Create(ctx, cluster)
	case !equality.Semantic.DeepEqual(live.Object, cluster.Object):
		err = r.Update(ctx, cluster)
	}
	if err != nil {
		return cluster, false, err
	}
	ready, _, _ := unstructured.NestedInt64(cluster.Object, "status", "readyInstances")
	return cluster, ready > 0, nil
}

// withEntry is m with key set to value, allocating m when it is nil.
func withEntry(m map[string]string, key, value string) map[string]string {
	if m == nil {
		m = map[string]string{}
	}
	m[key] = value
	return m
}

// pruneReadReplicaObjects deletes the Database's replica objects of gvk whose
// names keep does not hold.
func (r *DatabaseReconciler) pruneReadReplicaObjects(ctx context.Context, db *appv1alpha1.Database, gvk schema.GroupVersionKind, keep map[string]bool) error {
	items, err := r.listReadReplicaObjects(ctx, db, gvk)
	if err != nil {
		return err
	}
	for i := range items {
		if keep[items[i].GetName()] {
			continue
		}
		if err := deleteTolerantly(ctx, r.Client, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

// listReadReplicaObjects lists the replica objects of gvk the Database
// controls. Only a database that has or had replicas can own one, so the
// others list nothing: a removed replica stays in the saved statuses until
// its objects are gone (readReplicaStatusesStale).
func (r *DatabaseReconciler) listReadReplicaObjects(ctx context.Context, db *appv1alpha1.Database, gvk schema.GroupVersionKind) ([]unstructured.Unstructured, error) {
	if len(db.Spec.ReadReplicas) == 0 && len(db.Status.ReadReplicaStatuses) == 0 {
		return nil, nil
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := r.List(ctx, list, client.InNamespace(db.Namespace), client.MatchingLabels{labelReadReplicaOf: db.Name}); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}
	owned := list.Items[:0]
	for _, item := range list.Items {
		if metav1.IsControlledBy(&item, db) {
			owned = append(owned, item)
		}
	}
	return owned, nil
}
