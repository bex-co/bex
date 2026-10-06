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

package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCustomDomainRefusalsAreCodedOnEverySurface (w5/m118): an add the platform
// refuses carries a stable code on REST, GraphQL and MCP alike, so the
// dashboard branches on CUSTOM_DOMAIN_IN_USE and CUSTOM_DOMAIN_RESERVED instead
// of the English it used to match ("another site", "reserved platform").
func TestCustomDomainRefusalsAreCodedOnEverySurface(t *testing.T) {
	holder := sampleApp("holder")
	holder.Spec.Hosts = []string{"taken.example.com"}
	svc, _ := newBaseDomainService("", "www.foo.com", sampleApp("web"), holder)
	ctx := context.Background()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema := gqlSchema(t, svc)
	session := displayNameMCPSession(t, svc)
	surfaces := map[string]func(host string) (status int, code string){
		"REST": func(host string) (int, string) {
			body, _ := json.Marshal(map[string]string{"name": host})
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services/web/custom-domains", strings.NewReader(string(body))))
			var out struct{ Code string }
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
			return rec.Code, out.Code
		},
		"GraphQL": func(host string) (int, string) {
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString:  `mutation($name: String!) { addCustomDomain(id: "web", name: $name) { name } }`,
				VariableValues: map[string]any{"name": host}})
			if len(res.Errors) != 1 {
				return 0, ""
			}
			code, _ := res.Errors[0].Extensions["code"].(string)
			return 0, code
		},
		"MCP": func(host string) (int, string) {
			res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "add_custom_domain", Arguments: map[string]any{"serviceId": "web", "name": host}})
			if err != nil || res == nil || !res.IsError || len(res.Content) != 1 {
				return 0, ""
			}
			text, _ := res.Content[0].(*mcp.TextContent)
			code, _, _ := strings.Cut(text.Text, ": ")
			return 0, code
		},
	}
	for _, tc := range []struct {
		host, code string
		status     int
	}{
		{"*.example.com", "CUSTOM_DOMAIN_INVALID", http.StatusBadRequest},
		{"localhost", "CUSTOM_DOMAIN_INVALID", http.StatusBadRequest},
		{"co.uk", "CUSTOM_DOMAIN_INVALID", http.StatusBadRequest},
		{"github.io", "CUSTOM_DOMAIN_INVALID", http.StatusBadRequest},
		{"api.foo.com", "CUSTOM_DOMAIN_RESERVED", http.StatusBadRequest},
		{"taken.example.com", "CUSTOM_DOMAIN_IN_USE", http.StatusConflict},
	} {
		for name, add := range surfaces {
			status, code := add(tc.host)
			if code != tc.code || (name == "REST" && status != tc.status) {
				t.Errorf("%s add %q = %d %q, want %d %s", name, tc.host, status, code, tc.status, tc.code)
			}
		}
	}
}
