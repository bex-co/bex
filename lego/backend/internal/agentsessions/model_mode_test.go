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

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w10/m592: "Allow all models" is a mode of its own, as on Devin's settings
// page, so an allowlist that lists every current catalog model stays an
// allowlist and leaves the next model off.

// expectMode checks allowAllModels on REST, GraphQL and MCP.
func (m modelSurfaces) expectMode(label string, want bool) {
	t := m.a.t
	t.Helper()
	code, rest := m.a.rest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil, "admin")
	gql, gqlErr := m.a.gql(`{ agentEnvironment(ownerId: "tea-a") { allowAllModels } }`, "admin")
	mcpEnv, mcpErr := m.a.mcp("get_agent_environment", map[string]any{"ownerId": "tea-a"}, "admin")
	if code != http.StatusOK || gqlErr != "" || mcpErr != "" {
		t.Fatalf("%s: reads = %d %s %s", label, code, gqlErr, mcpErr)
	}
	for surface, env := range map[string]map[string]any{"REST": rest, "GraphQL": gql["agentEnvironment"].(map[string]any), "MCP": mcpEnv} {
		if got, ok := env["allowAllModels"].(bool); !ok || got != want {
			t.Errorf("%s, %s: allowAllModels %v, want %t", label, surface, env["allowAllModels"], want)
		}
	}
}

// writeMode writes input on one surface as the admin of tea-a.
func (m modelSurfaces) writeMode(surface string, input map[string]any) {
	t := m.a.t
	t.Helper()
	switch surface {
	case "REST":
		if code, body := m.a.rest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", input, "admin"); code != http.StatusOK {
			t.Fatalf("REST write %v = %d %v", input, code, body)
		}
	case "GraphQL":
		result := graphql.Do(graphql.Params{Schema: environmentSchema(t, m.a.svc), Context: caller("admin"),
			RequestString:  `mutation($input: AgentEnvironmentInput!) { updateAgentEnvironment(ownerId: "tea-a", input: $input) { allowAllModels } }`,
			VariableValues: map[string]any{"input": input}})
		if len(result.Errors) > 0 {
			t.Fatalf("GraphQL write %v: %v", input, result.Errors)
		}
	case "MCP":
		if _, errText := m.a.mcp("update_agent_environment", withOwner(input), "admin"); errText != "" {
			t.Fatalf("MCP write %v: %s", input, errText)
		}
	default:
		t.Fatalf("no surface %q", surface)
	}
}

var modeSurfaces = []string{"REST", "GraphQL", "MCP"}

// DoD 2, 6: with "Allow all models" turned off on any surface, the workspace
// keeps an allowlist of every current catalog model. A model added to the
// catalog afterwards is off everywhere a turned-off model is and reads as
// new, and so it is when the allowlist is written explicitly, every model
// named.
func TestAnAllowlistListingEveryModelLeavesTheNextOneOff(t *testing.T) {
	for _, tc := range []struct {
		name, surface string
		input         map[string]any
	}{
		{"allowAllModels false on REST", "REST", map[string]any{"allowAllModels": false}},
		{"allowAllModels false on GraphQL", "GraphQL", map[string]any{"allowAllModels": false}},
		{"allowAllModels false on MCP", "MCP", map[string]any{"allowAllModels": false}},
		{"every model named with allowAllModels false", "REST", map[string]any{"allowAllModels": false, "enabledModels": testCatalog}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enablePlatformCatalog(t)
			svc, st, _ := adminSession(t, nil)
			reads := newModelSurfaces(t, svc)
			reads.expectMode("no row", true)
			reads.writeMode(tc.surface, tc.input)
			reads.expectMode("an allowlist of every model", false)
			reads.expect("an allowlist of every model", testCatalog, testCatalog, []string{})
			if row := svc.Environments.(*fakeEnvironments).rows["tea-a"]; !slices.Equal(row.KnownModels, testCatalog) || len(row.DisabledModels) != 0 || row.EnabledProviders != nil {
				t.Fatalf("stored known %v disabled %v providers %v, want the catalog as the allowlist", row.KnownModels, row.DisabledModels, row.EnabledProviders)
			}

			growCatalog(t, catalogAdded)
			reads.expectMode("after the catalog grew", false)
			reads.expect("after the catalog grew", grownCatalog, testCatalog, []string{catalogAdded})

			input := createInput()
			input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogAdded, Task: "fix the tests"}
			if _, err := svc.Create(caller("alice"), input); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "agentConfig.model", testCatalog) {
				t.Fatalf("create on the added model = %v", err)
			}
			if tc.surface != "REST" {
				return
			}
			// The other refusals, once.
			input.AgentConfig.Model = catalogOpus
			created, err := svc.Create(caller("alice"), input)
			if err != nil {
				t.Fatal(err)
			}
			settleTurn(st, created.ID)
			if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "harder", Model: catalogAdded}); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "model", testCatalog) {
				t.Fatalf("a message on the added model = %v", err)
			}
			autos, _ := automationTestService("tea-a", newFakeAutomations())
			autos.Environments = svc.Environments
			in := automationInput("nightly")
			in.Template = &AutomationTemplate{Task: "triage", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "claude", Model: catalogAdded}, MaxSpendUSD: ptr(2.0)}
			if _, err := autos.CreateAgentAutomation(caller("dev"), "tea-a", in); !refusedAsTurnedOff(err, CodeSessionModelNotOffered, "agentConfig.model", testCatalog) {
				t.Fatalf("automation create on the added model = %v", err)
			}
			if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogAdded)}); !refusedAsTurnedOff(err, CodeEnvironmentModelNotOffered, "defaultModel", testCatalog) {
				t.Fatalf("defaultModel naming the added model = %v", err)
			}
		})
	}
}

