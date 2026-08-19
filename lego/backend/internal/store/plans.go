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

package store

import (
	"fmt"
	"slices"
	"strings"
)

// Workspace plans mirror Render's post-2026-04-23 capability lineup (verified
// in .pm/w6/RESEARCH-workspaces.md): Hobby is capped, while higher plans lift
// member/service caps and add roles. Monthly workspace fees live in
// lego/backend/internal/pricing (ADR030): Render's flat subscriptions × 0.70
// (Hobby $0, Pro $17.50, Scale $349.30; Enterprise custom). tenants.plan is
// still the capability vehicle; resource-tier usage is billed separately
// (ADR040, ADR046).
const (
	PlanHobby      = "hobby"
	PlanPro        = "pro"
	PlanScale      = "scale"
	PlanEnterprise = "enterprise"
)

// DefaultPlan is the plan a new workspace gets when the caller names none — the
// free tier, matching Render's create flow default.
const DefaultPlan = PlanHobby

// WorkspacePlans is the catalog in ladder order (cheapest first), the source of
// truth for validation and any plan picker.
var WorkspacePlans = []string{PlanHobby, PlanPro, PlanScale, PlanEnterprise}

// PlanLimits are the caps and capabilities a plan grants — the one place a
// plan's full shape is defined. A zero numeric field means "unlimited" — the
// paid plans lift Hobby's caps entirely (Render's Pro/Scale/Enterprise all
// allow unlimited members and services), so only Hobby carries non-zero caps.
type PlanLimits struct {
	// MaxServices caps services per workspace, counting suspended ones (Render:
	// "Up to 25 services, all service types, including suspended"). 0 = unlimited.
	MaxServices int
	// MaxMembers caps members per workspace. Hobby is single-member (1); paid
	// plans are unlimited (0).
	MaxMembers int
	// MaxWorkspacesPerUser caps how many workspaces of this plan one user may
	// own. Render allows five free Hobby workspaces per user and unlimited paid
	// ones, so only Hobby sets this (5). 0 = unlimited.
	MaxWorkspacesPerUser int
	// AllowedRoles is the role set assignable on the plan — Render's plan-gated
	// role catalog (RESEARCH-workspaces.md finding 5, docs/render-artifacts/team-members.graphql):
	// Hobby is single-member (no invites, the sole member is always admin); Pro
	// adds Developer; Scale and Enterprise add Contributor, Viewer, and Billing.
	// Lowercase (the stored/FGA form), matching members.Roles.
	AllowedRoles []string
}

// LimitsFor returns the caps and capabilities for a plan. Unknown plans get
// the (unlimited, full-role) paid shape rather than Hobby's — validation
// rejects unknown plans upstream, so this is only a safe default, never a
// silent downgrade.
func LimitsFor(plan string) PlanLimits {
	switch plan {
	case PlanHobby:
		return PlanLimits{MaxServices: 25, MaxMembers: 1, MaxWorkspacesPerUser: 5, AllowedRoles: []string{"admin"}}
	case PlanPro:
		return PlanLimits{AllowedRoles: []string{"admin", "developer"}}
	default: // scale, enterprise, and any future higher tier
		return PlanLimits{AllowedRoles: []string{"admin", "developer", "contributor", "viewer", "billing"}}
	}
}

// QuotaCaps are a plan's per-workspace object-count ceilings for
// Services/Postgres/KeyValues — the same numbers the per-namespace
// ResourceQuota (quotaForPlan, namespaces.go) enforces at the API server in
// place of the retired BEX_MAX_SERVICES/_POSTGRES/_KEYVALUES app-code caps
// (ADR043 D3). One source of truth so enforcement and the "3/5 services"
// display surface (workspaces.Service.ResourceLimits) can't drift apart.
type QuotaCaps struct {
	Services  int64
	Postgres  int64
	KeyValues int64
}

// QuotaCapsForPlan returns plan's object-count ceilings. Matches on PlanHobby
// (plus "" and "free" for legacy/test rows created before plan normalization)
// so every real Hobby workspace — whose stored plan is always "hobby" via
// NormalizePlan — gets the intended Render-Hobby-anchored ceiling rather than
// silently falling through to the generous paid default. Services reuses
// LimitsFor(PlanHobby).MaxServices — the same Render-Hobby-anchor number —
// rather than a second hardcoded literal, so the two catalogs can't drift.
func QuotaCapsForPlan(plan string) QuotaCaps {
	switch plan {
	case PlanHobby, "", "free":
		return QuotaCaps{Services: int64(LimitsFor(PlanHobby).MaxServices), Postgres: 1, KeyValues: 1}
	default: // pro, scale, enterprise, and any future higher tier
		return QuotaCaps{Services: 100, Postgres: 25, KeyValues: 25}
	}
}

// RoleAllowedOnPlan reports whether role is assignable on plan — the single
// predicate members.guardPlanRole (invite/change-role) and workspaces.ChangePlan's
// downgrade guard both consult, mirroring CanAddMember's shape below.
func RoleAllowedOnPlan(plan, role string) bool {
	return slices.Contains(LimitsFor(plan).AllowedRoles, role)
}

// CanAddMember reports whether a workspace on the given plan can take another
// member given its current count — the single-member guard keyed on plan (Hobby
// caps at 1, paid plans are unlimited). A pure function so w4/m12's invite verb
// consults the one rule without a second copy, and so it stays out of the
// service-layer authz sweep (it's a predicate, not a gated verb).
func CanAddMember(plan string, currentMembers int) bool {
	limit := LimitsFor(plan).MaxMembers
	return limit == 0 || currentMembers < limit
}

// NormalizePlan validates a caller-supplied workspace plan, defaulting empty to
// Hobby. Unknown plans are ErrInvalid, listing the valid ones — the same shape
// as the internal API's tier validation.
func NormalizePlan(plan string) (string, error) {
	if plan == "" {
		return DefaultPlan, nil
	}
	for _, p := range WorkspacePlans {
		if p == plan {
			return plan, nil
		}
	}
	return "", fmt.Errorf("%w: plan must be one of %s", ErrInvalid, strings.Join(WorkspacePlans, "|"))
}
