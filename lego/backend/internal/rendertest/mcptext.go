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

package rendertest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPCursorMarker separates Render's paged list text from its cursor
// (list_deploys, list_events).
const MCPCursorMarker = "\n\n cursor: "

// MCPText returns a tool result's first text content: the contract Render's
// MCP server answers in, which sends no structuredContent (w4/m378).
func MCPText(t testing.TB, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("tool result has no content: %+v", res)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("first content is %T, want text", res.Content[0])
	}
	return text.Text
}

// DecodeMCPText decodes the first text content as JSON into out.
func DecodeMCPText(t testing.TB, res *mcp.CallToolResult, out any) {
	t.Helper()
	text := MCPText(t, res)
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("decode tool text: %v (%s)", err, text)
	}
}

// DecodeMCPListWithCursor decodes Render's paged list text — a JSON array,
// then MCPCursorMarker and the cursor — into out, and returns the cursor, with
// Render's quoted empty string ("") read as "".
func DecodeMCPListWithCursor(t testing.TB, res *mcp.CallToolResult, out any) string {
	t.Helper()
	text := MCPText(t, res)
	body, cursor, ok := strings.Cut(text, MCPCursorMarker)
	if !ok {
		t.Fatalf("tool text has no %q suffix: %s", MCPCursorMarker, text)
	}
	if err := json.Unmarshal([]byte(body), out); err != nil {
		t.Fatalf("decode tool list: %v (%s)", err, body)
	}
	if cursor == `""` {
		return ""
	}
	return cursor
}
