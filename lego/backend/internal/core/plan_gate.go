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
)

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
	return plan == "scale" || plan == "enterprise"
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
	if b == nil || b.Plans == nil || workspaceID == "" || workspaceID == DefaultTenant || !RestrictsInbound(next, emptyDeniesAll) {
		return nil
	}
	if slices.Equal(next, current) {
		return nil
	}
	plan, err := b.Plans.WorkspacePlan(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("reading workspace plan: %w", err)
	}
	if InboundIPRulesPlan(plan) {
		return nil
	}
	return fmt.Errorf("%w: ipAllowList: %s", ErrBadRequest, InboundIPRulesMessage)
}
