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

// recordingResults is bex-api's result verb with the storage replaced: it
// records what the signed envelope named.
type recordingResults struct {
	envelopes []agentsession.ResultEnvelope
	refuse    *agentsession.ResultError
}

func (s *recordingResults) SubmitResult(_ context.Context, envelope agentsession.ResultEnvelope) (agentsession.ResultResponse, error) {
	s.envelopes = append(s.envelopes, envelope)
	if s.refuse != nil {
		return agentsession.ResultResponse{}, s.refuse
	}
	return agentsession.ResultResponse{Accepted: true, Final: envelope.Final}, nil
}

// repoLessLabels are a chat-only session's Pod labels: the session binding and
// no repository or branch digest.
func repoLessLabels(t *testing.T, workspace, session string) map[string]string {
	t.Helper()
	labels, err := agentsession.BindingLabels(session, "", "")
	if err != nil {
		t.Fatal(err)
	}
	labels[agentsession.LabelWorkspace] = workspace
	labels[agentsession.LabelRegime] = agentsession.RegimeSandbox
	return labels
}

func resultHarness(t *testing.T) (*Broker, *recordingResults, *credentialAudit) {
	t.Helper()
	secret := []byte("result-secret")
	sink := &recordingResults{}
	api := httptest.NewServer(&agentsession.ResultHandler{Secret: secret, Results: sink, Nonce: &onceNonces{seen: map[string]bool{}}})
	t.Cleanup(api.Close)
	audit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: repoLessLabels(t, "tea-a", "ags-one")},
			"10.0.0.2": {Name: "sbx-b-0", UID: "uid-b", Labels: credentialLabels(t, "tea-a", "ags-two", "bex-agent/task-1")},
			"10.0.0.4": {Name: "sbx-a-0", UID: "uid-d", Labels: credentialLabels(t, "tea-b", "ags-one", "bex-agent/task-1")},
		}},
		API:     &agentsession.Client{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Results: &agentsession.ResultClient{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Audit:   audit,
	}
	return broker, sink, audit
}

func resultURL(t *testing.T, namespace, session string) string {
	t.Helper()
	u, err := agentsession.ResultURL("http://gateway.internal:8082", namespace, session)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

type resultReply struct {
	code  int
	raw   string
	error struct{ Code, Message, Hint string }
}

func postResult(t *testing.T, broker *Broker, target, sourceIP, body string, mutate func(*http.Request)) resultReply {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	request.RemoteAddr = sourceIP + ":43210"
	request.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(request)
	}
	recorder := httptest.NewRecorder()
	broker.ResultHandler().ServeHTTP(recorder, request)
	reply := resultReply{code: recorder.Code, raw: recorder.Body.String()}
	var envelope struct {
		Error *struct{ Code, Message, Hint string } `json:"error"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &envelope) == nil && envelope.Error != nil {
		reply.error = *envelope.Error
	}
	return reply
}

// A repo-less session's Pod has no branch label; the route still serves it.
func TestResultRouteSubmitsForTheVerifiedSession(t *testing.T) {
	broker, sink, audit := resultHarness(t)
	reply := postResult(t, broker, resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", `{"result":{"tests_passed":true},"final":true}`, nil)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"accepted":true`) {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
	if len(sink.envelopes) != 1 {
		t.Fatalf("sink got %d submissions", len(sink.envelopes))
	}
	got := sink.envelopes[0]
	if got.Kind != agentsession.ResultKind || got.Workspace != "tea-a" || got.SessionID != "ags-one" || got.PodName != "sbx-a-0" || got.PodUID != "uid-a" ||
		string(got.Result) != `{"tests_passed":true}` || !got.Final {
		t.Fatalf("envelope = %+v", got)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbResult || audit.events[0].Target != "agent-session:ags-one" {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestResultRouteRefusesForeignPods(t *testing.T) {
	cases := map[string]struct{ target, ip string }{
		"unresolved source pod":      {resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.9"},
		"sibling session's pod":      {resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.2"},
		"other workspace's pod":      {resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.4"},
		"URL claims another session": {resultURL(t, "tea-a-sandbox", "ags-two"), "10.0.0.1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, sink, _ := resultHarness(t)
			reply := postResult(t, broker, tc.target, tc.ip, `{"result":1,"final":true}`, nil)
			if reply.code != http.StatusForbidden || reply.error.Code != agentsession.ResultForbidden {
				t.Fatalf("status=%d body=%s, want 403 forbidden", reply.code, reply.raw)
			}
			if len(sink.envelopes) != 0 {
				t.Fatal("a refused submission reached bex-api")
			}
		})
	}
}

func TestResultRouteValidatesTheRequestBeforeForwarding(t *testing.T) {
	cases := map[string]struct {
		body   string
		mutate func(*http.Request)
		code   string
	}{
		"not JSON content type": {`{"result":1,"final":true}`, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, agentsession.ResultInvalidRequest},
		"query string":          {`{"result":1,"final":true}`, func(r *http.Request) { r.URL.RawQuery = "x=1" }, agentsession.ResultInvalidRequest},
		"unknown field":         {`{"result":1,"final":true,"session":"ags-two"}`, nil, agentsession.ResultInvalidRequest},
		"no result":             {`{"final":true}`, nil, agentsession.ResultInvalidRequest},
		"not an object":         {`[1]`, nil, agentsession.ResultInvalidRequest},
		"over the cap":          {`{"result":"` + strings.Repeat("a", agentsession.MaxResultBytes) + `","final":true}`, nil, agentsession.ResultTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, sink, _ := resultHarness(t)
			reply := postResult(t, broker, resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", tc.body, tc.mutate)
			if reply.error.Code != tc.code || len(sink.envelopes) != 0 {
				t.Fatalf("status=%d body=%.200s, want %s and nothing forwarded", reply.code, reply.raw, tc.code)
			}
		})
	}
}

func TestResultRouteRelaysRefusalsAndUnavailability(t *testing.T) {
	broker, sink, _ := resultHarness(t)
	sink.refuse = agentsession.NewResultError(agentsession.ResultSchemaMismatch, "/tests_passed: must be boolean")
	reply := postResult(t, broker, resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", `{"result":{"tests_passed":1},"final":true}`, nil)
	if reply.code != http.StatusUnprocessableEntity || reply.error.Code != agentsession.ResultSchemaMismatch || reply.error.Message != "/tests_passed: must be boolean" {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}

	broker.Results = nil
	reply = postResult(t, broker, resultURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", `{"result":1,"final":true}`, nil)
	if reply.code != http.StatusServiceUnavailable || reply.error.Code != agentsession.ResultUnavailable {
		t.Fatalf("status=%d body=%s, want unavailable without the verb", reply.code, reply.raw)
	}
}
