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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

// declaredConfig is an image whose manifest declares a config of size bytes,
// as a hostile registry's would.
type declaredConfig struct {
	v1.Image
	size int64
}

func (d declaredConfig) Manifest() (*v1.Manifest, error) {
	m, err := d.Image.Manifest()
	if err != nil {
		return nil, err
	}
	m = m.DeepCopy()
	m.Config.Size = d.size
	return m, nil
}

// TestConfigSizeIsCheckedBeforeTheConfigIsRead is round 24's S-2: bex-api read
// a source image's config whole, bounded only by the size its manifest
// declared, so a declared 400 MB (or -1 with a chunked body) took a 128 MiB
// replica down. The declaration is now checked first.
func TestConfigSizeIsCheckedBeforeTheConfigIsRead(t *testing.T) {
	img, err := random.Image(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckConfigSize(img); err != nil {
		t.Fatalf("an ordinary image's config is refused: %v", err)
	}
	for _, size := range []int64{-1, 0, MaxConfigBytes + 1, 400_000_000} {
		if err := CheckConfigSize(declaredConfig{img, size}); err == nil {
			t.Errorf("a manifest declaring a %d-byte config was accepted", size)
		}
	}
}

// TestManifestResponsesAreCapped: a source registry's manifest is read into
// memory, so its body is bounded too, whether or not it declares a length.
func TestManifestResponsesAreCapped(t *testing.T) {
	huge := strings.Repeat("x", MaxManifestBytes+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("length") {
		case "declared":
			w.Header().Set("Content-Length", strconv.Itoa(len(huge)))
		default:
			w.Header().Set("Transfer-Encoding", "chunked")
		}
		_, _ = io.WriteString(w, huge)
	}))
	defer srv.Close()
	client := &http.Client{Transport: CapManifests{Next: http.DefaultTransport}}

	for _, length := range []string{"declared", "chunked"} {
		resp, err := client.Get(fmt.Sprintf("%s/v2/acme/app/manifests/1?length=%s", srv.URL, length))
		if err == nil {
			_, err = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}
		if err == nil {
			t.Errorf("a %s %d-byte manifest was read whole", length, len(huge))
		}
	}
	// Blobs (layers) stream through the copy and are not capped here.
	resp, err := client.Get(srv.URL + "/v2/acme/app/blobs/sha256:abc?length=declared")
	if err != nil {
		t.Fatalf("blob request refused: %v", err)
	}
	n, err := io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err != nil || n != int64(len(huge)) {
		t.Fatalf("blob read %d bytes (%v), want all %d", n, err, len(huge))
	}
}

// The manifest cap applies to manifest reads only, including a redirected
// one, and never to a layer of a repository that happens to be named
// "manifests".
func TestManifestCapMatchesManifestReadsOnly(t *testing.T) {
	req := func(p string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "https://registry.example"+p, nil)
		return r
	}
	for p, want := range map[string]bool{
		"/v2/acme/app/manifests/1":            true,
		"/v2/acme/app/manifests/sha256:abc":   true,
		"/v2/acme/manifests/blobs/sha256:abc": false,
		"/v2/manifests/app/blobs/uploads/":    false,
		"/v2/acme/app/blobs/sha256:abc":       false,
	} {
		if got := isManifestRequest(req(p)); got != want {
			t.Errorf("isManifestRequest(%s) = %v, want %v", p, got, want)
		}
	}
	hop := req("/cdn/opaque-object")
	hop.Response = &http.Response{Request: req("/v2/acme/app/manifests/1")}
	if !isManifestRequest(hop) {
		t.Error("a redirected manifest read escaped the cap")
	}
}
