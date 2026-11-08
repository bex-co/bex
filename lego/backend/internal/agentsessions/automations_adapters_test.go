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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The ADR101 D8 adapters over the one automation core: REST routes, GraphQL
// fields and MCP tools, with the same fields and error codes on each.

type automationAdapters struct {
	t    *testing.T
	svc  *Service
	fake *fakeAutomations
	mux  *http.ServeMux
}

func newAutomationAdapters(t *testing.T) *automationAdapters {
	t.Helper()
	fake := newFakeAutomations()
	svc, _ := automationTestService("tea-a", fake)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return &automationAdapters{t: t, svc: svc, fake: fake, mux: mux}
}

// rest serves one request as who and returns the status and decoded body.
func (a *automationAdapters) rest(method, path string, body any, who string) (int, map[string]any) {
	a.t.Helper()
	var reader *strings.Reader
	switch b := body.(type) {
	case nil:
		reader = strings.NewReader("")
	case string:
		reader = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = strings.NewReader(string(raw))
	}
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, httptest.NewRequest(method, path, reader).WithContext(caller(who)))
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			a.t.Fatalf("%s %s: body %q is not JSON: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// gql runs a query as who and returns data and the first error's code.
func (a *automationAdapters) gql(query, who string) (map[string]any, string) {
	a.t.Helper()
	result := graphql.Do(graphql.Params{Schema: environmentSchema(a.t, a.svc), Context: caller(who), RequestString: query})
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

// mcp calls a tool as who: the structured result, or the error text.
func (a *automationAdapters) mcp(tool string, args map[string]any, who string) (map[string]any, string) {
	a.t.Helper()
	res, err := newMCPClient(a.t, caller(who), a.svc).CallTool(caller(who), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		a.t.Fatalf("MCP %s: %v", tool, err)
	}
	if res.IsError {
		return nil, res.Content[0].(*mcp.TextContent).Text
	}
	out, _ := res.StructuredContent.(map[string]any)
	return out, ""
}

func automationBody(name string) map[string]any {
	return map[string]any{
		"name": name,
		"triggers": []any{map[string]any{"type": "schedule",
			"schedule": map[string]any{"frequency": "day", "at": "09:00", "timezone": "Europe/Berlin"}}},
		"template": map[string]any{"task": "update the dependencies", "repo": "bex-co/example",
			"agentConfig": map[string]any{"agent": "codex", "model": "gpt-5"}},
	}
}

// withOwner is a request body as MCP tool arguments in workspace tea-a.
func withOwner(body map[string]any) map[string]any {
	out := map[string]any{"ownerId": "tea-a"}
	maps.Copy(out, body)
	return out
}

func TestAutomationRESTRoutes(t *testing.T) {
	a := newAutomationAdapters(t)
	status, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
	if status != http.StatusCreated {
		t.Fatalf("create = %d %v", status, created)
	}
	id, _ := created["id"].(string)
	if !strings.HasPrefix(id, "aut-") || created["createdBy"] != "dev" || created["enabled"] != true || created["version"] != float64(1) || created["nextFireAt"] == nil {
		t.Fatalf("created = %v", created)
	}

	// Read by id and by name; the list envelope pages with cursor and limit.
	for _, ref := range []string{id, "nightly"} {
		if status, got := a.rest(http.MethodGet, "/v1/agent-automations/"+ref+"?ownerId=tea-a", nil, "viewer"); status != http.StatusForbidden {
			t.Fatalf("a viewer read %s = %d %v", ref, status, got)
		}
		if status, got := a.rest(http.MethodGet, "/v1/agent-automations/"+ref+"?ownerId=tea-a", nil, "dev2"); status != http.StatusOK || got["id"] != id {
			t.Fatalf("get %s = %d %v", ref, status, got)
		}
	}
	if status, _ := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("weekly"), "dev2"); status != http.StatusCreated {
		t.Fatalf("second create = %d", status)
	}
	status, page := a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a&limit=1", nil, "dev")
	list, _ := page["automations"].([]any)
	if status != http.StatusOK || len(list) != 1 || page["nextCursor"] == "" || page["nextCursor"] == nil || page["ownerId"] != "tea-a" {
		t.Fatalf("page 1 = %d %v", status, page)
	}
	status, page = a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a&limit=1&cursor="+page["nextCursor"].(string), nil, "dev")
	if list, _ = page["automations"].([]any); status != http.StatusOK || len(list) != 1 || page["nextCursor"] != nil {
		t.Fatalf("page 2 = %d %v", status, page)
	}
	status, mine := a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a&mine=true", nil, "dev2")
	if list, _ = mine["automations"].([]any); status != http.StatusOK || len(list) != 1 || list[0].(map[string]any)["name"] != "weekly" {
		t.Fatalf("mine = %d %v", status, mine)
	}
	if status, _ = a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a&mine=maybe", nil, "dev"); status != http.StatusBadRequest {
		t.Fatalf("mine=maybe = %d", status)
	}
	if status, _ = a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a&limit=0", nil, "dev"); status != http.StatusBadRequest {
		t.Fatalf("limit=0 = %d", status)
	}

	// Update, pause, resume, take over, delete.
	status, updated := a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", map[string]any{"ifVersion": 1, "template": automationBody("x")["template"]}, "dev")
	if status != http.StatusOK || updated["version"] != float64(2) {
		t.Fatalf("patch = %d %v", status, updated)
	}
	status, paused := a.rest(http.MethodPost, "/v1/agent-automations/"+id+"/pause?ownerId=tea-a", nil, "dev")
	if status != http.StatusOK || paused["enabled"] != false {
		t.Fatalf("pause = %d %v", status, paused)
	}
	status, resumed := a.rest(http.MethodPost, "/v1/agent-automations/"+id+"/resume?ownerId=tea-a", nil, "admin")
	if status != http.StatusOK || resumed["enabled"] != true || resumed["nextFireAt"] == nil {
		t.Fatalf("resume = %d %v", status, resumed)
	}
	if status, _ = a.rest(http.MethodPost, "/v1/agent-automations/"+id+"/take-over?ownerId=tea-a", nil, "dev2"); status != http.StatusForbidden {
		t.Fatalf("a developer took over = %d", status)
	}
	status, taken := a.rest(http.MethodPost, "/v1/agent-automations/"+id+"/take-over?ownerId=tea-a", nil, "admin")
	if status != http.StatusOK || taken["createdBy"] != "admin" {
		t.Fatalf("take-over = %d %v", status, taken)
	}
	if status, _ = a.rest(http.MethodDelete, "/v1/agent-automations/"+id+"?ownerId=tea-a", nil, "dev"); status != http.StatusForbidden {
		t.Fatalf("delete by a non-creator developer = %d", status)
	}
	if status, _ = a.rest(http.MethodDelete, "/v1/agent-automations/"+id+"?ownerId=tea-a", nil, "admin"); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ = a.rest(http.MethodGet, "/v1/agent-automations/"+id+"?ownerId=tea-a", nil, "admin"); status != http.StatusNotFound {
		t.Fatalf("get after delete = %d", status)
	}
}

