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
