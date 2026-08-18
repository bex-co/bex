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

package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/drivergrant"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	metadataOwner         = "bex.co/owner"
	metadataWorkspace     = "bex.co/workspace"
	metadataNetworkPolicy = "bex.co/network-policy"
	metadataPlan          = "bex.co/plan"
	metadataRegion        = "bex.co/region"
	metadataTimeout       = "bex.co/timeout-seconds"
	metadataComputeWeight = "bex.co/compute-weight-milli"
	metadataTemplate      = "bex.co/template"
	metadataRegime        = "app.bex.co/regime"
	metadataSandboxRegime = "sandbox"
	metadataAgentSession  = agentsession.LabelSession
	minSandboxTimeout     = 60
	maxSandboxTimeout     = 86400
)

// KeyProvider mints/looks up a workspace's OpenSandbox tenant key (OSEP-0014):
// the value bex-api sends as OPEN-SANDBOX-API-KEY so the server scopes lifecycle
// ops to that workspace's `<ws>-sandbox` namespace (ADR042 D4, m32 t006).
// Implemented by the bex-api key store; nil => no key (dev/single-tenant), in
// which case the server must run single-tenant.
type KeyProvider interface {
	WorkspaceKey(ctx context.Context, workspaceID string) (string, error)
}

// Template fixes a sandbox's image, entrypoint, and resource limits at
// registration time — bex's Create is template-based (ADR014 D2), never an
// arbitrary caller image. The OpenSandbox server requires an entrypoint when an
// image is given and resource limits when no pool is referenced (validated live).
type Template struct {
	Image      string
	Entrypoint []string
	CPU        string
	Memory     string
}

// SessionEgress is the network-mechanism seam used by the agent-session
// lifecycle (ADR047 D5). Implementations must install setup policy before Pod
// creation, make the setup→agent transition one-way, and leave the structural
// sandbox default-deny in place during cleanup.
type SessionEgress interface {
	PrepareSetup(ctx context.Context, namespace, sessionID, modelEndpoint string, extra []string) error
	TransitionToAgent(ctx context.Context, namespace, sessionID, modelEndpoint string, extra []string) error
	Delete(ctx context.Context, namespace, sessionID string) error
}

// Service is the authorized sandbox feature. It is stateless: the per-workspace
// tenant key scopes OpenSandbox's own list to the workspace, so OpenSandbox is
// the source of truth and bex keeps no sandbox table (ADR042 D4). Client nil =>
// every verb returns ErrSandboxesUnavailable (BEX_OPENSANDBOX_URL unset).
type Service struct {
	*core.Base
	Client      *Client
	Keys        KeyProvider
	Templates   map[string]Template
	DefaultPlan Plan
	// DefaultTemplate is used when a create names no template — the Render CLI's
	// `ea sandbox create` sends only a plan (no template flag exists), so an empty
	// template must resolve to a registered default rather than 400 (w3/m32 t009).
	DefaultTemplate string
	// Exec wires `render ea sandbox exec` (w3/m33); nil => the exec verb 503s.
	Exec *ExecConfig
	// Meter records durable lifecycle observations. nil keeps the pre-metering
	// path byte-identical for store-off deployments.
	Meter *Meter
	// SessionEgress is required only for agent-session lifecycle mechanics.
	// Ordinary Render
	// sandboxes retain the m35 compatibility policy when it is nil.
	SessionEgress SessionEgress
}

// CreateRequest is the caller's create input. OwnerID binds the workspace (as
// every create does); Template selects a registered image (empty ⇒ the default,
// since the Render CLI sends no template); Plan/Region/TimeoutSeconds/NetworkPolicy
// are Render CLI create fields echoed back on the resource.
type CreateRequest struct {
	OwnerID        string
	Template       string
	Plan           Plan
	Region         string
	TimeoutSeconds int
	NetworkPolicy  *NetworkPolicy
}

func (s *Service) enabled() bool { return s.Client != nil }

// workspaceKey resolves the caller's workspace tenant key. Empty when no key
// provider is wired (single-tenant OpenSandbox). It keys off the RESOLVED
// workspace (s.Tenant — the caller's default when they named none, or the named
// one after the membership check), never the raw named value: a create with no
// ownerId must mint the key for the caller's own workspace, not an empty one.
func (s *Service) workspaceKey(ctx context.Context) (string, error) {
	if s.Keys == nil {
		return "", nil
	}
	ws, ok := s.Tenant(ctx)
	if !ok {
		// Multi-tenant OpenSandbox but the caller resolves to no workspace (store
		// off, or an unbound machine key): there is no `<ws>-sandbox` to scope to.
		return "", fmt.Errorf("%w: no workspace resolved for the sandbox tenant key", core.ErrForbidden)
	}
	return s.Keys.WorkspaceKey(ctx, ws)
}

