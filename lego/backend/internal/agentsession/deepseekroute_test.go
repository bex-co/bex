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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func mintDeepSeek(t *testing.T, workspace string, keys *recordingKV, spend *spendLedger) (ModelMintResponse, error) {
	t.Helper()
	config, _ := json.Marshal(map[string]string{"agent": "deepseek", "modelEndpoint": "https://openrouter.ai/api"})
	session := store.AgentSession{ID: "ags-one", WorkspaceID: workspace, SandboxID: "sbx-1", Phase: "running", AgentConfig: config}
	minter := &ModelMinter{Keys: keys, Sessions: fakeSessions{session: session}, Spend: spend}
	return minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: workspace, PodName: "sbx-1-0", PodUID: "uid-one"})
}

// The deepseek profile (ADR114) runs on the platform OpenRouter key, pinned
// to its own model rather than the route's claude model, under OpenRouter's
// /api prefix, even where the workspace set a key of its own (which is never
// read for it).
func TestDeepSeekMintsThePlatformOpenRouterKeyPinnedToTheProfileModel(t *testing.T) {
	enablePlatformRoute(t)
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "workspace-own-key"}}}
	got, err := mintDeepSeek(t, "tea-a", keys, &spendLedger{plan: store.PlanPro, byo: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != testPlatformKey || got.EndpointHost != OpenRouterHost || got.Scheme != AuthSchemeBearer ||
		got.PathPrefix != "/api" || got.PinnedModel != "~deepseek/deepseek-flash-latest" {
		t.Fatalf("mint = host %q scheme %q prefix %q pin %q platformKey=%v", got.EndpointHost, got.Scheme, got.PathPrefix, got.PinnedModel, got.Credential == testPlatformKey)
	}
	if len(keys.reads) != 0 {
		t.Fatalf("the workspace key was read for deepseek: %v", keys.reads)
	}
	if !PlatformRouteServes("deepseek") || !ProfileServed("deepseek") || !PlatformRouteMeters("deepseek", true) {
		t.Fatal("a workspace must be served and metered for deepseek, its own key notwithstanding")
	}
	// The OpenRouter key never serves muse, whose provider is Meta.
	if PlatformRouteServes("muse") {
		t.Fatal("the OpenRouter route served a Meta profile")
	}
}

// Without the OpenRouter platform route deepseek runs nowhere: the Meta key
// alone serves Meta's profiles, never this one.
func TestDeepSeekNeverRunsWithoutThePlatformRoute(t *testing.T) {
	enableMetaRoute(t)
	if PlatformRouteServes("deepseek") || ProfileServed("deepseek") {
		t.Fatal("the Meta route served an OpenRouter profile")
	}
	if _, err := mintDeepSeek(t, "tea-a", &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "own"}}}, &spendLedger{plan: store.PlanPro}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("mint without the OpenRouter route = %v, want forbidden", err)
	}
}

func TestDeepSeekAppliesThePlatformSpendGates(t *testing.T) {
	enablePlatformRoute(t)
	capUSD := int64(1_000_000)
	got, err := mintDeepSeek(t, "tea-a", &recordingKV{memoryKV: memoryKV{}}, &spendLedger{plan: store.PlanPro, override: &capUSD, used: capUSD})
	if !errors.Is(err, ErrModelSpendCapReached) || got.Credential != "" {
		t.Fatalf("mint at the cap = %+v, %v; want spend cap reached", got, err)
	}
}
