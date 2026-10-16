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