func normalizeNetworkPolicy(policy *NetworkPolicy) (*NetworkPolicy, error) {
	if policy == nil || policy.Default == "" {
		return &NetworkPolicy{Default: NetworkPolicyDenyAll}, nil
	}
	if policy.Default != NetworkPolicyDenyAll {
		return nil, core.NewBadRequestError(
			"SANDBOX_NETWORK_POLICY_UNSUPPORTED",
			fmt.Sprintf("sandbox networkPolicy.default %q is unsupported; only %q is enforced", policy.Default, NetworkPolicyDenyAll),
			map[string]any{"field": "networkPolicy.default", "supported": []string{string(NetworkPolicyDenyAll)}},
		)
	}
	return &NetworkPolicy{Default: NetworkPolicyDenyAll}, nil
}

func validateCreateMetadata(region string, timeout int) error {
	if region != "" && len(validation.IsValidLabelValue(region)) != 0 {
		return core.NewBadRequestError(
			"SANDBOX_REGION_INVALID",
			"sandbox region must be a Kubernetes label-safe value",
			map[string]any{"field": "region"},
		)
	}
	if timeout < 0 || (timeout > 0 && timeout < minSandboxTimeout) || timeout > maxSandboxTimeout {
		return core.NewBadRequestError(
			"SANDBOX_TIMEOUT_INVALID",
			fmt.Sprintf("sandbox timeoutSeconds must be 0 (no expiry) or between %d and %d", minSandboxTimeout, maxSandboxTimeout),
			map[string]any{"field": "timeoutSeconds", "minimumNonZero": minSandboxTimeout, "max": maxSandboxTimeout},
		)
	}
	return nil
}

// ownerID is stable and Kubernetes-label-safe because OpenSandbox persists
// request metadata as labels. Normal identity ids round-trip unchanged; an
// unusual subject is represented by a non-reversible digest instead of being
// rejected or written into metadata in an invalid form.
func ownerID(ctx context.Context) string {
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return "local"
	}
	if len(validation.IsValidLabelValue(id.Subject)) == 0 {
		return id.Subject
	}
	sum := sha256.Sum256([]byte(id.Subject))
	return "subject-" + hex.EncodeToString(sum[:20])
}

func sandboxMetadata(ctx context.Context, workspace, template string, plan Plan, region string, timeout int, policy *NetworkPolicy, weight int64) map[string]string {
	metadata := map[string]string{
		metadataOwner:         ownerID(ctx),
		metadataWorkspace:     workspace,
		metadataNetworkPolicy: string(policy.Default),
		metadataPlan:          string(plan),
		metadataTimeout:       strconv.Itoa(timeout),
		metadataTemplate:      template,
		metadataRegime:        metadataSandboxRegime,
		metadataComputeWeight: strconv.FormatInt(weight, 10),
	}
	if region != "" {
		metadata[metadataRegion] = region
	}
	return metadata
}

func sandboxNotFound(id string) error {
	return core.NewNotFoundError("SANDBOX_NOT_FOUND", "sandbox not found", map[string]any{"id": id})
}

// isWorkspaceAdmin answers the cross-owner admin override with an authoritative
// (uncached) decision whenever the wired checker supports core.FreshChecker
// (round-11 #5): the override gates exec-ticket minting and lifecycle mutation
// in another member's sandbox, so a just-demoted admin must not ride a positive
// cached on a different API replica for up to core.PositiveTTL. An uncached
// checker is already authoritative and degrades to plain Check.
func (s *Service) isWorkspaceAdmin(ctx context.Context, workspace string) (bool, error) {
	if s.Authz == nil {
		return false, nil
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return false, core.ErrForbidden
	}
	check := s.Authz.Check
	if fresh, ok := s.Authz.(core.FreshChecker); ok {
		check = fresh.CheckFresh
	}
	allowed, err := check(ctx, "user:"+id.Subject, core.RelCanManage, core.WorkspaceObject(workspace))
	if err != nil {
		return false, fmt.Errorf("%w: %v", core.ErrAuthzUnavailable, err)
	}
	return allowed, nil
}

func validOwnedSandbox(raw osSandbox, workspace string) bool {
	return raw.ID != "" &&
		raw.Metadata[metadataOwner] != "" &&
		raw.Metadata[metadataWorkspace] == workspace &&
		raw.Metadata[metadataRegime] == metadataSandboxRegime &&
		raw.Metadata[metadataNetworkPolicy] == string(NetworkPolicyDenyAll)
}

