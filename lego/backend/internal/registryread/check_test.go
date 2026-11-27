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

package registryread

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	registryHost = "registry.test"
	cdnHost      = "cdn.test"
	user         = "robot"
	password     = "s3cret-token"
	goodToken    = "token-for-robot"
)

var linuxARM64 = v1.Platform{OS: "linux", Architecture: "arm64"}

// fakeRegistry is go-containerregistry's in-process registry behind
// registry.test. A private one answers only a bearer token its own /token
// endpoint issues for robot's password, as Docker Hub, GHCR and GitLab do.
type fakeRegistry struct {
	t       *testing.T
	reg     http.Handler
	push    *httptest.Server // the registry with no auth, for fixtures
	srv     *httptest.Server // what registry.test reaches
	private bool
	// realm overrides the token endpoint named in the challenge.
	realm string
	// override answers registry API requests in the registry's place when it
	// returns true.
	override func(w http.ResponseWriter, r *http.Request) bool

	mu   sync.Mutex
	seen []*http.Request // every request registry.test and cdn.test received
}

func newFakeRegistry(t *testing.T, private bool) *fakeRegistry {
	t.Helper()
	f := &fakeRegistry{t: t, reg: registry.New(), private: private}
	f.push = httptest.NewServer(f.reg)
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(func() { f.push.Close(); f.srv.Close() })
	return f
}

func (f *fakeRegistry) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen = append(f.seen, r.Clone(context.Background()))
}

