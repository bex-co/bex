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

// nativeHugo is the reviewed Hugo every native static build carries (w8/m106).
// Render's static builds have Hugo: its Hugo quickstart builds with a bare
// `hugo --gc --minify`. The release is Debian 12's, 0.111.3 extended, because
// Render's native environment is Debian 12 and the quickstart's pinned Ananke
// theme still calls resources.ToCSS, which later Hugo releases removed — a
// current Hugo fails Render's own quickstart. The official linux tarballs are
// per architecture; each checksum is that tarball's sha256, inventoried in
// toolchain-freshness.json. Extended links only glibc, libstdc++ and libgcc,
// which every bookworm toolchain image has.
var nativeHugo = struct{ version, amd64, arm64 string }{
	version: "0.111.3",
	amd64:   "sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b",
	arm64:   "sha256:695176416dbe09d8c84abeb093f5f27132e6811221569811c855b577122bd670",
}

// nativeStaticTools returns, for a native static build, the helper stages
// that fetch and unpack Hugo and the lines that put it on the image PATH
// before the source and the authored build. Each architecture's tarball is an
// ADD --checksum (content-addressed, so it is fetched once whatever the base)
// and only the build platform's (TARGETARCH) is reached; the pinned busybox
// unpacks just the binary, without the archive's owner. hugo is copied to / for
// the reason nativeMediaTools copies ffmpeg there: a --link copy that creates
// no parent directory is byte-reproducible, so every static image on every
// base carries one identical layer, stored and pulled once. Builders are
// amd64 or arm64 only; another TARGETARCH names no stage and fails the build.
// Compute builds get neither.
func nativeStaticTools(o Options) (stages, lines string) {
	if !o.StaticSite {
		return "", ""
	}
	release := "https://github.com/gohugoio/hugo/releases/download/v" + nativeHugo.version + "/hugo_extended_" + nativeHugo.version + "_linux-"
	return "FROM scratch AS bex-hugo-amd64\n" +
			"ADD --checksum=" + nativeHugo.amd64 + " " + release + "amd64.tar.gz /hugo.tar.gz\n" +
			"FROM scratch AS bex-hugo-arm64\n" +
			"ADD --checksum=" + nativeHugo.arm64 + " " + release + "arm64.tar.gz /hugo.tar.gz\n" +
			"FROM bex-hugo-${TARGETARCH} AS bex-hugo-tarball\n" +
			"FROM " + nativePreparerImage + " AS bex-hugo\n" +
			"RUN --mount=type=bind,from=bex-hugo-tarball,target=/in mkdir /out && tar -xzof /in/hugo.tar.gz -C /out hugo\n",
		"COPY --link --from=bex-hugo /out/hugo /hugo\n" +
			"RUN ln -sf /hugo /usr/local/bin/hugo\n"
}
