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
	"slices"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
					if _, err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
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
			if _, err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
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
			if _, err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
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
			if _, err := r.reconcileCluster(context.Background(), db, cluster, params); err != nil {
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
	if len(jobs.Items) != 1 || jobs.Items[0].Spec.Template.Spec.InitContainers[0].Image != cnpgExportImage("14") {
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

// operandRollFixture is a Cluster CloudNativePG serves from instance Pods
// that run whatever image the Cluster named when each was created, as
// CloudNativePG creates them.
type operandRollFixture struct {
	t       *testing.T
	r       *DatabaseReconciler
	db      *appv1alpha1.Database
	cluster *unstructured.Unstructured
	params  clusterParams
}

func (f operandRollFixture) instance(name, image string, phase corev1.PodPhase) {
	f.t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: f.cluster.GetNamespace(), Labels: map[string]string{
			cnpgClusterLabel: f.cluster.GetName(), cnpgPodRoleLabel: cnpgPodRoleInstance,
		}},
		Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "postgres", Image: image}}},
		Status: corev1.PodStatus{Phase: phase},
	}
	if err := f.r.Create(context.Background(), pod); err != nil {
		f.t.Fatal(err)
	}
}

// roll is CloudNativePG replacing one instance Pod: the new one runs the
// image the Cluster names now.
func (f operandRollFixture) roll(name string) {
	f.t.Helper()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: f.cluster.GetNamespace()}}
	if err := f.r.Delete(context.Background(), pod); err != nil {
		f.t.Fatal(err)
	}
	image, _, _ := unstructured.NestedString(f.cluster.Object, "spec", "imageName")
	f.instance(name, image, corev1.PodRunning)
}

// pass runs one reconcile and fails if the Cluster it leaves names pgextwlist
// as a connection preload while an instance still runs an upstream image:
// CloudNativePG's reload would make that instance refuse every new
// connection, its own status probe included, so it would never roll (w21/125).
func (f operandRollFixture) pass() (held bool, image, preload string) {
	f.t.Helper()
	held, err := f.r.reconcileCluster(context.Background(), f.db, f.cluster, f.params)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.r.Get(context.Background(), client.ObjectKeyFromObject(f.cluster), f.cluster); err != nil {
		f.t.Fatal(err)
	}
	image, _, _ = unstructured.NestedString(f.cluster.Object, "spec", "imageName")
	preload, _, _ = unstructured.NestedString(f.cluster.Object, "spec", "postgresql", "parameters", connectionPreload)
	assertNoUpstreamInstanceUnderPreload(f.t, f.r, f.cluster)
	return held, image, preload
}

