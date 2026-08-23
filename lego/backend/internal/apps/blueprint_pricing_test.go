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

// blueprint_pricing_test.go covers the Blueprint estimated-pricing projection
// (w8/m18): the always-on monthly cost object attached to a valid dry-run.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// beancountManifest mirrors bex-co/discourse_docker's
// _infra/bex/bex-beancount.yaml plans: standard web + standard keyvalue +
// basic-1gb database — the milestone's definition-of-done fixture.
const beancountManifest = `services:
  - name: beancount-forum
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: standard
  - type: keyvalue
    name: beancount-forum-redis
    plan: standard
    ipAllowList: []
databases:
  - name: beancount-forum-db
    plan: basic-1gb
`

func TestValidateBlueprintEstimatedPricingBeancountFixture(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", beancountManifest)
	if err != nil || !v.Valid {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", v, err)
	}
	est := v.EstimatedPricing
	if est == nil {
		t.Fatal("EstimatedPricing missing on a valid manifest")
	}
	// standard web $17.50 + basic-1gb db $14.00 + 5 GB × $0.21 + standard KV
	// $21.00 + 5 GB × $0.21 (both datastores price their plan storage floor).
	want := map[string]string{
		"beancount-forum":       "17.50",
		"beancount-forum-db":    "15.05",
		"beancount-forum-redis": "22.05",
	}
	if len(est.Lines) != len(want) {
		t.Fatalf("lines = %+v, want %d entries", est.Lines, len(want))
	}
	for _, l := range est.Lines {
		if want[l.Name] != l.MonthlyUSD {
			t.Errorf("line %q = %s/mo, want %s", l.Name, l.MonthlyUSD, want[l.Name])
		}
	}
	if est.TotalUSD != "54.60" {
		t.Errorf("TotalUSD = %s, want 54.60", est.TotalUSD)
	}
	if len(est.Variable) != 0 {
		t.Errorf("Variable = %+v, want empty", est.Variable)
	}
	// The database line carries the instance/storage breakdown the panel's
	// tooltip renders.
	for _, l := range est.Lines {
		if l.Name == "beancount-forum-db" && (l.InstanceUSD != "14.00" || l.StorageUSD != "1.05" || l.StorageGB != 5) {
			t.Errorf("db line breakdown = %+v, want instance 14.00 + storage 1.05 (5 GB)", l)
		}
	}
}

func TestValidateBlueprintEstimatedPricingAllFree(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
databases:
  - name: db
    plan: free
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", v, err)
	}
	// Plan-less services default to bex's free tier; the object is present
	// (surfaces stay deterministic) but empty, and the dashboard hides the panel.
	if v.EstimatedPricing == nil || len(v.EstimatedPricing.Lines) != 0 || v.EstimatedPricing.TotalUSD != "0.00" {
		t.Fatalf("all-free estimate = %+v, want empty lines at 0.00", v.EstimatedPricing)
	}
}

func TestValidateBlueprintEstimatedPricingInvalidManifestHasNone(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", "services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: nginx:1}\n    plan: mega\n")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || v.EstimatedPricing != nil {
		t.Fatalf("invalid manifest must carry no estimate, got valid=%v est=%+v", v.Valid, v.EstimatedPricing)
	}
}

