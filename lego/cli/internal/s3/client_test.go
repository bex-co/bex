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

package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRequestsAreSignedWithTheCachedKeyAndSentToTheEndpointOnly: every request
// is path style, signed with the key bex made, and goes to the endpoint, with a
// person's own AWS setup (a profile, a credentials file, an endpoint, a region,
// a FIPS or dual-stack switch) changing none of it.
func TestRequestsAreSignedWithTheCachedKeyAndSentToTheEndpointOnly(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755); err != nil {
		t.Fatal(err)
	}
	poisoned := "[default]\nregion = eu-west-1\nendpoint_url = https://elsewhere.invalid\naws_access_key_id = AKIATHEIRS\naws_secret_access_key = their-secret\ns3 =\n  addressing_style = virtual\n  use_accelerate_endpoint = true\n"
	for _, name := range []string{"config", "credentials"} {
		if err := os.WriteFile(filepath.Join(home, ".aws", name), []byte(poisoned), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for name, value := range map[string]string{
		"AWS_PROFILE": "work", "AWS_DEFAULT_PROFILE": "work", "AWS_ACCESS_KEY_ID": "AKIATHEIRS", "AWS_SECRET_ACCESS_KEY": "their-secret",
		"AWS_SESSION_TOKEN": "their-token", "AWS_REGION": "eu-west-1", "AWS_DEFAULT_REGION": "eu-west-1",
		"AWS_ENDPOINT_URL": "https://elsewhere.invalid", "AWS_ENDPOINT_URL_S3": "https://elsewhere.invalid",
		"AWS_CONFIG_FILE": filepath.Join(home, ".aws", "config"), "AWS_SHARED_CREDENTIALS_FILE": filepath.Join(home, ".aws", "credentials"),
		"AWS_USE_FIPS_ENDPOINT": "true", "AWS_USE_DUALSTACK_ENDPOINT": "true", "AWS_S3_USE_ARN_REGION": "true",
		"AWS_REQUEST_CHECKSUM_CALCULATION": "when_required", "AWS_CA_BUNDLE": filepath.Join(home, "no-such-ca.pem"),
		"AWS_EC2_METADATA_SERVICE_ENDPOINT": "http://elsewhere.invalid", "AWS_ROLE_ARN": "arn:aws:iam::1:role/x", "AWS_WEB_IDENTITY_TOKEN_FILE": "/no/such/token",
	} {
		t.Setenv(name, value)
	}

	g := newRig(t)
	g.s3.put("photos", "site/a.txt", []byte("hello"), testNow)
	g.s3.accessKey = testKeyID
	g.must("ls", "s3://photos/site/")
	g.must("cp", "s3://photos/site/a.txt", filepath.Join(t.TempDir(), "a.txt"))

	log := g.s3.requests("")
	if len(log) == 0 {
		t.Fatal("no request reached the endpoint")
	}
	for _, line := range log {
		if _, path, _ := strings.Cut(line, " "); !strings.HasPrefix(path, "/photos") {
			t.Errorf("request %q is not path style (bucket first in the path)", line)
		}
	}
	for _, auth := range g.s3.auth {
		if !strings.Contains(auth, "Credential="+testKeyID+"/") || !strings.Contains(auth, "/oregon/s3/aws4_request") {
			t.Errorf("a request was signed %q, want the key bex made and the oregon region", auth)
		}
		if strings.Contains(auth, "AKIATHEIRS") {
			t.Errorf("a request was signed with the person's own key: %q", auth)
		}
	}
	for _, agent := range g.s3.agents {
		if !strings.Contains(agent, "app/bex-cli") {
			t.Errorf("a request carried the User-Agent %q, want bex-cli named in it", agent)
		}
	}
	if g.api.mintCount() != 1 {
		t.Errorf("mints = %d, want one key for both commands", g.api.mintCount())
	}
}

// TestAnExpiredKeyNamesThePersistentKeyAndIsForgotten: a 403 for a key the
// server no longer knows says so, says what a transfer over hours needs, and
// drops the cached key so the next command makes a new one.
func TestAnExpiredKeyNamesThePersistentKeyAndIsForgotten(t *testing.T) {
	g := newRig(t)
	g.s3.bucket("photos")
	g.s3.accessKey = "s3k-someone-else"
	msg := g.fails("ls", "s3://photos")
	for _, want := range []string{"expired or been revoked", "bex s3 keys create", "BEX_S3_ACCESS_KEY_ID", "InvalidAccessKeyId"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not say %q", msg, want)
		}
	}
	if files, _ := filepath.Glob(filepath.Join(g.state, "s3", "key-*.json")); len(files) != 0 {
		t.Errorf("the refused key is still cached: %v", files)
	}
	g.s3.accessKey = testKeyID
	g.must("ls", "s3://photos")
	if g.api.mintCount() != 2 {
		t.Errorf("mints = %d, want a new key after the refusal", g.api.mintCount())
	}

	// A HEAD answer has no body to carry the code, so the 403 alone is read the same way.
	g.s3.put("photos", "k", []byte("x"), testNow)
	g.s3.accessKey = "s3k-someone-else"
	if msg := g.fails("info", "s3://photos/k"); !strings.Contains(msg, "bex s3 keys create") {
		t.Errorf("a refused HEAD says %q, want the persistent-key hint", msg)
	}
}

