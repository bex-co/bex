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

package build

import (
	"strings"
	"testing"
)

// hugoDigestMismatch is the captured tail of a static build on 2026-11-08,
// when GitHub's release CDN answered 504 with an HTML page whose sha256 is
// bef140a1… (w8/m148).
const hugoDigestMismatch = `#14 [bex-hugo-arm64 1/1] ADD --checksum=sha256:695176416dbe09d8c84abeb093f5f27132e6811221569811c855b577122bd670 https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-arm64.tar.gz /hugo.tar.gz
#14 CANCELED
#15 [bex-hugo-amd64 1/1] ADD --checksum=sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz /hugo.tar.gz
#15 ERROR: digest mismatch sha256:bef140a1a96994029153dca8c00b1750b9a5a764fb9db2dc68d7bb40e8a29e8a: sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b
------
 > [bex-hugo-amd64 1/1] ADD --checksum=sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz /hugo.tar.gz:
------
error: failed to solve: digest mismatch sha256:bef140a1a96994029153dca8c00b1750b9a5a764fb9db2dc68d7bb40e8a29e8a: sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b`

func TestPlatformFetchFailureNamesTheDownload(t *testing.T) {
	for _, tc := range []struct {
		name, transcript, want string
	}{
		{"the 2026-11-08 Hugo outage", hugoDigestMismatch,
			"bex could not download its Hugo toolchain (github.com served a file that did not match its pinned checksum)"},
		{"an HTTP status", `#9 [bex-pnpm-amd64 1/1] ADD --checksum=sha256:aa https://registry.npmjs.org/@pnpm/linux-x64/-/linux-x64-10.34.7.tgz /pnpm.tgz
#9 ERROR: invalid response status 504
------
 > [bex-pnpm-amd64 1/1] ADD --checksum=sha256:aa https://registry.npmjs.org/@pnpm/linux-x64/-/linux-x64-10.34.7.tgz /pnpm.tgz:
------
error: failed to solve: invalid response status 504`,
			"bex could not download its pnpm toolchain (HTTP 504 from registry.npmjs.org)"},
		{"a platform image pull", `#3 [internal] load metadata for ghcr.io/bex-co/bex-hugo:0.111.3@sha256:0000000000000000000000000000000000000000000000000000000000000000
#3 ERROR: failed to resolve source metadata for ghcr.io/bex-co/bex-hugo:0.111.3: unexpected status from HEAD request to https://ghcr.io/v2/bex-co/bex-hugo/manifests/sha256:0: 503 Service Unavailable
------
 > [internal] load metadata for ghcr.io/bex-co/bex-hugo:0.111.3@sha256:0000000000000000000000000000000000000000000000000000000000000000:
------
error: failed to solve: ghcr.io/bex-co/bex-hugo: 503 Service Unavailable`,
			"bex could not download its Hugo toolchain (HTTP 503 from ghcr.io)"},
		{"the progress header scrolled off the capture", `------
 > [bex-hugo-amd64 1/1] ADD --checksum=sha256:b382 https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz /hugo.tar.gz:
------
error: failed to solve: failed to load cache key: invalid response status 502`,
			"bex could not download its Hugo toolchain (HTTP 502 from github.com)"},
		{"the main stage's pinned pnpm package", `#21 [stage-5 4/14] ADD --link --checksum=sha256:c729 https://registry.npmjs.org/pnpm/-/pnpm-10.34.7.tgz /opt/bex/pnpm.tgz
#21 ERROR: invalid response status 503`,
			"bex could not download its pnpm toolchain (HTTP 503 from registry.npmjs.org)"},
		{"a helper image copied into the main stage", `#30 [stage-5 6/14] COPY --link --from=mwader/static-ffmpeg:9.0.2-1@sha256:f5d1 /ffmpeg /ffprobe /
#30 ERROR: failed to copy: httpReadSeeker: failed open: unexpected status code https://registry-1.docker.io/v2/mwader/static-ffmpeg/blobs/sha256:aa: 429 Too Many Requests`,
			"bex could not download its FFmpeg toolchain (HTTP 429 from docker.io)"},
		{"the native base image", `#2 [internal] load metadata for docker.io/library/busybox:1.37.0@sha256:bdf5
#2 ERROR: failed to resolve source metadata for docker.io/library/busybox:1.37.0: 500 Internal Server Error`,
			"bex could not download its build helper image (HTTP 500 from docker.io)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := platformFetchFailure(tc.transcript)
			if !ok || !strings.HasPrefix(got, tc.want+"; this is a platform problem") || !strings.Contains(got, "retry") {
				t.Fatalf("platformFetchFailure = %q, %v; want it to start %q and say to retry", got, ok, tc.want)
			}
			if strings.Contains(got, "sha256:") {
				t.Errorf("the sentence carries a raw digest: %q", got)
			}
		})
	}
}

// The tenant's own failing step keeps its existing reason: its stage is the
// native build's main stage, and its output may mention anything.
func TestPlatformFetchFailureLeavesTenantStepsAlone(t *testing.T) {
	for _, transcript := range []string{
		`#25 [stage-5 11/12] RUN build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz'
#25 0.3 curl: (22) The requested URL returned error: 504
#25 0.3 [bex-hugo-amd64 1/1] ghcr.io/bex-co/bex-hugo
#25 ERROR: build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz' exited with code 22
------
 > [stage-5 11/12] RUN build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz':
0.3 curl: (22) The requested URL returned error: 504
------
error: build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz' exited with code 22`,
		"fatal: unable to access repo\nerror: git fetch failed",
		"",
	} {
		if got, ok := platformFetchFailure(transcript); ok {
			t.Errorf("platformFetchFailure(%q) = %q, want no platform verdict", transcript, got)
		}
	}
}
