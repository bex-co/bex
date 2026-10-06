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
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
			app, d := preDeployingDeploy(t, s, ten.ID, fmt.Sprintf("pd-close-%d-%d", run, i), tc.preDeploy)
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

// w5/085: a cancel leaves a running pre-deploy step to finish, so the canceled
// row's settled "canceled" gives way to the verdict the operator records for
// that release, together with the pre_deploy_ended fact, in one statement. No
// other row qualifies, and a repeat changes nothing.
func TestPGCanceledReleaseStepTakesItsVerdict(t *testing.T) {
	s := openLifecyclePG(t)
	ctx := context.Background()
	run := time.Now().UnixNano()
	ten, err := s.CreateTenant(ctx, fmt.Sprintf("canceled-step-%d", run), PlanPro)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), ten.ID) })
	// closeMidStep opens a deploy with its step running, then closes it as
	// close ("" leaves it open).
	closeMidStep := func(name, close string) (App, Deploy) {
		t.Helper()
		app, d := preDeployingDeploy(t, s, ten.ID, fmt.Sprintf("%s-%d", name, run), PreDeployRunning)
		if close != "" {
			if won, err := s.TransitionDeploy(ctx, d.ID, close, "", "", "", "", nil); err != nil || !won {
				t.Fatalf("close %s = (%v, %v)", close, won, err)
			}
		}
		return app, d
	}
	reread := func(app App, d Deploy) Deploy {
		t.Helper()
		got, err := s.GetDeploy(ctx, app.ID, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	ended := func(d Deploy) (status string, at time.Time, ok bool) {
		t.Helper()
		err := s.Pool.QueryRow(ctx, `SELECT status, at FROM service_event_facts WHERE source_key = $1`,
			preDeployEndedFact(d, time.Time{}, "").SourceKey).Scan(&status, &at)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, false
		}
		if err != nil {
			t.Fatal(err)
		}
		return status, at, true
	}
	finished := time.Date(2026, 10, 6, 9, 4, 0, 0, time.UTC)

	app, d := closeMidStep("canceled", DeployCanceled)
	if got := reread(app, d); got.PreDeployStatus != PreDeployCanceled {
		t.Fatalf("precondition: the close settled the step %q, want canceled", got.PreDeployStatus)
	}
	if settled, err := s.SettleCanceledPreDeploy(ctx, app.ID, d.Generation, PreDeploySucceeded, finished); err != nil || !settled {
		t.Fatalf("settle = (%v, %v), want settled", settled, err)
	}
	got := reread(app, d)
	if got.Status != DeployCanceled || got.PreDeployStatus != PreDeploySucceeded {
		t.Fatalf("settled row = %s, step %q; want canceled, step succeeded", got.Status, got.PreDeployStatus)
	}
	if status, at, ok := ended(d); !ok || status != EventStatusSucceeded || !at.Equal(finished) {
		t.Fatalf("pre_deploy_ended = %q at %s (recorded %v), want succeeded at %s", status, at, ok, finished)
	}
	if settled, err := s.SettleCanceledPreDeploy(ctx, app.ID, d.Generation, PreDeployFailed, finished); err != nil || settled {
		t.Fatalf("repeat with another verdict = (%v, %v), want a no-op", settled, err)
	}
	if again := reread(app, d); again.PreDeployStatus != PreDeploySucceeded || !again.UpdatedAt.Equal(got.UpdatedAt) {
		t.Fatalf("repeat rewrote the row: step %q, updated_at %s → %s", again.PreDeployStatus, got.UpdatedAt, again.UpdatedAt)
	}

	for i, tc := range []struct {
		name, close string
		generation  int64
	}{
		{"an open row", "", 0},
		{"another close of a running step", DeployUpdateFailed, 0},
		{"another release", DeployCanceled, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := closeMidStep(fmt.Sprintf("untouched-%d", i), tc.close)
			before := reread(app, d)
			settled, err := s.SettleCanceledPreDeploy(ctx, app.ID, d.Generation+tc.generation, PreDeploySucceeded, finished)
			if err != nil || settled {
				t.Fatalf("settle = (%v, %v), want a no-op", settled, err)
			}
			if after := reread(app, d); after.PreDeployStatus != before.PreDeployStatus {
				t.Fatalf("step %q → %q, want untouched", before.PreDeployStatus, after.PreDeployStatus)
			}
			if _, _, ok := ended(d); ok {
				t.Fatal("pre_deploy_ended recorded for a row that was not settled")
			}
		})
	}

	if _, err := s.SettleCanceledPreDeploy(ctx, app.ID, d.Generation, PreDeployRunning, finished); err == nil {
		t.Fatal("settling to running was accepted; only a verdict may settle a step")
	}
}

// preDeployingDeploy creates an app whose first deploy is in its pre-deploy
// phase, with the step at step ("" for none).
func preDeployingDeploy(t *testing.T, s *PGStore, tenantID, name, step string) (App, Deploy) {
	t.Helper()
	ctx := context.Background()
	app, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: name,
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
	if step != "" {
		if _, err := s.SetDeployPreDeployStatus(ctx, d.ID, step); err != nil {
			t.Fatalf("set pre-deploy: %v", err)
		}
	}
	return app, d
}