// Each named refusal answers the same code and status on all three surfaces.
func TestAutomationRefusalsShareCodesAcrossSurfaces(t *testing.T) {
	a := newAutomationAdapters(t)
	_, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
	id := created["id"].(string)
	missing := ids.New(ids.AgentAutomation)

	noTemplate := automationBody("bad")
	delete(noTemplate, "template")
	unknownField := automationBody("bad")
	unknownField["template"].(map[string]any)["idempotencyKey"] = "k"
	badName := automationBody("Not A Name")

	cases := []struct {
		name   string
		status int
		code   string
		rest   func() (int, map[string]any)
		gql    string
		who    string
		tool   string
		args   map[string]any
	}{
		{name: "missing template", status: 400, code: CodeAutomationInvalid, who: "dev",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", noTemplate, "dev")
			},
			gql:  `mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "bad", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}]}) { id } }`,
			tool: "create_agent_automation", args: withOwner(noTemplate)},
		{name: "invalid name", status: 400, code: CodeAutomationInvalid, who: "dev",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", badName, "dev")
			},
			gql:  `mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "Not A Name", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}], template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex"}}}) { id } }`,
			tool: "create_agent_automation", args: withOwner(badName)},
		{name: "not the creator", status: 403, code: CodeAutomationNotCreator, who: "dev2",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", map[string]any{"name": "renamed"}, "dev2")
			},
			gql:  fmt.Sprintf(`mutation { updateAgentAutomation(ownerId: "tea-a", id: %q, input: {name: "renamed"}) { id } }`, id),
			tool: "update_agent_automation", args: map[string]any{"ownerId": "tea-a", "automation": id, "name": "renamed"}},
		{name: "unknown automation", status: 404, code: CodeAutomationNotFound, who: "dev",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodGet, "/v1/agent-automations/"+missing+"?ownerId=tea-a", nil, "dev")
			},
			gql:  fmt.Sprintf(`{ agentAutomation(ownerId: "tea-a", id: %q) { id } }`, missing),
			tool: "get_agent_automation", args: map[string]any{"ownerId": "tea-a", "automation": missing}},
		{name: "duplicate name", status: 409, code: CodeAutomationConflict, who: "dev",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
			},
			gql:  `mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "nightly", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}], template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex"}}}) { id } }`,
			tool: "create_agent_automation", args: withOwner(automationBody("nightly"))},
		{name: "stale version", status: 409, code: CodeAutomationConflict, who: "dev",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", map[string]any{"name": "renamed", "ifVersion": 99}, "dev")
			},
			gql:  fmt.Sprintf(`mutation { updateAgentAutomation(ownerId: "tea-a", id: %q, input: {name: "renamed", ifVersion: 99}) { id } }`, id),
			tool: "update_agent_automation", args: map[string]any{"ownerId": "tea-a", "automation": id, "name": "renamed", "ifVersion": 99}},
		{name: "viewer cannot read", status: 403, who: "viewer",
			rest: func() (int, map[string]any) {
				return a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a", nil, "viewer")
			},
			gql:  `{ agentAutomations(ownerId: "tea-a") { ownerId } }`,
			tool: "list_agent_automations", args: map[string]any{"ownerId": "tea-a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := tc.rest()
			if status != tc.status {
				t.Fatalf("REST = %d %v, want %d", status, body, tc.status)
			}
			if tc.code == "" {
				// The generic forbidden carries no named code; the other two
				// surfaces still refuse.
				if _, code := a.gql(tc.gql, tc.who); code == "" {
					t.Fatal("GraphQL allowed it")
				}
				if _, text := a.mcp(tc.tool, tc.args, tc.who); text == "" {
					t.Fatal("MCP allowed it")
				}
				return
			}
			if body["code"] != tc.code {
				t.Fatalf("REST code = %v, want %s (%v)", body["code"], tc.code, body)
			}
			if _, code := a.gql(tc.gql, tc.who); code != tc.code {
				t.Fatalf("GraphQL code = %q, want %s", code, tc.code)
			}
			if _, text := a.mcp(tc.tool, tc.args, tc.who); !strings.Contains(text, tc.code) {
				t.Fatalf("MCP error = %q, want %s", text, tc.code)
			}
		})
	}

	// A template field the model does not define is the named 400 on REST
	// and GraphQL has no such input field; MCP refuses it at the schema.
	if status, body := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", unknownField, "dev"); status != 400 || body["code"] != CodeAutomationInvalid {
		t.Fatalf("template with idempotencyKey = %d %v", status, body)
	}
	if status, body := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", `{"name":"x","bogus":1}`, "dev"); status != 400 || body["code"] != CodeAutomationInvalid {
		t.Fatalf("unknown top-level field = %d %v", status, body)
	}
	if res, _ := newMCPClient(t, caller("dev"), a.svc).CallTool(caller("dev"), &mcp.CallToolParams{Name: "create_agent_automation", Arguments: withOwner(unknownField)}); res == nil || !res.IsError {
		t.Fatalf("MCP accepted a template with idempotencyKey: %+v", res)
	}
}

