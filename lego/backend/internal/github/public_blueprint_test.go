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

package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// publicBlueprintHost serves one repository's advertisement and raw file,
// recording every URL the reads asked for.
type publicBlueprintHost struct {
	advertisement string // the advertisement URL it answers
	raw           string // the raw-file URL it answers
	refs          func() *http.Response
	file          func() *http.Response
	asked         []string
}

func (h *publicBlueprintHost) client() *http.Client {
	return &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		h.asked = append(h.asked, req.URL.String())
		switch req.URL.String() {
		case h.advertisement:
			return h.refs(), nil
		case h.raw:
			return h.file(), nil
		}
		return nil, errors.New("unexpected request " + req.URL.String())
	})}
}

func masterAdvertisement() *http.Response {
	return gitResponse(http.StatusOK, gitAdvertisement(
		testCommitA+" HEAD\x00symref=HEAD:refs/heads/master\n",
		testCommitA+" refs/heads/master\n",
		testCommitB+" refs/heads/feature\n",
	))
}

func rawFile(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader(body))}
}

// TestBlueprintOnAnotherHostIsReadFromThatHost is w8/344: a GitLab or
// Bitbucket Blueprint pinned its commit and read render.yaml from github.com
// under the same owner/repo. On production on 2026-11-20 a preview of
// https://gitlab.com/render-examples/elasticsearch answered GitHub's main
// (2c1495cc) and GitHub's manifest, although the GitLab repository has no main
// and its master (1bdf6f5d) holds a different file. Its branch now resolves
// from that repository's advertisement and its file from that host's raw
// endpoint, and the GitHub client is never called, even when the workspace's
// GitHub connection has the same owner.
func TestBlueprintOnAnotherHostIsReadFromThatHost(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, repo, advertisement, raw string
	}{
		{"gitlab", "https://gitlab.com/render-examples/elasticsearch",
			"https://gitlab.com/render-examples/elasticsearch/info/refs?service=git-upload-pack",
			"https://gitlab.com/render-examples/elasticsearch/-/raw/" + testCommitA + "/render.yaml"},
		{"gitlab .git URL", "https://gitlab.com/render-examples/elasticsearch.git/",
			"https://gitlab.com/render-examples/elasticsearch.git/info/refs?service=git-upload-pack",
			"https://gitlab.com/render-examples/elasticsearch/-/raw/" + testCommitA + "/render.yaml"},
		{"gitlab subgroup", "https://GitLab.com/render-examples/Sub/elasticsearch",
			"https://gitlab.com/render-examples/Sub/elasticsearch/info/refs?service=git-upload-pack",
			"https://gitlab.com/render-examples/Sub/elasticsearch/-/raw/" + testCommitA + "/render.yaml"},
		{"bitbucket", "https://bitbucket.org/render-examples/elasticsearch",
			"https://bitbucket.org/render-examples/elasticsearch/info/refs?service=git-upload-pack",
			"https://bitbucket.org/render-examples/elasticsearch/raw/" + testCommitA + "/render.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{token: "installation-token", commitSHA: testCommitB, fileContents: "services: [github]\n"}
			svc := newBlueprintSourceService(fc)
			svc.Store.(*fakeStore).conns = []store.GitConnection{{WorkspaceID: "ws-1", AccountLogin: "render-examples", InstallationID: 7}}
			host := &publicBlueprintHost{advertisement: tc.advertisement, raw: tc.raw, refs: masterAdvertisement,
				file: func() *http.Response { return rawFile(http.StatusOK, "services: [host]\n") }}
			svc.publicGitHTTPClient = host.client()
			f := svc.BlueprintFileFetcher()

			sha, err := f.ResolveBlueprintCommit(ctx, "ws-1", tc.repo, "master")
			if err != nil || sha != testCommitA {
				t.Fatalf("resolve = %q, %v; want the host's master %s", sha, err, testCommitA)
			}
			contents, err := f.FetchBlueprintFileAtCommit(ctx, "ws-1", tc.repo, sha, "render.yaml")
			if err != nil || contents != "services: [host]\n" {
				t.Fatalf("fetch = %q, %v; want the host's file", contents, err)
			}
			if fc.resolveCalls != 0 || fc.gotFileRef != nil || fc.gotMintInst != 0 {
				t.Fatalf("the GitHub client was called (resolve %d, file %v, mint %d)", fc.resolveCalls, fc.gotFileRef, fc.gotMintInst)
			}
			if len(host.asked) != 2 {
				t.Fatalf("asked %v, want one advertisement and one raw read", host.asked)
			}
		})
	}
}

