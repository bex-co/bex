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
	"fmt"
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func replicaSyncDatabase(name, workspace string, readyAt time.Time, statuses ...appv1alpha1.DatabaseReadReplicaStatus) *appv1alpha1.Database {
	db := readyDatabase(readyAt)
	db.Name = name
	db.Labels = map[string]string{LabelTenant: workspace}
	db.Status.ReadReplicaStatuses = statuses
	return db
}

func staleReplicaFactsOf(facts []DatastoreEventFact) []DatastoreEventFact {
	var out []DatastoreEventFact
	for _, fact := range facts {
		if fact.Type == DatastoreFactPostgresReadReplicaStale {
			out = append(out, fact)
		}
	}
	return out
}

func TestObservedDatabaseCarriesStaleReadReplicas(t *testing.T) {
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	db := replicaSyncDatabase("dpg-repobs000000000001", testWorkspace, t0,
		appv1alpha1.DatabaseReadReplicaStatus{Name: "initial"},
		appv1alpha1.DatabaseReadReplicaStatus{Name: "hysteresis", Syncing: new(true), SyncStoppedAt: t0.Format(time.RFC3339)},
		appv1alpha1.DatabaseReadReplicaStatus{Name: "broken", Syncing: new(false), SyncStoppedAt: t0.Format(time.RFC3339)},
	)
	obs, ok := observedDatabaseStateFor(db)
	if !ok {
		t.Fatal("no observation")
	}
	if len(obs.StaleReadReplicas) != 1 || obs.StaleReadReplicas[0].Name != "broken" || !obs.StaleReadReplicas[0].StoppedAt.Equal(t0) {
		t.Fatalf("StaleReadReplicas = %+v, want only [broken @ %v]", obs.StaleReadReplicas, t0)
	}
}

func TestReadReplicaStaleOncePerEpisode(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	const name = "dpg-repmem000000000001"
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	at := t0
	observe := func(suspended bool, statuses ...appv1alpha1.DatabaseReadReplicaStatus) []DatastoreEventFact {
		t.Helper()
		at = at.Add(30 * time.Second)
		db := replicaSyncDatabase(name, testWorkspace, t0, statuses...)
		db.Spec.Suspended = suspended
		obs, ok := observedDatabaseStateFor(db)
		if !ok {
			t.Fatal("no observation")
		}
		obs.At = at
		facts, err := st.RecordObservedDatastoreState(ctx, obs)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return staleReplicaFactsOf(facts)
	}

	// Baseline: healthy replica records nothing.
	if got := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(true)}); len(got) != 0 {
		t.Fatalf("baseline: got %+v", got)
	}
	// Hysteresis (still Syncing=true with SyncStoppedAt stamped): records nothing.
	ep1 := t0.Add(time.Minute)
	if got := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(true), SyncStoppedAt: ep1.Format(time.RFC3339)}); len(got) != 0 {
		t.Fatalf("hysteresis: got %+v", got)
	}
	// Episode 1 crosses threshold (Syncing=false): records exactly one fact.
	got := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: ep1.Format(time.RFC3339)})
	if len(got) != 1 || got[0].Type != DatastoreFactPostgresReadReplicaStale {
		t.Fatalf("episode 1 = %+v, want 1 fact", got)
	}
	key1 := got[0].SourceKey
	// Repeated resyncs during same episode record nothing.
	for i := 0; i < 3; i++ {
		if dup := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: ep1.Format(time.RFC3339)}); len(dup) != 0 {
			t.Fatalf("steady stale pass %d = %+v, want 0", i, dup)
		}
	}
	// Recovery records nothing and re-arms.
	if rec := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(true)}); len(rec) != 0 {
		t.Fatalf("recovery = %+v, want 0", rec)
	}
	// Episode 2 records a second fact with a distinct key.
	ep2 := t0.Add(10 * time.Minute)
	got2 := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: ep2.Format(time.RFC3339)})
	if len(got2) != 1 || got2[0].SourceKey == key1 {
		t.Fatalf("episode 2 = %+v (key1=%q), want 1 new fact", got2, key1)
	}
	// Out-of-order re-read of older episode 1 neither emits nor moves latch backwards.
	if older := observe(false, appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: ep1.Format(time.RFC3339)}); len(older) != 0 {
		t.Fatalf("older re-read = %+v, want 0", older)
	}
	// Independent second replica "b" becoming stale while "a" stays stale records one fact for "b".
	gotB := observe(false,
		appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: ep2.Format(time.RFC3339)},
		appv1alpha1.DatabaseReadReplicaStatus{Name: "b", Syncing: new(false), SyncStoppedAt: ep2.Format(time.RFC3339)},
	)
	if len(gotB) != 1 {
		t.Fatalf("second replica = %+v, want 1", gotB)
	}
	// Suspended database never emits stale facts.
	if susp := observe(true, appv1alpha1.DatabaseReadReplicaStatus{Name: "c", Syncing: new(false), SyncStoppedAt: ep2.Format(time.RFC3339)}); len(susp) != 0 {
		t.Fatalf("suspended = %+v, want 0", susp)
	}
}

