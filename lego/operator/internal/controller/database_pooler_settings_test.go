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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m458: every plan's PgBouncer runs Render's settings
// (render.com/docs/postgresql-connection-pooling, read 2026-11-17), not
// PgBouncer's defaults of 100 clients and a pool of 20.
func TestEveryPlansPoolerRunsRendersPgBouncerSettings(t *testing.T) {
	for _, id := range tiers.Postgres.IDs() {
		plan, _ := tiers.Postgres.ByID(id)
		pool := strconv.Itoa(int(plan.MaxConnections) - 10)
		want := map[string]any{
			"max_client_conn":     "30000",
			"default_pool_size":   pool,
			"max_db_connections":  pool,
			"client_idle_timeout": "86400",
			"server_login_retry":  "2",
		}
		for _, poolerType := range []string{poolerTypePrimary, poolerTypeReplicas} {
			got, _, _ := unstructured.NestedMap(poolerSpec("db", poolerType, plan), "pgbouncer", "parameters")
			if !reflect.DeepEqual(got, want) {
				t.Errorf("plan %q %s pooler parameters = %v, want %v", id, poolerType, got, want)
			}
		}
	}
	// The pool follows the plan's own figure: a Render 2c-8g database
	// (max_connections 200, w6/m332) pools 190.
	got, _, _ := unstructured.NestedString(poolerSpec("db", poolerTypePrimary, tiers.PostgresTier{MaxConnections: 200}), "pgbouncer", "parameters", "default_pool_size")
	if got != "190" {
		t.Errorf("a 200-connection plan pools %q, want 190", got)
	}
}

// w6/m458: a pooled database's read replicas are pooled too, by one reader
// PgBouncer that comes and goes with pooling and with the replicas; a plan
// change rewrites the pool without rolling PgBouncer.
func TestReadReplicasArePooledWithTheirDatabase(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-a", Namespace: "tea-a", UID: "db-uid"},
		Spec: appv1alpha1.DatabaseSpec{
			Plan: "basic-1gb", Pooler: true,
			ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "analytics"}, {Name: "reports"}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	cluster := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{}}}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	reconcile := func() {
		t.Helper()
		if _, err := r.reconcilePooler(ctx, db, cluster); err != nil {
			t.Fatal(err)
		}
	}
	pooler := func(name string) (*unstructured.Unstructured, bool) {
		t.Helper()
		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(cnpgPoolerGVK)
		err := cl.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: name}, live)
		if apierrors.IsNotFound(err) {
			return nil, false
		}
		if err != nil {
			t.Fatal(err)
		}
		return live, true
	}

	reconcile()
	primary, ok := pooler("dpg-a-pooler")
	if !ok {
		t.Fatal("a pooled database has no primary Pooler")
	}
	readers, ok := pooler("dpg-a-pooler-ro")
	if !ok {
		t.Fatal("a pooled database's read replicas have no Pooler")
	}
	for name, live := range map[string]*unstructured.Unstructured{"rw": primary, "ro": readers} {
		if got, _, _ := unstructured.NestedString(live.Object, "spec", "type"); got != name {
			t.Errorf("%s Pooler type = %q", name, got)
		}
		if got, _, _ := unstructured.NestedString(live.Object, "spec", "pgbouncer", "parameters", "default_pool_size"); got != "90" {
			t.Errorf("%s Pooler pool = %q, want basic-1gb's 100 - 10", name, got)
		}
		if owners := live.GetOwnerReferences(); len(owners) != 1 || owners[0].UID != db.UID {
			t.Errorf("%s Pooler owners = %v, want the Database", name, owners)
		}
	}

	// A plan with another connection limit (w6/m332's 2c-8g has 200) rewrites
	// the pool, and only the parameters: the pod template, whose change would
	// roll PgBouncer and drop its clients, stays as it was.
	before, _, _ := unstructured.NestedMap(primary.Object, "spec", "template")
	if err := r.upsertPooler(ctx, db, cluster, poolerTypePrimary, tiers.PostgresTier{MaxConnections: 200}, ""); err != nil {
		t.Fatal(err)
	}
	primary, _ = pooler("dpg-a-pooler")
	if got, _, _ := unstructured.NestedString(primary.Object, "spec", "pgbouncer", "parameters", "max_db_connections"); got != "190" {
		t.Errorf("after the plan change the pool is %q, want 190", got)
	}
	if after, _, _ := unstructured.NestedMap(primary.Object, "spec", "template"); !reflect.DeepEqual(before, after) {
		t.Errorf("a plan change rewrote the pod template %v → %v, which would roll PgBouncer", before, after)
	}

	// Removing the replicas removes their pooler, not the primary's.
	db.Spec.ReadReplicas = nil
	reconcile()
	if _, ok := pooler("dpg-a-pooler-ro"); ok {
		t.Error("a database without replicas kept the read replica Pooler")
	}
	if _, ok := pooler("dpg-a-pooler"); !ok {
		t.Error("removing the replicas removed the primary's Pooler")
	}

	// Turning pooling off removes both.
	db.Spec.ReadReplicas = []appv1alpha1.DatabaseReadReplica{{Name: "analytics"}}
	reconcile()
	if _, ok := pooler("dpg-a-pooler-ro"); !ok {
		t.Fatal("re-adding a replica did not pool it")
	}
	db.Spec.Pooler = false
	reconcile()
	for _, name := range []string{"dpg-a-pooler", "dpg-a-pooler-ro"} {
		if _, ok := pooler(name); ok {
			t.Errorf("an unpooled database kept Pooler %s", name)
		}
	}
	if db.Status.PoolerHost != "" {
		t.Errorf("an unpooled database reports pooler host %q", db.Status.PoolerHost)
	}
}

