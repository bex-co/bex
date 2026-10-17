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
	// nativeStaticInstallCopy writes the step into the image; the resolve
	// phase reads it as "this build installs a static site's dependencies".
	nativeStaticInstallCopy = "COPY --chmod=0755 <<'" + nativeStaticInstallEOF + "' " + nativeStaticInstallPath + "\n"
)

// nativeStaticInstallScript decides at build time, under the effective
// environment and in the service's root directory, as the authored command
// will run; its checks are bash builtins, so a narrow PATH cannot skew them.
//
// A package.json gets the project of the one package manager its lockfiles
// and packageManager field name, as Render runs yarn or npm install (w8/m90):
// an npm project (an npm lockfile, or none) with `npm install --include=dev`,
// and a Yarn Classic one with `yarn install --production=false`, each keeping
// a current lockfile's versions and adding the build tools a
// NODE_ENV=production environment would otherwise omit. A project another
// manager owns (pnpm or Bun), a Yarn 2+ project (its lockfile, or
// packageManager naming it, which the installed Yarn 1 refuses), and one whose
// signals name more than one manager are left to the build command, with a
// line saying so. A Gemfile gets `bundle install` from the Ruby the resolve
// phase adds for it (w8/m105), as Render's Jekyll quickstart builds with a bare
// `bundle exec jekyll build`. A failed install does not fail the build: it is
// announced, and the authored command runs as it did before automatic
// installs, so a site that installs in its own command (a private registry,
// custom flags) keeps working. Only this step's processes gain the image's
// tool directories after the service's PATH; the authored command keeps its
// PATH exactly.
var nativeStaticInstallScript = `#!/bin/bash
case "${SKIP_INSTALL_DEPS:-}" in
  [Tt][Rr][Uu][Ee]) echo "==> Skipping dependency installation (SKIP_INSTALL_DEPS=true)"; exit 0 ;;
esac
tools="${PATH:+$PATH:}/usr/local/bin:/usr/bin:/bin"
run_install() {
  echo "==> Installing dependencies with $1"
  shift
  PATH=$tools "$@"
  local status=$?
  if [ "$status" -ne 0 ]; then
    echo "==> Automatic dependency installation failed (exit $status); running the build command anyway. Set SKIP_INSTALL_DEPS=true to install in the build command instead"
  fi
}
javascript() {
  local signals= managers= yarn_declared= manifest head
  signal() {
    signals="$signals $2"
    [[ " $managers " == *" $1 "* ]] || managers="$managers $1"
  }
  for lock in npm-shrinkwrap.json package-lock.json; do [ -e "$lock" ] && signal npm "$lock"; done
  [ -e yarn.lock ] && signal yarn yarn.lock
  [ -e pnpm-lock.yaml ] && signal pnpm pnpm-lock.yaml
  for lock in ` + strings.Join(nativeBunLockfiles, " ") + `; do [ -e "$lock" ] && signal bun "$lock"; done
  manifest=$(<package.json)
  if [[ $manifest =~ \"packageManager\"[[:space:]]*:[[:space:]]*\"(npm|yarn|pnpm|bun)@([^\"]*)\" ]]; then
    [ "${BASH_REMATCH[1]}" = yarn ] && yarn_declared=${BASH_REMATCH[2]}
    signal "${BASH_REMATCH[1]}" "packageManager ${BASH_REMATCH[1]}@${BASH_REMATCH[2]}"
  fi
  skip() {
    echo "==> Not installing dependencies automatically: $1 (${signals# }); install them in the build command"
  }
  case "$managers" in
    "" | " npm")
      run_install "npm${signals:+ (${signals# })}" npm install --include=dev --no-audit --no-fund ;;
    " yarn")
      # A Yarn 2+ lockfile has a __metadata block after its short comment
      # header; Yarn Classic's has none.
      if ! [[ -z $yarn_declared || $yarn_declared == 1 || $yarn_declared == 1.* ]]; then
        skip "this is a Yarn 2+ project"
        return
      fi
      if [ -f yarn.lock ]; then
        mapfile -n 8 -t head < yarn.lock
        if [[ ${head[*]} == *__metadata:* ]]; then
          skip "this is a Yarn 2+ project"
          return
        fi
      fi
      run_install "Yarn (${signals# })" yarn install --production=false --non-interactive ;;
    " "*" "*) skip "more than one package manager is named" ;;
    *) skip "this is a${managers} project" ;;
  esac
}
[ -f package.json ] && javascript
if [ -f Gemfile ]; then
  if PATH=$tools command -v bundle >/dev/null; then
    lockfile=
    [ -f Gemfile.lock ] && lockfile=" (Gemfile.lock)"
    run_install "Bundler$lockfile" bundle install
  else
    echo "==> Not installing Gemfile dependencies automatically: Bundler is not available; install them in the build command"
  fi
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
	return nativeStaticInstallCopy + nativeStaticInstallScript + nativeStaticInstallEOF + "\n",
		nativeStaticInstallStep
}

// publishesRoot reports whether a static site's publish path is its service
// root directory itself; an unset path says nothing.
func publishesRoot(publishPath string) bool {
	publishPath = strings.TrimSpace(publishPath)
	return publishPath != "" && path.Clean("/"+publishPath) == "/"
}
