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
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const ticketTTL = 90 * time.Second

type Store interface {
	CreateAgentSession(context.Context, store.AgentSession) (store.AgentSession, error)
	GetAgentSession(context.Context, string) (store.AgentSession, error)
	ListAgentSessions(context.Context, string) ([]store.AgentSession, error)
	ListAgentSessionsByPhases(context.Context, []string) ([]store.AgentSession, error)
	SetAgentSessionLifecycle(context.Context, string, string, string, string, bool) (store.AgentSession, error)
	RecordAgentSessionDispatch(context.Context, string, string, string, string, string) (store.AgentSession, error)
	FinalizeAgentSession(context.Context, string, string, string, string, int, json.RawMessage, string) (store.AgentSession, error)
	DeleteAgentSession(context.Context, string) error
}

// TupleWriter establishes the resource-parent edge in OpenFGA. The production
// checker implements it directly; keeping the seam here makes the domain tests
// prove that persistence alone is never treated as authorization.
type TupleWriter interface {
	GrantAgentSessionWorkspace(context.Context, string, string) error
}

type SandboxLifecycle interface {
	CreateAgentSessionSandbox(ctx context.Context, workspaceID, template, sessionID, repository, branch, modelEndpoint, modelAPIKey string, egressAllowlist []string, driverEnv map[string]string) (sandbox.Sandbox, error)
	EnterAgentSessionPhase(ctx context.Context, workspaceID, sessionID, sandboxID, modelEndpoint string, egressAllowlist []string) error
	ResumeAgentSessionSandbox(context.Context, string, string, string) error
	CancelAgentSessionSandbox(context.Context, string, string, string) error
	ReadSessionStatus(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error)
}

// PullRequestOpener opens (or idempotently reuses) the session's draft PR. The
// production GitHub App client satisfies it; keeping the seam here lets the
// completion tests stub GitHub at the client boundary (ADR047 D4).
type PullRequestOpener interface {
	OpenDraftPullRequest(ctx context.Context, installationID int64, owner, repo, head, base, title, body string) (github.PullRequest, error)
}

// ConnectionStore resolves a workspace's GitHub App installation, so the
// Completer can open a PR under the same account the session token was minted
// for (ADR026/ADR047 D2).
type ConnectionStore interface {
	GetGitConnection(context.Context, string) (store.GitConnection, error)
}

// modelKeySecretPath is the OpenBao KV v2 path a workspace's BYO agent-session
// model provider key is stored at (ADR047 D7). core.SecretKV's concrete store
// only tenant-scopes a path via an unexported context key private to the
// secrets package (internal/secrets's withTenant) — envgroups, a different
// package sharing this same store, works around that by baking its own opaque
// unguessable id (envg-<xid>) directly into the path instead of relying on
// that mechanism, and this follows the identical precedent: workspaceID
// (tea-<xid>) is exactly as opaque and unguessable, so folding it into the
// path achieves the same per-workspace isolation. v1 provisioning is
// operator/manual (`bao kv put`) — no REST/GraphQL/MCP surface exists yet to
// let a tenant self-serve this, matching m37's explicit no-API-surface scope.
// The stored map's one key is sandbox.ModelAPIKeyEnvVar so the fetch needs no
// name translation before landing in the sandbox pod's env.
func modelKeySecretPath(workspaceID string) string {
	return "agent-sessions/" + workspaceID + "/model-key"
}

type Service struct {
	*core.Base
	Store        Store
	Tuples       TupleWriter
	Sandbox      SandboxLifecycle
	TicketSecret []byte
	GatewayURL   string
	// CredentialURL is the in-cluster gateway git-credential broker endpoint the
	// sandbox's helper calls (ADR047 D2). Empty => defaultCredentialURL.
	CredentialURL string
	// ModelKeys, when set (BEX_OPENBAO_URL wired), sources each workspace's BYO
	// agent-session model provider key at session-create time (ADR047 D7). nil
	// => sessions start with no model key (byte-identical to before this field
	// existed) — a missing/never-provisioned key is not an error, since the key
	// is optional per workspace.
	ModelKeys core.SecretKV
}

