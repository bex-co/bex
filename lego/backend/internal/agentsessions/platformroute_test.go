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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const platformKey = "sk-or-v1-PLATFORM-SECRET"

var routeModelVars = []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"}

// testCatalog is the platform catalog enablePlatformCatalog installs: the
// route's default first.
var testCatalog = []string{agentsession.DefaultDevModel, "anthropic/claude-sonnet-5.5", "anthropic/claude-opus-5.5"}

// enablePlatformCatalog is enablePlatformRoute with testCatalog's models
// offered beside the default (w10/m516).
func enablePlatformCatalog(t *testing.T) {
	t.Helper()
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", strings.Join(testCatalog[1:], ","), platformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", "", "") })
}

func enablePlatformRoute(t *testing.T) {
	t.Helper()
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", "", platformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", "", "") })
}

func assertNoPlatformKey(t *testing.T, lifecycle *fakeLifecycle) {
	t.Helper()
	if lifecycle.modelAPIKey == platformKey || !strings.HasPrefix(lifecycle.modelAPIKey, "bex-model-proxy-placeholder-") {
		t.Fatal("the sandbox received something other than the session placeholder")
	}
	for name, value := range lifecycle.driverEnv {
		if strings.Contains(value, platformKey) {
			t.Fatalf("driver env %s carries the platform key", name)
		}
	}
}

// A workspace's new claude session is provisioned against
// OpenRouter with the pinned model in its driver env — even when the request
// names api.anthropic.com — while the sandbox holds only the placeholder.
func TestCreateInPlatformRouteWorkspaceTargetsOpenRouter(t *testing.T) {
	enablePlatformRoute(t)
	svc, _, _, lifecycle := fixture()
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", ModelEndpoint: "https://api.anthropic.com/v1", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.ModelEndpoint != "https://openrouter.ai/api" || lifecycle.modelEndpoint != "https://openrouter.ai/api" {
		t.Fatalf("endpoint = view %q sandbox %q, want OpenRouter", created.AgentConfig.ModelEndpoint, lifecycle.modelEndpoint)
	}
	for _, name := range routeModelVars {
		if lifecycle.driverEnv[name] != agentsession.DefaultDevModel {
			t.Fatalf("%s = %q", name, lifecycle.driverEnv[name])
		}
	}
	assertNoPlatformKey(t, lifecycle)
}

// With the route off a workspace runs on api.anthropic.com with no model
// vars, and an explicit OpenRouter endpoint is refused.
func TestCreateWithoutPlatformRouteIsUnchanged(t *testing.T) {
	svc, _, _, lifecycle := fixture()
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.ModelEndpoint != "https://api.anthropic.com/v1" || lifecycle.modelEndpoint != "https://api.anthropic.com/v1" {
		t.Fatalf("endpoint = view %q sandbox %q", created.AgentConfig.ModelEndpoint, lifecycle.modelEndpoint)
	}
	for _, name := range routeModelVars {
		if _, ok := lifecycle.driverEnv[name]; ok {
			t.Fatalf("%s set without the route", name)
		}
	}
	assertNoPlatformKey(t, lifecycle)

	input.AgentConfig.ModelEndpoint = "https://openrouter.ai/api"
	if _, err := svc.Create(caller("alice"), input); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("explicit OpenRouter endpoint without the route = %v, want bad request", err)
	}
}

// The route is decided per workspace at dispatch time, not from the stored
// endpoint: a session created on api.anthropic.com moves onto OpenRouter on
// its next steer once the platform route is on.
func TestSteerSwitchesAnExistingSessionOntoThePlatformRoute(t *testing.T) {
	_ = agentsession.EnablePlatformModelRoute("", "", "", "")
	svc, st, _, lifecycle := fixture()
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if lifecycle.modelEndpoint != "https://api.anthropic.com/v1" {
		t.Fatalf("pre-route endpoint = %q", lifecycle.modelEndpoint)
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row

	enablePlatformRoute(t)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if lifecycle.modelEndpoint != "https://openrouter.ai/api" || lifecycle.driverEnv["ANTHROPIC_MODEL"] != agentsession.DefaultDevModel {
		t.Fatalf("steer endpoint = %q model = %q, want the platform route", lifecycle.modelEndpoint, lifecycle.driverEnv["ANTHROPIC_MODEL"])
	}
	assertNoPlatformKey(t, lifecycle)
}

// Capabilities: with the platform route on, every workspace is model-key
// ready through the platform key without consulting OpenBao; with it off a
// workspace needs its own BYO key.
func TestCapabilitiesCountPlatformRouteAsModelKeyReady(t *testing.T) {
	enablePlatformRoute(t)
	svc, _, _, _ := fixture()
	svc.GitHub = &fakeGitHub{conn: map[string]github.Connection{
		"tea-a": {Connected: true}, "tea-b": {Connected: true},
	}}
	// A broken store proves the routed path never reads the BYO key.
	svc.ModelKeys = &fakeModelKeys{err: errors.New("openbao unreachable")}
	for _, who := range []string{"alice", "bob"} {
		caps, err := svc.Capabilities(caller(who), "")
		if err != nil {
			t.Fatalf("%s: routed capabilities read OpenBao: %v", who, err)
		}
		if !caps.ModelKeyReady || !caps.Ready {
			t.Fatalf("%s: routed workspace not ready: %#v", who, caps)
		}
	}
	_ = agentsession.EnablePlatformModelRoute("", "", "", "")
	if _, err := svc.Capabilities(caller("bob"), ""); err == nil {
		t.Fatal("capabilities without the route skipped its BYO key read")
	}
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{
		modelKeySecretPath("tea-a"): {sandbox.ModelAPIKeyEnvVar: ""},
	}}
	capsB, err := svc.Capabilities(caller("bob"), "")
	if err != nil {
		t.Fatal(err)
	}
	if capsB.ModelKeyReady || capsB.Ready {
		t.Fatalf("workspace without the route or a BYO key reported ready: %#v", capsB)
	}
}