func assertNoUpstreamInstanceUnderPreload(t *testing.T, r *DatabaseReconciler, cluster *unstructured.Unstructured) {
	t.Helper()
	preload, _, _ := unstructured.NestedString(cluster.Object, "spec", "postgresql", "parameters", connectionPreload)
	if preload == "" {
		return
	}
	var pods corev1.PodList
	if err := r.List(context.Background(), &pods, client.InNamespace(cluster.GetNamespace()), client.MatchingLabels{cnpgClusterLabel: cluster.GetName()}); err != nil {
		t.Fatal(err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning && strings.HasPrefix(pod.Spec.Containers[0].Image, "ghcr.io/cloudnative-pg/postgresql:") {
			t.Fatalf("%s carries %s = %q while instance %s still runs the upstream image %s",
				cluster.GetName(), connectionPreload, preload, pod.Name, pod.Spec.Containers[0].Image)
		}
	}
}

// TestAdoptingABexOperandMovesTheImageBeforeTheConnectionPreload drives an HA
// Cluster from the upstream PG16 image onto a bex operand, back by the
// rollback (the map emptied) and onto it again. The image change carries
// every restart-only setting, so the roll is the one restart. The preload
// follows by reload once no instance runs the upstream image, with no Pod
// deleted by hand, and a finished Pod left on the upstream image does not
// hold it back.
func TestAdoptingABexOperandMovesTheImageBeforeTheConnectionPreload(t *testing.T) {
	const bex = "ghcr.io/bex-co/bex-operator:16.16-postgres-reviewed@sha256:0123"
	upstream := cnpgExportImages["16"]
	withBexOperands(t, map[string]string{})
	r, db, cluster, params := reconcileOperandFixture(t, "16", true)
	params.version, params.highAvailability = "16", true
	f := operandRollFixture{t: t, r: r, db: db, cluster: cluster, params: params}
	if _, image, _ := f.pass(); image != upstream {
		t.Fatalf("before adoption: imageName %q, want %q", image, upstream)
	}
	f.instance("operand-keep-1", upstream, corev1.PodRunning)
	f.instance("operand-keep-2", upstream, corev1.PodRunning)
	f.instance("operand-keep-1-initdb", upstream, corev1.PodSucceeded)

	adopt := func(round string) {
		t.Helper()
		withBexOperands(t, map[string]string{"16": bex})
		held, image, preload := f.pass()
		shared, _, _ := unstructured.NestedStringSlice(cluster.Object, "spec", "postgresql", "shared_preload_libraries")
		allowlist, _, _ := unstructured.NestedString(cluster.Object, "spec", "postgresql", "parameters", "extwlist.extensions")
		if image != bex || !held || preload != "" || !slices.Contains(shared, "timescaledb") || allowlist == "" {
			t.Fatalf("%s, first pass: image %q held %v preload %q shared %v allowlist %q; want the operand, its restart-only settings and no preload yet",
				round, image, held, preload, shared, allowlist)
		}
		f.roll("operand-keep-2") // the standby first
		if held, _, preload := f.pass(); !held || preload != "" {
			t.Fatalf("%s, one instance rolled: held %v preload %q, want it held for the primary", round, held, preload)
		}
		f.roll("operand-keep-1")
		if held, _, preload := f.pass(); held || preload != "pgextwlist" {
			t.Fatalf("%s, every instance rolled: held %v preload %q, want pgextwlist", round, held, preload)
		}
		version := cluster.GetResourceVersion()
		if f.pass(); cluster.GetResourceVersion() != version {
			t.Fatalf("%s: the adopted Cluster did not converge", round)
		}
	}
	adopt("adoption")

	withBexOperands(t, map[string]string{})
	if held, image, preload := f.pass(); held || image != upstream || preload != "" {
		t.Fatalf("rollback: held %v image %q preload %q, want the upstream image and no preload", held, image, preload)
	}
	f.roll("operand-keep-2")
	f.roll("operand-keep-1")
	f.pass()
	adopt("re-adoption")
}

// TestAClusterProjectedWithThePreloadBeforeItsRollRecovers: an operator
// before w21/125 wrote the image and the preload in one update. A Cluster left
// so, its instance still on the upstream image and refusing connections, gets
// the preload withdrawn on the next pass, which is what lets CloudNativePG
// roll it.
func TestAClusterProjectedWithThePreloadBeforeItsRollRecovers(t *testing.T) {
	const bex = "ghcr.io/bex-co/bex-operator:16.16-postgres-reviewed@sha256:0123"
	withBexOperands(t, map[string]string{"16": bex})
	r, db, cluster, params := reconcileOperandFixture(t, "16", true)
	params.version = "16"
	if preload, _, _ := unstructured.NestedString(cluster.Object, "spec", "postgresql", "parameters", connectionPreload); preload != "pgextwlist" {
		t.Fatalf("fixture: preload %q, want the one-update projection", preload)
	}
	f := operandRollFixture{t: t, r: r, db: db, cluster: cluster, params: params}
	f.instance("operand-keep-1", cnpgExportImages["16"], corev1.PodRunning)
	if held, image, preload := f.pass(); !held || image != bex || preload != "" {
		t.Fatalf("stuck cluster: held %v image %q preload %q, want the preload withdrawn", held, image, preload)
	}
	f.roll("operand-keep-1")
	if held, _, preload := f.pass(); held || preload != "pgextwlist" {
		t.Fatalf("rolled: held %v preload %q, want pgextwlist", held, preload)
	}
}

// TestAReadReplicaTakesThePrimarysPreloadOnlyOnItsOwnOperand: a replica
// cluster copies the primary's settings, but it rolls onto the operand on its
// own. So it gets the primary's connection preload only once its own instance
// runs the operand, and a new replica, with no instance yet, gets it at once.
func TestAReadReplicaTakesThePrimarysPreloadOnlyOnItsOwnOperand(t *testing.T) {
	const bex = "ghcr.io/bex-co/bex-operator:16.16-postgres-reviewed@sha256:0123"
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	primary := func() map[string]any {
		return cnpgClusterSpec(clusterParams{plan: plan, storageGB: 1, version: "16", dbname: "app", owner: "app", namespace: "default"})
	}
	withBexOperands(t, map[string]string{})
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "operand-keep", Namespace: "default", UID: "owned-db"}, Spec: appv1alpha1.DatabaseSpec{Plan: "free"}}
	r := databaseStorageFixture(t, db, 1)
	upstream := readReplicaSource{settled: true, major: "16", spec: primary()}
	replica, _, held, err := r.reconcileReadReplicaCluster(context.Background(), db, "r1", upstream)
	if err != nil || held {
		t.Fatalf("upstream replica: held %v err %v", held, err)
	}
	f := operandRollFixture{t: t, r: r, db: db, cluster: replica}
	f.instance(replica.GetName()+"-1", cnpgExportImages["16"], corev1.PodRunning)

	withBexOperands(t, map[string]string{"16": bex})
	adopted := readReplicaSource{settled: true, major: "16", spec: primary()}
	if preload, _, _ := unstructured.NestedString(adopted.spec, "postgresql", "parameters", connectionPreload); preload != "pgextwlist" {
		t.Fatalf("primary spec: preload %q, want pgextwlist", preload)
	}
	replicaPass := func(name string) (bool, string, string) {
		t.Helper()
		cluster, _, held, err := r.reconcileReadReplicaCluster(context.Background(), db, name, adopted)
		if err != nil {
			t.Fatal(err)
		}
		image, _, _ := unstructured.NestedString(cluster.Object, "spec", "imageName")
		preload, _, _ := unstructured.NestedString(cluster.Object, "spec", "postgresql", "parameters", connectionPreload)
		assertNoUpstreamInstanceUnderPreload(t, r, cluster)
		f.cluster = cluster
		return held, image, preload
	}
	if held, image, preload := replicaPass("r1"); !held || image != bex || preload != "" {
		t.Fatalf("replica on the upstream image: held %v image %q preload %q, want the operand without the preload", held, image, preload)
	}
	f.roll(replica.GetName() + "-1")
	if held, _, preload := replicaPass("r1"); held || preload != "pgextwlist" {
		t.Fatalf("replica rolled: held %v preload %q, want pgextwlist", held, preload)
	}
	if held, image, preload := replicaPass("r2"); held || image != bex || preload != "pgextwlist" {
		t.Fatalf("new replica: held %v image %q preload %q, want the operand with its preload", held, image, preload)
	}
}
