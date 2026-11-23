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

// w10/m573: a workspace admin may allow the catalog's models by provider
// instead of one by one, Devin's other filter type ("Filter by specific
// models or by provider (one filter type at a time)"): every catalog model of
// a listed provider is offered, one added later included, plus the catalog
// default, and a model of any other provider is off.

// otherProvider is a catalog model of a provider testCatalog does not carry.
const otherProvider = "moonshotai/kimi-k3"

// testProviders are testCatalog's providers, in catalog order.
var testProviders = []string{"deepseek", "anthropic"}

// expectProviders checks the environment's providers and enabledProviders on
// REST, GraphQL and MCP; a nil enabled is null, a list filtered by model.
func (m modelSurfaces) expectProviders(label string, providers, enabled []string) {
	t := m.a.t
	t.Helper()
	code, rest := m.a.rest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil, "admin")
	gql, gqlErr := m.a.gql(`{ agentEnvironment(ownerId: "tea-a") { providers enabledProviders } }`, "admin")
	mcpEnv, mcpErr := m.a.mcp("get_agent_environment", map[string]any{"ownerId": "tea-a"}, "admin")
	if code != http.StatusOK || gqlErr != "" || mcpErr != "" {
		t.Fatalf("%s: reads = %d %s %s", label, code, gqlErr, mcpErr)
	}
	for surface, env := range map[string]map[string]any{"REST": rest, "GraphQL": gql["agentEnvironment"].(map[string]any), "MCP": mcpEnv} {
		if g := strs(t, env["providers"]); !slices.Equal(g, providers) {
			t.Errorf("%s, %s: providers %v, want %v", label, surface, g, providers)
		}
		raw, present := env["enabledProviders"]
		switch {
		case !present:
			t.Errorf("%s, %s: no enabledProviders field", label, surface)
		case enabled == nil && raw != nil:
			t.Errorf("%s, %s: enabledProviders %v, want null", label, surface, raw)
		case enabled != nil && (raw == nil || !slices.Equal(strs(t, raw), enabled)):
			t.Errorf("%s, %s: enabledProviders %v, want %v", label, surface, raw, enabled)
		}
	}
}

// refusedOff reports whether err refuses model in field as a model turned
// off in this workspace, listing offered.
func refusedOff(err error, code, field, model string, offered []string) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == code && coded.Params["field"] == field &&
		coded.Params["model"] == model && slices.Equal(coded.Params["models"].([]string), offered) &&
		strings.Contains(coded.Error(), model+" is turned off in this workspace")
}

