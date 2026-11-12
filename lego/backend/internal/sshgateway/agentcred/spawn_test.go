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
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// recordingSpawns is bex-api's spawn verbs with the service replaced: it
// records the signed envelopes the gateway sent.
type recordingSpawns struct {
	envelopes []agentsession.SpawnEnvelope
	refuse    *agentsession.SpawnError
	// duringWait runs inside a wait, while the gateway holds it open.
	duringWait func()
}

func (s *recordingSpawns) SpawnSession(_ context.Context, envelope agentsession.SpawnEnvelope) (agentsession.SpawnResponse, error) {
	s.envelopes = append(s.envelopes, envelope)
	if s.refuse != nil {
		return agentsession.SpawnResponse{}, s.refuse
	}
	return agentsession.SpawnResponse{SessionID: "ags-child", State: "working", Created: true}, nil
}

func (s *recordingSpawns) ReadChildSession(_ context.Context, envelope agentsession.SpawnEnvelope) (agentsession.ChildView, error) {
	s.envelopes = append(s.envelopes, envelope)
	if s.refuse != nil {
		return agentsession.ChildView{}, s.refuse
	}
	return agentsession.ChildView{SessionID: envelope.ChildID, ParentSessionID: envelope.SessionID, State: "succeeded", Outcome: json.RawMessage(`{"result":{"n":1}}`)}, nil
}

func (s *recordingSpawns) MessageChildSession(_ context.Context, envelope agentsession.SpawnEnvelope) (agentsession.ChildView, error) {
	s.envelopes = append(s.envelopes, envelope)
	if s.refuse != nil {
		return agentsession.ChildView{}, s.refuse
	}
	return agentsession.ChildView{SessionID: envelope.ChildID, ParentSessionID: envelope.SessionID, State: "working"}, nil
}

func (s *recordingSpawns) CancelChildSession(_ context.Context, envelope agentsession.SpawnEnvelope) (agentsession.ChildView, error) {
	s.envelopes = append(s.envelopes, envelope)
	if s.refuse != nil {
		return agentsession.ChildView{}, s.refuse
	}
	return agentsession.ChildView{SessionID: envelope.ChildID, ParentSessionID: envelope.SessionID, State: "canceled"}, nil
}

func (s *recordingSpawns) WaitChildSessions(_ context.Context, envelope agentsession.SpawnEnvelope) (agentsession.SpawnWaitResponse, error) {
	s.envelopes = append(s.envelopes, envelope)
	if s.duringWait != nil {
		s.duringWait()
	}
	if s.refuse != nil {
		return agentsession.SpawnWaitResponse{}, s.refuse
	}
	return agentsession.SpawnWaitResponse{Settled: envelope.Wait.ChildIDs, Children: []agentsession.SpawnWaitChild{{SessionID: envelope.Wait.ChildIDs[0], State: "succeeded"}}}, nil
}

func spawnHarness(t *testing.T) (*Broker, *recordingSpawns, *credentialAudit) {
	t.Helper()
	secret := []byte("spawn-secret")
	sink := &recordingSpawns{}
	api := httptest.NewServer(&agentsession.SpawnHandler{Secret: secret, Spawns: sink, Nonce: &onceNonces{seen: map[string]bool{}}})
	t.Cleanup(api.Close)
	audit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: repoLessLabels(t, "tea-a", "ags-one")},
			"10.0.0.2": {Name: "sbx-b-0", UID: "uid-b", Labels: credentialLabels(t, "tea-a", "ags-two", "bex-agent/task-1")},
			"10.0.0.4": {Name: "sbx-a-0", UID: "uid-d", Labels: credentialLabels(t, "tea-b", "ags-one", "bex-agent/task-1")},
		}},
		API:    &agentsession.Client{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Spawns: &agentsession.SpawnClient{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Audit:  audit,
	}
	return broker, sink, audit
}

