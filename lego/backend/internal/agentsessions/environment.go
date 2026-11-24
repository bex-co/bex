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
	"fmt"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The workspace agent environment (w10/m17, ADR047 D5 amendment 2026-10-27)
// is configured once per workspace instead of per session, the way Codex
// cloud, Claude Code on the web, Devin and Copilot configure theirs: the
// sandbox network tier, the custom hostnames the `custom` tier opens, the
// default model, the default sandbox template (w10/m519), whether its
// sessions get a `gh` (w10/m524), which platform catalog models its members
// may choose (w10/m527) and, on Enterprise, how long the sandbox of a session
// a person started stays up after its turn (w6/m563).
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
// environment. DefaultModel nil means the provider default, or on the
// platform model route the catalog's first model. Models is that whole
// catalog (w10/m516), empty off the route, where any model is free-form;
// EnabledModels is the part of it the workspace's admins left on (w10/m527),
// the models a claude session, an automation and DefaultModel may name, the
// catalog's default always among them: every catalog model while
// AllowAllModels (w10/m592), else the allowlist an admin left on (w10/m568),
// or under a filter by provider every model of a listed provider (w10/m573).
// AllowAllModels is Devin's "Allow all models" mode, true off the route,
// where nothing narrows the free-form model. NewModels
// are the catalog models off because that allowlist predates them, apart from
// the ones an admin turned off. Providers are the catalog's providers, and
// EnabledProviders the ones the filter by provider lists, nil while the list
// is by model. A DefaultModel outside EnabledModels
// is stored but does not take effect: the catalog's default runs. ModelPinned
// is true on the route, whose gateway pins each session to its chosen model;
// PinnedModel is the catalog's default, kept for clients older than Models.
// UpdatedAt is nil until the environment is first written.
type Environment struct {
	OwnerID          string   `json:"ownerId"`
	NetworkTier      string   `json:"networkTier"`
	CustomHosts      []string `json:"customHosts"`
	DefaultModel     *string  `json:"defaultModel"`
	Models           []string `json:"models"`
	AllowAllModels   bool     `json:"allowAllModels"`
	EnabledModels    []string `json:"enabledModels"`
	NewModels        []string `json:"newModels"`
	Providers        []string `json:"providers"`
	EnabledProviders []string `json:"enabledProviders"`
	ModelPinned      bool     `json:"modelPinned"`
	PinnedModel      *string  `json:"pinnedModel"`
	TemplateOffer
	// ModelSource is the credential the workspace's claude sessions use;
	// ModelKeySet is ModelSource == ModelSourceBYO (environmentView derives
	// it). The key itself is write-only and never returned by any surface.
	ModelKeySet   bool        `json:"modelKeySet"`
	ModelSource   ModelSource `json:"modelSource"`
	RegistryHosts []string    `json:"registryHosts"`
	// EnabledAgents are the agent profiles members can start sessions with,
	// as the workspace's admins chose (w7/m392); AgentDataUse lists each
	// profile whose model provider may train on what its sessions send, and
	// the admin acknowledgement that turning it on requires.
	EnabledAgents []string       `json:"enabledAgents"`
	AgentDataUse  []AgentDataUse `json:"agentDataUse"`
	// GitHubCLIEnabled is whether the workspace's repo sessions get a `gh`
	// that reaches their repository through bex (w10/m524); true unless an
	// admin turned it off.
	GitHubCLIEnabled bool `json:"githubCliEnabled"`
	// AutomationsGitHubCLIEnabled is the automations default (w10/m561): nil
	// inherits GitHubCLIEnabled for the sessions automations start.
	// GitHubCLIMandatory makes the workspace's off a floor no automation or
	// session turns back on; it has no effect while GitHubCLIEnabled is true.
	AutomationsGitHubCLIEnabled *bool `json:"automationsGithubCliEnabled"`
	GitHubCLIMandatory          bool  `json:"githubCliMandatory"`
	// IdleTimeoutMinutes is how long the sandbox of a session a person started
	// stays up after its turn ends or its last editor disconnects (w6/m563):
	// the workspace's own timeout on Enterprise, else the platform's 5
	// minutes. IdleTimeoutConfigurable is whether the plan lets the
	// workspace's admins set it.
	IdleTimeoutMinutes      int        `json:"idleTimeoutMinutes"`
	IdleTimeoutConfigurable bool       `json:"idleTimeoutConfigurable"`
	UpdatedAt               *time.Time `json:"updatedAt"`
	// The agent-model spend controls (ADR091 D5, D10.1); all nil when the
	// store is off, and all but AgentModelSpendCapReached and
	// BillingBalanceDue nil for a member who cannot view billing details
	// (spendFor). AgentModelMonthlySpendCapUSD is the effective cap:
	// min(the admin override or the plan default, the trust tier's ceiling).
	AgentModelMonthlySpendCapUSD  *float64 `json:"agentModelMonthlySpendCapUsd"`
	AgentModelPlanSpendCapUSD     *float64 `json:"agentModelPlanSpendCapUsd"`
	AgentModelSpendCapOverrideUSD *float64 `json:"agentModelSpendCapOverrideUsd"`
	AgentModelSpendMonthToDateUSD *float64 `json:"agentModelSpendMonthToDateUsd"`
	AgentModelMaxSpendCapUSD      *float64 `json:"agentModelMaxSpendCapUsd"`
	TrustTier                     *string  `json:"trustTier"`
	// NextChargeThresholdUSD is the spend beyond billing credits that
	// charges the card mid-cycle (nil at the top tier, which bills at month
	// end); nil too wherever bex would not charge it — a payment hold, an
	// unpaid invoice, the workspace's own key, or an unchargeable workspace
	// — so no reader can promise a charge the mint will never make.
	NextChargeThresholdUSD *float64 `json:"nextChargeThresholdUsd"`
	// AgentModelCreditUSD is the billing credit the mint takes off the charge
	// window's spend (credits pay first, D10.2a; 0 until one is read in the
	// window), and AgentModelUnbilledUSD the spend a charge would collect now:
	// the window's spend less that credit and the paid charges.
	AgentModelCreditUSD   *float64 `json:"agentModelCreditUsd"`
	AgentModelUnbilledUSD *float64 `json:"agentModelUnbilledUsd"`
	// TrustTierLocked: a failed or disputed payment holds the workspace at
	// the first tier (ADR091 D10.1). AgentModelSpendCapTierLimited: the trust
	// tier's ceiling, not the plan or override, sets the effective cap.
	// BillingBalanceDue: a failed invoice is open or a written-off one unpaid,
	// so platform-model requests are refused with BILLING_BALANCE_DUE
	// (D10.3). AgentModelSpendCapReached: the mint refuses a new
	// platform-model request at the cap now; a workspace whose next request
	// would charge its card instead (D10.2a) is not reached, so clients need
	// not re-derive the verdict from the figures. All nil when the store is
	// off.
	TrustTierLocked               *bool `json:"trustTierLocked"`
	AgentModelSpendCapTierLimited *bool `json:"agentModelSpendCapTierLimited"`
	BillingBalanceDue             *bool `json:"billingBalanceDue"`
	AgentModelSpendCapReached     *bool `json:"agentModelSpendCapReached"`
	// AgentModelChargesAtThreshold: reaching nextChargeThresholdUsd charges
	// the workspace's card and it keeps working (D10.2a); false where bex
	// cannot charge it (no subscription, excluded, comped), on its own model
	// key, or under a payment hold. Nil when the store is off.
	AgentModelChargesAtThreshold *bool `json:"agentModelChargesAtThreshold"`
}

