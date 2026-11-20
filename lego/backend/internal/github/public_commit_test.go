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
	"testing/synctest"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The answers below are the hosts' own anonymous answers for
// gitlab.com/render-examples/flask-hello-world and
// bitbucket.org/tutorials/markdowndemo, trimmed to the fields read (w8/345).
const (
	gitlabCommit        = "d832d0dfa36e0afe7e9add9fb69b03cf29d8a226"
	gitlabCommitJSON    = `{"id":"d832d0dfa36e0afe7e9add9fb69b03cf29d8a226","short_id":"d832d0df","title":"Update app.py","message":"Update app.py","authored_date":"2019-05-28T13:09:29.000-07:00","committed_date":"2019-05-28T13:09:29.000-07:00"}`
	gitlabNoCommit      = `{"message":"404 Commit Not Found"}`
	gitlabNoProject     = `{"message":"404 Project Not Found"}`
	bitbucketCommit     = "3146839a8816e957e2981866fc35963e343c83be"
	bitbucketCommitJSON = `{"type": "commit", "hash": "3146839a8816e957e2981866fc35963e343c83be", "date": "2019-10-18T20:19:58+00:00", "message": "Fixed readme table of contents links\n"}`
	bitbucketNoCommit   = `{"type": "error", "error": {"message": "Commit not found", "data": {"shas": ["0123456789abcdef0123456789abcdef01234567"]}}}`
	unknownCommit       = "0123456789abcdef0123456789abcdef01234567"
)

// publicCommitHost is a repository's host: its advertisement (whose only
// branch tip is testCommitA) and the commit API's answer to every lookup,
// recording every URL asked for.
type publicCommitHost struct {
	advertisement string
	api           func(*http.Request) (*http.Response, error)
	asked         []string
}

func (h *publicCommitHost) client() *http.Client {
	return &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		h.asked = append(h.asked, req.URL.String())
		for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
			if req.Header.Get(header) != "" {
				return nil, errors.New("credential header " + header)
			}
		}
		if req.URL.String() == h.advertisement {
			return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" HEAD\x00symref=HEAD:refs/heads/master\n", testCommitA+" refs/heads/master\n")), nil
		}
		if h.api == nil {
			return nil, errors.New("unexpected request " + req.URL.String())
		}
		return h.api(req)
	})}
}

func apiAnswer(status int, body string) func(*http.Request) (*http.Response, error) {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}

// TestPublicHostCommitResolvesAPrefix is w8/345: on production on 2026-11-20
// `deploys create --commit d832d0d` of a GitLab service (and 3146839 on
// Bitbucket) recorded no commit, built "@master" and failed with "couldn't
// find remote ref d832d0d", since an advertisement lists only ref tips and
// upload-pack takes no prefix. The host's commit API now expands it to the
// full SHA the build pins, with the message and author time GitHub's path
// records.
func TestPublicHostCommitResolvesAPrefix(t *testing.T) {
	gitlabAt := time.Date(2019, 5, 28, 20, 9, 29, 0, time.UTC)
	bitbucketAt := time.Date(2019, 10, 18, 20, 19, 58, 0, time.UTC)
	for _, tc := range []struct {
		name, repo, advertisement, ref, api, answer string
		want                                        store.CommitInfo
	}{
		{"gitlab short", "https://gitlab.com/render-examples/flask-hello-world", "https://gitlab.com/render-examples/flask-hello-world/info/refs?service=git-upload-pack",
			"D832D0D", "https://gitlab.com/api/v4/projects/render-examples%2Fflask-hello-world/repository/commits/d832d0d", gitlabCommitJSON,
			store.CommitInfo{Hash: gitlabCommit, Message: "Update app.py", AuthorAt: &gitlabAt}},
		{"gitlab subgroup .git", "https://gitlab.com/Group/Sub/flask.git", "https://gitlab.com/Group/Sub/flask.git/info/refs?service=git-upload-pack",
			"d832d0df", "https://gitlab.com/api/v4/projects/Group%2FSub%2Fflask/repository/commits/d832d0df", gitlabCommitJSON,
			store.CommitInfo{Hash: gitlabCommit, Message: "Update app.py", AuthorAt: &gitlabAt}},
		{"gitlab full SHA no tip names", "https://gitlab.com/render-examples/flask-hello-world", "https://gitlab.com/render-examples/flask-hello-world/info/refs?service=git-upload-pack",
			gitlabCommit, "https://gitlab.com/api/v4/projects/render-examples%2Fflask-hello-world/repository/commits/" + gitlabCommit, gitlabCommitJSON,
			store.CommitInfo{Hash: gitlabCommit, Message: "Update app.py", AuthorAt: &gitlabAt}},
		{"bitbucket short", "https://bitbucket.org/tutorials/markdowndemo", "https://bitbucket.org/tutorials/markdowndemo/info/refs?service=git-upload-pack",
			"3146839", "https://api.bitbucket.org/2.0/repositories/tutorials/markdowndemo/commit/3146839", bitbucketCommitJSON,
			store.CommitInfo{Hash: bitbucketCommit, Message: "Fixed readme table of contents links", AuthorAt: &bitbucketAt}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &publicCommitHost{advertisement: tc.advertisement, api: apiAnswer(http.StatusOK, tc.answer)}
			svc := &Service{publicGitHTTPClient: host.client()}
			for range 2 { // the second is served from the cache
				info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", tc.repo, tc.ref)
				if err != nil || !ok || info.Hash != tc.want.Hash || info.Message != tc.want.Message || info.AuthorAt == nil || !info.AuthorAt.Equal(*tc.want.AuthorAt) || info.AuthorAt.Location() != time.UTC {
					t.Fatalf("resolve = %+v,%v,%v; want %+v", info, ok, err, tc.want)
				}
			}
			if got, want := strings.Join(host.asked, " "), tc.advertisement+" "+tc.api; got != want {
				t.Fatalf("asked %q, want %q", got, want)
			}
		})
	}
}

