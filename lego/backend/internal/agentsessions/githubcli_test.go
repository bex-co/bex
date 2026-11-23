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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w10/m561: the `gh` binding chain on every surface and at every dispatch.

// lockedReason is the reason param of a GITHUB_CLI_LOCKED refusal, "" when
// err is not one with code.
func lockedReason(err error, code string) string {
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != code || !errors.Is(err, core.ErrForbidden) {
		return ""
	}
	reason, _ := coded.Params["reason"].(string)
	return reason
}

// The agent environment's automations default and mandatory flag read as
// inherit and not mandatory with no row, an admin's write on any surface
// reads back on the others (an explicit null returning the default to
// inherit), a member's is refused on each, and an automations default
// turning gh on under a mandatory off is refused with the named code on each.
func TestEnvironmentGitHubCLIBindingsOnEverySurface(t *testing.T) {
	envs := newFakeEnvironments()
	svc := environmentService("tea-a", envs)
	schema := environmentSchema(t, svc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	patch := func(body, who string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(body)).WithContext(caller(who)))
		return rec
	}
	type bindings struct{ automations, mandatory any }
	read := func() (rest, gql, mcpRead bindings) {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil).WithContext(caller("viewer")))
		var body map[string]any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
			t.Fatalf("REST read = %d %s", rec.Code, rec.Body.String())
		}
		if _, present := body["automationsGithubCliEnabled"]; !present {
			t.Fatalf("REST read omits automationsGithubCliEnabled: %s", rec.Body.String())
		}
		rest = bindings{body["automationsGithubCliEnabled"], body["githubCliMandatory"]}
		result := graphql.Do(graphql.Params{Schema: schema, Context: caller("viewer"), RequestString: `{ agentEnvironment(ownerId: "tea-a") { automationsGithubCliEnabled githubCliMandatory } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL read: %v", result.Errors)
		}
		env := result.Data.(map[string]any)["agentEnvironment"].(map[string]any)
		gql = bindings{env["automationsGithubCliEnabled"], env["githubCliMandatory"]}
		res, err := newMCPClient(t, caller("viewer"), svc).CallTool(caller("viewer"), &mcp.CallToolParams{Name: "get_agent_environment", Arguments: map[string]any{"ownerId": "tea-a"}})
		if err != nil || res.IsError {
			t.Fatalf("MCP read = %+v err=%v", res, err)
		}
		structured := res.StructuredContent.(map[string]any)
		mcpRead = bindings{structured["automationsGithubCliEnabled"], structured["githubCliMandatory"]}
		return rest, gql, mcpRead
	}
	expect := func(when string, automations, mandatory any) {
		t.Helper()
		want := bindings{automations, mandatory}
		if rest, gql, mcpRead := read(); rest != want || gql != want || mcpRead != want {
			t.Fatalf("%s: REST %+v GraphQL %+v MCP %+v, want %+v", when, rest, gql, mcpRead, want)
		}
	}
	expect("no row", nil, false)

	// A member cannot change either on any surface.
	if rec := patch(`{"automationsGithubCliEnabled":false}`, "dev"); rec.Code != http.StatusForbidden {
		t.Fatalf("REST member write = %d", rec.Code)
	}
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("dev"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {githubCliMandatory: true}) { githubCliMandatory } }`}); len(result.Errors) == 0 {
		t.Fatal("GraphQL member write succeeded")
	}
	if res, err := newMCPClient(t, caller("dev"), svc).CallTool(caller("dev"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "githubCliMandatory": true}}); err != nil || !res.IsError {
		t.Fatalf("MCP member write = %+v err=%v", res, err)
	}
	if len(envs.rows) != 0 {
		t.Fatalf("a member's write was stored: %+v", envs.rows)
	}

	// An admin's write on each surface reads back on the others; an omitted
	// field keeps its value and an explicit null returns to inherit.
	if rec := patch(`{"automationsGithubCliEnabled":false,"githubCliMandatory":true}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST admin write = %d %s", rec.Code, rec.Body.String())
	}
	expect("after the REST write", false, true)
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {automationsGithubCliEnabled: true}) { automationsGithubCliEnabled githubCliMandatory } }`}); len(result.Errors) != 0 {
		t.Fatalf("GraphQL admin write: %v", result.Errors)
	}
	expect("after the GraphQL write", true, true)
	// graphql-go cannot carry an explicit null, so GraphQL resets with the flag.
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {resetAutomationsGithubCli: true}) { automationsGithubCliEnabled } }`}); len(result.Errors) != 0 {
		t.Fatalf("GraphQL reset: %v", result.Errors)
	}
	expect("after the GraphQL reset", nil, true)
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {resetAutomationsGithubCli: true, automationsGithubCliEnabled: false}) { automationsGithubCliEnabled } }`}); len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != CodeEnvironmentGitHubCLIInvalid {
		t.Fatalf("GraphQL reset and a value = %v", result.Errors)
	}
	if res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "automationsGithubCliEnabled": false, "githubCliMandatory": false}}); err != nil || res.IsError {
		t.Fatalf("MCP admin write = %+v err=%v", res, err)
	}
	expect("after the MCP write", false, false)
	if res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "automationsGithubCliEnabled": nil}}); err != nil || res.IsError {
		t.Fatalf("MCP null write = %+v err=%v", res, err)
	}
	expect("after the MCP null", nil, false)
	if rec := patch(`{"automationsGithubCliEnabled":true}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST on = %d", rec.Code)
	}
	if rec := patch(`{"networkTier":"full"}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST tier = %d", rec.Code)
	}
	expect("after a write naming neither", true, false)
	if rec := patch(`{"automationsGithubCliEnabled":null}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST null = %d", rec.Code)
	}
	expect("after the REST null", nil, false)
	if rec := patch(`{"automationsGithubCliEnabled":true}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST on = %d", rec.Code)
	}
	if res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "resetAutomationsGithubCli": true}}); err != nil || res.IsError {
		t.Fatalf("MCP reset = %+v err=%v", res, err)
	}
	expect("after the MCP reset", nil, false)

	// Under a mandatory off, an automations default turning gh on is refused
	// with the named code on every surface, in the same write or a later one,
	// and nothing is written; turning it off stays allowed.
	if rec := patch(`{"githubCliEnabled":false,"githubCliMandatory":true,"automationsGithubCliEnabled":true}`, "admin"); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), CodeEnvironmentGitHubCLILocked) {
		t.Fatalf("REST mandatory and on in one write = %d %s", rec.Code, rec.Body.String())
	}
	if envs.rows["tea-a"].GitHubCLIDisabled {
		t.Fatal("a refused write turned gh off")
	}
	if rec := patch(`{"githubCliEnabled":false,"githubCliMandatory":true}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST mandatory off = %d", rec.Code)
	}
	_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{AutomationsGitHubCLIEnabled: ptr(true)})
	if lockedReason(err, CodeEnvironmentGitHubCLILocked) != "mandatory" {
		t.Fatalf("service on under a mandatory off = %v", err)
	}
	if rec := patch(`{"automationsGithubCliEnabled":true}`, "admin"); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), CodeEnvironmentGitHubCLILocked) {
		t.Fatalf("REST on under a mandatory off = %d %s", rec.Code, rec.Body.String())
	}
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {automationsGithubCliEnabled: true}) { automationsGithubCliEnabled } }`}); len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != CodeEnvironmentGitHubCLILocked {
		t.Fatalf("GraphQL on under a mandatory off = %v", result.Errors)
	}
	if res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "automationsGithubCliEnabled": true}}); err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, CodeEnvironmentGitHubCLILocked) {
		t.Fatalf("MCP on under a mandatory off = %+v err=%v", res, err)
	}
	expect("after the refusals", nil, true)
	if rec := patch(`{"automationsGithubCliEnabled":false}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST off under a mandatory off = %d", rec.Code)
	}
	// Turning the workspace back on leaves the flag, which then binds nothing:
	// an automations default turning gh on is allowed again.
	if rec := patch(`{"githubCliEnabled":true,"automationsGithubCliEnabled":true}`, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("REST workspace on with automations on = %d %s", rec.Code, rec.Body.String())
	}
	expect("back on", true, true)
}

// adminAndMembers is adminSession with an admin who may also operate the
// workspace's sessions, so a child create's parent check passes for them.
func adminAndMembers(t *testing.T, row *store.AgentEnvironment) (*Service, *fakeStore, *fakeLifecycle) {
	t.Helper()
	svc, st, lifecycle := sessionWithEnvironment(t, row)
	svc.Workspace.(resolver).members["admin"] = "tea-a"
	members := svc.Authz
	svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		if subject == "user:admin" {
			if id, ok := strings.CutPrefix(object, "agent_session:"); ok {
				return st.rows[id].WorkspaceID == "tea-a", nil
			}
			return object == "workspace:tea-a", nil
		}
		return members.Check(ctx, subject, relation, object)
	})
	return svc, st, lifecycle
}

func githubCliOf(t *testing.T, config json.RawMessage) *bool {
	t.Helper()
	binding, err := agentsession.SessionGitHubCLI(config)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

// refusalReason is the reason a GITHUB_CLI_LOCKED refusal's MCP text names,
// which carries the message but not params: "" when it names neither.
func refusalReason(text string) string {
	switch {
	case strings.Contains(text, "only a workspace admin"):
		return "admin_required"
	case strings.Contains(text, "made its gh off mandatory"):
		return "mandatory"
	}
	return ""
}

// A session create carries agentConfig.githubCli on REST, GraphQL and MCP.
// Only a workspace admin names a binding, as Devin's "Members without the
// permission can't enumerate, select, or change profiles" (w10/044): a
// member's false and true are refused admin_required on all three, an
// admin's either is stored and read back on all three, and a member's create
// naming none inherits. Under a mandatory off an admin's true is refused
// mandatory on all three, an admin's false is kept, and a member's false is
// still refused admin_required.
func TestSessionGitHubCLIBindingOnEverySurface(t *testing.T) {
	svc, st, _ := adminAndMembers(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries})
	schema := environmentSchema(t, svc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body := func(binding any) map[string]any {
		return map[string]any{"ownerId": "tea-a", "repo": "bex-co/example",
			"agentConfig": map[string]any{"agent": "codex", "task": "t", "githubCli": binding}}
	}
	type created struct {
		id     string
		code   string
		reason string
	}
	creates := map[string]func(binding bool, who string) created{
		"REST": func(binding bool, who string) created {
			raw, _ := json.Marshal(body(binding))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(string(raw))).WithContext(caller(who)))
			var out struct {
				ID     string         `json:"id"`
				Code   string         `json:"code"`
				Params map[string]any `json:"params"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
			if rec.Code == http.StatusCreated {
				return created{id: out.ID}
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("REST create = %d %s", rec.Code, rec.Body.String())
			}
			reason, _ := out.Params["reason"].(string)
			return created{code: out.Code, reason: reason}
		},
		"GraphQL": func(binding bool, who string) created {
			result := graphql.Do(graphql.Params{Schema: schema, Context: caller(who),
				RequestString:  `mutation($b: Boolean) { createAgentSession(ownerId: "tea-a", repo: "bex-co/example", agentConfig: {agent: "codex", task: "t", githubCli: $b}) { id agentConfig { githubCli } } }`,
				VariableValues: map[string]any{"b": binding}})
			if len(result.Errors) != 0 {
				code, _ := result.Errors[0].Extensions["code"].(string)
				reason, _ := result.Errors[0].Extensions["reason"].(string)
				return created{code: code, reason: reason}
			}
			view := result.Data.(map[string]any)["createAgentSession"].(map[string]any)
			if got := view["agentConfig"].(map[string]any)["githubCli"]; got != binding {
				t.Fatalf("GraphQL create returned githubCli %v, want %v", got, binding)
			}
			return created{id: view["id"].(string)}
		},
		"MCP": func(binding bool, who string) created {
			res, err := newMCPClient(t, caller(who), svc).CallTool(caller(who), &mcp.CallToolParams{Name: "create_agent_session", Arguments: body(binding)})
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				text := res.Content[0].(*mcp.TextContent).Text
				code, _, _ := strings.Cut(text, ":")
				return created{code: code, reason: refusalReason(text)}
			}
			return created{id: res.StructuredContent.(map[string]any)["id"].(string)}
		},
	}
	refused := func(t *testing.T, surface, what string, got created, reason string) {
		t.Helper()
		if got.id != "" || got.code != CodeSessionGitHubCLILocked || got.reason != reason {
			t.Fatalf("%s: %s = %+v, want %s (%s)", surface, what, got, CodeSessionGitHubCLILocked, reason)
		}
	}
	readBack := func(id string) (rest, gql, mcpRead any) {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id, nil).WithContext(caller("alice")))
		var view map[string]any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &view) != nil {
			t.Fatalf("REST read = %d %s", rec.Code, rec.Body.String())
		}
		rest = view["agentConfig"].(map[string]any)["githubCli"]
		result := graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `{ agentSession(id: "` + id + `") { agentConfig { githubCli } } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL read: %v", result.Errors)
		}
		gql = result.Data.(map[string]any)["agentSession"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"]
		res, err := newMCPClient(t, caller("alice"), svc).CallTool(caller("alice"), &mcp.CallToolParams{Name: "get_agent_session", Arguments: map[string]any{"id": id}})
		if err != nil || res.IsError {
			t.Fatalf("MCP read = %+v err=%v", res, err)
		}
		mcpRead = res.StructuredContent.(map[string]any)["agentConfig"].(map[string]any)["githubCli"]
		return rest, gql, mcpRead
	}
	for surface, create := range creates {
		rows := len(st.rows)
		refused(t, surface, "a member's githubCli false", create(false, "alice"), "admin_required")
		refused(t, surface, "a member's githubCli true", create(true, "alice"), "admin_required")
		if len(st.rows) != rows {
			t.Fatalf("%s: a member's refused create stored a session", surface)
		}
		for _, binding := range []bool{false, true} {
			got := create(binding, "admin")
			if got.id == "" {
				t.Fatalf("%s: an admin's githubCli %v was refused: %+v", surface, binding, got)
			}
			if stored := githubCliOf(t, st.rows[got.id].AgentConfig); stored == nil || *stored != binding {
				t.Fatalf("%s: stored githubCli = %v, want %v", surface, stored, binding)
			}
			if rest, gql, mcpRead := readBack(got.id); rest != binding || gql != binding || mcpRead != binding {
				t.Fatalf("%s: read back REST %v GraphQL %v MCP %v, want %v", surface, rest, gql, mcpRead, binding)
			}
		}
	}
	for _, binding := range []bool{false, true} {
		if _, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: ptr(binding)}}); lockedReason(err, CodeSessionGitHubCLILocked) != "admin_required" {
			t.Fatalf("service: a member's %v = %v, want admin_required", binding, err)
		}
	}
	omitted, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t"}})
	if err != nil || omitted.AgentConfig.GitHubCLI != nil || githubCliOf(t, st.rows[omitted.ID].AgentConfig) != nil {
		t.Fatalf("a member's create naming no binding = %+v err=%v, want it absent", omitted.AgentConfig, err)
	}
	if rest, gql, mcpRead := readBack(omitted.ID); rest != nil || gql != nil || mcpRead != nil {
		t.Fatalf("an unbound session reads REST %v GraphQL %v MCP %v, want null on each", rest, gql, mcpRead)
	}

	// Under the workspace's mandatory off nobody turns gh on, on any surface;
	// an admin may still turn it off, and a member still names nothing.
	envs := svc.Environments.(*fakeEnvironments)
	envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, GitHubCLIDisabled: true, GitHubCLIMandatory: true}
	for surface, create := range creates {
		refused(t, surface, "an admin's true under a mandatory off", create(true, "admin"), "mandatory")
		refused(t, surface, "a member's true under a mandatory off", create(true, "alice"), "mandatory")
		refused(t, surface, "a member's false under a mandatory off", create(false, "alice"), "admin_required")
		if got := create(false, "admin"); got.id == "" {
			t.Fatalf("%s: an admin's false under a mandatory off was refused: %+v", surface, got)
		}
	}
	if _, err := svc.Create(caller("admin"), CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: ptr(true)}}); lockedReason(err, CodeSessionGitHubCLILocked) != "mandatory" {
		t.Fatalf("service: an admin's true under a mandatory off = %v, want mandatory", err)
	}
}

// An automation template carries agentConfig.githubCli on REST, GraphQL and
// MCP. Only a workspace admin sets or changes it (w10/044): a member's create
// naming false or true is refused admin_required on all three, an admin's is
// stored and read back on all three, a member creator's update that changes
// the stored binding in any direction (inherit included) is refused, and one
// that keeps it, or names no template, keeps it. Under a mandatory off
// nobody turns it on, an admin may turn it off, and a stored on is kept by a
// save that leaves it alone.
func TestAutomationGitHubCLIBindingOnEverySurface(t *testing.T) {
	a := newAutomationAdapters(t)
	withBinding := func(name string, binding any) map[string]any {
		body := automationBody(name)
		body["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] = binding
		return body
	}
	gqlCreate := func(name, binding string) string {
		return `mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "` + name + `", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}], template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex", model: "gpt-5", githubCli: ` + binding + `}}}) { id template { agentConfig { githubCli } } } }`
	}
	bindingOf := func(id string) any {
		t.Helper()
		row, err := a.fake.GetAgentAutomation(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		binding, err := agentsession.AutomationGitHubCLI(row.Template)
		if err != nil {
			t.Fatal(err)
		}
		if binding == nil {
			return nil
		}
		return *binding
	}
	template := func(task string, binding *bool) *AutomationTemplate {
		return &AutomationTemplate{Task: task, Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5", GitHubCLI: binding}}
	}

	// A member names no binding on any surface, in either direction.
	for _, binding := range []bool{false, true} {
		name := fmt.Sprintf("member-%v", binding)
		if status, out := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-"+name, binding), "dev"); status != http.StatusForbidden || out["code"] != CodeAutomationGitHubCLILocked || out["params"].(map[string]any)["reason"] != "admin_required" {
			t.Fatalf("REST member create %v = %d %v", binding, status, out)
		}
		if _, code := a.gql(gqlCreate("gql-"+name, fmt.Sprint(binding)), "dev"); code != CodeAutomationGitHubCLILocked {
			t.Fatalf("GraphQL member create %v = %s", binding, code)
		}
		if _, errText := a.mcp("create_agent_automation", withOwner(withBinding("mcp-"+name, binding)), "dev"); !strings.HasPrefix(errText, CodeAutomationGitHubCLILocked) || refusalReason(errText) != "admin_required" {
			t.Fatalf("MCP member create %v = %s", binding, errText)
		}
		if _, err := a.svc.CreateAgentAutomation(caller("dev"), "tea-a", AgentAutomationInput{Name: str("svc-" + name), Triggers: dailyTrigger("09:00"), Template: template("t", ptr(binding))}); lockedReason(err, CodeAutomationGitHubCLILocked) != "admin_required" {
			t.Fatalf("service: a member's create %v = %v, want admin_required", binding, err)
		}
	}
	if len(a.fake.rows) != 0 {
		t.Fatalf("a member's refused create stored %d automations", len(a.fake.rows))
	}

	// An admin's off is stored and reads back on every surface.
	status, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-off", false), "admin")
	if status != http.StatusCreated || created["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
		t.Fatalf("REST admin create off = %d %v", status, created)
	}
	restID := created["id"].(string)
	data, code := a.gql(gqlCreate("gql-off", "false"), "admin")
	if code != "" || data["createAgentAutomation"].(map[string]any)["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
		t.Fatalf("GraphQL admin create off = %v %s", data, code)
	}
	gqlID := data["createAgentAutomation"].(map[string]any)["id"].(string)
	out, errText := a.mcp("create_agent_automation", withOwner(withBinding("mcp-off", false)), "admin")
	if errText != "" {
		t.Fatalf("MCP admin create off = %s", errText)
	}
	mcpID := out["id"].(string)
	for _, id := range []string{restID, gqlID, mcpID} {
		if bindingOf(id) != false {
			t.Fatalf("%s stored githubCli %v, want false", id, bindingOf(id))
		}
		_, read := a.rest(http.MethodGet, "/v1/agent-automations/"+id+"?ownerId=tea-a", nil, "dev")
		if read["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
			t.Fatalf("REST read of %s = %v", id, read)
		}
	}
	data, _ = a.gql(`{ agentAutomation(ownerId: "tea-a", id: "`+mcpID+`") { template { agentConfig { githubCli } } } }`, "dev")
	if data["agentAutomation"].(map[string]any)["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
		t.Fatalf("GraphQL read = %v", data)
	}
	if read, _ := a.mcp("get_agent_automation", map[string]any{"ownerId": "tea-a", "automation": restID}, "dev"); read["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
		t.Fatalf("MCP read = %v", read)
	}
	adminOn, err := a.svc.CreateAgentAutomation(caller("admin"), "tea-a", AgentAutomationInput{Name: str("admin-on"), Triggers: dailyTrigger("09:00"), Template: template("t", ptr(true))})
	if err != nil || bindingOf(adminOn.ID) != true {
		t.Fatalf("an admin's on = %+v err=%v", adminOn, err)
	}
	// The admin who created it changes it in any direction.
	for _, binding := range []*bool{nil, ptr(false), ptr(true)} {
		if _, err := a.svc.UpdateAgentAutomation(caller("admin"), "tea-a", adminOn.ID, AgentAutomationInput{Template: template("t", binding)}); err != nil {
			t.Fatalf("an admin's change to %v = %v", binding, err)
		}
	}

	// A member's own automation: one they saved with no binding cannot gain
	// one, and one that carries a binding (an admin's, or one saved before
	// w10/044) is kept by a save that leaves it alone and cannot be changed,
	// not even back to inherit.
	ctx := caller("dev")
	status, created = a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", automationBody("member-inherit"), "dev")
	if status != http.StatusCreated {
		t.Fatalf("a member's create naming no binding = %d %v", status, created)
	}
	memberID := created["id"].(string)
	for _, binding := range []*bool{ptr(false), ptr(true)} {
		if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", memberID, AgentAutomationInput{Template: template("t", binding)}); lockedReason(err, CodeAutomationGitHubCLILocked) != "admin_required" {
			t.Fatalf("a member's change from inherit to %v = %v", *binding, err)
		}
	}
	if status, out := a.rest(http.MethodPatch, "/v1/agent-automations/"+memberID+"?ownerId=tea-a", map[string]any{"template": withBinding("x", false)["template"]}, "dev"); status != http.StatusForbidden || out["code"] != CodeAutomationGitHubCLILocked {
		t.Fatalf("REST member update to off = %d %v", status, out)
	}
	if _, code := a.gql(`mutation { updateAgentAutomation(ownerId: "tea-a", id: "`+memberID+`", input: {template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex", model: "gpt-5", githubCli: false}}}) { id } }`, "dev"); code != CodeAutomationGitHubCLILocked {
		t.Fatalf("GraphQL member update to off = %s", code)
	}
	if _, errText := a.mcp("update_agent_automation", map[string]any{"ownerId": "tea-a", "automation": memberID, "template": withBinding("x", false)["template"]}, "dev"); !strings.HasPrefix(errText, CodeAutomationGitHubCLILocked) {
		t.Fatalf("MCP member update to off = %s", errText)
	}
	if bindingOf(memberID) != nil {
		t.Fatalf("a refused update changed the binding to %v", bindingOf(memberID))
	}
	for _, stored := range []bool{false, true} {
		row, _ := a.fake.GetAgentAutomation(context.Background(), memberID)
		row.Template, _ = json.Marshal(template("t", ptr(stored)))
		a.fake.rows[memberID] = row
		if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", memberID, AgentAutomationInput{Template: template("a new task", ptr(stored))}); err != nil || bindingOf(memberID) != stored {
			t.Fatalf("a member's save keeping a stored %v = %v (binding %v)", stored, err, bindingOf(memberID))
		}
		if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", memberID, AgentAutomationInput{Name: str(fmt.Sprintf("renamed-%v", stored))}); err != nil || bindingOf(memberID) != stored {
			t.Fatalf("a rename = %v (binding %v), want the stored %v kept", err, bindingOf(memberID), stored)
		}
		for _, binding := range []*bool{nil, ptr(!stored)} {
			if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", memberID, AgentAutomationInput{Template: template("t", binding)}); lockedReason(err, CodeAutomationGitHubCLILocked) != "admin_required" {
				t.Fatalf("a member's change from %v to %v = %v, want admin_required", stored, binding, err)
			}
			if bindingOf(memberID) != stored {
				t.Fatalf("a refused update changed the stored %v to %v", stored, bindingOf(memberID))
			}
		}
	}

	// Under a mandatory off nobody turns it on, an admin may turn it off, a
	// member still names nothing, and a save that keeps a stored on keeps it.
	envs := a.svc.Environments.(*fakeEnvironments)
	envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, GitHubCLIDisabled: true, GitHubCLIMandatory: true}
	if _, err := a.svc.CreateAgentAutomation(caller("admin"), "tea-a", AgentAutomationInput{Name: str("admin-on-locked"), Triggers: dailyTrigger("09:00"), Template: template("t", ptr(true))}); lockedReason(err, CodeAutomationGitHubCLILocked) != "mandatory" {
		t.Fatalf("an admin's on under a mandatory off = %v", err)
	}
	if status, out := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-on-locked", true), "admin"); status != http.StatusForbidden || out["params"].(map[string]any)["reason"] != "mandatory" {
		t.Fatalf("REST admin create on under a mandatory off = %d %v", status, out)
	}
	if status, _ := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-off-locked", false), "admin"); status != http.StatusCreated {
		t.Fatalf("REST admin create off under a mandatory off = %d", status)
	}
	if status, out := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-member-off-locked", false), "dev"); status != http.StatusForbidden || out["params"].(map[string]any)["reason"] != "admin_required" {
		t.Fatalf("REST member create off under a mandatory off = %d %v", status, out)
	}
	if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", memberID, AgentAutomationInput{Template: template("another task", ptr(true))}); err != nil || bindingOf(memberID) != true {
		t.Fatalf("a save keeping a stored on under a mandatory off = %v (binding %v)", err, bindingOf(memberID))
	}
}

