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
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The MCP gateway strips ownerId from every advertised schema and refuses it
// on a call (mcp_workspace.go), but the SDK validates a call against the
// schema the tool registered. A registered schema that still requires ownerId
// therefore makes the tool impossible to call: create_build_source failed on
// production with "missing properties: [ownerId]" (w9/m180). Read the
// registered schemas, before the gateway rewrites them.
func TestNoMCPToolRegistersARequiredOwnerID(t *testing.T) {
	s := NewServer(&core.Base{}, Deps{})
	raw := mcp.NewServer(&mcp.Implementation{Name: "registered", Version: "0"}, nil)
	for _, f := range s.features() {
		if r, ok := f.(mcpRegistrar); ok {
			r.RegisterMCP(raw)
		}
	}
	a, b := mcp.NewInMemoryTransports()
	if _, err := raw.Connect(context.Background(), a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "owner-required", Version: "0"}, nil).Connect(context.Background(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) == 0 {
		t.Fatal("no tools registered; the sweep would pass vacuously")
	}
	for _, tool := range listed.Tools {
		if _, callerScoped := mcpCallerScopedTools[tool.Name]; callerScoped {
			continue
		}
		encoded, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(encoded, &schema); err != nil {
			t.Fatalf("%s schema: %v", tool.Name, err)
		}
		if slices.Contains(schema.Required, "ownerId") || slices.Contains(schema.Required, "ownerID") {
			t.Errorf("%s registers a required ownerId, which the gateway refuses: bind an args type without it and read core.NamedWorkspace", tool.Name)
		}
	}
}
