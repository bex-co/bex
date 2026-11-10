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

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Sandbox flags the pinned Render CLI v2.35 sends (w2/m263): each one does what
// Render documents, or is refused by name before any OpenSandbox call.

// recordingUpstream answers creates like OpenSandbox and records the body.
func recordingUpstream(t *testing.T) (*Service, *createRequest) {
	t.Helper()
	got := &createRequest{}
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(got); err != nil {
			t.Errorf("decode create: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Running"}}`))
	})
	return svc, got
}

func refusingUpstream(t *testing.T) *Service {
	t.Helper()
	return stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a refused create reached OpenSandbox: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestPlanSizesTheSandbox(t *testing.T) {
	for _, tc := range []struct {
		plan      Plan
		cpu, mem  string
		weightMil string
	}{
		{Plan1C2G, "1", "2Gi", "1211"},
		{Plan2C4G, "2", "4Gi", "2422"},
		{Plan4C8G, "4", "8Gi", "4845"},
		{Plan8C16G, "8", "16Gi", "9689"},
		// Deprecated aliases keep Render's sizes: starter is the 0.5 CPU
		// rung, standard ≡ 1c-2g, pro ≡ 2c-4g.
		{PlanStarter, "500m", "512Mi", "553"},
		{PlanStandard, "1", "2Gi", "1211"},
		{PlanPro, "2", "4Gi", "2422"},
	} {
		t.Run(string(tc.plan), func(t *testing.T) {
			svc, got := recordingUpstream(t)
			sb, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: tc.plan})
			if err != nil {
				t.Fatal(err)
			}
			if got.ResourceLimits.CPU != tc.cpu || got.ResourceLimits.Memory != tc.mem {
				t.Errorf("limits = %+v, want %s/%s", got.ResourceLimits, tc.cpu, tc.mem)
			}
			// Billing follows the resources (weighted compute seconds), so a
			// larger plan accrues proportionally with no pricing entry of its own.
			if got.Metadata[metadataComputeWeight] != tc.weightMil {
				t.Errorf("compute weight = %s, want %s", got.Metadata[metadataComputeWeight], tc.weightMil)
			}
			if sb.Plan != tc.plan || got.Metadata[metadataPlan] != string(tc.plan) {
				t.Errorf("plan echo = %q / metadata %q, want %q", sb.Plan, got.Metadata[metadataPlan], tc.plan)
			}
		})
	}
}

func TestOmittedPlanKeepsTheTemplateSize(t *testing.T) {
	svc, got := recordingUpstream(t)
	svc.Templates["agent"] = Template{Image: "agent:1", Entrypoint: []string{"sh"}, CPU: "2", Memory: "4Gi"}
	svc.DefaultPlan = PlanStarter
	sb, err := svc.Create(callerCtx(), CreateRequest{Template: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ResourceLimits.CPU != "2" || got.ResourceLimits.Memory != "4Gi" || sb.Plan != PlanStarter {
		t.Errorf("limits = %+v plan %q, want the template's 2/4Gi labelled with the default plan", got.ResourceLimits, sb.Plan)
	}
}

func TestUnknownPlanIsRefusedWithTheAcceptedNames(t *testing.T) {
	_, err := refusingUpstream(t).Create(callerCtx(), CreateRequest{Template: "node", Plan: "16c-32g"})
	wantCode(t, err, "SANDBOX_PLAN_INVALID")
	for _, name := range []string{"1c-2g", "2c-4g", "4c-8g", "8c-16g", "starter", "standard", "pro"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("message %q does not list %s", err.Error(), name)
		}
	}
}

// TestIdleTimeout pins Render's idleTimeoutSeconds contract (w2/m278): an
// omitted value is 900 s, 0 disables idle suspension, 300–86400 is honored,
// and the value is stamped where the idle sweeper reads it and reported back.
func TestIdleTimeout(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       string
	}{
		{"omitted defaults to 900", `{"template":"node"}`, "900"},
		{"zero disables idle suspension", `{"template":"node","idleTimeoutSeconds":0}`, "0"},
		{"minimum", `{"template":"node","idleTimeoutSeconds":300}`, "300"},
		{"maximum", `{"template":"node","idleTimeoutSeconds":86400}`, "86400"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, got := recordingUpstream(t)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes",
				strings.NewReader(tc.body)).WithContext(callerCtx()))
			if rec.Code != http.StatusCreated {
				t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
			}
			if got.Metadata[metadataIdleTimeout] != tc.want {
				t.Errorf("stamped idle timeout = %q, want %s", got.Metadata[metadataIdleTimeout], tc.want)
			}
			if !strings.Contains(rec.Body.String(), `"idleTimeoutSeconds":`+tc.want+`,`) {
				t.Errorf("detail %s does not report idleTimeoutSeconds %s", rec.Body.String(), tc.want)
			}
		})
	}
	for _, idle := range []int{299, 86401, -1, 1} {
		_, err := refusingUpstream(t).Create(callerCtx(), CreateRequest{Template: "node", IdleTimeoutSeconds: &idle})
		wantCode(t, err, "SANDBOX_IDLE_TIMEOUT_INVALID")
	}
}

// TestIdleTimeoutAgreesAcrossGraphQLAndMCP: GraphQL and MCP create through
// the same resolve, so an omitted value is 900 and an explicit one is honored
// and reported on both.
func TestIdleTimeoutAgreesAcrossGraphQLAndMCP(t *testing.T) {
	svc, got := recordingUpstream(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ args, want string }{
		{`template: "node"`, "900"},
		{`template: "node", idleTimeoutSeconds: 600`, "600"},
		{`template: "node", idleTimeoutSeconds: 0`, "0"},
	} {
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: `mutation { createSandbox(` + tc.args + `) { id idleTimeoutSeconds } }`,
			Context:       callerCtx(),
		})
		if len(res.Errors) != 0 {
			t.Fatalf("GraphQL %s: %#v", tc.args, res.Errors)
		}
		raw, _ := json.Marshal(res.Data)
		if !strings.Contains(string(raw), `"idleTimeoutSeconds":`+tc.want) || got.Metadata[metadataIdleTimeout] != tc.want {
			t.Errorf("GraphQL %s = %s (stamped %q), want %s", tc.args, raw, got.Metadata[metadataIdleTimeout], tc.want)
		}
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createSandbox(template: "node", idleTimeoutSeconds: 100) { id } }`,
		Context:       callerCtx(),
	})
	if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != "SANDBOX_IDLE_TIMEOUT_INVALID" {
		t.Fatalf("GraphQL refusal = %#v", res.Errors)
	}

	client := mcpClient(t, svc)
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"template": "node"}, "900"},
		{map[string]any{"template": "node", "idleTimeoutSeconds": 1800}, "1800"},
		{map[string]any{"template": "node", "idleTimeoutSeconds": 0}, "0"},
	} {
		toolRes, err := client.CallTool(callerCtx(), &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: tc.args})
		if err != nil || toolRes.IsError {
			t.Fatalf("MCP %v: %v %#v", tc.args, err, toolRes)
		}
		raw, _ := json.Marshal(toolRes.StructuredContent)
		if !strings.Contains(string(raw), `"idleTimeoutSeconds":`+tc.want) || got.Metadata[metadataIdleTimeout] != tc.want {
			t.Errorf("MCP %v = %s (stamped %q), want %s", tc.args, raw, got.Metadata[metadataIdleTimeout], tc.want)
		}
	}
	toolRes, err := client.CallTool(callerCtx(), &mcp.CallToolParams{
		Name: "spawn_sandbox", Arguments: map[string]any{"template": "node", "idleTimeoutSeconds": 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !toolRes.IsError || !strings.Contains(toolRes.Content[0].(*mcp.TextContent).Text, "SANDBOX_IDLE_TIMEOUT_INVALID") {
		t.Fatalf("MCP refusal = %#v", toolRes.Content)
	}
}

func mcpClient(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := callerCtx()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestImageCreateFieldsAreRefusedByName(t *testing.T) {
	for _, tc := range []struct {
		name, body, code, flag string
	}{
		{"--image by id", `{"buildId":"sbb-aaaaaaaaaaaaaaaaaaaa"}`, CodeSandboxImagesUnsupported, "--image"},
		{"--image by name", `{"buildName":"python"}`, CodeSandboxImagesUnsupported, "--image"},
		{"raw image ref", `{"image":"python:3.13"}`, CodeSandboxImagesUnsupported, "--image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			refusingUpstream(t).RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(tc.body)).WithContext(callerCtx()))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.code) || !strings.Contains(rec.Body.String(), tc.flag) {
				t.Fatalf("= %d %s, want 400 %s naming %s", rec.Code, rec.Body.String(), tc.code, tc.flag)
			}
			if strings.Contains(rec.Body.String(), "unknown field") {
				t.Fatalf("refusal leaks the wire key: %s", rec.Body.String())
			}
		})
	}
}

// listUpstream lists one sandbox per status, all owned by the caller.
func listUpstream(t *testing.T) *Service {
	t.Helper()
	return stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			return
		}
		var items []map[string]any
		for _, state := range []string{"Running", "Paused", "Creating", "Terminated"} {
			items = append(items, map[string]any{
				"id": "os-" + strings.ToLower(state),
				"metadata": map[string]string{
					metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
					metadataNetworkPolicy: string(NetworkPolicyDenyAll),
				},
				"status": map[string]string{"state": state},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
}

func listStatuses(t *testing.T, mux *http.ServeMux, query string) (int, []string, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sandboxes"+query, nil).WithContext(callerCtx()))
	var envelopes []SandboxEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &envelopes)
	var out []string
	for _, e := range envelopes {
		out = append(out, string(e.Sandbox.Status))
	}
	return rec.Code, out, rec.Body.String()
}

func TestListNeverIgnoresAFilter(t *testing.T) {
	svc := listUpstream(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// The fixture must actually list through the upstream decoder.
	if code, got, body := listStatuses(t, mux, ""); code != http.StatusOK || strings.Join(got, ",") != "running,suspended,creating" {
		t.Fatalf("default list = %d %v (%s), want every status but terminated", code, got, body)
	}
	for query, want := range map[string]string{
		"?status=running":                   "running",
		"?status=running&status=creating":   "running,creating",
		"?status=running,suspended":         "running,suspended",
		"?status=terminated":                "terminated",
		"?status=suspending":                "",
		"?status=creating,running,errored,": "running,creating",
	} {
		code, got, body := listStatuses(t, mux, query)
		if code != http.StatusOK || strings.Join(got, ",") != want {
			t.Errorf("list%s = %d %v (%s), want [%s]", query, code, got, body, want)
		}
	}
	if code, _, body := listStatuses(t, mux, "?status=sleeping"); code != http.StatusBadRequest || !strings.Contains(body, "SANDBOX_STATUS_INVALID") {
		t.Errorf("unknown status = %d %s, want 400 SANDBOX_STATUS_INVALID", code, body)
	}
	// A group filter this deployment cannot apply (no group store) is refused
	// rather than answered with the unfiltered list; a malformed one is a 400
	// naming Render's code. (The filter itself is covered in groups_test.go.)
	if code, _, body := listStatuses(t, mux, "?sandboxGroupId=sbg-aaaaaaaaaaaaaaaaaaaa"); code != http.StatusServiceUnavailable || errCode(body) != CodeSandboxGroupsUnavailable {
		t.Errorf("group filter without a group store = %d %s, want 503", code, body)
	}
	if code, _, body := listStatuses(t, mux, "?sandboxGroupId=nope"); code != http.StatusBadRequest || !strings.Contains(body, CodeInvalidSandboxGroupID) {
		t.Errorf("malformed group filter = %d %s, want 400 %s", code, body, CodeInvalidSandboxGroupID)
	}
}

// Without the control-plane store a create that names a group is refused, not
// created outside any group, and an unnamed create still works with no limit.
func TestCreateNamingAGroupNeedsTheGroupStore(t *testing.T) {
	var created int
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		created++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Running"}}`))
	})
	svc.DefaultTemplate = "node"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(body)).WithContext(callerCtx()))
		return rec
	}
	if rec := post(`{"sandboxGroupId":"sbg-aaaaaaaaaaaaaaaaaaaa"}`); rec.Code != http.StatusServiceUnavailable || created != 0 {
		t.Fatalf("create naming a group = %d %s (created %d), want a 503 before anything is created", rec.Code, rec.Body, created)
	}
	if rec := post(`{}`); rec.Code != http.StatusCreated || created != 1 || strings.Contains(rec.Body.String(), "sandboxGroupId") {
		t.Fatalf("unnamed create without a store = %d %s (created %d), want a plain create", rec.Code, rec.Body, created)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/sandbox-groups"},
		{http.MethodPost, "/v1/sandbox-groups"},
		{http.MethodGet, "/v1/sandbox-groups/sbg-aaaaaaaaaaaaaaaaaaaa"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, strings.NewReader(`{"name":"x"}`)).WithContext(callerCtx()))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s without a group store = %d %s, want 503", route.method, route.path, rec.Code, rec.Body)
		}
	}
}

