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
	"errors"
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The independent repo's tips; its GitHub default is master, and main does
// not exist (sweep344's expressjs/serve-static).
const (
	ownMasterTip  = "109a299c26d2d532027bb6e66e50e365eeb8fcdd"
	ownReleaseTip = "5555555555555555555555555555555555555555"
	ownMainTip    = "6666666666666666666666666666666666666666"
)

func ownRepoTips() *commitsByRef {
	return &commitsByRef{tips: map[string]string{"master": ownMasterTip, "release": ownReleaseTip, "main": ownMainTip}}
}

// omittedBranchManifest declares an independent repo with no branch.
var omittedBranchManifest = sourceServiceManifest("    repo: " + declaredRepo + "\n")

type defaultBranchCase struct {
	name, manifest, branch, commit string
	lookups                        int
}

// defaultBranchCases are an omitted branch and its explicit controls, at the
// root and nested in a project environment.
func defaultBranchCases() []defaultBranchCase {
	root := []defaultBranchCase{
		{"omitted branch builds the repo default", omittedBranchManifest, "master", ownMasterTip, 1},
		{"explicit master", sourceServiceManifest("    repo: " + declaredRepo + "\n    branch: master\n"), "master", ownMasterTip, 0},
		{"explicit main is kept on a master-default repo", sourceServiceManifest("    repo: " + declaredRepo + "\n    branch: main\n"), "main", ownMainTip, 0},
		{"explicit branch", sourceServiceManifest("    repo: " + declaredRepo + "\n    branch: release\n"), "release", ownReleaseTip, 0},
	}
	out := make([]defaultBranchCase, 0, 2*len(root))
	for _, tc := range root {
		nested := tc
		tc.name, nested.name = "root/"+tc.name, "nested/"+tc.name
		nested.manifest = nestSourceManifest(tc.manifest)
		out = append(out, tc, nested)
	}
	return out
}

// Through the real provided-manifest sync, an own image-born service moves
// to an independent repo whose default is master. With branch omitted, the
// App, the canonical row and the first Git deploy's commit all take master,
// not the image's placeholder main; an explicit branch always wins and asks
// GitHub nothing. An identical reapply asks again for nothing and opens no
// deploy (w8/m80).
func TestBlueprintSyncSwitchBuildsTheRepositoryDefaultBranch(t *testing.T) {
	for _, tc := range defaultBranchCases() {
		t.Run(tc.name, func(t *testing.T) {
			existing := ownImageBornApp()
			appID := existing.Labels[core.LabelAppID]
			fs := newFakeBlueprintStore(containingBlueprint())
			rec := &recordingStore{}
			svc, ctx := ownSourceService(fs, rec, existing)
			gh := &fakeCloneTokens{defaultBranch: "master"}
			svc.GitHub = gh
			commits := ownRepoTips()
			svc.Commits = commits

			if _, err := svc.SyncBlueprint(ctx, "blp-own", "tea-a", tc.manifest, "", nil); err != nil {
				t.Fatalf("SyncBlueprint: %v", err)
			}
			if got := fs.blueprints["blp-own"].Manifest; got != tc.manifest {
				t.Errorf("stored manifest = %q, want the exact authored YAML", got)
			}
			// The sync's plan and its apply each resolve the switch.
			lookups := gh.validateCalls
			if lookups != 2*tc.lookups || (tc.lookups > 0 && gh.lastRepo != declaredRepo) {
				t.Errorf("repository lookups = %d (last %q), want %d of %q", lookups, gh.lastRepo, 2*tc.lookups, declaredRepo)
			}
			got := getApp(t, svc.Client, "own-source").Spec
			if got.Repo != declaredRepo || got.Branch != tc.branch || got.Image != "" {
				t.Errorf("App source = repo %q branch %q image %q, want %q %s and no image", got.Repo, got.Branch, got.Image, declaredRepo, tc.branch)
			}
			wantWrite := struct{ id, repo, image, branch string }{appID, declaredRepo, "", tc.branch}
			if len(rec.sourceWrites) != 1 || rec.sourceWrites[0] != wantWrite {
				t.Errorf("canonical source writes = %+v, want %+v", rec.sourceWrites, wantWrite)
			}
			if len(rec.deployCalls) != 1 || rec.deployCalls[0].Commit != tc.commit {
				t.Fatalf("deploys = %+v, want one at %s's commit %s", rec.deployCalls, tc.branch, tc.commit)
			}

			if _, err := svc.SyncBlueprint(ctx, "blp-own", "tea-a", tc.manifest, "", nil); err != nil {
				t.Fatalf("identical SyncBlueprint: %v", err)
			}
			if len(rec.deployCalls) != 1 || len(rec.sourceWrites) != 1 || gh.validateCalls != lookups {
				t.Errorf("identical reapply: %d deploys, %d source writes, %d lookups; want no new ones",
					len(rec.deployCalls), len(rec.sourceWrites), gh.validateCalls)
			}
		})
	}
}

