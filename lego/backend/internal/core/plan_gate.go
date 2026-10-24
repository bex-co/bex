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

package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Workspace plan ids, ladder order. Mirrors store.PlanHobby…PlanEnterprise
// (core cannot import store).
const (
	planHobby      = "hobby"
	planPro        = "pro"
	planScale      = "scale"
	planEnterprise = "enterprise"
)

var planLadder = []string{planHobby, planPro, planScale, planEnterprise}

// PlanFeature is a capability Render gates by workspace plan
// (render.com/docs/platform-features-by-plan, w8/m137, w8/m138).
type PlanFeature string

const (
	FeatureInboundIPRules   PlanFeature = "inbound IP rules"
	FeatureNetworkIsolation PlanFeature = "network-isolated environments"
	FeatureWebhooks         PlanFeature = "webhooks"
	FeatureRequestLogs      PlanFeature = "HTTP request logs"
)

// planFeatureMinimum is the one table of the lowest plan that includes each
// feature. A feature absent from it is open on every plan.
var planFeatureMinimum = map[PlanFeature]string{
	FeatureInboundIPRules:   planScale,
	FeatureNetworkIsolation: planPro,
	FeatureWebhooks:         planPro,
	FeatureRequestLogs:      planPro,
}

// PlanAllows reports whether plan includes feature. An unknown plan is treated
// as allowed (store.LimitsFor's safe default: validation rejects unknown plans
// upstream, so this is never a silent downgrade).
func PlanAllows(plan string, feature PlanFeature) bool {
	minimum, gated := planFeatureMinimum[feature]
	if !gated {
		return true
	}
	have := slices.Index(planLadder, plan)
	return have < 0 || have >= slices.Index(planLadder, minimum)
}

// PlanFeatureMessage is the one refusal sentence for a feature the workspace
// plan does not include.
func PlanFeatureMessage(plan string, feature PlanFeature) string {
	minimum := planFeatureMinimum[feature]
	title := func(p string) string { return strings.ToUpper(p[:1]) + p[1:] }
	return fmt.Sprintf("%s require the %s plan or higher; this workspace is on %s", feature, title(minimum), title(plan))
}

// MaxEnvironmentsPerProject is Render's per-project environment cap for a plan:
// Hobby is limited to two, every paid plan is unlimited (0).
func MaxEnvironmentsPerProject(plan string) int {
	if plan == planHobby {
		return 2
	}
	return 0
}

// WebhookDestinationLimit is how many webhook destinations a plan allows:
// none on Hobby, one on Pro, up to 100 on Scale and above.
func WebhookDestinationLimit(plan string) int {
	switch plan {
	case planHobby:
		return 0
	case planPro:
		return 1
	default:
		return 100
	}
}

// WorkspacePlanReader reads a workspace's plan (hobby, pro, scale, enterprise).
// The composition root wires it from the tenants store; nil leaves every
// plan-gated feature open, the store-off degrade the other Base gates use.
type WorkspacePlanReader interface {
	WorkspacePlan(ctx context.Context, workspaceID string) (string, error)
}

// WorkspacePlanFunc adapts a function to WorkspacePlanReader.
type WorkspacePlanFunc func(ctx context.Context, workspaceID string) (string, error)

func (f WorkspacePlanFunc) WorkspacePlan(ctx context.Context, workspaceID string) (string, error) {
	return f(ctx, workspaceID)
}

// InboundIPRulesMessage is the one sentence every surface uses when a workspace
// below Scale sets inbound IP rules on a service, static site or environment
// (w8/m137). Postgres and Key Value rules are available on every plan.
const InboundIPRulesMessage = "inbound IP rules for web services, static sites and environments require the Scale plan or higher"

// InboundIPRulesPlan reports whether a workspace plan may set inbound IP rules
// on services, static sites and environments (Render: Scale and Enterprise).
func InboundIPRulesPlan(plan string) bool {
	return PlanAllows(plan, FeatureInboundIPRules)
}

// RestrictsInbound reports whether entries restrict who may connect: anything
// beyond the allow-all pair every environment starts with. An empty list
// restricts only where emptiness means deny-all (an environment); on a service
// an empty list is no rule at all.
func RestrictsInbound(entries []IPAllowListEntry, emptyDeniesAll bool) bool {
	if len(entries) == 0 {
		return emptyDeniesAll
	}
	return slices.ContainsFunc(entries, func(e IPAllowListEntry) bool {
		return e.CIDRBlock != "0.0.0.0/0" && e.CIDRBlock != "::/0"
	})
}

// RequireInboundIPRules refuses next below the Scale plan when it restricts
// inbound traffic and differs from current. A rule that already exists keeps
// being enforced and can always be cleared or re-sent unchanged, so a client
// that round-trips a whole settings object never fails on a legacy rule.
func (b *Base) RequireInboundIPRules(ctx context.Context, workspaceID string, next, current []IPAllowListEntry, emptyDeniesAll bool) error {
	if !RestrictsInbound(next, emptyDeniesAll) || slices.Equal(next, current) {
		return nil
	}
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok || InboundIPRulesPlan(plan) {
		return err
	}
	return fmt.Errorf("%w: ipAllowList: %s", ErrBadRequest, InboundIPRulesMessage)
}

// WorkspacePlanOf reads workspaceID's plan. ok is false, with no error, where no
// plan applies (no reader wired, no workspace, the default tenant), so callers
// leave the feature open exactly as the other Base gates degrade.
func (b *Base) WorkspacePlanOf(ctx context.Context, workspaceID string) (plan string, ok bool, err error) {
	if b == nil || b.Plans == nil || workspaceID == "" || workspaceID == DefaultTenant {
		return "", false, nil
	}
	plan, err = b.Plans.WorkspacePlan(ctx, workspaceID)
	if err != nil {
		return "", false, fmt.Errorf("reading workspace plan: %w", err)
	}
	return plan, true, nil
}

// RequirePlanFeature refuses a workspace whose plan does not include feature.
func (b *Base) RequirePlanFeature(ctx context.Context, workspaceID string, feature PlanFeature) error {
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok || PlanAllows(plan, feature) {
		return err
	}
	return fmt.Errorf("%w: %s", ErrBadRequest, PlanFeatureMessage(plan, feature))
}

// RequireNetworkIsolation refuses turning environment network isolation on
// below Pro. Leaving it off, or re-sending the state it already has, is always
// accepted, so a client that round-trips a whole ACL never trips on a legacy
// isolated environment.
func (b *Base) RequireNetworkIsolation(ctx context.Context, workspaceID string, next, current bool) error {
	if !next || current {
		return nil
	}
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok || PlanAllows(plan, FeatureNetworkIsolation) {
		return err
	}
	return fmt.Errorf("%w: networkIsolationEnabled: %s", ErrBadRequest, PlanFeatureMessage(plan, FeatureNetworkIsolation))
}

// RequireEnvironmentCapacity refuses a project's next environment once the
// workspace plan's per-project cap (Hobby: two) is reached. existing is the
// project's current environment count.
func (b *Base) RequireEnvironmentCapacity(ctx context.Context, workspaceID string, existing int) error {
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok {
		return err
	}
	if limit := MaxEnvironmentsPerProject(plan); limit > 0 && existing >= limit {
		return fmt.Errorf("%w: environments: a %s workspace is limited to %d environments per project; upgrade the plan to add more",
			ErrBadRequest, strings.ToUpper(plan[:1])+plan[1:], limit)
	}
	return nil
}
