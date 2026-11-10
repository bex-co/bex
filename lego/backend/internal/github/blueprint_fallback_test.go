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
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/m150 t002: an idempotent GitHub read retries a 5xx or a transport error,
// never a 4xx or a write, and never past its deadline.
func TestRetryTransportRetriesOnlyTransientReads(t *testing.T) {
	for _, tc := range []struct {
		name      string
		method    string
		answers   []int // 0 = transport error
		wantCalls int32
		wantCode  int
	}{
		{"502 then 200", http.MethodGet, []int{502, 200}, 2, 200},
		{"reset then 200", http.MethodGet, []int{0, 200}, 2, 200},
		{"three 503s return the last", http.MethodGet, []int{503, 503, 503, 200}, 3, 503},
		{"404 is the repository's answer", http.MethodGet, []int{404, 200}, 1, 404},
		{"429 is not retried", http.MethodGet, []int{429, 200}, 1, 429},
		{"a write is not retried", http.MethodPost, []int{502, 200}, 1, 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			base := publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
				n := calls.Add(1)
				code := tc.answers[n-1]
				if code == 0 {
					return nil, errors.New("http2: stream reset")
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("x"))}, nil
			})
			client := &http.Client{Transport: retryTransport{base: base, wait: func(int) time.Duration { return 0 }}}
			req, _ := http.NewRequest(tc.method, "https://api.github.com/x", nil)
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != tc.wantCode || calls.Load() != tc.wantCalls {
				t.Fatalf("got %v, %v after %d calls; want %d after %d", resp, err, calls.Load(), tc.wantCode, tc.wantCalls)
			}
		})
	}

	// A request whose deadline has passed stops retrying.
	var calls atomic.Int32
	slow := retryTransport{base: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
	}), wait: func(int) time.Duration { return time.Hour }}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/x", nil)
	if _, err := slow.RoundTrip(req); !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("expiring context: err %v after %d calls, want DeadlineExceeded after 1", err, calls.Load())
	}
}

// fallbackNet serves a public repo over git transport and GitHub's raw CDN,
// counting what the fallback reached.
type fallbackNet struct {
	refs, raw atomic.Int32
	rawStatus int
}

func (n *fallbackNet) client() *http.Client {
	return &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "github.com":
			n.refs.Add(1)
			return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" HEAD\x00symref=HEAD:refs/heads/main\n", testCommitA+" refs/heads/main\n")), nil
		case "raw.githubusercontent.com":
			n.raw.Add(1)
			if !strings.HasSuffix(req.URL.Path, "/"+testCommitA+"/examples/render.yaml") {
				return nil, errors.New("unexpected raw path " + req.URL.Path)
			}
			status := n.rawStatus
			if status == 0 {
				status = http.StatusOK
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("services: []\n"))}, nil
		}
		return nil, errors.New("unexpected host " + req.URL.Host)
	})}
}

