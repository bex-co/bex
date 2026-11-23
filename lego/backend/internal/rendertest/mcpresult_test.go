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

func textResult(texts ...string) *mcp.CallToolResult {
	res := &mcp.CallToolResult{}
	for _, text := range texts {
		res.Content = append(res.Content, &mcp.TextContent{Text: text})
	}
	return res
}

func objectResult(t *testing.T, text string, extra ...string) *mcp.CallToolResult {
	t.Helper()
	var structured any
	if err := json.Unmarshal([]byte(text), &structured); err != nil {
		t.Fatalf("objectResult invalid JSON %q: %v", text, err)
	}
	res := textResult(append([]string{text}, extra...)...)
	res.StructuredContent = structured
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
		tool string
		res  *mcp.CallToolResult
		ok   bool
		want string
	}{
		{"list_log_label_values", textResult(`["build","deploy"]`), true, ""},
		{"list_log_label_values", textResult(`{"values":["build"]}`), false, "want a JSON array"},
		{"list_deploys", textResult("[]\n\n cursor: \"\""), true, ""},
		{"list_deploys", textResult(`{"cursor":"","deploys":[]}`), false, "cursor"},
		{"list_logs", objectResult(t, `{"hasMore":false,"nextStartTime":"t","nextEndTime":"t","logs":[`+logLine+`]}`), true, ""},
		{"list_logs", objectResult(t, `{"hasMore":false,"nextStartTime":"t","nextEndTime":"t","logs":[{"labels":{"service":"srv-1"},"message":"m","timestamp":"t"}]}`), false, "lacks \"id\""},
		{"list_logs", objectResult(t, `{"hasMore":false,"nextStartTime":"t","nextEndTime":"t","logs":[`+strings.Replace(logLine, `"name":"resource"`, `"key":"resource"`, 1)+`]}`), false, "lacks \"name\""},
		{"list_key_value", textResult("[" + kv + "]"), true, ""},
		{"list_key_value", textResult("No Key Value instances found"), true, ""},
		{"list_key_value", textResult(`{"keyValues":[` + kv + `]}`), false, "want a JSON array"},
		{"get_key_value", objectResult(t, strings.Replace(kv, `"dashboardUrl":"d",`, ``, 1)), true, ""},
		{"get_key_value", objectResult(t, strings.Replace(kv, owner, `"ownerId":"tea-1"`, 1)), false, "lacks \"owner\""},
		{"query_render_postgres", textResult(`[{"a":1}]`), true, ""},
		{"query_render_postgres", textResult(`{"columns":["a"],"rows":[[1]],"rowCount":1,"truncated":false}`), false, "want a JSON array"},
		{"get_metrics", textResult(`{"resourceId":"srv-1","timeRange":{},"metrics":[{"type":"cpu_usage","data":[{"labels":[{"field":"resource","value":"srv-1"}],"unit":"cpu","values":[]}]}]}`), true, ""},
		{"get_metrics", textResult(`{"series":[{"labels":{"metric":"cpu_usage"},"points":[],"unit":"cpu"}]}`), false, "lacks"},
		{"create_web_service", objectResult(t, `{"id":"srv-1","name":"web"}`), false, "carries"},
		{"update_environment_variables", textResult("Environment variables updated."), true, ""},
	} {
		err := CheckRenderMCPResult(tc.tool, tc.res)
		if tc.ok && err != nil {
			t.Errorf("%s: %v, want Render's shape to pass", tc.tool, err)
		}
		if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s: %v, want a failure naming %q", tc.tool, err, tc.want)
		}
	}
	if err := CheckRenderMCPResult("get_workspace_limits", textResult(`{}`)); err == nil {
		t.Error("a bex-only tool must not pass as a shared one")
	}
}

