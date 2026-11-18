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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const replicaDB = "dpg-d4replicastest000000"

// replicaPrimary is a live primary Cluster of a basic-1gb Database, projected
// by cnpgClusterSpec, running PostgreSQL major with ready instances.
func replicaPrimary(t *testing.T, db *appv1alpha1.Database, ready int64, major int64) *unstructured.Unstructured {
	t.Helper()
	plan, _ := resolvePlan(db.Spec)
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 10, version: "18", dbname: "app", owner: "app",
		highAvailability: db.Spec.HighAvailability, serverAltDNSNames: databaseServerAltDNSNames(db, "db.example")})
	cluster := &unstructured.Unstructured{Object: map[string]any{"spec": spec, "status": map[string]any{
		"readyInstances": ready, "phase": "Cluster in healthy state",
		"pgDataImageInfo": map[string]any{"majorVersion": major},
	}}}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(db.Name)
	cluster.SetNamespace(db.Namespace)
	return cluster
}

func replicaFixture(t *testing.T, db *appv1alpha1.Database) (*DatabaseReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, gvk := range []schema.GroupVersionKind{cnpgClusterGVK, cnpgPoolerGVK} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	}
	withStatus := &unstructured.Unstructured{}
	withStatus.SetGroupVersionKind(cnpgClusterGVK)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).WithStatusSubresource(withStatus).Build()
	return &DatabaseReconciler{Client: cl, Scheme: scheme, DBDomain: "db.example"}, cl
}

func liveObject(t *testing.T, cl client.Client, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, bool) {
	t.Helper()
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(gvk)
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, o)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return o, true
}

