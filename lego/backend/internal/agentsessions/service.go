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
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/drivergrant"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const ticketTTL = 90 * time.Second

type Store interface {
	CreateAgentSession(context.Context, store.AgentSession) (store.AgentSession, error)
	ListAgentDispatchesDue(context.Context, time.Time) ([]store.AgentDispatch, error)
	AbandonAgentDispatch(context.Context, store.AgentDispatch, time.Time, string) error
	DeferAgentDispatchCleanup(context.Context, store.AgentDispatch, time.Time) error
	GetAgentSession(context.Context, string) (store.AgentSession, error)
	ListAgentSessions(context.Context, string, store.AgentSessionListQuery) ([]store.AgentSession, error)
	ListAgentSessionsByPhases(context.Context, []string) ([]store.AgentSession, error)
	SetAgentSessionLifecycle(context.Context, string, string, string, string, bool) (store.AgentSession, error)
	// SetAgentSessionFailure terminalizes an active session with a reason recorded
	// in failure_reason (w5/m80 t005): the background failure paths use it so a
	// provisioning failure carries the same surfaced reason a driver failure does.
	SetAgentSessionFailure(context.Context, string, string, string) (store.AgentSession, error)
	// Archive (ADR065 D1): the orthogonal list-state flag; never touches phase.
	SetAgentSessionArchived(context.Context, string, bool) (store.AgentSession, error)
	BeginAgentSessionTurn(context.Context, string, string, string, string, string) (store.AgentSession, error)
	RecordAgentSessionDispatch(context.Context, string, string, string, string, string, int) (store.AgentSession, error)
	FinalizeAgentSession(context.Context, string, string, string, string, int, json.RawMessage, string) (store.AgentSession, error)
	DeleteAgentSession(context.Context, string) error
	// Deferred-teardown reaping (ADR054 D6, generalized by ADR059 D2 / w2/m67):
	// the Completer defers a finished session's sandbox teardown while an editor
	// SSH session is open AND for the Active-tier idle grace after the last
	// interaction. AgentSessionSSHActivity returns both signals: whether a fresh
	// still-open editor session pins the sandbox, and the last SSH disconnect time
	// (feeding the idle clock alongside the session's turn-end).
	AgentSessionSSHActivity(ctx context.Context, resourceID string, freshSince time.Time) (hasFreshOpen bool, lastEnded *time.Time, err error)
	ListTerminalAgentSessionsWithSandbox(ctx context.Context, since time.Time) ([]store.AgentSession, error)
	ClearAgentSessionSandbox(ctx context.Context, id string) error
	// CountLiveAgentSessionSandboxes bounds the ADR059 D6 per-workspace live-
	// sandbox cap: sessions in a live phase holding a sandbox id for the workspace.
	CountLiveAgentSessionSandboxes(ctx context.Context, workspaceID string, phases []string) (int, error)
	// Hibernation (ADR059 D3/D5, w2/m68): claim→snapshot→hibernate, rehydrate on
	// resume, pin/unpin, and the retention sweep.
	ClaimAgentSessionForHibernation(ctx context.Context, id string) (store.AgentSession, error)
	HibernateAgentSession(ctx context.Context, id, snapshotRef string, snapshotBytes int64, snapshotSHA string, retainUntil time.Time) (store.AgentSession, error)
	BeginRehydrate(ctx context.Context, id, prompt, deliveryMode string) (store.AgentSession, error)
	AbortRehydrate(ctx context.Context, id string) (store.AgentSession, error)
	RehydrateAgentSession(ctx context.Context, id, sandboxID, phase, status, deliveryMode string) (store.AgentSession, error)
	SetAgentSessionPinned(ctx context.Context, id string, pinned bool, retainUntil *time.Time) (store.AgentSession, error)
	ListHibernatedForRetention(ctx context.Context, now time.Time, limit int) ([]store.AgentSession, error)
	ExpireHibernatedAgentSession(ctx context.Context, id, snapshotRef string) (store.AgentSession, error)
	CountPinnedAgentSessions(ctx context.Context, workspaceID string) (int, error)
	// Transcript persistence (ADR051): the Completer harvests the driver's log
	// at completion and appends it here (idempotent by session+turn+part index).
	AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []store.AgentSessionTranscriptPart) error
	AgentSessionTurns(ctx context.Context, sessionID string) ([]store.AgentSessionTurn, error)
	CompleteAgentSessionTurn(ctx context.Context, sessionID string, turn int, complete, truncated bool, reason string) error
	AgentSessionTranscriptBytes(ctx context.Context, sessionID string) (int64, error)
	AgentSessionTranscriptProgress(ctx context.Context, sessionID string, turn int, indexes []int64) (int64, []int64, error)
	// Transcript read (ADR065 D2): the poll-shaped seq-keyset page backing the
	// REST/GraphQL/MCP transcript verb (byte budget + row limit bound one page).
	AgentSessionTranscript(ctx context.Context, sessionID string, afterSeq int64, maxBytes int64, limit int) ([]store.AgentSessionTranscriptPart, error)
}

// TupleWriter establishes the resource-parent edge in OpenFGA. The production
// checker implements it directly; keeping the seam here makes the domain tests
// prove that persistence alone is never treated as authorization.
type TupleWriter interface {
	GrantAgentSessionWorkspace(context.Context, string, string) error
}

type SandboxLifecycle interface {
	CleanupAgentDispatches(context.Context, []store.AgentDispatch) error
	CreateAgentSessionSandbox(ctx context.Context, workspaceID, template, sessionID, repository, branch, modelEndpoint, modelAPIKey string, egressAllowlist []string, driverEnv map[string]string) (sandbox.Sandbox, error)
	EnterAgentSessionPhase(ctx context.Context, workspaceID, sessionID, sandboxID, modelEndpoint string, egressAllowlist []string) error
	ResumeAgentSessionSandbox(context.Context, string, string, string) error
	CancelAgentSessionSandbox(context.Context, string, string, string) error
	// HibernateAgentSessionSandbox snapshots the mutable workspace state to the
	// presigned PUT URL (ADR059 D3), returning the snapshot's size, sha256, and
	// git-dirty flag. It does NOT terminate the pod; an error leaves the pod
	// running so the caller can fall back to Terminate.
	HibernateAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID, putURL string) (sandbox.SnapshotResult, error)
	ReadSessionStatus(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error)
	// ReadSessionTranscript returns the driver's redacted per-part session log
	// (JSONL) over the same pods/exec boundary as ReadSessionStatus, so the
	// Completer can persist the conversation before teardown (ADR051).
	ReadSessionTranscript(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error)
}

// PullRequestOpener opens (or idempotently reuses) the session's draft PR. The
// production GitHub App client satisfies it; keeping the seam here lets the
// completion tests stub GitHub at the client boundary (ADR047 D4).
type PullRequestOpener interface {
	OpenDraftPullRequest(ctx context.Context, installationID int64, owner, repo, head, base, title, body string) (github.PullRequest, error)
}

// ConnectionStore resolves a workspace's GitHub App installation for a repo's
// account, so the Completer can open a PR under the same installation the session
// token was minted for (ADR026/ADR047 D2). Since ADR075 a workspace may hold
// several connections, so this resolves by the repo's owner, not the workspace
// alone.
type ConnectionStore interface {
	GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (store.GitConnection, error)
}

// modelKeySecretPath delegates to the one shared definition in the agentsession
// contract (ADR047 D7), shared by readiness and the ADR062 model-credential mint
// so they cannot drift on where the BYO key lives. workspaceID (tea-<xid>) is
// opaque and unguessable, so
// folding it into the OpenBao path achieves per-workspace isolation without the
// store's tenant-context mechanism. v1 provisioning stays operator/manual
// (`bao kv put`); there is no self-serve REST/GraphQL/MCP surface yet.
func modelKeySecretPath(workspaceID string) string {
	return agentsession.ModelKeySecretPath(workspaceID)
}

