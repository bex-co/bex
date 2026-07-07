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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeChecker records what was asked and answers uniformly.
type fakeChecker struct {
	allow bool
	err   error

	lastSubject  string
	lastRelation string
	lastObject   string
}

func (f *fakeChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	f.lastSubject, f.lastRelation, f.lastObject = subject, relation, object
	return f.allow, f.err
}

func authzServer(t *testing.T, chk Checker) http.Handler {
	t.Helper()
	srv := &Server{
		Core: &Core{
			Client:    fakeAppClient(t, sampleApp("web")),
			Namespace: "default",
			APIKeys:   newFakeKeyStore(),
			Authz:     chk,
		},
		HydraAdminURL: fakeHydraURL(t), // authenticates testToken as client-1
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h
}

// TestAuthzEnforcement drives the full handler (auth gate + Core guard) so the
// subject seen by the checker is the one the auth middleware resolved.
func TestAuthzEnforcement(t *testing.T) {
	t.Run("deny is 403 on every surface", func(t *testing.T) {
		chk := &fakeChecker{allow: false}
		h := authzServer(t, chk)

		if code := do(t, h, "GET", "/v1/services", testToken, "").Code; code != 403 {
			t.Fatalf("REST read: got %d, want 403", code)
		}
		if chk.lastSubject != "user:client-1" || chk.lastRelation != relCanView || chk.lastObject != defaultWorkspace {
			t.Fatalf("check asked for %s/%s/%s", chk.lastSubject, chk.lastRelation, chk.lastObject)
		}
		if code := do(t, h, "POST", "/v1/services/web/suspend", testToken, "").Code; code != 403 {
			t.Fatalf("REST manage: got %d, want 403", code)
		}
		if chk.lastRelation != relCanOperate {
			t.Fatalf("suspend checked %s, want can_operate", chk.lastRelation)
		}
		if code := do(t, h, "POST", "/v1/api-keys", testToken, `{"name":"x"}`).Code; code != 403 {
			t.Fatalf("REST mint: got %d, want 403", code)
		}
		if chk.lastRelation != relCanManageKeys {
			t.Fatalf("mint checked %s, want can_manage_keys", chk.lastRelation)
		}

		// GraphQL surfaces the same denial as an errors entry (HTTP stays 200).
		w := do(t, h, "POST", "/graphql", testToken, `{"query":"{ services { id } }"}`)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "forbidden") {
			t.Fatalf("graphql deny: code %d body %s", w.Code, w.Body.String())
		}
	})

	t.Run("allow passes through", func(t *testing.T) {
		h := authzServer(t, &fakeChecker{allow: true})
		if code := do(t, h, "GET", "/v1/services", testToken, "").Code; code != 200 {
			t.Fatalf("allowed read: got %d, want 200", code)
		}
		if code := do(t, h, "POST", "/v1/api-keys", testToken, `{"name":"x"}`).Code; code != 201 {
			t.Fatalf("allowed mint: got %d, want 201", code)
		}
	})

	t.Run("checker error fails closed with 503", func(t *testing.T) {
		h := authzServer(t, &fakeChecker{err: errors.New("fga down")})
		if code := do(t, h, "GET", "/v1/services", testToken, "").Code; code != 503 {
			t.Fatalf("checker outage: got %d, want 503", code)
		}
	})

	t.Run("wired checker with no identity denies", func(t *testing.T) {
		core := &Core{Client: fakeAppClient(t), Namespace: "default", Authz: &fakeChecker{allow: true}}
		if _, err := core.List(context.Background()); !errors.Is(err, ErrForbidden) {
			t.Fatalf("no identity in ctx: got %v, want ErrForbidden", err)
		}
	})

}

// TestAuthzGuardsEveryCoreVerb sweeps every exported Core method by reflection
// with a deny-all checker: each must return ErrForbidden before doing anything
// else. A new Core verb that forgets its c.authorize guard fails this sweep
// automatically — the CLAUDE.md rule, enforced.
func TestAuthzGuardsEveryCoreVerb(t *testing.T) {
	core := &Core{Client: fakeAppClient(t), Namespace: "default", APIKeys: newFakeKeyStore(), Authz: &fakeChecker{allow: false}}
	ctx := context.WithValue(context.Background(), ctxKey{}, Identity{Subject: "client-1", Method: "oauth2"})

	cv := reflect.ValueOf(core)
	ct := cv.Type()
	swept := 0
	for i := 0; i < ct.NumMethod(); i++ {
		m := ct.Method(i)
		mt := m.Func.Type()
		// A verb: exported method whose first real arg is a context and whose
		// last return is an error.
		if mt.NumIn() < 2 || mt.In(1) != reflect.TypeFor[context.Context]() {
			continue
		}
		if mt.NumOut() == 0 || mt.Out(mt.NumOut()-1) != reflect.TypeFor[error]() {
			continue
		}
		swept++
		args := []reflect.Value{cv, reflect.ValueOf(ctx)}
		for a := 2; a < mt.NumIn(); a++ {
			at := mt.In(a)
			if at.Kind() == reflect.Func { // e.g. FollowLogs' emit callback
				args = append(args, reflect.MakeFunc(at, func(_ []reflect.Value) []reflect.Value {
					outs := make([]reflect.Value, at.NumOut())
					for o := range outs {
						outs[o] = reflect.Zero(at.Out(o))
					}
					return outs
				}))
				continue
			}
			args = append(args, reflect.Zero(at))
		}
		out := m.Func.Call(args)
		errv := out[len(out)-1].Interface()
		err, _ := errv.(error)
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: unguarded — returned %v, want ErrForbidden (add c.authorize as its first statement)", m.Name, err)
		}
	}
	if swept < 17 {
		t.Fatalf("sweep found only %d verbs — reflection filter broke?", swept)
	}
}

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

func TestOpenFGAChecker(t *testing.T) {
	var hits atomic.Int32
	fga := fakeFGA(t, &hits, "user:good")
	chk := NewOpenFGAChecker(fga.URL, "fga-key")
	ctx := context.Background()

	// Positive checks cache; the second identical check costs no upstream call.
	for i := range 2 {
		ok, err := chk.Check(ctx, "user:good", relCanView, defaultWorkspace)
		if err != nil || !ok {
			t.Fatalf("check %d: ok=%v err=%v", i, ok, err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream checks = %d, want 1 (positive cached)", got)
	}

	// Negatives are never cached.
	for range 2 {
		ok, err := chk.Check(ctx, "user:bad", relCanView, defaultWorkspace)
		if err != nil || ok {
			t.Fatalf("deny expected: ok=%v err=%v", ok, err)
		}
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("upstream checks = %d, want 3 (negatives uncached)", got)
	}

	// Wrong preshared key => error, not a silent deny-or-allow.
	bad := NewOpenFGAChecker(fga.URL, "wrong-key")
	if _, err := bad.Check(ctx, "user:good", relCanView, defaultWorkspace); err == nil {
		t.Fatal("bad key should error")
	}

	// Unreachable OpenFGA => error (fail closed at the caller).
	dead := NewOpenFGAChecker(deadServer(t), "fga-key")
	if _, err := dead.Check(ctx, "user:good", relCanView, defaultWorkspace); err == nil {
		t.Fatal("dead server should error")
	}
}
