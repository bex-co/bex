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
	"encoding/json"
	"errors"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

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
	UpsertAgentEnvironmentLocked(ctx context.Context, workspaceID string, patch store.AgentEnvironmentPatch, write func(ctx context.Context, current store.AgentEnvironment) error) (store.AgentEnvironment, error)
}

// Environment is the one cross-surface projection of a workspace's agent
// environment. DefaultModel nil means the provider default. ModelPinned is
// true on the platform model route, whose PinnedModel every claude session
// runs; UpdatedAt is nil until the environment is first written.
type Environment struct {
	OwnerID      string   `json:"ownerId"`
	NetworkTier  string   `json:"networkTier"`
	CustomHosts  []string `json:"customHosts"`
	DefaultModel *string  `json:"defaultModel"`
	ModelPinned  bool     `json:"modelPinned"`
	PinnedModel  *string  `json:"pinnedModel"`
	// ModelSource is the credential the workspace's claude sessions use;
	// ModelKeySet is ModelSource == ModelSourceBYO (environmentView derives
	// it). The key itself is write-only and never returned by any surface.
	ModelKeySet   bool        `json:"modelKeySet"`
	ModelSource   ModelSource `json:"modelSource"`
	RegistryHosts []string    `json:"registryHosts"`
	UpdatedAt     *time.Time  `json:"updatedAt"`
	// The agent-model spend controls (ADR091 D5, D10.1); all nil when the
	// store is off. AgentModelMonthlySpendCapUSD is the effective cap:
	// min(the admin override or the plan default, the trust tier's ceiling).
	AgentModelMonthlySpendCapUSD  *float64 `json:"agentModelMonthlySpendCapUsd"`
	AgentModelPlanSpendCapUSD     *float64 `json:"agentModelPlanSpendCapUsd"`
	AgentModelSpendCapOverrideUSD *float64 `json:"agentModelSpendCapOverrideUsd"`
	AgentModelSpendMonthToDateUSD *float64 `json:"agentModelSpendMonthToDateUsd"`
	AgentModelMaxSpendCapUSD      *float64 `json:"agentModelMaxSpendCapUsd"`
	TrustTier                     *string  `json:"trustTier"`
	NextChargeThresholdUSD        *float64 `json:"nextChargeThresholdUsd"`
}

// EnvironmentInput is a partial update: an omitted (or null) field keeps its
// value. CustomHosts replaces the whole list ([] clears it); DefaultModel ""
// returns to the provider default. AgentModelMonthlySpendCapUSD is the one
// exception to "null keeps": an explicit null clears the override back to
// the plan default, which the wire adapters record in
// AgentModelMonthlySpendCapSet.
type EnvironmentInput struct {
	NetworkTier                    *string   `json:"networkTier,omitempty"`
	CustomHosts                    *[]string `json:"customHosts,omitempty"`
	DefaultModel                   *string   `json:"defaultModel,omitempty"`
	AgentModelMonthlySpendCapUSD   *float64  `json:"agentModelMonthlySpendCapUsd,omitempty" jsonschema:"monthly agent-model spend cap in USD (admins only), up to 10x the plan default; null returns to the plan default"`
	ResetAgentModelMonthlySpendCap bool      `json:"resetAgentModelMonthlySpendCap,omitempty" jsonschema:"true returns the spend cap to the plan default"`
	// ModelAPIKey sets or rotates the workspace's own model provider key.
	// Write-only: it goes to OpenBao and never into a response, an error, a
	// log line or the audit trail.
	ModelAPIKey      *string `json:"modelApiKey,omitempty" jsonschema:"write-only: sets or rotates the workspace's own model provider key (admins only); it is never returned, and sessions then use it instead of the bex-provided model"`
	ClearModelAPIKey bool    `json:"clearModelApiKey,omitempty" jsonschema:"true removes the workspace's own model provider key"`
	// AgentModelMonthlySpendCapSet is true when the field was present, so a
	// nil value with it set clears the override.
	AgentModelMonthlySpendCapSet bool `json:"-"`
}

// spendCapField is the wire name of the spend-cap override.
const spendCapField = "agentModelMonthlySpendCapUsd"

// ModelSource is the model credential an environment reports (ADR091 D7):
// the workspace's own key, the bex-provided platform model (billed at
// provider cost + 30%), or neither.
type ModelSource string

const (
	ModelSourceBYO      ModelSource = "byo"
	ModelSourcePlatform ModelSource = "platform"
	ModelSourceNone     ModelSource = "none"
)

// ModelKeyAuditTarget is the audit target of an update that sets or clears
// the workspace model key: the event names the action, never the value.
const ModelKeyAuditTarget = "model-key"

