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

package agentsession

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDevModelRouteIsRefusedInProduction(t *testing.T) {
	t.Cleanup(func() { _ = EnableDevModelRoute("", "", "") })
	for _, identity := range []string{"", " ", "production"} {
		if err := EnableDevModelRoute(DevModelRouteOpenRouter, "", identity); err == nil || !strings.Contains(err.Error(), "dev-only") {
			t.Fatalf("identity %q: err = %v, want a dev-only refusal", identity, err)
		}
	}
	if err := EnableDevModelRoute("bedrock", "", "dev-10"); err == nil {
		t.Fatal("unknown route accepted")
	}
	if err := EnableDevModelRoute(DevModelRouteOpenRouter, "deepseek flash; rm", "dev-10"); err == nil {
		t.Fatal("malformed model id accepted")
	}
	// Every refusal left the route off: the release endpoint is the only one.
	if _, err := RegisteredModelEndpoint("claude", openRouterEndpoint); err == nil {
		t.Fatal("OpenRouter endpoint accepted with the route off")
	}
	if env := DevModelRouteEnv("claude"); env != nil {
		t.Fatalf("route env with the route off: %v", env)
	}
}

func TestDevModelRouteSwapsOnlyTheClaudeEndpoint(t *testing.T) {
	if err := EnableDevModelRoute(DevModelRouteOpenRouter, "", "dev-10"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EnableDevModelRoute("", "", "") })

	for _, requested := range []string{"", openRouterEndpoint, "https://api.anthropic.com/v1"} {
		got, err := RegisteredModelEndpoint("claude", requested)
		if err != nil || got != "https://openrouter.ai/api" {
			t.Fatalf("claude endpoint for %q = %q err=%v", requested, got, err)
		}
	}
	if _, err := RegisteredModelEndpoint("claude", "https://evil.example/v1"); err == nil {
		t.Fatal("an arbitrary endpoint was accepted")
	}
	if got, err := RegisteredModelEndpoint("codex", ""); err != nil || got != "https://api.openai.com/v1" {
		t.Fatalf("codex endpoint = %q err=%v, want the release endpoint", got, err)
	}
	// The release manifest itself is untouched.
	if profile, _ := LookupAgentProfile("claude"); profile.ModelEndpoint != "https://api.anthropic.com/v1" {
		t.Fatalf("release claude profile endpoint = %q", profile.ModelEndpoint)
	}
	env := DevModelRouteEnv(" Claude ")
	for _, name := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if env[name] != DefaultDevModel {
			t.Fatalf("%s = %q, want %q", name, env[name], DefaultDevModel)
		}
	}
	if DevModelRouteEnv("codex") != nil {
		t.Fatal("route env leaked to another profile")
	}

	// The mint hands the gateway the OpenRouter host, the Bearer scheme, and
	// the /api prefix Claude Code's /v1/messages needs.
	config, _ := json.Marshal(map[string]string{"agent": "claude", "modelEndpoint": "https://api.anthropic.com/v1"})
	session := activeSession()
	session.AgentConfig = config
	minter := &ModelMinter{
		Keys:     memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "sk-or-v1-key"}},
		Sessions: fakeSessions{session: session},
	}
	got, err := minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "sbx-1-0", PodUID: "uid-one"})
	if err != nil {
		t.Fatal(err)
	}
	if got.EndpointHost != "openrouter.ai" || got.Scheme != AuthSchemeBearer || got.PathPrefix != "/api" {
		t.Fatalf("mint = host %q scheme %q prefix %q", got.EndpointHost, got.Scheme, got.PathPrefix)
	}
}

func TestDevModelRouteHonorsAnExplicitModel(t *testing.T) {
	if err := EnableDevModelRoute(DevModelRouteOpenRouter, "deepseek/deepseek-v4.1-flash", "dev-3"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EnableDevModelRoute("", "", "") })
	if got := DevModelRouteEnv("claude")["ANTHROPIC_MODEL"]; got != "deepseek/deepseek-v4.1-flash" {
		t.Fatalf("ANTHROPIC_MODEL = %q", got)
	}
}

// memoryKV is a read-only core.SecretKV over a fixed map.
type memoryKV map[string]map[string]string

func (m memoryKV) Get(_ context.Context, path string) (map[string]string, error) { return m[path], nil }
func (memoryKV) Put(context.Context, string, map[string]string) error            { return nil }
func (memoryKV) Delete(context.Context, string) error                            { return nil }
func (memoryKV) List(context.Context, string) ([]string, error)                  { return nil, nil }