// The platform route serves only the claude profile, so a routed workspace
// without its own key reports claude ready and the other profiles not;
// setting a key makes every profile ready (076 live run: codex was offered as
// ready and failed on the model proxy).
func TestCapabilitiesReportModelReadinessPerAgent(t *testing.T) {
	enablePlatformRoute(t)
	svc, _, _, _ := fixture()
	ready := func() map[string]bool {
		t.Helper()
		caps, err := svc.Capabilities(caller("alice"), "")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, p := range caps.Agents {
			out[p.ID] = p.ModelKeyReady
		}
		return out
	}
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{}}
	if got := ready(); !got["claude"] || got["codex"] || got["gemini"] {
		t.Fatalf("routed workspace without its own key: %v, want only claude ready", got)
	}
	svc.ModelKeys = fixtureModelKeys("tea-a")
	if got := ready(); !got["claude"] || !got["codex"] || !got["gemini"] {
		t.Fatalf("workspace with its own key: %v, want every profile ready", got)
	}
}

// REST, GraphQL and MCP report the same per-agent model-key readiness (w10/m14
// t008): with the platform route on and no key of its own, a workspace is
// ready only for claude and for deepseek, the platform-only profile that
// route's key serves (ADR114 D4), and setting its key makes every profile
// ready.
func TestCapabilitiesModelReadinessIsTheSameOnEverySurface(t *testing.T) {
	enablePlatformRoute(t)
	svc, _, _, _ := fixture()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	a := &automationAdapters{t: t, svc: svc, mux: mux}
	surfaces := func() map[string]map[string]bool {
		t.Helper()
		readiness := func(name string, agents any) map[string]bool {
			list, ok := agents.([]any)
			if !ok || len(list) == 0 {
				t.Fatalf("%s: agents = %#v", name, agents)
			}
			out := map[string]bool{}
			for _, raw := range list {
				agent, _ := raw.(map[string]any)
				id, _ := agent["id"].(string)
				ready, ok := agent["modelKeyReady"].(bool)
				if id == "" || !ok {
					t.Fatalf("%s: agent %#v has no id or modelKeyReady", name, raw)
				}
				out[id] = ready
			}
			return out
		}
		code, rest := a.rest("GET", "/v1/agent-sessions/capabilities", nil, "alice")
		if code != http.StatusOK {
			t.Fatalf("REST capabilities = %d %v", code, rest)
		}
		gql, gqlErr := a.gql(`{ agentSessionCapabilities { agents { id modelKeyReady } } }`, "alice")
		if gqlErr != "" {
			t.Fatalf("GraphQL capabilities: %s", gqlErr)
		}
		mcpOut, mcpErr := a.mcp("get_agent_session_capabilities", map[string]any{}, "alice")
		if mcpErr != "" {
			t.Fatalf("MCP capabilities: %s", mcpErr)
		}
		gqlCaps, _ := gql["agentSessionCapabilities"].(map[string]any)
		return map[string]map[string]bool{
			"REST":    readiness("REST", rest["agents"]),
			"GraphQL": readiness("GraphQL", gqlCaps["agents"]),
			"MCP":     readiness("MCP", mcpOut["agents"]),
		}
	}
	check := func(label string, want func(id string) bool) {
		t.Helper()
		for surface, got := range surfaces() {
			for id, ready := range got {
				if ready != want(id) {
					t.Errorf("%s, %s: %s modelKeyReady = %v, want %v", label, surface, id, ready, want(id))
				}
			}
		}
	}
	a.svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{}}
	check("platform route, no own key", func(id string) bool { return id == "claude" || id == "deepseek" })
	a.svc.ModelKeys = fixtureModelKeys("tea-a")
	check("own key", func(string) bool { return true })
}

