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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestRepoSwitchResolvesDefaultBranchAcrossSurfaces is w32/108: switching a
// service to another Git repository (from a prebuilt image or another repo)
// without specifying a branch resolves the new repository's default branch from
// ValidateRepo instead of keeping an image's hidden "main" placeholder or the
// old repository's branch. The resolved branch reaches the view, CR, store row,
// and dry-run preview, and ApplyServicePatch probes ValidateRepo exactly once.
func TestRepoSwitchResolvesDefaultBranchAcrossSurfaces(t *testing.T) {
	const targetRepo = "https://github.com/render-examples/go-web-server"

	newImageApp := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Repo = ""
		a.Spec.Image = "docker.io/library/nginx:1.27-alpine"
		a.Spec.Runtime = "image"
		a.Spec.Branch = appv1alpha1.DefaultBranch
		return a
	}
	newGitApp := func(branch string) *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Repo = "https://github.com/bex-co/bex"
		a.Spec.Branch = branch
		return a
	}

	t.Run("REST PATCH image to master-default repo", func(t *testing.T) {
		st := &recordingStore{}
		svc, cl := newService(st, newImageApp())
		gh := &fakeCloneTokens{defaultBranch: "master", repoID: 707}
		svc.GitHub = gh

		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		// Dry-run previews the resolved default branch without mutating.
		dryRec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(dryRec, httptest.NewRequest("PATCH", "/v1/services/web?dryRun=true",
			strings.NewReader(`{"repo":"`+targetRepo+`"}`)))
		if dryRec.Code != http.StatusOK {
			t.Fatalf("dryRun PATCH %d: %s", dryRec.Code, dryRec.Body.String())
		}
		var dryBody map[string]any
		if err := json.Unmarshal(dryRec.Body.Bytes(), &dryBody); err != nil {
			t.Fatalf("decode dryRun body: %v", err)
		}
		if dryBody["branch"] != "master" {
			t.Errorf("dryRun branch = %v, want master", dryBody["branch"])
		}
		if len(st.sourceWrites) != 0 {
			t.Errorf("dryRun wrote store rows: %+v", st.sourceWrites)
		}
		gh.validateCalls = 0

		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web",
			strings.NewReader(`{"repo":"`+targetRepo+`"}`)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode PATCH body: %v", err)
		}
		if body["repo"] != targetRepo || body["branch"] != "master" {
			t.Fatalf("PATCH response repo=%v branch=%v, want %q / master", body["repo"], body["branch"], targetRepo)
		}
		if gh.validateCalls != 1 {
			t.Errorf("ValidateRepo called %d times, want 1 (preflight must share its resolved branch with apply)", gh.validateCalls)
		}
		got := getApp(t, cl, "web")
		if got.Spec.Repo != targetRepo || got.Spec.Branch != "master" || got.Spec.Image != "" {
			t.Errorf("CR spec = repo %q branch %q image %q, want %q / master / empty", got.Spec.Repo, got.Spec.Branch, got.Spec.Image, targetRepo)
		}
		if core.GitHubRepoID(got) != 707 {
			t.Errorf("CR githubRepoID = %d, want 707", core.GitHubRepoID(got))
		}
		if len(st.sourceWrites) != 1 || st.sourceWrites[0].branch != "master" || st.sourceWrites[0].repo != targetRepo {
			t.Errorf("store sourceWrites = %+v, want repo=%q branch=master", st.sourceWrites, targetRepo)
		}
	})

	t.Run("GraphQL setRepo repo-only and with runtime", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			app   *appv1alpha1.App
			query string
		}{
			{
				name:  "image to repo (direct SetSourceAndRegistryCredential)",
				app:   newImageApp(),
				query: `mutation { setRepo(id: "web", repo: "` + targetRepo + `") { repo branch imagePath } }`,
			},
			{
				name:  "git main to repo with runtime (ApplyServicePatch)",
				app:   newGitApp("main"),
				query: `mutation { setRepo(id: "web", repo: "` + targetRepo + `", runtime: "docker") { repo branch } }`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				st := &recordingStore{}
				svc, cl := newService(st, tc.app)
				gh := &fakeCloneTokens{defaultBranch: "master", repoID: 808}
				svc.GitHub = gh

				res := graphql.Do(graphql.Params{
					Schema:        sourceSwapSchema(t, svc),
					Context:       context.Background(),
					RequestString: tc.query,
				})
				if len(res.Errors) > 0 {
					t.Fatalf("gql %q: %v", tc.query, res.Errors)
				}
				got := res.Data.(map[string]any)["setRepo"].(map[string]any)
				if got["branch"] != "master" || got["repo"] != targetRepo {
					t.Fatalf("setRepo = %+v, want repo=%q branch=master", got, targetRepo)
				}
				if gh.validateCalls != 1 {
					t.Errorf("ValidateRepo called %d times, want 1", gh.validateCalls)
				}
				if spec := getApp(t, cl, "web").Spec; spec.Branch != "master" || spec.Repo != targetRepo {
					t.Errorf("CR spec = repo %q branch %q, want %q / master", spec.Repo, spec.Branch, targetRepo)
				}
				if len(st.sourceWrites) != 1 || st.sourceWrites[0].branch != "master" {
					t.Errorf("store sourceWrites = %+v, want branch=master", st.sourceWrites)
				}
			})
		}
	})

	t.Run("MCP update_service switches repo without branch", func(t *testing.T) {
		st := &recordingStore{}
		svc, cl := newService(st, newImageApp())
		gh := &fakeCloneTokens{defaultBranch: "trunk", repoID: 909}
		svc.GitHub = gh

		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()

		call("update_service", map[string]any{
			"serviceId": "web",
			"repo":      targetRepo,
		})
		if gh.validateCalls != 1 {
			t.Errorf("ValidateRepo called %d times, want 1", gh.validateCalls)
		}
		if spec := getApp(t, cl, "web").Spec; spec.Repo != targetRepo || spec.Branch != "trunk" {
			t.Errorf("CR spec = repo %q branch %q, want %q / trunk", spec.Repo, spec.Branch, targetRepo)
		}
		if len(st.sourceWrites) != 1 || st.sourceWrites[0].branch != "trunk" {
			t.Errorf("store sourceWrites = %+v, want branch=trunk", st.sourceWrites)
		}
	})
}

