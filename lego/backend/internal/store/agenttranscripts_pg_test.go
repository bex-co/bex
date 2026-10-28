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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/agenttranscript/uimessagetest"
)

func legacySession(t *testing.T, s *PGStore, workspace string, chunks []json.RawMessage, settle bool, reason string) string {
	t.Helper()
	parts := make([]AgentSessionTranscriptPart, len(chunks))
	for i, c := range chunks {
		parts[i] = AgentSessionTranscriptPart{Turn: 1, PartIndex: int64(i), Part: c}
	}
	return seedSession(t, s, workspace, parts, settle, reason)
}

// seedSession creates a session whose turn 1 holds parts as stored rows.
func seedSession(t *testing.T, s *PGStore, workspace string, parts []AgentSessionTranscriptPart, settle bool, reason string) string {
	t.Helper()
	ctx := context.Background()
	session, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: workspace, Repo: "bex-co/bex", Branch: "bex-agent/s1",
		AgentConfig: []byte(`{"agent":"claude","task":"t"}`), InitialPrompt: "t",
	})
	if err != nil {
		t.Fatal(err)
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

// storedFullOutputs reads every full output of one session turn, keyed by
// uimessagetest.FullKey.
func storedFullOutputs(t *testing.T, s *PGStore, sessionID string, turn int) map[string]string {
	t.Helper()
	rows, err := s.Pool.Query(context.Background(), `
		SELECT tool_call_id, field, output FROM agent_session_tool_outputs WHERE session_id = $1 AND turn = $2`, sessionID, turn)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, field, text string
		if err := rows.Scan(&id, &field, &text); err != nil {
			t.Fatal(err)
		}
		out[uimessagetest.FullKey(id, field)] = text
	}
	return out
}

