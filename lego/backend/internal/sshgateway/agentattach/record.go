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

package agentattach

import (
	"bufio"
	"context"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// recordDrainTimeout bounds the recorder's drain of the driver replay. Unlike the
// browser attach (which lives for the whole turn, up to SessionTimeout), the
// recorder drains an already-complete per-turn replay that ends in seconds; a
// tight bound keeps a `[DONE]`-less driver from stalling the Completer teardown.
const recordDrainTimeout = 2 * time.Minute

// The headless recorder (ADR051) closes the phase-1 transcript-persistence gap:
// a fire-and-forget session runs headless with no browser attached, so the live
// tee (streamAgentAttach / forwardAgentTurn) never fires and nothing lands in
// agent_session_transcripts — every completed session then replays empty ("No
// conversation yet."). This internal, non-browser endpoint is the missing
// trigger: the bex-api Completer calls it once per turn, just before it tears the
// sandbox down, while the driver's in-memory hub is still alive and its GET
// /stream still replays the full turn. The recorder dials that stream and tees
// the whole replay into the durable store — the same byte-transparent path the
// browser splice uses.
//
// It is authenticated by RecordSecret (the sandbox-exec HMAC, distinct from the
// browser ticket key), has NO edge route, and is idempotent per turn: a turn
// whose parts already exist (a Completer retry, or a live viewer that already
// teed it) is skipped, so it never double-stores a conversation.

// RecordEnabled reports whether the internal recorder endpoint is configured.
func (s *Server) RecordEnabled() bool {
	return len(s.RecordSecret) > 0 && s.Store != nil && s.Pods != nil
}

// RecordHandler serves the internal POST record endpoint. Mount it only on the
// gateway's internal (non-edge) listener.
func (s *Server) RecordHandler() http.Handler {
	s.defaults()
	return http.HandlerFunc(s.serveRecord)
}

func (s *Server) serveRecord(w http.ResponseWriter, r *http.Request) {
	if !s.RecordEnabled() {
		http.Error(w, "agent session recorder not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// The record ticket reuses the agent-session claim shape but is signed with
	// RecordSecret (the exec HMAC), so a browser ticket cannot reach this path.
	// No nonce is consumed: the caller is the trusted Completer and the write is
	// idempotent per turn, so a retry must be able to re-present the same ticket.
	claims, err := agentsessionticket.Verify(s.RecordSecret, r.Header.Get(TicketHeader), s.now())
	if err != nil {
		s.Metrics.Authentication("rejected_key")
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	n, err := s.recordSession(r.Context(), claims)
	if err != nil {
		// Best-effort: the Completer proceeds to finalize/teardown regardless, so
		// a failure here leaves the transcript as it was (never strands a session).
		log.Printf("agent record: session=%s turn=%d: %v", claims.SessionID, claims.Turn, err)
		http.Error(w, "record failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "recorded "+strconv.Itoa(n))
}

// recordSession dials the still-live driver's GET /stream and tees its full
// replay into the durable transcript for one turn. It is idempotent per turn
// (skips a turn that already has parts) and appends after the current stored max
// so a redispatched turn concatenates onto prior turns rather than colliding.
//
// It assumes a per-turn driver hub (the fire-and-forget shape: one turn per
// sandbox/driver, closeHub=true) — the driver's replay is exactly this turn's
// parts. That is the flow this recorder exists for.
func (s *Server) recordSession(ctx context.Context, claims agentsessionticket.Claims) (int, error) {
	// Idempotency + live-tee-overlap guard: if this turn already has stored parts
	// (a retry, or a browser that teed it live), do not record again.
	recorded, err := s.Store.AgentSessionTranscriptTurnRecorded(ctx, claims.SessionID, claims.Turn)
	if err != nil {
		return 0, err
	}
	if recorded {
		return 0, nil
	}
	// Base = current max seq across all prior turns, so this turn's parts append
	// after them (redispatched-turn concatenation). Empty transcript => -1 => 0-based.
	base, _, err := s.Store.AgentSessionTranscriptMaxSeq(ctx, claims.SessionID)
	if err != nil {
		return 0, err
	}
	podIP, err := s.Pods.PodIP(ctx, claims.Namespace, claims.Pod)
	if err != nil {
		// No live driver (already gone): nothing to record. Not an error — the
		// session simply has no capturable transcript (the documented limit).
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(ctx, recordDrainTimeout)
	defer cancel()
	resp, err := s.dialDriverStream(ctx, podIP)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Drain the whole (bounded, already-complete) replay, then persist in ONE
	// batched append — no live client observes these rows mid-drain, so the
	// per-part insert the browser splice needs would only add round-trips here. A
	// read error before `[DONE]`/EOF returns without writing, so a retry captures
	// the turn whole rather than leaving it partial.
	seq := base
	total := 0 // transcript bytes accumulated this recording
	var parts []store.AgentSessionTranscriptPart
	reader := bufio.NewReader(resp.Body)
	for {
		// Bound each part (maxSSEPartBytes) and the recording total
		// (maxSessionTranscriptBytes) so tenant-controlled driver output can't
		// grow Postgres or the gateway without limit on this hop either (w1/m65 F10).
		payload, done, err := readSSEData(reader, maxSSEPartBytes)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if done || err == io.EOF { // full replay delivered
			break
		}
		if payload == "" {
			continue
		}
		if total+len(payload) > maxSessionTranscriptBytes {
			log.Printf("agent record: session transcript byte quota reached, truncating (session=%s)", claims.SessionID)
			break
		}
		total += len(payload)
		seq++
		parts = append(parts, store.AgentSessionTranscriptPart{Seq: seq, Turn: claims.Turn, Part: []byte(payload)})
	}
	if err := s.Store.AppendAgentSessionTranscript(ctx, claims.SessionID, parts); err != nil {
		return 0, err
	}
	return len(parts), nil
}
