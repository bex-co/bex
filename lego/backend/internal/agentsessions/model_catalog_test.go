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
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// storedModel is the model a session's stored agent config names.
func storedModel(t *testing.T, st *fakeStore, id string) string {
	t.Helper()
	var config AgentConfig
	if err := json.Unmarshal(st.rows[id].AgentConfig, &config); err != nil {
		t.Fatal(err)
	}
	return config.Model
}

// settleTurn ends a session's running turn so the next message starts one.
func settleTurn(st *fakeStore, id string) {
	row := st.rows[id]
	row.Phase = PhaseCompleted
	st.rows[id] = row
}

// w10/m516 t003: a message may switch the session's model from the turn it
// starts, as Devin's agent selector does: the sandbox runs it under all four
// names, the session records it and reads it back, and the next message
// without a model keeps it. A model the catalog does not offer is refused at
// send, naming the field and the offered models, and starts nothing.
func TestAMessageSwitchesTheModelFromItsTurn(t *testing.T) {
	enablePlatformCatalog(t)
	svc, st, lifecycle := sessionWithEnvironment(t, nil)
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.Model != agentsession.DefaultDevModel {
		t.Fatalf("created on %q, want the catalog default", created.AgentConfig.Model)
	}

	opus := testCatalog[2]
	settleTurn(st, created.ID)
	steered, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "now think harder", Model: " " + opus + " "})
	if err != nil {
		t.Fatalf("steer with %s: %v", opus, err)
	}
	if steered.AgentConfig.Model != opus || storedModel(t, st, created.ID) != opus {
		t.Fatalf("after the switch: view %q stored %q, want %q", steered.AgentConfig.Model, storedModel(t, st, created.ID), opus)
	}
	for _, name := range routeModelVars {
		if lifecycle.driverEnv[name] != opus {
			t.Fatalf("switched turn %s = %q, want %q", name, lifecycle.driverEnv[name], opus)
		}
	}
	if read, err := svc.Get(caller("alice"), created.ID); err != nil || read.AgentConfig.Model != opus {
		t.Fatalf("session read = %q err=%v, want %q", read.AgentConfig.Model, err, opus)
	}

	settleTurn(st, created.ID)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "and the docs"}); err != nil {
		t.Fatal(err)
	}
	if lifecycle.driverEnv["ANTHROPIC_MODEL"] != opus || storedModel(t, st, created.ID) != opus {
		t.Fatalf("a message without a model ran %q (stored %q), want the session's %q", lifecycle.driverEnv["ANTHROPIC_MODEL"], storedModel(t, st, created.ID), opus)
	}

	settleTurn(st, created.ID)
	turns, sandboxes := st.rows[created.ID].Turns, lifecycle.created
	_, err = svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "cheaper please", Model: "anthropic/claude-sonnet-4.5"})
	var coded *core.CodedError
	if !isCode(err, CodeSessionModelNotOffered) || !errors.As(err, &coded) || coded.Params["field"] != "model" ||
		!slices.Equal(coded.Params["models"].([]string), testCatalog) {
		t.Fatalf("steer with a model outside the catalog = %v, want %s on field model", err, CodeSessionModelNotOffered)
	}
	if st.rows[created.ID].Turns != turns || lifecycle.created != sandboxes || storedModel(t, st, created.ID) != opus {
		t.Fatal("a refused model started a turn or changed the session's model")
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "x", Model: "claude opus; rm -rf /"}); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("malformed model = %v, want AGENT_SESSION_INPUT_INVALID", err)
	}
}

// On the workspace's own key a message names any model its provider serves;
// a profile with one fixed model refuses another (AGENT_SESSION_MODEL_PINNED).
func TestAMessageModelOffTheCatalog(t *testing.T) {
	enablePlatformCatalog(t)
	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	svc, st, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, ModelKeySetAt: &at})
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	settleTurn(st, created.ID)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again", Model: "claude-opus-4-1"}); err != nil {
		t.Fatalf("own-key steer with a free-form model: %v", err)
	}
	if lifecycle.driverEnv["ANTHROPIC_MODEL"] != "claude-opus-4-1" || storedModel(t, st, created.ID) != "claude-opus-4-1" {
		t.Fatalf("own-key switch ran %q stored %q", lifecycle.driverEnv["ANTHROPIC_MODEL"], storedModel(t, st, created.ID))
	}

	// A deepseek session (ADR114) runs only its own model.
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	row.AgentConfig = []byte(`{"agent":"deepseek","model":"~deepseek/deepseek-flash-latest","task":"fix the tests"}`)
	st.rows[created.ID] = row
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again", Model: testCatalog[1]}); !isCode(err, "AGENT_SESSION_MODEL_PINNED") {
		t.Fatalf("deepseek steer with another model = %v, want AGENT_SESSION_MODEL_PINNED", err)
	}
}