// w10/m18: both writers store a clipped output's full copy idempotently (the
// first write wins, as the gateway role cannot UPDATE), the read is per tool
// call and turn, and the per-session cap refuses a copy that would exceed it.
func TestPGToolFullOutputs(t *testing.T) {
	s, _, _ := transcriptBatchPGFixture(t)
	ctx := context.Background()
	tenant, err := s.CreateTenant(ctx, "tool-full-output-test", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
	tool := func(index int64, id, text string) AgentSessionTranscriptPart {
		return AgentSessionTranscriptPart{Turn: 1, PartIndex: index, Item: true,
			Part:        json.RawMessage(`{"type":"dynamic-tool","toolCallId":"` + id + `","state":"output-available","fullOutput":true}`),
			FullOutputs: []agenttranscript.FullOutput{{ToolCallID: id, Field: agenttranscript.FieldOutput, Text: text}}}
	}
	session := seedSession(t, s, tenant.ID, []AgentSessionTranscriptPart{tool(0, "a", "tee copy")}, false, "")

	// A repeat (a second gateway replica, a reattach) keeps the first copy.
	if err := s.AppendAgentSessionTranscript(ctx, session, []AgentSessionTranscriptPart{tool(0, "a", "other copy")}); err != nil {
		t.Fatal(err)
	}
	// The harvest merge stores the copies of the items it folds.
	if _, err := s.MergeAgentSessionTranscriptItems(ctx, session, TranscriptItemMerge{Turn: 1, Quota: MaxAgentSessionTranscriptBytes,
		Items: []AgentSessionTranscriptPart{tool(0, "a", "harvest copy"), tool(5, "b", "harvested b")}}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.AgentSessionToolOutputs(ctx, session, 1, "a"); err != nil || len(got) != 1 || got["output"] != "tee copy" {
		t.Fatalf("tool a = %v err=%v, want the first copy", got, err)
	}
	if got, _ := s.AgentSessionToolOutputs(ctx, session, 1, "b"); got["output"] != "harvested b" {
		t.Fatalf("tool b = %v", got)
	}
	if got, _ := s.AgentSessionToolOutputs(ctx, session, 2, "a"); len(got) != 0 {
		t.Fatalf("another turn's read = %v, want none", got)
	}

	// The session cap counts every stored copy.
	var used int64
	if err := s.Pool.QueryRow(ctx, `SELECT sum(octet_length(output)) FROM agent_session_tool_outputs WHERE session_id = $1`, session).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO agent_session_tool_outputs (session_id, turn, tool_call_id, field, output)
		VALUES ($1, 9, 'filler', 'output', repeat('x', $2))`, session, MaxAgentSessionToolOutputBytes-used-10); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendAgentSessionTranscript(ctx, session, []AgentSessionTranscriptPart{tool(7, "big", "eleven byte")}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendAgentSessionTranscript(ctx, session, []AgentSessionTranscriptPart{tool(8, "fits", "ten bytes!")}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.AgentSessionToolOutputs(ctx, session, 1, "big"); len(got) != 0 {
		t.Fatalf("a copy over the session cap was stored: %v", got)
	}
	if got, _ := s.AgentSessionToolOutputs(ctx, session, 1, "fits"); got["output"] != "ten bytes!" {
		t.Fatalf("a copy exactly at the cap = %v", got)
	}
	// The items themselves are stored either way (their inline head + tail).
	if rows := storedTranscript(t, s, session); len(rows) != 4 {
		t.Fatalf("transcript rows = %d, want 4", len(rows))
	}
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
	full := storedFullOutputs(t, s, recorded, 1)
	var fullBytes int
	for _, text := range full {
		fullBytes += len(text)
	}
	t.Logf("recorded turn: %d full outputs / %d bytes out of line", len(full), fullBytes)
	if len(full) == 0 {
		t.Fatal("compaction stored no full outputs for the clipped tool outputs")
	}
	for i := range want {
		if want[i]["state"] == "streaming" {
			got[i]["state"] = "streaming" // the recorded turn ends inside a reasoning block
		}
		if _, err := uimessagetest.RestoreFullOutput(got[i], full); err != nil {
			t.Fatalf("part %d: %v", i, err)
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

// The part-cap flag is cleared only when the stored rows themselves prove the
// turn complete. A gap in the chunk ordinals, or a turn that already holds an
// item row (a tee that spanned the deploy), still compacts but keeps its flag.
func TestPGCompactionKeepsTheFlagWithoutProofOfCompleteness(t *testing.T) {
	s, _, _ := transcriptBatchPGFixture(t)
	ctx := context.Background()
	tenant, err := s.CreateTenant(ctx, "transcript-compaction-flag-test", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
	row := func(index int64, part string, item bool) AgentSessionTranscriptPart {
		return AgentSessionTranscriptPart{Turn: 1, PartIndex: index, Part: json.RawMessage(part), Item: item}
	}
	gap := seedSession(t, s, tenant.ID, []AgentSessionTranscriptPart{
		row(0, `{"type":"start"}`, false),
		row(1, `{"type":"text-start","id":"t"}`, false),
		row(2, `{"type":"text-delta","id":"t","delta":"hi"}`, false),
		row(3, `{"type":"text-end","id":"t"}`, false),
		row(5, `{"type":"finish"}`, false),
	}, true, LegacyPartLimitReason)
	mixed := seedSession(t, s, tenant.ID, []AgentSessionTranscriptPart{
		row(0, `{"type":"start"}`, false),
		row(1, `{"type":"text","id":"t","text":"hi","state":"done"}`, true),
		row(2, `{"type":"finish"}`, false),
	}, true, LegacyPartLimitReason)

	stats, err := s.CompactAgentSessionTranscripts(ctx, TranscriptCompactionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.FlagsCleared != 0 {
		t.Fatalf("cleared %d flags without proof of completeness: %+v", stats.FlagsCleared, stats)
	}
	for name, id := range map[string]string{"gap": gap, "mixed": mixed} {
		rows := storedTranscript(t, s, id)
		if len(rows) != 3 || string(rows[1].Part) != `{"type":"text","id":"t","text":"hi","state":"done"}` {
			t.Fatalf("%s: compacted rows = %+v", name, rows)
		}
		for _, r := range rows {
			if !r.Item || r.Partial {
				t.Fatalf("%s: row %+v not a complete item", name, r)
			}
		}
		turns, _ := s.AgentSessionTurns(ctx, id)
		if !turns[0].TranscriptTruncated || turns[0].TruncationReason != LegacyPartLimitReason {
			t.Fatalf("%s: flag changed: %+v", name, turns[0])
		}
	}
}

// The per-session cap counts the copies one transaction already stored: a
// batch whose outputs together exceed the cap keeps the ones that fit, in
// order, and a repeated (conflicting) copy adds nothing to the count.
func TestPGToolFullOutputCapCountsTheSameTransaction(t *testing.T) {
	s, _, _ := transcriptBatchPGFixture(t)
	ctx := context.Background()
	tenant, err := s.CreateTenant(ctx, "tool-full-output-batch-test", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
	session := seedSession(t, s, tenant.ID, nil, false, "")
	half := strings.Repeat("h", MaxAgentSessionToolOutputBytes/2-8)
	tool := func(index int64, id string, outputs ...agenttranscript.FullOutput) AgentSessionTranscriptPart {
		return AgentSessionTranscriptPart{Turn: 1, PartIndex: index, Item: true,
			Part:        json.RawMessage(`{"type":"dynamic-tool","toolCallId":"` + id + `","state":"output-available","fullOutput":true}`),
			FullOutputs: outputs}
	}
	out := func(id, text string) agenttranscript.FullOutput {
		return agenttranscript.FullOutput{ToolCallID: id, Field: agenttranscript.FieldOutput, Text: text}
	}
	if err := s.AppendAgentSessionTranscript(ctx, session, []AgentSessionTranscriptPart{
		tool(0, "a", out("a", half)),
		tool(1, "a-again", out("a", half)), // same key as a: a conflict, not counted
		tool(2, "b", out("b", half)),
		tool(3, "c", out("c", half)), // a + b leave no room
	}); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{"a": true, "b": true, "c": false} {
		if got, _ := s.AgentSessionToolOutputs(ctx, session, 1, id); (len(got) == 1) != want {
			t.Errorf("tool %s stored = %v, want %v", id, len(got) == 1, want)
		}
	}
}