// A branch tip, a branch or tag name, a ref too short to name an object, and
// every repository on another host never reach a commit API.
func TestPublicHostCommitAskedOnlyForAnUnresolvedObjectName(t *testing.T) {
	for _, tc := range []struct{ repo, ref, want string }{
		{"https://gitlab.com/o/r", testCommitA, testCommitA},
		{"https://gitlab.com/o/r", "master", testCommitA},
		{"https://bitbucket.org/o/r", "no-such-branch", ""},
		{"https://gitlab.com/o/r", "d83", ""},
		{"https://git.example/o/r", "d832d0d", ""},
		{"https://gitlab.example.com/o/r", "d832d0d", ""},
	} {
		t.Run(tc.repo+"@"+tc.ref, func(t *testing.T) {
			host := &publicCommitHost{advertisement: strings.TrimSuffix(tc.repo, "/") + "/info/refs?service=git-upload-pack"}
			svc := &Service{publicGitHTTPClient: host.client()}
			info, ok, err := svc.resolvePublicGitCommit(context.Background(), tc.repo, tc.ref)
			if err != nil || ok != (tc.want != "") || info.Hash != tc.want || info.Message != "" {
				t.Fatalf("resolve = %+v,%v,%v; want %q from the advertisement alone", info, ok, err, tc.want)
			}
			if len(host.asked) != 1 {
				t.Fatalf("asked %q, want the advertisement only", host.asked)
			}
		})
	}
}

// A SHA the repository does not have opened a deploy that failed at checkout
// ("upload-pack: not our ref"). The host's "no such commit" is now GitHub's
// noSuchCommit, store.ErrCommitNotFound, which refuses the deploy with 404
// commit_not_found before anything is written; it is cached half a minute.
func TestPublicHostNoSuchCommitIsErrCommitNotFound(t *testing.T) {
	for _, tc := range []struct{ name, repo, answer string }{
		{"gitlab", "https://gitlab.com/render-examples/flask-hello-world", gitlabNoCommit},
		{"bitbucket", "https://bitbucket.org/tutorials/markdowndemo", bitbucketNoCommit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				host := &publicCommitHost{advertisement: tc.repo + "/info/refs?service=git-upload-pack", api: apiAnswer(http.StatusNotFound, tc.answer)}
				svc := &Service{publicGitHTTPClient: host.client()}
				for _, ref := range []string{unknownCommit, unknownCommit, "0123456"} {
					info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", tc.repo, ref)
					if !errors.Is(err, store.ErrCommitNotFound) || ok || info.Hash != "" {
						t.Fatalf("%s = %+v,%v,%v; want store.ErrCommitNotFound", ref, info, ok, err)
					}
				}
				if len(host.asked) != 4 { // the repeated full SHA is served from the cache
					t.Fatalf("asked %d times: %q", len(host.asked), host.asked)
				}
				time.Sleep(publicCommitNotFoundTTL + time.Second)
				_, _, _ = svc.resolvePublicGitCommit(context.Background(), tc.repo, unknownCommit)
				if len(host.asked) != 5 { // the advertisement's miss lasts publicCommitMissTTL
					t.Fatalf("after the TTL asked %d times, want the API asked again", len(host.asked))
				}
			})
		})
	}
}

