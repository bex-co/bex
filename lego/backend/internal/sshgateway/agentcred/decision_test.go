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

package agentcred

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// recordingDecisions is bex-api's decision verb with the storage replaced.
type recordingDecisions struct {
	envelopes []agentsession.DecisionEnvelope
	// awaitState is what an await answers ("pending" when empty: a held
	// await whose hold elapsed).
	awaitState string
}

func (s *recordingDecisions) OpenDecision(_ context.Context, e agentsession.DecisionEnvelope) (agentsession.DecisionResponse, error) {
	s.envelopes = append(s.envelopes, e)
	return agentsession.DecisionResponse{ID: "dcs-00000000000000000001", State: "pending"}, nil
}

func (s *recordingDecisions) AwaitDecision(_ context.Context, e agentsession.DecisionEnvelope) (agentsession.DecisionResponse, error) {
	s.envelopes = append(s.envelopes, e)
	state := s.awaitState
	if state == "" {
		state = "pending"
	}
	return agentsession.DecisionResponse{ID: e.DecisionID, State: state}, nil
}

func decisionHarness(t *testing.T, perSession int) (*Broker, *recordingDecisions, *credentialAudit) {
	t.Helper()
	secret := []byte("decision-secret")
	sink := &recordingDecisions{}
	api := httptest.NewServer(&agentsession.DecisionHandler{Secret: secret, Decisions: sink, Nonce: &onceNonces{seen: map[string]bool{}}})
	t.Cleanup(api.Close)
	audit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: repoLessLabels(t, "tea-a", "ags-one")},
			"10.0.0.2": {Name: "sbx-b-0", UID: "uid-b", Labels: credentialLabels(t, "tea-a", "ags-two", "bex-agent/task-1")},
		}},
		API:                   &agentsession.Client{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Decisions:             &agentsession.DecisionClient{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Audit:                 audit,
		MaxRequestsPerSession: perSession,
	}
	return broker, sink, audit
}

