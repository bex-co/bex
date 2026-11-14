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
)

// The workspace inbound-IP layer (w9/m388, docs/ADR032-environments.md §
// Workspace layer). Render's Scale and Enterprise workspaces set rules for an
// entire workspace on top of environment and service rules, and "an inbound IP
// must be allowed by each level's rules". bex stores a workspace's rules in
// its own row (workspace_ip_allow_lists) and projects them, CIDRs only, onto
// spec.workspaceIPAllowList of every eligible App, Database and KeyValue in the
// workspace, where the operator chains a third middleware (HTTP) or a third
// SNI check (Postgres, Key Value).
//
// A workspace with no row never set rules: no layer is projected, so nothing
// about an existing workspace changes, and reads show Render's seeded
// allow-all (DefaultEnvironmentAllowList). A saved empty list is deny-all, as
// on Render and for environments: it projects the DenyAllCIDR placeholder.

// WorkspaceIPRulesReader reads a workspace's stored inbound-IP rules. set is
// false when the workspace never saved any (no row). The composition root
// wires it from the control-plane store; nil (store off) means no workspace
// ever has a layer.
type WorkspaceIPRulesReader interface {
	WorkspaceIPAllowList(ctx context.Context, workspaceID string) (entries []IPAllowListEntry, set bool, err error)
}

// ReadWorkspaceIPLayer reads workspaceID's rules through rules and projects
// them onto the flat layer member CRs carry: nil when the workspace never set
// rules (no layer), else EnvironmentLayerCIDRs (an explicitly empty list
// projects the deny-all placeholder). A nil reader or an unscoped workspace
// has no layer. The control-plane projector calls it directly; feature
// services go through Base.WorkspaceIPLayer.
func ReadWorkspaceIPLayer(ctx context.Context, rules WorkspaceIPRulesReader, workspaceID string) ([]string, error) {
	if rules == nil || workspaceID == "" || workspaceID == DefaultTenant {
		return nil, nil
	}
	entries, set, err := rules.WorkspaceIPAllowList(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("reading workspace inbound IP rules: %w", err)
	}
	if !set {
		return nil, nil
	}
	return EnvironmentLayerCIDRs(entries), nil
}

// WorkspaceIPLayer is the workspace layer a resource created in workspaceID is
// born with. Every create path — services and static sites, Blueprint
// applies, Postgres create and recovery, Key Value create — stamps it where it
// stamps the environment layer, so a new resource never starts outside its
// workspace's rules. A context carrying WithWorkspaceIPLayer for the same
// workspace answers from it.
func (b *Base) WorkspaceIPLayer(ctx context.Context, workspaceID string) ([]string, error) {
	if memo, ok := ctx.Value(workspaceIPLayerKey{}).(workspaceIPLayerMemo); ok && memo.workspaceID == workspaceID {
		return memo.layer, nil
	}
	if b == nil {
		return nil, nil
	}
	return ReadWorkspaceIPLayer(ctx, b.WorkspaceIPRules, workspaceID)
}

type workspaceIPLayerKey struct{}

type workspaceIPLayerMemo struct {
	workspaceID string
	layer       []string
}

// WithWorkspaceIPLayer reads workspaceID's layer once and carries it on ctx,
// for a run of creates in one workspace (a Blueprint apply or preview) that
// would otherwise each read it.
func (b *Base) WithWorkspaceIPLayer(ctx context.Context, workspaceID string) (context.Context, error) {
	layer, err := b.WorkspaceIPLayer(ctx, workspaceID)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, workspaceIPLayerKey{}, workspaceIPLayerMemo{workspaceID: workspaceID, layer: layer}), nil
}
