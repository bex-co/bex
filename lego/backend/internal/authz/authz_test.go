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

func TestGrantAgentSessionWorkspaceWritesFirstClassParentTuple(t *testing.T) {
	var got tupleKey
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stores":
			_, _ = fmt.Fprint(w, `{"stores":[{"id":"store-1","name":"bex"}]}`)
		case "/stores/store-1/write":
			var in writeRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.Writes != nil && len(in.Writes.TupleKeys) == 1 {
				got = in.Writes.TupleKeys[0]
			}
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	writer := NewOpenFGAChecker(srv.URL, "").(interface {
		GrantAgentSessionWorkspace(context.Context, string, string) error
	})
	if err := writer.GrantAgentSessionWorkspace(context.Background(), "ags-1", "tea-a"); err != nil {
		t.Fatal(err)
	}
	want := tupleKey{User: "workspace:tea-a", Relation: "workspace", Object: "agent_session:ags-1"}
	if got != want {
		t.Fatalf("tuple = %+v, want %+v", got, want)
	}
}

// TestRevokeWorkspaceMemberEvictsCachedPositives pins codex-security round-6
// #16: revoking a member must evict the subject's cached positive decisions in
// the same replica — every key for that subject (workspace and derived
// resource objects alike), and only that subject's.
func TestRevokeWorkspaceMemberEvictsCachedPositives(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/stores":
			_, _ = fmt.Fprint(w, `{"stores":[{"id":"store-1","name":"bex"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/stores/store-1/check":
			hits.Add(1)
			_, _ = fmt.Fprint(w, `{"allowed":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/stores/store-1/write":
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	chk := NewOpenFGAChecker(srv.URL, "")
	revoker := chk.(interface {
		RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error
	})
	ctx := context.Background()

	// Warm three positives: the revoked subject's workspace + a derived
	// resource decision, and an unrelated subject's decision.
	for _, c := range [][2]string{
		{"user:mallory", "workspace:tea-a"},
		{"user:mallory", "service:srv-1"},
		{"user:alice", "workspace:tea-a"},
	} {
		if ok, err := chk.Check(ctx, c[0], core.RelCanManage, c[1]); err != nil || !ok {
			t.Fatalf("warm %v: ok=%v err=%v", c, ok, err)
		}
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("warm upstream checks = %d, want 3", got)
	}

	if err := revoker.RevokeWorkspaceMember(ctx, "tea-a", "user:mallory", "admin"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Both of mallory's decisions must re-consult upstream; alice stays cached.
	if _, err := chk.Check(ctx, "user:mallory", core.RelCanManage, "workspace:tea-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := chk.Check(ctx, "user:mallory", core.RelCanManage, "service:srv-1"); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 5 {
		t.Fatalf("post-revoke upstream checks = %d, want 5 (both of the subject's entries evicted)", got)
	}
	if _, err := chk.Check(ctx, "user:alice", core.RelCanManage, "workspace:tea-a"); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 5 {
		t.Fatalf("unrelated subject's cache entry was evicted (checks = %d, want 5)", got)
	}
}
