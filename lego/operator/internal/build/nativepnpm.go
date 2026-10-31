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

import "strings"

// nativePnpm is the reviewed pnpm every native Node build carries (w8/m73).
// Render's native Node environment has pnpm at build and runtime with no
// opt-in, but the official node images ship none: Node 22/24 bundle a Corepack
// without an enabled pnpm launcher and Node 25+ bundle no Corepack at all. The
// pinned npm tarball is pure JavaScript (no dependencies, no install scripts,
// engines node>=18.12), so one checksum covers every reviewed Node line and
// architecture. The checksum is the tarball's sha256, inventoried in
// toolchain-freshness.json; changing the pin changes the generated Dockerfile
// and therefore BuildKit's cache key for the pnpm layers.
var nativePnpm = struct{ version, checksum string }{
	version:  "10.34.7",
	checksum: "sha256:c729f6538e005d234889910466fd1c4c44affb0b28ea86d93fcf88aa1aefa694",
}

// nativeNodeTools is the generated layers that complete a primary Node
// toolchain image, right after its FROM and before the source and the authored
// build, so the build, the pre-deploy step and the started service all find
// them: Yarn 1 from the reviewed Yarn source (Node 26's official image ships
// none, w4/320; on 22/24 it lays down the same 1.22.22 the image carries) and
// the pinned pnpm, unpacked into npm's global prefix in the official node
// images — exactly where `npm install -g pnpm` would put it, on the image PATH
// beside node and npm. Other runtimes get nothing here. ADD --checksum verifies
// the tarball before anything runs; --link keeps these layers cached when the
// primary Node line changes. The RUNs are bex's own, under the image's PATH,
// never the tenant's env.
func nativeNodeTools(o Options) string {
	if nativeRuntime(o) != nativeNodeRuntime {
		return ""
	}
	return nativeYarnInstall() + nativePnpmInstall()
}

// nativePnpmInstall is the pnpm layer pair itself, shared by primary Node
// builds and the JavaScript companion of Ruby and Python builds (w8/m77).
func nativePnpmInstall() string {
	return "ADD --link --checksum=" + nativePnpm.checksum + " https://registry.npmjs.org/pnpm/-/pnpm-" + nativePnpm.version + ".tgz /opt/bex/pnpm.tgz\n" +
		"RUN mkdir -p /usr/local/lib/node_modules/pnpm" +
		" && tar -xzf /opt/bex/pnpm.tgz -C /usr/local/lib/node_modules/pnpm --strip-components=1 --no-same-owner && rm /opt/bex/pnpm.tgz" +
		" && ln -sf ../lib/node_modules/pnpm/bin/pnpm.cjs /usr/local/bin/pnpm && ln -sf ../lib/node_modules/pnpm/bin/pnpx.cjs /usr/local/bin/pnpx\n"
}

// nativeLegacyPnpm is the same pnpm release as pnpm's official standalone
// Linux build (@pnpm/linux-x64, @pnpm/linux-arm64), for the Node lines its
// JavaScript entrypoint refuses (w8/226): pnpm 10 needs Node >= 18.12 to run,
// so on Node 14 and 16 the package above exits 1 before doing anything. The
// standalone carries its own Node, so these lines keep pnpm 10 rather than a
// silent downgrade to an older major. Each checksum is that architecture's
// tarball sha256, inventoried in toolchain-freshness.json.
var nativeLegacyPnpm = struct{ amd64, arm64 string }{
	amd64: "sha256:0ccd4ab9ffee62e9905396dfd9f9c9d67737f58563a4e9199811cf4665965062",
	arm64: "sha256:e5f004bc5e0b1acab7b6fda01a7d41f77670f07f0ea45ce9e56878938d898292",
}

// nativeLegacyPnpmLine reports whether a Node line (major or full version)
// needs the standalone pnpm.
func nativeLegacyPnpmLine(line string) bool {
	major, _, _ := strings.Cut(line, ".")
	return major == "14" || major == "16"
}