// DoD 1, 3, 6: a filter by provider written on each surface reads back on
// every one. A model added to the catalog later is offered when its provider
// is listed and refused everywhere a turned-off model is when it is not.
func TestAProviderFilterOffersEveryModelOfAListedProvider(t *testing.T) {
	enablePlatformCatalog(t)
	svc, st, _ := adminSession(t, nil)
	reads := newModelSurfaces(t, svc)
	reads.expectProviders("no row", testProviders, nil)

	if code, body := reads.a.rest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", map[string]any{"enabledProviders": []string{" anthropic", "anthropic"}}, "admin"); code != http.StatusOK {
		t.Fatalf("REST write = %d %v", code, body)
	}
	reads.expect("anthropic allowed", testCatalog, testCatalog, []string{})
	reads.expectProviders("anthropic allowed", testProviders, []string{"anthropic"})

	growCatalog(t, catalogAdded, otherProvider)
	grown := append(slices.Clone(grownCatalog), otherProvider)
	offered := slices.Clone(grownCatalog)
	reads.expect("after the catalog grew", grown, offered, []string{})
	reads.expectProviders("after the catalog grew", []string{"deepseek", "anthropic", "moonshotai"}, []string{"anthropic"})

	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogAdded, Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil || created.AgentConfig.Model != catalogAdded {
		t.Fatalf("create on the added model of a listed provider = %+v err=%v", created.AgentConfig, err)
	}
	input.AgentConfig.Model = otherProvider
	if _, err := svc.Create(caller("alice"), input); !refusedOff(err, CodeSessionModelNotOffered, "agentConfig.model", otherProvider, offered) {
		t.Fatalf("create on a model of an unlisted provider = %v", err)
	}
	settleTurn(st, created.ID)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "harder", Model: otherProvider}); !refusedOff(err, CodeSessionModelNotOffered, "model", otherProvider, offered) {
		t.Fatalf("a message on a model of an unlisted provider = %v", err)
	}
	autos, _ := automationTestService("tea-a", newFakeAutomations())
	autos.Environments = svc.Environments
	in := automationInput("nightly")
	in.Template = &AutomationTemplate{Task: "triage", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "claude", Model: otherProvider}, MaxSpendUSD: ptr(2.0)}
	if _, err := autos.CreateAgentAutomation(caller("dev"), "tea-a", in); !refusedOff(err, CodeSessionModelNotOffered, "agentConfig.model", otherProvider, offered) {
		t.Fatalf("automation create on a model of an unlisted provider = %v", err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(otherProvider)}); !refusedOff(err, CodeEnvironmentModelNotOffered, "defaultModel", otherProvider, offered) {
		t.Fatalf("defaultModel naming a model of an unlisted provider = %v", err)
	}

	// GraphQL allows the default's provider alone: only deepseek models, the
	// default among them.
	if _, code := reads.a.gql(`mutation { updateAgentEnvironment(ownerId: "tea-a", input: {enabledProviders: ["deepseek"]}) { enabledProviders } }`, "admin"); code != "" {
		t.Fatalf("GraphQL write: %s", code)
	}
	reads.expect("deepseek allowed", grown, []string{catalogDefault}, []string{})
	reads.expectProviders("deepseek allowed", []string{"deepseek", "anthropic", "moonshotai"}, []string{"deepseek"})

	// MCP lists two, out of order: they read in catalog order.
	if _, errText := reads.a.mcp("update_agent_environment", map[string]any{"ownerId": "tea-a", "enabledProviders": []string{"moonshotai", "deepseek"}}, "admin"); errText != "" {
		t.Fatalf("MCP write: %s", errText)
	}
	reads.expect("moonshotai and deepseek allowed", grown, []string{catalogDefault, otherProvider}, []string{})
	reads.expectProviders("moonshotai and deepseek allowed", []string{"deepseek", "anthropic", "moonshotai"}, []string{"deepseek", "moonshotai"})

	// moonshotai leaves the catalog: the read lists only the providers the
	// catalog still carries, so a write built from it is accepted.
	enablePlatformCatalog(t)
	reads.expect("after moonshotai left the catalog", testCatalog, []string{catalogDefault}, []string{})
	reads.expectProviders("after moonshotai left the catalog", testProviders, []string{"deepseek"})
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledProviders: &[]string{"deepseek", "anthropic"}}); err != nil {
		t.Fatalf("a write built from the read: %v", err)
	}
}

