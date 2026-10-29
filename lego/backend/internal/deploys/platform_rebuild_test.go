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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rebuildSince is when the patched toolchain finished rolling out; the seeded
// live deploys predate it.
var rebuildSince = time.Date(2026, 9, 14, 13, 0, 0, 0, time.UTC)

func liveBefore(ds *fakeStore, storeID string) {
	ds.byApp[storeID] = []store.Deploy{{
		ID: "dep-" + storeID, AppID: storeID, Status: store.DeployLive, Trigger: store.TriggerAPI,
		Commit: runningCommit, CreatedAt: rebuildSince.Add(-time.Hour),
	}}
}

func runtimeApp(name, storeID string, mutate func(*appv1alpha1.App)) *appv1alpha1.App {
	a := repoApp(name, storeID, "main")
	a.Spec.Runtime = "node"
	if mutate != nil {
		mutate(a)
	}
	return a
}

func rebuildFleet(ds *fakeStore) (*Service, client.Client) {
	objs := []client.Object{
		runtimeApp("node-web", "srv-node", nil),
		runtimeApp("cnb-worker", "srv-cnb", func(a *appv1alpha1.App) { a.Spec.Runtime, a.Spec.Builder = "", "buildpack" }),
		runtimeApp("site", "srv-site", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite }),
		runtimeApp("docker", "srv-docker", func(a *appv1alpha1.App) { a.Spec.Runtime = "docker" }),
		runtimeApp("dockerfile", "srv-dockerfile", func(a *appv1alpha1.App) { a.Spec.Runtime, a.Spec.Builder = "", "dockerfile" }),
		// The default builder with no runtime builds the repo's Dockerfile.
		runtimeApp("auto", "srv-auto", func(a *appv1alpha1.App) { a.Spec.Runtime, a.Spec.Builder = "", "auto" }),
		runtimeApp("paused", "srv-paused", func(a *appv1alpha1.App) { a.Spec.Suspended = true }),
		sampleApp("image", "srv-image"),
	}
	for _, id := range []string{"srv-node", "srv-cnb", "srv-site", "srv-docker", "srv-dockerfile", "srv-auto", "srv-paused", "srv-image"} {
		liveBefore(ds, id)
	}
	svc, cl := newService(ds, objs...)
	svc.Commits = &branchResolver{}
	return svc, cl
}

func services(items []PlatformRebuildItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Service)
	}
	return out
}

// Only services running a bex-pinned toolchain are rebuilt: a source build
// that no Dockerfile drives. Static sites, Dockerfile builds, prebuilt images
// and suspended services are out; a dry run opens nothing.
func TestPlatformRebuildDryRunPlansOnlyToolchainBuiltServices(t *testing.T) {
	ds := newFakeStore()
	svc, _ := rebuildFleet(ds)

	res, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{Since: rebuildSince})
	if err != nil {
		t.Fatalf("PlatformRebuild: %v", err)
	}
	if got, want := strings.Join(services(res.Planned), ","), "cnb-worker,node-web"; got != want {
		t.Errorf("planned = %s, want %s", got, want)
	}
	if len(res.Triggered) != 0 || res.Applied {
		t.Errorf("dry run result = %+v, want 2 pending and nothing triggered", res)
	}
	if n := len(ds.byApp["srv-node"]); n != 1 {
		t.Errorf("dry run opened deploys: srv-node has %d rows", n)
	}
}

// Applying rebuilds at the live commit (never the branch head), as Render's
// own deployed_by_render deploy, and a re-run skips what is already rebuilt.
func TestPlatformRebuildApplyRebuildsTheLiveCommitOnce(t *testing.T) {
	ds := newFakeStore()
	svc, cl := rebuildFleet(ds)

	res, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{Since: rebuildSince, Apply: true, Limit: 10})
	if err != nil {
		t.Fatalf("PlatformRebuild: %v", err)
	}
	if got, want := strings.Join(services(res.Triggered), ","), "cnb-worker,node-web"; got != want {
		t.Fatalf("triggered = %s, want %s (failed: %+v)", got, want, res.Failed)
	}
	for _, it := range res.Triggered {
		if it.Commit != runningCommit {
			t.Errorf("%s rebuilt commit %q, want the live commit %q", it.Service, it.Commit, runningCommit)
		}
	}
	if got := getApp(t, cl, "node-web").Spec.BuildCommit; got != runningCommit {
		t.Errorf("spec.buildCommit = %q, want the live commit %q", got, runningCommit)
	}
	if got := ds.byApp["srv-node"][0].Trigger; got != store.TriggerDeployedByRender {
		t.Errorf("trigger = %q, want %q", got, store.TriggerDeployedByRender)
	}
	if got := renderTrigger(store.TriggerDeployedByRender); got != "deployed_by_render" {
		t.Errorf("renderTrigger = %q, want Render's deployed_by_render", got)
	}
	// A settings save and an environment save stay apart in bex's history
	// (only the environment one reads envUpdated, w8/m135); both are Render's
	// service_updated.
	for _, trigger := range []string{store.TriggerConfigChange, store.TriggerSettingsChange} {
		if got := renderTrigger(trigger); got != "service_updated" {
			t.Errorf("renderTrigger(%s) = %q, want service_updated", trigger, got)
		}
	}

	again, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{Since: rebuildSince, Apply: true, Limit: 10})
	if err != nil {
		t.Fatalf("second PlatformRebuild: %v", err)
	}
	if len(again.Triggered) != 0 || again.Done != 2 {
		t.Errorf("re-run = %+v, want nothing triggered and 2 done", again)
	}
}

