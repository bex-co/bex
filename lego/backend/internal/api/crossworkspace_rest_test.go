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
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// serveMuxPatterns walks a populated http.ServeMux's routing tree and returns
// every registered pattern string (e.g. "GET /v1/services/{id}"). Go's ServeMux
// exposes no public route enumeration, so this reads the unexported tree via
// reflection — the standard technique for asserting a stdlib mux's route set.
// The callers floor-guard the count (a Go release that reshapes ServeMux would
// return zero patterns and fail loudly rather than pass vacuously).
//
// reflect's typed getters (String) read unexported fields; only Interface()/Set
// are blocked, and the tree is reached entirely through addressable Values.
func serveMuxPatterns(mux *http.ServeMux) []string {
	root := reflect.ValueOf(mux).Elem().FieldByName("tree") // routingNode
	set := map[string]bool{}
	var walk func(node reflect.Value)
	walk = func(node reflect.Value) {
		if pat := node.FieldByName("pattern"); !pat.IsNil() {
			set[pat.Elem().FieldByName("str").String()] = true
		}
		// children mapping[string, *routingNode]: either a small slice `s` or a
		// map `m` (net/http/mapping.go) — walk whichever is populated.
		children := node.FieldByName("children")
		if s := children.FieldByName("s"); s.Kind() == reflect.Slice {
			for i := 0; i < s.Len(); i++ {
				if v := s.Index(i).FieldByName("value"); !v.IsNil() {
					walk(v.Elem())
				}
			}
		}
		if m := children.FieldByName("m"); m.Kind() == reflect.Map {
			for _, k := range m.MapKeys() {
				if v := m.MapIndex(k); !v.IsNil() {
					walk(v.Elem())
				}
			}
		}
		for _, name := range []string{"multiChild", "emptyChild"} {
			if c := node.FieldByName(name); !c.IsNil() {
				walk(c.Elem())
			}
		}
	}
	walk(root)
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// splitPattern splits a "METHOD /path" pattern into its parts; a method-less
// pattern (bare path) yields an empty method (matches any method).
func splitPattern(pattern string) (method, path string) {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		return pattern[:i], pattern[i+1:]
	}
	return "", pattern
}

// fillWildcards replaces every {…} path segment with the tea-b resource name so
// the synthesized request targets the workspace-B fixture; {$} (the end-of-path
// anchor) is dropped. A trailing {id...} multi-wildcard is filled the same way.
func fillWildcards(path, name string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		switch {
		case s == "{$}":
			segs[i] = ""
		case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
			segs[i] = name
		}
	}
	return strings.TrimRight(strings.Join(segs, "/"), "/")
}

// hasPathWildcard reports whether a path addresses a specific resource/workspace
// by a path segment ({id}, {workspaceId}, {ownerId}, …) — the routes the deny
// matrix drives against the tea-b fixture. A route WITHOUT one is a collection,
// create, or query-param-scoped verb that acts in the caller's OWN workspace
// (callerScopedRoutes below); its cross-workspace path (ownerId/body) is covered
// by the t001 verb sweep and the t004 real-FGA E2E, not this route matrix.
func hasPathWildcard(path string) bool {
	for _, s := range strings.Split(path, "/") {
		if strings.HasPrefix(s, "{") && s != "{$}" {
			return true
		}
	}
	return false
}

// publicRoutes mount their own credential (or deliberately none) and carry no
// workspace-member semantics — separately audit-verified 2026-07-30 (w7/m54) and
// documented in internal/api/CLAUDE.md's always-public inventory. They are
// enumerated for completeness but not asserted-denied: a member check is not the
// gate that protects them.
var publicRoutes = map[string]bool{
	"GET /v1/git/callback": true, // HMAC-signed OAuth state, not a bearer/session
}

