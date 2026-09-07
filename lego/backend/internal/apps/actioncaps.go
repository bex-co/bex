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
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ActionCapabilities projects the apps-owned lifecycle verbs' decisions for
// ONE service in the acting workspace (ADR087, w6/m136): per action, the
// tri-state permission (core.CanDecision — the exact relation the verb gates
// on, pinned by api/roleladder_test.go) plus the bounded precondition its
// execute path would enforce, computed by the same predicate the verb calls
// (appProtected, RequireBillingMutation, pendingCronRun). Read-only and
// non-auditing: can_view gates the projection itself; a snapshot is never a
// bearer grant — the verb still runs its own guards at dispatch.
//
// The target binds to the ACTING workspace (RequireResourceInActingWorkspace):
// a service owned elsewhere reads as absent here even when the caller could
// reach it there, so a client cannot render workspace A's screen from
// workspace B's resource. Cron actions appear only on a cron_job — an absent
// action means "does not exist for this type", never "forbidden".
func (s *Service) ActionCapabilities(ctx context.Context, name string) ([]core.ActionDecision, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	if err := core.NotFoundIfDeleting(a); err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, a.Labels); err != nil {
		return nil, err
	}
	// The binding above proved the resource lives in the acting workspace, so
	// its own workspace object IS the acting one — probe it directly rather
	// than re-resolving per relation. Preconditions are computed only for an
	// allowed decision (DecideAction drops them otherwise, and a denied
	// caller's projection must not pay store reads — or leak protection
	// state — for buttons it will never enable), and the billing gate runs at
	// most once per request however many actions consult it.
	operate := s.CanDecisionOn(ctx, core.RelCanOperate, core.WorkspaceObject(s.WorkspaceOrDefault(ctx)))
	billing := sync.OnceValue(func() string { return s.BillingPreconditionFor(ctx, a.Labels[core.LabelTenant]) })
	precond := func(compute func() string) string {
		if !operate.Allowed() {
			return ""
		}
		return compute()
	}
	out := []core.ActionDecision{
		core.DecideAction(core.ActionRestart, operate, ""),
		core.DecideAction(core.ActionSuspend, operate, precond(func() string { return s.suspendPrecondition(ctx, a) })),
		core.DecideAction(core.ActionResume, operate, precond(billing)),
	}
	if a.Spec.Type == appv1alpha1.TypeCronJob {
		out = append(out,
			core.DecideAction(core.ActionCronRunNow, operate, precond(func() string {
				if a.Spec.Suspended { // TriggerCronRun's guard order: state, then billing
					return core.PrecondSuspended
				}
				return billing()
			})),
			core.DecideAction(core.ActionCronCancelRun, operate, precond(func() string {
				if _, ok := pendingCronRun(a); !ok { // CancelCurrentCronRun's scan
					return core.PrecondNoActiveRun
				}
				return ""
			})),
		)
	}
	return out, nil
}

// suspendPrecondition mirrors setSuspended(true)'s guard: the
// protected-environment confirmation, by the same appProtected predicate
// requireUnprotected consults and the same classification the datastore
// projections use.
func (s *Service) suspendPrecondition(ctx context.Context, a *appv1alpha1.App) string {
	return core.ProtectionPrecondition(s.appProtected(ctx, a))
}
