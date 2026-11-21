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
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// subscriberRoles returns the names of the projected managed roles that are
// members of pg_create_subscription.
func subscriberRoles(spec map[string]any) []string {
	managed, _ := spec["managed"].(map[string]any)
	roles, _ := managed["roles"].([]any)
	var names []string
	for _, item := range roles {
		role := item.(map[string]any)
		inRoles, _ := role["inRoles"].([]any)
		if slices.Contains(inRoles, any(pgCreateSubscriptionRole)) {
			names = append(names, role["name"].(string))
		}
	}
	return names
}

// Only the subscriber role the request names joins pg_create_subscription,
// and only once the cluster runs a PostgreSQL that has the role: a 13-15
// cluster, or one still upgrading from 15, would fail every managed role.
func TestSubscriberRoleJoinsPgCreateSubscription(t *testing.T) {
	subscriber := func(role string) *appv1alpha1.DatabaseLogicalReplication {
		return &appv1alpha1.DatabaseLogicalReplication{SubscriberRole: role}
	}
	for _, tc := range []struct {
		name             string
		lr               *appv1alpha1.DatabaseLogicalReplication
		version, running string
		creating         bool
		want             []string
	}{
		{name: "not a subscriber", version: "18"},
		{name: "a publisher only", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader"}, version: "18"},
		{name: "the owner on 18", lr: subscriber("orders_user"), version: "18", running: "18", want: []string{"orders_user"}},
		{name: "a dedicated credential on 16", lr: subscriber("cdc_reader"), version: "16", running: "16", want: []string{"cdc_reader"}},
		{name: "an owner-privileged credential on 17", lr: subscriber("app_rotated"), version: "17", running: "17", want: []string{"app_rotated"}},
		{name: "a retired login", lr: subscriber("old_reader"), version: "18", running: "18"},
		{name: "a role the database does not have", lr: subscriber("nobody"), version: "18", running: "18"},
		{name: "PostgreSQL 15", lr: subscriber("orders_user"), version: "15", running: "15"},
		{name: "upgrading from 15 to 16", lr: subscriber("orders_user"), version: "16", running: "15"},
		{name: "an unversioned database being created", lr: subscriber("orders_user"), creating: true, want: []string{"orders_user"}},
		{name: "an unversioned database whose major is not observed yet", lr: subscriber("orders_user")},
		{name: "an unversioned database running 18", lr: subscriber("orders_user"), running: "18", want: []string{"orders_user"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := publisherParams(tc.lr)
			p.version, p.runningVersion, p.creating = tc.version, tc.running, tc.creating
			spec := cnpgClusterSpec(p)
			if got := subscriberRoles(spec); !slices.Equal(got, tc.want) {
				t.Fatalf("members of %s = %v, want %v", pgCreateSubscriptionRole, got, tc.want)
			}
			if tc.lr != nil && !tc.lr.Publishes() {
				// A subscriber alone publishes nothing: no REPLICATION, no WAL bound.
				if roles := replicationRoles(t, spec); len(roles) != 0 {
					t.Errorf("a subscriber gave REPLICATION to %v", roles)
				}
				if v, ok := spec["postgresql"].(map[string]any)["parameters"].(map[string]any)[paramMaxSlotWALKeepSize]; ok {
					t.Errorf("a subscriber projects %s = %v", paramMaxSlotWALKeepSize, v)
				}
			}
		})
	}
	// The membership is added to what the role already had, and nothing else
	// changes: the owner keeps pg_monitor, an owner-privileged credential its
	// inherited owners.
	for _, role := range []string{"orders_user", "app_rotated", "cdc_reader"} {
		plain := publisherParams(nil)
		plain.version = "18"
		enabled := publisherParams(subscriber(role))
		enabled.version = "18"
		before := cnpgClusterSpec(plain)["managed"].(map[string]any)["roles"].([]any)
		after := cnpgClusterSpec(enabled)["managed"].(map[string]any)["roles"].([]any)
		for i := range before {
			was, is := before[i].(map[string]any), after[i].(map[string]any)
			if is["name"] == role {
				wantRoles, _ := was["inRoles"].([]any)
				wantRoles = append(slices.Clone(wantRoles), pgCreateSubscriptionRole)
				if !reflect.DeepEqual(is["inRoles"], wantRoles) {
					t.Errorf("%s inRoles = %v, want %v", role, is["inRoles"], wantRoles)
				}
				is, was = maps.Clone(is), maps.Clone(was)
				delete(is, "inRoles")
				delete(was, "inRoles")
			}
			if !reflect.DeepEqual(is, was) {
				t.Errorf("subscriber %s changed role %d: %#v, want %#v", role, i, is, was)
			}
		}
	}
}

func logicalSubscription(name string) appv1alpha1.DatabaseLogicalSubscription {
	return appv1alpha1.DatabaseLogicalSubscription{
		Name: name, Publication: "pub", Host: "dpg-source.db.example.test", DBName: "source", User: "source_user",
		PasswordSecret: appv1alpha1.LogicalSubscriptionObjectName("dpg-sub", name),
	}
}

// Each 13-15 subscription's publisher is an externalClusters entry named
// like its Subscription, whose password is a passfile from the Secret, never
// a value in the Cluster spec; a recovery's source stays first.
func TestSubscriptionExternalClusters(t *testing.T) {
	sub := logicalSubscription("from_render")
	custom := logicalSubscription("from_elsewhere")
	custom.Port, custom.SSLMode = 6543, "prefer"
	p := publisherParams(&appv1alpha1.DatabaseLogicalReplication{Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{sub, custom}})
	p.name, p.version = "dpg-sub", "15"
	p.recovery, p.store = &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-old"}, &BackupStore{}
	entries, _ := cnpgClusterSpec(p)["externalClusters"].([]any)
	if len(entries) != 3 || entries[0].(map[string]any)["name"] != recoverySource {
		t.Fatalf("externalClusters = %#v, want the recovery source and then two publishers", entries)
	}
	want := map[string]any{
		"name": appv1alpha1.LogicalSubscriptionObjectName("dpg-sub", "from_render"),
		"connectionParameters": map[string]any{
			"host": "dpg-source.db.example.test", "port": "5432", "dbname": "source", "user": "source_user", "sslmode": "require",
		},
		"password": map[string]any{"name": sub.PasswordSecret, "key": "password"},
	}
	if !reflect.DeepEqual(entries[1], want) {
		t.Errorf("publisher entry = %#v, want %#v", entries[1], want)
	}
	params := entries[2].(map[string]any)["connectionParameters"].(map[string]any)
	if params["port"] != "6543" || params["sslmode"] != "prefer" {
		t.Errorf("a named port and sslmode = %#v", params)
	}
	if entries[1].(map[string]any)["name"] != subscriptionSpec("dpg-sub", "orders", sub)["externalClusterName"] {
		t.Error("the Subscription does not name its publisher's externalClusters entry")
	}
	// Without subscriptions there is no entry, and a plain database has no key.
	p.logicalReplication = nil
	if entries, _ := cnpgClusterSpec(p)["externalClusters"].([]any); len(entries) != 1 {
		t.Errorf("externalClusters without subscriptions = %#v", entries)
	}
	p.recovery = nil
	if _, ok := cnpgClusterSpec(p)["externalClusters"]; ok {
		t.Error("a database with neither a recovery nor subscriptions projects externalClusters")
	}
}

// subscriptionHarness reconciles one Database on a fake client and reads
// back its Subscriptions, Cluster and condition.
type subscriptionHarness struct {
	t  *testing.T
	cl client.Client
	r  *DatabaseReconciler
	nn types.NamespacedName
}

func (h subscriptionHarness) reconcileTwice() *appv1alpha1.Database {
	h.t.Helper()
	for range 2 {
		_, _ = h.r.Reconcile(context.Background(), reconcile.Request{NamespacedName: h.nn})
	}
	var got appv1alpha1.Database
	if err := h.cl.Get(context.Background(), h.nn, &got); err != nil {
		h.t.Fatal(err)
	}
	return &got
}

func (h subscriptionHarness) update(db *appv1alpha1.Database) *appv1alpha1.Database {
	h.t.Helper()
	if err := h.cl.Update(context.Background(), db); err != nil {
		h.t.Fatal(err)
	}
	return h.reconcileTwice()
}

func (h subscriptionHarness) subscription(name string) (*unstructured.Unstructured, error) {
	child := &unstructured.Unstructured{}
	child.SetGroupVersionKind(cnpgSubscriptionGVK)
	return child, h.cl.Get(context.Background(), types.NamespacedName{Name: appv1alpha1.LogicalSubscriptionObjectName(h.nn.Name, name), Namespace: h.nn.Namespace}, child)
}

// setVerdict writes CNPG's verdict on one Subscription.
func (h subscriptionHarness) setVerdict(name string, applied bool, message string) {
	h.t.Helper()
	child, err := h.subscription(name)
	if err != nil {
		h.t.Fatal(err)
	}
	_ = unstructured.SetNestedField(child.Object, applied, "status", "applied")
	_ = unstructured.SetNestedField(child.Object, child.GetGeneration(), "status", "observedGeneration")
	_ = unstructured.SetNestedField(child.Object, message, "status", "message")
	if err := h.cl.Update(context.Background(), child); err != nil {
		h.t.Fatal(err)
	}
}

func (h subscriptionHarness) externalClusters() []any {
	h.t.Helper()
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := h.cl.Get(context.Background(), h.nn, cluster); err != nil {
		h.t.Fatal(err)
	}
	entries, _, _ := unstructured.NestedSlice(cluster.Object, "spec", "externalClusters")
	return entries
}

func logicalCondition(db *appv1alpha1.Database) *metav1.Condition {
	return meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionLogicalReplication)
}

