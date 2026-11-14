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

package envgroups

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// linkedBlueprintGroup is group shared (A=1) linked to the store-managed web
// (srv-web), with the link's own deploy row cleared and the clock moved on,
// so a later roll stamps a new release.
func linkedBlueprintGroup(t *testing.T) (*Service, *recordingDeploys, context.Context) {
	t.Helper()
	ctx := testCtx()
	rec := &recordingDeploys{}
	svc := trackedService(newFakeStore(), rec, "web")
	if _, err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "1"}, nil); err != nil {
		t.Fatalf("ApplyEnvGroup create: %v", err)
	}
	if err := svc.LinkEnvGroup(ctx, "shared", "web"); err != nil {
		t.Fatalf("LinkEnvGroup: %v", err)
	}
	rec.rows = nil
	svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
	return svc, rec, ctx
}

// w4/633: a Blueprint's group change rolls nothing by itself. It returns the
// roll its linked services are owed, which deploys each once, recorded as the
// Blueprint sync, not as a config_change the Blueprint's own deploy of the
// same service would cancel.
func TestApplyEnvGroupDefersItsRollToTheBlueprint(t *testing.T) {
	svc, rec, ctx := linkedBlueprintGroup(t)
	linked := getApp(t, svc.Client, "web").Spec.RestartedAt
	roll, err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "2"}, nil)
	if err != nil {
		t.Fatalf("ApplyEnvGroup: %v", err)
	}
	if roll == nil {
		t.Fatal("a changed group linked to web returned no roll")
	}
	if len(rec.rows) != 0 || getApp(t, svc.Client, "web").Spec.RestartedAt != linked {
		t.Fatalf("ApplyEnvGroup rolled web itself (deploy rows %+v)", rec.rows)
	}
	if err := roll(ctx, nil); err != nil {
		t.Fatalf("roll: %v", err)
	}
	if len(rec.rows) != 1 || rec.rows[0].AppID != "srv-web" || rec.rows[0].Trigger != store.TriggerBlueprintSync {
		t.Fatalf("deploy rows = %+v, want one Blueprint deploy of srv-web", rec.rows)
	}
	if getApp(t, svc.Client, "web").Spec.RestartedAt == linked {
		t.Fatal("the roll did not stamp a new release")
	}
}

// A service the Blueprint already deployed in the same apply is skipped: its
// release reads the group's new values.
func TestApplyEnvGroupRollSkipsADeployedService(t *testing.T) {
	svc, rec, ctx := linkedBlueprintGroup(t)
	roll, err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "2"}, nil)
	if err != nil || roll == nil {
		t.Fatalf("ApplyEnvGroup = %v, %v; want a roll", roll != nil, err)
	}
	if err := roll(ctx, map[string]bool{"srv-web": true}); err != nil {
		t.Fatalf("roll: %v", err)
	}
	if len(rec.rows) != 0 {
		t.Fatalf("deploy rows = %+v, want none for a service the Blueprint deployed", rec.rows)
	}
}

// An unchanged re-apply owes nothing; a group edit outside a Blueprint keeps
// its immediate config_change roll.
func TestApplyEnvGroupUnchangedOwesNoRoll(t *testing.T) {
	svc, rec, ctx := linkedBlueprintGroup(t)
	roll, err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "1"}, nil)
	if err != nil || roll != nil {
		t.Fatalf("unchanged ApplyEnvGroup = roll %v, err %v; want no roll", roll != nil, err)
	}
	gid := idNamed(ctx, svc, "shared")
	if _, err := svc.SetEnvGroupVars(ctx, gid, []EnvVarView{{Key: "A", Value: "dashboard"}}); err != nil {
		t.Fatalf("SetEnvGroupVars: %v", err)
	}
	if len(rec.rows) != 1 || rec.rows[0].Trigger != store.TriggerConfigChange {
		t.Fatalf("dashboard edit deploy rows = %+v, want one config_change", rec.rows)
	}
}
