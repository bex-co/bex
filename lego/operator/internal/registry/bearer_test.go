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
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// w4/m243: an anonymous read of a public registry answers its Bearer
// challenge with a credential-less token fetch, then retries.
func TestReadImageConfigAnonymousBearer(t *testing.T) {
	config := `{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{"80/tcp":{}}}}`
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(config)))
	manifest := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"token":"anon-token"}`))
		case r.Header.Get("Authorization") != "Bearer anon-token":
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+srv.URL+`/token",service="registry.test",scope="repository:traefik/whoami:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/v2/traefik/whoami/blobs/"+configDigest:
			_, _ = w.Write([]byte(config))
		default:
			_, _ = w.Write([]byte(manifest))
		}
	}))
	defer srv.Close()

	cfg, err := ReadImageConfig(t.Context(), srv.Client(), srv.URL, "traefik/whoami", "v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TCPPorts) != 1 || cfg.TCPPorts[0] != 80 {
		t.Fatalf("ports = %v, want [80]", cfg.TCPPorts)
	}
}

// A credentialed read never downgrades to the anonymous token flow.
func TestReadImageConfigCredentialedDoesNotFetchToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			t.Error("token endpoint called for a credentialed read")
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+"https://example.test/token"+`"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	if _, err := ReadImageConfig(t.Context(), srv.Client(), srv.URL, "traefik/whoami", "v1", "Basic xyz"); err == nil {
		t.Fatal("credentialed 401 accepted")
	}
}
