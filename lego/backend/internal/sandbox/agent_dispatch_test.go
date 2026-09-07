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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestDispatchCleanupFindsOnlyOwnedGenerationAndPredecessor(t *testing.T) {
	raw := func(id, workspace, session, turn string) osSandbox {
		return osSandbox{ID: id, Metadata: map[string]string{
			metadataOwner: "alice", metadataWorkspace: workspace, metadataRegime: metadataSandboxRegime, metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: session, agentsession.LabelDispatchTurn: turn,
		}}
	}
	rows := []osSandbox{raw("orphan", "tea-a", "ags-a", "2"), raw("previous", "tea-a", "ags-a", "1"), raw("newer", "tea-a", "ags-a", "3"), raw("foreign", "tea-b", "ags-a", "2"), raw("other", "tea-a", "ags-b", "2"), raw("legacy", "tea-a", "ags-legacy", ""), raw("new-legacy", "tea-a", "ags-legacy", "4")}
	deleted := []string{}
	lists := 0
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/sandboxes" {
			lists++
			_ = json.NewEncoder(w).Encode(rows)
			return
		}
		if r.Method == http.MethodDelete {
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/sandboxes/"))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	})
	eg := &fakeSessionEgress{}
	svc.SessionEgress = eg
	err := NewAgentSessionLifecycle(svc).CleanupAgentDispatches(context.Background(), []store.AgentDispatch{
		{SessionID: "ags-a", WorkspaceID: "tea-a", Turn: 2, PreviousSandboxID: "previous"},
		{SessionID: "ags-legacy", WorkspaceID: "tea-a", Turn: 1, Legacy: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if lists != 1 || !slices.Equal(deleted, []string{"orphan", "previous", "legacy"}) {
		t.Fatalf("lists=%d deleted=%v", lists, deleted)
	}
	if len(eg.calls) != 0 {
		t.Fatalf("cleanup altered newer generation session policy: %+v", eg.calls)
	}
}

func TestDispatchCleanupContinuesAfterOneDeletionFails(t *testing.T) {
	deleted := []string{}
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]osSandbox{
				{ID: "stuck", Metadata: map[string]string{metadataOwner: "alice", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime, metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-a", agentsession.LabelDispatchTurn: "1"}},
				{ID: "removable", Metadata: map[string]string{metadataOwner: "alice", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime, metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-a", agentsession.LabelDispatchTurn: "1"}},
			})
			return
		}
		deleted = append(deleted, r.URL.Path)
		if r.URL.Path == "/sandboxes/stuck" {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	err := NewAgentSessionLifecycle(svc).CleanupAgentDispatches(context.Background(), []store.AgentDispatch{{SessionID: "ags-a", WorkspaceID: "tea-a", Turn: 1}})
	if err == nil || !slices.Equal(deleted, []string{"/sandboxes/stuck", "/sandboxes/removable"}) {
		t.Fatalf("deletions=%v error=%v", deleted, err)
	}
}

func TestDispatchCleanupDeletedSessionRetriesLatePolicy(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode([]osSandbox{}) })
	eg := &fakeSessionEgress{}
	svc.SessionEgress = eg
	d := store.AgentDispatch{SessionID: "ags-deleted", WorkspaceID: "tea-a", Turn: 1, SessionDeleted: true}
	for range 2 {
		if err := NewAgentSessionLifecycle(svc).CleanupAgentDispatches(context.Background(), []store.AgentDispatch{d}); err != nil {
			t.Fatal(err)
		}
	}
	if len(eg.calls) != 2 || eg.calls[0].op != "delete" || eg.calls[1].session != "ags-deleted" {
		t.Fatalf("late policy cleanup not retried: %+v", eg.calls)
	}
}
