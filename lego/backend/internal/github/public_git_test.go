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
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type publicGitRoundTripFunc func(*http.Request) (*http.Response, error)

func (f publicGitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func gitPacket(payload string) string { return fmt.Sprintf("%04x%s", len(payload)+4, payload) }

func gitAdvertisement(records ...string) string {
	var body strings.Builder
	body.WriteString(gitPacket("# service=git-upload-pack\n"))
	body.WriteString("0000")
	for _, record := range records {
		body.WriteString(gitPacket(record))
	}
	body.WriteString("0000")
	return body.String()
}

func gitResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {publicGitAdvertisementType}}, Body: io.NopCloser(strings.NewReader(body))}
}

func missingPublicGitClient() *http.Client {
	return &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return gitResponse(http.StatusNotFound, "missing"), nil
	})}
}

func TestPublicGitCommitAdvertisedRefs(t *testing.T) {
	const tagObject = "cccccccccccccccccccccccccccccccccccccccc"
	sha256 := strings.Repeat("d", 64)
	body := gitAdvertisement(
		testCommitA+" HEAD\x00symref=HEAD:refs/heads/main multi_ack\n",
		testCommitA+" refs/heads/main\n",
		testCommitB+" refs/heads/Feature/Case\n",
		testCommitB+" refs/heads/release\n",
		tagObject+" refs/tags/release\n",
		testCommitA+" refs/tags/release^{}\n",
		tagObject+" refs/archive/tag-object\n",
		tagObject+" refs/tags/v1\n",
		testCommitA+" refs/tags/v1^{}\n",
		testCommitB+" refs/tags/lightweight\n",
		sha256+" refs/heads/sha256\n",
	)
	for _, tc := range []struct{ ref, want string }{
		{"HEAD", testCommitA}, {"main", testCommitA}, {"refs/heads/main", testCommitA},
		{"Feature/Case", testCommitB}, {"feature/case", ""}, {"release", testCommitB},
		{"refs/tags/release", testCommitA}, {"v1", testCommitA}, {"lightweight", testCommitB},
		{testCommitA, testCommitA}, {strings.ToUpper(testCommitB), testCommitB},
		{tagObject, testCommitA}, {strings.Repeat("e", 40), strings.Repeat("e", 40)},
		{strings.Repeat("E", 64), strings.Repeat("e", 64)}, {sha256, sha256}, {"180e352", ""},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got, want := req.URL.String(), "https://git.example/Group/Sub/Repo/info/refs?service=git-upload-pack"; got != want {
					t.Errorf("request URL = %q, want %q", got, want)
				}
				if req.Method != http.MethodGet || req.Header.Get("Accept") != publicGitAdvertisementType {
					t.Errorf("request method/Accept = %s/%q", req.Method, req.Header.Get("Accept"))
				}
				for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Git-Protocol"} {
					if req.Header.Get(header) != "" {
						t.Errorf("unexpected %s header", header)
					}
				}
				return gitResponse(http.StatusOK, body), nil
			})}}
			info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/Group/Sub/Repo/", tc.ref)
			if err != nil || ok != (tc.want != "") || info.Hash != tc.want || info.Message != "" || info.AuthorAt != nil {
				t.Fatalf("resolve = %+v,%v,%v; want hash %q and no fabricated metadata", info, ok, err, tc.want)
			}
		})
	}
}

func TestPublicGitResolveWithGitHubDisabled(t *testing.T) {
	body := gitAdvertisement(testCommitA + " refs/heads/main\x00\n")
	svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return gitResponse(http.StatusOK, body), nil
	})}}
	info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "https://git.example/project.git", "main")
	if err != nil || !ok || info.Hash != testCommitA {
		t.Fatalf("GitHub-disabled public resolve = %+v,%v,%v", info, ok, err)
	}
}