type Service struct {
	*core.Base
	Store        Store
	Tuples       TupleWriter
	Sandbox      SandboxLifecycle
	TicketSecret []byte
	GatewayURL   string
	// APIPublicURL (BEX_API_PUBLIC_URL) is the externally reachable API origin
	// (e.g. https://api.bex.co) this Service builds View.StreamURL from — the
	// same coordinate Completer.APIPublicURL uses for copy-ready PR body links.
	// Empty => View.StreamURL is omitted (w3/013).
	APIPublicURL string
	// SSHHost, when set (BEX_SSH_HOST, same activation gates as ADR035), is the
	// public SSH gateway hostname projected onto View.SSHAddress as
	// `ags-<xid>@<host>` for the "Open in Zed" affordance (ADR054 D5). Empty =>
	// no address is surfaced and the feature is invisible (byte-identical default).
	SSHHost string
	// GitProxyURL is the trusted in-cluster smart-HTTP origin. Empty uses the
	// gateway default; the derived repository URL carries no credential.
	GitProxyURL string
	// ModelProxyURL (BEX_AGENT_MODEL_PROXY_URL) routes agent model
	// traffic through the gateway's credential-injecting proxy (ADR062): the
	// sandbox receives only a placeholder + this per-session base URL, and the real
	// BYO key is injected on the gateway→vendor hop. It is mandatory: an empty
	// value disables agent-session create/steer/rehydrate rather than placing the
	// reusable key in an untrusted sandbox environment.
	ModelProxyURL string
	// ModelKeys supplies key-readiness for Capabilities. The gateway-only
	// ModelMinter reads the same store lazily for each provider exchange; session
	// creation never reads or receives the reusable credential.
	ModelKeys core.SecretKV
	// GitHub, when set, supplies the workspace's GitHub App connection readiness
	// for the mobile Capabilities projection (w11/m6 t001). nil => GitHub is
	// reported not-connected with no install URL (the projection never fabricates
	// readiness). *github.Service satisfies it; only the already-safe Connection
	// projection is consumed, never installation tokens/ids.
	GitHub GitHubReadiness
	// MaxLiveSandboxes caps the concurrent live agent-session sandboxes one
	// workspace may hold (ADR059 D6 / w2/m67, BEX_AGENT_MAX_LIVE_SANDBOXES_PER_
	// WORKSPACE). A create/steer/resume that would exceed it is refused with
	// AGENT_SESSION_LIVE_LIMIT rather than silently queued. 0 => uncapped
	// (byte-identical to before this field existed).
	MaxLiveSandboxes int
	// Snapshots, when set (BEX_AGENT_SNAPSHOT_S3_* wired), is the ADR059 D3/D4
	// hibernation object store: Resume/Steer of a hibernated session mints a
	// presigned restore URL from it. nil ⇒ a hibernated session cannot be
	// rehydrated (the Completer never hibernates either), so the whole tier is off.
	Snapshots SnapshotStore
	// MaxPinnedSandboxes caps a workspace's pinned (never-expire) sessions
	// (ADR059 D5 pin quota, BEX_AGENT_MAX_PINNED_SANDBOXES_PER_WORKSPACE). A pin
	// beyond it is refused with AGENT_SESSION_PIN_LIMIT. 0 ⇒ uncapped.
	MaxPinnedSandboxes int
	// RetentionTTL mirrors the Completer's window so an unpin puts a hibernated
	// row back on the clock with a consistent deadline. 0 ⇒ the 7d default.
	RetentionTTL time.Duration
	// TurnTimeout bounds one agent turn inside the sandbox (w5/m80 t002,
	// BEX_AGENT_TURN_TIMEOUT). It is injected as BEX_AGENT_TURN_TIMEOUT_MS so a
	// turn that never converges — a hung model call or an infinite tool loop —
	// is bounded here instead of the driver's 4h fallback. <= 0 ⇒ the 30m
	// defaultTurnTimeout; there is deliberately no "disable" (removing the bound
	// is the failure mode this fixes).
	TurnTimeout time.Duration
	// Metrics records sandbox provisioning latency (w5/m81 t002). It is the same
	// *CompletionMetrics the Completer holds, so lifecycle timing shares one
	// registration; nil ⇒ no observation (byte-identical to before).
	Metrics *CompletionMetrics
	// dispatchRunner runs the slow background provisioning half of create/steer/
	// resume (w2/m64). Left nil in production => a detached goroutine, so the
	// mutation returns before the sandbox exists; tests inject a synchronous (or
	// gated) runner so the async lifecycle is deterministic without sleeps.
	dispatchRunner func(func())
}

// liveSandboxPhases are the phases that hold a provisioned (or provisioning)
// sandbox and thus count against the per-workspace live cap — the same set the
// Completer treats as active. A terminal session in its idle grace is excluded.
var liveSandboxPhases = activePhases

// admitDispatch is the ONE admission gate every verb that provisions fresh
// sandbox compute (Create, Steer, rehydrate) runs, in a fixed order: the
// ADR075 D7 (w6/m42) bound-payment-method marker — checked synchronously at
// admission because the dispatch-time check inside the sandbox lifecycle fires
// in a background goroutine where no 402 can reach the caller — then the
// codex #6 billing lifecycle gate (a delinquent/enforced workspace must not
// provision new compute), then the ADR059 D6 live-sandbox cap so a longer
// idle grace can't let one workspace exhaust the pool. One helper so a new
// dispatch path cannot wire only part of the trio — the same drift class
// core.Base.RequirePlanBilling exists to close for the resource creates.
func (s *Service) admitDispatch(ctx context.Context, workspaceID string) error {
	if err := s.RequirePaymentMethod(ctx, workspaceID); err != nil {
		return err
	}
	if err := s.RequireBillingMutation(ctx, workspaceID); err != nil {
		return err
	}
	return s.enforceLiveSandboxCap(ctx, workspaceID)
}

// enforceLiveSandboxCap refuses a create/steer/resume that would push the
// workspace past MaxLiveSandboxes concurrent live sandboxes (ADR059 D6). It is a
// best-effort pre-check (no lock): the dispatch CAS and the reconcile loop bound
// any small overshoot from a race. 0 => uncapped. A store error is surfaced (the
// caller fails closed) rather than silently allowing an unbounded fan-out.
func (s *Service) enforceLiveSandboxCap(ctx context.Context, workspaceID string) error {
	if s.MaxLiveSandboxes <= 0 {
		return nil
	}
	n, err := s.Store.CountLiveAgentSessionSandboxes(ctx, workspaceID, liveSandboxPhases)
	if err != nil {
		return mapStoreError("", err)
	}
	if n >= s.MaxLiveSandboxes {
		return core.NewConflictError("AGENT_SESSION_LIVE_LIMIT",
			fmt.Sprintf("this workspace already has %d live agent-session sandboxes (limit %d); stop or cancel an idle session before starting another", n, s.MaxLiveSandboxes),
			map[string]any{"live": n, "limit": s.MaxLiveSandboxes})
	}
	return nil
}

// dispatchTimeout bounds one background provisioning attempt (sandbox create +
// egress phase, or a resume) so a hung mechanism call can never leak its
// goroutine forever. Generous: a cold image pull + pod schedule takes minutes.
const dispatchTimeout = 15 * time.Minute

// runBackground executes the slow provisioning half detached from the request.
// Production spawns a goroutine (the mutation has already returned); tests inject
// dispatchRunner to run it synchronously (or gate it) for deterministic asserts.
func (s *Service) runBackground(fn func()) {
	if s.dispatchRunner != nil {
		s.dispatchRunner(fn)
		return
	}
	go fn()
}

// detach runs fn on a context that outlives the request — values preserved so
// audit/tracing survive, cancellation dropped so writing the HTTP response does
// not abort provisioning — under a hard provisioning ceiling.
func (s *Service) detach(ctx context.Context, fn func(context.Context)) {
	bg := context.WithoutCancel(ctx)
	s.runBackground(func() {
		ctx, cancel := context.WithTimeout(bg, dispatchTimeout)
		defer cancel()
		fn(ctx)
	})
}

// phaseSettledOrCanceling is true for the phases a background provisioning step
// must not overwrite: a terminal state (completed/failed/canceled) or canceling
// (converging to canceled). Guarding on it stops a slow provision from
// resurrecting a session a concurrent Cancel already took, or flipping a
// user-canceled session to failed.
func phaseSettledOrCanceling(phase string) bool {
	switch phase {
	case PhaseCompleted, PhaseFailed, PhaseCanceled, PhaseCanceling:
		return true
	}
	return false
}

// setLifecycleIfActive advances a session's phase from the background dispatch
// unless a concurrent cancel/complete already took it settled/canceling. The
// read-then-write leaves a small window; the resource-safety-critical
// resurrection path is closed airtight at the DB by RecordAgentSessionDispatch's
// CAS, and the Completer's status watch is the backstop that reconciles any
// residue (a phase left running for a torn-down sandbox reads NotFound and fails).
// The final argument is a status word for a non-terminal transition (e.g.
// "running") and a human failure REASON for PhaseFailed. A failure records it in
// failure_reason (the field the dashboard's failed-session callout reads) with a
// terminal 'failed' status, matching the Completer's driver-failure path so a
// provisioning failure never shows a blank card (w5/m80 t005, closes w5/048).
func (s *Service) setLifecycleIfActive(ctx context.Context, id, sandboxID, phase, statusOrReason string) {
	if cur, err := s.Store.GetAgentSession(ctx, id); err == nil && phaseSettledOrCanceling(cur.Phase) {
		return
	}
	if phase == PhaseFailed {
		_, _ = s.Store.SetAgentSessionFailure(ctx, id, sandboxID, statusOrReason)
		return
	}
	_, _ = s.Store.SetAgentSessionLifecycle(ctx, id, sandboxID, phase, statusOrReason, false)
}

// GitHubReadiness yields a workspace's GitHub App connection as the neutral,
// already-secret-free github.Connection (connected flag + account login +
// install URL). It is the readiness source for the mobile Capabilities verb.
type GitHubReadiness interface {
	GetConnection(ctx context.Context, ownerID string) (github.Connection, error)
}

