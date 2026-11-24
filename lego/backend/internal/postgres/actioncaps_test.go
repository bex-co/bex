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

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func actionByID(t *testing.T, acts []core.ActionDecision, id string) core.ActionDecision {
	t.Helper()
	for _, a := range acts {
		if a.Action == id {
			return a
		}
	}
	t.Fatalf("no %q action in %+v", id, acts)
	return core.ActionDecision{}
}

// operateOnlyDenier grants exactly can_view — the viewer shape.
type operateOnlyDenier struct{}

func (operateOnlyDenier) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation == core.RelCanView, nil
}

// TestActionCapabilities_SuspendResumeAgreeWithTheVerb (w4/m578): for an Admin
// and a Developer, on a member of a protected and of an unprotected
// environment, the projection's suspend and resume decisions are the verbs'
// own answers. Before w4/m578 the projection decided both on can_operate alone,
// so a Developer on a protected database was offered a Suspend and a Resume the
// verb then refused with 403 (requireUnprotected asks for the Admin role
// there). An Admin keeps the confirmation hint on suspend, and the Developer's
// refusal is named so a client can say only an Admin may.
func TestActionCapabilities_SuspendResumeAgreeWithTheVerb(t *testing.T) {
	roles := []struct {
		name  string
		authz core.Checker
		admin bool
	}{{"admin", allowAllRelations{}, true}, {"developer", denyWorkspaceManage{}, false}}
	for _, role := range roles {
		for _, protected := range []bool{true, false} {
			for _, action := range []string{core.ActionSuspend, core.ActionResume} {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", role.name, action, protected), func(t *testing.T) {
					obj := databaseForProtection("dpg-orders", "orders", true)
					if !protected {
						obj.Labels[core.LabelEnvironment] = "env-staging" // a member, of an unprotected environment
					}
					obj.Spec.Suspended = action == core.ActionResume // the state the verb acts on
					svc, cl, prot := protectedPostgresService(obj)
					svc.Authz = role.authz
					ctx := ctxAsCaller()

					acts, err := svc.ActionCapabilities(ctx, obj.Name)
					if err != nil {
						t.Fatalf("ActionCapabilities: %v", err)
					}
					if prot.calls != 1 {
						t.Errorf("projection read protection %d times, want once for both verbs", prot.calls)
					}
					got := actionByID(t, acts, action)
					verb := svc.Suspend
					if action == core.ActionResume {
						verb = svc.Resume
					}
					_, verbErr := verb(ctx, obj.Name)

					allowed := got.Outcome == core.DecisionAllowed
					if want := role.admin || !protected; allowed != want || (verbErr == nil) != want {
						t.Fatalf("projection %+v and verb error %v disagree, want allowed=%t for both", got, verbErr, want)
					}
					switch {
					case !allowed:
						if got.Reason != core.ReasonProtectedEnvironmentAdmin || got.Precondition != "" || !errors.Is(verbErr, core.ErrForbidden) {
							t.Errorf("refused %s = %+v (verb %v), want denied/%s with no precondition and ErrForbidden",
								action, got, verbErr, core.ReasonProtectedEnvironmentAdmin)
						}
						var after appv1alpha1.Database
						if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), &after); err != nil {
							t.Fatalf("get: %v", err)
						}
						if after.Spec.Suspended != obj.Spec.Suspended {
							t.Error("a refused verb must not change the database")
						}
					case action == core.ActionSuspend && protected:
						if got.Precondition != core.PrecondProtectedConfirmation {
							t.Errorf("an Admin's protected suspend = %+v, want the confirmation hint", got)
						}
					default:
						if got.Precondition != "" {
							t.Errorf("%s = %+v, want ready", action, got)
						}
					}
				})
			}
		}
	}
}

