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
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func byIDRoutingStore(t *testing.T) (*PGStore, *pgxpool.Pool) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewPGStore(pool), pool
}

// TestServiceEventWorkspacesRoutesByID is w4/m172's events routing read: every
// workspace indexing an evt-… id, ordered, ErrNotFound for an unknown id.
func TestServiceEventWorkspacesRoutesByID(t *testing.T) {
	s, pool := byIDRoutingStore(t)
	ctx := context.Background()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	var tenants []string
	for _, suffix := range []string{"a", "b"} {
		tenant, err := s.CreateTenant(ctx, "byid-event-route-"+suffix+"-"+stamp, PlanHobby)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
		tenants = append(tenants, tenant.ID)
	}
	slices.Sort(tenants)
	index := func(workspace, key string) string {
		t.Helper()
		var eventID string
		if err := pool.QueryRow(ctx, `INSERT INTO service_event_index
			(workspace_id, event_key, source, source_row_id, phase, service_id, service_name)
			VALUES ($1, $2, 'audit', $3, '', 'dpg-byid', 'byid') RETURNING event_id`,
			workspace, key, key+"-"+workspace).Scan(&eventID); err != nil {
			t.Fatal(err)
		}
		if eventID != ids.Derive(ids.Event, key) {
			t.Fatalf("indexed id %q drifted from ids.Derive", eventID)
		}
		return eventID
	}
	own := index(tenants[1], "byid-own-"+stamp+":")
	if got, err := s.ServiceEventWorkspaces(ctx, own); err != nil || !slices.Equal(got, []string{tenants[1]}) {
		t.Fatalf("own event = %v, %v; want [%s]", got, err, tenants[1])
	}
	// One event key indexed under two workspaces (a workspace:default audit row
	// attributed to each owner): both, in workspace order.
	shared := "byid-shared-" + stamp + ":"
	index(tenants[1], shared)
	sharedID := index(tenants[0], shared)
	if got, err := s.ServiceEventWorkspaces(ctx, sharedID); err != nil || !slices.Equal(got, tenants) {
		t.Fatalf("shared event = %v, %v; want %v", got, err, tenants)
	}
	if _, err := s.ServiceEventWorkspaces(ctx, ids.Derive(ids.Event, "byid-missing-"+stamp)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id = %v, want ErrNotFound", err)
	}
}

// TestPushNotificationWorkspacesRoutesBySubject is w4/m172's mark-read routing
// read: only the subject's own inbox rows, across all their workspaces.
func TestPushNotificationWorkspacesRoutesBySubject(t *testing.T) {
	s, pool := byIDRoutingStore(t)
	ctx := context.Background()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	alice, mallory := "byid-alice-"+stamp, "byid-mallory-"+stamp
	var tenants []string
	for _, suffix := range []string{"a", "b"} {
		tenant, err := s.CreateWorkspace(ctx, "byid-push-route-"+suffix+"-"+stamp, PlanHobby, alice)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
		tenants = append(tenants, tenant.ID)
	}
	slices.Sort(tenants)
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'member')`, tenants[1], mallory); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	insert := func(tenant, subject, key string) string {
		t.Helper()
		n := validPushNotificationForTest(tenant, subject, ids.New(ids.Service), now)
		n.SourceEventKey, n.EventID = key, ids.Derive(ids.Event, key)
		if _, err := pool.Exec(ctx, `INSERT INTO push_notifications
			(tenant_id, subject, source_event_key, event_id, event_type, title, body, urgency,
			 resource_kind, resource_id, deep_link, occurred_at, deliver_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			n.TenantID, n.Subject, n.SourceEventKey, n.EventID, n.EventType, n.Title, n.Body, n.Urgency,
			n.ResourceKind, n.ResourceID, n.DeepLink, n.OccurredAt, n.DeliverAt); err != nil {
			t.Fatal(err)
		}
		return n.EventID
	}
	aliceB := insert(tenants[1], alice, "byid-alice-b-"+stamp)
	malloryB := insert(tenants[1], mallory, "byid-mallory-b-"+stamp)
	if got, err := s.PushNotificationWorkspaces(ctx, alice, aliceB); err != nil || !slices.Equal(got, []string{tenants[1]}) {
		t.Fatalf("own item = %v, %v; want [%s]", got, err, tenants[1])
	}
	if _, err := s.PushNotificationWorkspaces(ctx, alice, malloryB); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another member's item = %v, want ErrNotFound", err)
	}
	if _, err := s.PushNotificationWorkspaces(ctx, alice, ids.Derive(ids.Event, "byid-missing-"+stamp)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id = %v, want ErrNotFound", err)
	}
	shared := "byid-shared-" + stamp
	insert(tenants[0], alice, shared)
	sharedID := insert(tenants[1], alice, shared)
	if got, err := s.PushNotificationWorkspaces(ctx, alice, sharedID); err != nil || !slices.Equal(got, tenants) {
		t.Fatalf("shared item = %v, %v; want %v", got, err, tenants)
	}
}
