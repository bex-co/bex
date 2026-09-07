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

package deploys

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
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

// TestActionCapabilities_RollbackEligibilityMatchesVerb (ADR087, w6/m136):
// the projection reports no_eligible_rollback_target exactly while the verb
// would 409 every candidate, and clears the moment an eligible row exists —
// both sides of the one shared RollbackEligible predicate. Cancel likewise
// mirrors the open-deploy window.
func TestActionCapabilities_RollbackEligibilityMatchesVerb(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	ctx := context.Background()

	// Empty history: nothing to cancel, nothing to roll back to.
	acts, err := svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if d := actionByID(t, acts, core.ActionDeploy); d.Outcome != core.DecisionAllowed || d.Precondition != "" {
		t.Fatalf("bare deploy = %+v, want allowed and ready", d)
	}
	if c := actionByID(t, acts, core.ActionCancelDeploy); c.Precondition != core.PrecondNoActiveDeploy {
		t.Fatalf("cancel with empty history = %+v, want no_active_deploy", c)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondNoEligibleRollbackTarget {
		t.Fatalf("rollback with empty history = %+v, want no_eligible_rollback_target", r)
	}

	// A finished-but-never-live deploy is still ineligible — and the verb
	// agrees with the projection.
	now := fixedNow()
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-failed", AppID: "srv-1", Status: store.DeployBuildFailed,
		CreatedAt: now, UpdatedAt: now, FinishedAt: &now,
	}}
	acts, err = svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondNoEligibleRollbackTarget {
		t.Fatalf("rollback with only a failed deploy = %+v, want no_eligible_rollback_target", r)
	}
	if _, err := svc.Rollback(ctx, "web", "dep-failed"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Rollback to a never-live deploy = %v, want ErrConflict — the verb and projection share one predicate", err)
	}

	// A live deploy with a resolved image: rollback ready, and the verb accepts.
	ds.byApp["srv-1"] = append([]store.Deploy{{
		ID: "dep-live", AppID: "srv-1", Status: store.DeployLive, ResolvedImage: "img:1",
		CreatedAt: now, UpdatedAt: now, FinishedAt: &now,
	}}, ds.byApp["srv-1"]...)
	acts, err = svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != "" {
		t.Fatalf("rollback with a live target = %+v, want ready", r)
	}
	if _, err := svc.Rollback(ctx, "web", "dep-live"); err != nil {
		t.Fatalf("Rollback to the live target: %v", err)
	}

	// An open deploy makes cancel ready.
	ds.byApp["srv-1"] = append([]store.Deploy{{
		ID: "dep-open", AppID: "srv-1", Status: store.DeployBuildInProgress,
		CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}}, ds.byApp["srv-1"]...)
	acts, err = svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if c := actionByID(t, acts, core.ActionCancelDeploy); c.Precondition != "" {
		t.Fatalf("cancel with an open deploy = %+v, want ready", c)
	}
}

// TestActionCapabilities_SuspendedAndUnmanagedPreconditions: a suspended
// service blocks deploy and rollback (Trigger/Rollback's shared guard) but
// not cancel; a hand-applied service with no store row has no deploy
// machinery at all (unavailable — the ErrDeploysUnavailable analogue).
func TestActionCapabilities_SuspendedAndUnmanagedPreconditions(t *testing.T) {
	ds := newFakeStore()
	suspended := sampleApp("asleep", "srv-2")
	suspended.Spec.Suspended = true
	now := fixedNow()
	ds.byApp["srv-2"] = []store.Deploy{{
		ID: "dep-live", AppID: "srv-2", Status: store.DeployLive, ResolvedImage: "img:1",
		CreatedAt: now, UpdatedAt: now, FinishedAt: &now,
	}}
	svc, _ := newService(ds, suspended, sampleApp("bare", ""))
	ctx := context.Background()

	acts, err := svc.ActionCapabilities(ctx, "asleep")
	if err != nil {
		t.Fatalf("ActionCapabilities(suspended): %v", err)
	}
	if d := actionByID(t, acts, core.ActionDeploy); d.Precondition != core.PrecondSuspended {
		t.Fatalf("deploy on suspended = %+v, want suspended", d)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondSuspended {
		t.Fatalf("rollback on suspended = %+v, want suspended (the verb refuses before eligibility)", r)
	}
	if c := actionByID(t, acts, core.ActionCancelDeploy); c.Precondition != core.PrecondNoActiveDeploy {
		t.Fatalf("cancel is exempt from the suspended gate, got %+v", c)
	}

	bareActs, err := svc.ActionCapabilities(ctx, "bare")
	if err != nil {
		t.Fatalf("ActionCapabilities(bare): %v", err)
	}
	for _, id := range []string{core.ActionDeploy, core.ActionCancelDeploy, core.ActionRollback} {
		if a := actionByID(t, bareActs, id); a.Precondition != core.PrecondUnavailable {
			t.Errorf("unmanaged %s = %+v, want unavailable", id, a)
		}
	}
}
