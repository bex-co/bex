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

package core

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// authorizedatastore_test.go holds AuthorizeDatabase and AuthorizeKeyValue to
// ONE contract. They were byte-identical copies until their shared body was folded
// into authorizeDatastore; these tests are what makes that fold —
// and any future edit to it — safe, because every line of that body is a
// security decision that fails SILENTLY when it drifts:
//
//   - authorizing a MISS against the resource's workspace instead of the
//     caller's turns the seam into an existence oracle,
//   - authorizing a HIT against the caller's workspace instead of the
//     resource's reintroduces the w6/m17 two-workspace intersection bug,
//   - losing the audit target keeps the verb's name but drops it out of the
//     events feed, which filters on verb AND target AND outcome.
//
// Each property runs over BOTH siblings from one table, so a change that fixes
// or breaks one has to do the same to the other.

// datastoreSeam is one of the two datastore authorize entry points, named so a
// failure says which sibling diverged. call returns the resolved namespace so a
// success can be asserted without naming the concrete CR type.
type datastoreSeam struct {
	name   string
	id     string
	target string
	seed   func(ns, tenant string) client.Object
	call   func(*Base, context.Context, string, string) (string, error)
}

func datastoreSeams() []datastoreSeam {
	return []datastoreSeam{
		{
			name: "AuthorizeDatabase", id: "dpg-x", target: DatabaseTarget("dpg-x"),
			seed: func(ns, tenant string) client.Object { return tenantDatabase(ns, "dpg-x", tenant) },
			call: func(b *Base, ctx context.Context, relation, name string) (string, error) {
				d, err := b.AuthorizeDatabase(ctx, relation, name)
				if err != nil {
					return "", err
				}
				return d.Namespace, nil
			},
		},
		{
			name: "AuthorizeKeyValue", id: "red-x", target: KeyValueTarget("red-x"),
			seed: func(ns, tenant string) client.Object { return tenantKeyValue(ns, "red-x", tenant) },
			call: func(b *Base, ctx context.Context, relation, name string) (string, error) {
				kv, err := b.AuthorizeKeyValue(ctx, relation, name)
				if err != nil {
					return "", err
				}
				return kv.Namespace, nil
			},
		},
	}
}

// TestDatastoreSeamsAuthorizeAMissAgainstTheCaller pins 403-before-404 on the
// arm that has no resource to derive a workspace from. A denied caller asking
// for a datastore that does not exist must be refused, not 404'd: the 404 would
// confirm non-existence to someone with no right to ask.
func TestDatastoreSeamsAuthorizeAMissAgainstTheCaller(t *testing.T) {
	for _, seam := range datastoreSeams() {
		t.Run(seam.name, func(t *testing.T) {
			b := resolvingBase() // nothing seeded
			b.Authz = fakeDenyChecker{}
			sink := &fakeAuditSink{}
			b.Audit = sink

			_, err := seam.call(b, actingCtx(), RelCanView, seam.id)
			if errors.Is(err, ErrNotFound) {
				t.Fatalf("%s 404'd a denied caller — an existence oracle for every datastore id", seam.name)
			}
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("%s on a miss = %v, want ErrForbidden", seam.name, err)
			}
			// A denied read IS audited (only denials are, for reads), and it must
			// carry the datastore target or the row never reaches the feed.
			if sink.len() != 1 {
				t.Fatalf("%s recorded %d audit events, want exactly 1", seam.name, sink.len())
			}
			if got := sink.events[0].Target; got != seam.target {
				t.Errorf("%s recorded target = %q, want %q", seam.name, got, seam.target)
			}
			if got := sink.events[0].Outcome; got != AuditDenied {
				t.Errorf("%s recorded outcome = %q, want %q", seam.name, got, AuditDenied)
			}
		})
	}
}