// callerScopedRoutes are the enumerated routes the deny matrix does NOT assert,
// each justified: none addresses a specific resource by a path segment, so none
// can read or mutate another workspace's resource by URL. They act on the
// caller's own resolved workspace (a collection list, a create, a whoami) or read
// a resource named in the query string / body — paths covered by the t001 verb
// sweep (List/Create/metrics/logs verbs) and the t004 real-FGA E2E. A NEW
// wildcard-free route absent from this set fails the completeness guard, forcing
// a classification decision rather than silent omission.
var callerScopedRoutes = map[string]bool{
	// Collection lists — scoped to the caller's own workspace (or identity).
	"GET /v1/services":                                true,
	"GET /v1/postgres":                                true,
	"GET /v1/key-value":                               true,
	"GET /v1/env-groups":                              true,
	"GET /v1/environments":                            true,
	"GET /v1/projects":                                true,
	"GET /v1/registrycredentials":                     true,
	"GET /v1/ssh-keys":                                true,
	"GET /v1/api-keys":                                true,
	"GET /v1/webhooks":                                true,
	"GET /v1/webhooks/event-types":                    true,
	"GET /v1/sandboxes":                               true,
	"GET /v1/agent-sessions":                          true,
	// Caller/workspace-scoped readiness projection (w11/m6 t001): scoped by
	// ownerId like the list route, addresses no session by path; its own
	// Authorize denies non-member callers (TestCapabilitiesProjection).
	"GET /v1/agent-sessions/capabilities": true,
	// The caller's OWN effective permissions (w9/m84): scoped by the ownerId
	// query param, addresses no resource by path; its own can_view gate denies
	// an ownerId naming a workspace the caller isn't in.
	"GET /v1/viewer/capabilities": true,
	"GET /v1/owners":                                  true,
	"GET /v1/blueprints":                              true,
	"GET /v1/usage":                                   true,
	"GET /v1/users":                                   true,
	"GET /v1/repos":                                   true,
	"GET /v1/notification-settings":                   true,
	"GET /v1/notification-settings/push":              true,
	"GET /v1/notification-settings/push/availability": true, // caller-scoped feature capability; no resource id
	"GET /v1/notifications":                           true, // caller's own tenant + subject are derived from auth context
	"GET /v1/notification-device-subscriptions":       true, // caller's own workspace + subject; no owner path arg
	"GET /v1/notification-webpush-subscriptions":      true, // caller's own browser subscriptions; no owner path arg
	"GET /v1/git/connection":                          true,
	// The caller's own workspace git connection(s) — no resource in the path
	// (ADR075). DELETE /v1/git/connections/{installationId} carries a GitHub
	// installation id (not a bex resource id) so it rides the wildcard deny matrix.
	"DELETE /v1/git/connection": true,
	"GET /v1/git/connections":   true,
	// Creates — the new resource's workspace comes from the request context
	// (ownerId / caller default), not a path; a create naming a non-member
	// workspace is refused in the t004 E2E.
	"POST /v1/services":                            true,
	"POST /v1/postgres":                            true,
	"POST /v1/key-value":                           true,
	"POST /v1/env-groups":                          true,
	"POST /v1/environments":                        true,
	"POST /v1/projects":                            true,
	"POST /v1/registrycredentials":                 true,
	"POST /v1/api-keys":                            true,
	"POST /v1/webhooks":                            true,
	"POST /v1/ssh-keys":                            true,
	"POST /v1/sandboxes":                           true,
	"POST /v1/agent-sessions":                      true,
	"POST /v1/git/connect":                         true,
	// The ADR075 §3a claim start — same ownerId/body scoping as connect; its
	// cross-workspace path is the verb's own can_manage gate.
	"POST /v1/git/claim": true,
	"POST /v1/invites/accept":                      true, // redeems by the CALLER's own identity
	"POST /v1/notification-device-subscriptions":   true, // registers only the caller's own device capability
	"POST /v1/notification-webpush-subscriptions":  true, // registers only the caller's own browser subscription
	"POST /v1/blueprints":                          true, // creates a new git-connected blueprint; workspace from request context (w2/m62)
	"POST /v1/blueprints/validate":                 true, // validates a posted manifest, no resource
	"POST /v1/blueprints/preview":                  true, // dry-run fetch+validate; workspace from request context, no resource
	"POST /v1/blueprints/generate":                 true, // export manifest; each body-named resource is authorized through AuthorizeApp/Database/KeyValue against its own workspace (w8/m22)
	"PATCH /v1/notification-settings":              true, // the caller's own notification prefs
	"PATCH /v1/notification-settings/push":         true, // caller's own push policy; workspace and subject come from auth context
	"DELETE /v1/notification-device-subscriptions": true, // revokes only the caller's own devices
	"POST /v1/blueprints/deploy":                   true, // applies a posted Blueprint in the caller-scoped ownerId workspace
	// Logs + metrics — the resource is named in the ?resource= query string, not
	// the path; the QueryLogs/GetMetrics verbs (t001) authorize it via GetApp.
	"GET /v1/logs":                    true,
	"GET /v1/logs/subscribe":          true,
	"GET /v1/logs/values":             true,
	"GET /v1/metrics/cpu":             true,
	"GET /v1/metrics/memory":          true,
	"GET /v1/metrics/disk":            true,
	"GET /v1/metrics/bandwidth":       true,
	"GET /v1/metrics/instance-count":  true,
	"GET /v1/metrics/cpu-target":      true,
	"GET /v1/metrics/memory-target":   true,
	"GET /v1/metrics/http-requests":   true,
	"GET /v1/metrics/http-latency":    true,
	"GET /v1/metrics/db-connections":  true,
	"GET /v1/metrics/disk-capacity":   true,
	"GET /v1/metrics/kv-connections":  true,
	"GET /v1/metrics/kv-memory":       true,
	"GET /v1/metrics/replication-lag": true,
}