// Each answer of another host's repository keeps the GitHub path's meaning,
// so the preview and sync reasons read the same: missing repository, missing
// branch, missing file, a host that did not answer.
func TestBlueprintOnAnotherHostClassifiesItsAnswers(t *testing.T) {
	ctx := context.Background()
	const repo = "https://gitlab.com/render-examples/elasticsearch"
	advertisement := "https://gitlab.com/render-examples/elasticsearch/info/refs?service=git-upload-pack"
	raw := "https://gitlab.com/render-examples/elasticsearch/-/raw/" + testCommitA + "/render.yaml"
	resolve := func(refs func() *http.Response, branch string) error {
		svc := newBlueprintSourceService(&fakeClient{commitSHA: testCommitB})
		svc.publicGitHTTPClient = (&publicBlueprintHost{advertisement: advertisement, refs: refs}).client()
		_, err := svc.BlueprintFileFetcher().ResolveBlueprintCommit(ctx, "ws-1", repo, branch)
		return err
	}
	for _, tc := range []struct {
		name   string
		refs   func() *http.Response
		branch string
		want   error // nil: an error that is neither sentinel
	}{
		{"missing repository", func() *http.Response { return rawFile(http.StatusNotFound, "") }, "master", ErrRepoNotFoundOrNoAccess},
		{"private repository", func() *http.Response { return rawFile(http.StatusUnauthorized, "") }, "master", ErrRepoNotFoundOrNoAccess},
		{"not Git", func() *http.Response { return plainResponse(http.StatusOK, "text/html", "<html>") }, "master", ErrRepoNotFoundOrNoAccess},
		{"missing branch", masterAdvertisement, "main", ErrBranchNotFound},
		{"a tag is not a branch", func() *http.Response {
			return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" HEAD\x00symref=HEAD:refs/heads/master\n", testCommitA+" refs/tags/main\n"))
		}, "main", ErrBranchNotFound},
		{"host outage", func() *http.Response { return rawFile(http.StatusBadGateway, "") }, "master", nil},
		{"rate limited", func() *http.Response { return rawFile(http.StatusTooManyRequests, "") }, "master", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := resolve(tc.refs, tc.branch)
			switch {
			case err == nil:
				t.Fatal("resolve succeeded, want an error")
			case tc.want != nil && !errors.Is(err, tc.want):
				t.Fatalf("resolve = %v, want %v", err, tc.want)
			case tc.want == nil && (errors.Is(err, ErrRepoNotFoundOrNoAccess) || errors.Is(err, ErrBranchNotFound) || errors.Is(err, core.ErrBadRequest)):
				t.Fatalf("resolve = %v, want an unavailable error, not the repository's answer", err)
			}
		})
	}

	read := func(file func() *http.Response) error {
		svc := newBlueprintSourceService(&fakeClient{fileContents: "services: [github]\n"})
		svc.publicGitHTTPClient = (&publicBlueprintHost{raw: raw, file: file}).client()
		_, err := svc.BlueprintFileFetcher().FetchBlueprintFileAtCommit(ctx, "ws-1", repo, testCommitA, "render.yaml")
		return err
	}
	var notFound *APIError
	if err := read(func() *http.Response { return rawFile(http.StatusNotFound, "<html>") }); !errors.As(err, &notFound) || notFound.Status != http.StatusNotFound {
		t.Errorf("raw 404 = %v, want the contents API's not-found", err)
	}
	// GitLab sends a project it does not show to its sign-in page; that page
	// is never read as the Blueprint.
	signIn := func() *http.Response {
		r := rawFile(http.StatusFound, "")
		r.Header.Set("Location", "https://gitlab.com/users/sign_in")
		return r
	}
	if err := read(signIn); !errors.Is(err, ErrRepoNotFoundOrNoAccess) {
		t.Errorf("raw redirect = %v, want ErrRepoNotFoundOrNoAccess", err)
	}
	if err := read(func() *http.Response { return rawFile(http.StatusOK, strings.Repeat("x", maxBlueprintFileBytes+1)) }); !errors.Is(err, errFileTooLarge) {
		t.Errorf("oversized raw file = %v, want errFileTooLarge", err)
	}
	if err := read(func() *http.Response { return rawFile(http.StatusServiceUnavailable, "") }); err == nil || errors.Is(err, ErrRepoNotFoundOrNoAccess) {
		t.Errorf("raw 503 = %v, want an unavailable error", err)
	}
}

