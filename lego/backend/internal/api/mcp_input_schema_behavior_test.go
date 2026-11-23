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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type mcpBehaviorDeployStore struct {
	deployHookStore
	lastClearCache bool
}

func (s *mcpBehaviorDeployStore) CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string, clearCache ...bool) (store.Deploy, error) {
	s.lastClearCache = len(clearCache) > 0 && clearCache[0]
	return s.deployHookStore.CreateDeploy(ctx, appID, trigger, image, generation, commit, triggeredBy, clearCache...)
}

func (*mcpBehaviorDeployStore) SetAppImage(context.Context, string, string) error { return nil }

// TestMCPComposedInputSchemaValidation exercises w5/m600 end-to-end through
// Server.MCPServer(): the MCP Go SDK's JSON Schema validator must accept valid
// Render calls and documented bex extensions (envVars[].generateValue, runtime
// "image", string/number postgres version, boolean/string clearCache) while
// rejecting invalid enum and union-type arguments cleanly.
func TestMCPComposedInputSchemaValidation(t *testing.T) {
	cl := fakeClient(sampleApp("web"))
	base := &core.Base{Client: cl, Namespace: "default"}
	ds := &mcpBehaviorDeployStore{}
	srv := NewServer(base, Deps{DeployStore: ds})
	cs := mcpSessionAs(t, srv, "user-1")

	callRaw := func(t *testing.T, tool string, args map[string]any) (*mcp.CallToolResult, string) {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error: %v", tool, err)
		}
		var sb strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		return res, sb.String()
	}

	t.Run("envVars generateValue passes MCP schema validation on all three create tools", func(t *testing.T) {
		for _, tc := range []struct {
			tool string
			args map[string]any
		}{
			{
				tool: "create_web_service",
				args: map[string]any{
					"name":    "gen-web",
					"runtime": "image",
					"image":   "nginx:alpine",
					"envVars": []map[string]any{{"key": "SECRET", "generateValue": true}},
					"dryRun":  true,
				},
			},
			{
				tool: "create_cron_job",
				args: map[string]any{
					"name":     "gen-cron",
					"runtime":  "image",
					"image":    "busybox:1.36",
					"schedule": "0 * * * *",
					"envVars":  []map[string]any{{"key": "SECRET", "generateValue": true}},
					"dryRun":   true,
				},
			},
			{
				tool: "create_static_site",
				args: map[string]any{
					"name":        "gen-static",
					"repo":        "https://github.com/acme/site",
					"publishPath": "dist",
					"envVars":     []map[string]any{{"key": "SECRET", "generateValue": true}},
					"dryRun":      true,
				},
			},
		} {
			res, text := callRaw(t, tc.tool, tc.args)
			if res.IsError {
				t.Errorf("%s with generateValue:true failed: %s", tc.tool, text)
			}
		}

		// Neither value nor generateValue, and both at once, must still fail in toEnvVars.
		for _, bad := range []map[string]any{
			{"key": "SECRET"},
			{"key": "SECRET", "value": "a", "generateValue": true},
		} {
			res, text := callRaw(t, "create_web_service", map[string]any{
				"name":    "bad-env-web",
				"runtime": "image",
				"image":   "nginx:alpine",
				"envVars": []map[string]any{bad},
				"dryRun":  true,
			})
			if !res.IsError || !strings.Contains(text, "ENVIRONMENT_VALUE_INPUT_INVALID") {
				t.Errorf("create_web_service envVars=%v => isError=%v text=%q, want ENVIRONMENT_VALUE_INPUT_INVALID", bad, res.IsError, text)
			}
		}
	})

	t.Run("runtime superset accepts image and Render runtimes and rejects unknown", func(t *testing.T) {
		for _, tool := range []string{"create_web_service", "create_cron_job"} {
			baseArgs := func(rt string) map[string]any {
				a := map[string]any{"name": "rt-" + rt, "runtime": rt, "dryRun": true}
				if tool == "create_cron_job" {
					a["schedule"] = "0 * * * *"
				}
				switch rt {
				case "image":
					a["image"] = "nginx:alpine"
				case "node":
					a["repo"] = "https://github.com/acme/app"
					a["buildCommand"] = "npm ci"
					a["startCommand"] = "npm start"
				case "docker":
					a["repo"] = "https://github.com/acme/app"
				}
				return a
			}
			for _, rt := range []string{"image", "node", "docker"} {
				res, text := callRaw(t, tool, baseArgs(rt))
				if res.IsError {
					t.Errorf("%s runtime=%q failed: %s", tool, rt, text)
				}
			}
			res, text := callRaw(t, tool, baseArgs("cobol"))
			if !res.IsError || !strings.Contains(text, "/properties/runtime") {
				t.Errorf("%s runtime=cobol => isError=%v text=%q, want /properties/runtime schema refusal", tool, res.IsError, text)
			}
		}
	})

	t.Run("create_postgres version union accepts number and string and rejects boolean", func(t *testing.T) {
		for _, v := range []any{16, "16"} {
			res, text := callRaw(t, "create_postgres", map[string]any{
				"name": "pg-ver", "plan": "free", "version": v, "dryRun": true,
			})
			if res.IsError {
				t.Errorf("create_postgres version=%v (%T) failed: %s", v, v, text)
			}
		}
		res, text := callRaw(t, "create_postgres", map[string]any{
			"name": "pg-ver-bad", "plan": "free", "version": true, "dryRun": true,
		})
		if !res.IsError || !strings.Contains(text, "/properties/version") {
			t.Errorf("create_postgres version=true => isError=%v text=%q, want /properties/version schema refusal", res.IsError, text)
		}
	})

	t.Run("trigger_deploy clearCache union accepts boolean and string and rejects number", func(t *testing.T) {
		// Seed a store-managed repo-backed app so trigger_deploy succeeds cleanly for valid clearCache values.
		repoApp := sampleApp("repo-web")
		repoApp.Labels = map[string]string{core.LabelAppID: "srv-repo-web"}
		repoApp.Spec.Image = ""
		repoApp.Spec.Repo = "https://github.com/acme/repo-web"
		if err := cl.Create(context.Background(), repoApp); err != nil {
			t.Fatalf("seed repo-web: %v", err)
		}
		for _, v := range []any{true, "clear"} {
			ds.lastClearCache = false
			res, text := callRaw(t, "trigger_deploy", map[string]any{
				"serviceId": "repo-web", "clearCache": v,
			})
			if res.IsError || !ds.lastClearCache {
				t.Errorf("trigger_deploy clearCache=%v (%T) => isError=%v clearCache=%v text=%q, want success with clearCache=true", v, v, res.IsError, ds.lastClearCache, text)
			}
		}
		res, text := callRaw(t, "trigger_deploy", map[string]any{
			"serviceId": "repo-web", "clearCache": 123,
		})
		if !res.IsError || !strings.Contains(text, "/properties/clearCache") {
			t.Errorf("trigger_deploy clearCache=123 => isError=%v text=%q, want /properties/clearCache schema refusal", res.IsError, text)
		}
	})

	t.Run("enum rejections fire at MCP schema validation", func(t *testing.T) {
		for _, tc := range []struct {
			tool string
			prop string
			args map[string]any
		}{
			{"list_logs", "direction", map[string]any{"resource": []string{"web"}, "direction": "sideways"}},
			{"list_log_label_values", "label", map[string]any{"resource": []string{"web"}, "label": "bogus"}},
			{"get_metrics", "cpuUsageAggregationMethod", map[string]any{"resourceId": "web", "metricTypes": []string{"cpu_usage"}, "cpuUsageAggregationMethod": "P99"}},
			{"create_blueprint", "existingResources", map[string]any{"repoUrl": "https://github.com/acme/app", "existingResources": "overwrite"}},
			{"create_key_value", "persistenceMode", map[string]any{"name": "kv-bad", "persistenceMode": "aof"}},
		} {
			res, text := callRaw(t, tc.tool, tc.args)
			if !res.IsError || !strings.Contains(text, "/properties/"+tc.prop) {
				t.Errorf("%s invalid %s => isError=%v text=%q, want /properties/%s schema refusal", tc.tool, tc.prop, res.IsError, text, tc.prop)
			}
		}
	})

	// Ensure dryRun creates wrote no App CRs beyond the two seeded fixtures.
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Errorf("apps after dry-run/refused calls = %d, want 2 seeded fixtures", len(list.Items))
	}
}
