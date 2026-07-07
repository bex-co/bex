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

package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeFGA fakes the two OpenFGA endpoints the checker uses.
func fakeFGA(t *testing.T, hits *atomic.Int32, allowedSubject string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fga-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/stores":
			_, _ = fmt.Fprint(w, `{"stores":[{"id":"store-1","name":"bex"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/stores/store-1/check":
			hits.Add(1)
			var in struct {
				TupleKey struct {
					User string `json:"user"`
				} `json:"tuple_key"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			_, _ = fmt.Fprintf(w, `{"allowed":%v}`, in.TupleKey.User == allowedSubject)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func deadServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

func TestOpenFGAChecker(t *testing.T) {
	var hits atomic.Int32
	fga := fakeFGA(t, &hits, "user:good")
	chk := NewOpenFGAChecker(fga.URL, "fga-key")
	ctx := context.Background()

	// Positive checks cache; the second identical check costs no upstream call.
	for i := range 2 {
		ok, err := chk.Check(ctx, "user:good", core.RelCanView, core.DefaultWorkspace)
		if err != nil || !ok {
			t.Fatalf("check %d: ok=%v err=%v", i, ok, err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream checks = %d, want 1 (positive cached)", got)
	}

	// Negatives are never cached.
	for range 2 {
		ok, err := chk.Check(ctx, "user:bad", core.RelCanView, core.DefaultWorkspace)
		if err != nil || ok {
			t.Fatalf("deny expected: ok=%v err=%v", ok, err)
		}
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("upstream checks = %d, want 3 (negatives uncached)", got)
	}

	// Wrong preshared key => error, not a silent deny-or-allow.
	if _, err := NewOpenFGAChecker(fga.URL, "wrong-key").Check(ctx, "user:good", core.RelCanView, core.DefaultWorkspace); err == nil {
		t.Fatal("bad key should error")
	}
	// Unreachable OpenFGA => error (fail closed at the caller).
	if _, err := NewOpenFGAChecker(deadServer(t), "fga-key").Check(ctx, "user:good", core.RelCanView, core.DefaultWorkspace); err == nil {
		t.Fatal("dead server should error")
	}
}

func TestGrantWorkspaceAdmin(t *testing.T) {
	var wrote atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/stores":
			_, _ = fmt.Fprint(w, `{"stores":[{"id":"store-1","name":"bex"}]}`)
		case r.URL.Path == "/stores/store-1/write":
			var in writeRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			if len(in.Writes.TupleKeys) == 1 && in.Writes.TupleKeys[0].Relation == "admin" &&
				in.Writes.TupleKeys[0].Object == "workspace:tea-1" {
				wrote.Add(1)
			}
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := NewOpenFGAChecker(srv.URL, "fga-key").(interface {
		GrantWorkspaceAdmin(context.Context, string, string) error
	})
	if err := g.GrantWorkspaceAdmin(context.Background(), "tea-1", "user:owner"); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if wrote.Load() != 1 {
		t.Errorf("expected one membership tuple write, got %d", wrote.Load())
	}
}
