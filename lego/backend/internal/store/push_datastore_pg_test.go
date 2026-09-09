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

package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The w3/m82 t005 chain, end to end against real Postgres: an observed
// datastore availability edge recorded through the checkpoint diff must reach
// an OPTED-IN member's durable push inbox and per-device queue, routed to the
// datastore rather than a service — while a member on the untouched default
// policy and a member of another workspace receive nothing.
//
// It runs against real Postgres because every link in that chain is
// SQL-shaped: the checkpoint CAS, the feed's datastore UNION arm, the shared
// push watermark, and — the reason m78 needed a migration at all — the
// push_notifications event_type / resource / deep_link CHECK vocabulary that
// 0109 widened. Package store_test so it can import the notifications
// PushWorker without a cycle AND share the store test binary, running serially
// with TestPGStore's `TRUNCATE tenants CASCADE` instead of racing it.
func TestPushWorkerEnqueuesObservedDatastoreFacts(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	alice, bob, carol := "push-ds-alice-"+stamp, "push-ds-bob-"+stamp, "push-ds-carol-"+stamp
	tenant, err := st.CreateWorkspace(ctx, "push-ds-"+stamp, store.PlanHobby, alice)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	if err := st.AddMember(ctx, bob, tenant.ID, "developer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	other, err := st.CreateWorkspace(ctx, "push-ds-other-"+stamp, store.PlanHobby, carol)
	if err != nil {
		t.Fatalf("create other workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), other.ID) })

	// Datastore ids are the CR's own immutable metadata.name, so the test picks
	// well-formed opaque ids directly rather than minting rows.
	databaseID := "dpg-c185th5c2rvvnhbfiltg"
	keyValueID := "red-c185th5c2rvvnhbfilth"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM datastore_event_facts WHERE datastore_id = ANY($1)`,
			[]string{databaseID, keyValueID})
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM datastore_observed_checkpoints WHERE datastore_id = ANY($1)`,
			[]string{databaseID, keyValueID})
	})

	optedIn := notifications.PushSettingsView{
		Enabled: true,
		Events: []notifications.DeliveryEvent{
			notifications.DeliveryEventPostgresUnavailable, notifications.DeliveryEventPostgresAvailable,
			notifications.DeliveryEventKeyValueUnhealthy, notifications.DeliveryEventKeyValueAvailable,
		},
		MinimumUrgency:     notifications.DeliveryUrgencyRoutine,
		TimeZone:           "UTC",
		MaxDeferralSeconds: 3600,
	}
	encoded, err := json.Marshal(optedIn)
	if err != nil {
		t.Fatal(err)
	}
	// alice opts in; carol holds the SAME opt-in in another workspace, so any
	// row she receives is a workspace-scoping bug rather than a preference one.
	// bob's policy is never written: he stays on the stored default.
	for _, member := range []struct {
		subject string
		tenant  string
		policy  json.RawMessage
	}{
		{alice, tenant.ID, encoded},
		{carol, other.ID, encoded},
	} {
		if _, err := st.UpsertNotificationPushPolicy(ctx, member.tenant, member.subject, member.policy); err != nil {
			t.Fatalf("upsert push policy for %s: %v", member.subject, err)
		}
	}
	for _, device := range []struct {
		subject string
		tenant  string
	}{{alice, tenant.ID}, {bob, tenant.ID}, {carol, other.ID}} {
		if _, err := st.UpsertDevicePushSubscription(ctx, store.DevicePushSubscription{
			TenantID: device.tenant, Subject: device.subject, DeviceID: "ios-" + device.subject,
			SessionID: "session-" + device.subject, Provider: "expo", Platform: "ios",
			Token: "ExponentPushToken[m82-" + device.subject + "]",
		}); err != nil {
			t.Fatalf("register device for %s: %v", device.subject, err)
		}
	}
	// The push watermark is a global singleton seeded at first dispatch; pin it
	// behind this test's facts so a previous run's cursor cannot skip them.
	if _, err := pool.Exec(ctx,
		`INSERT INTO push_watermark (id, at, key) VALUES (true, $1, '')
		 ON CONFLICT (id) DO UPDATE SET at = $1, key = ''`, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("pin push watermark: %v", err)
	}

	// healthy → unhealthy → healthy for each datastore, every state observed
	// twice: the checkpoint diff must yield exactly one fact per edge. The
	// first healthy observation is the baseline that ARMS the outage edge
	// (nextDatastoreAvailability): without it a never-Ready datastore is
	// provisioning, not down.
	base := time.Now().UTC()
	tick := 0
	for _, datastore := range []struct{ id, kind string }{
		{databaseID, store.DatastoreKindPostgres},
		{keyValueID, store.DatastoreKindKeyValue},
	} {
		for _, availability := range []string{"healthy", "unhealthy", "healthy"} {
			for i := 0; i < 2; i++ {
				obs := store.ObservedDatastoreState{
					DatastoreID: datastore.id, WorkspaceID: tenant.ID, Kind: datastore.kind,
					Phase: "running", Availability: availability, AvailabilityObserved: true,
					At: base.Add(time.Duration(tick) * time.Second),
				}
				if availability == "unhealthy" {
					obs.ReasonCode = store.EventReasonReadinessFailed
				}
				obs.ReadyTransitionAt = obs.At
				tick++
				if _, err := st.RecordObservedDatastoreState(ctx, obs); err != nil {
					t.Fatalf("record observed datastore state: %v", err)
				}
			}
		}
	}

	// No Sender/Receipts: dispatch enqueues durably and nothing is sent — the
	// transport-off default, which is exactly what this test should prove.
	worker := &notifications.PushWorker{Store: st, Clock: func() time.Time { return time.Now().Add(30 * time.Second) }}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run once: %v", err)
	}

	inbox, err := st.ListOwnPushNotifications(ctx, tenant.ID, alice, 50, nil)
	if err != nil {
		t.Fatalf("list alice inbox: %v", err)
	}
	wantRoute := map[string]struct{ kind, id, link string }{
		"postgres_unavailable": {"database", databaseID, "/databases/" + databaseID},
		"postgres_available":   {"database", databaseID, "/databases/" + databaseID},
		"key_value_unhealthy":  {"keyValue", keyValueID, "/key-values/" + keyValueID},
		"key_value_available":  {"keyValue", keyValueID, "/key-values/" + keyValueID},
	}
	counts := map[string]int{}
	for _, n := range inbox {
		counts[n.EventType]++
		want, known := wantRoute[n.EventType]
		if !known {
			t.Fatalf("unexpected event %q in the opted-in inbox", n.EventType)
		}
		if n.ResourceKind != want.kind || n.ResourceID != want.id || n.DeepLink != want.link {
			t.Errorf("%s routed to %s/%s (%s), want %s/%s (%s)", n.EventType,
				n.ResourceKind, n.ResourceID, n.DeepLink, want.kind, want.id, want.link)
		}
	}
	for event := range wantRoute {
		if counts[event] != 1 {
			t.Fatalf("opted-in inbox counts = %v, want exactly one %q", counts, event)
		}
	}
	// One device row per logical notification, so the queue agrees with the
	// inbox rather than merely containing it.
	var deliveries int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM push_deliveries WHERE tenant_id = $1 AND subject = $2`,
		tenant.ID, alice).Scan(&deliveries); err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if deliveries != len(wantRoute) {
		t.Errorf("alice has %d device deliveries, want %d (one per fact)", deliveries, len(wantRoute))
	}

	for _, excluded := range []struct {
		name, subject, tenantID, why string
	}{
		{"default policy", bob, tenant.ID, "datastore events are additive opt-in"},
		{"other workspace", carol, other.ID, "facts belong to another workspace"},
	} {
		rows, err := st.ListOwnPushNotifications(ctx, excluded.tenantID, excluded.subject, 50, nil)
		if err != nil {
			t.Fatalf("list %s inbox: %v", excluded.name, err)
		}
		if len(rows) != 0 {
			t.Errorf("%s member received %d pushes, want 0 — %s: %+v", excluded.name, len(rows), excluded.why, rows)
		}
	}
}
