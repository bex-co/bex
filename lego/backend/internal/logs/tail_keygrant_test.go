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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A scoped API key's live log tail lasts only while its grant covers "View
// service logs": the tail's watchdog re-reads the grant on every tick, so a
// key whose permission an Admin removes loses its open tail at the next one,
// as a caller whose role is removed does.

// narrowingGrant answers every permission until narrow is set, then the
// grant with every permission cleared.
type narrowingGrant struct {
	mu     sync.Mutex
	narrow bool
	calls  int
}

func (g *narrowingGrant) APIKeyGrant(context.Context, string) (core.KeyGrant, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.narrow {
		return core.KeyGrant{Grants: []core.KeyGrantEntry{}}, nil
	}
	return core.AllKeyGrant(), nil
}

func (g *narrowingGrant) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func (g *narrowingGrant) clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.narrow = true
}

func TestKeyTailEndsWhenItsGrantNoLongerCoversLogs(t *testing.T) {
	checker := &freshGateChecker{cached: true, fresh: true}
	svc := revalidationService(checker, newScriptedStream(nil).follow, sampleApp("nightly"))
	grants := &narrowingGrant{}
	svc.KeyGrants = grants
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	key := core.Identity{Subject: "key-client", ClientID: "key-client", Method: core.MethodOAuth2}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(core.WithIdentity(r.Context(), key)))
	}))
	defer srv.Close()

	resp := subscribeSSE(t, srv, "nightly")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("subscribe status = %d, want 200", resp.StatusCode)
	}
	done := streamDone(resp.Body)

	// Two healthy ticks: each read the key's grant before the role check.
	waitForFreshCalls(t, checker, 2)
	if grants.callCount() == 0 {
		t.Fatal("the tail's watchdog never read the key's grant")
	}
	select {
	case <-done:
		t.Fatal("the tail ended while the key's grant still covered logs")
	default:
	}

	grants.clear()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the tail outlived the key's grant losing View service logs")
	}
	waitForSlotRelease(t, svc)
}
