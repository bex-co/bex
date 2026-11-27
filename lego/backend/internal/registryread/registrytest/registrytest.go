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

// Package registrytest serves an in-process image registry under a public
// host name for tests of the image check (w1/m673). Repositories under
// "private/" answer only a token its /token endpoint issues for Username and
// Password, as Docker Hub, GHCR and GitLab do.
package registrytest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// The registry's host and its one account.
const (
	Host     = "registry.test"
	Username = "robot"
	Password = "fixture-registry-token"
	token    = "fixture-bearer-token"
)

// Registry is the running registry.
type Registry struct {
	t    testing.TB
	push *httptest.Server // the registry with no auth, for fixtures
	srv  *httptest.Server // what Host reaches
}

// New starts a registry for the test's lifetime.
func New(t testing.TB) *Registry {
	t.Helper()
	reg := registry.New()
	r := &Registry{t: t, push: httptest.NewServer(reg)}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/token" {
			u, p, ok := req.BasicAuth()
			switch {
			case !ok:
				_ = json.NewEncoder(w).Encode(map[string]string{"token": "anonymous"})
			case u == Username && p == Password:
				_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
			default:
				http.Error(w, `{"details":"incorrect username or password"}`, http.StatusUnauthorized)
			}
			return
		}
		// The /v2/ ping challenges every caller, as Docker Hub's does, so a
		// client starts the token flow before it reads a private repository.
		auth := req.Header.Get("Authorization")
		ping := req.URL.Path == "/v2/" && !strings.HasPrefix(auth, "Bearer ")
		private := strings.HasPrefix(req.URL.Path, "/v2/private/") && auth != "Bearer "+token
		if ping || private {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="https://%s/token",service=%q`, Host, Host))
			http.Error(w, `{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`, http.StatusUnauthorized)
			return
		}
		reg.ServeHTTP(w, req)
	}))
	t.Cleanup(func() { r.push.Close(); r.srv.Close() })
	return r
}

// Transport reaches Host in process and refuses every other host.
func (r *Registry) Transport() http.RoundTripper { return routes{r.srv} }

type routes struct{ srv *httptest.Server }

func (rt routes) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != Host {
		return nil, fmt.Errorf("registrytest: no route to %s", req.URL.Host)
	}
	target, _ := url.Parse(rt.srv.URL)
	out := req.Clone(req.Context())
	out.URL.Scheme, out.URL.Host = target.Scheme, target.Host
	out.Host = Host
	return http.DefaultTransport.RoundTrip(out)
}

// Ref is repoTag's reference on Host.
func Ref(repoTag string) string { return Host + "/" + repoTag }

// Push writes a single-platform image to repoTag.
func (r *Registry) Push(repoTag string, platform v1.Platform) {
	r.t.Helper()
	img, err := random.Image(256, 1)
	if err != nil {
		r.t.Fatal(err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		r.t.Fatal(err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = platform.OS, platform.Architecture
	if img, err = mutate.ConfigFile(img, cfg); err != nil {
		r.t.Fatal(err)
	}
	ref, err := name.ParseReference(strings.TrimPrefix(r.push.URL, "http://") + "/" + repoTag)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		r.t.Fatal(err)
	}
}
