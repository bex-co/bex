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

package deploys

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w1/m148: a restart keeps the commit that is running. Render's restart
// "always uses the exact same Git commit and configuration as the running
// instance"; bex's restart used to resolve the branch head, so a service pinned
// to an older commit silently shipped every commit pushed since.

const (
	runningCommit = "f3284af44e2f00fbfe2b2f10f04ae5243e1e2bdf"
	branchHead    = "dd508200284bee128caa975aa9af47a39bcb2223"
)

// seedDeploys stores rows for srv-1, newest first like the real store.
func seedDeploys(ds *fakeStore, rows ...store.Deploy) {
	base := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	for i := range rows {
		rows[i].AppID = "srv-1"
		rows[i].CreatedAt = base.Add(-time.Duration(i) * time.Minute)
	}
	ds.byApp["srv-1"] = rows
}

// branchResolver resolves every ref the way GitHub resolves the branch: a
// commit SHA to itself, the branch name to its head.
type branchResolver struct{ gotRef string }

func (r *branchResolver) ResolveCommit(_ context.Context, _, _, ref string) (store.CommitInfo, bool, error) {
	r.gotRef = ref
	if ref == "main" {
		return store.CommitInfo{Hash: branchHead}, true, nil
	}
	return store.CommitInfo{Hash: ref}, true, nil
}

func TestRestartPinsTheLiveCommitNotTheBranchHead(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit})
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	resolver := &branchResolver{}
	svc.Commits = resolver

	d, err := svc.Restart(context.Background(), "web")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if resolver.gotRef != runningCommit {
		t.Errorf("resolved ref = %q, want the live commit %q", resolver.gotRef, runningCommit)
	}
	if d.CommitID != runningCommit {
		t.Errorf("restart deploy commitId = %q, want the live commit %q (branch head is %q)", d.CommitID, runningCommit, branchHead)
	}
	if got := getApp(t, cl, "web").Spec.BuildCommit; got != runningCommit {
		t.Errorf("spec.buildCommit = %q, want %q", got, runningCommit)
	}
	if d.Trigger != store.TriggerAPI {
		t.Errorf("trigger = %q, want %q: a restart is a deploy-history row like any manual trigger", d.Trigger, store.TriggerAPI)
	}

	// Control: a deploy without commitId still builds the branch head.
	d, err = svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if d.CommitID != branchHead {
		t.Errorf("deploy latest commitId = %q, want branch head %q", d.CommitID, branchHead)
	}
}

// A restart targets the live release, not a newer deploy still building or one
// that failed.
func TestRestartTargetsTheLiveReleaseNotANewerDeploy(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds,
		store.Deploy{ID: "dep-building", Status: store.DeployBuildInProgress, Commit: branchHead, Generation: 3},
		store.Deploy{ID: "dep-failed", Status: store.DeployBuildFailed, Commit: branchHead, Generation: 2},
		store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit, Generation: 1},
	)
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))

	if _, err := svc.Restart(context.Background(), "web"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.BuildCommit; got != runningCommit {
		t.Errorf("spec.buildCommit = %q, want the live commit %q", got, runningCommit)
	}
}

// With no live deploy there is no running release to restart; deploying the
// branch head instead is exactly the bug.
func TestRestartWithoutALiveDeployIsRefused(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-failed", Status: store.DeployBuildFailed, Commit: branchHead})
	app := repoApp("web", "srv-1", "main")
	svc, cl := newService(ds, app)

	_, err := svc.Restart(context.Background(), "web")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Restart with no live deploy: want core.ErrConflict, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.RestartedAt; got != "" {
		t.Errorf("refused restart patched spec.restartedAt = %q", got)
	}
	if len(ds.byApp["srv-1"]) != 1 {
		t.Errorf("refused restart opened a deploy row: %+v", ds.byApp["srv-1"])
	}
}

// A live deploy whose commit was never resolved (no GitHub connection) falls
// back to the ref the running build was pinned to.
func TestRestartFallsBackToTheBuildCommitPin(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive})
	app := repoApp("web", "srv-1", "main")
	app.Spec.BuildCommit = runningCommit
	svc, cl := newService(ds, app)

	if _, err := svc.Restart(context.Background(), "web"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.BuildCommit; got != runningCommit {
		t.Errorf("spec.buildCommit = %q, want the pinned commit %q kept", got, runningCommit)
	}
}

// An image-backed service has no commit: restart redeploys its configured image
// and needs no prior live deploy.
func TestRestartImageBackedKeepsItsImage(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleApp("web", "srv-1"))

	d, err := svc.Restart(context.Background(), "web")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.Image != "web:v1" || app.Spec.BuildCommit != "" {
		t.Errorf("image-backed restart spec = image %q buildCommit %q, want web:v1 and no commit", app.Spec.Image, app.Spec.BuildCommit)
	}
	if app.Spec.RestartedAt == "" || d.ID == "" {
		t.Errorf("image-backed restart must roll a release and open a row: restartedAt %q, deploy %+v", app.Spec.RestartedAt, d)
	}
}

// commitId is refused for a cron job as caller input, but a restart's commit is
// the running one, so a repo-backed cron job restarts on it too.
func TestRestartCronJobKeepsTheLiveCommit(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit})
	app := repoApp("nightly", "srv-1", "main")
	app.Spec.Type = appv1alpha1.TypeCronJob
	svc, cl := newService(ds, app)

	if _, err := svc.Restart(context.Background(), "nightly"); err != nil {
		t.Fatalf("Restart cron job: %v", err)
	}
	if got := getApp(t, cl, "nightly").Spec.BuildCommit; got != runningCommit {
		t.Errorf("spec.buildCommit = %q, want %q", got, runningCommit)
	}
	if _, err := svc.Trigger(context.Background(), "nightly", TriggerParams{CommitID: runningCommit}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("caller commitId on a cron job: want core.ErrBadRequest, got %v", err)
	}
}

// GraphQL restartServer is the dashboard's restart and must run the same verb.
func TestGraphQLRestartServerKeepsTheLiveCommit(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit})
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	svc.Commits = &branchResolver{}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { restartServer(serviceId: "web") { id commitId trigger } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("restartServer: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["restartServer"].(map[string]any)
	if got["commitId"] != runningCommit {
		t.Errorf("restartServer commitId = %v, want the live commit %q", got["commitId"], runningCommit)
	}
	if bc := getApp(t, cl, "web").Spec.BuildCommit; bc != runningCommit {
		t.Errorf("spec.buildCommit = %q, want %q", bc, runningCommit)
	}
}