func (f *fakeRegistry) serve(w http.ResponseWriter, r *http.Request) {
	f.record(r)
	if r.Host == cdnHost { // a CDN serves the blobs it is sent to, with no auth of its own
		f.reg.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/token" {
		u, p, ok := r.BasicAuth()
		switch {
		case !ok:
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "anonymous"})
		case u == user && p == password:
			_ = json.NewEncoder(w).Encode(map[string]string{"token": goodToken})
		default:
			http.Error(w, `{"details":"incorrect username or password"}`, http.StatusUnauthorized)
		}
		return
	}
	if f.private && r.Header.Get("Authorization") != "Bearer "+goodToken {
		realm := f.realm
		if realm == "" {
			realm = "https://" + registryHost + "/token"
		}
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q`, realm, registryHost))
		http.Error(w, `{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`, http.StatusUnauthorized)
		return
	}
	if f.override != nil && f.override(w, r) {
		return
	}
	f.reg.ServeHTTP(w, r)
}

// requests lists what host received, in order.
func (f *fakeRegistry) requests(host string) []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*http.Request
	for _, r := range f.seen {
		if r.Host == host {
			out = append(out, r)
		}
	}
	return out
}

// routes reaches registry.test and cdn.test in process, over plain HTTP, and
// sends every other host through the dial guard.
type routes map[string]*httptest.Server

func (rt routes) RoundTrip(r *http.Request) (*http.Response, error) {
	srv, ok := rt[r.URL.Host]
	if !ok {
		return GuardedTransport.RoundTrip(r)
	}
	target, _ := url.Parse(srv.URL)
	out := r.Clone(r.Context())
	out.URL.Scheme, out.URL.Host = target.Scheme, target.Host
	out.Host = r.URL.Host
	return http.DefaultTransport.RoundTrip(out)
}

func (f *fakeRegistry) checker() *Checker {
	return &Checker{Transport: routes{registryHost: f.srv, cdnHost: f.srv}}
}

func (f *fakeRegistry) pushRef(repo string) string {
	return strings.TrimPrefix(f.push.URL, "http://") + "/" + repo
}

func imageFor(t *testing.T, p v1.Platform) v1.Image {
	t.Helper()
	img, err := random.Image(256, 2)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = p.OS, p.Architecture
	if img, err = mutate.ConfigFile(img, cfg); err != nil {
		t.Fatal(err)
	}
	return img
}

// pushImage writes a single-platform image to repo:tag.
func (f *fakeRegistry) pushImage(repoTag string, p v1.Platform) v1.Image {
	f.t.Helper()
	img := imageFor(f.t, p)
	ref, err := name.ParseReference(f.pushRef(repoTag))
	if err != nil {
		f.t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		f.t.Fatal(err)
	}
	return img
}

// pushIndex writes a multi-platform index with one image per platform.
func (f *fakeRegistry) pushIndex(repoTag string, platforms ...v1.Platform) {
	f.t.Helper()
	var idx v1.ImageIndex = empty.Index
	for _, p := range platforms {
		p := p
		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{
			Add: imageFor(f.t, p), Descriptor: v1.Descriptor{Platform: &p},
		})
	}
	ref, err := name.ParseReference(f.pushRef(repoTag))
	if err != nil {
		f.t.Fatal(err)
	}
	if err := remote.WriteIndex(ref, idx); err != nil {
		f.t.Fatal(err)
	}
}

// pushSized writes repoTag as a copy of an amd64 image whose manifest declares
// layers totalling total bytes; no layer bytes exist, as none are read.
func (f *fakeRegistry) pushSized(repoTag string, total int64) {
	f.t.Helper()
	repo, tag, _ := strings.Cut(repoTag, ":")
	img := f.pushImage(repo+":base", DefaultPlatform)
	m, err := img.Manifest()
	if err != nil {
		f.t.Fatal(err)
	}
	m = m.DeepCopy()
	share := total / int64(len(m.Layers))
	for i := range m.Layers {
		m.Layers[i].Size = share
	}
	m.Layers[0].Size += total - share*int64(len(m.Layers))
	body, err := json.Marshal(m)
	if err != nil {
		f.t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, f.push.URL+"/v2/"+repo+"/manifests/"+tag, bytes.NewReader(body))
	req.Header.Set("Content-Type", string(m.MediaType))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		f.t.Fatalf("PUT sized manifest = %d", resp.StatusCode)
	}
}

func ref(repoTag string) string { return registryHost + "/" + repoTag }

func wantVerdict(t *testing.T, got Verdict, deployable bool, code string) {
	t.Helper()
	if got.Deployable != deployable || got.Code != code {
		t.Fatalf("verdict = %+v, want deployable=%v code=%q", got, deployable, code)
	}
	if !deployable && got.Reason == "" {
		t.Fatalf("an undeployable verdict carries no reason: %+v", got)
	}
}

func TestCheckPublicImages(t *testing.T) {
	f := newFakeRegistry(t, false)
	f.pushImage("acme/app:1", DefaultPlatform)
	f.pushImage("acme/arm:1", linuxARM64)
	f.pushIndex("acme/multi:1", DefaultPlatform, linuxARM64)
	f.pushIndex("acme/armonly:1", linuxARM64)
	c := f.checker()
	ctx := context.Background()

	wantVerdict(t, c.Check(ctx, ref("acme/app:1"), nil), true, "")
	wantVerdict(t, c.Check(ctx, ref("acme/app:2"), nil), false, FailureDoesNotExist)
	wantVerdict(t, c.Check(ctx, ref("acme/nothing:1"), nil), false, FailureDoesNotExist)
	wantVerdict(t, c.Check(ctx, ref("acme/arm:1"), nil), false, FailureUnsupportedPlatform)
	wantVerdict(t, c.Check(ctx, ref("acme/multi:1"), nil), true, "")
	wantVerdict(t, c.Check(ctx, ref("acme/armonly:1"), nil), false, FailureUnsupportedPlatform)
}

// The platform follows the hosting node pool (BEX_IMAGE_PLATFORM), as the
// operator's metadata read does.
func TestCheckFollowsTheHostingPlatform(t *testing.T) {
	f := newFakeRegistry(t, false)
	f.pushImage("acme/app:1", DefaultPlatform)
	f.pushImage("acme/arm:1", linuxARM64)
	f.pushIndex("acme/multi:1", DefaultPlatform, linuxARM64)
	c := f.checker()
	c.Platform = linuxARM64
	ctx := context.Background()

	wantVerdict(t, c.Check(ctx, ref("acme/arm:1"), nil), true, "")
	wantVerdict(t, c.Check(ctx, ref("acme/multi:1"), nil), true, "")
	wantVerdict(t, c.Check(ctx, ref("acme/app:1"), nil), false, FailureUnsupportedPlatform)
}

// An index entry that claims the hosting platform over a config for another
// one is refused, as the deploy's config check refuses it.
func TestCheckReadsTheConfigBehindAnIndexEntry(t *testing.T) {
	f := newFakeRegistry(t, false)
	lie := DefaultPlatform
	idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: imageFor(t, linuxARM64), Descriptor: v1.Descriptor{Platform: &lie},
	})
	r, _ := name.ParseReference(f.pushRef("acme/lie:1"))
	if err := remote.WriteIndex(r, idx); err != nil {
		t.Fatal(err)
	}
	wantVerdict(t, f.checker().Check(context.Background(), ref("acme/lie:1"), nil), false, FailureUnsupportedPlatform)
}

func TestCheckSizeLimit(t *testing.T) {
	f := newFakeRegistry(t, false)
	f.pushSized("acme/at:1", MaxImageBytes)
	f.pushSized("acme/over:1", MaxImageBytes+1)
	f.pushSized("acme/under:1", 9_900_000_000)
	c := f.checker()
	ctx := context.Background()

	wantVerdict(t, c.Check(ctx, ref("acme/under:1"), nil), true, "")
	wantVerdict(t, c.Check(ctx, ref("acme/at:1"), nil), true, "")
	got := c.Check(ctx, ref("acme/over:1"), nil)
	wantVerdict(t, got, false, FailureImageTooLarge)
	if !strings.Contains(got.Reason, "10000000001") {
		t.Errorf("reason %q does not name the image's size", got.Reason)
	}
}

func TestCheckPrivateImages(t *testing.T) {
	f := newFakeRegistry(t, true)
	f.pushImage("acme/private:1", DefaultPlatform)
	c := f.checker()
	ctx := context.Background()
	right := &Credential{Username: user, Password: password}
	wrong := &Credential{Username: user, Password: "not-it"}

	// Anonymously, a private image reads as missing, as on Render.
	wantVerdict(t, c.Check(ctx, ref("acme/private:1"), nil), false, FailureDoesNotExist)
	wantVerdict(t, c.Check(ctx, ref("acme/private:1"), right), true, "")
	// The token endpoint refuses the password: the credential is wrong.
	wantVerdict(t, c.Check(ctx, ref("acme/private:1"), wrong), false, FailureInvalidRegistryCredential)
	// The credential signs in, but the tag is not there.
	wantVerdict(t, c.Check(ctx, ref("acme/private:2"), right), false, FailureDoesNotExist)
}

// The credential signs in, and the registry still refuses this repository:
// Render's "The provided image URL and registry credential are invalid.".
func TestCheckCredentialThatCannotSeeTheImage(t *testing.T) {
	f := newFakeRegistry(t, true)
	f.pushImage("acme/private:1", DefaultPlatform)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/manifests/") {
			http.Error(w, `{"errors":[{"code":"DENIED","message":"requested access to the resource is denied"}]}`, http.StatusForbidden)
			return true
		}
		return false
	}
	wantVerdict(t, f.checker().Check(context.Background(), ref("acme/private:1"), &Credential{Username: user, Password: password}),
		false, FailureDoesNotExist)
}

// A registry that fails in a way none of Render's codes names answers no
// code, so the dashboard shows its generic sentence.
func TestCheckRegistryFailuresCarryNoCode(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway} {
		f := newFakeRegistry(t, false)
		f.pushImage("acme/app:1", DefaultPlatform)
		f.override = func(w http.ResponseWriter, r *http.Request) bool {
			if strings.Contains(r.URL.Path, "/manifests/") {
				http.Error(w, "busy", status)
				return true
			}
			return false
		}
		got := f.checker().Check(context.Background(), ref("acme/app:1"), nil)
		wantVerdict(t, got, false, "")
		if n := len(f.requests(registryHost)); n > 3 {
			t.Errorf("status %d: the check made %d requests; it must not retry", status, n)
		}
	}
}

func TestCheckTimesOut(t *testing.T) {
	f := newFakeRegistry(t, false)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		<-release
		return true
	}
	c := f.checker()
	c.Timeout = 100 * time.Millisecond
	start := time.Now()
	got := c.Check(context.Background(), ref("acme/app:1"), nil)
	wantVerdict(t, got, false, "")
	if !strings.Contains(got.Reason, "did not answer") {
		t.Errorf("reason = %q, want the registry did not answer", got.Reason)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("the check took %s past its 100ms timeout", elapsed)
	}
}

// The dial guard holds through a token realm and a redirect: neither reaches
// a loopback address, and the verdict is invalid_input_url.
func TestCheckGuardRefusesLoopbackRealmAndRedirect(t *testing.T) {
	loopback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the guard let a request through to loopback: %s", r.URL)
	}))
	t.Cleanup(loopback.Close)
	_, port, _ := strings.Cut(strings.TrimPrefix(loopback.URL, "http://"), ":")

	realm := newFakeRegistry(t, true)
	realm.realm = "https://localhost:" + port + "/token"
	realm.pushImage("acme/app:1", DefaultPlatform)
	wantVerdict(t, realm.checker().Check(context.Background(), ref("acme/app:1"), &Credential{Username: user, Password: password}),
		false, FailureInvalidInputURL)

	redirect := newFakeRegistry(t, false)
	redirect.pushImage("acme/app:1", DefaultPlatform)
	redirect.override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/manifests/") {
			http.Redirect(w, r, "http://localhost:"+port+"/stolen", http.StatusTemporaryRedirect)
			return true
		}
		return false
	}
	wantVerdict(t, redirect.checker().Check(context.Background(), ref("acme/app:1"), nil), false, FailureInvalidInputURL)
}

// The production transport refuses a registry host that resolves to a
// private address, before any request is sent.
func TestCheckGuardedTransportRefusesAPrivateRegistry(t *testing.T) {
	got := (&Checker{}).Check(context.Background(), "localhost:1/acme/app:1", nil)
	wantVerdict(t, got, false, FailureInvalidInputURL)
}

// A credential's password and its token go to the registry and its token
// endpoint only: a blob the registry redirects to another host is fetched
// without them.
func TestCheckSendsTheCredentialToItsRegistryOnly(t *testing.T) {
	f := newFakeRegistry(t, true)
	f.pushImage("acme/private:1", DefaultPlatform)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/blobs/") {
			http.Redirect(w, r, "https://"+cdnHost+r.URL.Path, http.StatusTemporaryRedirect)
			return true
		}
		return false
	}

	wantVerdict(t, f.checker().Check(context.Background(), ref("acme/private:1"), &Credential{Username: user, Password: password}), true, "")
	cdn := f.requests(cdnHost)
	if len(cdn) == 0 {
		t.Fatal("the config blob was never fetched from the redirected host")
	}
	for _, r := range cdn {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("%s received Authorization %q; the credential is the registry's alone", cdnHost, auth)
		}
	}
}

func TestCheckRefusesOversizedManifests(t *testing.T) {
	f := newFakeRegistry(t, false)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = io.WriteString(w, strings.Repeat(" ", MaxManifestBytes+1))
			return true
		}
		return false
	}
	wantVerdict(t, f.checker().Check(context.Background(), ref("acme/app:1"), nil), false, "")
}

func TestAcquireLimitsChecksPerWorkspace(t *testing.T) {
	c := &Checker{PerWorkspace: 2}
	a, ok := c.Acquire("tea-a")
	if !ok {
		t.Fatal("first slot refused")
	}
	if _, ok := c.Acquire("tea-a"); !ok {
		t.Fatal("second slot refused")
	}
	if _, ok := c.Acquire("tea-a"); ok {
		t.Fatal("a third concurrent check in one workspace was allowed")
	}
	if release, ok := c.Acquire("tea-b"); !ok {
		t.Fatal("another workspace was held by tea-a's checks")
	} else {
		release()
	}
	a()
	a() // a second release is a no-op
	if _, ok := c.Acquire("tea-a"); !ok {
		t.Fatal("a released slot was not returned")
	}
	if _, ok := c.Acquire("tea-a"); ok {
		t.Fatal("a double release returned two slots")
	}
}

func TestParsePlatformMatchesTheOperator(t *testing.T) {
	for value, want := range map[string]v1.Platform{"": DefaultPlatform, "linux/amd64": DefaultPlatform, "linux/arm64": linuxARM64} {
		if got, err := ParsePlatform(value); err != nil || got.OS != want.OS || got.Architecture != want.Architecture {
			t.Errorf("ParsePlatform(%q) = %+v, %v; want %+v", value, got, err, want)
		}
	}
	for _, value := range []string{"linux/arm", "windows/amd64", "amd64", "linux/arm64/v8"} {
		if _, err := ParsePlatform(value); err == nil {
			t.Errorf("ParsePlatform(%q) accepted a value the operator refuses", value)
		}
	}
}
