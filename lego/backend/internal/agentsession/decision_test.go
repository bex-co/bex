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

package agentsession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type recordingDecisions struct {
	envelopes []DecisionEnvelope
	refuse    error
}

func (s *recordingDecisions) OpenDecision(_ context.Context, e DecisionEnvelope) (DecisionResponse, error) {
	s.envelopes = append(s.envelopes, e)
	if s.refuse != nil {
		return DecisionResponse{}, s.refuse
	}
	return DecisionResponse{ID: "dcs-00000000000000000001", State: "pending"}, nil
}

func (s *recordingDecisions) AwaitDecision(_ context.Context, e DecisionEnvelope) (DecisionResponse, error) {
	s.envelopes = append(s.envelopes, e)
	if s.refuse != nil {
		return DecisionResponse{}, s.refuse
	}
	return DecisionResponse{ID: e.DecisionID, State: "allowed", Answer: json.RawMessage(`{"allow":true,"scope":"once"}`)}, nil
}

func openEnvelope() DecisionEnvelope {
	return DecisionEnvelope{Kind: DecisionKind, Op: DecisionOpOpen, Workspace: "tea-a", SessionID: "ags-one", PodName: "sbx-1-0", PodUID: "uid-1",
		Content: DecisionContent{RequestID: "perm-1", Kind: DecisionApproval, Title: "Run: rm -rf /var/data", Detail: json.RawMessage(`{"command":"rm -rf /var/data"}`)},
		Nonce:   "nonce-1"}
}

func serveDecision(t *testing.T, sink DecisionSink, envelope DecisionEnvelope, mac func([]byte) []byte) *httptest.ResponseRecorder {
	t.Helper()
	secret, now := []byte("s"), time.Unix(1_800_000_000, 0)
	raw, _ := json.Marshal(envelope)
	request := httptest.NewRequest(http.MethodPost, InternalDecisionPath, bytes.NewReader(raw))
	signHeaders(request.Header, secret, mac(raw), now)
	recorder := httptest.NewRecorder()
	(&DecisionHandler{Secret: secret, Decisions: sink, Nonce: &fakeNonceClaimer{}, Now: func() time.Time { return now }}).ServeHTTP(recorder, request)
	return recorder
}

