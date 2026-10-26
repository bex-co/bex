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
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/agenttranscript/uimessagetest"
)

func legacySession(t *testing.T, s *PGStore, workspace string, chunks []json.RawMessage, settle bool, reason string) string {
	t.Helper()
	ctx := context.Background()
	session, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: workspace, Repo: "bex-co/bex", Branch: "bex-agent/s1",
		AgentConfig: []byte(`{"agent":"claude","task":"t"}`), InitialPrompt: "t",
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := make([]AgentSessionTranscriptPart, len(chunks))
	for i, c := range chunks {
		parts[i] = AgentSessionTranscriptPart{Turn: 1, PartIndex: int64(i), Part: c}
	}
	for offset := 0; offset < len(parts); offset += 1000 {
		if err := s.AppendAgentSessionTranscript(ctx, session.ID, parts[offset:min(offset+1000, len(parts))]); err != nil {
			t.Fatal(err)
		}
	}
	if settle {
		if err := s.CompleteAgentSessionTurn(ctx, session.ID, 1, false, reason != "", reason); err != nil {
			t.Fatal(err)
		}
	}
	return session.ID
}

func storedTranscript(t *testing.T, s *PGStore, sessionID string) []AgentSessionTranscriptPart {
	t.Helper()
	rows, err := s.AgentSessionTranscript(context.Background(), sessionID, -1, MaxAgentSessionTranscriptBytes, 0)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func replayedParts(t *testing.T, rows []AgentSessionTranscriptPart) []uimessagetest.Part {
	t.Helper()
	var chunks []json.RawMessage
	for _, row := range rows {
		if row.Item {
			chunks = append(chunks, agenttranscript.Expand(row.Part)...)
		} else {
			chunks = append(chunks, row.Part)
		}
	}
	parts, err := uimessagetest.Merge(chunks)
	if err != nil {
		t.Fatal(err)
	}
	return parts
}

// The w10/m16 backfill: a legacy raw-chunk turn (the recorded 5,602-chunk
// production turn) compacts to items that render the same content, keeps its
// replay order, clears a part-cap truncation flag only when the rows prove the
// turn complete, leaves a possibly-running turn alone, and re-running is a no-op.
func TestPGCompactAgentSessionTranscripts(t *testing.T) {
	s, _, _ := transcriptBatchPGFixture(t)
	ctx := context.Background()
	tenant, err := s.CreateTenant(ctx, "transcript-compaction-test", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
	fixture, err := uimessagetest.LoadFixture()
	if err != nil {
		t.Fatal(err)
	}
	recorded := legacySession(t, s, tenant.ID, fixture, true, LegacyPartLimitReason)
	small := []json.RawMessage{
		json.RawMessage(`{"type":"start"}`),
		json.RawMessage(`{"type":"text-start","id":"t"}`),
		json.RawMessage(`{"type":"text-delta","id":"t","delta":"all "}`),
		json.RawMessage(`{"type":"text-delta","id":"t","delta":"there"}`),
		json.RawMessage(`{"type":"text-end","id":"t"}`),
		json.RawMessage(`{"type":"finish"}`),
	}
	complete := legacySession(t, s, tenant.ID, small, true, LegacyPartLimitReason)
	running := legacySession(t, s, tenant.ID, small[:3], false, "")
	want := replayedParts(t, storedTranscript(t, s, recorded))

	plan, err := s.CompactAgentSessionTranscripts(ctx, TranscriptCompactionOptions{DryRun: true, BatchSessions: 2})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Turns < 2 || plan.SkippedOpenTurns < 1 || plan.FlagsCleared < 1 {
		t.Fatalf("dry run plan = %+v", plan)
	}
	if n := len(storedTranscript(t, s, recorded)); n != len(fixture) {
		t.Fatalf("dry run wrote: %d rows", n)
	}

	stats, err := s.CompactAgentSessionTranscripts(ctx, TranscriptCompactionOptions{BatchSessions: 2, Logf: t.Logf})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("compaction: %+v", stats)
	rows := storedTranscript(t, s, recorded)
	var bytes int
	for i, row := range rows {
		bytes += len(row.Part)
		if !row.Item || (i > 0 && row.PartIndex <= rows[i-1].PartIndex) {
			t.Fatalf("row %d = %+v: not an item in index order", i, row)
		}
	}
	t.Logf("recorded turn: %d rows → %d rows / %d bytes", len(fixture), len(rows), bytes)
	if len(rows) > 600 {
		t.Fatalf("compacted to %d rows, want ≤ 600", len(rows))
	}
	got := replayedParts(t, rows)
	if len(got) != len(want) {
		t.Fatalf("compacted replay rebuilt %d parts, want %d", len(got), len(want))
	}
	for i := range want {
		if want[i]["state"] == "streaming" {
			got[i]["state"] = "streaming" // the recorded turn ends inside a reasoning block
		}
		if fmt.Sprint(want[i]) != fmt.Sprint(got[i]) {
			t.Fatalf("part %d differs after compaction", i)
		}
	}
	turns, _ := s.AgentSessionTurns(ctx, recorded)
	if !turns[0].TranscriptTruncated || turns[0].TruncationReason != LegacyPartLimitReason {
		t.Fatalf("recorded turn has no finish, yet its flag changed: %+v", turns[0])
	}
	turns, _ = s.AgentSessionTurns(ctx, complete)
	if turns[0].TranscriptTruncated || !turns[0].TranscriptComplete || turns[0].TruncationReason != "" {
		t.Fatalf("complete turn still flagged: %+v", turns[0])
	}
	if rows := storedTranscript(t, s, complete); len(rows) != 3 || string(rows[1].Part) != `{"type":"text","id":"t","text":"all there","state":"done"}` {
		t.Fatalf("small turn = %+v", rows)
	}
	if rows := storedTranscript(t, s, running); len(rows) != 3 || rows[1].Item {
		t.Fatalf("an open turn was rewritten: %+v", rows)
	}

	again, err := s.CompactAgentSessionTranscripts(ctx, TranscriptCompactionOptions{})
	if err != nil || again.Turns != 0 || again.SkippedOpenTurns < 1 {
		t.Fatalf("re-run = %+v err=%v, want a no-op", again, err)
	}
	if after := storedTranscript(t, s, recorded); fmt.Sprint(after) != fmt.Sprint(rows) {
		t.Fatal("re-run changed the compacted rows")
	}
}
