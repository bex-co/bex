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

package modelproxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/modelproxy"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// cannedTransport answers every upstream request with one fixed response and
// keeps the request (with its body) for assertions.
type cannedTransport struct {
	contentType string
	body        string
	mu          sync.Mutex
	last        *http.Request
	lastBody    []byte
}

func (c *cannedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.last, c.lastBody = r.Clone(context.Background()), body
	c.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {c.contentType}},
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}, nil
}

// usageStore is bex-api's side of the platform harness: the session row, the
// idempotent usage ledger, the spend-cap inputs and the terminal CAS.
type usageStore struct {
	mu        sync.Mutex
	session   store.AgentSession
	recorded  map[string]store.AgentModelUsage
	reports   int
	plan      string
	trust     store.BillingTrust
	due       bool
	usedMicro int64
	reason    string
	clock     time.Time
	// sessionSpent is the session's recorded price (ADR094 D8's ceiling reads it).
	sessionSpent int64
}

func (s *usageStore) AgentSessionModelSpendMicroUSD(context.Context, string, string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionSpent, nil
}

func (s *usageStore) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	if id != s.session.ID {
		return store.AgentSession{}, store.ErrNotFound
	}
	return s.session, nil
}

func (s *usageStore) RecordAgentModelUsage(_ context.Context, u store.AgentModelUsage) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports++
	if _, dup := s.recorded[u.RequestID]; dup {
		return false, nil
	}
	s.recorded[u.RequestID] = u
	return true, nil
}

func (s *usageStore) AgentSpendInputs(context.Context, string, time.Time) (store.AgentSpendInputs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return store.AgentSpendInputs{Plan: s.plan, Trust: s.trust, MonthToDateMicroUSD: s.usedMicro, BalanceDue: s.due}, nil
}

// now is the minter's clock; advance moves it past the mint's spend cache.
func (s *usageStore) now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clock
}

func (s *usageStore) advance(d time.Duration) {
	s.mu.Lock()
	s.clock = s.clock.Add(d)
	s.mu.Unlock()
}

func (s *usageStore) FinalizeAgentSession(_ context.Context, _, _, _, _, reason string) (store.AgentSession, store.TerminalTurnFact, error) {
	s.mu.Lock()
	s.reason = reason
	s.mu.Unlock()
	return store.AgentSession{ID: "ags-one", Phase: "failed"}, store.TerminalTurnFact{Turn: 1}, nil
}

func (s *usageStore) snapshot() ([]store.AgentModelUsage, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.AgentModelUsage, 0, len(s.recorded))
	for _, u := range s.recorded {
		out = append(out, u)
	}
	return out, s.reason
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// platformHarness enables the platform route for tea-a and wires the real
// mint, usage and auth-failure verbs behind the shared HMAC, with the
// shipped spend caps (a fresh Hobby workspace: T0, $10).
func platformHarness(t *testing.T, upstream *cannedTransport) (*httptest.Server, *usageStore) {
	t.Helper()
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", []string{"tea-a"}, platformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
	st := &usageStore{
		session: store.AgentSession{
			ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
			AgentConfig: json.RawMessage(`{"agent":"claude","modelEndpoint":"https://api.anthropic.com/v1"}`),
		},
		recorded: map[string]store.AgentModelUsage{},
		plan:     store.PlanHobby,
		clock:    time.Now().UTC(),
	}
	minter := &agentsession.ModelMinter{Keys: fakeKV{}, Sessions: st, Spend: st, SessionSpend: st, Now: st.now}
	mux := http.NewServeMux()
	mux.Handle(agentsession.InternalModelMintPath, &agentsession.ModelHandler{Secret: secret, Minter: minter})
	mux.Handle(agentsession.InternalModelUsagePath, &agentsession.ModelUsageHandler{Secret: secret, Recorder: &agentsession.ModelUsageRecorder{Store: st}})
	mux.Handle(agentsession.InternalModelAuthFailurePath, &agentsession.ModelAuthFailureHandler{Secret: secret, Failer: &agentsession.ModelAuthFailer{Sessions: st, Spend: st}})
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	broker := &modelproxy.Broker{
		Pods: fakePods{pod: sandboxPod("ags-one")},
		API:  &agentsession.ModelClient{URL: api.URL + agentsession.InternalModelMintPath, Secret: secret, HTTP: api.Client()},
		HTTP: &http.Client{Transport: upstream},
	}
	proxy := httptest.NewServer(broker.Handler())
	t.Cleanup(proxy.Close)
	return proxy, st
}

