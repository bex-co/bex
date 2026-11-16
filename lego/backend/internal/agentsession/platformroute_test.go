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

func enablePlatformRoute(t *testing.T) {
	t.Helper()
	if err := EnablePlatformModelRoute(DevModelRouteOpenRouter, "", testPlatformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EnablePlatformModelRoute("", "", "") })
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
	minter := &ModelMinter{Keys: keys, Sessions: fakeSessions{session: session}, Spend: &spendLedger{plan: store.PlanPro}}
	return minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: workspace, PodName: "sbx-1-0", PodUID: "uid-one"})
}

func TestPlatformModelRouteConfigRefusals(t *testing.T) {
	t.Cleanup(func() { _ = EnablePlatformModelRoute("", "", "") })
	cases := []struct {
		name, route, model, key, want string
	}{
		{"unknown route", "bedrock", "", "k", "not supported"},
		{"bad model", DevModelRouteOpenRouter, "anthropic/claude opus", "k", "BEX_AGENT_PLATFORM_MODEL"},
		{"missing key", DevModelRouteOpenRouter, "", " ", "BEX_AGENT_PLATFORM_MODEL_KEY"},
	}
	for _, tc := range cases {
		err := EnablePlatformModelRoute(tc.route, tc.model, tc.key)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want it to name %q", tc.name, err, tc.want)
		}
	}
	if PlatformModelKeyReady("tea-a", false) || ModelRouteEnv("tea-a", "claude", false) != nil {
		t.Fatal("a refused configuration left a route installed")
	}
	// Production identity is irrelevant to the platform route (it has none).
	if err := ValidatePlatformModelRoute(DevModelRouteOpenRouter, ""); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

// A workspace's claude sessions run on OpenRouter with the
// platform key and a pinned model, whatever endpoint the session stored; the
// workspace BYO key is never read.
func TestPlatformModelRouteMintsThePlatformKeyForAWorkspace(t *testing.T) {
	enablePlatformRoute(t)
	for _, stored := range []string{"https://api.anthropic.com/v1", openRouterEndpoint, ""} {
		keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "sk-ant-api03-BYO"}}}
		got, err := mintFor(t, "tea-a", stored, keys)
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
	if _, err := mintFor(t, "tea-a", "", &recordingKV{memoryKV: memoryKV{}}); err != nil {
		t.Fatalf("workspace without a BYO key: %v", err)
	}
	if !PlatformModelKeyReady("tea-a", false) {
		t.Fatal("workspace not model-key ready")
	}
	env := ModelRouteEnv("tea-a", "claude", false)
	for _, name := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if env[name] != DefaultDevModel {
			t.Fatalf("%s = %q", name, env[name])
		}
	}
	// Other profiles in the workspace are untouched.
	if got, err := RegisteredModelEndpoint("tea-a", "codex", "", false); err != nil || got != "https://api.openai.com/v1" {
		t.Fatalf("codex endpoint = %q err=%v", got, err)
	}
	if ModelRouteEnv("tea-a", "codex", false) != nil {
		t.Fatal("route env leaked to codex")
	}
}

// The platform route covers every workspace: any workspace without its own
// key runs claude on OpenRouter with the platform key and the pinned model.
func TestPlatformModelRouteCoversEveryWorkspace(t *testing.T) {
	enablePlatformRoute(t)
	for _, ws := range []string{"tea-a", "tea-other"} {
		if got, err := RegisteredModelEndpoint(ws, "claude", "", false); err != nil || got != openRouterEndpoint {
			t.Fatalf("%s: endpoint = %q err=%v", ws, got, err)
		}
		if !PlatformModelKeyReady(ws, false) || ModelRouteEnv(ws, "claude", false)["ANTHROPIC_MODEL"] != DefaultDevModel {
			t.Fatalf("%s: not on the platform route", ws)
		}
		got, err := mintFor(t, ws, "", &recordingKV{memoryKV: memoryKV{}})
		if err != nil || got.Credential != testPlatformKey || got.PinnedModel != DefaultDevModel {
			t.Fatalf("%s: mint = pin %q platformKey=%v err=%v", ws, got.PinnedModel, got.Credential == testPlatformKey, err)
		}
	}
}