// TestDatastoreSeamsAuthorizeAHitAgainstTheResourcesWorkspace is the other arm:
// once the CR is found, the workspace that decides is the RESOURCE's, not the
// caller's default. The setup makes the two differ — the caller acts in tea-a,
// the datastore belongs to tea-b — and the checker allows ONLY the caller's, so
// a seam that consulted the wrong workspace would wrongly succeed here.
func TestDatastoreSeamsAuthorizeAHitAgainstTheResourcesWorkspace(t *testing.T) {
	for _, seam := range datastoreSeams() {
		t.Run(seam.name, func(t *testing.T) {
			b := resolvingBase(seam.seed("tea-b", "tea-b"))
			b.Authz = denyAllButChecker{allowed: WorkspaceObject("tea-a")}
			sink := &fakeAuditSink{}
			b.Audit = sink

			if _, err := seam.call(b, actingCtx(), RelCanView, seam.id); !errors.Is(err, ErrForbidden) {
				t.Fatalf("%s = %v, want ErrForbidden — the checker allows only the CALLER's workspace, "+
					"so success means the seam authorized against the wrong one", seam.name, err)
			}
			if sink.len() != 1 {
				t.Fatalf("%s recorded %d audit events, want exactly 1", seam.name, sink.len())
			}
			if got, want := sink.events[0].Resource, WorkspaceObject("tea-b"); got != want {
				t.Errorf("%s authorized against %q, want %q (the resource's own workspace)", seam.name, got, want)
			}
		})
	}
}

// TestDatastoreSeamsPropagateLookupErrors pins the third arm: a lookup failure
// that is NOT a NotFound is returned as-is, never converted into absence. An
// unreachable API server must not read as "no such database" — that would turn
// a transient outage into a 404, and would skip the authorize call entirely.
func TestDatastoreSeamsPropagateLookupErrors(t *testing.T) {
	boom := errors.New("apiserver unreachable")
	for _, seam := range datastoreSeams() {
		t.Run(seam.name, func(t *testing.T) {
			b := resolvingBase()
			b.Client = failingGetClient{Client: b.Client, err: boom}
			sink := &fakeAuditSink{}
			b.Audit = sink

			_, err := seam.call(b, actingCtx(), RelCanView, seam.id)
			if !errors.Is(err, boom) {
				t.Fatalf("%s = %v, want the underlying lookup error", seam.name, err)
			}
			if errors.Is(err, ErrNotFound) {
				t.Fatalf("%s reported an outage as absence", seam.name)
			}
			if sink.len() != 0 {
				t.Errorf("%s recorded %d audit events for a failed lookup, want 0", seam.name, sink.len())
			}
		})
	}
}

// TestDatastoreSeamsRecordTheCallingVerbsName is the fold's own regression.
// callerVerb walks the stack by a FIXED depth (verbFrameSkip) to name the audit
// verb, and the events feed is keyed off that name. AuthorizeDatabase and
// AuthorizeKeyValue now delegate their body to a shared helper; had callerVerb
// moved down into that helper along with it, every datastore verb would record
// "core.AuthorizeDatabase" instead of its own name and drop out of the feed —
// with nothing failing, because internal/events asserts these strings as table
// literals rather than driving the code that produces them.
func TestDatastoreSeamsRecordTheCallingVerbsName(t *testing.T) {
	for _, tc := range []struct {
		want string
		call func(*Base, context.Context) error
	}{
		{"core.deleteDatabaseLikeVerb", deleteDatabaseLikeVerb},
		{"core.deleteKeyValueLikeVerb", deleteKeyValueLikeVerb},
	} {
		t.Run(tc.want, func(t *testing.T) {
			b := resolvingBase(tenantDatabase("tea-a", "dpg-x", "tea-a"), tenantKeyValue("tea-a", "red-x", "tea-a"))
			sink := &fakeAuditSink{}
			b.Audit = sink

			if err := tc.call(b, actingCtx()); err != nil {
				t.Fatalf("verb returned %v", err)
			}
			if sink.len() != 1 {
				t.Fatalf("recorded %d audit events, want exactly 1: %+v", sink.len(), sink.events)
			}
			if got := sink.events[0].Verb; got != tc.want {
				t.Errorf("recorded verb = %q, want %q — the events feed is keyed off this name, so a "+
					"shifted frame count empties every datastore activity feed silently", got, tc.want)
			}
		})
	}
}

// deleteDatabaseLikeVerb / deleteKeyValueLikeVerb stand in for a feature verb
// (postgres.DeletePostgres, keyvalue.DeleteKeyValue): the whole body is the one
// authorize call every verb opens with, so callerVerb must resolve to this
// function's own name. RelCanManage is a WRITE relation, which is audited on
// success — a read relation would only record on denial.
func deleteDatabaseLikeVerb(b *Base, ctx context.Context) error {
	_, err := b.AuthorizeDatabase(ctx, RelCanManage, "dpg-x")
	return err
}

func deleteKeyValueLikeVerb(b *Base, ctx context.Context) error {
	_, err := b.AuthorizeKeyValue(ctx, RelCanManage, "red-x")
	return err
}

