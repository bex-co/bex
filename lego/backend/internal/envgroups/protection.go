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

package envgroups

import (
	"context"
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// requireEnvironmentAdmin keeps verb, on something in environmentID, to the
// Admins of workspace when that environment is protected (w4/m594).
// render.com/docs/projects: "Deleting a group and viewing or changing its
// variables or secret files are admin-only actions in a protected
// environment." A non-Admin is refused with PROTECTED_ENVIRONMENT_ADMIN_REQUIRED;
// a group with no environment belongs to no protected one, and a failed
// protection read refuses. A Blueprint's own writes never come through here:
// Render lets anyone who publishes the render.yaml change what it manages
// (ApplyEnvGroup, LinkEnvGroup).
func (s *Service) requireEnvironmentAdmin(ctx context.Context, workspace, environmentID, verb string) error {
	return core.RequireEnvironmentAdmin(ctx, s.Protection, environmentID, verb, func() error {
		return s.AuthorizeFreshOn(ctx, core.RelCanManage, core.WorkspaceObject(workspace))
	})
}

// requireGroupAdmin is requireEnvironmentAdmin on g's own environment.
func (s *Service) requireGroupAdmin(ctx context.Context, g store.EnvGroup, verb string) error {
	return s.requireEnvironmentAdmin(ctx, g.TenantID, g.EnvironmentID, verb)
}

// valuesWithheld reports, without recording anything, that a REST read of g
// answers without its values: g is in a protected environment and the caller
// is not a workspace Admin (retrieved). A probe that cannot answer withholds
// nothing, so the read takes the audited, fresh path and fails there.
func (s *Service) valuesWithheld(ctx context.Context, g renderEnvGroup) bool {
	// The view spells the environment by its public evm- alias; the
	// protection store keys the row by its storage id.
	protected, err := core.EnvironmentProtected(ctx, s.Protection, id.EnvironmentStorageID(g.EnvironmentID))
	if err != nil || !protected {
		return false
	}
	return s.CanDecisionOn(ctx, core.RelCanManage, core.WorkspaceObject(g.OwnerID)).Outcome == core.DecisionDenied
}

// ActionCapabilities projects the decisions of an environment group's
// restricted verbs for ONE group in the acting workspace (ADR087): revealing
// its values (can_view_sensitive), changing its variables, secret files and
// links (can_create) and deleting it (can_create). In a protected environment
// each reads as the verbs' own fresh workspace-manage check
// (core.RestrictedDecision, w4/m594), so the dashboard says "This action is
// limited to admins." before the click. Read-only; the verbs re-run their own
// guards at dispatch.
func (s *Service) ActionCapabilities(ctx context.Context, gid string) ([]core.ActionDecision, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanView, gid)
	if err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, map[string]string{core.LabelTenant: g.TenantID}); err != nil {
		return nil, err
	}
	workspace := core.WorkspaceObject(g.TenantID)
	protected := sync.OnceValues(func() (bool, error) {
		return core.EnvironmentProtected(ctx, s.Protection, g.EnvironmentID)
	})
	admin := sync.OnceValue(func() error { return s.AuthorizeFreshOn(ctx, core.RelCanManage, workspace) })
	create := s.CanDecisionOn(ctx, core.RelCanCreate, workspace)
	view, _ := core.RestrictedDecision(core.VerbEnvironmentVariables, s.CanDecisionOn(ctx, core.RelCanViewSensitive, workspace), protected, admin)
	update, _ := core.RestrictedDecision(core.VerbEnvironmentVariables, create, protected, admin)
	remove, _ := core.RestrictedDecision("delete", create, protected, admin)
	return []core.ActionDecision{
		core.DecideAction(core.ActionViewEnvVars, view, ""),
		core.DecideAction(core.ActionUpdateEnvVars, update, ""),
		core.DecideAction(core.ActionDelete, remove, ""),
	}, nil
}
