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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPGDatastoreEventFromValuesHydrate is w4/m379's store half: a Postgres
// disk resize and replica change keep both sides through real Postgres, and an
// upgrade's facts keep the version pair, each read back by evt- id the way
// Retrieve event reads it.
func TestPGDatastoreEventFromValuesHydrate(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	name := fmt.Sprintf("dpg-details%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})
	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	ten, fifteen, zero, two := int32(10), int32(15), int32(0), int32(2)
	for i, ev := range []core.AuditEvent{
		{Verb: core.AuditVerbPostgresDiskSizeChanged, DiskSizeFromGB: &ten, DiskSizeGB: &fifteen},
		{Verb: core.AuditVerbPostgresReadReplicasChanged, ReadReplicasFrom: &zero, ReadReplicasTo: &two},
	} {
		ev.Caller, ev.Resource, ev.Target, ev.TargetName = "fixture-owner", core.WorkspaceObject(tenant.ID), core.DatabaseTarget(name), "orders"
		ev.Outcome, ev.At = core.AuditAllowed, at.Add(time.Duration(i)*time.Second)
		if err := st.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	base := ObservedDatastoreState{
		DatastoreID: name, WorkspaceID: tenant.ID, Kind: DatastoreKindPostgres, At: at,
		Phase: string(appv1alpha1.DBPhaseReady), Availability: "healthy", AvailabilityObserved: true,
		ReadyTransitionAt: at, SpecVersion: "17", CurrentVersion: "16",
	}
	upgrading := base
	upgrading.At, upgrading.Phase, upgrading.Availability, upgrading.AvailabilityObserved = at.Add(time.Minute), string(appv1alpha1.DBPhaseUpgrading), "", false
	done := base
	done.At, done.ReadyTransitionAt, done.CurrentVersion = at.Add(2*time.Minute), at.Add(2*time.Minute), "17"
	for _, obs := range []ObservedDatastoreState{base, upgrading, done} {
		if _, err := st.RecordObservedDatastoreState(ctx, obs); err != nil {
			t.Fatal(err)
		}
	}

	feed, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute),
		[]string{core.AuditVerbPostgresDiskSizeChanged, core.AuditVerbPostgresReadReplicasChanged}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range feed {
		if row.ServiceID != name {
			continue
		}
		lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, row.Key))
		if err != nil {
			t.Fatalf("hydrate %s: %v", row.Key, err)
		}
		e := lookup.Event
		switch {
		case e.Verb == core.AuditVerbPostgresDiskSizeChanged:
			if e.DiskSizeFromGB == nil || *e.DiskSizeFromGB != 10 || e.DiskSizeGB == nil || *e.DiskSizeGB != 15 {
				t.Errorf("disk size = %v -> %v, want 10 -> 15", e.DiskSizeFromGB, e.DiskSizeGB)
			}
		case e.Verb == core.AuditVerbPostgresReadReplicasChanged:
			if e.ReadReplicasFrom == nil || *e.ReadReplicasFrom != 0 || e.ReadReplicasTo == nil || *e.ReadReplicasTo != 2 {
				t.Errorf("replicas = %v -> %v, want 0 -> 2", e.ReadReplicasFrom, e.ReadReplicasTo)
			}
		case e.FactType == string(DatastoreFactPostgresUpgradeStarted), e.FactType == string(DatastoreFactPostgresUpgradeSucceeded):
			if e.VersionFrom != "16" || e.VersionTo != "17" {
				t.Errorf("%s versions = %q -> %q, want 16 -> 17", e.FactType, e.VersionFrom, e.VersionTo)
			}
		default:
			continue
		}
		seen[e.Verb+e.FactType] = true
	}
	if len(seen) != 4 {
		t.Fatalf("hydrated %v, want the resize, the replica change and both upgrade edges", seen)
	}
}
