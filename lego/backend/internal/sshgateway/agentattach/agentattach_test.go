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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestReadSSEDataBounded pins w1/m65 F10: a single SSE part larger than the cap
// is refused with errPartTooLarge (bounded read) instead of being buffered
// without limit by the old ReadString('\n'), while a normal part and the [DONE]
// sentinel still parse.
func TestReadSSEDataBounded(t *testing.T) {
	// A normal part parses verbatim.
	r := bufio.NewReader(strings.NewReader("data: {\"ok\":true}\n\n"))
	payload, done, err := readSSEData(r, maxSSEPartBytes)
	if err != nil || done || payload != `{"ok":true}` {
		t.Fatalf("normal part = %q,%v,%v", payload, done, err)
	}
	// A part far larger than the supplied cap is refused with a bounded read.
	big := "data: " + strings.Repeat("A", 8192) + "\n\n"
	r = bufio.NewReader(strings.NewReader(big))
	if _, _, err := readSSEData(r, 512); !errors.Is(err, errPartTooLarge) {
		t.Fatalf("oversized part err = %v, want errPartTooLarge", err)
	}
	// The [DONE] sentinel is recognized.
	r = bufio.NewReader(strings.NewReader("data: [DONE]\n\n"))
	if _, isDone, err := readSSEData(r, maxSSEPartBytes); err != nil || !isDone {
		t.Fatalf("done sentinel = %v,%v", isDone, err)
	}
}

// fakeAttachStore is an in-memory transcript Store (ordered parts keyed by
// (session, seq) with idempotent append) plus a single-use nonce claim, so it
// also backs the test's NonceGuard.
type fakeAttachStore struct {
	mu     sync.Mutex
	parts  map[string]map[int64]store.AgentSessionTranscriptPart
	claims map[string]bool
}

func newFakeAttachStore() *fakeAttachStore {
	return &fakeAttachStore{parts: map[string]map[int64]store.AgentSessionTranscriptPart{}, claims: map[string]bool{}}
}

func (f *fakeAttachStore) AppendAgentSessionTranscript(_ context.Context, id string, parts []store.AgentSessionTranscriptPart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.parts[id] == nil {
		f.parts[id] = map[int64]store.AgentSessionTranscriptPart{}
	}
	for _, p := range parts {
		if _, exists := f.parts[id][p.Seq]; exists {
			continue // idempotent on the driver-ordinal key
		}
		f.parts[id][p.Seq] = p
	}
	return nil
}

func (f *fakeAttachStore) AgentSessionTranscript(_ context.Context, id string, afterSeq int64, maxBytes int64) ([]store.AgentSessionTranscriptPart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.AgentSessionTranscriptPart, 0)
	for seq, p := range f.parts[id] {
		if seq > afterSeq {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	// Honor the store's bounded-read contract: never return past the budget.
	var total int64
	for i, p := range out {
		if total+int64(len(p.Part)) > maxBytes {
			return out[:i], nil
		}
		total += int64(len(p.Part))
	}
	return out, nil
}

func (f *fakeAttachStore) AgentSessionTranscriptBytes(_ context.Context, id string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var total int64
	for _, p := range f.parts[id] {
		total += int64(len(p.Part))
	}
	return total, nil
}

func (f *fakeAttachStore) AgentSessionTranscriptMaxSeq(_ context.Context, id string) (int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	max, ok := int64(-1), false
	for seq := range f.parts[id] {
		if !ok || seq > max {
			max, ok = seq, true
		}
	}
	return max, ok, nil
}

func (f *fakeAttachStore) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claims[nonce] {
		return false, nil
	}
	f.claims[nonce] = true
	return true, nil
}

type fixedPodIP struct {
	ip  string
	err error
}

func (r fixedPodIP) PodIP(context.Context, string, string) (string, error) { return r.ip, r.err }

// fakeDriver serves the driver's SSE `/stream`: it replays the given history
// parts (each a `data: <json>` line) then the `[DONE]` sentinel, exactly like
// the in-sandbox UIMessageStreamHub on a closed stream.
func fakeDriver(history []string) (*httptest.Server, string, int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-vercel-ai-ui-message-stream", "v1")
		fl := w.(http.Flusher)
		for _, part := range history {
			_, _ = io.WriteString(w, "data: "+part+"\n\n")
			fl.Flush()
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		fl.Flush()
	})
	srv := httptest.NewServer(mux)
	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	return srv, host, port
}