// The dispatch resolves the chain as the mints do: a session's own off, its
// automation's off, the automations default's off and the workspace's
// mandatory off each reach the driver with their level, re-resolved at every
// turn, and a child is governed by its parent's chain.
func TestDispatchResolvesTheGitHubCLIBindingChain(t *testing.T) {
	svc, st, lifecycle := adminAndMembers(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries})
	automations := newFakeAutomations()
	svc.Automations = automations
	envs := svc.Environments.(*fakeEnvironments)
	setEnv := func(row store.AgentEnvironment) {
		row.WorkspaceID, row.NetworkTier = "tea-a", NetworkTierRegistries
		envs.rows["tea-a"] = row
	}
	expect := func(when, level string) {
		t.Helper()
		got, present := lifecycle.driverEnv[envGitHubCLI]
		if level == "" {
			if present {
				t.Fatalf("%s: %s = %q, want gh on", when, envGitHubCLI, got)
			}
			if _, ok := lifecycle.driverEnv[envGitHubCLILevel]; ok {
				t.Fatalf("%s: a level with gh on", when)
			}
			return
		}
		if got != "0" || lifecycle.driverEnv[envGitHubCLILevel] != level {
			t.Fatalf("%s: %s=%q %s=%q, want 0 and %q", when, envGitHubCLI, got, envGitHubCLILevel, lifecycle.driverEnv[envGitHubCLILevel], level)
		}
	}
	steer := func(t *testing.T, who, id string) {
		t.Helper()
		row := st.rows[id]
		row.Phase = PhaseCompleted
		st.rows[id] = row
		if _, err := svc.Steer(caller(who), SteerRequest{SessionID: id, Prompt: "again"}); err != nil {
			t.Fatal(err)
		}
	}
	input := func(binding *bool) CreateRequest {
		return CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: binding}}
	}

	// The session's own off under a workspace on, which only an admin names.
	own, err := svc.Create(caller("admin"), input(ptr(false)))
	if err != nil {
		t.Fatal(err)
	}
	expect("a session's own off", "session")
	// An admin's on under a workspace off that is not mandatory, until the
	// off is made mandatory: its next turn has no gh.
	setEnv(store.AgentEnvironment{GitHubCLIDisabled: true})
	pinned, err := svc.Create(caller("admin"), input(ptr(true)))
	if err != nil {
		t.Fatal(err)
	}
	expect("an admin's on under a workspace off", "")
	setEnv(store.AgentEnvironment{GitHubCLIDisabled: true, GitHubCLIMandatory: true})
	steer(t, "admin", pinned.ID)
	expect("an admin's on once the off is mandatory", "workspace")
	setEnv(store.AgentEnvironment{})
	steer(t, "admin", own.ID)
	expect("a session's own off with everything above on", "session")

	// An automation's session: the automation's off, then (inherit) the
	// automations default's off, then the automations default's on under a
	// workspace off, read live at each turn.
	template, _ := json.Marshal(AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", GitHubCLI: ptr(false)}})
	automations.rows["aut-nightly"] = store.AgentAutomation{ID: "aut-nightly", WorkspaceID: "tea-a", CreatedBy: "alice", Template: template}
	fired := input(nil)
	fired.AutomationID = "aut-nightly"
	automated, err := svc.Create(caller("alice"), fired)
	if err != nil {
		t.Fatal(err)
	}
	expect("an automation's off", "automation")
	template, _ = json.Marshal(AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex"}})
	automations.rows["aut-nightly"] = store.AgentAutomation{ID: "aut-nightly", WorkspaceID: "tea-a", CreatedBy: "alice", Template: template}
	setEnv(store.AgentEnvironment{AutomationsGitHubCLI: ptr(false)})
	steer(t, "alice", automated.ID)
	expect("the automations default's off", "automations-default")
	setEnv(store.AgentEnvironment{GitHubCLIDisabled: true, AutomationsGitHubCLI: ptr(true)})
	steer(t, "alice", automated.ID)
	expect("the automations default's on under a workspace off", "")
	// The same automations default never binds a session a person started.
	steer(t, "admin", own.ID)
	expect("a person's session under the automations default's on", "session")
	delete(automations.rows, "aut-nightly")
	setEnv(store.AgentEnvironment{AutomationsGitHubCLI: ptr(false)})
	steer(t, "alice", automated.ID)
	expect("a deleted automation's session", "automations-default")

	// A child: its parent's off governs it, even when an admin's create names
	// on, and a spawn (which names nothing) inherits it.
	setEnv(store.AgentEnvironment{})
	parent, err := svc.Create(caller("admin"), input(ptr(false)))
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[parent.ID]
	row.Phase = PhaseRunning
	st.rows[parent.ID] = row
	childIn := input(ptr(true))
	childIn.ParentSessionID = parent.ID
	child, err := svc.Create(caller("admin"), childIn)
	if err != nil {
		t.Fatal(err)
	}
	if child.AgentConfig.GitHubCLI == nil || !*child.AgentConfig.GitHubCLI {
		t.Fatalf("the child's stored binding = %v, want the true it named", child.AgentConfig.GitHubCLI)
	}
	expect("a child naming on under its parent's off", "session")
	row = st.rows[parent.ID]
	row.CreatedBy = "alice"
	st.rows[parent.ID] = row
	if _, err := svc.SpawnChild(context.Background(), parent.ID, SpawnRequest{Task: "a sub-task"}); err != nil {
		t.Fatal(err)
	}
	expect("a spawned child of a session with gh off", "session")
	// A child of an automation's session follows the automation.
	template, _ = json.Marshal(AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", GitHubCLI: ptr(false)}})
	automations.rows["aut-nightly"] = store.AgentAutomation{ID: "aut-nightly", WorkspaceID: "tea-a", CreatedBy: "alice", Template: template}
	row = st.rows[automated.ID]
	row.Phase, row.CreatedBy = PhaseRunning, "alice"
	st.rows[automated.ID] = row
	if _, err := svc.SpawnChild(context.Background(), automated.ID, SpawnRequest{Task: "a sub-task"}); err != nil {
		t.Fatal(err)
	}
	expect("a child of an automation's session", "automation")

	// A repo-less session never gets the variables: it has no gh at all.
	if _, err := svc.Create(caller("admin"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: ptr(false)}}); err != nil {
		t.Fatal(err)
	}
	expect("a repo-less session", "")
}