// restMatrixFixture builds the mux from the guarded canonical service inventory
// (sweepableServices — TestSweepCoversEveryWiredService keeps it == features()),
// wired to the shared negative-authorization fixture (crossWorkspaceBase): an
// allow-own/deny-other checker, a resolver making the caller a member of tea-a
// only, and every named resource (App/Database/KeyValue "target") owned by tea-b.
func restMatrixFixture() *http.ServeMux {
	mux := http.NewServeMux()
	for _, svc := range sweepableServices(crossWorkspaceBase()) {
		if r, ok := svc.(restRegistrar); ok {
			r.RegisterREST(mux)
		}
	}
	return mux
}

// driveNonMember serves one request as the workspace-A caller "attacker",
// recovering a handler panic on nil feature deps into a synthetic non-2xx code
// (a panic is not a leak) so the sweep's one question — did a non-member get a
// 2xx against a workspace-B resource? — stays the only thing that fails it.
func driveNonMember(mux http.Handler, method, path, body string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			code = 599
		}
	}()
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "attacker", Method: "session"})
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code
}

// TestCrossWorkspaceRESTMatrix is t002: every registered REST route that
// addresses a resource by a path segment must refuse a non-member caller
// targeting a workspace-B resource (never 2xx), and every wildcard-free route
// must be a documented caller-scoped or public route. A newly registered route
// joins the deny matrix (if it takes a path id) or fails the completeness guard
// (if it does not and isn't justified) automatically — no hand-maintained route
// list, per the m30 precedent.
func TestCrossWorkspaceRESTMatrix(t *testing.T) {
	// One mux for both enumeration and driving: every resource route denies
	// before its verb touches state, so the tea-b fixture is never mutated across
	// the sweep and need not be rebuilt per route (a leak that DID mutate would
	// already have failed the assertion below).
	mux := restMatrixFixture()
	patterns := serveMuxPatterns(mux)
	if len(patterns) < 150 {
		t.Fatalf("route enumeration found only %d patterns — the ServeMux tree walk broke (Go release?)", len(patterns))
	}
	asserted := 0
	for _, p := range patterns {
		method, path := splitPattern(p)
		switch {
		case publicRoutes[p]:
			continue
		case !hasPathWildcard(path):
			if !callerScopedRoutes[p] {
				t.Errorf("route %q addresses no resource by path and is not a documented caller-scoped/public route — "+
					"classify it (add to callerScopedRoutes with a justification, or give it a resource path segment)", p)
			}
			continue
		}
		// Resource-targeted: drive it at the tea-b fixture as a non-member.
		reqPath := fillWildcards(path, crossWorkspaceTarget)
		body := ""
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
			body = "{}"
		}
		code := driveNonMember(mux, method, reqPath, body)
		if code >= 200 && code < 300 {
			t.Errorf("%s -> %s: returned %d for a non-member targeting a workspace-B resource — want a refusal (4xx), never 2xx", p, reqPath, code)
		}
		asserted++
	}
	// Guard the guard: if a refactor drops the wildcard routes, the matrix would
	// pass while asserting almost nothing.
	if asserted < 100 {
		t.Fatalf("only %d resource-targeted routes asserted — the matrix has gone vacuous", asserted)
	}
	// Completeness: a callerScopedRoutes/publicRoutes entry naming a route no
	// longer registered is stale cover and must be removed.
	registered := map[string]bool{}
	for _, p := range patterns {
		registered[p] = true
	}
	assertNoStaleExclusions(t, "callerScopedRoutes", callerScopedRoutes, registered)
	assertNoStaleExclusions(t, "publicRoutes", publicRoutes, registered)
	t.Logf("asserted non-member deny on %d resource-targeted routes; %d caller-scoped + %d public excused", asserted, len(callerScopedRoutes), len(publicRoutes))
}
