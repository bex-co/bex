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
	"reflect"
	"strings"
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

// roleRelations answers a check with whether the role holds the relation, as
// ADR012's matrix grants it.
type roleRelations map[string]bool

func (r roleRelations) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return r[relation], nil
}

// TestActionCapabilities_RollbackPerRoleAgreesWithTheVerb (w9/m622): Render's
// role table gives "Trigger rollbacks" to an Admin, a Developer and a
// Contributor and denies it a Viewer and a Billing member. For each role the
// projection's rollback decision is the verb's own answer, a refused rollback
// writes nothing, and the deploy decision answers alike. Before w9/m622 both
// read can_create, so a Contributor was shown no Rollback and refused it.
func TestActionCapabilities_RollbackPerRoleAgreesWithTheVerb(t *testing.T) {
	roles := []struct {
		name string
		rels []string
		may  bool
	}{
		{"developer", []string{core.RelCanView, core.RelCanViewActivity, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate}, true},
		{"contributor", []string{core.RelCanView, core.RelCanViewActivity, core.RelCanViewLogs, core.RelCanOperate}, true},
		{"viewer", []string{core.RelCanView, core.RelCanViewActivity}, false},
		{"billing", []string{core.RelCanView, core.RelCanViewBilling}, false},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			ds := newFakeStore()
			now := fixedNow()
			ds.byApp["srv-1"] = []store.Deploy{{
				ID: "dep-prior", AppID: "srv-1", Status: store.DeployDeactivated, ResolvedImage: "web@sha256:" + strings.Repeat("a", 64),
				Generation: 1, CreatedAt: now, UpdatedAt: now, FinishedAt: &now,
			}}
			svc, cl := newService(ds, sampleApp("web", "srv-1"))
			grants := roleRelations{}
			for _, rel := range role.rels {
				grants[rel] = true
			}
			svc.Base.Authz = grants
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: role.name, Method: "session"})
			before := getApp(t, cl, "web").Spec

			acts, err := svc.ActionCapabilities(ctx, "web")
			if err != nil {
				t.Fatalf("ActionCapabilities: %v", err)
			}
			for _, action := range []string{core.ActionDeploy, core.ActionRollback} {
				got := actionByID(t, acts, action)
				if allowed := got.Outcome == core.DecisionAllowed; allowed != role.may || (allowed && got.Precondition != "") {
					t.Errorf("%s's %s = %+v, want allowed=%t and ready", role.name, action, got, role.may)
				}
			}

			_, err = svc.Rollback(ctx, "web", "dep-prior")
			if role.may {
				if err != nil {
					t.Fatalf("%s's rollback = %v, want accepted", role.name, err)
				}
				return
			}
			if !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("%s's rollback = %v, want ErrForbidden", role.name, err)
			}
			if _, err := svc.Trigger(ctx, "web", TriggerParams{ImageURL: "web:v2"}); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("%s's deploy of another tag = %v, want ErrForbidden", role.name, err)
			}
			if len(ds.byApp["srv-1"]) != 1 {
				t.Errorf("a refused rollback or deploy opened a deploy: %+v", ds.byApp["srv-1"])
			}
			if after := getApp(t, cl, "web").Spec; !reflect.DeepEqual(after, before) {
				t.Errorf("a refused rollback or deploy changed the service:\n  before %+v\n  after  %+v", before, after)
			}
		})
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

	// A paid plan saved on a service the Free instance hours suspended is
	// deployable: the deploy lifts the suspension (w6/m609). Rollback is not.
	suspended.Spec.Tier = "starter"
	suspended.Annotations = map[string]string{core.AnnotFreeHoursSuspended: "2026-11"}
	suspended.Status.ServedTier = "free"
	svc, _ = newService(ds, suspended)
	acts, err = svc.ActionCapabilities(ctx, "asleep")
	if err != nil {
		t.Fatalf("ActionCapabilities(saved paid plan): %v", err)
	}
	if d := actionByID(t, acts, core.ActionDeploy); d.Precondition != "" {
		t.Fatalf("deploy of a saved paid plan over the Free hours suspension = %+v, want ready", d)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondSuspended {
		t.Fatalf("rollback with a saved paid plan = %+v, want suspended", r)
	}
	svc, _ = newService(ds, sampleApp("bare", ""))

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

// TestActionCapabilities_FreshServiceCannotRollBack is w4/110, the case the
// shared predicate missed: on a service whose only deploy is its live first
// one, the projection said `rollback: allowed` with no precondition while
// every possible target answered 409. The precondition vocabulary already had
// the right word for it — cancel_deploy has used `no_active_deploy` for
// exactly this "permitted, not applicable right now" shape all along.
func TestActionCapabilities_FreshServiceCannotRollBack(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	svc, _ := newService(ds, app)
	ctx := context.Background()

	// The live first deploy, running exactly the image the service is on —
	// the state a service is in the moment it finishes creating.
	now := fixedNow()
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-first", AppID: "srv-1", Status: store.DeployLive,
		ResolvedImage: app.Spec.Image,
		CreatedAt:     now, UpdatedAt: now, FinishedAt: &now,
	}}

	acts, err := svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondNoEligibleRollbackTarget {
		t.Fatalf("rollback on a fresh service = %+v, want no_eligible_rollback_target", r)
	}
	// And the projection is right: the verb refuses the only candidate.
	if _, err := svc.Rollback(ctx, "web", "dep-first"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Rollback to the live running deploy = %v, want ErrConflict", err)
	}

	// The narrowness matters: once spec.image drifts off the still-live
	// deploy — what a failed rollout leaves behind — rolling back to it is a
	// legitimate recovery, and both sides must say so again.
	app.Spec.Image = "web:v2-failed"
	if err := svc.Client.Update(ctx, app); err != nil {
		t.Fatalf("update app: %v", err)
	}
	acts, err = svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != "" {
		t.Fatalf("rollback after the image drifted = %+v, want ready", r)
	}
	if _, err := svc.Rollback(ctx, "web", "dep-first"); err != nil {
		t.Fatalf("Rollback as recovery: %v", err)
	}
}