// modelAPIKey reads only the workspace readiness signal for Capabilities. A
// missing path returns "" (core.SecretKV.Get's documented not-found behavior),
// while a genuine OpenBao failure fails the readiness call instead of falsely
// reporting that configuration is merely absent. The value never enters create
// or a sandbox Pod; the gateway's minter reads it only during a provider exchange.
func (s *Service) modelAPIKey(ctx context.Context, workspaceID string) (string, error) {
	if s.ModelKeys == nil {
		return "", nil
	}
	data, err := s.ModelKeys.Get(ctx, modelKeySecretPath(workspaceID))
	if err != nil {
		log.Printf("agent-session capabilities: read model-key readiness failed (workspace=%s): %v", workspaceID, err)
		return "", core.NewAgentSessionDependencyUnavailableError()
	}
	return data[sandbox.ModelAPIKeyEnvVar], nil
}

func (s *Service) enabled() bool {
	return s.Store != nil && s.Tuples != nil && s.Sandbox != nil
}

func (s *Service) modelProxyEnabled() bool { return s.modelProxyBaseURL() != "" }

func (s *Service) ticketEnabled() bool {
	return len(s.TicketSecret) > 0 && strings.TrimSpace(s.GatewayURL) != ""
}

func sessionObject(id string) string { return agentsession.SessionObject(id) }

func validateSessionID(value string) error {
	kind, ok := ids.KindOf(value)
	if !ok || kind != ids.AgentSession {
		return core.NewBadRequestError("AGENT_SESSION_ID_INVALID", "invalid agent session id", map[string]any{"id": value})
	}
	return nil
}

func validateCreate(req *CreateRequest) error {
	if strings.TrimSpace(req.AgentConfig.Agent) == "" || strings.TrimSpace(req.AgentConfig.Task) == "" {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "agentConfig.agent and agentConfig.task are required", nil)
	}
	agent := strings.ToLower(strings.TrimSpace(req.AgentConfig.Agent))
	if _, ok := agentsession.LookupAgentProfile(agent); !ok {
		return core.NewBadRequestError("AGENT_SESSION_AGENT_INVALID", "agentConfig.agent must name a supported agent profile", map[string]any{"field": "agentConfig.agent"})
	}
	req.AgentConfig.Agent = agent
	if len(req.Repo) > 2048 || len(req.Branch) > 255 || len(req.AgentConfig.Agent) > 128 || len(req.AgentConfig.Model) > 255 || len(req.AgentConfig.ModelEndpoint) > 2048 || len(req.AgentConfig.Task) > 100_000 || len(req.AgentConfig.Template) > 128 {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "agent session input exceeds its size limit", nil)
	}
	// Repo-less (chat-only) mode: with no repository the session just runs the
	// agent against the prompt in an empty sandbox — no clone, no branch, and no
	// PR delivery (driverEnv sets BEX_AGENT_DELIVER=0, and a turn that pushes
	// nothing completes as an honest no-op). A branch is meaningless here.
	if strings.TrimSpace(req.Repo) == "" {
		req.Repo = ""
		req.Branch = ""
		return nil
	}
	if strings.TrimSpace(req.Branch) == "" {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "branch is required", map[string]any{"field": "branch"})
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
//
// Accept fast, provision async (w2/m64): everything cheap and fail-fast —
// authorization, input validation, egress derivation, the durable row, and the
// OpenFGA parent tuple — runs synchronously, then the
// caller returns immediately in the creating phase. The slow sandbox provisioning
// (pod schedule + image pull, tens of seconds) runs in the background so the
// dashboard can navigate to the session and render progress instead of blocking
// the submit. No attach ticket is minted here — there is no sandbox to bind one
// to yet; the client mints lazily via AttachTicket once a sandbox exists.
func (s *Service) Create(ctx context.Context, req CreateRequest) (View, error) {
	if req.ResumeSessionID != "" {
		return s.Resume(ctx, req.ResumeSessionID)
	}
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	// SECURITY: a new session provisions a sandbox that runs attacker-chosen tasks
	// against repo content and may use the workspace's provider credential through
	// the operation-scoped proxy, so creation is can_create (developer and up), not
	// the lifecycle can_operate relation.
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return View{}, err
	}
	if !s.enabled() || !s.modelProxyEnabled() || !s.ticketEnabled() {
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
	if err := s.admitDispatch(ctx, workspaceID); err != nil {
		return View{}, err
	}
	config, err := json.Marshal(req.AgentConfig)
	if err != nil {
		return View{}, err
	}
	record, err := s.Store.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID:   workspaceID,
		Repo:          req.Repo,
		Branch:        req.Branch,
		AgentConfig:   config,
		InitialPrompt: strings.TrimSpace(req.AgentConfig.Task),
	})
	if err != nil {
		return View{}, mapStoreError(record.ID, err)
	}
	// The row is not a reachable resource until this edge exists. If OpenFGA is
	// unavailable, compensate the row and refuse before any sandbox is started.
	if err := s.Tuples.GrantAgentSessionWorkspace(ctx, record.ID, workspaceID); err != nil {
		_ = s.Store.DeleteAgentSession(ctx, record.ID)
		log.Printf("agent-session create: establish authorization failed (session=%s workspace=%s): %v", record.ID, workspaceID, err)
		return View{}, core.NewAgentSessionDependencyUnavailableError()
	}
	modelAPIKey := agentsession.ModelKeyPlaceholder(record.ID)
	spec := dispatchSpec{
		template:      strings.TrimSpace(req.AgentConfig.Template),
		modelEndpoint: modelEndpoint,
		modelAPIKey:   modelAPIKey,
		egress:        egressAllowlist,
		env:           s.driverEnv(req.AgentConfig, record),
		turn:          record.Turns,
	}
	s.detach(ctx, func(ctx context.Context) { s.runDispatch(ctx, record, spec) })
	return s.toView(record)
}

// dispatchSpec is the immutable provisioning input a background turn carries from
// its accepting verb (Create/Steer) through the run*/dispatch chain — the model
// endpoint/placeholder, egress allowlist, driver env, sandbox template, and delivery mode.
type dispatchSpec struct {
	template          string
	modelEndpoint     string
	modelAPIKey       string
	egress            []string
	env               map[string]string
	delivery          string
	turn              int
	previousSandboxID string
	// task carries agentConfig.Task into the background dispatch so
	// continuityEnv (its transcript read stays OFF the accept path) can
	// re-deliver it on the rung-3 shape (w5/m84).
	task string
}

// runDispatch executes the slow sandbox-provisioning half of a create/steer turn
// in the background and logs its outcome — it never returns to a caller. dispatch
// owns the session's failure bookkeeping and the cancel-race convergence.
func (s *Service) runDispatch(ctx context.Context, record store.AgentSession, spec dispatchSpec) {
	if _, err := s.dispatch(ctx, record, spec); err != nil {
		log.Printf("agent-session dispatch: background provisioning ended (session=%s delivery=%q): %v", record.ID, spec.delivery, err)
	}
}

// dispatch starts (or re-starts) the session's sandbox, transitions it into the
// agent egress phase, and records the new turn. On any failure it marks the
// session failed (without clobbering a concurrent cancel) and returns the error;
// a store failure after the sandbox is up best-effort terminates it (a sandbox
// without a durable binding is unmanageable). Shared by the initial Create and by
// Steer's re-dispatch so their rollback semantics stay in lock-step.
func (s *Service) dispatch(ctx context.Context, record store.AgentSession, spec dispatchSpec) (store.AgentSession, error) {
	ws := record.WorkspaceID
	// CreateAgentSessionSandbox blocks until the pod is Running with an IP (or
	// fails/times out), so its wall-clock is the provisioning latency the user
	// waits through after the fast create-accept (w5/m81 t002).
	provisionStart := time.Now()
	sb, err := s.Sandbox.CreateAgentSessionSandbox(ctx, ws, spec.template, record.ID, record.Repo, record.Branch, spec.modelEndpoint, spec.modelAPIKey, spec.egress, spec.env)
	if err != nil {
		s.Metrics.observeProvision(provisionFailed, time.Since(provisionStart))
		// Log the underlying reason: dispatch failures were previously invisible
		// (the row records only "sandbox create failed" and the 500 body is
		// unreadable cross-origin), which hid a real create failure during the
		// w3/m43 live E2E. Never logs the model placeholder or any env value.
		log.Printf("agent-session dispatch: sandbox create failed (session=%s repo=%s): %v", record.ID, record.Repo, err)
		// Record a distinct reason for a plan-limit refusal so the dashboard can
		// offer an upgrade action instead of a dead-end retry (the common local /
		// free-tier failure: too many live sandboxes for the workspace's quota).
		reason := "sandbox create failed"
		if sandbox.IsCapacityLimit(err) {
			reason = sandbox.CapacityFailureReason
		}
		s.abandonDispatch(ctx, record, spec.turn, reason)
		s.settleDispatchTurn(ctx, record.ID, spec.turn, "sandbox provisioning failed")
		return store.AgentSession{}, err
	}
	s.Metrics.observeProvision(provisionRunning, time.Since(provisionStart))
	if err := s.Sandbox.EnterAgentSessionPhase(ctx, ws, record.ID, sb.ID, spec.modelEndpoint, spec.egress); err != nil {
		log.Printf("agent-session dispatch: egress phase transition failed (session=%s): %v", record.ID, err)
		s.abandonDispatch(ctx, record, spec.turn, "egress phase transition failed")
		s.settleDispatchTurn(ctx, record.ID, spec.turn, "sandbox egress transition failed")
		return store.AgentSession{}, err
	}
	phase := PhaseCreating
	if sb.Status == sandbox.StatusRunning {
		phase = PhaseRunning
	}
	// The CAS in RecordAgentSessionDispatch (see its doc) rejects a session a
	// concurrent Cancel already took terminal, returning ErrNotFound; we then tear
	// the just-created sandbox back down rather than orphan it.
	bound, err := s.Store.RecordAgentSessionDispatch(ctx, record.ID, sb.ID, phase, string(sb.Status), spec.delivery, spec.turn)
	if err != nil {
		s.abandonDispatch(ctx, record, spec.turn, "sandbox dispatch record failed")
		s.settleDispatchTurn(ctx, record.ID, spec.turn, "sandbox dispatch record failed")
		return store.AgentSession{}, mapStoreError(record.ID, err)
	}
	return bound, nil
}