func newAttachGateway(st *fakeAttachStore, pods PodIPResolver, secret []byte, port int) *Server {
	return &Server{
		Secret: secret, Store: st, Pods: pods, DriverPort: port,
		AllowedOrigins: []string{"https://dashboard.bex.co"},
		Now:            func() time.Time { return time.Unix(1_800_000_000, 0) },
		Metrics:        sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:         sshgateway.NewSessionLimiter(100, 5),
		Nonces:         &sshgateway.NonceGuard{Store: st},
		SessionTimeout: time.Minute,
	}
}

func attachTicket(t *testing.T, secret []byte, session string) string {
	t.Helper()
	tok, err := agentsessionticket.Mint(secret, agentsessionticket.Claims{
		Subject: "alice", SessionID: session, SandboxID: "sandbox-1", Pod: "sandbox-1-0",
		Workspace: "tea-a", Namespace: "tea-a-sandbox", Action: agentsessionticket.ActionRead,
		IssuedAt: 1_800_000_000, ExpiresAt: 1_800_000_090,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func turnTicket(t *testing.T, secret []byte, session string) string {
	t.Helper()
	tok, err := agentsessionticket.Mint(secret, agentsessionticket.Claims{
		Subject: "alice", SessionID: session, SandboxID: "sandbox-1", Pod: "sandbox-1-0",
		Workspace: "tea-a", Namespace: "tea-a-sandbox", Action: agentsessionticket.ActionTurn,
		IssuedAt: 1_800_000_000, ExpiresAt: 1_800_000_090,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// dataPayloads extracts the `data:` payloads (excluding the [DONE] sentinel)
// from an SSE response body, preserving order.
func dataPayloads(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if payload, ok := strings.CutPrefix(line, "data: "); ok && payload != "[DONE]" {
			out = append(out, payload)
		}
	}
	return out
}

func TestAgentAttachReplaysThenSplicesLiveAndTees(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000001"
	st := newFakeAttachStore()
	// Pre-seed the transcript with the first part, as if an earlier attach had
	// already teed it. The driver still replays its full history.
	_ = st.AppendAgentSessionTranscript(context.Background(), session, []store.AgentSessionTranscriptPart{
		{Seq: 0, Part: []byte(`{"type":"start"}`)},
	})
	driver, host, port := fakeDriver([]string{`{"type":"start"}`, `{"type":"text-delta","delta":"hi"}`})
	defer driver.Close()

	gw := newAttachGateway(st, fixedPodIP{ip: host}, secret, port)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(TicketHeader, attachTicket(t, secret, session))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("x-vercel-ai-ui-message-stream") != "v1" {
		t.Fatalf("missing v1 marker: %v", resp.Header)
	}
	body, _ := io.ReadAll(resp.Body)
	got := string(body)

	// The client sees each part exactly once (replay of stored seq 0, then live
	// seq 1 spliced after it) and terminates with [DONE].
	payloads := dataPayloads(got)
	want := []string{`{"type":"start"}`, `{"type":"text-delta","delta":"hi"}`}
	if fmt.Sprint(payloads) != fmt.Sprint(want) {
		t.Fatalf("client parts = %v, want %v (no dup of the replayed part)", payloads, want)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "data: [DONE]") {
		t.Fatalf("stream did not terminate with [DONE]:\n%s", got)
	}
	// Verbatim: the exact framing the client received matches what the driver emits.
	if !strings.Contains(got, "data: {\"type\":\"text-delta\",\"delta\":\"hi\"}\n\n") {
		t.Fatalf("live part not forwarded verbatim:\n%q", got)
	}
	// The tee persisted the live part (seq 1) idempotently alongside the seed.
	stored, _ := st.AgentSessionTranscript(context.Background(), session, -1, 1<<30)
	if len(stored) != 2 || stored[1].Seq != 1 || string(stored[1].Part) != `{"type":"text-delta","delta":"hi"}` {
		t.Fatalf("tee = %+v, want seed + live seq 1", stored)
	}
}

func TestAgentAttachTerminalSessionReplaysThenDone(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000002"
	st := newFakeAttachStore()
	_ = st.AppendAgentSessionTranscript(context.Background(), session, []store.AgentSessionTranscriptPart{
		{Seq: 0, Part: []byte(`{"type":"start"}`)},
		{Seq: 1, Part: []byte(`{"type":"finish"}`)},
	})
	// No live pod: the terminal-session history path.
	gw := newAttachGateway(st, fixedPodIP{err: fmt.Errorf("pod gone")}, secret, 8787)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(TicketHeader, attachTicket(t, secret, session))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	payloads := dataPayloads(string(body))
	if fmt.Sprint(payloads) != fmt.Sprint([]string{`{"type":"start"}`, `{"type":"finish"}`}) {
		t.Fatalf("terminal replay = %v", payloads)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(body)), "data: [DONE]") {
		t.Fatalf("terminal stream did not end with [DONE]:\n%s", body)
	}
}

func TestAgentAttachRejectsBadTicketReplayAndMismatch(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000003"
	st := newFakeAttachStore()
	driver, host, port := fakeDriver(nil)
	defer driver.Close()
	gw := newAttachGateway(st, fixedPodIP{ip: host}, secret, port)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	do := func(ticket string) int {
		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		if ticket != "" {
			req.Header.Set(TicketHeader, ticket)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := do(""); code != http.StatusUnauthorized {
		t.Errorf("missing ticket = %d, want 401", code)
	}
	if code := do("garbage.sig"); code != http.StatusUnauthorized {
		t.Errorf("bad ticket = %d, want 401", code)
	}
	// Wrong secret is a signature failure.
	wrong := attachTicket(t, []byte("other-secret"), session)
	if code := do(wrong); code != http.StatusUnauthorized {
		t.Errorf("wrong-secret ticket = %d, want 401", code)
	}
	// Valid ticket works once; a replay of the same nonce is rejected.
	tok := attachTicket(t, secret, session)
	if code := do(tok); code != http.StatusOK {
		t.Errorf("first use = %d, want 200", code)
	}
	if code := do(tok); code != http.StatusUnauthorized {
		t.Errorf("replay = %d, want 401", code)
	}
}

// TestAgentAttachCORS pins the cross-origin contract the dashboard depends on
// (dashboard.bex.co -> api.bex.co): the OPTIONS preflight is answered 204 with
// the echoed origin + allowed ticket header, and the actual stream response
// exposes the v1 marker. Without this the browser blocks the stream even though
// curl works — the gap a live prod probe caught.
func TestAgentAttachCORS(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000009"
	st := newFakeAttachStore()
	_ = st.AppendAgentSessionTranscript(context.Background(), session, []store.AgentSessionTranscriptPart{
		{Seq: 0, Part: []byte(`{"type":"start"}`)},
	})
	gw := newAttachGateway(st, fixedPodIP{err: fmt.Errorf("terminal")}, secret, 8787)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	origin := "https://dashboard.bex.co"

	// Preflight: 204, echoed origin, ticket header allowed, credentials allowed.
	pre, _ := http.NewRequest(http.MethodOptions, srv.URL, nil)
	pre.Header.Set("Origin", origin)
	pre.Header.Set("Access-Control-Request-Method", "GET")
	pre.Header.Set("Access-Control-Request-Headers", TicketHeader)
	presp, err := http.DefaultClient.Do(pre)
	if err != nil {
		t.Fatal(err)
	}
	defer presp.Body.Close()
	if presp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", presp.StatusCode)
	}
	if presp.Header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("preflight allow-origin = %q, want %q", presp.Header.Get("Access-Control-Allow-Origin"), origin)
	}
	if !strings.Contains(presp.Header.Get("Access-Control-Allow-Headers"), TicketHeader) {
		t.Fatalf("preflight does not allow the ticket header: %q", presp.Header.Get("Access-Control-Allow-Headers"))
	}
	if presp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("preflight missing allow-credentials")
	}

	// Actual GET: echoed origin + the v1 marker exposed to the reader.
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Origin", origin)
	req.Header.Set(TicketHeader, attachTicket(t, secret, session))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("stream allow-origin = %q, want %q", resp.Header.Get("Access-Control-Allow-Origin"), origin)
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Expose-Headers"), uiMessageStreamHeader) {
		t.Fatalf("stream does not expose the v1 marker: %q", resp.Header.Get("Access-Control-Expose-Headers"))
	}

	// A disallowed origin gets no allow-origin header (fails closed in the browser).
	bad, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	bad.Header.Set("Origin", "https://evil.example.com")
	bad.Header.Set(TicketHeader, attachTicket(t, secret, session))
	bresp, err := http.DefaultClient.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	defer bresp.Body.Close()
	if bresp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed origin got allow-origin %q", bresp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestAgentAttachDisabledWhenNoSecret(t *testing.T) {
	gw := &Server{Metrics: sshgateway.NewMetrics(prometheus.NewRegistry())}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("disabled attach = %d, want 503", resp.StatusCode)
	}
}

// fakeTurnDriver serves the driver's POST /turn SSE: the turn's parts then the
// `[DONE]` sentinel, exactly like the in-sandbox driver answering a live prompt.
func fakeTurnDriver(parts []string) (*httptest.Server, string, int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/turn", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-vercel-ai-ui-message-stream", "v1")
		fl := w.(http.Flusher)
		for _, part := range parts {
			_, _ = io.WriteString(w, "data: "+part+"\n\n")
			fl.Flush()
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		fl.Flush()
	})
	srv := httptest.NewServer(mux)
	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	return srv, host, port
}

