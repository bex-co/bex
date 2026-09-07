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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func validPushNotificationForTest(tenantID, subject, serviceID string, at time.Time) PushNotification {
	return PushNotification{
		TenantID: tenantID, Subject: subject, SourceEventKey: "dep-test:ended",
		EventID:   ids.Derive(ids.Event, tenantID, subject, "dep-test:ended"),
		EventType: "deploy_failed", Title: "Deploy failed", Body: "api deploy failed.",
		Urgency: "important", ResourceKind: "service", ResourceID: serviceID,
		DeepLink: "/services/" + serviceID, OccurredAt: at, DeliverAt: at,
	}
}

func TestValidatePushNotificationFailsClosed(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	valid := validPushNotificationForTest(
		"tea-c185th5c2rvvnhbfiltg", "alice", "srv-c185th5c2rvvnhbfiltg", now,
	)
	if err := ValidatePushNotification(valid); err != nil {
		t.Fatalf("valid notification: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*PushNotification)
	}{
		{"unknown event", func(n *PushNotification) { n.EventType = "workspace_deleted" }},
		{"non opaque resource", func(n *PushNotification) { n.ResourceID = "acme-api" }},
		{"external route", func(n *PushNotification) { n.DeepLink = "https://example.com" }},
		{"wrong route target", func(n *PushNotification) { n.DeepLink = "/services/srv-other" }},
		{"empty body", func(n *PushNotification) { n.Body = "" }},
		{"zero delivery time", func(n *PushNotification) { n.DeliverAt = time.Time{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notification := valid
			test.mutate(&notification)
			if err := ValidatePushNotification(notification); err == nil {
				t.Fatal("invalid notification was accepted")
			}
		})
	}
}

func TestDuePushDeliveryJSONRedactsToken(t *testing.T) {
	delivery := DuePushDelivery{Token: "secret-device-capability", Provider: "expo"}
	encoded, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), delivery.Token) {
		t.Fatalf("delivery JSON leaked token: %s", encoded)
	}
}