// w8/209 hosted run: `deploys create --commit <full sha>` on a GitLab repo
// recorded commit null and narrated "@main". An explicit full SHA is the
// commit the clone pins and verifies, so it is recorded as requested whether
// the advertisement lists it (historical commits never are) or the lookup
// missed outright; an abbreviated SHA and a non-GitHub branch miss stay misses,
// and github.com keeps its own API path.
func TestPublicGitExplicitFullSHAIsRecordedAsRequested(t *testing.T) {
	const historical = "180e352cc63d9c887ce4b1bffe84c6d5def07cd5"
	advertised := &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" refs/heads/main\x00\n")), nil
	})}
	for name, client := range map[string]*http.Client{"advertised elsewhere": advertised, "lookup missed": missingPublicGitClient()} {
		t.Run(name, func(t *testing.T) {
			svc := &Service{publicGitHTTPClient: client}
			for range 2 { // the second call is served from the cached miss
				info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "https://gitlab.com/pages/plain-html", strings.ToUpper(historical))
				if err != nil || !ok || info.Hash != historical || info.Message != "" || info.AuthorAt != nil {
					t.Fatalf("explicit SHA = %+v,%v,%v; want %s with no fabricated metadata", info, ok, err, historical)
				}
			}
			for _, ref := range []string{"180e352", "no-such-branch"} {
				if info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "https://gitlab.com/pages/plain-html", ref); err != nil || ok || info.Hash != "" {
					t.Fatalf("%s = %+v,%v,%v; want a miss", ref, info, ok, err)
				}
			}
		})
	}
	svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("a github.com repo reached the generic Git transport")
		return nil, nil
	})}}
	if info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "https://github.com/o/r", historical); err != nil || ok || info.Hash != "" {
		t.Fatalf("GitHub with the App disabled = %+v,%v,%v; want its unchanged miss", info, ok, err)
	}
	if info, ok, err := svc.DeployCommitSource().ResolveCommit(context.Background(), "default", "http://gitlab.com/pages/plain-html", historical); err != nil || ok || info.Hash != "" {
		t.Fatalf("non-HTTPS repo = %+v,%v,%v; want a miss", info, ok, err)
	}
}

func TestPublicGitMalformedAdvertisementIsCachedMiss(t *testing.T) {
	prefix := gitPacket("# service=git-upload-pack\n") + "0000"
	valid := gitAdvertisement(testCommitA + " refs/heads/main\x00\n")
	cases := map[string]string{
		"missing service":      "0000",
		"missing header flush": gitPacket("# service=git-upload-pack\n") + gitPacket(testCommitA+" refs/heads/main\x00\n") + "0000",
		"bad hex":              prefix + "+01e" + strings.Repeat("x", 26),
		"truncated":            prefix + "003fshort",
		"packet oversized":     prefix + "ffff" + strings.Repeat("x", 65531),
		"missing capabilities": gitAdvertisement(testCommitA + " refs/heads/main\n"),
		"later capabilities":   gitAdvertisement(testCommitA+" HEAD\x00\n", testCommitA+" refs/heads/main\x00caps\n"),
		"duplicate version":    prefix + gitPacket("version 1\n") + gitPacket("version 1\n") + gitPacket(testCommitA+" refs/heads/main\x00\n") + "0000",
		"conflicting ref":      gitAdvertisement(testCommitA+" refs/heads/main\x00\n", testCommitB+" refs/heads/main\n"),
		"invalid ref":          gitAdvertisement(testCommitA + " refs/heads/../main\x00\n"),
		"zero object":          gitAdvertisement(strings.Repeat("0", 40) + " refs/heads/main\x00\n"),
		"error packet":         prefix + gitPacket("ERR not permitted\n") + "0000",
		"trailing bytes":       valid + "ignored",
		"missing final flush":  strings.TrimSuffix(valid, "0000"),
		"body oversized":       strings.Repeat("x", maxPublicGitAdvertisement+1),
		"too many packets":     prefix + gitPacket(testCommitA+" refs/heads/main\x00\n") + strings.Repeat(gitPacket(testCommitA+" refs/heads/main\n"), maxPublicGitPackets) + "0000",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			calls := 0
			svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return gitResponse(http.StatusOK, body), nil })}}
			for range 2 {
				info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/project", "main")
				if err != nil || ok || info.Hash != "" {
					t.Fatalf("malformed resolve = %+v,%v,%v", info, ok, err)
				}
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want cached miss", calls)
			}
		})
	}
}

func TestPublicGitProtocolVariants(t *testing.T) {
	for name, body := range map[string]string{
		"v1":                  gitPacket("# service=git-upload-pack") + "0000" + gitPacket("version 1\n") + gitPacket(testCommitA+" refs/heads/main\x00symref=HEAD:refs/heads/main") + "0000",
		"symref fallback":     gitAdvertisement(testCommitA + " refs/heads/main\x00symref=HEAD:refs/heads/main\n"),
		"duplicate same hash": gitAdvertisement(testCommitA+" HEAD\x00\n", testCommitA+" HEAD\n"),
	} {
		t.Run(name, func(t *testing.T) {
			refs, head, ok := parsePublicGitAdvertisement([]byte(body))
			if !ok || publicGitAdvertisedCommit(refs, head, "HEAD") != testCommitA {
				t.Fatalf("variant refs=%v head=%q ok=%v", refs, head, ok)
			}
		})
	}
	refs, _, ok := parsePublicGitAdvertisement([]byte(gitAdvertisement(strings.Repeat("0", 40) + " capabilities^{}\x00\n")))
	if !ok || len(refs) != 0 {
		t.Fatalf("empty repository = %v,%v", refs, ok)
	}
}