func TestReadReplicaStaleBaselineDoesNotEmit(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	db := replicaSyncDatabase("dpg-repbase00000000001", testWorkspace, t0,
		appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: t0.Format(time.RFC3339)},
	)
	obs, _ := observedDatabaseStateFor(db)
	obs.At = t0.Add(3 * time.Minute)
	if facts, err := st.RecordObservedDatastoreState(ctx, obs); err != nil || len(staleReplicaFactsOf(facts)) != 0 {
		t.Fatalf("baseline already-stale: facts=%+v err=%v", facts, err)
	}
	obs.At = obs.At.Add(30 * time.Second)
	if facts, err := st.RecordObservedDatastoreState(ctx, obs); err != nil || len(staleReplicaFactsOf(facts)) != 0 {
		t.Fatalf("second pass on baseline already-stale: facts=%+v err=%v", facts, err)
	}
}

func TestPGReadReplicaStaleFactsAndWebhookFeed(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	name := fmt.Sprintf("dpg-repstale-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})

	t0 := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	at := t0
	observe := func(statuses ...appv1alpha1.DatabaseReadReplicaStatus) []DatastoreEventFact {
		t.Helper()
		at = at.Add(30 * time.Second)
		db := replicaSyncDatabase(name, tenant.ID, t0, statuses...)
		obs, ok := observedDatabaseStateFor(db)
		if !ok {
			t.Fatal("no observation")
		}
		obs.At = at
		facts, err := st.RecordObservedDatastoreState(ctx, obs)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return staleReplicaFactsOf(facts)
	}

	// Baseline healthy.
	if got := observe(appv1alpha1.DatabaseReadReplicaStatus{Name: "reader", Syncing: new(true)}); len(got) != 0 {
		t.Fatalf("baseline = %+v", got)
	}

	// Concurrent observers seeing episode 1 produce exactly one row.
	ep1 := t0.Add(2 * time.Minute)
	db1 := replicaSyncDatabase(name, tenant.ID, t0, appv1alpha1.DatabaseReadReplicaStatus{
		Name: "reader", Syncing: new(false), SyncStoppedAt: ep1.Format(time.RFC3339),
	})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var emitted []DatastoreEventFact
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			obs, _ := observedDatabaseStateFor(db1)
			obs.At = ep1.Add(2 * time.Minute)
			facts, err := st.RecordObservedDatastoreState(ctx, obs)
			if err != nil {
				t.Errorf("concurrent record: %v", err)
				return
			}
			mu.Lock()
			emitted = append(emitted, staleReplicaFactsOf(facts)...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(emitted) != 1 {
		t.Fatalf("concurrent episode 1 emitted %d facts, want 1", len(emitted))
	}

	// Indexed in service_event_index and retrievable by evt- id.
	eventID := ids.Derive(ids.Event, "fact:"+emitted[0].SourceKey)
	lookup, err := st.GetServiceEvent(ctx, tenant.ID, eventID)
	if err != nil {
		t.Fatalf("GetServiceEvent(%s): %v", eventID, err)
	}
	if lookup.ServiceID != name || lookup.Event.FactType != string(DatastoreFactPostgresReadReplicaStale) {
		t.Fatalf("lookup = %+v", lookup)
	}

	// Visible on webhook feed.
	feed, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), nil, []string{tenant.ID}, 50)
	if err != nil {
		t.Fatalf("ListWebhookEvents: %v", err)
	}
	foundFeed := false
	for _, row := range feed {
		if row.ServiceID == name && row.FactType == string(DatastoreFactPostgresReadReplicaStale) {
			foundFeed = true
		}
	}
	if !foundFeed {
		t.Fatalf("webhook feed missing %s: %+v", DatastoreFactPostgresReadReplicaStale, feed)
	}

	// Recovery emits nothing; episode 2 emits a second row.
	if rec := observe(appv1alpha1.DatabaseReadReplicaStatus{Name: "reader", Syncing: new(true)}); len(rec) != 0 {
		t.Fatalf("recovery = %+v", rec)
	}
	ep2 := ep1.Add(5 * time.Minute)
	if ep2Facts := observe(appv1alpha1.DatabaseReadReplicaStatus{
		Name: "reader", Syncing: new(false), SyncStoppedAt: ep2.Format(time.RFC3339),
	}); len(ep2Facts) != 1 {
		t.Fatalf("episode 2 = %+v, want 1", ep2Facts)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM datastore_event_facts WHERE datastore_id = $1 AND fact_type = 'postgres_read_replica_stale'`, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("row count = %d, want 2", count)
	}
}