// A Git-backed Blueprint create builds a new service's omitted branch from
// its own repository's default: the returned view, the App, the store row and
// the first deploy's commit agree, and no containing-repo branch or commit is
// lent to it (w8/m80).
func TestCreateBlueprintBuildsTheRepositoryDefaultBranch(t *testing.T) {
	for _, tc := range defaultBranchCases() {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStore{}
			svc, ctx := ownSourceService(newFakeBlueprintStore(), rec)
			svc.GitFetcher = fakeBlueprintFetcher{contents: tc.manifest}
			gh := &fakeCloneTokens{defaultBranch: "master"}
			svc.GitHub = gh
			commits := ownRepoTips()
			svc.Commits = commits

			if _, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: containingRepo, Branch: containingBranch}); err != nil {
				t.Fatalf("CreateBlueprint: %v", err)
			}
			if gh.validateCalls != tc.lookups {
				t.Errorf("repository lookups = %d, want %d", gh.validateCalls, tc.lookups)
			}
			if got := oneApp(t, svc.Client).Spec; got.Repo != declaredRepo || got.Branch != tc.branch || got.BuildCommit != tc.commit {
				t.Errorf("created source = (%q, %q) at %q, want (%q, %q) at %q", got.Repo, got.Branch, got.BuildCommit, declaredRepo, tc.branch, tc.commit)
			}
			if len(rec.appCreates) != 1 || rec.appCreates[0].Branch != tc.branch || rec.appCreates[0].FirstDeployCommit.Hash != tc.commit {
				t.Fatalf("store rows = %+v, want one on %s at %s", rec.appCreates, tc.branch, tc.commit)
			}
		})
	}
}

// The containing source keeps its contract: an entry on the Blueprint's own
// repo inherits the Blueprint's branch and asks GitHub nothing (w8/m78).
func TestBlueprintOwnRepoEntryInheritsTheBlueprintBranch(t *testing.T) {
	for _, manifest := range []string{sourceServiceManifest(""), sourceServiceManifest("    repo: " + containingRepo + "\n")} {
		svc, ctx := ownSourceService(newFakeBlueprintStore(), &recordingStore{})
		svc.GitFetcher = fakeBlueprintFetcher{contents: manifest}
		gh := &fakeCloneTokens{defaultBranch: "master"}
		svc.GitHub = gh
		if _, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: containingRepo, Branch: "canary"}); err != nil {
			t.Fatalf("CreateBlueprint: %v", err)
		}
		if got := oneApp(t, svc.Client).Spec.Branch; got != "canary" || gh.validateCalls != 0 {
			t.Errorf("own-repo entry branch = %q after %d lookups, want the Blueprint's canary and none", got, gh.validateCalls)
		}
	}
}

// An existing service's branch is its own while the repo stays: a re-sync
// that omits branch keeps a deliberately chosen one, however the repo is
// spelled, and reads no repository metadata. A move to another repository
// builds that repository's default instead of carrying the old repo's ref.
func TestBlueprintExistingGitServiceBranchOnOmission(t *testing.T) {
	native := func(repo string) string {
		return "services:\n- {name: own-source, type: web, runtime: node, repo: " + repo + ", plan: free, buildCommand: 'true', startCommand: 'node server.js'}\n"
	}
	for _, tc := range []struct {
		name, repo, branch string
		lookups            int
	}{
		{"same repo keeps its branch", switchRepo, "release", 0},
		{"respelled same repo keeps its branch", switchRepo + ".git", "release", 0},
		{"another repo builds its default", "https://github.com/expressjs/express", "master", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := gitBornSpec()
			initial.Branch = "release"
			gh := &fakeCloneTokens{defaultBranch: "master"}
			got, _ := switchSourceWithGitHub(t, initial, DeployRequest{Manifest: native(tc.repo)}, gh)
			if got.Branch != tc.branch || gh.validateCalls != tc.lookups {
				t.Errorf("branch = %q after %d lookups, want %q after %d", got.Branch, gh.validateCalls, tc.branch, tc.lookups)
			}
		})
	}
}

