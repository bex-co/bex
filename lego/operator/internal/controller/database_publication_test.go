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
	"errors"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// replicationRoles returns the names of the projected managed roles that carry
// CNPG's replication attribute, and fails on any other value of the key.
func replicationRoles(t *testing.T, spec map[string]any) []string {
	t.Helper()
	managed, _ := spec["managed"].(map[string]any)
	roles, _ := managed["roles"].([]any)
	var names []string
	for _, item := range roles {
		role := item.(map[string]any)
		switch v, ok := role["replication"]; {
		case !ok:
		case v == true:
			names = append(names, role["name"].(string))
		default:
			t.Fatalf("role %v projects replication %v; only true or absent is meaningful", role["name"], v)
		}
	}
	return names
}

func publisherParams(lr *appv1alpha1.DatabaseLogicalReplication) clusterParams {
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"})
	return clusterParams{
		plan: plan, storageGB: 10, dbname: "orders", owner: "orders_user",
		users: []appv1alpha1.DatabaseUser{
			{Name: "app_rotated", OwnerPrivileges: true},
			{Name: "cdc_reader"},
		},
		deletedUsers:       []string{"old_reader"},
		logicalReplication: lr,
	}
}

// Only the publisher role the request names gets REPLICATION (DoD 1, 3): the
// owner or any active credential, never a retired login, and no role at all
// while the database is not a publisher.
func TestPublisherRoleReplication(t *testing.T) {
	for _, tc := range []struct {
		name string
		lr   *appv1alpha1.DatabaseLogicalReplication
		want []string
	}{
		{name: "not a publisher", lr: nil},
		{name: "a dedicated reader", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader"}, want: []string{"cdc_reader"}},
		{name: "the owner", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "orders_user"}, want: []string{"orders_user"}},
		{name: "an owner-privileged credential", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "app_rotated"}, want: []string{"app_rotated"}},
		{name: "a retired login", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "old_reader"}},
		{name: "a role the database does not have", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "nobody"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := replicationRoles(t, cnpgClusterSpec(publisherParams(tc.lr)))
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("roles with REPLICATION = %v, want %v", got, tc.want)
			}
		})
	}
	// Every other attribute of every role is the projection without a
	// publisher, so enabling changes nothing but the one attribute.
	plain := cnpgClusterSpec(publisherParams(nil))["managed"].(map[string]any)["roles"].([]any)
	enabled := cnpgClusterSpec(publisherParams(&appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader"}))["managed"].(map[string]any)["roles"].([]any)
	for i := range plain {
		role := enabled[i].(map[string]any)
		delete(role, "replication")
		if !reflect.DeepEqual(role, plain[i]) {
			t.Errorf("role %d = %#v, want %#v plus replication only", i, role, plain[i])
		}
	}
}

// An enabled database bounds the WAL a slot may retain, scaled with its disk
// and above any legacy tenant value; a never-enabled one keeps PostgreSQL's
// unbounded default (DoD 5).
func TestSlotWALKeepSizeOnlyWhilePublishing(t *testing.T) {
	params := func(spec map[string]any) map[string]any {
		return spec["postgresql"].(map[string]any)["parameters"].(map[string]any)
	}
	off := publisherParams(nil)
	off.parameters = map[string]string{"work_mem": "8MB"}
	if v, ok := params(cnpgClusterSpec(off))[paramMaxSlotWALKeepSize]; ok {
		t.Fatalf("a database that is not a publisher projects %s = %v", paramMaxSlotWALKeepSize, v)
	}
	for storage, want := range map[int32]string{10: "2048MB", 50: "10240MB", 256: "52428MB"} {
		on := publisherParams(&appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader"})
		on.storageGB = storage
		on.parameters = map[string]string{paramMaxSlotWALKeepSize: "-1", " MAX_SLOT_WAL_KEEP_SIZE": "-1"}
		got := params(cnpgClusterSpec(on))
		if got[paramMaxSlotWALKeepSize] != want || len(got) != len(params(cnpgClusterSpec(publisherParams(nil))))+1 {
			t.Errorf("%d GB publisher: parameters %v, want only %s = %s added", storage, got, paramMaxSlotWALKeepSize, want)
		}
	}
}

func publicationScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(cnpgDatabaseGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(cnpgPublicationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(cnpgSubscriptionGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(cnpgSubscriptionGVK.GroupVersion().WithKind(cnpgSubscriptionGVK.Kind+"List"), &unstructured.UnstructuredList{})
	return scheme
}

// The all-tables publication follows spec.logicalReplication.allTables: an
// owned CNPG Publication with reclaim policy delete while set, gone when
// cleared (DoD 4), and CNPG's verdict on the LogicalReplication condition. The
// Cluster's managed role loses REPLICATION when the field goes.
func TestAllTablesPublicationFollowsTheSpec(t *testing.T) {
	ctx := context.Background()
	scheme := publicationScheme()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-pub", Namespace: "default", UID: "uid-dpg-pub"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "orders", Plan: "basic-1gb", StorageGB: 10, Users: []appv1alpha1.DatabaseUser{{Name: "cdc_reader"}},
			LogicalReplication: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}, AllTables: true},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).WithStatusSubresource(&appv1alpha1.Database{}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	reconcileTwice := func() *appv1alpha1.Database {
		t.Helper()
		for range 2 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		var got appv1alpha1.Database
		if err := cl.Get(ctx, nn, &got); err != nil {
			t.Fatal(err)
		}
		return &got
	}
	publication := func() (*unstructured.Unstructured, error) {
		child := &unstructured.Unstructured{}
		child.SetGroupVersionKind(cnpgPublicationGVK)
		return child, cl.Get(ctx, types.NamespacedName{Name: "dpg-pub-all-tables", Namespace: "default"}, child)
	}
	clusterReplicators := func() []string {
		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(cnpgClusterGVK)
		if err := cl.Get(ctx, nn, cluster); err != nil {
			t.Fatal(err)
		}
		spec, _, _ := unstructured.NestedMap(cluster.Object, "spec")
		return replicationRoles(t, spec)
	}

	got := reconcileTwice()
	child, err := publication()
	if err != nil {
		t.Fatalf("all-tables publication: %v", err)
	}
	spec, _, _ := unstructured.NestedMap(child.Object, "spec")
	cluster, _, _ := unstructured.NestedString(spec, "cluster", "name")
	allTables, _, _ := unstructured.NestedBool(spec, "target", "allTables")
	if cluster != db.Name || spec["dbname"] != db.Spec.EffectiveDatabaseName(db.Name) || spec["name"] != appv1alpha1.LogicalReplicationAllTablesPublication ||
		!allTables || spec["publicationReclaimPolicy"] != "delete" {
		t.Fatalf("publication spec = %#v", spec)
	}
	if owners := child.GetOwnerReferences(); len(owners) != 1 || owners[0].Name != db.Name {
		t.Errorf("owners = %+v, want the bex Database", owners)
	}
	if c := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionLogicalReplication); c == nil || c.Status != metav1.ConditionUnknown {
		t.Errorf("condition before CNPG applied = %+v, want Unknown", c)
	}
	if roles := clusterReplicators(); len(roles) != 1 || roles[0] != "cdc_reader" {
		t.Fatalf("Cluster roles with REPLICATION = %v, want cdc_reader", roles)
	}

	// CNPG's refusal reaches the Database.
	_ = unstructured.SetNestedField(child.Object, false, "status", "applied")
	_ = unstructured.SetNestedField(child.Object, "permission denied for database orders", "status", "message")
	if err := cl.Update(ctx, child); err != nil {
		t.Fatal(err)
	}
	got = reconcileTwice()
	if c := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionLogicalReplication); c == nil || c.Status != metav1.ConditionFalse ||
		c.Reason != reasonLogicalReplicationApplyFailed || c.Message != "permission denied for database orders" {
		t.Errorf("condition after a refusal = %+v, want False with CNPG's message", c)
	}

	// Clearing allTables removes the publication and keeps the publisher.
	got.Spec.LogicalReplication.AllTables = false
	if err := cl.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	got = reconcileTwice()
	if _, err := publication(); err == nil {
		t.Fatal("the publication survived clearing allTables")
	}
	if c := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionLogicalReplication); c == nil || c.Status != metav1.ConditionTrue || c.Reason != reasonLogicalReplicationEnabled {
		t.Errorf("condition without allTables = %+v, want True/Enabled", c)
	}

	// Disabling takes REPLICATION back off and leaves no condition.
	got.Spec.LogicalReplication = nil
	if err := cl.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	got = reconcileTwice()
	if c := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionLogicalReplication); c != nil {
		t.Errorf("condition after disable = %+v, want none", c)
	}
	if roles := clusterReplicators(); len(roles) != 0 {
		t.Errorf("Cluster roles with REPLICATION after disable = %v, want none", roles)
	}
}