func (s *Service) mayAccessSandbox(ctx context.Context, workspace string, raw osSandbox) (bool, error) {
	if !validOwnedSandbox(raw, workspace) {
		return false, nil // legacy, foreign, or incompletely hardened metadata fails closed
	}
	if raw.Metadata[metadataOwner] == ownerID(ctx) {
		return true, nil
	}
	return s.isWorkspaceAdmin(ctx, workspace)
}

// ownedSandbox resolves one durable OpenSandbox object and applies the shared
// owner/admin boundary used by reads, lifecycle verbs, and exec. Returning the
// same named not-found for absent, foreign, legacy, or incompletely hardened
// objects prevents every adapter from becoming an existence oracle.
func (s *Service) ownedSandbox(ctx context.Context, key, workspace, id string) (osSandbox, error) {
	raw, err := s.Client.Get(ctx, key, id)
	if err != nil {
		if errors.Is(err, errOpenSandboxNotFound) {
			return osSandbox{}, sandboxNotFound(id)
		}
		return osSandbox{}, err
	}
	allowed, err := s.mayAccessSandbox(ctx, workspace, raw)
	if err != nil {
		return osSandbox{}, err
	}
	if !allowed {
		return osSandbox{}, sandboxNotFound(id)
	}
	s.Meter.Observe(ctx, raw)
	return raw, nil
}

func sandboxFromOpenSandbox(raw osSandbox, workspace string) Sandbox {
	policy := &NetworkPolicy{Default: NetworkPolicyDefault(raw.Metadata[metadataNetworkPolicy])}
	if policy.Default == "" {
		policy = nil
	}
	timeout, _ := strconv.Atoi(raw.Metadata[metadataTimeout])
	return Sandbox{
		ID:             raw.ID,
		Plan:           Plan(raw.Metadata[metadataPlan]),
		Status:         mapOpenSandboxStatus(raw.Status.State),
		Region:         raw.Metadata[metadataRegion],
		TimeoutSeconds: timeout,
		NetworkPolicy:  policy,
		Owner:          raw.Metadata[metadataOwner],
		Workspace:      raw.Metadata[metadataWorkspace],
		CreatedAt:      raw.Created,
		Image:          raw.Image.URI,
	}
}

// Create authorizes, resolves the template, and starts a sandbox scoped to the
// caller's workspace.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Sandbox, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Sandbox{}, err
	}
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	plan := req.Plan
	if plan == "" {
		plan = s.DefaultPlan
	}
	if plan != "" && !ValidPlan(plan) {
		return Sandbox{}, fmt.Errorf("%w: unknown plan %q", core.ErrBadRequest, plan)
	}
	if err := validateCreateMetadata(req.Region, req.TimeoutSeconds); err != nil {
		return Sandbox{}, err
	}
	policy, err := normalizeNetworkPolicy(req.NetworkPolicy)
	if err != nil {
		return Sandbox{}, err
	}
	name := req.Template
	if name == "" {
		name = s.DefaultTemplate // Render CLI create sends no template — use the default.
	}
	tmpl, ok := s.Templates[name]
	if !ok {
		return Sandbox{}, fmt.Errorf("%w: unknown template %q", core.ErrBadRequest, name)
	}
	ws := s.WorkspaceOrDefault(ctx)
	return s.createResolved(ctx, ws, name, tmpl, plan, req.Region, req.TimeoutSeconds, policy, nil, nil)
}