func TestValidateBlueprintEstimatedPricingVariableCosts(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `services:
  - name: nightly
    type: cron
    runtime: image
    image: {url: nginx:1}
    schedule: "0 0 * * *"
    plan: starter
  - name: scaled
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: starter
    numInstances: 3
databases:
  - name: db
    plan: basic-1gb
    highAvailability: {enabled: true}
    readReplicas:
      - name: db-reader
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid || v.EstimatedPricing == nil {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", v, err)
	}
	est := v.EstimatedPricing
	// cron: variable only. scaled: one base line + multi_instance variable.
	// db: primary + standby + replica lines at $15.05 each.
	wantVar := map[string]string{"nightly": pricing.VariableCron, "scaled": pricing.VariableMultiInstance}
	if len(est.Variable) != len(wantVar) {
		t.Fatalf("Variable = %+v, want %v", est.Variable, wantVar)
	}
	for _, vc := range est.Variable {
		if wantVar[vc.Name] != vc.Reason {
			t.Errorf("variable %q reason = %s, want %s", vc.Name, vc.Reason, wantVar[vc.Name])
		}
	}
	wantLines := []struct{ name, usd string }{
		{"scaled", "4.90"},
		{"db", "15.05"},
		{"db (standby)", "15.05"},
		{"db (replica)", "15.05"},
	}
	if len(est.Lines) != len(wantLines) {
		t.Fatalf("lines = %+v, want %d entries", est.Lines, len(wantLines))
	}
	for i, w := range wantLines {
		if est.Lines[i].Name != w.name || est.Lines[i].MonthlyUSD != w.usd {
			t.Errorf("line %d = %+v, want %q at %s", i, est.Lines[i], w.name, w.usd)
		}
	}
	if est.TotalUSD != "50.05" { // 4.90 + 3 × 15.05
		t.Errorf("TotalUSD = %s, want 50.05", est.TotalUSD)
	}
}

func TestValidateBlueprintEstimatedPricingExplicitDiskOverridesFloor(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `databases:
  - name: db
    plan: basic-1gb
    diskSizeGB: 20
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid || v.EstimatedPricing == nil {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", v, err)
	}
	l := v.EstimatedPricing.Lines[0]
	// 20 GB × $0.21 = $4.20 storage on top of the $14.00 instance.
	if l.MonthlyUSD != "18.20" || l.StorageGB != 20 || l.StorageUSD != "4.20" {
		t.Errorf("explicit-disk line = %+v, want 18.20 with 20 GB at 4.20", l)
	}
}

func TestValidateBlueprintEstimatedPricingStaticSitesUnpriced(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	// Static sites run no instance (object-store + shared server) and carry
	// no compute line; the schema has no plan field for them either.
	manifest := `services:
  - name: site
    type: web
    runtime: static
    repo: https://github.com/bex/site
    staticPublishPath: dist
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid || v.EstimatedPricing == nil {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", v, err)
	}
	if len(v.EstimatedPricing.Lines) != 0 || v.EstimatedPricing.TotalUSD != "0.00" {
		t.Errorf("static site must not price, got %+v", v.EstimatedPricing)
	}
}

// --- adapters: the same object over GraphQL and REST ---

func TestPreviewBlueprintCarriesEstimatedPricing(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: fakeBlueprintFetcher{contents: beancountManifest, sha: "abc1234"},
	}
	p, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "main", "")
	if err != nil {
		t.Fatalf("PreviewBlueprint: %v", err)
	}
	if !p.Found || p.Validation == nil || p.Validation.EstimatedPricing == nil {
		t.Fatalf("preview = %+v, want validation.estimatedPricing", p)
	}
	if p.Validation.EstimatedPricing.TotalUSD != "54.60" {
		t.Errorf("preview TotalUSD = %s, want 54.60", p.Validation.EstimatedPricing.TotalUSD)
	}
}

func TestGraphQLValidateBlueprintEstimatedPricing(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	schema := blueprintSchema(t, svc)

	q := fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid estimatedPricing { totalUsd lines { name resourceKind tier tierLabel monthlyUsd instanceUsd storageUsd storageGb } variable { name reason } } } }`, beancountManifest)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("validateBlueprint: %v", res.Errors)
	}
	v := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	est, ok := v["estimatedPricing"].(map[string]any)
	if !ok {
		t.Fatalf("estimatedPricing missing: %+v", v)
	}
	if est["totalUsd"] != "54.60" {
		t.Errorf("totalUsd = %v, want 54.60", est["totalUsd"])
	}
	lines := est["lines"].([]any)
	if len(lines) != 3 {
		t.Fatalf("lines = %+v, want 3", lines)
	}
	for _, raw := range lines {
		l := raw.(map[string]any)
		if l["name"] == "beancount-forum-db" {
			if l["tier"] != "basic-1gb" || l["tierLabel"] != "Basic 1gb" || l["monthlyUsd"] != "15.05" || l["instanceUsd"] != "14.00" || l["storageUsd"] != "1.05" || l["storageGb"] != 5 {
				t.Errorf("db line = %+v", l)
			}
		}
	}
}

