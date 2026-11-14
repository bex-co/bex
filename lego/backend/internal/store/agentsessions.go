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

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// MaxAgentSessionPromptBytes caps all durable user intent for one session.
// Individual prompts are already limited to 100 KiB at the API and schema;
// this aggregate bound prevents an unbounded number of valid turns from
// growing Postgres independently of the assistant-transcript quota.
const MaxAgentSessionPromptBytes = 8 << 20

var (
	ErrAgentSessionPromptQuota = fmt.Errorf("agent session prompt byte quota reached: %w", ErrConflict)
	ErrAgentSessionTurnState   = fmt.Errorf("agent session cannot accept another turn in its current state: %w", ErrConflict)
)

// AgentSessionTranscriptPart is one durable transcript row (ADR047 D9, w3/m43,
// amended by w5/m71 and the ADR051 item model, w10/m16). Seq is the
// store-allocated monotonic replay cursor; PartIndex is the turn-local ordinal
// (for an item, the ordinal of its first chunk). Part is a completed item (a
// UIMessage text/reasoning/tool part, a data snapshot, or a standalone chunk);
// rows written before w10/m16 are raw UI-message chunks (Item false).
// agenttranscript.Expand turns either into the chunks a Vercel AI SDK client
// replays.
type AgentSessionTranscriptPart struct {
	Seq       int64
	PartIndex int64
	Turn      int
	Part      json.RawMessage
	// Item marks a coalesced item row; Partial an item its stream ended inside
	// of, which a later complete copy may replace.
	Item      bool
	Partial   bool
	CreatedAt time.Time
	// FullOutputs are the complete texts of the outputs this item clipped
	// (agenttranscript.Item.FullOutputs); every writer stores them in
	// agent_session_tool_outputs in the same transaction (write-only: reads
	// never fill it).
	FullOutputs []agenttranscript.FullOutput
}

// Transcript row shapes (agent_session_transcripts.item_state).
const (
	transcriptRowLegacyChunk = 0
	transcriptRowItem        = 1
	transcriptRowPartialItem = 2
)

func transcriptRowState(p AgentSessionTranscriptPart) int16 {
	switch {
	case !p.Item:
		return transcriptRowLegacyChunk
	case p.Partial:
		return transcriptRowPartialItem
	default:
		return transcriptRowItem
	}
}

func (p *AgentSessionTranscriptPart) setRowState(state int16) {
	p.Item = state != transcriptRowLegacyChunk
	p.Partial = state == transcriptRowPartialItem
}

// AgentSessionTurn is the durable user intent for one accepted agent turn.
// Assistant output stays in AgentSessionTranscriptPart so the gateway can replay
// its exact wire bytes; this row supplies the role and completeness facts the
// byte stream itself cannot encode.
type AgentSessionTurn struct {
	SessionID           string
	Turn                int
	Prompt              string
	DeliveryMode        string
	TranscriptComplete  bool
	TranscriptTruncated bool
	TruncationReason    string
	CreatedAt           time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
	// Attachments are the uploads this turn attached (ADR095 D3), snapshotted
	// at binding.
	Attachments []TurnAttachment
	// OutputSchema is the JSON Schema this turn's result must satisfy (ADR094
	// D5); nil for a free-form turn. OutputRequired makes a turn that ends
	// without a final result fail. Result is the agent's last valid submission
	// and ResultFinal says it declared that one final.
	OutputSchema   json.RawMessage
	OutputRequired bool
	Result         json.RawMessage
	ResultFinal    bool
	// Skills are the skills this turn invoked explicitly (ADR099 D4).
	Skills []TurnSkill
}

// AgentTurnOutput is the structured-output contract a turn is accepted with
// (ADR094 D5): the already-validated schema, or nil for a free-form turn.
type AgentTurnOutput struct {
	Schema   json.RawMessage
	Required bool
}

// turnSkillsJSON is the turn's skills column value, never null.
func turnSkillsJSON(skills []TurnSkill) []byte {
	if len(skills) == 0 {
		return []byte("[]")
	}
	encoded, _ := json.Marshal(skills)
	return encoded
}

// nullable is the argument for a jsonb column: SQL NULL for an empty value.
func (o AgentTurnOutput) nullable() any {
	if len(o.Schema) == 0 {
		return nil
	}
	return []byte(o.Schema)
}

// TerminalTurnFact is the immutable timing for a successful terminal CAS on the
// session's current turn (w5/m88). Callers observe metrics only when Finalize*
// succeeds; ErrNotFound means another writer already owned the transition.
type TerminalTurnFact struct {
	Turn       int
	AcceptedAt time.Time
	StartedAt  *time.Time // nil when the turn never reached running (no bind)
	TerminalAt time.Time
}

// AgentSession is the durable control-plane record for one cloud coding-agent
// task (ADR047 D3). AgentConfig is kept as JSON because the driver-specific
// knobs evolve independently of the lifecycle contract; callers validate its
// public shape before persistence.
type AgentSession struct {
	ID           string
	WorkspaceID  string
	Repo         string
	Branch       string
	AgentConfig  json.RawMessage
	SandboxID    string
	Phase        string
	Status       string
	HeadSHA      string
	PRURL        string
	PRNumber     int
	Turns        int
	DeliveryMode string
	// DeliveryNote is the readable reason a completed turn left work the agent
	// did not deliver (ADR090: a dirty tree, unpushed commits, or a pushed branch
	// with no PR). Empty when there is nothing to report.
	DeliveryNote  string
	FailureReason string
	// FailureReasonCode is a stable code for the failure's cause when a client
	// acts on it, such as SANDBOX_CAPACITY_LIMIT; empty otherwise (w5/m132). Only
	// AbandonAgentDispatch writes one; every other failure write clears it.
	FailureReasonCode string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CanceledAt        *time.Time
	// ArchivedAt is the ADR065 D1 archive flag: LIST-state, orthogonal to phase.
	// NULL ⇒ the session is in the working set; set ⇒ it is archived (hidden from
	// the default list, mutation verbs refused) but still viewable.
	ArchivedAt *time.Time
	// Title and Tags are the display title and filter tags (ADR094 D3). The
	// title is set at create; tags can be replaced afterwards (w7/m296).
	Title string
	Tags  []string
	// MaxSpendMicroUSD is the per-session ceiling on the platform-route model
	// price (ADR094 D8), in micro-USD; nil = none. Create-only, read-only after.
	MaxSpendMicroUSD *int64
	// SecretNames are the names of the secrets attached to the session
	// (ADR098 D1): workspace agent secret references and inline session
	// secrets. Values live only in OpenBao; this never holds one.
	SecretNames []string
	// PRState is the pull request's state as bex last read it from GitHub
	// (PRStateOpen, PRStateMerged, PRStateClosed), "" until first read;
	// PRStateAt is when it last changed (w9/m386).
	PRState   string
	PRStateAt *time.Time
	// AppliedSecrets are the secrets the latest dispatch applied, each with
	// its source (w9/m385): named, inline, or by a workspace or repo scope.
	// Names only; written by SetAgentSessionAppliedSecrets.
	AppliedSecrets []AppliedSecret
	// InitialPrompt is create-only input used to atomically insert turn 1 with
	// the session row. It is not an agent_sessions column and is never scanned.
	InitialPrompt string
	// AutomationID is the agent automation (ADR101) that started the session;
	// empty for a session a person or client started. Set only by the scheduler.
	AutomationID string
	// ParentSessionID, RootSessionID and Depth are the ADR102 lineage: the
	// session that spawned this one, the top of its tree and the distance from
	// it. All zero for a top-level session. On create only ParentSessionID is
	// input; the store derives the other two under a lock on the parent.
	ParentSessionID string
	RootSessionID   string
	Depth           int
	// CreatedBy is the subject that created the session (ADR102 D9): the
	// authority its in-sandbox spawn acts with. Empty for older rows.
	CreatedBy string
	// TakeoverPR is the pull request a `/bex` comment handed the session
	// (ADR047 D11), 0 for every other session, and TakeoverHeadRef that pull
	// request's head branch. The session still works on its own bex-agent/
	// Branch, started from that head; its PR verb fast-forwards
	// TakeoverHeadRef to it. Set at create (with PRNumber, PRURL and HeadSHA)
	// by the internal PR intake only, and never changed.
	TakeoverPR      int
	TakeoverHeadRef string
	// InitialOrigin is create-only input: the comment that started a takeover
	// session, stored in the same transaction as PR feedback delivered by turn
	// 1, so the session's turn origins show it and a redelivery of the comment
	// queues nothing. Never scanned.
	InitialOrigin *AgentSessionFeedback
	// TakeoverScope is create-only input: the workspaces whose sessions count
	// as already owning TakeoverPR. The create is refused with
	// ErrAgentSessionPullRequestOwned when one of them has a live session on
	// it, checked under a per-pull-request advisory lock so two concurrent
	// `/bex` comments start one session. Never scanned.
	TakeoverScope []string
	// IdempotencyKey and IdempotencyHash are create-only input (ADR094 D4): the
	// client's key and the digest of its normalized request. Not scanned by
	// scanAgentSession; FindAgentSessionByIdempotencyKey reads the hash back.
	IdempotencyKey  string
	IdempotencyHash string
	// InitialAttachments is create-only input bound to turn 1 in the same
	// transaction (ADR095 D3); never scanned.
	InitialAttachments AgentAttachmentBinding
	// InitialOutput is create-only input stored on turn 1 (ADR094 D5); never scanned.
	InitialOutput AgentTurnOutput
	// InitialSkills is create-only input stored on turn 1 (ADR099 D4); never scanned.
	InitialSkills []TurnSkill
	// AdmissionLimit is create-only input: when positive, the insert is
	// refused with ErrAgentSessionAdmissionLimit if the workspace already has
	// that many active sessions matching AdmissionCount. The count and the
	// insert share a transaction under a per-workspace advisory lock, so
	// concurrent creates cannot overshoot (ADR091 D10.1 trust tiers).
	AdmissionLimit int
	AdmissionCount AgentSessionCount
}

