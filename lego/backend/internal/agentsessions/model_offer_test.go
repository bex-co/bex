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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w10/m527: a workspace admin chooses which platform catalog models its
// members may pick, as Devin's admins choose the agents and models on offer.

var (
	catalogDefault = testCatalog[0]
	catalogSonnet  = testCatalog[1]
	catalogOpus    = testCatalog[2]
)

// withoutOpus is testCatalog with its opus model turned off.
var withoutOpus = []string{catalogDefault, catalogSonnet}

// adminSession is sessionWithEnvironment whose workspace also has an admin,
// who holds every relation on it, for the environment writes.
func adminSession(t *testing.T, row *store.AgentEnvironment) (*Service, *fakeStore, *fakeLifecycle) {
	t.Helper()
	svc, st, lifecycle := sessionWithEnvironment(t, row)
	members := svc.Authz
	svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		if subject == "user:admin" {
			return object == "workspace:tea-a", nil
		}
		return members.Check(ctx, subject, relation, object)
	})
	return svc, st, lifecycle
}

// turnOff stores models as the workspace's turned-off list, as an admin's
// enabledModels write leaves it.
func turnOff(t *testing.T, svc *Service, models ...string) {
	t.Helper()
	envs := svc.Environments.(*fakeEnvironments)
	row, ok := envs.rows["tea-a"]
	if !ok {
		row = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries}
	}
	row.DisabledModels = models
	envs.rows["tea-a"] = row
}

// t002: REST, GraphQL and MCP read the whole catalog as enabled for a
// workspace with no row, keep `models` the whole catalog, and an admin's
// write on any surface reads back on the others. A member cannot change it.
func TestEnabledModelsOnEverySurface(t *testing.T) {
	enablePlatformCatalog(t)
	svc := environmentService("tea-a", newFakeEnvironments())
	schema := environmentSchema(t, svc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	list := func(raw any) []string {
		out := []string{}
		for _, m := range raw.([]any) {
			out = append(out, m.(string))
		}
		return out
	}
	reads := func() map[string][2][]string {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil).WithContext(caller("viewer")))
		var rest map[string]any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &rest) != nil {
			t.Fatalf("REST read = %d %s", rec.Code, rec.Body.String())
		}
		result := graphql.Do(graphql.Params{Schema: schema, Context: caller("viewer"), RequestString: `{ agentEnvironment(ownerId: "tea-a") { models enabledModels } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL read: %v", result.Errors)
		}
		gql := result.Data.(map[string]any)["agentEnvironment"].(map[string]any)
		res, err := newMCPClient(t, caller("viewer"), svc).CallTool(caller("viewer"), &mcp.CallToolParams{Name: "get_agent_environment", Arguments: map[string]any{"ownerId": "tea-a"}})
		var mcpEnv map[string]any
		if err != nil || res.IsError || json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &mcpEnv) != nil {
			t.Fatalf("MCP read = %+v err=%v", res, err)
		}
		return map[string][2][]string{
			"REST":    {list(rest["models"]), list(rest["enabledModels"])},
			"GraphQL": {list(gql["models"]), list(gql["enabledModels"])},
			"MCP":     {list(mcpEnv["models"]), list(mcpEnv["enabledModels"])},
		}
	}
	expect := func(label string, enabled []string) {
		t.Helper()
		for surface, got := range reads() {
			if !slices.Equal(got[0], testCatalog) || !slices.Equal(got[1], enabled) {
				t.Errorf("%s, %s: models %v enabledModels %v, want %v and %v", label, surface, got[0], got[1], testCatalog, enabled)
			}
		}
	}
	expect("no row", testCatalog)

	// A member cannot change it on any surface.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"enabledModels":["`+catalogDefault+`"]}`)).WithContext(caller("dev")))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("REST member write = %d, want 403", rec.Code)
	}
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("dev"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {enabledModels: ["` + catalogDefault + `"]}) { enabledModels } }`}); len(result.Errors) == 0 {
		t.Fatal("GraphQL member write succeeded")
	}
	if res, err := newMCPClient(t, caller("dev"), svc).CallTool(caller("dev"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "enabledModels": []string{catalogDefault}}}); err != nil || !res.IsError {
		t.Fatalf("MCP member write = %+v err=%v, want a refusal", res, err)
	}
	expect("after refused member writes", testCatalog)

	// An admin's write on each surface reads back on the others, in catalog
	// order whatever order it was sent in.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"enabledModels":["`+catalogSonnet+`"," `+catalogDefault+`"]}`)).WithContext(caller("admin")))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST admin write = %d %s", rec.Code, rec.Body.String())
	}
	expect("after the REST write", withoutOpus)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {enabledModels: ["` + catalogDefault + `", "` + catalogOpus + `"]}) { enabledModels } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL admin write: %v", result.Errors)
	}
	expect("after the GraphQL write", []string{catalogDefault, catalogOpus})
	if res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "enabledModels": testCatalog}}); err != nil || res.IsError {
		t.Fatalf("MCP admin write = %+v err=%v", res, err)
	}
	expect("after the MCP write", testCatalog)
	if row := svc.Environments.(*fakeEnvironments).rows["tea-a"]; len(row.DisabledModels) != 0 {
		t.Fatalf("the whole catalog back on stored %v turned off", row.DisabledModels)
	}
}