func (s *Service) settleDispatchTurn(ctx context.Context, sessionID string, turn int, reason string) {
	if turn <= 0 {
		return
	}
	if err := s.Store.CompleteAgentSessionTurn(ctx, sessionID, turn, false, false, reason); err != nil {
		log.Printf("agent-session dispatch: turn completeness write failed (session=%s turn=%d): %v", sessionID, turn, err)
	}
}

// Capabilities projects the workspace's mobile-safe agent-composer readiness
// (w11/m6 t001): which agent profiles are selectable and whether GitHub and a
// BYO model key are provisioned — never model endpoints, templates, egress,
// installation ids, or credentials. It authorizes against the target workspace
// first (cross-workspace callers are denied like every other verb). When the
// feature is not wired it returns Enabled:false with empty readiness rather than
// an error, so the phone can render a desktop-configuration callout; an unready
// GitHub App carries its install URL as the remediation, not a phone-editable
// secret. Ready folds both provisioning gates into a single submit signal.
func (s *Service) Capabilities(ctx context.Context, ownerID string) (Capabilities, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return Capabilities{}, err
	}
	caps := Capabilities{Enabled: s.enabled() && s.modelProxyEnabled() && s.ticketEnabled()}
	if !caps.Enabled {
		return caps, nil
	}
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return Capabilities{}, core.ErrForbidden
	}
	caps.Agents = agentProfiles()
	var key string
	var connection github.Connection
	var group errgroup.Group
	group.Go(func() error {
		var err error
		key, err = s.modelAPIKey(ctx, workspaceID)
		return err
	})
	if s.GitHub != nil {
		group.Go(func() error {
			var err error
			connection, err = s.GitHub.GetConnection(ctx, workspaceID)
			if err != nil {
				log.Printf("agent-session capabilities: read GitHub readiness failed (workspace=%s): %v", workspaceID, err)
				return core.NewAgentSessionDependencyUnavailableError()
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return Capabilities{}, err
	}
	caps.ModelKeyReady = key != ""
	if s.GitHub != nil {
		caps.GitHub = GitHubReadinessView{
			Connected:    connection.Connected,
			AccountLogin: connection.AccountLogin,
			InstallURL:   connection.InstallURL,
		}
	}
	caps.Ready = caps.GitHub.Connected && caps.ModelKeyReady
	return caps, nil
}

// validPhases is the closed phase vocabulary a list filter may name.
var validPhases = map[string]bool{
	PhaseCreating: true, PhaseRunning: true, PhaseResuming: true,
	PhaseRedispatching: true, PhaseCompleted: true, PhaseFailed: true,
	PhaseCanceling: true, PhaseCanceled: true,
	PhaseHibernating: true, PhaseHibernated: true,
}

// normalizeLimit applies the shared page-bound policy of the ADR065 reads:
// omitted (0) takes the default, negative is the coded 400 (never a silent
// default page), oversized clamps to max. Kept local rather than on core's
// clampers, which deliberately don't reject negatives.
func normalizeLimit(limit, def, max int) (int, error) {
	switch {
	case limit == 0:
		return def, nil
	case limit < 0:
		return 0, core.NewBadRequestError("AGENT_SESSION_FILTER_INVALID",
			"limit must be a positive integer", map[string]any{"limit": limit})
	case limit > max:
		return max, nil
	}
	return limit, nil
}

// storeQuery validates the wire filters and normalizes the page bounds into the
// store's typed query. Nothing accepted is ignored: an unknown archived value
// or phase is a named 400, never a silent full (or empty) list.
func (req *ListRequest) storeQuery() (store.AgentSessionListQuery, error) {
	q := store.AgentSessionListQuery{
		Repo:          strings.TrimSpace(req.Repo),
		CreatedBefore: req.CreatedBefore,
		CreatedAfter:  req.CreatedAfter,
		Cursor:        strings.TrimSpace(req.Cursor),
	}
	switch req.Archived {
	case "", ArchivedFalse:
		q.Archived = store.ArchivedExclude
	case ArchivedTrue:
		q.Archived = store.ArchivedOnly
	case ArchivedAll:
		q.Archived = store.ArchivedInclude
	default:
		return store.AgentSessionListQuery{}, core.NewBadRequestError("AGENT_SESSION_FILTER_INVALID",
			"archived must be one of false|true|all", map[string]any{"archived": req.Archived})
	}
	for _, phase := range req.Phases {
		if !validPhases[phase] {
			return store.AgentSessionListQuery{}, core.NewBadRequestError("AGENT_SESSION_FILTER_INVALID",
				"unknown phase filter", map[string]any{"phase": phase})
		}
	}
	q.Phases = req.Phases
	limit, err := normalizeLimit(req.Limit, defaultListLimit, maxListLimit)
	if err != nil {
		return store.AgentSessionListQuery{}, err
	}
	q.Limit = limit
	return q, nil
}

// List returns one filtered page of the workspace's sessions (ADR065 D3),
// newest first. The default request is the unarchived working set, one page of
// defaultListLimit; the prior unbounded full-history read no longer exists. The
// next page's cursor is the returned page's last item id (Render's idiom: a
// shorter or empty page signals the end).
func (s *Service) List(ctx context.Context, req ListRequest) ([]View, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return nil, err
	}
	if !s.enabled() {
		return nil, core.ErrAgentSessionsUnavailable
	}
	query, err := req.storeQuery()
	if err != nil {
		return nil, err
	}
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return nil, core.ErrForbidden
	}
	rows, err := s.Store.ListAgentSessions(ctx, workspaceID, query)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, row := range rows {
		view, err := s.toView(row)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

// errArchived is the coded refusal every mutation verb returns on an archived
// session (ADR065 D1): identical across REST/GraphQL/MCP, matching Devin's
// "archived sessions can be viewed but not modified or resumed". Unarchive
// (and Cancel, the safety verb) stay allowed.
func errArchived(phase string) error {
	return core.NewConflictError("AGENT_SESSION_ARCHIVED",
		"agent session is archived; unarchive it before modifying it", map[string]any{"phase": phase})
}

// Archive puts a session out of the working set (ADR065 D1). It is idempotent,
// allowed in any phase, and never touches the lifecycle — but it zeroes the
// Active-tier idle grace: an archived session's still-live sandbox is reclaimed
// at the Completer's next tick (an in-flight turn is NOT canceled; the turn
// finishes and delivers, then the sandbox is reclaimed immediately).
func (s *Service) Archive(ctx context.Context, sessionID string) (View, error) {
	return s.setArchived(ctx, sessionID, true)
}

// Unarchive returns an archived session to the working set, re-enabling the
// mutation verbs. It does not rehydrate — the next Resume/Steer does.
func (s *Service) Unarchive(ctx context.Context, sessionID string) (View, error) {
	return s.setArchived(ctx, sessionID, false)
}

func (s *Service) setArchived(ctx context.Context, sessionID string, archived bool) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.SetAgentSessionArchived(ctx, sessionID, archived)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return s.toView(record)
}

// finishedPhase reports a session past all live work: terminal (completed/
// failed/canceled) or hibernated. Two ADR065 verbs key off it — Delete accepts
// only finished sessions (a live one must Cancel first; deliberately no
// archive/cancel escape hatch on a destructive verb), and a pod-less finished
// session's history is replayable via a replay-only read ticket (D2).
func finishedPhase(phase string) bool {
	switch phase {
	case PhaseCompleted, PhaseFailed, PhaseCanceled, PhaseHibernated:
		return true
	}
	return false
}

// Delete hard-deletes a session (ADR065 D4): the row, its cascading transcript,
// and — blob-before-row, mirroring the retention sweep — its hibernation
// snapshot. It is the one destructive agent-session verb, so the irreversible
// sink is gated by a FRESH (uncached) re-authorization per the ADR061 #8 /
// ADR063 #7 seam. A terminal session still holding its idle-grace sandbox is
// torn down first so the reaper (which works off the row) can never be left
// with an orphaned pod. The session's OpenFGA parent tuple is left behind as an
// accepted orphan: ids are never re-minted, and every verb 404s at the store
// fetch, so the dangling tuple grants access to nothing.
func (s *Service) Delete(ctx context.Context, sessionID string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return err
	}
	if !s.enabled() {
		return core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return mapStoreError(sessionID, err)
	}
	if !finishedPhase(record.Phase) {
		return core.NewConflictError("AGENT_SESSION_NOT_DELETABLE",
			"agent session is live; cancel it before deleting", map[string]any{"phase": record.Phase})
	}
	// Fresh re-check immediately before the destructive sink: a membership
	// revoked within the positive-cache TTL must not ride a stale allow into an
	// irreversible delete. The preceding AuthorizeOn already audited the verb.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return err
	}
	if record.SandboxID != "" {
		if err := s.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
			return err // retryable: never delete the row while its pod may survive
		}
	}
	if record.SnapshotRef != "" {
		// Blob BEFORE row (the sweep's ordering): a failed object delete refuses
		// the whole verb (retryable) rather than orphaning an unreachable blob.
		if s.Snapshots == nil {
			return core.NewAgentSessionSnapshotUnavailableError()
		}
		if err := s.Snapshots.Delete(ctx, record.SnapshotRef); err != nil {
			log.Printf("agent-session delete: snapshot delete failed (session=%s): %v", record.ID, err)
			return core.NewAgentSessionSnapshotUnavailableError()
		}
	}
	if err := s.Store.DeleteAgentSession(ctx, sessionID); err != nil {
		return mapStoreError(sessionID, err)
	}
	return nil
}

