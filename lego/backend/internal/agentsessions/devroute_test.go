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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR090: with the dev model route on, a claude session is provisioned against
// OpenRouter and its sandbox env names the route's model under every variable
// Claude Code reads; the release profile env (BEX_AGENT_ENV_JSON) is unchanged
// because the driver requires it to match the manifest exactly.
func TestCreateUnderDevModelRouteTargetsOpenRouter(t *testing.T) {
	if err := agentsession.EnableDevModelRoute(agentsession.DevModelRouteOpenRouter, "", "dev-10"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnableDevModelRoute("", "", "") })
	svc, _, _, lifecycle := fixture()
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.ModelEndpoint != "https://openrouter.ai/api" || lifecycle.modelEndpoint != "https://openrouter.ai/api" {
		t.Fatalf("endpoint = view %q sandbox %q, want the OpenRouter route", created.AgentConfig.ModelEndpoint, lifecycle.modelEndpoint)
	}
	for _, name := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if lifecycle.driverEnv[name] != agentsession.DefaultDevModel {
			t.Fatalf("%s = %q, want %q", name, lifecycle.driverEnv[name], agentsession.DefaultDevModel)
		}
	}
	_, releaseEnv := agentsession.AgentProfileRuntimeJSON("claude")
	if lifecycle.driverEnv["BEX_AGENT_ENV_JSON"] != releaseEnv {
		t.Fatalf("BEX_AGENT_ENV_JSON = %q, want the release profile's %q", lifecycle.driverEnv["BEX_AGENT_ENV_JSON"], releaseEnv)
	}
}

// With the route off (the default and the only production state), nothing of
// it reaches a sandbox.
func TestDriverEnvCarriesNoDevModelRouteByDefault(t *testing.T) {
	_ = agentsession.EnableDevModelRoute("", "", "")
	env := (&Service{}).driverEnv(AgentConfig{Agent: "claude"}, store.AgentSession{ID: "ags-x", WorkspaceID: "tea-a", Turns: 1})
	for _, name := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if _, ok := env[name]; ok {
			t.Fatalf("%s set with the dev route off", name)
		}
	}
}
