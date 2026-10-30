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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m171 t004: postgres_cluster_leader_changed records once per promotion of
// a new primary — none at create, none on a resync or an unrelated status
// change, none for an older re-read another bex-api replica commits late.

// primaryDatabase is a Ready Database whose CNPG primary is `primary`,
// promoted at `since` (CNPG's currentPrimaryTimestamp format).
func primaryDatabase(name, workspace string, readyAt time.Time, primary string, since time.Time) *appv1alpha1.Database {
	db := readyDatabase(readyAt)
	db.Name = name
	db.Labels = map[string]string{LabelTenant: workspace}
	db.Status.CurrentPrimary = primary
	if !since.IsZero() {
		db.Status.CurrentPrimaryTimestamp = since.UTC().Format("2006-01-02T15:04:05.000000Z")
	}
	return db
}

func leaderFactsOf(facts []DatastoreEventFact) []DatastoreEventFact {
	var out []DatastoreEventFact
	for _, fact := range facts {
		if fact.Type == DatastoreFactPostgresClusterLeaderChanged {
			out = append(out, fact)
		}
	}
	return out
}

func TestObservedDatabaseCarriesThePrimaryAndItsPromotion(t *testing.T) {
	since := time.Date(2026, 10, 29, 10, 0, 0, 123456789, time.UTC)
	obs, ok := observedDatabaseStateFor(primaryDatabase("dpg-leaderobs000000001", testWorkspace, since, "dpg-leaderobs000000001-2", since))
	if !ok {
		t.Fatal("no observation")
	}
	if obs.CurrentPrimary != "dpg-leaderobs000000001-2" || !obs.CurrentPrimarySince.Equal(since.Truncate(time.Microsecond)) {
		t.Fatalf("observed %q @ %v", obs.CurrentPrimary, obs.CurrentPrimarySince)
	}
}

func TestClusterLeaderChangedOncePerPromotion(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	const name = "dpg-leadermem000000001"
	t0 := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	at := t0
	observe := func(primary string, since time.Time) []DatastoreEventFact {
		t.Helper()
		at = at.Add(30 * time.Second)
		obs, _ := observedDatabaseStateFor(primaryDatabase(name, testWorkspace, t0, primary, since))
		obs.At = at
		facts, err := st.RecordObservedDatastoreState(ctx, obs)
		if err != nil {
			t.Fatal(err)
		}
		return leaderFactsOf(facts)
	}

	if got := observe(name+"-1", t0); len(got) != 0 {
		t.Fatalf("create recorded %+v", got)
	}
	if got := observe(name+"-1", t0); len(got) != 0 {
		t.Fatalf("resync recorded %+v", got)
	}
	t1 := t0.Add(10 * time.Minute)
	got := observe(name+"-2", t1)
	if len(got) != 1 || got[0].LeaderID != name+"-2" || !got[0].At.Equal(t1) {
		t.Fatalf("failover recorded %+v, want one change to -2 at the promotion", got)
	}
	if got := observe(name+"-2", t1); len(got) != 0 {
		t.Fatalf("resync after failover recorded %+v", got)
	}
	// Another replica commits a read from before the failover: older promotion,
	// so neither a change back nor (on the next pass) a change forward again.
	if got := observe(name+"-1", t0); len(got) != 0 {
		t.Fatalf("stale re-read recorded %+v", got)
	}
	if got := observe(name+"-2", t1); len(got) != 0 {
		t.Fatalf("pass after the stale re-read recorded %+v", got)
	}
	// An unreported primary (CNPG status unread) moves nothing.
	if got := observe("", time.Time{}); len(got) != 0 {
		t.Fatalf("unreported primary recorded %+v", got)
	}
	if got := observe(name+"-2", t1); len(got) != 0 {
		t.Fatalf("primary reported again recorded %+v", got)
	}
	// The failover verb's switchover back is a second, distinct change.
	t2 := t1.Add(10 * time.Minute)
	if got := observe(name+"-1", t2); len(got) != 1 || got[0].LeaderID != name+"-1" {
		t.Fatalf("switchover back recorded %+v", got)
	}
}