// modelAPIKey best-effort reads the workspace's BYO model key. A missing path
// returns "" (core.SecretKV.Get's documented not-found behavior), which is the
// common case until a workspace provisions one — never an error. A genuine
// OpenBao failure DOES fail the create: silently starting a keyless session
// when the store is actually reachable-but-erroring would waste a sandbox the
// agent can never authenticate from, with no signal to the caller why.
func (s *Service) modelAPIKey(ctx context.Context, workspaceID string) (string, error) {
	if s.ModelKeys == nil {
		return "", nil
	}
	data, err := s.ModelKeys.Get(ctx, modelKeySecretPath(workspaceID))
	if err != nil {
		return "", fmt.Errorf("%w: read agent session model key: %v", core.ErrSecretsUnavailable, err)
	}
	return data[sandbox.ModelAPIKeyEnvVar], nil
}

func (s *Service) enabled() bool {
	return s.Store != nil && s.Tuples != nil && s.Sandbox != nil
}

func (s *Service) ticketEnabled() bool {
	return len(s.TicketSecret) > 0 && strings.TrimSpace(s.GatewayURL) != ""
}

func sessionObject(id string) string { return "agent_session:" + id }

func validateSessionID(value string) error {
	kind, ok := ids.KindOf(value)
	if !ok || kind != ids.AgentSession {
		return core.NewBadRequestError("AGENT_SESSION_ID_INVALID", "invalid agent session id", map[string]any{"id": value})
	}
	return nil
}

func validateCreate(req *CreateRequest) error {
	if strings.TrimSpace(req.Repo) == "" || strings.TrimSpace(req.AgentConfig.Agent) == "" || strings.TrimSpace(req.AgentConfig.Task) == "" {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "repo, agentConfig.agent, and agentConfig.task are required", nil)
	}
	if strings.TrimSpace(req.Branch) == "" {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "branch is required", map[string]any{"field": "branch"})
	}
	if len(req.Repo) > 2048 || len(req.Branch) > 255 || len(req.AgentConfig.Agent) > 128 || len(req.AgentConfig.Model) > 255 || len(req.AgentConfig.ModelEndpoint) > 2048 || len(req.AgentConfig.Task) > 100_000 || len(req.AgentConfig.Template) > 128 {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "agent session input exceeds its size limit", nil)
	}
	repository, err := agentsession.NormalizeRepository(req.Repo)
	if err != nil {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "repo must identify a GitHub owner/repository", map[string]any{"field": "repo"})
	}
	branch := strings.TrimSpace(req.Branch)
	if err := agentsession.ValidateBranch(branch); err != nil {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "branch must start with bex-agent/", map[string]any{"field": "branch"})
	}
	req.Repo = repository
	req.Branch = branch
	return nil
}

// Create starts a new session or resumes ResumeSessionID. New sessions check
// can_operate on the named workspace before persistence; resume checks the
// first-class agent_session object directly, avoiding a caller-default-workspace
// gate that could disagree with the resource's own parent tuple.
func (s *Service) Create(ctx context.Context, req CreateRequest) (View, error) {
	if req.ResumeSessionID != "" {
		return s.Resume(ctx, req.ResumeSessionID)
	}
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateCreate(&req); err != nil {
		return View{}, err
	}
	modelEndpoint, egressAllowlist, err := createEgress(req.AgentConfig, req.EgressAllowlist)
	if err != nil {
		return View{}, err
	}
	req.AgentConfig.ModelEndpoint = modelEndpoint
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return View{}, core.ErrForbidden
	}
	modelAPIKey, err := s.modelAPIKey(ctx, workspaceID)
	if err != nil {
		return View{}, err
	}
	config, err := json.Marshal(req.AgentConfig)
	if err != nil {
		return View{}, err
	}
	record, err := s.Store.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: workspaceID,
		Repo:        req.Repo,
		Branch:      req.Branch,
		AgentConfig: config,
	})
	if err != nil {
		return View{}, mapStoreError(record.ID, err)
	}
	// The row is not a reachable resource until this edge exists. If OpenFGA is
	// unavailable, compensate the row and refuse before any sandbox is started.
	if err := s.Tuples.GrantAgentSessionWorkspace(ctx, record.ID, workspaceID); err != nil {
		_ = s.Store.DeleteAgentSession(ctx, record.ID)
		return View{}, fmt.Errorf("%w: establish agent session authorization: %v", core.ErrAuthzUnavailable, err)
	}
	env := driverEnv(req.AgentConfig, record.Repo, record.Branch, record.ID, store.SandboxNamespace(record.WorkspaceID), s.credentialURL())
	record, err = s.dispatch(ctx, record, strings.TrimSpace(req.AgentConfig.Template), modelEndpoint, modelAPIKey, egressAllowlist, env, "")
	if err != nil {
		return View{}, err
	}
	return s.withTicket(ctx, record)
}

