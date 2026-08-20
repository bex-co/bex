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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func scopedAPI(t *testing.T, sub, clientID, scope string, aud []string, platform map[string]bool) (http.Handler, *Server, *fakeAuditSink) {
	t.Helper()
	sink := &fakeAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Audit: sink}
	hydra := newClassHydraScoped(t, sub, clientID, scope, aud, platform)
	srv := NewServer(base, Deps{})
	srv.HydraAdminURL = hydra.url
	srv.OAuthResource = bexResource
	srv.OAuthRequireAudience = true
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h, srv, sink
}

func auditOfVerb(events []core.AuditEvent, verb string) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range events {
		if ev.Verb == verb {
			out = append(out, ev)
		}
	}
	return out
}

func assertRESTInsufficientScope(t *testing.T, w *httptest.ResponseRecorder, wantCap string) {
	t.Helper()
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s, want 403", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v (%s)", err, w.Body.String())
	}
	if body["code"] != core.InsufficientScopeCode {
		t.Errorf("code = %v, want %s", body["code"], core.InsufficientScopeCode)
	}
	if body["message"] == "" || body["error"] == "" || body["id"] == "" {
		t.Errorf("Render dialect missing error/message/id: %v", body)
	}
	params, _ := body["params"].(map[string]any)
	if params["required"] != wantCap {
		t.Errorf("params.required = %v, want %s", params["required"], wantCap)
	}
}

func assertGQLInsufficientScope(t *testing.T, w *httptest.ResponseRecorder, wantCap string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("graphql status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Errors []struct {
			Message    string         `json:"message"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v (%s)", err, w.Body.String())
	}
	if len(body.Errors) == 0 {
		t.Fatalf("want GraphQL errors, got %s", w.Body.String())
	}
	ext := body.Errors[0].Extensions
	if ext["code"] != core.InsufficientScopeCode {
		t.Errorf("extensions.code = %v, want %s", ext["code"], core.InsufficientScopeCode)
	}
	if ext["required"] != wantCap {
		t.Errorf("extensions.required = %v, want %s", ext["required"], wantCap)
	}
}

func TestScopeClassEnforcementReadToken(t *testing.T) {
	h, srv, sink := scopedAPI(t, "identity-1", "dcr-client", "openid "+core.ScopeRead,
		[]string{bexResource}, map[string]bool{"dcr-client": false})

	if got := do(t, h, http.MethodGet, "/v1/services", testToken, "").Code; got != http.StatusOK {
		t.Fatalf("GET /v1/services = %d, want 200", got)
	}
	if n := len(auditOfVerb(sink.events, core.AuditVerbScopeClass)); n != 0 {
		t.Fatalf("allowed read must not audit ScopeClass, got %d", n)
	}

	assertRESTInsufficientScope(t, do(t, h, http.MethodPost, "/v1/services/web/suspend", testToken, ""), core.ScopeWrite)
	assertRESTInsufficientScope(t, do(t, h, http.MethodPost, "/v1/api-keys", testToken, `{"name":"x"}`), core.ScopeWrite)
	assertRESTInsufficientScope(t, do(t, h, http.MethodGet, "/v1/services/web/env-vars", testToken, ""), core.ScopeSensitive)

	refusals := auditOfVerb(sink.events, core.AuditVerbScopeClass)
	if len(refusals) != 3 {
		t.Fatalf("scope-class refusals = %d, want 3", len(refusals))
	}
	for _, ev := range refusals {
		if ev.Outcome != core.AuditDenied {
			t.Errorf("outcome = %s, want denied", ev.Outcome)
		}
		if ev.OAuthClientID != "dcr-client" || ev.OAuthAudience != bexResource {
			t.Errorf("oauth provenance = %+v", ev)
		}
		if !slices.Contains(ev.OAuthScopes, core.ScopeRead) {
			t.Errorf("scopes = %v, want bex.read", ev.OAuthScopes)
		}
		if ev.Caller != "identity-1" || ev.CallerMethod != "oauth2" {
			t.Errorf("caller = %s/%s", ev.Caller, ev.CallerMethod)
		}
		if strings.Contains(strings.ToLower(ev.Target), "bearer") || strings.Contains(ev.Target, testToken) {
			t.Errorf("audit target leaked token material: %q", ev.Target)
		}
		if ev.Relation != "" {
			t.Errorf("typed system event must leave relation empty, got %q", ev.Relation)
		}
	}

	w := do(t, h, http.MethodPost, "/graphql", testToken, `{"query":"{ services { id } }"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("read query status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), core.InsufficientScopeCode) {
		t.Fatalf("read query refused: %s", w.Body.String())
	}
	assertGQLInsufficientScope(t, do(t, h, http.MethodPost, "/graphql", testToken,
		`{"query":"mutation { suspendService(id:\"web\") { id } }"}`), core.ScopeWrite)

	cs := mcpSessionIdentity(t, srv, core.Identity{
		Subject: "identity-1", Method: "oauth2", ClientID: "dcr-client", Human: true,
		CanonicalScopes: core.ScopeRead, AcceptedAudience: bexResource,
	})
	list, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_services"})
	if err != nil || list.IsError {
		t.Fatalf("list_services: %v isErr=%v", err, list != nil && list.IsError)
	}
	suspend, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "suspend_service", Arguments: map[string]any{"serviceId": "web"},
	})
	if err != nil {
		t.Fatalf("suspend_service transport: %v", err)
	}
	if suspend == nil || !suspend.IsError {
		t.Fatal("suspend_service must be a tool error")
	}
	if !strings.Contains(fmtMCP(suspend), core.InsufficientScopeCode) {
		t.Errorf("MCP error = %s, want INSUFFICIENT_SCOPE", fmtMCP(suspend))
	}
}

