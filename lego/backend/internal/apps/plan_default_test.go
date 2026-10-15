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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/122 (user decision 2026-10-15): a service created without a plan follows
// Render's 0.5c-512mb default, bex's starter, on every type that runs an
// instance; a static site has no plan; an explicit free stays valid where the
// type allows it.
func TestAnOmittedPlanFollowsRendersPaidDefault(t *testing.T) {
	for _, tc := range []struct {
		svcType, plan, want, wantErr string
	}{
		{svcType: appv1alpha1.TypeWebService, want: "starter"},
		{svcType: appv1alpha1.TypeCronJob, want: "starter"},
		{svcType: appv1alpha1.TypeBackgroundWorker, want: "starter"},
		{svcType: appv1alpha1.TypePrivateService, want: "starter"},
		{svcType: appv1alpha1.TypeStaticSite, want: tiers.Compute.Default().ID},
		{svcType: appv1alpha1.TypeWebService, plan: "free", want: "free"},
		{svcType: appv1alpha1.TypeCronJob, plan: "free", wantErr: "requires a paid plan"},
		{svcType: appv1alpha1.TypeWebService, plan: "0.5c-512mb", want: "starter"},
		{svcType: appv1alpha1.TypeBackgroundWorker, plan: "free", wantErr: "requires a paid plan"},
		{svcType: appv1alpha1.TypePrivateService, plan: "free", wantErr: "requires a paid plan"},
	} {
		t.Run(tc.svcType+"/"+tc.plan, func(t *testing.T) {
			got, err := normalizeTierForType(tc.svcType, tc.plan)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("normalizeTierForType = %q, %v; want a refusal of %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("normalizeTierForType = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// A Blueprint that omits a web service's plan is paid intent: it is priced at
// starter and trips the payment-method gate, the same as an explicit starter.
func TestAPlanLessBlueprintServiceIsPricedAndGatedAsStarter(t *testing.T) {
	manifest := `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
`
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("ValidateBlueprint: %+v, %v", v, err)
	}
	if v.EstimatedPricing == nil || len(v.EstimatedPricing.Lines) != 1 || v.EstimatedPricing.Lines[0].Tier != "starter" {
		t.Fatalf("estimate = %+v, want one starter line", v.EstimatedPricing)
	}
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatal(err)
	}
	if !stackHasPaidPlan(st) {
		t.Fatal("a plan-less web service did not count as paid intent for the payment-method gate")
	}
}

// Generate used to leave plan out for a free service, because free was the
// default. With the default now paid, that export would re-create the service
// on starter, so a free service must export plan: free and round-trip free.
func TestGenerateBlueprintSaysFreeForAFreeService(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(nil)
	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1", Plan: "free"}); err != nil {
		t.Fatal(err)
	}
	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{"web"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Manifest, "plan: free") {
		t.Fatalf("the export of a free service omits its plan:\n%s", out.Manifest)
	}
	stack := parseBlueprintStackForTest(t, out.Manifest)
	spec, err := specFromCreate(stack.services[0].req)
	if err != nil || spec.Tier != "free" {
		t.Fatalf("the export re-creates tier %q (%v), want free", spec.Tier, err)
	}
}

// A cron left on free before w8/048 keeps running, but no valid Blueprint can
// declare it: the export names the service and the remedy rather than failing
// its own validation as a bex bug.
func TestGenerateBlueprintRefusesALegacyFreeCron(t *testing.T) {
	legacy := cronApp("nightly")
	legacy.Spec.Tier = "free"
	svc, _ := newService(nil, legacy)
	_, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{"nightly"}})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_GENERATE_PAID_PLAN_REQUIRED" || !errors.Is(err, core.ErrBadRequest) ||
		!strings.Contains(err.Error(), `service "nightly": a cron_job requires a paid plan`) {
		t.Fatalf("GenerateBlueprint(legacy free cron) = %v, want the named paid-plan refusal", err)
	}
}
