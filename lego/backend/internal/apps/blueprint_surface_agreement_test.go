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
	"context"
	"encoding/json"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/098, w8/100, w8/101, w8/102: the CLI's REST validation, the dashboard's
// GraphQL validateBlueprint and MCP validate_bex_yml return the same refusal,
// path and line for each fix's invalid document.
func TestBlueprintRefusalsAgreeAcrossValidationSurfaces(t *testing.T) {
	const image = `"runtime":"image","image":{"url":"nginx:1"}`
	docs := map[string]string{
		"098 key value envVarKey": `{"services":[{"name":"source","type":"keyvalue","plan":"free","ipAllowList":[]},
  {"name":"caller","type":"web",` + image + `,"envVars":[{"key":"QA_TARGET","fromService":{"type":"keyvalue","name":"source","envVarKey":"QA_UNDECLARED"}}]}]}`,
		"100 pooled reference": `{"databases":[{"name":"db","plan":"free"}],"services":[{"name":"caller","type":"web",` + image + `,
  "envVars":[{"key":"PLAIN","value":"ok"},{"key":"DB_URL","fromDatabase":{"name":"db","property":"connectionPoolString"}}]}]}`,
		"101 static rule": `{"services":[{"name":"site","type":"web","runtime":"static","repo":"https://github.com/acme/site","staticPublishPath":"dist",
  "routes":[{"type":"redirect","source":"/old","destination":"/new"},{"type":"redirect","source":"no-slash","destination":"/new"}]}]}`,
		"102 shadowed copy": `{"services":[{"name":"source","type":"web",` + image + `,"envVars":[{"key":"SOURCE_VALUE","value":"source value"}]},
  {"name":"caller","type":"web",` + image + `,"envVars":[{"key":"VALUE","value":"literal"},{"key":"VALUE","fromService":{"type":"web","name":"source","envVarKey":"SOURCE_VALUE"}}]},
  {"name":"consumer","type":"web",` + image + `,"envVars":[{"key":"COPY_VALUE","fromService":{"type":"web","name":"caller","envVarKey":"VALUE"}}]}]}`,
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	schema := mustSchema(t, svc)
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	mc, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = mc.Close() })

	type detail struct {
		Error string `json:"error"`
		Path  string `json:"path"`
		Line  int    `json:"line"`
	}
	for name, doc := range docs {
		t.Run(name, func(t *testing.T) {
			direct, err := svc.ValidateBlueprint(ctx, "", doc, "")
			if err != nil || direct.Valid || len(direct.Errors) != 1 || direct.Errors[0].Path == nil || direct.Errors[0].Line == nil {
				t.Fatalf("direct validation = %+v, %v; want one located refusal", direct, err)
			}
			want := detail{direct.Errors[0].Error, *direct.Errors[0].Path, *direct.Errors[0].Line}

			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `query($y: String!) { validateBlueprint(bexYaml: $y) { valid errorDetails { error path line } } }`,
				VariableValues: map[string]any{"y": doc}})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL: %v", res.Errors)
			}
			raw, _ := json.Marshal(res.Data.(map[string]any)["validateBlueprint"])
			var gql struct {
				Valid        bool     `json:"valid"`
				ErrorDetails []detail `json:"errorDetails"`
			}
			if err := json.Unmarshal(raw, &gql); err != nil || gql.Valid || len(gql.ErrorDetails) != 1 || gql.ErrorDetails[0] != want {
				t.Fatalf("GraphQL = %s, want %+v", raw, want)
			}

			call, err := mc.CallTool(ctx, &mcp.CallToolParams{Name: "validate_bex_yml", Arguments: map[string]any{"bexYaml": doc}})
			if err != nil || call.IsError {
				t.Fatalf("MCP validate_bex_yml: %v %+v", err, call)
			}
			raw, _ = json.Marshal(call.StructuredContent)
			var m struct {
				Valid  bool     `json:"valid"`
				Errors []detail `json:"errors"`
			}
			if err := json.Unmarshal(raw, &m); err != nil || m.Valid || len(m.Errors) != 1 || m.Errors[0] != want {
				t.Fatalf("MCP = %s, want %+v", raw, want)
			}
		})
	}
}