// TestDatastoreSeamsResolveTheActingWorkspaceOnce pins the hoist the fold made
// possible. Both halves of the seam need the acting workspace — the finder to
// pick namespace candidates, the authorize arm to decide which workspace
// decides — and each used to resolve it independently, so every managed
// Postgres/KeyValue request cost two resolutions.
//
// That is cheap for a normal caller (the tenant resolver caches positives) but
// not for an UNBOUND one: negatives are deliberately never cached, so a machine
// key with no workspace binding paid two identical TenantForIdentity queries on
// every request, forever. Hence the assertion is on the count, which is the
// only observable difference — the answers were always identical.
func TestDatastoreSeamsResolveTheActingWorkspaceOnce(t *testing.T) {
	for _, seam := range datastoreSeams() {
		for _, hit := range []bool{true, false} {
			name := seam.name + "/miss"
			if hit {
				name = seam.name + "/hit"
			}
			t.Run(name, func(t *testing.T) {
				calls := 0
				b := resolvingBase(seam.seed("tea-a", "tea-a"))
				b.Workspace = countingWorkspaceResolver{fakeWorkspace{"identity-a": "tea-a"}, &calls}

				lookup := seam.id
				if !hit {
					lookup = "absent-x"
				}
				_, _ = seam.call(b, actingCtx(), RelCanView, lookup)

				if calls != 1 {
					t.Errorf("Workspace.Tenant called %d times, want exactly 1 — the seam resolved the "+
						"acting workspace more than once for a single request", calls)
				}
			})
		}
	}
}

// TestGetInDatastoreNamespacesStopsOnTheFirstHit guards the shared direct-Get
// half of findDatabase/findKeyValue. The order is load-bearing during the
// ADR043 cutover — a datastore can briefly exist in BOTH namespaces, and the
// shared copy is the one being retired — so a sweep that kept going would hand
// the caller connection details for the wrong instance.
func TestGetInDatastoreNamespacesStopsOnTheFirstHit(t *testing.T) {
	b := resolvingBase(
		tenantDatabase("tea-a", "dpg-x", "tea-a"),   // the live, migrated copy
		tenantDatabase("default", "dpg-x", "tea-a"), // the shared copy being retired
	)
	var d appv1alpha1.Database
	found, err := b.getInDatastoreNamespaces(actingCtx(), "dpg-x", &d)
	if err != nil || !found {
		t.Fatalf("getInDatastoreNamespaces = (%v, %v), want (true, nil)", found, err)
	}
	if d.Namespace != "tea-a" {
		t.Errorf("resolved namespace = %q, want tea-a (the workspace's own, tried first)", d.Namespace)
	}
}

// TestGetInDatastoreNamespacesReportsAbsenceNotError pins the two ways the
// sweep can end. Every candidate coming back NotFound is (false, nil) — the
// caller's cue to fall through to the cluster-wide sweep — while any other
// error stops it, because an unreachable API server is not an absent resource.
//
// This is NOT subsumed by TestDatastoreSeamsPropagateLookupErrors one level up,
// which is the obvious-looking conclusion: measured by mutating this helper to
// return (false, nil) on a transport error, the seam test still PASSES — the
// finder falls through to its cluster-wide List, which fails for the same
// reason and re-raises an equivalent error. Only this test distinguishes
// "stopped correctly" from "stopped by accident one layer down".
func TestGetInDatastoreNamespacesReportsAbsenceNotError(t *testing.T) {
	var d appv1alpha1.Database

	b := resolvingBase()
	if found, err := b.getInDatastoreNamespaces(actingCtx(), "dpg-missing", &d); found || err != nil {
		t.Fatalf("all-NotFound = (%v, %v), want (false, nil)", found, err)
	}

	boom := errors.New("apiserver unreachable")
	b = resolvingBase()
	b.Client = failingGetClient{Client: b.Client, err: boom}
	if found, err := b.getInDatastoreNamespaces(actingCtx(), "dpg-x", &d); found || !errors.Is(err, boom) {
		t.Fatalf("transport error = (%v, %v), want (false, the error) — an outage must not read as absence", found, err)
	}
}

// failingGetClient fails every Get and List with err, leaving the rest of the
// client intact — the shape of an API-server outage, which the seams must tell
// apart from a NotFound.
type failingGetClient struct {
	client.Client
	err error
}

func (c failingGetClient) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return c.err
}

func (c failingGetClient) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return c.err
}