// EnvironmentInput is a partial update: an omitted (or null) field keeps its
// value. CustomHosts replaces the whole list ([] clears it); DefaultModel ""
// returns to the provider default and DefaultTemplate "" to the platform's
// (sandbox.DefaultAgentTemplate). AgentModelMonthlySpendCapUSD is the one
// exception to "null keeps": an explicit null clears the override back to
// the plan default, which the wire adapters record in
// AgentModelMonthlySpendCapSet.
type EnvironmentInput struct {
	NetworkTier                    *string   `json:"networkTier,omitempty"`
	CustomHosts                    *[]string `json:"customHosts,omitempty"`
	DefaultModel                   *string   `json:"defaultModel,omitempty"`
	DefaultTemplate                *string   `json:"defaultTemplate,omitempty" jsonschema:"the sandbox template sessions and automation runs that name none run on: one of the environment's templates (\"\" returns to agent); applies to sessions created afterwards"`
	AgentModelMonthlySpendCapUSD   *float64  `json:"agentModelMonthlySpendCapUsd,omitempty" jsonschema:"monthly agent-model spend cap in USD (admins only), up to 10x the plan default; null returns to the plan default"`
	ResetAgentModelMonthlySpendCap bool      `json:"resetAgentModelMonthlySpendCap,omitempty" jsonschema:"true returns the spend cap to the plan default"`
	// ModelAPIKey sets or rotates the workspace's own model provider key.
	// Write-only: it goes to OpenBao and never into a response, an error, a
	// log line or the audit trail.
	ModelAPIKey      *string `json:"modelApiKey,omitempty" jsonschema:"write-only: sets or rotates the workspace's own model provider key (admins only); it is never returned, and sessions then use it instead of the bex-provided model"`
	ClearModelAPIKey bool    `json:"clearModelApiKey,omitempty" jsonschema:"true removes the workspace's own model provider key"`
	// EnabledAgents replaces the agent profiles members can use;
	// AcknowledgeAgentDataUse records the acknowledgement a profile in
	// agentDataUse needs before it can be enabled.
	EnabledAgents           *[]string `json:"enabledAgents,omitempty" jsonschema:"replaces the agent profiles members can start sessions with (admins only); the others are turned off, and running sessions keep running. A profile listed in agentDataUse can be enabled only once acknowledged"`
	AcknowledgeAgentDataUse []string  `json:"acknowledgeAgentDataUse,omitempty" jsonschema:"agent profiles from agentDataUse whose data use this admin acknowledges: their model provider may train on the prompts, code, files, context and responses their sessions send"`
	GitHubCLIEnabled        *bool     `json:"githubCliEnabled,omitempty" jsonschema:"false turns gh off for the workspace's agent sessions (admins only): turns dispatched afterwards get no gh configuration, and a running session's next gh call is refused; Git pushes to the session branch and the bex MCP tools keep working. true turns it back on"`
	// AutomationsGitHubCLIEnabled sets the automations default; an explicit
	// null returns it to inherit, which the wire adapters record in
	// AutomationsGitHubCLIEnabledSet (w10/m561).
	AutomationsGitHubCLIEnabled *bool `json:"automationsGithubCliEnabled,omitempty" jsonschema:"the gh binding of the sessions automations start (admins only): false turns gh off for them, true turns it on for them, null returns to inheriting githubCliEnabled. An automation's own binding wins over it. true is refused with AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED while the workspace's gh off is mandatory"`
	// ResetAutomationsGitHubCLI is the explicit null graphql-go cannot carry.
	ResetAutomationsGitHubCLI bool  `json:"resetAutomationsGithubCli,omitempty" jsonschema:"true returns the automations default to inheriting githubCliEnabled, as an explicit null automationsGithubCliEnabled does"`
	GitHubCLIMandatory        *bool `json:"githubCliMandatory,omitempty" jsonschema:"true makes the workspace's gh off mandatory (admins only): while githubCliEnabled is false, no automations default, automation or session turns gh back on, and saving one that would is refused with a GITHUB_CLI_LOCKED code; it has no effect while githubCliEnabled is true. false lets lower levels choose again"`
	// AllowAllModels is Devin's mode (w10/m592): true allows every catalog
	// model, one added later included; false alone keeps the models on now
	// as an allowlist.
	AllowAllModels *bool `json:"allowAllModels,omitempty" jsonschema:"Devin's \"Allow all models\" mode (admins only): true offers every platform catalog model, one added to the catalog later included, from an allowlist or a filter by provider, and is refused with AGENT_ENVIRONMENT_MODELS_INVALID beside enabledModels or enabledProviders. false alone keeps the models on now as an allowlist, so a model added to the catalog later stays off (newModels) until an admin turns it on; it changes nothing on an allowlist or a filter by provider. With enabledModels, false stores that allowlist even when it lists every catalog model. Applies to the platform model route only"`
	// EnabledModels replaces the platform catalog models members may choose;
	// the catalog's others are turned off (w10/m527). The mode stays as it is
	// (w10/m592): an allowlist listing every catalog model stays one, and a
	// model off makes a workspace that allows all an allowlist (w10/m568).
	EnabledModels *[]string `json:"enabledModels,omitempty" jsonschema:"replaces the platform catalog models members may choose (admins only); the catalog's others are turned off, and a filter by provider is replaced. It keeps the allowAllModels mode: an allowlist stays an allowlist even when it lists every catalog model, and a model added to the catalog later stays off until an admin turns it on (it reads in newModels); on a workspace that allows all models, listing every model changes nothing and leaving any model off makes the list an allowlist. allowAllModels true returns to allowing all. It must keep the catalog's default (models[0]) and name only catalog models. A session, message or automation naming a turned-off model is refused, and one already on it runs the workspace default from its next turn (the catalog default when the workspace default is off too). Applies to the platform model route only"`
	// EnabledProviders filters the catalog by provider instead (w10/m573),
	// Devin's other filter type; one write names one of the two.
	EnabledProviders *[]string `json:"enabledProviders,omitempty" jsonschema:"filters the platform catalog models members may choose by provider instead of by model (admins only), as Devin's one filter type at a time: the workspace offers every catalog model whose id starts with <provider>/ for a listed provider, one added to the catalog later included, plus the catalog's default, which is always on; a model of any other provider is off, and a workspace that allowed all models no longer does. Names providers from the environment's providers (at least one); a write naming enabledModels or allowAllModels true too is refused with AGENT_ENVIRONMENT_MODELS_INVALID, and a later enabledModels write returns to the by-model list. Applies to the platform model route only"`
	// IdleTimeoutMinutes sets the workspace's own idle timeout (w6/m563).
	IdleTimeoutMinutes *int `json:"idleTimeoutMinutes,omitempty" jsonschema:"how long, in minutes, the sandbox of a session a person started stays up after its turn ends or its last editor disconnects (admins only, Enterprise plan only): 5 to 120, else AGENT_ENVIRONMENT_IDLE_TIMEOUT_INVALID; any other plan is refused with PLAN_FEATURE_REQUIRED and keeps 5. A closed session's next message runs in a fresh sandbox. Automations' and child sessions' sandboxes close when their turn ends whatever it says"`
	// AgentModelMonthlySpendCapSet is true when the field was present, so a
	// nil value with it set clears the override.
	AgentModelMonthlySpendCapSet bool `json:"-"`
	// AutomationsGitHubCLIEnabledSet is true when the automations default
	// was present, so a nil value with it set returns it to inherit.
	AutomationsGitHubCLIEnabledSet bool `json:"-"`
}