func TestRepoSwitchBranchRules(t *testing.T) {
	const (
		repoA = "https://github.com/bex-co/bex"
		repoB = "https://github.com/render-examples/go-web-server"
	)
	str := func(s string) *string { return &s }

	for _, tc := range []struct {
		name              string
		startRepo         string
		startBranch       string
		patch             sourcePatch
		gh                *fakeCloneTokens
		wantRepo          string
		wantBranch        string
		wantValidateCalls int
	}{
		{
			name:              "git main to master-default repo omits branch",
			startRepo:         repoA,
			startBranch:       "main",
			patch:             sourcePatch{Repo: str(repoB)},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoB,
			wantBranch:        "master",
			wantValidateCalls: 1,
		},
		{
			name:              "git custom to master-default repo omits branch",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoB)},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoB,
			wantBranch:        "master",
			wantValidateCalls: 1,
		},
		{
			name:              "git switch with explicit empty branch resolves default",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoB), Branch: str("")},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoB,
			wantBranch:        "master",
			wantValidateCalls: 1,
		},
		{
			name:              "git switch with explicit branch wins over default",
			startRepo:         repoA,
			startBranch:       "main",
			patch:             sourcePatch{Repo: str(repoB), Branch: str("release")},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoB,
			wantBranch:        "release",
			wantValidateCalls: 1,
		},
		{
			name:              "git switch with explicit main wins over master default",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoB), Branch: str("main")},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoB,
			wantBranch:        "main",
			wantValidateCalls: 1,
		},
		{
			name:              "git switch with explicit branch ignores unbuildable default",
			startRepo:         repoA,
			startBranch:       "main",
			patch:             sourcePatch{Repo: str(repoB), Branch: str("release")},
			gh:                &fakeCloneTokens{defaultBranch: "-x;y"},
			wantRepo:          repoB,
			wantBranch:        "release",
			wantValidateCalls: 1,
		},
		{
			name:              "git switch when ValidateRepo has no opinion falls back to DefaultBranch",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoB)},
			gh:                &fakeCloneTokens{defaultBranch: ""},
			wantRepo:          repoB,
			wantBranch:        appv1alpha1.DefaultBranch,
			wantValidateCalls: 1,
		},
		{
			name:              "git switch when GitHub is nil falls back to DefaultBranch",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoB)},
			gh:                nil,
			wantRepo:          repoB,
			wantBranch:        appv1alpha1.DefaultBranch,
			wantValidateCalls: 0,
		},
		{
			name:              "same repo exact match without branch preserves custom branch",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoA)},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoA,
			wantBranch:        "custom",
			wantValidateCalls: 0,
		},
		{
			name:              "same canonical repo spelling (.git) without branch preserves custom branch",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Repo: str(repoA + ".git")},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoA + ".git",
			wantBranch:        "custom",
			wantValidateCalls: 1,
		},
		{
			name:              "same repo explicit empty branch restores DefaultBranch",
			startRepo:         repoA,
			startBranch:       "custom",
			patch:             sourcePatch{Branch: str("")},
			gh:                &fakeCloneTokens{defaultBranch: "master"},
			wantRepo:          repoA,
			wantBranch:        appv1alpha1.DefaultBranch,
			wantValidateCalls: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &recordingStore{}
			app := managedRepoApp("web")
			app.Spec.Repo = tc.startRepo
			app.Spec.Branch = tc.startBranch
			svc, cl := newService(st, app)
			if tc.gh != nil {
				svc.GitHub = tc.gh
			}

			v, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", tc.patch)
			if err != nil {
				t.Fatalf("SetSourceAndRegistryCredential: %v", err)
			}
			if v.Repo != tc.wantRepo || v.Branch != tc.wantBranch {
				t.Errorf("view repo=%q branch=%q, want %q / %q", v.Repo, v.Branch, tc.wantRepo, tc.wantBranch)
			}
			if spec := getApp(t, cl, "web").Spec; spec.Repo != tc.wantRepo || spec.Branch != tc.wantBranch {
				t.Errorf("CR spec repo=%q branch=%q, want %q / %q", spec.Repo, spec.Branch, tc.wantRepo, tc.wantBranch)
			}
			if tc.gh != nil && tc.gh.validateCalls != tc.wantValidateCalls {
				t.Errorf("ValidateRepo calls = %d, want %d", tc.gh.validateCalls, tc.wantValidateCalls)
			}
		})
	}
}

