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
	"log"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Continuity envs (ADR047 D3 continuity amendment, w5/m84). A steer or resume
// that dispatches a FRESH sandbox starts a blank agent process while the
// dashboard replays the full durable history — ags-da9mh5vj596c73en5eq0
// answered "what should I try again?" beneath its own transcript. bex-api owns
// selecting the raw material because only it can read the store; the driver
// owns rendering and the final byte budget (ADR051). At most one of these envs
// is injected:
//
//   - BEX_AGENT_CONTEXT_JSON — prior conversation extract (ladder rung 2):
//     [{"turn":N,"prompt":"…","reply":"…"}, …] oldest→newest. The driver
//     prepends a bounded preamble unless ACP session/load succeeded (rung 1).
//   - BEX_AGENT_ORIGINAL_TASK — the session's task (rung 3), injected when
//     prior turns exist but none ever produced agent output (setup-phase
//     failures): "try again" must retry the task, not interrogate a blank
//     agent.
const (
	continuityContextEnv = "BEX_AGENT_CONTEXT_JSON"
	continuityTaskEnv    = "BEX_AGENT_ORIGINAL_TASK"

	// continuityMaxSerializedBytes caps the serialized extract handed to the
	// dispatch env. Sized to the driver's 24 KiB rendered-preamble budget plus
	// JSON/framing headroom — bytes beyond that are guaranteed-discarded env
	// weight on every fresh-dispatch Pod spec.
	continuityMaxSerializedBytes = 32 << 10
	// continuityMaxReplyBytes bounds one turn's assistant-reply extract; the
	// tail carries the conclusion, so keep the end, not the start.
	continuityMaxReplyBytes = 8 << 10
	// continuityTranscriptReadBytes/Rows bound the store read.
	continuityTranscriptReadBytes = 4 << 20
	continuityTranscriptReadRows  = 5000
)

// continuityTurn is one prior exchange in the rung-2 extract.
type continuityTurn struct {
	Turn   int    `json:"turn"`
	Prompt string `json:"prompt"`
	Reply  string `json:"reply,omitempty"`
}

// continuityEnv computes the rung-2/rung-3 material for a fresh-sandbox
// dispatch of a session with prior turns and stamps it into env. Best-effort
// by design: a store error degrades to no priming (logged) rather than
// blocking the dispatch — a cold turn is bad, a stuck session is worse.
func (s *Service) continuityEnv(ctx context.Context, env map[string]string, record store.AgentSession, task string) {
	turns, err := s.Store.AgentSessionTurns(ctx, record.ID)
	if err != nil {
		log.Printf("agent-session continuity: turns read failed (session=%s): %v", record.ID, err)
		return
	}
	// The current (just-accepted) turn's prompt is the one being dispatched —
	// prior turns are strictly older ones.
	prior := make([]store.AgentSessionTurn, 0, len(turns))
	for _, t := range turns {
		if t.Turn < record.Turns {
			prior = append(prior, t)
		}
	}
	if len(prior) == 0 {
		return
	}
	replies, sawParts := s.assistantReplies(ctx, record.ID)
	if !sawParts {
		// Prior turns exist but the agent never produced a single durable part:
		// it never really ran (the setup-failure shape). Rung 3.
		if strings.TrimSpace(task) != "" {
			env[continuityTaskEnv] = task
		}
		return
	}
	// Newest-first selection under the serialized cap (single pass — each
	// exchange is marshaled once), then chronological order for the driver,
	// which renders oldest→newest and applies its own tighter budget.
	kept := make([]continuityTurn, 0, len(prior))
	used := 2 // enclosing []
	for i := len(prior) - 1; i >= 0; i-- {
		entry := continuityTurn{Turn: prior[i].Turn, Prompt: prior[i].Prompt, Reply: replies[prior[i].Turn]}
		element, err := json.Marshal(entry)
		if err != nil {
			log.Printf("agent-session continuity: marshal failed (session=%s): %v", record.ID, err)
			return
		}
		if used+len(element)+1 > continuityMaxSerializedBytes {
			break
		}
		kept = append(kept, entry)
		used += len(element) + 1 // element plus separating comma
	}
	if len(kept) == 0 {
		return
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	serialized, err := json.Marshal(kept)
	if err != nil || len(serialized) > continuityMaxSerializedBytes {
		if err != nil {
			log.Printf("agent-session continuity: marshal failed (session=%s): %v", record.ID, err)
		}
		return
	}
	env[continuityContextEnv] = string(serialized)
}

// assistantReplies assembles per-turn assistant text from the durable
// transcript's UI-message chunks (`text-delta` deltas concatenated per turn,
// tail-trimmed to continuityMaxReplyBytes). sawParts reports whether ANY
// durable part exists for the session — the rung-2 vs rung-3 discriminator —
// independent of whether those parts contained plain text.
func (s *Service) assistantReplies(ctx context.Context, sessionID string) (replies map[int]string, sawParts bool) {
	// afterSeq -1: the read is exclusive (`seq > afterSeq`) and the first seq a
	// store allocates may be 0, so "everything" is -1, not 0.
	parts, err := s.Store.AgentSessionTranscript(ctx, sessionID, -1, continuityTranscriptReadBytes, continuityTranscriptReadRows)
	if err != nil {
		log.Printf("agent-session continuity: transcript read failed (session=%s): %v", sessionID, err)
		return nil, false
	}
	replies = make(map[int]string)
	builders := make(map[int]*strings.Builder)
	for _, part := range parts {
		sawParts = true
		var chunk struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}
		if json.Unmarshal(part.Part, &chunk) != nil || chunk.Type != "text-delta" || chunk.Delta == "" {
			continue
		}
		b := builders[part.Turn]
		if b == nil {
			b = &strings.Builder{}
			builders[part.Turn] = b
		}
		b.WriteString(chunk.Delta)
	}
	for turn, b := range builders {
		text := b.String()
		if len(text) > continuityMaxReplyBytes {
			text = text[len(text)-continuityMaxReplyBytes:]
		}
		replies[turn] = text
	}
	return replies, sawParts
}