// nativePnpmFor is the pnpm layers for the selected Node line.
func nativePnpmFor(line string) string {
	if nativeLegacyPnpmLine(line) {
		return nativeLegacyPnpmInstall()
	}
	return nativePnpmInstall()
}

// nativeLegacyPnpmInstall puts the standalone under /opt/bex/pnpm and fronts
// it with launchers on the image PATH where the package would link pnpm and
// pnpx. The architecture comes from dpkg, not a TARGETARCH build ARG, which
// would reach the authored build's environment. Two things the JavaScript
// package does on its own the launcher restores for the standalone:
//
//   - Lifecycle scripts get NODE and npm_node_execpath from pnpm's own
//     executable unless NODE is set, which would point them at the pnpm
//     binary; the launcher sets NODE to the app's node when the caller has
//     not set it.
//   - pnpm puts its bundled node-gyp on a lifecycle script's PATH, but the
//     standalone's copy lives inside the executable where sh cannot run it,
//     so `node-gyp rebuild` (every gypfile package's default install) is not
//     found. The launcher prepends a node-gyp shim that honours an explicit
//     npm_config_node_gyp naming a real file and otherwise runs the node-gyp
//     the app's own npm bundles, with the app's node.
func nativeLegacyPnpmInstall() string {
	registry := "https://registry.npmjs.org/@pnpm/linux-$arch/-/linux-$arch-" + nativePnpm.version + ".tgz"
	return "RUN case \"$(dpkg --print-architecture)\" in" +
		" amd64) arch=x64 sum=" + strings.TrimPrefix(nativeLegacyPnpm.amd64, "sha256:") + " ;;" +
		" arm64) arch=arm64 sum=" + strings.TrimPrefix(nativeLegacyPnpm.arm64, "sha256:") + " ;;" +
		" *) echo \"pnpm: no standalone build for $(dpkg --print-architecture)\" >&2; exit 1 ;; esac" +
		" && mkdir -p /opt/bex/pnpm/bin /opt/bex/pnpm/node-gyp-bin" +
		" && curl -fsSL --retry 3 -o /opt/bex/pnpm.tgz \"" + registry + "\"" +
		" && echo \"$sum  /opt/bex/pnpm.tgz\" | sha256sum -c - >/dev/null" +
		" && tar -xzf /opt/bex/pnpm.tgz -C /opt/bex/pnpm/bin --strip-components=1 --no-same-owner package/pnpm && rm /opt/bex/pnpm.tgz\n" +
		"COPY --chmod=755 <<'BEX_PNPM' /usr/local/bin/pnpm\n" +
		"#!/bin/sh\n" +
		"if [ -z \"${NODE+x}\" ] && node=$(command -v node); then export NODE=\"$node\"; fi\n" +
		"PATH=\"/opt/bex/pnpm/node-gyp-bin:$PATH\" exec /opt/bex/pnpm/bin/pnpm \"$@\"\n" +
		"BEX_PNPM\n" +
		"COPY --chmod=755 <<'BEX_PNPX' /usr/local/bin/pnpx\n" +
		"#!/bin/sh\n" +
		"exec /usr/local/bin/pnpm dlx \"$@\"\n" +
		"BEX_PNPX\n" +
		"COPY --chmod=755 <<'BEX_NODE_GYP' /opt/bex/pnpm/node-gyp-bin/node-gyp\n" +
		"#!/bin/sh\n" +
		"if [ -n \"$npm_config_node_gyp\" ] && [ -f \"$npm_config_node_gyp\" ]; then exec \"$npm_config_node_gyp\" \"$@\"; fi\n" +
		"exec node /usr/local/lib/node_modules/npm/node_modules/node-gyp/bin/node-gyp.js \"$@\"\n" +
		"BEX_NODE_GYP\n"
}
