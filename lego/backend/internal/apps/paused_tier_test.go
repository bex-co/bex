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

// paused_tier_test.go — the temporary Pro Ultra pause (w2/m250, user decision
// 2026-11-02): every surface that selects a compute tier refuses a paused one
// with 400 INSTANCE_TYPE_UNAVAILABLE, a service already on it is left alone,
// and InstanceTypes keeps listing it as unavailable. When the pause is
// released (the `paused` field deleted from tiers.yaml) these tests skip.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const wantPausedMessage = `"Pro Ultra" is temporarily unavailable; choose Pro Max or a smaller instance type.`

func requirePausedProUltra(t *testing.T) {
	t.Helper()
	if !tiers.Compute.Paused("pro-ultra") {
		t.Skip("pro-ultra is no longer paused (w2/m250 released)")
	}
}

// assertPausedRefusal checks err is the coded 400 with the one message.
func assertPausedRefusal(t *testing.T, what string, err error) {
	t.Helper()
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != InstanceTypeUnavailableCode {
		t.Fatalf("%s = %v, want 400 %s", what, err, InstanceTypeUnavailableCode)
	}
	if !strings.Contains(err.Error(), wantPausedMessage) {
		t.Errorf("%s message = %q, want it to contain %q", what, err.Error(), wantPausedMessage)
	}
	if coded.Params["plan"] != "pro_ultra" {
		t.Errorf("%s params = %v, want plan=pro_ultra", what, coded.Params)
	}
}

func TestNormalizeTierForTypeRefusesPausedTier(t *testing.T) {
	requirePausedProUltra(t)
	for _, plan := range []string{"pro_ultra", "8c-32g", "pro-ultra"} {
		_, err := normalizeTierForType(appv1alpha1.TypeWebService, plan, "")
		assertPausedRefusal(t, "normalizeTierForType("+plan+")", err)
	}
	if tier, err := normalizeTierForType(appv1alpha1.TypeWebService, "pro_ultra", "pro-ultra"); err != nil || tier != "pro-ultra" {
		t.Errorf("a service already on pro-ultra re-declaring it = %q, %v; want accepted", tier, err)
	}
	if tier, err := normalizeTierForType(appv1alpha1.TypeBackgroundWorker, "pro_max", ""); err != nil || tier != "pro-max" {
		t.Errorf("pro_max = %q, %v; want accepted", tier, err)
	}
	// Classification (billing gate, pricing) still resolves the paused tier.
	if tier, err := resolveTierForType(appv1alpha1.TypeWebService, "pro_ultra"); err != nil || tier != "pro-ultra" {
		t.Errorf("resolveTierForType(pro_ultra) = %q, %v; want pro-ultra", tier, err)
	}
}

func TestUnknownPlanHintsDoNotAdvertisePausedTier(t *testing.T) {
	requirePausedProUltra(t)
	for _, svcType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeBackgroundWorker} {
		_, err := normalizeTierForType(svcType, "gold", "")
		if err == nil || strings.Contains(err.Error(), "pro_ultra") || !strings.Contains(err.Error(), "pro_max") {
			t.Errorf("%s unknown-plan hint = %v, want pro_max listed and pro_ultra absent", svcType, err)
		}
	}
	if err := errFreePlanForType(appv1alpha1.TypeCronJob); strings.Contains(err.Error(), "pro_ultra") {
		t.Errorf("paid-plan hint advertises pro_ultra: %v", err)
	}
}

func TestCreateRefusesPausedTier(t *testing.T) {
	requirePausedProUltra(t)
	svc, cl := newService(nil)
	for _, dryRun := range []bool{false, true} {
		_, err := svc.Create(context.Background(), CreateRequest{Name: "ultra", Image: "nginx:1", Plan: "pro_ultra", DryRun: dryRun})
		assertPausedRefusal(t, "Create(pro_ultra)", err)
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("a refused create wrote %d App(s)", n)
	}
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "max", Image: "nginx:1", Plan: "pro_max"}); err != nil {
		t.Fatalf("Create(pro_max) = %v, want accepted", err)
	}
}

func TestSetPlanRefusesPausedTier(t *testing.T) {
	requirePausedProUltra(t)
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "web", "starter")
	seedAppWithPlan(t, cl, "already", "pro-ultra")

	_, err := svc.SetPlan(context.Background(), "web", "pro_ultra")
	assertPausedRefusal(t, "SetPlan(pro_ultra)", err)
	_, err = svc.SetPlanDryRun(context.Background(), "web", "pro_ultra")
	assertPausedRefusal(t, "SetPlanDryRun(pro_ultra)", err)
	if got := getApp(t, cl, "web").Spec.Tier; got != "starter" {
		t.Fatalf("a refused plan change wrote spec.tier=%q", got)
	}
	if _, err := svc.SetPlan(context.Background(), "web", "pro_max"); err != nil {
		t.Fatalf("SetPlan(pro_max) = %v, want accepted", err)
	}
	// A service already on the paused tier is left alone: re-sending its own
	// plan is not a new selection.
	if _, err := svc.SetPlan(context.Background(), "already", "pro_ultra"); err != nil {
		t.Fatalf("SetPlan on a pro-ultra service re-sending pro_ultra = %v, want accepted", err)
	}
}