func TestPushDeliveryMigrationSeparatesTokenFromLogicalPayload(t *testing.T) {
	sql, err := migrationsFS.ReadFile("migrations/0063_push_deliveries.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(sql)
	logicalStart := strings.Index(text, "CREATE TABLE push_notifications")
	deliveryStart := strings.Index(text, "CREATE TABLE push_deliveries")
	if logicalStart < 0 || deliveryStart < logicalStart {
		t.Fatal("push notification/delivery tables missing")
	}
	logicalDDL := text[logicalStart:deliveryStart]
	if strings.Contains(logicalDDL, " token") || strings.Contains(logicalDDL, "endpoint") {
		t.Fatal("logical notification schema contains destination capability material")
	}
	for _, contract := range []string{
		"PRIMARY KEY (tenant_id, subject, source_event_key)",
		"PRIMARY KEY (tenant_id, subject, device_id, source_event_key)",
		"CREATE TABLE push_watermark",
		"deep_link = '/services/' || resource_id",
		"read_at         timestamptz",
		"attempt_count   integer NOT NULL DEFAULT 0",
		"provider_ticket_id text NOT NULL DEFAULT ''",
		"accepted_token_digest text NOT NULL DEFAULT ''",
		"receipt_due_at  timestamptz",
		"ambiguous_at    timestamptz",
	} {
		if !strings.Contains(text, contract) {
			t.Errorf("migration missing %q", contract)
		}
	}
}

func TestPGStorePushDeliveryQueue(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	subject := "push-queue-" + stamp
	tenant, err := store.CreateWorkspace(ctx, "push-queue-"+stamp, PlanHobby, subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.DeleteTenant(context.Background(), tenant.ID) })
	app, err := store.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "api", Image: "traefik/whoami", Branch: "main",
		Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens := []string{"ExponentPushToken[queue-a-" + stamp + "]", "ExponentPushToken[queue-b-" + stamp + "]"}
	for index, token := range tokens {
		if _, err := store.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
			TenantID: tenant.ID, Subject: subject, DeviceID: fmt.Sprintf("device-%d", index), SessionID: "session-subject",
			Provider: "expo", Platform: "ios", Token: token,
		}); err != nil {
			t.Fatal(err)
		}
	}
	destinations, err := store.ListActivePushSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var own []ActivePushSubscription
	for _, destination := range destinations {
		if destination.TenantID == tenant.ID && destination.Subject == subject {
			own = append(own, destination)
		}
	}
	if len(own) != 2 || own[0].Token == "" || own[0].Role != "admin" {
		t.Fatalf("active destinations = %+v", own)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, _, err := store.EnsurePushWatermark(ctx, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	item := PushNotificationBatchItem{
		Notification: validPushNotificationForTest(tenant.ID, subject, app.ID, now),
		DeviceIDs:    []string{"device-0", "device-1"},
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	errors := make([]error, 2)
	for index := range errors {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errors[index] = store.EnqueuePushNotifications(ctx, []PushNotificationBatchItem{item}, now, "dep-test:ended")
		}(index)
	}
	close(start)
	wait.Wait()
	if errors[0] != nil || errors[1] != nil {
		t.Fatalf("concurrent enqueue errors = %v", errors)
	}
	var logicalCount, deliveryCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM push_notifications WHERE tenant_id = $1 AND subject = $2`, tenant.ID, subject,
	).Scan(&logicalCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM push_deliveries WHERE tenant_id = $1 AND subject = $2`, tenant.ID, subject,
	).Scan(&deliveryCount); err != nil {
		t.Fatal(err)
	}
	if logicalCount != 1 || deliveryCount != 2 {
		t.Fatalf("logical=%d deliveries=%d, want 1/2", logicalCount, deliveryCount)
	}

	unread, err := store.CountUnreadPushNotifications(ctx, tenant.ID, subject, nil)
	if err != nil || unread != 1 {
		t.Fatalf("unread=%d error=%v, want 1", unread, err)
	}
	inbox, err := store.ListOwnPushNotifications(ctx, tenant.ID, subject, 10, nil)
	if err != nil || len(inbox) != 1 || inbox[0].EventID != item.Notification.EventID || inbox[0].ReadAt != nil {
		t.Fatalf("own inbox=%+v error=%v", inbox, err)
	}
	// The destination-gated exclusion (w6/m137) filters list and count
	// identically in SQL; an unrelated exclusion filters nothing.
	if rows, err := store.ListOwnPushNotifications(ctx, tenant.ID, subject, 10, []string{item.Notification.EventType}); err != nil || len(rows) != 0 {
		t.Fatalf("excluded-event list=%+v error=%v, want empty", rows, err)
	}
	if n, err := store.CountUnreadPushNotifications(ctx, tenant.ID, subject, []string{item.Notification.EventType}); err != nil || n != 0 {
		t.Fatalf("excluded-event unread=%d error=%v, want 0", n, err)
	}
	if rows, err := store.ListOwnPushNotifications(ctx, tenant.ID, subject, 10, []string{"agent_needs_decision"}); err != nil || len(rows) != 1 {
		t.Fatalf("unrelated exclusion list=%+v error=%v, want the row back", rows, err)
	}
	if changed, err := store.MarkOwnPushNotificationRead(ctx, tenant.ID, subject+"-foreign", item.Notification.EventID, now); err != nil || changed {
		t.Fatalf("foreign read changed=%v error=%v", changed, err)
	}
	if changed, err := store.MarkOwnPushNotificationRead(ctx, tenant.ID, subject, "evt-00000000000000000000", now); err != nil || changed {
		t.Fatalf("unknown read changed=%v error=%v", changed, err)
	}
	readAt := now.Add(time.Second)
	if changed, err := store.MarkOwnPushNotificationRead(ctx, tenant.ID, subject, item.Notification.EventID, readAt); err != nil || !changed {
		t.Fatalf("own read changed=%v error=%v", changed, err)
	}
	// Repeated acknowledgement is successful but cannot rewrite the first read.
	if changed, err := store.MarkOwnPushNotificationRead(ctx, tenant.ID, subject, item.Notification.EventID, readAt.Add(time.Hour)); err != nil || !changed {
		t.Fatalf("repeat read changed=%v error=%v", changed, err)
	}
	inbox, err = store.ListOwnPushNotifications(ctx, tenant.ID, subject, 10, nil)
	if err != nil || len(inbox) != 1 || inbox[0].ReadAt == nil || !inbox[0].ReadAt.Equal(readAt) {
		t.Fatalf("read inbox=%+v error=%v", inbox, err)
	}
	unread, err = store.CountUnreadPushNotifications(ctx, tenant.ID, subject, nil)
	if err != nil || unread != 0 {
		t.Fatalf("unread=%d error=%v, want 0", unread, err)
	}

	// Two replicas lease disjoint device rows.
	claimed := make([][]DuePushDelivery, 2)
	errors = make([]error, 2)
	start = make(chan struct{})
	for index := range claimed {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			claimed[index], errors[index] = store.ClaimDuePushDeliveries(ctx, now, now.Add(time.Minute), 10)
		}(index)
	}
	close(start)
	wait.Wait()
	if errors[0] != nil || errors[1] != nil || len(claimed[0])+len(claimed[1]) != 2 {
		t.Fatalf("concurrent claims lengths=%d/%d errors=%v", len(claimed[0]), len(claimed[1]), errors)
	}
	seenDevices := map[string]bool{}
	for _, batch := range claimed {
		for _, delivery := range batch {
			if seenDevices[delivery.DeviceID] || delivery.Token == "" {
				t.Fatalf("duplicate/redacted internal claim = %+v", delivery)
			}
			seenDevices[delivery.DeviceID] = true
			if changed, err := store.ReleasePushDelivery(ctx, delivery); err != nil || !changed {
				t.Fatalf("release changed=%v error=%v", changed, err)
			}
		}
	}

	// Restart-style reclaim sees both pending rows; provider acceptance closes
	// exactly one while the other remains retryable.
	reclaimed, err := store.ClaimDuePushDeliveries(ctx, now, now.Add(2*time.Minute), 10)
	if err != nil || len(reclaimed) != 2 {
		t.Fatalf("reclaim = %+v error=%v", reclaimed, err)
	}
	if changed, err := store.AcceptPushDelivery(ctx, reclaimed[0], "ticket-one", now, now.Add(15*time.Minute)); err != nil || !changed {
		t.Fatalf("accept changed=%v error=%v", changed, err)
	}
	// A token rotation after acceptance must not let a later invalid receipt
	// revoke the replacement capability.
	replacementToken := "ExponentPushToken[replacement-" + stamp + "]"
	if _, err := store.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
		TenantID: tenant.ID, Subject: subject, DeviceID: reclaimed[0].DeviceID, SessionID: "session-replacement",
		Provider: "expo", Platform: "ios", Token: replacementToken,
	}); err != nil {
		t.Fatal(err)
	}
	receiptClaims, err := store.ClaimDuePushReceipts(ctx, now.Add(15*time.Minute), now.Add(16*time.Minute), 10)
	if err != nil || len(receiptClaims) != 1 || receiptClaims[0].TokenDigest == "" {
		t.Fatalf("receipt claims=%+v error=%v", receiptClaims, err)
	}
	if changed, err := store.RevokeExactPushSubscription(ctx, receiptClaims[0]); err != nil || changed {
		t.Fatalf("rotated token revoke changed=%v error=%v", changed, err)
	}
	if changed, err := store.RecordPushReceipt(ctx, receiptClaims[0], "", now.Add(15*time.Minute), now.Add(15*time.Minute), true, false, false); err != nil || !changed {
		t.Fatalf("record delivered changed=%v error=%v", changed, err)
	}
	if changed, err := store.ReleasePushDelivery(ctx, reclaimed[1]); err != nil || !changed {
		t.Fatalf("release second changed=%v error=%v", changed, err)
	}
	remaining, err := store.ClaimDuePushDeliveries(ctx, now, now.Add(3*time.Minute), 10)
	if err != nil || len(remaining) != 1 || remaining[0].DeviceID != reclaimed[1].DeviceID {
		t.Fatalf("remaining retryable = %+v error=%v", remaining, err)
	}
}