func TestPublicGitInputValidationAndPathCase(t *testing.T) {
	for _, raw := range []string{"http://git.example/repo", "ssh://git.example/repo", "https://user:pass@git.example/repo", "https://git.example/repo?token=x", "https://git.example/repo?", "https://git.example/repo#ref", "https://127.0.0.1/repo", "https://10.0.0.1/repo", "https://169.254.169.254/repo", "https://[::1]/repo", "https://localhost/repo", "https://a.localhost/repo", "https://git.example:0/repo", "https://git.example:65536/repo", "https://git.example/repo/../other", "https://git.example/repo%2f", "https://git.example", "not a URL"} {
		t.Run(raw, func(t *testing.T) {
			svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("invalid URL reached transport"); return nil, nil })}}
			if info, ok, err := svc.resolvePublicGitCommit(context.Background(), raw, "main"); err != nil || ok || info.Hash != "" {
				t.Fatalf("invalid URL = %+v,%v,%v", info, ok, err)
			}
		})
	}
	var paths []string
	svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		return gitResponse(http.StatusNotFound, ""), nil
	})}}
	for _, raw := range []string{"https://GIT.example/Group/Repo", "https://git.example/Group/Repo/", "https://git.example/group/repo", "https://git.example/Group/Repo.git"} {
		_, _, _ = svc.resolvePublicGitCommit(context.Background(), raw, "main")
	}
	if got, want := strings.Join(paths, ","), "/Group/Repo/info/refs,/group/repo/info/refs,/Group/Repo.git/info/refs"; got != want {
		t.Fatalf("paths=%q,want %q", got, want)
	}
}

func TestPublicGitCacheTTLAndTimeout(t *testing.T) {
	t.Run("success and miss TTL", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			hash := testCommitA
			status := http.StatusOK
			svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return gitResponse(status, gitAdvertisement(hash+" refs/heads/main\x00\n")), nil
			})}}
			resolve := func(want string) {
				t.Helper()
				info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/repo", "main")
				if err != nil || info.Hash != want || ok != (want != "") {
					t.Fatalf("resolve=%+v,%v,%v want %q", info, ok, err, want)
				}
			}
			resolve(testCommitA)
			hash = testCommitB
			resolve(testCommitA)
			time.Sleep(11 * time.Second)
			resolve(testCommitB)
			status = http.StatusForbidden
			time.Sleep(11 * time.Second)
			resolve("")
			status = http.StatusOK
			time.Sleep(15 * time.Second)
			resolve("")
			time.Sleep(2 * time.Minute)
			resolve(testCommitB)
			if calls != 4 {
				t.Fatalf("calls=%d,want4", calls)
			}
		})
	})
	t.Run("five second budget and cached cancellation", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			svc := &Service{publicGitHTTPClient: &http.Client{Timeout: time.Hour, Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				<-req.Context().Done()
				return nil, req.Context().Err()
			})}}
			start := time.Now()
			info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/slow", "main")
			if err != nil || ok || info.Hash != "" || time.Since(start) != 5*time.Second {
				t.Fatalf("timeout=%+v,%v,%v after%s", info, ok, err, time.Since(start))
			}
			_, _, _ = svc.resolvePublicGitCommit(context.Background(), "https://git.example/slow", "main")
			if calls != 1 {
				t.Fatalf("timeout calls=%d,want1", calls)
			}
		})
	})
	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		svc := &Service{publicGitHTTPClient: missingPublicGitClient()}
		if info, ok, err := svc.resolvePublicGitCommit(ctx, "https://git.example/slow", "main"); err != nil || ok || info.Hash != "" {
			t.Fatalf("canceled=%+v,%v,%v", info, ok, err)
		}
	})
}

func TestPublicGitConcurrentLookupsShareRequest(t *testing.T) {
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" refs/heads/main\x00\n")), nil
	})}}
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/repo", "main")
			if err != nil || !ok || info.Hash != testCommitA {
				t.Errorf("concurrent=%+v,%v,%v", info, ok, err)
			}
		})
	}
	<-entered
	close(release)
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls=%d,want1", calls.Load())
	}
}

