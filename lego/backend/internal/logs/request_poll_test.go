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

package logs

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRequestPollStreamsNewLinesOnce (w4/m237): the store poll emits request
// lines as they reach the store — each exactly once, including siblings
// sharing one timestamp — and forwards the subscription's structured
// filters to the store query, so streamed lines match the history beside
// them. Later polls over the same store re-emit nothing.
func TestRequestPollStreamsNewLinesOnce(t *testing.T) {
	var mu sync.Mutex
	stored := []LogEntry{
		{Timestamp: "2026-07-05T00:00:01Z", Message: "GET /a 200", Labels: map[string]string{LabelInstance: "web-1"}},
		{Timestamp: "2026-07-05T00:00:02Z", Message: "GET /b 500", Labels: map[string]string{LabelInstance: "web-1"}},
		{Timestamp: "2026-07-05T00:00:02Z", Message: "POST /b 500", Labels: map[string]string{LabelInstance: "web-1"}},
	}
	var pollQueries []LogQuery
	svc := newService(nil, sampleApp("web"), podFor("web", "web-1"))
	svc.RequestPollInterval = 5 * time.Millisecond
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		mu.Lock()
		defer mu.Unlock()
		pollQueries = append(pollQueries, q)
		// Emulate the store: it applies the forwarded filters server-side and
		// builds fresh entries per query (labels are not shared with the
		// stored set — the drain rewrites instance labels in place).
		var out []LogEntry
		for _, e := range stored {
			if len(q.StatusCode) > 0 && !strings.HasSuffix(e.Message, " "+q.StatusCode[0]) {
				continue
			}
			labels := make(map[string]string, len(e.Labels))
			for k, v := range e.Labels {
				labels[k] = v
			}
			out = append(out, LogEntry{Timestamp: e.Timestamp, Message: e.Message, Labels: labels})
		}
		return out, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var emitted []LogEntry
	done := make(chan error, 1)
	go func() {
		done <- svc.FollowLogs(ctx, LogQuery{
			App: "web", Types: []string{LogTypeRequest},
			StatusCode: []string{"500"},
		}, func(e LogEntry) error {
			mu.Lock()
			emitted = append(emitted, e)
			mu.Unlock()
			return nil
		})
	}()

	waitFor := func(want int) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			n := len(emitted)
			mu.Unlock()
			if n >= want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("emitted %d lines, want %d: %v", len(emitted), want, emitted)
	}

	waitFor(2) // both :02 500 siblings; the :01 200 stays in the store

	mu.Lock()
	stored = append(stored,
		LogEntry{Timestamp: "2026-07-05T00:00:03Z", Message: "GET /c 500", Labels: map[string]string{LabelInstance: "web-1"}},
		LogEntry{Timestamp: "2026-07-05T00:00:04Z", Message: "GET /d 200", Labels: map[string]string{LabelInstance: "web-1"}},
	)
	mu.Unlock()

	waitFor(3)
	cancel()
	if err := <-done; err != nil && err != context.Canceled {
		t.Fatalf("FollowLogs: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"GET /b 500", "POST /b 500", "GET /c 500"}
	if len(emitted) != len(want) {
		t.Fatalf("emitted %d lines, want %d exactly once each: %v", len(emitted), len(want), emitted)
	}
	for i, w := range want {
		if emitted[i].Message != w {
			t.Errorf("emitted[%d] = %q, want %q", i, emitted[i].Message, w)
		}
	}
	if len(pollQueries) == 0 {
		t.Fatal("the store was never polled")
	}
	for _, q := range pollQueries {
		if len(q.Types) != 1 || q.Types[0] != LogTypeRequest {
			t.Errorf("poll query types = %v, want [request]", q.Types)
		}
		if len(q.StatusCode) != 1 || q.StatusCode[0] != "500" {
			t.Errorf("poll query statusCode = %v, want the subscription's filter forwarded", q.StatusCode)
		}
		if q.Direction != DirectionForward {
			t.Errorf("poll query direction = %q, want forward (drain oldest-first)", q.Direction)
		}
	}
}