func (s *Service) createResolved(ctx context.Context, workspace, template string, tmpl Template, plan Plan, region string, timeout int, policy *NetworkPolicy, env, extraMetadata map[string]string) (Sandbox, error) {
	cpu, mem := tmpl.CPU, tmpl.Memory
	if cpu == "" {
		cpu = "500m"
	}
	if mem == "" {
		mem = "512Mi"
	}
	weight, err := computeWeightMilli(cpu, mem)
	if err != nil {
		return Sandbox{}, fmt.Errorf("sandbox template resources: %w", err)
	}
	if err := s.RequirePaymentMethod(ctx, workspace); err != nil {
		return Sandbox{}, err
	}
	// codex #6: enforce the billing lifecycle gate — a delinquent/enforced
	// workspace must not start new sandbox compute, not just lack a payment method.
	if err := s.RequireBillingMutation(ctx, workspace); err != nil {
		return Sandbox{}, err
	}
	key := ""
	if s.Keys != nil {
		key, err = s.Keys.WorkspaceKey(ctx, workspace)
		if err != nil {
			return Sandbox{}, err
		}
	}
	entry := tmpl.Entrypoint
	if len(entry) == 0 {
		entry = []string{"sleep", "infinity"}
	}
	metadata := sandboxMetadata(ctx, workspace, template, plan, region, timeout, policy, weight)
	for k, v := range extraMetadata {
		metadata[k] = v
	}
	raw, err := s.Client.Create(ctx, key, tmpl.Image, entry, cpu, mem, timeout, env, metadata)
	if err != nil {
		return Sandbox{}, err
	}
	if raw.Metadata == nil {
		raw.Metadata = make(map[string]string, len(metadata))
	}
	// The create response is informational; use the exact server-bound values
	// even when an OpenSandbox version omits or partially echoes metadata. Durable
	// list/get authorization still reads the separately persisted upstream copy.
	for key, value := range metadata {
		raw.Metadata[key] = value
	}
	if raw.Image.URI == "" {
		raw.Image.URI = tmpl.Image
	}
	s.Meter.Observe(ctx, raw)
	return sandboxFromOpenSandbox(raw, workspace), nil
}

// SnapshotResult is the outcome of a hibernation snapshot (ADR059 D3): the
// archive's size (storage-metering + quota dimension) and sha256 (integrity),
// plus whether the workspace's git tree was dirty (uncommitted work), which
// extends the retention window so unpushed edits survive longer.
type SnapshotResult struct {
	Bytes    int64
	SHA256   string
	DirtyGit bool
}

// AgentSessionLifecycle is intentionally separate from Service's public API
// verb method set. Its caller (agentsessions.Service) owns authorization and
// audit; exposing these mechanics as sandbox verbs would create a second policy
// entry point and make the repository's verb guard correctly reject them.
type AgentSessionLifecycle struct{ service *Service }

func NewAgentSessionLifecycle(service *Service) *AgentSessionLifecycle {
	if service == nil {
		return nil
	}
	return &AgentSessionLifecycle{service: service}
}

// ModelAPIKeyEnvVar is the exact env var name the agent-image driver reads its
// BYO model provider credential from (lego/agent-image/driver/src/config.mjs,
// ADR047 D7). The tenant secret's OpenBao value must be stored under this same
// key so the fetch-to-injection path needs no translation.
const ModelAPIKeyEnvVar = agentsession.ModelKeyField

// CreateAgentSessionSandbox is the narrow trusted lifecycle seam used by the
// agent-sessions feature after it has performed the first-class session FGA
// check. It preserves every reserved sandbox hardening stamp and adds the
// durable session id, without re-checking can_create (session creation is
// intentionally can_operate per ADR047 D3).
func (l *AgentSessionLifecycle) CreateAgentSessionSandbox(ctx context.Context, workspaceID, template, sessionID, repository, branch, modelEndpoint, modelAPIKey string, egressAllowlist []string, driverEnv map[string]string) (Sandbox, error) {
	s := l.service
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	if s.SessionEgress == nil {
		return Sandbox{}, core.ErrAgentSessionsUnavailable
	}
	if workspaceID == "" || sessionID == "" {
		return Sandbox{}, fmt.Errorf("%w: workspace and agent session id are required", core.ErrBadRequest)
	}
	if !agentsession.IsModelKeyPlaceholder(modelAPIKey, sessionID) {
		return Sandbox{}, fmt.Errorf("%w: agent session model-proxy placeholder is required", core.ErrAgentSessionsUnavailable)
	}
	if template == "" {
		template = "agent"
	}
	tmpl, ok := s.Templates[template]
	if !ok {
		return Sandbox{}, fmt.Errorf("%w: unknown agent sandbox template %q", core.ErrBadRequest, template)
	}
	plan := s.DefaultPlan
	if plan == "" {
		plan = PlanStarter
	}
	bindings, err := agentsession.BindingLabels(sessionID, repository, branch)
	if err != nil {
		return Sandbox{}, fmt.Errorf("%w: invalid agent session binding: %v", core.ErrBadRequest, err)
	}
	namespace := store.SandboxNamespace(workspaceID)
	if err := s.SessionEgress.PrepareSetup(ctx, namespace, sessionID, modelEndpoint, egressAllowlist); err != nil {
		return Sandbox{}, err
	}
	policy := &NetworkPolicy{Default: NetworkPolicyDenyAll}
	// The egress allowlist (a JSON array) and model endpoint (a URL) are NOT
	// stamped as sandbox metadata: OpenSandbox persists metadata as Kubernetes
	// labels, and `[]`/`https://…` are invalid label values (a `[]` empty
	// allowlist is rejected outright — caught live on prod, w3/m43). The phase
	// transition instead receives both values directly from the caller
	// (EnterAgentSessionPhase), which already holds them, so nothing needs to
	// round-trip through a label.
	// driverEnv carries the non-secret run env (prompt, agent, branch, repo,
	// delivery toggle — ADR047 D4). modelAPIKey is required to be the exact
	// session-bound proxy placeholder above; the reusable BYO key never enters a
	// sandbox Pod spec. Metadata lands in status reads and audit, so neither value
	// is stamped there.
	var env map[string]string
	if len(driverEnv) > 0 || modelAPIKey != "" {
		env = map[string]string{}
		for k, v := range driverEnv {
			env[k] = v
		}
		if modelAPIKey != "" {
			env[ModelAPIKeyEnvVar] = modelAPIKey
		}
	}
	sb, err := s.createResolved(ctx, workspaceID, template, tmpl, plan, "", 0, policy, env, bindings)
	if err != nil {
		if cleanupErr := s.SessionEgress.Delete(ctx, namespace, sessionID); cleanupErr != nil {
			return Sandbox{}, errors.Join(err, fmt.Errorf("rollback session egress policy: %w", cleanupErr))
		}
		return Sandbox{}, err
	}
	return sb, nil
}