// A database that never published costs no publication read, so the whole
// fleet's reconciles are unchanged; a failed removal is reported, not
// swallowed.
func TestPublicationReadsOnlyForPublishers(t *testing.T) {
	ctx := context.Background()
	scheme := publicationScheme()
	reads := 0
	failDelete := false
	funcs := interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if u, ok := obj.(*unstructured.Unstructured); ok && u.GroupVersionKind() == cnpgPublicationGVK {
				reads++
				if failDelete {
					return errors.New("the API server is unavailable")
				}
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-plain", Namespace: "default", UID: "uid-dpg-plain"}, Spec: appv1alpha1.DatabaseSpec{Name: "plain"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).WithStatusSubresource(&appv1alpha1.Database{}).WithInterceptorFuncs(funcs).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	for range 3 {
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
	}
	if reads != 0 {
		t.Fatalf("a database that never published read its publication %d times", reads)
	}

	// A publisher whose publication cannot be checked keeps the condition, so
	// the removal is retried and visible.
	var got appv1alpha1.Database
	if err := cl.Get(ctx, nn, &got); err != nil {
		t.Fatal(err)
	}
	got.Spec.LogicalReplication = &appv1alpha1.DatabaseLogicalReplication{PublisherRole: got.Spec.EffectiveDatabaseUser(got.Name)}
	if err := cl.Update(ctx, &got); err != nil {
		t.Fatal(err)
	}
	failDelete = true
	_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
	if err := cl.Get(ctx, nn, &got); err != nil {
		t.Fatal(err)
	}
	if c := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionLogicalReplication); c == nil || c.Status != metav1.ConditionFalse || c.Reason != reasonLogicalReplicationRemoveError {
		t.Errorf("condition = %+v, want False/%s", c, reasonLogicalReplicationRemoveError)
	}
}