// With the platform route off, a workspace behaves exactly as with no route:
// api.anthropic.com, its own BYO key, no pin, no model env — even when the
// session or request names the OpenRouter endpoint explicitly.
func TestPlatformModelRouteOffLeavesWorkspacesOnTheirOwnKey(t *testing.T) {
	if got, err := RegisteredModelEndpoint("tea-other", "claude", "", false); err != nil || got != "https://api.anthropic.com/v1" {
		t.Fatalf("endpoint = %q err=%v", got, err)
	}
	if _, err := RegisteredModelEndpoint("tea-other", "claude", openRouterEndpoint, false); err == nil {
		t.Fatal("an off-route workspace accepted the OpenRouter endpoint")
	}
	if ModelRouteEnv("tea-other", "claude", false) != nil || PlatformModelKeyReady("tea-other", false) {
		t.Fatal("route env or readiness without a route")
	}
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-other"): {ModelKeyField: "sk-ant-api03-BYO"}}}
	got, err := mintFor(t, "tea-other", "https://api.anthropic.com/v1", keys)
	if err != nil {
		t.Fatal(err)
	}
	if got != (ModelMintResponse{Credential: "sk-ant-api03-BYO", EndpointHost: "api.anthropic.com", Scheme: AuthSchemeAnthropicKey}) {
		t.Fatalf("mint differs from the no-route response: host %q scheme %q prefix %q pin %q",
			got.EndpointHost, got.Scheme, got.PathPrefix, got.PinnedModel)
	}
	if got, err := mintFor(t, "tea-other", openRouterEndpoint, keys); err == nil || got.Credential != "" {
		t.Fatalf("stored OpenRouter endpoint minted without a route (err=%v)", err)
	}
	if _, err := mintFor(t, "tea-other", "", &recordingKV{memoryKV: memoryKV{}}); !errors.Is(err, ErrModelKeyMissing) {
		t.Fatalf("err = %v, want ErrModelKeyMissing", err)
	}
}

// With both routes on (a dev control plane), the platform route wins for
// every workspace without its own key; a workspace with one keeps the dev
// route's BYO semantics.
func TestPlatformRouteTakesPrecedenceOverDevRoute(t *testing.T) {
	if err := EnableDevModelRoute(DevModelRouteOpenRouter, "deepseek/dev-model", "dev-10"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EnableDevModelRoute("", "", "") })
	enablePlatformRoute(t)
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-byo"): {ModelKeyField: "sk-or-v1-BYO"}}}
	got, err := mintWithSpend(t, "tea-byo", "", keys, &spendLedger{plan: store.PlanPro, byo: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "sk-or-v1-BYO" || got.EndpointHost != OpenRouterHost || got.PinnedModel != "" {
		t.Fatalf("dev-route mint = host %q pin %q platformKey=%v", got.EndpointHost, got.PinnedModel, got.Credential == testPlatformKey)
	}
	if ModelRouteEnv("tea-byo", "claude", true)["ANTHROPIC_MODEL"] != "deepseek/dev-model" ||
		ModelRouteEnv("tea-a", "claude", false)["ANTHROPIC_MODEL"] != DefaultDevModel {
		t.Fatal("route env did not follow the per-workspace route")
	}
	if got, err := mintFor(t, "tea-a", "", &recordingKV{memoryKV: memoryKV{}}); err != nil || got.Credential != testPlatformKey {
		t.Fatalf("platform mint under both routes: err=%v platformKey=%v", err, got.Credential == testPlatformKey)
	}
}

func mintWithSpend(t *testing.T, workspace, endpoint string, keys *recordingKV, spend SpendStore) (ModelMintResponse, error) {
	t.Helper()
	config, _ := json.Marshal(map[string]string{"agent": "claude", "modelEndpoint": endpoint})
	session := store.AgentSession{ID: "ags-one", WorkspaceID: workspace, SandboxID: "sbx-1", Phase: "running", AgentConfig: config}
	minter := &ModelMinter{Keys: keys, Sessions: fakeSessions{session: session}, Spend: spend}
	return minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: workspace, PodName: "sbx-1-0", PodUID: "uid-one"})
}

