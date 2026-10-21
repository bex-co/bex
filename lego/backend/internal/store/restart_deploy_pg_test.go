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
	"strings"
	"testing"
	"time"
)

// A Restart's row replaces the latest-pending deploy like any newer trigger,
// but the replaced row names the restart and the change it leaves undeployed
// rather than reading "Superseded by" it (w8/m119). The deploy in flight is
// left to the reconciler, which settles it from the release the restart adopts.
func TestPGRestartNamesThePendingDeployItReplaces(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ten := lifecycleTenant(t, st, "restart")
	row, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: fmt.Sprintf("web-%d", time.Now().UnixNano()), Repo: "https://example.com/acme/web.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatal(err)
	}
	history, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(history) != 1 {
		t.Fatalf("CreateApp history = %+v (err %v), want its first deploy", history, err)
	}
	executing := history[0]
	if _, err := st.TransitionDeploy(ctx, executing.ID, DeployBuildInProgress, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	pending, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", executing.Generation+1, CommitInfo{}, "")
	if err != nil || pending.Status != DeployQueued {
		t.Fatalf("pending = %+v (err %v), want queued", pending, err)
	}

	restart, err := st.CreateRestartDeploy(ctx, row.ID, "registry.example/acme/web:gen-0", executing.Generation+2, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if restart.Trigger != TriggerRestart || restart.Status != DeployQueued {
		t.Errorf("restart row = %+v, want a restart-triggered row queued behind the deploy in flight", restart)
	}
	pending, _ = st.GetDeploy(ctx, row.ID, pending.ID)
	if pending.Status != DeployCanceled || !strings.Contains(pending.CancelReason, "Canceled by restart "+restart.ID) ||
		!strings.Contains(pending.CancelReason, "was not deployed") {
		t.Errorf("replaced pending row = %q %q, want canceled naming restart %s", pending.Status, pending.CancelReason, restart.ID)
	}
	executing, _ = st.GetDeploy(ctx, row.ID, executing.ID)
	if executing.Status != DeployBuildInProgress {
		t.Errorf("deploy in flight = %q, want left open for the reconciler", executing.Status)
	}
}