// DoD 2, 6: an unknown provider, an empty list, both filter types at once and
// a write where there is no catalog are each a typed 400 on every surface,
// and nothing is written.
func TestEnabledProvidersRefusalsOnEverySurface(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     map[string]any
		byo, off  bool // the workspace's own key; no platform route
		check     func(params map[string]any) bool
		inMessage string
	}{
		{"an unknown provider", map[string]any{"enabledProviders": []any{"anthropic", "openai"}}, false, false, func(p map[string]any) bool {
			return p["provider"] == "openai" && slices.Equal(p["providers"].([]string), testProviders)
		}, `"openai" is not a provider of the platform catalog, whose providers are deepseek, anthropic`},
		{"an empty list", map[string]any{"enabledProviders": []any{}}, false, false, func(p map[string]any) bool {
			return slices.Equal(p["providers"].([]string), testProviders)
		}, "at least one of the platform catalog's providers"},
		{"both filter types", map[string]any{"enabledProviders": []any{"anthropic"}, "enabledModels": []any{catalogDefault}}, false, false, func(p map[string]any) bool {
			return slices.Equal(p["fields"].([]string), []string{"enabledModels", "enabledProviders"})
		}, "set enabledModels or enabledProviders, not both"},
		{"the workspace's own key", map[string]any{"enabledProviders": []any{"anthropic"}}, true, false, func(p map[string]any) bool { return len(p) == 1 }, "its own model key"},
		{"no platform route", map[string]any{"enabledProviders": []any{"anthropic"}}, false, true, func(p map[string]any) bool { return len(p) == 1 }, "which this bex-api does not offer"},
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
			}
			before := envs.rows["tea-a"]

			var input EnvironmentInput
			if raw, ok := tc.input["enabledProviders"].([]any); ok {
				input.EnabledProviders = ptr(stringsOf(raw))
			}
			if raw, ok := tc.input["enabledModels"].([]any); ok {
				input.EnabledModels = ptr(stringsOf(raw))
			}
			_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", input)
			var coded *core.CodedError
			if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != ModelListInvalidCode ||
				coded.Params["field"] != "enabledProviders" || !tc.check(coded.Params) || !strings.Contains(coded.Error(), tc.inMessage) {
				t.Fatalf("service = %#v, want %s on enabledProviders saying %q", err, ModelListInvalidCode, tc.inMessage)
			}

			a := newModelSurfaces(t, svc).a
			if code, body := a.rest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", tc.input, "admin"); code != http.StatusBadRequest || body["code"] != ModelListInvalidCode {
				t.Fatalf("REST = %d %v", code, body)
			}
			if _, errText := a.mcp("update_agent_environment", withOwner(tc.input), "admin"); !strings.Contains(errText, ModelListInvalidCode) {
				t.Fatalf("MCP = %q", errText)
			}
			result := graphql.Do(graphql.Params{Schema: environmentSchema(t, svc), Context: caller("admin"),
				RequestString:  `mutation($input: AgentEnvironmentInput!) { updateAgentEnvironment(ownerId: "tea-a", input: $input) { enabledProviders } }`,
				VariableValues: map[string]any{"input": tc.input}})
			if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != ModelListInvalidCode {
				t.Fatalf("GraphQL = %#v", result.Errors)
			}
			if got := envs.rows["tea-a"]; !slices.Equal(got.DisabledModels, before.DisabledModels) || !slices.Equal(got.KnownModels, before.KnownModels) || got.EnabledProviders != nil {
				t.Fatalf("a refused filter was written: %+v", got)
			}
		})
	}
}

func stringsOf(raw []any) []string {
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i] = v.(string)
	}
	return out
}