// Without CNPG's promotion time (an operator that predates it) a change still
// records, once; a checkpoint that predates the latch adopts the current
// primary silently.
func TestClusterLeaderChangedWithoutPromotionTime(t *testing.T) {
	previous := datastoreCheckpointExtras{}
	obs := ObservedDatastoreState{DatastoreID: "dpg-x", WorkspaceID: testWorkspace, Kind: DatastoreKindPostgres, At: time.Now(), CurrentPrimary: "dpg-x-1"}
	next := previous
	if got := leaderFacts(obs, previous, &next); len(got) != 0 || next.CurrentPrimary != "dpg-x-1" {
		t.Fatalf("pre-latch checkpoint: facts %+v, latch %q", got, next.CurrentPrimary)
	}
	previous = next
	obs.CurrentPrimary = "dpg-x-2"
	if got := leaderFacts(obs, previous, &next); len(got) != 1 || got[0].LeaderID != "dpg-x-2" || !got[0].At.Equal(obs.At) {
		t.Fatalf("untimed change: %+v", got)
	}
	// A KeyValue has no primary to report.
	kv := obs
	kv.Kind = DatastoreKindKeyValue
	if got, _ := observedDatastoreLifecycleFacts(kv, previous); len(got) != 0 {
		t.Fatalf("key value recorded %+v", got)
	}
}

// TestClusterLeaderChangedMixedPromotionTimes walks the edges where only one
// side of the comparison has CNPG's promotion time: an untimed read never
// erases the latched time, an untimed change still records (fail open) but
// keeps the old time so a later timed re-read of the old primary is still
// recognized as stale, a latch without a time accepts the first timed change,
// and an unparseable timestamp counts as unreported rather than as the zero
// time.
func TestClusterLeaderChangedMixedPromotionTimes(t *testing.T) {
	t0 := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Minute)
	base := ObservedDatastoreState{DatastoreID: "dpg-x", WorkspaceID: testWorkspace, Kind: DatastoreKindPostgres, At: t1.Add(time.Hour)}
	step := func(previous datastoreCheckpointExtras, primary string, since time.Time) ([]DatastoreEventFact, datastoreCheckpointExtras) {
		obs := base
		obs.CurrentPrimary, obs.CurrentPrimarySince = primary, since
		next := previous
		return leaderFacts(obs, previous, &next), next
	}

	latched := datastoreCheckpointExtras{CurrentPrimary: "dpg-x-1", CurrentPrimarySince: t0}
	got, next := step(latched, "dpg-x-1", time.Time{})
	if len(got) != 0 || next.CurrentPrimary != "dpg-x-1" || !next.CurrentPrimarySince.Equal(t0) {
		t.Fatalf("untimed resync: facts %+v, latch %q @ %v (want -1 @ %v kept)", got, next.CurrentPrimary, next.CurrentPrimarySince, t0)
	}

	got, next = step(latched, "dpg-x-2", time.Time{})
	if len(got) != 1 || got[0].LeaderID != "dpg-x-2" || !got[0].At.Equal(base.At) {
		t.Fatalf("untimed change: %+v, want one change to -2 at the observation time", got)
	}
	if next.CurrentPrimary != "dpg-x-2" || !next.CurrentPrimarySince.Equal(t0) {
		t.Fatalf("untimed change latched %q @ %v, want -2 keeping %v", next.CurrentPrimary, next.CurrentPrimarySince, t0)
	}
	if got, after := step(next, "dpg-x-1", t0); len(got) != 0 || after.CurrentPrimary != "dpg-x-2" {
		t.Fatalf("timed stale re-read after an untimed change: facts %+v, latch %q", got, after.CurrentPrimary)
	}
	if got, after := step(next, "dpg-x-2", t1); len(got) != 0 || !after.CurrentPrimarySince.Equal(t1) {
		t.Fatalf("time arriving for the latched primary: facts %+v, latch since %v (want %v)", got, after.CurrentPrimarySince, t1)
	}

	untimed := datastoreCheckpointExtras{CurrentPrimary: "dpg-x-1"}
	got, next = step(untimed, "dpg-x-2", t1)
	if len(got) != 1 || !got[0].At.Equal(t1) || next.CurrentPrimary != "dpg-x-2" || !next.CurrentPrimarySince.Equal(t1) {
		t.Fatalf("first timed change over an untimed latch: facts %+v, latch %q @ %v", got, next.CurrentPrimary, next.CurrentPrimarySince)
	}
	if got[0].SourceKey != fmt.Sprintf("dpg-x:leader:dpg-x-2:%d", t1.UnixNano()) {
		t.Fatalf("source key %q must name the promotion", got[0].SourceKey)
	}

	db := primaryDatabase("dpg-x", testWorkspace, t0, "dpg-x-2", time.Time{})
	db.Status.CurrentPrimaryTimestamp = "not-a-time"
	obs, ok := observedDatabaseStateFor(db)
	if !ok || obs.CurrentPrimary != "dpg-x-2" || !obs.CurrentPrimarySince.IsZero() {
		t.Fatalf("unparseable timestamp observed %q @ %v, want -2 with no time", obs.CurrentPrimary, obs.CurrentPrimarySince)
	}
}