// ModelKeyInvalidCode refuses a malformed model key; its message never
// echoes the value.
const ModelKeyInvalidCode = "AGENT_ENVIRONMENT_MODEL_KEY_INVALID"

// maxModelKeyBytes bounds a model key (provider keys and OAuth tokens are
// well under 1 KB).
const maxModelKeyBytes = 4096

// modelKeyChange validates the write-only key fields: set reports a change,
// and value is "" for a clear.
func modelKeyChange(input EnvironmentInput) (set bool, value string, err error) {
	invalid := func(msg string) error {
		return core.NewBadRequestError(ModelKeyInvalidCode, msg, map[string]any{"field": "modelApiKey"})
	}
	if input.ClearModelAPIKey {
		if input.ModelAPIKey != nil {
			return false, "", invalid("set modelApiKey or clearModelApiKey, not both")
		}
		return true, "", nil
	}
	if input.ModelAPIKey == nil {
		return false, "", nil
	}
	value = strings.TrimSpace(*input.ModelAPIKey)
	if value == "" {
		return false, "", invalid("modelApiKey must not be empty; use clearModelApiKey to remove the key")
	}
	if len(value) > maxModelKeyBytes {
		return false, "", invalid("modelApiKey must be at most 4096 bytes")
	}
	if strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false, "", invalid("modelApiKey must not contain spaces or control characters")
	}
	return true, value, nil
}

// markSpendCapPresence records whether the raw JSON object named the spend
// cap, so an explicit null clears it instead of keeping it.
func (in *EnvironmentInput) markSpendCapPresence(raw []byte) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) == nil {
		_, in.AgentModelMonthlySpendCapSet = fields[spendCapField]
	}
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
	view := s.environmentView(row)
	if s.Spend == nil {
		return view, nil
	}
	status, err := s.readSpend(ctx, workspaceID)
	if err != nil {
		return Environment{}, err
	}
	return withSpend(view, status), nil
}

// UpdateEnvironment applies a partial update. Admin-only (can_manage on the
// exact workspace), like every other workspace setting; the authorize call
// records the audit event, targeted at ModelKeyAuditTarget when the update
// sets or clears the model key. Each field is validated before anything is
// written, so a refused update changes nothing.
func (s *Service) UpdateEnvironment(ctx context.Context, ownerID string, input EnvironmentInput) (Environment, error) {
	keyChanged, key, keyErr := modelKeyChange(input)
	target := ""
	if keyChanged || keyErr != nil {
		target = ModelKeyAuditTarget
	}
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, target)
	if err != nil {
		return Environment{}, err
	}
	if keyErr != nil {
		return Environment{}, keyErr
	}
	if s.Environments == nil || (keyChanged && s.ModelKeys == nil) {
		return Environment{}, core.ErrAgentSessionsUnavailable
	}
	byoKey, err := s.byoKeyAfter(ctx, workspaceID, input, keyChanged, key)
	if err != nil {
		return Environment{}, err
	}
	patch, err := validateEnvironmentInput(workspaceID, input, byoKey)
	if err != nil {
		return Environment{}, err
	}
	// One spend read serves both the cap bound and the returned view; the
	// written override is applied to it in hand.
	var status agentsession.SpendStatus
	if s.Spend != nil {
		if status, err = s.readSpend(ctx, workspaceID); err != nil {
			return Environment{}, err
		}
	} else if input.AgentModelMonthlySpendCapUSD != nil {
		return Environment{}, core.ErrAgentSessionsUnavailable
	}
	if err := s.validateSpendCap(status, input, &patch); err != nil {
		return Environment{}, err
	}
	var row store.AgentEnvironment
	if keyChanged {
		patch.ModelKeySet = &byoKey
		row, err = s.writeModelKey(ctx, workspaceID, key, patch)
	} else {
		row, err = s.Environments.UpsertAgentEnvironment(ctx, workspaceID, patch)
	}
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			return Environment{}, core.NewNotFoundError("WORKSPACE_NOT_FOUND", "workspace not found", map[string]any{"ownerId": workspaceID})
		case errors.Is(err, errModelKeyWrite):
			log.Printf("agent environment: model key write failed (workspace=%s clear=%t)", workspaceID, key == "")
		default:
			log.Printf("agent environment: write failed (workspace=%s): %v", workspaceID, err)
		}
		return Environment{}, core.NewAgentSessionDependencyUnavailableError()
	}
	view := s.environmentView(row)
	if s.Spend == nil {
		return view, nil
	}
	return withSpend(view, status.WithOverride(row.SpendCapOverrideMicroUSD)), nil
}

