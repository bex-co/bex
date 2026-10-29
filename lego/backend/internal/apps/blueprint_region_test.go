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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// w8/152 (user decision D1, 2026-10-29): every surface applies one region
// policy. A Render region is accepted and the resource lands in BEX_REGION,
// in a Blueprint exactly as on create; a value outside Render's enum is still
// refused, by the pinned schema at its path.

// regionBlueprint declares `region` on every resource kind that carries it,
// at the top level and under projects[].environments[]. An empty region
// omits the key.
func regionBlueprint(region string) string {
	line := func(indent string) string {
		if region == "" {
			return ""
		}
		return indent + "region: " + region + "\n"
	}
	return `services:
  - type: web
    name: web
    runtime: image
    image: {url: nginx:1.27}
` + line("    ") + `  - type: cron
    name: nightly
    runtime: image
    image: {url: busybox:1.36}
    schedule: "0 3 * * *"
` + line("    ") + `  - type: keyvalue
    name: cache
    ipAllowList: []
` + line("    ") + `databases:
  - name: data
` + line("    ") + `projects:
  - name: shop
    environments:
      - name: production
        services:
          - type: worker
            name: queue
            runtime: image
            image: {url: busybox:1.36}
` + line("            ") + `          - type: keyvalue
            name: sessions
            ipAllowList: []
` + line("            ") + `        databases:
          - name: orders
` + line("            ")
}

func TestBlueprintRenderRegionNormalizesToBexRegion(t *testing.T) {
	t.Parallel()
	for _, region := range append([]string{""}, resourcemeta.RenderRegions...) {
		t.Run("region="+region, func(t *testing.T) {
			t.Parallel()
			manifest := regionBlueprint(region)
			if region != "" && strings.Count(manifest, "region: "+region) != 7 {
				t.Fatalf("fixture declares region on %d resources, want 7:\n%s", strings.Count(manifest, "region: "), manifest)
			}
			if _, problems := CompileBlueprintSource(manifest); len(problems) != 0 {
				t.Fatalf("region %q: problems = %+v", region, problems)
			}
			st, err := parseStack(DeployRequest{Manifest: manifest})
			if err != nil {
				t.Fatalf("parseStack(region %q): %v", region, err)
			}
			if len(st.services) != 3 || len(st.databases) != 2 || len(st.keyValues) != 2 {
				t.Fatalf("parsed services=%d databases=%d keyValues=%d, want 3/2/2", len(st.services), len(st.databases), len(st.keyValues))
			}
		})
	}

	// Outside Render's enum the schema still refuses, at the exact path —
	// including BEX_REGION's own spelling, which Render's schema does not know.
	for _, tc := range []struct{ manifest, path string }{
		{regionBlueprint("Vietnam"), "#/services/0/region"},
		{"databases:\n  - name: data\n    region: mars\n", "#/databases/0/region"},
		{"projects:\n  - name: p\n    environments:\n      - name: e\n        databases:\n          - name: data\n            region: fsn1\n", "#/projects/0/environments/0/databases/0/region"},
	} {
		_, problems := CompileBlueprintSource(tc.manifest)
		if p := findBlueprintProblemAtPath(problems, tc.path); p == nil || p.Line == 0 {
			t.Errorf("invalid region: want a located problem at %s, got %+v", tc.path, problems)
		}
	}
}

func findBlueprintProblemAtPath(problems []BlueprintSourceProblem, path string) *BlueprintSourceProblem {
	for i := range problems {
		if problems[i].Path == path {
			return &problems[i]
		}
	}
	return nil
}

// renderExportBlueprint is shaped like Render's Blueprint export: the header
// comment, unindented sequences, `region` on every service and database, and
// the empty ipAllowList and pinned postgresMajorVersion the exporter writes.
const renderExportBlueprint = `# Exported from Render on 2026-10-20T08:15:42Z
services:
- type: web
  name: shop-api
  runtime: node
  repo: https://github.com/acme/shop
  plan: starter
  envVars:
  - key: DATABASE_URL
    fromDatabase:
      name: shop-db
      property: connectionString
  - key: REDIS_URL
    fromService:
      name: shop-cache
      type: keyvalue
      property: connectionString
  region: oregon
  buildCommand: npm ci && npm run build
  startCommand: npm start
  autoDeployTrigger: commit
- type: worker
  name: shop-worker
  runtime: python
  repo: https://github.com/acme/shop
  plan: starter
  region: virginia
  buildCommand: pip install -r requirements.txt
  startCommand: python worker.py
- type: cron
  name: shop-nightly
  runtime: docker
  repo: https://github.com/acme/shop
  plan: starter
  schedule: 0 3 * * *
  region: frankfurt
  dockerfilePath: ./Dockerfile
  dockerContext: .
- type: keyvalue
  name: shop-cache
  plan: starter
  region: oregon
  ipAllowList: []
  maxmemoryPolicy: allkeys-lru
databases:
- name: shop-db
  databaseName: shop
  user: shop
  plan: basic-256mb
  region: singapore
  ipAllowList: []
  postgresMajorVersion: "16"
version: "1"
`