// IsTakeover reports whether a `/bex` comment handed the session a pull
// request to take over.
func (s AgentSession) IsTakeover() bool { return s.TakeoverPR > 0 }

// ErrAgentSessionAdmissionLimit is a create refused at its AdmissionLimit.
var ErrAgentSessionAdmissionLimit = errors.New("agent session admission limit reached")

// ErrAgentSessionPullRequestOwned is a takeover create refused because a live
// session in its scope already works on the pull request.
var ErrAgentSessionPullRequestOwned = errors.New("a session already works on this pull request")

// ActiveAgentSessionPhases are the non-terminal phases a workspace's session
// counts are taken over (agentsessions.activePhases; a test there keeps the
// two equal). activeAgentSessionPhasesSQL is the same set as the predicate of
// the partial agent_sessions_active_workspace_idx (migration 0194), so the
// order matters.
var ActiveAgentSessionPhases = []string{"creating", "running", "resuming", "redispatching"}

// TerminalAgentSessionPhases are the phases a session ends in; canceling is
// still working.
var TerminalAgentSessionPhases = []string{"completed", "failed", "canceled"}

var activeAgentSessionPhasesSQL = "phase IN ('" + strings.Join(ActiveAgentSessionPhases, "', '") + "')"

// AgentSessionCount narrows CountActiveAgentSessions.
type AgentSessionCount struct {
	// WithSandbox counts only sessions holding a sandbox id: the ADR059 D6
	// live-sandbox working set. Otherwise a session counts from the moment it
	// is accepted.
	WithSandbox bool
	// Agent, when set, counts only sessions of that (normalized) agent profile.
	Agent string
	// ExcludeID leaves one session out: the one a steer or resume reactivates
	// must not count against itself.
	ExcludeID string
	// AllWorkspaces counts across every workspace (the workspace argument is
	// ignored): a limit shared by one platform key, such as Meta's per-key
	// rate limit for the muse profile (ADR108).
	AllWorkspaces bool
}

// DefaultAgentBranchPrefix prefixes the branch a session gets when the caller
// names none; it is the bex-agent/ prefix agentsession.ValidateBranch requires.
const DefaultAgentBranchPrefix = "bex-agent/"

// ErrAgentSessionBranchInUse is a create refused because another session in
// the workspace holds the branch it named on the same repository (w5/m382;
// see lockUnheldBranch for what holds a branch).
var ErrAgentSessionBranchInUse = errors.New("agent session branch is held by another live session")

// ErrAgentSessionIdempotencyKey is a create that lost the race for its key: a
// session already holds it in this workspace. Callers look the holder up.
var ErrAgentSessionIdempotencyKey = errors.New("agent session idempotency key already in use")

// AgentSessionIdempotencyWindow is how long a key stays bound to its session.
const AgentSessionIdempotencyWindow = 24 * time.Hour

const agentSessionColumns = `id, workspace_id, repo, branch, agent_config, sandbox_id,
	phase, status, head_sha, pr_url, pr_number, turns, delivery_mode, delivery_note,
	failure_reason, failure_reason_code, created_at, updated_at, canceled_at,
	archived_at, title, tags, max_spend_microusd, secret_names, coalesce(automation_id, ''),
	coalesce(parent_session_id, ''), coalesce(root_session_id, ''), depth, created_by, takeover_pr_number, takeover_head_ref,
	applied_secrets, pr_state, pr_state_at`

func scanAgentSession(row pgx.Row) (AgentSession, error) {
	var s AgentSession
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.Repo, &s.Branch, &s.AgentConfig,
		&s.SandboxID, &s.Phase, &s.Status, &s.HeadSHA, &s.PRURL, &s.PRNumber,
		&s.Turns, &s.DeliveryMode, &s.DeliveryNote, &s.FailureReason, &s.FailureReasonCode,
		&s.CreatedAt, &s.UpdatedAt, &s.CanceledAt, &s.ArchivedAt, &s.Title, &s.Tags, &s.MaxSpendMicroUSD, &s.SecretNames, &s.AutomationID,
		&s.ParentSessionID, &s.RootSessionID, &s.Depth, &s.CreatedBy, &s.TakeoverPR, &s.TakeoverHeadRef,
		&s.AppliedSecrets, &s.PRState, &s.PRStateAt)
	return s, err
}