func postDecision(t *testing.T, broker *Broker, target, sourceIP, body string) resultReply {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	request.RemoteAddr = sourceIP + ":43210"
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	broker.DecisionHandler().ServeHTTP(recorder, request)
	reply := resultReply{code: recorder.Code, raw: recorder.Body.String()}
	var envelope struct {
		Error *struct{ Code, Message, Hint string } `json:"error"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &envelope) == nil && envelope.Error != nil {
		reply.error = *envelope.Error
	}
	return reply
}

const approvalBody = `{"requestId":"perm-1","kind":"approval","title":"Run: git push --force","detail":{"command":"git push --force"}}`

func TestDecisionRouteOpensForTheVerifiedSession(t *testing.T) {
	broker, sink, audit := decisionHarness(t, 0)
	open, err := agentsession.DecisionURL("http://gateway.internal:8082", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	reply := postDecision(t, broker, open, "10.0.0.1", approvalBody)
	if reply.code != http.StatusOK || len(sink.envelopes) != 1 {
		t.Fatalf("open: %d %s", reply.code, reply.raw)
	}
	got := sink.envelopes[0]
	if got.Op != agentsession.DecisionOpOpen || got.SessionID != "ags-one" || got.Workspace != "tea-a" || got.PodName != "sbx-a-0" || got.Content.RequestID != "perm-1" {
		t.Fatalf("envelope = %+v", got)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbDecision {
		t.Fatalf("audit = %+v", audit.events)
	}
	// Another session's Pod cannot ask for this one.
	if reply := postDecision(t, broker, open, "10.0.0.2", approvalBody); reply.error.Code != agentsession.ResultForbidden {
		t.Fatalf("foreign Pod: %d %s", reply.code, reply.raw)
	}
	// Malformed opens never reach bex-api.
	for _, body := range []string{`{"requestId":"r","kind":"vote","title":"t"}`, `{"requestId":"r","kind":"question","title":"t","x":1}`, `not json`} {
		if reply := postDecision(t, broker, open, "10.0.0.1", body); reply.error.Code != agentsession.ResultInvalidRequest {
			t.Errorf("%s: %d %s", body, reply.code, reply.raw)
		}
	}
	if len(sink.envelopes) != 1 {
		t.Fatalf("refused requests reached bex-api: %d", len(sink.envelopes))
	}
}

// An await is a repeated long poll while a person decides: it must not spend
// the session's request budget, which an open still does.
func TestDecisionAwaitDoesNotSpendTheSessionBudget(t *testing.T) {
	broker, sink, _ := decisionHarness(t, 1)
	open, _ := agentsession.DecisionURL("http://gateway.internal:8082", "tea-a-sandbox", "ags-one")
	if reply := postDecision(t, broker, open, "10.0.0.1", approvalBody); reply.code != http.StatusOK {
		t.Fatalf("open: %d %s", reply.code, reply.raw)
	}
	await := open + "/dcs-00000000000000000001"
	for range 5 {
		if reply := postDecision(t, broker, await, "10.0.0.1", `{}`); reply.code != http.StatusOK {
			t.Fatalf("await: %d %s", reply.code, reply.raw)
		}
	}
	if last := sink.envelopes[len(sink.envelopes)-1]; last.Op != agentsession.DecisionOpAwait || last.DecisionID != "dcs-00000000000000000001" {
		t.Fatalf("await envelope = %+v", last)
	}
	// The budget of one is spent by the first open.
	if reply := postDecision(t, broker, open, "10.0.0.1", strings.Replace(approvalBody, "perm-1", "perm-2", 1)); reply.error.Code != agentsession.ResultRateLimited {
		t.Fatalf("second open past the budget: %d %s", reply.code, reply.raw)
	}
}

// Awaits hold their slot on their own limiter: a sandbox paused on decisions
// must not take the slots Git, PR, share and result requests need, and a
// busy shared limiter does not stop an answer from arriving.
func TestDecisionAwaitUsesItsOwnLimiter(t *testing.T) {
	broker, _, _ := decisionHarness(t, 0)
	broker.Limits = sshgateway.NewSessionLimiter(1, 1)
	open, _ := agentsession.DecisionURL("http://gateway.internal:8082", "tea-a-sandbox", "ags-one")
	if ok, _ := broker.Limits.Acquire("10.0.0.1"); !ok {
		t.Fatal("could not take the shared slot")
	}
	if reply := postDecision(t, broker, open+"/dcs-00000000000000000001", "10.0.0.1", `{}`); reply.code != http.StatusOK {
		t.Fatalf("await with the shared limiter full: %d %s", reply.code, reply.raw)
	}
	if reply := postDecision(t, broker, open, "10.0.0.1", approvalBody); reply.error.Code != agentsession.ResultRateLimited {
		t.Fatalf("open with the shared limiter full: %d %s", reply.code, reply.raw)
	}
	broker.Limits.Release("10.0.0.1")
	broker.AwaitLimits = sshgateway.NewSessionLimiter(1, 1)
	if ok, _ := broker.AwaitLimits.Acquire("10.0.0.1"); !ok {
		t.Fatal("could not take the await slot")
	}
	if reply := postDecision(t, broker, open+"/dcs-00000000000000000001", "10.0.0.1", `{}`); reply.error.Code != agentsession.ResultRateLimited {
		t.Fatalf("await with the await limiter full: %d %s", reply.code, reply.raw)
	}
}

// An await bex-api answered at once (here a final state; equally an unknown
// decision) is charged to the quick-await budget, so a sandbox cannot spin
// on them; a held await that came back pending stays free.
func TestQuickDecisionAwaitsAreBudgeted(t *testing.T) {
	broker, sink, _ := decisionHarness(t, 0)
	broker.MaxQuickAwaitsPerSession = 2
	await, _ := agentsession.DecisionURL("http://gateway.internal:8082", "tea-a-sandbox", "ags-one")
	await += "/dcs-00000000000000000001"
	for range 5 {
		if reply := postDecision(t, broker, await, "10.0.0.1", `{}`); reply.code != http.StatusOK {
			t.Fatalf("held await: %d %s", reply.code, reply.raw)
		}
	}
	sink.awaitState = "allowed"
	for i := range 2 {
		if reply := postDecision(t, broker, await, "10.0.0.1", `{}`); reply.code != http.StatusOK {
			t.Fatalf("quick await %d: %d %s", i, reply.code, reply.raw)
		}
	}
	reached := len(sink.envelopes)
	if reply := postDecision(t, broker, await, "10.0.0.1", `{}`); reply.error.Code != agentsession.ResultRateLimited {
		t.Fatalf("quick await past the budget: %d %s", reply.code, reply.raw)
	}
	if len(sink.envelopes) != reached {
		t.Fatal("a refused await still reached bex-api")
	}
}
