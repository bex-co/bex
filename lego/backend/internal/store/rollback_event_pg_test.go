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
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w4/690 against real Postgres: a rollback deploy's started row carries the
// deploy it restores (Render's trigger.rollbackTargetDeployId) on the feed
// and on the by-id read; the deploy it restores and its ended row carry none.
func TestPGRollbackDeployStartedRowNamesItsTarget(t *testing.T) {
	st, _, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	unique := ids.New(ids.Deploy)
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "rollback-" + unique[len(unique)-8:],
		Image: "nginx:1", Type: "web_service", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	rb, err := st.CreateRollbackDeploy(ctx, app.ID, "nginx:1", app.FirstDeployID, 2, CommitInfo{}, "user-x")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{
		Phases: []string{EventPhaseStarted, EventPhaseEnded}, Since: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	started := 0
	for _, r := range rows {
		want := ""
		if r.Key == rb.ID+":"+EventPhaseStarted {
			want = app.FirstDeployID
			started++
		}
		if r.RollbackOf != want {
			t.Errorf("feed row %s rollbackOf = %q, want %q", r.Key, r.RollbackOf, want)
		}
		lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, r.Key))
		if err != nil {
			t.Fatalf("get %s: %v", r.Key, err)
		}
		if lookup.Event.RollbackOf != want {
			t.Errorf("by-id row %s rollbackOf = %q, want %q", r.Key, lookup.Event.RollbackOf, want)
		}
	}
	if started != 1 {
		t.Fatalf("feed has %d started rows for the rollback deploy %s, want 1 (rows %+v)", started, rb.ID, rows)
	}
}