// When the default an omitted branch depends on cannot be read or built, the
// sync refuses the service before any write, as a direct create does (w8/m76),
// and the preview refuses it as well; with no opinion (a non-GitHub host or
// GitHub off) the legacy main stays.
func TestBlueprintUnresolvableRepositoryDefault(t *testing.T) {
	for _, tc := range []struct {
		name   string
		gh     *fakeCloneTokens
		want   error
		branch string
	}{
		{name: "lookup failure", gh: &fakeCloneTokens{validateErr: fmt.Errorf("checking public repository access: %w", errors.New("github: 503"))}},
		{name: "inaccessible repository", gh: &fakeCloneTokens{validateErr: fmt.Errorf("%w: repository is not accessible", core.ErrBadRequest)}, want: core.ErrBadRequest},
		{name: "unbuildable default", gh: &fakeCloneTokens{defaultBranch: "-x;y"}, want: core.ErrBadRequest},
		{name: "no opinion keeps main", gh: &fakeCloneTokens{}, branch: appv1alpha1.DefaultBranch},
	} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%v", tc.name, existing), func(t *testing.T) {
				var apps []*appv1alpha1.App
				if existing {
					apps = append(apps, ownImageBornApp())
				}
				fs := newFakeBlueprintStore(containingBlueprint())
				rec := &recordingStore{}
				svc, ctx := ownSourceService(fs, rec, apps...)
				svc.GitHub = tc.gh
				svc.Commits = ownRepoTips()

				_, err := svc.SyncBlueprint(ctx, "blp-own", "tea-a", omittedBranchManifest, "", nil)
				if tc.branch != "" {
					if err != nil {
						t.Fatalf("SyncBlueprint: %v", err)
					}
					if got := oneApp(t, svc.Client).Spec.Branch; got != tc.branch {
						t.Errorf("branch = %q, want %q", got, tc.branch)
					}
					return
				}
				want := tc.want
				if want == nil {
					want = tc.gh.validateErr
				}
				if err == nil || !errors.Is(err, want) {
					t.Fatalf("SyncBlueprint = %v, want %v", err, want)
				}
				if len(rec.appCreates) != 0 || len(rec.sourceWrites) != 0 || len(rec.deployCalls) != 0 {
					t.Errorf("a refused sync wrote %d rows, %d source writes, %d deploys", len(rec.appCreates), len(rec.sourceWrites), len(rec.deployCalls))
				}
				if existing {
					if got := getApp(t, svc.Client, "own-source").Spec; got.Image == "" || got.Repo != "" {
						t.Errorf("a refused sync changed the service's source: %+v", got)
					}
				}
			})
		}
	}
}

// The preview plans what the apply refuses: a switch whose omitted branch
// cannot resolve is refused at the service, before anything is written.
func TestPreviewBlueprintRefusesAnUnbuildableRepositoryDefault(t *testing.T) {
	for _, existing := range []bool{false, true} {
		var apps []*appv1alpha1.App
		if existing {
			apps = append(apps, ownImageBornApp())
		}
		svc, ctx := ownSourceService(newFakeBlueprintStore(), &recordingStore{}, apps...)
		svc.GitFetcher = fakeBlueprintFetcher{contents: omittedBranchManifest}
		svc.GitHub = &fakeCloneTokens{defaultBranch: "-x;y"}
		preview, err := svc.PreviewBlueprint(ctx, "tea-a", containingRepo, containingBranch, "", "")
		if err != nil || preview.Validation == nil {
			t.Fatalf("existing=%v PreviewBlueprint: %+v %v", existing, preview, err)
		}
		if preview.Validation.Valid || len(preview.Validation.Errors) == 0 {
			t.Errorf("existing=%v preview = %+v, want the unbuildable default refused", existing, preview.Validation)
		}
	}
}
