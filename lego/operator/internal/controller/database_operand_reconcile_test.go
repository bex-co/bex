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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func reconcileOperandFixture(t *testing.T, major string, annotated bool) (*DatabaseReconciler, *appv1alpha1.Database, *unstructured.Unstructured, clusterParams) {
	t.Helper()
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "operand-keep", Namespace: "default", UID: "owned-db"}, Spec: appv1alpha1.DatabaseSpec{Plan: "free"}}
	r := databaseStorageFixture(t, db, 1)
	plan, _ := resolvePlan(db.Spec)
	params := clusterParams{plan: plan, storageGB: 1, version: major, dbname: "app", owner: "app", namespace: db.Namespace}
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(db), cluster); err != nil {
		t.Fatal(err)
	}
	spec := cnpgClusterSpec(params)
	if annotated {
		if err := projectUnstructuredSpec(cluster, spec); err != nil {
			t.Fatal(err)
		}
	} else {
		cluster.Object["spec"] = spec
	}
	if err := r.Update(context.Background(), cluster); err != nil {
		t.Fatal(err)
	}
	params.version = ""
	return r, db, cluster, params
}

func TestUnversionedReconcileKeepsTheLiveImageAcrossProjection(t *testing.T) {
	withBexOperands(t, map[string]string{"18": "ghcr.io/bex-co/bex-operator:18.6-postgres-reviewed@sha256:0123"})
	for _, annotated := range []bool{false, true} {
		for _, observed := range []string{"", "14"} {
			t.Run(strings.Join([]string{map[bool]string{false: "legacy", true: "projected"}[annotated], "observed" + observed}, "-"), func(t *testing.T) {
				r, db, cluster, params := reconcileOperandFixture(t, "14", annotated)
				params.runningVersion = observed
				for range 2 {
					if err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
						t.Fatal(err)
					}
					if err := r.Get(context.Background(), client.ObjectKeyFromObject(db), cluster); err != nil {
						t.Fatal(err)
					}
					image, _, _ := unstructured.NestedString(cluster.Object, "spec", "imageName")
					if image != cnpgExportImages["14"] {
						t.Fatalf("projection withdrew live PG14 image %q and could select a different default major", image)
					}
				}
			})
		}
	}
}

func TestUnversionedReconcileUsesObservedMajorForTheFullProfile(t *testing.T) {
	const pg14 = "ghcr.io/bex-co/bex-operator:14.24-postgres-reviewed@sha256:0123"
	const pg18 = "ghcr.io/bex-co/bex-operator:18.6-postgres-reviewed@sha256:4567"
	for _, major := range []string{"14", "18"} {
		t.Run(major, func(t *testing.T) {
			// Create the existing upstream projection before adopting either pin.
			r, db, cluster, params := reconcileOperandFixture(t, major, true)
			withBexOperands(t, map[string]string{"14": pg14, "18": pg18})
			params.runningVersion = major
			if err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
				t.Fatal(err)
			}
			if err := r.Get(context.Background(), client.ObjectKeyFromObject(db), cluster); err != nil {
				t.Fatal(err)
			}
			image, _, _ := unstructured.NestedString(cluster.Object, "spec", "imageName")
			settings, _, _ := unstructured.NestedStringMap(cluster.Object, "spec", "postgresql", "parameters")
			preload, _, _ := unstructured.NestedSlice(cluster.Object, "spec", "postgresql", "shared_preload_libraries")
			if image != bexOperandImages[major] || settings["extwlist.extensions"] != strings.Join(pgExtensionAllowlist[major], ",") || settings["shared_buffers"] != "64MB" {
				t.Fatalf("observed major profile: image=%s settings=%v", image, settings)
			}
			duck := major == "18"
			if strings.Contains(strings.Join(anyStrings(preload), ","), "pg_duckdb") != duck || (settings["duckdb.postgres_role"] == "app") != duck {
				t.Fatalf("wrong major preload/owner projection: %v %v", preload, settings)
			}
		})
	}
}

