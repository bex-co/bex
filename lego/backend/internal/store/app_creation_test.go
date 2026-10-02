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
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestPendingAppCreationWithholdsProjectionAndRetainsExistingCR(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "before CR", true: "after complete CR"}[existing], func(t *testing.T) {
			ctx := context.Background()
			rec, st, cl := newTestReconciler(t)
			tenant, err := st.CreateTenant(ctx, "pending", "free")
			if err != nil {
				t.Fatal(err)
			}
			row := seedApp(t, st, tenant.ID, appv1alpha1.TypeBackgroundWorker)
			row.CreationPending = true
			st.apps[row.ID] = row
			var resourceVersion string
			if existing {
				app := rec.projectApp(ctx, DesiredApp{App: row, TenantName: tenant.Name})
				app.Spec.EnvFromSecrets = []string{"initial-group"}
				if err := cl.Create(ctx, app); err != nil {
					t.Fatal(err)
				}
				resourceVersion = app.ResourceVersion
			}
			for range 2 {
				if err := rec.ReconcileOnce(ctx); err != nil {
					t.Fatal(err)
				}
				var apps appv1alpha1.AppList
				if err := cl.List(ctx, &apps); err != nil {
					t.Fatal(err)
				}
				if !existing && len(apps.Items) != 0 {
					t.Fatal("projector dispatched incomplete creation")
				}
				if existing && (len(apps.Items) != 1 || apps.Items[0].ResourceVersion != resourceVersion || len(apps.Items[0].Spec.EnvFromSecrets) != 1) {
					t.Fatalf("pending complete CR changed or deleted: %+v", apps.Items)
				}
				deploys, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
				if err != nil || len(deploys) != 1 || deploys[0].Status != DeployCreated {
					t.Fatalf("pending initial deploy was advanced: %+v, %v", deploys, err)
				}
			}
			row.CreationPending = false
			st.apps[row.ID] = row
			if err := rec.ReconcileOnce(ctx); err != nil {
				t.Fatal(err)
			}
			var apps appv1alpha1.AppList
			if err := cl.List(ctx, &apps); err != nil {
				t.Fatal(err)
			}
			if len(apps.Items) != 1 {
				t.Fatalf("completed creation failed to project: %+v", apps.Items)
			}
		})
	}
}

func TestAbandonedInitialCompositionTimesOutWithoutDispatch(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	tenant, err := st.CreateTenant(ctx, "abandoned", "free")
	if err != nil {
		t.Fatal(err)
	}
	row := seedApp(t, st, tenant.ID, appv1alpha1.TypeBackgroundWorker)
	row.CreationPending = true
	st.apps[row.ID] = row
	for id, deploy := range st.deploys {
		deploy.UpdatedAt = time.Now().Add(-24 * time.Hour)
		st.deploys[id] = deploy
	}
	for range 2 {
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps); err != nil || len(apps.Items) != 0 {
		t.Fatalf("abandoned initialization dispatched: %+v, %v", apps.Items, err)
	}
	deploys, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].Status != DeployUpdateFailed || !strings.Contains(deploys[0].FailureReason, "initial service configuration") {
		t.Fatalf("abandoned creation did not report a truthful terminal failure: %+v, %v", deploys, err)
	}
	if !st.apps[row.ID].CreationPending {
		t.Fatal("timeout released incomplete configuration")
	}
}
