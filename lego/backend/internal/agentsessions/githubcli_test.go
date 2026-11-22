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

// A session create carries agentConfig.githubCli on REST, GraphQL and MCP:
// a member's false is stored and read back on all three, a member's true is
// refused admin_required on all three, and under a mandatory off an admin's
// true is refused mandatory on all three.
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
		id   string
		code string
	}
	creates := map[string]func(binding bool, who string) created{
		"REST": func(binding bool, who string) created {
			raw, _ := json.Marshal(body(binding))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(string(raw))).WithContext(caller(who)))
			var out map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
			if rec.Code == http.StatusCreated {
				return created{id: out["id"].(string)}
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("REST create = %d %s", rec.Code, rec.Body.String())
			}
			return created{code: rec.Body.String()}
		},
		"GraphQL": func(binding bool, who string) created {
			result := graphql.Do(graphql.Params{Schema: schema, Context: caller(who),
				RequestString:  `mutation($b: Boolean) { createAgentSession(ownerId: "tea-a", repo: "bex-co/example", agentConfig: {agent: "codex", task: "t", githubCli: $b}) { id agentConfig { githubCli } } }`,
				VariableValues: map[string]any{"b": binding}})
			if len(result.Errors) != 0 {
				code, _ := result.Errors[0].Extensions["code"].(string)
				return created{code: code}
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
				return created{code: res.Content[0].(*mcp.TextContent).Text}
			}
			return created{id: res.StructuredContent.(map[string]any)["id"].(string)}
		},
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
		got := create(false, "alice")
		if got.id == "" {
			t.Fatalf("%s: a member's githubCli false was refused: %s", surface, got.code)
		}
		if binding := githubCliOf(t, st.rows[got.id].AgentConfig); binding == nil || *binding {
			t.Fatalf("%s: stored githubCli = %v, want false", surface, binding)
		}
		if rest, gql, mcpRead := readBack(got.id); rest != false || gql != false || mcpRead != false {
			t.Fatalf("%s: read back REST %v GraphQL %v MCP %v, want false", surface, rest, gql, mcpRead)
		}
		rows := len(st.rows)
		if got := create(true, "alice"); got.id != "" || !strings.Contains(got.code, CodeSessionGitHubCLILocked) {
			t.Fatalf("%s: a member's githubCli true = %+v, want %s", surface, got, CodeSessionGitHubCLILocked)
		}
		if got := create(true, "admin"); got.id == "" {
			t.Fatalf("%s: an admin's githubCli true was refused: %s", surface, got.code)
		}
		if len(st.rows) != rows+1 {
			t.Fatalf("%s: rows = %d, want only the admin's create added", surface, len(st.rows))
		}
	}
	if _, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: ptr(true)}}); lockedReason(err, CodeSessionGitHubCLILocked) != "admin_required" {
		t.Fatalf("service: a member's true = %v, want admin_required", err)
	}
	omitted, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t"}})
	if err != nil || omitted.AgentConfig.GitHubCLI != nil || githubCliOf(t, st.rows[omitted.ID].AgentConfig) != nil {
		t.Fatalf("a create naming no binding = %+v err=%v, want it absent", omitted.AgentConfig, err)
	}
	if rest, gql, mcpRead := readBack(omitted.ID); rest != nil || gql != nil || mcpRead != nil {
		t.Fatalf("an unbound session reads REST %v GraphQL %v MCP %v, want null on each", rest, gql, mcpRead)
	}

	// Under the workspace's mandatory off nobody turns gh on, on any surface.
	envs := svc.Environments.(*fakeEnvironments)
	envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, GitHubCLIDisabled: true, GitHubCLIMandatory: true}
	for surface, create := range creates {
		if got := create(true, "admin"); got.id != "" || !strings.Contains(got.code, CodeSessionGitHubCLILocked) {
			t.Fatalf("%s: an admin's true under a mandatory off = %+v, want %s", surface, got, CodeSessionGitHubCLILocked)
		}
		if got := create(false, "alice"); got.id == "" {
			t.Fatalf("%s: a member's false under a mandatory off was refused: %s", surface, got.code)
		}
	}
	if _, err := svc.Create(caller("admin"), CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: ptr(true)}}); lockedReason(err, CodeSessionGitHubCLILocked) != "mandatory" {
		t.Fatalf("service: an admin's true under a mandatory off = %v, want mandatory", err)
	}
}