func TestBlueprintRenderExportWithRegionsValidatesAndPlans(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Metadata: resourcemeta.Config{Region: "fsn1"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", renderExportBlueprint, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if !v.Valid || len(v.Errors) != 0 {
		t.Fatalf("Render export: valid=%v errors=%+v", v.Valid, v.Errors)
	}
	if v.Plan == nil || len(v.Plan.Services) != 3 || len(v.Plan.Databases) != 1 || len(v.Plan.KeyValue) != 1 {
		t.Fatalf("Render export plan = %+v, want 3 services, 1 database, 1 key value", v.Plan)
	}

	// The same answer on every validate surface.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/validate", strings.NewReader(fmt.Sprintf(`{"bexYaml":%q}`, renderExportBlueprint))))
	var rest BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || !rest.Valid {
		t.Fatalf("REST validate: code=%d valid=%v err=%v body=%s", rec.Code, rest.Valid, err, rec.Body)
	}
	res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: context.Background(), RequestString: fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errors } }`, renderExportBlueprint)})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL validate: %v", res.Errors)
	}
	if got := res.Data.(map[string]any)["validateBlueprint"].(map[string]any); got["valid"] != true {
		t.Fatalf("GraphQL validate = %+v", got)
	}
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	if got := call("validate_bex_yml", map[string]any{"bexYaml": renderExportBlueprint}); got["valid"] != true {
		t.Fatalf("MCP validate = %+v", got)
	}
}

// Apply honors the same policy: the declared Render regions are accepted and
// every resource reads back in BEX_REGION. (Projects need the workspace store,
// so apply runs the top-level half; validation covers the nested half above.)
func TestBlueprintApplyWithRenderRegionReadsBackBexRegion(t *testing.T) {
	svc, _ := newService(nil)
	svc.Metadata = resourcemeta.Config{Region: "fsn1"}
	ctx := context.Background()
	manifest, _, _ := strings.Cut(regionBlueprint("oregon"), "projects:")
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("DeployStack(region oregon): %v", err)
	}
	if len(res.Databases) != 1 || len(res.KeyValues) != 1 {
		t.Fatalf("applied databases=%+v keyValues=%+v, want 1 each", res.Databases, res.KeyValues)
	}
	for _, name := range []string{"web", "nightly"} {
		v, err := svc.Get(ctx, name)
		if err != nil {
			t.Fatalf("Get(%s): %v", name, err)
		}
		if v.Region != "fsn1" {
			t.Errorf("%s region = %q, want BEX_REGION fsn1", name, v.Region)
		}
	}
}

// One policy on every create surface: a Render region (or BEX_REGION itself)
// is accepted and normalized, an absent one is fine, anything else is a 400.
func TestCreateServiceRegionPolicyAcrossSurfaces(t *testing.T) {
	type outcome struct {
		ok  bool
		err string
	}
	surfaces := map[string]func(t *testing.T, svc *Service, name, region string) outcome{
		"REST": func(t *testing.T, svc *Service, name, region string) outcome {
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			details := `"plan":"starter","runtime":"image"`
			if region != "" {
				details += fmt.Sprintf(`,"region":%q`, region)
			}
			body := fmt.Sprintf(`{"name":%q,"type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":""},"serviceDetails":{%s}}`, name, details)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
			if rec.Code != http.StatusCreated {
				return outcome{err: fmt.Sprintf("%d %s", rec.Code, rec.Body.String())}
			}
			return outcome{ok: true}
		},
		"GraphQL": func(t *testing.T, svc *Service, name, region string) outcome {
			arg := ""
			if region != "" {
				arg = fmt.Sprintf(", region: %q", region)
			}
			res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: context.Background(),
				RequestString: fmt.Sprintf(`mutation { createService(name: %q, image: "nginx:alpine", runtime: "image"%s) { region } }`, name, arg)})
			if len(res.Errors) > 0 {
				return outcome{err: res.Errors[0].Message}
			}
			return outcome{ok: true}
		},
		"MCP": func(t *testing.T, svc *Service, name, region string) outcome {
			args := map[string]any{"name": name, "image": "nginx:alpine", "runtime": "image"}
			if region != "" {
				args["region"] = region
			}
			result := callMCPTool(t, svc, "create_web_service", args)
			if result.IsError {
				return outcome{err: mcpResultText(result)}
			}
			return outcome{ok: true}
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
				{"virginia", true},
				{"", true},
				{"fsn1", true},
				{"Vietnam", false},
				{"us-east-1", false},
			} {
				svc, _ := newService(nil)
				svc.Metadata = resourcemeta.Config{Region: "fsn1"}
				got := create(t, svc, fmt.Sprintf("web-%d", i), tc.region)
				if got.ok != tc.ok {
					t.Errorf("region %q: ok=%v (%s), want ok=%v", tc.region, got.ok, got.err, tc.ok)
					continue
				}
				if tc.ok {
					v, err := svc.Get(context.Background(), fmt.Sprintf("web-%d", i))
					if err != nil || v.Region != "fsn1" {
						t.Errorf("region %q: readback region = %q (err %v), want BEX_REGION fsn1", tc.region, v.Region, err)
					}
				}
				if !tc.ok && !strings.Contains(got.err, "region must be one of: frankfurt, ohio, oregon, singapore, virginia") {
					t.Errorf("region %q: error = %q, want the shared region message", tc.region, got.err)
				}
			}
		})
	}
}

// callMCPTool calls one tool and returns the raw result, error results
// included (appsMCPClient fails the test on those).
func callMCPTool(t *testing.T, svc *Service, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func mcpResultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}
