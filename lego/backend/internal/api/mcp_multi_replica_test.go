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

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type sharedMCPSelections struct {
	mu    sync.Mutex
	byKey map[string]string
}

func (s *sharedMCPSelections) GetMCPWorkspaceSelection(_ context.Context, sessionID, subject string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byKey[sessionID+"\x00"+subject]
	return id, ok, nil
}

func (s *sharedMCPSelections) SetMCPWorkspaceSelection(_ context.Context, sessionID, subject, workspaceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byKey[sessionID+"\x00"+subject] = workspaceID
	return nil
}

func TestMCPStreamableHTTPAlternatesReplicas(t *testing.T) {
	workspaceStore := newFakeWSStore()
	first := mustCreate(t, workspaceStore, "first", "hobby", "client-1")
	second := mustCreate(t, workspaceStore, "second", "hobby", "client-1")
	shared := &sharedMCPSelections{byKey: map[string]string{}}
	base := &core.Base{Client: fakeClient(
		appWithOwnerLabel("first-web", first.ID),
		appWithOwnerLabel("second-web", second.ID),
	), Namespace: "default"}

	deps := Deps{WorkspaceStore: workspaceStore, MCPWorkspaceSelections: shared}
	replicas := [2]http.Handler{
		NewServer(base, deps).mcpHTTPHandler(),
		NewServer(base, deps).mcpHTTPHandler(),
	}
	var next atomic.Uint64
	var hits [2]atomic.Uint64
	loadBalancer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int((next.Add(1) - 1) % 2)
		hits[i].Add(1)
		ctx := core.WithIdentity(r.Context(), core.Identity{Subject: "client-1", Method: "session"})
		replicas[i].ServeHTTP(w, r.WithContext(ctx))
	}))
	defer loadBalancer.Close()

	ctx := context.Background()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "multi-replica-test", Version: "0"}, nil).Connect(
		ctx,
		&mcp.StreamableClientTransport{Endpoint: loadBalancer.URL},
		nil,
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cs.Close()

	selected := callTool[struct {
		Selected struct{ ID string }
	}](t, cs, "select_workspace", map[string]any{"ownerID": second.ID})
	if selected.Selected.ID != second.ID {
		t.Fatalf("select_workspace = %+v, want %s", selected, second.ID)
	}

	got := callTool[struct {
		Selected struct{ ID string }
	}](t, cs, "get_selected_workspace", nil)
	if got.Selected.ID != second.ID {
		t.Fatalf("get_selected_workspace = %+v, want %s", got, second.ID)
	}

	services := callTool[struct{ Services []struct{ Name string } }](t, cs, "list_services", nil)
	if len(services.Services) != 1 || services.Services[0].Name != "second-web" {
		t.Fatalf("scoped list_services = %+v, want only second-web", services)
	}
	if hits[0].Load() == 0 || hits[1].Load() == 0 {
		t.Fatalf("replica hits = [%d %d], want both replicas", hits[0].Load(), hits[1].Load())
	}
}
