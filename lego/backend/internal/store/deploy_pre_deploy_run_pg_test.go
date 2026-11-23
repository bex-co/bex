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

// w32/m572: an open row takes its release's pre-deploy record. The launch with
// its command and native launch, the container's start, and the end with its
// exit status are each written once, by the first write that carries them; a
// terminal row keeps what it had; and none of it touches updated_at. A row
// with no step reads NULLs, and the deploy read carries every column back.
func TestPGDeployPreDeployRunStampsAnOpenRowOnce(t *testing.T) {
	ctx := context.Background()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("predeploy-run-%d", time.Now().UnixNano()), PlanHobby, "test-predeploy-run")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()
	app, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "busybox:1.36", Type: "web_service",
		Branch: "main", Port: 10000, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployPreDeployInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("start the step = (%v, %v)", ok, err)
	}
	read := func() DeployPreDeployRun {
		t.Helper()
		d, err := st.GetDeploy(ctx, app.ID, app.FirstDeployID)
		if err != nil {
			t.Fatal(err)
		}
		return d.PreDeployRun
	}
	var nulls bool
	if err := pool.QueryRow(ctx, `SELECT pre_deploy_started_at IS NULL AND pre_deploy_container_started_at IS NULL
		AND pre_deploy_finished_at IS NULL AND pre_deploy_command IS NULL AND pre_deploy_native IS NULL
		AND pre_deploy_exit_code IS NULL AND pre_deploy_oom_killed IS NULL FROM deploys WHERE id = $1`, app.FirstDeployID).Scan(&nulls); err != nil || !nulls {
		t.Fatalf("a row with no step: all NULL = %v (%v)", nulls, err)
	}
	if got := read(); got != (DeployPreDeployRun{}) {
		t.Fatalf("a row with no step reads %+v", got)
	}
	updatedAt := func() time.Time {
		t.Helper()
		d, err := st.GetDeploy(ctx, app.ID, app.FirstDeployID)
		if err != nil {
			t.Fatal(err)
		}
		return d.UpdatedAt
	}
	updated := updatedAt()
	set := func(r DeployPreDeployRun) {
		t.Helper()
		if err := st.SetDeployPreDeployRun(ctx, app.FirstDeployID, r); err != nil {
			t.Fatal(err)
		}
	}
	launched := time.Date(2026, 11, 22, 11, 29, 14, 990123000, time.UTC)
	container := time.Date(2026, 11, 22, 11, 29, 27, 0, time.UTC)
	settled := time.Date(2026, 11, 22, 11, 29, 31, 490456000, time.UTC)
	later := settled.Add(time.Minute)
	code, other := int32(127), int32(3)

	set(DeployPreDeployRun{StartedAt: &launched, Command: "npx prisma migrate deploy", Native: true})
	got := read()
	if got.StartedAt == nil || !got.StartedAt.Equal(launched) || got.Command != "npx prisma migrate deploy" || !got.Native ||
		got.ContainerStartedAt != nil || got.FinishedAt != nil || got.ExitCode != nil {
		t.Fatalf("the launch: run = %+v", got)
	}

	set(DeployPreDeployRun{StartedAt: &later, Command: "other", ContainerStartedAt: &container})
	got = read()
	if !got.StartedAt.Equal(launched) || got.Command != "npx prisma migrate deploy" || !got.Native {
		t.Fatalf("a later record moved the launch: %+v", got)
	}
	if got.ContainerStartedAt == nil || !got.ContainerStartedAt.Equal(container) {
		t.Fatalf("the container's start = %v", got.ContainerStartedAt)
	}

	set(DeployPreDeployRun{StartedAt: &launched, ContainerStartedAt: &later, FinishedAt: &settled, ExitCode: &code})
	got = read()
	if !got.ContainerStartedAt.Equal(container) || got.FinishedAt == nil || !got.FinishedAt.Equal(settled) ||
		got.ExitCode == nil || *got.ExitCode != 127 || got.OOMKilled {
		t.Fatalf("the end: run = %+v", got)
	}
	set(DeployPreDeployRun{StartedAt: &launched, FinishedAt: &later, ExitCode: &other, OOMKilled: true})
	if got := read(); !got.FinishedAt.Equal(settled) || *got.ExitCode != 127 || got.OOMKilled {
		t.Fatalf("a later end moved the record: %+v", got)
	}
	if u := updatedAt(); !u.Equal(updated) {
		t.Fatalf("updated_at moved from %v to %v: an observation is not a transition", updated, u)
	}

	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployPreDeployFailed, RanImage{}, "the pre-deploy command exited with code 127", "", "", nil); err != nil || !ok {
		t.Fatalf("close = (%v, %v)", ok, err)
	}
	closed := read()
	set(DeployPreDeployRun{StartedAt: &later, ContainerStartedAt: &later, FinishedAt: &later, ExitCode: &other})
	if got := read(); !samePreDeployRun(got, closed) {
		t.Fatalf("a closed row changed: %+v, was %+v", got, closed)
	}

	// A record whose step was refused before it launched keeps only its end.
	app2, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "worker", Image: "busybox:1.36", Type: "background_worker",
		Branch: "main", Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDeployPreDeployRun(ctx, app2.FirstDeployID, DeployPreDeployRun{FinishedAt: &settled}); err != nil {
		t.Fatal(err)
	}
	d, err := st.GetDeploy(ctx, app2.ID, app2.FirstDeployID)
	if err != nil {
		t.Fatal(err)
	}
	if run := d.PreDeployRun; run.StartedAt != nil || run.Command != "" || run.FinishedAt == nil || !run.FinishedAt.Equal(settled) || run.ExitCode != nil {
		t.Fatalf("a refused step: run = %+v", run)
	}
}

// samePreDeployRun compares two records by value.
func samePreDeployRun(a, b DeployPreDeployRun) bool {
	sameTime := func(x, y *time.Time) bool { return (x == nil) == (y == nil) && (x == nil || x.Equal(*y)) }
	sameCode := func(x, y *int32) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	return sameTime(a.StartedAt, b.StartedAt) && sameTime(a.ContainerStartedAt, b.ContainerStartedAt) && sameTime(a.FinishedAt, b.FinishedAt) &&
		a.Command == b.Command && a.Native == b.Native && sameCode(a.ExitCode, b.ExitCode) && a.OOMKilled == b.OOMKilled
}
