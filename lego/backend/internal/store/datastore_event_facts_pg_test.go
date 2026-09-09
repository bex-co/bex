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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// openDatastoreTestStore is the shared prologue of the datastore observation
// PG tests: migrate, connect, and mint a workspace whose id the facts are
// scoped to. Datastore facts carry the workspace directly (no apps row), so a
// real tenant row is only needed for the webhook-feed assertion.
func openDatastoreTestStore(t *testing.T) (*PGStore, *pgxpool.Pool, Tenant) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := NewPGStore(pool)

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "datastore-edge-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	return st, pool, tenant
}

func datastoreFactTypes(t *testing.T, pool *pgxpool.Pool, datastoreID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT fact_type FROM datastore_event_facts WHERE datastore_id = $1 ORDER BY at, source_key`, datastoreID)
	if err != nil {
		t.Fatalf("list datastore facts: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var factType string
		if err := rows.Scan(&factType); err != nil {
			t.Fatalf("scan fact: %v", err)
		}
		out = append(out, factType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate facts: %v", err)
	}
	return out
}

// A managed Postgres that goes down and comes back, observed by the reconciler
// across many resync ticks, must land as exactly one postgres_unavailable +
// one postgres_available — the datastore_observed_checkpoints diff, not any
// producer-side care, is what keeps a level-triggered 30s poll from re-emitting
// the same edge. The edges must then be visible on the composed feed the
// outbound-webhook worker tails.
func TestPGObservedDatabaseEdgeEmitsExactlyOnePair(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	rec := &Reconciler{Store: st}

	base := time.Now().Add(-time.Hour).UTC()
	db := readyDatabase(base)
	db.Name = "dpg-" + fmt.Sprintf("%d", time.Now().UnixNano())
	db.Labels[LabelTenant] = tenant.ID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, db.Name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, db.Name)
	})

	retarget := func(src *appv1alpha1.Database) *appv1alpha1.Database {
		src.Name = db.Name
		src.Labels = map[string]string{LabelTenant: tenant.ID}
		return src
	}
	observe := func(src *appv1alpha1.Database, ticks int) {
		t.Helper()
		obs, ok := observedDatabaseStateFor(retarget(src))
		if !ok {
			t.Fatalf("no observation for %s", src.Name)
		}
		for i := 0; i < ticks; i++ {
			rec.recordDatastoreObservation(ctx, obs)
		}
	}

	observe(readyDatabase(base), 3)                     // baseline + steady healthy replays
	observe(downDatabase(base.Add(10*time.Minute)), 3)  // the outage, re-observed across resyncs
	observe(readyDatabase(base.Add(20*time.Minute)), 3) // the recovery, re-observed across resyncs

	got := datastoreFactTypes(t, pool, db.Name)
	want := []string{string(DatastoreFactPostgresUnavailable), string(DatastoreFactPostgresAvailable)}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("recorded %v, want %v", got, want)
	}

	rows, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), []string{}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	feed := map[string]string{}
	for _, row := range rows {
		if row.Source == EventSourceFact {
			feed[row.FactType] = row.ServiceID
		}
	}
	for _, factType := range want {
		if feed[factType] != db.Name {
			t.Fatalf("feed service id for %s = %q, want %q (feed: %v)", factType, feed[factType], db.Name, feed)
		}
	}

	// The retrievable evt-... index must carry the datastore facts too, or the
	// webhook delivery they produce has no source row to look up.
	var indexed int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM service_event_index WHERE source = 'fact' AND service_id = $1`, db.Name).Scan(&indexed); err != nil {
		t.Fatalf("count indexed facts: %v", err)
	}
	if indexed != len(want) {
		t.Fatalf("indexed %d datastore facts, want %d", indexed, len(want))
	}

	// …and the index entry must actually hydrate (w3/m82 t004). Being indexed
	// only makes the evt-… id resolvable; until getServiceEventQuery joined
	// datastore_event_facts the lookup found the index row, matched no source
	// table, and answered not-found — a delivered webhook whose data.id led
	// nowhere.
	hydrated := 0
	for _, row := range rows {
		if row.Source != EventSourceFact {
			continue
		}
		eventID := ids.Derive(ids.Event, row.Key)
		lookup, err := st.GetServiceEvent(ctx, tenant.ID, eventID)
		if err != nil {
			t.Fatalf("hydrate %s: %v", row.FactType, err)
		}
		if lookup.ServiceID != db.Name || lookup.Event.FactType != row.FactType || lookup.Event.Source != EventSourceFact {
			t.Errorf("hydrated %+v, want the %s fact on %s", lookup, row.FactType, db.Name)
		}
		if !lookup.Event.At.Equal(row.At) {
			t.Errorf("hydrated %s at %v, want %v", row.FactType, lookup.Event.At, row.At)
		}
		// A foreign workspace must still see nothing: the fact carries its own
		// workspace, so the owner scope is the only thing standing between two
		// tenants' datastore events.
		if _, err := st.GetServiceEvent(ctx, "tea-foreign00000000", eventID); !errors.Is(err, ErrNotFound) {
			t.Errorf("foreign-workspace lookup of %s = %v, want ErrNotFound", row.FactType, err)
		}
		hydrated++
	}
	if hydrated != len(want) {
		t.Fatalf("hydrated %d datastore facts by evt-… id, want %d", hydrated, len(want))
	}
}

