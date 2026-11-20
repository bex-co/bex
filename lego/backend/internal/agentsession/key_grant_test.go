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

package agentsession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeKeyGrants is a store of machine rows: each bound key's grant, read
// uncached on every question.
type fakeKeyGrants struct {
	mu     sync.Mutex
	grants map[string]core.KeyGrant
	err    error
	reads  int
}

func (f *fakeKeyGrants) BoundAPIKeyGrant(_ context.Context, clientID string) (core.KeyGrant, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.err != nil {
		return core.KeyGrant{}, false, f.err
	}
	g, ok := f.grants[clientID]
	if !ok {
		return core.AllKeyGrant(), false, nil
	}
	return g, true, nil
}

func (f *fakeKeyGrants) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

func (f *fakeKeyGrants) set(clientID string, entries ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants[clientID] = core.KeyGrantFromEntries(entries)
}

func keyGrantVerb(t *testing.T, grants KeyGrantReader) *KeyGrantClient {
	t.Helper()
	srv := httptest.NewServer(&KeyGrantHandler{Secret: []byte("exec-secret"), Grants: grants})
	t.Cleanup(srv.Close)
	return &KeyGrantClient{URL: srv.URL, Secret: []byte("exec-secret")}
}

// The verb answers what bex-api's own gate decides on the key's next request:
// the grant read uncached on every question, so a narrowing is seen on the
// very next tick; a person is not a key and is always allowed.
func TestKeyGrantVerbAnswersTheKeysCurrentGrant(t *testing.T) {
	grants := &fakeKeyGrants{grants: map[string]core.KeyGrant{}}
	grants.set("key-1", "services:read_write")
	grants.grants["key-all"] = core.AllKeyGrant() // before the verb serves
	client := keyGrantVerb(t, grants)
	ctx := context.Background()

	for name, tc := range map[string]struct {
		subject, action string
		key, allowed    bool
	}{
		"covered action":      {"key-1", string(core.KeyServicesJobs), true, true},
		"uncovered action":    {"key-1", string(core.KeySandboxesExec), true, false},
		"a key with All":      {"key-all", string(core.KeyAgentsManage), true, true},
		"a person is not one": {"person", string(core.KeySandboxesExec), false, true},
	} {
		t.Run(name, func(t *testing.T) {
			key, allowed, err := client.Ask(ctx, tc.subject, tc.action)
			if err != nil || key != tc.key || allowed != tc.allowed {
				t.Fatalf("Ask(%s, %s) = (key %v, allowed %v, %v), want (%v, %v)", tc.subject, tc.action, key, allowed, err, tc.key, tc.allowed)
			}
		})
	}

	// Narrowing applies to the next question: nothing is cached.
	grants.set("key-1", "services:read")
	if _, allowed, err := client.Ask(ctx, "key-1", string(core.KeyServicesJobs)); err != nil || allowed {
		t.Fatalf("after narrowing to Services Read: allowed %v, %v; want refused", allowed, err)
	}
	reads := grants.readCount()
	if _, _, err := client.Ask(ctx, "key-1", string(core.KeyServicesJobs)); err != nil || grants.readCount() != reads+1 {
		t.Fatalf("a second question read the grant %d more times (%v), want once", grants.readCount()-reads, err)
	}
}

// Anything the verb cannot establish is an error, which the gateway refuses
// on: an unreadable grant, no store, a question it does not understand, and a
// request without the gateway's signature.
func TestKeyGrantVerbFailsClosed(t *testing.T) {
	ctx := context.Background()
	t.Run("unreadable grant", func(t *testing.T) {
		client := keyGrantVerb(t, &fakeKeyGrants{err: errors.New("postgres down")})
		if key, allowed, err := client.Ask(ctx, "key-1", string(core.KeyServicesJobs)); err == nil {
			t.Fatalf("Ask = (%v, %v, nil), want an error", key, allowed)
		}
	})
	t.Run("no store", func(t *testing.T) {
		client := keyGrantVerb(t, nil)
		if key, allowed, err := client.Ask(ctx, "key-1", string(core.KeyServicesJobs)); err == nil {
			t.Fatalf("Ask = (%v, %v, nil), want an error", key, allowed)
		}
	})
	grants := &fakeKeyGrants{grants: map[string]core.KeyGrant{}}
	client := keyGrantVerb(t, grants)
	for name, req := range map[string][2]string{
		"no subject":     {"", string(core.KeyServicesJobs)},
		"unknown action": {"key-1", "services.everything"},
		"no action":      {"key-1", ""},
		"a marker":       {"key-1", string(core.KeyAnyGrant)},
	} {
		t.Run(name, func(t *testing.T) {
			if key, allowed, err := client.Ask(ctx, req[0], req[1]); err == nil {
				t.Fatalf("Ask = (%v, %v, nil), want an error", key, allowed)
			}
		})
	}
	if n := grants.readCount(); n != 0 {
		t.Errorf("a refused question read the store %d times", n)
	}
	t.Run("wrong secret", func(t *testing.T) {
		wrong := &KeyGrantClient{URL: client.URL, Secret: []byte("not-the-secret")}
		if _, _, err := wrong.Ask(ctx, "key-1", string(core.KeyServicesJobs)); err == nil {
			t.Fatal("an unsigned question was answered")
		}
	})
}

// The client takes no answer for another question, and no answer that refuses
// a subject it says is not a key.
func TestKeyGrantClientRefusesAMismatchedAnswer(t *testing.T) {
	for name, out := range map[string]KeyGrantResponse{
		"another subject":     {Subject: "key-2", Action: string(core.KeyServicesJobs), Key: true, Allowed: true},
		"another action":      {Subject: "key-1", Action: string(core.KeySandboxesExec), Key: true, Allowed: true},
		"a refused non-key":   {Subject: "key-1", Action: string(core.KeyServicesJobs), Key: false, Allowed: false},
		"an empty answer":     {},
		"no action echoed":    {Subject: "key-1", Key: true, Allowed: true},
		"no subject echoed":   {Action: string(core.KeyServicesJobs), Key: true, Allowed: true},
		"refused, other key2": {Subject: "key-2", Action: string(core.KeyServicesJobs), Key: true},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(out)
			}))
			t.Cleanup(srv.Close)
			client := &KeyGrantClient{URL: srv.URL, Secret: []byte("exec-secret")}
			if key, allowed, err := client.Ask(context.Background(), "key-1", string(core.KeyServicesJobs)); err == nil {
				t.Fatalf("Ask took %+v as (%v, %v)", out, key, allowed)
			}
		})
	}
}
