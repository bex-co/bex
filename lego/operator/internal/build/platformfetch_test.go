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
		name, transcript string
		want             PlatformFetch
	}{
		{"the 2026-11-08 Hugo outage", hugoDigestMismatch,
			PlatformFetch{Tool: "Hugo toolchain", Host: "github.com", ChecksumMismatch: true}},
		{"an HTTP status", `#9 [bex-pnpm-amd64 1/1] ADD --checksum=sha256:aa https://registry.npmjs.org/@pnpm/linux-x64/-/linux-x64-10.34.7.tgz /pnpm.tgz
#9 ERROR: invalid response status 504
------
 > [bex-pnpm-amd64 1/1] ADD --checksum=sha256:aa https://registry.npmjs.org/@pnpm/linux-x64/-/linux-x64-10.34.7.tgz /pnpm.tgz:
------
error: failed to solve: invalid response status 504`,
			PlatformFetch{Tool: "pnpm toolchain", Host: "registry.npmjs.org", Status: "504"}},
		{"the pinned Hugo image's pull", `#3 [internal] load metadata for ghcr.io/bex-co/bex-hugo:0.111.3@sha256:0000000000000000000000000000000000000000000000000000000000000000
#3 ERROR: failed to resolve source metadata for ghcr.io/bex-co/bex-hugo:0.111.3: unexpected status from HEAD request to https://ghcr.io/v2/bex-co/bex-hugo/manifests/sha256:0: 503 Service Unavailable
------
 > [internal] load metadata for ghcr.io/bex-co/bex-hugo:0.111.3@sha256:0000000000000000000000000000000000000000000000000000000000000000:
------
error: failed to solve: ghcr.io/bex-co/bex-hugo: 503 Service Unavailable`,
			PlatformFetch{Tool: "Hugo toolchain", Host: "ghcr.io", Status: "503"}},
		{"the progress header scrolled off the capture", `------
 > [bex-hugo-amd64 1/1] ADD --checksum=sha256:b382 https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz /hugo.tar.gz:
------
error: failed to solve: failed to load cache key: invalid response status 502`,
			PlatformFetch{Tool: "Hugo toolchain", Host: "github.com", Status: "502"}},
		{"the main stage's pinned pnpm package", `#21 [stage-0 4/14] ADD --link --checksum=sha256:c729 https://registry.npmjs.org/pnpm/-/pnpm-10.34.7.tgz /opt/bex/pnpm.tgz
#21 ERROR: invalid response status 503`,
			PlatformFetch{Tool: "pnpm toolchain", Host: "registry.npmjs.org", Status: "503"}},
		{"a helper image copied into the main stage", `#30 [stage-0 6/14] COPY --link --from=mwader/static-ffmpeg:9.0.2-1@sha256:f5d1 /ffmpeg /ffprobe /
#30 ERROR: failed to copy: httpReadSeeker: failed open: unexpected status code https://registry-1.docker.io/v2/mwader/static-ffmpeg/blobs/sha256:aa: 429 Too Many Requests`,
			PlatformFetch{Tool: "FFmpeg toolchain", Host: "docker.io", Status: "429"}},
		{"the native base image", `#2 [internal] load metadata for docker.io/library/busybox:1.37.0@sha256:bdf5
#2 ERROR: failed to resolve source metadata for docker.io/library/busybox:1.37.0: 500 Internal Server Error`,
			PlatformFetch{Tool: "build helper image", Host: "docker.io", Status: "500"}},
		{"another step's status is not this one's", `#14 [bex-pnpm-arm64 1/1] ADD --checksum=sha256:aa https://registry.npmjs.org/@pnpm/linux-arm64/-/linux-arm64-10.34.7.tgz /pnpm.tgz
#14 0.2 retrying: invalid response status 429
#15 [bex-hugo-amd64 1/1] ADD --checksum=sha256:b382 https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz /hugo.tar.gz
#15 ERROR: digest mismatch sha256:bef1: sha256:b382`,
			PlatformFetch{Tool: "Hugo toolchain", Host: "github.com", ChecksumMismatch: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := platformFetchFailure(tc.transcript); got == nil || *got != tc.want {
				t.Fatalf("platformFetchFailure = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The tenant's own failing step keeps its existing reason, and so does a
// compile step inside a helper stage: only a fetch is the platform's download.
func TestPlatformFetchFailureLeavesOtherStepsAlone(t *testing.T) {
	for _, transcript := range []string{
		`#25 [stage-0 11/12] RUN build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz'
#25 0.3 curl: (22) The requested URL returned error: 504
#25 0.3 [bex-hugo-amd64 1/1] ghcr.io/bex-co/bex-hugo
#25 ERROR: build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz' exited with code 22
------
 > [stage-0 11/12] RUN build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz':
0.3 curl: (22) The requested URL returned error: 504
------
error: build command 'curl -fsS https://github.com/x/y/releases/download/v1/z.tgz' exited with code 22`,
		`#12 [bex-node-build 3/5] RUN make -j4 node
#12 ERROR: process "/bin/sh -c make -j4 node" did not complete successfully: exit code: 137`,
		"fatal: unable to access repo\nerror: git fetch failed",
		"",
	} {
		if got := platformFetchFailure(transcript); got != nil {
			t.Errorf("platformFetchFailure(%q) = %+v, want no platform fetch", transcript, got)
		}
	}
}