// The w6/m41 class for datastores, against a real checkpoint: an unhealthy
// conclusion whose Ready transition predates the recorded healthy checkpoint
// is a time-traveled re-read, and two consecutive ticks of it — enough to pass
// the debounce — must still record nothing.
func TestPGStaleDatastoreConclusionEmitsNoPhantomPair(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	rec := &Reconciler{Store: st}

	base := time.Now().Add(-time.Hour).UTC()
	name := "dpg-stale-" + fmt.Sprintf("%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})
	observe := func(src *appv1alpha1.Database, ticks int) {
		t.Helper()
		src.Name = name
		src.Labels = map[string]string{LabelTenant: tenant.ID}
		obs, ok := observedDatabaseStateFor(src)
		if !ok {
			t.Fatalf("no observation for %s", name)
		}
		for i := 0; i < ticks; i++ {
			rec.recordDatastoreObservation(ctx, obs)
		}
	}

	observe(readyDatabase(base.Add(10*time.Minute)), 1) // baseline + recorded healthy checkpoint
	observe(downDatabase(base), 2)                      // the time-traveled conclusion, twice
	observe(readyDatabase(base.Add(10*time.Minute)), 1) // the informer catches up

	if got := datastoreFactTypes(t, pool, name); len(got) != 0 {
		t.Fatalf("a time-traveled conclusion recorded %v, want nothing", got)
	}
}

// A KeyValue that has never been Ready is provisioning, not down — and a
// suspended one is intentional downtime. Neither may record an outage, and the
// vocabulary the recovery eventually uses must be Render's key_value_* one.
func TestPGObservedKeyValueArmsOnFirstReady(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	rec := &Reconciler{Store: st}

	base := time.Now().Add(-time.Hour).UTC()
	name := "red-" + fmt.Sprintf("%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})
	observe := func(src *appv1alpha1.KeyValue, ticks int) {
		t.Helper()
		src.Name = name
		src.Labels = map[string]string{LabelTenant: tenant.ID}
		obs, ok := observedKeyValueStateFor(src)
		if !ok {
			t.Fatalf("no observation for %s", name)
		}
		for i := 0; i < ticks; i++ {
			rec.recordDatastoreObservation(ctx, obs)
		}
	}
	provisioning := func(at time.Time) *appv1alpha1.KeyValue {
		return keyValueCR(appv1alpha1.KVPhaseProvisioning, metav1.ConditionFalse, "Provisioning", at)
	}
	ready := func(at time.Time) *appv1alpha1.KeyValue {
		return keyValueCR(appv1alpha1.KVPhaseReady, metav1.ConditionTrue, datastoreReasonProvisioned, at)
	}

	observe(provisioning(base), 3) // never been Ready
	if got := datastoreFactTypes(t, pool, name); len(got) != 0 {
		t.Fatalf("initial provisioning recorded %v, want nothing", got)
	}

	observe(ready(base.Add(5*time.Minute)), 2) // arms the edge, silently
	if got := datastoreFactTypes(t, pool, name); len(got) != 0 {
		t.Fatalf("the first Ready recorded %v, want nothing", got)
	}

	suspended := ready(base.Add(10 * time.Minute))
	suspended.Status.Conditions[0].Reason = appv1alpha1.ReasonSuspended
	suspended.Spec.Suspended = true
	observe(suspended, 2)
	if got := datastoreFactTypes(t, pool, name); len(got) != 0 {
		t.Fatalf("suspension recorded %v, want nothing", got)
	}

	observe(ready(base.Add(15*time.Minute)), 1)        // resumed
	observe(provisioning(base.Add(20*time.Minute)), 3) // a real outage
	observe(ready(base.Add(30*time.Minute)), 2)        // recovery

	got := datastoreFactTypes(t, pool, name)
	want := []string{string(DatastoreFactKeyValueUnhealthy), string(DatastoreFactKeyValueAvailable)}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("recorded %v, want %v", got, want)
	}
}