// DoD 3, 6: allowAllModels true on any surface allows all, from an allowlist
// and from a filter by provider: every catalog model is offered, none is new,
// and a model added to the catalog later is offered too.
func TestAllowAllModelsOffersALaterModel(t *testing.T) {
	for _, from := range []struct {
		name  string
		input EnvironmentInput
	}{
		{"an allowlist", EnvironmentInput{EnabledModels: &withoutOpus}},
		{"an allowlist of every model", EnvironmentInput{AllowAllModels: ptr(false)}},
		{"a filter by provider", EnvironmentInput{EnabledProviders: &[]string{"deepseek"}}},
	} {
		for _, surface := range modeSurfaces {
			t.Run(from.name+" on "+surface, func(t *testing.T) {
				enablePlatformCatalog(t)
				svc, _, _ := adminSession(t, nil)
				reads := newModelSurfaces(t, svc)
				if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", from.input); err != nil {
					t.Fatal(err)
				}
				reads.expectMode(from.name, false)
				reads.writeMode(surface, map[string]any{"allowAllModels": true})
				reads.expectMode("allow all", true)
				reads.expect("allow all", testCatalog, testCatalog, []string{})
				reads.expectProviders("allow all", testProviders, nil)

				growCatalog(t, catalogAdded)
				reads.expect("allow all, the catalog grown", grownCatalog, grownCatalog, []string{})
				input := createInput()
				input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogAdded, Task: "fix the tests"}
				if created, err := svc.Create(caller("alice"), input); err != nil || created.AgentConfig.Model != catalogAdded {
					t.Fatalf("create on the added model = %+v err=%v", created.AgentConfig, err)
				}
			})
		}
	}
}

// The rows a workspace can hold before a model write.
var (
	modeAllowAll    = store.AgentEnvironment{DisabledModels: []string{}}
	modeAllowlist   = store.AgentEnvironment{DisabledModels: []string{catalogOpus}, KnownModels: testCatalog}
	modeEveryListed = store.AgentEnvironment{DisabledModels: []string{}, KnownModels: testCatalog}
	modeFilter      = store.AgentEnvironment{DisabledModels: []string{catalogSonnet, catalogOpus}, KnownModels: testCatalog, EnabledProviders: []string{"deepseek"}}
	modeLegacy      = store.AgentEnvironment{DisabledModels: []string{catalogOpus}}
)