// t002: leaving the catalog's default off (an empty list does), naming a
// model outside the catalog, and narrowing a workspace on its own key are each
// a typed 400 on every surface, and nothing is written.
func TestEnabledModelsRefusalsOnEverySurface(t *testing.T) {
	enablePlatformCatalog(t)
	for _, tc := range []struct {
		name   string
		models []string
		byo    bool
		model  any // the refusal's model param; nil when it names none
	}{
		{"default off", []string{catalogSonnet, catalogOpus}, false, catalogDefault},
		{"empty list", []string{}, false, catalogDefault},
		{"outside the catalog", []string{catalogDefault, "anthropic/claude-sonnet-4.5"}, false, "anthropic/claude-sonnet-4.5"},
		{"own key", []string{catalogDefault}, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envs := newFakeEnvironments()
			svc := environmentService("tea-a", envs)
			if tc.byo {
				at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
				envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, ModelKeySetAt: &at}
			}
			before := envs.rows["tea-a"]
			_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &tc.models})
			var coded *core.CodedError
			if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != ModelListInvalidCode ||
				coded.Params["field"] != "enabledModels" || coded.Params["model"] != tc.model {
				t.Fatalf("service = %#v, want %s naming enabledModels and %v", err, ModelListInvalidCode, tc.model)
			}
			if tc.model != nil && !slices.Equal(coded.Params["models"].([]string), testCatalog) {
				t.Fatalf("params models = %v, want the catalog", coded.Params["models"])
			}

			body, _ := json.Marshal(map[string]any{"enabledModels": tc.models})
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(string(body))).WithContext(caller("admin")))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), ModelListInvalidCode) {
				t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
			}
			res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "enabledModels": tc.models}})
			if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, ModelListInvalidCode) {
				t.Fatalf("MCP = %+v err=%v", res, err)
			}
			models := make([]any, len(tc.models))
			for i, m := range tc.models {
				models[i] = m
			}
			result := graphql.Do(graphql.Params{Schema: environmentSchema(t, svc), Context: caller("admin"),
				RequestString:  `mutation($input: AgentEnvironmentInput!) { updateAgentEnvironment(ownerId: "tea-a", input: $input) { enabledModels } }`,
				VariableValues: map[string]any{"input": map[string]any{"enabledModels": models}}})
			if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != ModelListInvalidCode {
				t.Fatalf("GraphQL = %#v", result.Errors)
			}
			if got := envs.rows["tea-a"]; !slices.Equal(got.DisabledModels, before.DisabledModels) || !got.UpdatedAt.Equal(before.UpdatedAt) {
				t.Fatalf("a refused list was written: %+v", got)
			}
		})
	}
}