// AgentDataUse is one profile whose model provider may train on what its
// sessions send, and who acknowledged that and when (nil until someone did).
type AgentDataUse struct {
	Agent          string     `json:"agent"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	AcknowledgedBy *string    `json:"acknowledgedBy"`
}

// Agent availability refusals (w7/m392).
const (
	// AgentDisabledCode refuses a session of an agent the workspace's admins
	// turned off (or one that needs a data-use acknowledgement nobody gave).
	AgentDisabledCode = "AGENT_SESSION_AGENT_DISABLED"
	// AgentListInvalidCode refuses an unknown profile id in enabledAgents or
	// acknowledgeAgentDataUse, or an acknowledgement a profile does not need.
	AgentListInvalidCode = "AGENT_ENVIRONMENT_AGENT_INVALID"
	// DataUseUnacknowledgedCode refuses enabling a profile in agentDataUse
	// without its acknowledgement.
	DataUseUnacknowledgedCode = "AGENT_ENVIRONMENT_DATA_USE_UNACKNOWLEDGED"
	// ModelListInvalidCode refuses an enabledModels that turns the catalog's
	// default off (an empty list does), names a model outside the catalog, or
	// is sent where there is no catalog to narrow (w10/m527); an
	// enabledProviders that is empty, names a provider no catalog model
	// carries, is sent with enabledModels or where there is no catalog
	// (w10/m573); and an allowAllModels sent where there is no catalog, or
	// true beside either list (w10/m592).
	ModelListInvalidCode = "AGENT_ENVIRONMENT_MODELS_INVALID"
)

// agentEnabled reports whether members of the workspace env belongs to can
// start agent: a known profile its admins did not turn off and, if its
// provider may train on session data, whose acknowledgement is on record.
func agentEnabled(env store.AgentEnvironment, agent string) bool {
	profile, ok := agentsession.LookupAgentProfile(agent)
	if !ok || slices.Contains(env.DisabledAgents, profile.ID) {
		return false
	}
	_, acknowledged := env.DataUseAcks[profile.ID]
	return !profile.RequiresDataUseAcknowledgement || acknowledged
}

// agentAvailabilityPatch validates an enabledAgents / acknowledgeAgentDataUse
// update against the stored acknowledgements and fills patch: the turned-off
// list is every profile not enabled, so a profile added later starts on
// unless it needs an acknowledgement. An acknowledgement alone never turns an
// agent on: the agents on now are written as the list.
func agentAvailabilityPatch(current store.AgentEnvironment, input EnvironmentInput, subject string, patch *store.AgentEnvironmentPatch) error {
	acknowledged := map[string]bool{}
	for id := range current.DataUseAcks {
		acknowledged[id] = true
	}
	for _, agent := range input.AcknowledgeAgentDataUse {
		profile, ok := agentsession.LookupAgentProfile(agent)
		if !ok || !profile.RequiresDataUseAcknowledgement {
			return core.NewBadRequestError(AgentListInvalidCode,
				fmt.Sprintf("acknowledgeAgentDataUse: %q is not an agent profile that needs a data-use acknowledgement", agent),
				map[string]any{"field": "acknowledgeAgentDataUse", "agent": agent})
		}
		if !acknowledged[profile.ID] {
			patch.AcknowledgeDataUse = append(patch.AcknowledgeDataUse, profile.ID)
			acknowledged[profile.ID] = true
		}
	}
	if len(patch.AcknowledgeDataUse) > 0 {
		patch.AcknowledgedBy = subject
	}
	enabled := map[string]bool{}
	if input.EnabledAgents == nil {
		for _, id := range agentsession.AgentProfileIDs() {
			enabled[id] = agentEnabled(current, id)
		}
	}
	var enabledAgents []string
	if input.EnabledAgents != nil {
		enabledAgents = *input.EnabledAgents
	}
	for _, agent := range enabledAgents {
		profile, ok := agentsession.LookupAgentProfile(agent)
		if !ok {
			return core.NewBadRequestError(AgentListInvalidCode,
				fmt.Sprintf("enabledAgents: %q is not an agent profile", agent),
				map[string]any{"field": "enabledAgents", "agent": agent})
		}
		if profile.RequiresDataUseAcknowledgement && !acknowledged[profile.ID] {
			return core.NewBadRequestError(DataUseUnacknowledgedCode,
				fmt.Sprintf("the %s agent's model provider may train on what its sessions send; acknowledge that (acknowledgeAgentDataUse) to turn it on", profile.ID),
				map[string]any{"field": "enabledAgents", "agent": profile.ID})
		}
		enabled[profile.ID] = true
	}
	disabled := []string{}
	for _, id := range agentsession.AgentProfileIDs() {
		if !enabled[id] {
			disabled = append(disabled, id)
		}
	}
	patch.DisabledAgents = &disabled
	return nil
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

// ModelKeyRequiredCode refuses a session whose agent the workspace cannot run:
// not served by the platform model route and no model key of its own.
const ModelKeyRequiredCode = "AGENT_SESSION_MODEL_KEY_REQUIRED"

// AgentUnavailableCode refuses a platform-only agent (ADR108 D5) in a
// workspace no bex-owned model route serves it in: a workspace key never runs
// it.
const AgentUnavailableCode = "AGENT_SESSION_AGENT_UNAVAILABLE"

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

// markNullablePresence records whether the raw JSON object named the spend
// cap and the automations `gh` default, so an explicit null clears the one
// and returns the other to inherit instead of keeping them.
func (in *EnvironmentInput) markNullablePresence(raw []byte) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) == nil {
		_, in.AgentModelMonthlySpendCapSet = fields[spendCapField]
		_, in.AutomationsGitHubCLIEnabledSet = fields[automationsGitHubCLIField]
	}
}

// automationsGitHubCLIField is the wire name of the automations `gh` default.
const automationsGitHubCLIField = "automationsGithubCliEnabled"

// modelIDPattern bounds a model id that reaches a sandbox env var: provider
// ids (claude-sonnet-4-5, claude-sonnet-4-5[1m]) and OpenRouter slugs
// (deepseek/deepseek-v4.1-flash-1017), never whitespace or shell syntax.
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@~+\[\]-]{0,254}$`)

