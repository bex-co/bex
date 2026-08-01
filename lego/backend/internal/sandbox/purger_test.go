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
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
)

// lookupKeys is a fake PurgeKeyLookup: it returns a stored key per workspace and
// can be told a workspace has none (nothing ever created).
type lookupKeys struct {
	keys map[string]string
	err  error
}

func (l lookupKeys) SandboxKeyLookup(_ context.Context, workspaceID string) (string, bool, error) {
	if l.err != nil {
		return "", false, l.err
	}
	k, ok := l.keys[workspaceID]
	return k, ok, nil
}

// TestPurgeWorkspaceStopsEverySandbox proves the purger enumerates the
// workspace's sandboxes (scoped by its tenant key) and terminates each.
func TestPurgeWorkspaceStopsEverySandbox(t *testing.T) {
	var (
		mu         sync.Mutex
		terminated []string
		sawKey     string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		sawKey = r.Header.Get(tenantKeyHeader)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			_, _ = w.Write([]byte(`[{"id":"sb-1","status":{"state":"Running"}},{"id":"sb-2","status":{"state":"Paused"}}]`))
		case r.Method == http.MethodDelete:
			terminated = append(terminated, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &WorkspacePurger{
		Client: NewClient(srv.URL),
		Keys:   lookupKeys{keys: map[string]string{"tea-a": "osk-a"}},
	}
	if err := p.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	sort.Strings(terminated)
	want := []string{"/sandboxes/sb-1", "/sandboxes/sb-2"}
	if len(terminated) != 2 || terminated[0] != want[0] || terminated[1] != want[1] {
		t.Fatalf("terminated = %v, want %v", terminated, want)
	}
	if sawKey != "osk-a" {
		t.Errorf("tenant key header = %q, want osk-a (list/terminate must be workspace-scoped)", sawKey)
	}
}

// TestPurgeWorkspaceNoKeyIsNoop proves a workspace that never created a sandbox
// (no key was ever minted) makes no OpenSandbox call at all.
func TestPurgeWorkspaceNoKeyIsNoop(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()

	p := &WorkspacePurger{Client: NewClient(srv.URL), Keys: lookupKeys{keys: map[string]string{}}}
	if err := p.PurgeWorkspace(context.Background(), "tea-empty"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if called {
		t.Error("purger contacted OpenSandbox for a workspace with no tenant key")
	}
}

// TestPurgeWorkspaceToleratesAlreadyGone proves the purger is idempotent: a
// sandbox that 404s on terminate (already reaped by the namespace prune or a
// prior retry) does not fail the delete.
func TestPurgeWorkspaceToleratesAlreadyGone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"id":"sb-gone","status":{"state":"Running"}}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound) // already gone
	}))
	defer srv.Close()

	p := &WorkspacePurger{Client: NewClient(srv.URL), Keys: lookupKeys{keys: map[string]string{"tea-a": "osk-a"}}}
	if err := p.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("purge must tolerate an already-gone sandbox: %v", err)
	}
}

// TestPurgeWorkspaceSurfacesTransportError proves a transport failure propagates
// (so workspaces.Delete leaves the row intact and the delete is retryable).
func TestPurgeWorkspaceSurfacesTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := &WorkspacePurger{Client: NewClient(srv.URL), Keys: lookupKeys{keys: map[string]string{"tea-a": "osk-a"}}}
	if err := p.PurgeWorkspace(context.Background(), "tea-a"); err == nil {
		t.Fatal("want the list error surfaced")
	}
}

// TestPurgeWorkspaceDisabledIsNoop proves a nil client or key lookup (feature
// off) is a safe no-op.
func TestPurgeWorkspaceDisabledIsNoop(t *testing.T) {
	if err := (&WorkspacePurger{}).PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("nil-client purge: %v", err)
	}
	if err := (&WorkspacePurger{Client: NewClient("http://unused")}).PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("nil-keys purge: %v", err)
	}
}

// TestPurgeWorkspacePropagatesLookupError proves a key-store failure is not
// silently swallowed.
func TestPurgeWorkspacePropagatesLookupError(t *testing.T) {
	p := &WorkspacePurger{Client: NewClient("http://unused"), Keys: lookupKeys{err: errors.New("db down")}}
	if err := p.PurgeWorkspace(context.Background(), "tea-a"); err == nil {
		t.Fatal("want the key-lookup error surfaced")
	}
}
