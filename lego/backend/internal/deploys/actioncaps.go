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
	"slices"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// RollbackEligible reports whether a deploy row is a valid rollback target —
// the exact predicate Rollback enforces: only a deploy that itself reached
// live (or was deactivated from live) carries an artifact trustworthy enough
// to restore blind. Shared by the verb and the capability projection (ADR087,
// w6/m136) so the projection cannot drift from enforcement.
//
// The artifact is the deploy's ResolvedImage, or, for a static site published
// straight from its repository with no build image (ADR029), the commit it
// published (directStaticTarget). Before w4/m141 no such static deploy was
// ever eligible, so rollback on a no-build static site could never happen.
// A static target is restored from its retained publication (w4/m197), which
// the verb and the projection additionally require (Service.restorableRollbackTarget).
func RollbackEligible(a *appv1alpha1.App, d store.Deploy) bool {
	if d.Status != store.DeployLive && d.Status != store.DeployDeactivated {
		return false
	}
	return d.ResolvedImage != "" || directStaticTarget(a, d)
}

// directStaticTarget reports whether d is a static site's publish straight
// from its repository: no build image, only the commit it published.
func directStaticTarget(a *appv1alpha1.App, d store.Deploy) bool {
	return d.ResolvedImage == "" && d.Commit != "" &&
		a.Spec.Type == appv1alpha1.TypeStaticSite && a.Spec.Repo != ""
}

// RollbackActionable is RollbackEligible plus the one refusal eligibility does
// not cover: Rollback rejects the currently-live deploy when it is already what
// is running (the same image, or for a re-published static site the same
// commit), because that changes nothing while still restarting the service and
// minting a redundant deploy (w4/051).
//
// The projection used RollbackEligible alone, so a freshly created service —
// whose only deploy is its live first one — was told `rollback: allowed` with
// no precondition, and every possible target answered 409 (w4/110). The
// dashboard was not misled because it gates the control a second way; the
// exposure was to a client binding an affordance straight to the capability
// response, which is what that query is for.
//
// "Already what is running" is decided by release, never by how an image
// reference is spelled (w8/261): a live deploy's ResolvedImage is the
// digest-pinned reference the operator ran, while the App holds the caller's
// spelling (a tag in spec.image, the string an image override sent).
//
//   - With a selected runtime, the selection names the release: the live deploy
//     of that generation is the one running. Saved settings intentionally
//     differ after a rollback, so they are not compared.
//   - Without one, the live deploy is still a recovery when a newer release
//     was asked for and is not live (a failed configuration-only release), or
//     when the saved image has moved off the one this deploy was created from
//     (a failed release left spec.image drifted).
//   - A row from before deploys recorded their release (generation 0) cannot
//     be judged that way and keeps the comparison of references it always had.
func RollbackActionable(a *appv1alpha1.App, d store.Deploy) bool {
	if !RollbackEligible(a, d) {
		return false
	}
	if d.Status != store.DeployLive {
		return true
	}
	if selected := a.ActiveReleaseConfig(); selected != nil {
		return d.Generation != selected.Generation
	}
	if directStaticTarget(a, d) {
		return d.Commit != a.Spec.BuildCommit
	}
	if d.Generation <= 0 {
		return d.ResolvedImage != a.Spec.Image
	}
	if requested, _ := strconv.ParseInt(a.Annotations[appv1alpha1.AnnotationReleaseGeneration], 10, 64); requested > d.Generation {
		return true
	}
	// d.Image is the saved image the deploy was created from; a repository
	// build is created from an empty one and keeps it.
	return a.Spec.Image != d.Image && a.Spec.Image != d.ResolvedImage
}

