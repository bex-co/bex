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
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transcriptQueryCounts struct {
	begins, commits, locks, cursors, inserts int
}

type transcriptQueryTracer struct {
	mu     sync.Mutex
	counts transcriptQueryCounts
}

func (tr *transcriptQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	sql := strings.TrimSpace(strings.ToLower(data.SQL))
	switch {
	case sql == "begin":
		tr.counts.begins++
	case sql == "commit":
		tr.counts.commits++
	case strings.Contains(sql, "pg_advisory_xact_lock"):
		tr.counts.locks++
	case strings.Contains(sql, "max(seq)"):
		tr.counts.cursors++
	case strings.HasPrefix(sql, "insert into agent_session_transcripts"):
		tr.counts.inserts++
	}
	return ctx
}

func (*transcriptQueryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (tr *transcriptQueryTracer) reset() transcriptQueryCounts {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	counts := tr.counts
	tr.counts = transcriptQueryCounts{}
	return counts
}

func transcriptBatchPGFixture(t *testing.T) (*PGStore, string, *transcriptQueryTracer) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		t.Fatal(err)
	}
	tracer := &transcriptQueryTracer{}
	config.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(context.Background(), "transcript-batch-test", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.DeleteTenant(context.Background(), tenant.ID); err != nil {
			t.Error(err)
		}
	})
	session, err := s.CreateAgentSession(context.Background(), AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"transcript batching"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	tracer.reset()
	return s, session.ID, tracer
}

func transcriptBatchParts() []AgentSessionTranscriptPart {
	payloads := []string{
		`{"type":"text-delta","id":"text","delta":"hello"}`,
		`{"type":"reasoning-delta","id":"reason","delta":"inspect"}`,
		`{"type":"tool-input-available","toolCallId":"tool","toolName":"read","input":{}}`,
		`{"type":"data-acp","data":{"kind":"plan","steps":["test"]}}`,
		`{"type":"data-acp","data":{"kind":"diff","path":"main.go"}}`,
	}
	parts := make([]AgentSessionTranscriptPart, 96)
	for i := range parts {
		parts[i] = AgentSessionTranscriptPart{Turn: 1, PartIndex: int64(i), Part: []byte(payloads[i%len(payloads)])}
	}
	parts[0].Part = []byte(`{"type":"start"}`)
	parts[len(parts)-1].Part = []byte(`{"type":"finish"}`)
	return parts
}

func assertBatchTranscript(t *testing.T, s *PGStore, sessionID string, want []AgentSessionTranscriptPart) {
	t.Helper()
	got, err := s.AgentSessionTranscript(context.Background(), sessionID, -1, MaxAgentSessionTranscriptBytes, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("durable parts=%d, want %d", len(got), len(want))
	}
	var wantBytes int64
	for i, p := range got {
		if p.Seq != int64(i) || p.PartIndex != want[i].PartIndex || p.Turn != want[i].Turn || string(p.Part) != string(want[i].Part) {
			t.Fatalf("part %d: got %+v, want %+v with cursor %d", i, p, want[i], i)
		}
		wantBytes += int64(len(want[i].Part))
	}
	if total, err := s.AgentSessionTranscriptBytes(context.Background(), sessionID); err != nil || total != wantBytes {
		t.Fatalf("durable bytes=%d err=%v, want %d (duplicates must not consume quota)", total, err, wantBytes)
	}
	for _, turn := range []int{1, 2, 3} {
		requested := []int64{0, 1, 95, 9999}
		wantExisting := make(map[int64]bool)
		for _, p := range want {
			if p.Turn == turn {
				for _, index := range requested {
					if index == p.PartIndex {
						wantExisting[index] = true
					}
				}
			}
		}
		total, existing, err := s.AgentSessionTranscriptProgress(context.Background(), sessionID, turn, requested)
		if err != nil || total != wantBytes || len(existing) != len(wantExisting) {
			t.Fatalf("turn %d progress=(%d,%v) err=%v, want (%d,%v)", turn, total, existing, err, wantBytes, wantExisting)
		}
		for _, index := range existing {
			if !wantExisting[index] {
				t.Fatalf("unexpected or duplicate ordinal %d", index)
			}
			delete(wantExisting, index)
		}
	}

}