func TestRESTRefusesPausedTier(t *testing.T) {
	requirePausedProUltra(t)
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "web", "starter")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, tc := range []struct{ method, path, body string }{
		{"PATCH", "/v1/services/web", `{"serviceDetails":{"plan":"pro_ultra"}}`},
		{"PATCH", "/v1/services/web", `{"serviceDetails":{"plan":"pro_ultra"},"dryRun":true}`},
		{"POST", "/v1/services", `{"name":"ultra","type":"web_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"pro_ultra","runtime":"image"}}`},
	} {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		var body struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusBadRequest || body.Code != InstanceTypeUnavailableCode || body.Message != wantPausedMessage {
			t.Errorf("%s %s %s => %d %s, want 400 %s", tc.method, tc.path, tc.body, rec.Code, rec.Body, InstanceTypeUnavailableCode)
		}
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "starter" {
		t.Fatalf("a refused PATCH wrote spec.tier=%q", got)
	}
}

func TestGraphQLRefusesPausedTierAndReportsAvailability(t *testing.T) {
	requirePausedProUltra(t)
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "web", "starter")
	schema, err := appsGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, q := range []string{
		`mutation { updateServicePlan(id:"web", plan:"pro_ultra") { id } }`,
		`mutation { updateServicePlan(id:"web", plan:"pro_ultra", dryRun:true) { id } }`,
		`mutation { createService(name:"ultra", image:"nginx:1", plan:"pro_ultra") { id } }`,
	} {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: context.Background()})
		if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != InstanceTypeUnavailableCode || res.Errors[0].Message != wantPausedMessage {
			t.Errorf("%s => %v, want one %s error", q, res.Errors, InstanceTypeUnavailableCode)
		}
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ instanceTypes { id available unavailableReason } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("instanceTypes: %v", res.Errors)
	}
	for _, raw := range res.Data.(map[string]any)["instanceTypes"].([]any) {
		it := raw.(map[string]any)
		paused := it["id"] == "pro_ultra"
		if it["available"] != !paused {
			t.Errorf("%v available = %v, want %v", it["id"], it["available"], !paused)
		}
		wantReason := ""
		if paused {
			wantReason = wantPausedMessage
		}
		if it["unavailableReason"] != wantReason {
			t.Errorf("%v unavailableReason = %q, want %q", it["id"], it["unavailableReason"], wantReason)
		}
	}
}

func TestInstanceTypesKeepsPausedTierListedAsUnavailable(t *testing.T) {
	requirePausedProUltra(t)
	svc, _ := newService(nil)
	types, err := svc.InstanceTypes(context.Background())
	if err != nil {
		t.Fatalf("InstanceTypes: %v", err)
	}
	if len(types) != len(tiers.Compute.IDs()) {
		t.Fatalf("InstanceTypes lists %d tiers, want all %d (a paused tier stays listed)", len(types), len(tiers.Compute.IDs()))
	}
	for _, it := range types {
		switch it.ID {
		case "pro_ultra":
			if it.Available || it.UnavailableReason != wantPausedMessage || it.Name != "Pro Ultra" {
				t.Errorf("pro_ultra = %+v, want unavailable with the paused message", it)
			}
		default:
			if !it.Available || it.UnavailableReason != "" {
				t.Errorf("%s = %+v, want available", it.ID, it)
			}
		}
	}
}

func TestMCPRefusesPausedTier(t *testing.T) {
	requirePausedProUltra(t)
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "web", "starter")
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
	defer func() { _ = cs.Close() }()

	for _, call := range []struct {
		tool string
		args map[string]any
	}{
		{"update_service", map[string]any{"serviceId": "web", "plan": "pro_ultra"}},
		{"update_service", map[string]any{"serviceId": "web", "plan": "pro_ultra", "dryRun": true}},
		{"create_web_service", map[string]any{"name": "ultra", "image": "nginx:1", "runtime": "image", "buildCommand": "", "startCommand": "", "plan": "pro_ultra"}},
	} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: call.tool, Arguments: call.args})
		if err != nil {
			t.Fatalf("%s: %v", call.tool, err)
		}
		text := ""
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text += tc.Text
			}
		}
		if !res.IsError || !strings.Contains(text, InstanceTypeUnavailableCode+": ") || !strings.Contains(text, wantPausedMessage) {
			t.Errorf("%s %v => isError=%v %q, want the %s refusal", call.tool, call.args, res.IsError, text, InstanceTypeUnavailableCode)
		}
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "starter" {
		t.Fatalf("a refused MCP plan change wrote spec.tier=%q", got)
	}
}

func TestBlueprintRefusesPausedTierForNewSelectionsOnly(t *testing.T) {
	requirePausedProUltra(t)
	manifest := `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: pro ultra
`
	svc, _ := newService(nil)
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) != 1 || v.Errors[0].Code != InstanceTypeUnavailableCode || !strings.Contains(v.Errors[0].Error, wantPausedMessage) {
		t.Fatalf("new pro ultra service validation = %+v, want one %s error", v, InstanceTypeUnavailableCode)
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantPausedMessage) {
		t.Fatalf("DeployStack(new pro ultra) = %v, want the paused refusal", err)
	}

	// An existing service already on pro-ultra re-declaring it syncs as usual.
	existing := sampleApp("web")
	existing.Spec.Image = "nginx:1"
	existing.Spec.Type = appv1alpha1.TypeWebService
	existing.Spec.Runtime = "image"
	existing.Spec.Tier = "pro-ultra"
	svc, _ = newService(nil, existing)
	v, err = svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("existing pro-ultra service validation = %+v, %v; want valid", v, err)
	}

	maxManifest := strings.Replace(manifest, "pro ultra", "pro max", 1)
	svc, _ = newService(nil)
	if v, err := svc.ValidateBlueprint(context.Background(), "", maxManifest, ""); err != nil || !v.Valid {
		t.Fatalf("pro max validation = %+v, %v; want valid", v, err)
	}
}
