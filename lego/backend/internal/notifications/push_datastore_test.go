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

package notifications

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	testDatabaseID = "dpg-c185th5c2rvvnhbfiltg"
	testKeyValueID = "red-c185th5c2rvvnhbfilth"
)

// TestProjectDatastorePushEvents pins the datastore vocabulary, its urgencies,
// and — just as importantly — what does NOT project. Availability mirrors the
// service pair (Critical outage, Important recovery); only the failure half of
// backup/restore/upgrade is push-worthy.
func TestProjectDatastorePushEvents(t *testing.T) {
	tests := []struct {
		factType string
		want     string
		urgency  string
		ok       bool
	}{
		{string(store.DatastoreFactPostgresUnavailable), "postgres_unavailable", string(DeliveryUrgencyCritical), true},
		{string(store.DatastoreFactPostgresAvailable), "postgres_available", string(DeliveryUrgencyImportant), true},
		{string(store.DatastoreFactKeyValueUnhealthy), "key_value_unhealthy", string(DeliveryUrgencyCritical), true},
		{string(store.DatastoreFactKeyValueAvailable), "key_value_available", string(DeliveryUrgencyImportant), true},
		{string(store.DatastoreFactPostgresBackupFailed), "postgres_backup_failed", string(DeliveryUrgencyImportant), true},
		{string(store.DatastoreFactPostgresRestoreFailed), "postgres_restore_failed", string(DeliveryUrgencyImportant), true},
		{string(store.DatastoreFactPostgresUpgradeFailed), "postgres_upgrade_failed", string(DeliveryUrgencyImportant), true},
		{string(store.DatastoreFactPostgresBackupCompleted), "", "", false},
		{string(store.DatastoreFactPostgresRestoreSucceeded), "", "", false},
		{string(store.DatastoreFactPostgresUpgradeSucceeded), "", "", false},
		{string(store.DatastoreFactPostgresUpgradeStarted), "", "", false},
	}
	for _, test := range tests {
		t.Run(test.factType, func(t *testing.T) {
			// The datastore feed arm has no App: the id lands in ServiceID and
			// ServiceName, and the serviceID argument is empty.
			row := store.WebhookEventRow{
				Source: store.EventSourceFact, FactType: test.factType,
				ServiceID: testDatabaseID, ServiceName: testDatabaseID,
			}
			got, ok := projectPushEvent(row, "", "")
			if ok != test.ok || got.event != test.want || got.urgency != test.urgency {
				t.Fatalf("projectPushEvent(%s) = (%+v,%v), want event=%q urgency=%q ok=%v",
					test.factType, got, ok, test.want, test.urgency, test.ok)
			}
			if test.ok && got.body == "" {
				t.Fatalf("%s projected an empty body — the copy must name the datastore", test.factType)
			}
		})
	}
}

