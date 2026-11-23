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

package logs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func subscriptionPGStore(t *testing.T) *store.PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	if _, err := pool.Exec(context.Background(), `DELETE FROM log_subscription_slots`); err != nil {
		t.Fatalf("clean log_subscription_slots: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM log_subscription_slots`)
	})
	return st
}

// w8/m579 DoD 3: two logs.Service replicas sharing Postgres enforce one
// global, per-subject, and per-workspace cap across both instances, and
// closing a stream on one replica immediately frees its slot on the other.
func TestSharedLogSubscriptionCapsAcrossTwoReplicasPG(t *testing.T) {
	st := subscriptionPGStore(t)

	appA := sampleApp("a")
	appA.Labels = map[string]string{core.LabelTenant: "tea-a"}
	appB := sampleApp("b")
	appB.Labels = map[string]string{core.LabelTenant: "tea-b"}
	cl := fakeClientWith(appA, appB)

	newReplica := func() *Service {
		return &Service{
			Base:                    &core.Base{Client: cl, Namespace: "default"},
			MaxSSEConns:             4,
			MaxSSEConnsPerSubject:   2,
			MaxSSEConnsPerWorkspace: 3,
			SubscriptionStore:       st,
		}
	}
	repA := newReplica()
	repB := newReplica()

	subCtx := func(sub string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: sub, Method: core.MethodOAuth2})
	}

	// Subject 1 opens one stream on replica A and one on replica B (per-subject cap = 2).
	rel1, err := repA.acquireSubscription(subCtx("user-1"), LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("user-1 replica A: %v", err)
	}
	defer rel1()
	rel2, err := repB.acquireSubscription(subCtx("user-1"), LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("user-1 replica B: %v", err)
	}
	defer rel2()

	// 3rd stream for user-1 on replica A is refused (per-subject cap reached across replicas).
	if _, err := repA.acquireSubscription(subCtx("user-1"), LogQuery{App: "a"}); !errors.Is(err, errSubscriptionLimit) {
		t.Fatalf("user-1 3rd across replicas = %v, want errSubscriptionLimit", err)
	}
	// HTTP subscribe handler answers 429 too many active log subscriptions.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource=a", nil).WithContext(subCtx("user-1"))
	repA.logsSubscribe(rec, req)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "too many active log subscriptions") {
		t.Fatalf("HTTP 3rd subscribe = %d %q, want 429 too many active log subscriptions", rec.Code, rec.Body.String())
	}

	// user-2 in tea-a takes the 3rd workspace slot on replica B.
	rel3, err := repB.acquireSubscription(subCtx("user-2"), LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("user-2 tea-a replica B: %v", err)
	}
	defer rel3()

	// 4th stream in tea-a (user-3) on replica A hits the per-workspace cap (3).
	if _, err := repA.acquireSubscription(subCtx("user-3"), LogQuery{App: "a"}); !errors.Is(err, errSubscriptionLimit) {
		t.Fatalf("4th in tea-a across replicas = %v, want errSubscriptionLimit", err)
	}

	// user-3 in tea-b takes the 4th global slot on replica A.
	rel4, err := repA.acquireSubscription(subCtx("user-3"), LogQuery{App: "b"})
	if err != nil {
		t.Fatalf("user-3 tea-b replica A: %v", err)
	}
	defer rel4()

	// 5th stream globally (user-4 in tea-b) on replica B hits the global cap (4).
	if _, err := repB.acquireSubscription(subCtx("user-4"), LogQuery{App: "b"}); !errors.Is(err, errSubscriptionLimit) {
		t.Fatalf("5th globally across replicas = %v, want errSubscriptionLimit", err)
	}

	// Releasing user-1's stream on replica A immediately frees its slot on replica B.
	rel1()
	rel5, err := repB.acquireSubscription(subCtx("user-1"), LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("user-1 on replica B after release on A: %v", err)
	}
	rel5()
	rel2()
	rel3()
	rel4()

	if gotA, gotB := repA.sseConns.Load(), repB.sseConns.Load(); gotA != 0 || gotB != 0 {
		t.Fatalf("local sseConns leaked: repA=%d repB=%d", gotA, gotB)
	}
}