// TestPGClusterLeaderChangedOncePerPromotion is the same contract on real
// Postgres through the reconciler's own path: the checkpoint columns migration
// 0173 adds round-trip, two replicas recording the same promotion at once
// store one row, unrelated status updates record nothing, and the fact
// hydrates by its evt- id with the new primary as leaderId.
func TestPGClusterLeaderChangedOncePerPromotion(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	rec := &Reconciler{Store: st}
	name := "dpg-" + fmt.Sprintf("%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})
	t0 := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	observe := func(db *appv1alpha1.Database, ticks int) {
		t.Helper()
		obs, ok := observedDatabaseStateFor(db)
		if !ok {
			t.Fatal("no observation")
		}
		for range ticks {
			rec.recordDatastoreObservation(ctx, obs)
		}
	}
	db := func(primary string, since time.Time) *appv1alpha1.Database {
		return primaryDatabase(name, tenant.ID, t0, primary, since)
	}

	observe(db(name+"-1", t0), 3) // create + resyncs
	// Unrelated status updates: an outage and its recovery, and a backup.
	down := downDatabase(t0.Add(5 * time.Minute))
	down.Name, down.Labels = name, map[string]string{LabelTenant: tenant.ID}
	down.Status.CurrentPrimary, down.Status.CurrentPrimaryTimestamp = name+"-1", t0.Format(time.RFC3339Nano)
	observe(down, 3)
	backedUp := db(name+"-1", t0)
	backedUp.Status.LastBackup = &appv1alpha1.DatabaseLastBackupStatus{Name: "nightly-1", Phase: "completed", CompletedAt: t0.Add(6 * time.Minute).Format(time.RFC3339)}
	observe(backedUp, 2)

	// The failover: two replicas record the same promotion concurrently.
	t1 := t0.Add(10 * time.Minute)
	promoted, _ := observedDatabaseStateFor(db(name+"-2", t1))
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.RecordObservedDatastoreState(ctx, promoted); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	observe(db(name+"-2", t1), 2) // resyncs
	observe(db(name+"-1", t0), 2) // a replica's late, older read
	observe(db(name+"-2", t1), 1)
	t2 := t1.Add(10 * time.Minute)
	observe(db(name+"-1", t2), 2) // the failover verb's switchover back

	var leaders []string
	rows, err := pool.Query(ctx, `SELECT leader_id, at FROM datastore_event_facts
		WHERE datastore_id = $1 AND fact_type = 'postgres_cluster_leader_changed' ORDER BY at`, name)
	if err != nil {
		t.Fatal(err)
	}
	var ats []time.Time
	for rows.Next() {
		var leader string
		var at time.Time
		if err := rows.Scan(&leader, &at); err != nil {
			t.Fatal(err)
		}
		leaders, ats = append(leaders, leader), append(ats, at)
	}
	rows.Close()
	if len(leaders) != 2 || leaders[0] != name+"-2" || leaders[1] != name+"-1" {
		t.Fatalf("leader changes = %v, want [-2 -1]", leaders)
	}
	if !ats[0].Equal(t1) || !ats[1].Equal(t2) {
		t.Fatalf("leader change times = %v, want the promotions %v, %v", ats, t1, t2)
	}

	// Delivered on the webhook feed, and hydrated by evt- id with leaderId.
	feed, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), []string{}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	hydrated := 0
	for _, row := range feed {
		if row.FactType != string(DatastoreFactPostgresClusterLeaderChanged) {
			continue
		}
		if row.ServiceID != name {
			t.Fatalf("feed row service = %q, want %q", row.ServiceID, name)
		}
		lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, row.Key))
		if err != nil {
			t.Fatalf("hydrate: %v", err)
		}
		if lookup.ServiceID != name || lookup.Event.LeaderID == "" || lookup.Event.FactType != row.FactType {
			t.Fatalf("hydrated %+v", lookup)
		}
		if _, err := st.GetServiceEvent(ctx, "tea-foreign00000000", ids.Derive(ids.Event, row.Key)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign workspace lookup = %v", err)
		}
		hydrated++
	}
	if hydrated != 2 {
		t.Fatalf("feed carried %d leader changes, want 2", hydrated)
	}
}