// dispatch starts (or re-starts) the session's sandbox, transitions it into the
// agent egress phase, and records the new turn. On any failure it marks the
// session failed and returns the error; a store failure after the sandbox is up
// best-effort terminates it (a sandbox without a durable binding is unmanageable).
// Shared by the initial Create and by Steer's re-dispatch so their rollback
// semantics stay in lock-step.
func (s *Service) dispatch(ctx context.Context, record store.AgentSession, template, modelEndpoint, modelAPIKey string, egressAllowlist []string, env map[string]string, deliveryMode string) (store.AgentSession, error) {
	ws := record.WorkspaceID
	sb, err := s.Sandbox.CreateAgentSessionSandbox(ctx, ws, template, record.ID, record.Repo, record.Branch, modelEndpoint, modelAPIKey, egressAllowlist, env)
	if err != nil {
		_, _ = s.Store.SetAgentSessionLifecycle(ctx, record.ID, "", PhaseFailed, "sandbox create failed", false)
		return store.AgentSession{}, err
	}
	if err := s.Sandbox.EnterAgentSessionPhase(ctx, ws, record.ID, sb.ID, modelEndpoint, egressAllowlist); err != nil {
		_ = s.Sandbox.CancelAgentSessionSandbox(ctx, ws, record.ID, sb.ID)
		_, _ = s.Store.SetAgentSessionLifecycle(ctx, record.ID, sb.ID, PhaseFailed, "egress phase transition failed", false)
		return store.AgentSession{}, err
	}
	phase := PhaseCreating
	if sb.Status == sandbox.StatusRunning {
		phase = PhaseRunning
	}
	record, err = s.Store.RecordAgentSessionDispatch(ctx, record.ID, sb.ID, phase, string(sb.Status), deliveryMode)
	if err != nil {
		_ = s.Sandbox.CancelAgentSessionSandbox(ctx, ws, record.ID, sb.ID)
		return store.AgentSession{}, mapStoreError(record.ID, err)
	}
	return record, nil
}

func (s *Service) List(ctx context.Context, ownerID string) ([]View, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return nil, err
	}
	if !s.enabled() {
		return nil, core.ErrAgentSessionsUnavailable
	}
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return nil, core.ErrForbidden
	}
	rows, err := s.Store.ListAgentSessions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, row := range rows {
		view, err := viewOf(row)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, sessionID string) (View, error) {
	// Authorize the named object before validating/fetching it: unauthorized and
	// cross-workspace probes are a tuple denial, never a DB existence oracle.
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return viewOf(record)
}

func (s *Service) Resume(ctx context.Context, sessionID string) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.Phase == PhaseCanceled || record.Phase == PhaseCanceling || record.SandboxID == "" {
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_RESUMABLE", "agent session cannot be resumed from its current phase", map[string]any{"phase": record.Phase})
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseResuming, "resuming", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if err := s.Sandbox.ResumeAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
		_, _ = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseFailed, "sandbox resume failed", false)
		return View{}, err
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseRunning, "running", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return s.withTicket(ctx, record)
}

func (s *Service) Cancel(ctx context.Context, sessionID string) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.Phase == PhaseCanceled {
		return viewOf(record) // idempotent public cancel
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseCanceling, "canceling", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.SandboxID != "" {
		if err := s.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
			return View{}, err // leave canceling: a retry converges it
		}
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseCanceled, "canceled", true)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return viewOf(record)
}

// agentCommand maps a public agent selector to its installed ACP adapter
// command inside the image. A short name maps to its adapter binary; anything
// else passes through so an operator can register a new adapter without a code
// change (the image, not this map, is the security boundary).
func agentCommand(agent string) string {
	switch strings.ToLower(strings.TrimSpace(agent)) {
	case "claude", "claude-code", "claude-code-acp":
		return "claude-code-acp"
	case "gemini", "gemini-cli":
		return "gemini"
	case "codex", "codex-acp":
		return "codex-acp"
	default:
		return agent
	}
}