// modeWrites are the model writes t001 records, from each starting row, with
// the row they leave (nil known: allow all) and the offer it reads.
var modeWrites = []struct {
	name     string
	from     store.AgentEnvironment
	input    EnvironmentInput
	known    []string
	disabled []string
	filter   []string
	allowAll bool
	offered  []string
}{
	{"true from an allowlist", modeAllowlist, EnvironmentInput{AllowAllModels: ptr(true)}, nil, []string{}, nil, true, testCatalog},
	{"true from an allowlist of every model", modeEveryListed, EnvironmentInput{AllowAllModels: ptr(true)}, nil, []string{}, nil, true, testCatalog},
	{"true from a filter", modeFilter, EnvironmentInput{AllowAllModels: ptr(true)}, nil, []string{}, nil, true, testCatalog},
	{"true from a 0315 row", modeLegacy, EnvironmentInput{AllowAllModels: ptr(true)}, nil, []string{}, nil, true, testCatalog},
	{"true while it allows all", modeAllowAll, EnvironmentInput{AllowAllModels: ptr(true)}, nil, []string{}, nil, true, testCatalog},
	{"false while it allows all", modeAllowAll, EnvironmentInput{AllowAllModels: ptr(false)}, testCatalog, []string{}, nil, false, testCatalog},
	{"false on an allowlist", modeAllowlist, EnvironmentInput{AllowAllModels: ptr(false)}, testCatalog, []string{catalogOpus}, nil, false, withoutOpus},
	{"false on a filter", modeFilter, EnvironmentInput{AllowAllModels: ptr(false)}, testCatalog, []string{catalogSonnet, catalogOpus}, []string{"deepseek"}, false, []string{catalogDefault}},
	{"false with every model", modeAllowAll, EnvironmentInput{AllowAllModels: ptr(false), EnabledModels: &testCatalog}, testCatalog, []string{}, nil, false, testCatalog},
	{"false with a filter", modeAllowAll, EnvironmentInput{AllowAllModels: ptr(false), EnabledProviders: &[]string{"anthropic"}}, testCatalog, []string{}, []string{"anthropic"}, false, testCatalog},
	{"every model while it allows all", modeAllowAll, EnvironmentInput{EnabledModels: &testCatalog}, nil, []string{}, nil, true, testCatalog},
	{"every model on an allowlist", modeAllowlist, EnvironmentInput{EnabledModels: &testCatalog}, testCatalog, []string{}, nil, false, testCatalog},
	{"every model on a filter", modeFilter, EnvironmentInput{EnabledModels: &testCatalog}, testCatalog, []string{}, nil, false, testCatalog},
	{"every model on a 0315 row", modeLegacy, EnvironmentInput{EnabledModels: &testCatalog}, testCatalog, []string{}, nil, false, testCatalog},
	{"a model off while it allows all", modeAllowAll, EnvironmentInput{EnabledModels: &withoutOpus}, testCatalog, []string{catalogOpus}, nil, false, withoutOpus},
	{"a filter while it allows all", modeAllowAll, EnvironmentInput{EnabledProviders: &[]string{"anthropic", "deepseek"}}, testCatalog, []string{}, []string{"deepseek", "anthropic"}, false, testCatalog},
}

// DoD 3, 6: every write stores the row t001 records and reads the mode and
// the offer it says.
func TestTheModeWrites(t *testing.T) {
	for _, tc := range modeWrites {
		t.Run(tc.name, func(t *testing.T) {
			enablePlatformCatalog(t)
			from := tc.from
			from.WorkspaceID, from.NetworkTier = "tea-a", NetworkTierRegistries
			svc, _, _ := adminSession(t, &from)
			env, err := svc.UpdateEnvironment(caller("admin"), "tea-a", tc.input)
			if err != nil {
				t.Fatal(err)
			}
			row := svc.Environments.(*fakeEnvironments).rows["tea-a"]
			if !slices.Equal(row.KnownModels, tc.known) || (row.KnownModels == nil) != (tc.known == nil) ||
				!slices.Equal(row.DisabledModels, tc.disabled) || !slices.Equal(row.EnabledProviders, tc.filter) {
				t.Errorf("stored known %v disabled %v providers %v, want known %v disabled %v providers %v",
					row.KnownModels, row.DisabledModels, row.EnabledProviders, tc.known, tc.disabled, tc.filter)
			}
			if env.AllowAllModels != tc.allowAll || !slices.Equal(env.EnabledModels, tc.offered) || len(env.NewModels) != 0 {
				t.Errorf("read allowAllModels %t enabled %v new %v, want %t %v []", env.AllowAllModels, env.EnabledModels, env.NewModels, tc.allowAll, tc.offered)
			}
		})
	}
}

