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

package agentsessions

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// newRESTRequest is a request made as who, for the cases that set a header.
func newRESTRequest(t *testing.T, method, path, body, who string) *http.Request {
	t.Helper()
	return httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(caller(who))
}

func serve(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ADR047 D12 through the REST routes, the GraphQL fields and the MCP tools,
// over the real store: one queue, one shape on every surface.

func newQueueAdapters(t *testing.T) (*lineageAdapters, queueEnv) {
	t.Helper()
	q := newQueueEnv(t)
	mux := http.NewServeMux()
	q.svc.RegisterREST(mux)
	return &lineageAdapters{t: t, e: q.lineageEnv, mux: mux}, q
}

func promptsOf(list any) []string {
	var out []string
	for _, item := range list.([]any) {
		out = append(out, item.(map[string]any)["prompt"].(string))
	}
	return out
}

func TestQueueOverREST(t *testing.T) {
	a, q := newQueueAdapters(t)
	base := "/v1/agent-sessions/" + q.id

	// A steer during the turn answers 202 with the queued message in the body.
	status, body := a.rest(http.MethodPost, base+"/steer", map[string]any{"prompt": "first"}, "alice")
	queued, _ := body["queuedMessage"].(map[string]any)
	if status != http.StatusAccepted || queued == nil || queued["prompt"] != "first" || queued["position"] != float64(1) || queued["state"] != "pending" || body["pendingMessages"] != float64(1) {
		t.Fatalf("steer during a turn = %d %v", status, body)
	}
	status, _ = a.rest(http.MethodPost, base+"/steer", map[string]any{"prompt": "second"}, "alice")
	if status != http.StatusAccepted {
		t.Fatalf("second steer = %d", status)
	}
	// The Idempotency-Key header makes a retried send harmless, as on create.
	raw := func(key string) int {
		req := newRESTRequest(t, http.MethodPost, base+"/steer", `{"prompt":"third"}`, "alice")
		req.Header.Set("Idempotency-Key", key)
		rec := serve(a.mux, req)
		return rec.Code
	}
	if raw("k-1") != http.StatusAccepted || raw("k-1") != http.StatusAccepted {
		t.Fatal("a keyed send was refused")
	}

	status, list := a.rest(http.MethodGet, base+"/queued-messages", nil, "alice")
	if status != http.StatusOK || !slices.Equal(promptsOf(list["queuedMessages"]), []string{"first", "second", "third"}) {
		t.Fatalf("list = %d %v", status, list)
	}
	secondID := list["queuedMessages"].([]any)[1].(map[string]any)["id"].(string)

	status, edited := a.rest(http.MethodPatch, base+"/queued-messages/"+secondID, map[string]any{"prompt": "second, reworded"}, "alice")
	if status != http.StatusOK || edited["prompt"] != "second, reworded" {
		t.Fatalf("edit = %d %v", status, edited)
	}
	status, moved := a.rest(http.MethodPost, base+"/queued-messages/"+secondID+"/move", map[string]any{"position": 1}, "alice")
	if status != http.StatusOK || !slices.Equal(promptsOf(moved["queuedMessages"]), []string{"second, reworded", "first", "third"}) {
		t.Fatalf("move = %d %v", status, moved)
	}
	if status, body = a.rest(http.MethodPost, base+"/queued-messages/"+secondID+"/move", map[string]any{"position": 0}, "alice"); status != http.StatusBadRequest {
		t.Fatalf("move to place 0 = %d %v", status, body)
	}
	if status, _ = a.rest(http.MethodDelete, base+"/queued-messages/"+secondID, nil, "alice"); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, body = a.rest(http.MethodDelete, base+"/queued-messages/"+secondID, nil, "alice"); status != http.StatusNotFound || body["code"] != codeQueuedMessageNotFound {
		t.Fatalf("deleting again = %d %v", status, body)
	}

	// A viewer reads and writes nothing.
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, base + "/queued-messages"}, {http.MethodPost, base + "/interrupt"}, {http.MethodDelete, base + "/queued-messages/" + secondID},
	} {
		if status, _ = a.rest(call.method, call.path, nil, "viewer"); status != http.StatusForbidden {
			t.Errorf("a viewer's %s %s = %d, want 403", call.method, call.path, status)
		}
	}

	// Stop turn: 202, the session is still the running one until the Completer settles it.
	status, view := a.rest(http.MethodPost, base+"/interrupt", nil, "alice")
	if status != http.StatusAccepted || view["id"] != q.id {
		t.Fatalf("interrupt = %d %v", status, view)
	}
	if len(q.lc.interrupts) != 1 {
		t.Fatalf("the driver was asked %v", q.lc.interrupts)
	}
	q.settle(t, q.id)
	if status, body = a.rest(http.MethodPost, base+"/interrupt", nil, "alice"); status != http.StatusConflict || body["code"] != codeNoTurnRunning {
		t.Fatalf("interrupt on an idle session = %d %v", status, body)
	}
	// With messages waiting an idle session still queues; the 202 is the answer.
	if status, _ = a.rest(http.MethodPost, base+"/steer", map[string]any{"prompt": "fourth"}, "alice"); status != http.StatusAccepted {
		t.Fatalf("steer on an idle session with a queue = %d", status)
	}
	// Once the queue drains an idle session starts the turn and answers 200 with no queuedMessage.
	for _, id := range idsOf(mustList(t, a, base)) {
		if status, _ = a.rest(http.MethodDelete, base+"/queued-messages/"+id, nil, "alice"); status != http.StatusNoContent {
			t.Fatalf("delete %s = %d", id, status)
		}
	}
	status, body = a.rest(http.MethodPost, base+"/steer", map[string]any{"prompt": "go"}, "alice")
	if status != http.StatusOK || body["queuedMessage"] != nil || body["turns"] != float64(2) {
		t.Fatalf("steer that starts a turn = %d %v", status, body)
	}
}

