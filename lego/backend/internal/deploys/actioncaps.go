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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// RollbackEligible reports whether a deploy row is a valid rollback target —
// the exact predicate Rollback enforces: only a deploy that itself reached
// live (or was deactivated from live) carries a ResolvedImage trustworthy
// enough to restore blind. Shared by the verb and the capability projection
// (ADR087, w6/m136) so the projection cannot drift from enforcement.
func RollbackEligible(d store.Deploy) bool {
	return (d.Status == store.DeployLive || d.Status == store.DeployDeactivated) && d.ResolvedImage != ""
}

// eligibilityScanLimit bounds the projection's deploy-history scan. The verb
// validates the exact NAMED target; the projection only answers "is there
// anything to act on", so a recent-history page is the honest cost bound — a
// target older than this reads as no_eligible_rollback_target, matching what
// a client paging recent history would offer anyway.
const eligibilityScanLimit = 20

// ActionCapabilities projects the deploy verbs' decisions for ONE service in
// the acting workspace (ADR087, w6/m136): bare deploy and cancel are
// lifecycle (can_operate); rollback selects executable content and stays
// create-like (can_create — the m68/roleladder pin). Preconditions come from
// the same predicates the verbs enforce: Trigger/Rollback share the
// suspended + billing gates, Cancel needs an open deploy, Rollback an
// eligible target (RollbackEligible). Read-only and non-auditing beyond its
// own can_view gate; the verbs re-run every guard at dispatch.
func (s *Service) ActionCapabilities(ctx context.Context, service string) ([]core.ActionDecision, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, service)
	if err != nil {
		return nil, err
	}
	if err := core.NotFoundIfDeleting(a); err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, a.Labels); err != nil {
		return nil, err
	}
	// The binding above proved the resource lives in the acting workspace —
	// probe its own workspace object directly. Preconditions are computed
	// only for an allowed decision: a viewer's projection pays no deploy-row
	// scan or billing read for actions it will never enable.
	object := core.WorkspaceObject(s.WorkspaceOrDefault(ctx))
	operate := s.CanDecisionOn(ctx, core.RelCanOperate, object)
	create := s.CanDecisionOn(ctx, core.RelCanCreate, object)
	deployPre, cancelPre, rollbackPre := "", "", ""
	if operate.Allowed() || create.Allowed() {
		deployPre, cancelPre, rollbackPre = s.deployPreconditions(ctx, a)
	}
	return []core.ActionDecision{
		core.DecideAction(core.ActionDeploy, operate, deployPre),
		core.DecideAction(core.ActionCancelDeploy, operate, cancelPre),
		core.DecideAction(core.ActionRollback, create, rollbackPre),
	}, nil
}

// deployPreconditions computes the three verbs' blocking conditions in one
// pass. Guard order mirrors the verbs: store-less / unmanaged services have
// no deploy machinery at all (unavailable, like ErrDeploysUnavailable);
// Trigger and Rollback refuse a suspended service and share the billing gate
// (one shared gate precondition — the two ladders are identical until
// rollback's eligibility scan); Cancel is exempt from both (canceling must
// stay possible under dunning).
func (s *Service) deployPreconditions(ctx context.Context, a *appv1alpha1.App) (deployPre, cancelPre, rollbackPre string) {
	if s.Store == nil || appStoreID(a) == "" {
		return core.PrecondUnavailable, core.PrecondUnavailable, core.PrecondUnavailable
	}
	gatePre := "" // suspended, else billing — shared by deploy and rollback
	if a.Spec.Suspended {
		gatePre = core.PrecondSuspended
	} else {
		gatePre = s.BillingPreconditionFor(ctx, a.Labels[core.LabelTenant])
	}
	deployPre, rollbackPre = gatePre, gatePre
	rows, err := s.Store.ListDeploys(ctx, appStoreID(a), store.DeployFilter{Limit: eligibilityScanLimit})
	if err != nil {
		cancelPre = core.PrecondUnavailable
		if rollbackPre == "" {
			rollbackPre = core.PrecondUnavailable
		}
		return deployPre, cancelPre, rollbackPre
	}
	cancelPre = core.PrecondNoActiveDeploy
	for _, d := range rows {
		if d.FinishedAt == nil {
			cancelPre = ""
			break
		}
	}
	if rollbackPre == "" {
		rollbackPre = core.PrecondNoEligibleRollbackTarget
		for _, d := range rows {
			if RollbackEligible(d) {
				rollbackPre = ""
				break
			}
		}
	}
	return deployPre, cancelPre, rollbackPre
}
