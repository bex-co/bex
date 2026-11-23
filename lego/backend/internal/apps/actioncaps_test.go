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

package apps

import (
	"context"
	"errors"
	"fmt"
	"testing"

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

func hasAction(acts []core.ActionDecision, id string) bool {
	for _, a := range acts {
		if a.Action == id {
			return true
		}
	}
	return false
}

// TestActionCapabilities_SuspendResumeAgreeWithTheVerb (w4/m578): for an Admin
// and a Developer, on a member of a protected and of an unprotected
// environment, the projection's suspend and resume decisions are the verbs'
// own answers. Before w4/m578 the projection decided both on can_operate alone,
// so a Developer on a protected member was offered a Suspend and a Resume the
// verb then refused with 403 (requireUnprotected asks for the Admin role
// there). An Admin keeps the confirmation hint on suspend, and the Developer's
// refusal is named so a client can say only an Admin may.
func TestActionCapabilities_SuspendResumeAgreeWithTheVerb(t *testing.T) {
	roles := []struct {
		name  string
		as    func(*Service)
		admin bool
	}{{"admin", asAdmin, true}, {"developer", asNonAdmin, false}}
	for _, role := range roles {
		for _, protected := range []bool{true, false} {
			for _, action := range []string{core.ActionSuspend, core.ActionResume} {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", role.name, action, protected), func(t *testing.T) {
					status := core.ProtectedStatusUnprotected
					if protected {
						status = core.ProtectedStatusProtected
					}
					rec := &recordingStore{protectedStatus: map[string]string{"srv-1": status}}
					a := managedApp("web", "srv-1")
					a.Spec.Suspended = action == core.ActionResume // the state the verb acts on
					svc, cl := newService(rec, a)
					role.as(svc)
					ctx := ctxAs("user-a")

					acts, err := svc.ActionCapabilities(ctx, "web")
					if err != nil {
						t.Fatalf("ActionCapabilities: %v", err)
					}
					if rec.protectedCalls != 1 {
						t.Errorf("projection read protection %d times, want once for both verbs", rec.protectedCalls)
					}
					got := actionByID(t, acts, action)
					if restart := actionByID(t, acts, core.ActionRestart); restart.Outcome != core.DecisionAllowed {
						t.Errorf("restart is not a restricted verb, got %+v", restart)
					}
					verb := svc.Suspend
					if action == core.ActionResume {
						verb = svc.Resume
					}
					_, verbErr := verb(ctx, "web")

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
						if getApp(t, cl, "web").Spec.Suspended != a.Spec.Suspended {
							t.Error("a refused verb must not change the service")
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

// TestActionCapabilities_ProtectionReadFailureIsUnavailable: when the
// protection lookup fails, suspend and resume read as unavailable, never as a
// role refusal and never as allowed, as the verb fails closed on the same
// read. Restart does not consult it.
func TestActionCapabilities_ProtectionReadFailureIsUnavailable(t *testing.T) {
	rec := &recordingStore{protectedErr: errors.New("db down")}
	svc, _ := newService(rec, managedApp("web", "srv-1"))
	asNonAdmin(svc)

	acts, err := svc.ActionCapabilities(ctxAs("user-a"), "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	for _, id := range []string{core.ActionSuspend, core.ActionResume} {
		if a := actionByID(t, acts, id); a.Outcome != core.DecisionUnavailable || a.Reason != core.ReasonAuthzUnavailable {
			t.Errorf("%s with a failed protection read = %+v, want unavailable/authz_unavailable", id, a)
		}
	}
	if restart := actionByID(t, acts, core.ActionRestart); restart.Outcome != core.DecisionAllowed {
		t.Errorf("restart = %+v, want allowed", restart)
	}
	if _, err := svc.Suspend(ctxAs("user-a"), "web"); err == nil {
		t.Error("Suspend with a failed protection read succeeded; the verb must fail closed")
	}
}

// TestActionCapabilities_CronRowsOnlyForCronJobs: cron actions exist exactly
// for cron_job services (an absent action means "does not exist for this
// type"), a suspended cron reports the suspended precondition for run-now,
// and cancel reports no_active_run when nothing is pending — the same
// predicates TriggerCronRun / CancelCurrentCronRun enforce.
func TestActionCapabilities_CronRowsOnlyForCronJobs(t *testing.T) {
	cron := managedApp("nightly", "srv-2")
	cron.Spec.Type = appv1alpha1.TypeCronJob
	svc, _ := newService(&recordingStore{}, cron, managedApp("web", "srv-1"))

	acts, err := svc.ActionCapabilities(context.Background(), "nightly")
	if err != nil {
		t.Fatalf("ActionCapabilities(cron): %v", err)
	}
	if run := actionByID(t, acts, core.ActionCronRunNow); run.Precondition != "" {
		t.Fatalf("idle cron run-now precondition = %q, want none", run.Precondition)
	}
	if cancel := actionByID(t, acts, core.ActionCronCancelRun); cancel.Precondition != core.PrecondNoActiveRun {
		t.Fatalf("cancel with no pending run = %+v, want no_active_run", cancel)
	}

	webActs, err := svc.ActionCapabilities(context.Background(), "web")
	if err != nil {
		t.Fatalf("ActionCapabilities(web): %v", err)
	}
	if hasAction(webActs, core.ActionCronRunNow) || hasAction(webActs, core.ActionCronCancelRun) {
		t.Fatalf("a web service must project no cron actions: %+v", webActs)
	}

	// Suspended cron: run-now blocks on state, exactly like TriggerCronRun.
	suspended := managedApp("asleep", "srv-3")
	suspended.Spec.Type = appv1alpha1.TypeCronJob
	suspended.Spec.Suspended = true
	svc2, _ := newService(&recordingStore{}, suspended)
	acts2, err := svc2.ActionCapabilities(context.Background(), "asleep")
	if err != nil {
		t.Fatalf("ActionCapabilities(suspended cron): %v", err)
	}
	if run := actionByID(t, acts2, core.ActionCronRunNow); run.Precondition != core.PrecondSuspended {
		t.Fatalf("suspended cron run-now = %+v, want suspended precondition", run)
	}
}

// TestActionCapabilities_ViewerDeniedWithoutPreconditionLeak: a viewer gets
// denied/insufficient_permission for every operate action — with NO
// precondition attached, since readiness detail on a forbidden action is
// mildly disclosive (the protected status here would otherwise leak).
func TestActionCapabilities_ViewerDeniedWithoutPreconditionLeak(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))
	checker := &relationChecker{allow: map[string]bool{core.RelCanView: true}}
	svc.Base.Authz = checker
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer-1", Method: "session"})

	acts, err := svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	for _, id := range []string{core.ActionRestart, core.ActionSuspend, core.ActionResume} {
		a := actionByID(t, acts, id)
		if a.Outcome != core.DecisionDenied || a.Reason != core.ReasonInsufficientPermission {
			t.Errorf("viewer %s = %+v, want denied/insufficient_permission", id, a)
		}
		if a.Precondition != "" {
			t.Errorf("denied %s leaked precondition %q", id, a.Precondition)
		}
	}
	// Nor does it read protection or ask the admin check: a viewer pays no
	// guard read for buttons it will never enable (w4/m578).
	if rec.protectedCalls != 0 {
		t.Errorf("viewer projection read protection %d times, want none", rec.protectedCalls)
	}
	for _, relation := range checker.asked {
		if relation == core.RelCanManage {
			t.Errorf("viewer projection asked can_manage: %v", checker.asked)
		}
	}
}

// TestActionCapabilities_IssuesNoWrites: the projection is a pure read — no
// CR patch, no store mutation, no confirmation arming.
func TestActionCapabilities_IssuesNoWrites(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))
	before := getApp(t, cl, "web").ResourceVersion
	if _, err := svc.ActionCapabilities(context.Background(), "web"); err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if after := getApp(t, cl, "web").ResourceVersion; after != before {
		t.Fatalf("projection mutated the CR (resourceVersion %s -> %s)", before, after)
	}
}

// TestActionCapabilities_ForeignWorkspaceResourceReadsAbsent (w6/m136/t003):
// a resource owned by another workspace reads as absent in the acting one —
// the projection answers about "this resource HERE", never about elsewhere.
func TestActionCapabilities_ForeignWorkspaceResourceReadsAbsent(t *testing.T) {
	a := managedApp("web", "srv-1")
	a.Labels[core.LabelTenant] = "tea-elsewhere"
	svc, _ := newService(&recordingStore{}, a)

	if _, err := svc.ActionCapabilities(context.Background(), "web"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("foreign-workspace resource = %v, want ErrNotFound", err)
	}
}

// TestActionCapabilities_SuspendResumeReadTheResourceState is w4/132: the two
// lifecycle decisions used to compute protection and billing only, so they read
// `allowed` in every state — live, three readings across three distinct states
// produced byte-identical output, while `resumeService` on a running service
// and `suspendService` on a suspended one were both accepted no-ops. A
// projection whose whole job is to say which buttons do something must not
// enable one that does nothing.
func TestActionCapabilities_SuspendResumeReadTheResourceState(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		svc, _ := newService(&recordingStore{}, managedApp("web", "srv-1"))
		acts, err := svc.ActionCapabilities(context.Background(), "web")
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		// Suspend is the verb that does something here, so it stays ready.
		if s := actionByID(t, acts, core.ActionSuspend); s.Precondition != "" {
			t.Errorf("running suspend precondition = %q, want none", s.Precondition)
		}
		if r := actionByID(t, acts, core.ActionResume); r.Precondition != core.PrecondNotSuspended {
			t.Errorf("running resume = %+v, want the not_suspended precondition", r)
		}
	})

	t.Run("suspended", func(t *testing.T) {
		asleep := managedApp("web", "srv-1")
		asleep.Spec.Suspended = true
		svc, _ := newService(&recordingStore{}, asleep)
		acts, err := svc.ActionCapabilities(context.Background(), "web")
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		if s := actionByID(t, acts, core.ActionSuspend); s.Precondition != core.PrecondSuspended {
			t.Errorf("suspended suspend = %+v, want the suspended precondition", s)
		}
		if r := actionByID(t, acts, core.ActionResume); r.Precondition != "" {
			t.Errorf("suspended resume precondition = %q, want none", r.Precondition)
		}
	})

	// State is checked before protection, the order the cron branch documents
	// and the verbs themselves take: a suspended resource cannot be suspended
	// again whatever protection answers, so reporting a confirmation phrase for
	// it would demand a phrase for a call that would no-op anyway.
	t.Run("suspended and protected reports state, not the phrase", func(t *testing.T) {
		asleep := managedApp("web", "srv-1")
		asleep.Spec.Suspended = true
		rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
		svc, _ := newService(rec, asleep)
		acts, err := svc.ActionCapabilities(context.Background(), "web")
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		if s := actionByID(t, acts, core.ActionSuspend); s.Precondition != core.PrecondSuspended {
			t.Errorf("suspended+protected suspend = %+v, want the suspended precondition", s)
		}
	})
}