// w8/m150 t003: when GitHub's API fails for a public repo, the branch resolves
// over git transport and the file reads from the raw CDN at that commit.
func TestBlueprintPublicRepoFallsBackWhenTheAPIFails(t *testing.T) {
	ctx := context.Background()
	for _, apiErr := range []error{&APIError{Status: http.StatusBadGateway}, errors.New("http2: stream reset"), context.DeadlineExceeded} {
		t.Run(apiErr.Error(), func(t *testing.T) {
			if errors.Is(apiErr, context.DeadlineExceeded) {
				apiErr = &httptestTimeout{}
			}
			net := &fallbackNet{}
			svc := newBlueprintSourceService(&fakeClient{commitSHAErr: apiErr, fileErr: apiErr})
			svc.publicGitHTTPClient = net.client()
			f := svc.BlueprintFileFetcher()
			sha, err := f.ResolveBlueprintCommit(ctx, "ws-1", "https://github.com/bex-co/bex", "main")
			if err != nil || sha != testCommitA {
				t.Fatalf("resolve = %q, %v; want %s over git transport", sha, err, testCommitA)
			}
			contents, err := f.FetchBlueprintFileAtCommit(ctx, "ws-1", "https://github.com/bex-co/bex", sha, "examples/render.yaml")
			if err != nil || contents != "services: []\n" || net.refs.Load() != 1 || net.raw.Load() != 1 {
				t.Fatalf("fetch = %q, %v (refs %d, raw %d); want the raw CDN's file", contents, err, net.refs.Load(), net.raw.Load())
			}
		})
	}

	// A raw 404 answers as the API's not-found does.
	net := &fallbackNet{rawStatus: http.StatusNotFound}
	svc := newBlueprintSourceService(&fakeClient{fileErr: &APIError{Status: http.StatusServiceUnavailable}})
	svc.publicGitHTTPClient = net.client()
	_, err := svc.BlueprintFileFetcher().FetchBlueprintFileAtCommit(ctx, "ws-1", "https://github.com/bex-co/bex", testCommitA, "examples/render.yaml")
	var notFound *APIError
	if !errors.As(err, &notFound) || notFound.Status != http.StatusNotFound {
		t.Fatalf("raw 404 = %v, want the contents API's not-found", err)
	}
}

// The fallback answers only for the platform's failures on anonymous reads:
// a 404 is the repository's answer, and a connected (private) repo never
// leaves its installation's API.
func TestBlueprintFallbackKeepsTheRepositorysAnswers(t *testing.T) {
	ctx := context.Background()
	net := &fallbackNet{}
	svc := newBlueprintSourceService(&fakeClient{commitSHAErr: &APIError{Status: http.StatusNotFound, Body: `{"message":"Branch not found"}`}, fileErr: &APIError{Status: http.StatusNotFound}})
	svc.publicGitHTTPClient = net.client()
	f := svc.BlueprintFileFetcher()
	if _, err := f.ResolveBlueprintCommit(ctx, "ws-1", "https://github.com/bex-co/bex", "nope"); !errors.Is(err, ErrBranchNotFound) {
		t.Errorf("API 404 branch = %v, want ErrBranchNotFound", err)
	}
	if _, err := f.FetchBlueprintFileAtCommit(ctx, "ws-1", "https://github.com/bex-co/bex", testCommitA, "examples/render.yaml"); err == nil {
		t.Error("API 404 file: want the not-found error")
	}

	connected := newBlueprintSourceService(&fakeClient{token: "installation-token", commitSHAErr: &APIError{Status: http.StatusBadGateway}, fileErr: &APIError{Status: http.StatusBadGateway}})
	connected.Store.(*fakeStore).conns = []store.GitConnection{{WorkspaceID: "ws-1", AccountLogin: "bex-co", InstallationID: 7}}
	connected.publicGitHTTPClient = net.client()
	cf := connected.BlueprintFileFetcher()
	if _, err := cf.ResolveBlueprintCommit(ctx, "ws-1", "https://github.com/bex-co/bex", "main"); err == nil {
		t.Error("connected repo resolve: want the API's error")
	}
	if _, err := cf.FetchBlueprintFileAtCommit(ctx, "ws-1", "https://github.com/bex-co/bex", testCommitA, "examples/render.yaml"); err == nil {
		t.Error("connected repo fetch: want the API's error")
	}
	if net.refs.Load() != 0 || net.raw.Load() != 0 {
		t.Fatalf("the fallback answered a 404 or a connected repo (refs %d, raw %d)", net.refs.Load(), net.raw.Load())
	}
}

// httptestTimeout is a client timeout as net/http reports one.
type httptestTimeout struct{}

func (*httptestTimeout) Error() string   { return "Client.Timeout exceeded while awaiting headers" }
func (*httptestTimeout) Timeout() bool   { return true }
func (*httptestTimeout) Temporary() bool { return true }
