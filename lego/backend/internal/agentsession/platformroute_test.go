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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const testPlatformKey = "sk-or-v1-PLATFORM-SECRET"

func enablePlatformRoute(t *testing.T, workspaces ...string) {
	t.Helper()
	if err := EnablePlatformModelRoute(DevModelRouteOpenRouter, "", workspaces, testPlatformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EnablePlatformModelRoute("", "", nil, "") })
}

// recordingKV is a core.SecretKV that records every path read.
type recordingKV struct {
	memoryKV
	reads []string
}

func (r *recordingKV) Get(ctx context.Context, path string) (map[string]string, error) {
	r.reads = append(r.reads, path)
	return r.memoryKV.Get(ctx, path)
}

func mintFor(t *testing.T, workspace, endpoint string, keys *recordingKV) (ModelMintResponse, error) {
	t.Helper()
	config, _ := json.Marshal(map[string]string{"agent": "claude", "modelEndpoint": endpoint})
	session := store.AgentSession{ID: "ags-one", WorkspaceID: workspace, SandboxID: "sbx-1", Phase: "running", AgentConfig: config}
	minter := &ModelMinter{Keys: keys, Sessions: fakeSessions{session: session}}
	return minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: workspace, PodName: "sbx-1-0", PodUID: "uid-one"})
}