// TestCheckRenderMCPResultStructuredContentAndBlueprintTypes pins w5/m601:
// structuredContent parity rules across array/arrayCursor/text/emptyText/object
// tools, plus key-level checking on the 6 newly typed Blueprint/cron-cancel tools.
func TestCheckRenderMCPResultStructuredContentAndBlueprintTypes(t *testing.T) {
	deployJSON := `{"id":"dep-1","status":"live"}`
	queuedText := "The deploy request was accepted, but the deploy has not been created yet. Use list_deploys to check on the service's deploys."
	metricsJSON := `{"resourceId":"srv-1","timeRange":{},"metrics":[{"type":"cpu_usage","data":[]}]}`
	bpCore := `"id":"exs-1","name":"app","ownerId":"tea-1","repo":"https://github.com/a/app","branch":"main","path":"render.yaml","autoSync":true,"status":"created"`
	bpDetailJSON := `{` + bpCore + `,"resources":[{"id":"srv-1","name":"web","type":"web_service"}],"manifest":"services: []","createdAt":"2026-11-23T00:00:00Z","updatedAt":"2026-11-23T00:00:00Z"}`
	syncPlanJSON := `{"id":"exe-1","commit":{"id":"abc"},"dashboardUrl":"https://dashboard.bex.co","rawPlan":{},"requiredManualEnvVars":[{"key":"API_KEY","serviceName":"web","serviceType":"web"}],"state":"created"}`
	createBPJSON := `{"blueprint":{` + bpCore + `},"notices":[{"message":"ok","severity":"warning"}],"planningOutcome":"planned","sync":` + syncPlanJSON + `}`
	updateBPJSON := `{"blueprint":{` + bpCore + `},"error":null,"notices":[],"planningOutcome":null,"sync":null}`
	cancelCronJSON := `{"serviceId":"crn-1","canceled":true,"run":{"id":"crr-1","status":"canceled"}}`

	for _, tc := range []struct {
		name string
		tool string
		res  *mcp.CallToolResult
		ok   bool
		want string
	}{
		// Non-object forms must never carry structuredContent.
		{
			name: "array tool with structuredContent fails",
			tool: "list_log_label_values",
			res:  &mcp.CallToolResult{Content: textResult(`["build"]`).Content, StructuredContent: []string{"build"}},
			ok:   false,
			want: "must not carry structuredContent",
		},
		{
			name: "arrayCursor tool with structuredContent fails",
			tool: "list_deploys",
			res:  &mcp.CallToolResult{Content: textResult("[]\n\n cursor: \"\"").Content, StructuredContent: map[string]any{}},
			ok:   false,
			want: "must not carry structuredContent",
		},
		{
			name: "text tool with structuredContent fails",
			tool: "update_environment_variables",
			res:  &mcp.CallToolResult{Content: textResult("Environment variables updated.").Content, StructuredContent: map[string]any{"updated": true}},
			ok:   false,
			want: "must not carry structuredContent",
		},
		{
			name: "emptyText list tool with structuredContent fails",
			tool: "list_key_value",
			res:  &mcp.CallToolResult{Content: textResult("No Key Value instances found").Content, StructuredContent: map[string]any{}},
			ok:   false,
			want: "must not carry structuredContent",
		},
		// trigger_deploy queued (emptyText exception, w8/378).
		{
			name: "trigger_deploy queued with valid Deploy structuredContent passes",
			tool: "trigger_deploy",
			res:  &mcp.CallToolResult{Content: textResult(queuedText).Content, StructuredContent: map[string]any{"id": "dep-q1", "status": "queued"}},
			ok:   true,
		},
		{
			name: "trigger_deploy queued without structuredContent fails",
			tool: "trigger_deploy",
			res:  textResult(queuedText),
			ok:   false,
			want: "lacks structuredContent",
		},
		{
			name: "trigger_deploy queued with malformed Deploy structuredContent fails",
			tool: "trigger_deploy",
			res:  &mcp.CallToolResult{Content: textResult(queuedText).Content, StructuredContent: map[string]any{"status": "queued"}},
			ok:   false,
			want: `lacks "id"`,
		},
		// get_metrics text-only object exception.
		{
			name: "get_metrics text-only passes",
			tool: "get_metrics",
			res:  textResult(metricsJSON),
			ok:   true,
		},
		{
			name: "get_metrics with structuredContent fails",
			tool: "get_metrics",
			res:  objectResult(t, metricsJSON),
			ok:   false,
			want: "must not carry structuredContent",
		},
		// Typed object tools require matching structuredContent.
		{
			name: "typed object tool with matching structuredContent passes",
			tool: "get_deploy",
			res:  objectResult(t, deployJSON),
			ok:   true,
		},
		{
			name: "typed object tool missing structuredContent fails",
			tool: "get_deploy",
			res:  textResult(deployJSON),
			ok:   false,
			want: "must carry structuredContent",
		},
		{
			name: "typed object tool with mismatched structuredContent fails",
			tool: "get_deploy",
			res:  &mcp.CallToolResult{Content: textResult(deployJSON).Content, StructuredContent: map[string]any{"id": "dep-OTHER", "status": "live"}},
			ok:   false,
			want: "does not match Content[0].Text",
		},
		// Newly pinned Blueprint and cron-cancel types: positive + negative checks.
		{
			name: "get_blueprint with documented REST-shared extensions passes",
			tool: "get_blueprint",
			res:  objectResult(t, bpDetailJSON),
			ok:   true,
		},
		{
			name: "get_blueprint missing required resources fails",
			tool: "get_blueprint",
			res:  objectResult(t, `{`+bpCore+`}`),
			ok:   false,
			want: `lacks "resources"`,
		},
		{
			name: "get_blueprint with unrecorded extra key fails",
			tool: "get_blueprint",
			res:  objectResult(t, `{`+bpCore+`,"resources":[],"bogus":1}`),
			ok:   false,
			want: `carries ["bogus"]`,
		},
		{
			name: "create_blueprint valid nested types pass",
			tool: "create_blueprint",
			res:  objectResult(t, createBPJSON),
			ok:   true,
		},
		{
			name: "create_blueprint missing nested requiredManualEnvVars key fails",
			tool: "create_blueprint",
			res:  objectResult(t, strings.Replace(createBPJSON, `"serviceType":"web"`, `"bad":"web"`, 1)),
			ok:   false,
			want: `lacks "serviceType"`,
		},
		{
			name: "update_blueprint no-op with second text block passes",
			tool: "update_blueprint",
			res:  objectResult(t, updateBPJSON, "The Blueprint already has the supplied settings. No update was needed, and planning was skipped."),
			ok:   true,
		},
		{
			name: "cancel_cron_job_run with documented run extension passes",
			tool: "cancel_cron_job_run",
			res:  objectResult(t, cancelCronJSON),
			ok:   true,
		},
		{
			name: "cancel_cron_job_run with unknown extra key fails",
			tool: "cancel_cron_job_run",
			res:  objectResult(t, `{"serviceId":"crn-1","canceled":true,"extra":1}`),
			ok:   false,
			want: `carries ["extra"]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckRenderMCPResult(tc.tool, tc.res)
			if tc.ok && err != nil {
				t.Fatalf("CheckRenderMCPResult(%s) = %v, want nil", tc.tool, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("CheckRenderMCPResult(%s) = %v, want error containing %q", tc.tool, err, tc.want)
			}
		})
	}
}

// TestRenderMCPResultsObjectToolsAreTyped prevents bare {"form":"object"}
// blind spots from returning in render-mcp-results.json (w5/m601) and checks
// that structuredContent exception maps name real shared tools.
func TestRenderMCPResultsObjectToolsAreTyped(t *testing.T) {
	pin, err := LoadMCPResults()
	if err != nil {
		t.Fatal(err)
	}
	for tool, spec := range pin.Tools {
		if spec.Form == "object" && spec.Type == "" {
			t.Errorf("shared object tool %q lacks a recorded type in render-mcp-results.json", tool)
		}
		if spec.Type != "" && spec.Type != "string" {
			if _, ok := pin.Types[spec.Type]; !ok {
				t.Errorf("shared tool %q names unrecorded type %q", tool, spec.Type)
			}
		}
	}
	for tool := range mcpTextOnlyObjectTools {
		spec, ok := pin.Tools[tool]
		if !ok || spec.Form != "object" {
			t.Errorf("mcpTextOnlyObjectTools names %q, which is not a shared object tool", tool)
		}
	}
	for tool := range mcpEmptyTextWithStructuredContent {
		spec, ok := pin.Tools[tool]
		if !ok || spec.EmptyText == "" {
			t.Errorf("mcpEmptyTextWithStructuredContent names %q, which has no emptyText", tool)
		}
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
