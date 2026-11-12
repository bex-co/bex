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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rollback_free_depth_test.go is w8/262. Render, Free instances: "Rollbacks
// (only to the two most recent previous deploys)", and upgrading the workspace
// plan "does not remove limitations on Free instances". bex let a Free web
// service roll back to any earlier deploy.

// fourLiveDeploys is an image service whose four deploys each went live in
// turn: D4 is live and D3, D2, D1 are the previous ones, newest first.
func fourLiveDeploys(t *testing.T, tier, svcType string) (svc *Service, ds *fakeStore, d1, d2, d3, d4 store.Deploy) {
	t.Helper()
	ds = newFakeStore()
	app := sampleApp("web", "srv-1")
	app.Spec.Tier, app.Spec.Type = tier, svcType
	app.Spec.Image = "docker.io/traefik/whoami:v1.10.4"
	mk := func(gen int64, tag, digest string) store.Deploy {
		return liveDeploy(t, ds, "srv-1", "docker.io/traefik/whoami:"+tag, gen, "docker.io/traefik/whoami@sha256:"+strings.Repeat(digest, 64))
	}
	d1, d2, d3, d4 = mk(1, "v1.10.1", "1"), mk(2, "v1.10.2", "2"), mk(3, "v1.10.3", "3"), mk(4, "v1.10.4", "4")
	svc, _ = newService(ds, app)
	return svc, ds, d1, d2, d3, d4
}

func TestFreeWebServiceRollsBackOnlyToItsTwoMostRecentPreviousDeploys(t *testing.T) {
	ctx := context.Background()
	svc, ds, d1, d2, d3, _ := fourLiveDeploys(t, "free", appv1alpha1.TypeWebService)
	before, _ := ds.ListDeploys(ctx, "srv-1", store.DeployFilter{Limit: 50})

	_, err := svc.Rollback(ctx, "web", d1.ID)
	if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), "two most recent previous deploys") || !strings.Contains(err.Error(), d1.ID) {
		t.Fatalf("rollback to the third previous deploy = %v, want the Free refusal naming it", err)
	}
	// The fixture is image-backed: it has no commit to deploy again.
	if !strings.Contains(err.Error(), "deploy its image again") || strings.Contains(err.Error(), "commit") {
		t.Errorf("an image-backed service's refusal = %q, want it to say to deploy its image again", err)
	}
	if after, _ := ds.ListDeploys(ctx, "srv-1", store.DeployFilter{Limit: 50}); len(after) != len(before) {
		t.Fatalf("the refused rollback added a deploy row: %d, was %d", len(after), len(before))
	}

	// Every read marks the same rows, so no surface offers what the verb refuses.
	rows, err := svc.List(ctx, "web", ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	gone := map[string]bool{}
	for _, r := range rows {
		gone[r.ID] = r.BuildGone
	}
	if !gone[d1.ID] || gone[d2.ID] || gone[d3.ID] {
		t.Errorf("BuildGone = %v, want only the third previous deploy marked", gone)
	}
	if one, err := svc.Get(ctx, "web", d1.ID); err != nil || !one.BuildGone {
		t.Errorf("Get(third previous) BuildGone = %v (err %v), want true", one.BuildGone, err)
	}

	// The two most recent previous deploys roll back.
	if _, err := svc.Rollback(ctx, "web", d2.ID); err != nil {
		t.Fatalf("rollback to the second previous deploy: %v", err)
	}
}

// The limit is the Free instance's, and a web service's.
func TestRollbackDepthLimitsOnlyAFreeWebService(t *testing.T) {
	for name, tc := range map[string]struct{ tier, svcType string }{
		"a paid web service":     {"starter", appv1alpha1.TypeWebService},
		"a free private service": {"free", appv1alpha1.TypePrivateService},
	} {
		svc, _, d1, _, _, _ := fourLiveDeploys(t, tc.tier, tc.svcType)
		if _, err := svc.Rollback(context.Background(), "web", d1.ID); err != nil {
			t.Errorf("%s: rollback to the third previous deploy = %v, want it accepted", name, err)
		}
	}
}

// With every previous deploy inside the depth, the action is offered; the
// projection asks the same question the verb answers.
func TestDeployActionsFollowTheFreeRollbackDepth(t *testing.T) {
	svc, _, _, _, _, _ := fourLiveDeploys(t, "free", appv1alpha1.TypeWebService)
	if pre := rollbackPrecondition(t, svc, "web"); pre != "" {
		t.Errorf("rollback precondition = %q, want none: the two most recent previous deploys are targets", pre)
	}
}

// A rollback refusal says how to bring the deploy back, in the terms of the
// target deploy's own source: its commit, its image, or a build of the linked
// build source (QA round 161).
func TestRedeployHintFollowsTheTargetsSource(t *testing.T) {
	repo := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "https://github.com/a/app"}}
	image := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Image: "docker.io/traefik/whoami:v1.10.1"}}
	linked := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{BuildSource: &appv1alpha1.BuildSourceLink{}}}
	built := store.Deploy{Commit: "7e9ae1d5bf898167b01d5fad9327d8d08daabc03"}
	ran := store.Deploy{Image: "docker.io/traefik/whoami:v1.10.0"}
	for name, tc := range map[string]struct {
		app     *appv1alpha1.App
		target  store.Deploy
		capital bool
		want    string
	}{
		"a build":                               {repo, built, false, "deploy its commit again"},
		"a build, capitalized":                  {repo, built, true, "Deploy its commit again"},
		"a build with no recorded commit":       {repo, store.Deploy{}, false, "deploy its commit again"},
		"an image":                              {image, ran, false, "deploy its image again"},
		"an image, capitalized":                 {image, ran, true, "Deploy its image again"},
		"an image before the switch to a repo":  {repo, ran, false, "deploy its image again"},
		"a build before the switch to an image": {image, built, false, "deploy its commit again"},
		"a linked service's build":              {linked, built, false, "build its commit again in the build source"},
	} {
		if got := redeployHint(tc.app, tc.target, tc.capital); got != tc.want {
			t.Errorf("%s: redeployHint = %q, want %q", name, got, tc.want)
		}
	}
}