// t002/t003: defaultModel may not name a turned-off model, in the same update
// or after it, but turning off the model the stored default names is allowed
// (Devin's allowlist overrides a pinned default): the default then stops
// taking effect, and the catalog's default runs.
func TestDefaultModelAndTheEnabledModels(t *testing.T) {
	enablePlatformCatalog(t)
	svc, _, _ := adminSession(t, nil)
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogOpus), EnabledModels: &withoutOpus}); !isCode(err, CodeEnvironmentModelNotOffered) {
		t.Fatalf("a default the same update turns off = %v, want %s", err, CodeEnvironmentModelNotOffered)
	}
	got, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogOpus)})
	if err != nil || *got.DefaultModel != catalogOpus {
		t.Fatalf("default opus = %+v err=%v", got, err)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &withoutOpus})
	if err != nil || got.DefaultModel == nil || *got.DefaultModel != catalogOpus || !slices.Equal(got.EnabledModels, withoutOpus) {
		t.Fatalf("turning the default's model off = %+v err=%v, want it allowed and the default kept", got, err)
	}
	caps, err := svc.Capabilities(caller("alice"), "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	claude := caps.Agents[slices.IndexFunc(caps.Agents, func(a AgentProfile) bool { return a.ID == "claude" })]
	if claude.Model == nil || *claude.Model != catalogDefault || !slices.Equal(claude.Models, withoutOpus) {
		t.Fatalf("capabilities claude = %v %v, want the catalog default and %v", claude.Model, claude.Models, withoutOpus)
	}
	_, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogOpus)})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeEnvironmentModelNotOffered || !slices.Equal(coded.Params["models"].([]string), withoutOpus) ||
		!strings.Contains(coded.Error(), "turned off in this workspace") {
		t.Fatalf("default naming a turned-off model = %v, want %s listing %v", err, CodeEnvironmentModelNotOffered, withoutOpus)
	}
	if _, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr(catalogSonnet)}); err != nil {
		t.Fatalf("an offered default: %v", err)
	}
	if _, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr("")}); err != nil {
		t.Fatalf("clearing the default: %v", err)
	}
}

// t003: a create, a message, an automation create and an automation update
// naming a turned-off model are refused with AGENT_SESSION_MODEL_NOT_OFFERED,
// listing only the offered models, and the capabilities read leaves it out.
func TestATurnedOffModelIsRefusedEverywhere(t *testing.T) {
	enablePlatformCatalog(t)
	svc, st, lifecycle := sessionWithEnvironment(t, nil)
	turnOff(t, svc, catalogOpus)
	offered := func(err error, field string) bool {
		var coded *core.CodedError
		return errors.As(err, &coded) && coded.Code == CodeSessionModelNotOffered && coded.Params["field"] == field &&
			coded.Params["model"] == catalogOpus && slices.Equal(coded.Params["models"].([]string), withoutOpus)
	}

	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogOpus, Task: "fix the tests"}
	if _, err := svc.Create(caller("alice"), input); !offered(err, "agentConfig.model") {
		t.Fatalf("create on a turned-off model = %v", err)
	}
	if lifecycle.created != 0 || len(st.rows) != 0 {
		t.Fatal("a refused create started something")
	}
	input.AgentConfig.Model = catalogSonnet
	created, err := svc.Create(caller("alice"), input)
	if err != nil || created.AgentConfig.Model != catalogSonnet {
		t.Fatalf("create on an offered model = %+v err=%v", created.AgentConfig, err)
	}
	settleTurn(st, created.ID)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "harder", Model: catalogOpus}); !offered(err, "model") {
		t.Fatalf("a message on a turned-off model = %v", err)
	}
	if storedModel(t, st, created.ID) != catalogSonnet {
		t.Fatal("a refused message changed the session's model")
	}

	autos, _ := automationTestService("tea-a", newFakeAutomations())
	autos.Environments = svc.Environments
	in := automationInput("nightly")
	in.Template = &AutomationTemplate{Task: "triage", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "claude", Model: catalogOpus}, MaxSpendUSD: ptr(2.0)}
	if _, err := autos.CreateAgentAutomation(caller("dev"), "tea-a", in); !offered(err, "agentConfig.model") {
		t.Fatalf("automation create on a turned-off model = %v", err)
	}
	in.Template.AgentConfig.Model = catalogSonnet
	saved, err := autos.CreateAgentAutomation(caller("dev"), "tea-a", in)
	if err != nil {
		t.Fatal(err)
	}
	update := *in.Template
	update.AgentConfig.Model = catalogOpus
	if _, err := autos.UpdateAgentAutomation(caller("dev"), "tea-a", saved.ID, AgentAutomationInput{Template: &update}); !offered(err, "agentConfig.model") {
		t.Fatalf("automation update to a turned-off model = %v", err)
	}

	caps, err := svc.Capabilities(caller("alice"), "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	claude := caps.Agents[slices.IndexFunc(caps.Agents, func(a AgentProfile) bool { return a.ID == "claude" })]
	if !slices.Equal(claude.Models, withoutOpus) || claude.Model == nil || *claude.Model != catalogDefault {
		t.Fatalf("capabilities claude = %v %v, want %v", claude.Model, claude.Models, withoutOpus)
	}
}

