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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const haOnSubCPUPlan = `databases:
  - name: small-db
    plan: basic-1gb
    highAvailability:
      enabled: true
`

// TestValidateBlueprintRefusesHAOnSubCPUPlan: validate reports a sub-1-CPU
// plan's highAvailability at that field with a line/column, in the API's
// words (w8/m43 t002).
func TestValidateBlueprintRefusesHAOnSubCPUPlan(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", haOnSubCPUPlan, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) != 1 {
		t.Fatalf("want one error, got %+v", v)
	}
	e := v.Errors[0]
	if e.Path == nil || *e.Path != "databases[0].highAvailability" {
		t.Errorf("path = %v, want databases[0].highAvailability", e.Path)
	}
	// The mapping's value node: the `enabled: true` line under highAvailability.
	if e.Line == nil || *e.Line != 5 || e.Column == nil {
		t.Errorf("line/column = %v/%v, want line 5", e.Line, e.Column)
	}
	if !strings.Contains(e.Error, "high availability requires a Postgres plan with at least 1 CPU") || strings.Contains(e.Error, "bad request") {
		t.Errorf("error = %q", e.Error)
	}

	for _, plan := range []string{"1c-2g", "pro-4gb", "1c-4g", "basic-4gb", "2c-4g"} {
		manifest := strings.Replace(haOnSubCPUPlan, "plan: basic-1gb", "plan: "+plan, 1)
		if v, err := svc.ValidateBlueprint(context.Background(), "", manifest, ""); err != nil || !v.Valid {
			t.Errorf("plan %s offers HA and must validate clean (w2/m171): %+v err=%v", plan, v, err)
		}
	}
}

// TestApplyBlueprintDatabaseSpecRefusesPlanChangeUnderHA: a sync that moves an
// HA database to a plan without HA is a field conflict with the remedy, and
// turning HA off in the same sync is allowed.
func TestApplyBlueprintDatabaseSpecRefusesPlanChangeUnderHA(t *testing.T) {
	dst := appv1alpha1.DatabaseSpec{Plan: "basic-256mb", HighAvailability: true}
	_, err := applyBlueprintDatabaseSpecForTest(&dst, appv1alpha1.DatabaseSpec{Plan: "free"}, map[string]BlueprintField{"plan": {}})
	var conflict *BlueprintFieldConflictError
	if !errors.As(err, &conflict) || conflict.Path != "plan" || !strings.Contains(conflict.Message, "disable high availability first") {
		t.Fatalf("plan change under HA => want plan conflict, got %v", err)
	}
	changed, err := applyBlueprintDatabaseSpecForTest(&dst, appv1alpha1.DatabaseSpec{Plan: "free"},
		map[string]BlueprintField{"plan": {}, "highAvailability": {}})
	if err != nil || !changed || dst.HighAvailability || dst.Plan != "free" {
		t.Errorf("plan change + HA off => allowed, got changed=%v err=%v spec=%+v", changed, err, dst)
	}

	// Between HA-eligible plans (w2/m171) a sync keeps HA on, Render alias or not.
	dst = appv1alpha1.DatabaseSpec{Plan: "1c-2g", HighAvailability: true}
	changed, err = applyBlueprintDatabaseSpecForTest(&dst, appv1alpha1.DatabaseSpec{Plan: "basic-4gb"}, map[string]BlueprintField{"plan": {}})
	if err != nil || !changed || !dst.HighAvailability || dst.Plan != "basic-4gb" {
		t.Errorf("1c-2g → basic-4gb under HA => allowed, got changed=%v err=%v spec=%+v", changed, err, dst)
	}
	_, err = applyBlueprintDatabaseSpecForTest(&dst, appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, map[string]BlueprintField{"plan": {}})
	if !errors.As(err, &conflict) || conflict.Path != "plan" || !strings.Contains(conflict.Message, "disable high availability first") {
		t.Errorf("basic-4gb → basic-1gb under HA => want plan conflict, got %v", err)
	}
}

// TestGenerateBlueprintOmitsUnsupportedHA: a database that ran HA on a sub-1-CPU
// plan before the gate exports without highAvailability, so the generated
// Blueprint still validates and an omitted field preserves the live setting.
func TestGenerateBlueprintOmitsUnsupportedHA(t *testing.T) {
	svc := generateFixtureService()
	db := &appv1alpha1.Database{}
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-abc123"}, db); err != nil {
		t.Fatal(err)
	}
	db.Spec.Plan = "basic-1gb" // the fixture runs HA on 1c-2g; make it grandfathered
	if err := svc.Client.Update(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{PostgresIDs: []string{"dpg-abc123"}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if strings.Contains(out.Manifest, "highAvailability") {
		t.Errorf("HA on basic-1gb must not be exported:\n%s", out.Manifest)
	}
}
