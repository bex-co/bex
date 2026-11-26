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
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// w4/m671: the availability check of a running service's image is one HEAD
// on its manifest, after the same anonymous token exchange a metadata read
// makes, and never a GET, which Docker Hub counts as a pull.
func TestCheckImageManifestIsOneHeadAfterTheTokenExchange(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"token":"anon-token"}`))
		case r.Header.Get("Authorization") != "Bearer anon-token":
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+srv.URL+`/token",service="registry.test",scope="repository:library/nginx:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method != http.MethodHead:
			t.Errorf("%s %s: the check downloaded a manifest", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
		case r.Header.Get("Accept") != manifestAccept:
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		case r.URL.Path == "/v2/library/nginx/manifests/1.27":
			w.Header().Set("Docker-Content-Digest", "sha256:abc")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	if err := CheckImageManifest(t.Context(), srv.Client(), srv.URL, "library/nginx", "1.27", ""); err != nil {
		t.Fatalf("present image: %v", err)
	}
	want := []string{"HEAD /v2/library/nginx/manifests/1.27", "GET /token", "HEAD /v2/library/nginx/manifests/1.27"}
	if len(seen) != len(want) {
		t.Fatalf("requests = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("requests = %v, want %v", seen, want)
		}
	}

	err := CheckImageManifest(t.Context(), srv.Client(), srv.URL, "library/nginx", "gone", "")
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Status != http.StatusNotFound {
		t.Fatalf("deleted tag: %v, want a 404 refusal", err)
	}
}

// Each registry answer comes back in the form the check's rule reads: a
// refusal for 401, 403 and 404, transient for 429 and 5xx, and the
// credential travels as given.
func TestCheckImageManifestClassifiesTheRegistryAnswer(t *testing.T) {
	for _, tc := range []struct {
		status    int
		refused   bool
		transient bool
	}{
		{http.StatusOK, false, false},
		{http.StatusNotFound, true, false},
		{http.StatusForbidden, true, false},
		{http.StatusTooManyRequests, false, true},
		{http.StatusBadGateway, false, true},
		{http.StatusServiceUnavailable, false, true},
	} {
		client := imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Basic fixture" || r.Method != http.MethodHead {
				w.WriteHeader(http.StatusTeapot)
				return
			}
			w.WriteHeader(tc.status)
		}))
		err := CheckImageManifest(t.Context(), client, "https://registry.test", "owned/app", "sha256:"+sha64, "Basic fixture")
		var refused *RefusedError
		var transient *TransientError
		if errors.As(err, &refused) != tc.refused || errors.As(err, &transient) != tc.transient || (tc.status == http.StatusOK) != (err == nil) {
			t.Fatalf("status %d: err = %v", tc.status, err)
		}
		if tc.refused && refused.Status != tc.status {
			t.Fatalf("status %d: refusal names %d", tc.status, refused.Status)
		}
	}
	// A credential the registry rejects at its token endpoint is refused too,
	// as a metadata read's is.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+srv.URL+`/token"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := CheckImageManifest(t.Context(), srv.Client(), srv.URL, "owned/app", "v1", "Basic expired")
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Status != http.StatusUnauthorized {
		t.Fatalf("expired credential: %v, want a 401 refusal", err)
	}
}

// An invalid reference is refused before any request.
func TestCheckImageManifestValidatesTheReference(t *testing.T) {
	client := imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request sent for an invalid reference: %s", r.URL)
	}))
	if err := CheckImageManifest(t.Context(), client, "https://registry.test", "Owned/App", "v1", ""); err == nil {
		t.Fatal("invalid repository accepted")
	}
}

const sha64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
