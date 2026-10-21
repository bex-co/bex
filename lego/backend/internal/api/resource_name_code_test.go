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
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestAFreeFormNameIsAcceptedOnEverySurface (w5/m153, replacing w5/093's
// refusal): a service, Postgres or Key Value created with a free-form name,
// as Render's creates allow, succeeds on REST, GraphQL and MCP.
func TestAFreeFormNameIsAcceptedOnEverySurface(t *testing.T) {
	h, srv := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	cs := mcpSessionAs(t, srv, "dana")
	// Render's creates take a free-form name (w5/m153): a service maps it onto
	// a resource name and keeps it as its label; a Postgres or Key Value keeps
	// it as its name. Each surface gets its own name, so none collides.
	for _, tc := range []struct {
		kind, path, body, mutation, tool string
		args                             map[string]any
	}{{
		kind: "service", path: "/v1/services", body: `{"name":"My API","ownerId":"tea-a","image":{"ownerId":"tea-a","imagePath":"nginx"}}`,
		mutation: `mutation { createService(name: "My Gql API", image: "nginx") { id displayName } }`,
		tool:     "create_web_service", args: map[string]any{"name": "My Mcp API", "image": "nginx", "runtime": "image"},
	}, {
		kind: "Postgres", path: "/v1/postgres", body: `{"name":"Prod DB","plan":"free"}`,
		mutation: `mutation { createDatabase(name: "Gql DB", plan: "free") { id } }`,
		tool:     "create_postgres", args: map[string]any{"name": "Mcp DB", "plan": "free"},
	}, {
		kind: "Key Value", path: "/v1/key-value", body: `{"name":"Prod Cache","plan":"free"}`,
		mutation: `mutation { createKeyValue(name: "Gql Cache", plan: "free") { id } }`,
		tool:     "create_key_value", args: map[string]any{"name": "Mcp Cache", "plan": "free"},
	}} {
		t.Run(tc.kind, func(t *testing.T) {
			if rec := do(t, h, http.MethodPost, tc.path, testToken, tc.body); rec.Code != http.StatusCreated {
				t.Fatalf("REST create => %d %s", rec.Code, rec.Body)
			}
			if data := gql(t, h, tc.mutation); data == nil {
				t.Fatal("GraphQL create returned no data")
			}
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil || res.IsError {
				t.Fatalf("MCP %s = %s, %v", tc.tool, fmtMCP(res), err)
			}
		})
	}
}
