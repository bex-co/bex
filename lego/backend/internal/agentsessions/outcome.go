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

// outcomeSummary returns the final assistant text of the latest completed turn,
// clipped head-and-tail, or "" when none exists or the read fails (the summary
// is a convenience; the transcript tool stays the source of truth).
func (s *Service) outcomeSummary(ctx context.Context, record store.AgentSession) string {
	turns, err := s.Store.AgentSessionTurns(ctx, record.ID)
	if err != nil {
		log.Printf("agent-session outcome: list turns session=%s: %v", record.ID, err)
		return ""
	}
	latest := -1
	for _, t := range turns {
		if t.CompletedAt != nil && t.Turn > latest {
			latest = t.Turn
		}
	}
	if latest < 0 {
		return ""
	}
	items, err := s.Store.AgentSessionTurnTail(ctx, record.ID, latest, outcomeTailBytesLimit, outcomeTailItems)
	if err != nil {
		log.Printf("agent-session outcome: read turn tail session=%s: %v", record.ID, err)
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