// TestReadReplicaClusterSpec (w6/m467): a replica is one instance of its own
// that streams from the primary, with the primary's image, plan, storage and
// settings, its own Service, and a certificate that chains to the root its
// clients already trust for the primary.
func TestReadReplicaClusterSpec(t *testing.T) {
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: replicaDB, Namespace: "tea-a"},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Public: true, Pooler: true, HighAvailability: true,
			ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}}}}
	primary, _, _ := unstructured.NestedMap(replicaPrimary(t, db, 2, 18).Object, "spec")
	public := readReplicaPublicNames(db, "db.example", "a")
	if want := []string{replicaDB + "-ro-a.db.example", replicaDB + "-rp-a.db.example"}; !reflect.DeepEqual(public, want) {
		t.Fatalf("replica public names = %v, want %v", public, want)
	}
	spec := readReplicaClusterSpec(db, "a", primary, public)

	for _, tc := range []struct {
		path []string
		want any
	}{
		{[]string{"instances"}, int64(1)},
		{[]string{"enablePDB"}, false},
		{[]string{"replica", "enabled"}, true},
		{[]string{"replica", "source"}, readReplicaSourceName},
		{[]string{"bootstrap", "pg_basebackup", "source"}, readReplicaSourceName},
		{[]string{"priorityClassName"}, appv1alpha1.DatastorePriorityClassName},
		{[]string{"imageName"}, primary["imageName"]},
		{[]string{"storage"}, primary["storage"]},
		{[]string{"resources"}, primary["resources"]},
		{[]string{"postgresql", "parameters", "max_connections"}, "100"},
		{[]string{"certificates", "serverCASecret"}, replicaDB + "-ca"},
		{[]string{"certificates", "serverAltDNSNames"}, stringsAsAny(public)},
	} {
		got, _, _ := unstructured.NestedFieldNoCopy(spec, tc.path...)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v = %#v, want %#v", tc.path, got, tc.want)
		}
	}
	if _, has := spec["plugins"]; has {
		t.Error("replica spec carries the primary's WAL archiver")
	}
	if _, has, _ := unstructured.NestedFieldNoCopy(spec, "managed", "roles"); has {
		t.Error("replica spec carries the primary's managed roles, which a replica cannot apply")
	}
	if _, has, _ := unstructured.NestedFieldNoCopy(spec, "affinity", "enablePodAntiAffinity"); has {
		t.Error("replica spec carries the primary's HA affinity")
	}
	away, _, _ := unstructured.NestedSlice(spec, "affinity", "additionalPodAntiAffinity", "preferredDuringSchedulingIgnoredDuringExecution")
	if len(away) != 1 {
		t.Fatalf("replica anti-affinity = %v", away)
	}
	if got, _, _ := unstructured.NestedString(away[0].(map[string]any), "podAffinityTerm", "labelSelector", "matchLabels", cnpgClusterLabel); got != replicaDB {
		t.Errorf("replica prefers a node away from %q, want the primary's %q", got, replicaDB)
	}
	preload, _, _ := unstructured.NestedFieldNoCopy(spec, "postgresql", "shared_preload_libraries")
	if want, _, _ := unstructured.NestedFieldNoCopy(primary, "postgresql", "shared_preload_libraries"); !reflect.DeepEqual(preload, want) {
		t.Errorf("shared_preload_libraries = %v, want the primary's %v", preload, want)
	}
	sources, _, _ := unstructured.NestedSlice(spec, "externalClusters")
	wantSource := map[string]any{
		"name": readReplicaSourceName,
		"connectionParameters": map[string]any{
			"host": replicaDB + "-rw.tea-a.svc", "user": "streaming_replica", "dbname": "postgres", "sslmode": "verify-ca",
		},
		"sslKey":      map[string]any{"name": replicaDB + "-replication", "key": "tls.key"},
		"sslCert":     map[string]any{"name": replicaDB + "-replication", "key": "tls.crt"},
		"sslRootCert": map[string]any{"name": replicaDB + "-ca", "key": "ca.crt"},
	}
	if len(sources) != 1 || !reflect.DeepEqual(sources[0], wantSource) {
		t.Errorf("externalClusters = %#v, want %#v", sources, wantSource)
	}
	services, _, _ := unstructured.NestedSlice(spec, "managed", "services", "additional")
	if len(services) != 1 {
		t.Fatalf("additional services = %v", services)
	}
	service := services[0].(map[string]any)
	name, _, _ := unstructured.NestedString(service, "serviceTemplate", "metadata", "name")
	if service["selectorType"] != "rw" || name != replicaDB+"-ro-a" {
		t.Errorf("replica Service = %v, want %s on the designated primary", service, replicaDB+"-ro-a")
	}

	// A public database's issued certificate names every replica, so the
	// replica serves it, and streams verifying that certificate's root.
	if err := unstructured.SetNestedField(primary, map[string]any{
		"serverTLSSecret": databasePublicTLSName(db), "serverCASecret": databasePublicCAName(db),
	}, "certificates"); err != nil {
		t.Fatal(err)
	}
	spec = readReplicaClusterSpec(db, "a", primary, public)
	if got := spec["certificates"]; !reflect.DeepEqual(got, map[string]any{
		"serverTLSSecret": databasePublicTLSName(db), "serverCASecret": databasePublicCAName(db),
	}) {
		t.Errorf("certificates with an issued certificate = %v", got)
	}
	if got, _, _ := unstructured.NestedString(spec, "externalClusters"); got != "" {
		t.Fatal("externalClusters is not a list")
	}
	sources, _, _ = unstructured.NestedSlice(spec, "externalClusters")
	if root, _, _ := unstructured.NestedString(sources[0].(map[string]any), "sslRootCert", "name"); root != databasePublicCAName(db) {
		t.Errorf("streaming verifies %q, want the issued certificate's root %q", root, databasePublicCAName(db))
	}
}

// TestReadReplicaSourceSettles: a replica is cloned only from a primary that
// serves and runs the major its spec names.
func TestReadReplicaSourceSettles(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ready     int64
		major     int64
		phase     string
		version   string
		suspended bool
		want      bool
	}{
		{"serving", 1, 18, "Cluster in healthy state", "18", false, true},
		{"provisioning", 0, 18, "Setting up primary", "18", false, false},
		{"major unknown", 1, 0, "Cluster in healthy state", "", false, false},
		{"upgrade running", 1, 17, "Upgrading Postgres major version", "18", false, false},
		{"upgrade accepted, not started", 1, 17, "Cluster in healthy state", "18", false, false},
		{"suspended", 1, 18, "Cluster in healthy state", "18", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Version: tc.version, Suspended: tc.suspended}}
			primary := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{
				"readyInstances": tc.ready, "phase": tc.phase, "pgDataImageInfo": map[string]any{"majorVersion": tc.major},
			}}}
			if got := readReplicaSourceOf(db, primary); got.settled != tc.want {
				t.Fatalf("settled = %v, want %v", got.settled, tc.want)
			}
		})
	}
}

