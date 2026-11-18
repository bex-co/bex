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

package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w7/m429: Render keeps 5, 15, 30 and 30 builds for rollback on Hobby, Pro,
// Scale and Enterprise. The projector labels each App with its workspace
// plan's count, which the operator's registry retention follows, and a plan
// change reaches the label on the next pass.
func TestReconcileLabelsTheBuildsTheWorkspacePlanKeeps(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", PlanHobby)
	if _, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Repo: "https://github.com/acme/web",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter"}); err != nil {
		t.Fatal(err)
	}
	setPlan := func(plan string) {
		st.mu.Lock()
		tenant := st.tenants[ten.ID]
		tenant.Plan = plan
		st.tenants[ten.ID] = tenant
		st.mu.Unlock()
	}
	for _, step := range []struct{ plan, want string }{
		{PlanHobby, "5"}, // the create path
		{PlanPro, "15"},  // an upgrade, on the update path
		{PlanScale, "30"},
		{PlanEnterprise, "30"},
		{PlanHobby, "5"}, // a downgrade
		// No count for a plan bex does not know: the label goes, and the
		// registry keeps its default.
		{"free", ""},
	} {
		setPlan(step.plan)
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatalf("%s: reconcile: %v", step.plan, err)
		}
		got, ok := getApp(t, cl).Labels[appv1alpha1.LabelKeptBuilds]
		if got != step.want || ok != (step.want != "") {
			t.Errorf("%s workspace: %s = %q (present %v), want %q", step.plan, appv1alpha1.LabelKeptBuilds, got, ok, step.want)
		}
	}
}

// ListDesiredApps reads the plan off the workspace row, so a plan change
// (UpdateTenantPlan, every plan change's one write) is in the next pass.
func TestPGDesiredAppsCarryTheWorkspacePlan(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ten := lifecycleTenant(t, st, "kept-builds")
	row, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: fmt.Sprintf("web-%d", time.Now().UnixNano()), Repo: "https://example.com/acme/web.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	label := func() string {
		t.Helper()
		desired, err := st.ListDesiredApps(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range desired {
			if d.ID == row.ID {
				var app appv1alpha1.App
				stampLabels(&app, d, "test", false)
				return app.Labels[appv1alpha1.LabelKeptBuilds]
			}
		}
		t.Fatalf("app %s is not among the desired apps", row.ID)
		return ""
	}
	if got := label(); got != "5" {
		t.Fatalf("Hobby workspace: kept builds = %q, want 5", got)
	}
	if _, err := st.UpdateTenantPlan(ctx, ten.ID, PlanScale); err != nil {
		t.Fatal(err)
	}
	if got := label(); got != "30" {
		t.Fatalf("after the upgrade to Scale: kept builds = %q, want 30", got)
	}
}