// defaultCredentialURL is the in-cluster gateway git-credential broker endpoint
// (ADR047 D2). The sandbox's git-credential-bex helper POSTs here; the gateway
// authenticates the source Pod and HMAC-proxies a scoped token mint to bex-api.
const defaultCredentialURL = "http://bex-ssh-gateway.bex-system.svc:8082" + agentsession.GatewayPath

func (s *Service) credentialURL() string {
	if strings.TrimSpace(s.CredentialURL) != "" {
		return s.CredentialURL
	}
	return defaultCredentialURL
}

// driverEnv renders the non-secret environment the sandbox driver reads to run
// one headless delivery turn (lego/agent-image/driver). The BYO model key is
// NOT here: it is sourced from OpenBao and injected as pod-spec env by the
// sandbox lifecycle (ADR047 D7), so no secret flows through this map. The
// git-credential-bex vars (ADR047 D2) ARE here: without them the setup-phase
// clone cannot authenticate and every session dies with "missing its session
// broker configuration" (caught live on prod, w3/m43). They carry no secret —
// only the session's non-secret identity + the internal broker URL; the token
// is minted on demand through the gateway and never lands on disk.
func driverEnv(config AgentConfig, repo, branch, sessionID, namespace, credentialURL string) map[string]string {
	return map[string]string{
		"BEX_AGENT_COMMAND":         agentCommand(config.Agent),
		"BEX_AGENT_PROMPT":          config.Task,
		"BEX_AGENT_BRANCH":          branch,
		"BEX_AGENT_REPO_URL":        "https://github.com/" + repo + ".git",
		"BEX_AGENT_DELIVER":         "1",
		"BEX_AGENT_EXIT_AFTER_TURN": "0",
		"BEX_AGENT_CREDENTIAL_URL":  credentialURL,
		"BEX_SANDBOX_NAMESPACE":     namespace,
		"BEX_AGENT_SESSION_ID":      sessionID,
		"BEX_AGENT_REPOSITORY":      repo,
	}
}

// Steer runs a follow-up prompt turn on an existing session (ADR047 D8 phase 1,
// ADR047 D8 phase 1). In phase 1 a new prompt cannot ride the original sandbox — its
// prompt env is fixed at creation and there is no live attach yet — so steering
// re-dispatches a fresh sandbox that re-clones the same bex-agent/* branch and
// runs the new prompt; the Completer then updates the same draft PR. The
// delivery mode is recorded on the row.
func (s *Service) Steer(ctx context.Context, req SteerRequest) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(req.SessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() || !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(req.SessionID); err != nil {
		return View{}, err
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" || len(prompt) > 100_000 {
		return View{}, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "prompt is required", map[string]any{"field": "prompt"})
	}
	record, err := s.Store.GetAgentSession(ctx, req.SessionID)
	if err != nil {
		return View{}, mapStoreError(req.SessionID, err)
	}
	switch record.Phase {
	case PhaseCanceled, PhaseCanceling:
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_STEERABLE", "a canceled agent session cannot be steered", map[string]any{"phase": record.Phase})
	case PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching:
		return View{}, core.NewConflictError("AGENT_SESSION_TURN_IN_FLIGHT", "a turn is already running; wait for it to finish before steering", map[string]any{"phase": record.Phase})
	}
	config, err := decodeAgentConfig(record)
	if err != nil {
		return View{}, err
	}
	modelEndpoint, egressAllowlist, err := createEgress(config, req.EgressAllowlist)
	if err != nil {
		return View{}, err
	}
	config.ModelEndpoint = modelEndpoint
	if _, err := s.Store.SetAgentSessionLifecycle(ctx, record.ID, "", PhaseRedispatching, "redispatching", false); err != nil {
		return View{}, mapStoreError(record.ID, err)
	}
	// Tear the previous turn's sandbox down (idempotent) before re-dispatching.
	if record.SandboxID != "" {
		if err := s.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
			_, _ = s.Store.SetAgentSessionLifecycle(ctx, record.ID, "", PhaseFailed, "steer teardown failed", false)
			return View{}, err
		}
	}
	modelAPIKey, err := s.modelAPIKey(ctx, record.WorkspaceID)
	if err != nil {
		return View{}, err
	}
	env := driverEnv(config, record.Repo, record.Branch, record.ID, store.SandboxNamespace(record.WorkspaceID), s.credentialURL())
	env["BEX_AGENT_PROMPT"] = prompt // the steering prompt overrides the original task
	record, err = s.dispatch(ctx, record, strings.TrimSpace(config.Template), modelEndpoint, modelAPIKey, egressAllowlist, env, DeliveryRedispatch)
	if err != nil {
		return View{}, err
	}
	return s.withTicket(ctx, record)
}

