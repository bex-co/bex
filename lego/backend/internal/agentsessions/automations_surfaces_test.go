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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// automationOperations is ADR101 D8: every operation on its three adapters.
// The dashboard's own operations are pinned in its api/agent-automations.graphql
// test against the same GraphQL names.
var automationOperations = []struct {
	op           string
	restMethod   string
	restPattern  string // the ServeMux pattern, wildcards as {id}
	gqlField     string
	gqlMutation  bool
	mcpTool      string
	adrRESTRoute string
}{
	{"list", "GET", "/v1/agent-automations", "agentAutomations", false, "list_agent_automations", "GET /v1/agent-automations"},
	{"get", "GET", "/v1/agent-automations/{id}", "agentAutomation", false, "get_agent_automation", "GET /v1/agent-automations/{id}"},
	{"create", "POST", "/v1/agent-automations", "createAgentAutomation", true, "create_agent_automation", "POST /v1/agent-automations"},
	{"update", "PATCH", "/v1/agent-automations/{id}", "updateAgentAutomation", true, "update_agent_automation", "PATCH /v1/agent-automations/{id}"},
	{"delete", "DELETE", "/v1/agent-automations/{id}", "deleteAgentAutomation", true, "delete_agent_automation", "DELETE /v1/agent-automations/{id}"},
	{"pause", "POST", "/v1/agent-automations/{id}/pause", "pauseAgentAutomation", true, "pause_agent_automation", "POST …/{id}/pause"},
	{"resume", "POST", "/v1/agent-automations/{id}/resume", "resumeAgentAutomation", true, "resume_agent_automation", "POST …/{id}/resume"},
	{"run now", "POST", "/v1/agent-automations/{id}/run", "runAgentAutomation", true, "run_agent_automation", "POST …/{id}/run"},
	{"take over", "POST", "/v1/agent-automations/{id}/take-over", "takeOverAgentAutomation", true, "take_over_agent_automation", "POST …/{id}/take-over"},
	{"list runs", "GET", "/v1/agent-automations/{id}/runs", "agentAutomationRuns", false, "list_agent_automation_runs", "GET …/{id}/runs"},
}

// Each automation operation exists on REST, GraphQL and MCP under the names
// ADR101 D8 documents, so a surface cannot lose or rename one unnoticed.
func TestAutomationOperationsExistOnEverySurface(t *testing.T) {
	a := newAutomationAdapters(t)
	schema := environmentSchema(t, a.svc)
	tools := map[string]bool{}
	for _, tool := range listAgentTools(t) {
		tools[tool.Name] = true
	}
	adr, adrErr := os.ReadFile("../../../../docs/ADR101-agent-automations.md")
	if adrErr != nil {
		t.Logf("ADR101 not readable (%v): skipping the documentation check", adrErr)
	}
	for _, op := range automationOperations {
		path := strings.ReplaceAll(op.restPattern, "{id}", "aut-x")
		_, pattern := a.mux.Handler(httptest.NewRequest(op.restMethod, path, nil))
		if want := op.restMethod + " " + op.restPattern; pattern != want {
			t.Errorf("%s: REST route %q not registered (mux matched %q)", op.op, want, pattern)
		}
		fields := schema.QueryType().Fields()
		if op.gqlMutation {
			fields = schema.MutationType().Fields()
		}
		if fields[op.gqlField] == nil {
			t.Errorf("%s: GraphQL field %s is not registered", op.op, op.gqlField)
		}
		if !tools[op.mcpTool] {
			t.Errorf("%s: MCP tool %s is not registered", op.op, op.mcpTool)
		}
		if adrErr == nil {
			for _, name := range []string{op.adrRESTRoute, op.gqlField, op.mcpTool} {
				if !strings.Contains(string(adr), name) {
					t.Errorf("%s: ADR101 D8 does not mention %q", op.op, name)
				}
			}
		}
	}
	if len(automationOperations) != 10 {
		t.Errorf("ADR101 D8 defines ten operations, the table has %d", len(automationOperations))
	}
}

const gqlDailyTrigger = `[{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}]`

func gqlCreate(name, triggers, template string) string {
	return fmt.Sprintf(`mutation { createAgentAutomation(ownerId: "tea-a", input: {name: %q, triggers: %s, template: %s}) { id } }`, name, triggers, template)
}

const gqlTemplate = `{task: "t", repo: "bex-co/example", agentConfig: {agent: "codex"}}`

func restTrigger(schedule map[string]any) []any {
	return []any{map[string]any{"type": "schedule", "schedule": schedule}}
}

