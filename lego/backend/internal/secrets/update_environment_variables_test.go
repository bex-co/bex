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

package secrets

import (
	"context"
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// renderEnvService wires the rebuild hook the composition root provides:
// rebuilds counts Render's triggered deploys, and deploy is the Render JSON of
// the deploy each rebuild opens.
type renderEnvService struct {
	*Service
	store    *fakeSecretStore
	rebuilds int
	deploy   json.RawMessage
}

func newRenderEnvService(t *testing.T, env map[string]string, rebuildErr error) *renderEnvService {
	t.Helper()
	store := newFakeSecretStore()
	if env != nil {
		store.m[envPath("web")] = maps.Clone(env)
	}
	r := &renderEnvService{Service: newService(store, blueprintApp("web")), store: store,
		deploy: json.RawMessage(`{"id":"dep-abc","status":"created","trigger":"service_updated"}`)}
	r.RebuildService = func(context.Context, string) (json.RawMessage, error) {
		r.rebuilds++
		if rebuildErr != nil {
			return nil, rebuildErr
		}
		return r.deploy, nil
	}
	return r
}

func (r *renderEnvService) call(t *testing.T, args map[string]any) (*mcp.CallToolResult, error) {
	t.Helper()
	return mcpSession(t, r.Service).CallTool(context.Background(), &mcp.CallToolParams{Name: "update_environment_variables", Arguments: args})
}

func toolText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func vars(kv ...string) []map[string]any {
	out := make([]map[string]any, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		out = append(out, map[string]any{"key": kv[i], "value": kv[i+1]})
	}
	return out
}

// TestUpdateEnvironmentVariablesMatchesRender pins w2/046: Render's official
// update_environment_variables (render-oss/render-mcp-server
// pkg/service/tools.go) merges by default, replaces only with replace:true,
// then triggers one deploy and answers with its fixed text.
func TestUpdateEnvironmentVariablesMatchesRender(t *testing.T) {
	const triggered = "Environment variables updated. A new deploy has been triggered to pick up the changes.\n\n" +
		"Response from deploying service: "
	existing := map[string]string{"KEEP": "k", "CHANGE": "old"}

	cases := []struct {
		name    string
		replace bool
		envVars []map[string]any
		want    map[string]string
	}{
		{"merge keeps unmentioned keys", false, vars("CHANGE", "new", "ADD", "a"),
			map[string]string{"KEEP": "k", "CHANGE": "new", "ADD": "a"}},
		{"replace drops unmentioned keys", true, vars("CHANGE", "new"),
			map[string]string{"CHANGE": "new"}},
		{"a later duplicate key wins", false, vars("CHANGE", "first", "CHANGE", "second"),
			map[string]string{"KEEP": "k", "CHANGE": "second"}},
		{"empty merge changes nothing and still deploys", false, vars(), existing},
		{"empty replace clears the store-owned set", true, vars(), map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRenderEnvService(t, existing, nil)
			res, err := r.call(t, map[string]any{"serviceId": "web", "replace": tc.replace, "envVars": tc.envVars})
			if err != nil || res.IsError {
				t.Fatalf("call: err=%v result=%q", err, toolText(res))
			}
			if got := r.store.m[envPath("web")]; !maps.Equal(got, tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
				t.Fatalf("stored env = %v, want %v", got, tc.want)
			}
			if r.rebuilds != 1 {
				t.Fatalf("deploys triggered = %d, want exactly 1", r.rebuilds)
			}
			if want := triggered + string(r.deploy); toolText(res) != want {
				t.Fatalf("text = %q, want %q", toolText(res), want)
			}
			// A Blueprint manifest's literal is not bex's to write, even on replace.
			for _, v := range getApp(t, r.Client, "web").Spec.Env {
				if v.Name == "MESSAGE" && v.Value != "hello from bex" {
					t.Fatalf("manifest literal changed: %+v", v)
				}
			}
		})
	}
}

func TestUpdateEnvironmentVariablesReportsTheDeploy(t *testing.T) {
	t.Run("rebuild reports no deploy", func(t *testing.T) {
		r := newRenderEnvService(t, nil, nil)
		r.deploy = nil
		res, err := r.call(t, map[string]any{"serviceId": "web", "envVars": vars("A", "1")})
		if err != nil || res.IsError {
			t.Fatalf("call: err=%v", err)
		}
		const want = "Environment variables updated. A deploy was requested to pick up the changes; " +
			"the request was accepted but the deploy has not been created yet. Use list_deploys to check on the service's deploys."
		if toolText(res) != want {
			t.Fatalf("text = %q", toolText(res))
		}
	})

	t.Run("deploy refused after the save", func(t *testing.T) {
		r := newRenderEnvService(t, nil, core.NewConflictError("PAYMENT_REQUIRED", "a payment method is required to deploy", nil))
		res, err := r.call(t, map[string]any{"serviceId": "web", "envVars": vars("A", "1")})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if !res.IsError || !strings.Contains(toolText(res), "a payment method is required to deploy") {
			t.Fatalf("want a tool error naming the refused deploy, got isError=%v %q", res.IsError, toolText(res))
		}
		if r.store.m[envPath("web")]["A"] != "1" {
			t.Fatal("the update must stand even though its deploy was refused, as on Render")
		}
	})
}

func TestUpdateEnvironmentVariablesRefusesOffContractInput(t *testing.T) {
	for name, args := range map[string]map[string]any{
		"generateValue is not Render's":  {"serviceId": "web", "envVars": []map[string]any{{"key": "A", "generateValue": true}}},
		"value is required":              {"serviceId": "web", "envVars": []map[string]any{{"key": "A"}}},
		"envVars is required":            {"serviceId": "web"},
		"envVars may not be null":        {"serviceId": "web", "envVars": nil},
		"a manifest literal is refused":  {"serviceId": "web", "envVars": vars("MESSAGE", "changed")},
		"unknown item fields are denied": {"serviceId": "web", "envVars": []map[string]any{{"key": "A", "value": "1", "extra": "x"}}},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRenderEnvService(t, map[string]string{"KEEP": "k"}, nil)
			res, err := r.call(t, args)
			if err == nil && (res == nil || !res.IsError) {
				t.Fatalf("accepted off-contract input: %q", toolText(res))
			}
			if r.rebuilds != 0 || !maps.Equal(r.store.m[envPath("web")], map[string]string{"KEEP": "k"}) {
				t.Fatalf("refused call still wrote: rebuilds=%d env=%v", r.rebuilds, r.store.m[envPath("web")])
			}
		})
	}
}
