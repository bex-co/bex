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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	gitlabBlueprintRepo    = "https://gitlab.com/render-examples/elasticsearch"
	bitbucketBlueprintRepo = "https://bitbucket.org/tutorials/markdowndemo"
)

// w8/346: since w8/344 bex reads a Blueprint from gitlab.com and bitbucket.org
// anonymously, but a failed read still answered in GitHub's words ("bex's
// GitHub app cannot access it", "Could not reach GitHub"), sending the author
// to a GitHub installation that has nothing to do with the repository. Each
// reason now names the repository's own host and no app.
func TestBlueprintFetchFailureNamesTheRepositoryHost(t *testing.T) {
	for _, tc := range []struct {
		name, repo string
		err        error
		reason     BlueprintPreviewReason
		want       string
	}{
		{"a missing GitLab repository", gitlabBlueprintRepo,
			fmt.Errorf("%w: gitlab.com has no public Git repository at this URL", github.ErrRepoNotFoundOrNoAccess),
			BlueprintFetchRepoNotFound, "Repository not found on gitlab.com, or it is not public. bex reads a Blueprint there from a public repository."},
		{"a GitLab outage", gitlabBlueprintRepo,
			errors.New("github: blueprint repository on gitlab.com did not answer: context deadline exceeded"),
			BlueprintFetchUnavailable, "Could not reach gitlab.com to read the Blueprint file. Try again in a few minutes."},
		{"a GitLab raw read that failed", gitlabBlueprintRepo,
			errors.New("github: gitlab.com raw file answered status 503"),
			BlueprintFetchUnavailable, "Could not reach gitlab.com to read the Blueprint file. Try again in a few minutes."},
		{"a Bitbucket repository not served publicly", bitbucketBlueprintRepo,
			fmt.Errorf("%w: bitbucket.org did not serve the file publicly", github.ErrRepoNotFoundOrNoAccess),
			BlueprintFetchRepoNotFound, "Repository not found on bitbucket.org, or it is not public. bex reads a Blueprint there from a public repository."},
		{"a Bitbucket refusal", bitbucketBlueprintRepo,
			fmt.Errorf("denied: %w", &github.APIError{Status: 403}),
			BlueprintFetchAccessDenied, "bitbucket.org did not let bex read this repository. bex reads a Blueprint there from a public repository."},
		{"a Bitbucket rate limit", bitbucketBlueprintRepo,
			fmt.Errorf("limited: %w", &github.APIError{Status: 429}),
			BlueprintFetchRateLimited, "bitbucket.org is rate-limiting bex right now. Try again in a few minutes."},
		// The github.com control keeps its wording, word for word.
		{"a missing GitHub repository", "https://github.com/a/app",
			fmt.Errorf("%w: %w", github.ErrRepoNotFoundOrNoAccess, &github.APIError{Status: 404}),
			BlueprintFetchRepoNotFound, "Repository not found, or bex's GitHub app cannot access it."},
		{"a GitHub refusal", "https://github.com/a/app",
			fmt.Errorf("github: denied: %w", &github.APIError{Status: 401}),
			BlueprintFetchAccessDenied, "bex's GitHub app is not authorized to read this repository. Re-check the installation and its repository access."},
		{"a GitHub rate limit", "https://github.com/a/app",
			fmt.Errorf("github: limited: %w", &github.APIError{Status: 429}),
			BlueprintFetchRateLimited, "GitHub is rate-limiting bex right now. Try again in a few minutes."},
		{"a GitHub outage", "git@github.com:a/app.git",
			fmt.Errorf("github: boom: %w", &github.APIError{Status: 500}),
			BlueprintFetchUnavailable, "Could not reach GitHub to read the Blueprint file. Try again in a few minutes."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{
				Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
				GitFetcher: fakeBlueprintFetcher{err: tc.err},
			}
			p, err := svc.PreviewBlueprint(context.Background(), "", tc.repo, "main", "", "")
			if err != nil || p.Found || p.Reason != tc.reason || p.Error != tc.want {
				t.Fatalf("preview = %q %q, %v; want %q %q", p.Reason, p.Error, err, tc.reason, tc.want)
			}
			if !strings.Contains(tc.repo, "github.com") && strings.Contains(p.Error, "GitHub") {
				t.Fatalf("a %s repository's refusal names GitHub: %q", tc.repo, p.Error)
			}
			assertNoUpstreamLeak(t, p.Error, testCommitSHA)
		})
	}
}

