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
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w32/m560: an open row takes its release's rollout record. The start, command
// and default are written once; the app starts merge distinct, oldest first,
// up to the bound, whatever order the record is read in; a terminal row keeps
// what it had; and none of it touches updated_at. A row with no record reads
// NULLs, and the deploy read carries every column back.
func TestPGDeployRolloutStampsAnOpenRowOnce(t *testing.T) {
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
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("rollout-%d", time.Now().UnixNano()), PlanHobby, "test-rollout")
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
	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("start rollout = (%v, %v)", ok, err)
	}
	read := func() Deploy {
		t.Helper()
		d, err := st.GetDeploy(ctx, app.ID, app.FirstDeployID)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	var nulls bool
	if err := pool.QueryRow(ctx, `SELECT rollout_started_at IS NULL AND app_started_at IS NULL AND rollout_command IS NULL
		AND rollout_web_concurrency IS NULL FROM deploys WHERE id = $1`, app.FirstDeployID).Scan(&nulls); err != nil || !nulls {
		t.Fatalf("a row with no record: all NULL = %v (%v)", nulls, err)
	}
	if d := read(); d.Rollout.StartedAt != nil || d.Rollout.AppStarts != nil || d.Rollout.Command != "" || d.Rollout.WebConcurrency != 0 {
		t.Fatalf("a row with no record reads %+v", d.Rollout)
	}
	updated := read().UpdatedAt

	started := time.Date(2026, 11, 22, 22, 14, 46, 270123000, time.UTC)
	at := func(s int) time.Time {
		return time.Date(2026, 11, 22, 22, 14, 58, 0, time.UTC).Add(time.Duration(s) * time.Second)
	}
	set := func(r DeployRollout) {
		t.Helper()
		if err := st.SetDeployRollout(ctx, app.FirstDeployID, r); err != nil {
			t.Fatal(err)
		}
	}
	same := func(got, want []time.Time) bool {
		return slices.EqualFunc(got, want, time.Time.Equal)
	}

	set(DeployRollout{StartedAt: &started, Command: "uvicorn app:app --port $PORT", WebConcurrency: 1})
	d := read()
	if d.Rollout.StartedAt == nil || !d.Rollout.StartedAt.Equal(started) || d.Rollout.Command != "uvicorn app:app --port $PORT" ||
		d.Rollout.WebConcurrency != 1 || len(d.Rollout.AppStarts) != 0 {
		t.Fatalf("first write: rollout = %+v", d.Rollout)
	}

	later := started.Add(time.Minute)
	set(DeployRollout{StartedAt: &later, Command: "other", WebConcurrency: 4, AppStarts: []time.Time{at(8), at(0)}})
	d = read()
	if !d.Rollout.StartedAt.Equal(started) || d.Rollout.Command != "uvicorn app:app --port $PORT" || d.Rollout.WebConcurrency != 1 {
		t.Fatalf("a later record moved the stamp: %+v", d.Rollout)
	}
	if !same(d.Rollout.AppStarts, []time.Time{at(0), at(8)}) {
		t.Fatalf("starts = %v, want them oldest first", d.Rollout.AppStarts)
	}

	set(DeployRollout{StartedAt: &started, AppStarts: []time.Time{at(8), at(4), at(0)}})
	if got := read().Rollout.AppStarts; !same(got, []time.Time{at(0), at(4), at(8)}) {
		t.Fatalf("starts = %v, want the new one merged once", got)
	}
	var many []time.Time
	for i := range 2 * appv1alpha1.MaxReleaseRolloutAppStarts {
		many = append(many, at(100+i))
	}
	set(DeployRollout{StartedAt: &started, AppStarts: many})
	got := read().Rollout.AppStarts
	if len(got) != appv1alpha1.MaxReleaseRolloutAppStarts || !got[0].Equal(at(0)) || !got[3].Equal(at(100)) {
		t.Fatalf("bounded starts = %v, want the oldest %d", got, appv1alpha1.MaxReleaseRolloutAppStarts)
	}
	if d := read(); !d.UpdatedAt.Equal(updated) {
		t.Fatalf("updated_at moved from %v to %v: an observation is not a transition", updated, d.UpdatedAt)
	}

	if ok, err := st.TransitionDeploy(ctx, app.FirstDeployID, DeployLive, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close = (%v, %v)", ok, err)
	}
	set(DeployRollout{StartedAt: &later, AppStarts: []time.Time{at(-5)}})
	if d := read(); !d.Rollout.StartedAt.Equal(started) || !same(d.Rollout.AppStarts, got) {
		t.Fatalf("a closed row changed: %+v", d.Rollout)
	}
}
