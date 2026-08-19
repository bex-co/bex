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
	"fmt"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// crossworkspace_matrix_test.go is w7/m55's negative-authorization matrix: the
// standing proof that no service verb, REST route, GraphQL resolver, or MCP tool
// lets a caller reach ANOTHER workspace's resource. It is the security-regression
// analog of the m30 contract-conformance suite (w7's CI-guard charter).
//
// The core here (t001) is the service-verb sweep. It differs from
// TestAuthzGuardsEveryVerb (server_test.go) in exactly the way that lets it catch
// a whole failure class that sweep provably cannot:
//
//   - TestAuthzGuardsEveryVerb uses a DENY-ALL checker, so EVERY verb 403s no
//     matter which workspace it authorizes against. A verb that authorizes the
//     CALLER'S OWN workspace instead of the target resource's (a confused deputy)
//     passes it — the deny-all checker refuses both objects identically.
//   - This sweep uses an ALLOW-OWN / DENY-OTHER checker plus a resolver that makes
//     the caller a member of workspace A ONLY. Every resource in the fixture is
//     owned by workspace B. A correctly-wired verb authorizes against the
//     RESOURCE'S workspace (B) and is refused by the membership gate; a confused
//     deputy authorizes against the caller's OWN workspace (A), is allowed, and
//     leaks B's resource — surfacing as a success this sweep flags.
//
// The fixture (nonMemberResolver, allowOwnChecker) and the swept resource names
// (ownedApp/ownedDB/ownedKV in relationguard_test.go, all owned by "tea-b") are
// shared with t002/t003's HTTP/GraphQL/MCP matrices below.

// nonMemberResolver makes the caller a member of tea-a (their default workspace)
// and NOTHING else. Every resource in the matrix fixture lives in tea-b, so the
// caller is provably a non-member of the workspace that owns everything they try
// to reach — the negative half of relationguard_test.go's twoWorkspaceResolver
// (which, being a member of BOTH, is the wrong fixture for a DENY matrix).
type nonMemberResolver struct{}

func (nonMemberResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-a", true
}

func (nonMemberResolver) IsMember(_ context.Context, _ core.Identity, tenantID string) (bool, error) {
	return tenantID == "tea-a", nil
}

// allowOwnChecker grants the caller EVERY relation on their own workspace
// (tea-a) and DENIES every relation on any other object. This is the load-bearing
// difference from TestAuthzGuardsEveryVerb's deny-all fakeChecker: a confused-
// deputy verb that authorizes workspace:tea-a (the caller's own) instead of
// workspace:tea-b (the resource's) is ALLOWED here, so the leak it would cause
// becomes an observable success the matrix flags — a deny-all checker masks it.
type allowOwnChecker struct{}

func (allowOwnChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return object == core.WorkspaceObject("tea-a"), nil
}

// callVerbResult is callVerbNamed (relationguard_test.go) that RETURNS the verb's
// error result instead of discarding it, with a panic guard: a caller-scoped verb
// (a create/list/catalog that legitimately acts in the caller's own workspace)
// may reach its body against nil feature dependencies and panic — which is not a
// cross-workspace leak, so it is folded into a non-nil error (a non-success),
// keeping the sweep's single question — "did this verb SUCCEED against a
// workspace-B target?" — the only thing that fails it.
func callVerbResult(cv reflect.Value, m reflect.Method, ctx context.Context, name string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	mt := m.Func.Type()
	args := []reflect.Value{cv, reflect.ValueOf(ctx)}
	for a := 2; a < mt.NumIn(); a++ {
		at := mt.In(a)
		switch {
		case at.Kind() == reflect.Func:
			args = append(args, reflect.MakeFunc(at, func(_ []reflect.Value) []reflect.Value {
				outs := make([]reflect.Value, at.NumOut())
				for o := range outs {
					outs[o] = reflect.Zero(at.Out(o))
				}
				return outs
			}))
		case at.Kind() == reflect.String:
			args = append(args, reflect.ValueOf(name))
		default:
			args = append(args, reflect.Zero(at))
		}
	}
	out := m.Func.Call(args)
	e, _ := out[len(out)-1].Interface().(error)
	return e
}