// DoD 2, 6: one filter type at a time. A provider write replaces a by-model
// list and a by-model write the provider filter; allowAllModels true allows
// all again (w10/m592). A filter listing every provider stays a filter, so a
// model of a provider the catalog gains later is off, as Devin's tag rule
// reads, and so does a list naming every model.
func TestSwitchingBetweenTheFilterTypes(t *testing.T) {
	enablePlatformCatalog(t)
	svc, _, _ := adminSession(t, nil)
	reads := newModelSurfaces(t, svc)
	row := func() store.AgentEnvironment { return svc.Environments.(*fakeEnvironments).rows["tea-a"] }
	write := func(label string, input EnvironmentInput) {
		t.Helper()
		if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", input); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}

	write("by model", EnvironmentInput{EnabledModels: &withoutOpus})
	reads.expectProviders("by model", testProviders, nil)
	write("by provider", EnvironmentInput{EnabledProviders: &[]string{"anthropic"}})
	if r := row(); !slices.Equal(r.EnabledProviders, []string{"anthropic"}) || !slices.Equal(r.KnownModels, testCatalog) || len(r.DisabledModels) != 0 {
		t.Fatalf("a provider write stored providers %v known %v disabled %v", r.EnabledProviders, r.KnownModels, r.DisabledModels)
	}
	reads.expect("by provider", testCatalog, testCatalog, []string{})
	reads.expectProviders("by provider", testProviders, []string{"anthropic"})
	write("back by model", EnvironmentInput{EnabledModels: &withoutOpus})
	if r := row(); r.EnabledProviders != nil {
		t.Fatalf("a by-model write kept the providers %v", r.EnabledProviders)
	}
	reads.expect("back by model", testCatalog, withoutOpus, []string{})
	reads.expectProviders("back by model", testProviders, nil)

	// The default must be offered after the write that sets it.
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogOpus), EnabledProviders: &[]string{"deepseek"}}); !isCode(err, CodeEnvironmentModelNotOffered) {
		t.Fatalf("a default the provider write turns off = %v, want %s", err, CodeEnvironmentModelNotOffered)
	}
	write("a default the provider filter offers", EnvironmentInput{DefaultModel: ptr(catalogOpus), EnabledProviders: &[]string{"anthropic"}})

	// Every provider listed is still a filter: a new provider's model is off.
	write("every provider", EnvironmentInput{EnabledProviders: &[]string{"anthropic", "deepseek"}})
	growCatalog(t, catalogAdded, otherProvider)
	grown := append(slices.Clone(grownCatalog), otherProvider)
	reads.expect("every provider, the catalog grown", grown, grownCatalog, []string{})
	reads.expectProviders("every provider, the catalog grown", []string{"deepseek", "anthropic", "moonshotai"}, testProviders)

	// Every model listed is an allowlist (w10/m592), allowAllModels true
	// allows all, the next model and provider included.
	write("every model", EnvironmentInput{EnabledModels: &grown})
	if r := row(); r.EnabledProviders != nil || !slices.Equal(r.KnownModels, grown) || len(r.DisabledModels) != 0 {
		t.Fatalf("every model on stored providers %v known %v disabled %v, want an allowlist of every model", r.EnabledProviders, r.KnownModels, r.DisabledModels)
	}
	write("allow all", EnvironmentInput{AllowAllModels: ptr(true)})
	if r := row(); r.EnabledProviders != nil || r.KnownModels != nil || len(r.DisabledModels) != 0 {
		t.Fatalf("allowAllModels stored providers %v known %v disabled %v, want allow all", r.EnabledProviders, r.KnownModels, r.DisabledModels)
	}
	growCatalog(t, catalogAdded, otherProvider, "openai/gpt-6")
	everything := append(slices.Clone(grown), "openai/gpt-6")
	reads.expect("allow all, the catalog grown again", everything, everything, []string{})
	reads.expectProviders("allow all, the catalog grown again", []string{"deepseek", "anthropic", "moonshotai", "openai"}, nil)
}