// CreateAgentSession mints the only valid agent-session id kind and persists
// the row in creating phase before any external OpenFGA/OpenSandbox mutation.
func (s *PGStore) CreateAgentSession(ctx context.Context, in AgentSession) (AgentSession, error) {
	in.ID = ids.New(ids.AgentSession)
	// A repository session with no branch gets the session's own (ADR094 D3):
	// an LLM caller should not have to invent a name that passes the prefix rule.
	namedBranch := in.Branch != ""
	if in.Repo != "" && in.Branch == "" {
		in.Branch = DefaultAgentBranchPrefix + in.ID
	}
	// A takeover records its pull request from the start, so its feedback and
	// CI find the session.
	if in.IsTakeover() && (in.Repo == "" || in.TakeoverHeadRef == "" || in.PRNumber != in.TakeoverPR) {
		return AgentSession{}, fmt.Errorf("%w: a takeover session needs its repository and pull request", ErrInvalid)
	}
	if len(in.AgentConfig) == 0 {
		in.AgentConfig = json.RawMessage(`{}`)
	}
	var row AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		if in.AdmissionLimit > 0 {
			if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
				"agent-session-admission:"+in.WorkspaceID); err != nil {
				return err
			}
			n, err := countActiveAgentSessions(ctx, tx, in.WorkspaceID, in.AdmissionCount)
			if err != nil {
				return err
			}
			if n >= in.AdmissionLimit {
				return ErrAgentSessionAdmissionLimit
			}
		}
		if in.ParentSessionID != "" {
			lineage, err := lockParentLineage(ctx, tx, in.WorkspaceID, in.ParentSessionID)
			if err != nil {
				return err
			}
			in.RootSessionID, in.Depth = lineage.root, lineage.depth
		}
		if in.IsTakeover() {
			scope := in.TakeoverScope
			if len(scope) == 0 {
				scope = []string{in.WorkspaceID}
			}
			if err := lockUnownedPullRequest(ctx, tx, in.Repo, in.TakeoverPR, scope, ""); err != nil {
				return err
			}
		}
		if in.IdempotencyKey != "" {
			// Release a key past its window so it can bind to this new session.
			if _, err = tx.Exec(ctx, `
				UPDATE agent_sessions SET idempotency_key=NULL, idempotency_hash=NULL
				WHERE workspace_id=$1 AND idempotency_key=$2 AND created_at <= now() - $3::interval`,
				in.WorkspaceID, in.IdempotencyKey, AgentSessionIdempotencyWindow.String()); err != nil {
				return err
			}
		}
		row, err = scanAgentSession(tx.QueryRow(ctx, `
			INSERT INTO agent_sessions (id, workspace_id, repo, branch, agent_config, phase, status, turns, idempotency_key, idempotency_hash, title, tags, max_spend_microusd, secret_names, automation_id, parent_session_id, root_session_id, depth, created_by, takeover_pr_number, takeover_head_ref, pr_number, pr_url, head_sha, pr_next_check_at)
			VALUES ($1, $2, $3, $4, $5, 'creating', '', CASE WHEN $6 <> '' THEN 1 ELSE 0 END, NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, NULLIF($13, ''), NULLIF($14, ''), NULLIF($15, ''), $16, $17, $18, $19, $20, $21, $22,
			        CASE WHEN $20 > 0 THEN now() END)
			RETURNING `+agentSessionColumns,
			in.ID, in.WorkspaceID, in.Repo, in.Branch, in.AgentConfig, in.InitialPrompt, in.IdempotencyKey, in.IdempotencyHash, in.Title, nonNil(in.Tags), in.MaxSpendMicroUSD, nonNil(in.SecretNames), in.AutomationID, in.ParentSessionID, in.RootSessionID, in.Depth, in.CreatedBy, in.TakeoverPR, in.TakeoverHeadRef, in.PRNumber, in.PRURL, in.HeadSHA))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "agent_sessions_idempotency_key_idx" {
				return ErrAgentSessionIdempotencyKey
			}
			return err
		}
		// After the insert, so a retry with the same idempotency key fails on
		// its key (and replays) before it could be refused for the branch.
		if namedBranch && in.Repo != "" {
			if err := lockUnheldBranch(ctx, tx, in.WorkspaceID, in.Repo, in.Branch, in.ID, in.ParentSessionID != ""); err != nil {
				return err
			}
		}
		if in.InitialPrompt != "" {
			_, err = tx.Exec(ctx, `
				INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode, output_schema, output_required, skills)
				VALUES ($1, 1, $2, '', $3, $4, $5)`, in.ID, in.InitialPrompt, in.InitialOutput.nullable(), in.InitialOutput.Required, turnSkillsJSON(in.InitialSkills))
			if err == nil {
				err = bindAgentAttachments(ctx, tx, in.WorkspaceID, in.ID, 1, in.InitialAttachments)
			}
		}
		if err == nil && in.InitialOrigin != nil {
			origin := *in.InitialOrigin
			origin.SessionID = in.ID
			_, err = insertAgentSessionFeedback(ctx, tx, origin, 1)
		}
		if err != nil {
			return err
		}
		return insertAgentDispatch(ctx, tx, row, "")
	})
	if errors.Is(err, ErrAgentSessionIdempotencyKey) || errors.Is(err, ErrAgentSessionAdmissionLimit) ||
		errors.Is(err, ErrAgentSessionPullRequestOwned) || errors.Is(err, ErrAgentSessionBranchInUse) || isAgentSessionLineageRefusal(err) {
		return AgentSession{}, err
	}
	if err != nil {
		if isAgentAttachmentRefusal(err) {
			return AgentSession{}, err
		}
		return AgentSession{}, classify("agent session", err)
	}
	return row, nil
}

// liveAgentSessionSQL is the predicate of a session that still works on its
// pull request: not archived, not canceled or canceling.
const liveAgentSessionSQL = `archived_at IS NULL AND phase NOT IN ('canceling', 'canceled')`

// lockUnownedPullRequest serializes everything that makes a takeover session
// live on one pull request (its create, its unarchive) and refuses while
// another live session in scope already works on it, so the pull request
// has one live session at most. except is the session being unarchived.
func lockUnownedPullRequest(ctx context.Context, tx pgx.Tx, repo string, prNumber int, scope []string, except string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"agent-session-takeover:"+strings.ToLower(repo)+"#"+strconv.Itoa(prNumber)); err != nil {
		return err
	}
	var owned bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM agent_sessions
		WHERE repo = lower($1) AND pr_number = $2 AND workspace_id = ANY($3) AND id <> $4
		  AND `+liveAgentSessionSQL+`)`,
		repo, prNumber, scope, except).Scan(&owned); err != nil {
		return err
	}
	if owned {
		return ErrAgentSessionPullRequestOwned
	}
	return nil
}

// FindAgentSessionByIdempotencyKey returns the session bound to key in the
// workspace within the idempotency window, with the request digest it was
// created from. ErrNotFound when none holds the key (or it has expired).
func (s *PGStore) FindAgentSessionByIdempotencyKey(ctx context.Context, workspaceID, key string) (AgentSession, string, error) {
	var hash string
	var out AgentSession
	row := s.Pool.QueryRow(ctx, `
		SELECT `+agentSessionColumns+`, idempotency_hash FROM agent_sessions
		WHERE workspace_id=$1 AND idempotency_key=$2 AND created_at > now() - $3::interval`,
		workspaceID, key, AgentSessionIdempotencyWindow.String())
	err := row.Scan(&out.ID, &out.WorkspaceID, &out.Repo, &out.Branch, &out.AgentConfig,
		&out.SandboxID, &out.Phase, &out.Status, &out.HeadSHA, &out.PRURL, &out.PRNumber,
		&out.Turns, &out.DeliveryMode, &out.DeliveryNote, &out.FailureReason, &out.FailureReasonCode,
		&out.CreatedAt, &out.UpdatedAt, &out.CanceledAt, &out.ArchivedAt, &out.Title, &out.Tags, &out.MaxSpendMicroUSD, &out.SecretNames, &out.AutomationID,
		&out.ParentSessionID, &out.RootSessionID, &out.Depth, &out.CreatedBy, &out.TakeoverPR, &out.TakeoverHeadRef,
		&out.AppliedSecrets, &out.PRState, &out.PRStateAt, &hash)
	if err != nil {
		return AgentSession{}, "", classify("agent session", err)
	}
	return out, hash, nil
}

func (s *PGStore) GetAgentSession(ctx context.Context, id string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx,
		`SELECT `+agentSessionColumns+` FROM agent_sessions WHERE id=$1`, id))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// BeginAgentSessionTurn atomically records an accepted follow-up prompt and
// moves the session into its asynchronous dispatch phase. The public turn count
// advances here, with prompt acceptance, so provisioning failure cannot erase
// or renumber what the user submitted. A non-nil secretNames replaces the
// session's attached secret names with the turn (ADR098 D1: steer computes the
// union), so a refused turn attaches none; nil leaves them unchanged. skills
// are the skills the turn invokes explicitly (ADR099 D4).
func (s *PGStore) BeginAgentSessionTurn(ctx context.Context, id, prompt, deliveryMode, phase, status string, attachments AgentAttachmentBinding, output AgentTurnOutput, secretNames []string, skills []TurnSkill) (AgentSession, error) {
	var out AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var turns int
		var currentPhase, previousSandboxID, workspaceID string
		var archivedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT turns, phase, archived_at, sandbox_id, workspace_id FROM agent_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&turns, &currentPhase, &archivedAt, &previousSandboxID, &workspaceID); err != nil {
			return err
		}
		// Recheck after taking the row lock. Two concurrent Steers can both pass
		// the service's optimistic Get; only the first may persist a prompt.
		if archivedAt != nil || (currentPhase != "completed" && currentPhase != "failed") {
			return ErrAgentSessionTurnState
		}
		if err := enforceAgentSessionPromptQuota(ctx, tx, id, prompt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode, output_schema, output_required, skills)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, turns+1, prompt, deliveryMode, output.nullable(), output.Required, turnSkillsJSON(skills)); err != nil {
			return err
		}
		if err := bindAgentAttachments(ctx, tx, workspaceID, id, turns+1, attachments); err != nil {
			return err
		}
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `
			UPDATE agent_sessions
			SET sandbox_id='', phase=$2, status=$3, turns=turns+1,
			    failure_reason='', failure_reason_code='', updated_at=now(),
			    secret_names=COALESCE($4::text[], secret_names)
			WHERE id=$1
			RETURNING `+agentSessionColumns, id, phase, status, secretNames))
		if err != nil {
			return err
		}
		if phase == "redispatching" {
			return insertAgentDispatch(ctx, tx, out, previousSandboxID)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAgentSessionPromptQuota) || errors.Is(err, ErrAgentSessionTurnState) || isAgentAttachmentRefusal(err) {
			return AgentSession{}, err
		}
		return AgentSession{}, classify("agent session turn", err)
	}
	return out, nil
}

func enforceAgentSessionPromptQuota(ctx context.Context, tx pgx.Tx, sessionID, prompt string) error {
	var stored int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(octet_length(prompt)), 0)
		FROM agent_session_turns WHERE session_id=$1`, sessionID).Scan(&stored); err != nil {
		return err
	}
	if stored+int64(len(prompt)) > MaxAgentSessionPromptBytes {
		return ErrAgentSessionPromptQuota
	}
	return nil
}