// An automation template carries agentConfig.githubCli on REST, GraphQL and
// MCP; a member's true is refused, an admin's is kept, a creator's save that
// leaves a stored true alone keeps it, and under a mandatory off nobody turns
// it on.
func TestAutomationGitHubCLIBindingOnEverySurface(t *testing.T) {
	a := newAutomationAdapters(t)
	withBinding := func(name string, binding any) map[string]any {
		body := automationBody(name)
		body["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] = binding
		return body
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

	// REST
	status, created := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-off", false), "dev")
	if status != http.StatusCreated || created["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
		t.Fatalf("REST create off = %d %v", status, created)
	}
	restID := created["id"].(string)
	if status, out := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-on", true), "dev"); status != http.StatusForbidden || !strings.Contains(out["error"].(string)+out["code"].(string), CodeAutomationGitHubCLILocked) {
		t.Fatalf("REST member create on = %d %v", status, out)
	}
	// GraphQL
	data, code := a.gql(`mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "gql-off", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}], template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex", model: "gpt-5", githubCli: false}}}) { id template { agentConfig { githubCli } } } }`, "dev")
	if code != "" || data["createAgentAutomation"].(map[string]any)["template"].(map[string]any)["agentConfig"].(map[string]any)["githubCli"] != false {
		t.Fatalf("GraphQL create off = %v %s", data, code)
	}
	gqlID := data["createAgentAutomation"].(map[string]any)["id"].(string)
	if _, code := a.gql(`mutation { createAgentAutomation(ownerId: "tea-a", input: {name: "gql-on", triggers: [{type: "schedule", schedule: {frequency: "day", at: "09:00", timezone: "Europe/Berlin"}}], template: {task: "t", repo: "bex-co/example", agentConfig: {agent: "codex", model: "gpt-5", githubCli: true}}}) { id } }`, "dev"); code != CodeAutomationGitHubCLILocked {
		t.Fatalf("GraphQL member create on = %s", code)
	}
	// MCP
	out, errText := a.mcp("create_agent_automation", withOwner(withBinding("mcp-off", false)), "dev")
	if errText != "" {
		t.Fatalf("MCP create off = %s", errText)
	}
	mcpID := out["id"].(string)
	if _, errText := a.mcp("create_agent_automation", withOwner(withBinding("mcp-on", true)), "dev"); !strings.Contains(errText, CodeAutomationGitHubCLILocked) {
		t.Fatalf("MCP member create on = %s", errText)
	}
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

	// A member's change to on is refused; an admin may create one.
	ctx := caller("dev")
	if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", restID, AgentAutomationInput{Template: &AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5", GitHubCLI: ptr(true)}}}); lockedReason(err, CodeAutomationGitHubCLILocked) != "admin_required" {
		t.Fatalf("a member's change to on = %v", err)
	}
	if bindingOf(restID) != false {
		t.Fatal("a refused update changed the binding")
	}
	adminOn, err := a.svc.CreateAgentAutomation(caller("admin"), "tea-a", AgentAutomationInput{Name: str("admin-on"), Triggers: dailyTrigger("09:00"),
		Template: &AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5", GitHubCLI: ptr(true)}}})
	if err != nil || bindingOf(adminOn.ID) != true {
		t.Fatalf("an admin's on = %+v err=%v", adminOn, err)
	}
	// A creator whose automation already carries on keeps it when a save
	// leaves it alone, and may turn it off or back to inherit.
	row, _ := a.fake.GetAgentAutomation(context.Background(), restID)
	row.Template, _ = json.Marshal(AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5", GitHubCLI: ptr(true)}})
	a.fake.rows[restID] = row
	if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", restID, AgentAutomationInput{Template: &AutomationTemplate{Task: "a new task", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5", GitHubCLI: ptr(true)}}}); err != nil || bindingOf(restID) != true {
		t.Fatalf("a creator's save keeping an admin's on = %v (binding %v)", err, bindingOf(restID))
	}
	if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", restID, AgentAutomationInput{Name: str("renamed")}); err != nil || bindingOf(restID) != true {
		t.Fatalf("a rename = %v (binding %v), want the binding kept", err, bindingOf(restID))
	}
	if _, err := a.svc.UpdateAgentAutomation(ctx, "tea-a", restID, AgentAutomationInput{Template: &AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"}}}); err != nil || bindingOf(restID) != nil {
		t.Fatalf("back to inherit = %v (binding %v)", err, bindingOf(restID))
	}

	// Under a mandatory off nobody turns it on, and off stays allowed.
	envs := a.svc.Environments.(*fakeEnvironments)
	envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, GitHubCLIDisabled: true, GitHubCLIMandatory: true}
	if _, err := a.svc.CreateAgentAutomation(caller("admin"), "tea-a", AgentAutomationInput{Name: str("admin-on-locked"), Triggers: dailyTrigger("09:00"),
		Template: &AutomationTemplate{Task: "t", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5", GitHubCLI: ptr(true)}}}); lockedReason(err, CodeAutomationGitHubCLILocked) != "mandatory" {
		t.Fatalf("an admin's on under a mandatory off = %v", err)
	}
	if status, out := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-on-locked", true), "admin"); status != http.StatusForbidden {
		t.Fatalf("REST admin create on under a mandatory off = %d %v", status, out)
	}
	if status, _ := a.rest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", withBinding("rest-off-locked", false), "dev"); status != http.StatusCreated {
		t.Fatalf("REST member create off under a mandatory off = %d", status)
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

	// The session's own off under a workspace on.
	own, err := svc.Create(caller("alice"), input(ptr(false)))
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
	steer(t, "alice", own.ID)
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
	steer(t, "alice", own.ID)
	expect("a person's session under the automations default's on", "session")
	delete(automations.rows, "aut-nightly")
	setEnv(store.AgentEnvironment{AutomationsGitHubCLI: ptr(false)})
	steer(t, "alice", automated.ID)
	expect("a deleted automation's session", "automations-default")

	// A child: its parent's off governs it, even when an admin's create names
	// on, and a spawn (which names nothing) inherits it.
	setEnv(store.AgentEnvironment{})
	parent, err := svc.Create(caller("alice"), input(ptr(false)))
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
	if _, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "codex", Task: "t", GitHubCLI: ptr(false)}}); err != nil {
		t.Fatal(err)
	}
	expect("a repo-less session", "")
}