// A sync of a GitLab Blueprint records the failure in its host's words in the
// sync history, which REST, GraphQL, MCP and the detail page show, and the
// caller gets the same sentence.
func TestBlueprintSyncFailureNamesTheRepositoryHost(t *testing.T) {
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: gitlabBlueprintRepo,
		Branch: "master", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	svc := newBlueprintService(fs, fakeWorkspace{"user-a": "tea-a"})
	svc.GitFetcher = fakeBlueprintFetcher{err: errors.New("github: blueprint repository on gitlab.com did not answer: EOF")}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	const want = "Could not reach gitlab.com to read the Blueprint file. Try again in a few minutes."
	_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "", nil)
	if !errors.Is(err, core.ErrUnavailable) || err.Error() != want {
		t.Fatalf("SyncBlueprint = %v, want 503 %q", err, want)
	}
	if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].ErrorMessage == nil || *fs.insertedSyncs[0].ErrorMessage != string(BlueprintFetchUnavailable)+": "+want {
		t.Fatalf("sync history = %+v, want one run recording %q", fs.insertedSyncs, want)
	}
}

// With no branch, a preview or create looked the default branch up before the
// host refusal (w8/344's known gap): https://example.com/a/b was probed, and a
// refusal there read "bex's GitHub app cannot access it". A host bex reads no
// Blueprint from is now refused by name with no lookup, as a read with a
// branch refuses it, and a failed lookup on GitLab names gitlab.com.
func TestBlueprintWithoutBranchNamesTheRepositoryHost(t *testing.T) {
	ctx := ownershipCtx()
	svc, _ := connectionService(t)
	gh := &fakeCloneTokens{defaultBranch: "main"}
	svc.GitHub = gh

	const example = "https://example.com/a/b"
	p, err := svc.PreviewBlueprint(ctx, connOwner, example, "", "", "")
	if err != nil || p.Found || p.Reason != BlueprintFetchRepoNotFound ||
		!strings.Contains(p.Error, "example.com is not one of them") || strings.Contains(p.Error, "GitHub app") {
		t.Fatalf("preview = %q %q, %v; want the host refusal naming example.com", p.Reason, p.Error, err)
	}
	_, err = svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: example, Name: "bpExample"})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != string(BlueprintFetchRepoNotFound) ||
		!strings.Contains(err.Error(), "example.com is not one of them") {
		t.Fatalf("create = %v, want 400 %s naming example.com, as with a branch", err, BlueprintFetchRepoNotFound)
	}
	if gh.validateCalls != 0 {
		t.Fatalf("looked up example.com's default branch %d times, want none", gh.validateCalls)
	}

	for name, tc := range map[string]struct {
		err    error
		reason BlueprintPreviewReason
		want   string
	}{
		"not public Git": {fmt.Errorf("%w: repository is not accessible as public Git", core.ErrBadRequest),
			BlueprintFetchRepoNotFound, "Repository not found on gitlab.com, or it is not public."},
		"unreachable": {core.NewUnavailableError("UNAVAILABLE", "no answer", nil),
			BlueprintFetchUnavailable, "Could not reach gitlab.com to read the Blueprint file."},
	} {
		svc.GitHub = &fakeCloneTokens{validateErr: tc.err}
		p, err := svc.PreviewBlueprint(ctx, connOwner, gitlabBlueprintRepo, "", "", "")
		if err != nil || p.Found || p.Reason != tc.reason || !strings.HasPrefix(p.Error, tc.want) || strings.Contains(p.Error, "GitHub") {
			t.Errorf("%s: preview = %q %q, %v; want %q %q", name, p.Reason, p.Error, err, tc.reason, tc.want)
		}
	}
}

