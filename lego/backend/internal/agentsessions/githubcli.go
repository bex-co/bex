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

package agentsessions

import (
	"cmp"
	"context"
	"log"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The `gh` binding chain (w10/m561, ADR047 D2): a session's own
// agentConfig.githubCli, an automation's template.agentConfig.githubCli and
// the agent environment's automationsGithubCliEnabled and githubCliMandatory
// over its githubCliEnabled. agentsession.GitHubCLIChain resolves them; this
// file holds who may set what.

// The refusals of a `gh` binding the caller may not choose. Each carries
// params.reason: "mandatory" when it turns `gh` on under the workspace's
// mandatory off, "admin_required" when the caller is not a workspace admin.
const (
	CodeSessionGitHubCLILocked     = "AGENT_SESSION_GITHUB_CLI_LOCKED"
	CodeAutomationGitHubCLILocked  = "AGENT_AUTOMATION_GITHUB_CLI_LOCKED"
	CodeEnvironmentGitHubCLILocked = "AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED"
	// CodeEnvironmentGitHubCLIInvalid refuses an update that both sets the
	// automations default and resets it.
	CodeEnvironmentGitHubCLIInvalid = "AGENT_ENVIRONMENT_GITHUB_CLI_INVALID"
)

// The fields a session's and an automation's `gh` refusals name.
const (
	sessionGitHubCLIField  = "agentConfig.githubCli"
	templateGitHubCLIField = "template.agentConfig.githubCli"
)

func isTrue(b *bool) bool { return b != nil && *b }

// sameBinding reports whether two `gh` bindings are equal, nil (inherit)
// included.
func sameBinding(a, b *bool) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// githubCLILocked is the refusal of binding `gh` on under the workspace's
// mandatory off.
func githubCLILocked(code, field string) error {
	return core.NewForbiddenError(code,
		"this workspace's admins made its gh off mandatory, so nothing turns gh on for an automation or a session; leave it out to inherit the off",
		map[string]any{"field": field, "reason": "mandatory"})
}

// refuseGitHubCLIBinding refuses changing a session's or an automation's
// `gh` binding from stored (nil on a create) to binding when the caller may
// not: only a workspace admin (can_manage, bex's "Manage security profiles")
// chooses or changes one, in either direction, as Devin's "Members without
// the permission can't enumerate, select, or change profiles" (w10/044), and
// nobody turns `gh` on under the workspace's mandatory off. Leaving it
// unchanged is never refused, so a create naming none inherits and a save
// that keeps the stored binding keeps it. The verb's own Authorize already
// admitted the caller to workspaceID and recorded it, so the admin probe
// records nothing.
func (s *Service) refuseGitHubCLIBinding(ctx context.Context, workspaceID string, stored, binding *bool, code, field string) error {
	if sameBinding(stored, binding) {
		return nil
	}
	if isTrue(binding) {
		env, err := s.workspaceEnvironment(ctx, workspaceID)
		if err != nil {
			return err
		}
		if env.GitHubCLIDisabled && env.GitHubCLIMandatory {
			return githubCLILocked(code, field)
		}
	}
	if !s.CanDecisionOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)).Allowed() {
		return core.NewForbiddenError(code,
			"only a workspace admin chooses or changes gh for an automation or a session; leave it out to inherit, or unchanged on an update",
			map[string]any{"field": field, "reason": "admin_required"})
	}
	return nil
}

// automationsGitHubCLILocked refuses an environment update whose automations
// default turns `gh` on while the workspace's off is mandatory once the
// update applies.
func automationsGitHubCLILocked(current store.AgentEnvironment, patch store.AgentEnvironmentPatch) error {
	off := cmp.Or(patch.GitHubCLIDisabled, &current.GitHubCLIDisabled)
	mandatory := cmp.Or(patch.GitHubCLIMandatory, &current.GitHubCLIMandatory)
	if *off && *mandatory {
		return githubCLILocked(CodeEnvironmentGitHubCLILocked, automationsGitHubCLIField)
	}
	return nil
}

// githubCLIOffLevel resolves record's `gh` binding chain under its
// workspace's environment row env, as the mints do: "" when the session gets
// `gh`, else the level that turned it off. record may be a session not
// stored yet (a create): its parent and automation are what is read. A
// repo-less session has no `gh` to bind, so nothing is read for it.
func (s *Service) githubCLIOffLevel(ctx context.Context, record store.AgentSession, env store.AgentEnvironment) (agentsession.GitHubCLILevel, error) {
	if strings.TrimSpace(record.Repo) == "" {
		return "", nil
	}
	level, err := agentsession.ResolveGitHubCLI(ctx, s.Store, s.Automations, env, record)
	if err != nil {
		log.Printf("agent-session gh binding: read failed (session=%s workspace=%s): %v", record.ID, record.WorkspaceID, err)
		return "", core.NewAgentSessionDependencyUnavailableError()
	}
	return level, nil
}
