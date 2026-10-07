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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// codedRefusal is what one refusal answers on every surface: REST's status,
// body code and params, GraphQL's extensions (params flattened) and MCP's
// "CODE: " text. An empty msg checks the code and params without the wording.
type codedRefusal struct {
	status int
	code   string
	msg    string
	params map[string]any
}

func (want codedRefusal) onREST(t *testing.T, h http.Handler, method, path, body string) {
	t.Helper()
	res := do(t, h, method, path, testToken, body)
	var got struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Params  map[string]any `json:"params"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil || res.Code != want.status || got.Code != want.code ||
		!want.says(got.Message) || !want.carries(got.Params) {
		t.Fatalf("%s %s = %d %s, want %d %s %q %v", method, path, res.Code, res.Body, want.status, want.code, want.msg, want.params)
	}
}

func (want codedRefusal) onGraphQL(t *testing.T, h http.Handler, query string) {
	t.Helper()
	_, errs, body := rawGraphQL(t, h, query)
	if len(errs) != 1 {
		t.Fatalf("errors = %s, want one", body)
	}
	gqlErr, _ := errs[0].(map[string]any)
	extensions, _ := gqlErr["extensions"].(map[string]any)
	message, _ := gqlErr["message"].(string)
	if extensions["code"] != want.code || !want.says(message) || !want.carries(extensions) {
		t.Fatalf("GraphQL error = %s, want %s %q %v", body, want.code, want.msg, want.params)
	}
}

func (want codedRefusal) onMCP(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil || !res.IsError || len(res.Content) == 0 {
		t.Fatalf("%s = %s, %v; want a tool error", tool, fmtMCP(res), err)
	}
	text, _ := res.Content[0].(*mcp.TextContent)
	if text == nil || !strings.HasPrefix(text.Text, want.code+": ") || !want.says(strings.TrimPrefix(text.Text, want.code+": ")) {
		t.Fatalf("%s error = %s, want %s: %q", tool, fmtMCP(res), want.code, want.msg)
	}
}

func (want codedRefusal) says(msg string) bool { return want.msg == "" || msg == want.msg }

func (want codedRefusal) carries(got map[string]any) bool {
	for key, value := range want.params {
		if got[key] != value {
			return false
		}
	}
	return true
}

// TestTheDashboardsRefusalsAreCodedOnEverySurface (w5/m128, w5/m130): the
// refusals the dashboard branches on carry one code on REST, GraphQL and MCP, so
// rewording a message can no longer change what the dashboard shows. Render CLI
// users read the messages, so they are unchanged, except that a coded refusal
// drops the sentinel prefix every coded error drops (`bad request: `,
// `conflict: `), and MCP's text gains the `CODE: ` prefix. The protected
// refusal is made in two places, the service guard and the datastores', and
// both are pinned.
func TestTheDashboardsRefusalsAreCodedOnEverySurface(t *testing.T) {
	unavailable := func(code, msg string) codedRefusal {
		return codedRefusal{status: http.StatusServiceUnavailable, code: code, msg: msg}
	}
	protected := func(name, confirm string) codedRefusal {
		return codedRefusal{
			status: http.StatusBadRequest, code: "PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED",
			msg:    `"` + name + `" is a member of a protected environment; retry with confirm="` + confirm + `" to suspend it`,
			params: map[string]any{"confirm": confirm, "verb": "suspend", "name": name},
		}
	}
	for _, tc := range []struct {
		name         string
		deny         bool          // the checker refuses every relation
		overQuota    bool          // the workspace already holds its 25 services
		setup        func(*Server) // state the refusal needs, set after wiring
		method, path string
		body         string
		want         codedRefusal
		query, tool  string
		args         map[string]any
	}{{
		name: "pod-log source missing", method: http.MethodGet, path: "/v1/logs?resource=web",
		want:  unavailable("LOGS_UNAVAILABLE", "logs source not configured"),
		query: `{ logs(resource: "web") { logs { timestamp } } }`,
		tool:  "list_logs", args: map[string]any{"resource": []string{"web"}},
	}, {
		name: "durable log store missing", method: http.MethodGet, path: "/v1/logs/values?resource=web&label=level",
		want:  unavailable("LOG_STORE_UNAVAILABLE", "request logs and structured log filters require the durable log store"),
		query: `{ logLabelValues(resource: "web", label: "level") }`,
		tool:  "list_log_label_values", args: map[string]any{"resource": []string{"web"}, "label": "level"},
	}, {
		name: "metrics source missing", method: http.MethodGet, path: "/v1/metrics/memory?resource=web",
		want:  unavailable("METRICS_UNAVAILABLE", "metrics source not configured"),
		query: `{ metrics(query: {name: "MEMORY", filters: [{field: "RESOURCE", values: ["web"]}]}) { unit } }`,
		tool:  "get_metrics", args: map[string]any{"resourceId": "web", "metricTypes": []string{"memory_usage"}},
	}, {
		name: "secret store missing", method: http.MethodGet, path: "/v1/services/web/env-vars",
		want:  unavailable("SECRETS_UNAVAILABLE", "secret store not configured"),
		query: `{ envVars(serviceId: "web") { cursor } }`,
		tool:  "list_env_vars", args: map[string]any{"serviceId": "web"},
	}, {
		name: "audit log store missing", method: http.MethodGet, path: "/v1/owners/tea-a/audit-logs",
		want:  unavailable("AUDIT_LOG_UNAVAILABLE", "audit log store not configured"),
		query: `{ auditLogs(ownerId: "tea-a") { id } }`,
	}, {
		name: "not found", method: http.MethodGet, path: "/v1/services/missing",
		want:  codedRefusal{status: http.StatusNotFound, code: "NOT_FOUND", msg: "not found"},
		query: `{ service(id: "missing") { id } }`,
		tool:  "get_service", args: map[string]any{"serviceId": "missing"},
	}, {
		name: "a named resource not found", method: http.MethodGet, path: "/v1/services/web/custom-domains/shop.example.com",
		want:  codedRefusal{status: http.StatusNotFound, code: "NOT_FOUND", msg: "custom domain not found"},
		query: `{ customDomain(id: "web", name: "shop.example.com") { name } }`,
		tool:  "get_custom_domain", args: map[string]any{"serviceId": "web", "name": "shop.example.com"},
	}, {
		name: "forbidden", deny: true, method: http.MethodGet, path: "/v1/services",
		want:  codedRefusal{status: http.StatusForbidden, code: "FORBIDDEN", msg: "forbidden"},
		query: `{ services { id } }`,
		tool:  "list_services", args: map[string]any{},
	}, {
		name:   "protected service",
		setup:  func(srv *Server) { srv.Apps.Store = protectedServices{status: core.ProtectedStatusProtected} },
		method: http.MethodPost, path: "/v1/services/web/suspend",
		want:  protected("web", "sudo suspend service web"),
		query: `mutation { suspendService(id: "web") { id } }`,
		tool:  "suspend_service", args: map[string]any{"serviceId": "web"},
	}, {
		name: "protected Postgres",
		setup: func(srv *Server) {
			srv.Postgres.Protection = protectedEnvironments{"env-1": core.ProtectedStatusProtected}
		},
		method: http.MethodPost, path: "/v1/postgres/dpg-orders/suspend",
		want:  protected("orders", "sudo suspend database orders"),
		query: `mutation { suspendDatabase(id: "dpg-orders") { id } }`,
		tool:  "suspend_postgres", args: map[string]any{"postgresId": "dpg-orders"},
	}, {
		name: "Postgres name taken", method: http.MethodPost, path: "/v1/postgres", body: `{"name":"orders","plan":"free"}`,
		want: codedRefusal{
			status: http.StatusConflict, code: "CONFLICT",
			msg: `a Postgres database named "orders" already exists in this workspace`,
		},
		query: `mutation { createDatabase(name: "orders", plan: "free") { id } }`,
		tool:  "create_postgres", args: map[string]any{"name": "orders", "plan": "free"},
	}, {
		name: "Key Value name taken", method: http.MethodPost, path: "/v1/key-value", body: `{"name":"cache","plan":"free"}`,
		want: codedRefusal{
			status: http.StatusConflict, code: "CONFLICT",
			msg: `a key-value store named "cache" already exists in this workspace`,
		},
		query: `mutation { createKeyValue(name: "cache", plan: "free") { id } }`,
		tool:  "create_key_value", args: map[string]any{"name": "cache", "plan": "free"},
	}, {
		name: "workspace at its service cap", overQuota: true,
		method: http.MethodPost, path: "/v1/services", body: `{"name":"web2","ownerId":"default","image":{"ownerId":"default","imagePath":"nginx"}}`,
		want: codedRefusal{
			status: http.StatusBadRequest, code: "WORKSPACE_RESOURCE_LIMIT",
			msg: "workspace is limited to 25 services; delete an existing service to create another", params: map[string]any{"limit": float64(25)},
		},
		query: `mutation { createService(name: "web2", image: "nginx") { id } }`,
		tool:  "create_web_service", args: map[string]any{"name": "web2", "image": "nginx", "runtime": "image"},
	}, {
		name: "GitHub integration unconfigured", method: http.MethodGet, path: "/v1/git/connections?ownerId=default",
		want:  unavailable("GITHUB_UNAVAILABLE", "github integration not configured"),
		query: `{ gitConnections(ownerId: "default") { installationId } }`,
		tool:  "list_git_connections", args: map[string]any{},
	}, {
		name: "web shell unconfigured", method: http.MethodPost, path: "/v1/services/web/shell-ticket",
		want:  unavailable("SHELL_UNAVAILABLE", "web shell transport not configured"),
		query: `mutation { createShellSession(id: "web") { ticket } }`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			web := sampleApp("web")
			web.Labels = map[string]string{core.LabelAppID: "srv-web", store.LabelManagedBy: store.ManagedByValue}
			orders := &appv1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: "default", Labels: map[string]string{core.LabelEnvironment: "env-1"}},
				Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "free"},
			}
			cache := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-cache", Namespace: "default"},
				Spec:       appv1alpha1.KeyValueSpec{Name: "cache", Plan: "free"},
			}
			adm := &admission{}
			if tc.overQuota {
				adm.limits = map[string]int{store.AppsQuotaCountKey: 25}
				adm.used = map[string]int{store.AppsQuotaCountKey: 25}
			}
			base := &core.Base{Client: adm.client(web, orders, cache), Namespace: "default", Authz: &fakeChecker{allow: !tc.deny}}
			// Every other source is simply left nil; the audit log is mounted
			// only when its service is, so it is wired without a store.
			h, srv := serverWith(t, base, Deps{Audit: &audit.Service{Base: base}})
			if tc.setup != nil {
				tc.setup(srv)
			}

			t.Run("REST", func(t *testing.T) { tc.want.onREST(t, h, tc.method, tc.path, tc.body) })
			t.Run("GraphQL", func(t *testing.T) { tc.want.onGraphQL(t, h, tc.query) })
			t.Run("MCP", func(t *testing.T) {
				if tc.tool == "" {
					t.Skip("no MCP tool makes this refusal")
				}
				tc.want.onMCP(t, mcpSessionAs(t, srv, "dana"), tc.tool, tc.args)
			})
		})
	}
}