// TestEachReadReplicaHasItsOwnClusterAndPooler (w6/m467): two pooled replicas
// get two replica clusters and two PgBouncers, each pool max_connections - 10
// on its own replica; a plan change rewrites both pools without rolling
// either; removing one replica leaves the other's objects untouched; turning
// pooling off removes the replicas' poolers.
func TestEachReadReplicaHasItsOwnClusterAndPooler(t *testing.T) {
	ctx := context.Background()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: replicaDB, Namespace: "tea-a", UID: "db-uid"},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, Pooler: true, HighAvailability: true,
			ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}, {Name: "b"}}},
	}
	r, cl := replicaFixture(t, db)
	primary := replicaPrimary(t, db, 2, 18)
	plan, _ := resolvePlan(db.Spec)
	reconcile := func(plan tiers.PostgresTier) readReplicaState {
		t.Helper()
		state, err := r.reconcileReadReplicas(ctx, db, primary, plan, 10)
		if err != nil {
			t.Fatal(err)
		}
		r.updateExternalAddressStatus(db, state.provisioned)
		return state
	}

	if state := reconcile(plan); !state.provisioned || state.ready {
		t.Fatalf("state = %+v, want provisioned replicas not ready yet", state)
	}
	clusters := map[string]*unstructured.Unstructured{}
	for _, name := range []string{"a", "b"} {
		clusters[name] = assertOwnReplicaObjects(t, cl, db, name)
	}
	for i, status := range db.Status.ReadReplicaStatuses {
		name := db.Spec.ReadReplicas[i].Name
		if status.InternalHost != appv1alpha1.ReadReplicaServiceName(db.Name, name)+".tea-a.svc" ||
			status.PoolerHost != appv1alpha1.ReadReplicaPoolerName(db.Name, name)+".tea-a.svc" {
			t.Errorf("replica %s status = %+v, want its own hosts", name, status)
		}
	}

	// Ready once each replica's own instance is.
	for _, cluster := range clusters {
		if err := unstructured.SetNestedField(cluster.Object, int64(1), "status", "readyInstances"); err != nil {
			t.Fatal(err)
		}
		if err := cl.Status().Update(ctx, cluster); err != nil {
			t.Fatal(err)
		}
	}
	if state := reconcile(plan); !state.ready {
		t.Fatal("ready replica clusters did not make the replicas ready")
	}
	// A pass with nothing to change writes nothing.
	versions := map[string]string{}
	for _, gvk := range []schema.GroupVersionKind{cnpgClusterGVK, cnpgPoolerGVK} {
		for _, name := range []string{appv1alpha1.ReadReplicaClusterName(db.Name, "a"), appv1alpha1.ReadReplicaPoolerName(db.Name, "a")} {
			if live, ok := liveObject(t, cl, gvk, db.Namespace, name); ok {
				versions[gvk.Kind+"/"+name] = live.GetResourceVersion()
			}
		}
	}
	reconcile(plan)
	for key, version := range versions {
		kind, name, _ := strings.Cut(key, "/")
		live, _ := liveObject(t, cl, cnpgClusterGVK.GroupVersion().WithKind(kind), db.Namespace, name)
		if live.GetResourceVersion() != version {
			t.Errorf("an unchanged pass rewrote %s", key)
		}
	}

	// A plan with another connection limit (w6/m332's 200) rewrites both
	// pools and only their parameters: the pod templates, whose change would
	// roll PgBouncer, stay.
	templates := map[string]any{}
	for _, name := range []string{"a", "b"} {
		pooler, _ := liveObject(t, cl, cnpgPoolerGVK, db.Namespace, appv1alpha1.ReadReplicaPoolerName(db.Name, name))
		templates[name], _, _ = unstructured.NestedMap(pooler.Object, "spec", "template")
	}
	bigger := plan
	bigger.MaxConnections = 200
	reconcile(bigger)
	for _, name := range []string{"a", "b"} {
		pooler, _ := liveObject(t, cl, cnpgPoolerGVK, db.Namespace, appv1alpha1.ReadReplicaPoolerName(db.Name, name))
		if got, _, _ := unstructured.NestedString(pooler.Object, "spec", "pgbouncer", "parameters", "max_db_connections"); got != "190" {
			t.Errorf("replica %s pool after the plan change = %q, want 190", name, got)
		}
		if after, _, _ := unstructured.NestedMap(pooler.Object, "spec", "template"); !reflect.DeepEqual(templates[name], after) {
			t.Errorf("a plan change rewrote replica %s's pooler template, which would roll PgBouncer", name)
		}
	}

	// Removing a leaves b's cluster and pooler exactly as they were.
	bCluster, _ := liveObject(t, cl, cnpgClusterGVK, db.Namespace, appv1alpha1.ReadReplicaClusterName(db.Name, "b"))
	bPooler, _ := liveObject(t, cl, cnpgPoolerGVK, db.Namespace, appv1alpha1.ReadReplicaPoolerName(db.Name, "b"))
	db.Spec.ReadReplicas = []appv1alpha1.DatabaseReadReplica{{Name: "b"}}
	reconcile(bigger)
	if _, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, appv1alpha1.ReadReplicaClusterName(db.Name, "a")); ok {
		t.Error("removed replica a kept its cluster")
	}
	if _, ok := liveObject(t, cl, cnpgPoolerGVK, db.Namespace, appv1alpha1.ReadReplicaPoolerName(db.Name, "a")); ok {
		t.Error("removed replica a kept its pooler")
	}
	for gvk, before := range map[schema.GroupVersionKind]*unstructured.Unstructured{cnpgClusterGVK: bCluster, cnpgPoolerGVK: bPooler} {
		after, ok := liveObject(t, cl, gvk, db.Namespace, before.GetName())
		if !ok || after.GetResourceVersion() != before.GetResourceVersion() {
			t.Errorf("removing replica a touched b's %s", gvk.Kind)
		}
	}

	// Pooling off removes the replica's pooler, not its instance.
	db.Spec.Pooler = false
	reconcile(bigger)
	if _, ok := liveObject(t, cl, cnpgPoolerGVK, db.Namespace, bPooler.GetName()); ok {
		t.Error("an unpooled database kept replica b's pooler")
	}
	if _, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, bCluster.GetName()); !ok {
		t.Error("turning pooling off removed replica b's instance")
	}

	// Removing the last replica removes what it created.
	db.Spec.ReadReplicas = nil
	reconcile(bigger)
	if _, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, bCluster.GetName()); ok {
		t.Error("removing the last replica kept its cluster")
	}
	if len(db.Status.ReadReplicaStatuses) != 0 {
		t.Errorf("statuses after removing every replica = %v", db.Status.ReadReplicaStatuses)
	}
}