var pinnedBody = `{"model":"` + agentsession.DefaultDevModel + `","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}`

// The SSE stream is relayed byte for byte and its message_start /
// message_delta usage (null cache-write, as OpenRouter sends it) is reported
// once to bex-api under the session's workspace and the pinned model.
func TestPlatformRouteMetersStreamingUsage(t *testing.T) {
	stream := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"gen-1","usage":{"input_tokens":1200,"output_tokens":1,"cache_creation_input_tokens":null,"cache_read_input_tokens":800}}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"the usage word is \"usage\""}}` + "\r\n\r\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":345}}` + "\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	upstream := &cannedTransport{contentType: "text/event-stream", body: stream}
	proxy, st := platformHarness(t, upstream)

	resp := requestBody(t, proxy, pinnedBody)
	relayed, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(relayed) != stream {
		t.Fatalf("status %d, relayed stream altered: %q", resp.StatusCode, relayed)
	}
	if got := upstream.last.Header.Get("Accept-Encoding"); got != "identity" {
		t.Fatalf("metered upstream Accept-Encoding = %q, want identity", got)
	}
	eventually(t, func() bool { got, _ := st.snapshot(); return len(got) == 1 })
	got, _ := st.snapshot()
	u := got[0]
	if u.WorkspaceID != "tea-a" || u.SessionID != "ags-one" || u.Model != agentsession.DefaultDevModel ||
		u.InputTokens != 1200 || u.OutputTokens != 345 || u.CacheReadTokens != 800 || u.CacheWriteTokens != 0 {
		t.Fatalf("recorded usage = %+v", u)
	}
	if !strings.HasPrefix(u.RequestID, "amr-") {
		t.Fatalf("request id = %q, want an amr- id", u.RequestID)
	}
	// No usage.cost on the stream and no price table in this harness: the
	// exchange is recorded unpriced at zero (ADR091 D2).
	if u.CostSource != store.AgentModelCostUnpriced || u.PriceMicroUSD != 0 {
		t.Fatalf("cost = %s/%d, want unpriced at 0", u.CostSource, u.PriceMicroUSD)
	}
}

func TestPlatformRouteMetersJSONUsage(t *testing.T) {
	upstream := &cannedTransport{contentType: "application/json", body: `{"id":"gen-2","type":"message","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":6,"output_tokens":5,"cache_creation_input_tokens":7,"cache_read_input_tokens":0,"cost":3.23e-06}}`}
	proxy, st := platformHarness(t, upstream)
	resp := requestBody(t, proxy, `{"model":"`+agentsession.DefaultDevModel+`","max_tokens":5,"messages":[]}`)
	resp.Body.Close()
	eventually(t, func() bool { got, _ := st.snapshot(); return len(got) == 1 })
	got, _ := st.snapshot()
	if u := got[0]; u.InputTokens != 6 || u.OutputTokens != 5 || u.CacheWriteTokens != 7 || u.CacheReadTokens != 0 {
		t.Fatalf("recorded usage = %+v", u)
	}
	// usage.cost $3.23e-06 travels to bex-api: upstream 3 µUSD, priced at
	// the shipped 1.30 markup → 4.199 → 4 µUSD (half-up).
	if u := got[0]; u.CostSource != store.AgentModelCostReported || u.UpstreamCostMicroUSD != 3 || u.PriceMicroUSD != 4 {
		t.Fatalf("recorded cost = %s upstream=%d price=%d, want reported 3/4", u.CostSource, u.UpstreamCostMicroUSD, u.PriceMicroUSD)
	}
}