// A deploy after since that reused an artifact (restart, rollback, config
// change) or failed did not ship the patch, so the service is still pending;
// a source build after since that is live or still running did.
func TestPlatformRebuildCountsOnlySourceBuildsSince(t *testing.T) {
	cases := []struct {
		trigger, status string
		done            bool
	}{
		{store.TriggerNewCommit, store.DeployLive, true},
		{store.TriggerAPI, store.DeployBuildInProgress, true},
		{store.TriggerRestart, store.DeployLive, false},
		{store.TriggerRollback, store.DeployLive, false},
		{store.TriggerConfigChange, store.DeployLive, false},
		{store.TriggerAPI, store.DeployBuildFailed, false},
	}
	for _, tc := range cases {
		t.Run(tc.trigger+"/"+tc.status, func(t *testing.T) {
			ds := newFakeStore()
			liveBefore(ds, "srv-node")
			ds.byApp["srv-node"] = append([]store.Deploy{{
				ID: "dep-after", AppID: "srv-node", Status: tc.status, Trigger: tc.trigger,
				Commit: runningCommit, CreatedAt: rebuildSince.Add(time.Minute),
			}}, ds.byApp["srv-node"]...)
			svc, _ := newService(ds, runtimeApp("node-web", "srv-node", nil))

			res, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{Since: rebuildSince})
			if err != nil {
				t.Fatalf("PlatformRebuild: %v", err)
			}
			if done := res.Done == 1; done != tc.done {
				t.Errorf("done = %v, want %v (%+v)", done, tc.done, res)
			}
		})
	}
}

// Limit bounds one applied batch; the remainder stays planned for the next.
func TestPlatformRebuildLimitBoundsTheBatch(t *testing.T) {
	ds := newFakeStore()
	svc, _ := rebuildFleet(ds)

	res, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{Since: rebuildSince, Apply: true, Limit: 1})
	if err != nil {
		t.Fatalf("PlatformRebuild: %v", err)
	}
	if len(res.Triggered) != 1 || len(res.Planned) != 1 {
		t.Errorf("batch = %+v, want 1 triggered and 1 pending", res)
	}
}

// A service without a live deploy has no running release to patch; it is
// reported, and the batch carries on.
func TestPlatformRebuildReportsAServiceWithoutALiveDeploy(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, runtimeApp("node-web", "srv-node", nil))

	res, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{Since: rebuildSince, Apply: true, Limit: 10})
	if err != nil {
		t.Fatalf("PlatformRebuild: %v", err)
	}
	if len(res.Failed) != 1 || len(res.Triggered) != 0 {
		t.Errorf("result = %+v, want the service reported as failed", res)
	}
}

func TestPlatformRebuildRequiresSince(t *testing.T) {
	svc, _ := newService(newFakeStore())
	if _, err := svc.PlatformRebuild(context.Background(), PlatformRebuildRequest{}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("err = %v, want ErrBadRequest", err)
	}
}

// The verb writes across tenants, so only the control-plane bearer opens it;
// an unset token never does.
func TestPlatformRebuildHandlerRequiresTheBearer(t *testing.T) {
	svc, _ := newService(newFakeStore())
	body := `{"since":"2026-09-14T13:00:00Z"}`
	for _, tc := range []struct {
		token, header string
		want          int
	}{
		{"secret", "Bearer secret", http.StatusOK},
		{"secret", "Bearer wrong", http.StatusUnauthorized},
		{"secret", "", http.StatusUnauthorized},
		{"", "Bearer ", http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(http.MethodPost, PlatformRebuildPath, strings.NewReader(body))
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		rec := httptest.NewRecorder()
		PlatformRebuildHandler(svc, tc.token).ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("token %q header %q: status %d, want %d", tc.token, tc.header, rec.Code, tc.want)
		}
	}
}
