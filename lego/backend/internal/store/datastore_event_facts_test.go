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

package store

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const testWorkspace = "tea-datastore"

// databaseCR builds the Database status the reconciler's observation path
// reads, with an explicit Ready-condition LastTransitionTime — the
// operator-side timestamp the stale-conclusion guard orders on.
func databaseCR(phase appv1alpha1.DatabasePhase, ready metav1.ConditionStatus, reason string, transitionedAt time.Time) *appv1alpha1.Database {
	db := &appv1alpha1.Database{}
	db.Name = "dpg-test"
	db.Labels = map[string]string{LabelTenant: testWorkspace}
	db.Status.Phase = phase
	db.Status.Conditions = []metav1.Condition{{
		Type:               appv1alpha1.ConditionReady,
		Status:             ready,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(transitionedAt),
	}}
	return db
}

func readyDatabase(transitionedAt time.Time) *appv1alpha1.Database {
	return databaseCR(appv1alpha1.DBPhaseReady, metav1.ConditionTrue, datastoreReasonProvisioned, transitionedAt)
}

// downDatabase is the ambiguous state that makes the arming rule load-bearing:
// zero ready CNPG instances reports exactly this whether the cluster is being
// created for the first time or has just lost its only instance.
func downDatabase(transitionedAt time.Time) *appv1alpha1.Database {
	return databaseCR(appv1alpha1.DBPhaseProvisioning, metav1.ConditionFalse, "Provisioning", transitionedAt)
}

func keyValueCR(phase appv1alpha1.KeyValuePhase, ready metav1.ConditionStatus, reason string, transitionedAt time.Time) *appv1alpha1.KeyValue {
	kv := &appv1alpha1.KeyValue{}
	kv.Name = "red-test"
	kv.Labels = map[string]string{LabelTenant: testWorkspace}
	kv.Status.Phase = phase
	kv.Status.Conditions = []metav1.Condition{{
		Type:               appv1alpha1.ConditionReady,
		Status:             ready,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(transitionedAt),
	}}
	return kv
}

func driveDatabase(t *testing.T, rec *Reconciler, db *appv1alpha1.Database, ticks int) {
	t.Helper()
	obs, ok := observedDatabaseStateFor(db)
	if !ok {
		t.Fatalf("database %s produced no observation", db.Name)
	}
	for i := 0; i < ticks; i++ {
		rec.recordDatastoreObservation(context.Background(), obs)
	}
}

func driveKeyValue(t *testing.T, rec *Reconciler, kv *appv1alpha1.KeyValue, ticks int) {
	t.Helper()
	obs, ok := observedKeyValueStateFor(kv)
	if !ok {
		t.Fatalf("keyvalue %s produced no observation", kv.Name)
	}
	for i := 0; i < ticks; i++ {
		rec.recordDatastoreObservation(context.Background(), obs)
	}
}

func datastoreEdgeCounts(st *memStore) map[DatastoreEventFactType]int {
	st.mu.Lock()
	defer st.mu.Unlock()
	counts := map[DatastoreEventFactType]int{}
	for _, fact := range st.datastoreFacts {
		counts[fact.Type]++
	}
	return counts
}

func datastoreStaleRejections(t *testing.T, m *ReconcilerMetrics) float64 {
	t.Helper()
	return testutil.ToFloat64(m.observationRejections.WithLabelValues(rejectReasonStaleTransition, rejectSubjectDatastore))
}

// The whole point of the checkpoint: a level-triggered 30s poll observes the
// same outage and the same recovery many times, and each real transition must
// land as exactly one durable fact.
func TestObservedDatabaseEdgesRecordEachTransitionOnce(t *testing.T) {
	rec, st, _ := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	driveDatabase(t, rec, readyDatabase(base), 3)                     // baseline + steady healthy replays
	driveDatabase(t, rec, downDatabase(base.Add(10*time.Minute)), 3)  // the outage, re-observed across resyncs
	driveDatabase(t, rec, readyDatabase(base.Add(20*time.Minute)), 3) // the recovery, re-observed across resyncs

	counts := datastoreEdgeCounts(st)
	if counts[DatastoreFactPostgresUnavailable] != 1 || counts[DatastoreFactPostgresAvailable] != 1 || len(counts) != 2 {
		t.Fatalf("edges = %v, want exactly one postgres_unavailable and one postgres_available", counts)
	}
	for _, fact := range st.datastoreFacts {
		if fact.WorkspaceID != testWorkspace || fact.DatastoreID != "dpg-test" || fact.Kind != DatastoreKindPostgres {
			t.Fatalf("fact identity = %+v, want the CR's own workspace/id/kind", fact)
		}
	}
	if got := st.datastoreFacts[datastoreFactSourceKeyFor(t, st, DatastoreFactPostgresUnavailable)].ReasonCode; got != EventReasonReadinessFailed {
		t.Fatalf("postgres_unavailable reason = %q, want %q", got, EventReasonReadinessFailed)
	}
}