// callerScopedVerbs are the verbs the matrix ENUMERATES but does not assert-deny,
// each with a one-line justification. A verb belongs here iff it takes no
// workspace-B-scoped target argument, so it cannot leak B's resources by
// construction: it acts on the caller's own identity, their default/resolved
// workspace, or a global catalog. Everything else must return an error (a denial)
// when a workspace-A caller invokes it against the tea-b fixture. Populated
// empirically (a verb that succeeds here and is NOT justified fails the sweep),
// so a newly-added leak cannot hide as a silent exclusion.
var callerScopedVerbs = map[string]bool{
	// Global compute/plan catalogs — the same ladder for every workspace, no
	// resource and no workspace in the signature (Authorize(can_view/can_create)
	// against the caller's own workspace, then a static tier list).
	"apps.Service.InstanceTypes":     true,
	"postgres.Service.InstanceTypes": true,
	"keyvalue.Service.InstanceTypes": true,
	// Whoami — returns the CALLER's own identity (email/name); the only workspace
	// it touches is the caller's own, to fill a missing email.
	"workspaces.Service.CurrentUser": true,
	// Name-availability probe — checks whether `name` is free in the CALLER's own
	// workspace (tenantNames(ctx)); `name` is a candidate string, never a lookup
	// into another workspace's resource.
	"apps.Service.NameAvailable": true,
	// Capability probe — returns only whether the caller's server has a push
	// transport; it accepts no resource/workspace argument.
	"notifications.Service.IsPushAvailable":     true,
	"notifications.Service.IsWebPushAvailable":  true,
	"notifications.Service.WebPushVAPIDPublicKey": true,
	// Durable push inbox verbs derive tenant + subject from the authenticated
	// caller; an exact event id is constrained inside that same scope.
	"notifications.Service.ListNotificationInbox":       true,
	"notifications.Service.UnreadPushNotificationCount": true,
	"notifications.Service.MarkPushNotificationRead":    true,
}

func TestCrossWorkspaceServiceVerbMatrix(t *testing.T) {
	const target = crossWorkspaceTarget
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "attacker", Method: "oauth2"})

	// A FRESH fixture per verb — the sweep calls destructive verbs (Delete, …)
	// too, and one shared fake apiserver would let an early Delete remove the
	// tea-b resource every later verb needs (relationguard_test.go's rationale).
	// crossWorkspaceBase seeds every named resource in tea-b, the workspace the
	// caller is NOT a member of.
	fixture := func() []any { return sweepableServices(crossWorkspaceBase()) }

	baseMethods := baseMethodNames()
	swept, denied := 0, 0
	seen := map[string]bool{}
	inventory := fixture()
	for si := range inventory {
		ct := reflect.TypeOf(inventory[si])
		for i := 0; i < ct.NumMethod(); i++ {
			m := ct.Method(i)
			if !isVerbMethod(baseMethods, m) {
				continue
			}
			swept++
			name := ct.Elem().String() + "." + m.Name
			seen[name] = true
			svc := fixture()[si]
			err := callVerbResult(reflect.ValueOf(svc), m, ctx, target)
			if err == nil {
				if callerScopedVerbs[name] {
					continue
				}
				t.Errorf("%s: returned success for a workspace-A caller acting on a workspace-B target — "+
					"either it leaks another workspace's resource (confused deputy / missing guard) or it is "+
					"caller-scoped and must be justified in callerScopedVerbs", name)
				continue
			}
			denied++
		}
	}
	if swept < wantMinSweptVerbs {
		t.Fatalf("sweep found only %d verbs — reflection filter broke?", swept)
	}
	assertNoStaleExclusions(t, "callerScopedVerbs", callerScopedVerbs, seen)
	t.Logf("%d/%d swept verbs deny a non-member cross-workspace caller; %d caller-scoped exclusions", denied, swept, len(callerScopedVerbs))
}

// assertNoStaleExclusions fails for any exclusion-map key the sweep no longer
// enumerates (`present`) — shared by all four m55 matrices so a renamed or
// deleted verb/route/field/tool can't leave dead cover behind in an exclusion
// list, which would let a future leak hide there.
func assertNoStaleExclusions(t *testing.T, label string, exclusions, present map[string]bool) {
	t.Helper()
	for k := range exclusions {
		if !present[k] {
			t.Errorf("%s lists %q, which the sweep no longer enumerates — remove the stale exclusion", label, k)
		}
	}
}
