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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func webhookIntervals(t *testing.T, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, endpointID string) []WebhookEligibility {
	t.Helper()
	rows, err := q.Query(context.Background(),
		`SELECT enabled_at, disabled_at FROM webhook_endpoint_eligibility
		  WHERE endpoint_id = $1 ORDER BY enabled_at`, endpointID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []WebhookEligibility
	for rows.Next() {
		var i WebhookEligibility
		if err := rows.Scan(&i.EnabledAt, &i.DisabledAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func dispatchEndpoint(t *testing.T, st *PGStore, since time.Time, id string) (WebhookEndpoint, bool) {
	t.Helper()
	endpoints, err := st.ListWebhookDispatchEndpoints(context.Background(), since)
	if err != nil {
		t.Fatalf("dispatch endpoints: %v", err)
	}
	for _, e := range endpoints {
		if e.ID == id {
			return e, true
		}
	}
	return WebhookEndpoint{}, false
}

// TestPGWebhookEligibilityFollowsEveryWriter (w4/m212): create, the manual
// toggle, the full update and the automatic disable all open or close the same
// interval history; a save that leaves enabled unchanged and a write that fails
// leave it alone; the dispatch read keeps an endpoint exactly while it has an
// interval the feed has not passed; pruning reclaims only closed intervals.
func TestPGWebhookEligibilityFollowsEveryWriter(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	ten := lifecycleTenant(t, st, "whelig")
	types := []string{"deploy_ended"}

	on, err := st.CreateWebhookEndpoint(ctx, ten.ID, "on", "https://a.example/hook", "whsec_on", types, true, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	off, err := st.CreateWebhookEndpoint(ctx, ten.ID, "off", "https://b.example/hook", "whsec_off", types, false, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := webhookIntervals(t, st.Pool, on.ID); len(got) != 1 || !got[0].EnabledAt.Equal(on.CreatedAt) || got[0].DisabledAt != nil {
		t.Fatalf("enabled create intervals = %+v, want one open from created_at %v", got, on.CreatedAt)
	}
	if got := webhookIntervals(t, st.Pool, off.ID); len(got) != 0 {
		t.Fatalf("disabled create intervals = %+v, want none", got)
	}

	setOn := func(enabled bool) {
		t.Helper()
		if _, err := st.SetWebhookEndpointEnabled(ctx, ten.ID, on.ID, enabled, "paused"); err != nil {
			t.Fatalf("set enabled=%v: %v", enabled, err)
		}
	}
	setOn(false)
	setOn(false) // no-op
	first := webhookIntervals(t, st.Pool, on.ID)
	if len(first) != 1 || first[0].DisabledAt == nil || first[0].DisabledAt.Before(first[0].EnabledAt) {
		t.Fatalf("after disable = %+v, want the interval closed once", first)
	}
	setOn(true)
	setOn(true) // no-op
	second := webhookIntervals(t, st.Pool, on.ID)
	if len(second) != 2 || second[1].DisabledAt != nil || !second[1].EnabledAt.After(*second[0].DisabledAt) ||
		!second[0].DisabledAt.Equal(*first[0].DisabledAt) {
		t.Fatalf("after re-enable = %+v, want a new open interval after the closed one", second)
	}

	// An unrelated full update changes nothing; toggling through it does.
	if _, err := st.UpdateWebhookEndpoint(ctx, ten.ID, on.ID, "renamed", "https://c.example/hook", types, true); err != nil {
		t.Fatal(err)
	}
	if got := webhookIntervals(t, st.Pool, on.ID); len(got) != 2 || got[1].DisabledAt != nil {
		t.Fatalf("after no-op update = %+v", got)
	}
	if _, err := st.UpdateWebhookEndpoint(ctx, ten.ID, on.ID, "renamed", "https://c.example/hook", types, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateWebhookEndpoint(ctx, ten.ID, on.ID, "renamed", "https://c.example/hook", types, true); err != nil {
		t.Fatal(err)
	}
	if got := webhookIntervals(t, st.Pool, on.ID); len(got) != 3 || got[1].DisabledAt == nil || got[2].DisabledAt != nil {
		t.Fatalf("after update toggle = %+v, want closed, closed, open", got)
	}

	// A write the store refuses (another workspace) leaves the history alone.
	other := lifecycleTenant(t, st, "whelig-other")
	if _, err := st.SetWebhookEndpointEnabled(ctx, other.ID, on.ID, false, ""); err == nil {
		t.Fatal("cross-workspace toggle succeeded")
	}
	if got := webhookIntervals(t, st.Pool, on.ID); len(got) != 3 || got[2].DisabledAt != nil {
		t.Fatalf("after refused write = %+v", got)
	}

	// The automatic disable after repeated failures closes the open interval.
	now := time.Now().UTC()
	delivery := WebhookDelivery{
		ID: ids.New(ids.WebhookDelivery), EndpointID: on.ID, EventID: "evt-m212autodisable0",
		EventType: "deploy_ended", ServiceID: "svc", Payload: `{}`, NextAttemptAt: now,
	}
	if _, err := st.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{delivery}, now, "m212", 0); err != nil {
		t.Fatal(err)
	}
	if done, err := st.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: delivery.ID, StatusCode: 503, CompletedAt: now, NextAttemptAt: now,
		Exhausted: true, DisableReason: "disabled automatically after repeated delivery failures",
	}); err != nil || !done {
		t.Fatalf("exhaust: %v %v", done, err)
	}
	final := webhookIntervals(t, st.Pool, on.ID)
	if len(final) != 3 || final[2].DisabledAt == nil {
		t.Fatalf("after automatic disable = %+v, want every interval closed", final)
	}

	// Enabling the initially disabled endpoint opens its first interval.
	if _, err := st.SetWebhookEndpointEnabled(ctx, ten.ID, off.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if got := webhookIntervals(t, st.Pool, off.ID); len(got) != 1 || got[0].DisabledAt != nil || got[0].EnabledAt.Before(off.CreatedAt) {
		t.Fatalf("initially disabled, then enabled = %+v", got)
	}

	// The dispatch read keeps a disabled endpoint while its last interval ends
	// after `since`, with every retained interval (a late-recorded fact can
	// belong to any of them), and drops it once `since` passes that end.
	before := final[0].EnabledAt.Add(-time.Second)
	if e, ok := dispatchEndpoint(t, st, before, on.ID); !ok || len(e.Eligibility) != 3 {
		t.Fatalf("dispatch read before every interval = %+v (present %v)", e.Eligibility, ok)
	}
	if e, ok := dispatchEndpoint(t, st, *final[1].DisabledAt, on.ID); !ok || len(e.Eligibility) != 3 ||
		!e.Eligibility[2].EnabledAt.Equal(final[2].EnabledAt) {
		t.Fatalf("dispatch read at the second close = %+v (present %v), want all three intervals", e.Eligibility, ok)
	}
	if _, ok := dispatchEndpoint(t, st, *final[2].DisabledAt, on.ID); ok {
		t.Fatal("an endpoint whose last interval ended before `since` is still dispatched")
	}
	if _, ok := dispatchEndpoint(t, st, time.Now().Add(time.Hour), off.ID); !ok {
		t.Fatal("an enabled endpoint dropped out of the dispatch read")
	}

	// Pruning removes only closed intervals that ended before the horizon.
	if _, err := st.PruneWebhookEligibility(ctx, final[1].DisabledAt.Add(time.Microsecond)); err != nil {
		t.Fatal(err)
	}
	if got := webhookIntervals(t, st.Pool, on.ID); len(got) != 1 || !got[0].EnabledAt.Equal(final[2].EnabledAt) {
		t.Fatalf("after prune = %+v, want only the last closed interval", got)
	}
	if got := webhookIntervals(t, st.Pool, off.ID); len(got) != 1 {
		t.Fatalf("prune touched an open interval: %+v", got)
	}

	if err := st.DeleteWebhookEndpoint(ctx, ten.ID, on.ID); err != nil {
		t.Fatal(err)
	}
	if got := webhookIntervals(t, st.Pool, on.ID); len(got) != 0 {
		t.Fatalf("deleted endpoint kept intervals: %+v", got)
	}
}

// TestWebhookEligibilityMigrationBackfill applies 0148 to an isolated schema:
// an enabled endpoint is eligible since creation (the CreatedAt guard it had),
// a disabled one for [created_at, updated_at) — never shorter than its real
// enabled stretch — and the trigger then tracks later writes.
func TestWebhookEligibilityMigrationBackfill(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	up, err := migrationsFS.ReadFile("migrations/0148_webhook_endpoint_eligibility.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0148_webhook_endpoint_eligibility.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0148_webhook_eligibility;
		SET LOCAL search_path TO migration_0148_webhook_eligibility;
		CREATE TABLE webhook_endpoints (
			id text PRIMARY KEY,
			enabled boolean NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		);
		INSERT INTO webhook_endpoints VALUES
			('whk-on', true, '2026-10-01T00:00:00Z', '2026-10-05T00:00:00Z'),
			('whk-off', false, '2026-10-01T00:00:00Z', '2026-10-03T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply up: %v", err)
	}
	read := func(id string) []WebhookEligibility { t.Helper(); return webhookIntervals(t, tx, id) }
	created := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if got := read("whk-on"); len(got) != 1 || !got[0].EnabledAt.Equal(created) || got[0].DisabledAt != nil {
		t.Fatalf("enabled backfill = %+v", got)
	}
	if got := read("whk-off"); len(got) != 1 || !got[0].EnabledAt.Equal(created) || got[0].DisabledAt == nil ||
		!got[0].DisabledAt.Equal(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("disabled backfill = %+v", got)
	}
	if _, err := tx.Exec(ctx, `UPDATE webhook_endpoints SET enabled = true WHERE id = 'whk-off'`); err != nil {
		t.Fatal(err)
	}
	if got := read("whk-off"); len(got) != 2 || got[1].DisabledAt != nil {
		t.Fatalf("trigger after migration = %+v", got)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("apply down: %v", err)
	}
}