// EnterAgentSessionPhase narrows one exact session sandbox from setup registry
// access to its immutable agent baseline. Durable sandbox metadata, rather than
// transition-call input, supplies the model endpoint and tenant widening.
func (l *AgentSessionLifecycle) EnterAgentSessionPhase(ctx context.Context, workspaceID, sessionID, sandboxID, modelEndpoint string, egressAllowlist []string) error {
	s := l.service
	if s.SessionEgress == nil {
		return core.ErrAgentSessionsUnavailable
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	// Re-validate that the sandbox is the exact one bound to this session before
	// narrowing its egress (target integrity), but take the model endpoint and
	// allowlist from the caller — they are not (and cannot be) stored as labels.
	if _, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID); err != nil {
		return err
	}
	return s.SessionEgress.TransitionToAgent(ctx, store.SandboxNamespace(workspaceID), sessionID, modelEndpoint, egressAllowlist)
}

// ResumeAgentSessionSandbox resumes only a fully hardened sandbox stamped for
// this exact durable session. Workspace membership is not used as a substitute
// authorization decision here; the caller feature already checked
// agent_session:<id> in OpenFGA and this seam only enforces target integrity.
func (l *AgentSessionLifecycle) ResumeAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID string) error {
	s := l.service
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return err
	}
	s.Meter.Observe(ctx, raw)
	if err := s.Client.Resume(ctx, key, sandboxID); err != nil {
		return err
	}
	raw.Status.State = string(StatusResuming)
	s.Meter.Observe(ctx, raw)
	return nil
}

// CancelAgentSessionSandbox terminates the exact session sandbox. OpenSandbox's
// terminate is idempotent, so a retry can safely converge a canceling record.
func (l *AgentSessionLifecycle) CancelAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID string) error {
	s := l.service
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			// Already absent is the desired terminal state. A mismatched/foreign
			// object is also deliberately indistinguishable from absent and is
			// never touched; the authorized durable session may still converge.
			if s.SessionEgress != nil {
				return s.SessionEgress.Delete(ctx, store.SandboxNamespace(workspaceID), sessionID)
			}
			return nil
		}
		return err
	}
	s.Meter.Observe(ctx, raw)
	if err := s.Client.Terminate(ctx, key, sandboxID); err != nil {
		return err
	}
	raw.Status.State = string(StatusTerminated)
	s.Meter.Observe(ctx, raw)
	if s.SessionEgress != nil {
		if err := s.SessionEgress.Delete(ctx, store.SandboxNamespace(workspaceID), sessionID); err != nil {
			return fmt.Errorf("delete terminated session egress policy: %w", err)
		}
	}
	return nil
}

// agentSessionStatusPath is the driver's machine-readable status file inside the
// sandbox (BEX_AGENT_STATUS_FILE default, lego/agent-image/driver/src/config.mjs).
const agentSessionStatusPath = "/var/run/bex-agent/status.json"