// byoKeyAfter is whether the workspace has its own model key after this
// update. Only a default model change on a workspace the platform route
// covers depends on it (the pin), so only that case reads the stored flag.
func (s *Service) byoKeyAfter(ctx context.Context, workspaceID string, input EnvironmentInput, keyChanged bool, key string) (bool, error) {
	if keyChanged {
		return key != "", nil
	}
	if _, covered := agentsession.PlatformPinnedModel(workspaceID, false); !covered || input.DefaultModel == nil {
		return false, nil
	}
	current, err := s.workspaceEnvironment(ctx, workspaceID)
	return current.ModelKeySet(), err
}

// errModelKeyWrite is an OpenBao failure while writing the model key. The
// cause is dropped, never logged: it may echo the request carrying the value.
var errModelKeyWrite = errors.New("agent environment: model key write failed")

// writeModelKey stores (key != "") or deletes (key == "") the workspace's
// model key in OpenBao and applies patch (which records the flag) in one
// transaction holding the environment row lock, so concurrent admin writes
// serialize and the flag commits only after OpenBao succeeded. When the flag
// write then fails after a Put, the just-written key is deleted best-effort
// if the workspace had none before (or no longer exists). A rotation cannot
// restore the previous key, so a failed rotation returns the error with the
// new key active under the still-set flag. A failed flag write after a
// delete leaves the flag over a missing key, which the mint refuses as
// ErrModelKeyMissing: a cleared key is never used again.
func (s *Service) writeModelKey(ctx context.Context, workspaceID, key string, patch store.AgentEnvironmentPatch) (store.AgentEnvironment, error) {
	path := modelKeySecretPath(workspaceID)
	put, rotation := false, false
	row, err := s.Environments.UpsertAgentEnvironmentLocked(ctx, workspaceID, patch, func(ctx context.Context, current store.AgentEnvironment) error {
		rotation = current.ModelKeySet()
		if key == "" {
			if s.ModelKeys.Delete(ctx, path) != nil {
				return errModelKeyWrite
			}
			return nil
		}
		if s.ModelKeys.Put(ctx, path, map[string]string{agentsession.ModelKeyField: key}) != nil {
			return errModelKeyWrite
		}
		put = true
		return nil
	})
	if err != nil {
		if put && (!rotation || errors.Is(err, store.ErrNotFound)) && s.ModelKeys.Delete(context.WithoutCancel(ctx), path) != nil {
			log.Printf("agent environment: model key cleanup after a failed flag write failed (workspace=%s)", workspaceID)
		}
		return store.AgentEnvironment{}, err
	}
	if s.ModelKeyChanged != nil {
		s.ModelKeyChanged(workspaceID)
	}
	return row, nil
}

func validateEnvironmentInput(workspaceID string, input EnvironmentInput, byoKey bool) (store.AgentEnvironmentPatch, error) {
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
		if pinned, ok := agentsession.PlatformPinnedModel(workspaceID, byoKey); ok && model != "" && model != pinned {
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
	switch pinned, ok := agentsession.PlatformPinnedModel(row.WorkspaceID, row.ModelKeySet()); {
	case row.ModelKeySet():
		view.ModelSource = ModelSourceBYO
	case ok:
		view.ModelSource = ModelSourcePlatform
		view.ModelPinned = true
		view.PinnedModel = &pinned
	default:
		view.ModelSource = ModelSourceNone
	}
	view.ModelKeySet = view.ModelSource == ModelSourceBYO
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
// model, else the workspace default. The claude session of a workspace on
// the platform route (one without its own model key) always runs the pinned
// model; an explicit different model is refused at create
// (AGENT_SESSION_MODEL_PINNED) instead of reaching the gateway's model pin as
// a 403 mid-turn. On steer, where the caller chose no model, the stored model
// follows the workspace's model source now: a session created before the
// workspace joined the route (or after it cleared its key) takes the pinned
// model, and one created on the route before the workspace set its own key
// drops the pinned OpenRouter model for the workspace default (empty: the
// agent profile's), which its own provider serves.
func sessionModel(workspaceID string, config AgentConfig, env store.AgentEnvironment, explicit bool) (string, error) {
	pinnedAgent := agentsession.PinnedAgent(config.Agent)
	if pinned, ok := agentsession.PlatformPinnedModel(workspaceID, env.ModelKeySet()); ok && pinnedAgent {
		if explicit && config.Model != "" && config.Model != pinned {
			return "", core.NewBadRequestError("AGENT_SESSION_MODEL_PINNED",
				"this workspace runs agents on the platform model; omit agentConfig.model or set it to the pinned model",
				map[string]any{"field": "agentConfig.model", "pinnedModel": pinned})
		}
		return pinned, nil
	}
	if routeModel, covered := agentsession.PlatformPinnedModel(workspaceID, false); pinnedAgent && !explicit && covered && config.Model == routeModel {
		return env.DefaultModel, nil
	}
	if config.Model != "" {
		return config.Model, nil
	}
	return env.DefaultModel, nil
}
