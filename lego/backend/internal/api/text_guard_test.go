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

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestBoundaryRefusesUnbindableText is w4/519: a GraphQL argument (literal,
// nested input or variable) or a REST query parameter that is not valid UTF-8
// or carries a NUL is refused at the boundary with INVALID_TEXT naming it, so
// no resolver, authorization check or store bind turns it into INTERNAL or
// "authorization service unavailable". Legal text passes untouched.
func TestBoundaryRefusesUnbindableText(t *testing.T) {
	srv := matrixServer(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	gql := func(body string) graphql.Result {
		t.Helper()
		rec := do(t, h, http.MethodPost, "/graphql", testToken, body)
		var result graphql.Result
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("status=%d body=%s: %v", rec.Code, rec.Body.String(), err)
		}
		return result
	}
	for name, tc := range map[string]struct{ body, arg string }{
		"literal":         {`{"query":"{ service(id:\"\\u0000\") { id } }"}`, "id"},
		"literal in list": {`{"query":"{ services(name:[\"ok\",\"a\\u0000\"]) { id } }"}`, "name"},
		"variable":        {`{"query":"query($id: String!) { service(id:$id) { id } }","variables":{"id":"x\u0000"}}`, "$id"},
		"nested variable": {`{"query":"query($v: [String]) { services(name:$v) { id } }","variables":{"v":["ok","\u0000"]}}`, "$v"},
	} {
		t.Run(name, func(t *testing.T) {
			result := gql(tc.body)
			if len(result.Errors) != 1 || result.Data != nil {
				t.Fatalf("result = %+v, want one error and no data", result)
			}
			e := result.Errors[0]
			if e.Extensions["code"] != core.InvalidTextCode || !strings.Contains(e.Message, `"`+tc.arg+`"`) {
				t.Fatalf("error = %q %v, want %s naming %q", e.Message, e.Extensions, core.InvalidTextCode, tc.arg)
			}
		})
	}
	// Legal text keeps the resolver's own answer.
	for _, e := range gql(`{"query":"{ service(id:\"😀\") { id } }"}`).Errors {
		if e.Extensions["code"] == core.InvalidTextCode {
			t.Fatalf("emoji control refused at the boundary: %q", e.Message)
		}
	}

	for _, query := range []string{"name=%00", "name=ok&name=a%00", "name=%FF", "%00=x"} {
		rec := do(t, h, http.MethodGet, "/v1/services?"+query, testToken, "")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid query parameter") {
			t.Errorf("GET /v1/services?%s = %d %s, want 400 invalid query parameter", query, rec.Code, rec.Body.String())
		}
	}
	if rec := do(t, h, http.MethodGet, "/v1/services?name=%F0%9F%98%80", testToken, ""); rec.Code != http.StatusOK {
		t.Errorf("emoji control = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodGet, "/v1/services?name=%00", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated = %d, want 401 ahead of the text check", rec.Code)
	}
}

// w4/519: an MCP tool call with such an argument is refused before the tool,
// its workspace binding or its authorization runs; legal text reaches the tool.
func TestMCPCallRefusesUnbindableText(t *testing.T) {
	reached := false
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		reached = true
		return &mcp.CallToolResult{}, nil
	}
	call := func(args string) mcp.Result {
		t.Helper()
		reached = false
		req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "get_service", Arguments: json.RawMessage(args)}}
		res, err := mcpBindCallWorkspace(context.Background(), &core.Base{}, "tools/call", req, next, nil)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for args, name := range map[string]string{
		`{"serviceId":"\u0000"}`:               "serviceId",
		`{"filter":{"tags":["ok","a\u0000"]}}`: "filter",
	} {
		res, ok := call(args).(*mcp.CallToolResult)
		if !ok || !res.IsError || reached {
			t.Fatalf("%s: result %+v reached=%v, want a refusal before the tool", args, res, reached)
		}
		raw, _ := json.Marshal(res.Content)
		if !strings.Contains(string(raw), `invalid argument \"`+name+`\"`) {
			t.Fatalf("%s: content %s, want it to name %q", args, raw, name)
		}
	}
	if call(`{"serviceId":"😀"}`); !reached {
		t.Fatal("legal text did not reach the tool")
	}
}