// Environment reads the workspace's agent environment. Any member may read
// it: it carries no secret, and the composer and the settings page both show
// it. Its spend figures are billing details, for the members who may view
// billing only (spendFor).
func (s *Service) Environment(ctx context.Context, ownerID string) (Environment, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanView, ownerID)
	if err != nil {
		return Environment{}, err
	}
	row, err := s.workspaceEnvironment(ctx, workspaceID)
	if err != nil {
		return Environment{}, err
	}
	configurable, err := s.idleTimeoutConfigurable(ctx, workspaceID)
	if err != nil {
		return Environment{}, err
	}
	view := s.withIdleTimeout(s.environmentView(row), row, configurable)
	if s.Spend == nil {
		return view, nil
	}
	status, err := s.readSpend(ctx, workspaceID)
	if err != nil {
		return Environment{}, err
	}
	return s.spendFor(ctx, workspaceID, view, status), nil
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
	patch, err := validateEnvironmentInput(input, s.agentTemplates())
	if err != nil {
		return Environment{}, err
	}
	idleRefusal, err := s.idleTimeoutRefusal(ctx, workspaceID)
	if err != nil {
		return Environment{}, err
	}
	if patch.IdleTimeoutMinutes != nil && idleRefusal != nil {
		return Environment{}, idleRefusal
	}
	keySet := key != "" // whether a key change leaves the workspace its own key
	// The model and agent lists and an automations `gh` default turning it on
	// are checked against the stored row, read only when the update names one
	// of them.
	agentsChanged := input.EnabledAgents != nil || len(input.AcknowledgeAgentDataUse) > 0
	automationsOn := isTrue(patch.AutomationsGitHubCLI)
	if input.DefaultModel != nil || input.AllowAllModels != nil || input.EnabledModels != nil || input.EnabledProviders != nil || agentsChanged || automationsOn {
		current, err := s.workspaceEnvironment(ctx, workspaceID)
		if err != nil {
			return Environment{}, err
		}
		if automationsOn {
			if err := automationsGitHubCLILocked(current, patch); err != nil {
				return Environment{}, err
			}
		}
		byoKey := current.ModelKeySet()
		if keyChanged {
			byoKey = keySet
		}
		if err := modelOfferPatch(current, input, byoKey, &patch); err != nil {
			return Environment{}, err
		}
		if agentsChanged {
			if err := agentAvailabilityPatch(current, input, core.SubjectFrom(ctx), &patch); err != nil {
				return Environment{}, err
			}
		}
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
		patch.ModelKeySet = &keySet
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
	view := s.withIdleTimeout(s.environmentView(row), row, idleRefusal == nil)
	if s.Spend == nil {
		return view, nil
	}
	return s.spendFor(ctx, workspaceID, view, status.WithOverride(row.SpendCapOverrideMicroUSD)), nil
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

func validateEnvironmentInput(input EnvironmentInput, templates []sandbox.AgentTemplate) (store.AgentEnvironmentPatch, error) {
	var patch store.AgentEnvironmentPatch
	if input.DefaultTemplate != nil {
		name := strings.TrimSpace(*input.DefaultTemplate)
		if name != "" && !hasTemplate(templates, name) {
			return patch, templateRefusal(CodeEnvironmentTemplateInvalid, "defaultTemplate", name, templates)
		}
		patch.DefaultTemplate = &name
	}
	if input.GitHubCLIEnabled != nil {
		disabled := !*input.GitHubCLIEnabled
		patch.GitHubCLIDisabled = &disabled
	}
	if input.ResetAutomationsGitHubCLI && input.AutomationsGitHubCLIEnabled != nil {
		return patch, core.NewBadRequestError(CodeEnvironmentGitHubCLIInvalid,
			"set automationsGithubCliEnabled or resetAutomationsGithubCli, not both", map[string]any{"field": automationsGitHubCLIField})
	}
	if input.AutomationsGitHubCLIEnabledSet || input.AutomationsGitHubCLIEnabled != nil || input.ResetAutomationsGitHubCLI {
		patch.SetAutomationsGitHubCLI, patch.AutomationsGitHubCLI = true, input.AutomationsGitHubCLIEnabled
	}
	patch.GitHubCLIMandatory = input.GitHubCLIMandatory
	if input.IdleTimeoutMinutes != nil {
		if err := validIdleTimeout(*input.IdleTimeoutMinutes); err != nil {
			return patch, err
		}
		patch.IdleTimeoutMinutes = input.IdleTimeoutMinutes
	}
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
		patch.DefaultModel = &model
	}
	return patch, nil
}