func TestPublicationCondition(t *testing.T) {
	child := func(generation int64, status map[string]any) *unstructured.Unstructured {
		o := &unstructured.Unstructured{Object: map[string]any{}}
		o.SetGeneration(generation)
		if status != nil {
			o.Object["status"] = status
		}
		return o
	}
	for _, tc := range []struct {
		name   string
		child  *unstructured.Unstructured
		status metav1.ConditionStatus
		reason string
	}{
		{"not yet reconciled", child(1, nil), metav1.ConditionUnknown, reasonLogicalReplicationPending},
		{"stale verdict", child(2, map[string]any{"applied": true, "observedGeneration": int64(1)}), metav1.ConditionUnknown, reasonLogicalReplicationPending},
		{"applied", child(2, map[string]any{"applied": true, "observedGeneration": int64(2)}), metav1.ConditionTrue, reasonLogicalReplicationApplied},
		{"failed", child(1, map[string]any{"applied": false, "message": "boom"}), metav1.ConditionFalse, reasonLogicalReplicationApplyFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := allTablesPublicationVerdict.condition(tc.child, 4, nil)
			if got.Status != tc.status || got.Reason != tc.reason || got.ObservedGeneration != 4 {
				t.Errorf("condition = %+v, want %s/%s at generation 4", got, tc.status, tc.reason)
			}
		})
	}
}

// The bound leaves the publisher's smallest disk most of its room: retained
// WAL alone stays well below the autoscaling trigger.
func TestSlotWALBoundSitsBelowAutoscaling(t *testing.T) {
	if float64(slotWALKeepPercent)/100 >= databaseDiskAutoscaleThreshold/2 {
		t.Fatalf("a slot may retain %d%% of the disk, too close to the %.0f%% autoscaling trigger", slotWALKeepPercent, databaseDiskAutoscaleThreshold*100)
	}
	if got := slotWALKeepSize(tiers.PostgresLogicalReplicationMinStorageGB); got != "2048MB" {
		t.Fatalf("the bound at Render's %d GB floor is %s, want 2048MB", tiers.PostgresLogicalReplicationMinStorageGB, got)
	}
}
