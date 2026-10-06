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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestAnInvalidResourceNameIsCodedOnEverySurface (w5/093): a service, Postgres
// or Key Value name outside the resource-name rule is refused with
// RESOURCE_NAME_INVALID on REST (400, params field and maxLength), GraphQL
// (extensions) and MCP (the text's code prefix). A Blueprint that declares
// such a name keeps the code when deployed, and its validation locates the
// refusal at the declaration's name.
func TestAnInvalidResourceNameIsCodedOnEverySurface(t *testing.T) {
	h, srv := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	cs := mcpSessionAs(t, srv, "dana")
	const bad = "Bad_Name"
	refused := codedRefusal{
		status: http.StatusBadRequest, code: "RESOURCE_NAME_INVALID",
		params: map[string]any{"field": "name", "maxLength": float64(appv1alpha1.MaxResourceNameLength)},
	}

	for _, tc := range []struct {
		kind, path, body, mutation, tool string
		args                             map[string]any
	}{{
		kind: "service", path: "/v1/services", body: `{"name":"` + bad + `","ownerId":"tea-a","image":{"ownerId":"tea-a","imagePath":"nginx"}}`,
		mutation: `mutation { createService(name: "` + bad + `", image: "nginx") { id } }`,
		tool:     "create_web_service", args: map[string]any{"name": bad, "image": "nginx", "runtime": "image"},
	}, {
		kind: "Postgres", path: "/v1/postgres", body: `{"name":"` + bad + `","plan":"free"}`,
		mutation: `mutation { createDatabase(name: "` + bad + `", plan: "free") { id } }`,
		tool:     "create_postgres", args: map[string]any{"name": bad, "plan": "free"},
	}, {
		kind: "Key Value", path: "/v1/key-value", body: `{"name":"` + bad + `","plan":"free"}`,
		mutation: `mutation { createKeyValue(name: "` + bad + `", plan: "free") { id } }`,
		tool:     "create_key_value", args: map[string]any{"name": bad, "plan": "free"},
	}} {
		t.Run(tc.kind, func(t *testing.T) {
			t.Run("REST", func(t *testing.T) { refused.onREST(t, h, http.MethodPost, tc.path, tc.body) })
			t.Run("GraphQL", func(t *testing.T) { refused.onGraphQL(t, h, tc.mutation) })
			t.Run("MCP", func(t *testing.T) { refused.onMCP(t, cs, tc.tool, tc.args) })
		})
	}

	for _, tc := range []struct{ kind, manifest, path string }{
		{"Key Value", "services:\n  - type: keyvalue\n    name: Cache_1\n    ipAllowList: []\n", "services[0].name"},
		{"service", "services:\n  - type: web\n    name: Web_1\n    runtime: image\n    image:\n      url: nginx\n", "services[0].name"},
		{"Postgres", "databases:\n  - name: Db_1\n", "databases[0].name"},
	} {
		t.Run("Blueprint "+tc.kind, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"bexYaml": tc.manifest})
			t.Run("REST deploy", func(t *testing.T) { refused.onREST(t, h, http.MethodPost, "/v1/blueprints/deploy", string(body)) })
			t.Run("MCP deploy", func(t *testing.T) { refused.onMCP(t, cs, "deploy", map[string]any{"bexYaml": tc.manifest}) })
			t.Run("GraphQL validate", func(t *testing.T) {
				query, _ := json.Marshal(tc.manifest)
				data := gql(t, h, `{ validateBlueprint(bexYaml: `+string(query)+`) { errorDetails { code path } } }`)
				details, _ := data["validateBlueprint"].(map[string]any)["errorDetails"].([]any)
				if len(details) != 1 {
					t.Fatalf("errorDetails = %v, want the one refusal", details)
				}
				if got := details[0].(map[string]any); got["code"] != "RESOURCE_NAME_INVALID" || got["path"] != tc.path {
					t.Fatalf("errorDetails = %v, want RESOURCE_NAME_INVALID at %s", got, tc.path)
				}
			})
		})
	}
}