// transcriptPageBytes bounds one transcript page's payload (ADR065 D2) — the
// gateway replay's page budget, reused so a poll-shaped read can never
// materialize the full 64 MiB session quota in one response.
const transcriptPageBytes = 4 << 20

// transcript part-count page bounds (ADR065 D2).
const (
	defaultTranscriptLimit = 200
	maxTranscriptLimit     = 1000
)

// Transcript returns one seq-keyset page of a session's durable conversation
// (ADR065 D2) — the poll-shaped REST/GraphQL/MCP twin of the gateway stream
// replay, readable for live, terminal, and archived sessions alike (viewable is
// the point of the archive). Parts are verbatim stored payloads.
func (s *Service) Transcript(ctx context.Context, sessionID string, afterSeq int64, limit int) (TranscriptPage, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return TranscriptPage{}, err
	}
	if !s.enabled() {
		return TranscriptPage{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return TranscriptPage{}, err
	}
	limit, err := normalizeLimit(limit, defaultTranscriptLimit, maxTranscriptLimit)
	if err != nil {
		return TranscriptPage{}, err
	}
	if afterSeq < -1 {
		afterSeq = -1
	}
	parts, err := s.Store.AgentSessionTranscript(ctx, sessionID, afterSeq, transcriptPageBytes, limit)
	if err != nil {
		return TranscriptPage{}, err
	}
	// An empty page is the one case where "unknown session" and "no transcript
	// yet" are ambiguous — resolve the row only then, so the common polled read
	// costs a single query while an unknown id still answers 404.
	if len(parts) == 0 {
		if _, err := s.Store.GetAgentSession(ctx, sessionID); err != nil {
			return TranscriptPage{}, mapStoreError(sessionID, err)
		}
	}
	turnRows, err := s.Store.AgentSessionTurns(ctx, sessionID)
	if err != nil {
		return TranscriptPage{}, err
	}
	page := TranscriptPage{
		Parts:        make([]TranscriptPart, 0, len(parts)),
		Turns:        make([]TranscriptTurn, 0, len(turnRows)),
		NextAfterSeq: afterSeq,
	}
	for _, turn := range turnRows {
		page.Turns = append(page.Turns, TranscriptTurn{
			Turn: turn.Turn, Prompt: turn.Prompt, DeliveryMode: turn.DeliveryMode,
			TranscriptComplete:  turn.TranscriptComplete,
			TranscriptTruncated: turn.TranscriptTruncated,
			TruncationReason:    turn.TruncationReason, CreatedAt: turn.CreatedAt,
			CompletedAt: turn.CompletedAt,
		})
	}
	for _, p := range parts {
		part := json.RawMessage(p.Part)
		if !json.Valid(part) {
			// Defensive: both writers store valid JSON, but a raw payload must
			// never be able to corrupt the response envelope.
			quoted, _ := json.Marshal(string(p.Part))
			part = quoted
		}
		page.Parts = append(page.Parts, TranscriptPart{Seq: p.Seq, PartIndex: p.PartIndex, Turn: p.Turn, Part: part, CreatedAt: p.CreatedAt})
		page.NextAfterSeq = p.Seq
	}
	return page, nil
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
	return s.toView(record)
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
	// Resume can reactivate or provision broker-eligible compute. A contributor
	// may operate a live session, but cannot use that lifecycle permission to
	// revive it after losing the workspace's create authority.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanCreate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if record.ArchivedAt != nil {
		return View{}, errArchived(record.Phase) // ADR065 D1: unarchive first
	}
	// ADR059 D4: a hibernated session has no live pod — rehydrate it from its
	// object-storage snapshot into a fresh sandbox instead of waking a pod.
	if record.Phase == PhaseHibernated {
		return s.rehydrate(ctx, record, "", DeliveryRehydrate)
	}
	if record.Phase == PhaseCanceled || record.Phase == PhaseCanceling || record.SandboxID == "" {
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_RESUMABLE", "agent session cannot be resumed from its current phase", map[string]any{"phase": record.Phase})
	}
	// ADR059 D6: a terminal session resuming re-enters a live phase, so it counts
	// toward the workspace's live-sandbox cap; refuse rather than exceed it.
	if err := s.enforceLiveSandboxCap(ctx, record.WorkspaceID); err != nil {
		return View{}, err
	}
	if err := s.RequireBillingMutation(ctx, record.WorkspaceID); err != nil {
		return View{}, err
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseResuming, "resuming", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	// Accept fast: the same sandbox is woken in the background, converging to
	// running (or failed). The client re-attaches via AttachTicket once running.
	s.detach(ctx, func(ctx context.Context) { s.runResume(ctx, record) })
	return s.toView(record)
}

// runResume wakes an idle session's existing sandbox in the background and
// converges the phase. A concurrent cancel is honored: setLifecycleIfActive
// refuses to flip a session a Cancel already took back to running.
func (s *Service) runResume(ctx context.Context, record store.AgentSession) {
	if err := s.Sandbox.ResumeAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
		log.Printf("agent-session resume: sandbox resume failed (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
		s.setLifecycleIfActive(ctx, record.ID, "", PhaseFailed, "sandbox resume failed")
		return
	}
	s.setLifecycleIfActive(ctx, record.ID, "", PhaseRunning, "running")
}

// restoreEnvVar is the non-secret env var carrying the presigned GET URL the
// fresh sandbox's setup fetches its hibernation snapshot from (ADR059 D4). The
// URL is single-object + time-boxed; the sandbox never receives a durable
// credential. The agent image's entrypoint untars it before the driver starts.
const restoreEnvVar = "BEX_AGENT_RESTORE_URL"

// restoreEnvSHA / restoreEnvBytes are the recorded snapshot digest and size
// the driver verifies after download, before tar extract (codex round-15 #7).
// Empty SHA / zero bytes leave verification optional so a row that predates
// digest recording still restores.
const (
	restoreEnvSHA   = "BEX_AGENT_RESTORE_SHA"
	restoreEnvBytes = "BEX_AGENT_RESTORE_BYTES"
)