func newSubscriptionHarness(t *testing.T, db *appv1alpha1.Database, objs ...client.Object) subscriptionHarness {
	scheme := publicationScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(objs, db)...).WithStatusSubresource(&appv1alpha1.Database{}).Build()
	return subscriptionHarness{t: t, cl: cl, r: &DatabaseReconciler{Client: cl, Scheme: scheme}, nn: client.ObjectKeyFromObject(db)}
}

func subscriberDatabase(subs ...appv1alpha1.DatabaseLogicalSubscription) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-sub", Namespace: "default", UID: "uid-dpg-sub"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "orders", Plan: "basic-1gb", StorageGB: 10, Version: "15",
			LogicalReplication: &appv1alpha1.DatabaseLogicalReplication{Subscriptions: subs},
		},
	}
}

// The 13-15 subscriptions follow spec.logicalReplication.subscriptions: one
// owned CNPG Subscription each, reclaim policy delete, labelled for the
// reconcile's list, and CNPG's verdict on the LogicalReplication condition.
func TestLogicalSubscriptionsFollowTheSpec(t *testing.T) {
	first, second := logicalSubscription("from_render"), logicalSubscription("from_elsewhere")
	h := newSubscriptionHarness(t, subscriberDatabase(first, second))
	got := h.reconcileTwice()
	dbname := got.Spec.EffectiveDatabaseName(got.Name)
	for _, sub := range []appv1alpha1.DatabaseLogicalSubscription{first, second} {
		child, err := h.subscription(sub.Name)
		if err != nil {
			t.Fatalf("subscription %s: %v", sub.Name, err)
		}
		spec, _, _ := unstructured.NestedMap(child.Object, "spec")
		if !reflect.DeepEqual(spec, subscriptionSpec(got.Name, dbname, sub)) || spec["subscriptionReclaimPolicy"] != "delete" || spec["dbname"] != dbname {
			t.Fatalf("subscription %s spec = %#v", sub.Name, spec)
		}
		if !metav1.IsControlledBy(child, got) || child.GetLabels()[labelLogicalSubscriptionOf] != got.Name {
			t.Errorf("subscription %s owners %+v, labels %v; want the Database's", sub.Name, child.GetOwnerReferences(), child.GetLabels())
		}
	}
	if c := logicalCondition(got); c == nil || c.Status != metav1.ConditionUnknown || c.Reason != reasonLogicalSubscriptionPending || !strings.Contains(c.Message, "from_render") {
		t.Errorf("condition before CNPG applied = %+v, want Unknown naming the subscription", c)
	}
	// Both applied: True. One refused: False with CNPG's message and its name.
	h.setVerdict(first.Name, true, "")
	h.setVerdict(second.Name, true, "")
	if c := logicalCondition(h.reconcileTwice()); c == nil || c.Status != metav1.ConditionTrue || c.Reason != reasonLogicalSubscriptionApplied {
		t.Errorf("condition with both applied = %+v, want True", c)
	}
	h.setVerdict(second.Name, false, `could not connect to the publisher: password authentication failed for user "source_user"`)
	if c := logicalCondition(h.reconcileTwice()); c == nil || c.Status != metav1.ConditionFalse || c.Reason != reasonLogicalSubscriptionApplyFailed ||
		c.Message != `subscription from_elsewhere: could not connect to the publisher: password authentication failed for user "source_user"` {
		t.Errorf("condition after a refusal = %+v, want False naming the subscription with CNPG's message", c)
	}
}

