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

package workspaces

import (
	"context"
	"slices"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// plan_blockers.go answers what keeps a workspace from moving to a plan
// before an admin selects it (w3/m708). Render's plan page
// (BillingUpdatePlan, docs/render-artifacts/workspace-plan-change.md) names
// each blocker under "Plan selection errors" with its fix link and keeps
// Select plan disabled until none is left. Every answer is one of
// ChangePlan's refusals, decided by its own predicates (downgradePreconditions,
// seatsExceeded, perUserCapReached, rolesOutsidePlan); the tests hold the two
// to the same verdicts.

// PlanChangeBlockers lists the refusals ChangePlan would give workspace id
// for plan, all at once: on an org's primary, the org's downgrade
// preconditions and then the plan's capacity, each kind once. Empty when the
// change would go through those checks: an upgrade, the same plan, an org
// plan on an org's primary. Billing (a payment method, dunning) and the
// Enterprise entitlement are not listed: the plan page reads the payment
// method itself, and Enterprise is Render's Get in touch, never selected.
// Admin only, as ChangePlan is.
func (s *Service) PlanChangeBlockers(ctx context.Context, id, plan string) ([]core.DowngradePrecondition, error) {
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, id)
	if err != nil {
		return nil, err
	}
	t, plan, err := s.planChangeTenant(ctx, id, plan)
	if err != nil {
		return nil, err
	}
	blockers := []core.DowngradePrecondition{}
	if t.Plan == plan {
		return blockers, nil
	}
	caller, err := s.planCallerSubject(ctx)
	if err != nil {
		return nil, err
	}
	if orgs, ok := s.organizations(); ok && t.OrganizationID != "" {
		o, err := orgs.GetOrganization(ctx, t.OrganizationID)
		if err != nil {
			return nil, mapStoreErr(err)
		}
		if t.ID != o.PrimaryWorkspaceID {
			return nil, core.NewOrganizationPlanOnPrimaryError(o.ID, o.PrimaryWorkspaceID)
		}
		if core.OrganizationPlan(plan) {
			return blockers, nil
		}
		workspaces, err := orgs.ListOrganizationWorkspaces(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		unmet, err := s.downgradePreconditions(ctx, orgs, o, workspaces, plan, caller)
		if err != nil {
			return nil, err
		}
		blockers = append(blockers, unmet...)
	}
	capacity, err := s.planCapacityBlockers(ctx, id, plan, caller, blockers)
	if err != nil {
		return nil, err
	}
	return append(blockers, capacity...), nil
}

// planCapacityBlockers is guardPlanCapacity's checks, each one reported
// instead of the first: the people beyond the plan's seats (named as the org
// refusal names them, unseated's order), the services beyond its cap, the
// caller's workspaces of that plan at its cap, and the members and pending
// invites holding a role the plan does not offer. Roles are judged among the
// people who keep a seat, since clearing the seats first is what the guard
// asks for: a Hobby target lists the people over its one seat, not their
// roles too. A kind already in found (an org primary's preconditions) is not
// checked again, so its people are never looked up twice.
func (s *Service) planCapacityBlockers(ctx context.Context, id, plan, caller string, found []core.DowngradePrecondition) ([]core.DowngradePrecondition, error) {
	checks := func(kind string) bool {
		return !slices.ContainsFunc(found, func(b core.DowngradePrecondition) bool { return b.Kind == kind })
	}
	members, err := s.Store.ListTenantMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	invites, err := s.Store.ListInvites(ctx, id)
	if err != nil {
		return nil, err
	}
	limits := store.LimitsFor(plan)
	var blockers []core.DowngradePrecondition
	if seatsExceeded(limits, members, invites) {
		over, overInvites := unseated(members, invites, limits.MaxMembers, caller)
		if checks(core.PreconditionMemberLimit) {
			blockers = append(blockers, core.DowngradePrecondition{
				Kind:  core.PreconditionMemberLimit,
				Items: append(s.memberRoles(ctx, over), invitedRoles(overInvites)...),
				Limit: limits.MaxMembers,
			})
		}
		members = slices.DeleteFunc(slices.Clone(members), func(m store.TenantMember) bool {
			return slices.ContainsFunc(over, func(o store.TenantMember) bool { return o.Subject == m.Subject })
		})
		invites = slices.DeleteFunc(slices.Clone(invites), func(inv store.Invite) bool {
			return slices.ContainsFunc(overInvites, func(o store.Invite) bool { return o.Email == inv.Email })
		})
	}
	_, exceeded, err := s.servicesExceeded(ctx, id, limits)
	if err != nil {
		return nil, err
	}
	if exceeded {
		blockers = append(blockers, core.DowngradePrecondition{Kind: core.PreconditionServiceLimit, Items: []string{}, Limit: limits.MaxServices})
	}
	if caller != "" {
		limit, reached, err := s.perUserCapReached(ctx, caller, plan)
		if err != nil {
			return nil, err
		}
		if reached {
			blockers = append(blockers, core.DowngradePrecondition{Kind: core.PreconditionWorkspaceLimit, Items: []string{}, Limit: limit})
		}
	}
	if blocked := rolesOutsidePlan(members, plan); len(blocked) > 0 && checks(core.PreconditionMemberRoles) {
		blockers = append(blockers, core.DowngradePrecondition{Kind: core.PreconditionMemberRoles, Items: s.memberRoles(ctx, blocked)})
	}
	if blocked := invitesOutsidePlan(invites, plan); len(blocked) > 0 && checks(core.PreconditionInviteRoles) {
		blockers = append(blockers, core.DowngradePrecondition{Kind: core.PreconditionInviteRoles, Items: invitedRoles(blocked)})
	}
	return blockers, nil
}

var planChangeBlockerGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "WorkspacePlanChangeBlocker",
	Description: "One refusal changeWorkspacePlan would give for a plan (w3/m708): Render's plan page names each under Plan selection errors before Select plan.",
	Fields: graphql.Fields{
		"kind": gqlutil.ReqStrField(func(b core.DowngradePrecondition) any { return b.Kind }),
		"items": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			func(b core.DowngradePrecondition) any { return b.Items }),
		"limit": gqlutil.Typed(graphql.Int, func(b core.DowngradePrecondition) any {
			if b.Limit == 0 {
				return nil
			}
			return b.Limit
		}),
	},
})

// planChangeBlockersField is the workspacePlanChangeBlockers query: admin
// only and GraphQL-only, like changeWorkspacePlan.
func (s *Service) planChangeBlockersField() *graphql.Field {
	return &graphql.Field{
		Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(planChangeBlockerGQLType))),
		Description: "What would keep changeWorkspacePlan from moving the workspace to plan, by its own checks; empty when nothing would (admin only).",
		Args: graphql.FieldConfigArgument{
			"id":   gqlutil.ReqArg(graphql.String),
			"plan": gqlutil.ReqArg(graphql.String),
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.PlanChangeBlockers(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
		},
	}
}