// Trace actual database operations, rather than counting calls on a fake store.
// Batch size one reproduces the pre-batching gateway's per-part persistence.
func TestPGTranscriptBatchOperationCounts(t *testing.T) {
	for _, size := range []int{1, 32} {
		t.Run(fmt.Sprintf("batch_%d", size), func(t *testing.T) {
			s, sessionID, tracer := transcriptBatchPGFixture(t)
			parts := transcriptBatchParts()
			start := time.Now()
			for offset := 0; offset < len(parts); offset += size {
				if err := s.AppendAgentSessionTranscript(context.Background(), sessionID, parts[offset:min(offset+size, len(parts))]); err != nil {
					t.Fatal(err)
				}
			}
			elapsed := time.Since(start)
			counts := tracer.reset()
			batches := (len(parts) + size - 1) / size
			if counts != (transcriptQueryCounts{begins: batches, commits: batches, locks: batches, cursors: batches, inserts: len(parts)}) {
				t.Fatalf("query counts=%+v for %d parts batched by %d", counts, len(parts), size)
			}
			t.Logf("parts=%d batch=%d transactions=%d advisory_locks=%d cursor_queries=%d inserts=%d elapsed=%s", len(parts), size, counts.begins, counts.locks, counts.cursors, counts.inserts, elapsed)
			assertBatchTranscript(t, s, sessionID, parts)
			tracer.reset()
			if err := s.AppendAgentSessionTranscript(context.Background(), sessionID, nil); err != nil {
				t.Fatal(err)
			}
			if counts := tracer.reset(); counts != (transcriptQueryCounts{}) {
				t.Fatalf("empty append issued queries: %+v", counts)
			}
		})
	}
}

func TestPGTranscriptConcurrentBatchesRecoverDisconnectedPrefix(t *testing.T) {
	s, sessionID, _ := transcriptBatchPGFixture(t)
	ctx := context.Background()
	parts := transcriptBatchParts()
	// A disconnect left only a prefix durable. Two gateways replay from zero
	// while the completion worker harvests the entire driver log in one call.
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, parts[:17]); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 3)
	for _, size := range []int{7, 32, len(parts)} {
		go func() {
			<-start
			for offset := 0; offset < len(parts); offset += size {
				if err := s.AppendAgentSessionTranscript(ctx, sessionID, parts[offset:min(offset+size, len(parts))]); err != nil {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}
	close(start)
	for range 3 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	assertBatchTranscript(t, s, sessionID, parts)
	budget := int64(len(parts[0].Part) + len(parts[1].Part))
	prefix, err := s.AgentSessionTranscript(ctx, sessionID, -1, budget, 0)
	if err != nil || len(prefix) != 2 || prefix[1].PartIndex != 1 {
		t.Fatalf("quota-bounded replay=%+v err=%v", prefix, err)
	}
	// The next turn restarts its local index but keeps the durable cursor.
	next := AgentSessionTranscriptPart{Turn: 2, PartIndex: 0, Part: []byte(`{"type":"start"}`)}
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, []AgentSessionTranscriptPart{next}); err != nil {
		t.Fatal(err)
	}
	assertBatchTranscript(t, s, sessionID, append(parts, next))
}

func TestPGTranscriptFailedBatchRollsBackAndRetries(t *testing.T) {
	s, sessionID, _ := transcriptBatchPGFixture(t)
	ctx := context.Background()
	parts := transcriptBatchParts()[:3]
	invalid := append([]AgentSessionTranscriptPart(nil), parts...)
	// Postgres text rejects NUL: fail the second insert after the first succeeded.
	invalid[1].Part = []byte{0}
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, invalid); err == nil {
		t.Fatal("invalid text batch unexpectedly succeeded")
	}
	assertBatchTranscript(t, s, sessionID, nil)
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, parts); err != nil {
		t.Fatal(err)
	}
	assertBatchTranscript(t, s, sessionID, parts)
}
