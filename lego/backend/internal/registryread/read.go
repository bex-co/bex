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

// Package registryread reads images from registries bex does not run: a
// sandbox image import's source (w2/m358) and an image a service is about to
// be created from (w1/m673). Such a registry is untrusted, so every read goes
// through the dial guard and holds manifests and configs to fixed sizes.
package registryread

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bex-co/bex/lego/types/netutil"
)

// GuardedTransport is the transport for an untrusted registry: every dial, a
// redirect's and a token realm's included, refuses a private destination, so
// a reference cannot probe the cluster or cloud metadata.
var GuardedTransport = func() http.RoundTripper {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = netutil.SafeDialContext(30 * time.Second)
	tr.Proxy = nil // an ambient proxy would dial in the guard's place
	return tr
}()

// A registry's manifest and an image's config are read into bex-api's memory
// (round 24, S-2: a config declared 400 MB, or -1 with a chunked body, took a
// 128 MiB bex-api replica down). A config is a few KiB; real manifests are
// under a few hundred KiB.
const (
	MaxManifestBytes = 4 << 20
	MaxConfigBytes   = 1 << 20
)

// CheckConfigSize refuses an image whose manifest declares its config empty,
// negative or larger than MaxConfigBytes, before the config is read: the read
// is bounded by the declared size, so the declaration is what to check.
func CheckConfigSize(img v1.Image) error {
	m, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("unreadable manifest: %w", err)
	}
	if size := m.Config.Size; size <= 0 || size > MaxConfigBytes {
		return fmt.Errorf("its manifest declares a %d-byte config; a config must be between 1 byte and %d bytes", size, MaxConfigBytes)
	}
	return nil
}

// CapManifests bounds every manifest response from a registry at
// MaxManifestBytes. Blobs are left alone: layers stream through a copy, and a
// config is bounded by CheckConfigSize.
type CapManifests struct{ Next http.RoundTripper }

func (t CapManifests) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.Next.RoundTrip(r)
	if err != nil || !isManifestRequest(r) {
		return resp, err
	}
	if resp.ContentLength > MaxManifestBytes {
		_ = resp.Body.Close()
		return nil, ErrManifestTooLarge
	}
	resp.Body = &cappedBody{ReadCloser: resp.Body, left: MaxManifestBytes}
	return resp, nil
}

// ErrManifestTooLarge is a manifest response over MaxManifestBytes.
var ErrManifestTooLarge = fmt.Errorf("the registry's manifest is over the %d-byte limit", MaxManifestBytes)

// isManifestRequest reports whether r, or the request a redirect chain
// started from, is a registry /v2/<name>/manifests/<reference> read.
func isManifestRequest(r *http.Request) bool {
	for r.Response != nil && r.Response.Request != nil {
		r = r.Response.Request
	}
	return path.Base(path.Dir(r.URL.Path)) == "manifests"
}

// cappedBody fails a read past its limit instead of truncating it.
type cappedBody struct {
	io.ReadCloser
	left int64
}

func (b *cappedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, ErrManifestTooLarge
	}
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.ReadCloser.Read(p)
	b.left -= int64(n)
	if b.left < 0 {
		return n + int(b.left), ErrManifestTooLarge
	}
	return n, err
}
