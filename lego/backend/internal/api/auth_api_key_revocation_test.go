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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestAPIKeyRevocationRejectsWarmAndColdTokensAcrossReplicas(t *testing.T) {
	hydra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := "departing-key"
		if r.FormValue("token") == "remaining-token" {
			client = "remaining-key"
		}
		// Model a stale positive from Hydra, even after deletion of its client.
		_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "sub": client, "client_id": client})
	}))
	defer hydra.Close()
	shared := &memoryRevocations{}
	var gates []*oryAuth
	var handlers []http.Handler
	for range 2 {
		gate := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
		gate.revocations = shared
		gates = append(gates, gate)
		handler := gate.middleware(echoIdentity)
		handlers = append(handlers, handler)
		if got := do(t, handler, http.MethodGet, "/probe", "warmed-token", "").Code; got != http.StatusOK {
			t.Fatalf("warm: %d", got)
		}
		if got := do(t, handler, http.MethodGet, "/probe", "remaining-token", "").Code; got != http.StatusOK {
			t.Fatalf("warm remaining: %d", got)
		}
	}
	if err := shared.BumpOAuthRevocation(context.Background(), "departing-key", "departing-key"); err != nil {
		t.Fatal(err)
	}
	for i, handler := range handlers {
		for _, token := range []string{"warmed-token", "uncached-token"} {
			if got := do(t, handler, http.MethodGet, "/probe", token, "").Code; got != http.StatusUnauthorized {
				t.Errorf("replica %d token %s: %d, want 401", i, token, got)
			}
		}
		if got := do(t, handler, http.MethodGet, "/probe", "remaining-token", "").Code; got != http.StatusOK {
			t.Errorf("replica %d remaining key: %d", i, got)
		}
	}
	// A stale response cached after the marker is still dead. Human OAuth
	// revocations only invalidate older cache entries; machine tombstones differ.
	gates[0].cache.Put("late-positive", cachedIdentity{
		Identity: core.Identity{Method: "oauth2", Subject: "departing-key", ClientID: "departing-key"},
		CachedAt: time.Now().Add(time.Hour),
	}, time.Now().Add(time.Hour))
	if got := do(t, handlers[0], http.MethodGet, "/probe", "late-positive", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("late positive: %d, want401", got)
	}
}

func TestAPIKeyRevocationRejectsInflightIntrospection(t *testing.T) {
	hydra, started, release, _ := blockingHydra(t, "departing-key", "departing-key")
	shared := &memoryRevocations{}
	gate := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
	gate.revocations = shared
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		result <- do(t, gate.middleware(echoIdentity), http.MethodGet, "/probe", "in-flight", "")
	}()
	<-started
	err := shared.BumpOAuthRevocation(context.Background(), "departing-key", "departing-key")
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if rec := <-result; rec.Code != http.StatusUnauthorized {
		t.Fatalf("in-flight credential returned %d, want401", rec.Code)
	}
	if _, ok := gate.cache.Get("in-flight"); ok {
		t.Fatal("revoked key repopulated cache")
	}
}
