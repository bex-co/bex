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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const recordSecret = "record-secret-distinct-from-browser"

func newRecordGateway(st *fakeAttachStore, pods PodIPResolver, port int) *Server {
	return &Server{
		// Deliberately different browser Secret vs RecordSecret, mirroring prod
		// (BEX_SHELL_TICKET_SECRET vs BEX_SANDBOX_EXEC_SECRET).
		Secret:         []byte("browser-secret"),
		RecordSecret:   []byte(recordSecret),
		Store:          st,
		Pods:           pods,
		DriverPort:     port,
		Now:            func() time.Time { return time.Unix(1_800_000_000, 0) },
		Metrics:        sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:         sshgateway.NewSessionLimiter(100, 5),
		Nonces:         &sshgateway.NonceGuard{Store: st},
		SessionTimeout: time.Minute,
	}
}

func recordTicket(t *testing.T, secret []byte, session string, turn int) string {
	t.Helper()
	tok, err := agentsessionticket.Mint(secret, agentsessionticket.Claims{
		Subject: "system:agent-session-completer", SessionID: session, SandboxID: "sandbox-1",
		Pod: "sandbox-1-0", Workspace: "tea-a", Namespace: "tea-a-sandbox", Turn: turn,
		IssuedAt: 1_800_000_000, ExpiresAt: 1_800_000_090,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func postRecord(t *testing.T, srv *Server, ticket string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/agent-record", nil)
	if ticket != "" {
		req.Header.Set(TicketHeader, ticket)
	}
	rec := httptest.NewRecorder()
	srv.RecordHandler().ServeHTTP(rec, req)
	return rec
}

// The headline fix: a fire-and-forget session that never had a browser attached
// must still get its conversation persisted, so a later replay is non-empty.
func TestRecordSessionTeesHeadlessTranscript(t *testing.T) {
	st := newFakeAttachStore()
	driver, ip, port := fakeDriver([]string{`{"type":"text","text":"hello"}`, `{"type":"text","text":"world"}`})
	defer driver.Close()
	srv := newRecordGateway(st, fixedPodIP{ip: ip}, port)

	rec := postRecord(t, srv, recordTicket(t, []byte(recordSecret), "ags-1", 1))
	if rec.Code != http.StatusOK {
		t.Fatalf("record status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	stored, _ := st.AgentSessionTranscript(context.Background(), "ags-1", -1)
	if len(stored) != 2 {
		t.Fatalf("stored %d parts, want 2 (headless transcript must not be empty)", len(stored))
	}
	if stored[0].Seq != 0 || stored[1].Seq != 1 {
		t.Fatalf("seqs = %d,%d, want 0,1", stored[0].Seq, stored[1].Seq)
	}
	for _, p := range stored {
		if p.Turn != 1 {
			t.Fatalf("part turn = %d, want 1", p.Turn)
		}
	}
}

// A Completer retry (or a live viewer that already teed the turn) must not
// double-store the conversation.
func TestRecordSessionIdempotentPerTurn(t *testing.T) {
	st := newFakeAttachStore()
	driver, ip, port := fakeDriver([]string{`{"type":"text","text":"a"}`, `{"type":"text","text":"b"}`})
	defer driver.Close()
	srv := newRecordGateway(st, fixedPodIP{ip: ip}, port)

	if rec := postRecord(t, srv, recordTicket(t, []byte(recordSecret), "ags-1", 1)); rec.Code != http.StatusOK {
		t.Fatalf("first record status = %d", rec.Code)
	}
	if rec := postRecord(t, srv, recordTicket(t, []byte(recordSecret), "ags-1", 1)); rec.Code != http.StatusOK {
		t.Fatalf("second record status = %d", rec.Code)
	}
	stored, _ := st.AgentSessionTranscript(context.Background(), "ags-1", -1)
	if len(stored) != 2 {
		t.Fatalf("stored %d parts after two records, want 2 (idempotent, no duplicates)", len(stored))
	}
}

// A redispatched (steered) turn runs in a fresh sandbox whose driver replay
// restarts at ordinal 0; the recorder must append it AFTER the prior turn.
func TestRecordSessionConcatenatesTurns(t *testing.T) {
	st := newFakeAttachStore()
	// Turn 1 already recorded (seq 0,1 turn=1).
	_ = st.AppendAgentSessionTranscript(context.Background(), "ags-1", []store.AgentSessionTranscriptPart{
		{Seq: 0, Turn: 1, Part: []byte(`{"type":"text","text":"t1a"}`)},
		{Seq: 1, Turn: 1, Part: []byte(`{"type":"text","text":"t1b"}`)},
	})
	driver, ip, port := fakeDriver([]string{`{"type":"text","text":"t2a"}`})
	defer driver.Close()
	srv := newRecordGateway(st, fixedPodIP{ip: ip}, port)

	if rec := postRecord(t, srv, recordTicket(t, []byte(recordSecret), "ags-1", 2)); rec.Code != http.StatusOK {
		t.Fatalf("record turn 2 status = %d", rec.Code)
	}
	stored, _ := st.AgentSessionTranscript(context.Background(), "ags-1", -1)
	if len(stored) != 3 {
		t.Fatalf("stored %d parts, want 3 (turn 2 appended, not colliding)", len(stored))
	}
	last := stored[2]
	if last.Seq != 2 || last.Turn != 2 || string(last.Part) != `{"type":"text","text":"t2a"}` {
		t.Fatalf("turn-2 part = %+v, want seq=2 turn=2", last)
	}
}

// A sandbox already gone (no live driver) is not an error: nothing to capture.
func TestRecordSessionGonePodIsNoOp(t *testing.T) {
	st := newFakeAttachStore()
	srv := newRecordGateway(st, fixedPodIP{err: context.DeadlineExceeded}, 65535)

	rec := postRecord(t, srv, recordTicket(t, []byte(recordSecret), "ags-1", 1))
	if rec.Code != http.StatusOK {
		t.Fatalf("gone-pod record status = %d, want 200 no-op", rec.Code)
	}
	stored, _ := st.AgentSessionTranscript(context.Background(), "ags-1", -1)
	if len(stored) != 0 {
		t.Fatalf("stored %d parts for gone pod, want 0", len(stored))
	}
}

// A browser ticket (or any wrong-secret ticket) must not drive the recorder.
func TestRecordRejectsWrongSecret(t *testing.T) {
	st := newFakeAttachStore()
	driver, ip, port := fakeDriver([]string{`{"type":"text","text":"x"}`})
	defer driver.Close()
	srv := newRecordGateway(st, fixedPodIP{ip: ip}, port)

	rec := postRecord(t, srv, recordTicket(t, []byte("browser-secret"), "ags-1", 1))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-secret record status = %d, want 401", rec.Code)
	}
}

func TestRecordDisabledWithoutSecret(t *testing.T) {
	st := newFakeAttachStore()
	srv := newRecordGateway(st, fixedPodIP{ip: "10.0.0.1"}, 8787)
	srv.RecordSecret = nil
	if srv.RecordEnabled() {
		t.Fatal("RecordEnabled with nil secret")
	}
	rec := postRecord(t, srv, "anything")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled record status = %d, want 503", rec.Code)
	}
}
