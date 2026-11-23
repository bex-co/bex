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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestUpdateSourceGraphQLDeploysOnceWhileRESTAndMCPSaveOnly pins w4/m593 through
// the real composed Server (appsSvc + deploysSvc):
//   - GraphQL setRepo / setImage with deploy: true saves atomically and opens
//     exactly one deploy row, returning its dep-... id in latestDeployId.
//   - REST PATCH /v1/services/{id} and MCP update_service /
//     link_service_to_build_source save without opening any deploy row.
//   - A suspended service saves cleanly with zero deploys and null latestDeployId.
func TestUpdateSourceGraphQLDeploysOnceWhileRESTAndMCPSaveOnly(t *testing.T) {
	ws := ids.New(ids.Workspace)
	webID := ids.New(ids.Service)
	siteID := ids.New(ids.Service)
	suspID := ids.New(ids.Service)
	bdsID := ids.New(ids.BuildSource)

	mkApp := func(name, id, svcType string, spec appv1alpha1.AppSpec) *appv1alpha1.App {
		a := &appv1alpha1.App{}
		a.Name, a.Namespace = core.CRName(ws, name), "default"
		a.Generation = 1
		a.Labels = map[string]string{
			core.LabelTenant:      ws,
			core.LabelServiceName: name,
			store.LabelAppID:      id,
		}
		a.Spec = spec
		a.Spec.Type = svcType
		return a
	}

	web := mkApp("web", webID, appv1alpha1.TypeWebService, appv1alpha1.AppSpec{
		Repo: "https://github.com/acme/web", Branch: "main",
		Runtime: "node", Builder: "native", BuildCommand: "npm ci", StartCommand: "npm start",
	})
	site := mkApp("site", siteID, appv1alpha1.TypeStaticSite, appv1alpha1.AppSpec{
		Repo: "https://github.com/acme/site", Branch: "main", PublishPath: "dist",
	})
	susp := mkApp("susp", suspID, appv1alpha1.TypeWebService, appv1alpha1.AppSpec{
		Repo: "https://github.com/acme/old", Branch: "main", Runtime: "docker", Builder: "dockerfile", Suspended: true,
	})
	carrier := mkApp("shared-build", bdsID, appv1alpha1.TypeBuildSource, appv1alpha1.AppSpec{
		Image: "ghcr.io/acme/shared:v1",
	})
	carrier.Status.ActiveRevision = "1"
	carrier.Status.Image = "ghcr.io/acme/shared:v1"

	depStore := newConformMutableDeployStore()
	base := &core.Base{
		Client:    fakeClient(web, site, susp, carrier),
		Namespace: "default",
		Authz:     &fakeChecker{allow: true},
		Workspace: fakeWorkspace{"client-1": ws},
	}
	h, _ := serverWith(t, base, Deps{DeployStore: depStore})

	mcpCall := func(tool string, args map[string]any) {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params":  map[string]any{"name": tool, "arguments": args},
		})
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(payload)))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"isError":true`) {
			t.Fatalf("MCP %s = %d %s", tool, rec.Code, rec.Body.String())
		}
	}

	gql := func(query string) map[string]any {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"query": query})
		res := do(t, h, http.MethodPost, "/graphql", testToken, string(payload))
		if res.Code != http.StatusOK {
			t.Fatalf("POST /graphql = %d %s", res.Code, res.Body)
		}
		var out struct {
			Data   map[string]any `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode graphql: %v (%s)", err, res.Body)
		}
		if len(out.Errors) > 0 {
			t.Fatalf("graphql errors: %+v", out.Errors)
		}
		return out.Data
	}

	// 1. REST PATCH saves source + runtime + commands without deploying.
	restRes := do(t, h, http.MethodPatch, "/v1/services/"+webID, testToken,
		`{"repo":"https://github.com/acme/web2","branch":"dev","serviceDetails":{"runtime":"python","envSpecificDetails":{"buildCommand":"pip install -r requirements.txt","startCommand":"gunicorn app:app"}}}`)
	if restRes.Code != http.StatusOK {
		t.Fatalf("REST PATCH = %d %s", restRes.Code, restRes.Body)
	}
	if got := len(depStore.byApp[webID]); got != 0 {
		t.Fatalf("REST PATCH opened %d deploy rows, want 0", got)
	}

	// 2. GraphQL setRepo with deploy: true opens exactly ONE deploy row.
	data := gql(`mutation { setRepo(id: "` + webID + `", repo: "https://github.com/acme/web3", branch: "main", runtime: "docker", deploy: true) { runtime latestDeployId } }`)
	gotWeb := data["setRepo"].(map[string]any)
	depID, _ := gotWeb["latestDeployId"].(string)
	if !strings.HasPrefix(depID, "dep-") {
		t.Fatalf("setRepo latestDeployId = %v, want dep-...", gotWeb["latestDeployId"])
	}
	if len(depStore.byApp[webID]) != 1 || depStore.byApp[webID][0].ID != depID {
		t.Fatalf("web deploy rows = %+v, want 1 row with id %s", depStore.byApp[webID], depID)
	}

	// 3. GraphQL setImage with deploy: true opens exactly ONE more deploy row.
	data = gql(`mutation { setImage(id: "` + webID + `", image: "nginx:1.27-alpine", deploy: true) { imagePath latestDeployId } }`)
	imgDepID, _ := data["setImage"].(map[string]any)["latestDeployId"].(string)
	if !strings.HasPrefix(imgDepID, "dep-") || imgDepID == depID {
		t.Fatalf("setImage latestDeployId = %v", imgDepID)
	}
	if len(depStore.byApp[webID]) != 2 {
		t.Fatalf("web deploy rows after setImage = %d, want 2", len(depStore.byApp[webID]))
	}

	// 4. Static site setRepo with publishPath + deploy: true opens ONE deploy row.
	data = gql(`mutation { setRepo(id: "` + siteID + `", repo: "https://github.com/acme/docs", branch: "main", buildCommand: "npm run build", publishPath: "out", deploy: true) { publishPath latestDeployId } }`)
	siteDepID, _ := data["setRepo"].(map[string]any)["latestDeployId"].(string)
	if !strings.HasPrefix(siteDepID, "dep-") || len(depStore.byApp[siteID]) != 1 {
		t.Fatalf("static site deploy = %v rows=%d, want 1", siteDepID, len(depStore.byApp[siteID]))
	}

	// 5. Suspended service saves cleanly with ZERO deploys and null latestDeployId.
	data = gql(`mutation { setRepo(id: "` + suspID + `", repo: "https://github.com/acme/new", branch: "main", deploy: true) { repo latestDeployId } }`)
	gotSusp := data["setRepo"].(map[string]any)
	if gotSusp["repo"] != "https://github.com/acme/new" || gotSusp["latestDeployId"] != nil {
		t.Fatalf("suspended setRepo = %+v, want saved repo + nil latestDeployId", gotSusp)
	}
	if len(depStore.byApp[suspID]) != 0 {
		t.Fatalf("suspended service opened %d deploy rows, want 0", len(depStore.byApp[suspID]))
	}

	// 6. MCP update_service and link_service_to_build_source save without opening deploys.
	before := len(depStore.byApp[webID])
	mcpCall("update_service", map[string]any{
		"serviceId": webID,
		"repo":      "https://github.com/acme/web-mcp",
		"branch":    "release",
	})
	if got := len(depStore.byApp[webID]); got != before {
		t.Fatalf("MCP update_service opened deploy rows: got %d, want %d", got, before)
	}
	mcpCall("link_service_to_build_source", map[string]any{
		"serviceId":     webID,
		"buildSourceId": bdsID,
	})
	if got := len(depStore.byApp[webID]); got != before {
		t.Fatalf("MCP link_service_to_build_source opened deploy rows: got %d, want %d", got, before)
	}
}
