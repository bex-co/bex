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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/agenttranscript/uimessagetest"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestCompleterPGRecoversTranscriptWithinExactQuota(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "completer-transcript-test", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pg.DeleteTenant(ctx, tenant.ID); err != nil {
			t.Error(err)
		}
	}()
	payloads := []string{`{"type":"start"}`, `{"type":"text-start","id":"text"}`, `{"type":"text-delta","id":"text","delta":"recovered"}`,
		`{"type":"data-acp-info","data":{},"transient":true}`, `{"type":"text-end","id":"text"}`, `{"type":"finish"}`}
	chunks := make([]agenttranscript.Chunk, len(payloads))
	var driverLog strings.Builder
	for i, payload := range payloads {
		chunks[i] = agenttranscript.Chunk{Index: int64(i), Part: []byte(payload)}
		fmt.Fprintf(&driverLog, "{\"turn\":1,\"partIndex\":%d,\"part\":%s}\n", i, payload)
	}
	items := map[int64]string{}
	var quota int64
	for _, it := range agenttranscript.Coalesce(chunks) {
		items[it.Index] = string(it.Part)
		quota += int64(len(it.Part))
	}
	item := func(index int64) store.AgentSessionTranscriptPart {
		return store.AgentSessionTranscriptPart{Turn: 1, PartIndex: index, Part: []byte(items[index]), Item: true}
	}
	partialText := store.AgentSessionTranscriptPart{Turn: 1, PartIndex: 1, Item: true, Partial: true,
		Part: []byte(`{"type":"text","id":"text","text":"recov","state":"streaming"}`)}
	legacy := func(index int64) store.AgentSessionTranscriptPart {
		return store.AgentSessionTranscriptPart{Turn: 1, PartIndex: index, Part: []byte(payloads[index])}
	}
	for _, tc := range []struct {
		name      string
		stored    []store.AgentSessionTranscriptPart
		wantOrder []int64
	}{
		{name: "empty", wantOrder: []int64{0, 1, 5}},
		{name: "prefix", stored: []store.AgentSessionTranscriptPart{item(0)}, wantOrder: []int64{0, 1, 5}},
		{name: "complete", stored: []store.AgentSessionTranscriptPart{item(0), item(1), item(5)}, wantOrder: []int64{0, 1, 5}},
		// The live tee flushed the in-flight text as partial at a disconnect: the
		// harvest replaces it with the complete item under a fresh seq (a poller
		// past the partial still sees it) at the same position.
		{name: "partial_upgrade", stored: []store.AgentSessionTranscriptPart{item(0), partialText, item(5)}, wantOrder: []int64{0, 5, 1}},
		// Earlier concurrent batch writers could commit a later batch after an
		// earlier one failed. Recover the hole without renumbering issued cursors.
		{name: "historical_gap", stored: []store.AgentSessionTranscriptPart{item(0), item(5)}, wantOrder: []int64{0, 5, 1}},
		// A tee from before the item model stored raw chunks for this turn: the
		// harvest, which covers the whole turn, supersedes them.
		{name: "legacy_chunks", stored: []store.AgentSessionTranscriptPart{legacy(0), legacy(1), legacy(2)}, wantOrder: []int64{0, 1, 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, err := pg.CreateAgentSession(ctx, store.AgentSession{
				WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "bex-agent/s1",
				AgentConfig: []byte(`{"agent":"codex","task":"fix the tests"}`), InitialPrompt: "fix the tests",
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pg.RecordAgentSessionDispatch(ctx, session.ID, "sandbox-1", PhaseRunning, "running", "", session.Turns); err != nil {
				t.Fatal(err)
			}
			c, _, lifecycle, _ := completerFixture(succeededStatus(true), nil)
			c.Store = pg
			c.MaxTranscriptBytes = quota
			lifecycle.transcriptLog = driverLog.String()
			if err := pg.AppendAgentSessionTranscript(ctx, session.ID, tc.stored); err != nil {
				t.Fatal(err)
			}
			c.Reconcile(ctx)
			got, err := pg.AgentSessionTranscript(ctx, session.ID, -1, c.MaxTranscriptBytes, 0)
			if err != nil || len(got) != len(tc.wantOrder) {
				t.Fatalf("harvest replay=%+v err=%v, want %d items", got, err, len(tc.wantOrder))
			}
			for i, p := range got {
				if (i > 0 && p.Seq <= got[i-1].Seq) || p.PartIndex != tc.wantOrder[i] || p.Turn != 1 || !p.Item || p.Partial || string(p.Part) != items[tc.wantOrder[i]] {
					t.Fatalf("item %d: %+v", i, p)
				}
			}
			// Ordered replay is UI part order whatever the write order was.
			replay, err := pg.AgentSessionTranscriptReplay(ctx, session.ID, store.TranscriptStart, c.MaxTranscriptBytes, 0)
			if err != nil || len(replay) != 3 {
				t.Fatalf("ordered replay=%+v err=%v", replay, err)
			}
			for i, want := range []int64{0, 1, 5} {
				if replay[i].PartIndex != want || string(replay[i].Part) != items[want] {
					t.Fatalf("replay item %d: %+v, want part_index %d", i, replay[i], want)
				}
			}
			// Paging by position resumes exactly after the cursor.
			if rest, err := pg.AgentSessionTranscriptReplay(ctx, session.ID, replay[0].Position(), c.MaxTranscriptBytes, 1); err != nil || len(rest) != 1 || rest[0].PartIndex != 1 {
				t.Fatalf("replay page after %+v = %+v err=%v", replay[0].Position(), rest, err)
			}
			total, err := pg.AgentSessionTranscriptBytes(ctx, session.ID)
			if err != nil || total != quota {
				t.Fatalf("quota usage=%d err=%v want=%d", total, err, quota)
			}
			turns, err := pg.AgentSessionTurns(ctx, session.ID)
			if err != nil || len(turns) != 1 || !turns[0].TranscriptComplete || turns[0].TranscriptTruncated {
				t.Fatalf("turn completeness=%+v err=%v", turns, err)
			}
			row, err := pg.GetAgentSession(ctx, session.ID)
			if err != nil || row.Phase != PhaseCompleted || lifecycle.canceled != 1 || !lifecycle.readTranscriptBeforeCancel {
				t.Fatalf("completion=%+v err=%v canceled=%d harvestedBeforeTeardown=%v", row, err, lifecycle.canceled, lifecycle.readTranscriptBeforeCancel)
			}
		})
	}
}

// The recorded 10-minute production turn (5,602 chunks, w10/m16) harvested
// from a driver log stores a few hundred item rows that replay to the same
// UIMessage parts as the raw chunks, and never trips the item cap.
func TestCompleterPGHarvestsTheRecordedTurnAsItems(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "completer-recorded-turn", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pg.DeleteTenant(ctx, tenant.ID); err != nil {
			t.Error(err)
		}
	}()
	fixture, err := uimessagetest.LoadFixture()
	if err != nil {
		t.Fatal(err)
	}
	var driverLog strings.Builder
	var rawBytes int
	for i, chunk := range fixture {
		fmt.Fprintf(&driverLog, "{\"turn\":1,\"partIndex\":%d,\"part\":%s}\n", i, chunk)
		rawBytes += len(chunk)
	}
	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/bex", Branch: "bex-agent/s1",
		AgentConfig: []byte(`{"agent":"claude","task":"/rt-simplify-logic"}`), InitialPrompt: "/rt-simplify-logic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.RecordAgentSessionDispatch(ctx, session.ID, "sandbox-1", PhaseRunning, "running", "", session.Turns); err != nil {
		t.Fatal(err)
	}
	c, _, lifecycle, _ := completerFixture(succeededStatus(true), nil)
	c.Store = pg
	lifecycle.transcriptLog = driverLog.String()
	c.Reconcile(ctx)
	rows, err := pg.AgentSessionTranscript(ctx, session.ID, -1, store.MaxAgentSessionTranscriptBytes, 0)
	if err != nil {
		t.Fatal(err)
	}
	var stored int
	var replayed []json.RawMessage
	for _, row := range rows {
		stored += len(row.Part)
		replayed = append(replayed, agenttranscript.Expand(row.Part)...)
		if strings.Contains(string(row.Part), `"type":"reasoning-delta"`) || strings.Contains(string(row.Part), `"type":"data-acp-info"`) {
			t.Fatalf("raw chunk stored: %.120s", row.Part)
		}
	}
	t.Logf("harvest: %d chunks / %d bytes → %d rows / %d bytes", len(fixture), rawBytes, len(rows), stored)
	if len(rows) > 600 {
		t.Fatalf("stored %d rows, want ≤ 600", len(rows))
	}
	want, err := uimessagetest.Merge(fixture)
	if err != nil {
		t.Fatal(err)
	}
	got, err := uimessagetest.Merge(replayed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("replay rebuilt %d parts, raw %d", len(got), len(want))
	}
	for i := range want {
		if want[i]["state"] == "streaming" {
			got[i]["state"] = "streaming" // the recorded turn ends inside a reasoning block
		}
		if fmt.Sprint(want[i]) != fmt.Sprint(got[i]) {
			t.Fatalf("part %d differs", i)
		}
	}
	turns, err := pg.AgentSessionTurns(ctx, session.ID)
	if err != nil || len(turns) != 1 || turns[0].TruncationReason == reasonItemLimit {
		t.Fatalf("turn=%+v err=%v", turns, err)
	}
}

