package agentsessions

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// wait_agent_sessions bounds (ADR094 D6). 25 s stays under bex-api's 30 s HTTP
// write timeout and the ~60 s tool-call timeout of common MCP clients; the
// caller calls again when timedOut is true.
const (
	maxWaitIDs         = 20
	maxWaitSeconds     = 25
	defaultWaitSeconds = 25
	defaultWaitPoll    = 2 * time.Second
	waitUntilAny       = "any"
	waitUntilAll       = "all"
)

// WaitRequest is wait_agent_sessions' input.
type WaitRequest struct {
	IDs            []string `json:"ids"`
	Until          string   `json:"until,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}

// WaitSession is one waited-on session's id, state and outcome.
type WaitSession struct {
	ID      string   `json:"id"`
	State   string   `json:"state"`
	Outcome *Outcome `json:"outcome,omitempty"`
}

// WaitResult is what a wait returns: every requested session's current summary,
// which of them have settled (state other than working), and whether the wait
// ended on its deadline.
type WaitResult struct {
	Sessions []WaitSession `json:"sessions"`
	Settled  []string      `json:"settled"`
	TimedOut bool          `json:"timedOut"`
}

// stateBus fans the pod's single LISTEN connection out to waiters. A waiter
// subscribes to a channel poked on a session's state change (any session's,
// or one named session's), then re-reads only its own sessions.
type stateBus struct {
	mu   sync.Mutex
	subs map[chan struct{}]string // channel → session filter ("" = all)
}

func (b *stateBus) subscribe() (<-chan struct{}, func()) { return b.subscribeSession("") }

// subscribeSession wakes only on sessionID's state changes; "" wakes on every
// session's.
func (b *stateBus) subscribeSession(sessionID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	if b.subs == nil {
		b.subs = map[chan struct{}]string{}
	}
	b.subs[ch] = sessionID
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

func (b *stateBus) poke(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch, want := range b.subs {
		if want != "" && want != sessionID {
			continue
		}
		select {
		case ch <- struct{}{}:
		default: // a wake-up is already pending
		}
	}
}

// bus starts the pod's one state listener on first use.
func (s *Service) bus() *stateBus {
	s.busOnce.Do(func() {
		s.stateBus = &stateBus{}
		if listener, ok := s.Store.(interface {
			ListenAgentSessionStates(ctx context.Context, fn func(id string)) error
		}); ok {
			go func() {
				_ = listener.ListenAgentSessionStates(context.Background(), func(id string) { s.stateBus.poke(id) })
			}()
		}
	})
	return s.stateBus
}

func (s *Service) waitPoll() time.Duration {
	if s.WaitPoll > 0 {
		return s.WaitPoll
	}
	return defaultWaitPoll
}

// Wait blocks until the named sessions settle (until=all, default) or any of
// them does (until=any), or the deadline passes. It reads once up front, so an
// already-settled set returns immediately, then re-reads on every phase-change
// notification and on a slow timer that also covers a missed notification.
// progress, when non-nil, is called with a summary line on each change.
func (s *Service) Wait(ctx context.Context, req WaitRequest, progress func(line string)) (WaitResult, error) {
	if len(req.IDs) == 0 {
		// Nothing to authorize per session: gate on the workspace so an
		// unauthorized caller is refused, not told how to shape the request.
		if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
			return WaitResult{}, err
		}
	}
	ids, until, timeout, err := normalizeWait(req)
	if err != nil {
		return WaitResult{}, err
	}
	// Authorize and validate every id before blocking: a bad id fails fast.
	rows, err := s.waitSnapshot(ctx, ids)
	if err != nil {
		return WaitResult{}, err
	}
	wake, unsubscribe := s.bus().subscribe()
	defer unsubscribe()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(s.waitPoll())
	defer tick.Stop()

	lastLine := ""
	for {
		result := waitResult(ids, rows)
		if progress != nil {
			if line := progressLine(result); line != lastLine {
				lastLine = line
				progress(line)
			}
		}
		if waitDone(result, until) {
			s.attachOutcomeDetail(ctx, result.Sessions, rows)
			return result, nil
		}
		select {
		case <-ctx.Done():
			return WaitResult{}, ctx.Err()
		case <-deadline.C:
			result.TimedOut = true
			s.attachOutcomeDetail(ctx, result.Sessions, rows)
			return result, nil
		case <-wake:
		case <-tick.C:
		}
		if rows, err = s.waitSnapshot(ctx, ids); err != nil {
			return WaitResult{}, err
		}
	}
}

func normalizeWait(req WaitRequest) (ids []string, until string, timeout time.Duration, err error) {
	if len(req.IDs) == 0 || len(req.IDs) > maxWaitIDs {
		return nil, "", 0, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "ids must name 1-20 agent sessions", map[string]any{"field": "ids"})
	}
	for _, id := range req.IDs {
		if err := validateSessionID(id); err != nil {
			return nil, "", 0, err
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	switch until = req.Until; until {
	case "":
		until = waitUntilAll
	case waitUntilAny, waitUntilAll:
	default:
		return nil, "", 0, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "until must be any or all", map[string]any{"field": "until"})
	}
	secs := req.TimeoutSeconds
	if secs == 0 {
		secs = defaultWaitSeconds
	}
	if secs < 1 || secs > maxWaitSeconds {
		return nil, "", 0, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", fmt.Sprintf("timeoutSeconds must be 1-%d", maxWaitSeconds), map[string]any{"field": "timeoutSeconds"})
	}
	return ids, until, time.Duration(secs) * time.Second, nil
}

// waitSnapshot reads each session once, authorized per id (can_operate, as Get).
func (s *Service) waitSnapshot(ctx context.Context, ids []string) (map[string]store.AgentSession, error) {
	if !s.enabled() {
		return nil, core.ErrAgentSessionsUnavailable
	}
	rows := make(map[string]store.AgentSession, len(ids))
	for _, id := range ids {
		if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(id)); err != nil {
			return nil, err
		}
		row, err := s.Store.GetAgentSession(ctx, id)
		if err != nil {
			return nil, mapStoreError(id, err)
		}
		rows[id] = row
	}
	return rows, nil
}

func waitResult(ids []string, rows map[string]store.AgentSession) WaitResult {
	out := WaitResult{Sessions: make([]WaitSession, 0, len(ids)), Settled: []string{}}
	for _, id := range ids {
		o := outcomeOf(rows[id])
		out.Sessions = append(out.Sessions, WaitSession{ID: id, State: o.State, Outcome: o})
		if o.State != StateWorking {
			out.Settled = append(out.Settled, id)
		}
	}
	return out
}

func waitDone(r WaitResult, until string) bool {
	if until == waitUntilAny {
		return len(r.Settled) > 0
	}
	return len(r.Settled) == len(r.Sessions)
}

func progressLine(r WaitResult) string {
	return fmt.Sprintf("%d of %d agent sessions settled", len(r.Settled), len(r.Sessions))
}

// attachOutcomeDetail fills the final-answer summary, shared files and result for
// settled sessions; the per-poll reads skip them so waiting costs no extra reads.
func (s *Service) attachOutcomeDetail(ctx context.Context, sessions []WaitSession, rows map[string]store.AgentSession) {
	for i := range sessions {
		if sessions[i].State == StateWorking {
			continue
		}
		s.fillOutcome(ctx, rows[sessions[i].ID], sessions[i].Outcome)
	}
}

func (s *Service) registerWaitMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{
		Name: "wait_agent_sessions",
		Description: "Block until cloud coding-agent sessions settle, instead of polling get_agent_session in a loop. " +
			"ids: 1-20 sessions. until: all (default) returns when every session has left state working, any when the first does; needs_input (paused on a decision, see outcome.pendingDecision) counts as left. " +
			"timeoutSeconds: 1-25 (default 25). Returns each session's id, state and outcome, the ids that settled, and timedOut. " +
			"When timedOut is true the sessions are still working: call again.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, in WaitRequest) (*mcp.CallToolResult, WaitResult, error) {
		var progress func(string)
		if req != nil && req.Params != nil && req.Session != nil {
			if token := req.Params.GetProgressToken(); token != nil {
				n := 0.0
				progress = func(line string) {
					n++
					_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: token, Message: line, Progress: n})
				}
			}
		}
		out, err := s.Wait(ctx, in, progress)
		return nil, out, err
	})
}

func boolPtr(b bool) *bool { return &b }
