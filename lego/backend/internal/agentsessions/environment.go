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
	"context"
	"errors"
	"log"
	"maps"
	"regexp"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The workspace agent environment (w10/m17, ADR047 D5 amendment 2026-10-27)
// is configured once per workspace instead of per session, the way Codex
// cloud, Claude Code on the web, Devin and Copilot configure theirs: the
// sandbox network tier, the custom hostnames the `custom` tier opens, and the
// default model.
const (
	// NetworkTierRegistries (the default, including for every workspace with
	// no row): GitHub, the gateway hops and the built-in package registries,
	// for the whole session.
	NetworkTierRegistries = "registries"
	// NetworkTierCustom adds the workspace's custom hostnames.
	NetworkTierCustom = "custom"
	// NetworkTierFull is the opt-in any-host tier (w10/m19): the session
	// policy has no direct public egress at all, and HTTP(S) to any public
	// host goes through the gateway egress proxy, which refuses the model
	// providers and private ranges (see sessionEgress).
	NetworkTierFull = sessionegress.NetworkTierFull
)

// EnvironmentStore persists the per-workspace environment row.
// *store.PGStore satisfies it; nil means the store is off (reads return the
// defaults, writes are unavailable).
type EnvironmentStore interface {
	GetAgentEnvironment(ctx context.Context, workspaceID string) (store.AgentEnvironment, error)
	UpsertAgentEnvironment(ctx context.Context, workspaceID string, patch store.AgentEnvironmentPatch) (store.AgentEnvironment, error)
}

// Environment is the one cross-surface projection of a workspace's agent
// environment. DefaultModel nil means the provider default. ModelPinned is
// true on the platform model route, whose PinnedModel every claude session
// runs; UpdatedAt is nil until the environment is first written.
type Environment struct {
	OwnerID       string     `json:"ownerId"`
	NetworkTier   string     `json:"networkTier"`
	CustomHosts   []string   `json:"customHosts"`
	DefaultModel  *string    `json:"defaultModel"`
	ModelPinned   bool       `json:"modelPinned"`
	PinnedModel   *string    `json:"pinnedModel"`
	RegistryHosts []string   `json:"registryHosts"`
	UpdatedAt     *time.Time `json:"updatedAt"`
}

// EnvironmentInput is a partial update: an omitted (or null) field keeps its
// value. CustomHosts replaces the whole list ([] clears it); DefaultModel ""
// returns to the provider default.
type EnvironmentInput struct {
	NetworkTier  *string   `json:"networkTier,omitempty"`
	CustomHosts  *[]string `json:"customHosts,omitempty"`
	DefaultModel *string   `json:"defaultModel,omitempty"`
}