// TestAgentTurnQuotaIsCumulative pins the F10 fix: the turn path's byte quota
// is seeded from the session's ALREADY-STORED transcript bytes, not a fresh
// per-POST counter — so N turns each under the cap can never grow the stored
// transcript past it.
func TestAgentTurnQuotaIsCumulative(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000010"
	st := newFakeAttachStore()
	// A prior turn already stored 60 bytes (seq 0).
	_ = st.AppendAgentSessionTranscript(context.Background(), session, []store.AgentSessionTranscriptPart{
		{Seq: 0, Turn: 0, Part: []byte(strings.Repeat("s", 60))},
	})
	driver, host, port := fakeTurnDriver([]string{
		strings.Repeat("a", 30), strings.Repeat("b", 30),
	})
	defer driver.Close()

	gw := newAttachGateway(st, fixedPodIP{ip: host}, secret, port)
	gw.MaxTranscriptBytes = 100 // small quota so the test needs no 64 MiB payloads
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	postTurn := func() string {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"prompt":"go"}`))
		req.Header.Set(TicketHeader, turnTicket(t, secret, session))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	// Turn 1: 60 stored + 30 fits (90 ≤ 100); the next 30 would reach 120, so
	// the turn stops there — the client still receives the fitting part.
	body := postTurn()
	if payloads := dataPayloads(body); len(payloads) != 1 || payloads[0] != strings.Repeat("a", 30) {
		t.Fatalf("turn 1 client parts = %v, want the one part that fits", payloads)
	}
	// Turn 2: the stored 90 bytes leave only 10 under the cap, so NOTHING in
	// this turn is appended — a fresh per-turn counter would have stored 60 more.
	_ = postTurn()

	stored, _ := st.AgentSessionTranscript(context.Background(), session, -1, 1<<30)
	var total int64
	for _, p := range stored {
		total += int64(len(p.Part))
	}
	if total > gw.MaxTranscriptBytes {
		t.Fatalf("stored transcript = %d bytes, exceeds the %d-byte session cap (quota not cumulative)", total, gw.MaxTranscriptBytes)
	}
	if total != 90 || len(stored) != 2 {
		t.Fatalf("stored = %d bytes in %d parts, want 90 bytes in 2 parts (seed + the one fitting turn part)", total, len(stored))
	}
}

// TestAgentAttachReplayReadIsBounded pins that the replay read itself is
// budgeted at the store: even a store holding more than the quota (e.g. rows
// written before the cumulative fix) replays only the capped prefix.
func TestAgentAttachReplayReadIsBounded(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000011"
	st := newFakeAttachStore()
	_ = st.AppendAgentSessionTranscript(context.Background(), session, []store.AgentSessionTranscriptPart{
		{Seq: 0, Part: []byte(strings.Repeat("a", 40))},
		{Seq: 1, Part: []byte(strings.Repeat("b", 40))},
		{Seq: 2, Part: []byte(strings.Repeat("c", 40))},
	})
	gw := newAttachGateway(st, fixedPodIP{err: fmt.Errorf("terminal")}, secret, 8787)
	gw.MaxTranscriptBytes = 100 // 40+40 fits; the third part would reach 120
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(TicketHeader, attachTicket(t, secret, session))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	payloads := dataPayloads(string(body))
	if len(payloads) != 2 || payloads[0] != strings.Repeat("a", 40) || payloads[1] != strings.Repeat("b", 40) {
		t.Fatalf("bounded replay = %d parts, want the 2-part capped prefix", len(payloads))
	}
	if !strings.HasSuffix(strings.TrimSpace(string(body)), "data: [DONE]") {
		t.Fatalf("bounded replay did not end with [DONE]:\n%s", body)
	}
}
