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
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestDeployJSONOmitsRepoAndBranch(t *testing.T) {
	raw, err := json.Marshal(Deploy{
		ID:      "dep-test",
		AppID:   "srv-test",
		Trigger: TriggerCreate,
		Repo:    "https://github.com/bex-co/bex",
		Branch:  "main",
		Status:  DeployLive,
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["repo"]; ok {
		t.Fatalf("public Deploy JSON leaked repo: %s", raw)
	}
	if _, ok := fields["branch"]; ok {
		t.Fatalf("public Deploy JSON leaked branch: %s", raw)
	}
}

func TestDeploySourceHelper(t *testing.T) {
	const repo = "https://github.com/bex-co/bex"
	for _, tc := range []struct {
		name, repo, branch, image, trigger string
		wantRepo, wantBranch               string
	}{
		{"git create", repo, "main", "", TriggerCreate, repo, "main"},
		{"git api deploy", repo, "release", "", TriggerAPI, repo, "release"},
		{"git new commit", repo, "main", "", TriggerNewCommit, repo, "main"},
		{"prebuilt image ignores default branch", "", "main", "nginx:1.27-alpine", TriggerCreate, "", ""},
		{"image override on git service", repo, "main", "nginx:1.27-alpine", TriggerAPI, "", ""},
		{"rollback", repo, "main", "registry/app@sha256:1", TriggerRollback, "", ""},
		{"static rollback with empty image", repo, "main", "", TriggerRollback, "", ""},
		{"container restart", repo, "main", "registry/app@sha256:1", TriggerRestart, "", ""},
		{"static restart with empty image", repo, "main", "", TriggerRestart, "", ""},
		{"linked build source with no own repo", "", "main", "", TriggerAPI, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotRepo, gotBranch := deploySource(tc.repo, tc.branch, tc.image, tc.trigger)
			if gotRepo != tc.wantRepo || gotBranch != tc.wantBranch {
				t.Fatalf("deploySource(%q, %q, %q, %q) = (%q, %q), want (%q, %q)",
					tc.repo, tc.branch, tc.image, tc.trigger, gotRepo, gotBranch, tc.wantRepo, tc.wantBranch)
			}
		})
	}
}

// w32/m608: every Git-backed deploy creation and promotion path snapshots the
// active repo and branch on the deploys row, later SetAppSource changes never
// mutate earlier deploy rows, non-build deploys (prebuilt image, image
// override, rollback, restart) record empty repo/branch, and pre-migration
// rows with empty repo/branch scan cleanly.
func TestPGDeploySourceRecordsAndSurvivesSourceSwitches(t *testing.T) {
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

	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("deploy-source-%d", time.Now().UnixNano()), PlanHobby, "test-source")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()

	const (
		repoA = "https://github.com/bex-co/bex"
		repoB = "https://github.com/expressjs/serve-static"
		repoC = "https://github.com/bex-co/hello-go"
		img   = "docker.io/library/nginx:1.27-alpine"
	)

	app, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: repoA, Branch: "main",
		Type: "web_service", Port: 10000, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatal(err)
	}

	assertSource := func(label, deployID, wantRepo, wantBranch string) Deploy {
		t.Helper()
		d, err := st.GetDeploy(ctx, app.ID, deployID)
		if err != nil {
			t.Fatalf("%s GetDeploy: %v", label, err)
		}
		if d.Repo != wantRepo || d.Branch != wantBranch {
			t.Fatalf("%s: got (repo=%q, branch=%q), want (%q, %q)", label, d.Repo, d.Branch, wantRepo, wantBranch)
		}
		return d
	}

	// 1. Initial deploy from CreateApp records repoA@main.
	d1 := assertSource("CreateApp", app.FirstDeployID, repoA, "main")
	if ok, err := st.TransitionDeploy(ctx, d1.ID, DeployLive, RanImage{Ref: "registry/app:1"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close d1 = (%v, %v)", ok, err)
	}

	// 2. CreateDeploy on Git service records repoA@main.
	d2, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "", 2, CommitInfo{Hash: "1111111aaaa"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if d2.Repo != repoA || d2.Branch != "main" {
		t.Fatalf("CreateDeploy returned (%q, %q), want (%q, %q)", d2.Repo, d2.Branch, repoA, "main")
	}
	assertSource("CreateDeploy", d2.ID, repoA, "main")
	if ok, err := st.TransitionDeploy(ctx, d2.ID, DeployLive, RanImage{Ref: "registry/app:2"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close d2 = (%v, %v)", ok, err)
	}

	// 3. EnsureCheckedDeploy records repoA@main.
	d3, err := st.EnsureCheckedDeploy(ctx, app.ID, 3, CommitInfo{Hash: "2222222bbbb"})
	if err != nil {
		t.Fatal(err)
	}
	assertSource("EnsureCheckedDeploy", d3.ID, repoA, "main")
	if ok, err := st.TransitionDeploy(ctx, d3.ID, DeployLive, RanImage{Ref: "registry/app:3"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close d3 = (%v, %v)", ok, err)
	}

	// 4. Image-override deploy on a repo-backed service records empty repo/branch.
	dOverride, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, img, 4, CommitInfo{}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("image override", dOverride.ID, "", "")
	if ok, err := st.TransitionDeploy(ctx, dOverride.ID, DeployLive, RanImage{Ref: img}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close dOverride = (%v, %v)", ok, err)
	}

	// 5. Rollback and restart deploys record empty repo/branch.
	dRollback, err := st.CreateRollbackDeploy(ctx, app.ID, "registry/app:2", d2.ID, 5, CommitInfo{Hash: "1111111aaaa"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("rollback", dRollback.ID, "", "")
	if ok, err := st.TransitionDeploy(ctx, dRollback.ID, DeployLive, RanImage{Ref: "registry/app:2"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close dRollback = (%v, %v)", ok, err)
	}

	dRestart, err := st.CreateRestartDeploy(ctx, app.ID, "registry/app:2", 6, CommitInfo{Hash: "1111111aaaa"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("container restart", dRestart.ID, "", "")
	if ok, err := st.TransitionDeploy(ctx, dRestart.ID, DeployLive, RanImage{Ref: "registry/app:2"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close dRestart = (%v, %v)", ok, err)
	}

	dStaticRestart, err := st.CreateRestartDeploy(ctx, app.ID, "", 7, CommitInfo{Hash: "1111111aaaa"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("static restart", dStaticRestart.ID, "", "")
	// Leave dStaticRestart open so the next CreateDeferredDeploy defers behind it.

	// 6. Deferred deploy queued under repoA@main, then source switches to repoB@release
	// before promotion: promotion records repoB@release (what actually runs).
	deferredGit, deferred, err := st.CreateDeferredDeploy(ctx, Deploy{
		AppID: app.ID, Trigger: TriggerAPI, TriggeredBy: "user-1",
	}, DeferredRequest{})
	if err != nil || !deferred {
		t.Fatalf("CreateDeferredDeploy = (%+v, %v, %v)", deferredGit, deferred, err)
	}
	assertSource("waiting deferred git", deferredGit.ID, repoA, "main")

	if _, err := st.SetAppSource(ctx, app.ID, repoB, "", "release", nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, dStaticRestart.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close dStaticRestart = (%v, %v)", ok, err)
	}
	promotedGit, err := st.CreateDeploy(WithDeferredDeploy(ctx, deferredGit.ID), app.ID, TriggerAPI, "", 8, CommitInfo{Hash: "3333333cccc"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if promotedGit.Repo != repoB || promotedGit.Branch != "release" {
		t.Fatalf("promoted deferred git = (%q, %q), want (%q, %q)", promotedGit.Repo, promotedGit.Branch, repoB, "release")
	}
	assertSource("promoted deferred git", promotedGit.ID, repoB, "release")

	// Deferred rollback promoted while app is repoB@release still records empty repo/branch.
	deferredRB, deferred, err := st.CreateDeferredDeploy(ctx, Deploy{
		AppID: app.ID, Trigger: TriggerRollback, Image: "registry/app:2", RollbackOf: d2.ID, TriggeredBy: "user-1",
	}, DeferredRequest{})
	if err != nil || !deferred {
		t.Fatalf("CreateDeferredDeploy rollback = (%+v, %v, %v)", deferredRB, deferred, err)
	}
	assertSource("waiting deferred rollback", deferredRB.ID, "", "")
	if ok, err := st.TransitionDeploy(ctx, promotedGit.ID, DeployLive, RanImage{Ref: "registry/app:8"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close promotedGit = (%v, %v)", ok, err)
	}
	promotedRB, err := st.CreateRollbackDeploy(WithDeferredDeploy(ctx, deferredRB.ID), app.ID, "registry/app:2", d2.ID, 9, CommitInfo{Hash: "1111111aaaa"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("promoted deferred rollback", promotedRB.ID, "", "")
	if ok, err := st.TransitionDeploy(ctx, promotedRB.ID, DeployLive, RanImage{Ref: "registry/app:2"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close promotedRB = (%v, %v)", ok, err)
	}

	// 7. Switch branch release -> staging, then Git -> prebuilt image, then prebuilt image -> repoC@trunk.
	if _, err := st.SetAppSource(ctx, app.ID, repoB, "", "staging", nil); err != nil {
		t.Fatal(err)
	}
	dBranchSwitch, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "", 10, CommitInfo{}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("after branch switch", dBranchSwitch.ID, repoB, "staging")
	if ok, err := st.TransitionDeploy(ctx, dBranchSwitch.ID, DeployLive, RanImage{Ref: "registry/app:10"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close dBranchSwitch = (%v, %v)", ok, err)
	}

	if _, err := st.SetAppSource(ctx, app.ID, "", img, "staging", nil); err != nil {
		t.Fatal(err)
	}
	dPrebuilt, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, img, 11, CommitInfo{}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("after switch to prebuilt image", dPrebuilt.ID, "", "")
	if ok, err := st.TransitionDeploy(ctx, dPrebuilt.ID, DeployLive, RanImage{Ref: img}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("close dPrebuilt = (%v, %v)", ok, err)
	}

	if _, err := st.SetAppSource(ctx, app.ID, repoC, "", "trunk", nil); err != nil {
		t.Fatal(err)
	}
	dRepoC, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "", 12, CommitInfo{Hash: "4444444dddd"}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	assertSource("after switch back to repoC", dRepoC.ID, repoC, "trunk")

	// 8. Re-verify EVERY earlier deploy row retained its own original repo/branch.
	assertSource("historical d1 after all switches", d1.ID, repoA, "main")
	assertSource("historical d2 after all switches", d2.ID, repoA, "main")
	assertSource("historical d3 after all switches", d3.ID, repoA, "main")
	assertSource("historical override after all switches", dOverride.ID, "", "")
	assertSource("historical promotedGit after all switches", promotedGit.ID, repoB, "release")
	assertSource("historical dBranchSwitch after all switches", dBranchSwitch.ID, repoB, "staging")
	assertSource("historical dPrebuilt after all switches", dPrebuilt.ID, "", "")

	// 9. Pre-migration rows (empty repo and branch) scan cleanly via GetDeploy and ListDeploys.
	if _, err := pool.Exec(ctx, `UPDATE deploys SET repo = '', branch = '' WHERE id = $1`, d1.ID); err != nil {
		t.Fatal(err)
	}
	assertSource("pre-migration empty source", d1.ID, "", "")
	listed, err := st.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil || len(listed) == 0 {
		t.Fatalf("ListDeploys = (%d, %v)", len(listed), err)
	}
	byID := map[string]Deploy{}
	for _, row := range listed {
		byID[row.ID] = row
	}
	if byID[d1.ID].Repo != "" || byID[d2.ID].Repo != repoA || byID[promotedGit.ID].Repo != repoB || byID[dRepoC.ID].Repo != repoC || byID[dPrebuilt.ID].Repo != "" {
		t.Fatalf("ListDeploys source mismatch: d1=%q d2=%q promoted=%q repoC=%q prebuilt=%q",
			byID[d1.ID].Repo, byID[d2.ID].Repo, byID[promotedGit.ID].Repo, byID[dRepoC.ID].Repo, byID[dPrebuilt.ID].Repo)
	}

	// 10. Migration 0342 applies cleanly down and back up (verified inside a
	// rolled-back transaction so the shared test database is untouched).
	downSQL, err := migrationsFS.ReadFile("migrations/0342_deploy_source.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	upSQL, err := migrationsFS.ReadFile("migrations/0342_deploy_source.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("migration 0342 down: %v", err)
	}
	if _, err := tx.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("migration 0342 up: %v", err)
	}
	var reAddedRepo, reAddedBranch string
	if err := tx.QueryRow(ctx, `SELECT repo, branch FROM deploys WHERE id = $1`, d2.ID).Scan(&reAddedRepo, &reAddedBranch); err != nil {
		t.Fatalf("select after migration 0342 round-trip: %v", err)
	}
	if reAddedRepo != "" || reAddedBranch != "" {
		t.Fatalf("re-added columns default = (%q, %q), want empty", reAddedRepo, reAddedBranch)
	}
}