// A Blueprint on any other host, or in a form its host is not read in, is
// refused with the host named, and nothing is read from anywhere: never
// github.com under the same owner/repo.
func TestBlueprintOnAnUnsupportedHostIsRefusedByName(t *testing.T) {
	ctx := context.Background()
	for repo, host := range map[string]string{
		"https://codeberg.org/forgejo/forgejo":   "codeberg.org",
		"https://example.com/a/b":                "example.com",
		"https://gitlab.example.com/a/b":         "gitlab.example.com",
		"https://evil.example/gitlab.com/a/b":    "evil.example",
		"git@gitlab.com:render-examples/app.git": "gitlab.com",
		"ssh://git@bitbucket.org/a/b.git":        "bitbucket.org",
		"http://gitlab.com/a/b":                  "gitlab.com",
		"https://gitlab.com:8443/a/b":            "gitlab.com",
		"https://user@gitlab.com/a/b":            "gitlab.com",
	} {
		t.Run(repo, func(t *testing.T) {
			fc := &fakeClient{commitSHA: testCommitB, fileContents: "services: [github]\n"}
			svc := newBlueprintSourceService(fc)
			asked := 0
			svc.publicGitHTTPClient = &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
				asked++
				return masterAdvertisement(), nil
			})}
			f := svc.BlueprintFileFetcher()
			_, resolveErr := f.ResolveBlueprintCommit(ctx, "ws-1", repo, "master")
			_, fetchErr := f.FetchBlueprintFileAtCommit(ctx, "ws-1", repo, testCommitA, "render.yaml")
			for _, err := range []error{resolveErr, fetchErr} {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != CodeBlueprintHostUnsupported || !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), host) {
					t.Fatalf("got %v, want a %s refusal naming %s", err, CodeBlueprintHostUnsupported, host)
				}
			}
			if asked != 0 || fc.resolveCalls != 0 || fc.gotFileRef != nil {
				t.Fatalf("read something: %d host requests, GitHub resolve %d, file %v", asked, fc.resolveCalls, fc.gotFileRef)
			}
		})
	}

	// A URL on a supported host that names no repository is the GitHub path's
	// owner/repo refusal.
	for _, repo := range []string{"https://bitbucket.org/a", "https://bitbucket.org/a/b/c", "https://gitlab.com/a/b/-/tree/main"} {
		svc := newBlueprintSourceService(&fakeClient{commitSHA: testCommitB})
		if _, err := svc.BlueprintFileFetcher().ResolveBlueprintCommit(ctx, "ws-1", repo, "master"); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s = %v, want a bad request", repo, err)
		}
	}
}

// The github.com control: a GitHub Blueprint still resolves and reads
// through the GitHub client, never another host's public reads.
func TestBlueprintOnGitHubKeepsTheGitHubPath(t *testing.T) {
	ctx := context.Background()
	fc := &fakeClient{commitSHA: testCommitB, fileContents: "services: [github]\n"}
	svc := newBlueprintSourceService(fc)
	f := svc.BlueprintFileFetcher()
	for _, repo := range []string{"https://github.com/render-examples/elasticsearch", "git@github.com:render-examples/elasticsearch.git"} {
		sha, err := f.ResolveBlueprintCommit(ctx, "ws-1", repo, "main")
		if err != nil || sha != testCommitB {
			t.Fatalf("%s resolve = %q, %v; want GitHub's %s", repo, sha, err, testCommitB)
		}
		contents, err := f.FetchBlueprintFileAtCommit(ctx, "ws-1", repo, sha, "render.yaml")
		if err != nil || contents != "services: [github]\n" {
			t.Fatalf("%s fetch = %q, %v; want GitHub's file", repo, contents, err)
		}
	}
	if fc.resolveCalls != 2 {
		t.Fatalf("GitHub resolved %d times, want 2", fc.resolveCalls)
	}
}

// A Blueprint's host refusal is answerable before any read (w8/346), so a
// preview or create with no branch refuses a host by name instead of probing
// it for a default branch, and a failed read names the host it went to.
func TestRefuseBlueprintHostNeedsNoRead(t *testing.T) {
	for repo, host := range map[string]string{
		"https://codeberg.org/forgejo/forgejo":   "codeberg.org",
		"https://example.com/a/b":                "example.com",
		"git@gitlab.com:render-examples/app.git": "gitlab.com",
		"https://gitlab.com:8443/a/b":            "gitlab.com",
	} {
		err := RefuseBlueprintHost(repo)
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != CodeBlueprintHostUnsupported || !strings.Contains(err.Error(), host) {
			t.Errorf("RefuseBlueprintHost(%s) = %v, want a %s refusal naming %s", repo, err, CodeBlueprintHostUnsupported, host)
		}
		if got := BlueprintHost(repo); got != host {
			t.Errorf("BlueprintHost(%s) = %q, want %q", repo, got, host)
		}
	}
	for repo, host := range map[string]string{
		"https://github.com/render-examples/elasticsearch": "",
		"git@github.com:render-examples/elasticsearch.git": "",
		"https://gitlab.com/render-examples/elasticsearch": "gitlab.com",
		"https://gitlab.com/group/sub/repo.git":            "gitlab.com",
		"https://bitbucket.org/tutorials/markdowndemo":     "bitbucket.org",
		// Not a repository URL: the read refuses it as such, not by host.
		"https://bitbucket.org/a": "bitbucket.org",
	} {
		if err := RefuseBlueprintHost(repo); err != nil {
			t.Errorf("RefuseBlueprintHost(%s) = %v, want nil", repo, err)
		}
		if got := BlueprintHost(repo); got != host {
			t.Errorf("BlueprintHost(%s) = %q, want %q", repo, got, host)
		}
	}
}