// modelOfferPatch checks an update's model fields against the platform
// catalog the workspace is on after it (byoKey: its own key then) and fills
// patch (w10/m527). allowAllModels is Devin's mode (w10/m592): true is "Allow
// all models", stored as no known models and none turned off, so a model
// added to the catalog later is on too; false on a workspace that allows all
// is "Allowlist specific models" of the models on now (w10/m568): the write
// records the catalog it saw, and a model added later is off until an admin
// turns it on. enabledModels replaces the turned-off list with every catalog
// model it leaves out and keeps the mode: an allowlist stays one even when it
// lists every model, and a workspace that allows all stays so unless a model
// is left out. It must keep the catalog's default and name only catalog
// models. Devin's other filter type, enabledProviders (w10/m573), lists
// providers instead: the write records them, the catalog it saw and, as the
// turned-off list, every model but the default of an unlisted provider, so a
// previous-release bex-api offers the same set. One write names one filter
// type, "Allow all models" names none, and every model field is refused off
// the route, where there is no catalog to narrow. defaultModel must be one of
// the models offered after the update. Turning off the model the stored
// default names is allowed, as Devin's allowlist overrides a pinned default:
// that default stops taking effect and the catalog's runs.
func modelOfferPatch(current store.AgentEnvironment, input EnvironmentInput, byoKey bool, patch *store.AgentEnvironmentPatch) error {
	catalog, routed := agentsession.PlatformModels(byoKey)
	after := current
	// setLists writes the three model columns together, into the patch and
	// the row defaultModel is checked against.
	setLists := func(disabled, known, providers []string) {
		patch.DisabledModels, patch.KnownModels, patch.EnabledProviders = &disabled, known, providers
		after.DisabledModels, after.KnownModels, after.EnabledProviders = disabled, known, providers
	}
	allowAll := input.AllowAllModels != nil && *input.AllowAllModels
	field := ""
	switch {
	case input.EnabledModels != nil && input.EnabledProviders != nil:
		return core.NewBadRequestError(ModelListInvalidCode,
			"set enabledModels or enabledProviders, not both: the models members may choose are filtered by model or by provider, one filter type at a time",
			map[string]any{"field": "enabledProviders", "fields": []string{"enabledModels", "enabledProviders"}})
	case input.EnabledModels != nil:
		field = "enabledModels"
	case input.EnabledProviders != nil:
		field = "enabledProviders"
	case input.AllowAllModels != nil:
		field = "allowAllModels"
	}
	if allowAll && field != "allowAllModels" {
		return core.NewBadRequestError(ModelListInvalidCode,
			"set allowAllModels: true or "+field+", not both: allowing all models offers every catalog model, one added to the catalog later included, and lists none",
			map[string]any{"field": "allowAllModels", "fields": []string{"allowAllModels", field}})
	}
	if field != "" && !routed {
		msg := field + " chooses the platform model catalog models members may pick, and this workspace runs agents on its own model key, where any model id is accepted"
		if !byoKey {
			msg = field + " chooses the platform model catalog models members may pick, which this bex-api does not offer"
		}
		return core.NewBadRequestError(ModelListInvalidCode, msg, map[string]any{"field": field})
	}
	if input.EnabledModels != nil {
		invalid := func(msg, model string) error {
			return core.NewBadRequestError(ModelListInvalidCode, msg,
				map[string]any{"field": "enabledModels", "model": model, "models": catalog})
		}
		enabled := map[string]bool{}
		for _, model := range *input.EnabledModels {
			model = strings.TrimSpace(model)
			if !slices.Contains(catalog, model) {
				return invalid(fmt.Sprintf("enabledModels: %q is not a platform catalog model; the catalog offers %s", model, strings.Join(catalog, ", ")), model)
			}
			enabled[model] = true
		}
		if !enabled[catalog[0]] {
			return invalid(fmt.Sprintf("enabledModels must keep %s, the platform catalog's default, on", catalog[0]), catalog[0])
		}
		disabled := []string{}
		for _, model := range catalog {
			if !enabled[model] {
				disabled = append(disabled, model)
			}
		}
		// The mode stays: only a workspace that allows all, and is not told
		// otherwise, keeps allowing all, and only while no model is left out.
		var known []string // nil: allow all
		if len(disabled) > 0 || input.AllowAllModels != nil || !allowsAll(current) {
			known = catalog
		}
		setLists(disabled, known, nil)
	}
	if allowAll || (field == "allowAllModels" && allowsAll(current)) {
		// true allows all; false on a workspace that allows all keeps the
		// models on now, the whole catalog, as the allowlist. false on an
		// allowlist or a filter by provider changes nothing.
		var known []string
		if !allowAll {
			known = catalog
		}
		setLists([]string{}, known, nil)
	}
	if input.EnabledProviders != nil {
		providers, err := listedProviders(catalog, *input.EnabledProviders)
		if err != nil {
			return err
		}
		setLists(providerFilterOff(catalog, providers), catalog, providers)
	}
	if input.DefaultModel != nil && *input.DefaultModel != "" && routed {
		if offered, _ := leftOn(catalog, after); !slices.Contains(offered, *input.DefaultModel) {
			return modelNotOfferedError(CodeEnvironmentModelNotOffered, "defaultModel", *input.DefaultModel, offered)
		}
	}
	return nil
}

