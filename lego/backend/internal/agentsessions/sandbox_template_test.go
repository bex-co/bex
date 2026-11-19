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
	"reflect"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// adminFGA makes fakeFGA's members admins of their workspace, so a test can
// also read and set its agent environment.
type adminFGA struct{ *fakeFGA }

func (a adminFGA) Check(ctx context.Context, subject, relation, object string) (bool, error) {
	if relation == core.RelCanView || relation == core.RelCanManage {
		relation = core.RelCanOperate
	}
	return a.fakeFGA.Check(ctx, subject, relation, object)
}

// templateFixture is a session service whose tea-a environment is row and
// whose member alice is an admin of tea-a.
func templateFixture(t *testing.T, row *store.AgentEnvironment) (*Service, *fakeStore, *fakeLifecycle) {
	t.Helper()
	svc, st, lc := sessionWithEnvironment(t, row)
	svc.Authz = adminFGA{svc.Authz.(*fakeFGA)}
	return svc, st, lc
}

// The workspace default sandbox template (w10/m519, ADR047 D1 amendment
// 2026-11-19): a session that names no template runs on it and records it,
// so its read reports it and a later change of the default leaves it alone;
// an explicit template still wins.
func TestCreateNamingNoTemplateRunsOnTheWorkspaceDefault(t *testing.T) {
	svc, st, lc := templateFixture(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultTemplate: "agent-4c-8g"})
	view, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if lc.template != "agent-4c-8g" || view.AgentConfig.Template != "agent-4c-8g" {
		t.Fatalf("a create naming no template ran on %q (view %q), want the workspace default agent-4c-8g", lc.template, view.AgentConfig.Template)
	}

	in := createInput()
	in.AgentConfig.Template = "agent"
	explicit, err := svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	if lc.template != "agent" || explicit.AgentConfig.Template != "agent" {
		t.Fatalf("a create naming agent ran on %q (view %q), want agent", lc.template, explicit.AgentConfig.Template)
	}

	// The default changes; the session keeps the template it was created on,
	// in its read and on its next turn.
	if _, err := svc.UpdateEnvironment(caller("alice"), "tea-a", EnvironmentInput{DefaultTemplate: ptr("")}); err != nil {
		t.Fatal(err)
	}
	read, err := svc.Get(caller("alice"), view.ID)
	if err != nil || read.AgentConfig.Template != "agent-4c-8g" {
		t.Fatalf("read after the default changed = %q err=%v, want agent-4c-8g", read.AgentConfig.Template, err)
	}
	row := st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: view.ID, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if lc.template != "agent-4c-8g" {
		t.Fatalf("the next turn ran on %q, want the session's agent-4c-8g", lc.template)
	}
	if next, err := svc.Create(caller("alice"), createInput()); err != nil || next.AgentConfig.Template != "agent" || lc.template != "agent" {
		t.Fatalf("a create after the default returned to agent = %q on %q err=%v", next.AgentConfig.Template, lc.template, err)
	}
}

// A stored default whose template the platform no longer registers falls
// back to the platform's, in the create and in both reads, never failing a
// session.
func TestARetiredDefaultTemplateRunsOnTheAgentTemplate(t *testing.T) {
	svc, _, lc := templateFixture(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultTemplate: "agent-2c-2g"})
	view, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if lc.template != sandbox.DefaultAgentTemplate || view.AgentConfig.Template != sandbox.DefaultAgentTemplate {
		t.Fatalf("a retired default ran on %q (view %q), want agent", lc.template, view.AgentConfig.Template)
	}
	env, err := svc.Environment(caller("alice"), "tea-a")
	if err != nil || !reflect.DeepEqual(env.TemplateOffer, offerOf("agent")) {
		t.Fatalf("environment with a retired default = %+v err=%v, want agent", env.TemplateOffer, err)
	}
	caps, err := svc.Capabilities(caller("alice"), "tea-a")
	if err != nil || !reflect.DeepEqual(caps.TemplateOffer, offerOf("agent")) {
		t.Fatalf("capabilities with a retired default = %+v err=%v, want agent", caps.TemplateOffer, err)
	}
}

// A default that is not an agent template is refused with the templates
// there are, and nothing is written.
func TestTheEnvironmentRefusesADefaultThatIsNotAnAgentTemplate(t *testing.T) {
	svc := environmentService("tea-a", newFakeEnvironments())
	_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultTemplate: ptr("base")})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != CodeEnvironmentTemplateInvalid ||
		coded.Params["field"] != "defaultTemplate" || coded.Params["template"] != "base" ||
		!reflect.DeepEqual(coded.Params["templates"], []string{"agent", "agent-4c-8g"}) {
		t.Fatalf("default base = %#v, want %s on defaultTemplate listing the templates", err, CodeEnvironmentTemplateInvalid)
	}
	if want := `defaultTemplate "base" is not an agent template; use one of agent (2 CPU, 4 GB), agent-4c-8g (4 CPU, 8 GB)`; coded.Error() != want {
		t.Fatalf("refusal = %q, want %q", coded.Error(), want)
	}
	if env, err := svc.Environment(caller("admin"), "tea-a"); err != nil || env.UpdatedAt != nil || env.DefaultTemplate != "agent" {
		t.Fatalf("a refused default was written: %+v err=%v", env, err)
	}
}