// TestPersistentKeyFromTheEnvironmentIsUsedAndNothingIsMinted is how CI, with
// no sign-in and no workspace, uses `bex s3`.
func TestPersistentKeyFromTheEnvironmentIsUsedAndNothingIsMinted(t *testing.T) {
	g := newRig(t)
	g.env[accessKeyEnv], g.env[secretKeyEnv] = "s3k-persistentkey0000000", "its-secret"
	g.d.workspace = func() (string, error) { return "", errors.New("no active workspace") }
	g.d.api = func() (*api, error) { return nil, errors.New("not signed in") }
	g.s3.bucket("photos")
	g.s3.accessKey = "s3k-persistentkey0000000"
	if out := g.must("ls"); !strings.Contains(out, "photos") {
		t.Errorf("ls = %q", out)
	}
	if g.api.mintCount() != 0 || len(g.api.requests()) != 0 {
		t.Errorf("bex-api was called %d times with a key in the environment", len(g.api.requests()))
	}
	if files, _ := filepath.Glob(filepath.Join(g.state, "s3", "*")); len(files) != 0 {
		t.Errorf("a persistent key left files behind: %v", files)
	}
	out := g.must("env")
	if !strings.Contains(out, "does not expire") || !strings.Contains(out, "its-secret") {
		t.Errorf("env printed %q", out)
	}

	// One of the two is a mistake worth saying.
	delete(g.env, secretKeyEnv)
	if msg := g.fails("ls"); !strings.Contains(msg, secretKeyEnv) {
		t.Errorf("err = %q, want it to name %s", msg, secretKeyEnv)
	}
}

// TestRegionFlagSignsForThatRegion: --region is the signing region, and what
// mb creates the bucket in.
func TestRegionFlagSignsForThatRegion(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos-eu", "--region", "frankfurt")
	if b := g.s3.bucket("photos-eu"); b.location != "frankfurt" {
		t.Errorf("location = %q, want frankfurt", b.location)
	}
	if !strings.Contains(g.s3.auth[0], "/frankfurt/s3/aws4_request") {
		t.Errorf("signed %q, want the frankfurt region", g.s3.auth[0])
	}
}