func mustList(t *testing.T, a *lineageAdapters, base string) []any {
	t.Helper()
	_, list := a.rest(http.MethodGet, base+"/queued-messages", nil, "alice")
	return list["queuedMessages"].([]any)
}

func TestQueueOverGraphQL(t *testing.T) {
	a, q := newQueueAdapters(t)
	data, code := a.gql(`mutation { steerAgentSession(id: "`+q.id+`", prompt: "queue me", idempotencyKey: "g-1") { turns pendingMessages queuedMessage { id position prompt state author } } }`, "alice")
	if code != "" {
		t.Fatalf("steer: %s", code)
	}
	steered := data["steerAgentSession"].(map[string]any)
	msg := steered["queuedMessage"].(map[string]any)
	if msg["prompt"] != "queue me" || msg["position"] != 1 || msg["state"] != "pending" || msg["author"] != "alice" || steered["turns"] != 1 || steered["pendingMessages"] != 1 {
		t.Fatalf("steer = %v", steered)
	}
	id := msg["id"].(string)
	if _, code = a.gql(`mutation { steerAgentSession(id: "`+q.id+`", prompt: "again") { queuedMessage { id } } }`, "alice"); code != "" {
		t.Fatal(code)
	}

	data, code = a.gql(`{ agentSession(id: "`+q.id+`") { pendingMessages queuedMessages { id prompt position blockedReason } transcript: id } }`, "alice")
	if code != "" {
		t.Fatal(code)
	}
	read := data["agentSession"].(map[string]any)
	if read["pendingMessages"] != 2 || len(read["queuedMessages"].([]any)) != 2 {
		t.Fatalf("session read = %v", read)
	}

	data, code = a.gql(`mutation { editQueuedAgentSessionMessage(id: "`+q.id+`", messageId: "`+id+`", prompt: "queue me, reworded") { prompt author } }`, "bob")
	if code != "" || data["editQueuedAgentSessionMessage"].(map[string]any)["author"] != "bob" {
		t.Fatalf("edit = %v %s", data, code)
	}
	data, code = a.gql(`mutation { moveQueuedAgentSessionMessage(id: "`+q.id+`", messageId: "`+id+`", position: 2) { prompt position } }`, "alice")
	if code != "" || !slices.Equal(promptsOf(data["moveQueuedAgentSessionMessage"]), []string{"again", "queue me, reworded"}) {
		t.Fatalf("move = %v %s", data, code)
	}
	if _, code = a.gql(`mutation { deleteQueuedAgentSessionMessage(id: "`+q.id+`", messageId: "`+id+`") }`, "viewer"); code == "" {
		t.Fatal("a viewer deleted a queued message")
	}
	if data, code = a.gql(`mutation { deleteQueuedAgentSessionMessage(id: "`+q.id+`", messageId: "`+id+`") }`, "alice"); code != "" || data["deleteQueuedAgentSessionMessage"] != true {
		t.Fatalf("delete = %v %s", data, code)
	}
	if _, code = a.gql(`mutation { deleteQueuedAgentSessionMessage(id: "`+q.id+`", messageId: "`+id+`") }`, "alice"); code != codeQueuedMessageNotFound {
		t.Fatalf("deleting again = %q", code)
	}
	data, code = a.gql(`mutation { interruptAgentSession(id: "`+q.id+`") { id phase } }`, "alice")
	if code != "" || data["interruptAgentSession"].(map[string]any)["phase"] != PhaseRunning || len(q.lc.interrupts) != 1 {
		t.Fatalf("interrupt = %v %s", data, code)
	}
	q.settle(t, q.id)
	if _, code = a.gql(`mutation { interruptAgentSession(id: "`+q.id+`") { id } }`, "alice"); code != codeNoTurnRunning {
		t.Fatalf("interrupt on an idle session = %q", code)
	}
	// A turn carries interrupted.
	data, code = a.gql(`{ agentSessionTranscript(id: "`+q.id+`") { turns { turn interrupted } } }`, "alice")
	if code != "" {
		t.Fatal(code)
	}
	if turns := data["agentSessionTranscript"].(map[string]any)["turns"].([]any); len(turns) != 1 || turns[0].(map[string]any)["interrupted"] != false {
		t.Fatalf("turns = %v", turns)
	}
}

