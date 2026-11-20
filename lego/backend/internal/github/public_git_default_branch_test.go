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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// plainResponse is a host's answer that is not an advertisement.
func plainResponse(status int, contentType, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}

// TestValidateRepoReadsAnotherGitHostAsPublicGit is w8/343: a repository on
// a Git host other than github.com was "no opinion", so an omitted branch
// built `main` whatever the repository's default (gitlab-foss's is master)
// and a missing repository was created and failed at clone. ValidateRepo now
// reads its public advertisement, with the GitHub App unconfigured too.
func TestValidateRepoReadsAnotherGitHostAsPublicGit(t *testing.T) {
	head := func(caps string) string {
		return gitAdvertisement(testCommitA+" HEAD\x00"+caps+"\n", testCommitA+" refs/heads/master\n", testCommitB+" refs/heads/main\n")
	}
	for _, tc := range []struct {
		name    string
		answer  func() (*http.Response, error)
		want    string
		refused bool
	}{
		{name: "HEAD names master", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, head("multi_ack symref=HEAD:refs/heads/master agent=git/2")), nil
		}, want: "master"},
		{name: "HEAD names main", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, head("symref=HEAD:refs/heads/main")), nil
		}, want: "main"},
		{name: "version 1 advertisement", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, gitAdvertisement("version 1\n", testCommitA+" HEAD\x00symref=HEAD:refs/heads/trunk\n")), nil
		}, want: "trunk"},
		{name: "no symref has no default", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, head("multi_ack")), nil
		}},
		{name: "HEAD on a non-branch has no default", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, head("symref=HEAD:refs/tags/v1")), nil
		}},
		{name: "empty repository is readable", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, gitAdvertisement(strings.Repeat("0", 40)+" capabilities^{}\x00multi_ack\n")), nil
		}},
		{name: "advertisement past the bound", answer: func() (*http.Response, error) {
			filler := strings.Repeat(gitPacket(testCommitA+" refs/environments/e/deployments/1\n"), maxPublicGitAdvertisement/40)
			return gitResponse(http.StatusOK, head("symref=HEAD:refs/heads/master")+filler), nil
		}, want: "master"},
		{name: "malformed body that calls itself Git is not refused", answer: func() (*http.Response, error) {
			return gitResponse(http.StatusOK, "garbage"), nil
		}},
		{name: "dumb HTTP refs listing is readable", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusOK, "text/plain", testCommitA+"\trefs/heads/master\n"), nil
		}},
		{name: "GitLab missing or private 401", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusUnauthorized, "text/plain; charset=utf-8", "HTTP Basic: Access denied."), nil
		}, refused: true},
		{name: "403", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusForbidden, "text/plain", "forbidden"), nil
		}, refused: true},
		{name: "404 page", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusNotFound, "text/html", "<!doctype html>"), nil
		}, refused: true},
		{name: "410", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusGone, "text/plain", "gone"), nil
		}, refused: true},
		{name: "non-Git 200", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusOK, "text/html; charset=utf-8", "<!doctype html><title>Example Domain</title>"), nil
		}, refused: true},
		{name: "host name does not exist", answer: func() (*http.Response, error) {
			return nil, &net.DNSError{Err: "no such host", Name: "gitlabb.example", IsNotFound: true}
		}, refused: true},
		{name: "5xx has no opinion", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusBadGateway, "text/html", "bad gateway"), nil
		}},
		{name: "429 has no opinion", answer: func() (*http.Response, error) {
			return plainResponse(http.StatusTooManyRequests, "text/plain", "slow down"), nil
		}},
		{name: "transport failure has no opinion", answer: func() (*http.Response, error) {
			return nil, errors.New("dial tcp: i/o timeout")
		}},
		{name: "DNS failure that is not a missing name has no opinion", answer: func() (*http.Response, error) {
			return nil, &net.DNSError{Err: "server misbehaving", Name: "gitlab.example", IsTemporary: true}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if got, want := req.URL.String(), "https://gitlab.example/Group/Repo/info/refs?service=git-upload-pack"; got != want {
					t.Errorf("request URL = %q, want %q", got, want)
				}
				for _, header := range []string{"Authorization", "Cookie", "Git-Protocol"} {
					if req.Header.Get(header) != "" {
						t.Errorf("unexpected %s header", header)
					}
				}
				return tc.answer()
			})}}
			for range 2 {
				got, id, err := svc.DeployTokenSource().ValidateRepo(context.Background(), "default", "https://gitlab.example/Group/Repo")
				if got != tc.want || id != 0 || (err != nil) != tc.refused {
					t.Fatalf("ValidateRepo = %q, %d, %v; want %q, refused %v", got, id, err, tc.want, tc.refused)
				}
				if tc.refused {
					rec := httptest.NewRecorder()
					core.WriteErr(rec, err)
					if body := rec.Body.String(); rec.Code != http.StatusBadRequest || !strings.Contains(body, "not accessible as public Git") ||
						strings.Contains(body, "Access denied") || strings.Contains(body, "no such host") {
						t.Fatalf("WriteErr = %d %s, want a 400 naming public Git with no upstream text", rec.Code, body)
					}
				}
			}
			if calls != 1 {
				t.Errorf("advertisement reads = %d, want 1 (the second check is cached)", calls)
			}
		})
	}
}

// github.com keeps its own API path (w8/m76, w4/105), whatever shape its URL
// takes, and a URL the public Git reader cannot address has no opinion: the
// advertisement is never fetched for either.
func TestValidateRepoPublicGitLeavesGitHubAlone(t *testing.T) {
	svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("public Git read of %s", req.URL)
		return nil, errors.New("unexpected")
	})}}
	for _, repo := range []string{
		"https://github.com/octo/app",
		"https://GitHub.com/octo/app.git",
		"git@github.com:octo/app.git",
		"https://github.com/octo/app/tree/main",
		"https://gitlab.example/group/repo?ref=main",
		"https://localhost/group/repo",
		"https://127.0.0.1/group/repo",
	} {
		if got, _, err := svc.DeployTokenSource().ValidateRepo(context.Background(), "default", repo); got != "" || err != nil {
			t.Errorf("ValidateRepo(%s) with GitHub off = %q, %v; want no opinion", repo, got, err)
		}
	}
}

// A refusal lasts only a burst: a repository created, or made public, right
// after one is read again a few seconds later.
func TestPublicGitDefaultBranchRefusalExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls, exists := 0, false
		svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if !exists {
				return plainResponse(http.StatusUnauthorized, "text/plain", "denied"), nil
			}
			return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" HEAD\x00symref=HEAD:refs/heads/master\n")), nil
		})}}
		check := func(want string, refused bool) {
			t.Helper()
			got, _, err := svc.DeployTokenSource().ValidateRepo(context.Background(), "default", "https://gitlab.example/new/repo")
			if got != want || (err != nil) != refused {
				t.Fatalf("ValidateRepo = %q, %v; want %q, refused %v", got, err, want, refused)
			}
		}
		check("", true)
		exists = true
		check("", true)
		time.Sleep(11 * time.Second)
		check("master", false)
		if calls != 2 {
			t.Fatalf("advertisement reads = %d, want 2", calls)
		}
	})
}
