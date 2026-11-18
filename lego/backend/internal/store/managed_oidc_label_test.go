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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m477: Render's Managed OIDC is for Pro workspaces and higher. The
// projector labels each App of such a workspace so the operator mounts a
// token for a release that sets AWS_ROLE_ARN; below Pro, or with no issuer to
// mint, the label is absent and nothing is mounted. A plan change reaches the
// label on the next pass.
func TestReconcileLabelsManagedOIDCByWorkspacePlan(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	rec.ManagedOIDC = true
	ten, _ := st.CreateTenant(ctx, "acme", PlanHobby)
	if _, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "nginx:1", Port: 80, Replicas: 1, Tier: "starter"}); err != nil {
		t.Fatal(err)
	}
	setPlan := func(plan string) {
		st.mu.Lock()
		tenant := st.tenants[ten.ID]
		tenant.Plan = plan
		st.tenants[ten.ID] = tenant
		st.mu.Unlock()
	}
	for _, step := range []struct {
		plan   string
		issuer bool
		want   string
	}{
		{PlanHobby, true, ""},                             // the create path
		{PlanPro, true, appv1alpha1.ManagedOIDCEnabled},   // an upgrade, on the update path
		{PlanScale, true, appv1alpha1.ManagedOIDCEnabled}, // every plan above Pro
		{PlanEnterprise, true, appv1alpha1.ManagedOIDCEnabled},
		{PlanPro, false, ""},  // no issuer configured: nobody would mint
		{PlanHobby, true, ""}, // a downgrade
		{"free", true, ""},    // a plan bex does not know
	} {
		setPlan(step.plan)
		rec.ManagedOIDC = step.issuer
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatalf("%s: reconcile: %v", step.plan, err)
		}
		got, ok := getApp(t, cl).Labels[appv1alpha1.LabelManagedOIDC]
		if got != step.want || ok != (step.want != "") {
			t.Errorf("%s workspace (issuer %v): %s = %q (present %v), want %q", step.plan, step.issuer, appv1alpha1.LabelManagedOIDC, got, ok, step.want)
		}
	}
}