// An automation whose template names none stores none, and each run takes
// the workspace default in force when it fires.
func TestAnAutomationNamingNoTemplateRunsOnTheDefaultWhenItFires(t *testing.T) {
	svc, st, lc := templateFixture(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultTemplate: "agent-4c-8g"})
	tpl := AutomationTemplate{Task: "update", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"}}
	saved, err := svc.validateAutomationTemplate(caller("alice"), "tea-a", tpl)
	if err != nil || saved.AgentConfig.Template != "" {
		t.Fatalf("saved template = %+v err=%v, want no template stored", saved.AgentConfig, err)
	}
	stored, _ := json.Marshal(saved)
	table := &claimingAutomations{fakeAutomations: newFakeAutomations(), settled: map[string]string{}}
	svc.Automations = table
	row, err := table.CreateAgentAutomation(context.Background(), store.AgentAutomation{
		ID: ids.New(ids.AgentAutomation), WorkspaceID: "tea-a", Name: "nightly", CreatedBy: "alice", Template: stored})
	if err != nil {
		t.Fatal(err)
	}
	fire := func(want string) {
		t.Helper()
		st.now = st.now.Add(time.Hour) // each run-now is its own fire instant
		if _, err := svc.RunAgentAutomationNow(caller("alice"), "tea-a", row.ID); err != nil {
			t.Fatal(err)
		}
		if lc.template != want {
			t.Fatalf("the run's sandbox template = %q, want %q", lc.template, want)
		}
		for _, session := range st.rows {
			config, _ := decodeAgentConfig(session)
			if session.AutomationID == row.ID && session.CreatedAt.Equal(st.now) && config.Template != want {
				t.Fatalf("the run's session records %q, want %q", config.Template, want)
			}
		}
	}
	fire("agent-4c-8g")
	if _, err := svc.UpdateEnvironment(caller("alice"), "tea-a", EnvironmentInput{DefaultTemplate: ptr("agent")}); err != nil {
		t.Fatal(err)
	}
	fire("agent")
}

// A session created before its template was recorded ran on the platform
// default, and its read says so.
func TestASessionWithNoRecordedTemplateReadsTheAgentTemplate(t *testing.T) {
	config, _ := json.Marshal(AgentConfig{Agent: "codex", Task: "fix"})
	view, err := viewOf(store.AgentSession{ID: "ags-legacy", AgentConfig: config})
	if err != nil || view.AgentConfig.Template != sandbox.DefaultAgentTemplate {
		t.Fatalf("legacy session template = %q err=%v, want agent", view.AgentConfig.Template, err)
	}
}

// The capabilities and environment reads serve the same templates, from the
// sandbox lifecycle (a made-up third one shows nothing is hard-coded), on
// REST, GraphQL and MCP, with the workspace default.
func TestTemplatesAreServedOnEverySurface(t *testing.T) {
	svc, _, lc := templateFixture(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultTemplate: "agent-8c-16g"})
	lc.agentTemplates = []sandbox.AgentTemplate{
		{Name: "agent", CPU: "2", Memory: "4Gi"}, {Name: "agent-4c-8g", CPU: "4", Memory: "8Gi"}, {Name: "agent-8c-16g", CPU: "8", Memory: "16Gi"},
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	a := &automationAdapters{t: t, svc: svc, mux: mux}
	want := []any{
		map[string]any{"name": "agent", "cpu": 2.0, "memoryGb": 4.0},
		map[string]any{"name": "agent-4c-8g", "cpu": 4.0, "memoryGb": 8.0},
		map[string]any{"name": "agent-8c-16g", "cpu": 8.0, "memoryGb": 16.0},
	}
	check := func(surface string, read map[string]any) {
		t.Helper()
		if read["defaultTemplate"] != "agent-8c-16g" || !reflect.DeepEqual(read["templates"], want) {
			t.Errorf("%s: defaultTemplate %v, templates %v; want agent-8c-16g and %v", surface, read["defaultTemplate"], read["templates"], want)
		}
	}
	fields := `defaultTemplate templates { name cpu memoryGb }`
	gql, gqlErr := a.gql(`{ agentSessionCapabilities(ownerId: "tea-a") { `+fields+` } agentEnvironment(ownerId: "tea-a") { `+fields+` } }`, "alice")
	if gqlErr != "" {
		t.Fatalf("GraphQL: %s", gqlErr)
	}
	check("GraphQL capabilities", gql["agentSessionCapabilities"].(map[string]any))
	check("GraphQL environment", gql["agentEnvironment"].(map[string]any))
	for _, read := range []struct{ path, tool string }{
		{"/v1/agent-sessions/capabilities?ownerId=tea-a", "get_agent_session_capabilities"},
		{"/v1/agent-environment?ownerId=tea-a", "get_agent_environment"},
	} {
		code, rest := a.rest(http.MethodGet, read.path, nil, "alice")
		if code != http.StatusOK {
			t.Fatalf("REST %s = %d %v", read.path, code, rest)
		}
		check("REST "+read.path, rest)
		out, mcpErr := a.mcp(read.tool, map[string]any{"ownerId": "tea-a"}, "alice")
		if mcpErr != "" {
			t.Fatalf("MCP %s: %s", read.tool, mcpErr)
		}
		check("MCP "+read.tool, out)
	}
}