func TestQueueOverMCP(t *testing.T) {
	a, q := newQueueAdapters(t)
	out, errText := a.mcp("send_agent_session_message", map[string]any{"id": q.id, "prompt": "one", "idempotencyKey": "m-1"}, "alice")
	queued, _ := out["queuedMessage"].(map[string]any)
	if errText != "" || queued == nil || queued["position"] != float64(1) {
		t.Fatalf("send = %v %s", out, errText)
	}
	if out, errText = a.mcp("steer_agent_session", map[string]any{"id": q.id, "prompt": "two"}, "alice"); errText != "" || out["queuedMessage"] == nil {
		t.Fatalf("steer alias = %v %s", out, errText)
	}
	list, errText := a.mcp("list_agent_session_queued_messages", map[string]any{"id": q.id}, "alice")
	if errText != "" || !slices.Equal(promptsOf(list["queuedMessages"]), []string{"one", "two"}) {
		t.Fatalf("list = %v %s", list, errText)
	}
	id := queued["id"].(string)
	args := map[string]any{"id": q.id, "messageId": id}
	if out, errText = a.mcp("edit_agent_session_queued_message", map[string]any{"id": q.id, "messageId": id, "prompt": "one, reworded"}, "alice"); errText != "" || out["prompt"] != "one, reworded" {
		t.Fatalf("edit = %v %s", out, errText)
	}
	if out, errText = a.mcp("move_agent_session_queued_message", map[string]any{"id": q.id, "messageId": id, "position": 2}, "alice"); errText != "" || !slices.Equal(promptsOf(out["queuedMessages"]), []string{"two", "one, reworded"}) {
		t.Fatalf("move = %v %s", out, errText)
	}
	if _, errText = a.mcp("delete_agent_session_queued_message", args, "viewer"); errText == "" {
		t.Fatal("a viewer deleted a queued message")
	}
	if out, errText = a.mcp("delete_agent_session_queued_message", args, "alice"); errText != "" || out["deleted"] != true {
		t.Fatalf("delete = %v %s", out, errText)
	}
	if out, errText = a.mcp("interrupt_agent_session", map[string]any{"id": q.id}, "alice"); errText != "" || out["id"] != q.id || len(q.lc.interrupts) != 1 {
		t.Fatalf("interrupt = %v %s", out, errText)
	}
	// The tools are in the cloud-agents toolset an orchestrator is given.
	for _, name := range []string{"list_agent_session_queued_messages", "edit_agent_session_queued_message", "delete_agent_session_queued_message", "move_agent_session_queued_message", "interrupt_agent_session"} {
		if !slices.Contains(CloudAgentsTools, name) {
			t.Errorf("%s is not in the cloud-agents toolset", name)
		}
	}
	if got := q.phase(t, q.id); got != PhaseRunning {
		t.Fatalf("phase = %s", got)
	}
}
