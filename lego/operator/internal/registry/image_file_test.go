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
	"archive/tar"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

const (
	proxyStep = "COPY /workspace/kv-internal-proxy /kv-internal-proxy # buildkit"
	proxyFile = "kv-internal-proxy"

	mediaOCIIndex     = "application/vnd.oci.image.index.v1+json"
	mediaOCIManifest  = "application/vnd.oci.image.manifest.v1+json"
	mediaOCILayerGzip = "application/vnd.oci.image.layer.v1.tar+gzip"
)

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// gzipTar is a layer holding the given files.
func gzipTar(files map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		must(tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write(body)
		must(err)
	}
	must(tw.Close())
	must(zw.Close())
	return buf.Bytes()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// testImage is a multi-platform image as deploy.yml pushes it: an OCI index
// with this architecture's image, a decoy for another one and a provenance
// attestation, behind GHCR's anonymous Bearer challenge.
type testImage struct {
	blobs map[string][]byte // digest → content, manifests included
	types map[string]string // manifest digest → media type
	index string            // the reference's digest
	layer string            // the proxy layer's digest
	repo  string
	token string
	fail  bool // every registry request past the token fails
}

func (img *testImage) add(mediaType string, v any) string {
	raw, err := json.Marshal(v)
	must(err)
	d := digestOf(raw)
	img.blobs[d] = raw
	img.types[d] = mediaType
	return d
}

func newTestImage(t *testing.T, proxy []byte, history []map[string]any) *testImage {
	t.Helper()
	img := &testImage{blobs: map[string][]byte{}, types: map[string]string{}, repo: "bex-co/bex-operator", token: "anon-token"}
	base := gzipTar(map[string][]byte{"manager": []byte("manager binary")})
	layer := gzipTar(map[string][]byte{proxyFile: proxy})
	img.blobs[digestOf(base)] = base
	img.layer = digestOf(layer)
	img.blobs[img.layer] = layer
	if history == nil {
		history = []map[string]any{
			{"created_by": "WORKDIR /", "empty_layer": true},
			{"created_by": "COPY /workspace/manager . # buildkit"},
			{"created_by": proxyStep},
			{"created_by": "USER 65532:65532", "empty_layer": true},
		}
	}
	// The other architecture's image carries another binary, so reading the
	// wrong platform answers the wrong hash.
	decoy := gzipTar(map[string][]byte{proxyFile: []byte("another architecture's binary")})
	img.blobs[digestOf(decoy)] = decoy
	image := func(arch string, layer []byte) string {
		config := img.add("", map[string]any{"os": "linux", "architecture": arch, "history": history})
		return img.add(mediaOCIManifest, map[string]any{
			"schemaVersion": 2, "mediaType": mediaOCIManifest,
			"config": map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": config, "size": len(img.blobs[config])},
			"layers": []map[string]any{
				{"mediaType": mediaOCILayerGzip, "digest": digestOf(base), "size": len(base)},
				{"mediaType": mediaOCILayerGzip, "digest": digestOf(layer), "size": len(layer)},
			},
		})
	}
	other := "arm64"
	if runtime.GOARCH == "arm64" {
		other = "amd64"
	}
	img.index = img.add(mediaOCIIndex, map[string]any{
		"schemaVersion": 2, "mediaType": mediaOCIIndex,
		"manifests": []map[string]any{
			{"mediaType": mediaOCIManifest, "digest": image(other, decoy), "platform": map[string]string{"os": "linux", "architecture": other}},
			{"mediaType": mediaOCIManifest, "digest": image(runtime.GOARCH, layer), "platform": map[string]string{"os": "linux", "architecture": runtime.GOARCH}},
			{"mediaType": mediaOCIManifest, "digest": img.add(mediaOCIManifest, map[string]any{"schemaVersion": 2}),
				"platform": map[string]string{"os": "unknown", "architecture": "unknown"}},
		},
	})
	return img
}

func (img *testImage) serve(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if r.URL.Query().Get("scope") != "repository:"+img.repo+":pull" {
				http.Error(w, "bad scope", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"token": img.token})
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+img.token {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+srv.URL+`/token",service="registry.test",scope="repository:`+img.repo+`:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if img.fail {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		rest, ok := strings.CutPrefix(r.URL.Path, "/v2/"+img.repo+"/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		kind, digest, _ := strings.Cut(rest, "/")
		body, found := img.blobs[digest]
		if !found {
			http.NotFound(w, r)
			return
		}
		if kind == "manifests" {
			w.Header().Set("Content-Type", img.types[digest])
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (img *testImage) fileSHA256(srv *httptest.Server, digest string) (string, error) {
	return ImageFileSHA256(context.Background(), srv.Client(), strings.TrimPrefix(srv.URL, "http://"), img.repo, digest, proxyStep, proxyFile)
}

// w6/m469: the operator compares the /kv-internal-proxy in a proxy's running
// image with its own, so the read must find exactly this architecture's file
// behind GHCR's anonymous token, past the other platform and the attestation.
func TestImageFileSHA256ReadsTheFileOutOfItsLayer(t *testing.T) {
	proxy := []byte("kv-internal-proxy for " + runtime.GOARCH)
	img := newTestImage(t, proxy, nil)
	got, err := img.fileSHA256(img.serve(t), img.index)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimPrefix(digestOf(proxy), "sha256:"); got != want {
		t.Fatalf("hash %s, want %s", got, want)
	}
}

// Every doubt is an error, so the operator rolls the proxy as it always did.
func TestImageFileSHA256RefusesWhatItCannotProve(t *testing.T) {
	proxy := bytes.Repeat([]byte("A"), 4096)
	for name, c := range map[string]struct {
		history []map[string]any
		mutate  func(t *testing.T, img *testImage)
		digest  string
		want    string
	}{
		"a tag reference": {
			digest: "latest",
			want:   "not a digest",
		},
		"no proxy build step": {
			history: []map[string]any{{"created_by": "COPY /workspace/manager . # buildkit"}, {"created_by": "COPY /workspace/other /other # buildkit"}},
			want:    "no build step",
		},
		"the build step twice": {
			history: []map[string]any{{"created_by": proxyStep}, {"created_by": proxyStep}},
			want:    "appears twice",
		},
		"history and layers disagree": {
			history: []map[string]any{{"created_by": proxyStep}},
			want:    "its manifest lists 2",
		},
		"an oversized layer": {
			mutate: func(t *testing.T, img *testImage) { resize(t, img, maxImageFileLayerBytes+1) },
			want:   "at most",
		},
		"a registry error": {
			mutate: func(_ *testing.T, img *testImage) { img.fail = true },
			want:   "status 503",
		},
		"a layer that is not the one the manifest names": {
			mutate: func(t *testing.T, img *testImage) {
				other := gzipTar(map[string][]byte{proxyFile: bytes.Repeat([]byte("B"), len(proxy))})
				if len(other) != len(img.blobs[img.layer]) {
					t.Fatalf("the substitute layer must keep the declared size: %d != %d", len(other), len(img.blobs[img.layer]))
				}
				img.blobs[img.layer] = other
			},
			want: "digest mismatch",
		},
	} {
		t.Run(name, func(t *testing.T) {
			img := newTestImage(t, proxy, c.history)
			if c.mutate != nil {
				c.mutate(t, img)
			}
			got, err := img.fileSHA256(img.serve(t), cmp.Or(c.digest, img.index))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("ImageFileSHA256 = %q, %v; want an error containing %q", got, err, c.want)
			}
		})
	}
}

// resize rewrites this architecture's manifest to declare the proxy layer at
// size bytes, and the index to point at the rewritten manifest.
func resize(t *testing.T, img *testImage, size int64) {
	t.Helper()
	var index map[string]any
	if err := json.Unmarshal(img.blobs[img.index], &index); err != nil {
		t.Fatal(err)
	}
	for _, m := range index["manifests"].([]any) {
		entry := m.(map[string]any)
		if entry["platform"].(map[string]any)["architecture"] != runtime.GOARCH {
			continue
		}
		var manifest map[string]any
		if err := json.Unmarshal(img.blobs[entry["digest"].(string)], &manifest); err != nil {
			t.Fatal(err)
		}
		layers := manifest["layers"].([]any)
		layers[len(layers)-1].(map[string]any)["size"] = size
		entry["digest"] = img.add(mediaOCIManifest, manifest)
	}
	img.index = img.add(mediaOCIIndex, index)
}
