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
	"strings"
)

// Workspace plans — Render's post-2026-04-23 flat-rate lineup (verified in
// .pm/w6/RESEARCH-workspaces.md): Hobby is free and single-member, the paid
// plans lift the member/service caps. This is the WORKSPACE plan (tenants.plan),
// deliberately separate from the per-service compute ladder in lego/types/tiers
// (apps.tier) — a workspace on any plan runs services of any instance size.
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

// PlanLimits are the caps a plan imposes. A zero field means "unlimited" — the
// paid plans lift Hobby's limits entirely (Render's Pro/Scale/Enterprise all
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
}

// LimitsFor returns the caps for a plan. Unknown plans get the (unlimited) paid
// shape rather than Hobby's caps — validation rejects unknown plans upstream, so
// this is only a safe default, never a silent downgrade.
func LimitsFor(plan string) PlanLimits {
	if plan == PlanHobby {
		return PlanLimits{MaxServices: 25, MaxMembers: 1, MaxWorkspacesPerUser: 5}
	}
	return PlanLimits{}
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