func datastoreFactSourceKeyFor(t *testing.T, st *memStore, typ DatastoreEventFactType) string {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	for key, fact := range st.datastoreFacts {
		if fact.Type == typ {
			return key
		}
	}
	t.Fatalf("no %s fact recorded", typ)
	return ""
}

// Suspension is intentional downtime and provisioning is not an outage: a
// datastore that has never been Ready must produce nothing at all, and the
// first Provisioned observation is what arms the edge for the outage after it.
func TestObservedDatastoreProvisioningAndSuspensionArmNothing(t *testing.T) {
	rec, st, _ := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	suspended := databaseCR(appv1alpha1.DBPhaseReady, metav1.ConditionFalse, appv1alpha1.ReasonSuspended, base)
	suspended.Spec.Suspended = true
	driveDatabase(t, rec, suspended, 2)
	driveDatabase(t, rec, downDatabase(base.Add(time.Minute)), 4) // never been Ready: provisioning, not an outage

	if counts := datastoreEdgeCounts(st); len(counts) != 0 {
		t.Fatalf("a datastore that was never Ready recorded %v, want nothing", counts)
	}

	// The first Provisioned arms the edge — and does so silently, because
	// finishing provisioning is not a recovery from an outage nobody reported.
	driveDatabase(t, rec, readyDatabase(base.Add(10*time.Minute)), 2)
	if counts := datastoreEdgeCounts(st); len(counts) != 0 {
		t.Fatalf("the first Provisioned emitted %v, want nothing", counts)
	}

	driveDatabase(t, rec, downDatabase(base.Add(20*time.Minute)), 2)
	counts := datastoreEdgeCounts(st)
	if counts[DatastoreFactPostgresUnavailable] != 1 || len(counts) != 1 {
		t.Fatalf("the armed outage recorded %v, want exactly one postgres_unavailable", counts)
	}
}

// The w6/m41 class, for datastores: an unhealthy conclusion whose Ready
// transition predates the recorded healthy checkpoint is a time-traveled
// re-read of a cached client, not a new outage. It records nothing however
// many consecutive ticks deliver it, and the counter says so under
// subject="datastore".
func TestStaleDatastoreConclusionRecordsNoPhantomPair(t *testing.T) {
	rec, st, metrics := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()
	recoveredAt := base.Add(10 * time.Minute)

	driveDatabase(t, rec, readyDatabase(recoveredAt), 1) // baseline + healthy checkpoint
	driveDatabase(t, rec, downDatabase(base), 2)         // crash-era conclusion, delivered twice
	driveDatabase(t, rec, readyDatabase(recoveredAt), 1)

	if counts := datastoreEdgeCounts(st); len(counts) != 0 {
		t.Fatalf("time-traveled conclusion recorded %v, want nothing", counts)
	}
	if got := datastoreStaleRejections(t, metrics); got != 2 {
		t.Fatalf("datastore stale rejections = %v, want 2 (one per refused tick)", got)
	}
	if got := staleRejections(t, metrics); got != 0 {
		t.Fatalf("a datastore rejection moved the app-subject counter to %v, want 0", got)
	}
}

// Render names a Postgres outage "unavailable" and a Key Value outage
// "unhealthy"; both ride the same path, and a suspended Key Value — which
// reports Ready=TRUE with the Suspended reason, unlike a suspended Database —
// must still record nothing.
func TestObservedKeyValueUsesTheKeyValueVocabulary(t *testing.T) {
	rec, st, _ := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	healthy := keyValueCR(appv1alpha1.KVPhaseReady, metav1.ConditionTrue, datastoreReasonProvisioned, base)
	down := keyValueCR(appv1alpha1.KVPhaseProvisioning, metav1.ConditionFalse, "PodUnready", base.Add(10*time.Minute))
	suspended := keyValueCR(appv1alpha1.KVPhaseReady, metav1.ConditionTrue, appv1alpha1.ReasonSuspended, base.Add(30*time.Minute))
	suspended.Spec.Suspended = true

	driveKeyValue(t, rec, healthy, 2)
	driveKeyValue(t, rec, down, 3)
	driveKeyValue(t, rec, keyValueCR(appv1alpha1.KVPhaseReady, metav1.ConditionTrue, datastoreReasonProvisioned, base.Add(20*time.Minute)), 2)
	driveKeyValue(t, rec, suspended, 2)

	counts := datastoreEdgeCounts(st)
	if counts[DatastoreFactKeyValueUnhealthy] != 1 || counts[DatastoreFactKeyValueAvailable] != 1 || len(counts) != 2 {
		t.Fatalf("edges = %v, want exactly one key_value_unhealthy and one key_value_available", counts)
	}
}