func TestScopeClassEnforcementWriteToken(t *testing.T) {
	h, _, _ := scopedAPI(t, "identity-1", "dcr-client", "openid "+core.ScopeWrite,
		[]string{bexResource}, map[string]bool{"dcr-client": false})

	if got := do(t, h, http.MethodPost, "/v1/services/web/suspend", testToken, "").Code; got != http.StatusAccepted {
		t.Fatalf("suspend with bex.write = %d, want 202", got)
	}
	assertRESTInsufficientScope(t, do(t, h, http.MethodGet, "/v1/services/web/env-vars", testToken, ""), core.ScopeSensitive)
	// Mint class is write at dispatch; AuthorizeMintClass still refuses a
	// third-party client (plain 403, not INSUFFICIENT_SCOPE).
	w := do(t, h, http.MethodPost, "/v1/api-keys", testToken, `{"name":"x"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("third-party write token mint = %d, want 403", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] == core.InsufficientScopeCode {
		t.Errorf("AuthorizeMintClass must stay a plain forbidden, got INSUFFICIENT_SCOPE: %v", body)
	}
}

func TestScopeClassExemptions(t *testing.T) {
	t.Run("api key", func(t *testing.T) {
		h, _, sink := scopedAPI(t, "key-1", "key-1", "", []string{bexResource}, map[string]bool{})
		if got := do(t, h, http.MethodPost, "/v1/services/web/suspend", testToken, "").Code; got != http.StatusAccepted {
			t.Fatalf("api-key suspend = %d, want 202", got)
		}
		if n := len(auditOfVerb(sink.events, core.AuditVerbScopeClass)); n != 0 {
			t.Fatalf("exempt api key audited ScopeClass %d times", n)
		}
		w := do(t, h, http.MethodPost, "/v1/api-keys", testToken, `{"name":"x"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("api-key self-mint = %d, want 403 (AuthorizeMintClass)", w.Code)
		}
	})
	t.Run("platform client without granular grant", func(t *testing.T) {
		h, _, _ := scopedAPI(t, "identity-1", "bex-mobile", identityScopes,
			[]string{bexResource}, map[string]bool{"bex-mobile": true})
		if got := do(t, h, http.MethodPost, "/v1/services/web/suspend", testToken, "").Code; got != http.StatusAccepted {
			t.Fatalf("platform suspend = %d, want 202", got)
		}
	})
	t.Run("audience-less device flow", func(t *testing.T) {
		h, _, _ := scopedAPI(t, "identity-1", "render-cli", identityScopes,
			nil, map[string]bool{"render-cli": true})
		if got := do(t, h, http.MethodPost, "/v1/services/web/suspend", testToken, "").Code; got != http.StatusAccepted {
			t.Fatalf("device-flow suspend = %d, want 202", got)
		}
	})
}

func TestGraphQLTopLevelOps(t *testing.T) {
	got := graphqlTopLevelOps("query { services { id } envVars { key } }", "")
	if strings.Join(got, ",") != "GQL Query.services,GQL Query.envVars" {
		t.Errorf("got %v", got)
	}
	got = graphqlTopLevelOps("mutation Foo { suspendService(id:\"web\") { id } } query Bar { services { id } }", "Foo")
	if strings.Join(got, ",") != "GQL Mutation.suspendService" {
		t.Errorf("named mutation = %v", got)
	}
	got = graphqlTopLevelOps("query { __schema { types { name } } services { id } }", "")
	if strings.Join(got, ",") != "GQL Query.services" {
		t.Errorf("introspection skipped = %v", got)
	}
}

func fmtMCP(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	b, _ := json.Marshal(res.Content)
	return string(b)
}
