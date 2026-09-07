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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// TestActionCapabilities_SuspendPreconditionMatchesGuard (ADR087, w6/m136):
// the projection reports protected_confirmation_required exactly when the
// execute path blocks an unconfirmed Suspend — both read the same
// EnvironmentProtected predicate — and a viewer's denied actions carry no
// precondition (no protection read, no leak).
func TestActionCapabilities_SuspendPreconditionMatchesGuard(t *testing.T) {
	db := databaseForProtection("dpg-orders", "orders", true)
	svc, _, prot := protectedPostgresService(db)

	acts, err := svc.ActionCapabilities(context.Background(), db.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if s := actionByID(t, acts, core.ActionSuspend); s.Outcome != core.DecisionAllowed || s.Precondition != core.PrecondProtectedConfirmation {
		t.Fatalf("protected suspend = %+v, want allowed + protected_confirmation_required", s)
	}
	if r := actionByID(t, acts, core.ActionRestart); r.Precondition != "" {
		t.Fatalf("restart is not protection-guarded, got %q", r.Precondition)
	}
	// The execute path enforces exactly what was projected.
	if _, err := svc.Suspend(context.Background(), db.Name); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unconfirmed Suspend on a protected database = %v, want ErrBadRequest", err)
	}

	// Unprotected database: the precondition clears.
	plain := databaseForProtection("dpg-scratch", "scratch", false)
	svc2, _, _ := protectedPostgresService(plain)
	acts2, err := svc2.ActionCapabilities(context.Background(), plain.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities (unprotected): %v", err)
	}
	if s := actionByID(t, acts2, core.ActionSuspend); s.Precondition != "" {
		t.Fatalf("unprotected suspend precondition = %q, want none", s.Precondition)
	}

	// A viewer is denied without preconditions — and without even reading the
	// protection store (readiness detail on a forbidden action would leak).
	svc3, _, prot3 := protectedPostgresService(databaseForProtection("dpg-locked", "locked", true))
	svc3.Base.Authz = operateOnlyDenier{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer-1", Method: "session"})
	acts3, err := svc3.ActionCapabilities(ctx, "dpg-locked")
	if err != nil {
		t.Fatalf("viewer ActionCapabilities: %v", err)
	}
	for _, id := range []string{core.ActionRestart, core.ActionSuspend, core.ActionResume} {
		a := actionByID(t, acts3, id)
		if a.Outcome != core.DecisionDenied || a.Reason != core.ReasonInsufficientPermission || a.Precondition != "" {
			t.Errorf("viewer %s = %+v, want denied/insufficient_permission with no precondition", id, a)
		}
	}
	if prot3.calls != 0 {
		t.Errorf("viewer projection read the protection store %d times — a denied caller must not pay or observe guard reads", prot3.calls)
	}
	_ = prot
}