// ReadSessionStatus returns the raw driver status-file contents from the exact
// session sandbox through the gateway exec boundary — the trusted completion
// signal the agent-sessions Completer polls (ADR047 D4). It re-validates the
// sandbox binding but intentionally performs no user-facing authorization: its
// only caller is a system loop that already owns the durable session. Empty
// output means the file is not present yet (the headless turn is still running).
func (l *AgentSessionLifecycle) ReadSessionStatus(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error) {
	s := l.service
	if !s.enabled() || !s.execEnabled() {
		return "", core.ErrSandboxesUnavailable
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	sb, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return "", err
	}
	// A sandbox whose pod has exited (a crashed/hung driver, a failed adapter
	// start) can never report a success status, and exec-ing into the dead pod
	// fails forever — which silently stranded the session in `running` (w3/m43
	// live E2E, the nonexistent-adapter crash leg). Surface it as NotFound so the
	// Completer finalizes it as a failed turn instead of retrying every tick.
	switch mapOpenSandboxStatus(sb.Status.State) {
	case StatusTerminated, StatusErrored:
		return "", sandboxNotFound(sandboxID)
	}
	resp, err := s.mintAndDial(ctx, workspaceID, sandboxID, sessionID,
		[]string{"/bin/sh", "-c", "cat " + agentSessionStatusPath + " 2>/dev/null || true"})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := bufferExec(resp)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// agentSessionLogPath is the driver's redacted per-part conversation log inside
// the sandbox (BEX_AGENT_SESSION_LOG default, lego/agent-image/driver/src/config.mjs).
const agentSessionLogPath = "/var/log/bex-agent/session.jsonl"

// maxTranscriptLogBytes bounds the exec payload when harvesting the log (the tail
// is read; the parser drops a partial leading line). A turn's transcript is far
// smaller in practice; this only guards a pathological driver.
const maxTranscriptLogBytes = 16 << 20

// ReadSessionTranscript returns the driver's session log (redacted JSONL, one
// UI-message part per line) from the exact session sandbox through the SAME
// gateway pods/exec boundary the Completer already uses for ReadSessionStatus
// (ADR051). Harvesting the on-disk log rides the proven status-read path instead
// of a separate driver-stream dial. A terminated/errored pod (its log gone with
// it) surfaces as NotFound so the caller treats it as "nothing to capture".
func (l *AgentSessionLifecycle) ReadSessionTranscript(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error) {
	s := l.service
	if !s.enabled() || !s.execEnabled() {
		return "", core.ErrSandboxesUnavailable
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	sb, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return "", err
	}
	switch mapOpenSandboxStatus(sb.Status.State) {
	case StatusTerminated, StatusErrored:
		return "", sandboxNotFound(sandboxID)
	}
	resp, err := s.mintAndDial(ctx, workspaceID, sandboxID, sessionID,
		[]string{"/bin/sh", "-c", fmt.Sprintf("tail -c %d %s 2>/dev/null || true", maxTranscriptLogBytes, agentSessionLogPath)})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	// The public buffered-exec cap is 2 MiB, but the driver's own redacted log is
	// bounded at 16 MiB. Collect that full trusted artifact with framing headroom
	// and fail explicitly if even this bound truncates it.
	result, err := bufferExecWithLimit(resp, maxTranscriptLogBytes+1<<20)
	if err != nil {
		return "", err
	}
	if result.Truncated {
		return "", fmt.Errorf("%w: agent session transcript exceeded harvest limit", core.ErrSandboxesUnavailable)
	}
	return result.Stdout, nil
}

// hibernateScript is the ADR059 D3 snapshot pipeline, run once via the trusted
// gateway exec boundary (stdout-only, signed argv). It scrubs credentials (the
// same bex-pre-snapshot hook Suspend uses), tars the mutable workspace state
// (`/workspace` + `~/.zed_server` when present) preserving ownership, prints the
// digest + byte count on stdout (the only channel back), then streams the
// archive straight to the presigned PUT URL — so **no durable credential ever
// enters the sandbox**, only a single-object time-boxed URL. `set -e` +
// `pipefail`-free curl `-f` make any failure a non-zero exit the caller treats
// as "hibernation failed, fall back to Terminate". The URL is single-quoted; a
// presigned URL is percent-encoded and never contains a single quote.
const hibernateScript = `set -e
%s
SNAP=/tmp/bex-hibernate.tgz
HOME_DIR="${HOME:-/home/bex}"
MEMBERS="/workspace"
[ -d "$HOME_DIR/.zed_server" ] && MEMBERS="$MEMBERS $HOME_DIR/.zed_server"
tar czf "$SNAP" --numeric-owner $MEMBERS 2>/dev/null
sha256sum "$SNAP" | cut -d' ' -f1
wc -c < "$SNAP"
if [ -n "$(git -C /workspace status --porcelain 2>/dev/null)" ]; then echo dirty; else echo clean; fi
curl -sf -X PUT --upload-file "$SNAP" %s
rm -f "$SNAP"`

// HibernateAgentSessionSandbox snapshots the session's mutable state to the
// presigned PUT URL and returns the archive's byte count + sha256 (parsed from
// the exec stdout) for durable metering + integrity (ADR059 D3). It does NOT
// terminate the pod — the Completer records the durable snapshot on the row
// first, then terminates via CancelAgentSessionSandbox, so a failure before the
// row write always leaves a live pod for the fallback Terminate reap and never
// strands a row. On ANY error the pod is untouched. Trusted background call:
// like ReadSessionStatus it re-validates the binding but performs no user
// authorization.
func (l *AgentSessionLifecycle) HibernateAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID, putURL string) (SnapshotResult, error) {
	s := l.service
	if !s.enabled() || !s.execEnabled() {
		return SnapshotResult{}, core.ErrSandboxesUnavailable
	}
	if putURL == "" {
		return SnapshotResult{}, fmt.Errorf("%w: snapshot upload URL is required", core.ErrBadRequest)
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return SnapshotResult{}, err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return SnapshotResult{}, err
	}
	switch mapOpenSandboxStatus(raw.Status.State) {
	case StatusTerminated, StatusErrored:
		return SnapshotResult{}, sandboxNotFound(sandboxID)
	}
	snapshotCommand, err := s.agentSnapshotCommand(sessionID)
	if err != nil {
		return SnapshotResult{}, err
	}
	command := fmt.Sprintf(hibernateScript, snapshotCommand, "'"+putURL+"'")
	resp, err := s.mintAndDial(ctx, workspaceID, sandboxID, sessionID, []string{"/bin/sh", "-c", command})
	if err != nil {
		return SnapshotResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := bufferExec(resp)
	if err != nil {
		return SnapshotResult{}, err
	}
	if result.ExitCode != 0 {
		return SnapshotResult{}, fmt.Errorf("hibernation snapshot failed with exit code %d", result.ExitCode)
	}
	return parseHibernateOutput(result.Stdout)
}

// parseHibernateOutput reads the digest, byte-count, and clean/dirty lines the
// snapshot script prints (in that order) from the exec stdout, tolerating
// trailing blank lines. The dirty flag drives the ADR059 D5 retention extension.
func parseHibernateOutput(stdout string) (SnapshotResult, error) {
	fields := strings.Fields(stdout)
	if len(fields) < 3 {
		return SnapshotResult{}, fmt.Errorf("hibernation snapshot produced incomplete output (stdout=%q)", stdout)
	}
	sha := fields[0]
	bytes, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || bytes <= 0 {
		return SnapshotResult{}, fmt.Errorf("hibernation snapshot reported invalid size %q", fields[1])
	}
	if len(sha) != 64 {
		return SnapshotResult{}, fmt.Errorf("hibernation snapshot reported invalid digest %q", sha)
	}
	return SnapshotResult{Bytes: bytes, SHA256: sha, DirtyGit: fields[2] == "dirty"}, nil
}

func (s *Service) agentSessionKey(ctx context.Context, workspaceID string) (string, error) {
	if !s.enabled() {
		return "", core.ErrSandboxesUnavailable
	}
	if s.Keys == nil {
		return "", nil
	}
	return s.Keys.WorkspaceKey(ctx, workspaceID)
}

func (s *Service) agentSessionSandbox(ctx context.Context, key, workspaceID, sessionID, sandboxID string) (osSandbox, error) {
	raw, err := s.Client.Get(ctx, key, sandboxID)
	if err != nil {
		if errors.Is(err, errOpenSandboxNotFound) {
			return osSandbox{}, sandboxNotFound(sandboxID)
		}
		return osSandbox{}, err
	}
	if !validOwnedSandbox(raw, workspaceID) || raw.Metadata[metadataAgentSession] != sessionID {
		return osSandbox{}, sandboxNotFound(sandboxID)
	}
	return raw, nil
}

// List returns the caller's workspace's sandboxes (tenant-key-scoped).
func (s *Service) List(ctx context.Context) ([]Sandbox, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.enabled() {
		return nil, core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return nil, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.Client.List(ctx, key)
	if err != nil {
		return nil, err
	}
	out := make([]Sandbox, 0, len(raw))
	callerOwner := ownerID(ctx)
	admin, adminChecked := false, false
	for _, r := range raw {
		if !validOwnedSandbox(r, ws) {
			continue
		}
		if r.Metadata[metadataOwner] == callerOwner {
			out = append(out, sandboxFromOpenSandbox(r, ws))
			continue
		}
		// A list can contain many sandboxes owned by other members. Resolve the
		// workspace-admin override once, not once per object/FGA round trip.
		if !adminChecked {
			admin, err = s.isWorkspaceAdmin(ctx, ws)
			if err != nil {
				return nil, err
			}
			adminChecked = true
		}
		if admin {
			out = append(out, sandboxFromOpenSandbox(r, ws))
		}
	}
	return out, nil
}

// Get returns one sandbox's status.
func (s *Service) Get(ctx context.Context, id string) (Sandbox, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Sandbox{}, err
	}
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return Sandbox{}, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.ownedSandbox(ctx, key, ws, id)
	if err != nil {
		return Sandbox{}, err
	}
	return sandboxFromOpenSandbox(raw, ws), nil
}

// Suspend/Resume/Terminate are the lifecycle verbs; suspend/resume need operate,
// terminate needs create (delete) — matching the other resource features.
func (s *Service) Suspend(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanOperate, id, StatusSuspended, s.clientSuspend)
}
func (s *Service) Resume(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanOperate, id, StatusResuming, s.clientResume)
}
func (s *Service) Terminate(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanCreate, id, StatusTerminated, s.clientTerminate)
}