func TestListStatusFilterAgreesAcrossGraphQLAndMCP(t *testing.T) {
	svc := listUpstream(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlStatuses := func(query string) ([]string, []string) {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: callerCtx()})
		var codes []string
		for _, e := range res.Errors {
			codes = append(codes, e.Extensions["code"].(string))
		}
		var out []string
		if data, ok := res.Data.(map[string]any); ok {
			list, _ := data["sandboxes"].([]any)
			for _, item := range list {
				out = append(out, item.(map[string]any)["status"].(string))
			}
		}
		return out, codes
	}
	if got, _ := gqlStatuses(`{ sandboxes { status } }`); strings.Join(got, ",") != "running,suspended,creating" {
		t.Errorf("GraphQL default = %v", got)
	}
	if got, _ := gqlStatuses(`{ sandboxes(status: ["suspended"]) { status } }`); strings.Join(got, ",") != "suspended" {
		t.Errorf("GraphQL status filter = %v", got)
	}
	if _, codes := gqlStatuses(`{ sandboxes(status: ["sleeping"]) { status } }`); len(codes) != 1 || codes[0] != "SANDBOX_STATUS_INVALID" {
		t.Errorf("GraphQL unknown status codes = %v", codes)
	}

	client := mcpClient(t, svc)
	call := func(args map[string]any) (*mcp.CallToolResult, []string) {
		res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_sandboxes", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var out listSandboxesResult
		_ = json.Unmarshal(raw, &out)
		var statuses []string
		for _, sb := range out.Sandboxes {
			statuses = append(statuses, string(sb.Status))
		}
		return res, statuses
	}
	if _, got := call(map[string]any{"status": []string{"running", "creating"}}); strings.Join(got, ",") != "running,creating" {
		t.Errorf("MCP status filter = %v", got)
	}
	if res, _ := call(map[string]any{"status": []string{"sleeping"}}); !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "SANDBOX_STATUS_INVALID") {
		t.Errorf("MCP unknown status = %#v", res.Content)
	}
}

