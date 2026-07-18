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

package registry

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testDigest = "sha256:341e43e9d1e4a1d8ad2b8a29bd28e6be3e0d33a9a26e6bd1e1f7a5b2c3d4e5f6"

// TestResolveDigest pins the wire contract (w9/013): HEAD the manifest with
// the OCI/Docker Accept set, read Docker-Content-Digest, pass basic auth only
// when a username is given, and error on non-2xx or a malformed digest.
func TestResolveDigest(t *testing.T) {
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/v2/tea-x-web/manifests/gen-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Docker-Content-Digest", testDigest)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	digest, err := ResolveDigest(context.Background(), nil, host, "tea-x-web", "gen-1", "app-tea-x-web", "pw")
	if err != nil || digest != testDigest {
		t.Fatalf("ResolveDigest = %q, %v; want %q", digest, err, testDigest)
	}
	if gotAuth == "" {
		t.Error("expected basic auth to be sent when a username is given")
	}
	if !strings.Contains(gotAccept, "application/vnd.oci.image.manifest.v1+json") {
		t.Errorf("Accept = %q, want the OCI manifest media type", gotAccept)
	}

	// Anonymous: no username -> no Authorization header.
	gotAuth = "unset"
	if _, err := ResolveDigest(context.Background(), nil, host, "tea-x-web", "gen-1", "", ""); err != nil {
		t.Fatalf("anonymous resolve: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("anonymous resolve sent Authorization %q, want none", gotAuth)
	}

	// Unknown repo/tag -> error, never an empty digest.
	if _, err := ResolveDigest(context.Background(), nil, host, "absent", "gen-1", "", ""); err == nil {
		t.Error("want error for a 404 manifest")
	}
}

func TestResolveDigestRejectsMalformedDigest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Docker-Content-Digest", "not-a-digest")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if _, err := ResolveDigest(context.Background(), nil, strings.TrimPrefix(srv.URL, "http://"), "r", "t", "", ""); err == nil {
		t.Error("want error for a malformed Docker-Content-Digest")
	}
}

func TestBasicAuthFromDockerConfig(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("bex-puller:s3cret"))
	cfg := []byte(`{"auths":{"zot.bex-registry.svc:5000":{"auth":"` + auth + `"}}}`)
	u, p, ok := BasicAuthFromDockerConfig(cfg, "zot.bex-registry.svc:5000")
	if !ok || u != "bex-puller" || p != "s3cret" {
		t.Fatalf("got (%q, %q, %v), want (bex-puller, s3cret, true)", u, p, ok)
	}
	if _, _, ok := BasicAuthFromDockerConfig(cfg, "other-host:5000"); ok {
		t.Error("want ok=false for a host absent from the config")
	}
	if _, _, ok := BasicAuthFromDockerConfig([]byte("not json"), "h"); ok {
		t.Error("want ok=false for unparseable config")
	}
	// username/password fields without the packed auth blob.
	cfg2 := []byte(`{"auths":{"h":{"username":"u","password":"p"}}}`)
	if u, p, ok := BasicAuthFromDockerConfig(cfg2, "h"); !ok || u != "u" || p != "p" {
		t.Errorf("field-form auth = (%q, %q, %v), want (u, p, true)", u, p, ok)
	}
}
