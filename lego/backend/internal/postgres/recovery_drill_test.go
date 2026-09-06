//go:build e2e

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

package postgres

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestManagedPostgresRecoveryDrill provisions only new, disposable Databases.
// It intentionally exercises Database.spec.recovery, not a hand-built CNPG
// recovery Cluster, so a healthy backup cannot hide a broken bex projection.
// See ADR031 for the explicit invocation and cleanup contract.
func TestManagedPostgresRecoveryDrill(t *testing.T) {
	ns := os.Getenv("BEX_PG_DRILL_NAMESPACE")
	if ns == "" {
		t.Skip("opt-in live drill: set BEX_PG_DRILL_NAMESPACE and KUBECONFIG")
	}
	if kind, ok := id.KindOf(ns); !ok || kind != id.Workspace {
		t.Fatal("drill namespace must be a workspace id")
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Fatal("an explicit KUBECONFIG is required")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Timeout = 20 * time.Second
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, rbacv1.AddToScheme, appv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Minute)
	defer cancel()
	poll := func(stage string, check func(context.Context) (bool, error)) {
		t.Helper()
		start := time.Now()
		if err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 12*time.Minute, true, check); err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		t.Logf("%s: passed in %s", stage, time.Since(start).Round(time.Second))
	}
	quotaKey := client.ObjectKey{Namespace: ns, Name: "tenant-quota"}
	quota := &corev1.ResourceQuota{}
	if err := cl.Get(ctx, quotaKey, quota); err != nil {
		t.Fatal(err)
	}
	quotaResource := corev1.ResourceName("count/databases.app.bex.co")
	used, hard := quota.Status.Used[quotaResource], quota.Spec.Hard[quotaResource]
	baseline := used.Value()
	if hard.Value()-baseline < 2 {
		t.Fatal("drill needs quota for two temporary databases")
	}
	sourceID, targetID := id.New(id.Postgres), id.New(id.Postgres)
	t.Logf("disposable source=%s target=%s namespace=%s", sourceID, targetID, ns)
	created := []*appv1alpha1.Database{}
	// Register before any create; UID preconditions prevent deletion of a
	// replacement resource. The operator finalizer must finish archive purging.
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		for i := len(created) - 1; i >= 0; i-- {
			db := created[i]
			if err := cl.Delete(cleanupCtx, db, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &db.UID}}); err != nil && !apierrors.IsNotFound(err) {
				t.Errorf("cleanup %s: %v", db.Name, err)
				continue
			}
		}
		// Request both deletions before waiting: one stuck finalizer must not
		// prevent cleanup from even starting for the other disposable DB.
		for i := len(created) - 1; i >= 0; i-- {
			db := created[i]
			if err := wait.PollUntilContextCancel(cleanupCtx, 3*time.Second, true, func(ctx context.Context) (bool, error) {
				got := &appv1alpha1.Database{}
				err := cl.Get(ctx, client.ObjectKeyFromObject(db), got)
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			}); err != nil {
				t.Errorf("cleanup/finalizer %s: %v", db.Name, err)
				continue
			}
			if err := wait.PollUntilContextCancel(cleanupCtx, 3*time.Second, true, func(ctx context.Context) (bool, error) {
				cluster := pgDrillObject(ns, db.Name, schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster"})
				err := cl.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
				if err != nil && !apierrors.IsNotFound(err) {
					return false, err
				}
				if err == nil {
					return false, nil
				}
				pvcs := &corev1.PersistentVolumeClaimList{}
				if err := cl.List(ctx, pvcs, client.InNamespace(ns), client.MatchingLabels{labelCNPGCluster: db.Name}); err != nil {
					return false, err
				}
				return len(pvcs.Items) == 0, nil
			}); err != nil {
				t.Errorf("cleanup children %s: %v", db.Name, err)
			} else {
				t.Logf("removed %s, its Cluster/PVCs; archive-purge finalizer completed", db.Name)
			}
		}
	})
	createDB := func(name, display string, recovery *appv1alpha1.DatabaseRecovery) *appv1alpha1.Database {
		t.Helper()
		db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: map[string]string{"app.bex.co/workspace": ns}}, Spec: appv1alpha1.DatabaseSpec{Name: display, Plan: "basic-256mb", Version: "16", Recovery: recovery}}
		if err := cl.Create(ctx, db); err != nil {
			t.Fatal(err)
		}
		created = append(created, db.DeepCopy())
		poll("Ready "+name, func(ctx context.Context) (bool, error) {
			if err := cl.Get(ctx, client.ObjectKeyFromObject(db), db); err != nil {
				return false, err
			}
			return db.Status.Phase == appv1alpha1.DBPhaseReady, nil
		})
		return db
	}
	source := createDB(sourceID, "recovery-drill-source", nil)
	poll("source backup wiring and quota", func(ctx context.Context) (bool, error) {
		for _, obj := range []client.Object{
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: sourceID + "-app", Namespace: ns}},
			&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: sourceID + "-barman-cloud", Namespace: ns}},
		} {
			if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
		}
		store := pgDrillObject(ns, tenantBackupObjectStoreName, barmanCloudObjectStoreGVK)
		if err := cl.Get(ctx, client.ObjectKeyFromObject(store), store); err != nil {
			return false, err
		}
		secretName, _, err := unstructured.NestedString(store.Object, "spec", "configuration", "s3Credentials", "accessKeyId", "name")
		if err != nil {
			return false, err
		}
		if secretName == "" {
			return false, fmt.Errorf("ObjectStore has no backup credential reference")
		}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: secretName}, &corev1.Secret{}); err != nil {
			return false, err
		}
		if err := cl.Get(ctx, quotaKey, quota); err != nil {
			return false, err
		}
		charged := quota.Status.Used[quotaResource]
		return charged.Value() >= baseline+1, nil
	})
	sql := func(database, query string) string {
		t.Helper()
		pods := &corev1.PodList{}
		if err := cl.List(ctx, pods, client.InNamespace(ns), client.MatchingLabels{labelCNPGCluster: database, "role": "primary"}); err != nil {
			t.Fatal(err)
		}
		if len(pods.Items) != 1 {
			t.Fatalf("%s: expected one primary pod, got %d", database, len(pods.Items))
		}
		cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cmdCtx, "kubectl", "--kubeconfig", kubeconfig, "-n", ns, "exec", pods.Items[0].Name, "-c", "postgres", "--", "psql", "-X", "-U", "postgres", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-Atc", query)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("SQL on disposable %s: %v: %s", database, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	sql(sourceID, "CREATE TABLE public.bex_recovery_drill(marker text PRIMARY KEY); INSERT INTO public.bex_recovery_drill VALUES ('"+sourceID+"');")
	backup := pgDrillObject(ns, sourceID+"-drill", cnpgBackupGVK)
	backup.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "Database", Name: source.Name, UID: source.UID}})
	backup.Object["spec"] = map[string]any{"method": "plugin", "pluginConfiguration": map[string]any{"name": "barman-cloud.cloudnative-pg.io"}, "cluster": map[string]any{"name": sourceID}}
	if err := cl.Create(ctx, backup); err != nil {
		t.Fatal(err)
	}
	poll("fresh source backup", func(ctx context.Context) (bool, error) {
		if err := cl.Get(ctx, client.ObjectKeyFromObject(backup), backup); err != nil {
			return false, err
		}
		phase, _, err := unstructured.NestedString(backup.Object, "status", "phase")
		if phase == "failed" {
			return false, fmt.Errorf("disposable backup failed")
		}
		return phase == "completed", err
	})
	// Intentionally omit SourceBackupServerName: this is the default path that
	// previously selected the recovery target's empty archive instead of source.
	createDB(targetID, "recovery-drill-target", &appv1alpha1.DatabaseRecovery{SourceDatabase: sourceID})
	got := sql(targetID, "SELECT marker FROM public.bex_recovery_drill;")
	if got != sourceID {
		t.Fatalf("recovered marker mismatch: got %q, want %q", got, sourceID)
	}
	t.Log("Database.spec.recovery restored the exact marker from a fresh source backup")
	// Record only non-sensitive aggregate evidence for the drill ledger.
	t.Logf("initial managed-database quota usage=%d", baseline)
}

func pgDrillObject(namespace, name string, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}
