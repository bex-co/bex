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
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w10/m568: the workspace model list is Devin's two modes. Every catalog
// model is allowed, one added later included, until an admin turns one off;
// from then on the list is an allowlist, and a model added to the catalog
// afterwards is off until an admin turns it on. w10/m592 makes the mode
// explicit: only allowAllModels true allows all again.

// catalogAdded is the model growCatalog appends to testCatalog.
const catalogAdded = "anthropic/claude-haiku-5.5"

// grownCatalog is testCatalog after growCatalog.
var grownCatalog = append(slices.Clone(testCatalog), catalogAdded)

// growCatalog restarts the platform route with testCatalog plus models, as
// bex-api starting with more BEX_AGENT_PLATFORM_MODELS does.
func growCatalog(t *testing.T, models ...string) {
	t.Helper()
	list := append(slices.Clone(testCatalog[1:]), models...)
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", strings.Join(list, ","), platformKey); err != nil {
		t.Fatal(err)
	}
}

// modelSurfaces reads tea-a's claude capabilities `models` and its agent
// environment's models, enabledModels and newModels on REST, GraphQL and MCP.
type modelSurfaces struct{ a *automationAdapters }

func newModelSurfaces(t *testing.T, svc *Service) modelSurfaces {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return modelSurfaces{&automationAdapters{t: t, svc: svc, mux: mux}}
}

func strs(t *testing.T, raw any) []string {
	t.Helper()
	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("%#v is not a list", raw)
	}
	out := []string{}
	for _, m := range list {
		out = append(out, m.(string))
	}
	return out
}

// expect checks every surface: the capabilities offer and the environment's
// three lists.
func (m modelSurfaces) expect(label string, models, enabled, added []string) {
	t := m.a.t
	t.Helper()
	claude := func(agents any) []string {
		for _, raw := range agents.([]any) {
			if agent := raw.(map[string]any); agent["id"] == "claude" {
				return strs(t, agent["models"])
			}
		}
		t.Fatalf("%s: no claude agent in %#v", label, agents)
		return nil
	}
	code, restCaps := m.a.rest(http.MethodGet, "/v1/agent-sessions/capabilities", nil, "alice")
	code2, restEnv := m.a.rest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil, "admin")
	if code != http.StatusOK || code2 != http.StatusOK {
		t.Fatalf("%s: REST reads = %d %d", label, code, code2)
	}
	gql, gqlErr := m.a.gql(`{ agentSessionCapabilities { agents { id models } } }`, "alice")
	gqlEnv, gqlErr2 := m.a.gql(`{ agentEnvironment(ownerId: "tea-a") { models enabledModels newModels } }`, "admin")
	mcpCaps, mcpErr := m.a.mcp("get_agent_session_capabilities", map[string]any{}, "alice")
	mcpEnv, mcpErr2 := m.a.mcp("get_agent_environment", map[string]any{"ownerId": "tea-a"}, "admin")
	if gqlErr != "" || gqlErr2 != "" || mcpErr != "" || mcpErr2 != "" {
		t.Fatalf("%s: reads: %s %s %s %s", label, gqlErr, gqlErr2, mcpErr, mcpErr2)
	}
	for surface, got := range map[string][2]map[string]any{
		"REST":    {restCaps, restEnv},
		"GraphQL": {gql["agentSessionCapabilities"].(map[string]any), gqlEnv["agentEnvironment"].(map[string]any)},
		"MCP":     {mcpCaps, mcpEnv},
	} {
		caps, env := got[0], got[1]
		if offered := claude(caps["agents"]); !slices.Equal(offered, enabled) {
			t.Errorf("%s, %s: capabilities claude models %v, want %v", label, surface, offered, enabled)
		}
		if g := strs(t, env["models"]); !slices.Equal(g, models) {
			t.Errorf("%s, %s: models %v, want %v", label, surface, g, models)
		}
		if g := strs(t, env["enabledModels"]); !slices.Equal(g, enabled) {
			t.Errorf("%s, %s: enabledModels %v, want %v", label, surface, g, enabled)
		}
		if g := strs(t, env["newModels"]); !slices.Equal(g, added) {
			t.Errorf("%s, %s: newModels %v, want %v", label, surface, g, added)
		}
	}
}

// refusedAsTurnedOff reports whether err refuses catalogAdded in field as a
// model turned off in this workspace, listing offered.
func refusedAsTurnedOff(err error, code, field string, offered []string) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == code && coded.Params["field"] == field &&
		coded.Params["model"] == catalogAdded && slices.Equal(coded.Params["models"].([]string), offered) &&
		strings.Contains(coded.Error(), catalogAdded+" is turned off in this workspace") &&
		strings.Contains(coded.Error(), "a workspace admin can turn it on")
}

