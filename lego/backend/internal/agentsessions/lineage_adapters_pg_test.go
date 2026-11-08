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
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR102 D8: lineage, the two list filters and the tree read, through the REST
// routes, the GraphQL fields and the MCP tools over the real store.

type lineageAdapters struct {
	t   *testing.T
	e   lineageEnv
	mux *http.ServeMux
}

func newLineageAdapters(t *testing.T) *lineageAdapters {
	t.Helper()
	e := newLineageEnv(t)
	mux := http.NewServeMux()
	e.svc.RegisterREST(mux)
	return &lineageAdapters{t: t, e: e, mux: mux}
}

func (a *lineageAdapters) rest(method, path string, body any, who string) (int, map[string]any) {
	a.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		a.t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(string(raw))).WithContext(caller(who)))
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			// a bare array: wrap it so callers read one shape
			var list []any
			if err2 := json.Unmarshal(rec.Body.Bytes(), &list); err2 != nil {
				a.t.Fatalf("%s %s: body %q is not JSON: %v", method, path, rec.Body.String(), err)
			}
			out = map[string]any{"list": list}
		}
	}
	return rec.Code, out
}

func (a *lineageAdapters) gql(query, who string) (map[string]any, string) {
	a.t.Helper()
	result := graphql.Do(graphql.Params{Schema: environmentSchema(a.t, a.e.svc), Context: caller(who), RequestString: query})
	if len(result.Errors) > 0 {
		code, _ := result.Errors[0].Extensions["code"].(string)
		if code == "" {
			a.t.Fatalf("GraphQL error without a code: %v", result.Errors)
		}
		return nil, code
	}
	data, _ := result.Data.(map[string]any)
	return data, ""
}

func (a *lineageAdapters) mcp(tool string, args map[string]any, who string) (map[string]any, string) {
	a.t.Helper()
	res, err := newMCPClient(a.t, caller(who), a.e.svc).CallTool(caller(who), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		a.t.Fatalf("MCP %s: %v", tool, err)
	}
	if res.IsError {
		return nil, res.Content[0].(*mcp.TextContent).Text
	}
	out, _ := res.StructuredContent.(map[string]any)
	return out, ""
}

func idsOf(list []any) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.(map[string]any)["id"].(string))
	}
	return out
}

