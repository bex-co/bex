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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Approval mode (ADR100): a running turn can pause in needs_input on a
// decision, either an approval the engine asked for before a gated tool call
// (waitingFor approval, Devin's waiting_for_approval) or a question the agent
// asked with ask_user (waitingFor user, Devin's waiting_for_user). A workspace
// member answers on the public API; the driver long-polls its Pod-bound
// decision URL for that answer.

// Approval modes (ADR100 D3), in increasing strictness.
const (
	ApprovalAutonomous = "autonomous"
	ApprovalRisky      = "risky"
	ApprovalAll        = "all"
)

var approvalRank = map[string]int{ApprovalAutonomous: 0, ApprovalRisky: 1, ApprovalAll: 2}

// waitingFor values a needs_input session reports (ADR100 D1).
const (
	WaitingForApproval = "approval"
	WaitingForUser     = "user"
)

const (
	// decisionTTL is how long a decision waits for its answer before it
	// resolves itself (ADR100 D2): an approval as denied, a question as
	// unanswered, and the turn continues.
	decisionTTL = 24 * time.Hour
	// maxDecisionAnswerBytes bounds a question's answer text.
	maxDecisionAnswerBytes = 16 << 10
	decisionSweepBatch     = 200
	// defaultDecisionAwaitPoll re-reads a held await's decision when no
	// notification arrived; the session-keyed notification is the main signal.
	defaultDecisionAwaitPoll = 15 * time.Second
)

// DecisionStore is the ADR100 decision table. *store.PGStore satisfies it;
// nil turns approval mode off (sessions run autonomous, the answer verb is
// unavailable).
type DecisionStore interface {
	OpenAgentDecision(ctx context.Context, in store.AgentDecisionInput) (store.AgentDecision, bool, error)
	GetAgentDecision(ctx context.Context, id string) (store.AgentDecision, error)
	ResolveAgentDecision(ctx context.Context, sessionID, id, state string, answer json.RawMessage, answeredBy string) (store.AgentDecision, error)
	ExpireAgentDecisions(ctx context.Context, limit int) ([]store.AgentDecision, error)
}

func (s *Service) decisionsEnabled() bool { return s.Decisions != nil }

// sessionState is a row's coarse state (ADR094 D7) including needs_input: a
// running turn paused on a decision.
func sessionState(record store.AgentSession) string {
	if record.Phase == PhaseRunning && record.PendingDecisionID != "" {
		return StateNeedsInput
	}
	return stateOf(record.Phase)
}

