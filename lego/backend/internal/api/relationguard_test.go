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
	"reflect"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// relationguard_test.go holds w6/m14's structural guard. The milestone made the
// shared fetch-by-name (core.Base.GetApp, postgres/keyvalue's fetch*) authorize
// the CALLING VERB'S relation against the RESOURCE'S OWN workspace — which only
// works if each verb hands it the relation it actually authorized. That is a
// convention (lego/backend/CLAUDE.md), and a convention across ~45 call sites is
// a convention that will be broken: a read verb copy-pasted into a write verb
// keeps compiling while its fetch still says can_view, and the result is a
// viewer of workspace B deleting B's service on the strength of being an admin
// of A. This sweep makes that a test failure instead of a CVE.
//
// It is deliberately mechanical — no per-verb table to maintain, like
// TestAuthzGuardsEveryVerb (whose service inventory it reuses, so a new feature
// joins this guard for free).

// relationRecorder allows everything and records every (relation, object) pair
// it was asked about — the two workspaces a cross-workspace verb touches.
type relationRecorder struct {
	calls []struct{ relation, object string }
}

func (r *relationRecorder) Check(_ context.Context, _, relation, object string) (bool, error) {
	r.calls = append(r.calls, struct{ relation, object string }{relation, object})
	return true, nil
}

// twoWorkspaceResolver: the caller belongs to tea-a (their default) and tea-b.
type twoWorkspaceResolver struct{}

func (twoWorkspaceResolver) Tenant(_ context.Context, _ core.Identity) (string, bool) {
	return "tea-a", true
}

func (twoWorkspaceResolver) IsMember(_ context.Context, _ core.Identity, tenantID string) (bool, error) {
	return tenantID == "tea-a" || tenantID == "tea-b", nil
}

func ownedApp(name, tenantID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: tenantID}
	return a
}

func ownedDB(name, tenantID string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default",
		Labels: map[string]string{core.LabelTenant: tenantID},
	}}
}

func ownedKV(name, tenantID string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default",
		Labels: map[string]string{core.LabelTenant: tenantID},
	}}
}

// callVerbNamed is callVerb with every string argument set to `name` instead of
// "" — which is what makes this sweep bite: with zero-value args every verb
// looks up a resource named "", finds nothing, and never reaches the gate under
// test. Non-string args stay zero (a verb that then fails validation before its
// fetch simply contributes no checks, and the guard passes vacuously for it —
// never falsely).
func callVerbNamed(cv reflect.Value, m reflect.Method, ctx context.Context, name string) {
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
	m.Func.Call(args)
}

// TestFetchByNameUsesTheVerbsOwnRelation: for every verb that reaches a
// resource living in the caller's OTHER workspace (tea-b), every relation it
// checks there must be one it also checked against its own workspace (tea-a) —
// i.e. the fetch was given the verb's own relation, not a weaker one.
//
// A verb that authorizes can_create and then fetches with can_view records
// {can_create} on tea-a and {can_view} on tea-b, and fails here.
func TestFetchByNameUsesTheVerbsOwnRelation(t *testing.T) {
	const name = "web"
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})

	// A FRESH fixture per verb: the sweep calls destructive verbs too, and a
	// single shared fake apiserver would let Delete (early, alphabetically) remove
	// the very resource every later verb needs — silently emptying the sweep.
	fixture := func(rec *relationRecorder) []any {
		return sweepableServices(&core.Base{
			// Every named resource lives in tea-b — the caller's OTHER workspace — so
			// every fetch-by-name takes the cross-workspace path this guard is about.
			Client:    fakeClient(ownedApp(name, "tea-b"), ownedDB(name, "tea-b"), ownedKV(name, "tea-b")),
			Namespace: "default",
			Authz:     rec,
			Workspace: twoWorkspaceResolver{},
		})
	}

	baseMethods := baseMethodNames()
	crossed := 0 // verbs that actually reached a resource in the other workspace
	inventory := fixture(&relationRecorder{})
	for si := range inventory {
		ct := reflect.TypeOf(inventory[si])
		for i := 0; i < ct.NumMethod(); i++ {
			m := ct.Method(i)
			if !isVerbMethod(baseMethods, m) {
				continue
			}
			rec := &relationRecorder{}
			svc := fixture(rec)[si]
			callVerbNamed(reflect.ValueOf(svc), m, ctx, name)

			onOwn, onOther := map[string]bool{}, map[string]bool{}
			for _, c := range rec.calls {
				switch c.object {
				case core.WorkspaceObject("tea-a"):
					onOwn[c.relation] = true
				case core.WorkspaceObject("tea-b"):
					onOther[c.relation] = true
				}
			}
			if len(onOther) == 0 {
				continue // this verb never reached a resource in the other workspace
			}
			crossed++
			for rel := range onOther {
				if !onOwn[rel] {
					t.Errorf("%s.%s: fetched a resource in the caller's other workspace with %q, "+
						"but the verb authorized %v — the fetch must be given the SAME relation the verb "+
						"authorized, or a role in one workspace leaks into another (lego/backend/CLAUDE.md)",
						ct.Elem().Name(), m.Name, rel, keys(onOwn))
				}
			}
		}
	}
	// Guard the guard: if a refactor stops these verbs from reaching the
	// cross-workspace path, this test would silently pass while checking nothing.
	if crossed < 15 {
		t.Fatalf("only %d verbs reached a resource in the caller's other workspace — the sweep has gone vacuous", crossed)
	}
	t.Logf("checked the fetch relation of %d cross-workspace verbs", crossed)
}

func keys(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