func spawnURL(t *testing.T, namespace, session string) string {
	t.Helper()
	u, err := agentsession.SpawnURL("http://gateway.internal:8082", namespace, session)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

type spawnReply struct {
	code  int
	raw   string
	error struct{ Code, Message, Hint string }
}

func callSpawn(t *testing.T, broker *Broker, method, target, sourceIP, body string, mutate func(*http.Request)) spawnReply {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.RemoteAddr = sourceIP + ":43210"
	request.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(request)
	}
	recorder := httptest.NewRecorder()
	broker.SpawnHandler().ServeHTTP(recorder, request)
	reply := spawnReply{code: recorder.Code, raw: recorder.Body.String()}
	var envelope struct {
		Error *struct{ Code, Message, Hint string } `json:"error"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &envelope) == nil && envelope.Error != nil {
		reply.error = *envelope.Error
	}
	return reply
}

// The parent is the session the verified Pod belongs to; the sandbox's body
// supplies only the child's own fields.
func TestSpawnRouteStartsAChildOfTheVerifiedSession(t *testing.T) {
	broker, sink, audit := spawnHarness(t)
	reply := callSpawn(t, broker, http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1",
		`{"task":"fix the build","tags":["a"],"secretNames":[],"idempotencyKey":"k1"}`, nil)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"sessionId":"ags-child"`) {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
	if len(sink.envelopes) != 1 {
		t.Fatalf("sink got %d requests", len(sink.envelopes))
	}
	got := sink.envelopes[0]
	if got.Kind != agentsession.SpawnKind || got.Op != agentsession.SpawnOpCreate || got.Workspace != "tea-a" || got.SessionID != "ags-one" ||
		got.PodName != "sbx-a-0" || got.PodUID != "uid-a" || got.ChildID != "" {
		t.Fatalf("envelope = %+v", got)
	}
	if got.Spawn == nil || got.Spawn.Task != "fix the build" || got.Spawn.IdempotencyKey != "k1" ||
		got.Spawn.SecretNames == nil || len(*got.Spawn.SecretNames) != 0 {
		t.Fatalf("spawn content = %+v (an empty secretNames list must survive both hops)", got.Spawn)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbSpawn || audit.events[0].Target != "agent-session:ags-one" {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestSpawnRouteRefusesAnotherSessionsToken(t *testing.T) {
	childID := id.New(id.AgentSession)
	cases := map[string]struct{ method, target, ip string }{
		"unresolved source pod":       {http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.9"},
		"sibling session's pod":       {http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.2"},
		"other workspace's pod":       {http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.4"},
		"URL claims another session":  {http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-two"), "10.0.0.1"},
		"read claims another session": {http.MethodGet, spawnURL(t, "tea-a-sandbox", "ags-two") + "/" + childID, "10.0.0.1"},
		"read from a sibling's pod":   {http.MethodGet, spawnURL(t, "tea-a-sandbox", "ags-one") + "/" + childID, "10.0.0.2"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, sink, _ := spawnHarness(t)
			body := `{"task":"t"}`
			if tc.method == http.MethodGet {
				body = ""
			}
			reply := callSpawn(t, broker, tc.method, tc.target, tc.ip, body, nil)
			if reply.code != http.StatusForbidden || reply.error.Code != agentsession.SpawnForbidden {
				t.Fatalf("status=%d body=%s, want 403 forbidden", reply.code, reply.raw)
			}
			if len(sink.envelopes) != 0 {
				t.Fatal("a refused request reached bex-api")
			}
		})
	}
}

func TestSpawnRouteBodyCannotNameTheParentOrAnythingElse(t *testing.T) {
	cases := map[string]struct {
		body   string
		mutate func(*http.Request)
		code   string
	}{
		"parentSessionId":       {`{"task":"t","parentSessionId":"ags-two"}`, nil, agentsession.SpawnInvalidRequest},
		"sessionId":             {`{"task":"t","sessionId":"ags-two"}`, nil, agentsession.SpawnInvalidRequest},
		"agent override":        {`{"task":"t","agent":"codex"}`, nil, agentsession.SpawnInvalidRequest},
		"inline secrets":        {`{"task":"t","secrets":[{"name":"X","value":"v"}]}`, nil, agentsession.SpawnInvalidRequest},
		"no task":               {`{"repo":"octo/repo"}`, nil, agentsession.SpawnInvalidRequest},
		"blank task":            {`{"task":"   "}`, nil, agentsession.SpawnInvalidRequest},
		"not an object":         {`[1]`, nil, agentsession.SpawnInvalidRequest},
		"long idempotency key":  {`{"task":"t","idempotencyKey":"` + strings.Repeat("k", 65) + `"}`, nil, agentsession.SpawnInvalidRequest},
		"not JSON content type": {`{"task":"t"}`, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, agentsession.SpawnInvalidRequest},
		"query string":          {`{"task":"t"}`, func(r *http.Request) { r.URL.RawQuery = "parent=ags-two" }, agentsession.SpawnInvalidRequest},
		"over the cap":          {`{"task":"` + strings.Repeat("a", agentsession.MaxSpawnTaskBytes+1) + `"}`, nil, agentsession.SpawnTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, sink, _ := spawnHarness(t)
			reply := callSpawn(t, broker, http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", tc.body, tc.mutate)
			if reply.error.Code != tc.code || len(sink.envelopes) != 0 {
				t.Fatalf("status=%d body=%.200s, want %s and nothing forwarded", reply.code, reply.raw, tc.code)
			}
		})
	}
}

func TestSpawnRouteReadsOneNamedDescendantAndNothingElse(t *testing.T) {
	broker, sink, audit := spawnHarness(t)
	childID := id.New(id.AgentSession)
	target := spawnURL(t, "tea-a-sandbox", "ags-one") + "/" + childID
	reply := callSpawn(t, broker, http.MethodGet, target, "10.0.0.1", "", nil)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"state":"succeeded"`) || !strings.Contains(reply.raw, `"n":1`) {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
	got := sink.envelopes[0]
	if got.Op != agentsession.SpawnOpRead || got.SessionID != "ags-one" || got.ChildID != childID || got.Spawn != nil {
		t.Fatalf("envelope = %+v", got)
	}
	if audit.events[0].Verb != agentsession.AuditVerbReadChild {
		t.Fatalf("audit = %+v", audit.events)
	}

	// The wrong method for the shape of the URL, a non-session id, no id.
	for name, tc := range map[string]struct{ method, target string }{
		"POST a spawn body to a child": {http.MethodPost, target},
		"GET without an id":            {http.MethodGet, spawnURL(t, "tea-a-sandbox", "ags-one")},
		"GET a traversal":              {http.MethodGet, spawnURL(t, "tea-a-sandbox", "ags-one") + "/..%2Fags-two"},
		"GET a foreign kind":           {http.MethodGet, spawnURL(t, "tea-a-sandbox", "ags-one") + "/app-abcdefghij"},
		"DELETE the collection":        {http.MethodDelete, spawnURL(t, "tea-a-sandbox", "ags-one")},
		"GET the wait route":           {http.MethodGet, spawnURL(t, "tea-a-sandbox", "ags-one") + "/wait"},
	} {
		before := len(sink.envelopes)
		reply := callSpawn(t, broker, tc.method, tc.target, "10.0.0.1", `{"task":"t"}`, nil)
		if reply.error.Code != agentsession.SpawnInvalidRequest || len(sink.envelopes) != before {
			t.Fatalf("%s: status=%d body=%.200s", name, reply.code, reply.raw)
		}
	}
}

func TestSpawnRouteRelaysCreatePathRefusalsVerbatim(t *testing.T) {
	broker, sink, _ := spawnHarness(t)
	for _, code := range []string{"AGENT_SESSION_DEPTH_LIMIT", "AGENT_SESSION_CHILD_LIMIT", "AGENT_SESSION_CHILD_SPEND_EXCEEDS_PARENT", "AGENT_SESSION_CHILD_SECRET_NOT_INHERITED"} {
		sink.refuse = agentsession.NewSpawnError(code, "message of "+code)
		reply := callSpawn(t, broker, http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", `{"task":"t"}`, nil)
		if reply.error.Code != code || reply.error.Message != "message of "+code || reply.error.Hint == "" {
			t.Fatalf("%s: status=%d body=%s", code, reply.code, reply.raw)
		}
	}
	broker.Spawns = nil
	reply := callSpawn(t, broker, http.MethodPost, spawnURL(t, "tea-a-sandbox", "ags-one"), "10.0.0.1", `{"task":"t"}`, nil)
	if reply.code != http.StatusServiceUnavailable || reply.error.Code != agentsession.SpawnUnavailable {
		t.Fatalf("status=%d body=%s, want unavailable without the verb", reply.code, reply.raw)
	}
}

// The coordinator routes (w9/m346): message and cancel one descendant, wait
// for several, each a signed envelope naming the verified parent; bodies are
// strict and bounded before anything is forwarded.
func TestSpawnRouteCoordinatesDescendants(t *testing.T) {
	broker, sink, audit := spawnHarness(t)
	base := spawnURL(t, "tea-a-sandbox", "ags-one")
	childID := id.New(id.AgentSession)

	reply := callSpawn(t, broker, http.MethodPost, base+"/"+childID, "10.0.0.1", `{"prompt":"also add tests"}`, nil)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"state":"working"`) {
		t.Fatalf("message: status=%d body=%s", reply.code, reply.raw)
	}
	if got := sink.envelopes[0]; got.Op != agentsession.SpawnOpMessage || got.SessionID != "ags-one" || got.ChildID != childID || got.Message == nil || got.Message.Prompt != "also add tests" {
		t.Fatalf("message envelope = %+v", got)
	}
	reply = callSpawn(t, broker, http.MethodDelete, base+"/"+childID, "10.0.0.1", "", nil)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"state":"canceled"`) || sink.envelopes[1].Op != agentsession.SpawnOpCancel {
		t.Fatalf("cancel: status=%d body=%s envelope=%+v", reply.code, reply.raw, sink.envelopes[1])
	}
	reply = callSpawn(t, broker, http.MethodPost, base+"/wait", "10.0.0.1", `{"childIds":["`+childID+`"]}`, nil)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"settled":["`+childID+`"]`) {
		t.Fatalf("wait: status=%d body=%s", reply.code, reply.raw)
	}
	// A wait reads no lineage, so it must not answer a zero parent and depth.
	if strings.Contains(reply.raw, "parentSessionId") || strings.Contains(reply.raw, `"depth"`) {
		t.Fatalf("wait answers lineage it never read: %s", reply.raw)
	}
	if got := sink.envelopes[2]; got.Op != agentsession.SpawnOpWait || got.Wait == nil || got.Wait.TimeoutSeconds != 0 {
		t.Fatalf("wait envelope = %+v (an omitted deadline is the wait verb's default)", got)
	}
	if audit.events[1].TargetName != childID {
		t.Fatalf("cancel audit names %q, want the child", audit.events[1].TargetName)
	}
	// A wait idles past admission without the sandbox's only slot: the
	// sandbox can still read a child while it is in flight.
	broker.Limits = sshgateway.NewSessionLimiter(1, 1)
	sink.duringWait = func() {
		if read := callSpawn(t, broker, http.MethodGet, base+"/"+childID, "10.0.0.1", "", nil); read.code != http.StatusOK {
			t.Errorf("a read during a wait: status=%d body=%s", read.code, read.raw)
		}
	}
	if reply := callSpawn(t, broker, http.MethodPost, base+"/wait", "10.0.0.1", `{"childIds":["`+childID+`"],"timeoutSeconds":5}`, nil); reply.code != http.StatusOK {
		t.Fatalf("wait under a one-slot limit: status=%d body=%s", reply.code, reply.raw)
	}
	sink.duringWait = nil
	verbs := []string{audit.events[0].Verb, audit.events[1].Verb, audit.events[2].Verb}
	if verbs[0] != agentsession.AuditVerbMessageChild || verbs[1] != agentsession.AuditVerbCancelChild || verbs[2] != agentsession.AuditVerbWaitChildren {
		t.Fatalf("audit verbs = %v", verbs)
	}

	for name, tc := range map[string]struct{ target, body string }{
		"an empty prompt":        {base + "/" + childID, `{"prompt":"  "}`},
		"a message naming more":  {base + "/" + childID, `{"prompt":"x","childId":"ags-two"}`},
		"no children to wait on": {base + "/wait", `{"childIds":[]}`},
		"a wait past 25 s":       {base + "/wait", `{"childIds":["` + childID + `"],"timeoutSeconds":26}`},
		"a wait on a non-id":     {base + "/wait", `{"childIds":["app-abc"]}`},
	} {
		before := len(sink.envelopes)
		reply := callSpawn(t, broker, http.MethodPost, tc.target, "10.0.0.1", tc.body, nil)
		if reply.error.Code != agentsession.SpawnInvalidRequest || len(sink.envelopes) != before {
			t.Fatalf("%s: status=%d body=%.200s", name, reply.code, reply.raw)
		}
	}
	if reply := callSpawn(t, broker, http.MethodPost, base+"/wait?x=1", "10.0.0.1", `{"childIds":["`+childID+`"]}`, nil); reply.error.Code != agentsession.SpawnInvalidRequest {
		t.Fatalf("a query string was accepted: %d %s", reply.code, reply.raw)
	}
}
