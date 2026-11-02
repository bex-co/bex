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
}
