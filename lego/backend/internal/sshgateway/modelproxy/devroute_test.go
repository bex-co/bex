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

package modelproxy_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestProxyDevOpenRouterRouteInjectsBearerAndDropsPlaceholderKey pins the
// ADR090 dev route end to end through the real mint: Claude Code's
// /v1/messages lands at https://openrouter.ai/api/v1/messages with the BYO key
// as a Bearer token, and the x-api-key placeholder Claude Code always sends is
// never forwarded.
func TestProxyDevOpenRouterRouteInjectsBearerAndDropsPlaceholderKey(t *testing.T) {
	if err := agentsession.EnableDevModelRoute(agentsession.DevModelRouteOpenRouter, "", "dev-10"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnableDevModelRoute("", "", "") })
	session := store.AgentSession{
		ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
		AgentConfig: json.RawMessage(`{"agent":"claude","modelEndpoint":"https://openrouter.ai/api"}`),
	}
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, session, "sk-or-v1-REAL")
	resp := request(t, proxy, "ags-one", "/v1/messages?beta=true", "X-Api-Key", agentsession.ModelKeyPlaceholder("ags-one"))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got := capture.last
	if got == nil {
		t.Fatal("no upstream request captured")
	}
	if got.URL.Scheme != "https" || got.URL.Host != "openrouter.ai" || got.URL.Path != "/api/v1/messages" {
		t.Fatalf("upstream url = %s, want https://openrouter.ai/api/v1/messages", got.URL)
	}
	if auth := got.Header.Get("Authorization"); auth != "Bearer sk-or-v1-REAL" {
		t.Fatalf("Authorization = %q, want the Bearer BYO key", auth)
	}
	if key := got.Header.Values("X-Api-Key"); len(key) != 0 {
		t.Fatalf("x-api-key forwarded on the Bearer route: %q", key)
	}
}

// TestProxyRefusesOpenRouterWithoutTheDevRoute: with the route off, a session
// whose stored config names OpenRouter cannot mint, so nothing reaches it.
func TestProxyRefusesOpenRouterWithoutTheDevRoute(t *testing.T) {
	_ = agentsession.EnableDevModelRoute("", "", "")
	session := store.AgentSession{
		ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
		AgentConfig: json.RawMessage(`{"agent":"claude","modelEndpoint":"https://openrouter.ai/api"}`),
	}
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, session, "sk-or-v1-REAL")
	resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || capture.last != nil {
		t.Fatalf("status = %d upstream=%v, want a refusal with no upstream request", resp.StatusCode, capture.last)
	}
}