// TestAtMostConcurrencyRequestsAreInFlight: the transport holds every request,
// whoever makes it, to the bound, and a slot is freed when a body ends or closes.
func TestAtMostConcurrencyRequestsAreInFlight(t *testing.T) {
	var inflight, peak atomic.Int32
	release := make(chan struct{})
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cur := inflight.Add(1)
		for p := peak.Load(); cur > p && !peak.CompareAndSwap(p, cur); p = peak.Load() {
		}
		<-release
		inflight.Add(-1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})
	client := newLimitedClient(base, 3)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.test/", nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			_ = resp.Body.Close()
		}()
	}
	time.Sleep(50 * time.Millisecond)
	if got := inflight.Load(); got != 3 {
		t.Errorf("%d requests in flight with a bound of 3", got)
	}
	close(release)
	wg.Wait()
	if peak.Load() > 3 {
		t.Errorf("peak %d requests in flight with a bound of 3", peak.Load())
	}

	// A request that cannot start because the bound is reached gives up when its context ends.
	full := newLimitedClient(base, 1)
	full.slots <- struct{}{}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test/", nil)
	if _, err := full.Do(req); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the context's", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestPartSizeGrowsToStayUnderTenThousandParts(t *testing.T) {
	const mib = 1 << 20
	for _, tc := range []struct{ size, want int64 }{
		{-1, 8 * mib}, {0, 8 * mib}, {1, 8 * mib}, {20 * mib, 8 * mib}, {5 << 30, 8 * mib},
		{8 * mib * 10000, 8 * mib}, {8*mib*10000 + 1, 9 * mib}, {100 << 30, 11 * mib},
	} {
		got := partSizeFor(tc.size)
		if got != tc.want {
			t.Errorf("partSizeFor(%d) = %d, want %d", tc.size, got, tc.want)
		}
		if tc.size > 0 && (tc.size+got-1)/got > maxParts {
			t.Errorf("an object of %d bytes in parts of %d needs more than %d parts", tc.size, got, maxParts)
		}
	}
}

func TestErrorsAreMappedToBexsWords(t *testing.T) {
	g := newRig(t)
	g.s3.bucket("photos")
	for _, tc := range []struct {
		status        int
		code, message string
		want          []string
	}{
		{403, "QuotaExceeded", "this workspace is at its 100 GB cap", []string{"QuotaExceeded", "100 GB cap"}},
		{501, "NotImplemented", "versioning is not implemented", []string{"does not serve this operation", "versioning"}},
		{403, "SignatureDoesNotMatch", "bad signature", []string{"signature was refused", "BEX_S3_SECRET_ACCESS_KEY"}},
		{403, "RequestTimeTooSkewed", "skew", []string{"clock"}},
		{404, "NoSuchBucket", "The specified bucket does not exist", []string{"NoSuchBucket"}},
	} {
		g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) {
			return tc.status, tc.code, tc.message, true
		}
		msg := g.fails("ls", "s3://photos")
		for _, want := range tc.want {
			if !strings.Contains(msg, want) {
				t.Errorf("%s: error %q does not say %q", tc.code, msg, want)
			}
		}
		if !strings.Contains(msg, "REQ0001") {
			t.Errorf("%s: error %q does not carry the request id", tc.code, msg)
		}
	}
}

// TestSlowDownIsRetriedBeforeItIsReported: the SDK retries a 503 SlowDown, so a
// command that meets one for a moment succeeds.
func TestSlowDownIsRetriedBeforeItIsReported(t *testing.T) {
	g := newRig(t)
	g.s3.bucket("photos")
	g.s3.put("photos", "k", []byte("x"), testNow)
	g.s3.intercept = func(r *http.Request, n int) (int, string, string, bool) {
		return http.StatusServiceUnavailable, "SlowDown", "Please reduce your request rate.", r.URL.Query().Get("list-type") == "2" && g.s3.count("list-type=2") <= 1
	}
	if out := g.must("ls", "s3://photos"); !strings.Contains(out, " k") {
		t.Errorf("ls = %q after a SlowDown", out)
	}
	if got := g.s3.count("list-type=2"); got < 2 {
		t.Errorf("%d list requests, want the SlowDown retried", got)
	}
}

// TestAnUnreachableEndpointIsNamedNotTheSDKsAttempts: the SDK's account of five
// failed attempts says nothing a person can act on; where the request was going
// does, and a query string (a signature) never reaches the message.
func TestAnUnreachableEndpointIsNamedNotTheSDKsAttempts(t *testing.T) {
	err := explain(fmt.Errorf("operation error S3: ListBuckets, exceeded maximum number of attempts, 5: %w",
		&url.Error{Op: "Get", URL: "https://objects.example.test/?X-Amz-Signature=secret&x-id=ListBuckets", Err: errors.New("dial tcp 192.0.2.1:443: connect: connection refused")}))
	msg := err.Error()
	if !strings.HasPrefix(msg, "could not reach objects.example.test: dial tcp") || strings.Contains(msg, "secret") || strings.Contains(msg, "attempts") {
		t.Errorf("message = %q", msg)
	}
}
