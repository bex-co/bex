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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

const platformKey = "sk-or-v1-PLATFORM-SECRET"

var routeModelVars = []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"}

func enablePlatformRoute(t *testing.T, workspaces ...string) {
	t.Helper()
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", workspaces, platformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
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

// An allowlisted workspace's new claude session is provisioned against
// OpenRouter with the pinned model in its driver env — even when the request
// names api.anthropic.com — while the sandbox holds only the placeholder.
func TestCreateInPlatformRouteWorkspaceTargetsOpenRouter(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
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

// A non-allowlisted workspace is byte-identical to no route: api.anthropic.com,
// no model vars, and an explicit OpenRouter endpoint is refused.
func TestCreateOutsidePlatformRouteAllowlistIsUnchanged(t *testing.T) {
	enablePlatformRoute(t, "tea-b")
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
			t.Fatalf("%s set outside the allowlist", name)
		}
	}
	assertNoPlatformKey(t, lifecycle)

	input.AgentConfig.ModelEndpoint = "https://openrouter.ai/api"
	if _, err := svc.Create(caller("alice"), input); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("explicit OpenRouter endpoint outside the allowlist = %v, want bad request", err)
	}
}

// The route is decided per workspace at dispatch time, not from the stored
// endpoint: a session created on api.anthropic.com moves onto OpenRouter on
// its next steer once its workspace is allowlisted.
func TestSteerSwitchesAnExistingSessionOntoThePlatformRoute(t *testing.T) {
	_ = agentsession.EnablePlatformModelRoute("", "", nil, "")
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

	enablePlatformRoute(t, "tea-a")
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if lifecycle.modelEndpoint != "https://openrouter.ai/api" || lifecycle.driverEnv["ANTHROPIC_MODEL"] != agentsession.DefaultDevModel {
		t.Fatalf("steer endpoint = %q model = %q, want the platform route", lifecycle.modelEndpoint, lifecycle.driverEnv["ANTHROPIC_MODEL"])
	}
	assertNoPlatformKey(t, lifecycle)
}

// Capabilities: an allowlisted workspace is model-key ready through the
// platform key without consulting OpenBao; every other workspace still needs
// its own BYO key.
func TestCapabilitiesCountPlatformRouteAsModelKeyReady(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	svc, _, _, _ := fixture()
	svc.GitHub = &fakeGitHub{conn: map[string]github.Connection{
		"tea-a": {Connected: true}, "tea-b": {Connected: true},
	}}
	// A broken store proves the allowlisted path never reads the BYO key.
	svc.ModelKeys = &fakeModelKeys{err: errors.New("openbao unreachable")}
	caps, err := svc.Capabilities(caller("alice"), "")
	if err != nil {
		t.Fatalf("allowlisted capabilities read OpenBao: %v", err)
	}
	if !caps.ModelKeyReady || !caps.Ready {
		t.Fatalf("allowlisted workspace not ready: %#v", caps)
	}
	if _, err := svc.Capabilities(caller("bob"), ""); err == nil {
		t.Fatal("non-allowlisted capabilities skipped its BYO key read")
	}
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{
		modelKeySecretPath("tea-a"): {sandbox.ModelAPIKeyEnvVar: ""},
	}}
	capsB, err := svc.Capabilities(caller("bob"), "")
	if err != nil {
		t.Fatal(err)
	}
	if capsB.ModelKeyReady || capsB.Ready {
		t.Fatalf("non-allowlisted workspace without a BYO key reported ready: %#v", capsB)
	}
}