func TestPlatformModelRouteConfigRefusals(t *testing.T) {
	t.Cleanup(func() { _ = EnablePlatformModelRoute("", "", nil, "") })
	cases := []struct {
		name, route, model string
		workspaces         []string
		key, want          string
	}{
		{"unknown route", "bedrock", "", []string{"tea-a"}, "k", "not supported"},
		{"empty allowlist", DevModelRouteOpenRouter, "", nil, "k", "BEX_AGENT_PLATFORM_MODEL_WORKSPACES"},
		{"blank allowlist", DevModelRouteOpenRouter, "", []string{" ", ""}, "k", "BEX_AGENT_PLATFORM_MODEL_WORKSPACES"},
		{"bad model", DevModelRouteOpenRouter, "anthropic/claude opus", []string{"tea-a"}, "k", "BEX_AGENT_PLATFORM_MODEL"},
		{"missing key", DevModelRouteOpenRouter, "", []string{"tea-a"}, " ", "BEX_AGENT_PLATFORM_MODEL_KEY"},
	}
	for _, tc := range cases {
		err := EnablePlatformModelRoute(tc.route, tc.model, tc.workspaces, tc.key)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want it to name %q", tc.name, err, tc.want)
		}
	}
	if PlatformModelKeyReady("tea-a") || ModelRouteEnv("tea-a", "claude") != nil {
		t.Fatal("a refused configuration left a route installed")
	}
	// Production identity is irrelevant to the platform route (it has none).
	if err := ValidatePlatformModelRoute(DevModelRouteOpenRouter, "", []string{"tea-a"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

// An allowlisted workspace's claude sessions run on OpenRouter with the
// platform key and a pinned model, whatever endpoint the session stored; the
// workspace BYO key is never read.
func TestPlatformModelRouteMintsThePlatformKeyForAllowlistedWorkspace(t *testing.T) {
	enablePlatformRoute(t, "tea-allow")
	for _, stored := range []string{"https://api.anthropic.com/v1", openRouterEndpoint, ""} {
		keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-allow"): {ModelKeyField: "sk-ant-api03-BYO"}}}
		got, err := mintFor(t, "tea-allow", stored, keys)
		if err != nil {
			t.Fatalf("stored %q: %v", stored, err)
		}
		if got.Credential != testPlatformKey || got.EndpointHost != OpenRouterHost || got.Scheme != AuthSchemeBearer ||
			got.PathPrefix != "/api" || got.PinnedModel != DefaultDevModel {
			t.Fatalf("stored %q: mint = host %q scheme %q prefix %q pin %q platformKey=%v",
				stored, got.EndpointHost, got.Scheme, got.PathPrefix, got.PinnedModel, got.Credential == testPlatformKey)
		}
		if len(keys.reads) != 0 {
			t.Fatalf("stored %q: the workspace BYO key was read: %v", stored, keys.reads)
		}
	}
	// No BYO key at all is fine: the platform key is the credential.
	if _, err := mintFor(t, "tea-allow", "", &recordingKV{memoryKV: memoryKV{}}); err != nil {
		t.Fatalf("allowlisted workspace without a BYO key: %v", err)
	}
	if !PlatformModelKeyReady("tea-allow") {
		t.Fatal("allowlisted workspace not model-key ready")
	}
	env := ModelRouteEnv("tea-allow", "claude")
	for _, name := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if env[name] != DefaultDevModel {
			t.Fatalf("%s = %q", name, env[name])
		}
	}
	// Other profiles in the allowlisted workspace are untouched.
	if got, err := RegisteredModelEndpoint("tea-allow", "codex", ""); err != nil || got != "https://api.openai.com/v1" {
		t.Fatalf("codex endpoint = %q err=%v", got, err)
	}
	if ModelRouteEnv("tea-allow", "codex") != nil {
		t.Fatal("route env leaked to codex")
	}
}

// Every other workspace behaves exactly as with no route: api.anthropic.com,
// its own BYO key, no pin, no model env — even when the session or request
// names the OpenRouter endpoint explicitly.
func TestPlatformModelRouteNeverReachesOtherWorkspaces(t *testing.T) {
	enablePlatformRoute(t, "tea-allow")
	if got, err := RegisteredModelEndpoint("tea-other", "claude", ""); err != nil || got != "https://api.anthropic.com/v1" {
		t.Fatalf("endpoint = %q err=%v", got, err)
	}
	if _, err := RegisteredModelEndpoint("tea-other", "claude", openRouterEndpoint); err == nil {
		t.Fatal("non-allowlisted workspace accepted the OpenRouter endpoint")
	}
	if ModelRouteEnv("tea-other", "claude") != nil || PlatformModelKeyReady("tea-other") {
		t.Fatal("route env or readiness leaked to a non-allowlisted workspace")
	}
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-other"): {ModelKeyField: "sk-ant-api03-BYO"}}}
	got, err := mintFor(t, "tea-other", "https://api.anthropic.com/v1", keys)
	if err != nil {
		t.Fatal(err)
	}
	if got != (ModelMintResponse{Credential: "sk-ant-api03-BYO", EndpointHost: "api.anthropic.com", Scheme: AuthSchemeAnthropicKey}) {
		t.Fatalf("mint differs from the no-route response: host %q scheme %q prefix %q pin %q platformKey=%v",
			got.EndpointHost, got.Scheme, got.PathPrefix, got.PinnedModel, got.Credential == testPlatformKey)
	}
	// A stored OpenRouter endpoint gets nothing — never the platform key.
	if got, err := mintFor(t, "tea-other", openRouterEndpoint, keys); err == nil || got.Credential != "" {
		t.Fatalf("stored OpenRouter endpoint minted for a non-allowlisted workspace (err=%v)", err)
	}
	// No BYO key ⇒ the usual tenant-actionable refusal, not the platform key.
	if _, err := mintFor(t, "tea-other", "", &recordingKV{memoryKV: memoryKV{}}); !errors.Is(err, ErrModelKeyMissing) {
		t.Fatalf("err = %v, want ErrModelKeyMissing", err)
	}
}

// With both routes on (a dev control plane), the platform route wins for its
// allowlist and the dev route's BYO semantics hold everywhere else.
func TestPlatformRouteTakesPrecedenceOverDevRoute(t *testing.T) {
	if err := EnableDevModelRoute(DevModelRouteOpenRouter, "deepseek/dev-model", "dev-10"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EnableDevModelRoute("", "", "") })
	enablePlatformRoute(t, "tea-allow")
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-other"): {ModelKeyField: "sk-or-v1-BYO"}}}
	got, err := mintFor(t, "tea-other", "", keys)
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "sk-or-v1-BYO" || got.EndpointHost != OpenRouterHost || got.PinnedModel != "" {
		t.Fatalf("dev-route mint = host %q pin %q platformKey=%v", got.EndpointHost, got.PinnedModel, got.Credential == testPlatformKey)
	}
	if ModelRouteEnv("tea-other", "claude")["ANTHROPIC_MODEL"] != "deepseek/dev-model" ||
		ModelRouteEnv("tea-allow", "claude")["ANTHROPIC_MODEL"] != DefaultDevModel {
		t.Fatal("route env did not follow the per-workspace route")
	}
	if got, err := mintFor(t, "tea-allow", "", &recordingKV{memoryKV: memoryKV{}}); err != nil || got.Credential != testPlatformKey {
		t.Fatalf("allowlisted mint under both routes: err=%v platformKey=%v", err, got.Credential == testPlatformKey)
	}
}