// listedProviders checks an enabledProviders write against catalog's
// providers and returns the ones it lists, folded and in catalog order. It
// must list at least one: Devin's empty list means "not this filter type", so
// an empty filter would read two ways, and enabledModels naming the default
// alone offers only it.
func listedProviders(catalog, input []string) ([]string, error) {
	offered := catalogProviders(catalog)
	listed := map[string]bool{}
	for _, provider := range input {
		provider = strings.TrimSpace(provider)
		if !slices.Contains(offered, provider) {
			return nil, core.NewBadRequestError(ModelListInvalidCode,
				fmt.Sprintf("enabledProviders: %q is not a provider of the platform catalog, whose providers are %s", provider, strings.Join(offered, ", ")),
				map[string]any{"field": "enabledProviders", "provider": provider, "providers": offered})
		}
		listed[provider] = true
	}
	if len(listed) == 0 {
		return nil, core.NewBadRequestError(ModelListInvalidCode,
			fmt.Sprintf("enabledProviders must list at least one of the platform catalog's providers (%s); to offer only the catalog's default, set enabledModels to it", strings.Join(offered, ", ")),
			map[string]any{"field": "enabledProviders", "providers": offered})
	}
	return slices.DeleteFunc(offered, func(p string) bool { return !listed[p] }), nil
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
	view.TemplateOffer = s.templateOffer(row)
	view.Models, view.EnabledModels, view.NewModels, view.Providers = []string{}, []string{}, []string{}, []string{}
	// Nothing narrows the free-form model off the route (w10/m592).
	view.AllowAllModels = true
	switch models, ok := agentsession.PlatformModels(row.ModelKeySet()); {
	case row.ModelKeySet():
		view.ModelSource = ModelSourceBYO
	case ok:
		view.ModelSource = ModelSourcePlatform
		view.Models = models
		view.EnabledModels, view.NewModels = leftOn(models, row)
		view.AllowAllModels = allowsAll(row)
		view.Providers = catalogProviders(models)
		if listed, ok := providerFilter(row); ok {
			// The catalog's providers it lists, so a write built from the
			// read never names one the catalog dropped.
			view.EnabledProviders = slices.DeleteFunc(slices.Clone(view.Providers), func(p string) bool { return !slices.Contains(listed, p) })
		}
		view.ModelPinned = true
		view.PinnedModel = &models[0]
	default:
		view.ModelSource = ModelSourceNone
	}
	view.ModelKeySet = view.ModelSource == ModelSourceBYO
	view.GitHubCLIEnabled = !row.GitHubCLIDisabled
	view.AutomationsGitHubCLIEnabled, view.GitHubCLIMandatory = row.AutomationsGitHubCLI, row.GitHubCLIMandatory
	view.EnabledAgents, view.AgentDataUse = []string{}, []AgentDataUse{}
	for _, id := range agentsession.AgentProfileIDs() {
		if agentEnabled(row, id) {
			view.EnabledAgents = append(view.EnabledAgents, id)
		}
		if profile, _ := agentsession.LookupAgentProfile(id); profile.RequiresDataUseAcknowledgement {
			use := AgentDataUse{Agent: id}
			if ack, ok := row.DataUseAcks[id]; ok {
				at, by := ack.At.UTC(), ack.By
				use.AcknowledgedAt, use.AcknowledgedBy = &at, &by
			}
			view.AgentDataUse = append(view.AgentDataUse, use)
		}
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

// SandboxTemplate is an agent sandbox template a session can name
// (agentConfig.template, w10/m519): its name, CPU cores and memory in GB.
type SandboxTemplate struct {
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`
	MemoryGB float64 `json:"memoryGb"`
}

// TemplateOffer is what the environment and capabilities reads say about
// sandbox templates (w10/m519). DefaultTemplate is the one a session or an
// automation run that names none runs on: the workspace's while it is
// registered, else sandbox.DefaultAgentTemplate. Templates are every one a
// session may name, smallest first.
type TemplateOffer struct {
	DefaultTemplate string            `json:"defaultTemplate"`
	Templates       []SandboxTemplate `json:"templates"`
}

// CodeEnvironmentTemplateInvalid refuses a defaultTemplate that is not a
// registered agent template; its params list the templates.
const CodeEnvironmentTemplateInvalid = "AGENT_ENVIRONMENT_TEMPLATE_INVALID"

// agentTemplates are the templates agentConfig.template and defaultTemplate
// can name, smallest first; none without a sandbox lifecycle.
func (s *Service) agentTemplates() []sandbox.AgentTemplate {
	if s.Sandbox == nil {
		return nil
	}
	return s.Sandbox.AgentTemplates()
}

func hasTemplate(templates []sandbox.AgentTemplate, name string) bool {
	return slices.ContainsFunc(templates, func(t sandbox.AgentTemplate) bool { return t.Name == name })
}

// templateRefusal refuses name in field, listing the templates with their
// sizes so the caller can pick one.
func templateRefusal(code, field, name string, templates []sandbox.AgentTemplate) error {
	names, sizes := make([]string, len(templates)), make([]string, len(templates))
	for i, t := range templates {
		names[i], sizes[i] = t.Name, t.Name+" ("+t.Size()+")"
	}
	return core.NewBadRequestError(code,
		fmt.Sprintf("%s %q is not an agent template; use one of %s", field, name, strings.Join(sizes, ", ")),
		map[string]any{"field": field, "template": name, "templates": names})
}

// sessionTemplate is the template a session of env's workspace that names
// none runs on: the workspace default while it is registered, else
// sandbox.DefaultAgentTemplate, so a default whose template was retired
// never fails a session.
func sessionTemplate(env store.AgentEnvironment, templates []sandbox.AgentTemplate) string {
	if hasTemplate(templates, env.DefaultTemplate) {
		return env.DefaultTemplate
	}
	return sandbox.DefaultAgentTemplate
}

// templateOffer is env's workspace's TemplateOffer.
func (s *Service) templateOffer(env store.AgentEnvironment) TemplateOffer {
	templates := s.agentTemplates()
	offer := TemplateOffer{DefaultTemplate: sessionTemplate(env, templates), Templates: make([]SandboxTemplate, len(templates))}
	for i, t := range templates {
		offer.Templates[i] = SandboxTemplate{Name: t.Name, CPU: t.Cores(), MemoryGB: t.MemoryGB()}
	}
	return offer
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

// The typed refusals of a model the platform route's catalog does not offer
// (w10/m516): at session create, on a message and in an automation's template
// (agentConfig.model), and for the workspace default (defaultModel). Their
// params name the refused model and the offered ones, so a client can show
// the choice instead of a 403 from the gateway mid-turn.
const (
	CodeSessionModelNotOffered     = "AGENT_SESSION_MODEL_NOT_OFFERED"
	CodeEnvironmentModelNotOffered = "AGENT_ENVIRONMENT_MODEL_NOT_OFFERED"
)

// offeredModels is what a workspace's claude sessions may choose from on the
// platform model route (w10/m527): the catalog models its list allows
// (leftOn), in catalog order with its default first, and whether the
// workspace is on the route at all (never with its own key, where any model is
// free-form). Every place that offers or checks a stored workspace's model
// takes it from here; an update checks the list it is about to store
// (modelOfferPatch).
func offeredModels(env store.AgentEnvironment) ([]string, bool) {
	catalog, routed := agentsession.PlatformModels(env.ModelKeySet())
	offered, _ := leftOn(catalog, env)
	return offered, routed
}

// inPlatformCatalog reports whether model is one of the platform route's
// catalog models, whatever any workspace turned off: a model a workspace does
// not offer is either turned off there (true) or no catalog model at all.
func inPlatformCatalog(model string) bool {
	catalog, _ := agentsession.PlatformModels(false)
	return slices.Contains(catalog, model)
}

// leftOn is what env's stored list allows of catalog, as new slices in
// catalog order: on, the models members may choose, and added, the ones off
// only because they are new in the catalog since an admin narrowed the list
// (w10/m568). Devin's rule: a model is allowed when the allowlist names it or
// it carries a listed provider tag. Under a filter by provider in force
// (providerFilter, w10/m573) every model of a listed provider is on, a later
// one included, and none is new. Otherwise the allowlist is KnownModels, the
// catalog the narrowing write saw, less DisabledModels, even when that turns
// none off (w10/m592). A list with no known models ("Allow all models", or
// narrowed under migration 0315 and not yet converted) allows every catalog
// model but the turned-off ones, as the opt-out did. The catalog's default
// is never off, even by a list stored before the catalog's default changed.
func leftOn(catalog []string, env store.AgentEnvironment) (on, added []string) {
	providers, byProvider := providerFilter(env)
	on, added = make([]string, 0, len(catalog)), []string{}
	for i, model := range catalog {
		switch {
		case i == 0:
			on = append(on, model)
		case byProvider:
			if slices.Contains(providers, modelProvider(model)) {
				on = append(on, model)
			}
		case slices.Contains(env.DisabledModels, model):
		case env.KnownModels == nil || slices.Contains(env.KnownModels, model):
			on = append(on, model)
		default:
			added = append(added, model)
		}
	}
	return on, added
}

// allowsAll reports whether env is in Devin's "Allow all models" mode
// (w10/m592): no known models and none turned off, so every catalog model is
// offered, one added later included. A row narrowed under migration 0315 and
// not yet converted turns some off and does not.
func allowsAll(env store.AgentEnvironment) bool {
	return env.KnownModels == nil && len(env.DisabledModels) == 0
}

// modelProvider is a catalog model's provider (w10/m573), Devin's provider
// tag: its id's prefix before the first "/", "" for an id with none, which
// no filter by provider allows.
func modelProvider(model string) string {
	provider, _, ok := strings.Cut(model, "/")
	if !ok {
		return ""
	}
	return provider
}

// catalogProviders are the providers of catalog's models, in catalog order.
func catalogProviders(catalog []string) []string {
	providers := []string{}
	for _, model := range catalog {
		if p := modelProvider(model); p != "" && !slices.Contains(providers, p) {
			providers = append(providers, p)
		}
	}
	return providers
}

// providerFilterOff is the turned-off list a filter by provider stores for
// catalog: every model but the default whose provider is not listed. A
// previous-release bex-api, which knows only the turned-off list, offers the
// same set under that catalog.
func providerFilterOff(catalog, providers []string) []string {
	off := []string{}
	for i, model := range catalog {
		if i > 0 && !slices.Contains(providers, modelProvider(model)) {
			off = append(off, model)
		}
	}
	return off
}

// providerFilter is env's filter by provider and whether it is in force: only
// while its turned-off list is still the one its write derived from the
// catalog it saw. A previous-release bex-api's enabledModels write changes
// that list and leaves enabled_providers in place; the row then reads by model,
// as that bex-api reads it.
func providerFilter(env store.AgentEnvironment) ([]string, bool) {
	if env.EnabledProviders == nil || env.KnownModels == nil {
		return nil, false
	}
	want, got := providerFilterOff(env.KnownModels, env.EnabledProviders), slices.Clone(env.DisabledModels)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		return nil, false
	}
	return env.EnabledProviders, true
}

// LegacyModelListStore converts the model lists narrowed under migration
// 0315's opt-out; *store.PGStore satisfies it.
type LegacyModelListStore interface {
	ConvertLegacyModelLists(ctx context.Context, catalog []string) (int64, error)
}

// ConvertLegacyModelLists makes every workspace model list narrowed under
// the opt-out an allowlist of the platform catalog this bex-api runs
// (w10/m568), so its offer is unchanged and a model added to the catalog
// later is off there. It reports how many rows it converted and whether it
// ran: off the platform route there is no catalog to convert with, and the
// rows keep reading as the opt-out. A second run converts none.
func ConvertLegacyModelLists(ctx context.Context, st LegacyModelListStore) (int64, bool, error) {
	catalog, routed := agentsession.PlatformModels(false)
	if !routed {
		return 0, false, nil
	}
	n, err := st.ConvertLegacyModelLists(ctx, catalog)
	return n, true, err
}

// modelNotOfferedError refuses model in field, listing the models offered.
// One the catalog has but the workspace's admins turned off says so.
func modelNotOfferedError(code, field, model string, models []string) error {
	msg := fmt.Sprintf("this workspace runs agents on the platform model route, which offers %s; %q is not one of them", strings.Join(models, ", "), model)
	if inPlatformCatalog(model) {
		msg = fmt.Sprintf("%s is turned off in this workspace, whose members may choose %s; a workspace admin can turn it on in Settings → Agents", model, strings.Join(models, ", "))
	}
	return core.NewBadRequestError(code, msg, map[string]any{"field": field, "model": model, "models": models})
}

// modelPinnedError refuses an explicit model other than the one a profile
// with a release-locked model runs (muse, ADR108 D4).
func modelPinnedError(message, pinned string) error {
	return core.NewBadRequestError("AGENT_SESSION_MODEL_PINNED", message,
		map[string]any{"field": "agentConfig.model", "pinnedModel": pinned})
}

// sessionModel resolves the model a session runs: the request's explicit
// model, else the workspace default. The claude session of a workspace on
// the platform route (one without its own model key) runs a model of the
// route's catalog (w10/m516) that the workspace's admins left on
// (offeredModels, w10/m527): an explicit model not offered is refused
// (AGENT_SESSION_MODEL_NOT_OFFERED) instead of reaching the gateway's
// per-session pin as a 403 mid-turn, and no model takes the workspace default
// while it is offered, else the catalog's first. Where the caller chose no
// model (a steer, a queued message delivered later), the stored model is kept
// while it is still offered and otherwise falls back the same way, so a model
// removed from the catalog or turned off never strands a session. A session created
// on the route before the workspace set its own key drops its catalog model
// for the workspace default (empty: the agent profile's), which its own
// provider serves.
func sessionModel(config AgentConfig, env store.AgentEnvironment, explicit bool) (string, error) {
	// A profile with a release-locked model (ADR108 D4, muse) runs only it,
	// whatever the workspace default.
	if profile, _ := agentsession.LookupAgentProfile(config.Agent); profile.Model != "" {
		if explicit && config.Model != "" && config.Model != profile.Model {
			return "", modelPinnedError(fmt.Sprintf("the %s agent runs only model %s; omit agentConfig.model or set it to that model", profile.ID, profile.Model), profile.Model)
		}
		return profile.Model, nil
	}
	pinnedAgent := agentsession.PinnedAgent(config.Agent)
	if models, ok := offeredModels(env); pinnedAgent && ok {
		switch {
		case config.Model != "" && slices.Contains(models, config.Model):
			return config.Model, nil
		case explicit && config.Model != "":
			return "", modelNotOfferedError(CodeSessionModelNotOffered, "agentConfig.model", config.Model, models)
		case slices.Contains(models, env.DefaultModel):
			return env.DefaultModel, nil
		}
		return models[0], nil
	}
	if pinnedAgent && !explicit && inPlatformCatalog(config.Model) {
		return env.DefaultModel, nil
	}
	if config.Model != "" {
		return config.Model, nil
	}
	return env.DefaultModel, nil
}

// checkModelID refuses a session's model that is not a model id: one that
// reaches a sandbox env var must hold no spaces or shell syntax.
func checkModelID(field, model string) error {
	if model != "" && !modelIDPattern.MatchString(model) {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", field+" must be a model id without spaces", map[string]any{"field": field})
	}
	return nil
}

// steerModel checks the model a message switches its session to (w10/m516)
// against the session's agent and its workspace now, so a model the session
// could not run is a 400 at send, never a queued message that cannot start.
// It returns the trimmed model, "" when the message names none.
func (s *Service) steerModel(ctx context.Context, record store.AgentSession, model string) (string, error) {
	if model = strings.TrimSpace(model); model == "" {
		return "", nil
	}
	if err := checkModelID("model", model); err != nil {
		return "", err
	}
	config, err := decodeAgentConfig(record)
	if err != nil {
		return "", err
	}
	env, err := s.workspaceEnvironment(ctx, record.WorkspaceID)
	if err != nil {
		return "", err
	}
	config.Model = model
	if _, err := sessionModel(config, env, true); err != nil {
		var coded *core.CodedError
		if errors.As(err, &coded) && coded.Params != nil {
			coded.Params["field"] = "model"
		}
		return "", err
	}
	return model, nil
}

// agentModels is what the capabilities read offers for agent in a workspace
// (w10/m516): the model a session that names none runs, and the models one
// may name. A profile with a fixed model offers only it; the claude profile on
// the platform route offers the catalog models the workspace's admins left on
// (offeredModels, w10/m527); elsewhere any model the workspace's
// own provider serves is accepted (no list), and only the claude profile reads
// the workspace default.
func agentModels(agent string, env store.AgentEnvironment) (*string, []string) {
	if profile, _ := agentsession.LookupAgentProfile(agent); profile.Model != "" {
		return &profile.Model, []string{profile.Model}
	}
	if !agentsession.PinnedAgent(agent) {
		return nil, []string{}
	}
	models, routed := offeredModels(env)
	model, _ := sessionModel(AgentConfig{Agent: agent}, env, false)
	if !routed {
		models = []string{}
	}
	if model == "" {
		return nil, models
	}
	return &model, models
}
