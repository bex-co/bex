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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGTerminalCloseSettlesARunningPreDeploy pins w4/187 against real
// Postgres: SetDeployPreDeployStatus only writes open rows, so a deploy
// superseded mid-pre-deploy kept `running` forever. The terminal close itself
// settles it — succeeded when the release went live, failed when the gate
// timed the step out (w5/m114: it read "canceled" beside "Pre-deploy failed"),
// canceled otherwise — and leaves an already-settled or absent step alone.
func TestPGTerminalCloseSettlesARunningPreDeploy(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := NewPGStore(pool)
	run := time.Now().UnixNano()
	ten, err := s.CreateTenant(ctx, fmt.Sprintf("predeploy-close-%d", run), PlanPro)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	defer func() { _ = s.DeleteTenant(ctx, ten.ID) }()

	for i, tc := range []struct {
		name, preDeploy, close, cancelReason, want string
	}{
		{"superseded mid-step", PreDeployRunning, DeployCanceled, "Superseded by dep-next", PreDeployCanceled},
		{"went live", PreDeployRunning, DeployLive, "", PreDeploySucceeded},
		{"gate timed the step out", PreDeployRunning, DeployPreDeployFailed, "", PreDeployFailed},
		{"already failed", PreDeployFailed, DeployUpdateFailed, "", PreDeployFailed},
		{"no pre-deploy step", "", DeployCanceled, "user", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: fmt.Sprintf("pd-close-%d-%d", run, i),
				Image: "registry.example/app:v1", Branch: "main", Port: 8080, Replicas: 1, Tier: "starter"})
			if err != nil {
				t.Fatalf("create app: %v", err)
			}
			deploys, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
			if err != nil || len(deploys) != 1 {
				t.Fatalf("deploy fixture = %+v (err %v)", deploys, err)
			}
			d := deploys[0]
			if _, err := s.TransitionDeploy(ctx, d.ID, DeployPreDeployInProgress, "", "", "", "", nil); err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if tc.preDeploy != "" {
				if _, err := s.SetDeployPreDeployStatus(ctx, d.ID, tc.preDeploy); err != nil {
					t.Fatalf("set pre-deploy: %v", err)
				}
			}
			if won, err := s.TransitionDeploy(ctx, d.ID, tc.close, "", "", "", tc.cancelReason, nil); err != nil || !won {
				t.Fatalf("close %s = (%v, %v)", tc.close, won, err)
			}
			got, err := s.GetDeploy(ctx, app.ID, d.ID)
			if err != nil {
				t.Fatalf("get deploy: %v", err)
			}
			if got.Status != tc.close || got.PreDeployStatus != tc.want {
				t.Fatalf("closed deploy = status %q preDeployStatus %q, want %q / %q", got.Status, got.PreDeployStatus, tc.close, tc.want)
			}
		})
	}
}

// w5/m114: a trigger a newer one replaces says by which, on both paths that
// cancel it — coalesced out of the pending slot, or arriving after a newer
// generation is already open — instead of a bare "canceled" the user never
// asked for.
func TestPGSupersededTriggerNamesItsReplacement(t *testing.T) {
	s := openLifecyclePG(t)
	ctx := context.Background()
	run := time.Now().UnixNano()
	ten, err := s.CreateTenant(ctx, fmt.Sprintf("superseded-trigger-%d", run), PlanPro)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), ten.ID) })
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: fmt.Sprintf("superseded-%d", run),
		Image: "registry.example/app:v1", Branch: "main", Port: 8080, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	trigger := func(generation int64) Deploy {
		t.Helper()
		d, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, app.Image, generation, CommitInfo{}, "")
		if err != nil {
			t.Fatalf("trigger generation %d: %v", generation, err)
		}
		return d
	}
	reread := func(d Deploy) Deploy {
		t.Helper()
		got, err := s.GetDeploy(ctx, app.ID, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	pending := trigger(2) // queued behind the create's own active row
	newest := trigger(3)  // takes the pending slot
	if got := reread(pending); got.Status != DeployCanceled || got.CancelReason != "Superseded by "+newest.ID {
		t.Fatalf("coalesced trigger = %s %q, want canceled naming %s", got.Status, got.CancelReason, newest.ID)
	}
	late := trigger(2) // a delayed request for an older generation
	if late.Status != DeployCanceled || reread(late).CancelReason != "Superseded by "+newest.ID {
		t.Fatalf("late trigger = %s %q, want canceled naming %s", late.Status, reread(late).CancelReason, newest.ID)
	}
	if got := reread(newest); got.Status != DeployQueued || got.CancelReason != "" {
		t.Fatalf("newest trigger = %s %q, want still queued", got.Status, got.CancelReason)
	}
}

// w5/m114: the reconciler can mark an adopted overlap row's pre-deploy step
// running just before a newer trigger coalesces that row out of the pending
// slot. The coalescing close settles the step like every other close, or
// migration 0142's CHECK would refuse the newer trigger itself.
func TestPGCoalescedRowSettlesItsRunningStep(t *testing.T) {
	s := openLifecyclePG(t)
	ctx := context.Background()
	run := time.Now().UnixNano()
	ten, err := s.CreateTenant(ctx, fmt.Sprintf("coalesced-step-%d", run), PlanPro)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), ten.ID) })
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: fmt.Sprintf("coalesced-%d", run),
		Image: "registry.example/app:v1", Branch: "main", Port: 8080, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	pending, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, app.Image, 2, CommitInfo{}, "")
	if err != nil || pending.Status != DeployQueued {
		t.Fatalf("pending trigger = %+v (err %v), want queued behind the active create", pending, err)
	}
	if won, err := s.SetDeployPreDeployStatus(ctx, pending.ID, PreDeployRunning); err != nil || !won {
		t.Fatalf("mark step running = (%v, %v)", won, err)
	}
	newest, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, app.Image, 3, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("newer trigger refused: %v", err)
	}
	got, err := s.GetDeploy(ctx, app.ID, pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != DeployCanceled || got.PreDeployStatus != PreDeployCanceled || got.CancelReason != "Superseded by "+newest.ID {
		t.Fatalf("coalesced row = %s step %q reason %q, want canceled, step canceled, naming %s", got.Status, got.PreDeployStatus, got.CancelReason, newest.ID)
	}
}