// A platform-only agent (ADR108 D5, muse) is offered and created only where a
// bex-owned route serves it. No route serves it yet, so even a workspace with
// its own model key neither sees it nor can start it, and the refusal is the
// agent-unavailable code, not a model-key prompt the key could never satisfy.
func TestPlatformOnlyAgentIsHiddenAndRefusedUntilARouteServesIt(t *testing.T) {
	svc, st, _, lifecycle := fixture()
	svc.ModelKeys = fixtureModelKeys("tea-a")
	enableMuse(svc, "tea-a")
	caps, err := svc.Capabilities(caller("alice"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range caps.Agents {
		if profile.ID == "muse" {
			t.Fatalf("capabilities offered muse without a route: %+v", caps.Agents)
		}
	}
	if len(caps.Agents) != 3 {
		t.Fatalf("capabilities agents = %+v, want claude, codex and gemini", caps.Agents)
	}
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "muse", Task: "fix the tests"}
	_, err = svc.Create(caller("alice"), input)
	if !isCode(err, AgentUnavailableCode) {
		t.Fatalf("create muse without a route = %v, want %s", err, AgentUnavailableCode)
	}
	if len(st.rows) != 0 || lifecycle.created != 0 {
		t.Fatalf("a refused create still created a session (rows=%d sandboxes=%d)", len(st.rows), lifecycle.created)
	}
}

// enableMuse records the admin acknowledgement muse needs (w7/m392), so the
// tests below exercise the route rather than the workspace's choice.
func enableMuse(svc *Service, workspaces ...string) {
	envs := newFakeEnvironments()
	for _, ws := range workspaces {
		envs.rows[ws] = store.AgentEnvironment{WorkspaceID: ws, NetworkTier: NetworkTierRegistries,
			DataUseAcks: map[string]store.AgentDataUseAck{"muse": {By: "admin"}}}
	}
	svc.Environments = envs
}

func enableMetaRoute(t *testing.T) {
	t.Helper()
	agentsession.EnablePlatformMetaRoute("meta-PLATFORM-SECRET")
	t.Cleanup(func() { agentsession.EnablePlatformMetaRoute("") })
}

func museInput() CreateRequest {
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "muse", Task: "fix the tests"}
	return input
}

// Where the Meta route serves it (ADR108), muse is offered and ready even
// without a workspace key, runs only its contributor model, never receives
// the Meta key in the sandbox, and is metered (maxSpendUSD is accepted).
func TestMuseRunsOnTheMetaRouteWithItsPinnedModel(t *testing.T) {
	enableMetaRoute(t)
	svc, _, _, lifecycle := fixture()
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{}}
	enableMuse(svc, "tea-a")
	caps, err := svc.Capabilities(caller("alice"), "")
	if err != nil {
		t.Fatal(err)
	}
	var muse *AgentProfile
	for i := range caps.Agents {
		if caps.Agents[i].ID == "muse" {
			muse = &caps.Agents[i]
		}
	}
	if muse == nil || !muse.ModelKeyReady || muse.Label != "Muse Code" {
		t.Fatalf("capabilities muse = %+v, want offered and ready", muse)
	}

	explicit := museInput()
	explicit.AgentConfig.Model = "muse-spark-1.3"
	if _, err := svc.Create(caller("alice"), explicit); !isCode(err, "AGENT_SESSION_MODEL_PINNED") {
		t.Fatalf("create muse on the standard-tier model = %v, want AGENT_SESSION_MODEL_PINNED", err)
	}
	limit := 5.0
	input := museInput()
	input.MaxSpendUSD = &limit
	view, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatalf("create muse: %v", err)
	}
	if view.AgentConfig.Model != "muse-spark-1.3-contributor" {
		t.Fatalf("muse model = %q, want the contributor model", view.AgentConfig.Model)
	}
	for name, value := range lifecycle.driverEnv {
		if strings.Contains(value, "meta-PLATFORM-SECRET") {
			t.Fatalf("the Meta key reached the sandbox env %s", name)
		}
	}
	if !strings.HasPrefix(lifecycle.modelAPIKey, "bex-model-proxy-placeholder-") {
		t.Fatal("the sandbox received something other than the session placeholder")
	}
}

// One Meta key carries every workspace's muse sessions, so live sessions are
// capped across workspaces; the session being steered never counts itself.
func TestMuseSessionsAreCappedAcrossWorkspaces(t *testing.T) {
	enableMetaRoute(t)
	svc, _, _, _ := fixture()
	enableMuse(svc, "tea-a", "tea-b")
	svc.MetaRouteMaxSessions = 2
	first, err := svc.Create(caller("alice"), museInput())
	if err != nil {
		t.Fatal(err)
	}
	other := museInput()
	other.OwnerID = "tea-b"
	other.Branch = "bex-agent/session-other"
	if _, err := svc.Create(caller("bob"), other); err != nil {
		t.Fatal(err)
	}
	third := museInput()
	third.Branch = "bex-agent/session-third"
	_, err = svc.Create(caller("alice"), third)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != MetaRouteCapacityCode || coded.Params["limit"] != 2 {
		t.Fatalf("third live muse session = %#v, want %s at limit 2", err, MetaRouteCapacityCode)
	}
	// Other agents are not affected.
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatalf("codex beside the muse cap: %v", err)
	}
	if err := svc.enforceMetaRouteCapacity(caller("alice"), "muse", first.ID); err != nil {
		t.Fatalf("steering a counted session must not count it twice: %v", err)
	}
}