// Removing a subscription deletes its Subscription (CNPG drops it) and its
// publisher's entry and keeps the others; a subscriber role alone reports
// Enabled; disabling leaves no condition. Another Database's Subscription,
// even one with the same label, is never touched.
func TestLogicalSubscriptionsAreRemoved(t *testing.T) {
	first, second := logicalSubscription("from_render"), logicalSubscription("from_elsewhere")
	foreign := &unstructured.Unstructured{}
	foreign.SetGroupVersionKind(cnpgSubscriptionGVK)
	foreign.SetName("dpg-other-sub-0123456789")
	foreign.SetNamespace("default")
	foreign.SetLabels(map[string]string{labelLogicalSubscriptionOf: "dpg-sub"})
	foreign.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "Database", Name: "dpg-other", UID: "uid-other", Controller: new(true)}})
	h := newSubscriptionHarness(t, subscriberDatabase(first, second), foreign)
	got := h.reconcileTwice()

	got.Spec.LogicalReplication.Subscriptions = []appv1alpha1.DatabaseLogicalSubscription{first}
	got = h.update(got)
	if _, err := h.subscription(second.Name); err == nil {
		t.Fatal("a removed subscription's Subscription survived")
	}
	if _, err := h.subscription(first.Name); err != nil {
		t.Fatalf("the kept subscription: %v", err)
	}
	if entries := h.externalClusters(); len(entries) != 1 || entries[0].(map[string]any)["name"] != appv1alpha1.LogicalSubscriptionObjectName(got.Name, first.Name) {
		t.Errorf("Cluster externalClusters = %#v, want the kept subscription's publisher only", entries)
	}

	got.Spec.LogicalReplication = &appv1alpha1.DatabaseLogicalReplication{SubscriberRole: "orders_user"}
	got = h.update(got)
	if _, err := h.subscription(first.Name); err == nil {
		t.Fatal("a subscription survived its removal from the spec")
	}
	if c := logicalCondition(got); c == nil || c.Status != metav1.ConditionTrue || c.Reason != reasonLogicalReplicationEnabled || !strings.Contains(c.Message, "orders_user may create subscriptions") {
		t.Errorf("condition for a subscriber role = %+v, want True/Enabled naming it", c)
	}

	got.Spec.LogicalReplication = nil
	if c := logicalCondition(h.update(got)); c != nil {
		t.Errorf("condition after disable = %+v, want none", c)
	}
	if err := h.cl.Get(context.Background(), client.ObjectKeyFromObject(foreign), foreign.DeepCopy()); err != nil {
		t.Errorf("another Database's Subscription: %v", err)
	}
}

