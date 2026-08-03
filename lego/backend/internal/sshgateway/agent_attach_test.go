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

package sshgateway

import (
	"context"
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
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeAttachStore is an in-memory AgentAttachStore: an ordered transcript keyed
// by (session, seq) with idempotent append, plus a single-use nonce claim.
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

func (f *fakeAttachStore) AgentSessionTranscript(_ context.Context, id string, afterSeq int64) ([]store.AgentSessionTranscriptPart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.AgentSessionTranscriptPart, 0)
	for seq, p := range f.parts[id] {
		if seq > afterSeq {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
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

func newAttachGateway(store AgentAttachStore, pods PodIPResolver, secret []byte, port int) *Server {
	return &Server{
		Metrics:        NewMetrics(prometheus.NewRegistry()),
		MaxSessions:    100,
		MaxPerIdentity: 5,
		SessionTimeout: time.Minute,
		AgentAttach: &AgentAttachConfig{
			Store: store, Pods: pods, Secret: secret, DriverPort: port,
			Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
		},
	}
}

func attachTicket(t *testing.T, secret []byte, session string) string {
	t.Helper()
	tok, err := agentsessionticket.Mint(secret, agentsessionticket.Claims{
		Subject: "alice", SessionID: session, SandboxID: "sandbox-1", Pod: "sandbox-1-0",
		Workspace: "tea-a", Namespace: "tea-a-sandbox",
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
	srv := httptest.NewServer(gw.AgentAttachHandler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(AgentAttachTicketHeader, attachTicket(t, secret, session))
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
	stored, _ := st.AgentSessionTranscript(context.Background(), session, -1)
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
	srv := httptest.NewServer(gw.AgentAttachHandler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(AgentAttachTicketHeader, attachTicket(t, secret, session))
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
	srv := httptest.NewServer(gw.AgentAttachHandler())
	defer srv.Close()

	do := func(ticket string) int {
		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		if ticket != "" {
			req.Header.Set(AgentAttachTicketHeader, ticket)
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

func TestAgentAttachDisabledWhenNoSecret(t *testing.T) {
	gw := &Server{Metrics: NewMetrics(prometheus.NewRegistry())}
	srv := httptest.NewServer(gw.AgentAttachHandler())
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