// TestActionCapabilities_ViewerDeniedWithoutGuardReads: a viewer is denied
// every action without preconditions, and without the projection reading the
// protection store or asking the admin check (readiness detail on a forbidden
// action would leak). Restart is not a restricted verb, so a Developer keeps
// it on a protected database.
func TestActionCapabilities_ViewerDeniedWithoutGuardReads(t *testing.T) {
	svc, _, prot := protectedPostgresService(databaseForProtection("dpg-locked", "locked", true))
	svc.Base.Authz = operateOnlyDenier{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer-1", Method: "session"})
	acts, err := svc.ActionCapabilities(ctx, "dpg-locked")
	if err != nil {
		t.Fatalf("viewer ActionCapabilities: %v", err)
	}
	for _, id := range []string{core.ActionRestart, core.ActionSuspend, core.ActionResume} {
		a := actionByID(t, acts, id)
		if a.Outcome != core.DecisionDenied || a.Reason != core.ReasonInsufficientPermission || a.Precondition != "" {
			t.Errorf("viewer %s = %+v, want denied/insufficient_permission with no precondition", id, a)
		}
	}
	if prot.calls != 0 {
		t.Errorf("viewer projection read the protection store %d times — a denied caller must not pay or observe guard reads", prot.calls)
	}

	svc2, _, _ := protectedPostgresService(databaseForProtection("dpg-orders", "orders", true))
	svc2.Authz = denyWorkspaceManage{}
	acts2, err := svc2.ActionCapabilities(ctxAsCaller(), "dpg-orders")
	if err != nil {
		t.Fatalf("developer ActionCapabilities: %v", err)
	}
	if r := actionByID(t, acts2, core.ActionRestart); r.Outcome != core.DecisionAllowed || r.Precondition != "" {
		t.Errorf("a Developer's restart on a protected database = %+v, want allowed and ready", r)
	}
}

// TestActionCapabilities_SuspendResumeReadTheResourceState is w4/132, the
// Postgres half — the one the note read in code but did not drive live,
// judging the two datastore projections near-identical. "Identical in shape"
// is an argument, not an observation, so it is asserted here.
func TestActionCapabilities_SuspendResumeReadTheResourceState(t *testing.T) {
	running := databaseForProtection("dpg-scratch", "scratch", false)
	svc, _, _ := protectedPostgresService(running)
	acts, err := svc.ActionCapabilities(context.Background(), running.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities (running): %v", err)
	}
	if s := actionByID(t, acts, core.ActionSuspend); s.Precondition != "" {
		t.Errorf("running suspend precondition = %q, want none", s.Precondition)
	}
	if r := actionByID(t, acts, core.ActionResume); r.Precondition != core.PrecondNotSuspended {
		t.Errorf("running resume = %+v, want the not_suspended precondition", r)
	}
	if rs := actionByID(t, acts, core.ActionRestart); rs.Outcome != core.DecisionAllowed || rs.Precondition != "" {
		t.Errorf("running restart = %+v, want allowed with no precondition", rs)
	}

	asleep := databaseForProtection("dpg-asleep", "asleep", true)
	asleep.Spec.Suspended = true
	svc2, _, _ := protectedPostgresService(asleep)
	acts2, err := svc2.ActionCapabilities(context.Background(), asleep.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities (suspended): %v", err)
	}
	if s := actionByID(t, acts2, core.ActionSuspend); s.Precondition != core.PrecondSuspended {
		t.Errorf("suspended suspend = %+v, want the suspended precondition", s)
	}
	if r := actionByID(t, acts2, core.ActionResume); r.Precondition != "" {
		t.Errorf("suspended resume precondition = %q, want none", r.Precondition)
	}
	if rs := actionByID(t, acts2, core.ActionRestart); rs.Outcome != core.DecisionAllowed || rs.Precondition != core.PrecondSuspended {
		t.Errorf("suspended restart = %+v, want allowed with suspended precondition", rs)
	}
}

// TestActionCapabilities_RestartBillingAndViewerGuards pins Restart's
// precondition ladder in ActionCapabilities: suspended wins over billing,
// billing blocks a running restart, and a viewer pays no billing reads.
func TestActionCapabilities_RestartBillingAndViewerGuards(t *testing.T) {
	t.Run("running under billing lock reports billing_blocked", func(t *testing.T) {
		db := databaseForProtection("dpg-run-bill", "run-bill", false)
		db.Labels[core.LabelTenant] = "tea-a"
		svc, _, _ := protectedPostgresService(db)
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		gate := &refusingBillingGate{}
		svc.Billing = gate

		acts, err := svc.ActionCapabilities(ctxAs("user-a"), db.Name)
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		if rs := actionByID(t, acts, core.ActionRestart); rs.Outcome != core.DecisionAllowed || rs.Precondition != core.PrecondBillingBlocked {
			t.Errorf("billing-blocked running restart = %+v, want allowed/%s", rs, core.PrecondBillingBlocked)
		}
		if len(gate.calls) != 1 {
			t.Errorf("billing calls = %d, want 1", len(gate.calls))
		}
	})

	t.Run("suspended under billing lock reports suspended on restart", func(t *testing.T) {
		db := databaseForProtection("dpg-susp-bill", "susp-bill", false)
		db.Labels[core.LabelTenant] = "tea-a"
		db.Spec.Suspended = true
		svc, _, _ := protectedPostgresService(db)
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		gate := &refusingBillingGate{}
		svc.Billing = gate

		acts, err := svc.ActionCapabilities(ctxAs("user-a"), db.Name)
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		if rs := actionByID(t, acts, core.ActionRestart); rs.Outcome != core.DecisionAllowed || rs.Precondition != core.PrecondSuspended {
			t.Errorf("suspended restart under billing lock = %+v, want allowed/%s", rs, core.PrecondSuspended)
		}
		if r := actionByID(t, acts, core.ActionResume); r.Outcome != core.DecisionAllowed || r.Precondition != core.PrecondBillingBlocked {
			t.Errorf("suspended resume under billing lock = %+v, want allowed/%s", r, core.PrecondBillingBlocked)
		}
	})

	t.Run("viewer on running database pays zero billing reads", func(t *testing.T) {
		db := databaseForProtection("dpg-view-bill", "view-bill", false)
		db.Labels[core.LabelTenant] = "tea-a"
		svc, _, _ := protectedPostgresService(db)
		svc.Workspace = fakeWorkspace{"viewer-1": "tea-a"}
		svc.Authz = operateOnlyDenier{}
		gate := &refusingBillingGate{}
		svc.Billing = gate

		acts, err := svc.ActionCapabilities(ctxAs("viewer-1"), db.Name)
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		if rs := actionByID(t, acts, core.ActionRestart); rs.Outcome != core.DecisionDenied || rs.Precondition != "" {
			t.Errorf("viewer restart = %+v, want denied with no precondition", rs)
		}
		if len(gate.calls) != 0 {
			t.Errorf("viewer paid %d billing reads, want 0", len(gate.calls))
		}
	})
}
