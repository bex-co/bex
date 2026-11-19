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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The platform catalog (w10/m516) is the route's default first, then
// BEX_AGENT_PLATFORM_MODELS in order; a malformed or repeated entry refuses
// startup instead of being dropped.
func TestPlatformCatalogConfig(t *testing.T) {
	t.Cleanup(func() { _ = EnablePlatformModelRoute("", "", "", "") })
	if err := EnablePlatformModelRoute(DevModelRouteOpenRouter, "", " anthropic/claude-sonnet-5.5 , "+DefaultDevModel+",anthropic/claude-opus-5.5", testPlatformKey); err != nil {
		t.Fatal(err)
	}
	want := []string{DefaultDevModel, "anthropic/claude-sonnet-5.5", "anthropic/claude-opus-5.5"}
	if got, ok := PlatformModels(false); !ok || !slices.Equal(got, want) {
		t.Fatalf("catalog = %v ok=%v, want %v", got, ok, want)
	}
	got, _ := PlatformModels(false)
	got[0] = "mutated"
	if again, _ := PlatformModels(false); again[0] != DefaultDevModel {
		t.Fatal("a caller's slice aliases the route's catalog")
	}
	if _, ok := PlatformModels(true); ok {
		t.Fatal("a workspace with its own key is offered the platform catalog")
	}
	if err := EnablePlatformModelRoute(DevModelRouteOpenRouter, "", "", testPlatformKey); err != nil {
		t.Fatal(err)
	}
	if got, _ := PlatformModels(false); !slices.Equal(got, []string{DefaultDevModel}) {
		t.Fatalf("catalog without BEX_AGENT_PLATFORM_MODELS = %v, want the default alone", got)
	}
	for list, want := range map[string]string{
		"anthropic/claude opus":                                 "is not a model id",
		"anthropic/claude-opus-5.5,":                            "is not a model id",
		"anthropic/claude-opus-5.5,anthropic/claude-opus-5.5":   "twice",
		"anthropic/claude-opus-5.5,,anthropic/claude-haiku-5.5": "is not a model id",
	} {
		err := ValidatePlatformModelRoute(DevModelRouteOpenRouter, "", list)
		if err == nil || !strings.Contains(err.Error(), "BEX_AGENT_PLATFORM_MODELS") || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: err = %v, want BEX_AGENT_PLATFORM_MODELS %s", list, err, want)
		}
	}
}

// The mint pins the catalog model the session recorded, so the gateway
// refuses every other request model; a stored model the catalog does not
// offer (or none) pins the default. The sandbox env names the same model.
func TestPlatformMintPinsTheSessionsCatalogModel(t *testing.T) {
	t.Cleanup(func() { _ = EnablePlatformModelRoute("", "", "", "") })
	if err := EnablePlatformModelRoute(DevModelRouteOpenRouter, "", "anthropic/claude-sonnet-5.5,anthropic/claude-opus-5.5", testPlatformKey); err != nil {
		t.Fatal(err)
	}
	for stored, want := range map[string]string{
		"anthropic/claude-opus-5.5":   "anthropic/claude-opus-5.5",
		"anthropic/claude-sonnet-5.5": "anthropic/claude-sonnet-5.5",
		DefaultDevModel:               DefaultDevModel,
		"":                            DefaultDevModel,
		"anthropic/claude-sonnet-4.5": DefaultDevModel,
	} {
		config, _ := json.Marshal(map[string]string{"agent": "claude", "model": stored})
		session := store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running", AgentConfig: config}
		minter := &ModelMinter{Keys: &memoryKV{}, Sessions: fakeSessions{session: session}, Spend: &spendLedger{plan: store.PlanPro}}
		got, err := minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "sbx-1-0", PodUID: "uid-one"})
		if err != nil {
			t.Fatalf("stored %q: %v", stored, err)
		}
		if got.PinnedModel != want || got.Credential != testPlatformKey {
			t.Fatalf("stored %q: pin %q platformKey=%v, want pin %q", stored, got.PinnedModel, got.Credential == testPlatformKey, want)
		}
		env := ModelRouteEnv("claude", stored, false)
		for _, name := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
			if env[name] != want {
				t.Fatalf("stored %q: %s = %q, want %q", stored, name, env[name], want)
			}
		}
	}
}