// DoD 3, 6: "Allow all models" lists none, so true beside either list is a
// typed 400 on every surface, and so is either value off the platform route
// and on the workspace's own key, where there is no catalog. Nothing is
// written.
func TestAllowAllModelsRefusalsOnEverySurface(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     map[string]any
		byo, off  bool // the workspace's own key; no platform route
		fields    []string
		inMessage string
	}{
		{"true with enabledModels", map[string]any{"allowAllModels": true, "enabledModels": []any{catalogDefault}}, false, false,
			[]string{"allowAllModels", "enabledModels"}, "set allowAllModels: true or enabledModels, not both"},
		{"true with enabledProviders", map[string]any{"allowAllModels": true, "enabledProviders": []any{"anthropic"}}, false, false,
			[]string{"allowAllModels", "enabledProviders"}, "set allowAllModels: true or enabledProviders, not both"},
		{"true on the workspace's own key", map[string]any{"allowAllModels": true}, true, false, nil, "its own model key"},
		{"false on the workspace's own key", map[string]any{"allowAllModels": false}, true, false, nil, "its own model key"},
		{"true with no platform route", map[string]any{"allowAllModels": true}, false, true, nil, "which this bex-api does not offer"},
		{"false with no platform route", map[string]any{"allowAllModels": false}, false, true, nil, "which this bex-api does not offer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enablePlatformCatalog(t)
			if tc.off {
				if err := agentsession.EnablePlatformModelRoute("", "", "", ""); err != nil {
					t.Fatal(err)
				}
			}
			svc, _, _ := adminSession(t, nil)
			envs := svc.Environments.(*fakeEnvironments)
			envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, DisabledModels: []string{catalogOpus}, KnownModels: testCatalog}
			if tc.byo {
				svc.ModelKeys = &fakeModelKeys{}
				if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr("sk-ant-own")}); err != nil {
					t.Fatal(err)
				}
				if env, err := svc.Environment(caller("admin"), "tea-a"); err != nil || !env.AllowAllModels {
					t.Fatalf("the own key's read = allowAllModels %t err=%v, want true", env.AllowAllModels, err)
				}
			}
			before := envs.rows["tea-a"]

			input := EnvironmentInput{AllowAllModels: ptr(tc.input["allowAllModels"].(bool))}
			if raw, ok := tc.input["enabledProviders"].([]any); ok {
				input.EnabledProviders = ptr(stringsOf(raw))
			}
			if raw, ok := tc.input["enabledModels"].([]any); ok {
				input.EnabledModels = ptr(stringsOf(raw))
			}
			_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", input)
			var coded *core.CodedError
			if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != ModelListInvalidCode ||
				coded.Params["field"] != "allowAllModels" || !strings.Contains(coded.Error(), tc.inMessage) {
				t.Fatalf("service = %#v, want %s on allowAllModels saying %q", err, ModelListInvalidCode, tc.inMessage)
			}
			if fields, _ := coded.Params["fields"].([]string); !slices.Equal(fields, tc.fields) {
				t.Fatalf("fields = %v, want %v", coded.Params["fields"], tc.fields)
			}

			a := newModelSurfaces(t, svc).a
			if code, body := a.rest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", tc.input, "admin"); code != http.StatusBadRequest || body["code"] != ModelListInvalidCode {
				t.Fatalf("REST = %d %v", code, body)
			}
			if _, errText := a.mcp("update_agent_environment", withOwner(tc.input), "admin"); !strings.Contains(errText, ModelListInvalidCode) {
				t.Fatalf("MCP = %q", errText)
			}
			result := graphql.Do(graphql.Params{Schema: environmentSchema(t, svc), Context: caller("admin"),
				RequestString:  `mutation($input: AgentEnvironmentInput!) { updateAgentEnvironment(ownerId: "tea-a", input: $input) { allowAllModels } }`,
				VariableValues: map[string]any{"input": tc.input}})
			if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != ModelListInvalidCode {
				t.Fatalf("GraphQL = %#v", result.Errors)
			}
			if got := envs.rows["tea-a"]; !slices.Equal(got.DisabledModels, before.DisabledModels) || !slices.Equal(got.KnownModels, before.KnownModels) || got.EnabledProviders != nil {
				t.Fatalf("a refused write was written: %+v", got)
			}
		})
	}

	// REST decodes strictly: the mode is a boolean.
	enablePlatformCatalog(t)
	svc, _, _ := adminSession(t, nil)
	if code, body := newModelSurfaces(t, svc).a.rest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", `{"allowAllModels":"yes"}`, "admin"); code != http.StatusBadRequest {
		t.Fatalf("a string mode = %d %v, want 400", code, body)
	}
}