// TestDatastorePushIsOptInOnly is the acceptance property: an opted-in member
// gets exactly one inbox row per fact, correctly routed to the datastore (not
// to a service); a member on the untouched default policy gets none; and a
// member of another workspace gets none however their policy reads.
func TestDatastorePushIsOptInOnly(t *testing.T) {
	now := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)

	optedIn := pushPolicyJSON(t, PushSettingsView{
		Enabled: true,
		Events: []DeliveryEvent{
			DeliveryEventPostgresUnavailable, DeliveryEventPostgresAvailable,
			DeliveryEventKeyValueUnhealthy, DeliveryEventPostgresBackupFailed,
		},
		MinimumUrgency: DeliveryUrgencyRoutine, TimeZone: "UTC", MaxDeferralSeconds: 3600,
	})

	queue := newFakePushWorkerStore()
	queue.watermarkAt = at.Add(-time.Hour)
	queue.destinations = []store.ActivePushSubscription{
		{TenantID: "tea-one", Subject: "alice", Role: "developer", DeviceID: "alice-ios",
			Provider: "expo", Platform: "ios", Token: "t1", CreatedAt: at.Add(-time.Hour), PushPolicy: optedIn},
		// bob's PushPolicy is nil: the stored default, which must not have grown
		// a datastore event.
		{TenantID: "tea-one", Subject: "bob", Role: "admin", DeviceID: "bob-ios",
			Provider: "expo", Platform: "ios", Token: "t2", CreatedAt: at.Add(-time.Hour)},
		// carol has the same opt-in but in a different workspace.
		{TenantID: "tea-two", Subject: "carol", Role: "admin", DeviceID: "carol-ios",
			Provider: "expo", Platform: "ios", Token: "t3", CreatedAt: at.Add(-time.Hour), PushPolicy: optedIn},
	}
	queue.events = []store.WebhookEventRow{
		datastoreFactRow(at, "fact:pg-down", testDatabaseID, store.DatastoreFactPostgresUnavailable),
		datastoreFactRow(at.Add(time.Second), "fact:pg-up", testDatabaseID, store.DatastoreFactPostgresAvailable),
		datastoreFactRow(at.Add(2*time.Second), "fact:kv-down", testKeyValueID, store.DatastoreFactKeyValueUnhealthy),
		datastoreFactRow(at.Add(3*time.Second), "fact:pg-backup-bad", testDatabaseID, store.DatastoreFactPostgresBackupFailed),
		// Not in alice's opt-in list, and not in the push vocabulary at all.
		datastoreFactRow(at.Add(4*time.Second), "fact:pg-backup-ok", testDatabaseID, store.DatastoreFactPostgresBackupCompleted),
	}

	worker := &PushWorker{Store: queue, Clock: func() time.Time { return now }}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()
	bySubject := map[string][]store.PushNotification{}
	for _, n := range queue.notifications {
		bySubject[n.Subject] = append(bySubject[n.Subject], n)
	}
	if got := len(bySubject["bob"]); got != 0 {
		t.Errorf("default-policy member received %d datastore pushes, want 0 — these events are opt-in", got)
	}
	if got := len(bySubject["carol"]); got != 0 {
		t.Errorf("member of another workspace received %d pushes, want 0", got)
	}
	if got := len(bySubject["alice"]); got != 4 {
		t.Fatalf("opted-in member received %d pushes, want one per opted-in fact: %+v", got, bySubject["alice"])
	}
	want := map[string]struct{ kind, id, link string }{
		"postgres_unavailable":   {"database", testDatabaseID, "/databases/" + testDatabaseID},
		"postgres_available":     {"database", testDatabaseID, "/databases/" + testDatabaseID},
		"key_value_unhealthy":    {"keyValue", testKeyValueID, "/key-values/" + testKeyValueID},
		"postgres_backup_failed": {"database", testDatabaseID, "/databases/" + testDatabaseID},
	}
	for _, n := range bySubject["alice"] {
		expected, known := want[n.EventType]
		if !known {
			t.Fatalf("unexpected event %q in the opted-in inbox", n.EventType)
		}
		if n.ResourceKind != expected.kind || n.ResourceID != expected.id || n.DeepLink != expected.link {
			t.Errorf("%s routed to %s/%s (%s), want %s/%s (%s)", n.EventType,
				n.ResourceKind, n.ResourceID, n.DeepLink, expected.kind, expected.id, expected.link)
		}
		// ValidatePushNotification is the enqueue-side admission gate; a
		// projection it rejects is silently dropped, so assert it directly.
		if err := store.ValidatePushNotification(n); err != nil {
			t.Errorf("%s failed producer validation: %v", n.EventType, err)
		}
		delete(want, n.EventType)
	}
	if len(want) != 0 {
		t.Errorf("opted-in member missed %v", want)
	}
}

func datastoreFactRow(at time.Time, key, datastoreID string, fact store.DatastoreEventFactType) store.WebhookEventRow {
	return store.WebhookEventRow{
		CursorAt: at, Key: key, At: at, TenantID: "tea-one",
		ServiceID: datastoreID, ServiceName: datastoreID,
		Source: store.EventSourceFact, FactType: string(fact),
	}
}

func pushPolicyJSON(t *testing.T, view PushSettingsView) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