// t003: a session whose stored model was turned off keeps it until its next
// dispatch, which runs the workspace default (or the catalog's default when
// that is off too) and records it as the session's model.
func TestASessionOnATurnedOffModelRunsTheDefaultFromItsNextTurn(t *testing.T) {
	enablePlatformCatalog(t)
	for _, tc := range []struct {
		name     string
		def      string
		disabled []string
		want     string
	}{
		{"the workspace default", catalogSonnet, []string{catalogOpus}, catalogSonnet},
		{"the catalog default when the workspace default is off too", catalogSonnet, []string{catalogSonnet, catalogOpus}, catalogDefault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, st, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultModel: tc.def})
			input := createInput()
			input.AgentConfig = AgentConfig{Agent: "claude", Model: catalogOpus, Task: "fix the tests"}
			created, err := svc.Create(caller("alice"), input)
			if err != nil || created.AgentConfig.Model != catalogOpus {
				t.Fatalf("create on opus = %+v err=%v", created.AgentConfig, err)
			}
			turnOff(t, svc, tc.disabled...)
			if read, err := svc.Get(caller("alice"), created.ID); err != nil || read.AgentConfig.Model != catalogOpus {
				t.Fatalf("the turn in flight reads %q err=%v, want it kept on opus", read.AgentConfig.Model, err)
			}

			settleTurn(st, created.ID)
			if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "and the docs"}); err != nil {
				t.Fatal(err)
			}
			if lifecycle.driverEnv["ANTHROPIC_MODEL"] != tc.want || storedModel(t, st, created.ID) != tc.want {
				t.Fatalf("next turn ran %q stored %q, want %q", lifecycle.driverEnv["ANTHROPIC_MODEL"], storedModel(t, st, created.ID), tc.want)
			}
			if read, err := svc.Get(caller("alice"), created.ID); err != nil || read.AgentConfig.Model != tc.want {
				t.Fatalf("session read = %q err=%v, want %q", read.AgentConfig.Model, err, tc.want)
			}
		})
	}
}

// t003: an automation whose template names a turned-off model runs the
// workspace's model and its run says why, while an offered one stands.
func TestAnAutomationOnATurnedOffModelRunsTheDefault(t *testing.T) {
	enablePlatformCatalog(t)
	svc, _, _ := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultModel: catalogSonnet})
	turnOff(t, svc, catalogOpus)
	req := CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Model: catalogSonnet}}
	if note := svc.retiredTemplateModel(caller("alice"), &req); note != "" || req.AgentConfig.Model != catalogSonnet {
		t.Fatalf("an offered model was replaced: %q %q", note, req.AgentConfig.Model)
	}
	req.AgentConfig.Model = catalogOpus
	note := svc.retiredTemplateModel(caller("alice"), &req)
	if req.AgentConfig.Model != "" || note != "the template's model "+catalogOpus+" is turned off in this workspace; this run used "+catalogSonnet {
		t.Fatalf("turned-off model: model %q note %q", req.AgentConfig.Model, note)
	}
}