// w8/m579 DoD 3: an active stream renews its lease past >1 TTL while an
// abandoned lease (simulating a crashed replica) expires automatically after
// its TTL and is reclaimed on the next acquire.
func TestSharedLogSubscriptionLeaseRenewalAndExpiryPG(t *testing.T) {
	st := subscriptionPGStore(t)

	appA := sampleApp("a")
	appA.Labels = map[string]string{core.LabelTenant: "tea-a"}
	cl := fakeClientWith(appA)

	repA := &Service{
		Base:                      &core.Base{Client: cl, Namespace: "default"},
		MaxSSEConns:               2,
		MaxSSEConnsPerSubject:     1,
		MaxSSEConnsPerWorkspace:   2,
		SubscriptionStore:         st,
		SubscriptionLeaseTTL:      150 * time.Millisecond,
		SubscriptionRenewInterval: 40 * time.Millisecond,
	}
	repB := &Service{
		Base:                    &core.Base{Client: cl, Namespace: "default"},
		MaxSSEConns:             2,
		MaxSSEConnsPerSubject:   1,
		MaxSSEConnsPerWorkspace: 2,
		SubscriptionStore:       st,
		SubscriptionLeaseTTL:    150 * time.Millisecond,
	}

	subCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-live", Method: core.MethodOAuth2})
	rel, err := repA.acquireSubscription(subCtx, LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("acquire live stream: %v", err)
	}
	defer rel()

	// Insert an abandoned lease row (simulating a crashed replica) expiring in 100ms.
	if err := st.AcquireLogSubscriptionSlot(context.Background(), time.Now(), "crashed-slot", "user-crashed", "tea-a", 100*time.Millisecond, 2, 1, 2); err != nil {
		t.Fatalf("seed crashed slot: %v", err)
	}

	// Wait past both the crashed slot's 100ms TTL and repA's 150ms initial TTL:
	// repA's heartbeat must have kept user-live's lease alive while crashed-slot expired.
	time.Sleep(240 * time.Millisecond)

	// user-live is STILL at its per-subject cap (heartbeat kept its lease valid).
	if _, err := repB.acquireSubscription(subCtx, LogQuery{App: "a"}); !errors.Is(err, errSubscriptionLimit) {
		t.Fatalf("user-live on repB past 1.5x TTL = %v, want errSubscriptionLimit (lease must stay renewed)", err)
	}

	// user-crashed's abandoned lease has expired and is reclaimed, so another
	// user can immediately take the 2nd global slot on repB.
	otherCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-other", Method: core.MethodOAuth2})
	relOther, err := repB.acquireSubscription(otherCtx, LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("reclaim expired crashed slot on repB: %v", err)
	}
	relOther()
	rel()
}

type failingSubscriptionStore struct {
	err error
}

func (f failingSubscriptionStore) AcquireLogSubscriptionSlot(context.Context, time.Time, string, string, string, time.Duration, int64, int, int) error {
	return f.err
}

func (f failingSubscriptionStore) RenewLogSubscriptionSlot(context.Context, string, time.Time) error {
	return f.err
}

func (f failingSubscriptionStore) ReleaseLogSubscriptionSlot(context.Context, string) error {
	return f.err
}

// w8/m579 DoD 4: when Postgres is unavailable, acquireSlot falls back fail-open
// to the in-process counters without returning 500s and still enforces caps.
func TestSharedLogSubscriptionFailsOpenToLocalOnStoreError(t *testing.T) {
	appA := sampleApp("a")
	appA.Labels = map[string]string{core.LabelTenant: "tea-a"}
	s := &Service{
		Base:                    &core.Base{Client: fakeClientWith(appA), Namespace: "default"},
		MaxSSEConns:             5,
		MaxSSEConnsPerSubject:   1,
		MaxSSEConnsPerWorkspace: 5,
		SubscriptionStore:       failingSubscriptionStore{err: errors.New("postgres unavailable")},
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: core.MethodOAuth2})
	rel, err := s.acquireSubscription(ctx, LogQuery{App: "a"})
	if err != nil {
		t.Fatalf("first acquire during DB outage = %v, want nil (fail-open)", err)
	}
	if _, err := s.acquireSubscription(ctx, LogQuery{App: "a"}); !errors.Is(err, errSubscriptionLimit) {
		t.Fatalf("second acquire during DB outage = %v, want errSubscriptionLimit (local cap)", err)
	}
	rel()
	if got := s.sseConns.Load(); got != 0 {
		t.Fatalf("sseConns leaked after fallback release: %d", got)
	}
}