// AttachTicket mints a fresh attach ticket for an already-started session
// without changing its lifecycle (ADR047 D9 target API shape). It closes the
// reconnect gap: create/resume/steer mint at dispatch time, but a page reload
// or a stream drop on a running session — or opening a terminal session's
// transcript replay — needs a ticket too, and those verbs don't fire again. The
// ticket carries the same 90s TTL + single-use nonce + subject/session/pod/
// namespace claims, so the gateway authorizes reattach and terminal replay
// identically to a first connect. A session that never started a sandbox has
// nothing to attach to.
func (s *Service) AttachTicket(ctx context.Context, sessionID string) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() || !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.SandboxID == "" {
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_ATTACHABLE",
			"agent session has not started a sandbox yet", map[string]any{"phase": record.Phase})
	}
	return s.withTicket(ctx, record)
}

func (s *Service) withTicket(ctx context.Context, record store.AgentSession) (View, error) {
	identity, ok := core.IdentityFrom(ctx)
	if !ok || identity.Subject == "" {
		return View{}, core.ErrForbidden
	}
	now := s.Now()
	expires := now.Add(ticketTTL)
	ticket, err := agentsessionticket.Mint(s.TicketSecret, agentsessionticket.Claims{
		Subject: identity.Subject, SessionID: record.ID, SandboxID: record.SandboxID,
		Pod: record.SandboxID + "-0", Workspace: record.WorkspaceID,
		Namespace: record.WorkspaceID + "-sandbox", IssuedAt: now.Unix(), ExpiresAt: expires.Unix(),
	})
	if err != nil {
		return View{}, err
	}
	view, err := viewOf(record)
	if err != nil {
		return View{}, err
	}
	view.Ticket, view.URL, view.ExpiresAt = ticket, s.GatewayURL, &expires
	return view, nil
}

// decodeAgentConfig is the one place the persisted config JSON is parsed.
func decodeAgentConfig(record store.AgentSession) (AgentConfig, error) {
	var config AgentConfig
	if err := json.Unmarshal(record.AgentConfig, &config); err != nil {
		return AgentConfig{}, fmt.Errorf("decode persisted agent config: %w", err)
	}
	return config, nil
}

func viewOf(record store.AgentSession) (View, error) {
	config, err := decodeAgentConfig(record)
	if err != nil {
		return View{}, err
	}
	var evidence *Evidence
	if len(record.Evidence) > 0 && string(record.Evidence) != "{}" {
		var e Evidence
		if err := json.Unmarshal(record.Evidence, &e); err != nil {
			return View{}, fmt.Errorf("decode persisted evidence: %w", err)
		}
		evidence = &e
	}
	return View{ID: record.ID, OwnerID: record.WorkspaceID, Repo: record.Repo, Branch: record.Branch,
		AgentConfig: config, SandboxID: record.SandboxID, Phase: record.Phase, Status: record.Status,
		HeadSHA: record.HeadSHA, PRURL: record.PRURL, PRNumber: record.PRNumber, Evidence: evidence,
		Turns: record.Turns, DeliveryMode: record.DeliveryMode, FailureReason: record.FailureReason,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, CanceledAt: record.CanceledAt}, nil
}

func mapStoreError(id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return core.NewNotFoundError("AGENT_SESSION_NOT_FOUND", "agent session not found", map[string]any{"id": id})
	}
	if errors.Is(err, store.ErrConflict) {
		return core.NewConflictError("AGENT_SESSION_CONFLICT", "agent session conflicts with existing state", map[string]any{"id": id})
	}
	return err
}
