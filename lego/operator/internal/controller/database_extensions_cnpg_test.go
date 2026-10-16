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
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestApplicationDatabaseVectorOnCNPG is w8/m84's actual-CNPG gate: on a real
// CloudNativePG (BEX_CNPG_KUBECONFIG names a cluster running it, e.g. kind
// plus the cnpg chart the platform pins) and each of bex's pinned major images
// (BEX_CNPG_MAJORS narrows them, e.g. "18"), the application owner's
// `CREATE EXTENSION IF NOT EXISTS vector` is refused until the operator's CNPG
// Database child is applied, then succeeds as a no-op and real vector values,
// distance, an HNSW index and the hstore and pg_stat_statements neighbors all
// work for that owner. Deleting the child keeps the database and its rows
// (retain), and reapplying it changes nothing. Opt-in: needs the cluster and
// kubectl.
func TestApplicationDatabaseVectorOnCNPG(t *testing.T) {
	kubeconfig := os.Getenv("BEX_CNPG_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set BEX_CNPG_KUBECONFIG to a cluster running CloudNativePG")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	majors := slices.Sorted(maps.Keys(cnpgExportImages))
	if only := os.Getenv("BEX_CNPG_MAJORS"); only != "" {
		majors = strings.Split(only, ",")
	}
	for _, major := range majors {
		t.Run("PG"+major, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ns := fmt.Sprintf("bex-vector-it-%s-%d", major, os.Getpid())
			if err := cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cl.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}) })
			const dbname, owner = "orders_prod", "orders_app"

			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			cluster.SetName("pg")
			cluster.SetNamespace(ns)
			// bex's own projection for a Free database on this major, minus what
			// only the hosted platform provides (its storage class, plan sizing),
			// so the pg_stat_statements management, roles and bootstrap are the
			// real ones.
			plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{})
			spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 1, version: major, dbname: dbname, owner: owner, namespace: ns})
			delete(spec["storage"].(map[string]any), "storageClass")
			delete(spec, "resources")
			if err := projectUnstructuredSpec(cluster, spec); err != nil {
				t.Fatal(err)
			}
			if err := cl.Create(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			waitFor(t, 5*time.Minute, "the cluster to be ready", func() bool {
				if err := cl.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
					return false
				}
				ready, _, _ := unstructured.NestedInt64(cluster.Object, "status", "readyInstances")
				return ready == 1
			})
			var app corev1.Secret
			if err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: "pg-app"}, &app); err != nil {
				t.Fatal(err)
			}
			asOwner := func(sql string) (string, error) {
				cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "-n", ns, "exec", "pg-1", "-c", "postgres", "--",
					"env", "PGPASSWORD="+string(app.Data["password"]), "psql", "-X", "-v", "ON_ERROR_STOP=1", "-At",
					"-h", "localhost", "-U", owner, "-d", dbname, "-c", sql)
				out, err := cmd.CombinedOutput()
				return strings.TrimSpace(string(out)), err
			}
			const migration = "CREATE EXTENSION IF NOT EXISTS vector"
			if out, err := asOwner(migration); err == nil || !strings.Contains(out, "permission denied") {
				t.Fatalf("before the child, the owner's migration = %v %q, want permission denied", err, out)
			}

			child := &unstructured.Unstructured{}
			child.SetGroupVersionKind(cnpgDatabaseGVK)
			child.SetName("pg-app")
			child.SetNamespace(ns)
			child.Object["spec"] = applicationDatabaseSpec("pg", dbname, owner)
			applyChild := func() {
				t.Helper()
				fresh := child.DeepCopy()
				if err := cl.Create(ctx, fresh); err != nil {
					t.Fatal(err)
				}
				waitFor(t, 2*time.Minute, "CNPG to apply the application database", func() bool {
					if err := cl.Get(ctx, client.ObjectKeyFromObject(fresh), fresh); err != nil {
						return false
					}
					return extensionsCondition(fresh, 1).Status == metav1.ConditionTrue
				})
			}
			applyChild()

			for _, step := range []struct{ sql, want string }{
				{migration, ""},
				{"SELECT extversion IS NOT NULL FROM pg_extension WHERE extname = 'vector'", "t"},
				{"CREATE TABLE items (id bigserial PRIMARY KEY, embedding vector(3)); INSERT INTO items (embedding) VALUES ('[1,2,3]'), ('[4,5,6]'), ('[1,1,1]')", "INSERT 0 3"},
				{"CREATE INDEX ON items USING hnsw (embedding vector_l2_ops)", "CREATE INDEX"},
				{"SELECT id FROM items ORDER BY embedding <-> '[1,1,2]' LIMIT 1", "3"},
				{"SELECT round((embedding <=> '[2,4,6]')::numeric, 6) FROM items WHERE id = 1", "0.000000"},
				{"CREATE EXTENSION IF NOT EXISTS hstore; SELECT 'a=>1'::hstore -> 'a'", "1"},
			} {
				out, err := asOwner(step.sql)
				if err != nil || !strings.HasSuffix(out, step.want) {
					t.Fatalf("%s = %v %q, want suffix %q", step.sql, err, out, step.want)
				}
			}
			// pg_stat_statements keeps CNPG's own management beside vector: still
			// installed in the application database and readable by the platform's
			// superuser connection.
			superuser := exec.Command("kubectl", "--kubeconfig", kubeconfig, "-n", ns, "exec", "pg-1", "-c", "postgres", "--",
				"psql", "-X", "-At", "-d", dbname, "-c", "SELECT string_agg(extname, ',' ORDER BY extname) FROM pg_extension",
				"-c", "SELECT count(*) >= 0 FROM pg_stat_statements")
			if out, err := superuser.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "hstore,pg_stat_statements,plpgsql,vector\nt" {
				t.Fatalf("application database extensions = %v %q, want hstore, pg_stat_statements, plpgsql, vector and a readable pg_stat_statements", err, out)
			}

			// Withdrawing the child keeps the database: retain.
			if err := cl.Delete(ctx, child.DeepCopy()); err != nil {
				t.Fatal(err)
			}
			waitFor(t, time.Minute, "the child to be deleted", func() bool {
				return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(child), child.DeepCopy()))
			})
			if out, err := asOwner("SELECT count(*) FROM items"); err != nil || out != "3" {
				t.Fatalf("after deleting the child: %v %q, want the database and its 3 rows kept", err, out)
			}
			// CNPG's refusal reaches the condition: a child naming an owner
			// role that does not exist fails, with CNPG's message.
			broken := child.DeepCopy()
			broken.SetName("pg-broken")
			broken.Object["spec"] = applicationDatabaseSpec("pg", "other_db", "missing_role")
			if err := cl.Create(ctx, broken); err != nil {
				t.Fatal(err)
			}
			waitFor(t, 2*time.Minute, "CNPG to refuse the broken child", func() bool {
				if err := cl.Get(ctx, client.ObjectKeyFromObject(broken), broken); err != nil {
					return false
				}
				condition := extensionsCondition(broken, 1)
				return condition.Status == metav1.ConditionFalse && condition.Message != ""
			})
			// Re-adopting an existing database with vector installed changes nothing.
			applyChild()
			if out, err := asOwner("SELECT count(*), (SELECT count(*) FROM pg_extension WHERE extname = 'vector') FROM items"); err != nil || out != "3|1" {
				t.Fatalf("after re-adopting: %v %q, want the 3 rows and vector kept", err, out)
			}

		})
	}
}

func waitFor(t *testing.T, timeout time.Duration, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		if done() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}
