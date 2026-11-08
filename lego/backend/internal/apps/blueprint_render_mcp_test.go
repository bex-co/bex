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
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m283: Render's Blueprint MCP tools (render-mcp-server 16fe2e9) on bex —
// create_blueprint plans and applies nothing, approve_blueprint applies,
// get/plan/update take Render's blueprintId.
func TestRenderBlueprintMCPTools(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	server := mcp.NewServer(&mcp.Implementation{Name: "m283", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "m283", Version: "0"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	call := func(tool string, args map[string]any) (map[string]any, bool) {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		raw, _ := json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(raw, &out)
		if res.IsError && len(res.Content) > 0 {
			if text, ok := res.Content[0].(*mcp.TextContent); ok {
				out = map[string]any{"error": text.Text}
			}
		}
		return out, res.IsError
	}
	apps := func() int {
		t.Helper()
		var list appv1alpha1.AppList
		if err := svc.Client.List(ctx, &list); err != nil {
			t.Fatal(err)
		}
		return len(list.Items)
	}

	for _, bad := range []string{"git@github.com:a/app.git", "https://github.com/a/app?ref=dev", "http://github.com/a/app", "https://github.com/"} {
		if _, refused := call("create_blueprint", map[string]any{"repoUrl": bad}); !refused {
			t.Errorf("create_blueprint repoUrl %q was accepted", bad)
		}
	}
	if _, refused := call("create_blueprint", map[string]any{"repoUrl": "https://github.com/a/app", "name": "  "}); !refused {
		t.Error("create_blueprint accepted a blank name")
	}

	created, refused := call("create_blueprint", map[string]any{"repoUrl": "ssh://git@github.com/a/app.git"})
	if refused || created["planningOutcome"] != PlanningPlanned {
		t.Fatalf("create_blueprint = %v (refused %v), want a planned first sync", created, refused)
	}
	bp := created["blueprint"].(map[string]any)
	id := bp["id"].(string)
	if bp["name"] != "app" || bp["repo"] != "https://github.com/a/app.git" || bp["status"] != store.BlueprintStatusCreated {
		t.Fatalf("blueprint = %v, want name app from the repo, the https form of the ssh URL, status created", bp)
	}
	if n := apps(); n != 0 {
		t.Fatalf("create_blueprint applied %d resources, want none before approval", n)
	}
	if mode := st.blueprints[ids.BlueprintStorageID(id)].ExistingResources; mode != store.BlueprintExistingCreateNew {
		t.Errorf("existingResources = %q, want Render MCP's create_new default", mode)
	}

	got, refused := call("get_blueprint", map[string]any{"blueprintId": id})
	if refused || got["id"] != id || got["status"] != store.BlueprintStatusCreated {
		t.Fatalf("get_blueprint = %v (refused %v)", got, refused)
	}
	if _, ok := got["resources"].([]any); !ok {
		t.Errorf("get_blueprint resources = %v, want the REST detail's array", got["resources"])
	}

	if _, refused := call("approve_blueprint", map[string]any{"blueprintId": id}); !refused {
		t.Fatal("approve_blueprint applied without the required manual env var")
	}
	// An agent answers the plan's own requiredManualEnvVars entry: create_new
	// suffixes the service name.
	required := created["sync"].(map[string]any)["requiredManualEnvVars"].([]any)[0].(map[string]any)
	approved, refused := call("approve_blueprint", map[string]any{"blueprintId": id, "manualEnvVars": []any{
		map[string]any{"serviceType": required["serviceType"], "serviceName": required["serviceName"], "key": "API_TOKEN", "value": "s3cret"}}})
	if refused || approved["state"] != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve_blueprint = %v (refused %v)", approved, refused)
	}
	if n := apps(); n != 1 {
		t.Fatalf("after approve_blueprint: %d resources, want 1", n)
	}

	plan, refused := call("plan_blueprint", map[string]any{"blueprintId": id})
	if refused || plan["planningOutcome"] != PlanningNoChanges {
		t.Fatalf("plan_blueprint = %v, want no_changes", plan)
	}

	for _, args := range []map[string]any{{"blueprintId": id}, {"blueprintId": id, "path": " "}} {
		if _, refused := call("update_blueprint", args); !refused {
			t.Errorf("update_blueprint %v was accepted", args)
		}
	}
	// The stored path is the normalized one: a padded spelling is no change.
	same, refused := call("update_blueprint", map[string]any{"blueprintId": id, "name": "app", "path": " render.yaml"})
	if refused || same["planningOutcome"] != nil || same["error"] != nil {
		t.Fatalf("no-op update_blueprint = %v, want planning skipped", same)
	}
	renamed, refused := call("update_blueprint", map[string]any{"blueprintId": id, "name": "stack", "autoSync": false})
	if refused || renamed["planningOutcome"] != nil || renamed["blueprint"].(map[string]any)["name"] != "stack" {
		t.Fatalf("rename update_blueprint = %v, want renamed without a plan", renamed)
	}
	moved, refused := call("update_blueprint", map[string]any{"blueprintId": id, "branch": "dev"})
	if refused || moved["planningOutcome"] != PlanningNoChanges || moved["blueprint"].(map[string]any)["branch"] != "dev" {
		t.Fatalf("branch update_blueprint = %v, want the move planned", moved)
	}
}

func TestParseBlueprintRepoURL(t *testing.T) {
	for _, c := range []struct{ in, url, name string }{
		{"https://github.com/a/app", "https://github.com/a/app", "app"},
		{"https://github.com/a/app.git/", "https://github.com/a/app.git", "app"},
		{"ssh://git@github.com:22/a/app.git", "https://github.com/a/app.git", "app"},
	} {
		url, name, err := parseBlueprintRepoURL(c.in)
		if err != nil || url != c.url || name != c.name {
			t.Errorf("parseBlueprintRepoURL(%q) = %q, %q, %v; want %q, %q", c.in, url, name, err, c.url, c.name)
		}
	}
}

// w5/181: a planned create without a branch reads the repository's default
// branch, as Render does; main only when GitHub has no opinion.
func TestPlannedBlueprintCreateReadsTheDefaultBranch(t *testing.T) {
	for _, c := range []struct {
		name, defaultBranch, want string
	}{
		{"github default", "trunk", "trunk"},
		{"no opinion", "", DefaultBlueprintBranch},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _, ctx := newOnboardingService(t, onboardingManifest)
			svc.GitHub = &fakeCloneTokens{ok: true, defaultBranch: c.defaultBranch}
			res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{
				Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Blueprint == nil || res.Blueprint.Branch != c.want {
				t.Fatalf("branch = %+v, want %q", res.Blueprint, c.want)
			}
		})
	}
}
