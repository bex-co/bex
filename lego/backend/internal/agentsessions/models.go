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

// Package agentsessions owns the tenant-facing lifecycle of cloud coding-agent
// sessions from ADR047 D3. It records intent in the control plane, delegates
// sandbox mechanism to internal/sandbox, and only mints an attach ticket: the
// isolated gateway remains the sole session ingress.
package agentsessions

import (
	"encoding/json"
	"time"
)

const (
	PhaseCreating      = "creating"
	PhaseRunning       = "running"
	PhaseResuming      = "resuming"
	PhaseRedispatching = "redispatching"
	PhaseCompleted     = "completed"
	PhaseFailed        = "failed"
	PhaseCanceling     = "canceling"
	PhaseCanceled      = "canceled"
)

// Session states (ADR094 D7): a coarse, derived answer to "is it done?" that a
// client reads instead of decoding Phase. Phase stays the lifecycle field.
const (
	StateWorking    = "working"
	StateNeedsInput = "needs_input" // reserved: emitted once the decision-request contract ships (w11/m7)
	StateSucceeded  = "succeeded"
	StateFailed     = "failed"
	StateCanceled   = "canceled"
)

// stateOf maps a lifecycle phase to its coarse state. An unknown phase reads as
// working: a client keeps waiting rather than treating it as finished.
func stateOf(phase string) string {
	switch phase {
	case PhaseCompleted:
		return StateSucceeded
	case PhaseFailed:
		return StateFailed
	case PhaseCanceled:
		return StateCanceled
	default:
		return StateWorking
	}
}

// phasesOfState lists the phases a state covers (the inverse of stateOf);
// ok is false for a name that is not a state. needs_input covers no phase yet.
func phasesOfState(state string) ([]string, bool) {
	switch state {
	case StateWorking:
		return []string{PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching, PhaseCanceling}, true
	case StateNeedsInput:
		return nil, true
	case StateSucceeded:
		return []string{PhaseCompleted}, true
	case StateFailed:
		return []string{PhaseFailed}, true
	case StateCanceled:
		return []string{PhaseCanceled}, true
	}
	return nil, false
}

// steerable reports whether a follow-up message is accepted now: the turn has
// ended (completed or failed), the session is not canceled, and it is not
// archived. It mirrors the refusals in Steer.
func steerable(phase string, archived bool) bool {
	return !archived && (phase == PhaseCompleted || phase == PhaseFailed)
}

// Delivery mode records how a turn's sandbox was obtained (ADR047 D8 phase 1):
// resume of the existing sandbox, or a re-dispatch onto a fresh sandbox+clone.
const (
	DeliveryResume     = "resume"
	DeliveryRedispatch = "redispatch"
)

// Driver turn states written to the sandbox status file
// (lego/agent-image/driver/src/session.mjs); the Completer switches on these.
const (
	driverStateRunning   = "running"
	driverStateSucceeded = "succeeded"
	driverStateFailed    = "failed"
)

// AgentConfig is the stable cross-surface driver contract. Agent selects the
// installed ACP adapter, Model is passed to that driver, Task is the initial
// fire-and-forget prompt, and Template selects a platform-registered agent
// sandbox image (never an arbitrary image supplied by a tenant).
type AgentConfig struct {
	Agent         string `json:"agent"`
	Model         string `json:"model,omitempty"`
	ModelEndpoint string `json:"modelEndpoint,omitempty"`
	Task          string `json:"task"`
	Template      string `json:"template,omitempty"`
}

// AgentProfile is a selectable, secret-free agent option for the phone composer
// (w11/m6 t001): the short ACP-adapter selector plus a human label. Model
// endpoints, templates, and egress destinations never appear here — those stay
// desktop configuration.
type AgentProfile struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// ModelKeyReady is whether this profile can reach its model in the
	// workspace: through the platform route (the claude profile on a routed
	// workspace) or the workspace's own model key. A profile that is not ready
	// is refused at create with AGENT_SESSION_MODEL_KEY_REQUIRED.
	ModelKeyReady bool `json:"modelKeyReady"`
}