func TestWorstCondition(t *testing.T) {
	c := func(status metav1.ConditionStatus, reason, message string) *metav1.Condition {
		return &metav1.Condition{Status: status, Reason: reason, Message: message}
	}
	if got := worstCondition(nil, nil); got != nil {
		t.Errorf("no verdicts = %+v, want nil", got)
	}
	if got := worstCondition(c(metav1.ConditionTrue, "A", "a"), c(metav1.ConditionUnknown, "B", "b"), c(metav1.ConditionFalse, "C", "c")); got.Reason != "C" {
		t.Errorf("a False among others = %+v, want it", got)
	}
	if got := worstCondition(c(metav1.ConditionTrue, "A", "a"), nil, c(metav1.ConditionUnknown, "B", "b")); got.Reason != "B" {
		t.Errorf("an Unknown among Trues = %+v, want it", got)
	}
	if got := worstCondition(c(metav1.ConditionTrue, "A", "a"), c(metav1.ConditionTrue, "B", "b")); got.Reason != "A" || got.Message != "a; b" {
		t.Errorf("all True = %+v, want the first reason and every message", got)
	}
}

// The CRD's own limits on a subscription are the ones the verb checks.
func TestSubscriptionCRDAgreesWithTheContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "crd", "bases", "app.bex.co_databases.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(raw, &crd); err != nil {
		t.Fatal(err)
	}
	subscriptions := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["logicalReplication"].Properties["subscriptions"]
	if subscriptions.MaxItems == nil || *subscriptions.MaxItems != appv1alpha1.LogicalSubscriptionsMax {
		t.Errorf("CRD maxItems = %v, want LogicalSubscriptionsMax %d", subscriptions.MaxItems, appv1alpha1.LogicalSubscriptionsMax)
	}
	enum := subscriptions.Items.Schema.Properties["sslmode"].Enum
	modes := make([]string, 0, len(enum))
	for _, v := range enum {
		var mode string
		if err := json.Unmarshal(v.Raw, &mode); err != nil {
			t.Fatal(err)
		}
		modes = append(modes, mode)
	}
	if !slices.Equal(modes, appv1alpha1.LogicalSubscriptionSSLModes) || !slices.Contains(modes, appv1alpha1.LogicalSubscriptionDefaultSSLMode) {
		t.Errorf("CRD sslmode enum = %v, want %v with the default %q", modes, appv1alpha1.LogicalSubscriptionSSLModes, appv1alpha1.LogicalSubscriptionDefaultSSLMode)
	}
}
