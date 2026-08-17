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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
)

// TestTwoWorkersDeliverEachEventExactlyOnce is w1/m58 t005's two-replica proof,
// done as a reproducible integration test rather than a manual dev-1 pod run: two
// real webhook Workers (= bex-api's two replicas) run concurrently against the
// SAME control-plane Postgres and one HTTP receiver, and every event must be
// POSTed EXACTLY once. It lives in package store_test (external) so it can import
// the webhooks package without an import cycle, and it shares the store test
// binary — so it runs serially with TestPGStore, never racing the claim query
// against another package's concurrently-executing PG test.
//
// Hermetic-by-default: skipped unless BEX_TEST_DB_URI points at a throwaway DB.
func TestTwoWorkersDeliverEachEventExactlyOnce(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	s := store.NewPGStore(pool)

	// Isolation: no other webhook endpoint may be enabled, or a worker's claim
	// (which is workspace-agnostic) could pull an unrelated tenant's deliveries.
	// Safe because this runs serially inside the store test binary.
	for _, tbl := range []string{"webhook_deliveries", "webhook_endpoints"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Fatalf("clean %s: %v", tbl, err)
		}
	}

	ten, err := s.CreateTenant(ctx, "wh-two-replica", store.PlanHobby)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	type received struct {
		count     int
		timestamp string
		signature string
		body      []byte
	}
	var mu sync.Mutex
	seen := map[string]received{} // webhook-id header -> signed delivery evidence
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		id := r.Header.Get("webhook-id")
		entry := seen[id]
		entry.count++
		entry.timestamp = r.Header.Get("webhook-timestamp")
		entry.signature = r.Header.Get("webhook-signature")
		entry.body = append([]byte(nil), body...)
		seen[id] = entry
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer srv.Close()

	ep, err := s.CreateWebhookEndpoint(ctx, ten.ID, "e2e", srv.URL, "whsec_e2e", []string{webhooks.TypeDeployStarted}, true, "user-1")
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	// Enqueue N due deliveries, each a distinct event (the webhook-id the receiver
	// dedupes on).
	const n = 30
	now := time.Now().UTC()
	batch := make([]store.WebhookDelivery, 0, n)
	eventIDs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ev := ids.New(ids.WebhookDelivery) // arbitrary unique per-event token
		eventIDs = append(eventIDs, ev)
		batch = append(batch, store.WebhookDelivery{
			ID: ids.New(ids.WebhookDelivery), EndpointID: ep.ID, EventID: ev,
			EventType: webhooks.TypeDeployStarted, ServiceID: "svc", Payload: `{"type":"deploy_started"}`,
			NextAttemptAt: now,
		})
	}
	if err := s.EnqueueWebhookDeliveries(ctx, batch, now, "e2e"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Two workers = two replicas. Each uses the httptest client (the production
	// client's SSRF guard blocks the loopback receiver) and the shared store.
	newWorker := func() *webhooks.Worker {
		return &webhooks.Worker{Store: s, Client: srv.Client(), Clock: func() time.Time { return time.Now().UTC() }}
	}
	workers := []*webhooks.Worker{newWorker(), newWorker()}

	deadline := time.Now().Add(30 * time.Second)
	for {
		var wg sync.WaitGroup
		for _, w := range workers {
			wg.Add(1)
			go func(w *webhooks.Worker) { defer wg.Done(); _ = w.RunOnce(ctx) }(w)
		}
		wg.Wait()
		mu.Lock()
		done := len(seen) >= n
		mu.Unlock()
		if done || time.Now().After(deadline) {
			break
		}
	}

	mu.Lock()
	if len(seen) != n {
		t.Fatalf("delivered %d distinct events, want %d", len(seen), n)
	}
	for _, ev := range eventIDs {
		received := seen[ev]
		if received.count != 1 {
			t.Errorf("event %s delivered %d times, want exactly 1 (two replicas must not double-deliver)", ev, received.count)
		}
		if !webhooks.Verify(ep.Secret, ev, received.timestamp, received.body, received.signature) {
			t.Errorf("event %s signature does not verify against its create-only secret", ev)
		}
	}
	mu.Unlock()

	// The same real rows exposed by REST history retain one stable first-sent
	// instant and the latest bounded HTTP response evidence.
	history, err := s.ListWebhookDeliveries(ctx, store.WebhookDeliveryFilter{EndpointID: ep.ID, Limit: 100})
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(history) != n {
		t.Fatalf("history rows = %d, want %d", len(history), n)
	}
	for _, delivery := range history {
		if delivery.SentAt == nil || delivery.LastStatus != http.StatusOK || delivery.ResponseBody != "accepted" {
			t.Errorf("history evidence for %s = sentAt %v status %d body %q", delivery.EventID, delivery.SentAt, delivery.LastStatus, delivery.ResponseBody)
		}
		mu.Lock()
		received := seen[delivery.EventID]
		mu.Unlock()
		if received.count != 1 {
			t.Errorf("history event %s has no matching receiver record", delivery.EventID)
		}
	}
}
