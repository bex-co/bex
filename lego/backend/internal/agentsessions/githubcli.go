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
	"encoding/json"
	"fmt"
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

// SessionUpdate changes a session after create (w10/m599), as Devin's
// session security profile is "changed later from the session's settings":
// today only its own `gh` binding. resetGithubCli is the explicit null
// GraphQL cannot carry.
type SessionUpdate struct {
	AgentConfig    *SessionConfigUpdate `json:"agentConfig,omitempty" jsonschema:"the session settings to change; an omitted field keeps its value"`
	ResetGitHubCLI bool                 `json:"resetGithubCli,omitempty" jsonschema:"true returns the session's gh to inheriting, as an explicit null agentConfig.githubCli does"`
}

// SessionConfigUpdate is the agentConfig a session update names.
// GitHubCLISet records that githubCli was named, so its null returns the
// session to inherit while its absence keeps the binding.
type SessionConfigUpdate struct {
	GitHubCLI    *bool `json:"githubCli,omitempty" jsonschema:"this session's gh binding, which only a workspace admin changes (AGENT_SESSION_GITHUB_CLI_LOCKED, reason admin_required; saving the stored binding is never refused): false turns gh off for it, true turns it on against an inherited off, which nobody may while the workspace's gh off is mandatory (reason mandatory), and null returns it to inheriting. An off refuses the session's next gh call; gh returns from its next turn. A child session can only turn gh off"`
	GitHubCLISet bool  `json:"-"`
}

// UnmarshalJSON records whether githubCli was named, and refuses any other
// key: it replaces the strict decoder's own check, which a custom
// unmarshaler bypasses.
func (c *SessionConfigUpdate) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for key, raw := range fields {
		if key != "githubCli" {
			return fmt.Errorf("json: unknown field %q in agentConfig", key)
		}
		c.GitHubCLISet = true
		if err := json.Unmarshal(raw, &c.GitHubCLI); err != nil {
			return err
		}
	}
	return nil
}

// gitHubCLI is the binding an update names: named false when it names none,
// binding nil for inherit.
func (in SessionUpdate) gitHubCLI() (named bool, binding *bool, err error) {
	if in.AgentConfig != nil && in.AgentConfig.GitHubCLISet {
		named, binding = true, in.AgentConfig.GitHubCLI
	}
	if in.ResetGitHubCLI {
		if binding != nil {
			return false, nil, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID",
				"set agentConfig.githubCli or resetGithubCli, not both", map[string]any{"field": "resetGithubCli"})
		}
		named = true
	}
	return named, binding, nil
}

// UpdateSession changes a session's settings after create (w10/m599): its
// own `gh` binding, which only a workspace admin changes, in either
// direction, and nobody turns on under the workspace's mandatory off, as at
// create. Any session the caller may operate takes it in any phase but an
// archived one, which is refused as every mutation verb refuses it (ADR065
// D1). The mints and every dispatch read the binding live, so an off refuses
// the session's next `gh` call, and its tree's, and an on reaches it from its
// next turn. A save that keeps the stored binding is never refused and
// writes nothing, so it cannot overwrite a concurrent change.
func (s *Service) UpdateSession(ctx context.Context, sessionID string, in SessionUpdate) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	named, binding, err := in.gitHubCLI()
	if err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.ArchivedAt != nil {
		return View{}, errArchived(record.Phase)
	}
	if !named {
		return s.toView(record)
	}
	stored, err := agentsession.SessionGitHubCLI(record.AgentConfig)
	if err != nil {
		return View{}, err
	}
	if err := s.refuseGitHubCLIBinding(ctx, record.WorkspaceID, stored, binding, CodeSessionGitHubCLILocked, sessionGitHubCLIField); err != nil {
		return View{}, err
	}
	if sameBinding(stored, binding) {
		return s.toView(record)
	}
	if record, err = s.Store.SetAgentSessionGitHubCLI(ctx, sessionID, binding); err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.ArchivedAt != nil {
		return View{}, errArchived(record.Phase)
	}
	log.Printf("agent-session gh binding changed (session=%s workspace=%s by=%s from=%s to=%s)",
		sessionID, record.WorkspaceID, core.SubjectFrom(ctx), bindingName(stored), bindingName(binding))
	return s.toView(record)
}

// bindingName is a `gh` binding in a log line.
func bindingName(b *bool) string {
	switch {
	case b == nil:
		return "inherit"
	case *b:
		return "on"
	}
	return "off"
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
