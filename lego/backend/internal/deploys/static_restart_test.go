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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/049 (QA run 128f): a static site's Restart rebuilt the live commit with
// the saved build command, root directory and publish directory, so a save
// that was never deployed went live through a "restart", or failed it. Render's
// Restart keeps "the exact same Git commit and configuration as the running
// instance": it re-serves the serving release's retained publication, as
// Rollback restores a target's (w4/m197), and the saved settings stay saved.

// savedButUndeployed is a static site serving generation 2 whose saved root
// and publish directories (and, built, its build command) were saved without
// a deploy.
func savedButUndeployed(buildCommand string) *appv1alpha1.App {
	a := staticSite()
	a.Spec.BuildCommit = ""
	a.Spec.BuildCommand = buildCommand
	a.Spec.RootDir = "examples/nope-128f"
	a.Spec.PublishPath = "missing-output"
	a.Status = appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ActiveRevision: "rev-2", ReleaseGeneration: 2, StaticPrefix: "tea-1/site/rev-2/"}
	return a
}

// assertStaticRestart checks d re-serves prefix with image as provenance, and
// that nothing saved moved: the legacy fallback pinned spec.buildCommit to
// the live commit and rebuilt from the saved settings.
func assertStaticRestart(t *testing.T, svc *Service, ds *fakeStore, app *appv1alpha1.App, d DeployView, prefix, image, commit string) {
	t.Helper()
	got := getApp(t, svc.Client, "site")
	row, err := ds.GetDeploy(context.Background(), "srv-site", d.ID)
	if err != nil {
		t.Fatalf("restart row %s: %v", d.ID, err)
	}
	sel := got.Spec.ReleaseConfig
	if sel == nil || sel.StaticPrefix != prefix || sel.Image != image || sel.Generation != row.Generation || sel.SourceGeneration != 0 {
		t.Fatalf("selection = %+v, want the serving publication %s (image %q) bound to the restart's release %d", sel, prefix, image, row.Generation)
	}
	if got.ActiveReleaseConfig() == nil {
		t.Fatal("the restart's selection must be the active release selection")
	}
	if d.CommitID != commit || row.Image != image || row.RollbackOf != "" {
		t.Fatalf("restart row = commit %q image %q rollbackOf %q, want the serving commit %q and image %q", d.CommitID, row.Image, row.RollbackOf, commit, image)
	}
	if got.Spec.BuildCommit != "" || got.Spec.RootDir != app.Spec.RootDir || got.Spec.PublishPath != app.Spec.PublishPath ||
		got.Spec.BuildCommand != app.Spec.BuildCommand || got.Spec.RestartedAt == "" {
		t.Fatalf("saved settings after restart = %+v; want them as saved, with only the restart stamp", got.Spec)
	}
}

func TestStaticRestartReservesTheServingPublication(t *testing.T) {
	ds := newFakeStore()
	liveStaticDeploys(t, ds)
	app := savedButUndeployed("")
	svc, _ := newService(ds, app, staticRecord(app, 1, "tea-1/site/rev-1/"), staticRecord(app, 2, "tea-1/site/rev-2/"))

	d, err := svc.Restart(context.Background(), "site")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	assertStaticRestart(t, svc, ds, app, d, "tea-1/site/rev-2/", "", "28aa2ab7")
}

// The QA 128f shape: a built site whose serving row is not projected Live yet.
// A no-build site's status carries no image at all, which a runtime restart
// refuses as unresolved; a static restart needs only the publication record.
func TestStaticRestartSelectsTheServingReleaseBeforeItsRowIsLive(t *testing.T) {
	for name, image := range map[string]string{"built": "zot/site:gen-2@sha256:bb", "no build": ""} {
		t.Run(name, func(t *testing.T) {
			ds := newFakeStore()
			ds.byApp["srv-site"] = []store.Deploy{
				{ID: "dep-serving", AppID: "srv-site", Trigger: store.TriggerAPI, Status: store.DeployUpdateInProgress, Generation: 2, Commit: "35252fd"},
				{ID: "dep-older", AppID: "srv-site", Trigger: store.TriggerAPI, Status: store.DeployLive, ResolvedImage: "zot/site:gen-1@sha256:aa", Generation: 1, Commit: "35252fd"},
			}
			app := savedButUndeployed("mkdir -p dist && echo QA128F-PUB2 > dist/marker.txt")
			app.Status.Image = image
			svc, _ := newService(ds, app, staticRecord(app, 1, "tea-1/site/rev-1/"), staticRecord(app, 2, "tea-1/site/rev-2/"))

			d, err := svc.Restart(context.Background(), "site")
			if err != nil {
				t.Fatalf("Restart while the serving row is not Live yet: %v", err)
			}
			assertStaticRestart(t, svc, ds, app, d, "tea-1/site/rev-2/", image, "35252fd")
		})
	}
}

// A serving release with no retained publication (published before w4/m197,
// or past the retained window) keeps the commit-pinned republish.
func TestStaticRestartWithoutARetainedPublicationRepublishesTheLiveCommit(t *testing.T) {
	ds := newFakeStore()
	liveStaticDeploys(t, ds)
	app := savedButUndeployed("")
	svc, cl := newService(ds, app, staticRecord(app, 1, "tea-1/site/rev-1/"))
	svc.Commits = &branchResolver{}

	d, err := svc.Restart(context.Background(), "site")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	got := getApp(t, cl, "site")
	if got.Spec.ReleaseConfig != nil || got.Spec.BuildCommit != "28aa2ab7" || d.CommitID != "28aa2ab7" {
		t.Fatalf("legacy restart = selection %+v buildCommit %q deploy commit %q; want no selection, pinned to the live commit",
			got.Spec.ReleaseConfig, got.Spec.BuildCommit, d.CommitID)
	}
}
