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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"

	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
)

// TestDisabledWebhookIntervalNeverReplays is the w4/m212 a675 replay against
// the real feed, store and a real receiver. A deploy that ends while endpoint A
// is disabled must never reach A after it is enabled again — whether or not
// another endpoint B consumed the event first (the a675 passing control) —
// while a deploy that ended before the disable but was not dispatched yet is
// still delivered (enabled-period backlog), and one after re-enable is too.
func TestDisabledWebhookIntervalNeverReplays(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
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

	for _, tc := range []struct {
		name      string
		withPeerB bool
	}{
		{name: "alone"},
		{name: "with an enabled peer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Serial inside the store binary: no other endpoint, delivery or
			// watermark may steer this worker.
			for _, tbl := range []string{"webhook_deliveries", "webhook_endpoints", "webhook_watermark"} {
				if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
					t.Fatalf("clean %s: %v", tbl, err)
				}
			}
			var mu sync.Mutex
			got := map[string][]string{} // receiver path -> service names
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var p struct {
					Data struct {
						ServiceName string `json:"serviceName"`
					} `json:"data"`
				}
				_ = json.Unmarshal(body, &p)
				mu.Lock()
				got[r.URL.Path] = append(got[r.URL.Path], p.Data.ServiceName)
				mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			ten, err := s.CreateTenant(ctx, fmt.Sprintf("wh-m212-%d", time.Now().UnixNano()%1e12), store.PlanHobby)
			if err != nil {
				t.Fatalf("tenant: %v", err)
			}
			subscribe := []string{webhooks.TypeDeployEnded}
			a, err := s.CreateWebhookEndpoint(ctx, ten.ID, "a", srv.URL+"/a", "whsec_a", subscribe, true, "user-1")
			if err != nil {
				t.Fatalf("endpoint a: %v", err)
			}
			if tc.withPeerB {
				if _, err := s.CreateWebhookEndpoint(ctx, ten.ID, "b", srv.URL+"/b", "whsec_b", subscribe, true, "user-1"); err != nil {
					t.Fatalf("endpoint b: %v", err)
				}
			}
			var offset time.Duration
			worker := &webhooks.Worker{Store: s, Client: srv.Client(), Clock: func() time.Time { return time.Now().UTC().Add(offset) }}
			if err := worker.RunOnce(ctx); err != nil { // seeds the watermark before any event
				t.Fatalf("seed pass: %v", err)
			}
			deployEnds := func(name string) {
				t.Helper()
				app, err := s.CreateApp(ctx, store.App{TenantID: ten.ID, Name: name, Image: "img", Port: 80, Replicas: 1, Tier: "free"})
				if err != nil {
					t.Fatalf("app %s: %v", name, err)
				}
				deploys, err := s.ListDeploys(ctx, app.ID, store.DeployFilter{})
				if err != nil || len(deploys) != 1 {
					t.Fatalf("deploys of %s = %+v, %v", name, deploys, err)
				}
				if _, err := s.TransitionDeploy(ctx, deploys[0].ID, store.DeployLive, "", "", "", "", nil); err != nil {
					t.Fatalf("close %s: %v", name, err)
				}
			}
			setA := func(enabled bool) {
				t.Helper()
				if _, err := s.SetWebhookEndpointEnabled(ctx, ten.ID, a.ID, enabled, ""); err != nil {
					t.Fatalf("set a enabled=%v: %v", enabled, err)
				}
			}

			deployEnds("backlog") // enabled, not dispatched before the disable
			setA(false)
			deployEnds("disabled") // inside A's disabled stretch
			if tc.withPeerB {
				// The control: B's pass consumes the disabled-period event first.
				offset = 2 * store.FeedCommitLag
				for range 2 {
					if err := worker.RunOnce(ctx); err != nil {
						t.Fatalf("peer pass: %v", err)
					}
				}
			}
			setA(true)
			deployEnds("reenabled")

			offset = 2 * store.FeedCommitLag
			for range 3 {
				if err := worker.RunOnce(ctx); err != nil {
					t.Fatalf("pass: %v", err)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			gotA := slices.Sorted(slices.Values(got["/a"]))
			if want := []string{"backlog", "reenabled"}; !slices.Equal(gotA, want) {
				t.Errorf("A received %v, want %v (never the disabled-period deploy)", gotA, want)
			}
			if tc.withPeerB {
				gotB := slices.Sorted(slices.Values(got["/b"]))
				if want := []string{"backlog", "disabled", "reenabled"}; !slices.Equal(gotB, want) {
					t.Errorf("B received %v, want %v", gotB, want)
				}
			}
		})
	}
}