// DoD 2, 3, 6: a model added to the catalog after a narrowing write is off
// everywhere a turned-off model is, and the environment reads name it as
// new, apart from the one an admin turned off. Turning it on offers it.
// Turning every model on keeps the allowlist (w10/m592), so the next added
// model is off, and allowAllModels true allows all again, so it is on.
func TestAModelAddedAfterANarrowingWriteStaysOff(t *testing.T) {
	enablePlatformCatalog(t)
	svc, st, _ := adminSession(t, nil)
	reads := newModelSurfaces(t, svc)
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &withoutOpus}); err != nil {
		t.Fatal(err)
	}
	reads.expect("narrowed", testCatalog, withoutOpus, []string{})
	growCatalog(t, catalogAdded)
	reads.expect("after the catalog grew", grownCatalog, withoutOpus, []string{catalogAdded})

	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogAdded, Task: "fix the tests"}
	if _, err := svc.Create(caller("alice"), input); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "agentConfig.model", withoutOpus) {
		t.Fatalf("create on the added model = %v", err)
	}
	input.AgentConfig.Model = catalogSonnet
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	settleTurn(st, created.ID)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "harder", Model: catalogAdded}); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "model", withoutOpus) {
		t.Fatalf("a message on the added model = %v", err)
	}
	autos, _ := automationTestService("tea-a", newFakeAutomations())
	autos.Environments = svc.Environments
	in := automationInput("nightly")
	in.Template = &AutomationTemplate{Task: "triage", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "claude", Model: catalogAdded}, MaxSpendUSD: ptr(2.0)}
	if _, err := autos.CreateAgentAutomation(caller("dev"), "tea-a", in); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "agentConfig.model", withoutOpus) {
		t.Fatalf("automation create on the added model = %v", err)
	}
	in.Template.AgentConfig.Model = catalogSonnet
	saved, err := autos.CreateAgentAutomation(caller("dev"), "tea-a", in)
	if err != nil {
		t.Fatal(err)
	}
	update := *in.Template
	update.AgentConfig.Model = catalogAdded
	if _, err := autos.UpdateAgentAutomation(caller("dev"), "tea-a", saved.ID, AgentAutomationInput{Template: &update}); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "agentConfig.model", withoutOpus) {
		t.Fatalf("automation update to the added model = %v", err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogAdded)}); !refusedAsTurnedOff(err, CodeEnvironmentModelNotOffered, "defaultModel", withoutOpus) {
		t.Fatalf("defaultModel naming the added model = %v", err)
	}
	req := CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Model: catalogAdded}}
	if note := svc.retiredTemplateModel(caller("alice"), &req); note != "the template's model "+catalogAdded+" is turned off in this workspace; this run used "+catalogDefault {
		t.Fatalf("an automation run on the added model notes %q", note)
	}

	// An admin turns it on as any other model; opus stays off.
	on := []string{catalogDefault, catalogSonnet, catalogAdded}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &on}); err != nil {
		t.Fatal(err)
	}
	reads.expect("the added model turned on", grownCatalog, on, []string{})
	input.AgentConfig.Model = catalogAdded
	if _, err := svc.Create(caller("alice"), input); err != nil {
		t.Fatalf("create on the model once turned on: %v", err)
	}

	// Every model back on keeps the allowlist (w10/m592): the next added
	// model is new and off.
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &grownCatalog}); err != nil {
		t.Fatal(err)
	}
	if row := svc.Environments.(*fakeEnvironments).rows["tea-a"]; !slices.Equal(row.KnownModels, grownCatalog) || len(row.DisabledModels) != 0 {
		t.Fatalf("every model on stored known %v disabled %v, want an allowlist of every model", row.KnownModels, row.DisabledModels)
	}
	const fable = "anthropic/claude-fable-5.5"
	growCatalog(t, catalogAdded, fable)
	everything := append(slices.Clone(grownCatalog), fable)
	reads.expect("every model listed, the catalog grown again", everything, grownCatalog, []string{fable})

	// allowAllModels true allows all: the model added last is on.
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{AllowAllModels: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	reads.expect("allow all, the catalog grown again", everything, everything, []string{})
}