// TestPGClusterLeaderChangedWithoutPromotionTime: with no promotion time the
// checkpoint's current_primary_since stays NULL through real Postgres, a
// change still records once, a resync records nothing, and a promotion time
// arriving later for the same primary records nothing either.
func TestPGClusterLeaderChangedWithoutPromotionTime(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	name := "dpg-" + fmt.Sprintf("%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})
	t0 := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	at := t0
	observe := func(primary string, since time.Time) []DatastoreEventFact {
		t.Helper()
		at = at.Add(time.Minute)
		obs, ok := observedDatabaseStateFor(primaryDatabase(name, tenant.ID, t0, primary, since))
		if !ok {
			t.Fatal("no observation")
		}
		obs.At = at
		facts, err := st.RecordObservedDatastoreState(ctx, obs)
		if err != nil {
			t.Fatal(err)
		}
		return leaderFactsOf(facts)
	}

	if got := observe(name+"-1", time.Time{}); len(got) != 0 {
		t.Fatalf("untimed create recorded %+v", got)
	}
	var since *time.Time
	if err := pool.QueryRow(ctx, `SELECT current_primary_since FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name).Scan(&since); err != nil {
		t.Fatal(err)
	}
	if since != nil {
		t.Fatalf("untimed latch stored current_primary_since %v, want NULL", *since)
	}
	if got := observe(name+"-2", time.Time{}); len(got) != 1 || got[0].LeaderID != name+"-2" || !got[0].At.Equal(at) {
		t.Fatalf("untimed failover recorded %+v, want one change to -2 at %v", got, at)
	}
	if got := observe(name+"-2", time.Time{}); len(got) != 0 {
		t.Fatalf("untimed resync recorded %+v", got)
	}
	if got := observe(name+"-2", t0.Add(30*time.Minute)); len(got) != 0 {
		t.Fatalf("promotion time arriving for the latched primary recorded %+v", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM datastore_event_facts WHERE datastore_id = $1 AND fact_type = 'postgres_cluster_leader_changed'`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("stored %d leader changes, want 1", n)
	}
}