func TestNotOfferedRoutesNameTheFeature(t *testing.T) {
	// Resolve each request to its table pattern the way the REST mux does
	// (internal/api mounts NotOfferedRoutes and covers the written 404).
	mux := http.NewServeMux()
	for pattern := range NotOfferedRoutes {
		mux.HandleFunc(pattern, func(http.ResponseWriter, *http.Request) {})
	}
	// The four group routes are served since w22/m311 and are no longer here.
	for _, served := range []string{
		"GET /v1/sandbox-groups", "POST /v1/sandbox-groups",
		"GET /v1/sandbox-groups/{sandboxGroupId}", "PATCH /v1/sandbox-groups/{sandboxGroupId}",
	} {
		if _, refused := NotOfferedRoutes[served]; refused {
			t.Errorf("%s is served by the Service, so it must not also be a not-offered refusal", served)
		}
	}
	for _, tc := range []struct{ method, path, code, mentions string }{
		{http.MethodPost, "/v1/sandbox-groups/sbg-1/builds", CodeSandboxImagesUnsupported, "images import"},
		{http.MethodGet, "/v1/sandbox-groups/sbg-1/builds/sbb-1", CodeSandboxImagesUnsupported, "images import|get"},
		{http.MethodPost, "/v1/sandboxes/sbx-1/snapshots", codeSandboxSnapshotsUnsupported, "snapshots"},
		{http.MethodDelete, "/v1/sandbox-groups/sbg-1/snapshots/snp-1", codeSandboxSnapshotsUnsupported, "snapshots"},
		// CLI v2.36's sandbox-groups delete, default set and credentials (w3/m303).
		{http.MethodDelete, "/v1/sandbox-groups/sbg-1", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodPost, "/v1/sandbox-groups/sbg-1/set-default", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodGet, "/v1/sandbox-groups/sbg-1/credentials", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodPost, "/v1/sandbox-groups/sbg-1/credentials", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodPatch, "/v1/sandbox-groups/sbg-1/credentials/sbc-1", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodDelete, "/v1/sandbox-groups/sbg-1/credentials/sbc-1", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodGet, "/v1/sandbox-groups/sbg-1/credentials/sbc-1/versions", CodeSandboxGroupsUnsupported, "sandbox groups"},
		{http.MethodPost, "/v1/sandbox-groups/sbg-1/credentials/sbc-1/restore", CodeSandboxGroupsUnsupported, "sandbox groups"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(tc.method, tc.path, nil))
		refusal, ok := NotOfferedRoutes[pattern]
		if !ok || refusal.Code != tc.code || !strings.Contains(refusal.Message, tc.mentions) {
			t.Errorf("%s %s = %q %+v, want %s mentioning %q", tc.method, tc.path, pattern, refusal, tc.code, tc.mentions)
		}
	}
}