// DoD 1: a workspace that never narrowed its list offers a model added to the
// catalog at once: one with no row, one whose row turns no model off, one
// whose admins listed every model while it allowed all, and one whose admins
// chose allowAllModels after narrowing it (w10/m592).
func TestAModelAddedToAWorkspaceThatNeverNarrowedIsOffered(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(t *testing.T, svc *Service)
	}{
		{"no row", func(*testing.T, *Service) {}},
		{"a row turning no model off", func(t *testing.T, svc *Service) {
			if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{NetworkTier: ptr(NetworkTierFull)}); err != nil {
				t.Fatal(err)
			}
		}},
		{"every model listed while it allows all", func(t *testing.T, svc *Service) {
			if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &testCatalog}); err != nil {
				t.Fatal(err)
			}
		}},
		{"allowAllModels after narrowing", func(t *testing.T, svc *Service) {
			for _, input := range []EnvironmentInput{{EnabledModels: &withoutOpus}, {AllowAllModels: ptr(true)}} {
				if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", input); err != nil {
					t.Fatal(err)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enablePlatformCatalog(t)
			svc, _, _ := adminSession(t, nil)
			tc.write(t, svc)
			growCatalog(t, catalogAdded)
			newModelSurfaces(t, svc).expect(tc.name, grownCatalog, grownCatalog, []string{})
			input := createInput()
			input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogAdded, Task: "fix the tests"}
			if created, err := svc.Create(caller("alice"), input); err != nil || created.AgentConfig.Model != catalogAdded {
				t.Fatalf("create on the added model = %+v err=%v", created.AgentConfig, err)
			}
		})
	}
}

// DoD 4 (m568's rollout read rule): a bex-api before m568 writes only
// disabled_models, the catalog less its list, and leaves known_models as it
// was. Under the catalog both run, the new read of every row it can leave is
// the opt-out read it makes itself, with no model new. Against a grown
// catalog a row narrowed under 0315 and not yet converted still reads as the
// opt-out, while an allowlist leaves the added model off, even one that turns
// no model off: w10/m592 retired the clause that read it as "allow all",
// once no Pod before m568 ran.
func TestTheModelListRolloutReadRule(t *testing.T) {
	enablePlatformCatalog(t)
	optOut := func(disabled []string) []string {
		out := []string{}
		for i, model := range testCatalog {
			if i == 0 || !slices.Contains(disabled, model) {
				out = append(out, model)
			}
		}
		return out
	}
	for name, row := range map[string]store.AgentEnvironment{
		"narrowed under 0315":                   {DisabledModels: []string{catalogOpus}},
		"narrowing a row that allows all":       {DisabledModels: []string{catalogSonnet, catalogOpus}},
		"every model on over an allowlist":      {DisabledModels: []string{}, KnownModels: testCatalog},
		"swapping which model an allowlist has": {DisabledModels: []string{catalogSonnet}, KnownModels: testCatalog},
	} {
		on, added := leftOn(testCatalog, row)
		if want := optOut(row.DisabledModels); !slices.Equal(on, want) || len(added) != 0 {
			t.Errorf("%s: leftOn = %v new %v, want the opt-out read %v", name, on, added, want)
		}
	}

	for _, tc := range []struct {
		name      string
		row       store.AgentEnvironment
		on, added []string
	}{
		{"a 0315 row, not yet converted", store.AgentEnvironment{DisabledModels: []string{catalogOpus}}, []string{catalogDefault, catalogSonnet, catalogAdded}, []string{}},
		{"an allowlist listing every model", store.AgentEnvironment{DisabledModels: []string{}, KnownModels: testCatalog}, testCatalog, []string{catalogAdded}},
		{"allow all", store.AgentEnvironment{DisabledModels: []string{}}, grownCatalog, []string{}},
		{"an allowlist", store.AgentEnvironment{DisabledModels: []string{catalogOpus}, KnownModels: testCatalog}, withoutOpus, []string{catalogAdded}},
		{"an allowlist naming the old default off", store.AgentEnvironment{DisabledModels: []string{catalogDefault, catalogOpus}, KnownModels: testCatalog}, withoutOpus, []string{catalogAdded}},
	} {
		on, added := leftOn(grownCatalog, tc.row)
		if !slices.Equal(on, tc.on) || !slices.Equal(added, tc.added) {
			t.Errorf("%s, grown catalog: leftOn = %v new %v, want %v new %v", tc.name, on, added, tc.on, tc.added)
		}
	}

	// opus leaves the catalog: it drops out of the reads, and the next
	// narrowing write stores the catalog it saw.
	enablePlatformCatalog(t)
	svc, _, _ := adminSession(t, nil)
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &[]string{catalogDefault, catalogOpus}}); err != nil {
		t.Fatal(err)
	}
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", catalogSonnet, platformKey); err != nil {
		t.Fatal(err)
	}
	env, err := svc.Environment(caller("admin"), "tea-a")
	if err != nil || !slices.Equal(env.EnabledModels, []string{catalogDefault}) || len(env.NewModels) != 0 {
		t.Fatalf("after opus left the catalog = enabled %v new %v err=%v", env.EnabledModels, env.NewModels, err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &[]string{catalogDefault}}); err != nil {
		t.Fatal(err)
	}
	if row := svc.Environments.(*fakeEnvironments).rows["tea-a"]; !slices.Equal(row.KnownModels, []string{catalogDefault, catalogSonnet}) || !slices.Equal(row.DisabledModels, []string{catalogSonnet}) {
		t.Fatalf("the next write stored known %v disabled %v", row.KnownModels, row.DisabledModels)
	}
}
