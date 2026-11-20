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

package gatewaytest

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// KeyGrants plays bex API keys' machine rows behind bex-api's real grant verb
// (agentsession.KeyGrantHandler, w9/125). Client is the gateway's real
// agentsession.KeyGrantClient pointed at it, so a transport test crosses the
// same signed hop the deployed gateway does. Every question reads the grant
// uncached, as the store does. Safe to change while server goroutines run.
type KeyGrants struct {
	mu     sync.Mutex
	grants map[string]core.KeyGrant
	err    error
	reads  int
}

// Set gives clientID, a bound key, the grant of the given "permission:level"
// entries (none is Render's Clear all). It returns how many questions were
// answered before the change, so a test can count the ones after it.
func (k *KeyGrants) Set(clientID string, entries ...string) int {
	return k.put(clientID, core.KeyGrantFromEntries(entries))
}

// SetAll gives clientID, a bound key, All.
func (k *KeyGrants) SetAll(clientID string) int { return k.put(clientID, core.AllKeyGrant()) }

func (k *KeyGrants) put(clientID string, g core.KeyGrant) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.grants == nil {
		k.grants = map[string]core.KeyGrant{}
	}
	k.grants[clientID] = g
	return k.reads
}

// Fail makes every later question fail like an unreadable store (nil heals);
// it returns the questions answered before.
func (k *KeyGrants) Fail(err error) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.err = err
	return k.reads
}

// Reads is how many questions the verb has read a grant for.
func (k *KeyGrants) Reads() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reads
}

// WaitReads waits until the verb has answered n questions in all.
func (k *KeyGrants) WaitReads(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for k.Reads() < n {
		if time.Now().After(deadline) {
			t.Fatalf("the grant verb answered %d questions, want %d", k.Reads(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// BoundAPIKeyGrant implements agentsession.KeyGrantReader: a subject never
// Set is not a key (a person), and has All.
func (k *KeyGrants) BoundAPIKeyGrant(_ context.Context, clientID string) (core.KeyGrant, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.reads++
	if k.err != nil {
		return core.KeyGrant{}, false, k.err
	}
	g, bound := k.grants[clientID]
	if !bound {
		return core.AllKeyGrant(), false, nil
	}
	return g, true, nil
}

// Client serves the verb over these grants for the test's lifetime and
// returns the gateway's client for it.
func (k *KeyGrants) Client(t *testing.T) *agentsession.KeyGrantClient {
	t.Helper()
	secret := []byte("key-grant-verb-secret")
	srv := httptest.NewServer(&agentsession.KeyGrantHandler{Secret: secret, Grants: k})
	t.Cleanup(srv.Close)
	return &agentsession.KeyGrantClient{URL: srv.URL, Secret: secret}
}