func TestRESTValidateBlueprintEstimatedPricing(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := fmt.Sprintf(`{"bexYaml":%q}`, beancountManifest)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("validate => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.EstimatedPricing == nil || out.EstimatedPricing.TotalUSD != "54.60" || len(out.EstimatedPricing.Lines) != 3 {
		t.Fatalf("REST estimatedPricing = %+v, want 3 lines at 54.60", out.EstimatedPricing)
	}
	// The wire object matches the core shape byte-for-byte (one core, thin
	// adapters): re-marshal and check the JSON field names the dashboard reads.
	raw := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	est := raw["estimatedPricing"].(map[string]any)
	if _, ok := est["totalUsd"]; !ok {
		t.Errorf("REST JSON missing totalUsd: %v", est)
	}
	if _, ok := est["lines"]; !ok {
		t.Errorf("REST JSON missing lines: %v", est)
	}
}

func TestMCPValidateBlueprintEstimatedPricing(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	result := call("validate_bex_yml", map[string]any{"bexYaml": beancountManifest})
	est, ok := result["estimatedPricing"].(map[string]any)
	if !ok {
		t.Fatalf("MCP estimatedPricing missing: %+v", result)
	}
	if est["totalUsd"] != "54.60" {
		t.Errorf("MCP totalUsd = %v, want 54.60", est["totalUsd"])
	}
	if lines, ok := est["lines"].([]any); !ok || len(lines) != 3 {
		t.Errorf("MCP lines = %+v, want 3", est["lines"])
	}
}

// TestValidateBlueprintPromotedFieldsCrossSurface (w8/m19 t008) runs one
// fixture exercising every field this milestone promoted — static
// buildCommand, dockerContext, image.creds — through REST, GraphQL, and the
// actual MCP tool, asserting all three surfaces accept it identically; a
// still-unsupported field (a service disk) is rejected at the same exact
// field path on all three.
func TestValidateBlueprintPromotedFieldsCrossSurface(t *testing.T) {
	svc := &Service{
		Base:          &core.Base{Client: fakeClient(), Namespace: "default"},
		RegistryCreds: &fakePullSecrets{credentialIDsByName: map[string]string{"acme-registry": "rgc-abc123"}},
	}
	manifest := `services:
  - name: site
    type: web
    runtime: static
    repo: https://github.com/bex/site
    buildCommand: npm run build
    staticPublishPath: dist
  - name: api
    type: web
    runtime: docker
    repo: https://github.com/bex/mono
    dockerfilePath: Dockerfile
    dockerContext: apps/api
  - name: worker
    type: worker
    runtime: image
    image:
      url: ghcr.io/acme/worker:1
      creds: {fromRegistryCreds: {name: acme-registry}}
`
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(fmt.Sprintf(`{"bexYaml":%q}`, manifest))))
	var rest BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || !rest.Valid {
		t.Fatalf("REST promoted fields: valid=%v err=%v body=%s", rest.Valid, err, rec.Body)
	}

	schema := blueprintSchema(t, svc)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errors } }`, manifest)})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	if v := res.Data.(map[string]any)["validateBlueprint"].(map[string]any); v["valid"] != true {
		t.Fatalf("GraphQL promoted fields: %+v", v)
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	if result := call("validate_bex_yml", map[string]any{"bexYaml": manifest}); result["valid"] != true {
		t.Fatalf("MCP promoted fields: %+v", result)
	}

	// A still-unsupported field rejects at the identical path on all three.
	// `disk` is no longer one — ADR082 D7 made it a translated handler — so the
	// cross-surface refusal is now demonstrated with `region`, which bex still
	// cannot honestly provide (one configured placement, not per-resource).
	disk := manifest + `    region: frankfurt
`
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(fmt.Sprintf(`{"bexYaml":%q}`, disk))))
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || rest.Valid || len(rest.Errors) == 0 || rest.Errors[0].Path == nil || *rest.Errors[0].Path != "services[2].region" {
		t.Fatalf("REST disk rejection = %+v err=%v", rest, err)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errorDetails { path } } }`, disk)})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL disk: %v", res.Errors)
	}
	gv := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	details, _ := gv["errorDetails"].([]any)
	if gv["valid"] != false || len(details) == 0 || details[0].(map[string]any)["path"] != "services[2].region" {
		t.Fatalf("GraphQL disk rejection = %+v", gv)
	}
	mcpResult := call("validate_bex_yml", map[string]any{"bexYaml": disk})
	if mcpResult["valid"] != false {
		t.Fatalf("MCP disk rejection = %+v", mcpResult)
	}
}
