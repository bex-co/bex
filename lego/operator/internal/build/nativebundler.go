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

// The Bundler a build's lockfile names (w9/128, docs/ADR060 D3).
//
// A lockfile records the Bundler that wrote it (`BUNDLED WITH`). When the
// build's Ruby ships another one, `bundle install` installs the recorded
// version and restarts on it, but into BUNDLE_PATH's own layout,
// /opt/render/project/.gems/ruby/<ABI>, where RubyGems does not look. The
// `bundle` binstub that install writes first on PATH asks RubyGems for that
// version, so a lockfile written by a Bundler newer than the Ruby's own
// (2.6.9 on Ruby 3.3's 2.5.22) failed every later `bundle exec` with exit 42:
// a Ruby service's at start, a Gemfile static site's in its build command.
//
// Render runs the recorded Bundler from GEM_PATH: its builds and services
// run /opt/render/project/.gems/gems/bundler-<BUNDLED WITH> (Bundler 2.4.21
// and 2.5.5 on Ruby 3.2.2, 2.5.11 on 3.3.3 and 2.6.7 on 3.4.3, in tracebacks
// Render users have published). bex does the same before the build command:
// the env loader installs the recorded Bundler into the first GEM_PATH
// directory, Render's /opt/render/project/.gems on a Ruby build, or into
// RubyGems' own where no GEM_PATH is set (a static site's Ruby), unless
// RubyGems already has it, and says which Bundler the build uses. Bundler's
// rules decide whether there is one: the lockfile beside the Gemfile Bundler
// would read (BUNDLE_GEMFILE, else gems.rb before Gemfile), and nothing when
// BUNDLER_VERSION or a BUNDLE_VERSION other than `lockfile` overrides it. A
// failed install does not fail the build: Bundler then goes on as it did
// before. The lockfile is read with shell builtins, since a service's PATH
// need not hold the system's tools.
const nativeLockedBundler = `bex_locked_bundler() {
  local gemfile lock line next= want= out dir=${GEM_PATH%%:*}
  local tools="${PATH:+$PATH:}/usr/local/bin:/usr/bin:/bin"
  [ -z "${BUNDLER_VERSION:-}" ] && [ "${BUNDLE_VERSION:-lockfile}" = lockfile ] || return 0
  gemfile=${BUNDLE_GEMFILE:-Gemfile}
  [ -n "${BUNDLE_GEMFILE:-}" ] || [ ! -f gems.rb ] || gemfile=gems.rb
  case $gemfile in
    *gems.rb) lock=${gemfile%.rb}.locked ;;
    *) lock=$gemfile.lock ;;
  esac
  [ -f "$gemfile" ] && [ -f "$lock" ] && PATH=$tools command -v gem >/dev/null || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    [ -z "$next" ] || want=$line next=
    [ "${line%$'\r'}" != "BUNDLED WITH" ] || next=1
  done < "$lock"
  want=${want//[[:space:]]/}
  [[ $want =~ ^[0-9]+(\.[0-9]+)+$ ]] || return 0
  if PATH=$tools gem list -i '^bundler$' -v "$want" >/dev/null 2>&1; then
    echo "==> Using Bundler $want (from ${lock##*/})"
    return 0
  fi
  echo "==> Installing Bundler $want (from ${lock##*/})"
  out=$(PATH=$tools gem install bundler -v "$want" ${dir:+--install-dir "$dir"} --no-document 2>&1) && return 0
  echo "==> Could not install Bundler $want:"
  printf '%s\n' "$out"
}
bex_locked_bundler
unset -f bex_locked_bundler
`

// nativeLockedBundlerStep is the step for o's build: a Ruby one, and a static
// site's whose dependencies bex installs, which gets a Ruby for its Gemfile
// (w8/m105); "" otherwise.
func nativeLockedBundlerStep(o Options) string {
	if nativeRuntime(o) != nativeRubyRuntime && !nativeStaticInstalls(o) {
		return ""
	}
	return nativeLockedBundler
}