// AgentSessionTurns returns the durable user intents in chronological order.
// Prompt input is capped individually and by MaxAgentSessionPromptBytes across
// the session; callers still stream assistant parts page-wise.
func (s *PGStore) AgentSessionTurns(ctx context.Context, sessionID string) ([]AgentSessionTurn, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT session_id, turn, prompt, delivery_mode,
		       transcript_complete, transcript_truncated, truncation_reason,
		       created_at, started_at, completed_at, attachments,
		       output_schema, output_required, result, result_final, skills
		FROM agent_session_turns WHERE session_id=$1 ORDER BY turn`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSessionTurn, 0)
	for rows.Next() {
		var turn AgentSessionTurn
		if err := rows.Scan(&turn.SessionID, &turn.Turn, &turn.Prompt, &turn.DeliveryMode,
			&turn.TranscriptComplete, &turn.TranscriptTruncated, &turn.TruncationReason,
			&turn.CreatedAt, &turn.StartedAt, &turn.CompletedAt, &turn.Attachments,
			&turn.OutputSchema, &turn.OutputRequired, &turn.Result, &turn.ResultFinal, &turn.Skills); err != nil {
			return nil, err
		}
		out = append(out, turn)
	}
	return out, rows.Err()
}

// CompleteAgentSessionTurn records whether the persisted assistant transcript
// is complete. A false complete value is intentional: the row still preserves
// the accepted user prompt while making loss/truncation visible to consumers.
// The turn keeps its first completion time: the Completer harvests again when
// it reaps the idle sandbox, hours after the turn ended.
func (s *PGStore) CompleteAgentSessionTurn(ctx context.Context, sessionID string, turn int, complete, truncated bool, reason string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_turns
		SET transcript_complete=$3, transcript_truncated=$4,
		    truncation_reason=$5, completed_at=COALESCE(completed_at, now())
		WHERE session_id=$1 AND turn=$2`, sessionID, turn, complete, truncated, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent session turn: %w", ErrNotFound)
	}
	return nil
}

// RecordAgentSessionResult stores the structured result the agent submitted
// for a session's current turn (ADR094 D5). It writes only while the session
// is in a credential-eligible live phase and only to a turn that carries an
// output schema; the last valid submission wins, except that a non-final
// submission is accepted but never replaces one the agent already declared
// final. It reports whether the turn took the submission, so a caller can tell
// a turn that takes none (no schema, or no longer running) from an accepted one.
func (s *PGStore) RecordAgentSessionResult(ctx context.Context, sessionID string, turn int, result json.RawMessage, final bool) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_turns t
		SET result = CASE WHEN t.result_final AND NOT $4 THEN t.result ELSE $3::jsonb END,
		    result_final = t.result_final OR $4
		FROM agent_sessions s
		WHERE t.session_id=$1 AND t.turn=$2 AND s.id=t.session_id AND s.turns=t.turn
		  AND `+"s."+activeAgentSessionPhasesSQL+`
		  AND t.output_schema IS NOT NULL`, sessionID, turn, []byte(result), final)
	if err != nil {
		return false, classify("agent session result", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Archived-membership selector for ListAgentSessions (ADR065 D3). The zero
// value excludes archived rows — the default working set.
type ArchivedFilter int

const (
	ArchivedExclude ArchivedFilter = iota // archived_at IS NULL (default)
	ArchivedOnly                          // archived_at IS NOT NULL
	ArchivedInclude                       // no predicate — the full history
)

// AgentSessionListQuery is the ADR065 D3 filter + keyset-page contract for the
// workspace session list. Cursor is the id of the prior page's last item
// (Render's cursor idiom); an unknown or foreign cursor yields an empty page.
// Limit must be normalized (>0) by the caller — the store never substitutes a
// default, so an unbounded read is impossible by construction.
type AgentSessionListQuery struct {
	Archived ArchivedFilter
	Phases   []string
	// Tag keeps only sessions carrying this tag (ADR094 D3).
	Tag  string
	Repo string
	// AutomationID keeps only the sessions an agent automation started (ADR101 D5).
	AutomationID string
	// ParentSessionID keeps only the direct children of this session (ADR102).
	ParentSessionID string
	// CreatedBy keeps only the sessions these subjects started (w7/m296).
	CreatedBy []string
	// PRStates keeps only sessions whose pull request is in one of these
	// states (open, merged, closed).
	PRStates      []string
	CreatedBefore time.Time
	CreatedAfter  time.Time
	Cursor        string
	Limit         int
}

// ListAgentSessions returns one filtered page of a workspace's sessions,
// newest first (created_at DESC, id DESC — the order the keyset cursor pages).
// The default query (zero filters) is the unarchived working set.
func (s *PGStore) ListAgentSessions(ctx context.Context, workspaceID string, q AgentSessionListQuery) ([]AgentSession, error) {
	if q.Limit <= 0 {
		return nil, fmt.Errorf("agent session list: limit must be positive")
	}
	// A filter text cannot hold matches nothing, like any unknown value
	// (w4/518): binding it would raise SQLSTATE 22021, a server fault.
	if !textBindable(q.Tag) || !textBindable(q.Repo) || !textBindable(q.Cursor) || !textBindable(q.AutomationID) || !textBindable(q.ParentSessionID) {
		return []AgentSession{}, nil
	}
	createdBy := slices.DeleteFunc(slices.Clone(q.CreatedBy), func(v string) bool { return !textBindable(v) })
	if len(q.CreatedBy) > 0 && len(createdBy) == 0 {
		return []AgentSession{}, nil
	}
	sql := `SELECT ` + agentSessionColumns + ` FROM agent_sessions WHERE workspace_id=$1`
	args := []any{workspaceID}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	switch q.Archived {
	case ArchivedOnly:
		sql += ` AND archived_at IS NOT NULL`
	case ArchivedInclude:
	default:
		sql += ` AND archived_at IS NULL`
	}
	if len(q.Phases) > 0 {
		sql += ` AND phase = ANY(` + arg(q.Phases) + `)`
	}
	if q.Tag != "" {
		sql += ` AND ` + arg(q.Tag) + ` = ANY(tags)`
	}
	if q.Repo != "" {
		sql += ` AND repo = ` + arg(q.Repo)
	}
	if q.AutomationID != "" {
		sql += ` AND automation_id = ` + arg(q.AutomationID)
	}
	if q.ParentSessionID != "" {
		sql += ` AND parent_session_id = ` + arg(q.ParentSessionID)
	}
	if len(q.PRStates) > 0 {
		sql += ` AND pr_number > 0 AND pr_state = ANY(` + arg(q.PRStates) + `)`
	}
	switch len(createdBy) {
	case 0:
	case 1:
		// Equality, not ANY: the creator index then yields rows already in
		// the keyset order, so the polled "mine" page stops at its LIMIT.
		sql += ` AND created_by = ` + arg(createdBy[0])
	default:
		sql += ` AND created_by = ANY(` + arg(createdBy) + `)`
	}
	if !q.CreatedBefore.IsZero() {
		sql += ` AND created_at < ` + arg(q.CreatedBefore)
	}
	if !q.CreatedAfter.IsZero() {
		sql += ` AND created_at > ` + arg(q.CreatedAfter)
	}
	if q.Cursor != "" {
		// Keyset: strictly after the cursor row in (created_at DESC, id DESC)
		// order. The subquery is workspace-scoped so a foreign/unknown cursor is
		// an empty page (Render's end-of-list behavior), never an ordering oracle.
		c := arg(q.Cursor)
		sql += ` AND (created_at, id) < (SELECT created_at, id FROM agent_sessions WHERE id = ` + c + ` AND workspace_id = $1)`
	}
	sql += ` ORDER BY created_at DESC, id DESC LIMIT ` + arg(q.Limit)
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSession, 0)
	for rows.Next() {
		v, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetAgentSessionArchived flips the ADR065 D1 archive flag. Idempotent both
// ways: re-archiving keeps the original archived_at (the first archive time is
// the honest one), and unarchiving a working-set row is a no-op. It never
// touches phase/status — archive is list-state, not lifecycle-state.
func (s *PGStore) SetAgentSessionArchived(ctx context.Context, id string, archived bool) (AgentSession, error) {
	var out AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if !archived {
			if err := lockUnarchivedTakeover(ctx, tx, id); err != nil {
				return err
			}
		}
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `
			UPDATE agent_sessions
			SET archived_at = CASE WHEN $2 THEN COALESCE(archived_at, now()) ELSE NULL END,
			    updated_at = now()
			WHERE id=$1
			RETURNING `+agentSessionColumns, id, archived))
		return err
	})
	if errors.Is(err, ErrAgentSessionPullRequestOwned) {
		return AgentSession{}, err
	}
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// lockUnheldBranch serializes the creates that name branch of repo in a
// workspace and refuses while another session there holds it (w5/m382), so
// two sessions never push over each other or re-sync each other's pull
// request. A session that is working holds its branch, archived or not
// (archiving does not stop a turn). For a child, which its parent's agent
// names, any session that could still push holds it too: a finished one can
// be messaged again. A person's or an automation's create may reuse a
// finished session's branch (an automation's next run does).
func lockUnheldBranch(ctx context.Context, tx pgx.Tx, workspaceID, repo, branch, self string, child bool) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"agent-session-branch:"+workspaceID+":"+repo+":"+branch); err != nil {
		return err
	}
	holds := activeAgentSessionPhasesSQL
	if child {
		holds = "(" + holds + " OR (" + liveAgentSessionSQL + "))"
	}
	var held bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_sessions
		WHERE workspace_id = $1 AND repo = $2 AND branch = $3 AND id <> $4 AND `+holds+`)`,
		workspaceID, repo, branch, self).Scan(&held); err != nil {
		return err
	}
	if held {
		return ErrAgentSessionBranchInUse
	}
	return nil
}

