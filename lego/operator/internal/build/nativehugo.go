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

// nativeHugoImage is the reviewed Hugo every native static build carries
// (w8/m106). Render's static builds have Hugo: its Hugo quickstart builds with a
// bare `hugo --gc --minify`. The release is Debian 12's, 0.111.3 extended,
// because Render's native environment is Debian 12 and the quickstart's pinned
// Ananke theme still calls resources.ToCSS, which later Hugo releases removed —
// a current Hugo fails Render's own quickstart. Extended links only glibc,
// libstdc++ and libgcc, which every bookworm toolchain image has.
//
// The image is bex's own (deploy/hugo, published by hugo-image.yml): the
// official linux tarballs' `hugo` binary at /hugo, per architecture, from the
// tarballs whose checksums toolchain-freshness.json inventories. Static builds
// used to ADD those tarballs from github.com on every build, and on 2026-11-08
// GitHub's release CDN answered 504 for over 90 minutes and every native
// static build failed (w8/m148); now they pull a digest-pinned image from the
// registry their toolchain bases already come from.
const (
	nativeHugoVersion = "0.111.3"
	nativeHugoImage   = "ghcr.io/bex-co/bex-hugo:" + nativeHugoVersion + "@sha256:bd3086eab038804155c0736780102f29efd5cd7731fda72449c49b00981f87e4"
)

// nativeStaticTools returns, for a native static build, the lines that put
// Hugo on the image PATH before the source and the authored build. hugo is
// copied to / for the reason nativeMediaTools copies ffmpeg there: a --link
// copy that creates no parent directory is byte-reproducible, so every static
// image on every base carries one identical layer, stored and pulled once.
// Compute builds get none.
func nativeStaticTools(o Options) string {
	if !o.StaticSite {
		return ""
	}
	return "COPY --link --from=" + nativeHugoImage + " /hugo /hugo\n" +
		"RUN ln -sf /hugo /usr/local/bin/hugo\n"
}
