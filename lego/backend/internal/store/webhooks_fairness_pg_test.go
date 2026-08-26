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

func TestWebhookDeliveryAdmissionIsBoundedPerWorkspace(t *testing.T) {
	ctx := context.Background()
	s, pool := webhookPGStore(t, ctx)
	defer pool.Close()

	const tenantName = "webhook-admission-bound"
	_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE name = $1`, tenantName)
	tenant, err := s.CreateTenant(ctx, tenantName, PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) }()
	endpointA := createFairnessEndpoint(t, ctx, s, tenant.ID, "admission-a")
	endpointB := createFairnessEndpoint(t, ctx, s, tenant.ID, "admission-b")
	base := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)

	// Two replicas race for the same three workspace-wide slots through
	// different endpoints. The advisory lock makes the read/count/insert one
	// serialized admission decision for this workspace.
	type enqueueResult struct {
		admission WebhookEnqueueResult
		err       error
	}
	results := make(chan enqueueResult, 2)
	start := make(chan struct{})
	for replica, endpointID := range []string{endpointA.ID, endpointB.ID} {
		go func(replica int, endpointID string) {
			<-start
			prefix := []string{"cap-a", "cap-b"}[replica]
			batch := fairnessDeliveries(endpointID, prefix, 2, base)
			result, enqueueErr := s.EnqueueWebhookDeliveries(
				ctx, batch, base.Add(time.Duration(replica)*time.Second), "cap-watermark", 3,
			)
			results <- enqueueResult{admission: result, err: enqueueErr}
		}(replica, endpointID)
	}
	close(start)
	var admitted, capped int
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent bounded enqueue: %v", result.err)
		}
		admitted += result.admission.Admitted
		capped += result.admission.Capped
	}
	if admitted != 3 || capped != 1 {
		t.Fatalf("concurrent admission = admitted %d capped %d, want 3/1", admitted, capped)
	}
	var open int
	if err := pool.QueryRow(ctx,
		`SELECT count(*)
		   FROM webhook_deliveries d
		   JOIN webhook_endpoints e ON e.id = d.endpoint_id
		  WHERE e.tenant_id = $1 AND d.delivered_at IS NULL AND d.failed_at IS NULL`,
		tenant.ID,
	).Scan(&open); err != nil || open != 3 {
		t.Fatalf("open workspace backlog = %d (err %v), want exactly 3", open, err)
	}

	// Zero is the explicit operational escape hatch: the next batch is admitted
	// despite the existing three open rows.
	unbounded := fairnessDeliveries(endpointA.ID, "uncapped", 4, base.Add(time.Minute))
	result, err := s.EnqueueWebhookDeliveries(ctx, unbounded, base.Add(time.Minute), "uncapped-watermark", 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Admitted != 4 || result.Capped != 0 {
		t.Fatalf("disabled cap result = %+v, want four admitted and none capped", result)
	}
}

func TestWebhookAttemptClaimIsFairAndDisjointAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	s, pool := webhookPGStore(t, ctx)
	defer pool.Close()

	const noisyName = "webhook-fair-noisy"
	const quietName = "webhook-fair-quiet"
	_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE name IN ($1, $2)`, noisyName, quietName)
	noisy, err := s.CreateTenant(ctx, noisyName, PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	quiet, err := s.CreateTenant(ctx, quietName, PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, noisy.ID, quiet.ID)
	}()
	noisyEndpoint := createFairnessEndpoint(t, ctx, s, noisy.ID, "noisy")
	quietEndpoint := createFairnessEndpoint(t, ctx, s, quiet.ID, "quiet")
	base := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base.Add(time.Hour)

	// The quiet workspace's one row is newer than all noisy rows. A global FIFO
	// limit would return two noisy rows; workspace-ranked order returns one each.
	noisyBatch := fairnessDeliveries(noisyEndpoint.ID, "initial-noisy", 4, base)
	quietBatch := fairnessDeliveries(quietEndpoint.ID, "initial-quiet", 1, base.Add(time.Minute))
	initial := append(noisyBatch, quietBatch...)
	if _, err := s.EnqueueWebhookDeliveries(ctx, initial, base, "fair-initial", 0); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimDueWebhookAttempts(ctx, now, now.Add(time.Minute), 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := claimedTenants(claimed); got[noisy.ID] != 1 || got[quiet.ID] != 1 {
		t.Fatalf("first fair claim tenants = %#v, want one noisy and one quiet", got)
	}
	if !containsAttempt(claimed, noisyBatch[0].ID) || !containsAttempt(claimed, quietBatch[0].ID) {
		t.Fatalf("first fair claim = %#v, want each workspace's oldest deterministic row", claimed)
	}
	initialIDs := make([]string, 0, len(initial))
	for _, delivery := range initial {
		initialIDs = append(initialIDs, delivery.ID)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM webhook_deliveries WHERE id = ANY($1)`, initialIDs); err != nil {
		t.Fatal(err)
	}

	// Two simultaneous workers claim the next eight rows. SKIP LOCKED keeps the
	// sets disjoint, while each result preserves workspace rotation.
	concurrent := append(
		fairnessDeliveries(noisyEndpoint.ID, "replica-noisy", 4, base),
		fairnessDeliveries(quietEndpoint.ID, "replica-quiet", 4, base)...,
	)
	if _, err := s.EnqueueWebhookDeliveries(ctx, concurrent, base, "fair-concurrent", 0); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	type claimResult struct {
		attempts []DueWebhookAttempt
		err      error
	}
	results := make(chan claimResult, 2)
	for range 2 {
		go func() {
			<-start
			attempts, claimErr := s.ClaimDueWebhookAttempts(ctx, now, now.Add(time.Minute), 4)
			results <- claimResult{attempts: attempts, err: claimErr}
		}()
	}
	close(start)
	seen := map[string]bool{}
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent claim: %v", result.err)
		}
		if tenants := claimedTenants(result.attempts); tenants[noisy.ID] == 0 || tenants[quiet.ID] == 0 {
			t.Errorf("one replica monopolized a workspace: %#v", tenants)
		}
		for _, attempt := range result.attempts {
			if seen[attempt.ID] {
				t.Errorf("attempt %s was claimed by both replicas", attempt.ID)
			}
			seen[attempt.ID] = true
		}
	}
	if len(seen) != len(concurrent) {
		t.Fatalf("concurrent claims covered %d/%d due attempts", len(seen), len(concurrent))
	}
}

// webhookPGStore is the skip/migrate/connect preamble every webhook pg test
// shares.
func webhookPGStore(t *testing.T, ctx context.Context) (*PGStore, *pgxpool.Pool) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	return NewPGStore(pool), pool
}

func createFairnessEndpoint(t *testing.T, ctx context.Context, s *PGStore, tenantID, name string) WebhookEndpoint {
	t.Helper()
	endpoint, err := s.CreateWebhookEndpoint(
		ctx, tenantID, name, "https://hooks.example.test/"+name,
		"whsec_"+name, []string{"deploy_started"}, true, "user-owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func fairnessDeliveries(endpointID, prefix string, count int, availableAt time.Time) []WebhookDelivery {
	deliveries := make([]WebhookDelivery, 0, count)
	for range count {
		deliveries = append(deliveries, WebhookDelivery{
			ID:            ids.New(ids.WebhookDelivery),
			EndpointID:    endpointID,
			EventID:       prefix + "-" + ids.New(ids.WebhookDelivery),
			EventType:     "deploy_started",
			ServiceID:     "fairness-service",
			Payload:       `{}`,
			NextAttemptAt: availableAt,
		})
	}
	return deliveries
}

func claimedTenants(attempts []DueWebhookAttempt) map[string]int {
	counts := map[string]int{}
	for _, attempt := range attempts {
		counts[attempt.TenantID]++
	}
	return counts
}