// modelIDPattern bounds a model id that reaches a sandbox env var: provider
// ids (claude-sonnet-4-5, claude-sonnet-4-5[1m]) and OpenRouter slugs
// (deepseek/deepseek-v4.1-flash-1017), never whitespace or shell syntax.
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@~+\[\]-]{0,254}$`)

// Environment reads the workspace's agent environment. Any member may read
// it: it carries no secret, and the composer and the settings page both show it.
func (s *Service) Environment(ctx context.Context, ownerID string) (Environment, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanView, ownerID)
	if err != nil {
		return Environment{}, err
	}
	row, err := s.workspaceEnvironment(ctx, workspaceID)
	if err != nil {
		return Environment{}, err
	}
	return s.environmentView(row), nil
}

// UpdateEnvironment applies a partial update. Admin-only (can_manage on the
// exact workspace), like every other workspace setting; the authorize call
// records the audit event. Each field is validated before anything is written,
// so a refused update changes nothing.
func (s *Service) UpdateEnvironment(ctx context.Context, ownerID string, input EnvironmentInput) (Environment, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, ownerID)
	if err != nil {
		return Environment{}, err
	}
	if s.Environments == nil {
		return Environment{}, core.ErrAgentSessionsUnavailable
	}
	patch, err := validateEnvironmentInput(workspaceID, input)
	if err != nil {
		return Environment{}, err
	}
	row, err := s.Environments.UpsertAgentEnvironment(ctx, workspaceID, patch)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Environment{}, core.NewNotFoundError("WORKSPACE_NOT_FOUND", "workspace not found", map[string]any{"ownerId": workspaceID})
		}
		log.Printf("agent environment: write failed (workspace=%s): %v", workspaceID, err)
		return Environment{}, core.NewAgentSessionDependencyUnavailableError()
	}
	return s.environmentView(row), nil
}

func validateEnvironmentInput(workspaceID string, input EnvironmentInput) (store.AgentEnvironmentPatch, error) {
	var patch store.AgentEnvironmentPatch
	if input.NetworkTier != nil {
		switch *input.NetworkTier {
		case NetworkTierRegistries, NetworkTierCustom, NetworkTierFull:
			patch.NetworkTier = input.NetworkTier
		default:
			return patch, core.NewBadRequestError("AGENT_ENVIRONMENT_TIER_INVALID",
				"networkTier must be registries, full or custom", map[string]any{"field": "networkTier"})
		}
	}
	if input.CustomHosts != nil {
		hosts, err := workspaceHosts(*input.CustomHosts)
		if err != nil {
			return patch, err
		}
		patch.CustomHosts = &hosts
	}
	if input.DefaultModel != nil {
		model := *input.DefaultModel
		if model != "" && !modelIDPattern.MatchString(model) {
			return patch, core.NewBadRequestError("AGENT_ENVIRONMENT_MODEL_INVALID",
				"defaultModel must be a model id without spaces (at most 255 characters)", map[string]any{"field": "defaultModel"})
		}
		if pinned, ok := agentsession.PlatformPinnedModel(workspaceID); ok && model != "" && model != pinned {
			return patch, core.NewBadRequestError("AGENT_ENVIRONMENT_MODEL_PINNED",
				"this workspace runs agents on the platform model; its default model cannot be changed",
				map[string]any{"field": "defaultModel", "pinnedModel": pinned})
		}
		patch.DefaultModel = &model
	}
	return patch, nil
}

// workspaceHosts validates a custom hostname list with the per-session
// allowlist rules (exact public DNS names, at most 32) and additionally
// refuses names already in every session's baseline, so a saved list can
// never make a later dispatch fail. Errors carry the AGENT_ENVIRONMENT_*
// code with the same reason/entry params as the session allowlist.
func workspaceHosts(entries []string) ([]string, error) {
	hosts, err := sessionegress.ExtraDestinations(entries)
	if err != nil {
		var coded *core.CodedError
		if errors.As(err, &coded) {
			params := maps.Clone(coded.Params)
			if params == nil {
				params = map[string]any{}
			}
			params["field"] = "customHosts"
			return nil, core.NewBadRequestError("AGENT_ENVIRONMENT_HOSTS_INVALID", coded.Error(), params)
		}
		return nil, err
	}
	for _, host := range hosts {
		if sessionegress.BaselineDestination(host) {
			return nil, core.NewBadRequestError("AGENT_ENVIRONMENT_HOSTS_INVALID",
				"destination is already part of the agent baseline or reachable only through the model proxy",
				map[string]any{"field": "customHosts", "reason": "already_baseline", "entry": host})
		}
	}
	return hosts, nil
}

// workspaceEnvironment returns the stored row or, for a workspace that never
// customized it (or with the store off), the defaults. A read failure fails
// the caller closed: a session must not dispatch on a guessed network posture.
func (s *Service) workspaceEnvironment(ctx context.Context, workspaceID string) (store.AgentEnvironment, error) {
	defaults := store.AgentEnvironment{WorkspaceID: workspaceID, NetworkTier: NetworkTierRegistries}
	if s.Environments == nil {
		return defaults, nil
	}
	row, err := s.Environments.GetAgentEnvironment(ctx, workspaceID)
	if errors.Is(err, store.ErrNotFound) {
		return defaults, nil
	}
	if err != nil {
		log.Printf("agent environment: read failed (workspace=%s): %v", workspaceID, err)
		return store.AgentEnvironment{}, core.NewAgentSessionDependencyUnavailableError()
	}
	return row, nil
}

func (s *Service) environmentView(row store.AgentEnvironment) Environment {
	view := Environment{
		OwnerID:       row.WorkspaceID,
		NetworkTier:   row.NetworkTier,
		CustomHosts:   slices.Clone(row.CustomHosts),
		RegistryHosts: s.registryHosts(),
	}
	if view.CustomHosts == nil {
		view.CustomHosts = []string{}
	}
	if row.DefaultModel != "" {
		model := row.DefaultModel
		view.DefaultModel = &model
	}
	if pinned, ok := agentsession.PlatformPinnedModel(row.WorkspaceID); ok {
		view.ModelPinned = true
		view.PinnedModel = &pinned
	}
	if !row.UpdatedAt.IsZero() {
		at := row.UpdatedAt.UTC()
		view.UpdatedAt = &at
	}
	return view
}

// registryHosts is the package-registry catalog the session policies render
// (BEX_AGENT_SETUP_REGISTRIES or the built-in default), for display.
func (s *Service) registryHosts() []string {
	if len(s.RegistryHosts) > 0 {
		return slices.Clone(s.RegistryHosts)
	}
	return sessionegress.DefaultRegistryDomains()
}

// effectiveHosts is the widening the workspace tier contributes to a session
// policy. The registry catalog itself is rendered by sessionegress for the
// registries and custom tiers; `full` renders no FQDN rule at all.
func effectiveHosts(env store.AgentEnvironment) []string {
	if env.NetworkTier == NetworkTierCustom {
		return env.CustomHosts
	}
	return nil
}

// sessionEgress is the destination set one dispatch renders: the full-internet
// proxy posture for the `full` tier, else the validated allowlist.
func sessionEgress(env store.AgentEnvironment, allowlist []string) sessionegress.Egress {
	return sessionegress.Egress{FullInternet: env.NetworkTier == NetworkTierFull, Hosts: allowlist}
}

// sessionModel resolves the model a session runs: the request's explicit
// model, else the workspace default. A platform-route workspace's claude
// session always runs the pinned model; an explicit different model is
// refused at create (AGENT_SESSION_MODEL_PINNED) instead of reaching the
// gateway's model pin as a 403 mid-turn. A stored model that differs (a
// session created before the workspace joined the route) is replaced
// silently on steer, where the caller chose no model.
func sessionModel(workspaceID string, config AgentConfig, env store.AgentEnvironment, explicit bool) (string, error) {
	if pinned, ok := agentsession.PlatformPinnedModel(workspaceID); ok && agentsession.PinnedAgent(config.Agent) {
		if explicit && config.Model != "" && config.Model != pinned {
			return "", core.NewBadRequestError("AGENT_SESSION_MODEL_PINNED",
				"this workspace runs agents on the platform model; omit agentConfig.model or set it to the pinned model",
				map[string]any{"field": "agentConfig.model", "pinnedModel": pinned})
		}
		return pinned, nil
	}
	if config.Model != "" {
		return config.Model, nil
	}
	return env.DefaultModel, nil
}