// rehydrate is the shared ADR059 D4 restore path used by Resume and by Steer of
// a hibernated session: it mints a presigned snapshot download URL, claims the
// row (hibernated → resuming, keeping the snapshot for retry), and dispatches a
// fresh sandbox that hydrates its workspace from the snapshot before running.
// steerPrompt, when non-empty, rides the turn (a hibernated Steer). It fails
// closed when the snapshot store is unwired or the row carries no snapshot.
func (s *Service) rehydrate(ctx context.Context, record store.AgentSession, steerPrompt, delivery string) (View, error) {
	if !s.modelProxyEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if s.Snapshots == nil || record.SnapshotRef == "" {
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_RESUMABLE",
			"hibernated session has no restorable snapshot", map[string]any{"phase": record.Phase})
	}
	if err := s.admitDispatch(ctx, record.WorkspaceID); err != nil {
		return View{}, err
	}
	restoreURL, err := s.Snapshots.PrepareDownload(ctx, record.SnapshotRef)
	if err != nil {
		log.Printf("agent-session rehydrate: prepare snapshot restore failed (session=%s): %v", record.ID, err)
		return View{}, core.NewAgentSessionSnapshotUnavailableError()
	}
	config, err := decodeAgentConfig(record)
	if err != nil {
		return View{}, err
	}
	modelEndpoint, egressAllowlist, err := createEgress(config, nil)
	if err != nil {
		return View{}, err
	}
	config.ModelEndpoint = modelEndpoint
	modelAPIKey, err := s.modelCredential(ctx, record.WorkspaceID, record.ID)
	if err != nil {
		return View{}, err
	}
	// Claim the row (hibernated → resuming). The snapshot stays on the row so a
	// background failure can AbortRehydrate back to hibernated and retry later.
	record, err = s.Store.BeginRehydrate(ctx, record.ID, steerPrompt, delivery)
	if err != nil {
		return View{}, mapStoreError(record.ID, err)
	}
	env := s.driverEnv(config, record)
	env[restoreEnvVar] = restoreURL
	if record.SnapshotSHA != "" {
		env[restoreEnvSHA] = record.SnapshotSHA
	}
	if record.SnapshotBytes > 0 {
		env[restoreEnvBytes] = strconv.FormatInt(record.SnapshotBytes, 10)
	}
	// Resume is restore-only: never rerun config.Task. A hibernated Steer carries
	// exactly the durable prompt inserted by BeginRehydrate above.
	env["BEX_AGENT_PROMPT"] = steerPrompt
	acceptedTurn := 0
	if steerPrompt != "" {
		env["BEX_AGENT_TURN"] = fmt.Sprintf("%d", record.Turns)
		acceptedTurn = record.Turns
	}
	spec := dispatchSpec{
		template:      strings.TrimSpace(config.Template),
		modelEndpoint: modelEndpoint,
		modelAPIKey:   modelAPIKey,
		egress:        egressAllowlist,
		env:           env,
		delivery:      delivery,
		turn:          acceptedTurn,
		task:          config.Task,
	}
	s.detach(ctx, func(ctx context.Context) { s.runRehydrate(ctx, record, spec) })
	return s.toView(record)
}

// runRehydrate provisions the fresh restore sandbox in the background and
// converges the phase (ADR059 D4). On any provisioning failure it reverts the
// row to hibernated (AbortRehydrate) so the snapshot survives for a later Resume
// — a rehydrate failure must never lose the durable workspace. On success it
// adopts the sandbox and clears the snapshot fields (RehydrateAgentSession).
func (s *Service) runRehydrate(ctx context.Context, record store.AgentSession, spec dispatchSpec) {
	ws := record.WorkspaceID
	// Fresh pod with prior turns and a prompt to run: hand the driver its
	// rung-2/3 priming material (ADR047 D3 ladder, w5/m84). Computed here, off
	// the accept path — the transcript read is background work. Rung 1
	// (session/load) stays the driver's call: it wins when the restored
	// snapshot carries agent state. A restore-only resume (turn 0) runs no
	// prompt and needs no priming.
	if spec.turn > 0 {
		s.continuityEnv(ctx, spec.env, record, spec.task)
	}
	// Resume-latency instrumentation (ADR059 D4 SLOs p50<~5s/p95<~15s): time the
	// cold restore from claim to a live sandbox. The dominant factor is the pod
	// schedule + the in-sandbox snapshot fetch/untar; the number is logged for the
	// SLO watch (a live acceptance records the distribution).
	start := s.Now()
	sb, err := s.Sandbox.CreateAgentSessionSandbox(ctx, ws, spec.template, record.ID, record.Repo, record.Branch, spec.modelEndpoint, spec.modelAPIKey, spec.egress, spec.env)
	if err != nil {
		log.Printf("agent-session rehydrate: sandbox create failed (session=%s): %v", record.ID, err)
		s.settleDispatchTurn(ctx, record.ID, spec.turn, "sandbox restore provisioning failed")
		s.abortRehydrate(ctx, record.ID)
		return
	}
	if err := s.Sandbox.EnterAgentSessionPhase(ctx, ws, record.ID, sb.ID, spec.modelEndpoint, spec.egress); err != nil {
		log.Printf("agent-session rehydrate: egress phase transition failed (session=%s): %v", record.ID, err)
		_ = s.Sandbox.CancelAgentSessionSandbox(ctx, ws, record.ID, sb.ID)
		s.settleDispatchTurn(ctx, record.ID, spec.turn, "sandbox restore egress transition failed")
		s.abortRehydrate(ctx, record.ID)
		return
	}
	phase := PhaseCreating
	if sb.Status == sandbox.StatusRunning {
		phase = PhaseRunning
	}
	if _, err := s.Store.RehydrateAgentSession(ctx, record.ID, sb.ID, phase, string(sb.Status), spec.delivery); err != nil {
		log.Printf("agent-session rehydrate: record dispatch failed (session=%s): %v", record.ID, err)
		_ = s.Sandbox.CancelAgentSessionSandbox(ctx, ws, record.ID, sb.ID)
		s.settleDispatchTurn(ctx, record.ID, spec.turn, "sandbox restore dispatch record failed")
		s.abortRehydrate(ctx, record.ID)
		return
	}
	log.Printf("agent-session rehydrate: session=%s resumed in %s (SLO p50<5s/p95<15s)", record.ID, s.Now().Sub(start).Round(time.Millisecond))
}

// abortRehydrate reverts a failed rehydrate to hibernated so the snapshot is
// retriable; a failure to revert is logged (the row stays resuming until a
// manual/retry converges it, but the durable snapshot is never deleted here).
func (s *Service) abortRehydrate(ctx context.Context, id string) {
	if _, err := s.Store.AbortRehydrate(ctx, id); err != nil {
		log.Printf("agent-session rehydrate: revert to hibernated failed (session=%s): %v", id, err)
	}
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
		return s.toView(record) // idempotent public cancel
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
	// ADR059: an explicit Cancel reclaims immediately, including a hibernated
	// session's durable snapshot. Best-effort delete before finalizing — a failed
	// object delete only orphans a blob (the retention sweep would have deleted it
	// anyway); it must not block the cancel.
	if record.SnapshotRef != "" && s.Snapshots != nil {
		if err := s.Snapshots.Delete(ctx, record.SnapshotRef); err != nil {
			log.Printf("agent-session cancel: delete snapshot failed (session=%s ref=%s): %v", record.ID, record.SnapshotRef, err)
		}
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseCanceled, "canceled", true)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return s.toView(record)
}

// Pin marks a session as never-expire (ADR059 D5): a pinned hibernated workspace
// is kept indefinitely (its retention deadline removed) but is still metered for
// storage and counted against the per-workspace pin quota. Pinning is authorized
// like the other lifecycle verbs (can_operate). Idempotent.
func (s *Service) Pin(ctx context.Context, sessionID string) (View, error) {
	return s.setPin(ctx, sessionID, true)
}

// Unpin removes the never-expire pin (ADR059 D5), putting a hibernated session
// back on the retention clock (now + the retention window) so it can expire
// normally; a non-hibernated session just loses the flag.
func (s *Service) Unpin(ctx context.Context, sessionID string) (View, error) {
	return s.setPin(ctx, sessionID, false)
}

func (s *Service) setPin(ctx context.Context, sessionID string, pinned bool) (View, error) {
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
	if record.ArchivedAt != nil {
		return View{}, errArchived(record.Phase) // ADR065 D1: unarchive first
	}
	if pinned && !record.Pinned {
		if err := s.enforcePinQuota(ctx, record.WorkspaceID); err != nil {
			return View{}, err
		}
	}
	var retainUntil *time.Time
	if !pinned {
		t := s.Now().Add(s.retentionTTL())
		retainUntil = &t
	}
	record, err = s.Store.SetAgentSessionPinned(ctx, sessionID, pinned, retainUntil)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return s.toView(record)
}

// retentionTTL mirrors the Completer's ADR059 D5 window for an unpin's new
// deadline. 0 ⇒ the 7d default.
func (s *Service) retentionTTL() time.Duration {
	if s.RetentionTTL > 0 {
		return s.RetentionTTL
	}
	return 7 * 24 * time.Hour
}

// enforcePinQuota bounds a workspace's pinned sessions (ADR059 D5). 0 ⇒ uncapped.
// Best-effort like the live cap; a store error fails closed.
func (s *Service) enforcePinQuota(ctx context.Context, workspaceID string) error {
	if s.MaxPinnedSandboxes <= 0 {
		return nil
	}
	n, err := s.Store.CountPinnedAgentSessions(ctx, workspaceID)
	if err != nil {
		return mapStoreError("", err)
	}
	if n >= s.MaxPinnedSandboxes {
		return core.NewConflictError("AGENT_SESSION_PIN_LIMIT",
			fmt.Sprintf("this workspace already has %d pinned agent sessions (limit %d); unpin one before pinning another", n, s.MaxPinnedSandboxes),
			map[string]any{"pinned": n, "limit": s.MaxPinnedSandboxes})
	}
	return nil
}

func agentCommand(agent string) string {
	return agentsession.AgentProfileCommand(agent)
}

func agentRuntimeConfig(agent string) (argsJSON, envJSON string) {
	return agentsession.AgentProfileRuntimeJSON(agent)
}