// lockUnarchivedTakeover refuses to unarchive a takeover session while
// another live session already works on its pull request: one in the
// session's workspace, or in a workspace sharing a GitHub App installation
// with it (the scope its `/bex` intake searched).
func lockUnarchivedTakeover(ctx context.Context, tx pgx.Tx, id string) error {
	var repo, workspaceID string
	var takeoverPR int
	var archivedAt *time.Time
	err := tx.QueryRow(ctx, `SELECT repo, workspace_id, takeover_pr_number, archived_at FROM agent_sessions WHERE id=$1`, id).
		Scan(&repo, &workspaceID, &takeoverPR, &archivedAt)
	if err != nil || takeoverPR == 0 || archivedAt == nil {
		return err
	}
	scope, err := collectWith(func(row pgx.Row) (string, error) {
		var ws string
		return ws, row.Scan(&ws)
	})(tx.Query(ctx, `
		SELECT $1::text UNION
		SELECT peer.workspace_id FROM git_connections own
		JOIN git_connections peer ON peer.installation_id = own.installation_id
		WHERE own.workspace_id = $1`, workspaceID))
	if err != nil {
		return err
	}
	return lockUnownedPullRequest(ctx, tx, repo, takeoverPR, scope, id)
}

// ReplaceAgentSessionTags replaces a session's whole tag set (w7/m296, Devin's
// PUT …/tags). An archived session is returned unchanged: the caller sees
// ArchivedAt set and refuses, the same rule as every other mutation verb
// (ADR065 D1). The check and the write are one statement, so an archive that
// lands concurrently can never be overwritten by a stale tag edit.
func (s *PGStore) ReplaceAgentSessionTags(ctx context.Context, id string, tags []string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET tags = CASE WHEN archived_at IS NULL THEN $2 ELSE tags END,
		    updated_at = CASE WHEN archived_at IS NULL THEN now() ELSE updated_at END
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, nonNil(tags)))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// SetAgentSessionLifecycle advances the durable session state. canceled=true
// stamps canceled_at exactly once; every transition advances updated_at.
func (s *PGStore) SetAgentSessionLifecycle(ctx context.Context, id, sandboxID, phase, status string, canceled bool) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id = CASE WHEN $2 <> '' THEN $2 ELSE sandbox_id END,
		    phase=$3, status=$4, updated_at=now(),
		    canceled_at=CASE WHEN $5 THEN COALESCE(canceled_at, now()) ELSE canceled_at END
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, sandboxID, phase, status, canceled))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// SetAgentSessionFailure terminalizes a still-active session from a background
// dispatch/resume/steer failure with a named reason. Unlike
// SetAgentSessionLifecycle it records the reason in failure_reason (the field the
// dashboard's failed-session callout reads) and sets status to the terminal
// 'failed', so a provisioning failure surfaces identically to a driver-reported
// failure via FinalizeAgentSession (w5/m80 t005, closes w5/048). It preserves
// SetAgentSessionLifecycle's conditional sandbox_id write so an egress-transition
// failure still records the just-canceled sandbox for the reaper. The caller
// (setLifecycleIfActive) guards against clobbering a settled/canceling session.
func (s *PGStore) SetAgentSessionFailure(ctx context.Context, id, sandboxID, reason string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id = CASE WHEN $2 <> '' THEN $2 ELSE sandbox_id END,
		    phase='failed', status='failed', failure_reason=$3, failure_reason_code='', updated_at=now()
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, sandboxID, reason))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// ListAgentSessionsByPhases returns every session across all workspaces in any
// of the given phases, oldest first. It powers the trusted background Completer
// loop (ADR047 D4) that finalizes running sessions; it performs no authorization
// and must never be reachable from a tenant-facing verb.
func (s *PGStore) ListAgentSessionsByPhases(ctx context.Context, phases []string) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions WHERE phase = ANY($1) ORDER BY updated_at ASC, id ASC`, phases)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSession, 0)
	for rows.Next() {
		v, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListTerminalAgentSessionsForPush returns completed/failed sessions updated at
// or after `since`, across all workspaces, for the push worker's agent-terminal
// projection (w11/m6 t005). Bounded by the recency window so the scan stays
// cheap; dedup to one push per (session, phase) is the notification's
// source_event_key ON CONFLICT, not this query. Like ListAgentSessionsByPhases
// it performs no authorization and is a trusted background read only.
func (s *PGStore) ListTerminalAgentSessionsForPush(ctx context.Context, since time.Time) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions
		WHERE phase = ANY($1) AND updated_at >= $2
		ORDER BY updated_at ASC, id ASC`, []string{"completed", "failed"}, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSession, 0)
	for rows.Next() {
		v, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListTerminalAgentSessionsWithSandbox returns terminal sessions (completed/
// failed/canceled) that still carry a sandbox id and are due for a reap attempt
// (sandbox_reap_after IS NULL or <= now), oldest first (ADR054 D6 / w5/m99).
// Eligibility is per-row — there is no now-relative updated_at window, so a
// teardown that fails for days is still retried once its backoff elapses.
// Trusted background read — no authorization. `now` drives the backoff gate.
func (s *PGStore) ListTerminalAgentSessionsWithSandbox(ctx context.Context, now time.Time) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions
		WHERE phase = ANY($1) AND sandbox_id <> ''
		  AND (sandbox_reap_after IS NULL OR sandbox_reap_after <= $2)
		ORDER BY COALESCE(sandbox_reap_after, updated_at) ASC, id ASC
		LIMIT 200`,
		TerminalAgentSessionPhases, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSession, 0)
	for rows.Next() {
		v, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ScheduleAgentSessionSandboxReap pushes the next deferred-teardown attempt for
// a terminal session whose terminate just failed. Attempts grow the backoff
// (1m, 2m, 4m, … capped at 1h) so a flapping substrate does not hammer the
// API, but the row never ages out of the scan (w5/m99 t002).
func (s *PGStore) ScheduleAgentSessionSandboxReap(ctx context.Context, id string, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_sessions
		SET sandbox_reap_attempts = sandbox_reap_attempts + 1,
		    sandbox_reap_after = $2::timestamptz + make_interval(mins => LEAST(60, (1 << LEAST(sandbox_reap_attempts, 6))::int)),
		    updated_at = updated_at
		WHERE id = $1 AND sandbox_id <> ''`, id, now)
	return err
}