// m573LeftOn is the previous release's read (w10/m573): a row turning no
// model off allows all, whatever known_models says, unless a filter by
// provider is in force.
func m573LeftOn(catalog []string, env store.AgentEnvironment) []string {
	providers, byProvider := providerFilter(env)
	allowAll := env.KnownModels == nil || len(env.DisabledModels) == 0
	on := []string{}
	for i, model := range catalog {
		switch {
		case i == 0:
			on = append(on, model)
		case byProvider:
			if slices.Contains(providers, modelProvider(model)) {
				on = append(on, model)
			}
		case slices.Contains(env.DisabledModels, model):
		case allowAll || slices.Contains(env.KnownModels, model):
			on = append(on, model)
		}
	}
	return on
}

// DoD 4, 6 (the rollout read rule): during the deploy a previous-release Pod
// runs m573's read. Every row that release or an earlier one can leave reads
// the same offer under any catalog, so no workspace's offer changes. Every
// row the new code writes reads the same offer under the catalog both run.
// Only an allowlist listing every model reads differently once the catalog
// grows, which is why the deploy that first ships m592 adds no catalog model.
func TestTheModeRolloutReadRule(t *testing.T) {
	enablePlatformCatalog(t)
	grown := append(slices.Clone(grownCatalog), otherProvider)
	for name, row := range map[string]store.AgentEnvironment{
		"no row":                  {},
		"allow all":               modeAllowAll,
		"an m568 allowlist":       modeAllowlist,
		"a filter by provider":    modeFilter,
		"every provider":          {DisabledModels: []string{}, KnownModels: testCatalog, EnabledProviders: testProviders},
		"a 0315 row":              modeLegacy,
		"m568 narrowing a filter": {DisabledModels: []string{catalogOpus}, KnownModels: testCatalog, EnabledProviders: []string{"deepseek"}},
	} {
		for _, catalog := range [][]string{testCatalog, grown} {
			if on, _ := leftOn(catalog, row); !slices.Equal(on, m573LeftOn(catalog, row)) {
				t.Errorf("%s, catalog %v: the new read %v, m573's %v", name, catalog, on, m573LeftOn(catalog, row))
			}
		}
		if want := row.KnownModels == nil && len(row.DisabledModels) == 0; allowsAll(row) != want {
			t.Errorf("%s: allowAllModels %t, want %t", name, allowsAll(row), want)
		}
	}

	// A row each new write leaves, read by m573's Pod under the same catalog.
	for _, tc := range modeWrites {
		from := tc.from
		from.WorkspaceID, from.NetworkTier = "tea-a", NetworkTierRegistries
		svc, _, _ := adminSession(t, &from)
		if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", tc.input); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		row := svc.Environments.(*fakeEnvironments).rows["tea-a"]
		if on, added := leftOn(testCatalog, row); !slices.Equal(on, m573LeftOn(testCatalog, row)) || len(added) != 0 {
			t.Errorf("%s: the new read %v new %v, m573's %v", tc.name, on, added, m573LeftOn(testCatalog, row))
		}
	}

	// m573's writes over a new allowlist of every model: its every-model
	// write allows all, as it reads it; its narrowing write is an allowlist;
	// a write of another field leaves the allowlist, the same offer.
	for name, row := range map[string]store.AgentEnvironment{
		"m573 every model": {DisabledModels: []string{}},
		"m573 narrowing":   modeAllowlist,
		"another field":    modeEveryListed,
	} {
		if on, added := leftOn(testCatalog, row); !slices.Equal(on, m573LeftOn(testCatalog, row)) || len(added) != 0 {
			t.Errorf("%s: the new read %v new %v, m573's %v", name, on, added, m573LeftOn(testCatalog, row))
		}
	}

	// Grown, an allowlist of every model is the one row the two reads tell
	// apart: the new one leaves the added model off.
	if on, added := leftOn(grownCatalog, modeEveryListed); !slices.Equal(on, testCatalog) || !slices.Equal(added, []string{catalogAdded}) {
		t.Errorf("an allowlist of every model, grown: %v new %v", on, added)
	}
	if prev := m573LeftOn(grownCatalog, modeEveryListed); !slices.Equal(prev, grownCatalog) {
		t.Errorf("m573 reads an allowlist of every model, grown, as %v", prev)
	}
}