func TestPublicGitRedirectsStayAnonymousAndHTTPS(t *testing.T) {
	for _, tc := range []struct {
		name, location string
		allowed        bool
	}{
		{"https", "https://other.example/new/info/refs?service=git-upload-pack", true},
		{"relative", "/moved/info/refs?service=git-upload-pack", true},
		{"http", "http://other.example/repo", false}, {"userinfo", "https://user:pass@other.example/repo", false},
		{"loopback", "https://127.0.0.1/repo", false}, {"private", "https://10.0.0.1/repo", false}, {"fragment", "https://other.example/repo#secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			jar, _ := cookiejar.New(nil)
			u, _ := url.Parse("https://git.example")
			jar.SetCookies(u, []*http.Cookie{{Name: "secret", Value: "credential"}})
			svc := &Service{publicGitHTTPClient: &http.Client{Jar: jar, Transport: publicGitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
					if req.Header.Get(name) != "" {
						t.Errorf("unexpected credential header %s", name)
					}
				}
				if calls == 1 {
					r := gitResponse(http.StatusFound, "")
					r.Header.Set("Location", tc.location)
					return r, nil
				}
				return gitResponse(http.StatusOK, gitAdvertisement(testCommitA+" refs/heads/main\x00\n")), nil
			})}}
			info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/repo", "main")
			if err != nil || ok != tc.allowed || ((info.Hash != "") != tc.allowed) {
				t.Fatalf("redirect=%+v,%v,%v", info, ok, err)
			}
			want := 1
			if tc.allowed {
				want = 2
			}
			if calls != want {
				t.Fatalf("requests=%d,want%d", calls, want)
			}
		})
	}
	t.Run("three redirect maximum", func(t *testing.T) {
		calls := 0
		svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			r := gitResponse(http.StatusFound, "")
			r.Header.Set("Location", fmt.Sprintf("https://git.example/redirect/%d", calls))
			return r, nil
		})}}
		_, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/repo", "main")
		if ok || err != nil || calls != 4 {
			t.Fatalf("redirect loop ok=%v err=%v calls=%d", ok, err, calls)
		}
	})
}

func TestPublicGitProductionTransport(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	transport, ok := anonymousPublicGitClient.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.DialTLSContext != nil || transport.DialContext == nil || anonymousPublicGitClient.Jar != nil {
		t.Fatal("production client must directly use the anonymous safe dialer")
	}
	for _, address := range []string{"127.0.0.1:443", "10.0.0.1:443", "169.254.169.254:443", "[::1]:443"} {
		conn, err := transport.DialContext(context.Background(), "tcp", address)
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "blocked address") {
			t.Errorf("safe dial %s = %v,want rejected private address", address, err)
		}
	}
}

func TestPublicGitFullSHAUsesPeeledTagBeforeCustomRef(t *testing.T) {
	refs := map[string]string{
		"refs/archive/tag-object": testCommitB,
		"refs/tags/v1":            testCommitB,
		"refs/tags/v1^{}":         testCommitA,
	}
	for range 1000 {
		if got := publicGitAdvertisedCommit(refs, "", testCommitB); got != testCommitA {
			t.Fatalf("tag object resolved to %q, want peeled %q", got, testCommitA)
		}
	}
}

func TestPublicGitHTTPFailuresAreCachedMisses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
	}{
		{"unauthorized", http.StatusUnauthorized, publicGitAdvertisementType},
		{"missing", http.StatusNotFound, publicGitAdvertisementType},
		{"rate limited", http.StatusTooManyRequests, publicGitAdvertisementType},
		{"provider failed", http.StatusInternalServerError, publicGitAdvertisementType},
		{"HTML response", http.StatusOK, "text/html"},
		{"invalid media type", http.StatusOK, "application/;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				response := gitResponse(tc.status, gitAdvertisement(testCommitA+" refs/heads/main\x00\n"))
				response.Header.Set("Content-Type", tc.contentType)
				return response, nil
			})}}
			for range 2 {
				if info, ok, err := svc.resolvePublicGitCommit(context.Background(), "https://git.example/repo", "main"); err != nil || ok || info.Hash != "" {
					t.Fatalf("HTTP failure resolved %+v,%v,%v", info, ok, err)
				}
			}
			if calls != 1 {
				t.Fatalf("requests = %d, want cached failure", calls)
			}
		})
	}
}

// An advertisement past the bound still resolves its default branch from the
// first record's HEAD symref; any other ref there stays a miss.
func TestPublicGitOversizedAdvertisementResolvesDefaultBranch(t *testing.T) {
	filler := strings.Repeat(gitPacket(testCommitA+" refs/environments/e/deployments/1\n"), maxPublicGitAdvertisement/40)
	body := gitAdvertisement(testCommitA+" HEAD\x00symref=HEAD:refs/heads/master agent=git\n") + filler
	for ref, want := range map[string]string{"master": testCommitA, "refs/heads/master": testCommitA, "HEAD": testCommitA, "main": "", "v1": ""} {
		svc := &Service{publicGitHTTPClient: &http.Client{Transport: publicGitRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return gitResponse(http.StatusOK, body), nil
		})}}
		info, ok, err := svc.resolveCommit(context.Background(), "default", "https://gitlab.example/big/repo", ref)
		if err != nil || info.Hash != want || ok != (want != "") {
			t.Errorf("%s = %+v,%v,%v, want %q", ref, info, ok, err, want)
		}
	}
}