func TestRepoSwitchRefusesUnbuildableDefaultBranchWithoutMutating(t *testing.T) {
	const targetRepo = "https://github.com/render-examples/go-web-server"

	for _, surface := range []string{"service", "REST", "GraphQL"} {
		t.Run(surface, func(t *testing.T) {
			st := &recordingStore{}
			app := managedRepoApp("web")
			app.Spec.Repo = "https://github.com/bex-co/bex"
			app.Spec.Branch = "main"
			svc, cl := newService(st, app)
			svc.GitHub = &fakeCloneTokens{defaultBranch: "-x;y"}

			switch surface {
			case "service":
				repo := targetRepo
				_, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Repo: &repo})
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "cannot be built") {
					t.Fatalf("err = %v, want ErrBadRequest mentioning cannot be built", err)
				}
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web",
					strings.NewReader(`{"repo":"`+targetRepo+`"}`)))
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "cannot be built") {
					t.Fatalf("PATCH %d: %s", rec.Code, rec.Body.String())
				}
			case "GraphQL":
				res := graphql.Do(graphql.Params{
					Schema:        sourceSwapSchema(t, svc),
					Context:       context.Background(),
					RequestString: `mutation { setRepo(id: "web", repo: "` + targetRepo + `") { repo branch } }`,
				})
				if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "cannot be built") {
					t.Fatalf("setRepo errors = %v, want cannot be built", res.Errors)
				}
			}

			if spec := getApp(t, cl, "web").Spec; spec.Repo != "https://github.com/bex-co/bex" || spec.Branch != "main" {
				t.Errorf("rejected switch mutated CR: repo=%q branch=%q", spec.Repo, spec.Branch)
			}
			if len(st.sourceWrites) != 0 {
				t.Errorf("rejected switch wrote store rows: %+v", st.sourceWrites)
			}
		})
	}
}

func TestBuildSourceUnlinkAndUpdateResolveDefaultBranch(t *testing.T) {
	const targetRepo = "https://github.com/render-examples/go-web-server"

	t.Run("unlink service buildSource to master-default repo", func(t *testing.T) {
		svc, _, ctx := newBuildSourceService(t)
		created, err := svc.CreateBuildSource(ctx, monorepoSource)
		if err != nil {
			t.Fatalf("CreateBuildSource: %v", err)
		}
		web, err := svc.Create(ctx, CreateRequest{
			Name: "web", Type: appv1alpha1.TypeWebService, Runtime: "docker",
			Repo: "https://github.com/bex-co/bex", Branch: "main",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		startCmd := "node server.js"
		if _, err := svc.LinkBuildSource(ctx, web.ID, created.ID, &startCmd, false); err != nil {
			t.Fatalf("LinkBuildSource: %v", err)
		}
		gh := &fakeCloneTokens{defaultBranch: "master"}
		svc.GitHub = gh

		repo := targetRepo
		v, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Repo: &repo})
		if err != nil {
			t.Fatalf("SetSourceAndRegistryCredential: %v", err)
		}
		if v.Repo != targetRepo || v.Branch != "master" {
			t.Errorf("view = repo %q branch %q, want %q / master", v.Repo, v.Branch, targetRepo)
		}
		if gh.validateCalls != 1 {
			t.Errorf("ValidateRepo calls = %d, want 1", gh.validateCalls)
		}
	})

	t.Run("UpdateBuildSource switches git repo without branch", func(t *testing.T) {
		svc, _, ctx := newBuildSourceService(t)
		created, err := svc.CreateBuildSource(ctx, monorepoSource)
		if err != nil {
			t.Fatalf("CreateBuildSource: %v", err)
		}
		gh := &fakeCloneTokens{defaultBranch: "master"}
		svc.GitHub = gh

		view, err := svc.UpdateBuildSource(ctx, created.ID, UpdateBuildSourceRequest{
			Git: &BuildSourceGit{RepoURL: targetRepo},
		})
		if err != nil {
			t.Fatalf("UpdateBuildSource: %v", err)
		}
		if view.Git == nil || view.Git.RepoURL != targetRepo || view.Git.Branch != "master" {
			t.Errorf("buildSource view = %+v, want repo=%q branch=master", view.Git, targetRepo)
		}
		if gh.validateCalls != 1 {
			t.Errorf("ValidateRepo calls = %d, want 1", gh.validateCalls)
		}
	})
}
