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

package apps

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// commitsByRef resolves only the refs it knows, recording every ref asked
// for, so a test sees which branch the first deploy's commit was read from.
type commitsByRef struct {
	tips  map[string]string
	asked []string
}

func (c *commitsByRef) ResolveCommit(_ context.Context, _, _, ref string) (store.CommitInfo, bool, error) {
	c.asked = append(c.asked, ref)
	sha, ok := c.tips[ref]
	return store.CommitInfo{Hash: sha}, ok, nil
}

// TestCreateOmittedBranchUsesTheRepositoryDefault is w8/m76: a native create
// that omits branch (the pinned Render CLI never serializes an empty one) must
// build the repository's default branch, per Render's Create service contract
// — not the hard-coded main, which made a master-default repo fail its very
// first clone with `couldn't find remote ref main`. The default is chosen once
// and is the same value on the returned view, the CR, the store row, the
// first deploy's commit lookup and the pinned build commit. An explicit branch
// always wins, including an explicit main on a non-main-default repo.
func TestCreateOmittedBranchUsesTheRepositoryDefault(t *testing.T) {
	const (
		masterTip = "1111111111111111111111111111111111111111"
		trunkTip  = "2222222222222222222222222222222222222222"
		mainTip   = "3333333333333333333333333333333333333333"
		relTip    = "4444444444444444444444444444444444444444"
	)
	tips := map[string]string{"master": masterTip, "trunk": trunkTip, "main": mainTip, "release": relTip}
	for _, tc := range []struct {
		name          string
		repoDefault   string
		branch        string
		wantBranch    string
		wantCommitSHA string
	}{
		{name: "omitted on a master-default repo", repoDefault: "master", wantBranch: "master", wantCommitSHA: masterTip},
		{name: "omitted on a trunk-default repo", repoDefault: "trunk", wantBranch: "trunk", wantCommitSHA: trunkTip},
		{name: "omitted on a main-default repo", repoDefault: "main", wantBranch: "main", wantCommitSHA: mainTip},
		{name: "explicit branch wins over the default", repoDefault: "master", branch: "release", wantBranch: "release", wantCommitSHA: relTip},
		{name: "explicit main on a master-default repo is kept", repoDefault: "master", branch: "main", wantBranch: "main", wantCommitSHA: mainTip},
		{name: "explicit branch ignores an unbuildable default", repoDefault: "-x;y", branch: "release", wantBranch: "release", wantCommitSHA: relTip},
		{name: "no opinion keeps the legacy default", repoDefault: "", wantBranch: appv1alpha1.DefaultBranch, wantCommitSHA: mainTip},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
			gh := &fakeCloneTokens{defaultBranch: tc.repoDefault}
			svc.GitHub = gh
			commits := &commitsByRef{tips: tips}
			svc.Commits = commits
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

			v, err := svc.create(ctx, CreateRequest{
				Name: "web", Type: appv1alpha1.TypeWebService, Runtime: "node",
				Repo:         "https://github.com/expressjs/serve-static",
				Branch:       tc.branch,
				BuildCommand: "node --version", StartCommand: "node server.js",
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if gh.validateCalls != 1 {
				t.Errorf("ValidateRepo called %d times, want once: the access check and the default share one read", gh.validateCalls)
			}
			if v.Branch != tc.wantBranch {
				t.Errorf("returned branch = %q, want %q", v.Branch, tc.wantBranch)
			}
			a := getTenantApp(t, cl, "tea-a", "web")
			if a.Spec.Branch != tc.wantBranch {
				t.Errorf("spec.branch = %q, want %q", a.Spec.Branch, tc.wantBranch)
			}
			if len(rec.appCreates) != 1 || rec.appCreates[0].Branch != tc.wantBranch {
				t.Fatalf("store row branch = %+v, want %q", rec.appCreates, tc.wantBranch)
			}
			if len(commits.asked) != 1 || commits.asked[0] != tc.wantBranch {
				t.Errorf("first deploy commit resolved for refs %q, want [%q]", commits.asked, tc.wantBranch)
			}
			if got := rec.appCreates[0].FirstDeployCommit.Hash; got != tc.wantCommitSHA {
				t.Errorf("first deploy row commit = %q, want %q", got, tc.wantCommitSHA)
			}
			if a.Spec.BuildCommit != tc.wantCommitSHA {
				t.Errorf("spec.buildCommit = %q, want %q: the first build clones the selected branch's commit", a.Spec.BuildCommit, tc.wantCommitSHA)
			}
		})
	}
}

// TestCreateDryRunPreviewsTheRepositoryDefault: a dry-run answers what the
// real create would store (w8/045's parity rule), so its preview names the
// resolved default branch too — and still writes nothing.
func TestCreateDryRunPreviewsTheRepositoryDefault(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	svc.GitHub = &fakeCloneTokens{defaultBranch: "master"}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	v, err := svc.create(ctx, CreateRequest{
		Name: "web", Type: appv1alpha1.TypeWebService, Runtime: "docker",
		Repo: "https://github.com/expressjs/serve-static", DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry-run create: %v", err)
	}
	if v.Branch != "master" {
		t.Errorf("dry-run branch = %q, want master", v.Branch)
	}
	if len(rec.appCreates) != 0 {
		t.Errorf("dry-run wrote %d store rows", len(rec.appCreates))
	}
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
		t.Errorf("dry-run left Apps behind: %d (%v)", len(apps.Items), err)
	}
}

// TestCreateUnresolvableRepositoryDefaultCreatesNothing: when the repository
// lookup an omitted branch depends on fails, or names a default the build
// cannot safely pass to git, the create refuses with an actionable error
// instead of fabricating main — and leaves no App or row. An explicit branch
// on that same repository is unaffected by its default.
func TestCreateUnresolvableRepositoryDefaultCreatesNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		gh   *fakeCloneTokens
		want error
	}{
		{name: "lookup failure", gh: &fakeCloneTokens{validateErr: fmt.Errorf("checking public repository access: %w", errors.New("github: 503"))}},
		{name: "inaccessible repository", gh: &fakeCloneTokens{validateErr: fmt.Errorf("%w: repository is not accessible", core.ErrBadRequest)}, want: core.ErrBadRequest},
		{name: "unbuildable default", gh: &fakeCloneTokens{defaultBranch: "-x;y"}, want: core.ErrBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
			svc.GitHub = tc.gh
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

			_, err := svc.create(ctx, CreateRequest{
				Name: "web", Type: appv1alpha1.TypeWebService, Runtime: "docker",
				Repo: "https://github.com/expressjs/serve-static",
			})
			want := tc.want
			if want == nil {
				want = tc.gh.validateErr
			}
			if err == nil || !errors.Is(err, want) {
				t.Fatalf("create = %v, want %v", err, want)
			}
			if len(rec.appCreates) != 0 {
				t.Errorf("a refused create wrote %d store rows", len(rec.appCreates))
			}
			var made appv1alpha1.App
			if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: core.CRName("tea-a", "web")}, &made); !apierrors.IsNotFound(err) {
				t.Errorf("a refused create left an App behind: %v", err)
			}
		})
	}
}
