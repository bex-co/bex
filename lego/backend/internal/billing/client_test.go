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

package billing

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubTransport is a programmable http.RoundTripper: respond decides each call's
// status by 0-based call index, so a test can script "429 then 200" or "always
// 500". It records the path of every call for batching/routing assertions.
type stubTransport struct {
	mu      sync.Mutex
	paths   []string
	respond func(callIdx int, req *http.Request) (status int, body string)
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	idx := len(s.paths)
	s.paths = append(s.paths, req.URL.Path)
	s.mu.Unlock()
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}
	status, body := s.respond(idx, req)
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (s *stubTransport) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.paths)
}

func (s *stubTransport) ingestCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, p := range s.paths {
		if strings.Contains(p, "ingest") {
			n++
		}
	}
	return n
}

// newTestClient builds a Client wired to stub over an instant (no-wait) backoff
// so retry classification is exercised without real sleeps.
func newTestClient(t *testing.T, stub *stubTransport) *Client {
	t.Helper()
	c := New(Config{Token: "test-token", HTTPClient: &http.Client{Transport: stub}})
	if c == nil {
		t.Fatal("New returned nil for a non-empty token")
	}
	c.backoff = []time.Duration{0, 0, 0} // 3 retries, no delay
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

func makeEvents(n int) []Event {
	events := make([]Event, n)
	for i := range events {
		events[i] = Event{
			TransactionID: "tx" + strings.Repeat("0", 3),
			CustomerID:    "tea-abc",
			EventType:     "instance_seconds",
			Timestamp:     time.Unix(int64(i)*3600, 0).UTC(),
			Properties:    map[string]string{"value": "1"},
		}
	}
	return events
}

func TestNewDisabledWhenTokenUnset(t *testing.T) {
	if c := New(Config{Token: ""}); c != nil {
		t.Fatalf("New with empty token = %v, want nil (feature disabled)", c)
	}
	if c := New(Config{Token: "x"}); c == nil {
		t.Fatal("New with a token returned nil")
	}
}

func TestIngestBatchChunksTo100(t *testing.T) {
	stub := &stubTransport{respond: func(int, *http.Request) (int, string) { return 200, "" }}
	c := newTestClient(t, stub)
	if err := c.IngestBatch(context.Background(), makeEvents(250)); err != nil {
		t.Fatalf("IngestBatch: %v", err)
	}
	// 250 events → 100 + 100 + 50 = 3 ingest calls.
	if got := stub.ingestCalls(); got != 3 {
		t.Fatalf("ingest calls = %d, want 3 (≤100 per call)", got)
	}
}

func TestIngestBatchRetriesThenSucceeds(t *testing.T) {
	// 429, then 200: one retry, then success.
	stub := &stubTransport{respond: func(idx int, _ *http.Request) (int, string) {
		if idx == 0 {
			return http.StatusTooManyRequests, `{"message":"slow down"}`
		}
		return 200, ""
	}}
	c := newTestClient(t, stub)
	if err := c.IngestBatch(context.Background(), makeEvents(1)); err != nil {
		t.Fatalf("IngestBatch after a 429 retry: %v", err)
	}
	if got := stub.count(); got != 2 {
		t.Fatalf("calls = %d, want 2 (429 retried once)", got)
	}
}

func TestIngestBatch5xxExhaustsAndErrors(t *testing.T) {
	stub := &stubTransport{respond: func(int, *http.Request) (int, string) {
		return http.StatusInternalServerError, `{"message":"boom"}`
	}}
	c := newTestClient(t, stub) // backoff len 3 → 4 attempts total
	err := c.IngestBatch(context.Background(), makeEvents(1))
	if err == nil {
		t.Fatal("IngestBatch on persistent 5xx = nil, want error")
	}
	if got := stub.count(); got != 4 {
		t.Fatalf("calls = %d, want 4 (initial + 3 retries)", got)
	}
}

func TestIngestBatch4xxDeadLettersWithoutRetry(t *testing.T) {
	stub := &stubTransport{respond: func(int, *http.Request) (int, string) {
		return http.StatusBadRequest, `{"message":"bad event"}`
	}}
	c := newTestClient(t, stub)
	// A non-429 4xx is dead-lettered (logged, dropped), so IngestBatch returns
	// nil and does NOT retry.
	if err := c.IngestBatch(context.Background(), makeEvents(1)); err != nil {
		t.Fatalf("IngestBatch on 4xx = %v, want nil (dead-lettered)", err)
	}
	if got := stub.count(); got != 1 {
		t.Fatalf("calls = %d, want 1 (4xx not retried)", got)
	}
}

func TestIngestBatch4xxDoesNotBlockLaterChunks(t *testing.T) {
	// Three chunks (250 events): the middle one 4xx-dead-letters; the loop must
	// still attempt the third chunk.
	stub := &stubTransport{respond: func(idx int, _ *http.Request) (int, string) {
		if idx == 1 {
			return http.StatusUnprocessableEntity, `{"message":"nope"}`
		}
		return 200, ""
	}}
	c := newTestClient(t, stub)
	if err := c.IngestBatch(context.Background(), makeEvents(250)); err != nil {
		t.Fatalf("IngestBatch = %v, want nil (4xx dead-lettered, loop continues)", err)
	}
	if got := stub.ingestCalls(); got != 3 {
		t.Fatalf("ingest calls = %d, want 3 (all chunks attempted despite a middle 4xx)", got)
	}
}

func TestEnsureCustomerIdempotentAcrossCalls(t *testing.T) {
	stub := &stubTransport{respond: func(int, *http.Request) (int, string) {
		return 200, `{"data":{"id":"cust_1"}}`
	}}
	c := newTestClient(t, stub)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := c.EnsureCustomer(ctx, "tea-abc"); err != nil {
			t.Fatalf("EnsureCustomer call %d: %v", i, err)
		}
	}
	// Cached after the first success — only one network create.
	if got := stub.count(); got != 1 {
		t.Fatalf("customer create calls = %d, want 1 (process-cached)", got)
	}
}

func TestEnsureCustomerConflictTreatedAsSuccess(t *testing.T) {
	stub := &stubTransport{respond: func(int, *http.Request) (int, string) {
		return http.StatusConflict, `{"message":"alias in use"}`
	}}
	c := newTestClient(t, stub)
	if err := c.EnsureCustomer(context.Background(), "tea-abc"); err != nil {
		t.Fatalf("EnsureCustomer on 409 = %v, want nil (already exists)", err)
	}
}