// Every other answer is a miss, as the advertisement's failures are: a
// missing or private repository answers like an outage, so the lookup is no
// existence oracle, and a deploy still opens. A full SHA is then recorded as
// requested (w8/209); a prefix records nothing.
func TestPublicHostCommitFailuresAreMisses(t *testing.T) {
	redirect := func(req *http.Request) (*http.Response, error) {
		if strings.HasPrefix(req.URL.Host, "elsewhere") {
			return nil, errors.New("followed a redirect")
		}
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://elsewhere.example/commit"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	for _, tc := range []struct {
		name string
		api  func(*http.Request) (*http.Response, error)
	}{
		{"gitlab project not found", apiAnswer(http.StatusNotFound, gitlabNoProject)},
		{"bitbucket repository not found page", apiAnswer(http.StatusNotFound, "<!DOCTYPE html><html>Commit not found</html>")},
		{"404 another message", apiAnswer(http.StatusNotFound, `{"error":{"message":"Repository not found"}}`)},
		{"401", apiAnswer(http.StatusUnauthorized, gitlabNoCommit)},
		{"403", apiAnswer(http.StatusForbidden, bitbucketNoCommit)},
		{"429", apiAnswer(http.StatusTooManyRequests, `{"message":"429 Too Many Requests"}`)},
		{"500", apiAnswer(http.StatusInternalServerError, gitlabNoCommit)},
		{"redirect refused", redirect},
		{"transport error", func(*http.Request) (*http.Response, error) { return nil, errors.New("connection reset") }},
		{"malformed 200", apiAnswer(http.StatusOK, `{"id":`)},
		{"200 for another commit", apiAnswer(http.StatusOK, `{"id":"`+testCommitB+`","message":"other"}`)},
		{"200 with a short id", apiAnswer(http.StatusOK, `{"id":"d832d0d","message":"short"}`)},
		{"oversized 200", apiAnswer(http.StatusOK, `{"id":"`+gitlabCommit+`","message":"`+strings.Repeat("x", maxBlueprintFileBytes)+`"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &publicCommitHost{advertisement: "https://gitlab.com/render-examples/flask-hello-world/info/refs?service=git-upload-pack", api: tc.api}
			svc := &Service{publicGitHTTPClient: host.client()}
			for range 2 { // a miss is cached
				info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "https://gitlab.com/render-examples/flask-hello-world", "d832d0d")
				if err != nil || ok || info.Hash != "" {
					t.Fatalf("prefix = %+v,%v,%v; want a miss", info, ok, err)
				}
			}
			info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "https://gitlab.com/render-examples/flask-hello-world", gitlabCommit)
			if err != nil || !ok || info.Hash != gitlabCommit || info.Message != "" || info.AuthorAt != nil {
				t.Fatalf("full SHA = %+v,%v,%v; want it recorded as requested", info, ok, err)
			}
			if len(host.asked) != 4 {
				t.Fatalf("asked %d times, want one advertisement read and one lookup per ref: %q", len(host.asked), host.asked)
			}
		})
	}
}

// A host that does not answer costs the deploy at most the public lookups'
// five-second budget, and the miss is cached.
func TestPublicHostCommitTimeoutIsACachedMiss(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		host := &publicCommitHost{advertisement: "https://bitbucket.org/tutorials/markdowndemo/info/refs?service=git-upload-pack", api: func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}}
		svc := &Service{publicGitHTTPClient: host.client()}
		start := time.Now()
		info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://bitbucket.org/tutorials/markdowndemo", "3146839")
		if err != nil || ok || info.Hash != "" || time.Since(start) != publicCommitResolveTimeout {
			t.Fatalf("timeout = %+v,%v,%v after %s", info, ok, err, time.Since(start))
		}
		_, _, _ = svc.resolvePublicGitCommit(context.Background(), "https://bitbucket.org/tutorials/markdowndemo", "3146839")
		if len(host.asked) != 2 {
			t.Fatalf("asked %q, want the miss cached", host.asked)
		}
	})
}