// TestCompleterPGFinalizesFromTheAgentPRAndNotesUndeliveredWork (ADR090): the
// Completer finalizes with the PR the agent's verb already recorded on the row,
// and a dirty tree completes with a delivery note — never failed, never a call
// to GitHub (the Completer holds no GitHub client at all).
func TestCompleterPGFinalizesFromTheAgentPRAndNotesUndeliveredWork(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "completer-agent-pr-test", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()
	running := func(t *testing.T) store.AgentSession {
		t.Helper()
		session, err := pg.CreateAgentSession(ctx, store.AgentSession{
			WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "bex-agent/s1",
			AgentConfig: []byte(`{"agent":"codex","task":"fix the tests"}`), InitialPrompt: "fix the tests",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pg.RecordAgentSessionDispatch(ctx, session.ID, "sandbox-1", PhaseRunning, "running", "", session.Turns); err != nil {
			t.Fatal(err)
		}
		return session
	}

	t.Run("agent-opened PR", func(t *testing.T) {
		session := running(t)
		if err := pg.RecordAgentSessionPullRequest(ctx, session.ID, "https://github.com/bex-co/example/pull/9", 9, remoteHead); err != nil {
			t.Fatal(err)
		}
		c, _, lifecycle, _ := completerFixture(succeededStatus(true), nil)
		c.Store = pg
		c.Reconcile(ctx)
		row, err := pg.GetAgentSession(ctx, session.ID)
		if err != nil || row.Phase != PhaseCompleted || row.PRURL != "https://github.com/bex-co/example/pull/9" || row.PRNumber != 9 || row.HeadSHA != remoteHead || row.DeliveryNote != "" {
			t.Fatalf("finalized row = %+v err=%v", row, err)
		}
		if lifecycle.canceled != 1 {
			t.Fatalf("sandbox not reclaimed: canceled=%d", lifecycle.canceled)
		}
	})
	t.Run("dirty tree", func(t *testing.T) {
		session := running(t)
		c, _, _, _ := completerFixture(gitStatus(statusGit{Dirty: true, UnpushedCommits: 1, Undelivered: []string{dirtyReason}}), nil)
		c.Store = pg
		c.Reconcile(ctx)
		row, err := pg.GetAgentSession(ctx, session.ID)
		if err != nil || row.Phase != PhaseCompleted || row.FailureReason != "" || row.PRURL != "" || row.DeliveryNote != dirtyReason {
			t.Fatalf("dirty row = %+v err=%v", row, err)
		}
	})
}

// TestIdleClosedSessionFollowUpPG (w10/m15) runs the idle-close path against the
// real store: a finished session keeps its sandbox through the 2h
// idle TTL, is then terminated with its transcript kept and its phase left
// completed, and a follow-up message redispatches a fresh sandbox on the same
// branch primed with the prior conversation (ADR047 D3) and the PR earlier
// turns opened (ADR090).
func TestIdleClosedSessionFollowUpPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "agent-idle-close-test", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()

	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "bex-agent/idle-close",
		AgentConfig: []byte(`{"agent":"codex","task":"fix the flaky test"}`), InitialPrompt: "fix the flaky test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.RecordAgentSessionDispatch(ctx, session.ID, "sandbox-idle-1", PhaseRunning, "running", "", session.Turns); err != nil {
		t.Fatal(err)
	}
	if err := pg.RecordAgentSessionPullRequest(ctx, session.ID, "https://github.com/bex-co/example/pull/11", 11, remoteHead); err != nil {
		t.Fatal(err)
	}

	c, _, lc, _ := completerFixture(succeededStatus(true), nil)
	c.Store, c.IdleTTL = pg, 2*time.Hour
	lc.transcriptLog = `{"turn":1,"partIndex":0,"part":{"type":"text-start","id":"t"}}
{"turn":1,"partIndex":1,"part":{"type":"text-delta","id":"t","delta":"Fixed the flaky test."}}
{"turn":1,"partIndex":2,"part":{"type":"text-end","id":"t"}}`
	start := time.Now()
	c.Now = func() time.Time { return start }
	c.Reconcile(ctx)
	row, err := pg.GetAgentSession(ctx, session.ID)
	if err != nil || row.Phase != PhaseCompleted || row.SandboxID != "sandbox-idle-1" || lc.canceled != 0 {
		t.Fatalf("finished turn inside the idle TTL = %+v canceled=%d err=%v", row, lc.canceled, err)
	}
	c.Now = func() time.Time { return start.Add(2*time.Hour + time.Minute) }
	c.Reconcile(ctx)
	row, err = pg.GetAgentSession(ctx, session.ID)
	if err != nil || row.Phase != PhaseCompleted || row.SandboxID != "" || lc.canceled != 1 {
		t.Fatalf("idle close = %+v canceled=%d err=%v, want completed with no sandbox", row, lc.canceled, err)
	}
	if parts, err := pg.AgentSessionTranscript(ctx, session.ID, -1, 1<<20, 0); err != nil || len(parts) != 1 {
		t.Fatalf("transcript after idle close = %+v err=%v", parts, err)
	}

	svc, _, _, slc := fixture()
	svc.Store = pg
	svc.Base.Authz = checkerFunc(func(context.Context, string, string, string) (bool, error) { return true, nil })
	view, err := svc.Steer(caller("alice"), SteerRequest{SessionID: session.ID, Prompt: "now add a regression test"})
	if err != nil {
		t.Fatalf("follow-up on the closed session: %v", err)
	}
	if view.Phase != PhaseRedispatching {
		t.Fatalf("follow-up accepted as %s, want redispatching", view.Phase)
	}
	row, err = pg.GetAgentSession(ctx, session.ID)
	if err != nil || row.Phase != PhaseRunning || row.SandboxID == "" || row.Turns != 2 || row.DeliveryMode != DeliveryRedispatch {
		t.Fatalf("follow-up row = %+v err=%v, want turn 2 running in a fresh sandbox", row, err)
	}
	if slc.branch != "bex-agent/idle-close" || slc.driverEnv["BEX_AGENT_PROMPT"] != "now add a regression test" {
		t.Fatalf("follow-up dispatch branch=%q prompt=%q", slc.branch, slc.driverEnv["BEX_AGENT_PROMPT"])
	}
	if !strings.Contains(slc.driverEnv[continuityContextEnv], "Fixed the flaky test.") ||
		!strings.Contains(slc.driverEnv[continuityContextEnv], "fix the flaky test") {
		t.Fatalf("continuity context = %q, want the prior prompt and reply", slc.driverEnv[continuityContextEnv])
	}
	if slc.driverEnv["BEX_AGENT_PR_URL"] != "https://github.com/bex-co/example/pull/11" || slc.driverEnv["BEX_AGENT_PR_NUMBER"] != "11" {
		t.Fatalf("follow-up PR env = %q/%q", slc.driverEnv["BEX_AGENT_PR_URL"], slc.driverEnv["BEX_AGENT_PR_NUMBER"])
	}
}