func TestLineageIsTheSameOnEverySurface(t *testing.T) {
	a := newLineageAdapters(t)
	e := a.e
	parent := e.create(t, "")
	body := map[string]any{"ownerId": e.ws, "repo": "bex-co/example", "agentConfig": map[string]any{"agent": "codex", "model": "gpt-5", "task": "child"}, "parentSessionId": parent.ID}

	// Create a child through each surface.
	status, restChild := a.rest(http.MethodPost, "/v1/agent-sessions", body, "alice")
	if status != http.StatusCreated || restChild["parentSessionId"] != parent.ID || restChild["rootSessionId"] != parent.ID || restChild["depth"] != float64(1) {
		t.Fatalf("REST child = %d %v", status, restChild)
	}
	data, code := a.gql(`mutation { createAgentSession(ownerId: "`+e.ws+`", repo: "bex-co/example", agentConfig: {agent: "codex", model: "gpt-5", task: "child"}, parentSessionId: "`+parent.ID+`") { id parentSessionId rootSessionId depth } }`, "alice")
	if code != "" {
		t.Fatalf("GraphQL create: %s", code)
	}
	gqlChild := data["createAgentSession"].(map[string]any)
	if gqlChild["parentSessionId"] != parent.ID || gqlChild["rootSessionId"] != parent.ID || gqlChild["depth"] != 1 {
		t.Fatalf("GraphQL child = %v", gqlChild)
	}
	mcpChild, errText := a.mcp("create_agent_session", map[string]any{"ownerId": e.ws, "repo": "bex-co/example", "agentConfig": map[string]any{"agent": "codex", "model": "gpt-5", "task": "child"}, "parentSessionId": parent.ID}, "alice")
	if errText != "" || mcpChild["parentSessionId"] != parent.ID || mcpChild["rootSessionId"] != parent.ID || mcpChild["depth"] != float64(1) {
		t.Fatalf("MCP child = %v %s", mcpChild, errText)
	}
	wantKids := []string{restChild["id"].(string), gqlChild["id"].(string), mcpChild["id"].(string)}
	slices.Sort(wantKids)

	// A grandchild under the REST child, then an unrelated top-level session
	// started by an automation.
	grand := e.create(t, restChild["id"].(string))
	other := e.input("")
	other.AutomationID = ids.New(ids.AgentAutomation)
	automated, err := e.svc.Create(caller("alice"), other)
	if err != nil {
		t.Fatal(err)
	}

	// The parent filter returns exactly the direct children on every surface.
	status, list := a.rest(http.MethodGet, "/v1/agent-sessions?ownerId="+e.ws+"&parentSessionId="+parent.ID, nil, "alice")
	got := idsOf(list["list"].([]any))
	slices.Sort(got)
	if status != 200 || !slices.Equal(got, wantKids) {
		t.Fatalf("REST parent filter = %d %v, want %v", status, got, wantKids)
	}
	data, code = a.gql(`{ agentSessions(ownerId: "`+e.ws+`", parentSessionId: "`+parent.ID+`") { id parentSessionId depth } }`, "alice")
	if code != "" {
		t.Fatal(code)
	}
	var gqlIDs []string
	for _, item := range data["agentSessions"].([]any) {
		gqlIDs = append(gqlIDs, item.(map[string]any)["id"].(string))
	}
	slices.Sort(gqlIDs)
	if !slices.Equal(gqlIDs, wantKids) {
		t.Fatalf("GraphQL parent filter = %v, want %v", gqlIDs, wantKids)
	}
	out, errText := a.mcp("list_agent_sessions", map[string]any{"ownerId": e.ws, "parentSessionId": parent.ID}, "alice")
	mcpIDs := idsOf(out["agentSessions"].([]any))
	slices.Sort(mcpIDs)
	if errText != "" || !slices.Equal(mcpIDs, wantKids) {
		t.Fatalf("MCP parent filter = %v %s, want %v", mcpIDs, errText, wantKids)
	}

	// The automation filter returns exactly the automation's sessions.
	_, list = a.rest(http.MethodGet, "/v1/agent-sessions?ownerId="+e.ws+"&automationId="+other.AutomationID, nil, "alice")
	if got := idsOf(list["list"].([]any)); !slices.Equal(got, []string{automated.ID}) {
		t.Fatalf("REST automation filter = %v", got)
	}
	data, _ = a.gql(`{ agentSessions(ownerId: "`+e.ws+`", automationId: "`+other.AutomationID+`") { id automationId } }`, "alice")
	if items := data["agentSessions"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != automated.ID {
		t.Fatalf("GraphQL automation filter = %v", items)
	}
	out, _ = a.mcp("list_agent_sessions", map[string]any{"ownerId": e.ws, "automationId": other.AutomationID}, "alice")
	if got := idsOf(out["agentSessions"].([]any)); !slices.Equal(got, []string{automated.ID}) {
		t.Fatalf("MCP automation filter = %v", got)
	}

	// The tree of a middle node is the whole tree of its root, ordered by depth
	// then creation, on every surface.
	middle := restChild["id"].(string)
	wantTree := []string{parent.ID}
	rows, err := e.pg.AgentSessionSubtree(context.Background(), e.ws, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantTree = wantTree[:0]
	for _, r := range rows {
		wantTree = append(wantTree, r.ID)
	}
	if len(wantTree) != 5 || wantTree[0] != parent.ID || wantTree[4] != grand.ID {
		t.Fatalf("tree rows = %v", wantTree)
	}
	status, tree := a.rest(http.MethodGet, "/v1/agent-sessions/"+middle+"/tree", nil, "alice")
	if status != 200 || tree["rootSessionId"] != parent.ID || !slices.Equal(idsOf(tree["sessions"].([]any)), wantTree) {
		t.Fatalf("REST tree = %d %v, want %v", status, tree, wantTree)
	}
	last := tree["sessions"].([]any)[4].(map[string]any)
	if last["parentSessionId"] != middle || last["depth"] != float64(2) || last["state"] == nil || last["phase"] == nil {
		t.Fatalf("tree item lacks lineage or state: %v", last)
	}
	data, code = a.gql(`{ agentSessionTree(id: "`+middle+`") { rootSessionId sessions { id parentSessionId depth state } } }`, "alice")
	if code != "" {
		t.Fatal(code)
	}
	gt := data["agentSessionTree"].(map[string]any)
	var gqlTree []string
	for _, item := range gt["sessions"].([]any) {
		gqlTree = append(gqlTree, item.(map[string]any)["id"].(string))
	}
	if gt["rootSessionId"] != parent.ID || !slices.Equal(gqlTree, wantTree) {
		t.Fatalf("GraphQL tree = %v, want %v", gt, wantTree)
	}
	out, errText = a.mcp("get_agent_session_tree", map[string]any{"id": middle}, "alice")
	if errText != "" || out["rootSessionId"] != parent.ID || !slices.Equal(idsOf(out["sessions"].([]any)), wantTree) {
		t.Fatalf("MCP tree = %v %s, want %v", out, errText, wantTree)
	}
	// A top-level session with no children is a tree of one.
	_, tree = a.rest(http.MethodGet, "/v1/agent-sessions/"+automated.ID+"/tree", nil, "alice")
	if tree["rootSessionId"] != automated.ID || len(tree["sessions"].([]any)) != 1 {
		t.Fatalf("single tree = %v", tree)
	}
}

func TestTreeReadIsWorkspaceScopedAndFollowsReadAccess(t *testing.T) {
	a := newLineageAdapters(t)
	e := a.e
	root := e.create(t, "")
	child := e.create(t, root.ID)

	// Another workspace's member: a 404 on every surface, never the tree.
	e.fga.members["mallory"] = "tea-elsewhere"
	// (The API server's real authorizer turns this denial into the 404 the
	// cross-workspace matrix pins; here it must be exactly what a Get answers.)
	getStatus, _ := a.rest(http.MethodGet, "/v1/agent-sessions/"+child.ID, nil, "mallory")
	if status, body := a.rest(http.MethodGet, "/v1/agent-sessions/"+child.ID+"/tree", nil, "mallory"); status != getStatus || status < 400 {
		t.Fatalf("cross-workspace tree = %d %v, a Get answers %d", status, body, getStatus)
	}
	if _, code := a.gql(`{ agentSessionTree(id: "`+child.ID+`") { rootSessionId } }`, "mallory"); code == "" {
		t.Fatal("GraphQL cross-workspace tree was served")
	}
	if _, errText := a.mcp("get_agent_session_tree", map[string]any{"id": child.ID}, "mallory"); errText == "" {
		t.Fatal("MCP cross-workspace tree was served")
	}
	// An unknown id of the right shape in this workspace is a 404.
	unknown := ids.New(ids.AgentSession)
	e.fga.parents[unknown] = e.ws
	if status, _ := a.rest(http.MethodGet, "/v1/agent-sessions/"+unknown+"/tree", nil, "alice"); status != http.StatusNotFound {
		t.Fatalf("unknown tree = %d, want 404", status)
	}

	// A viewer reads sessions (can_operate here) but cannot create a child.
	e.fga.members["viewer"] = e.ws
	inner := e.svc.Authz
	e.svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		if strings.TrimPrefix(subject, "user:") == "viewer" && relation == core.RelCanCreate {
			return false, nil
		}
		return inner.Check(ctx, subject, relation, object)
	})
	if status, tree := a.rest(http.MethodGet, "/v1/agent-sessions/"+root.ID+"/tree", nil, "viewer"); status != 200 || len(tree["sessions"].([]any)) != 2 {
		t.Fatalf("viewer tree read = %d %v", status, tree)
	}
	body := map[string]any{"ownerId": e.ws, "repo": "bex-co/example", "agentConfig": map[string]any{"agent": "codex", "model": "gpt-5", "task": "x"}, "parentSessionId": root.ID}
	if status, _ := a.rest(http.MethodPost, "/v1/agent-sessions", body, "viewer"); status != http.StatusForbidden {
		t.Fatalf("viewer child create = %d, want 403", status)
	}
}

