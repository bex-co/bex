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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// w6/m41 (source .pm/w3/016.md, found by w3/m78's live crash leg): the
// operator concludes App Ready from controller-runtime CACHED clients. When
// apiserver/etcd stall watch streams for minutes, a reconcile pass can
// re-conclude a crash-era Ready=False long after recovery. The m78 mitigations
// (RolloutSettling exclusion + the two-tick debounceUnhealthy) both read that
// same stale world, so a time-traveled condition delivered on two consecutive
// ticks passes the debounce and records a phantom server_failed /
// server_available pair — each one a Critical push page + webhook delivery.
//
// This test drives exactly that sequence through the reconciler's observation
// path: a healthy checkpoint, then a stale crash conclusion whose condition
// LastTransitionTime PREDATES that checkpoint (a time-traveled re-read, not a
// new outage), twice, then healthy again. No edge may be recorded.
func TestPGStaleConclusionEmitsNoPhantomPair(t *testing.T) {
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
	defer pool.Close()
	st := NewPGStore(pool)

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "stale-edge-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "web-" + stamp, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	// The crash happened at base; the service recovered and the reconciler
	// recorded a healthy checkpoint whose Ready=True transition is newer. The
	// stale re-read still carries the crash-era LastTransitionTime.
	base := time.Now().Add(-time.Hour).UTC()
	healthy := healthyCR(base.Add(10 * time.Minute))
	staleCrash := crashedCR(base)

	rec := &Reconciler{Store: st}

	driveObservations(t, rec, app.ID, healthy, 1)    // baseline + recorded healthy checkpoint
	driveObservations(t, rec, app.ID, staleCrash, 2) // the time-traveled conclusion, twice — enough to pass the tick debounce
	driveObservations(t, rec, app.ID, healthy, 1)    // the informer catches up

	var factTypes []string
	rows, err := pool.Query(ctx, `SELECT fact_type FROM service_event_facts WHERE app_id = $1 ORDER BY at, source_key`, app.ID)
	if err != nil {
		t.Fatalf("list facts: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ft string
		if err := rows.Scan(&ft); err != nil {
			t.Fatalf("scan fact: %v", err)
		}
		factTypes = append(factTypes, ft)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate facts: %v", err)
	}
	if len(factTypes) != 0 {
		t.Fatalf("a time-traveled conclusion recorded edges %v, want none (phantom server_failed/server_available pair)", factTypes)
	}
}