func TestDecisionHandlerAuthenticatesAndRoutesTheEnvelope(t *testing.T) {
	sink := &recordingDecisions{}
	if rec := serveDecision(t, sink, openEnvelope(), decisionMACInput); rec.Code != http.StatusOK || len(sink.envelopes) != 1 || sink.envelopes[0].Op != DecisionOpOpen {
		t.Fatalf("open: %d %s (%d calls)", rec.Code, rec.Body, len(sink.envelopes))
	}
	await := openEnvelope()
	await.Op, await.DecisionID, await.Content, await.Nonce = DecisionOpAwait, "dcs-00000000000000000001", DecisionContent{}, "nonce-2"
	if rec := serveDecision(t, sink, await, decisionMACInput); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"allowed"`) {
		t.Fatalf("await: %d %s", rec.Code, rec.Body)
	}
	// The result verb's MAC domain does not verify a decision envelope.
	if rec := serveDecision(t, &recordingDecisions{}, openEnvelope(), resultMACInput); rec.Code != http.StatusUnauthorized {
		t.Fatalf("cross-domain MAC: %d", rec.Code)
	}
	for name, mutate := range map[string]func(*DecisionEnvelope){
		"unknown op":        func(e *DecisionEnvelope) { e.Op = "answer" },
		"open naming an id": func(e *DecisionEnvelope) { e.DecisionID = "dcs-00000000000000000001" },
		"await without id":  func(e *DecisionEnvelope) { e.Op = DecisionOpAwait },
		"no pod":            func(e *DecisionEnvelope) { e.PodUID = "" },
	} {
		envelope := openEnvelope()
		mutate(&envelope)
		sink := &recordingDecisions{}
		if rec := serveDecision(t, sink, envelope, decisionMACInput); rec.Code != http.StatusBadRequest || len(sink.envelopes) != 0 {
			t.Errorf("%s: %d (%d calls)", name, rec.Code, len(sink.envelopes))
		}
	}
	// An invalid open is refused before the sink with the sandbox vocabulary.
	bad := openEnvelope()
	bad.Content.Kind = "vote"
	if rec := serveDecision(t, &recordingDecisions{}, bad, decisionMACInput); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), ResultInvalidRequest) {
		t.Fatalf("invalid content: %d %s", rec.Code, rec.Body)
	}
}

func TestDecisionContentBounds(t *testing.T) {
	ok := openEnvelope().Content
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid content refused: %v", err)
	}
	for name, mutate := range map[string]func(*DecisionContent){
		"request id with a slash": func(c *DecisionContent) { c.RequestID = "a/b" },
		"empty title":             func(c *DecisionContent) { c.Title = " " },
		"two-line title":          func(c *DecisionContent) { c.Title = "one\ntwo" },
		"long title":              func(c *DecisionContent) { c.Title = strings.Repeat("x", MaxDecisionTitleRunes+1) },
		"array detail":            func(c *DecisionContent) { c.Detail = json.RawMessage(`[1]`) },
		"NUL detail":              func(c *DecisionContent) { c.Detail = json.RawMessage(`{"a":"\u0000"}`) },
		"huge detail": func(c *DecisionContent) {
			c.Detail = json.RawMessage(`{"a":"` + strings.Repeat("x", MaxDecisionDetailBytes) + `"}`)
		},
	} {
		c := ok
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, invalid := DecodeDecisionContent([]byte(`{"requestId":"r","kind":"question","title":"t","extra":1}`)); invalid == nil {
		t.Fatal("an unknown field was accepted")
	}
}

func TestDecisionPathRoundTrip(t *testing.T) {
	url, err := DecisionURL("http://gw:8082", "ws-tea-a", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimPrefix(url, "http://gw:8082")
	ns, session, id, err := ParseDecisionPath(path)
	if err != nil || ns != "ws-tea-a" || session != "ags-one" || id != "" {
		t.Fatalf("open path → %q %q %q %v", ns, session, id, err)
	}
	if _, _, id, err := ParseDecisionPath(path + "/dcs-00000000000000000001"); err != nil || id != "dcs-00000000000000000001" {
		t.Fatalf("await path → %q %v", id, err)
	}
	for _, bad := range []string{path + "/", path + "/att-00000000000000000001", path + "/dcs-1/extra", "/agent-results/x/y"} {
		if _, _, _, err := ParseDecisionPath(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestDecisionClientRelaysOnlyTheSandboxVocabulary(t *testing.T) {
	sink := &recordingDecisions{refuse: NewDecisionError(ResultForbidden, "not this session")}
	secret := []byte("s")
	server := httptest.NewServer(&DecisionHandler{Secret: secret, Decisions: sink, Nonce: &fakeNonceClaimer{}})
	defer server.Close()
	client := &DecisionClient{URL: server.URL, Secret: secret}
	_, err := client.Forward(context.Background(), openEnvelope())
	var typed *ResultError
	if !errors.As(err, &typed) || typed.Code != ResultForbidden || typed.Status != http.StatusForbidden {
		t.Fatalf("refusal = %v", err)
	}
	sink.refuse = nil
	out, err := client.Forward(context.Background(), openEnvelope())
	if err != nil || out.State != "pending" || len(sink.envelopes) != 2 || sink.envelopes[1].Nonce == sink.envelopes[0].Nonce {
		t.Fatalf("open = %+v, %v (nonces must differ per call)", out, err)
	}
	// A wrong secret is reported as forbidden, never as the upstream's text.
	bad := &DecisionClient{URL: server.URL, Secret: []byte("other")}
	if _, err := bad.Forward(context.Background(), openEnvelope()); !errors.As(err, &typed) || typed.Code != ResultForbidden {
		t.Fatalf("wrong secret = %v", err)
	}
}