// t004: REST, GraphQL and MCP give every offered agent the model a session
// that names none runs, and the models it may name: the catalog for claude
// on the platform route (with the workspace default as its model), the one
// fixed model for deepseek, and none (free-form) on the workspace's own key.
func TestCapabilitiesShowEachAgentsModelOnEverySurface(t *testing.T) {
	enablePlatformCatalog(t)
	svc, _, _ := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultModel: testCatalog[1]})
	envs := svc.Environments.(*fakeEnvironments)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	a := &automationAdapters{t: t, svc: svc, mux: mux}
	type offer struct {
		model  any
		models []string
	}
	surfaces := func() map[string]map[string]offer {
		t.Helper()
		offers := func(name string, agents any) map[string]offer {
			list, _ := agents.([]any)
			if len(list) == 0 {
				t.Fatalf("%s: agents = %#v", name, agents)
			}
			out := map[string]offer{}
			for _, raw := range list {
				agent, _ := raw.(map[string]any)
				models, ok := agent["models"].([]any)
				if !ok {
					t.Fatalf("%s: agent %#v has no models list", name, raw)
				}
				o := offer{model: agent["model"], models: []string{}}
				for _, m := range models {
					o.models = append(o.models, m.(string))
				}
				out[agent["id"].(string)] = o
			}
			return out
		}
		code, rest := a.rest("GET", "/v1/agent-sessions/capabilities", nil, "alice")
		if code != http.StatusOK {
			t.Fatalf("REST capabilities = %d %v", code, rest)
		}
		gql, gqlErr := a.gql(`{ agentSessionCapabilities { agents { id model models } } }`, "alice")
		if gqlErr != "" {
			t.Fatalf("GraphQL capabilities: %s", gqlErr)
		}
		mcpOut, mcpErr := a.mcp("get_agent_session_capabilities", map[string]any{}, "alice")
		if mcpErr != "" {
			t.Fatalf("MCP capabilities: %s", mcpErr)
		}
		gqlCaps, _ := gql["agentSessionCapabilities"].(map[string]any)
		return map[string]map[string]offer{
			"REST": offers("REST", rest["agents"]), "GraphQL": offers("GraphQL", gqlCaps["agents"]), "MCP": offers("MCP", mcpOut["agents"]),
		}
	}
	check := func(label string, want map[string]offer) {
		t.Helper()
		for surface, got := range surfaces() {
			for id, w := range want {
				g, ok := got[id]
				if !ok || g.model != w.model || !slices.Equal(g.models, w.models) {
					t.Errorf("%s, %s: %s = %+v (offered %v), want %+v", label, surface, id, g, ok, w)
				}
			}
		}
	}
	deepseek := offer{model: "~deepseek/deepseek-flash-latest", models: []string{"~deepseek/deepseek-flash-latest"}}
	check("platform route", map[string]offer{
		"claude": {model: testCatalog[1], models: testCatalog}, "deepseek": deepseek, "codex": {model: nil, models: []string{}},
	})

	row := envs.rows["tea-a"]
	row.DefaultModel = ""
	envs.rows["tea-a"] = row
	check("platform route, no default", map[string]offer{"claude": {model: agentsession.DefaultDevModel, models: testCatalog}})

	// w10/m527: a model an admin turned off leaves every surface's models,
	// and a workspace default that is off gives way to the catalog default.
	row.DisabledModels, row.DefaultModel = []string{testCatalog[2]}, testCatalog[1]
	envs.rows["tea-a"] = row
	check("one model off", map[string]offer{"claude": {model: testCatalog[1], models: testCatalog[:2]}, "deepseek": deepseek})
	row.DisabledModels = []string{testCatalog[1], testCatalog[2]}
	envs.rows["tea-a"] = row
	check("the default off", map[string]offer{"claude": {model: agentsession.DefaultDevModel, models: testCatalog[:1]}})
	row.DisabledModels = nil

	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	row.ModelKeySetAt, row.DefaultModel = &at, "claude-opus-4-1"
	envs.rows["tea-a"] = row
	check("own key", map[string]offer{"claude": {model: "claude-opus-4-1", models: []string{}}, "deepseek": deepseek})
	row.DefaultModel = ""
	envs.rows["tea-a"] = row
	check("own key, no default", map[string]offer{"claude": {model: nil, models: []string{}}})
}

// An automation's template is held to the catalog when it is saved, like a
// manual create; a run whose stored model has since left the catalog runs the
// workspace's model and says so in its run record instead of failing.
func TestAutomationTemplateModelAndARetiredOne(t *testing.T) {
	enablePlatformCatalog(t)
	svc, _, _ := sessionWithEnvironment(t, nil)
	tpl := AutomationTemplate{Repo: "acme/api", Task: "triage", AgentConfig: AutomationAgentConfig{Agent: "claude", Model: "anthropic/claude-sonnet-4.5"}, MaxSpendUSD: ptr(5.0)}
	if _, err := svc.validateAutomationTemplate(caller("alice"), "tea-a", tpl); !isCode(err, CodeSessionModelNotOffered) {
		t.Fatalf("template with a model outside the catalog = %v, want %s", err, CodeSessionModelNotOffered)
	}
	tpl.AgentConfig.Model = testCatalog[2]
	saved, err := svc.validateAutomationTemplate(caller("alice"), "tea-a", tpl)
	if err != nil || saved.AgentConfig.Model != testCatalog[2] {
		t.Fatalf("template on %s = %+v err=%v", testCatalog[2], saved.AgentConfig, err)
	}

	req := CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Model: testCatalog[2]}}
	if note := svc.retiredTemplateModel(caller("alice"), &req); note != "" || req.AgentConfig.Model != testCatalog[2] {
		t.Fatalf("a catalog model was retired: %q %q", note, req.AgentConfig.Model)
	}
	req.AgentConfig.Model = "anthropic/claude-sonnet-4.5"
	note := svc.retiredTemplateModel(caller("alice"), &req)
	if req.AgentConfig.Model != "" || note != "the template's model anthropic/claude-sonnet-4.5 is no longer offered; this run used "+agentsession.DefaultDevModel {
		t.Fatalf("retired model: model %q note %q", req.AgentConfig.Model, note)
	}
}
