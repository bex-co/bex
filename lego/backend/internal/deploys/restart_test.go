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
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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

// Render: restart is "Not supported for cron jobs" (w8/040). The refusal is a
// named 400 before any mutation, on the verb every surface shares. Caller
// commitId stays refused for a cron job as well.
func TestRestartCronJobIsRefused(t *testing.T) {
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit})
	app := repoApp("nightly", "srv-1", "main")
	app.Spec.Type = appv1alpha1.TypeCronJob
	svc, cl := newService(ds, app)
	before := getApp(t, cl, "nightly")

	_, err := svc.Restart(context.Background(), "nightly")
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "restart is not supported for cron jobs") {
		t.Fatalf("Restart cron job: want named 400, got %v", err)
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { restartServer(serviceId: "nightly") { id } }`})
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "not supported for cron jobs") {
		t.Fatalf("restartServer on a cron job: errors = %v", res.Errors)
	}
	if after := getApp(t, cl, "nightly"); after.Spec.RestartedAt != before.Spec.RestartedAt || after.Spec.BuildCommit != before.Spec.BuildCommit {
		t.Fatalf("refused restart changed the App: %+v", after.Spec)
	}
	if ds.nextID != 0 {
		t.Fatal("refused restart opened a deploy row")
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

// w4/m201: the operator names a release in status.activeRevision as soon as it
// serves, while history projects its row Live only later. Restart selected the
// newest Live row, so in that window it restarted the older configuration:
// rollback V1 serving → Restart ran V2, and deploy V2 serving → Restart ran V1.

const servingImage = "python@sha256:5e1f" // same image, configuration-only releases

// servingRecord writes the operator's release record for generation.
func servingRecord(name string, generation int64, command string) client.Object {
	raw, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{StartCommand: command})
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName(name, generation)},
		Data:       map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: raw},
	}
}

// servingFixture: A (generation 1) settled Live; B (generation 2, trigger
// bTrigger) still update_in_progress, though status already reports it the
// running release. Both run servingImage with different recorded commands.
func servingFixture(t *testing.T, bTrigger string, wrap func(client.WithWatch) client.Client) (*Service, client.Client, *fakeStore) {
	t.Helper()
	ds := newFakeStore()
	seedDeploys(ds,
		store.Deploy{ID: "dep-b", Trigger: bTrigger, Status: store.DeployUpdateInProgress, Image: servingImage, Generation: 2, Commit: "b"},
		store.Deploy{ID: "dep-a", Trigger: store.TriggerAPI, Status: store.DeployLive, Image: servingImage, ResolvedImage: servingImage, Generation: 1, Commit: "a"},
	)
	app := sampleApp("web", "srv-1")
	app.Generation = 2
	app.Spec.Image = "python:3.13-alpine"
	app.Spec.StartCommand = "serve --saved"
	app.Status = appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ActiveRevision: "rev-2", ReleaseGeneration: 2, Image: servingImage}
	cl := fakeClient(app, servingRecord("web", 1, "serve --a"), servingRecord("web", 2, "serve --b"))
	if wrap != nil {
		cl = wrap(cl.(client.WithWatch))
	}
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: ds}, cl, ds
}

func assertRestartedGeneration(t *testing.T, cl client.Client, d DeployView, source int64, commit string) {
	t.Helper()
	got := getApp(t, cl, "web")
	selected := got.Spec.ReleaseConfig
	if selected == nil || selected.SourceGeneration != source || selected.Image != servingImage || !selected.PreserveGroupValues {
		t.Fatalf("restart selected %+v, want source generation %d running %s", selected, source, servingImage)
	}
	if d.CommitID != commit || d.Image != servingImage {
		t.Fatalf("restart row = commit %q image %q, want %q %q", d.CommitID, d.Image, commit, servingImage)
	}
	if got.Spec.StartCommand != "serve --saved" || got.Spec.Image != "python:3.13-alpine" {
		t.Fatalf("restart rewrote saved settings: %+v", got.Spec)
	}
}

func TestRestartSelectsTheServingReleaseBeforeItsRowIsLive(t *testing.T) {
	for _, trigger := range []string{store.TriggerRollback, store.TriggerAPI} {
		t.Run(trigger, func(t *testing.T) {
			svc, cl, _ := servingFixture(t, trigger, nil)
			d, err := svc.Restart(context.Background(), "web")
			if err != nil {
				t.Fatal(err)
			}
			assertRestartedGeneration(t, cl, d, 2, "b")
		})
	}
}

// Settled control: the serving row is already Live.
func TestRestartSelectsTheSettledServingRelease(t *testing.T) {
	svc, cl, ds := servingFixture(t, store.TriggerRollback, nil)
	if _, err := ds.CloseDeploy(context.Background(), "dep-b", store.DeployLive, servingImage); err != nil {
		t.Fatal(err)
	}
	d, err := svc.Restart(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	assertRestartedGeneration(t, cl, d, 2, "b")
}

// When the serving release cannot be established, Restart is refused before
// anything changes instead of restarting the older Live configuration.
func TestRestartRefusesAnUnresolvableServingRelease(t *testing.T) {
	for name, edit := range map[string]func(*appv1alpha1.App, *fakeStore, client.Client){
		"no deploy row": func(a *appv1alpha1.App, _ *fakeStore, _ client.Client) {
			a.Status.ActiveRevision, a.Status.ReleaseGeneration = "rev-3", 3
		},
		"newer release in flight": func(a *appv1alpha1.App, _ *fakeStore, _ client.Client) {
			a.Status.ReleaseGeneration = 3
		},
		"no active image": func(a *appv1alpha1.App, _ *fakeStore, _ client.Client) {
			a.Status.Image = ""
		},
		"record evicted": func(_ *appv1alpha1.App, _ *fakeStore, cl client.Client) {
			_ = cl.Delete(context.Background(), servingRecord("web", 2, ""))
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc, cl, ds := servingFixture(t, store.TriggerRollback, nil)
			a := getApp(t, cl, "web")
			edit(a, ds, cl)
			if err := cl.Update(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			before := getApp(t, cl, "web")
			if _, err := svc.Restart(context.Background(), "web"); !errors.Is(err, core.ErrConflict) {
				t.Fatalf("Restart = %v, want a retryable conflict", err)
			}
			after := getApp(t, cl, "web")
			if after.Spec.RestartedAt != before.Spec.RestartedAt || after.Spec.ReleaseConfig != nil || len(ds.byApp["srv-1"]) != 2 {
				t.Fatalf("refused restart changed state: spec %+v rows %+v", after.Spec, ds.byApp["srv-1"])
			}
		})
	}
}

// Legacy status (no release generation) keeps the newest-Live selection.
func TestRestartLegacyStatusSelectsTheNewestLiveRelease(t *testing.T) {
	svc, cl, _ := servingFixture(t, store.TriggerAPI, nil)
	a := getApp(t, cl, "web")
	a.Status.ReleaseGeneration = 0
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	d, err := svc.Restart(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	assertRestartedGeneration(t, cl, d, 1, "a")
}

// advanceToB moves status from serving A (rev-1, B rolling out) to serving B.
func advanceToB(ctx context.Context, c client.Client) error {
	a := &appv1alpha1.App{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, a); err != nil {
		return err
	}
	a.Status.ActiveRevision = "rev-2"
	return c.Update(ctx, a)
}

// servingA rewinds the fixture to A serving while B rolls out.
func servingA(t *testing.T, cl client.Client) {
	t.Helper()
	a := getApp(t, cl, "web")
	a.Status.ActiveRevision = "rev-1"
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}

// lookupHookStore runs onLookup once, after Restart's unlocked selection read.
type lookupHookStore struct {
	*fakeStore
	onLookup func()
}

func (s *lookupHookStore) DeployIDByGeneration(ctx context.Context, appID string, generation int64) (string, error) {
	if hook := s.onLookup; hook != nil {
		s.onLookup = nil
		defer hook()
	}
	return s.fakeStore.DeployIDByGeneration(ctx, appID, generation)
}

// B starts serving while Restart waits for the trigger lock: the selection is
// made again from the App re-read under the lock.
func TestRestartReselectsAfterWaitingForTheLock(t *testing.T) {
	ctx := context.Background()
	svc, cl, ds := servingFixture(t, store.TriggerAPI, nil)
	servingA(t, cl)
	svc.Store = &lookupHookStore{fakeStore: ds, onLookup: func() {
		if err := advanceToB(ctx, cl); err != nil {
			t.Error(err)
		}
	}}
	d, err := svc.Restart(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	assertRestartedGeneration(t, cl, d, 2, "b")
}

// B starts serving between the locked re-read and the patch: the optimistic
// conflict re-reads the App and selects again rather than reusing A.
func TestRestartReselectsOnOptimisticConflict(t *testing.T) {
	ctx := context.Background()
	patches := 0
	svc, cl, _ := servingFixture(t, store.TriggerAPI, func(c client.WithWatch) client.Client {
		return interceptor.NewClient(c, interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if patches++; patches == 1 {
					if err := advanceToB(ctx, c); err != nil {
						return err
					}
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		})
	})
	servingA(t, cl)
	d, err := svc.Restart(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if patches < 2 {
		t.Fatalf("patch attempts = %d, want a conflict retry", patches)
	}
	assertRestartedGeneration(t, cl, d, 2, "b")
}