// assertOwnReplicaObjects checks that replica name has its own cluster,
// Service and PgBouncer, owned and labelled by db, and returns the cluster.
func assertOwnReplicaObjects(t *testing.T, cl client.Client, db *appv1alpha1.Database, name string) *unstructured.Unstructured {
	t.Helper()
	cluster, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, appv1alpha1.ReadReplicaClusterName(db.Name, name))
	if !ok {
		t.Fatalf("replica %s has no cluster", name)
	}
	if !metav1.IsControlledBy(cluster, db) || cluster.GetLabels()[labelReadReplicaOf] != db.Name ||
		cluster.GetAnnotations()[annotationReplicaMajor] != "18" {
		t.Errorf("replica %s cluster metadata = %v %v %v", name, cluster.GetOwnerReferences(), cluster.GetLabels(), cluster.GetAnnotations())
	}
	service, _, _ := unstructured.NestedSlice(cluster.Object, "spec", "managed", "services", "additional")
	if got, _, _ := unstructured.NestedString(service[0].(map[string]any), "serviceTemplate", "metadata", "name"); got != appv1alpha1.ReadReplicaServiceName(db.Name, name) {
		t.Errorf("replica %s Service = %q", name, got)
	}
	pooler, ok := liveObject(t, cl, cnpgPoolerGVK, db.Namespace, appv1alpha1.ReadReplicaPoolerName(db.Name, name))
	if !ok {
		t.Fatalf("pooled replica %s has no pooler", name)
	}
	if got, _, _ := unstructured.NestedString(pooler.Object, "spec", "cluster", "name"); got != cluster.GetName() {
		t.Errorf("replica %s pooler fronts %q, want its own cluster %q", name, got, cluster.GetName())
	}
	if got, _, _ := unstructured.NestedString(pooler.Object, "spec", "pgbouncer", "parameters", "default_pool_size"); got != "90" {
		t.Errorf("replica %s pool = %q, want basic-1gb's 100 - 10", name, got)
	}
	if !metav1.IsControlledBy(pooler, db) || pooler.GetLabels()[labelReadReplicaOf] != db.Name {
		t.Errorf("replica %s pooler metadata = %v %v", name, pooler.GetOwnerReferences(), pooler.GetLabels())
	}
	return cluster
}

