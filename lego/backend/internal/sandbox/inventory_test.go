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

package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestReconcileWorkspaceInventoryReclaimsOrphans(t *testing.T) {
	created := time.Now().UTC().Add(-2 * time.Hour)
	var mu sync.Mutex
	deleted := map[string]bool{}
	sandboxes := []map[string]any{
		{
			"id": "orphan-terminal",
			"metadata": map[string]string{
				metadataWorkspace: "tea-a", metadataOwner: "id-a",
				metadataNetworkPolicy: "deny-all", metadataRegime: metadataSandboxRegime,
				agentsession.LabelSession: "ags-terminal", metadataTimeout: "86400",
			},
			"status":  map[string]string{"state": "Running"},
			"created": created.Format(time.RFC3339Nano),
		},
		{
			"id": "orphan-norow",
			"metadata": map[string]string{
				metadataWorkspace: "tea-a", metadataOwner: "id-a",
				metadataNetworkPolicy: "deny-all", metadataRegime: metadataSandboxRegime,
				agentsession.LabelSession: "ags-missing", metadataTimeout: "86400",
			},
			"status":  map[string]string{"state": "Running"},
			"created": created.Format(time.RFC3339Nano),
		},
		{
			"id": "claimed-live",
			"metadata": map[string]string{
				metadataWorkspace: "tea-a", metadataOwner: "id-a",
				metadataNetworkPolicy: "deny-all", metadataRegime: metadataSandboxRegime,
				agentsession.LabelSession: "ags-live", metadataTimeout: "86400",
			},
			"status":  map[string]string{"state": "Running"},
			"created": created.Format(time.RFC3339Nano),
		},
		{
			"id": "ea-inside-bound",
			"metadata": map[string]string{
				metadataWorkspace: "tea-a", metadataOwner: "id-a",
				metadataNetworkPolicy: "deny-all", metadataRegime: metadataSandboxRegime,
				metadataTimeout: "86400",
			},
			"status":  map[string]string{"state": "Running"},
			"created": created.Format(time.RFC3339Nano),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			var live []map[string]any
			for _, s := range sandboxes {
				id, _ := s["id"].(string)
				if deleted[id] {
					continue
				}
				live = append(live, s)
			}
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == http.MethodDelete:
			id := r.URL.Path[len("/sandboxes/"):]
			deleted[id] = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(srv.URL),
		Keys:   staticKey("ws-key"),
	}
	lookup := func(_ context.Context, sessionID string) (SessionClaim, error) {
		switch sessionID {
		case "ags-terminal":
			return SessionClaim{Found: true, Phase: "canceled", SandboxID: ""}, nil
		case "ags-live":
			return SessionClaim{Found: true, Phase: "running", SandboxID: "claimed-live"}, nil
		default:
			return SessionClaim{}, nil
		}
	}
	counts, err := NewAgentSessionLifecycle(svc).ReconcileWorkspaceInventory(context.Background(), "tea-a", "ws-key", time.Now().UTC(), lookup)
	if err != nil {
		t.Fatal(err)
	}
	if counts.TerminalOrphan != 1 || counts.NoRowOrphan != 1 || counts.Claimed != 1 || counts.Timed != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if counts.Terminated != 2 {
		t.Fatalf("terminated = %d, want 2", counts.Terminated)
	}
	mu.Lock()
	defer mu.Unlock()
	if !deleted["orphan-terminal"] || !deleted["orphan-norow"] {
		t.Fatalf("orphans not deleted: %v", deleted)
	}
	if deleted["claimed-live"] || deleted["ea-inside-bound"] {
		t.Fatalf("live/in-bound sandboxes must stay: %v", deleted)
	}
}

// TestReconcileWorkspaceInventoryHonorsLongLifetimes: a paid workspace's
// no-timeout or 30-day sandbox outlives 24 h untouched and is counted as
// long_lived (which SandboxOldestOverBound excludes); a 30-day sandbox past
// its own deadline and a legacy "0" past 24 h are still reaped (w2/m263).
func TestReconcileWorkspaceInventoryHonorsLongLifetimes(t *testing.T) {
	now := time.Now().UTC()
	box := func(id, timeout string, age time.Duration) map[string]any {
		return map[string]any{
			"id": id,
			"metadata": map[string]string{
				metadataWorkspace: "tea-a", metadataOwner: "id-a",
				metadataNetworkPolicy: "deny-all", metadataRegime: metadataSandboxRegime,
				metadataTimeout: timeout,
			},
			"status":    map[string]string{"state": "Running"},
			"createdAt": now.Add(-age).Format(time.RFC3339Nano),
		}
	}
	sandboxes := []map[string]any{
		box("no-timeout", timeoutNone, 90*24*time.Hour),
		box("thirty-days", "2592000", 10*24*time.Hour),
		box("thirty-days-over", "2592000", 31*24*time.Hour),
		box("legacy-zero", "0", 25*time.Hour),
	}
	var mu sync.Mutex
	deleted := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodDelete {
			deleted[r.URL.Path[len("/sandboxes/"):]] = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(sandboxes)
	}))
	t.Cleanup(srv.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(srv.URL),
		Keys:   staticKey("ws-key"),
	}
	counts, err := NewAgentSessionLifecycle(svc).ReconcileWorkspaceInventory(context.Background(), "tea-a", "ws-key", now,
		func(context.Context, string) (SessionClaim, error) { return SessionClaim{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if deleted["no-timeout"] || deleted["thirty-days"] || !deleted["thirty-days-over"] || !deleted["legacy-zero"] {
		t.Fatalf("deleted = %v counts %+v", deleted, counts)
	}
	if counts.LongLived != 2 || counts.Timed != 2 || counts.Terminated != 2 {
		t.Fatalf("counts = %+v", counts)
	}
	if age := counts.OldestAge[InventoryClassLongLived]; age < 89*24*3600 {
		t.Fatalf("long_lived oldest age = %v", age)
	}
}

// TestReconcileWorkspaceInventoryRestartsTimeoutOnResume: the reconcile reads
// the lifetime from the last resume, never reaps a suspended sandbox before
// 30 days suspended, and counts both as long_lived once their deadline is
// past createdAt + 24 h (w2/m278 t010, Render's sandboxes-limits).
func TestReconcileWorkspaceInventoryRestartsTimeoutOnResume(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	day := 24 * time.Hour
	box := func(id, state, timeout string, age time.Duration, stamps map[string]time.Duration) map[string]any {
		md := map[string]string{
			metadataWorkspace: "tea-a", metadataOwner: "id-a",
			metadataNetworkPolicy: "deny-all", metadataRegime: metadataSandboxRegime,
			metadataTimeout: timeout,
		}
		for k, ago := range stamps {
			md[k] = stampMetadataTime(now.Add(-ago))
		}
		return map[string]any{
			"id": id, "metadata": md,
			"status":    map[string]string{"state": state},
			"createdAt": now.Add(-age).Format(time.RFC3339Nano),
		}
	}
	sandboxes := []map[string]any{
		box("resumed-in-bound", "Running", "86400", 3*day, map[string]time.Duration{metadataSuspendedAt: 2 * day, metadataResumedAt: 2 * time.Hour}),
		box("resumed-over", "Running", "3600", 3*day, map[string]time.Duration{metadataResumedAt: 2 * time.Hour}),
		box("suspended-long", "Paused", "86400", 10*day, map[string]time.Duration{metadataSuspendedAt: 9 * day}),
		box("suspended-no-timeout", "Paused", timeoutNone, 40*day, map[string]time.Duration{metadataSuspendedAt: 31 * day}),
		box("suspended-over", "Paused", "86400", 40*day, map[string]time.Duration{metadataSuspendedAt: 31 * day}),
		box("never-resumed", "Running", "86400", 25*time.Hour, nil),
		box("fresh", "Running", "3600", 10*time.Minute, nil),
	}
	var mu sync.Mutex
	deleted := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodDelete {
			deleted[r.URL.Path[len("/sandboxes/"):]] = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(sandboxes)
	}))
	t.Cleanup(srv.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(srv.URL),
		Keys:   staticKey("ws-key"),
	}
	counts, err := NewAgentSessionLifecycle(svc).ReconcileWorkspaceInventory(context.Background(), "tea-a", "ws-key", now,
		func(context.Context, string) (SessionClaim, error) { return SessionClaim{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := map[string]bool{"resumed-over": true, "suspended-no-timeout": true, "suspended-over": true, "never-resumed": true}
	for _, s := range sandboxes {
		id := s["id"].(string)
		if deleted[id] != want[id] {
			t.Errorf("%s deleted = %v, want %v", id, deleted[id], want[id])
		}
	}
	if counts.LongLived != 2 || counts.Timed != 5 || counts.Terminated != 4 {
		t.Fatalf("counts = %+v", counts)
	}
	if age := counts.OldestAge[InventoryClassLongLived]; age < 9*24*3600 {
		t.Fatalf("long_lived oldest age = %v, want the suspended sandbox's 10 days", age)
	}
}

func TestReconcileWorkspaceInventoryListErrorTerminatesNothing(t *testing.T) {
	var deletes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletes++
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(srv.URL),
		Keys:   staticKey("ws-key"),
	}
	_, err := NewAgentSessionLifecycle(svc).ReconcileWorkspaceInventory(context.Background(), "tea-a", "ws-key", time.Now().UTC(),
		func(context.Context, string) (SessionClaim, error) { return SessionClaim{}, nil })
	if err == nil {
		t.Fatal("expected list error")
	}
	if deletes != 0 {
		t.Fatalf("deletes = %d, want 0", deletes)
	}
}

func TestSetInventoryMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	m.SetInventory(InventoryCounts{
		Claimed: 2, TerminalOrphan: 1, NoRowOrphan: 3, Timed: 4,
		OldestAge: map[string]float64{
			InventoryClassTerminalOrphan: 900,
			InventoryClassNoRowOrphan:    1200,
		},
	})
	if got := testutil.ToFloat64(m.live.WithLabelValues(InventoryClassTerminalOrphan)); got != 1 {
		t.Fatalf("terminal_orphan gauge = %v", got)
	}
	if got := testutil.ToFloat64(m.oldestAge.WithLabelValues(InventoryClassNoRowOrphan)); got != 1200 {
		t.Fatalf("oldest no_row = %v", got)
	}
	m.ObserveTeardownFailure(TeardownReasonPreviousSandbox)
	if got := testutil.ToFloat64(m.teardownFailures.WithLabelValues(TeardownReasonPreviousSandbox)); got != 1 {
		t.Fatalf("teardown failures = %v", got)
	}
}

// A finished session that still names its sandbox is in its idle grace (or held
// by an open editor SSH session): the Completer's reap owns that teardown, so
// inventory must not cut the 2h idle window short (w10/m15). A finished session
// that names a different sandbox, or none, is an orphan.
func TestClassifySessionSandboxLeavesIdleGraceToTheCompleter(t *testing.T) {
	cases := []struct {
		name  string
		claim SessionClaim
		stop  bool
	}{
		{"completed names this sandbox", SessionClaim{Found: true, Phase: "completed", SandboxID: "sb"}, false},
		{"failed names this sandbox", SessionClaim{Found: true, Phase: "failed", SandboxID: "sb"}, false},
		{"completed after reap", SessionClaim{Found: true, Phase: "completed"}, true},
		{"completed names another sandbox", SessionClaim{Found: true, Phase: "completed", SandboxID: "other"}, true},
		{"canceling names this sandbox", SessionClaim{Found: true, Phase: "canceling", SandboxID: "sb"}, true},
		{"redispatching blanked", SessionClaim{Found: true, Phase: "redispatching"}, false},
		{"running names another sandbox", SessionClaim{Found: true, Phase: "running", SandboxID: "other"}, true},
		{"no row", SessionClaim{}, true},
	}
	for _, tc := range cases {
		if _, stop := classifySessionSandbox(tc.claim, "sb"); stop != tc.stop {
			t.Errorf("%s: shouldStop = %v, want %v", tc.name, stop, tc.stop)
		}
	}
}
