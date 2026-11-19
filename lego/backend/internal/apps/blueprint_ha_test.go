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

// TestBlueprintSyncEnablingHAJudgesTheKeptPlan: a sync that enables high
// availability while omitting plan judges the plan it keeps, never the create
// default — the same kept-plan rule the paused-tier gate follows. On a capable
// kept plan the sync plans an update naming highAvailability and applies it;
// on an incapable one both plan and apply refuse, naming the kept plan.
func TestBlueprintSyncEnablingHAJudgesTheKeptPlan(t *testing.T) {
	cl := fakeClient()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: "databases:\n  - name: big\n    plan: 1c-2g\n"}); err != nil {
		t.Fatalf("create on 1c-2g: %v", err)
	}
	const enable = "databases:\n  - name: big\n    highAvailability:\n      enabled: true\n"
	validation, err := svc.ValidateBlueprint(ctx, "", enable, "")
	if err != nil || !validation.Valid {
		t.Fatalf("enabling HA on kept 1c-2g must validate: %+v, %v", validation, err)
	}
	var planned *BlueprintPlanAction
	if validation.Plan != nil {
		for i := range validation.Plan.Actions {
			if validation.Plan.Actions[i].Name == "big" {
				planned = &validation.Plan.Actions[i]
			}
		}
	}
	if planned == nil || planned.Operation != BlueprintPlanUpdate {
		t.Fatalf("big plan = %+v, want an update", planned)
	}
	found := false
	for _, change := range planned.ChangedFields {
		found = found || change.Path == "highAvailability"
	}
	if !found {
		t.Fatalf("big changed fields = %+v, want highAvailability", planned.ChangedFields)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: enable}); err != nil {
		t.Fatalf("enabling HA on kept 1c-2g: %v", err)
	}
	var databases appv1alpha1.DatabaseList
	if err := cl.List(ctx, &databases); err != nil || len(databases.Items) != 1 {
		t.Fatalf("database inventory: %+v, %v", databases.Items, err)
	}
	if live := databases.Items[0]; !live.Spec.HighAvailability || live.Spec.Plan != "1c-2g" {
		t.Fatalf("applied spec = %+v, want HA on kept 1c-2g", live.Spec)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: "databases:\n  - name: small\n    plan: basic-1gb\n"}); err != nil {
		t.Fatalf("create on basic-1gb: %v", err)
	}
	const enableSmall = "databases:\n  - name: small\n    highAvailability:\n      enabled: true\n"
	validation, err = svc.ValidateBlueprint(ctx, "", enableSmall, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("enabling HA on kept basic-1gb must be refused: %+v, %v", validation, err)
	}
	problem := validation.Errors[0]
	if problem.Path == nil || *problem.Path != "databases[0].plan" || !strings.Contains(problem.Error, `plan "basic-1gb"`) {
		t.Fatalf("want a refusal naming kept basic-1gb, got %+v", problem)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: enableSmall}); err == nil || !strings.Contains(err.Error(), "high availability requires") {
		t.Fatalf("enabling HA on kept basic-1gb applied: %v", err)
	}
	if err := cl.List(ctx, &databases); err != nil {
		t.Fatal(err)
	}
	for _, db := range databases.Items {
		if db.Spec.Name == "small" && db.Spec.HighAvailability {
			t.Fatalf("refused sync switched HA on: %+v", db.Spec)
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

// TestApplyBlueprintDatabaseSpecEnablingHAJudgesTheSyncPlan: turning high
// availability on judges the plan the sync leaves the database on, the same
// presence-aware gate the API patch runs — refused on an incapable plan,
// allowed on a capable one however the file spells it.
func TestApplyBlueprintDatabaseSpecEnablingHAJudgesTheSyncPlan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		plan    string
		refused bool
	}{
		{name: "capable kept plan", plan: "1c-2g"},
		{name: "incapable kept plan", plan: "basic-1gb", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dst := appv1alpha1.DatabaseSpec{Plan: tc.plan}
			changed, err := applyBlueprintDatabaseSpecForTest(&dst, appv1alpha1.DatabaseSpec{HighAvailability: true}, map[string]BlueprintField{"highAvailability": {}})
			if tc.refused {
				var conflict *BlueprintFieldConflictError
				if !errors.As(err, &conflict) || !strings.Contains(conflict.Message, "high availability requires") {
					t.Fatalf("enabling HA on %s => want HA refusal, got changed=%v err=%v", tc.plan, changed, err)
				}
				return
			}
			if err != nil || !changed || !dst.HighAvailability || dst.Plan != tc.plan {
				t.Fatalf("enabling HA on %s => allowed, got changed=%v err=%v spec=%+v", tc.plan, changed, err, dst)
			}
		})
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
