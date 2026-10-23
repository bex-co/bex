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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

const platformKey = "sk-or-v1-PLATFORM-SECRET"

func requestBody(t *testing.T, proxy *httptest.Server, body string) *http.Response {
	t.Helper()
	base, err := agentsession.ModelProxyURL(proxy.URL, "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages?beta=true", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", agentsession.ModelKeyPlaceholder("ags-one"))
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// TestProxyPlatformRoutePinsTheModel: on the workspace-allowlisted platform
// route (ADR090) the gateway injects the platform key only for a request whose
// top-level "model" is the pinned id, forwards that body byte for byte, and
// refuses every other model (or an ambiguous body) before reaching upstream.
func TestProxyPlatformRoutePinsTheModel(t *testing.T) {
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", []string{"tea-a"}, platformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
	session := store.AgentSession{
		ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
		AgentConfig: json.RawMessage(`{"agent":"claude","modelEndpoint":"https://api.anthropic.com/v1"}`),
	}
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, session, "sk-ant-api03-BYO")

	pinned := `{"model":"` + agentsession.DefaultDevModel + `","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	resp := requestBody(t, proxy, pinned)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || capture.last == nil {
		t.Fatalf("pinned model status = %d, want 200 upstream", resp.StatusCode)
	}
	got := capture.last
	if got.URL.Host != "openrouter.ai" || got.URL.Path != "/api/v1/messages" {
		t.Fatalf("upstream = %s", got.URL)
	}
	if got.Header.Get("Authorization") != "Bearer "+platformKey || len(got.Header.Values("X-Api-Key")) != 0 {
		t.Fatal("upstream auth is not the platform Bearer key")
	}
	forwarded, _ := io.ReadAll(got.Body)
	if string(forwarded) != pinned {
		t.Fatalf("forwarded body = %q, want the original bytes", forwarded)
	}

	for name, body := range map[string]string{
		"other model":     `{"model":"anthropic/claude-opus-4.5","messages":[]}`,
		"no model":        `{"messages":[]}`,
		"duplicate model": `{"model":"` + agentsession.DefaultDevModel + `","model":"anthropic/claude-opus-4.5"}`,
		"fallback models": `{"model":"` + agentsession.DefaultDevModel + `","models":["anthropic/claude-opus-4.5"]}`,
		"paid plugins":    `{"model":"` + agentsession.DefaultDevModel + `","plugins":[{"id":"web"}]}`,
		"non-string":      `{"model":["` + agentsession.DefaultDevModel + `"]}`,
		"array body":      `[{"model":"` + agentsession.DefaultDevModel + `"}]`,
		"trailing data":   `{"model":"` + agentsession.DefaultDevModel + `"}{"model":"x"}`,
		"not json":        `model=` + agentsession.DefaultDevModel,
	} {
		capture.last = nil
		resp := requestBody(t, proxy, body)
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden || capture.last != nil {
			t.Fatalf("%s: status = %d upstream=%v, want 403 with no upstream request", name, resp.StatusCode, capture.last != nil)
		}
		if !strings.Contains(string(msg), "model not allowed") || strings.Contains(string(msg), platformKey) {
			t.Fatalf("%s: refusal message = %q", name, msg)
		}
	}
}

// TestProxyPlatformRouteLeavesOtherWorkspacesOnTheirByoKey: with the platform
// route allowlisting another workspace, tea-a's claude call goes to
// api.anthropic.com with its own key and no model pin applies.
func TestProxyPlatformRouteLeavesOtherWorkspacesOnTheirByoKey(t *testing.T) {
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", []string{"tea-z"}, platformKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-BYO")
	resp := requestBody(t, proxy, `{"model":"claude-opus-4-5"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || capture.last == nil {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if capture.last.URL.Host != "api.anthropic.com" || capture.last.Header.Get("X-Api-Key") != "sk-ant-api03-BYO" || capture.last.Header.Get("Authorization") != "" {
		t.Fatalf("upstream = %s with non-BYO auth", capture.last.URL)
	}
	// A session that stored the OpenRouter endpoint gets nothing at all.
	session := store.AgentSession{
		ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
		AgentConfig: json.RawMessage(`{"agent":"claude","modelEndpoint":"https://openrouter.ai/api"}`),
	}
	proxy, capture, _ = harness(t, fakePods{pod: sandboxPod("ags-one")}, session, "sk-ant-api03-BYO")
	resp = requestBody(t, proxy, `{"model":"`+agentsession.DefaultDevModel+`"}`)
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || capture.last != nil {
		t.Fatalf("status = %d upstream=%v, want a refusal", resp.StatusCode, capture.last != nil)
	}
}
