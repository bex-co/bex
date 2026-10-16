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
	"path"
	"strings"
)

// nativeStaticInstallPath is the bex-owned step a native Node static build
// runs between the env loader and its authored build command (w8/m86): Render
// installs a static site's dependencies before its build unless
// SKIP_INSTALL_DEPS=true.
const (
	nativeStaticInstallPath = "/opt/bex/install-deps"
	nativeStaticInstallEOF  = "BEX_INSTALL_DEPS"
	// nativeStaticInstallStep invokes it through bash, so the step needs no
	// exec bit, and stops the RUN with the step's own status.
	nativeStaticInstallStep = "/bin/bash " + nativeStaticInstallPath + " || exit\n"
)

// nativeStaticInstallScript decides at build time, under the effective
// environment and in the service's root directory, as the authored command
// will run; its checks are bash builtins, so a narrow PATH cannot skew them.
//
// It installs an npm project (a package.json with an npm lockfile or none)
// with `npm install --include=dev`: the lockfile's versions when it is
// current, and the build tools a NODE_ENV=production environment would
// otherwise omit. A project that is another package manager's (its lockfile,
// or a packageManager field naming it) is left to its build command, with a
// line saying so. A failed install does not fail the build: it is announced,
// and the authored command runs as it did before automatic installs, so a
// site that installs in its own command (a private registry, custom flags)
// keeps working. Only this step's process gains the image's tool directories
// after the service's PATH; the authored command keeps its PATH exactly.
var nativeStaticInstallScript = `#!/bin/bash
case "${SKIP_INSTALL_DEPS:-}" in
  [Tt][Rr][Uu][Ee]) echo "==> Skipping dependency installation (SKIP_INSTALL_DEPS=true)"; exit 0 ;;
esac
[ -f package.json ] || exit 0
other=
for lock in yarn.lock pnpm-lock.yaml ` + strings.Join(nativeBunLockfiles, " ") + `; do
  [ -e "$lock" ] && other="$other $lock"
done
manifest=$(<package.json)
if [[ $manifest =~ \"packageManager\"[[:space:]]*:[[:space:]]*\"(yarn|pnpm|bun)@ ]]; then
  other="$other packageManager ${BASH_REMATCH[1]}"
fi
if [ -n "$other" ]; then
  echo "==> Not installing dependencies automatically:$other is another package manager's; install them in the build command"
  exit 0
fi
lockfile=
for lock in npm-shrinkwrap.json package-lock.json; do
  if [ -f "$lock" ]; then lockfile=" ($lock)"; break; fi
done
echo "==> Installing dependencies with npm$lockfile"
PATH="${PATH:+$PATH:}/usr/local/bin:/usr/bin:/bin" npm install --include=dev --no-audit --no-fund
status=$?
if [ "$status" -ne 0 ]; then
  echo "==> Automatic dependency installation failed (exit $status); running the build command anyway. Set SKIP_INSTALL_DEPS=true to install in the build command instead"
fi
exit 0
`

// nativeStaticInstall returns the Dockerfile lines that carry the install
// step and the RUN prefix that invokes it, for a native Node static build. A
// site publishing its whole root directory gets neither: installed modules
// would be published with it.
func nativeStaticInstall(o Options) (copyLines, runPrefix string) {
	if !o.StaticSite || nativeRuntime(o) != nativeNodeRuntime || publishesRoot(o.StaticPublishPath) {
		return "", ""
	}
	return "COPY --chmod=0755 <<'" + nativeStaticInstallEOF + "' " + nativeStaticInstallPath + "\n" + nativeStaticInstallScript + nativeStaticInstallEOF + "\n",
		nativeStaticInstallStep
}

// publishesRoot reports whether a static site's publish path is its service
// root directory itself; an unset path says nothing.
func publishesRoot(publishPath string) bool {
	publishPath = strings.TrimSpace(publishPath)
	return publishPath != "" && path.Clean("/"+publishPath) == "/"
}