// The codes the other refusal tests do not cover (schedule, spend ceiling,
// unavailable) are the same code and status on REST, GraphQL and MCP.
func TestAutomationScheduleSpendAndUnavailableCodesAcrossSurfaces(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	a := newAutomationAdapters(t)

	badZone := automationBody("bad-zone")
	badZone["triggers"] = restTrigger(map[string]any{"frequency": "day", "at": "09:00", "timezone": "Not/AZone"})
	tooFrequent := automationBody("too-frequent")
	tooFrequent["triggers"] = restTrigger(map[string]any{"frequency": "custom", "cron": "* * * * *", "timezone": "UTC"})
	past := automationBody("past")
	past["triggers"] = restTrigger(map[string]any{"frequency": "once", "runAt": "2020-01-01T00:00:00Z", "timezone": "UTC"})
	noCeiling := automationBody("no-ceiling")
	noCeiling["template"] = map[string]any{"task": "t", "repo": "bex-co/example", "agentConfig": map[string]any{"agent": "claude"}}

	cases := []struct {
		name   string
		status int
		code   string
		body   map[string]any
		gql    string
	}{
		{"unknown time zone", 400, CodeAutomationScheduleInvalid, badZone,
			gqlCreate("bad-zone", `[{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Not/AZone"}}]`, gqlTemplate)},
		{"every minute", 400, CodeAutomationScheduleTooFrequent, tooFrequent,
			gqlCreate("too-frequent", `[{type: "schedule", schedule: {frequency: "custom", cron: "* * * * *", timezone: "UTC"}}]`, gqlTemplate)},
		{"once in the past", 400, CodeAutomationSchedulePast, past,
			gqlCreate("past", `[{type: "schedule", schedule: {frequency: "once", runAt: "2020-01-01T00:00:00Z", timezone: "UTC"}}]`, gqlTemplate)},
		{"platform model without maxSpendUSD", 400, CodeAutomationSpendLimitNeeded, noCeiling,
			gqlCreate("no-ceiling", gqlDailyTrigger, `{task: "t", repo: "bex-co/example", agentConfig: {agent: "claude"}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", tc.body, "dev")
			if status != tc.status || body["code"] != tc.code {
				t.Fatalf("REST = %d %v, want %d %s", status, body, tc.status, tc.code)
			}
			if _, code := a.gql(tc.gql, "dev"); code != tc.code {
				t.Fatalf("GraphQL code = %q, want %s", code, tc.code)
			}
			if _, text := a.mcp("create_agent_automation", withOwner(tc.body), "dev"); !strings.Contains(text, tc.code) {
				t.Fatalf("MCP error = %q, want %s", text, tc.code)
			}
		})
	}

	// A deployment without the automation table refuses every verb the same way.
	a.svc.Automations = nil
	status, body := a.rest(http.MethodGet, "/v1/agent-automations?ownerId=tea-a", nil, "dev")
	if status != http.StatusServiceUnavailable || body["code"] != CodeAutomationsUnavailableError {
		t.Fatalf("REST unavailable = %d %v", status, body)
	}
	if _, code := a.gql(`{ agentAutomations(ownerId: "tea-a") { ownerId } }`, "dev"); code != CodeAutomationsUnavailableError {
		t.Fatalf("GraphQL unavailable code = %q", code)
	}
	if _, text := a.mcp("list_agent_automations", map[string]any{"ownerId": "tea-a"}, "dev"); !strings.Contains(text, CodeAutomationsUnavailableError) {
		t.Fatalf("MCP unavailable = %q", text)
	}
}

// The email notification setting (w9/m343) reads and writes the same way on
// REST, GraphQL and MCP: on_failure by default, kept by an update that omits
// it, and an unknown value is the named 400 on notifications.email.
func TestAutomationNotificationsAcrossSurfaces(t *testing.T) {
	a := newAutomationAdapters(t)
	status, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("nightly"), "dev")
	if status != http.StatusCreated {
		t.Fatalf("create = %d %v", status, created)
	}
	emailOf := func(view map[string]any) any {
		n, _ := view["notifications"].(map[string]any)
		return n["email"]
	}
	if got := emailOf(created); got != "on_failure" {
		t.Fatalf("default notifications.email = %v, want on_failure", got)
	}
	id := created["id"].(string)

	if status, body := a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", map[string]any{"notifications": map[string]any{"email": "on_success"}}, "dev"); status != http.StatusOK || emailOf(body) != "on_success" {
		t.Fatalf("REST update = %d %v", status, body)
	}
	if status, body := a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", map[string]any{"name": "nightly-two"}, "dev"); status != http.StatusOK || emailOf(body) != "on_success" {
		t.Fatalf("an update without notifications changed it: %d %v", status, body)
	}
	data, code := a.gql(fmt.Sprintf(`mutation { updateAgentAutomation(ownerId: "tea-a", id: %q, input: {notifications: {email: "never"}}) { notifications { email } } }`, id), "dev")
	if code != "" || emailOf(data["updateAgentAutomation"].(map[string]any)) != "never" {
		t.Fatalf("GraphQL update = %v %q", data, code)
	}
	got, text := a.mcp("get_agent_automation", map[string]any{"ownerId": "tea-a", "automation": id}, "dev")
	if text != "" || emailOf(got) != "never" {
		t.Fatalf("MCP get = %v %q", got, text)
	}
	created2, text := a.mcp("create_agent_automation", withOwner(map[string]any{
		"name": "always-mail", "triggers": automationBody("x")["triggers"], "template": automationBody("x")["template"],
		"notifications": map[string]any{"email": "always"}}), "dev")
	if text != "" || emailOf(created2) != "always" {
		t.Fatalf("MCP create with always = %v %q", created2, text)
	}

	bad := map[string]any{"notifications": map[string]any{"email": "sometimes"}}
	if status, body := a.rest(http.MethodPatch, "/v1/agent-automations/"+id+"?ownerId=tea-a", bad, "dev"); status != http.StatusBadRequest ||
		body["code"] != CodeAutomationInvalid || !strings.Contains(fmt.Sprint(body), "notifications.email") {
		t.Fatalf("REST unknown value = %d %v", status, body)
	}
	if _, code := a.gql(fmt.Sprintf(`mutation { updateAgentAutomation(ownerId: "tea-a", id: %q, input: {notifications: {email: "sometimes"}}) { id } }`, id), "dev"); code != CodeAutomationInvalid {
		t.Fatalf("GraphQL unknown value code = %q", code)
	}
	if _, text := a.mcp("update_agent_automation", map[string]any{"ownerId": "tea-a", "automation": id, "notifications": map[string]any{"email": "sometimes"}}, "dev"); !strings.Contains(text, CodeAutomationInvalid) {
		t.Fatalf("MCP unknown value = %q", text)
	}
}