// Decision is the one cross-surface view of a decision a turn paused on.
// Detail is what the agent asked ({toolKind, command, paths, rule} for an
// approval, {question, options} for a question); Answer is the final answer
// ({allow, scope} or {text}), absent while pending.
type Decision struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"sessionId"`
	Turn       int        `json:"turn"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Detail     any        `json:"detail"`
	State      string     `json:"state"`
	Answer     any        `json:"answer,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	AnsweredAt *time.Time `json:"answeredAt,omitempty"`
	ExpiresAt  time.Time  `json:"expiresAt"`
}

func decisionView(d store.AgentDecision) Decision {
	return Decision{
		ID: d.ID, SessionID: d.SessionID, Turn: d.Turn, Kind: d.Kind, Title: d.Title,
		Detail: jsonValue(d.Detail), State: d.State, Answer: jsonValue(d.Answer),
		CreatedAt: d.CreatedAt, AnsweredAt: d.AnsweredAt, ExpiresAt: d.ExpiresAt,
	}
}

// waitingForOf maps the session row's pending decision kind to waitingFor.
func waitingForOf(kind string) string {
	switch kind {
	case store.AgentDecisionApproval:
		return WaitingForApproval
	case store.AgentDecisionQuestion:
		return WaitingForUser
	}
	return ""
}

// pendingDecision reads the decision a running session is paused on for the
// single-session read and wait; nil when none, or on a read failure (the
// state still says needs_input).
func (s *Service) pendingDecision(ctx context.Context, record store.AgentSession) *Decision {
	if !s.decisionsEnabled() || record.PendingDecisionID == "" {
		return nil
	}
	d, err := s.Decisions.GetAgentDecision(ctx, record.PendingDecisionID)
	if err != nil {
		log.Printf("agent-session decision: read pending session=%s: %v", record.ID, err)
		return nil
	}
	view := decisionView(d)
	return &view
}

// OpenDecision records a decision the sandbox's running turn pauses on
// (ADR100 D5). The gateway has bound the request to a Pod carrying the
// session's labels and signed the envelope; the row is re-read and the Pod
// held to the current-sandbox rule like every other gateway verb.
func (s *Service) OpenDecision(ctx context.Context, envelope agentsession.DecisionEnvelope) (agentsession.DecisionResponse, error) {
	if !s.decisionsEnabled() {
		return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultUnavailable, "decisions are not configured")
	}
	record, err := s.Store.GetAgentSession(ctx, envelope.SessionID)
	if err != nil || !agentsession.CurrentSandboxCaller(record, envelope.Workspace, envelope.PodName, envelope.PodUID) {
		return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultForbidden, "this session is not running a sandbox")
	}
	content := envelope.Content
	d, created, err := s.Decisions.OpenAgentDecision(ctx, store.AgentDecisionInput{
		SessionID: record.ID, WorkspaceID: record.WorkspaceID, Turn: record.Turns,
		RequestID: content.RequestID, Kind: content.Kind, Title: content.Title, Detail: content.Detail,
		TTL: decisionTTL,
	})
	switch {
	case errors.Is(err, store.ErrAgentDecisionNotOpen), errors.Is(err, store.ErrNotFound):
		return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultForbidden, "this session is no longer running this turn")
	case err != nil:
		log.Printf("agent-session decision: open failed (session=%s): %v", record.ID, err)
		return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultUnavailable, "the decision could not be recorded")
	}
	if created {
		log.Printf("agent-session decision: opened %s (session=%s kind=%s)", d.ID, record.ID, d.Kind)
	}
	return decisionResponse(d), nil
}

func decisionResponse(d store.AgentDecision) agentsession.DecisionResponse {
	return agentsession.DecisionResponse{ID: d.ID, State: d.State, Answer: d.Answer}
}

func (s *Service) decisionAwaitPoll() time.Duration {
	if s.DecisionAwaitPoll > 0 {
		return s.DecisionAwaitPoll
	}
	return defaultDecisionAwaitPoll
}

func (s *Service) decisionHold() time.Duration {
	if s.DecisionAwaitHold > 0 {
		return s.DecisionAwaitHold
	}
	return agentsession.DecisionAwaitHold
}

// AwaitDecision holds the sandbox's await until the decision has its answer or
// the hold elapses (the response then says pending and the driver asks again).
// It wakes on this session's state notification an answer raises and re-reads
// on a slow timer that covers a missed one; a decision past its expiry is
// resolved here rather than waiting for the sweep, at most once per hold, so a
// failed or skipped sweep waits for the next wake instead of spinning.
func (s *Service) AwaitDecision(ctx context.Context, envelope agentsession.DecisionEnvelope) (agentsession.DecisionResponse, error) {
	if !s.decisionsEnabled() {
		return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultUnavailable, "decisions are not configured")
	}
	record, err := s.Store.GetAgentSession(ctx, envelope.SessionID)
	if err != nil || !agentsession.CurrentSandboxCaller(record, envelope.Workspace, envelope.PodName, envelope.PodUID) {
		return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultForbidden, "this session is not running a sandbox")
	}
	wake, unsubscribe := s.bus().subscribeSession(record.ID)
	defer unsubscribe()
	hold := time.NewTimer(s.decisionHold())
	defer hold.Stop()
	tick := time.NewTicker(s.decisionAwaitPoll())
	defer tick.Stop()
	swept := false
	for {
		d, err := s.Decisions.GetAgentDecision(ctx, envelope.DecisionID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && d.SessionID != record.ID) {
			return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.DecisionNotFound, "this session has no such decision")
		}
		if err != nil {
			return agentsession.DecisionResponse{}, agentsession.NewDecisionError(agentsession.ResultUnavailable, "the decision could not be read")
		}
		if d.State == store.AgentDecisionPending && !d.ExpiresAt.After(time.Now()) && !swept {
			swept = true
			s.expireDecisions(ctx)
			continue
		}
		if d.State != store.AgentDecisionPending {
			return decisionResponse(d), nil
		}
		select {
		case <-ctx.Done():
			return decisionResponse(d), nil
		case <-hold.C:
			return decisionResponse(d), nil
		case <-wake:
		case <-tick.C:
		}
	}
}

// AnswerDecisionRequest answers one decision: Allow (and Scope once|session)
// for an approval, Answer for a question.
type AnswerDecisionRequest struct {
	SessionID  string  `json:"-"`
	DecisionID string  `json:"-"`
	Allow      *bool   `json:"allow,omitempty" jsonschema:"for an approval: true runs the action, false refuses it"`
	Scope      string  `json:"scope,omitempty" jsonschema:"for an allowed approval: once (default) or session, which also allows the same kind of action for the rest of the session"`
	Answer     *string `json:"answer,omitempty" jsonschema:"for a question: the reply the agent reads"`
}

// AnswerDecision gives a pending decision its final answer (ADR100 D2). It
// needs can_create on the session, the relation steer needs: an answer
// directs the agent like a message does. The answer is idempotent by decision
// id: a second answer is refused 409 AGENT_SESSION_DECISION_ANSWERED and a
// late one 409 AGENT_SESSION_DECISION_EXPIRED.
func (s *Service) AnswerDecision(ctx context.Context, req AnswerDecisionRequest) (Decision, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, sessionObject(req.SessionID)); err != nil {
		return Decision{}, err
	}
	if !s.enabled() || !s.decisionsEnabled() {
		return Decision{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(req.SessionID); err != nil {
		return Decision{}, err
	}
	if kind, ok := ids.KindOf(req.DecisionID); !ok || kind != ids.AgentDecision {
		return Decision{}, core.NewBadRequestError("AGENT_SESSION_DECISION_INVALID", "invalid decision id", map[string]any{"decisionId": req.DecisionID})
	}
	current, err := s.Decisions.GetAgentDecision(ctx, req.DecisionID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && current.SessionID != req.SessionID) {
		return Decision{}, errDecisionNotFound(req.DecisionID)
	}
	if err != nil {
		return Decision{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	state, answer, err := decisionAnswer(current.Kind, req)
	if err != nil {
		return Decision{}, err
	}
	resolved, err := s.Decisions.ResolveAgentDecision(ctx, req.SessionID, req.DecisionID, state, answer, core.SubjectFrom(ctx))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return Decision{}, errDecisionNotFound(req.DecisionID)
	case errors.Is(err, store.ErrAgentDecisionAnswered):
		return Decision{}, core.NewConflictError("AGENT_SESSION_DECISION_ANSWERED", "this decision already has an answer", map[string]any{"decisionId": req.DecisionID})
	case errors.Is(err, store.ErrAgentDecisionExpired):
		return Decision{}, core.NewConflictError("AGENT_SESSION_DECISION_EXPIRED", "this decision expired before it was answered", map[string]any{"decisionId": req.DecisionID})
	case err != nil:
		return Decision{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	return decisionView(resolved), nil
}

func errDecisionNotFound(id string) error {
	return core.NewNotFoundError("AGENT_SESSION_DECISION_NOT_FOUND", "agent session decision not found", map[string]any{"decisionId": id})
}

// decisionAnswer validates an answer against the decision's kind and renders
// the final state and stored answer.
func decisionAnswer(kind string, req AnswerDecisionRequest) (string, json.RawMessage, error) {
	invalid := func(msg, field string) error {
		return core.NewBadRequestError("AGENT_SESSION_DECISION_INVALID", msg, map[string]any{"field": field})
	}
	if kind == store.AgentDecisionApproval {
		if req.Allow == nil || req.Answer != nil {
			return "", nil, invalid("an approval is answered with allow (true or false)", "allow")
		}
		if !*req.Allow {
			if req.Scope != "" {
				return "", nil, invalid("scope applies only to an allowed approval", "scope")
			}
			return store.AgentDecisionDenied, json.RawMessage(`{"allow":false}`), nil
		}
		scope := req.Scope
		if scope == "" {
			scope = store.AgentDecisionScopeOnce
		}
		if scope != store.AgentDecisionScopeOnce && scope != store.AgentDecisionScopeSession {
			return "", nil, invalid("scope must be once or session", "scope")
		}
		raw, _ := json.Marshal(map[string]any{"allow": true, "scope": scope})
		return store.AgentDecisionAllowed, raw, nil
	}
	if req.Answer == nil || req.Allow != nil || req.Scope != "" {
		return "", nil, invalid("a question is answered with answer text", "answer")
	}
	text := strings.TrimSpace(*req.Answer)
	if text == "" || len(text) > maxDecisionAnswerBytes || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return "", nil, invalid("answer must be 1-16384 bytes of text", "answer")
	}
	raw, _ := json.Marshal(map[string]string{"text": text})
	return store.AgentDecisionAnswered, raw, nil
}

// expireDecisions resolves every decision past its expiry, in batches.
func (s *Service) expireDecisions(ctx context.Context) {
	for {
		expired, err := s.Decisions.ExpireAgentDecisions(ctx, decisionSweepBatch)
		if err != nil {
			log.Printf("agent-session decision sweep: %v", err)
			return
		}
		for _, d := range expired {
			log.Printf("agent-session decision: %s expired unanswered (session=%s kind=%s)", d.ID, d.SessionID, d.Kind)
		}
		if len(expired) < decisionSweepBatch {
			return
		}
	}
}

// RunDecisionSweep expires unanswered decisions every interval until ctx ends.
func RunDecisionSweep(ctx context.Context, s *Service, interval time.Duration) {
	if s == nil || !s.decisionsEnabled() {
		return
	}
	core.Poll(ctx, "agent decision sweep", interval, func(ctx context.Context) error {
		s.expireDecisions(ctx)
		return nil
	})
}

// sessionApprovalMode resolves a create's approval mode against the
// workspace's (ADR100 D3): the workspace setting by default, explicitly raised
// at will, lowered (approvalMode below it, or bypassApproval) only when the
// workspace allows bypass. Without the decision store every session runs
// autonomous, and asking for more is refused rather than silently ignored.
func (s *Service) sessionApprovalMode(ctx context.Context, workspaceID, requested string, bypass bool) (string, error) {
	invalid := func(msg string) error {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", msg, map[string]any{"field": "approvalMode"})
	}
	if requested != "" {
		if _, ok := approvalRank[requested]; !ok {
			return "", invalid("approvalMode must be autonomous, risky or all")
		}
	}
	if bypass {
		if requested != "" && requested != ApprovalAutonomous {
			return "", invalid("bypassApproval runs the session autonomous; omit approvalMode or set it to autonomous")
		}
		requested = ApprovalAutonomous
	}
	if !s.decisionsEnabled() {
		if requested != "" && requested != ApprovalAutonomous {
			return "", core.NewUnavailableError("AGENT_SESSION_APPROVAL_UNAVAILABLE", "approval mode needs the decision store, which this deployment does not configure", nil)
		}
		return ApprovalAutonomous, nil
	}
	env, err := s.workspaceEnvironment(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	floor := env.ApprovalMode
	if _, ok := approvalRank[floor]; !ok {
		floor = ApprovalAutonomous
	}
	if requested == "" {
		return floor, nil
	}
	if approvalRank[requested] < approvalRank[floor] && !env.ApprovalBypassAllowed {
		return "", core.NewForbiddenError("AGENT_SESSION_APPROVAL_BYPASS_FORBIDDEN",
			"this workspace requires approval mode "+floor+" and does not allow sessions to lower it",
			map[string]any{"approvalMode": floor})
	}
	return requested, nil
}

// withDecisionEnv adds the approval contract to a driver env (ADR100 D5): the
// session's mode and, when it can pause, its Pod-bound decision URL. An
// autonomous session gets neither, so its sandbox behaves exactly as before.
func (s *Service) withDecisionEnv(env map[string]string, record store.AgentSession) map[string]string {
	mode := record.ApprovalMode
	if !s.decisionsEnabled() || mode == "" || mode == ApprovalAutonomous {
		return env
	}
	url, err := agentsession.DecisionURL(s.gitProxyURL(), store.SandboxNamespace(record.WorkspaceID), record.ID)
	if err != nil {
		log.Printf("agent-session dispatch: decision URL invalid (session=%s): %v", record.ID, err)
		return env
	}
	env = withEnv(env, "BEX_AGENT_APPROVAL_MODE", mode)
	return withEnv(env, "BEX_AGENT_DECISION_URL", url)
}