func TestLineageFiltersRefuseMalformedValuesWithANamed400(t *testing.T) {
	a := newLineageAdapters(t)
	e := a.e
	for _, tc := range []struct{ name, query string }{
		{"NUL parent", "parentSessionId=" + url.QueryEscape("ags-\x00x")},
		{"NUL automation", "automationId=" + url.QueryEscape("aut-\x00x")},
		{"wrong kind parent", "parentSessionId=" + ids.New(ids.AgentAutomation)},
		{"wrong kind automation", "automationId=" + ids.New(ids.AgentSession)},
		{"not an id", "parentSessionId=nope"},
	} {
		status, body := a.rest(http.MethodGet, "/v1/agent-sessions?ownerId="+e.ws+"&"+tc.query, nil, "alice")
		if status != http.StatusBadRequest || body["code"] != "AGENT_SESSION_FILTER_INVALID" {
			t.Errorf("%s: REST = %d %v, want 400 AGENT_SESSION_FILTER_INVALID", tc.name, status, body)
		}
	}
	_, code := a.gql(`{ agentSessions(ownerId: "`+e.ws+`", parentSessionId: "ags-\u0000x") { id } }`, "alice")
	if code != "AGENT_SESSION_FILTER_INVALID" {
		t.Errorf("GraphQL NUL filter code = %q", code)
	}
	_, errText := a.mcp("list_agent_sessions", map[string]any{"ownerId": e.ws, "automationId": "aut-\x00x"}, "alice")
	if !strings.Contains(errText, "automationId") {
		t.Errorf("MCP NUL filter error = %q", errText)
	}
	// A NUL path id is refused like a Get of it, never a 500.
	getStatus, _ := a.rest(http.MethodGet, "/v1/agent-sessions/"+url.PathEscape("ags-\x00x"), nil, "alice")
	if status, body := a.rest(http.MethodGet, "/v1/agent-sessions/"+url.PathEscape("ags-\x00x")+"/tree", nil, "alice"); status != getStatus || status >= 500 {
		t.Errorf("NUL tree id = %d %v, a Get answers %d", status, body, getStatus)
	}
}

