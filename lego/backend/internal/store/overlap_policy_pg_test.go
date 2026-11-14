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

// w4/653: under Render's Override policy a deploy behind one in progress
// starts at once (no latest-pending row, no deferred Restart), and the
// projector labels the workspace's Apps so the operator holds nothing; Wait,
// the default, still queues.
func TestPGOverridePolicyStartsTheNewDeployAtOnce(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, executing := deferredRestartApp(t, st)
	if policy, err := st.OverlappingDeployPolicy(ctx, row.TenantID); err != nil || policy != OverlapWait {
		t.Fatalf("default policy = %q (err %v), want wait", policy, err)
	}
	if err := st.SetOverlappingDeployPolicy(ctx, row.TenantID, OverlapOverride); err != nil {
		t.Fatal(err)
	}
	if err := st.SetOverlappingDeployPolicy(ctx, row.TenantID, "newest"); err == nil {
		t.Fatal("an unknown policy was stored")
	}

	started, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", executing.Generation+1, CommitInfo{}, "")
	if err != nil || started.Status != DeployCreated || started.OverlapPending {
		t.Fatalf("deploy behind one in progress under Override = %+v (err %v), want started", started, err)
	}
	if ended, _ := st.GetDeploy(ctx, row.ID, executing.ID); ended.Status != DeployCanceled || SupersedingDeployID(ended.CancelReason) != started.ID {
		t.Fatalf("deploy in progress under Override = %s (%q), want canceled, superseded by %s", ended.Status, ended.CancelReason, started.ID)
	}
	if _, deferred, err := st.CreateDeferredDeploy(ctx, Deploy{AppID: row.ID, Trigger: TriggerRestart}, DeferredRequest{}); err != nil || deferred {
		t.Fatalf("restart under Override: deferred=%v err=%v, want it to run at once", deferred, err)
	}
	desired, err := st.ListDesiredApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range desired {
		if d.ID != row.ID {
			continue
		}
		var app appv1alpha1.App
		stampLabels(&app, d, "test")
		if app.Labels[appv1alpha1.LabelOverlappingDeployPolicy] != appv1alpha1.OverlappingDeployPolicyOverride {
			t.Fatalf("Override App labels = %v, want the policy label", app.Labels)
		}
		d.OverlappingDeployPolicy = OverlapWait
		if !stampLabels(&app, d, "test") || app.Labels[appv1alpha1.LabelOverlappingDeployPolicy] != "" {
			t.Fatalf("back to Wait: labels = %v, want the policy label removed", app.Labels)
		}
	}

	if err := st.SetOverlappingDeployPolicy(ctx, row.TenantID, OverlapWait); err != nil {
		t.Fatal(err)
	}
	queued, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", started.Generation+1, CommitInfo{}, "")
	if err != nil || queued.Status != DeployQueued {
		t.Fatalf("deploy behind one in progress under Wait = %+v (err %v), want queued", queued, err)
	}
}