// defaultCredentialURL is the trusted in-cluster Git smart-HTTP proxy origin the
// sandbox clones from (BEX_AGENT_REPO_URL). It MUST be the fully-qualified
// `.svc.cluster.local` name, not the short `.svc` form: the sandbox's phase-split
// egress (ADR047 D5) applies a Cilium L7 DNS filter that allows resolving ONLY the
// exact FQDN (internal/sessionegress `credentialGatewayHost`). Under that filter a
// short-name lookup never reaches the allowed expansion and returns EAI_AGAIN, so
// the clone fails before the gateway is ever contacted (verified live in the
// tenant sandbox: FQDN resolves + connects on :8082, `.svc` fails DNS).
const defaultCredentialURL = "http://bex-ssh-gateway.bex-system.svc.cluster.local:8082"

func (s *Service) gitProxyURL() string {
	if strings.TrimSpace(s.GitProxyURL) != "" {
		return s.GitProxyURL
	}
	return defaultCredentialURL
}

// driverEnv renders the non-secret environment the sandbox driver reads to run
// one headless delivery turn (lego/agent-image/driver). The BYO model key never
// enters this map or the sandbox; a session-bound placeholder is paired with the
// mandatory gateway base URL. The Git remote is a non-secret, Pod-bound proxy URL;
// the raw GitHub token is minted and consumed only by the gateway. The
// driver-grant value is likewise only an Ed25519 public verification key.
func (s *Service) driverEnv(config AgentConfig, record store.AgentSession) map[string]string {
	namespace := store.SandboxNamespace(record.WorkspaceID)
	// Repo-less (chat-only) sessions carry no clone URL and never deliver: the
	// driver runs the agent against the prompt in an empty /workspace and the
	// completer records an honest no-op. With a repo, the sandbox clones through
	// the gateway Git proxy and delivers a draft PR (ADR047 D2/D4).
	repoURL, deliver := "", "0"
	if strings.TrimSpace(record.Repo) != "" {
		repoURL, _ = agentsession.ProxyRepositoryURL(s.gitProxyURL(), namespace, record.ID, record.Repo, record.Branch)
		deliver = "1"
	}
	env := map[string]string{
		"BEX_AGENT_PROFILE":          config.Agent,
		"BEX_AGENT_COMMAND":          agentCommand(config.Agent),
		"BEX_AGENT_PROMPT":           config.Task,
		"BEX_AGENT_BRANCH":           record.Branch,
		"BEX_AGENT_REPO_URL":         repoURL,
		"BEX_AGENT_DELIVER":          deliver,
		"BEX_AGENT_EXIT_AFTER_TURN":  "0",
		"BEX_SANDBOX_NAMESPACE":      namespace,
		"BEX_AGENT_SESSION_ID":       record.ID,
		"BEX_AGENT_REPOSITORY":       record.Repo,
		"BEX_AGENT_GRANT_PUBLIC_KEY": s.driverGrantPublicKey(),
		"BEX_AGENT_TURN":             fmt.Sprintf("%d", max(1, record.Turns)),
	}
	argsJSON, childEnvJSON := agentRuntimeConfig(config.Agent)
	if argsJSON != "" {
		env["BEX_AGENT_ARGS"] = argsJSON
	}
	if childEnvJSON != "" {
		env["BEX_AGENT_ENV_JSON"] = childEnvJSON
	}
	if base := s.modelProxyBaseURL(); base != "" {
		// The per-session model base URL carries no credential; the driver appends
		// the per-provider path suffix and points the agent at it (ADR062 D5).
		if url, err := agentsession.ModelProxyURL(base, namespace, record.ID); err == nil {
			env["BEX_AGENT_MODEL_PROXY_URL"] = url
		}
	}
	// Bound one turn (w5/m80 t002): the driver's own fallback is otherwise 4h, so
	// a hung model call or a runaway tool loop would hold the sandbox for hours.
	// Always inject a positive millisecond bound; <= 0 ⇒ the 30m default.
	env["BEX_AGENT_TURN_TIMEOUT_MS"] = fmt.Sprintf("%d", s.turnTimeout().Milliseconds())
	return env
}

// defaultTurnTimeout bounds one agent turn when BEX_AGENT_TURN_TIMEOUT is unset.
// It is generous enough for a real coding task yet far below the driver's 4h
// fallback, so a stuck turn converges to failed in tens of minutes, not hours.
const defaultTurnTimeout = 30 * time.Minute

func (s *Service) turnTimeout() time.Duration {
	if s.TurnTimeout > 0 {
		return s.TurnTimeout
	}
	return defaultTurnTimeout
}

// modelProxyBaseURL is the internal gateway model-proxy origin (BEX_AGENT_MODEL_
// PROXY_URL). Agent-session mutation verbs fail closed when it is empty.
func (s *Service) modelProxyBaseURL() string { return strings.TrimSpace(s.ModelProxyURL) }

// modelCredential returns what lands in the sandbox pod's BEX_AGENT_MODEL_API_KEY
// env: always a session-bound placeholder. The real key stays on the gateway;
// there is deliberately no direct-key fallback.
func (s *Service) modelCredential(_ context.Context, _, sessionID string) (string, error) {
	if s.modelProxyBaseURL() == "" {
		return "", core.ErrAgentSessionsUnavailable
	}
	return agentsession.ModelKeyPlaceholder(sessionID), nil
}

func (s *Service) driverGrantPublicKey() string {
	key, _ := drivergrant.PublicKey(s.TicketSecret) // Enabled() already requires a non-empty secret.
	return key
}