// ClearPreviousSandboxTeardown blanks previous_sandbox_id on a dispatch intent
// after the predecessor sandbox is confirmed gone (w5/m99 t004). When the
// intent is an abandoned cleanup-only tombstone with no other work, the row is
// deleted so the table does not grow without bound.
func (s *PGStore) ClearPreviousSandboxTeardown(ctx context.Context, sessionID string, turn int) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE agent_session_dispatches
			SET previous_sandbox_id = ''
			WHERE session_id = $1 AND turn = $2 AND previous_sandbox_id <> ''`, sessionID, turn)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM agent_session_dispatches
			WHERE session_id = $1 AND turn = $2 AND abandoned AND previous_sandbox_id = ''`, sessionID, turn)
		return err
	})
}

// ListPreviousSandboxTeardownsDue returns dispatch intents that still name a
// previous sandbox to tear down, due for a retry. Bound turns keep the intent
// as an abandoned tombstone until this cleanup succeeds (w5/m99 t004).
func (s *PGStore) ListPreviousSandboxTeardownsDue(ctx context.Context, now time.Time) ([]AgentDispatch, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT session_id, turn, workspace_id, legacy, previous_sandbox_id,
		       NOT EXISTS (SELECT 1 FROM agent_sessions WHERE id = session_id)
		FROM agent_session_dispatches
		WHERE previous_sandbox_id <> '' AND next_check_at <= $1
		ORDER BY next_check_at, session_id, turn
		LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentDispatch
	for rows.Next() {
		var d AgentDispatch
		if err := rows.Scan(&d.SessionID, &d.Turn, &d.WorkspaceID, &d.Legacy, &d.PreviousSandboxID, &d.SessionDeleted); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeferPreviousSandboxTeardown pushes next_check_at on a previous-sandbox
// teardown intent after a failed terminate, without requiring abandoned=true
// (active steers keep the intent live while the new turn provisions).
func (s *PGStore) DeferPreviousSandboxTeardown(ctx context.Context, sessionID string, turn int, next time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_dispatches SET next_check_at = $3
		WHERE session_id = $1 AND turn = $2 AND previous_sandbox_id <> ''`, sessionID, turn, next)
	return err
}

// CountActiveAgentSessions counts the workspace's sessions in an active phase
// matching c. A terminal session in its idle grace still carries a sandbox_id
// but is not active, so it never counts. Unlocked, it is the best-effort
// pre-check the live-sandbox cap and steer/resume use (the dispatch CAS and
// reconcile bound a small overshoot); a create that must not overshoot sets
// AgentSession.AdmissionLimit instead.
func (s *PGStore) CountActiveAgentSessions(ctx context.Context, workspaceID string, c AgentSessionCount) (int, error) {
	return countActiveAgentSessions(ctx, s.Pool, workspaceID, c)
}

func countActiveAgentSessions(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, workspaceID string, c AgentSessionCount) (int, error) {
	// Two statement texts, so a cached generic plan for the per-workspace
	// count keeps its workspace index.
	scope := `workspace_id = $1`
	if c.AllWorkspaces {
		scope = `$1::text IS NOT NULL`
	}
	var n int
	err := q.QueryRow(ctx, `
		SELECT count(*) FROM agent_sessions
		WHERE `+scope+` AND `+activeAgentSessionPhasesSQL+`
		  AND (NOT $2::boolean OR sandbox_id <> '')
		  AND ($3::text = '' OR agent_config->>'agent' = $3::text)
		  AND id <> $4::text`,
		workspaceID, c.WithSandbox, c.Agent, c.ExcludeID).Scan(&n)
	return n, err
}

// ClearAgentSessionSandbox blanks a session's sandbox_id after its sandbox has
// been torn down (ADR054 D6), dropping it from the deferred-teardown reaper's
// working set and releasing the live-sandbox unique index. It never changes
// phase/status — the session stays terminal — and is a no-op if already blank.
func (s *PGStore) ClearAgentSessionSandbox(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE agent_sessions SET sandbox_id = '', updated_at = now() WHERE id = $1 AND sandbox_id <> ''`, id)
	return err
}

// RecordAgentSessionDispatch binds a newly dispatched accepted turn: it advances the
// phase/status, adopts a fresh sandbox id when re-dispatched, records the
// delivery mode ("resume" or "redispatch"). Prompt acceptance already advanced
// the turn counter transactionally.
//
// Binding and abandonment serialize on the session and dispatch intent. Only
// the accepted turn in its provisioning phase can bind; cancellation, recovery,
// and newer turns fence out stale workers. Successful binding removes the intent
// in the same transaction so recovery cannot mistake its sandbox for an orphan.
func (s *PGStore) RecordAgentSessionDispatch(ctx context.Context, id, sandboxID, phase, status, deliveryMode string, turn int) (AgentSession, error) {
	var out AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Lock in the same order as abandonment: session, then intent.
		if err := tx.QueryRow(ctx, `SELECT id FROM agent_sessions WHERE id=$1 AND turns=$2
			AND phase IN ('creating','redispatching') FOR UPDATE`, id, turn).Scan(new(string)); err != nil {
			return err
		}
		var abandoned bool
		if err := tx.QueryRow(ctx, `SELECT abandoned FROM agent_session_dispatches WHERE session_id=$1 AND turn=$2 FOR UPDATE`, id, turn).Scan(&abandoned); err != nil {
			return err
		}
		if abandoned {
			return ErrNotFound
		}
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `UPDATE agent_sessions
			SET sandbox_id=$2, phase=$3, status=$4, delivery_mode=$5, failure_reason='', failure_reason_code='', updated_at=now()
			WHERE id=$1 RETURNING `+agentSessionColumns, id, sandboxID, phase, status, deliveryMode))
		if err != nil {
			return err
		}
		// Stamp the durable running-start once; pin/archive must not move it.
		if _, err := tx.Exec(ctx, `UPDATE agent_session_turns
			SET started_at = COALESCE(started_at, now())
			WHERE session_id=$1 AND turn=$2`, id, turn); err != nil {
			return err
		}
		// Keep a cleanup-only tombstone while previous_sandbox_id is still set so
		// a steer that blanked the session row never loses the only handle to the
		// predecessor sandbox (w5/m99 t004). Empty previous → delete as before.
		tag, err := tx.Exec(ctx, `
			UPDATE agent_session_dispatches
			SET abandoned = true, next_check_at = now()
			WHERE session_id = $1 AND turn = $2 AND previous_sandbox_id <> ''`, id, turn)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `DELETE FROM agent_session_dispatches WHERE session_id=$1 AND turn=$2`, id, turn)
		return err
	})
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// FinalizeAgentSession records a terminal turn outcome: the phase, the remote
// head, the turn's delivery note on success, or a named reason on failure. It
// never writes the pull request — that is RecordAgentSessionPullRequest's
// alone, mid-turn (ADR090) — so a later failed or no-op turn never erases a
// delivered PR. An empty head leaves the stored one untouched; the delivery
// note always reflects THIS turn. The phase guard is the cross-replica CAS: two
// Completers may observe the same running row, but only one can own its
// terminal transition.
//
// The returned TerminalTurnFact carries the current turn's durable timing for
// metrics (w5/m88). Callers must observe only when err == nil.
func (s *PGStore) FinalizeAgentSession(ctx context.Context, id, phase, headSHA, deliveryNote, failureReason string) (AgentSession, TerminalTurnFact, error) {
	var out AgentSession
	var fact TerminalTurnFact
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `
			UPDATE agent_sessions
			SET phase=$2, status=$3,
			    head_sha = CASE WHEN $4 <> '' THEN $4 ELSE head_sha END,
			    delivery_note=$5,
			    failure_reason=$6, failure_reason_code='', updated_at=now()
			WHERE id=$1 AND phase IN ('creating','running','resuming','redispatching')
			RETURNING `+agentSessionColumns,
			id, phase, phase, headSHA, deliveryNote, failureReason))
		if err != nil {
			return err
		}
		fact, err = stampTerminalTurn(ctx, tx, id, out.Turns, out.UpdatedAt)
		return err
	})
	if err != nil {
		return AgentSession{}, TerminalTurnFact{}, classify("agent session", err)
	}
	return out, fact, nil
}

