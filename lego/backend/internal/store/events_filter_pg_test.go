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
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestPGAutoDeployFilterIsAVerbScopedBitSet is w1/m165's store-level regression
// for Render's multi-type event filter (`type=autodeploy_enabled,plan_changed`).
//
// Before it, AutoDeployFilter was one enum value applied to EVERY audit row, so
// a request for "auto-deploy enabled OR anything else" silently dropped the
// anything-else rows (their auto_deploy_enabled is NULL, which is not true), and
// "enabled OR disabled" was not expressible at all. The filter is now a bit set
// that constrains only apps.SetAutoDeploy rows.
func TestPGAutoDeployFilterIsAVerbScopedBitSet(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)

	tenant, err := st.CreateWorkspace(ctx, "autodeploy-filter-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "web", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	keys := map[string]string{}
	record := func(label, verb string, enabled *bool, at time.Time) {
		t.Helper()
		auditID := ids.New(ids.Audit)
		if _, err := pool.Exec(ctx, `
			INSERT INTO audit_events (id, workspace_id, resource, target, verb, caller, outcome, at, auto_deploy_enabled)
			VALUES ($1, $2, $3, $4, $5, 'fixture-owner', 'allowed', $6, $7)`,
			auditID, tenant.ID, core.WorkspaceObject(tenant.ID), core.ServiceTarget(app.ID), verb, at, enabled); err != nil {
			t.Fatal(err)
		}
		keys[auditID+":"] = label
	}
	on, off := true, false
	record("enabled", core.AuditVerbSetAutoDeploy, &on, base.Add(1*time.Minute))
	record("disabled", core.AuditVerbSetAutoDeploy, &off, base.Add(2*time.Minute))
	record("legacy", core.AuditVerbSetAutoDeploy, nil, base.Add(3*time.Minute))
	record("scale", "apps.Scale", nil, base.Add(4*time.Minute))

	both := []string{core.AuditVerbSetAutoDeploy, "apps.Scale"}
	for _, tc := range []struct {
		name  string
		verbs []string
		mask  AutoDeployFilter
		want  []string // labels, newest first
	}{
		{"no constraint", both, AutoDeployFilterNone, []string{"scale", "legacy", "disabled", "enabled"}},
		{"enabled alone", []string{core.AuditVerbSetAutoDeploy}, AutoDeployFilterEnabled, []string{"enabled"}},
		{"legacy changed alone", []string{core.AuditVerbSetAutoDeploy}, AutoDeployFilterChanged, []string{"legacy"}},
		// The bug: the sub-type constraint must not reach the other verb's rows.
		{"enabled plus another verb keeps that verb", both, AutoDeployFilterEnabled, []string{"scale", "enabled"}},
		{"enabled or disabled", []string{core.AuditVerbSetAutoDeploy}, AutoDeployFilterEnabled | AutoDeployFilterDisabled, []string{"disabled", "enabled"}},
		{"all three sub-types", []string{core.AuditVerbSetAutoDeploy},
			AutoDeployFilterEnabled | AutoDeployFilterDisabled | AutoDeployFilterChanged, []string{"legacy", "disabled", "enabled"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{
				Since: base, Verbs: tc.verbs, AutoDeploy: tc.mask, Limit: 100,
			})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range rows {
				if label, ok := keys[r.Key]; ok {
					got = append(got, label)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}