// w6/m458: PgBouncer starts with its open-file soft limit at the container's
// hard limit. containerd 2 starts containers at a soft limit of 1,024, and
// PgBouncer, which needs a descriptor per client and per server connection,
// never raises it: on production 1,000 clients left it unable to accept or
// to open more than 10 of its 90 server connections.
func TestPgBouncerStartsWithTheHardFileLimit(t *testing.T) {
	want := []any{"/bin/sh", "-c", pgbouncerRunCommand}
	for _, poolerType := range []string{poolerTypePrimary, poolerTypeReplicas} {
		containers, _, _ := unstructured.NestedSlice(poolerSpec("db", poolerType, tiers.PostgresTier{MaxConnections: 100}), "template", "spec", "containers")
		if len(containers) != 1 {
			t.Fatalf("%s pooler template containers = %v, want only pgbouncer", poolerType, containers)
		}
		c, _ := containers[0].(map[string]any)
		if c["name"] != "pgbouncer" || !reflect.DeepEqual(c["command"], want) {
			t.Errorf("%s pooler pgbouncer container = %v, want command %v", poolerType, c, want)
		}
	}

	// Run the command as the container would, with a stand-in for CNPG's
	// manager that reports the limit it inherits and the arguments it gets,
	// under a soft limit of 256 and a hard limit of 4,096.
	const manager = "/controller/manager pgbouncer run"
	if !strings.HasSuffix(pgbouncerRunCommand, "exec "+manager) {
		t.Fatalf("the command %q does not end by exec'ing CNPG's %q", pgbouncerRunCommand, manager)
	}
	stub := filepath.Join(t.TempDir(), "manager")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho \"$(ulimit -n) $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := strings.Replace(pgbouncerRunCommand, "/controller/manager", stub, 1)
	out, err := exec.Command("/bin/sh", "-c", `ulimit -S -n 256 && ulimit -H -n 4096 && exec /bin/sh -c "$0"`, command).CombinedOutput()
	if err != nil {
		t.Fatalf("the command failed: %v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "4096 pgbouncer run" {
		t.Errorf("CNPG's manager started with %q, want the hard limit 4096 and its pgbouncer run arguments", got)
	}
}
