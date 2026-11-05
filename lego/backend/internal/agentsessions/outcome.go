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
