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

package deploys

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestDeployImageShaOnEverySurface (w8/m122): Render's image.sha — the digest
// a live image deploy ran — reaches REST and MCP inside the image object and
// GraphQL as imageSha, omitted while unknown. A Git-backed deploy keeps no
// image object, so its built digest is reported nowhere.
func TestDeployImageShaOnEverySurface(t *testing.T) {
	sha := "sha256:" + strings.Repeat("c", 64)
	at := time.Date(2026, 10, 20, 5, 43, 56, 0, time.UTC)
	const ref = "docker.io/ealen/echo-server:0.9.2"
	ds := newFakeStore()
	ds.byApp["srv-1"] = []store.Deploy{
		{ID: "dep-git", AppID: "srv-1", Trigger: store.TriggerAPI, Status: store.DeployDeactivated,
			ResolvedImage: "zot.bex-registry.svc:5000/ws/web:gen-1@" + sha, ImageSHA: sha,
			CreatedAt: at.Add(-2 * time.Hour), UpdatedAt: at},
		{ID: "dep-failed", AppID: "srv-1", Trigger: store.TriggerAPI, Status: store.DeployUpdateFailed,
			Image: "docker.io/ealen/echo-server:nope", CreatedAt: at.Add(-time.Hour), UpdatedAt: at},
		{ID: "dep-live", AppID: "srv-1", Trigger: store.TriggerCreate, Status: store.DeployLive,
			Image: ref, ResolvedImage: ref, ImageSHA: sha, CreatedAt: at, UpdatedAt: at},
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	want := map[string]any{ // the REST/MCP image object per deploy; nil = absent
		"dep-live":   map[string]any{"ref": ref, "sha": sha},
		"dep-failed": map[string]any{"ref": "docker.io/ealen/echo-server:nope"},
		"dep-git":    nil,
	}
	wantSha := map[string]any{"dep-live": sha, "dep-failed": "", "dep-git": ""}
	check := func(surface, id string, raw map[string]any) {
		t.Helper()
		got, ok := raw["image"]
		if want[id] == nil {
			if ok {
				t.Errorf("%s %s image = %v, want no image object", surface, id, got)
			}
			return
		}
		if !reflect.DeepEqual(got, want[id]) {
			t.Errorf("%s %s image = %v, want %v", surface, id, got, want[id])
		}
	}

	do := newRESTHarness(t, svc)
	for id := range want {
		var raw map[string]any
		if err := json.Unmarshal(do("GET", "/v1/services/web/deploys/"+id).Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		check("REST get", id, raw)
	}
	var restList []struct {
		Deploy map[string]any `json:"deploy"`
	}
	if err := json.Unmarshal(do("GET", "/v1/services/web/deploys").Body.Bytes(), &restList); err != nil || len(restList) != len(want) {
		t.Fatalf("REST list = %+v (err %v), want every deploy", restList, err)
	}
	for _, item := range restList {
		check("REST list", item.Deploy["id"].(string), item.Deploy)
	}

	res := graphql.Do(graphql.Params{Schema: testSchema(t, svc), Context: context.Background(), RequestString: `{
		live: deploy(serviceId: "web", deployId: "dep-live") { imageSha }
		deploys(serviceId: "web") { id image imageSha }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	if got := data["live"].(map[string]any)["imageSha"]; got != sha {
		t.Errorf("GraphQL deploy imageSha = %v, want %s", got, sha)
	}
	for _, d := range data["deploys"].([]any) {
		row := d.(map[string]any)
		if id := row["id"].(string); row["imageSha"] != wantSha[id] {
			t.Errorf("GraphQL list %s imageSha = %v, want %q", id, row["imageSha"], wantSha[id])
		}
	}

	cs := newMCPSession(t, svc)
	for id := range want {
		one, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_deploy",
			Arguments: map[string]any{"serviceId": "web", "deployId": id}})
		if err != nil || one.IsError {
			t.Fatalf("get_deploy %s: %v isErr=%v", id, err, one.IsError)
		}
		var raw map[string]any
		if err := decodeStructured(one.StructuredContent, &raw); err != nil {
			t.Fatal(err)
		}
		check("MCP get_deploy", id, raw)
	}
	for _, d := range callListDeploys(t, cs, map[string]any{"serviceId": "web"}).Deploys {
		var gotSha string
		if d.Image != nil {
			gotSha = d.Image.Sha
		}
		if gotSha != wantSha[d.ID] || (d.Image == nil) != (want[d.ID] == nil) {
			t.Errorf("MCP list_deploys %s image = %+v, want sha %q", d.ID, d.Image, wantSha[d.ID])
		}
	}
}