// A single-tick Ready=False must not reach the checkpoint; a sustained outage
// must, one tick late; a recovery must pass through immediately.
func TestDebounceDatastoreUnhealthySuppressesSingleTickBlips(t *testing.T) {
	once := map[string]bool{}
	unhealthy := ObservedDatastoreState{
		DatastoreID: "dpg-test", WorkspaceID: testWorkspace, Kind: DatastoreKindPostgres,
		Availability: "unhealthy", AvailabilityObserved: true, ReasonCode: EventReasonReadinessFailed,
	}
	healthy := ObservedDatastoreState{
		DatastoreID: "dpg-test", WorkspaceID: testWorkspace, Kind: DatastoreKindPostgres,
		Availability: "healthy", AvailabilityObserved: true,
	}

	if got := debounceDatastoreUnhealthy(unhealthy, once); got.AvailabilityObserved {
		t.Fatalf("first unhealthy tick recorded availability: %+v", got)
	}
	if got := debounceDatastoreUnhealthy(healthy, once); !got.AvailabilityObserved || got.Availability != "healthy" {
		t.Fatalf("healthy after a blip must pass through untouched: %+v", got)
	}
	if once["dpg-test"] {
		t.Fatal("blip left the datastore marked unhealthy-once")
	}

	if got := debounceDatastoreUnhealthy(unhealthy, once); got.AvailabilityObserved {
		t.Fatalf("first tick of a real outage recorded availability: %+v", got)
	}
	got := debounceDatastoreUnhealthy(unhealthy, once)
	if !got.AvailabilityObserved || got.Availability != "unhealthy" || got.ReasonCode != EventReasonReadinessFailed {
		t.Fatalf("second consecutive unhealthy tick must record: %+v", got)
	}
	if got := debounceDatastoreUnhealthy(healthy, once); !got.AvailabilityObserved {
		t.Fatalf("recovery must be immediate: %+v", got)
	}
}

// A major-version upgrade is planned downtime whose own postgres_upgrade_*
// facts describe it (t002); the availability dimension must stay silent so a
// scheduled maintenance window does not also page as an outage.
func TestMajorVersionUpgradeLeavesAvailabilityUnobserved(t *testing.T) {
	rec, st, _ := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	driveDatabase(t, rec, readyDatabase(base), 1)
	upgrading := databaseCR(appv1alpha1.DBPhaseUpgrading, metav1.ConditionFalse, datastoreReasonMajorVersionUpgrade, base.Add(10*time.Minute))
	driveDatabase(t, rec, upgrading, 3)
	driveDatabase(t, rec, readyDatabase(base.Add(20*time.Minute)), 2)

	if counts := datastoreEdgeCounts(st); len(counts) != 0 {
		t.Fatalf("a major-version upgrade without version fields recorded %v, want nothing (availability silent; upgrade facts need SpecVersion+CurrentVersion)", counts)
	}
}

// The observation pass reads CRs it does not project, so ownership has to come
// off the CR itself: a hand-applied datastore carries no tenant label and has
// no workspace whose feed an event could belong to.
func TestRecordDatastoreObservationsSkipsUnownedCRs(t *testing.T) {
	rec, st, cl := newTestReconciler(t)
	base := time.Now().Add(-time.Hour).UTC()

	owned := readyDatabase(base)
	unowned := readyDatabase(base)
	unowned.Name = "dpg-handapplied"
	unowned.Labels = nil
	for _, db := range []*appv1alpha1.Database{owned, unowned} {
		db.Namespace = "default"
		if err := cl.Create(context.Background(), db); err != nil {
			t.Fatalf("create Database: %v", err)
		}
	}
	if err := rec.recordDatastoreObservations(context.Background()); err != nil {
		t.Fatalf("record datastore observations: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.datastoreCheckpoints["dpg-test"]; !ok {
		t.Fatal("the labeled Database was not observed")
	}
	if _, ok := st.datastoreCheckpoints["dpg-handapplied"]; ok {
		t.Fatal("an unlabeled Database was attributed to a workspace")
	}
}
