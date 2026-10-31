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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// TestCreatePostgresRegionPolicyAcrossSurfaces pins w8/152's one region
// policy on Postgres create: a Render region (or BEX_REGION itself) is
// accepted and the database reads back in BEX_REGION; anything else is a 400
// with the shared message, on REST, GraphQL and MCP alike.
func TestCreatePostgresRegionPolicyAcrossSurfaces(t *testing.T) {
	type outcome struct{ id, err string }
	surfaces := map[string]func(t *testing.T, svc *Service, name, region string) outcome{
		"REST": func(t *testing.T, svc *Service, name, region string) outcome {
			body := map[string]any{"name": name, "plan": "free"}
			if region != "" {
				body["region"] = region
			}
			raw, _ := json.Marshal(body)
			w := serveREST(svc, http.MethodPost, "/v1/postgres", string(raw))
			if w.Code != http.StatusCreated {
				return outcome{err: fmt.Sprintf("%d %s", w.Code, w.Body.String())}
			}
			id, _ := decodeMap(t, w.Body.Bytes())["id"].(string)
			return outcome{id: id}
		},
		"GraphQL": func(t *testing.T, svc *Service, name, region string) outcome {
			schema, err := pgGQLSchema(svc)
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			arg := ""
			if region != "" {
				arg = fmt.Sprintf(", region: %q", region)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString: fmt.Sprintf(`mutation { createDatabase(name: %q, plan: "free"%s) { id region } }`, name, arg)})
			if len(res.Errors) > 0 {
				return outcome{err: res.Errors[0].Message}
			}
			return outcome{id: res.Data.(map[string]any)["createDatabase"].(map[string]any)["id"].(string)}
		},
		"MCP": func(t *testing.T, svc *Service, name, region string) outcome {
			cs, done := pgMCPSession(t, svc)
			defer done()
			args := map[string]any{"name": name, "plan": "free"}
			if region != "" {
				args["region"] = region
			}
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_postgres", Arguments: args})
			if err != nil {
				t.Fatalf("create_postgres: %v", err)
			}
			if res.IsError {
				var b strings.Builder
				for _, c := range res.Content {
					if text, ok := c.(*mcp.TextContent); ok {
						b.WriteString(text.Text)
					}
				}
				return outcome{err: b.String()}
			}
			var out struct {
				ID string `json:"id"`
			}
			raw, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(raw, &out)
			return outcome{id: out.ID}
		},
	}
	for surface, create := range surfaces {
		t.Run(surface, func(t *testing.T) {
			for i, tc := range []struct {
				region string
				ok     bool
			}{
				{"oregon", true},
				{"frankfurt", true},
				{"singapore", true},
				{"", true},
				{"fsn1", true},
				{"Vietnam", false},
			} {
				svc, _ := newService()
				svc.Metadata = resourcemeta.Config{Region: "fsn1"}
				got := create(t, svc, fmt.Sprintf("db-%d", i), tc.region)
				if (got.err == "") != tc.ok {
					t.Errorf("region %q: err=%q, want ok=%v", tc.region, got.err, tc.ok)
					continue
				}
				if !tc.ok {
					if !strings.Contains(got.err, "region must be one of: frankfurt, ohio, oregon, singapore, virginia") {
						t.Errorf("region %q: error = %q, want the shared region message", tc.region, got.err)
					}
					continue
				}
				v, err := svc.GetPostgres(context.Background(), got.id)
				if err != nil || v.Region != "frankfurt" {
					t.Errorf("region %q: readback region = %q (err %v), want public region frankfurt", tc.region, v.Region, err)
				}
			}
		})
	}
}