func (s *Service) clientSuspend(ctx context.Context, key string, raw osSandbox) error {
	// Agent sessions are the only sandbox class carrying this binding. Fail the
	// snapshot closed unless the image's hygiene hook runs successfully through
	// the already-authorized gateway exec boundary. Generic EA sandboxes retain
	// byte-identical suspend behavior.
	if raw.Metadata[agentsession.LabelSession] != "" {
		sessionID := raw.Metadata[agentsession.LabelSession]
		command, err := s.agentSnapshotCommand(sessionID)
		if err != nil {
			return fmt.Errorf("pre-snapshot credential scrub: %w", err)
		}
		// systemBufferedExec (not ExecBuffered): the scrub command is
		// platform-fixed, so it deliberately does NOT pass through dialGateway's
		// caller gate — a contributor (can_operate) may suspend a session
		// sandbox, and dialGateway now requires can_view_sensitive for
		// agent-session sandboxes on CALLER-chosen commands (round-13 #1).
		result, err := s.systemBufferedExec(ctx, raw.Metadata[metadataWorkspace], raw.ID, sessionID, command)
		if err != nil {
			return fmt.Errorf("pre-snapshot credential scrub: %w", err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("pre-snapshot credential scrub failed with exit code %d", result.ExitCode)
		}
	}
	return s.Client.Suspend(ctx, key, raw.ID)
}

func (s *Service) agentSnapshotCommand(sessionID string) (string, error) {
	if s.Exec == nil || len(s.Exec.DriverGrantSecret) == 0 || sessionID == "" {
		return "", errors.New("snapshot driver grant unavailable")
	}
	grant, err := drivergrant.MintAction(
		s.Exec.DriverGrantSecret,
		sessionID,
		drivergrant.ActionSnapshot,
		s.Now(),
		60*time.Second,
	)
	if err != nil {
		return "", err
	}
	// Ed25519 grant tokens contain only base64url bytes and a dot, so this
	// environment assignment has no shell metacharacters or quoting ambiguity.
	return "BEX_AGENT_SNAPSHOT_GRANT=" + grant + " /usr/local/bin/bex-pre-snapshot", nil
}
func (s *Service) clientResume(ctx context.Context, key string, raw osSandbox) error {
	return s.Client.Resume(ctx, key, raw.ID)
}
func (s *Service) clientTerminate(ctx context.Context, key string, raw osSandbox) error {
	return s.Client.Terminate(ctx, key, raw.ID)
}

func (s *Service) lifecycle(ctx context.Context, relation, id string, phase Status, op func(context.Context, string, osSandbox) error) error {
	if err := s.Authorize(ctx, relation); err != nil {
		return err
	}
	if !s.enabled() {
		return core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.ownedSandbox(ctx, key, ws, id)
	if err != nil {
		return err
	}
	if err := op(ctx, key, raw); err != nil {
		return err
	}
	raw.Status.State = string(phase)
	s.Meter.Observe(ctx, raw)
	if phase == StatusTerminated && s.SessionEgress != nil {
		if sessionID := raw.Metadata[metadataAgentSession]; sessionID != "" {
			if err := s.SessionEgress.Delete(ctx, store.SandboxNamespace(ws), sessionID); err != nil {
				return fmt.Errorf("delete terminated session egress policy: %w", err)
			}
		}
	}
	return nil
}
