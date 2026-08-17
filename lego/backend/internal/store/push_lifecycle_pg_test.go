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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The full chain behind ADR052 gap item 1 (w3/m78): observed lifecycle edges
// recorded through the checkpoint diff must reach a policy-enabled member's
// durable push inbox — one logical row per edge, including recovery and
// suspend/resume, not just server_failed. Runs against real Postgres because
// the dispatch path (feed tail, watermark, policy, DB CHECK vocabulary,
// enqueue) is SQL-shaped. It lives in package store_test so it can import the
// notifications PushWorker without a cycle AND shares the store test binary —
// running serially with TestPGStore's `TRUNCATE tenants CASCADE` instead of
// racing it from another package's concurrently-executing binary.
func TestPushWorkerEnqueuesObservedLifecycleFacts(t *testing.T) {
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
	alice := "push-lifecycle-alice-" + stamp
	tenant, err := st.CreateWorkspace(ctx, "push-lifecycle-"+stamp, store.PlanHobby, alice)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	app, err := st.CreateApp(ctx, store.App{
		TenantID: tenant.ID, Name: "web-" + stamp, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	policy := notifications.PushSettingsView{
		Enabled: true,
		Events: []notifications.DeliveryEvent{
			notifications.DeliveryEventServerFailed, notifications.DeliveryEventServerAvailable,
			notifications.DeliveryEventServiceSuspended, notifications.DeliveryEventServiceResumed,
		},
		MinimumUrgency:     notifications.DeliveryUrgencyRoutine,
		TimeZone:           "UTC",
		MaxDeferralSeconds: 3600,
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertNotificationPushPolicy(ctx, tenant.ID, alice, encoded); err != nil {
		t.Fatalf("upsert push policy: %v", err)
	}
	if _, err := st.UpsertDevicePushSubscription(ctx, store.DevicePushSubscription{
		TenantID: tenant.ID, Subject: alice, DeviceID: "ios-" + stamp, SessionID: "session-alice",
		Provider: "expo", Platform: "ios", Token: "ExponentPushToken[m78-" + stamp + "]",
	}); err != nil {
		t.Fatalf("register device: %v", err)
	}
	// The push watermark is a global singleton seeded at first dispatch; pin it
	// behind this test's facts so a previous run's cursor cannot skip them.
	if _, err := pool.Exec(ctx,
		`INSERT INTO push_watermark (id, at, key) VALUES (true, $1, '')
		 ON CONFLICT (id) DO UPDATE SET at = $1, key = ''`, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("pin push watermark: %v", err)
	}

	// Running → crashed → recovered → suspended → resumed, each state observed
	// repeatedly: the checkpoint diff must yield exactly one fact per edge.
	base := time.Now().UTC()
	running := store.ObservedServiceState{ServicePhase: string(appv1alpha1.PhaseRunning), Availability: "healthy", AvailabilityObserved: true}
	states := []store.ObservedServiceState{
		running,
		{ServicePhase: string(appv1alpha1.PhaseDeploying), Availability: "unhealthy", AvailabilityObserved: true, ReasonCode: store.EventReasonReadinessFailed},
		running,
		{ServicePhase: string(appv1alpha1.PhaseHibernated), AvailabilityObserved: true},
		running,
	}
	tick := 0
	for _, state := range states {
		for i := 0; i < 2; i++ {
			obs := state
			obs.AppID = app.ID
			obs.At = base.Add(time.Duration(tick) * time.Second)
			tick++
			if _, err := st.RecordObservedServiceState(ctx, obs); err != nil {
				t.Fatalf("record observed state: %v", err)
			}
		}
	}

	// No Sender/Receipts: dispatch enqueues durably, nothing is sent (the
	// transport-off default) — exactly what this test should prove.
	worker := &notifications.PushWorker{Store: st, Clock: func() time.Time { return time.Now().Add(30 * time.Second) }}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run once: %v", err)
	}

	inbox, err := st.ListOwnPushNotifications(ctx, tenant.ID, alice, 50)
	if err != nil {
		t.Fatalf("list inbox: %v", err)
	}
	counts := map[string]int{}
	for _, n := range inbox {
		counts[n.EventType]++
		if n.DeepLink != "/services/"+app.ID {
			t.Fatalf("deep link = %q, want %q", n.DeepLink, "/services/"+app.ID)
		}
	}
	for _, event := range policy.Events {
		if counts[string(event)] != 1 {
			t.Fatalf("inbox counts = %v, want exactly one %q", counts, event)
		}
	}
}