func TestUnobservedMajorKeepsAnOlderOperandProfileUntilObservation(t *testing.T) {
	const oldImage = "ghcr.io/bex-co/bex-operator:14.24-postgres-old@sha256:0123"
	withBexOperands(t, map[string]string{"14": oldImage})
	for _, annotated := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "projected"}[annotated], func(t *testing.T) {
			bexOperandImages["14"] = oldImage
			r, db, cluster, params := reconcileOperandFixture(t, "14", annotated)
			if annotated {
				// This live default is absent from the prior owned projection.
				if err := unstructured.SetNestedField(cluster.Object, "logical", "spec", "postgresql", "parameters", "wal_level"); err != nil {
					t.Fatal(err)
				}
				if err := r.Update(context.Background(), cluster); err != nil {
					t.Fatal(err)
				}
			}
			before, _, err := unstructured.NestedMap(cluster.Object, "spec", "postgresql")
			if err != nil {
				t.Fatal(err)
			}
			bexOperandImages["14"] = "ghcr.io/bex-co/bex-operator:14.24-postgres-new@sha256:4567"
			if err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
				t.Fatal(err)
			}
			if err := r.Get(context.Background(), client.ObjectKeyFromObject(db), cluster); err != nil {
				t.Fatal(err)
			}
			image, _, _ := unstructured.NestedString(cluster.Object, "spec", "imageName")
			after, _, err := unstructured.NestedMap(cluster.Object, "spec", "postgresql")
			if err != nil || image != oldImage || !reflect.DeepEqual(before, after) {
				t.Fatalf("unknown major split its old runtime from its extension profile: image=%s before=%v after=%v err=%v", image, before, after, err)
			}
			if annotated && strings.Contains(cluster.GetAnnotations()[annotationProjectedSpec], "wal_level") {
				t.Fatal("webhook default was adopted into platform-owned intent")
			}
			version := cluster.GetResourceVersion()
			if err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
				t.Fatal(err)
			}
			if err := r.Get(context.Background(), client.ObjectKeyFromObject(db), cluster); err != nil {
				t.Fatal(err)
			}
			if cluster.GetResourceVersion() != version {
				t.Fatal("unchanged old operand profile did not converge")
			}
		})
	}
}

func anyStrings(values []any) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		result = append(result, v.(string))
	}
	return result
}

func TestExportUsesOneSpecOrObservedMajorForImageAndHelper(t *testing.T) {
	const pg14 = "ghcr.io/bex-co/bex-operator:14.24-postgres-reviewed@sha256:0123"
	withBexOperands(t, map[string]string{"14": pg14})
	for _, spec := range []string{"", "17"} {
		db := exportTestDatabase(time.Now())
		db.Spec.Version = spec
		db.Status.CurrentVersion = "14"
		dump := exportJob(db, db.Spec.Exports[0], testStore).Spec.Template.Spec.InitContainers[0]
		if spec == "" {
			if dump.Image != pg14 || dump.Args[0] != "exec /usr/local/bin/bex-postgres-export" {
				t.Fatalf("observed14 export image/helper diverged: %+v", dump)
			}
		} else if dump.Image != cnpgExportImages["17"] || strings.Contains(dump.Args[0], "bex-postgres-export") {
			t.Fatalf("explicit17 intent lost precedence: %+v", dump)
		}
	}
}

func TestUnobservedExportWaitsButExistingCompletionIsNotStranded(t *testing.T) {
	now := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	db := exportTestDatabase(now)
	db.Spec.Version = ""
	r := newExportReconciler(t, &now, db)
	delay, err := r.reconcileExports(context.Background(), db, true)
	if err != nil || delay <= 0 {
		t.Fatalf("unknown major should requeue: %v %v", delay, err)
	}
	var jobs batchv1.JobList
	if err := r.List(context.Background(), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 || len(db.Status.Exports) != 1 || db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportCreated {
		t.Fatal("unknown existing major dispatched a guessed export")
	}
	db.Status.CurrentVersion = "14"
	if _, err := r.reconcileExports(context.Background(), db, true); err != nil {
		t.Fatal(err)
	}
	if err := r.List(context.Background(), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 1 || jobs.Items[0].Spec.Template.Spec.InitContainers[0].Image != cnpgExportImages["14"] {
		t.Fatal("observed14 did not unblock its own export")
	}
	job := jobs.Items[0]
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: "True"}}
	finished := metav1.NewTime(now)
	job.Status.CompletionTime = &finished
	if err := r.Status().Update(context.Background(), &job); err != nil {
		t.Fatal(err)
	}
	db.Status.CurrentVersion = ""
	if _, err := r.reconcileExports(context.Background(), db, true); err != nil {
		t.Fatal(err)
	}
	if db.Status.Exports[0].Phase != appv1alpha1.DatabaseExportAvailable {
		t.Fatal("existing completion was stranded by missing version status")
	}
}
