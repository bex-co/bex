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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestReconcileRestoresImageCompatibilityPolicy(t *testing.T) {
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1} {
		t.Run(policy, func(t *testing.T) {
			rec, st, cl := newTestReconciler(t)
			ctx := t.Context()
			tenant, err := st.CreateTenant(ctx, "compatibility", "hobby")
			if err != nil {
				t.Fatal(err)
			}
			mode := ""
			if policy == appv1alpha1.ContainerPolicyImageV1 {
				mode = appv1alpha1.PortModeImageV1
			}
			_, err = st.CreateApp(ctx, App{
				TenantID: tenant.ID, Name: "web", Type: appv1alpha1.TypePrivateService,
				Repo: "https://github.com/render-examples/clickhouse", Branch: "master",
				Port: 3000, Replicas: 1, Tier: "standard", ContainerPolicy: policy, PortMode: mode,
				InitialDisk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/lib/clickhouse", SizeGB: 10},
			})
			if err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := rec.ReconcileOnce(ctx); err != nil {
					t.Fatal(err)
				}
				app := getApp(t, cl)
				if app.Spec.ContainerPolicy != policy || app.Spec.PortMode != mode || app.Spec.Expose {
					t.Fatalf("restored policy/exposure differs: %+v", app.Spec)
				}
				if app.Spec.Disk == nil || app.Spec.Disk.MountPath != "/var/lib/clickhouse" || app.Spec.Disk.SizeGB != 10 {
					t.Fatalf("restored service lost its disk: %+v", app.Spec.Disk)
				}
				version := app.ResourceVersion
				if err := rec.ReconcileOnce(ctx); err != nil {
					t.Fatal(err)
				}
				if getApp(t, cl).ResourceVersion != version {
					t.Fatal("steady-state projection changed the release")
				}
				if attempt == 0 {
					if err := cl.Delete(ctx, app); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestPGImageCompatibilityPolicyRoundTrip(t *testing.T) {
	st, ctx := newBillingTestStore(t)
	tenant, err := st.CreateTenant(ctx, "compatibility", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1} {
		mode := ""
		if policy == appv1alpha1.ContainerPolicyImageV1 {
			mode = appv1alpha1.PortModeImageV1
		}
		app, err := st.CreateApp(ctx, App{
			TenantID: tenant.ID, Name: "app-" + policy, Type: appv1alpha1.TypePrivateService,
			Repo: "https://github.com/render-examples/clickhouse", Branch: "master",
			Port: 3000, Replicas: 1, Tier: "standard", ContainerPolicy: policy, PortMode: mode,
			InitialDisk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/lib/clickhouse", SizeGB: 10},
		})
		if err != nil {
			t.Fatal(err)
		}
		got, err := st.GetApp(ctx, app.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ContainerPolicy != policy || got.PortMode != mode {
			t.Fatalf("persisted policy differs: %+v", got)
		}
		rows, err := st.ListDesiredApps(ctx)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.ID == app.ID {
				found = true
				if row.ContainerPolicy != policy || row.PortMode != mode {
					t.Fatalf("desired policy differs: %+v", row)
				}
				if row.Disk == nil || row.Disk.MountPath != "/var/lib/clickhouse" || row.Disk.SizeGB != 10 {
					t.Fatalf("desired service lost its disk: %+v", row.Disk)
				}
				var periods int
				if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM service_disk_sizes WHERE disk_id = $1 AND size_gb = 10 AND to_ts IS NULL`, row.Disk.ID).Scan(&periods); err != nil || periods != 1 {
					t.Fatalf("disk metering periods = %d, error = %v", periods, err)
				}
			}
		}
		if !found {
			t.Fatal("service missing from desired projection")
		}
		if mode == appv1alpha1.PortModeImageV1 {
			if err := st.SetAppPort(ctx, app.ID, 9000, appv1alpha1.PortModeImageConfiguredV1); err != nil {
				t.Fatal(err)
			}
			updated, err := st.GetApp(ctx, app.ID)
			if err != nil || updated.Port != 9000 || updated.PortMode != appv1alpha1.PortModeImageConfiguredV1 {
				t.Fatalf("explicit port did not persist: port=%d mode=%s err=%v", updated.Port, updated.PortMode, err)
			}
		}
	}
}

func TestPGInitialDiskFailureRollsBackService(t *testing.T) {
	st, ctx := newBillingTestStore(t)
	tenant, err := st.CreateTenant(ctx, "invalid-disk", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "invalid-disk", Image: "image", Branch: "main",
		Port: 3000, Replicas: 1, Tier: "standard",
		InitialDisk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: -1},
	})
	if err == nil {
		t.Fatal("invalid initial disk accepted")
	}
	var count int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM apps WHERE tenant_id = $1`, tenant.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed disk left service rows: %d, error = %v", count, err)
	}
}