// Steer runs a follow-up prompt turn on an existing session (ADR047 D8 phase 1,
// ADR047 D8 phase 1). In phase 1 a new prompt cannot ride the original sandbox — its
// prompt env is fixed at creation and there is no live attach yet — so steering
// re-dispatches a fresh sandbox that re-clones the same bex-agent/* branch and
// runs the new prompt; the Completer then updates the same draft PR. The
// delivery mode is recorded on the row.
func (s *Service) Steer(ctx context.Context, req SteerRequest) (View, error) {
	// SECURITY: steering dispatches a fresh attacker-directed task with provider
	// access through the credential proxy and caller-supplied egress. Gate on the
	// same can_create relation as Create, against the session's own parent tuple.
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, sessionObject(req.SessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() || !s.modelProxyEnabled() || !s.ticketEnabled() {
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
	if record.ArchivedAt != nil {
		return View{}, errArchived(record.Phase) // ADR065 D1: unarchive first
	}
	switch record.Phase {
	case PhaseCanceled, PhaseCanceling:
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_STEERABLE", "a canceled agent session cannot be steered", map[string]any{"phase": record.Phase})
	case PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching, PhaseHibernating:
		return View{}, core.NewConflictError("AGENT_SESSION_TURN_IN_FLIGHT", "a turn is already running; wait for it to finish before steering", map[string]any{"phase": record.Phase})
	case PhaseHibernated:
		// ADR059 D4: steering a hibernated session rehydrates it from its snapshot
		// and runs the steer prompt on the restored workspace (uncommitted edits +
		// installed deps intact), instead of re-cloning over lost state.
		return s.rehydrate(ctx, record, prompt, DeliveryRehydrate)
	}
	if err := s.admitDispatch(ctx, record.WorkspaceID); err != nil {
		return View{}, err
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
	// Resolve the session-bound placeholder before flipping the phase so a
	// configuration error cannot strand the session in redispatching. The real
	// BYO key remains in OpenBao and is minted only to the gateway (ADR062).
	modelAPIKey, err := s.modelCredential(ctx, record.WorkspaceID, record.ID)
	if err != nil {
		return View{}, err
	}
	previousSandboxID := record.SandboxID
	record, err = s.Store.BeginAgentSessionTurn(ctx, record.ID, prompt, DeliveryRedispatch, PhaseRedispatching, "redispatching")
	if err != nil {
		return View{}, mapStoreError(record.ID, err)
	}
	env := s.driverEnv(config, record)
	env["BEX_AGENT_PROMPT"] = prompt // the steering prompt overrides the original task
	spec := dispatchSpec{
		template:          strings.TrimSpace(config.Template),
		modelEndpoint:     modelEndpoint,
		modelAPIKey:       modelAPIKey,
		egress:            egressAllowlist,
		env:               env,
		delivery:          DeliveryRedispatch,
		turn:              record.Turns,
		previousSandboxID: previousSandboxID,
		task:              config.Task,
	}
	// Accept fast: tear the previous turn's sandbox down and re-dispatch a fresh
	// one in the background. In phase 1 a new prompt can't ride the old sandbox
	// (its prompt env is fixed at creation, no live attach yet — ADR047 D8), so the
	// client sees redispatching immediately and the new turn streams once it
	// attaches; the Completer updates the same draft PR.
	s.detach(ctx, func(ctx context.Context) { s.runSteerDispatch(ctx, record, spec) })
	return s.toView(record)
}

// runSteerDispatch tears the previous turn's sandbox down (idempotent) then
// re-dispatches a fresh one for the steering prompt, in the background.
func (s *Service) runSteerDispatch(ctx context.Context, record store.AgentSession, spec dispatchSpec) {
	// A redispatch crosses a sandbox-generation boundary exactly like a
	// hibernated resume: prime the blank agent (ADR047 D3 ladder, w5/m84).
	// Off the accept path by design — see runRehydrate.
	s.continuityEnv(ctx, spec.env, record, spec.task)
	if spec.previousSandboxID != "" {
		if err := s.Sandbox.CleanupAgentDispatches(ctx, []store.AgentDispatch{{SessionID: record.ID, WorkspaceID: record.WorkspaceID, Turn: spec.turn, PreviousSandboxID: spec.previousSandboxID}}); err != nil {
			log.Printf("agent-session steer: teardown of previous sandbox failed (session=%s sandbox=%s): %v", record.ID, spec.previousSandboxID, err)
			s.abandonDispatch(ctx, record, spec.turn, "steer teardown failed")
			s.settleDispatchTurn(ctx, record.ID, spec.turn, "previous sandbox teardown failed")
			return
		}
	}
	s.runDispatch(ctx, record, spec)
}

// AttachTicket mints a fresh attach ticket for an already-started session
// without changing its lifecycle (ADR047 D9 target API shape). It closes the
// reconnect gap: create/resume/steer mint at dispatch time, but a page reload
// or a stream drop on a running session — or opening a terminal session's
// transcript replay — needs a ticket too, and those verbs don't fire again. The
// ticket carries the same 90s TTL + single-use nonce + subject/session/pod/
// namespace claims, so the gateway authorizes reattach and terminal replay
// identically to a first connect. A reaped terminal/hibernated session (empty
// sandbox id) mints a replay-only read ticket instead (ADR065 D2); only a
// session still provisioning its first sandbox has nothing to attach to.
func (s *Service) AttachTicket(ctx context.Context, sessionID, action string) (View, error) {
	// Determine required authorization based on requested action
	var relation string
	if action == agentsessionticket.ActionTurn {
		relation = core.RelCanCreate // Developer-only for live turns
	} else {
		relation = core.RelCanOperate // Contributors can read transcripts
	}

	if err := s.AuthorizeOn(ctx, relation, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() || !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	if action != "" && action != agentsessionticket.ActionRead && action != agentsessionticket.ActionTurn {
		return View{}, fmt.Errorf("invalid action: must be 'read' or 'turn'")
	}
	// Default to "read" for compatibility with callers that don't specify action
	if action == "" {
		action = agentsessionticket.ActionRead
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.SandboxID == "" {
		// ADR065 D2: a reaped session (terminal or hibernated, its sandbox gone)
		// mints a REPLAY-ONLY read ticket with an empty pod claim — the gateway
		// serves the durable-transcript replay and never dials. This is what makes
		// "archived sessions can still be viewed" true after the idle grace. Only
		// the genuinely unattachable window (provisioning/pre-dispatch, live
		// phases with no sandbox yet) keeps the retryable refusal.
		if action == agentsessionticket.ActionRead && finishedPhase(record.Phase) {
			return s.withTicket(ctx, record, action)
		}
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_ATTACHABLE",
			"agent session has not started a sandbox yet", map[string]any{"phase": record.Phase})
	}
	// round-5 finding 13: a live-turn ticket must be gated on the same lifecycle
	// and billing state as Create/Steer. A terminal or canceling session keeps its
	// sandbox reachable through the ADR054 editor grace window, so a nonempty
	// SandboxID alone would let a developer run un-metered, off-lifecycle model
	// turns on a "finished" session. Read tickets (transcript replay of a terminal
	// session) are intentionally exempt — that is the feature.
	if action == agentsessionticket.ActionTurn {
		if record.ArchivedAt != nil {
			return View{}, errArchived(record.Phase) // ADR065 D1: no turns while archived
		}
		if !liveSandboxPhase(record.Phase) {
			return View{}, core.NewConflictError("AGENT_SESSION_NOT_LIVE",
				"agent session is not accepting live turns", map[string]any{"phase": record.Phase})
		}
		if err := s.RequireBillingMutation(ctx, record.WorkspaceID); err != nil {
			return View{}, err
		}
		// w5/m71: the gateway POST path cannot transactionally persist a prompt
		// with the control-plane lifecycle and turn counter. Refuse it rather than
		// accept work that disappears on refresh; follow-ups use Steer after the
		// current turn settles.
		return View{}, core.NewConflictError("AGENT_SESSION_LIVE_TURN_UNAVAILABLE",
			"wait for the current turn to finish, then steer the session", map[string]any{"phase": record.Phase})
	}
	return s.withTicket(ctx, record, action)
}

func (s *Service) withTicket(ctx context.Context, record store.AgentSession, action string) (View, error) {
	identity, ok := core.IdentityFrom(ctx)
	if !ok || identity.Subject == "" {
		return View{}, core.ErrForbidden
	}
	now := s.Now()
	expires := now.Add(ticketTTL)
	claims := agentsessionticket.Claims{
		Subject: identity.Subject, SessionID: record.ID, SandboxID: record.SandboxID,
		Pod: record.SandboxID + "-0", Workspace: record.WorkspaceID,
		Namespace: record.WorkspaceID + "-sandbox", Action: action, Turn: record.Turns,
		IssuedAt: now.Unix(), ExpiresAt: expires.Unix(),
	}
	if record.SandboxID == "" || (action == agentsessionticket.ActionRead && finishedPhase(record.Phase)) {
		// Replay-only ticket: no pod to bind, so the gateway serves the durable
		// transcript replay and closes with [DONE] rather than dialing the driver.
		// Two cases collapse here: a reaped session whose sandbox is gone (ADR065
		// D2), AND a finished (completed/failed) session whose sandbox is still in
		// its idle-grace window. A finished turn has nothing live to splice, so a
		// pod-bound read would hold the SSE open on keep-alives and the AI-SDK
		// client would read it as a turn perpetually in progress (composer stuck
		// "Sending…"); replay-only + [DONE] lets the conversation settle so a
		// follow-up Steer can start the next turn. The pod triple is all-or-nothing
		// (agentsessionticket.Verify), so SandboxID is cleared with Pod/Namespace.
		claims.SandboxID, claims.Pod, claims.Namespace = "", "", ""
	}
	ticket, err := agentsessionticket.Mint(s.TicketSecret, claims)
	if err != nil {
		return View{}, err
	}
	view, err := s.toView(record)
	if err != nil {
		return View{}, err
	}
	view.Ticket, view.URL, view.ExpiresAt = ticket, s.GatewayURL, &expires
	if s.APIPublicURL != "" {
		view.StreamURL = fmt.Sprintf("%s/v1/agent-sessions/%s/stream",
			strings.TrimRight(s.APIPublicURL, "/"), record.ID)
	}
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

// toView projects a stored session to its cross-surface View and enriches it
// with the SSH address (ADR054 D5). Every verb that returns a View routes
// through it, so REST, GraphQL, and MCP surface sshAddress identically.
func (s *Service) toView(record store.AgentSession) (View, error) {
	view, err := viewOf(record)
	if err != nil {
		return View{}, err
	}
	view.SSHAddress = s.sshAddress(record)
	return view, nil
}

// sshAddress returns `ags-<xid>@<BEX_SSH_HOST>` when an editor could actually
// open the sandbox: a valid public host is configured AND the sandbox is live
// (liveSandboxPhase + a non-empty sandbox id) — the exact condition the gateway
// resolver enforces, so a surfaced address never dangles. Empty otherwise.
func (s *Service) sshAddress(record store.AgentSession) string {
	host := strings.ToLower(strings.TrimSpace(s.SSHHost))
	if host == "" || !strings.Contains(host, ".") || len(validation.IsDNS1123Subdomain(host)) != 0 {
		return ""
	}
	if !liveSandboxPhase(record.Phase) || record.SandboxID == "" {
		return ""
	}
	return record.ID + "@" + host
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
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, CanceledAt: record.CanceledAt,
		Pinned: record.Pinned, SnapshotBytes: record.SnapshotBytes,
		HibernatedAt: record.HibernatedAt, RetainUntil: record.RetainUntil,
		ArchivedAt: record.ArchivedAt}, nil
}

func mapStoreError(id string, err error) error {
	if errors.Is(err, store.ErrAgentSessionPromptQuota) {
		return core.NewConflictError("AGENT_SESSION_PROMPT_QUOTA", "agent session prompt history has reached its byte limit", map[string]any{"id": id, "maxBytes": store.MaxAgentSessionPromptBytes})
	}
	if errors.Is(err, store.ErrAgentSessionTurnState) {
		return core.NewConflictError("AGENT_SESSION_TURN_IN_FLIGHT", "a turn is already running; wait for it to finish before steering", map[string]any{"id": id})
	}
	if errors.Is(err, store.ErrNotFound) {
		return core.NewNotFoundError("AGENT_SESSION_NOT_FOUND", "agent session not found", map[string]any{"id": id})
	}
	if errors.Is(err, store.ErrConflict) {
		return core.NewConflictError("AGENT_SESSION_CONFLICT", "agent session conflicts with existing state", map[string]any{"id": id})
	}
	return err
}