// w32/070: with no branch, both creates answered a missing repository with
// the default-branch lookup's own error, the service create's uncoded 400
// ("repository is not accessible through this workspace's GitHub connections
// …"), where the preview and a create with a branch answer 400
// repo_not_found_or_no_access. A lookup failure the preview classifies is now
// the preview's reason and sentence on both creates, coded as a read with a
// branch codes them; with a branch nothing changes.
func TestBlueprintCreateWithoutBranchAnswersAFailedLookupAsThePreviewDoes(t *testing.T) {
	const githubRepo = "https://github.com/octocat/qa-no-such-repo"
	for _, tc := range []struct {
		name, repo    string
		lookup, fetch error
		status        error
		reason        BlueprintPreviewReason
		want          string
	}{
		{"a missing GitHub repository", githubRepo,
			fmt.Errorf("%w: repository is not accessible through this workspace's GitHub connections or as public Git; connect its owner and grant repository access, or use a public repository", core.ErrBadRequest),
			fmt.Errorf("%w: %w", github.ErrRepoNotFoundOrNoAccess, &github.APIError{Status: 404}),
			core.ErrBadRequest, BlueprintFetchRepoNotFound, "Repository not found, or bex's GitHub app cannot access it."},
		{"a missing GitLab repository", gitlabBlueprintRepo,
			fmt.Errorf("%w: repository is not accessible as public Git; check its URL, or use a public repository", core.ErrBadRequest),
			fmt.Errorf("%w: gitlab.com has no public Git repository at this URL", github.ErrRepoNotFoundOrNoAccess),
			core.ErrBadRequest, BlueprintFetchRepoNotFound, "Repository not found on gitlab.com, or it is not public. bex reads a Blueprint there from a public repository."},
		{"a GitHub rate limit", githubRepo,
			core.NewUnavailableError(github.CodeGitHubRateLimited, "GitHub is rate-limiting bex right now. Try again in a few minutes.", map[string]any{"retryAfterSeconds": 30}),
			fmt.Errorf("github: limited: %w", &github.APIError{Status: 429}),
			core.ErrUnavailable, BlueprintFetchRateLimited, "GitHub is rate-limiting bex right now. Try again in a few minutes."},
	} {
		for _, branch := range []string{"", "main"} {
			t.Run(fmt.Sprintf("%s, branch %q", tc.name, branch), func(t *testing.T) {
				svc, st, ctx := newOnboardingService(t, onboardingManifest)
				gh := &fakeCloneTokens{validateErr: tc.lookup}
				svc.GitHub = gh
				if branch != "" {
					svc.GitFetcher = fakeBlueprintFetcher{err: tc.fetch}
				}
				p, err := svc.PreviewBlueprint(ctx, "tea-a", tc.repo, branch, "", "")
				if err != nil || p.Found || p.Reason != tc.reason || p.Error != tc.want {
					t.Fatalf("preview = %q %q, %v; want %q %q", p.Reason, p.Error, err, tc.reason, tc.want)
				}
				var sourceBranch *string
				if branch != "" {
					sourceBranch = &branch
				}
				for name, create := range map[string]func() error{
					"CreateBlueprint": func() error {
						_, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: tc.repo, Branch: branch, Name: "bp"})
						return err
					},
					"CreatePlannedBlueprint": func() error {
						_, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "bp", OwnerID: "tea-a",
							Source: RenderBlueprintSource{Repo: tc.repo, Branch: sourceBranch}})
						return err
					},
				} {
					err := create()
					var c *core.CodedError
					if !errors.Is(err, tc.status) || !errors.As(err, &c) || c.Code != string(tc.reason) || err.Error() != tc.want {
						t.Errorf("%s = %v, want %v %s %q, as the preview answers", name, err, tc.status, tc.reason, tc.want)
						continue
					}
					// GitHub's Retry-After survives the lookup's answer.
					wantRetry := any(nil)
					if branch == "" && tc.reason == BlueprintFetchRateLimited {
						wantRetry = 30
					}
					if got := c.Params["retryAfterSeconds"]; got != wantRetry {
						t.Errorf("%s retryAfterSeconds = %v, want %v", name, got, wantRetry)
					}
				}
				if branch != "" && gh.validateCalls != 0 {
					t.Errorf("looked up the default branch %d times with a branch, want none", gh.validateCalls)
				}
				if len(st.blueprints) != 0 {
					t.Errorf("a refused create saved %d Blueprints, want none", len(st.blueprints))
				}
			})
		}
	}
}