func TestAutomationPausedAndLimitConflictsAcrossSurfaces(t *testing.T) {
	a := newAutomationAdapters(t)
	_, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
	id := created["id"].(string)
	if _, err := a.fake.SetAgentAutomationPaused(context.Background(), id, true, AutomationPausedRepeatedFailures); err != nil {
		t.Fatal(err)
	}
	status, body := a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", map[string]any{"enabled": true}, "dev")
	if status != http.StatusConflict || body["code"] != CodeAutomationPaused {
		t.Fatalf("REST enable a paused automation = %d %v", status, body)
	}
	_, code := a.gql(fmt.Sprintf(`mutation { updateAgentAutomation(ownerId: "tea-a", id: %q, input: {enabled: true}) { id } }`, id), "dev")
	_, text := a.mcp("update_agent_automation", map[string]any{"ownerId": "tea-a", "automation": id, "enabled": true}, "dev")
	if code != CodeAutomationPaused || !strings.Contains(text, CodeAutomationPaused) {
		t.Fatalf("GraphQL %q, MCP %q", code, text)
	}

	for i := 1; i < maxWorkspaceAutomations; i++ {
		if status, body = a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody(fmt.Sprintf("auto-%d", i)), "dev2"); status != http.StatusCreated {
			t.Fatalf("create %d = %d %v", i, status, body)
		}
	}
	status, body = a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("one-too-many"), "dev2")
	if status != http.StatusConflict || body["code"] != CodeAutomationLimit {
		t.Fatalf("REST past the limit = %d %v", status, body)
	}
	_, code = a.gql(`mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "one-too-many", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}], template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex"}}}) { id } }`, "dev2")
	_, text = a.mcp("create_agent_automation", withOwner(automationBody("one-too-many")), "dev2")
	if code != CodeAutomationLimit || !strings.Contains(text, CodeAutomationLimit) {
		t.Fatalf("GraphQL %q, MCP %q", code, text)
	}
}

