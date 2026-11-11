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
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/249: Render does not downgrade a Key Value instance, so a Blueprint that
// declares a smaller plan than the store has is refused in the plan and in the
// sync, as the API refuses it. A sync has no confirm channel, and the
// auto-sync worker runs unattended.
func TestBlueprintRefusesAKeyValuePlanDowngrade(t *testing.T) {
	manifest := func(plan string) string {
		out := "services:\n  - name: cache\n    type: redis\n    ipAllowList: []\n"
		if plan != "" {
			out += "    plan: " + plan + "\n"
		}
		return out
	}
	svc, cl := newService(nil)
	ctx := context.Background()
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("standard")})
	if err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	storedPlan := func() string {
		t.Helper()
		var kv appv1alpha1.KeyValue
		if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil {
			t.Fatal(err)
		}
		return kv.Spec.Plan
	}
	for _, plan := range []string{"starter", "free"} {
		validation, err := svc.ValidateBlueprint(ctx, "", manifest(plan), "")
		if err != nil {
			t.Fatalf("plan with %s: %v", plan, err)
		}
		if validation.Valid || len(validation.Errors) == 0 || !strings.Contains(validation.Errors[0].Error, "cannot move to a smaller plan") {
			t.Errorf("plan with %s = valid %v, errors %+v, want the downgrade refusal", plan, validation.Valid, validation.Errors)
		}
		_, err = svc.DeployStack(ctx, DeployRequest{Manifest: manifest(plan)})
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != keyvalue.PlanDowngradeUnsupportedCode {
			t.Errorf("sync with %s = %v, want %s", plan, err, keyvalue.PlanDowngradeUnsupportedCode)
		}
		if got := storedPlan(); got != "standard" {
			t.Fatalf("refused sync with %s changed the stored plan to %q", plan, got)
		}
	}
	// A Blueprint that does not declare a plan leaves the plan alone, so it is
	// not a downgrade to the default.
	if validation, err := svc.ValidateBlueprint(ctx, "", manifest(""), ""); err != nil || !validation.Valid {
		t.Errorf("plan without a declared plan = %+v, err %v", validation, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("")}); err != nil || storedPlan() != "standard" {
		t.Errorf("sync without a declared plan: stored %q, err %v", storedPlan(), err)
	}
	// The same plan and a larger one still sync.
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("standard")}); err != nil {
		t.Errorf("sync with the current plan: %v", err)
	}
	if validation, err := svc.ValidateBlueprint(ctx, "", manifest("pro"), ""); err != nil || !validation.Valid {
		t.Errorf("plan with a larger plan = %+v, err %v", validation, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("pro")}); err != nil || storedPlan() != "pro" {
		t.Errorf("sync with a larger plan: stored %q, err %v", storedPlan(), err)
	}
}