// ADR091 D7: a workspace that set its own key mints that key
// against the provider directly — no pinned model, so the gateway neither
// meters nor reports usage — and is never spend-capped, whatever endpoint
// the session stored (one created on the platform route keeps working).
func TestBYOKeyWinsOverThePlatformRouteAtMint(t *testing.T) {
	enablePlatformRoute(t)
	overCap := &spendLedger{plan: store.PlanPro, used: 1 << 40, byo: true}
	for _, stored := range []string{"https://api.anthropic.com/v1", openRouterEndpoint, ""} {
		keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "sk-ant-api03-BYO"}}}
		got, err := mintWithSpend(t, "tea-a", stored, keys, overCap)
		if err != nil {
			t.Fatalf("stored %q: %v (a BYO workspace over the cap is never capped)", stored, err)
		}
		if got.Credential != "sk-ant-api03-BYO" || got.EndpointHost != "api.anthropic.com" || got.Scheme != AuthSchemeAnthropicKey ||
			got.PinnedModel != "" || got.PathPrefix != "" {
			t.Fatalf("stored %q: BYO mint = host %q scheme %q pin %q prefix %q byo=%v",
				stored, got.EndpointHost, got.Scheme, got.PinnedModel, got.PathPrefix, got.Credential == "sk-ant-api03-BYO")
		}
	}
	if PlatformModelKeyReady("tea-a", true) || ModelRouteEnv("tea-a", "claude", true) != nil {
		t.Fatal("a BYO workspace is still on the platform route")
	}
	if _, ok := PlatformPinnedModel("tea-a", true); ok {
		t.Fatal("a BYO workspace is still pinned")
	}

	// Without the flag the same workspace is back on the platform key — and
	// its spend cap applies.
	overCap.byo = false
	keys := &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "sk-ant-api03-BYO"}}}
	if _, err := mintWithSpend(t, "tea-a", "", keys, overCap); !errors.Is(err, ErrModelSpendCapReached) {
		t.Fatalf("platform mint over the cap = %v", err)
	}
	got, err := mintWithSpend(t, "tea-a", "", keys, &spendLedger{plan: store.PlanPro})
	if err != nil || got.Credential != testPlatformKey || got.PinnedModel != DefaultDevModel || len(keys.reads) != 0 {
		t.Fatalf("platform mint = pin %q platformKey=%v err=%v BYO reads=%v", got.PinnedModel, got.Credential == testPlatformKey, err, keys.reads)
	}
}

// The flag is read from one cached spend read; a minter
// without that store, or a failed read, fails the mint closed rather than
// guessing which key to release. ForgetSpend makes this replica's next mint
// see a key write at once.
func TestMintKeyFlagLookup(t *testing.T) {
	enablePlatformRoute(t)
	ledger := &spendLedger{plan: store.PlanPro}
	if _, err := mintWithSpend(t, "tea-a", "", &recordingKV{memoryKV: memoryKV{}}, nil); !errors.Is(err, errSpendStoreUnwired) {
		t.Fatalf("platform-route mint without a spend store = %v, want it refused", err)
	}
	ledger.err = errors.New("db down")
	if _, err := mintWithSpend(t, "tea-a", "", &recordingKV{memoryKV: memoryKV{}}, ledger); err == nil {
		t.Fatal("an unreadable BYO flag minted a credential")
	}

	ledger.err = nil
	config, _ := json.Marshal(map[string]string{"agent": "claude"})
	minter := &ModelMinter{
		Keys:     &recordingKV{memoryKV: memoryKV{ModelKeySecretPath("tea-a"): {ModelKeyField: "sk-ant-api03-BYO"}}},
		Sessions: fakeSessions{session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running", AgentConfig: config}},
		Spend:    ledger,
	}
	req := ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "sbx-1-0", PodUID: "uid-one"}
	if got, err := minter.Mint(context.Background(), req); err != nil || got.Credential != testPlatformKey {
		t.Fatalf("platform mint = %v", err)
	}
	ledger.byo = true
	if got, _ := minter.Mint(context.Background(), req); got.Credential != testPlatformKey {
		t.Fatal("the cached status was not used within its TTL")
	}
	minter.ForgetSpend("tea-a")
	if got, err := minter.Mint(context.Background(), req); err != nil || got.Credential != "sk-ant-api03-BYO" {
		t.Fatalf("mint after ForgetSpend = %v, want the BYO key", err)
	}
}