// GitHubReadinessView is the mobile-safe slice of a workspace's GitHub App
// connection: whether a draft PR can be opened, the connected account login for
// display, and — when NOT connected — the desktop install URL to remediate.
// Installation ids and tokens never cross this projection.
type GitHubReadinessView struct {
	Connected    bool   `json:"connected"`
	AccountLogin string `json:"accountLogin,omitempty"`
	InstallURL   string `json:"installUrl,omitempty"`
}

// Capabilities is the mobile-safe composer/readiness projection (w11/m6 t001).
// It reveals only what a phone needs to compose and gate a fire-and-forget
// session — selectable agent profiles and readiness booleans — and deliberately
// carries none of the raw AgentConfig secret surface (model endpoint, template,
// egress) nor any credential. Enabled is false when the feature is not wired;
// Ready folds the two provisioning gates (GitHub App + BYO model key) so the
// composer can present a single submit affordance with desktop-configuration
// callouts for whatever is missing.
type Capabilities struct {
	Enabled       bool                `json:"enabled"`
	GitHub        GitHubReadinessView `json:"github"`
	ModelKeyReady bool                `json:"modelKeyReady"`
	Agents        []AgentProfile      `json:"agents"`
	Ready         bool                `json:"ready"`
	Attachments   AttachmentLimits    `json:"attachments"`
}

// agentProfiles is the fixed, secret-free presentation metadata for the
// operator-owned release-locked agent profile manifest (claude/codex/gemini). Tests require
// exact membership parity so an uninstalled adapter cannot be advertised.
func agentProfiles() []AgentProfile {
	return []AgentProfile{
		{ID: "claude", Label: "Claude Code"},
		{ID: "codex", Label: "Codex"},
		{ID: "gemini", Label: "Gemini"},
	}
}

