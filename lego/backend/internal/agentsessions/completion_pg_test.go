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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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
	for _, tc := range []struct {
		name      string
		stored    []int
		wantOrder []int
	}{
		{name: "empty", wantOrder: []int{0, 1, 2}},
		{name: "prefix", stored: []int{0}, wantOrder: []int{0, 1, 2}},
		{name: "complete", stored: []int{0, 1, 2}, wantOrder: []int{0, 1, 2}},
		// Earlier concurrent batch writers could commit a later batch after an
		// earlier one failed. Recover the hole without renumbering issued cursors.
		{name: "historical_gap", stored: []int{0, 2}, wantOrder: []int{0, 2, 1}},
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
			payloads := []string{`{"type":"start"}`, `{"type":"text-delta","id":"text","delta":"recovered"}`, `{"type":"finish"}`}
			parts := make([]store.AgentSessionTranscriptPart, len(payloads))
			var log strings.Builder
			for i, payload := range payloads {
				parts[i] = store.AgentSessionTranscriptPart{Turn: 1, PartIndex: int64(i), Part: []byte(payload)}
				fmt.Fprintf(&log, "{\"turn\":1,\"partIndex\":%d,\"part\":%s}\n", i, payload)
				c.MaxTranscriptBytes += int64(len(payload))
			}
			lifecycle.transcriptLog = log.String()
			var persisted []store.AgentSessionTranscriptPart
			for _, index := range tc.stored {
				persisted = append(persisted, parts[index])
			}
			if err := pg.AppendAgentSessionTranscript(ctx, session.ID, persisted); err != nil {
				t.Fatal(err)
			}
			c.Reconcile(ctx)
			got, err := pg.AgentSessionTranscript(ctx, session.ID, -1, c.MaxTranscriptBytes, 0)
			if err != nil || len(got) != len(parts) {
				t.Fatalf("harvest replay=%+v err=%v, want all %d parts", got, err, len(parts))
			}
			for i, p := range got {
				if p.Seq != int64(i) || p.PartIndex != int64(tc.wantOrder[i]) || p.Turn != 1 || string(p.Part) != payloads[tc.wantOrder[i]] {
					t.Fatalf("part %d: %+v", i, p)
				}
			}
			total, err := pg.AgentSessionTranscriptBytes(ctx, session.ID)
			if err != nil || total != c.MaxTranscriptBytes {
				t.Fatalf("quota usage=%d err=%v want=%d", total, err, c.MaxTranscriptBytes)
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