// t003: the list narrows only the platform catalog. On the workspace's own
// key a stored list changes nothing: any model id is accepted and the reads
// offer no list.
func TestTheModelListLeavesAnOwnKeyWorkspaceAlone(t *testing.T) {
	enablePlatformCatalog(t)
	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	svc, st, lifecycle := adminSession(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, ModelKeySetAt: &at, DisabledModels: []string{catalogOpus}})
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: "claude-opus-4-1", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil || created.AgentConfig.Model != "claude-opus-4-1" {
		t.Fatalf("own-key create = %+v err=%v", created.AgentConfig, err)
	}
	settleTurn(st, created.ID)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again", Model: catalogOpus}); err != nil ||
		lifecycle.driverEnv["ANTHROPIC_MODEL"] != catalogOpus || storedModel(t, st, created.ID) != catalogOpus {
		t.Fatalf("own-key message naming a model the list turns off = %v (ran %q, stored %q)", err, lifecycle.driverEnv["ANTHROPIC_MODEL"], storedModel(t, st, created.ID))
	}
	env, err := svc.Environment(caller("admin"), "tea-a")
	if err != nil || len(env.Models) != 0 || len(env.EnabledModels) != 0 || env.EnabledModels == nil {
		t.Fatalf("own-key read = models %v enabledModels %#v err=%v, want both empty", env.Models, env.EnabledModels, err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{DefaultModel: ptr("claude-opus-4-1")}); err != nil {
		t.Fatalf("own-key default model: %v", err)
	}

	// The list is judged by the key the workspace has after the update: one
	// that clears the key narrows the catalog, one that sets it is refused.
	got, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ClearModelAPIKey: true, EnabledModels: &withoutOpus})
	if err != nil || got.ModelSource != ModelSourcePlatform || !slices.Equal(got.EnabledModels, withoutOpus) {
		t.Fatalf("clearing the key with a list = %+v err=%v", got, err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr("sk-ant-new"), EnabledModels: &testCatalog}); !isCode(err, ModelListInvalidCode) {
		t.Fatalf("setting the key with a list = %v, want %s", err, ModelListInvalidCode)
	}
	if env, err := svc.Environment(caller("admin"), "tea-a"); err != nil || env.ModelKeySet || !slices.Equal(env.EnabledModels, withoutOpus) {
		t.Fatalf("a refused key-and-list update changed something: %+v err=%v", env, err)
	}
}

// t003: the catalog's default is offered even when a stored list names it
// (one written before the deploy configuration changed the default), and
// with the route off there is no catalog to offer or narrow.
func TestOfferedModelsKeepTheCatalogDefault(t *testing.T) {
	enablePlatformCatalog(t)
	got, routed := offeredModels(store.AgentEnvironment{DisabledModels: []string{catalogDefault, catalogOpus, "anthropic/claude-retired"}})
	if !routed || !slices.Equal(got, withoutOpus) {
		t.Fatalf("offered = %v routed=%v, want %v", got, routed, withoutOpus)
	}
	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	if got, routed := offeredModels(store.AgentEnvironment{ModelKeySetAt: &at}); routed || len(got) != 0 {
		t.Fatalf("own key offered %v routed=%v, want no catalog", got, routed)
	}

	if err := agentsession.EnablePlatformModelRoute("", "", "", ""); err != nil {
		t.Fatal(err)
	}
	svc := environmentService("tea-a", newFakeEnvironments())
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledModels: &withoutOpus}); !isCode(err, ModelListInvalidCode) ||
		!strings.Contains(err.Error(), "which this bex-api does not offer") {
		t.Fatalf("a list with the route off = %v, want %s", err, ModelListInvalidCode)
	}
}