// The platform data policy is JSON-merged into the forwarded body (the
// client's bytes kept verbatim ahead of it), and a client-supplied provider
// object is refused before any upstream request.
func TestPlatformRouteInjectsDataPolicyAndRefusesClientProvider(t *testing.T) {
	upstream := &cannedTransport{contentType: "application/json", body: `{"usage":{"input_tokens":1,"output_tokens":1}}`}
	proxy, _ := platformHarness(t, upstream)
	resp := requestBody(t, proxy, pinnedBody+"\n")
	resp.Body.Close()
	want := strings.TrimSuffix(pinnedBody, "}") + `,"provider":{"data_collection":"deny"}}`
	if string(upstream.lastBody) != want {
		t.Fatalf("forwarded body = %s, want %s", upstream.lastBody, want)
	}
	var decoded struct {
		Model    string `json:"model"`
		Provider struct {
			DataCollection string `json:"data_collection"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(upstream.lastBody, &decoded); err != nil || decoded.Provider.DataCollection != "deny" || decoded.Model != agentsession.DefaultDevModel {
		t.Fatalf("forwarded body is not valid JSON with the policy: %v %+v", err, decoded)
	}

	for _, provider := range []string{`{"data_collection":"allow"}`, `{"data_collection":"deny"}`, `{"order":["x"]}`} {
		upstream.last = nil
		resp := requestBody(t, proxy, `{"model":"`+agentsession.DefaultDevModel+`","provider":`+provider+`}`)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden || upstream.last != nil {
			t.Fatalf("client provider %s: status %d upstream=%v, want 403 and no upstream", provider, resp.StatusCode, upstream.last != nil)
		}
	}
}

// A workspace under its spend cap is served; at the cap the mint is refused
// with a typed 429 the agent CLI can show, and the session fails with the
// readable spend-cap reason (ADR091 D5). A fresh Hobby workspace is T0: it is
// charged at $10 and keeps working until $15 while that charge is unpaid
// (D10.2a), so its cap is $15.
func TestPlatformRouteSpendCapRefusesAtTheLimit(t *testing.T) {
	upstream := &cannedTransport{contentType: "application/json", body: `{"usage":{"input_tokens":1,"output_tokens":1}}`}
	proxy, st := platformHarness(t, upstream)
	st.usedMicro = 14_999_999
	resp := requestBody(t, proxy, pinnedBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("under the cap: status %d, want 200", resp.StatusCode)
	}

	st.mu.Lock()
	st.usedMicro = 15_000_000
	st.mu.Unlock()
	st.advance(time.Minute) // past the mint's spend cache
	upstream.last = nil
	resp = requestBody(t, proxy, pinnedBody)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || upstream.last != nil {
		t.Fatalf("at the cap: status %d upstream=%v, want 429 and no upstream", resp.StatusCode, upstream.last != nil)
	}
	var refusal struct {
		Type  string `json:"type"`
		Error struct {
			Type, Code, Message string
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil || refusal.Type != "error" ||
		refusal.Error.Code != agentsession.ModelSpendCapReachedCode || !strings.Contains(refusal.Error.Message, "spend cap") {
		t.Fatalf("refusal body = %s", body)
	}
	if strings.Contains(string(body), platformKey) {
		t.Fatal("refusal leaked the platform key")
	}
	eventually(t, func() bool { _, reason := st.snapshot(); return reason != "" })
	// T0's unpaid first charge, not the plan cap, is what binds: the reason says so.
	if _, reason := st.snapshot(); !strings.Contains(reason, agentsession.ModelSpendCapReachedCode) || !strings.Contains(reason, "not paid yet") {
		t.Fatalf("session failure reason = %q", reason)
	}
}

// A session with a maxSpendUSD ceiling is served under it; at it the mint is
// refused with the same code as the monthly cap, scope "session", and the
// session fails with the session-ceiling reason (ADR094 D8). The workspace is
// far under its own cap throughout.
func TestPlatformRouteSessionCeilingRefusesAtTheLimit(t *testing.T) {
	upstream := &cannedTransport{contentType: "application/json", body: `{"usage":{"input_tokens":1,"output_tokens":1}}`}
	proxy, st := platformHarness(t, upstream)
	ceiling := int64(2_000_000)
	st.mu.Lock()
	st.session.MaxSpendMicroUSD = &ceiling
	st.sessionSpent = ceiling - 1
	st.mu.Unlock()
	resp := requestBody(t, proxy, pinnedBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("under the ceiling: status %d, want 200", resp.StatusCode)
	}

	st.mu.Lock()
	st.sessionSpent = ceiling
	st.mu.Unlock()
	upstream.last = nil
	resp = requestBody(t, proxy, pinnedBody)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || upstream.last != nil {
		t.Fatalf("at the ceiling: status %d upstream=%v, want 429 and no upstream", resp.StatusCode, upstream.last != nil)
	}
	var refusal struct {
		Type  string `json:"type"`
		Error struct {
			Type, Code, Message, Scope string
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil || refusal.Type != "error" ||
		refusal.Error.Code != agentsession.ModelSpendCapReachedCode || refusal.Error.Scope != agentsession.ModelSpendScopeSession {
		t.Fatalf("refusal body = %s", body)
	}
	if strings.Contains(string(body), platformKey) {
		t.Fatal("refusal leaked the platform key")
	}
	eventually(t, func() bool { _, reason := st.snapshot(); return reason != "" })
	if _, reason := st.snapshot(); reason != agentsession.ModelSessionSpendCapReachedReason {
		t.Fatalf("session failure reason = %q", reason)
	}
}

// An unpaid failed invoice refuses the platform route with a typed 429
// BILLING_BALANCE_DUE even far under the cap (ADR091 D10.3: no grace period
// for model spend), and the session fails with the readable reason.
func TestPlatformRouteRefusesWhileABalanceIsDue(t *testing.T) {
	upstream := &cannedTransport{contentType: "application/json", body: `{"usage":{"input_tokens":1,"output_tokens":1}}`}
	proxy, st := platformHarness(t, upstream)
	st.mu.Lock()
	st.trust = store.BillingTrust{FailedInvoiceID: "in_failed", LockedUntil: store.LockedIndefinitely}
	st.due = true
	st.mu.Unlock()
	resp := requestBody(t, proxy, pinnedBody)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || upstream.last != nil {
		t.Fatalf("balance due: status %d upstream=%v, want 429 and no upstream", resp.StatusCode, upstream.last != nil)
	}
	var refusal struct {
		Error struct{ Type, Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil || refusal.Error.Code != agentsession.BillingBalanceDueCode ||
		refusal.Error.Type != "rate_limit_error" || !strings.Contains(refusal.Error.Message, "payment for this workspace failed") {
		t.Fatalf("refusal body = %s", body)
	}
	eventually(t, func() bool { _, reason := st.snapshot(); return reason != "" })
	if _, reason := st.snapshot(); reason != agentsession.BillingBalanceDueReason {
		t.Fatalf("session failure reason = %q, want %q", reason, agentsession.BillingBalanceDueReason)
	}
}

// A workspace BYO key is the tenant's own spend: never metered, never given
// the platform data policy.
func TestByoRouteIsNotMetered(t *testing.T) {
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-BYO")
	resp := requestBody(t, proxy, `{"model":"claude-opus-4-5"}`)
	resp.Body.Close()
	forwarded, _ := io.ReadAll(capture.last.Body)
	if string(forwarded) != `{"model":"claude-opus-4-5"}` || capture.last.Header.Get("Accept-Encoding") == "identity" {
		t.Fatalf("BYO request altered: %s", forwarded)
	}
}