type CreateRequest struct {
	OwnerID         string      `json:"ownerId,omitempty"`
	ResumeSessionID string      `json:"resumeSessionId,omitempty"`
	Repo            string      `json:"repo,omitempty"`
	Branch          string      `json:"branch,omitempty"`
	AgentConfig     AgentConfig `json:"agentConfig,omitempty"`
	EgressAllowlist []string    `json:"egressAllowlist,omitempty"`
	// IdempotencyKey makes a create safe to retry (ADR094 D4): the same key and
	// request within 24 h returns the first session (created=false); the same key
	// with a different request is a 409. 1-128 printable ASCII.
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	// Title is the display title (default: the task's first line); Tags are up to
	// 20 filter and cost-attribution labels, each [a-z0-9:_-]{1,64} (ADR094 D3).
	Title string   `json:"title,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	// AttachmentIDs are uploads to attach to the first turn (ADR095 D3).
	AttachmentIDs []string `json:"attachmentIds,omitempty"`
	// OutputSchema makes the first turn's result machine-readable (ADR094 D5):
	// a JSON Schema 2020-12 object of at most 64 KiB with no external $ref. The
	// agent submits a result that validates against it; outcome.result carries
	// the last valid final one. OutputRequired (default true when a schema is
	// set) fails the turn STRUCTURED_OUTPUT_MISSING if it ends without one.
	OutputSchema   map[string]any `json:"outputSchema,omitempty" jsonschema:"a JSON Schema 2020-12 object (at most 64 KiB, no external $ref) the turn's result must satisfy; read it from outcome.result"`
	OutputRequired *bool          `json:"outputRequired,omitempty" jsonschema:"default true when outputSchema is set: a turn that ends without a final valid result fails STRUCTURED_OUTPUT_MISSING"`
	// MaxSpendUSD is a per-session ceiling on the platform model's customer
	// price in USD (ADR094 D8), under the workspace's monthly cap: once the
	// session has spent it, its next model request is refused
	// AGENT_MODEL_SPEND_CAP_REACHED (scope session) and the turn fails. Only a
	// session on the platform model route is metered; any other session is
	// refused 400 AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE rather than left
	// unbounded.
	SessionSecretsInput
	MaxSpendUSD *float64 `json:"maxSpendUSD,omitempty" jsonschema:"a ceiling on this session's platform model spend in USD (more than 0, at most 100000); the turn fails AGENT_MODEL_SPEND_CAP_REACHED with scope session once reached. Needs the platform model route; a session on the workspace's own model key is refused AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE"`
}

// SteerRequest is a follow-up prompt turn on an existing session (ADR047 D8
// phase 1). The BYO model key is sourced from OpenBao (ADR047 D7), never the
// request, so no secret rides this input.
type SteerRequest struct {
	SessionID       string   `json:"-"`
	Prompt          string   `json:"prompt,omitempty"`
	EgressAllowlist []string `json:"egressAllowlist,omitempty"`
	// AttachmentIDs are uploads to attach to this turn (ADR095 D3).
	AttachmentIDs []string `json:"attachmentIds,omitempty"`
	// OutputSchema and OutputRequired are this turn's structured-output contract
	// (ADR094 D5), exactly as on CreateRequest; a turn without a schema is free-form.
	OutputSchema   map[string]any `json:"outputSchema,omitempty"`
	OutputRequired *bool          `json:"outputRequired,omitempty"`
	SessionSecretsInput
}

// SessionSecretsInput attaches secrets to a session (ADR098 D1), Devin's
// secret_ids and session_secrets: references to workspace agent secrets and
// inline values for this session only. Both reach the agent as env vars from
// the next dispatch; values are write-only and redacted from the transcript.
type SessionSecretsInput struct {
	SecretNames []string      `json:"secretNames,omitempty" jsonschema:"names of workspace agent secrets (list_agent_secrets) to expose to the agent as env vars of the same name"`
	Secrets     []SecretInput `json:"secrets,omitempty" jsonschema:"inline secrets for this session only, deleted with it; an inline name wins over a workspace secret of the same name. At most 20 secrets per session in total"`
}

// View is byte-for-byte the same domain shape projected by REST, GraphQL, and
// MCP. Ticket/URL/ExpiresAt are present only on create/resume/steer; list/get
// never mint ambient attach authority.
type View struct {
	ID          string      `json:"id"`
	OwnerID     string      `json:"ownerId"`
	Repo        string      `json:"repo"`
	Branch      string      `json:"branch"`
	Title       string      `json:"title,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	AgentConfig AgentConfig `json:"agentConfig"`
	SandboxID   string      `json:"sandboxId,omitempty"`
	Phase       string      `json:"phase"`
	// State is the coarse derived state (ADR094 D7): working, needs_input,
	// succeeded, failed or canceled. Steerable says a follow-up message is
	// accepted now, so a finished-but-open session is never ambiguous.
	State     string `json:"state"`
	Steerable bool   `json:"steerable"`
	// MaxSpendUSD is the session's maxSpendUSD ceiling as a decimal string, or
	// absent when it has none (ADR094 D8).
	MaxSpendUSD string `json:"maxSpendUSD,omitempty"`
	// SecretNames are the names of the secrets attached to the session
	// (ADR098); a value is never returned.
	SecretNames  []string `json:"secretNames,omitempty"`
	Status       string   `json:"status"`
	HeadSHA      string   `json:"headSha,omitempty"`
	PRURL        string   `json:"prUrl,omitempty"`
	PRNumber     int      `json:"prNumber,omitempty"`
	Turns        int      `json:"turns,omitempty"`
	DeliveryMode string   `json:"deliveryMode,omitempty"`
	// DeliveryNote is the driver's plain-language account of what a completed
	// turn left undelivered (ADR090): uncommitted changes, unpushed commits, or
	// a pushed branch with no pull request. The work stays in the sandbox only
	// until it is closed after the idle TTL. Empty when the agent delivered.
	DeliveryNote  string `json:"deliveryNote,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	// FailureReasonCode is a stable code for the failure's cause when a client
	// acts on it: SANDBOX_CAPACITY_LIMIT for a dispatch refused at the
	// workspace's sandbox limit (w5/m132). Clients decide by it, never by the
	// sentence's wording.
	FailureReasonCode string     `json:"failureReasonCode,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	CanceledAt        *time.Time `json:"canceledAt,omitempty"`
	// SSHAddress is the Render-shaped `ags-<xid>@<BEX_SSH_HOST>` an editor uses to
	// open the session's sandbox over SSH (ADR054 D5 — "Open in Zed"). Present only
	// when BEX_SSH_HOST is configured AND the sandbox is live (the exact condition
	// the gateway resolver enforces), so a surfaced address always connects; empty
	// otherwise. It carries no credential — auth is the caller's own SSH key.
	SSHAddress string `json:"sshAddress,omitempty"`
	Ticket     string `json:"ticket,omitempty"`
	// URL is the phase-2 raw-ACP WebSocket gateway origin (BEX_AGENT_SESSION_GATEWAY_URL)
	// — not where a client GETs the phase-1 SSE conversation stream. StreamURL is
	// that: the exact `<APIPublicURL>/v1/agent-sessions/<id>/stream` a caller
	// connects to for phase 1 (ADR047 D9), so clients stop deriving it themselves
	// (w3/013). Empty when BEX_API_PUBLIC_URL is unset — same graceful-degradation
	// shape as the deploy-hook URLs that field already drives.
	URL       string     `json:"url,omitempty"`
	StreamURL string     `json:"streamUrl,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// ArchivedAt marks the session archived (ADR065 D1): out of the working set,
	// mutation verbs refused, reads always allowed. Nil ⇒ in the working set.
	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
	// Created is set on create responses only: true for a new session, false
	// when an idempotent retry returned the session its key already named.
	Created *bool `json:"created,omitempty"`
	// Outcome is the compact result an LLM reads instead of the transcript
	// (ADR094 D7). Summary is filled only by the single-session read.
	Outcome *Outcome `json:"outcome,omitempty"`
}

// Outcome is the compact account of where a session stands (ADR094 D7): its
// state, whether it accepts a message, the latest turn's final answer, and
// what it delivered: the PR and the files it shared, and the validated
// structured result. cost arrives with a later milestone and stays absent until then.
type Outcome struct {
	State     string `json:"state"`
	Steerable bool   `json:"steerable"`
	Turn      int    `json:"turn"`
	// Summary is the final assistant text of the latest completed turn, clipped
	// to maxOutcomeSummaryChars head-and-tail. Empty until a turn completes.
	Summary           string       `json:"summary,omitempty"`
	PullRequest       *PullRequest `json:"pullRequest,omitempty"`
	DeliveryNote      string       `json:"deliveryNote,omitempty"`
	FailureReasonCode string       `json:"failureReasonCode,omitempty"`
	// Files are the files the agent shared (share_file) in the same turn as
	// Summary (ADR095 D11). Fetch one with get_agent_attachment / …/content.
	// Like Summary it is computed only on the single-session read and wait, so
	// a list page omits it rather than reporting "no files".
	Files []TurnAttachment `json:"files,omitempty"`
	// Result is the current turn's last valid final submission against its
	// outputSchema (ADR094 D5), or absent: the turn had no schema, has not
	// submitted a final result yet, or ended without one. Like Summary it is
	// computed only on the single-session read and wait. A non-final (partial)
	// result is on the transcript turn, not here.
	Result any `json:"result,omitempty"`
	// Cost is what the session's platform-route model requests cost so far
	// (ADR094 D7); absent when it made none (a workspace's own key bills at its
	// provider). Computed only on the single-session read, wait and webhook.
	Cost *Cost `json:"cost,omitempty"`
}

// Cost is the ADR091 meter's reading for one session.
type Cost struct {
	// ModelUSD is the session's total agent_model_cost_microusd as a decimal
	// string to the micro-dollar ("0.42", "0.001234"), never a float.
	ModelUSD string `json:"modelUSD"`
}

// PullRequest is the PR the agent opened for the session (ADR090).
type PullRequest struct {
	URL     string `json:"url"`
	Number  int    `json:"number,omitempty"`
	HeadSHA string `json:"headSha,omitempty"`
}

// Archived-filter wire vocabulary for the session list (ADR065 D3). Empty and
// ArchivedFalse both mean the default working set (archived excluded).
const (
	ArchivedFalse = "false"
	ArchivedTrue  = "true"
	ArchivedAll   = "all"
)

// List page bounds (ADR065 D3). Agent sessions are a bex extension (no Render
// twin), and the dashboard's working-set poll wants a fuller default page than
// Render's public 20/100 contract — so the bounds are local, not core's.
const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// ListRequest is the cross-surface session-list contract (ADR065 D3): the
// archived membership selector (default: unarchived — the working set), the
// phase/repo/created filters, and Render-idiom keyset pagination (Cursor is the
// prior page's last item id; a shorter/empty page signals the end). It is
// never wire-decoded — each adapter builds it field-by-field.
type ListRequest struct {
	OwnerID  string
	Archived string
	Phases   []string
	// States filters by the coarse derived state (ADR094 D7); it expands to the
	// phases it covers and intersects with Phases when both are given. Tag keeps
	// sessions carrying that tag (ADR094 D3).
	States        []string
	Tag           string
	Repo          string
	CreatedBefore time.Time
	CreatedAfter  time.Time
	Cursor        string
	Limit         int
}

// TranscriptPart is one durable conversation part in the poll-shaped transcript
// read (ADR065 D2). Part is the stored payload, surfaced raw — valid JSON from
// both writers (the gateway tee and the Completer harvest). Under the ADR051
// item model it is a completed item: a UIMessage `text` / `reasoning` /
// `dynamic-tool` part (a clipped tool output carries `truncated: true` and the
// `[bex: N chars clipped]` marker, plus `fullOutput: true` when its complete
// text is readable through ToolOutput), a data snapshot, or a standalone chunk
// (`start`, `finish`, …). The stream replay expands each item back into the
// standard chunks; rows from before the item model are still single chunks.
// Partial marks an item the live stream ended inside of: the harvest may
// later complete it, re-inserting the complete copy under a NEW seq with the
// same (Turn, PartIndex) — a poller merges by that identity, and renders parts
// in (Turn, PartIndex) order.
type TranscriptPart struct {
	Seq       int64           `json:"seq"`
	PartIndex int64           `json:"partIndex"`
	Turn      int             `json:"turn"`
	Part      json.RawMessage `json:"part"`
	Partial   bool            `json:"partial,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// ToolOutput is one tool call's complete outputs (w10/m18), keyed by field:
// "output", "errorText", or "terminal:<terminalId>" ("terminal" without an
// id). Each is at most 1 MiB; a longer one ends in
// "[bex: full output capped at 1 MiB, N chars omitted]".
type ToolOutput struct {
	SessionID  string            `json:"sessionId"`
	Turn       int               `json:"turn"`
	ToolCallID string            `json:"toolCallId"`
	Outputs    map[string]string `json:"outputs"`
}

// TranscriptTurn is the role-correct user side of a durable conversation turn,
// plus the honest completeness result for its assistant transcript.
type TranscriptTurn struct {
	Turn                int        `json:"turn"`
	Prompt              string     `json:"prompt"`
	DeliveryMode        string     `json:"deliveryMode"`
	TranscriptComplete  bool       `json:"transcriptComplete"`
	TranscriptTruncated bool       `json:"transcriptTruncated"`
	TruncationReason    string     `json:"truncationReason,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	CompletedAt         *time.Time `json:"completedAt,omitempty"`
	// Attachments are the files this turn attached (ADR095 D3); never null.
	Attachments []TurnAttachment `json:"attachments"`
	// OutputSchema is the JSON Schema the turn was accepted with (ADR094 D5),
	// absent for a free-form turn. Result is the agent's last valid submission,
	// readable while the turn runs; ResultFinal says it declared that one final
	// (only a final result becomes outcome.result).
	OutputSchema   any  `json:"outputSchema,omitempty"`
	OutputRequired bool `json:"outputRequired,omitempty"`
	Result         any  `json:"result,omitempty"`
	ResultFinal    bool `json:"resultFinal,omitempty"`
}

// TurnAttachment is one file of a turn: an input it attached, or a file the
// agent shared in it (Outcome.Files).
type TurnAttachment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

// TranscriptPage is one seq-keyset page of a session's transcript. Clients page
// by echoing NextAfterSeq as afterSeq; an empty Parts signals the end.
type TranscriptPage struct {
	Parts        []TranscriptPart `json:"parts"`
	Turns        []TranscriptTurn `json:"turns"`
	NextAfterSeq int64            `json:"nextAfterSeq"`
}
