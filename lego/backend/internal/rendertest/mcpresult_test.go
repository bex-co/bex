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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textResult(texts ...string) *mcp.CallToolResult {
	res := &mcp.CallToolResult{}
	for _, text := range texts {
		res.Content = append(res.Content, &mcp.TextContent{Text: text})
	}
	return res
}

// TestCheckRenderMCPResultCatchesTheOldShapes is the guard's own test
// (w4/m378 t003): each shape the milestone replaced fails it — a list wrapped
// in an object, the cursor moved into a field, a log label map keyed service,
// a flat datastore view, the console's {columns, rows} — and Render's own
// shapes pass.
func TestCheckRenderMCPResultCatchesTheOldShapes(t *testing.T) {
	logLine := `{"id":"l1","message":"m","timestamp":"2026-11-13T13:45:05Z","labels":[{"name":"resource","value":"srv-1"},{"name":"type","value":"build"}]}`
	owner := `"owner":{"id":"tea-1","name":"A","email":"a@x","type":"team"}`
	kv := `{"createdAt":"t","dashboardUrl":"d","id":"red-1","ipAllowList":[],"name":"k","options":{},` + owner + `,"plan":"free","region":"frankfurt","status":"available","updatedAt":"t","version":"8"}`
	for _, tc := range []struct {
		tool, text string
		ok         bool
		want       string
	}{
		{"list_log_label_values", `["build","deploy"]`, true, ""},
		{"list_log_label_values", `{"values":["build"]}`, false, "want a JSON array"},
		{"list_deploys", "[]\n\n cursor: \"\"", true, ""},
		{"list_deploys", `{"cursor":"","deploys":[]}`, false, "cursor"},
		{"list_logs", `{"hasMore":false,"nextStartTime":"t","nextEndTime":"t","logs":[` + logLine + `]}`, true, ""},
		{"list_logs", `{"hasMore":false,"nextStartTime":"t","nextEndTime":"t","logs":[{"labels":{"service":"srv-1"},"message":"m","timestamp":"t"}]}`, false, "lacks \"id\""},
		{"list_logs", `{"hasMore":false,"nextStartTime":"t","nextEndTime":"t","logs":[` + strings.Replace(logLine, `"name":"resource"`, `"key":"resource"`, 1) + `]}`, false, "lacks \"name\""},
		{"list_key_value", "[" + kv + "]", true, ""},
		{"list_key_value", "No Key Value instances found", true, ""},
		{"list_key_value", `{"keyValues":[` + kv + `]}`, false, "want a JSON array"},
		{"get_key_value", strings.Replace(kv, `"dashboardUrl":"d",`, ``, 1), true, ""},
		{"get_key_value", strings.Replace(kv, owner, `"ownerId":"tea-1"`, 1), false, "lacks \"owner\""},
		{"query_render_postgres", `[{"a":1}]`, true, ""},
		{"query_render_postgres", `{"columns":["a"],"rows":[[1]],"rowCount":1,"truncated":false}`, false, "want a JSON array"},
		{"get_metrics", `{"resourceId":"srv-1","timeRange":{},"metrics":[{"type":"cpu_usage","data":[{"labels":[{"field":"resource","value":"srv-1"}],"unit":"cpu","values":[]}]}]}`, true, ""},
		{"get_metrics", `{"series":[{"labels":{"metric":"cpu_usage"},"points":[],"unit":"cpu"}]}`, false, "lacks"},
		{"create_web_service", `{"id":"srv-1","name":"web"}`, false, "carries"},
		{"update_environment_variables", "Environment variables updated.", true, ""},
	} {
		err := CheckRenderMCPResult(tc.tool, textResult(tc.text))
		if tc.ok && err != nil {
			t.Errorf("%s %s: %v, want Render's shape to pass", tc.tool, tc.text, err)
		}
		if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s %s: %v, want a failure naming %q", tc.tool, tc.text, err, tc.want)
		}
	}
	if err := CheckRenderMCPResult("get_workspace_limits", textResult(`{}`)); err == nil {
		t.Error("a bex-only tool must not pass as a shared one")
	}
}

// TestMCPResultExtensionsAreNotRenderKeys keeps the extension list honest: a
// key Render's own type already declares is not a bex extension.
func TestMCPResultExtensionsAreNotRenderKeys(t *testing.T) {
	pin, err := LoadMCPResults()
	if err != nil {
		t.Fatal(err)
	}
	for typ, keys := range MCPResultExtensions {
		shape, ok := pin.Types[typ]
		if !ok {
			t.Errorf("extensions name %s, which the capture does not record", typ)
			continue
		}
		for _, key := range keys {
			for _, render := range append(shape.Required, shape.Optional...) {
				if key == render {
					t.Errorf("%s.%s is Render's own key, not a bex extension", typ, key)
				}
			}
		}
	}
}
