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

package modelproxy

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

// The tee sees the stream in whatever chunks the upstream produced; usage
// must come out the same when every byte arrives on its own.
func TestUsageMeterSurvivesByteAtATimeSSE(t *testing.T) {
	stream := `data: {"type":"message_start","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":4}}}` + "\r\n\r\n" +
		`data: {"type":"message_delta","usage":{"output_tokens":7,"cache_creation_input_tokens":3}}` + "\n\n"
	m := newUsageMeter("text/event-stream; charset=utf-8")
	for i := range len(stream) {
		if n, err := m.Write([]byte{stream[i]}); n != 1 || err != nil {
			t.Fatalf("Write = %d, %v", n, err)
		}
	}
	tokens, ok, _ := m.result()
	if !ok || tokens != [4]int64{10, 7, 4, 3} {
		t.Fatalf("tokens = %v ok=%v", tokens, ok)
	}
}

func TestUsageMeterSkipsOverlongLinesAndReportsLoss(t *testing.T) {
	m := newUsageMeter("text/event-stream")
	huge := "data: " + strings.Repeat("x", maxMeteredSSELine+10) + "\n"
	if n, _ := m.Write([]byte(huge)); n != len(huge) {
		t.Fatalf("Write reported %d of %d bytes", n, len(huge))
	}
	_, _ = m.Write([]byte(`data: {"type":"message_delta","usage":{"output_tokens":2}}` + "\n"))
	if tokens, ok, _ := m.result(); !ok || tokens[1] != 2 {
		t.Fatalf("usage after an overlong line = %v ok=%v", tokens, ok)
	}
	if _, ok, unparsed := newUsageMeter("text/event-stream").result(); ok || !unparsed {
		t.Fatal("a stream with no usage must count as an unparsed loss")
	}
	j := newUsageMeter("application/json")
	_, _ = j.Write([]byte(strings.Repeat(" ", maxMeteredJSONBytes+1)))
	if _, ok, unparsed := j.result(); ok || !unparsed {
		t.Fatal("an oversized JSON body must count as an unparsed loss")
	}
}

type flakyUsageAPI struct {
	mu       sync.Mutex
	failures int
	err      error
	calls    []agentsession.ModelUsageReport
}

func (f *flakyUsageAPI) ReportUsage(_ context.Context, rep agentsession.ModelUsageReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, rep)
	if len(f.calls) <= f.failures {
		return f.err
	}
	return nil
}

func (f *flakyUsageAPI) snapshot() []agentsession.ModelUsageReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]agentsession.ModelUsageReport(nil), f.calls...)
}

type notes struct {
	mu  sync.Mutex
	got []string
}

func (n *notes) note(result string) { n.mu.Lock(); n.got = append(n.got, result); n.mu.Unlock() }
func (n *notes) has(result string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, g := range n.got {
		if g == result {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A transient bex-api failure is retried under the SAME request id (so the
// ledger records it once); a refusal is dropped without retrying.
func TestUsageReporterRetriesUnderOneRequestID(t *testing.T) {
	api := &flakyUsageAPI{failures: 2, err: errors.New("agent credential broker unavailable")}
	n := &notes{}
	r := newUsageReporter(api, n.note, time.Millisecond)
	r.enqueue(agentsession.ModelUsageReport{RequestID: "amr-x", SessionID: "ags-one"})
	waitFor(t, func() bool { return n.has("reported") })
	calls := api.snapshot()
	if len(calls) != 3 || calls[0].RequestID != "amr-x" || calls[2].RequestID != "amr-x" || !n.has("retried") {
		t.Fatalf("calls = %+v notes = %v", calls, n.got)
	}

	refused := &flakyUsageAPI{failures: usageReportAttempts, err: agentsession.ErrForbidden}
	n2 := &notes{}
	r2 := newUsageReporter(refused, n2.note, time.Millisecond)
	r2.enqueue(agentsession.ModelUsageReport{RequestID: "amr-y"})
	waitFor(t, func() bool { return n2.has("dropped_failed") })
	if got := len(refused.snapshot()); got != 1 {
		t.Fatalf("a refusal was retried %d times", got)
	}
}

func TestUsageReporterDropsWhenTheQueueIsFull(t *testing.T) {
	n := &notes{}
	r := &usageReporter{note: n.note, queue: make(chan agentsession.ModelUsageReport)} // no worker, no buffer
	r.enqueue(agentsession.ModelUsageReport{RequestID: "amr-z"})
	if !n.has("dropped_queue_full") {
		t.Fatal("a full queue must drop loudly, never block the relay")
	}
}