// RecordAgentSessionPullRequest persists the draft PR the agent just opened or
// updated through the gateway PR verb (ADR090), mid-turn, so REST/GraphQL/MCP
// show it before the Completer finalizes. It writes only while the session is
// in a credential-eligible live phase (the verb's own gate) and never touches
// phase, status or updated_at: the turn's lifecycle stays the Completer's. A
// repeat sync of the same PR and head writes nothing, and neither does a
// session that already left its live phase (the PR exists on GitHub either way).
func (s *PGStore) RecordAgentSessionPullRequest(ctx context.Context, id, prURL string, prNumber int, headSHA string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_sessions
		SET pr_url=$2, pr_number=$3,
		    head_sha = CASE WHEN $4 <> '' THEN $4 ELSE head_sha END,
		    -- A new pull request starts unread and is read at once (w9/m386).
		    pr_state = CASE WHEN pr_number IS DISTINCT FROM $3 THEN '' ELSE pr_state END,
		    pr_state_at = CASE WHEN pr_number IS DISTINCT FROM $3 THEN NULL ELSE pr_state_at END,
		    pr_next_check_at = CASE WHEN pr_number IS DISTINCT FROM $3 AND $3 > 0 THEN now() ELSE pr_next_check_at END
		WHERE id=$1 AND phase IN ('creating','running','resuming','redispatching')
		  AND (pr_url IS DISTINCT FROM $2 OR pr_number IS DISTINCT FROM $3
		       OR ($4 <> '' AND head_sha IS DISTINCT FROM $4))`, id, prURL, prNumber, headSHA)
	if err != nil {
		return classify("agent session", err)
	}
	return nil
}

// SetAgentSessionCanceled is the Cancel verb's terminal CAS (w5/m88): first
// writer to leave a non-canceled phase wins, stamps the current turn's
// completed_at, and returns timing for metrics. A repeated Cancel after the
// session is already canceled returns ErrNotFound so observers do not double-count.
func (s *PGStore) SetAgentSessionCanceled(ctx context.Context, id string) (AgentSession, TerminalTurnFact, error) {
	var out AgentSession
	var fact TerminalTurnFact
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `
			UPDATE agent_sessions
			SET phase='canceled', status='canceled', updated_at=now(),
			    canceled_at=COALESCE(canceled_at, now())
			WHERE id=$1 AND phase <> 'canceled'
			RETURNING `+agentSessionColumns, id))
		if err != nil {
			return err
		}
		fact, err = stampTerminalTurn(ctx, tx, id, out.Turns, out.UpdatedAt)
		return err
	})
	if err != nil {
		return AgentSession{}, TerminalTurnFact{}, classify("agent session", err)
	}
	return out, fact, nil
}

// stampTerminalTurn sets completed_at on the current turn (once) and returns
// the immutable timing fact for metrics. Missing turn rows (turns=0) leave a
// fact with only Turn/TerminalAt — no fabricated started_at.
func stampTerminalTurn(ctx context.Context, tx pgx.Tx, sessionID string, turn int, fallbackTerminal time.Time) (TerminalTurnFact, error) {
	fact := TerminalTurnFact{Turn: turn, TerminalAt: fallbackTerminal}
	err := tx.QueryRow(ctx, `
		UPDATE agent_session_turns
		SET completed_at = COALESCE(completed_at, now())
		WHERE session_id=$1 AND turn=$2
		RETURNING created_at, started_at, completed_at`, sessionID, turn).
		Scan(&fact.AcceptedAt, &fact.StartedAt, &fact.TerminalAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fact, nil
		}
		return TerminalTurnFact{}, err
	}
	return fact, nil
}

// MaxAgentSessionTranscriptBytes caps the total payload bytes one session's
// transcript may hold (w1/m65 F10), bounding Postgres growth from
// tenant-controlled agent output and replay memory. Every write path (gateway
// live splice, gateway prompt turn, Completer harvest) seeds its byte counter
// from the already-stored total (AgentSessionTranscriptBytes) and stops
// appending at this cap; the replay read (AgentSessionTranscript) is budgeted
// by the same bound, so it never materializes more than the cap.
const MaxAgentSessionTranscriptBytes = 64 << 20

// AppendAgentSessionTranscript idempotently persists teed transcript parts.
// Driver ordinals restart at zero for every sandbox, so identity is the
// turn-local (session_id, turn, part_index). seq remains a monotonic per-session
// cursor allocated while holding a transaction-scoped advisory lock; concurrent
// gateway replicas therefore cannot collide or reorder the replay cursor.
func (s *PGStore) AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []AgentSessionTranscriptPart) error {
	if len(parts) == 0 {
		return nil
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Stable 64-bit hash lock, scoped to this transaction and session only.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, sessionID); err != nil {
			return err
		}
		var seq int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(max(seq), -1) FROM agent_session_transcripts WHERE session_id=$1`, sessionID).Scan(&seq); err != nil {
			return err
		}
		fullOutputs := toolOutputWriter{tx: tx, sessionID: sessionID}
		for _, p := range parts {
			// The full copies go first: an item flagged fullOutput never
			// commits without them (the cap aside).
			if err := fullOutputs.insert(ctx, p.Turn, p.FullOutputs); err != nil {
				return err
			}
			partIndex := p.PartIndex
			// Compatibility for internal callers/tests written before part_index:
			// their per-turn Seq was the driver ordinal. New writers set PartIndex.
			if partIndex == 0 && p.Seq > 0 {
				partIndex = p.Seq
			}
			part := string(p.Part)
			if part == "" {
				part = "null"
			}
			tag, err := tx.Exec(ctx, `
				INSERT INTO agent_session_transcripts (session_id, seq, turn, part_index, part, item_state)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (session_id, turn, part_index) DO NOTHING`,
				sessionID, seq+1, p.Turn, partIndex, part, transcriptRowState(p))
			if err != nil {
				return err
			}
			if tag.RowsAffected() > 0 {
				seq++
			}
		}
		return nil
	})
	return classify("agent session transcript", err)
}

// AgentSessionTranscript returns one seq-keyset page of a session's stored
// parts, strictly after afterSeq (pass -1 for the whole transcript), capped at
// maxBytes of cumulative payload: rows are scanned in seq order and the read
// stops before the part that would exceed the budget, so a read never
// materializes more than maxBytes even if the stored transcript ever outgrew
// it. limit additionally bounds the ROW count in SQL (ADR065 D2's poll-shaped
// page; <=0 ⇒ unbounded).
//
// seq is the WRITE (revision) order, the poll API's cursor: a harvest that
// completes a partial item re-inserts it under a fresh seq with the same
// (turn, part_index), so a poller past the partial still receives the upgrade
// and merges it by that identity. Readers that render the conversation use
// AgentSessionTranscriptReplay, which is in UI part order.
func (s *PGStore) AgentSessionTranscript(ctx context.Context, sessionID string, afterSeq int64, maxBytes int64, limit int) ([]AgentSessionTranscriptPart, error) {
	return s.readAgentSessionTranscript(ctx, `
		SELECT seq, part_index, turn, part, item_state, created_at
		FROM agent_session_transcripts
		WHERE session_id=$1 AND seq > $2
		ORDER BY seq ASC`, []any{sessionID, afterSeq}, maxBytes, limit)
}

