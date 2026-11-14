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
	"fmt"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Workspace inbound IP rules (w9/m388, docs/ADR032-environments.md §
// Workspace layer): Render's Scale and Enterprise workspaces set rules for an
// entire workspace, which apply to every web service, static site, Postgres
// and Key Value in it on top of environment and service rules. Only Admins
// edit them. Render's REST reads them as owner.ipAllowList and has no owner
// write, and its MCP has no workspace mutations, so the setter is GraphQL and
// dashboard only; the owners REST reads, MCP list_workspaces and the GraphQL
// field read them.

// ipRulesStore is the slice of the store the workspace rules read and write.
// *store.PGStore satisfies it; a store without it reads every workspace as
// unset and refuses writes as unavailable.
type ipRulesStore interface {
	core.WorkspaceIPRulesReader
	WorkspaceIPAllowLists(ctx context.Context, workspaceIDs []string) (map[string][]core.IPAllowListEntry, error)
	SetWorkspaceIPAllowList(ctx context.Context, workspaceID string, entries []core.IPAllowListEntry) error
}

// IPLayerTarget projects the workspace inbound-IP layer onto every resource of
// one datastore kind in a workspace, through the owning feature's own
// authorized write. *postgres.Service and *keyvalue.Service satisfy it, so
// this package imports neither.
type IPLayerTarget interface {
	SetWorkspaceIPAllowList(ctx context.Context, workspaceID string, cidrs []string) error
}

// SetIPAllowList replaces a workspace's inbound IP rules and projects them
// onto every web service, static site, Postgres and Key Value in it. Admin
// only (can_manage on the exact workspace). Below Scale a restricting change
// is refused with PLAN_FEATURE_REQUIRED (core.RequireWorkspaceInboundIPRules);
// re-sending the stored list and resetting to allow-all stay open, so a
// downgraded workspace keeps enforcing its rules and can always lift them.
// An empty list is deny-all. Re-saving the stored list re-runs the
// projection, which is how a change whose projection failed part-way is
// completed.
func (s *Service) SetIPAllowList(ctx context.Context, ownerID string, entries []core.IPAllowListEntry) (WorkspaceView, error) {
	// The one allowed-write row is recorded once the rules are saved; the
	// fan-out's own authorizations record denials only.
	deferred := core.WithDeferredAllowedWriteAudit(ctx)
	ownerID, err := s.AuthorizeWorkspace(deferred, core.RelCanManage, ownerID)
	if err != nil {
		return WorkspaceView{}, err
	}
	st, ok := s.Store.(ipRulesStore)
	if !ok {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	entries = core.ProvidedAllowList(entries)
	if err := core.ValidateAllowList(entries); err != nil {
		return WorkspaceView{}, err
	}
	t, err := s.Store.GetTenant(ctx, ownerID)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	current, set, err := st.WorkspaceIPAllowList(ctx, ownerID)
	if err != nil {
		return WorkspaceView{}, err
	}
	if !set {
		current = core.DefaultEnvironmentAllowList()
	}
	if err := s.RequireWorkspaceInboundIPRules(ctx, ownerID, entries, current); err != nil {
		return WorkspaceView{}, err
	}
	v := view(t, "admin")
	// A workspace that never saved rules and is sent the allow-all it already
	// reads as keeps having no layer: nothing to store or project.
	if !set && slices.Equal(entries, current) {
		v.IPAllowList = current
		return v, nil
	}
	if err := st.SetWorkspaceIPAllowList(ctx, ownerID, entries); err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	s.RecordAllowedWrite(ctx, core.AuditVerbSetWorkspaceIPAllowList, core.RelCanManage, ownerID)
	// Every lookup the projection makes is scoped to this workspace.
	saved, err := s.projectIPLayer(core.WithWorkspace(deferred, ownerID), st, ownerID, entries)
	if err != nil {
		return WorkspaceView{}, err
	}
	v.IPAllowList = saved
	return v, nil
}

// projectIPLayer writes the workspace layer of entries onto every eligible
// resource in the workspace, then re-reads the stored rules: when another
// Admin saved different rules meanwhile, it projects those instead, so two
// overlapping saves converge on the one stored last rather than leaving
// resources split between them. It returns the rules it converged on.
func (s *Service) projectIPLayer(ctx context.Context, st ipRulesStore, workspaceID string, entries []core.IPAllowListEntry) ([]core.IPAllowListEntry, error) {
	const maxPasses = 3
	for range maxPasses {
		cidrs := core.EnvironmentLayerCIDRs(entries)
		if err := s.projectIPLayerOnce(ctx, workspaceID, cidrs); err != nil {
			return nil, err
		}
		latest, set, err := st.WorkspaceIPAllowList(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		if !set || slices.Equal(core.EnvironmentLayerCIDRs(latest), cidrs) {
			return entries, nil
		}
		entries = latest
	}
	return entries, nil
}

// projectIPLayerOnce is one pass of projectIPLayer: the workspace's web
// services and static sites, then each wired datastore kind. Private
// services, background workers and cron jobs have no public entry point
// (Render's rules do not cover them) and are never touched.
func (s *Service) projectIPLayerOnce(ctx context.Context, workspaceID string, cidrs []string) error {
	if err := s.projectAppIPLayer(ctx, workspaceID, cidrs); err != nil {
		return err
	}
	for _, target := range s.IPLayerTargets {
		if err := target.SetWorkspaceIPAllowList(ctx, workspaceID, cidrs); err != nil {
			return err
		}
	}
	return nil
}

// projectAppIPLayer patches spec.workspaceIPAllowList on every publicly
// routable App of the workspace that does not already carry cidrs, from the
// listed objects (core.PatchEach). Client nil (no cluster wired) is the no-op
// every CR-touching feature degrades to.
func (s *Service) projectAppIPLayer(ctx context.Context, workspaceID string, cidrs []string) error {
	if s.Client == nil {
		return nil
	}
	var list appv1alpha1.AppList
	if err := s.ListByTenant(ctx, &list, workspaceID); err != nil {
		return fmt.Errorf("listing services for the workspace inbound IP rules: %w", err)
	}
	apps := slices.DeleteFunc(core.Pointers(core.ServiceApps(list.Items)), func(a *appv1alpha1.App) bool {
		return !a.Spec.PubliclyRoutable()
	})
	err := core.PatchEach(ctx, s.Client, apps, func(a *appv1alpha1.App) bool {
		if slices.Equal(a.Spec.WorkspaceIPAllowList, cidrs) {
			return false
		}
		a.Spec.WorkspaceIPAllowList = slices.Clone(cidrs)
		return true
	})
	if err != nil {
		return fmt.Errorf("applying the workspace inbound IP rules to its services: %w", err)
	}
	return nil
}

// withIPAllowLists fills each view's IPAllowList with one store read: what
// the workspace saved, or Render's seeded allow-all when it never saved any
// (also every workspace's answer on a store without the rules table).
func (s *Service) withIPAllowLists(ctx context.Context, views []WorkspaceView) ([]WorkspaceView, error) {
	stored := map[string][]core.IPAllowListEntry{}
	if st, ok := s.Store.(ipRulesStore); ok && len(views) > 0 {
		ids := make([]string, len(views))
		for i, v := range views {
			ids[i] = v.ID
		}
		var err error
		if stored, err = st.WorkspaceIPAllowLists(ctx, ids); err != nil {
			return nil, err
		}
	}
	for i := range views {
		// The workspaces list is exempt from the 2FA requirement so a blocked
		// member can see why, but the inbound IP rules are the workspace's
		// configuration: they are left out for a workspace that blocks the caller.
		if views[i].RequireTwoFactor && s.CheckTwoFactor(ctx, views[i].ID) != nil {
			continue
		}
		if entries, ok := stored[views[i].ID]; ok {
			views[i].IPAllowList = entries
		} else {
			views[i].IPAllowList = core.DefaultEnvironmentAllowList()
		}
	}
	return views, nil
}
