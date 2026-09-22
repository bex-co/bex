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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestWebhookEndpointLatestDeliveryIsTheSameOnEveryRead is w4/117: the
// latest-delivery columns came from a LEFT JOIN LATERAL that only the LIST
// query had, so `webhookEndpoint(id:)` reported an endpoint with a successful
// delivery 30 seconds earlier as having never delivered anything — in the same
// request where the list said otherwise. The enable/disable and update
// responses returned the same blanks.
//
// The divergence lives entirely in SQL, so this is the only place it can be
// proven; toView renders whatever the row carries and was never wrong.
func TestWebhookEndpointLatestDeliveryIsTheSameOnEveryRead(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE name='webhook-latest-delivery'`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "webhook-latest-delivery", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID) }()

	endpoint, err := s.CreateWebhookEndpoint(ctx, tenant.ID, "latest", "https://hooks.example.test/events", "whsec_latest", []string{"deploy_started"}, true, "user-owner")
	if err != nil {
		t.Fatal(err)
	}

	// One delivered attempt behind the endpoint — the state the hunt found.
	base := time.Date(2026, 9, 21, 10, 49, 25, 0, time.UTC)
	delivery := WebhookDelivery{
		ID: ids.New(ids.WebhookDelivery), EndpointID: endpoint.ID,
		EventID: "evt-latest0000000000", EventType: "deploy_started", ServiceID: "acme-api",
		Payload: `{"type":"deploy_started"}`, NextAttemptAt: base,
	}
	if _, err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{delivery}, base, "latest-watermark", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimDueWebhookAttempts(ctx, base, base.Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: delivery.ID, StatusCode: 200, Delivered: true,
		CompletedAt: base.Add(time.Second),
	}); err != nil || !ok {
		t.Fatalf("complete attempt = %v, %v", ok, err)
	}

	// The list is the reference: it always carried these.
	listed, err := s.ListWebhookEndpoints(ctx, []string{tenant.ID}, time.Time{}, "", 10)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list = %+v, %v", listed, err)
	}
	want := listed[0]
	if want.LatestAttemptStatus == "" || want.LatestAttemptAt == nil || want.LatestParentStatus == "" {
		t.Fatalf("the list must report a delivery for this fixture, got %+v — the test would be vacuous", want)
	}

	same := func(t *testing.T, label string, got WebhookEndpoint) {
		t.Helper()
		if got.LatestAttemptStatus != want.LatestAttemptStatus {
			t.Errorf("%s latestStatus = %q, want %q", label, got.LatestAttemptStatus, want.LatestAttemptStatus)
		}
		if got.LatestParentStatus != want.LatestParentStatus {
			t.Errorf("%s latestParentStatus = %q, want %q", label, got.LatestParentStatus, want.LatestParentStatus)
		}
		if got.LatestAttemptAt == nil || !got.LatestAttemptAt.Equal(*want.LatestAttemptAt) {
			t.Errorf("%s latestSentAt = %v, want %v", label, got.LatestAttemptAt, want.LatestAttemptAt)
		}
	}

	one, err := s.GetWebhookEndpoint(ctx, tenant.ID, endpoint.ID)
	if err != nil {
		t.Fatal(err)
	}
	same(t, "GetWebhookEndpoint", one)

	toggled, err := s.SetWebhookEndpointEnabled(ctx, tenant.ID, endpoint.ID, false, "paused by qa")
	if err != nil {
		t.Fatal(err)
	}
	same(t, "SetWebhookEndpointEnabled", toggled)
	if toggled.Enabled || toggled.DisabledReason != "paused by qa" {
		t.Errorf("the toggle must still do its own job: %+v", toggled)
	}

	updated, err := s.UpdateWebhookEndpoint(ctx, tenant.ID, endpoint.ID, "renamed", "https://hooks.example.test/v2", []string{"deploy_ended"}, true)
	if err != nil {
		t.Fatal(err)
	}
	same(t, "UpdateWebhookEndpoint", updated)
	if updated.Name != "renamed" || updated.URL != "https://hooks.example.test/v2" || !updated.Enabled || updated.DisabledReason != "" {
		t.Errorf("the update must still do its own job: %+v", updated)
	}
}