// BuildStored reports whether the build deploy d ran on can still be pulled
// (w8/m156). Render: "You can only roll back to a particular deploy if its
// build artifact is still available." The in-cluster registry keeps a bounded
// history, and the operator publishes what it holds for the App's own
// repository (status.storedImages); a build that is not listed is gone, and a
// rollback to it used to be accepted and then fail at the pull minutes later.
//
// Everything the listing does not cover counts as stored, so nothing is
// refused on a guess:
//   - no listing yet;
//   - a deploy made after the listing was read (its generation is newer), or
//     with no generation to compare: a reading says nothing about later builds,
//     so it stays safe however stale it is;
//   - the build that is serving right now, whatever the listing says;
//   - an image the registry was never asked to keep (an external image,
//     another repository's build source);
//   - a deploy with no image of its own, or one recorded by tag alone, which
//     names no manifest to look for.
func BuildStored(a *appv1alpha1.App, d store.Deploy) bool {
	stored := a.Status.StoredImages
	if stored == nil || !strings.Contains(d.ResolvedImage, "@") {
		return true
	}
	if d.Generation <= 0 || d.Generation > stored.Generation {
		return true
	}
	if serving := a.Status.ImageDigest; serving != nil && strings.HasSuffix(d.ResolvedImage, "@"+serving.Digest) {
		return true
	}
	theirs, err := store.ImageRepository(d.ResolvedImage)
	if err != nil {
		return true
	}
	if own, err := store.ImageRepository(a.Status.Image); err != nil || own != theirs {
		return true
	}
	return stored.HoldsImage(d.ResolvedImage)
}

// freeRollbackDepth is how far back a Free web service may roll. Render, Free
// instances: "Rollbacks (only to the two most recent previous deploys)", and
// "Upgrading your workspace plan does not remove limitations on Free
// instances" (render.com/docs/free, w8/262).
const freeRollbackDepth = 2

// freeRollbackTargets is the set of deploy ids a Free web service may roll
// back to: its freeRollbackDepth most recent deploys that went live and were
// since replaced. limited is false for every other service, which has no
// depth rule. The deploy that is still live is not a step back and is never
// judged by depth (RollbackActionable decides whether it is a recovery).
func (s *Service) freeRollbackTargets(ctx context.Context, a *appv1alpha1.App) (ids []string, limited bool, err error) {
	if core.PaidPlan(a.Spec.Tier) || (a.Spec.Type != "" && a.Spec.Type != appv1alpha1.TypeWebService) {
		return nil, false, nil
	}
	previous, err := s.Store.ListDeploys(ctx, appStoreID(a), store.DeployFilter{
		Statuses: []string{store.DeployDeactivated}, Limit: freeRollbackDepth,
	})
	if err != nil {
		return nil, true, err
	}
	ids = make([]string, len(previous))
	for i, p := range previous {
		ids[i] = p.ID
	}
	return ids, true, nil
}

// beyondFreeDepth reports whether d is a previous deploy a Free web service
// can no longer roll back to. An unreadable history is not a refusal here:
// the verb surfaces that error itself.
func (s *Service) beyondFreeDepth(ctx context.Context, a *appv1alpha1.App, d store.Deploy) bool {
	if d.Status != store.DeployDeactivated {
		return false
	}
	ids, limited, err := s.freeRollbackTargets(ctx, a)
	return err == nil && limited && !slices.Contains(ids, d.ID)
}

// restorableRollbackTarget is RollbackActionable plus the refusals the verb
// makes before dispatch: a build the registry no longer holds (BuildStored),
// a Free web service's deploy older than its two most recent previous ones
// (beyondFreeDepth), and a static-site target whose publication was never
// recorded, which has nothing retained to restore (w4/m197). Unreadable
// evidence is not a target either; the verb surfaces that error itself.
func (s *Service) restorableRollbackTarget(ctx context.Context, a *appv1alpha1.App, d store.Deploy) bool {
	if !RollbackActionable(a, d) || !BuildStored(a, d) || s.beyondFreeDepth(ctx, a, d) {
		return false
	}
	if a.Spec.Type != appv1alpha1.TypeStaticSite {
		return true
	}
	prefix, err := s.retainedStaticPublication(ctx, a, d)
	return err == nil && prefix != ""
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
			// RollbackActionable, not RollbackEligible: the projection has to
			// answer the question the verb will answer, and the verb also
			// refuses the live deploy when it is the running image (w4/110).
			if s.restorableRollbackTarget(ctx, a, d) {
				rollbackPre = ""
				break
			}
		}
	}
	// A rollback is not one of the actions Render restricts in a protected
	// environment, and none asks for a typed phrase (w8/187), so protection adds
	// no precondition here.
	return deployPre, cancelPre, rollbackPre
}