// DoD 4, 6 (the rollout read rule): a previous-release bex-api, m568's or the
// one before it, never writes enabled_providers. It offers the same set as
// the new code for a provider write under the catalog both run, and a row it
// writes afterwards reads as it reads it. The filter stays in force only
// while the turned-off list is the one its write derived.
func TestTheProviderFilterRolloutReadRule(t *testing.T) {
	enablePlatformCatalog(t)
	growCatalog(t, otherProvider)
	catalog := append(slices.Clone(testCatalog), otherProvider)
	// m568's read ignores enabled_providers; the read before m568 is the opt-out.
	m568 := func(row store.AgentEnvironment) []string {
		row.EnabledProviders = nil
		on, _ := leftOn(catalog, row)
		return on
	}
	optOut := func(row store.AgentEnvironment) []string {
		return slices.DeleteFunc(slices.Clone(catalog), func(m string) bool { return m != catalog[0] && slices.Contains(row.DisabledModels, m) })
	}

	svc, _, _ := adminSession(t, nil)
	envs := svc.Environments.(*fakeEnvironments)
	for _, providers := range [][]string{{"anthropic"}, {"anthropic", "moonshotai"}, {"deepseek"}} {
		if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledProviders: &providers}); err != nil {
			t.Fatal(err)
		}
		row := envs.rows["tea-a"]
		on, added := leftOn(catalog, row)
		if !slices.Equal(on, m568(row)) || !slices.Equal(on, optOut(row)) || len(added) != 0 {
			t.Errorf("a %v write: the new read %v, m568's %v, the opt-out %v; want one set", providers, on, m568(row), optOut(row))
		}
	}

	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledProviders: &[]string{"anthropic"}}); err != nil {
		t.Fatal(err)
	}
	filter := envs.rows["tea-a"] // disabled [kimi], known catalog, providers [anthropic]
	for _, tc := range []struct {
		name     string
		write    func(row store.AgentEnvironment) store.AgentEnvironment
		previous func(row store.AgentEnvironment) []string
		inForce  bool
	}{
		{"m568 narrowing it", func(r store.AgentEnvironment) store.AgentEnvironment {
			r.DisabledModels, r.KnownModels = []string{catalogOpus, otherProvider}, catalog
			return r
		}, m568, false},
		{"m568 turning every model on", func(r store.AgentEnvironment) store.AgentEnvironment {
			r.DisabledModels, r.KnownModels = []string{}, nil
			return r
		}, m568, false},
		{"the release before m568 narrowing it", func(r store.AgentEnvironment) store.AgentEnvironment {
			r.DisabledModels = []string{catalogSonnet, otherProvider}
			return r
		}, optOut, false},
		{"the release before m568 turning every model on", func(r store.AgentEnvironment) store.AgentEnvironment {
			r.DisabledModels = []string{}
			return r
		}, optOut, false},
		{"a list equal to the filter's", func(r store.AgentEnvironment) store.AgentEnvironment {
			r.DisabledModels = []string{otherProvider}
			return r
		}, m568, true},
		{"another field", func(r store.AgentEnvironment) store.AgentEnvironment {
			r.NetworkTier = NetworkTierFull
			return r
		}, optOut, true},
	} {
		row := tc.write(filter)
		if on, added := leftOn(catalog, row); !slices.Equal(on, tc.previous(row)) || len(added) != 0 {
			t.Errorf("%s: the new read %v new %v, want the previous release's %v", tc.name, on, added, tc.previous(row))
		}
		if _, inForce := providerFilter(row); inForce != tc.inForce {
			t.Errorf("%s: the filter in force = %t, want %t", tc.name, inForce, tc.inForce)
		}
		envs.rows["tea-a"] = row
		env, err := svc.Environment(caller("admin"), "tea-a")
		if err != nil || (env.EnabledProviders != nil) != tc.inForce {
			t.Errorf("%s: the read's enabledProviders = %v err=%v, want in force %t", tc.name, env.EnabledProviders, err, tc.inForce)
		}
	}

	// Against a grown catalog the filter offers anthropic's next model, while
	// the row m568 narrowed is an allowlist that leaves it new and off.
	growCatalog(t, otherProvider, catalogAdded)
	grown := append(slices.Clone(catalog), catalogAdded)
	if on, added := leftOn(grown, filter); !slices.Equal(on, []string{catalogDefault, catalogSonnet, catalogOpus, catalogAdded}) || len(added) != 0 {
		t.Errorf("the filter, grown catalog: %v new %v", on, added)
	}
	narrowed := filter
	narrowed.DisabledModels = []string{catalogOpus, otherProvider}
	if on, added := leftOn(grown, narrowed); !slices.Equal(on, []string{catalogDefault, catalogSonnet}) || !slices.Equal(added, []string{catalogAdded}) {
		t.Errorf("m568's narrowing, grown catalog: %v new %v", on, added)
	}
}