// TestReadReplicaStatusesStale: replica objects are listed for removal only
// when the saved statuses describe another set, pooling or provisioning than
// the spec asks for.
func TestReadReplicaStatusesStale(t *testing.T) {
	status := func(name string, pooled bool) appv1alpha1.DatabaseReadReplicaStatus {
		s := appv1alpha1.DatabaseReadReplicaStatus{Name: name, InternalHost: name + ".svc"}
		if pooled {
			s.PoolerHost = name + "-pool.svc"
		}
		return s
	}
	spec := func(names ...string) []appv1alpha1.DatabaseReadReplica {
		out := make([]appv1alpha1.DatabaseReadReplica, len(names))
		for i, name := range names {
			out[i].Name = name
		}
		return out
	}
	for _, tc := range []struct {
		name        string
		replicas    []appv1alpha1.DatabaseReadReplica
		pooled      bool
		statuses    []appv1alpha1.DatabaseReadReplicaStatus
		provisioned bool
		want        bool
	}{
		{"unchanged", spec("a", "b"), true, []appv1alpha1.DatabaseReadReplicaStatus{status("a", true), status("b", true)}, true, false},
		{"none ever", nil, false, nil, false, false},
		{"one removed", spec("b"), true, []appv1alpha1.DatabaseReadReplicaStatus{status("a", true), status("b", true)}, true, true},
		{"one renamed", spec("a", "c"), true, []appv1alpha1.DatabaseReadReplicaStatus{status("a", true), status("b", true)}, true, true},
		{"pooling off", spec("a"), false, []appv1alpha1.DatabaseReadReplicaStatus{status("a", true)}, true, true},
		{"no longer eligible", spec("a"), false, []appv1alpha1.DatabaseReadReplicaStatus{status("a", false)}, false, true},
		{"last removed", nil, false, []appv1alpha1.DatabaseReadReplicaStatus{status("a", false)}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{ReadReplicas: tc.replicas, Pooler: tc.pooled},
				Status: appv1alpha1.DatabaseStatus{ReadReplicaStatuses: tc.statuses}}
			if got := readReplicaStatusesStale(db, tc.provisioned); got != tc.want {
				t.Fatalf("stale = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReadReplicasWaitForASettledPrimary: no replica is cloned from a primary
// that is not serving, and an existing replica keeps its spec while the
// primary is away, but hibernates with the Database.
func TestReadReplicasWaitForASettledPrimary(t *testing.T) {
	ctx := context.Background()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: replicaDB, Namespace: "tea-a", UID: "db-uid"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}}},
	}
	r, cl := replicaFixture(t, db)
	plan, _ := resolvePlan(db.Spec)
	name := appv1alpha1.ReadReplicaClusterName(db.Name, "a")

	state, err := r.reconcileReadReplicas(ctx, db, replicaPrimary(t, db, 0, 0), plan, 10)
	if err != nil {
		t.Fatal(err)
	}
	if state.ready {
		t.Error("replicas reported ready before any was cloned")
	}
	if _, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, name); ok {
		t.Fatal("a replica was cloned from a primary that is not serving")
	}

	if _, err := r.reconcileReadReplicas(ctx, db, replicaPrimary(t, db, 1, 18), plan, 10); err != nil {
		t.Fatal(err)
	}
	before, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, name)
	if !ok {
		t.Fatal("no replica once the primary served")
	}
	beforeSpec, _, _ := unstructured.NestedMap(before.Object, "spec")

	db.Spec.Suspended = true
	hibernated := replicaPrimary(t, db, 0, 18)
	if err := unstructured.SetNestedField(hibernated.Object, "postgres:19-is-not-here", "spec", "imageName"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.reconcileReadReplicas(ctx, db, hibernated, plan, 10); err != nil {
		t.Fatal(err)
	}
	after, _ := liveObject(t, cl, cnpgClusterGVK, db.Namespace, name)
	if after.GetAnnotations()[hibernationAnnotation] != "on" {
		t.Error("a suspended database's replica did not hibernate")
	}
	if afterSpec, _, _ := unstructured.NestedMap(after.Object, "spec"); !reflect.DeepEqual(beforeSpec, afterSpec) {
		t.Error("a replica followed a primary that is not serving")
	}
}