// The same create, get and list return the same fields on every surface.
// sameAutomationValue compares a GraphQL value with its REST twin; times are
// the same instant however each surface spells the zone and precision.
func sameAutomationValue(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		if at, err := time.Parse(time.RFC3339, as); err == nil {
			bt, err := time.Parse(time.RFC3339, bs)
			return err == nil && at.Truncate(time.Second).Equal(bt.Truncate(time.Second))
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func TestAutomationReadsShareFieldsAcrossSurfaces(t *testing.T) {
	a := newAutomationAdapters(t)
	status, restCreated := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
	if status != http.StatusCreated {
		t.Fatalf("create = %d %v", status, restCreated)
	}
	id := restCreated["id"].(string)

	_, restGot := a.rest(http.MethodGet, "/v1/agent-automations/"+id+"?ownerId=tea-a", nil, "dev")
	mcpGot, text := a.mcp("get_agent_automation", map[string]any{"ownerId": "tea-a", "automation": id}, "dev")
	if text != "" {
		t.Fatalf("MCP get: %s", text)
	}
	if !reflect.DeepEqual(restGot, mcpGot) {
		t.Fatalf("REST and MCP get differ:\n rest %v\n  mcp %v", restGot, mcpGot)
	}
	data, code := a.gql(fmt.Sprintf(`{ agentAutomation(ownerId: "tea-a", id: %q) {
		id ownerId name enabled createdBy overlapPolicy nextFireAt lastFireAt lastSessionId pausedReason consecutiveFailures version createdAt updatedAt
		triggers { type schedule { frequency at timezone weekdays minute runAt cron endsAt } }
		template { task repo branch title tags maxSpendUSD secretNames outputSchema outputRequired egressAllowlist agentConfig { agent model template } skills { name } }
	} }`, id), "dev")
	if code != "" {
		t.Fatalf("GraphQL get: %s", code)
	}
	got, _ := data["agentAutomation"].(map[string]any)
	for _, key := range []string{"id", "ownerId", "name", "enabled", "createdBy", "overlapPolicy", "nextFireAt", "consecutiveFailures", "version", "createdAt", "updatedAt"} {
		if !sameAutomationValue(got[key], restGot[key]) {
			t.Errorf("GraphQL %s = %v, REST %v", key, got[key], restGot[key])
		}
	}
	trigger := got["triggers"].([]any)[0].(map[string]any)["schedule"].(map[string]any)
	if trigger["at"] != "09:00" || trigger["timezone"] != "Europe/Berlin" || trigger["frequency"] != "day" {
		t.Errorf("GraphQL schedule = %v", trigger)
	}
	template := got["template"].(map[string]any)
	if template["task"] != "update the dependencies" || template["agentConfig"].(map[string]any)["model"] != "gpt-5" || template["repo"] != "bex-co/example" {
		t.Errorf("GraphQL template = %v", template)
	}

	// The list envelope.
	_, restList := a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a&mine=true", nil, "dev")
	mcpList, text := a.mcp("list_agent_automations", map[string]any{"ownerId": "tea-a", "mine": true}, "dev")
	if text != "" || !reflect.DeepEqual(restList, mcpList) {
		t.Fatalf("list differs:\n rest %v\n  mcp %v (%s)", restList, mcpList, text)
	}
	data, code = a.gql(`{ agentAutomations(ownerId: "tea-a", mine: true) { ownerId nextCursor automations { id name } } }`, "dev")
	list := data["agentAutomations"].(map[string]any)
	if code != "" || list["ownerId"] != "tea-a" || len(list["automations"].([]any)) != 1 || list["automations"].([]any)[0].(map[string]any)["id"] != id {
		t.Fatalf("GraphQL list = %v (%s)", data, code)
	}

	// Create through GraphQL and MCP lands the same automation shape.
	data, code = a.gql(`mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "from-gql", triggers: [{type: "schedule", schedule: {frequency: "week", at: "08:30", weekdays: ["mon", "fri"], timezone: "UTC"}}], template: {task: "t", repo: "bex-co/example", tags: ["a"], outputSchema: "{\"type\":\"object\"}", agentConfig: {agent: "codex", model: "gpt-5"}}}) { id createdBy template { tags outputSchema } triggers { schedule { weekdays } } } }`, "dev")
	made, _ := data["createAgentAutomation"].(map[string]any)
	if code != "" || made["createdBy"] != "dev" || made["template"].(map[string]any)["outputSchema"] != `{"type":"object"}` {
		t.Fatalf("GraphQL create = %v (%s)", data, code)
	}
	if weekdays := made["triggers"].([]any)[0].(map[string]any)["schedule"].(map[string]any)["weekdays"].([]any); !slices.Equal([]any{"mon", "fri"}, weekdays) {
		t.Fatalf("GraphQL weekdays = %v", weekdays)
	}
	made, text = a.mcp("create_agent_automation", withOwner(automationBody("from-mcp")), "dev")
	if text != "" || made["createdBy"] != "dev" || made["name"] != "from-mcp" {
		t.Fatalf("MCP create = %v (%s)", made, text)
	}
}

// Every JSON field of the shared views is a GraphQL field of the same name,
// so the three surfaces cannot drift apart silently.
func TestAutomationGraphQLTypesMirrorTheJSONViews(t *testing.T) {
	jsonKeys := func(v any) []string {
		var keys []string
		typ := reflect.TypeOf(v)
		for i := 0; i < typ.NumField(); i++ {
			name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			keys = append(keys, name)
		}
		slices.Sort(keys)
		return keys
	}
	gqlKeys := func(o *graphql.Object) []string {
		var keys []string
		for name := range o.Fields() {
			keys = append(keys, name)
		}
		slices.Sort(keys)
		return keys
	}
	for _, tc := range []struct {
		view any
		gql  *graphql.Object
	}{
		{AgentAutomation{}, agentAutomationGQLType},
		{AgentAutomationList{}, agentAutomationListGQLType},
		{AutomationTrigger{}, automationTriggerGQLType},
		{AutomationSchedule{}, automationScheduleGQLType},
		{AutomationTemplate{}, automationTemplateGQLType},
		{AutomationAgentConfig{}, automationAgentConfigGQLType},
		{AgentAutomationRun{}, agentAutomationRunGQLType},
		{AgentAutomationRunList{}, agentAutomationRunListGQLType},
	} {
		if got, want := gqlKeys(tc.gql), jsonKeys(tc.view); !slices.Equal(got, want) {
			t.Errorf("%s: GraphQL fields %v, JSON fields %v", tc.gql.Name(), got, want)
		}
	}
}

func TestAutomationMCPToolContract(t *testing.T) {
	byName := map[string]*mcp.Tool{}
	for _, tool := range listAgentTools(t) {
		byName[tool.Name] = tool
	}
	want := map[string]struct{ readOnly, idempotent, destructive bool }{
		"list_agent_automations":     {true, true, false},
		"get_agent_automation":       {true, true, false},
		"list_agent_automation_runs": {true, true, false},
		"create_agent_automation":    {false, false, false},
		"update_agent_automation":    {false, false, true},
		"delete_agent_automation":    {false, false, true},
		"pause_agent_automation":     {false, true, false},
		"resume_agent_automation":    {false, true, false},
		"take_over_agent_automation": {false, true, false},
		"run_agent_automation":       {false, false, false},
	}
	for name, w := range want {
		tool := byName[name]
		if tool == nil {
			t.Errorf("tool %s is not registered", name)
			continue
		}
		if !slices.Contains(CloudAgentsTools, name) {
			t.Errorf("%s is missing from CloudAgentsTools, so /mcp/cloud-agents would not serve it", name)
		}
		ann := tool.Annotations
		if ann == nil || ann.ReadOnlyHint != w.readOnly || ann.IdempotentHint != w.idempotent || ann.DestructiveHint == nil || *ann.DestructiveHint != w.destructive {
			t.Errorf("%s annotations = %+v, want readOnly=%t idempotent=%t destructive=%t", name, ann, w.readOnly, w.idempotent, w.destructive)
		}
	}
}

// An aut- id needs no ownerId on REST: its workspace comes from the row, and
// a member of another workspace cannot reach it.
func TestAutomationRESTByIDRoutesToItsWorkspace(t *testing.T) {
	svc, _, _, _ := fixture()
	svc.Automations = newFakeAutomations()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, path, body, who string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(caller(who)))
		return rec
	}
	raw, _ := json.Marshal(automationBody("nightly"))
	rec := serve(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", string(raw), "alice")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	var created AgentAutomation
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec = serve(http.MethodGet, "/v1/agent-automations/"+created.ID, "", "alice"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ownerId":"tea-a"`) {
		t.Fatalf("by id without ownerId = %d %s", rec.Code, rec.Body.String())
	}
	if rec = serve(http.MethodGet, "/v1/agent-automations/"+created.ID, "", "bob"); rec.Code == http.StatusOK {
		t.Fatalf("a member of another workspace read it: %s", rec.Body.String())
	}
	if rec = serve(http.MethodPost, "/v1/agent-automations/"+created.ID+"/pause", "", "bob"); rec.Code == http.StatusOK {
		t.Fatalf("a member of another workspace paused it: %s", rec.Body.String())
	}
}

// runHistoryFake adds a fixed, newest-first run history to the automation
// table fake.
type runHistoryFake struct {
	*fakeAutomations
	runs []store.AgentAutomationRun
}

func (f *runHistoryFake) ListRuns(_ context.Context, automationID string, cursor time.Time, limit int) ([]store.AgentAutomationRun, error) {
	out := []store.AgentAutomationRun{}
	for _, r := range f.runs {
		if r.AutomationID == automationID && (cursor.IsZero() || r.FireAt.Before(cursor)) {
			out = append(out, r)
		}
	}
	return out[:min(len(out), limit)], nil
}

func TestAutomationRunAndRunsAdapters(t *testing.T) {
	a := newAutomationAdapters(t)
	history := &runHistoryFake{fakeAutomations: a.fake}
	a.svc.Automations = history
	_, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
	id := created["id"].(string)
	base := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	for i := range 3 {
		history.runs = append(history.runs, store.AgentAutomationRun{
			AutomationID: id, FireAt: base.Add(-time.Duration(i) * 24 * time.Hour), Outcome: "started",
			SessionID: fmt.Sprintf("ags-%d", i), CreatedAt: base,
		})
	}

	// Runs page newest first with the same cursor on all three surfaces.
	status, page := a.rest(http.MethodGet, "/v1/agent-automations/"+id+"/runs?ownerId=tea-a&limit=2", nil, "dev2")
	runs, _ := page["runs"].([]any)
	if status != http.StatusOK || len(runs) != 2 || page["automationId"] != id || page["nextCursor"] == nil {
		t.Fatalf("REST runs = %d %v", status, page)
	}
	if first := runs[0].(map[string]any); first["outcome"] != "started" || first["sessionId"] != "ags-0" || first["missedTicks"] != float64(0) {
		t.Fatalf("REST first run = %v", first)
	}
	mcpPage, text := a.mcp("list_agent_automation_runs", map[string]any{"ownerId": "tea-a", "automation": "nightly", "limit": 2}, "dev2")
	if text != "" || !reflect.DeepEqual(page, mcpPage) {
		t.Fatalf("MCP runs differ (%s):\n rest %v\n  mcp %v", text, page, mcpPage)
	}
	data, code := a.gql(fmt.Sprintf(`{ agentAutomationRuns(ownerId: "tea-a", id: %q, limit: 2) { automationId nextCursor runs { automationId outcome sessionId missedTicks fireAt } } }`, id), "dev2")
	got, _ := data["agentAutomationRuns"].(map[string]any)
	if code != "" || got["nextCursor"] != page["nextCursor"] || len(got["runs"].([]any)) != 2 || got["runs"].([]any)[0].(map[string]any)["sessionId"] != "ags-0" {
		t.Fatalf("GraphQL runs = %v (%s)", data, code)
	}
	status, page2 := a.rest(http.MethodGet, "/v1/agent-automations/"+id+"/runs?ownerId=tea-a&limit=2&cursor="+page["nextCursor"].(string), nil, "dev")
	if runs, _ = page2["runs"].([]any); status != http.StatusOK || len(runs) != 1 || page2["nextCursor"] != nil {
		t.Fatalf("REST runs page 2 = %d %v", status, page2)
	}
	if status, body := a.rest(http.MethodGet, "/v1/agent-automations/"+id+"/runs?ownerId=tea-a&cursor=yesterday", nil, "dev"); status != 400 || body["code"] != CodeAutomationInvalid {
		t.Fatalf("bad cursor = %d %v", status, body)
	}
	if status, _ := a.rest(http.MethodGet, "/v1/agent-automations/"+id+"/runs?ownerId=tea-a", nil, "viewer"); status != http.StatusForbidden {
		t.Fatalf("a viewer listed runs = %d", status)
	}

	// Run now refuses a paused automation, and an unknown one, the same way
	// on every surface; the run itself is the service's (automation_run.go).
	if _, err := history.SetAgentAutomationPaused(context.Background(), id, true, AutomationPausedRepeatedFailures); err != nil {
		t.Fatal(err)
	}
	status, body := a.rest(http.MethodPost, "/v1/agent-automations/"+id+"/run?ownerId=tea-a", nil, "dev")
	_, gqlCode := a.gql(fmt.Sprintf(`mutation { runAgentAutomation(ownerId: "tea-a", id: %q) { outcome } }`, id), "dev")
	_, mcpText := a.mcp("run_agent_automation", map[string]any{"ownerId": "tea-a", "automation": id}, "dev")
	if status != http.StatusConflict || body["code"] != CodeAutomationPaused || gqlCode != CodeAutomationPaused || !strings.Contains(mcpText, CodeAutomationPaused) {
		t.Fatalf("run a paused automation: REST %d %v, GraphQL %q, MCP %q", status, body, gqlCode, mcpText)
	}
	missing := ids.New(ids.AgentAutomation)
	if status, body = a.rest(http.MethodPost, "/v1/agent-automations/"+missing+"/run?ownerId=tea-a", nil, "dev"); status != http.StatusNotFound || body["code"] != CodeAutomationNotFound {
		t.Fatalf("run an unknown automation = %d %v", status, body)
	}
	if status, _ = a.rest(http.MethodPost, "/v1/agent-automations/"+id+"/run?ownerId=tea-a", nil, "viewer"); status != http.StatusForbidden {
		t.Fatalf("a viewer ran an automation = %d", status)
	}
}
