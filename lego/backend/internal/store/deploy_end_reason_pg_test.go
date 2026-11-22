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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w8/m146: a failing close stores the structured end cause, and both the
// feed and the by-id read return it on the deploy's ended row.
func TestPGDeployEndReasonReachesEventReads(t *testing.T) {
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
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("end-reason-%d", time.Now().UnixNano()), PlanHobby, "test-end-reason")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()
	app, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("start rollout = (%v, %v)", ok, err)
	}
	exit := int32(1)
	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployUpdateFailed, RanImage{}, "container exited", "", "", nil,
		DeployEnd{ExitCode: &exit, OOMKilled: true}); err != nil || !ok {
		t.Fatalf("fail rollout = (%v, %v)", ok, err)
	}
	page, err := st.ListServiceEvents(ctx, app.ID, ten.ID, ServiceEventFilter{Phases: []string{EventPhaseEnded}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Key != app.FirstDeployID+":ended" {
		t.Fatalf("ended rows = %+v, want the failed deploy", page)
	}
	want := DeployEnd{ExitCode: &exit, OOMKilled: true}
	check := func(where string, got DeployEnd) {
		t.Helper()
		if got.ExitCode == nil || *got.ExitCode != 1 || got.OOMKilled != want.OOMKilled || got.TimedOutSeconds != 0 {
			t.Fatalf("%s End = %+v, want exit 1, OOM", where, got)
		}
	}
	check("feed", page[0].End)
	lookup, err := st.GetServiceEvent(ctx, ten.ID, ids.Derive(ids.Event, page[0].Key))
	if err != nil {
		t.Fatal(err)
	}
	check("by-id", lookup.Event.End)

	// w8/383: a start command that exits 0 is stored as exit 0, not dropped,
	// so deploy_ended can report Render's earlyExit for it.
	clean, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "clean-exit", Image: "busybox:1.36", Type: "web_service",
		Branch: "main", Port: 10000, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, clean.FirstDeployID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("start clean-exit rollout = (%v, %v)", ok, err)
	}
	zero := int32(0)
	if ok, err := st.TransitionDeploy(ctx, clean.FirstDeployID, DeployUpdateFailed, RanImage{}, "container exited", "", "", nil,
		DeployEnd{ExitCode: &zero}); err != nil || !ok {
		t.Fatalf("fail clean-exit rollout = (%v, %v)", ok, err)
	}
	ended, err := st.ListServiceEvents(ctx, clean.ID, ten.ID, ServiceEventFilter{Phases: []string{EventPhaseEnded}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(ended) != 1 || ended[0].End.ExitCode == nil || *ended[0].End.ExitCode != 0 || ended[0].End.OOMKilled {
		t.Fatalf("clean-exit ended rows = %+v, want one with exit 0", ended)
	}

	// w8/391: the deploy reads carry the rollout budget a deadline close ran
	// out of, so the deploy log closes it with Render's `==> Timed Out`; a
	// close before the deadline reads zero.
	late, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "no-port", Image: "busybox:1.36", Type: "web_service",
		Branch: "main", Port: 10000, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, late.FirstDeployID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("start no-port rollout = (%v, %v)", ok, err)
	}
	if ok, err := st.TransitionDeploy(ctx, late.FirstDeployID, DeployUpdateFailed, RanImage{}, "health check never succeeded", "", "", nil,
		DeployEnd{TimedOutSeconds: 900}); err != nil || !ok {
		t.Fatalf("fail no-port rollout = (%v, %v)", ok, err)
	}
	got, err := st.GetDeploy(ctx, late.ID, late.FirstDeployID)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := st.ListDeploys(ctx, late.ID, DeployFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got.End.TimedOutSeconds != 900 || got.End.ExitCode != nil || len(listed) != 1 || listed[0].End.TimedOutSeconds != 900 {
		t.Fatalf("timed-out deploy reads %+v by id and %+v listed, want 900", got.End, listed)
	}
	// w8/393: the deploy reads carry the exit a crash's close recorded, so
	// the deploy log closes it with Render's `==> Exited with status N`.
	for _, c := range []struct {
		appID, id string
		exit      int32
		oom       bool
	}{{app.ID, app.FirstDeployID, 1, true}, {clean.ID, clean.FirstDeployID, 0, false}} {
		d, err := st.GetDeploy(ctx, c.appID, c.id)
		if err != nil || d.End.TimedOutSeconds != 0 || d.End.ExitCode == nil || *d.End.ExitCode != c.exit || d.End.OOMKilled != c.oom {
			t.Fatalf("deploy %s closed before its deadline reads (%+v, %v), want exit %d, OOM %v, no budget", c.id, d.End, err, c.exit, c.oom)
		}
	}
}