// TestReadReplicaIsClonedAgainAfterAMajorUpgrade: a physical standby cannot
// cross a major, so a replica cloned at the primary's previous major is
// deleted and cloned again from the upgraded primary.
func TestReadReplicaIsClonedAgainAfterAMajorUpgrade(t *testing.T) {
	ctx := context.Background()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: replicaDB, Namespace: "tea-a", UID: "db-uid"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}}},
	}
	r, cl := replicaFixture(t, db)
	plan, _ := resolvePlan(db.Spec)
	name := appv1alpha1.ReadReplicaClusterName(db.Name, "a")
	if _, err := r.reconcileReadReplicas(ctx, db, replicaPrimary(t, db, 1, 17), plan, 10); err != nil {
		t.Fatal(err)
	}
	upgraded := replicaPrimary(t, db, 1, 18)
	if state, err := r.reconcileReadReplicas(ctx, db, upgraded, plan, 10); err != nil || state.ready {
		t.Fatalf("state = %+v, err = %v; want the stale replica not ready", state, err)
	}
	if _, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, name); ok {
		t.Fatal("a replica cloned at 17 survived the primary's upgrade to 18")
	}
	if _, err := r.reconcileReadReplicas(ctx, db, upgraded, plan, 10); err != nil {
		t.Fatal(err)
	}
	cluster, ok := liveObject(t, cl, cnpgClusterGVK, db.Namespace, name)
	if !ok || cluster.GetAnnotations()[annotationReplicaMajor] != "18" {
		t.Fatalf("replica was not cloned again at 18: %v", cluster)
	}
}

// TestSwitchoverTargetsTheHAStandby (w6/m467): the primary Cluster holds only
// the primary and its HA standby, so a planned failover promotes the standby,
// never a replica, whose instance is a cluster of its own.
func TestSwitchoverTargetsTheHAStandby(t *testing.T) {
	ctx := context.Background()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: replicaDB, Namespace: "tea-a", UID: "db-uid"},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-4gb", StorageGB: 10, HighAvailability: true,
			ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}, {Name: "b"}}},
	}
	r, cl := replicaFixture(t, db)
	primary := replicaPrimary(t, db, 2, 18)
	if got := primary.Object["spec"].(map[string]any)["instances"]; got != int64(2) {
		t.Fatalf("primary Cluster instances = %v, want the primary and its HA standby", got)
	}
	if err := unstructured.SetNestedField(primary.Object, replicaDB+"-1", "status", "currentPrimary"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedStringSlice(primary.Object, []string{replicaDB + "-1", replicaDB + "-2"}, "status", "instanceNames"); err != nil {
		t.Fatal(err)
	}
	status := primary.Object["status"]
	if err := cl.Create(ctx, primary); err != nil {
		t.Fatal(err)
	}
	primary.Object["status"] = status
	if err := cl.Status().Update(ctx, primary); err != nil {
		t.Fatal(err)
	}
	r.triggerCNPGSwitchover(ctx, primary)
	live, _ := liveObject(t, cl, cnpgClusterGVK, db.Namespace, db.Name)
	if got, _, _ := unstructured.NestedString(live.Object, "status", "targetPrimary"); got != replicaDB+"-2" {
		t.Fatalf("switchover targets %q, want the HA standby %s-2", got, replicaDB)
	}
}