// AgentSessionTurnTail returns up to limit of a turn's complete transcript items
// (not legacy chunks, not partial items), last first, under the byte budget. It
// backs the session outcome summary (ADR094 D7), which wants only the final
// assistant text, not the whole transcript.
func (s *PGStore) AgentSessionTurnTail(ctx context.Context, sessionID string, turn int, maxBytes int64, limit int) ([]AgentSessionTranscriptPart, error) {
	return s.readAgentSessionTranscript(ctx, `
		SELECT seq, part_index, turn, part, item_state, created_at
		FROM agent_session_transcripts
		WHERE session_id=$1 AND turn=$2 AND item_state=`+strconv.Itoa(transcriptRowItem)+`
		ORDER BY part_index DESC`, []any{sessionID, turn}, maxBytes, limit)
}

// TranscriptPosition is a part's place in the conversation, (turn,
// part_index): the UI part order and the replay keyset cursor.
type TranscriptPosition struct {
	Turn      int
	PartIndex int64
}

// TranscriptStart precedes every stored part.
var TranscriptStart = TranscriptPosition{Turn: -1, PartIndex: -1}

// Position is the part's replay cursor.
func (p AgentSessionTranscriptPart) Position() TranscriptPosition {
	return TranscriptPosition{Turn: p.Turn, PartIndex: p.PartIndex}
}

// AgentSessionTranscriptReplay returns a session's stored parts in UI part
// order — by (turn, part_index), strictly after `after` (TranscriptStart for
// the whole transcript) — under the same byte budget and row limit as
// AgentSessionTranscript. It is the replay source for reattach and for
// terminal-session history (ADR047 D9) and for continuity re-priming: it reads
// the durable store, never the sandbox, so it works after the sandbox is gone.
// Ordering by position rather than seq keeps an upgraded partial item (fresh
// seq) and a harvest-recovered hole in their place.
func (s *PGStore) AgentSessionTranscriptReplay(ctx context.Context, sessionID string, after TranscriptPosition, maxBytes int64, limit int) ([]AgentSessionTranscriptPart, error) {
	return s.readAgentSessionTranscript(ctx, `
		SELECT seq, part_index, turn, part, item_state, created_at
		FROM agent_session_transcripts
		WHERE session_id=$1 AND (turn, part_index) > ($2, $3)
		ORDER BY turn ASC, part_index ASC`, []any{sessionID, after.Turn, after.PartIndex}, maxBytes, limit)
}

func (s *PGStore) readAgentSessionTranscript(ctx context.Context, sql string, args []any, maxBytes int64, limit int) ([]AgentSessionTranscriptPart, error) {
	if limit > 0 {
		sql += fmt.Sprintf(` LIMIT $%d`, len(args)+1)
		args = append(args, limit)
	}
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSessionTranscriptPart, 0)
	var total int64
	for rows.Next() {
		var p AgentSessionTranscriptPart
		var state int16
		if err := rows.Scan(&p.Seq, &p.PartIndex, &p.Turn, &p.Part, &state, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.setRowState(state)
		if total+int64(len(p.Part)) > maxBytes {
			break
		}
		total += int64(len(p.Part))
		out = append(out, p)
	}
	return out, rows.Err()
}

// AgentSessionTranscriptBytes returns the total stored payload bytes of a
// session's transcript. Write paths seed their cumulative quota counter from
// it so the stored transcript can never exceed MaxAgentSessionTranscriptBytes
// regardless of how many turns or attaches append.
func (s *PGStore) AgentSessionTranscriptBytes(ctx context.Context, sessionID string) (int64, error) {
	var total int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(sum(octet_length(part)), 0) FROM agent_session_transcripts WHERE session_id=$1`,
		sessionID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// AgentSessionTranscriptMaxSeq returns the highest stored seq for a session and
// whether any part exists. The gateway uses it to skip driver-replayed parts it
// has already persisted, so its live tee resumes exactly where the store ends.
func (s *PGStore) AgentSessionTranscriptMaxSeq(ctx context.Context, sessionID string) (int64, bool, error) {
	var maxSeq *int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT max(seq) FROM agent_session_transcripts WHERE session_id=$1`, sessionID).Scan(&maxSeq); err != nil {
		return 0, false, err
	}
	if maxSeq == nil {
		return -1, false, nil
	}
	return *maxSeq, true, nil
}

// AgentSessionTranscriptTurnMaxIndex returns the largest stored local driver
// ordinal for one turn. A reconnect uses it to skip the prefix already replayed
// from Postgres while still accepting a fresh sandbox's ordinal zero.
func (s *PGStore) AgentSessionTranscriptTurnMaxIndex(ctx context.Context, sessionID string, turn int) (int64, bool, error) {
	var maxIndex *int64
	if err := s.Pool.QueryRow(ctx, `
		SELECT max(part_index) FROM agent_session_transcripts WHERE session_id=$1 AND turn=$2`,
		sessionID, turn).Scan(&maxIndex); err != nil {
		return 0, false, err
	}
	if maxIndex == nil {
		return -1, false, nil
	}
	return *maxIndex, true, nil
}

// Transcript retention is deliberate cascade-only (ADR065 D2): a session's
// conversation is PRODUCT data that lives exactly as long as its row — deleted
// only by the D4 delete verb or the workspace cascade, never by an age sweep.
// (An unwired age-based prune previously sat here; removed rather than wired.)

// DeleteAgentSession removes a session row; its transcript parts cascade with
// it. Two callers: the create-path compensation (a row whose OpenFGA parent
// tuple could not be established) and the ADR065 D4 public delete verb — the
// service layer owns the D4 guardrails (fresh authorization, no-live-sandbox
// phase gate). Cancel still preserves the
// audit/history row.
func (s *PGStore) DeleteAgentSession(ctx context.Context, id string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		// The session's files go with it: retired here, purged by the sweep.
		if err := retireSessionAttachments(ctx, tx, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM agent_sessions WHERE id=$1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("agent session: %w", ErrNotFound)
		}
		return nil
	})
}

// StampAgentSessionFailureCode sets the stable failureReasonCode on a session
// that FinalizeAgentSession has just failed with exactly reason, so a client
// can act on the cause (e.g. AGENT_MODEL_SPEND_CAP_REACHED). The reason
// match keeps a later, different failure write from inheriting the code.
func (s *PGStore) StampAgentSessionFailureCode(ctx context.Context, id, reason, code string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_sessions SET failure_reason_code=$3
		WHERE id=$1 AND phase='failed' AND failure_reason=$2`, id, reason, code)
	return err
}

// AgentSessionStateChannel is the Postgres NOTIFY channel announcing an agent
// session's phase change (payload: the session id).
const AgentSessionStateChannel = "agent_session_state"

// ListenAgentSessionStates delivers the id of every agent session whose phase
// changes to fn until ctx ends, on one dedicated connection (so a pod holds a
// single LISTEN no matter how many waiters it serves). A dropped connection
// reconnects after a short pause; notifications missed meanwhile are recovered
// by callers re-reading on a timer.
func (s *PGStore) ListenAgentSessionStates(ctx context.Context, fn func(id string)) error {
	for ctx.Err() == nil {
		if err := s.listenOnce(ctx, fn); err != nil && ctx.Err() == nil {
			log.Printf("agent-session state listener: %v (reconnecting)", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	return ctx.Err()
}

func (s *PGStore) listenOnce(ctx context.Context, fn func(id string)) error {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+AgentSessionStateChannel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			// A listening connection must not return to the pool with LISTEN set.
			_ = conn.Conn().Close(context.Background())
			return err
		}
		fn(n.Payload)
	}
}

// AppliedSecret is one secret a session's latest dispatch applied and why:
// inline, explicit (named), repo or workspace. Never a value.
type AppliedSecret struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// SetAgentSessionAppliedSecrets records what the session's latest dispatch
// applied (names and sources only).
func (s *PGStore) SetAgentSessionAppliedSecrets(ctx context.Context, sessionID string, applied []AppliedSecret) error {
	if applied == nil {
		applied = []AppliedSecret{}
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET applied_secrets = $2 WHERE id = $1`, sessionID, applied); err != nil {
		return fmt.Errorf("record agent session applied secrets: %w", err)
	}
	return nil
}
