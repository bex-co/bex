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

// TestActionCapabilities_SuspendPreconditionMatchesGuard (ADR087, w6/m136):
// the projection reports protected_confirmation_required for suspend exactly
// when the execute path blocks an unconfirmed Suspend — both sides of the one
// shared appProtected predicate, asserted against each other so the
// projection cannot drift from enforcement.
func TestActionCapabilities_SuspendPreconditionMatchesGuard(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))

	acts, err := svc.ActionCapabilities(context.Background(), "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	suspend := actionByID(t, acts, core.ActionSuspend)
	if suspend.Outcome != core.DecisionAllowed || suspend.Precondition != core.PrecondProtectedConfirmation {
		t.Fatalf("protected suspend = %+v, want allowed + protected_confirmation_required", suspend)
	}
	if restart := actionByID(t, acts, core.ActionRestart); restart.Precondition != "" {
		t.Fatalf("restart is not protection-guarded, got precondition %q", restart.Precondition)
	}
	// The execute path enforces exactly what the projection reported.
	if _, err := svc.Suspend(context.Background(), "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unconfirmed Suspend on a protected member = %v, want ErrBadRequest", err)
	}

	// Unprotected: the precondition clears and the same verb passes.
	rec2 := &recordingStore{protectedStatus: map[string]string{"srv-1": "unprotected"}}
	svc2, _ := newService(rec2, managedApp("web", "srv-1"))
	acts2, err := svc2.ActionCapabilities(context.Background(), "web")
	if err != nil {
		t.Fatalf("ActionCapabilities (unprotected): %v", err)
	}
	if s2 := actionByID(t, acts2, core.ActionSuspend); s2.Precondition != "" {
		t.Fatalf("unprotected suspend precondition = %q, want none", s2.Precondition)
	}
	if _, err := svc2.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend on an unprotected member: %v", err)
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
	svc.Base.Authz = &relationChecker{allow: map[string]bool{core.RelCanView: true}}
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
