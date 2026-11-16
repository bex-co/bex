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

const testMetaKey = "meta-PLATFORM-SECRET"

func enableMetaRoute(t *testing.T) {
	t.Helper()
	EnablePlatformMetaRoute(testMetaKey)
	t.Cleanup(func() { EnablePlatformMetaRoute("") })
}

func mintMuse(t *testing.T, workspace string, keys *recordingKV, spend *spendLedger) (ModelMintResponse, error) {
	t.Helper()
	config, _ := json.Marshal(map[string]string{"agent": "muse", "modelEndpoint": "https://api.meta.ai/v1"})
	session := store.AgentSession{ID: "ags-one", WorkspaceID: workspace, SandboxID: "sbx-1", Phase: "running", AgentConfig: config}
	minter := &ModelMinter{Keys: keys, Sessions: fakeSessions{session: session}, Spend: spend}
	return minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: workspace, PodName: "sbx-1-0", PodUID: "uid-one"})
}

// The muse profile (ADR108) runs on the bex-owned Meta key in a
// workspace, pinned to its contributor model, even where the workspace set a
// key of its own (which is never read for it).
func TestMetaRouteMintsTheMetaKeyPinnedToTheProfileModel(t *testing.T) {
	enableMetaRoute(t)
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "workspace-own-key"}}}
	got, err := mintMuse(t, "tea-a", keys, &spendLedger{plan: store.PlanPro, byo: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != testMetaKey || got.EndpointHost != MetaModelHost || got.Scheme != AuthSchemeBearer ||
		got.PathPrefix != "" || got.PinnedModel != "muse-spark-1.3-contributor" {
		t.Fatalf("mint = host %q scheme %q prefix %q pin %q metaKey=%v", got.EndpointHost, got.Scheme, got.PathPrefix, got.PinnedModel, got.Credential == testMetaKey)
	}
	if len(keys.reads) != 0 {
		t.Fatalf("the workspace key was read for muse: %v", keys.reads)
	}
	if !PlatformRouteServes("muse") || !ProfileServed("muse") || !PlatformRouteMeters("muse", true) {
		t.Fatal("a workspace must be served and metered for muse, its own key notwithstanding")
	}
	// The Meta route serves platform-only profiles alone.
	if PlatformRouteServes("codex") || PlatformRouteMeters("codex", false) {
		t.Fatal("the Meta route leaked to a BYO profile")
	}
}

// The Meta route covers every workspace; only an empty key turns it off.
func TestMetaRouteServesEveryWorkspace(t *testing.T) {
	enableMetaRoute(t)
	got, err := mintMuse(t, "tea-other", &recordingKV{memoryKV: memoryKV{}}, &spendLedger{plan: store.PlanPro})
	if err != nil || got.Credential != testMetaKey {
		t.Fatalf("mint in another workspace = %v, metaKey=%v", err, got.Credential == testMetaKey)
	}
	if !PlatformRouteServes("muse") || !ProfileServed("muse") {
		t.Fatal("muse not served in every workspace")
	}
	EnablePlatformMetaRoute("")
	if PlatformRouteServes("muse") || ProfileServed("muse") {
		t.Fatal("a route without a key still serves muse")
	}
	if _, err := mintMuse(t, "tea-other", &recordingKV{memoryKV: memoryKV{}}, &spendLedger{plan: store.PlanPro}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("mint with the route off = %v, want forbidden", err)
	}
}

// The Meta route spends bex's money exactly like the OpenRouter route: the
// workspace cap and an unpaid balance refuse the exchange before the key is
// handed out.
func TestMetaRouteAppliesThePlatformSpendGates(t *testing.T) {
	enableMetaRoute(t)
	keys := &recordingKV{memoryKV: memoryKV{}}
	capUSD := int64(1_000_000)
	got, err := mintMuse(t, "tea-a", keys, &spendLedger{plan: store.PlanPro, override: &capUSD, used: capUSD})
	if !errors.Is(err, ErrModelSpendCapReached) || got.Credential != "" {
		t.Fatalf("mint at the cap = %+v, %v; want spend cap reached", got, err)
	}
	if _, err := mintMuse(t, "tea-a", keys, &spendLedger{plan: store.PlanPro, err: errors.New("store down")}); err == nil {
		t.Fatal("an unreadable spend position must fail closed")
	}
}
