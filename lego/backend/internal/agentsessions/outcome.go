package agentsessions

import (
	"context"
	"encoding/json"
	"log"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// maxOutcomeSummaryChars bounds Outcome.Summary (ADR094 D7).
	maxOutcomeSummaryChars = 4000
	// outcomeTailItems bounds how many trailing items are scanned for the final
	// assistant text; a turn ends in text unless it ended mid tool call.
	outcomeTailItems      = 40
	outcomeTailBytesLimit = 1 << 20
)

// outcomeOf builds the outcome from the stored row alone (no transcript read).
func outcomeOf(record store.AgentSession) *Outcome {
	out := &Outcome{
		State:             stateOf(record.Phase),
		Steerable:         steerable(record.Phase, record.ArchivedAt != nil),
		Turn:              record.Turns,
		DeliveryNote:      record.DeliveryNote,
		FailureReasonCode: record.FailureReasonCode,
	}
	if record.PRURL != "" {
		out.PullRequest = &PullRequest{URL: record.PRURL, Number: record.PRNumber, HeadSHA: record.HeadSHA}
	}
	return out
}

// fillOutcome adds what costs reads to an outcome built from the row alone: the
// latest completed turn's final assistant text (clipped head-and-tail) and the
// files the agent shared in it, and the current turn's final structured result.
// Each stays empty when there is nothing to report or a read fails: the outcome
// is a convenience, and the transcript, attachment list and turn rows stay the
// source of truth.
func (s *Service) fillOutcome(ctx context.Context, record store.AgentSession, out *Outcome) {
	turns, err := s.Store.AgentSessionTurns(ctx, record.ID)
	if err != nil {
		log.Printf("agent-session outcome: list turns session=%s: %v", record.ID, err)
		return
	}
	out.Cost = s.outcomeCost(ctx, record.ID)
	latest := -1
	for _, t := range turns {
		if t.CompletedAt != nil && t.Turn > latest {
			latest = t.Turn
		}
		// The result belongs to the current turn, whose completed_at may not be
		// stamped yet when the phase turns terminal.
		if t.Turn == record.Turns && t.ResultFinal {
			out.Result = jsonValue(t.Result)
		}
	}
	if latest >= 0 {
		out.Summary, out.Files = s.outcomeSummary(ctx, record.ID, latest), s.outcomeFiles(ctx, record.ID, latest)
	}
}

// outcomeCost is the session's platform-route model cost so far, to the
// micro-dollar from the same events ADR091 bills (ModelCost reads them to the
// cent). Nil when it made no priced request, and on a read failure.
func (s *Service) outcomeCost(ctx context.Context, sessionID string) *Cost {
	if s.ModelCosts == nil {
		return nil
	}
	read, err := s.ModelCosts.AgentSessionModelUsage(ctx, sessionID)
	if err != nil {
		log.Printf("agent-session outcome: read model cost session=%s: %v", sessionID, err)
		return nil
	}
	var total, requests int64
	for _, t := range read.Turns {
		total += t.PriceMicroUSD
		requests += t.Requests
	}
	if requests == 0 {
		return nil
	}
	return &Cost{ModelUSD: microUSDDecimal(total)}
}

func (s *Service) outcomeSummary(ctx context.Context, sessionID string, turn int) string {
	items, err := s.Store.AgentSessionTurnTail(ctx, sessionID, turn, outcomeTailBytesLimit, outcomeTailItems)
	if err != nil {
		log.Printf("agent-session outcome: read turn tail session=%s: %v", sessionID, err)
		return ""
	}
	for _, item := range items { // newest first
		var part struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(item.Part, &part) == nil && part.Type == "text" && part.Text != "" {
			return agenttranscript.ClipHeadTail(part.Text, maxOutcomeSummaryChars)
		}
	}
	return ""
}

func (s *Service) outcomeFiles(ctx context.Context, sessionID string, turn int) []TurnAttachment {
	if s.AttachmentRows == nil {
		return nil
	}
	rows, err := s.AttachmentRows.ListAgentSessionAttachments(ctx, sessionID, store.AgentAttachmentListQuery{
		Direction: store.AgentAttachmentOutput, Turn: turn, Limit: store.MaxAgentSessionAttachments,
	})
	if err != nil {
		log.Printf("agent-session outcome: list shared files session=%s: %v", sessionID, err)
		return nil
	}
	files := make([]TurnAttachment, 0, len(rows))
	for _, a := range rows {
		files = append(files, TurnAttachment{ID: a.ID, Name: a.Name, MediaType: a.MediaType, Size: a.SizeBytes})
	}
	return files
}

// TurnOutcomes is the webhook worker's read of a turn event's outcome
// (webhooks.AgentSessionOutcomes). It is a separate type, not a Service verb:
// it acts for the platform delivering to a subscribed endpoint and has no
// caller identity to authorize, so it must stay out of the verb surface the
// transports and the authorization sweep enumerate.
type TurnOutcomes struct{ Service *Service }

// AgentSessionTurnOutcome is the payload of an agent_session_turn_* webhook
// (ADR094 D9): the session's state for the phase its turn ended in and the
// outcome the session read carries, without the summary (which can quote
// the repository and tool output) and the file list. It acts for the platform
// delivering to a subscribed endpoint, not for a caller, so it does no
// authorization. When the session has since moved on to another turn, only the
// state and turn are reported: the row's other fields belong to the newer
// turn. store.ErrNotFound when the session is gone.
func (o TurnOutcomes) AgentSessionTurnOutcome(ctx context.Context, sessionID string, turn int, phase string) (string, json.RawMessage, error) {
	s := o.Service
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return "", nil, err
	}
	state := stateOf(phase)
	out := &Outcome{State: state, Turn: turn}
	if record.Turns == turn {
		out = outcomeOf(record)
		out.State = state
		out.Steerable = steerable(record.Phase, record.ArchivedAt != nil)
		s.fillOutcome(ctx, record, out)
		out.Summary, out.Files = "", nil
	}
	raw, err := json.Marshal(out)
	return state, raw, err
}
