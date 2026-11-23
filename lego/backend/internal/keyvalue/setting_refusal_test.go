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

package keyvalue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestUnknownKeyValueSettingsListRenderSpellings pins w32/051: a create or an
// update whose maxmemoryPolicy or persistenceMode neither spelling names answers
// 400 listing Render's spellings, the values Render's API and the pinned Render
// CLI accept, never the CRD's hyphenated ones (`journal-snapshot`). REST, MCP and
// GraphQL answer the same sentence. REST's schema gate, which refuses the value
// first on the composed stack, is pinned in internal/api.
func TestUnknownKeyValueSettingsListRenderSpellings(t *testing.T) {
	const (
		policies = "noeviction | allkeys_lru | allkeys_lfu | volatile_lru | volatile_lfu | allkeys_random | volatile_random | volatile_ttl"
		modes    = "journal_snapshot | snapshot | off"
	)
	settings := []struct{ field, value, want string }{
		{"persistenceMode", "aof", `unknown persistenceMode "aof" (valid: ` + modes + `)`},
		{"maxmemoryPolicy", "lru", `unknown maxmemoryPolicy "lru" (valid: ` + policies + `)`},
	}
	restRefusal := func(t *testing.T, method, path string, body map[string]string) string {
		t.Helper()
		raw, _ := json.Marshal(body)
		svc, _ := newService(memoryOnlyStore(nil))
		w := serveREST(svc, method, path, string(raw))
		var refusal struct{ Message string }
		if err := json.Unmarshal(w.Body.Bytes(), &refusal); w.Code != http.StatusBadRequest || err != nil {
			t.Fatalf("%s %s = %d %s, want 400", method, path, w.Code, w.Body.String())
		}
		return refusal.Message
	}
	mcpRefusal := func(t *testing.T, tool string, args map[string]any) string {
		t.Helper()
		svc, _ := newService(memoryOnlyStore(nil))
		cs := kvMCPSession(t, svc)
		defer cs.Close()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil || !res.IsError {
			t.Fatalf("%s = %v %+v, want a tool error", tool, err, res)
		}
		var text strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text.WriteString(tc.Text)
			}
		}
		return text.String()
	}
	graphqlRefusal := func(t *testing.T, mutation string) string {
		t.Helper()
		svc, _ := newService(memoryOnlyStore(nil))
		schema, err := kvGQLSchema(svc)
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: mutation})
		if len(res.Errors) == 0 {
			t.Fatalf("%s succeeded, want a refusal", mutation)
		}
		return res.Errors[0].Message
	}
	writers := map[string]func(t *testing.T, field, value string) string{
		"REST create": func(t *testing.T, field, value string) string {
			return restRefusal(t, http.MethodPost, "/v1/key-value", map[string]string{"name": "kv-new", "plan": "free", field: value})
		},
		"REST PATCH": func(t *testing.T, field, value string) string {
			return restRefusal(t, http.MethodPatch, "/v1/key-value/red-cache", map[string]string{field: value})
		},
		"MCP create_key_value": func(t *testing.T, field, value string) string {
			return mcpRefusal(t, "create_key_value", map[string]any{"name": "kv-new", "plan": "free", field: value})
		},
		"MCP update_key_value": func(t *testing.T, field, value string) string {
			return mcpRefusal(t, "update_key_value", map[string]any{"keyValueId": "red-cache", field: value})
		},
		"GraphQL createKeyValue": func(t *testing.T, field, value string) string {
			return graphqlRefusal(t, fmt.Sprintf(`mutation { createKeyValue(name: "kv-new", plan: "free", %s: %q) { id } }`, field, value))
		},
		"GraphQL set mutation": func(t *testing.T, field, value string) string {
			mutation := map[string]string{"persistenceMode": "setKeyValuePersistenceMode", "maxmemoryPolicy": "setKeyValueMaxmemoryPolicy"}[field]
			return graphqlRefusal(t, fmt.Sprintf(`mutation { %s(id: "red-cache", %s: %q) { id } }`, mutation, field, value))
		},
	}
	for surface, refuse := range writers {
		for _, setting := range settings {
			t.Run(surface+"/"+setting.field, func(t *testing.T) {
				got := refuse(t, setting.field, setting.value)
				if surface == "MCP create_key_value" {
					// create_key_value enforces Render's enum at the MCP JSON Schema boundary (w5/m600).
					expected := validPersistenceModes
					if setting.field == "maxmemoryPolicy" {
						expected = validMaxmemoryPolicies
					}
					if !strings.Contains(got, "/properties/"+setting.field) {
						t.Errorf("refusal = %q, want schema property path for %s", got, setting.field)
					}
					for _, v := range expected {
						if !strings.Contains(got, v) {
							t.Errorf("refusal = %q, missing Render spelling %q", got, v)
						}
					}
				} else if !strings.Contains(got, setting.want) {
					t.Errorf("refusal = %q, want it to contain %q", got, setting.want)
				}
				if strings.Contains(got, "journal-snapshot") || strings.Contains(got, "allkeys-lru") {
					t.Errorf("refusal = %q lists the CRD's hyphenated spellings", got)
				}
			})
		}
	}
	// The REST body carries exactly the Service's sentence.
	if got := restRefusal(t, http.MethodPatch, "/v1/key-value/red-cache", map[string]string{"persistenceMode": "aof"}); got != "bad request: "+settings[0].want {
		t.Errorf("PATCH message = %q, want %q", got, "bad request: "+settings[0].want)
	}
}