// Every named refusal of a child create (ADR102 D2 to D4) is the same code,
// with the same class of HTTP status, on REST, GraphQL and MCP: the adapters
// add no rule and rename none.
func TestChildCreateRefusalsCarryTheSameCodeOnEverySurface(t *testing.T) {
	a := newLineageAdapters(t)
	e := a.e
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", []string{e.ws}, "sk-platform-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
	root := func(usd *float64) View {
		t.Helper()
		in := e.input("")
		in.AgentConfig, in.MaxSpendUSD = AgentConfig{Agent: "claude", Task: "split the work"}, usd
		view, err := e.svc.Create(caller("alice"), in)
		if err != nil {
			t.Fatal(err)
		}
		return view
	}

	unknown := ids.New(ids.AgentSession)
	e.fga.parents[unknown] = e.ws
	finished := root(nil)
	e.setPhase(t, finished.ID, PhaseCompleted)
	deep := root(nil)
	for range store.MaxAgentSessionDepth {
		deep = e.create(t, deep.ID)
	}
	full := root(nil)
	for range store.MaxLiveAgentSessionChildren {
		e.create(t, full.ID)
	}
	capped := root(ptr(5.0))
	spent := root(ptr(1.0))
	e.spend(t, spent.ID, 1_000_000)

	for _, tc := range []struct {
		name   string
		args   map[string]any
		status int
		code   string
	}{
		{"parent not found", map[string]any{"parentSessionId": unknown}, http.StatusNotFound, ParentNotFoundCode},
		{"parent not active", map[string]any{"parentSessionId": finished.ID}, http.StatusConflict, ParentNotActiveCode},
		{"depth limit", map[string]any{"parentSessionId": deep.ID}, http.StatusConflict, DepthLimitCode},
		{"child limit", map[string]any{"parentSessionId": full.ID}, http.StatusConflict, ChildLimitCode},
		{"spend above the parent's", map[string]any{"parentSessionId": capped.ID, "maxSpendUSD": 6.0}, http.StatusBadRequest, ChildSpendExceedsParentCode},
		{"spend exhausted", map[string]any{"parentSessionId": spent.ID}, http.StatusConflict, ChildSpendExhaustedCode},
		{"secret not inherited", map[string]any{"parentSessionId": capped.ID, "secretNames": []string{"NOT_THE_PARENTS"}}, http.StatusBadRequest, ChildSecretNotInheritedCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"ownerId": e.ws, "repo": "bex-co/example", "agentConfig": map[string]any{"agent": "claude", "task": "x"}}
			maps.Copy(body, tc.args)
			status, restBody := a.rest(http.MethodPost, "/v1/agent-sessions", body, "alice")
			if status != tc.status || restBody["code"] != tc.code {
				t.Errorf("REST = %d %v, want %d %s", status, restBody, tc.status, tc.code)
			}
			gqlArgs := fmt.Sprintf(`ownerId: %q, repo: "bex-co/example", agentConfig: {agent: "claude", task: "x"}`, e.ws)
			for key, value := range tc.args {
				raw, _ := json.Marshal(value)
				gqlArgs += fmt.Sprintf(", %s: %s", key, raw)
			}
			if _, code := a.gql(`mutation { createAgentSession(`+gqlArgs+`) { id } }`, "alice"); code != tc.code {
				t.Errorf("GraphQL code = %q, want %s", code, tc.code)
			}
			if _, errText := a.mcp("create_agent_session", body, "alice"); !strings.Contains(errText, tc.code) {
				t.Errorf("MCP error = %q, want it to carry %s", errText, tc.code)
			}
		})
	}
}
