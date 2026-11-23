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

package keyvalue

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

// TestCreateKeyValueRegionPolicyAcrossSurfaces pins w8/152's one region
// policy on Key Value create: a Render region is accepted and the store is
// placed in BEX_REGION; anything else, BEX_REGION's own spelling included, is
// a 400 with the shared message, which names no bex placement, on REST,
// GraphQL and MCP alike (w8/415). The create's answer and every later read
// report the Render region it was created with, as Render's do, and the
// platform's public alias when it named none (w9/105: Render's Terraform
// provider aborts an apply that reads back another region).
func TestCreateKeyValueRegionPolicyAcrossSurfaces(t *testing.T) {
	type outcome struct{ id, region, err string }
	surfaces := map[string]func(t *testing.T, svc *Service, name, region string) outcome{
		"REST": func(t *testing.T, svc *Service, name, region string) outcome {
			body := map[string]any{"name": name, "plan": "free"}
			if region != "" {
				body["region"] = region
			}
			raw, _ := json.Marshal(body)
			w := serveREST(svc, http.MethodPost, "/v1/key-value", string(raw))
			if w.Code != http.StatusCreated {
				return outcome{err: fmt.Sprintf("%d %s", w.Code, w.Body.String())}
			}
			created := decodeMap(t, w.Body.Bytes())
			id, _ := created["id"].(string)
			answered, _ := created["region"].(string)
			return outcome{id: id, region: answered}
		},
		"GraphQL": func(t *testing.T, svc *Service, name, region string) outcome {
			schema, err := kvGQLSchema(svc)
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			arg := ""
			if region != "" {
				arg = fmt.Sprintf(", region: %q", region)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString: fmt.Sprintf(`mutation { createKeyValue(name: %q, plan: "free"%s) { id region } }`, name, arg)})
			if len(res.Errors) > 0 {
				return outcome{err: res.Errors[0].Message}
			}
			created := res.Data.(map[string]any)["createKeyValue"].(map[string]any)
			return outcome{id: created["id"].(string), region: created["region"].(string)}
		},
		"MCP": func(t *testing.T, svc *Service, name, region string) outcome {
			cs := kvMCPSession(t, svc)
			defer cs.Close()
			args := map[string]any{"name": name, "plan": "free"}
			if region != "" {
				args["region"] = region
			}
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_key_value", Arguments: args})
			if err != nil {
				t.Fatalf("create_key_value: %v", err)
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
				ID     string `json:"id"`
				Region string `json:"region"`
			}
			raw, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(raw, &out)
			return outcome{id: out.ID, region: out.Region}
		},
	}
	for surface, create := range surfaces {
		t.Run(surface, func(t *testing.T) {
			for i, tc := range []struct {
				region string
				ok     bool
				want   string
			}{
				{"oregon", true, "oregon"},
				{"frankfurt", true, "frankfurt"},
				{"singapore", true, "singapore"},
				{"", true, "frankfurt"},
				{"fsn1", false, ""},
				{"Vietnam", false, ""},
			} {
				svc, _ := newService()
				svc.Metadata = resourcemeta.Config{Region: "fsn1"}
				got := create(t, svc, fmt.Sprintf("kv-%d", i), tc.region)
				if (got.err == "") != tc.ok {
					t.Errorf("region %q: err=%q, want ok=%v", tc.region, got.err, tc.ok)
					continue
				}
				if !tc.ok {
					if !strings.Contains(got.err, "region must be one of: frankfurt, ohio, oregon, singapore, virginia") || strings.Contains(got.err, "fsn1") {
						t.Errorf("region %q: error = %q, want the shared region message and no placement", tc.region, got.err)
					}
					continue
				}
				if got.region != tc.want {
					t.Errorf("region %q: create answered region %q, want %q", tc.region, got.region, tc.want)
				}
				v, err := svc.GetKeyValue(context.Background(), got.id)
				if err != nil || v.Region != tc.want {
					t.Errorf("region %q: readback region = %q (err %v), want %q", tc.region, v.Region, err, tc.want)
				}
			}
		})
	}
}
