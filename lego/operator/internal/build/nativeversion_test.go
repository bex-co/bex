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
	"context"
	"encoding/base64"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/types/toolchains"
)

func files(m map[string]string) func(string) ([]byte, bool) {
	return func(name string) ([]byte, bool) {
		v, ok := m[name]
		return []byte(v), ok
	}
}

// discourseLock is the tail of sweep354's real Discourse Gemfile.lock: its
// RUBY VERSION block pins 3.4.7 with a patchlevel.
const discourseLock = `PLATFORMS
  ruby
  x86_64-linux

RUBY VERSION
   ruby 3.4.7p58

BUNDLED WITH
   2.6.9
`

// TestResolveNativeVersion pins w8/m51's resolution policy per runtime. Every
// non-default row fails on the single-pin code it replaces.
func TestResolveNativeVersion(t *testing.T) {
	cases := []struct {
		name, runtime string
		env, files    map[string]string
		line, source  string
	}{
		{"python default", "python", nil, nil, "3.13", ""},
		{"python env, full patch → minor line", "python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil, "3.11", "PYTHON_VERSION=3.11.9"},
		{"python env beats file", "python", map[string]string{"PYTHON_VERSION": "3.12"}, map[string]string{".python-version": "3.10"}, "3.12", "PYTHON_VERSION=3.12"},
		{"python file", "python", nil, map[string]string{".python-version": "# pinned\n3.10.4\n"}, "3.10", ".python-version"},
		{"node default", "node", nil, nil, "24", ""},
		// Versions Render still installs (w8/173).
		{"node 20", "node", map[string]string{"NODE_VERSION": "20"}, nil, "20", "NODE_VERSION=20"},
		{"node 14 default-table pin", "node", map[string]string{"NODE_VERSION": "14.17.0"}, nil, "14", "NODE_VERSION=14.17.0"},
		{"node .nvmrc 18", "node", nil, map[string]string{".nvmrc": "18"}, "18", ".nvmrc"},
		{"node tilde engines", "node", nil, map[string]string{"package.json": `{"engines":{"node":"~16.13.0"}}`}, "16", "package.json engines.node"},
		{"node bounded old range", "node", nil, map[string]string{"package.json": `{"engines":{"node":">=16 <21"}}`}, "20", "package.json engines.node"},
		{"python 3.9", "python", map[string]string{"PYTHON_VERSION": "3.9.18"}, nil, "3.9", "PYTHON_VERSION=3.9.18"},
		{"python 3.7 floor", "python", map[string]string{"PYTHON_VERSION": "3.7.3"}, nil, "3.7", "PYTHON_VERSION=3.7.3"},
		// Python 3.15 was released 2026-10-09; Render takes any released version (w8/335).
		{"python 3.15 exact", "python", map[string]string{"PYTHON_VERSION": "3.15.2"}, nil, "3.15", "PYTHON_VERSION=3.15.2"},
		{"python 3.15 file", "python", nil, map[string]string{".python-version": "3.15\n"}, "3.15", ".python-version"},
		{"ruby 3.2 Render rails example", "ruby", nil, map[string]string{".ruby-version": "ruby-3.2.0"}, "3.2", ".ruby-version"},
		{"ruby 3.2 Gemfile", "ruby", nil, map[string]string{"Gemfile": "ruby '3.2.0'"}, "3.2", "Gemfile"},
		{"elixir 1.20", "elixir", map[string]string{"ELIXIR_VERSION": "1.20.4"}, nil, "1.20", "ELIXIR_VERSION=1.20.4"},
		{"node env major", "node", map[string]string{"NODE_VERSION": "22"}, nil, "22", "NODE_VERSION=22"},
		{"node env full", "node", map[string]string{"NODE_VERSION": "v22.11.0"}, nil, "22", "NODE_VERSION=v22.11.0"},
		{"node .node-version beats .nvmrc", "node", nil, map[string]string{".node-version": "26", ".nvmrc": "22"}, "26", ".node-version"},
		// Node 26 has been LTS since 2026-10-28 (w8/332).
		{"node .nvmrc lts", "node", nil, map[string]string{".nvmrc": "lts/*"}, "26", ".nvmrc"},
		{"node env lts", "node", map[string]string{"NODE_VERSION": "lts"}, nil, "26", "NODE_VERSION=lts"},
		{"node engines lts", "node", nil, map[string]string{"package.json": `{"engines":{"node":"lts/*"}}`}, "26", "package.json engines.node"},
		{"node .nvmrc codename", "node", nil, map[string]string{".nvmrc": "lts/jod"}, "22", ".nvmrc"},
		{"node env codename lithium", "node", map[string]string{"NODE_VERSION": "lts/lithium"}, nil, "26", "NODE_VERSION=lts/lithium"},
		{"node env codename krypton", "node", map[string]string{"NODE_VERSION": "lts/Krypton"}, nil, "24", "NODE_VERSION=lts/Krypton"},
		{"node .nvmrc beats engines", "node", nil, map[string]string{".nvmrc": "22", "package.json": `{"engines":{"node":">=26"}}`}, "22", ".nvmrc"},
		{"node bounded engines range", "node", nil, map[string]string{"package.json": `{"engines":{"node":">=20 <23"}}`}, "22", "package.json engines.node"},
		{"node caret engines", "node", nil, map[string]string{"package.json": `{"engines":{"node":"^22.5.0"}}`}, "22", "package.json engines.node"},
		{"node open engines → newest", "node", nil, map[string]string{"package.json": `{"engines":{"node":">=18"}}`}, "26", "package.json engines.node"},
		{"node or-range", "node", nil, map[string]string{"package.json": `{"engines":{"node":"20.x || 22.x"}}`}, "22", "package.json engines.node"},
		{"node package.json without engines", "node", nil, map[string]string{"package.json": `{"name":"x"}`}, "24", ""},
		// Render's Ruby precedence (w8/m85): lockfile RUBY VERSION, .ruby-version,
		// .tool-versions, then the Gemfile directive.
		{"ruby lockfile beats a permissive Gemfile range", "ruby", nil, map[string]string{
			"Gemfile": "source 'https://rubygems.org'\nruby '>= 3.4', '< 4.1'\n", "Gemfile.lock": discourseLock}, "3.4", "Gemfile.lock"},
		{"ruby lockfile beats every file", "ruby", nil, map[string]string{"Gemfile.lock": discourseLock, ".ruby-version": "4.0.1",
			".tool-versions": "ruby 3.3.6\n", "Gemfile": "ruby '4.0.0'"}, "3.4", "Gemfile.lock"},
		{"ruby gems.locked", "ruby", nil, map[string]string{"gems.locked": discourseLock}, "3.4", "gems.locked"},
		{"ruby gems.locked beats Gemfile.lock, as in Bundler", "ruby", nil, map[string]string{
			"Gemfile.lock": "RUBY VERSION\n   ruby 3.3.6p108\n", "gems.locked": discourseLock}, "3.4", "gems.locked"},
		{"ruby gems.locked without RUBY VERSION, Gemfile.lock with", "ruby", nil, map[string]string{
			"gems.locked": "GEM\n  remote: https://rubygems.org/\n", "Gemfile.lock": discourseLock}, "3.4", "Gemfile.lock"},
		{"ruby Bundler 4 lockfile without patchlevel, CRLF", "ruby", nil, map[string]string{"Gemfile.lock": "RUBY VERSION\r\n  ruby 3.3.6\r\n"}, "3.3", "Gemfile.lock"},
		{"ruby stale unsupported lock defers to the Gemfile", "ruby", nil, map[string]string{
			"Gemfile.lock": "RUBY VERSION\n   ruby 2.6.10p210\n", "Gemfile": "ruby '~> 3.2'"}, "3.4", "Gemfile"},
		{"ruby stale unsupported lock and no other signal keeps the default", "ruby", nil, map[string]string{
			"Gemfile.lock": "RUBY VERSION\n   ruby 2.7.8p225\n"}, "3.4", ""},
		{"ruby .tool-versions system defers", "ruby", nil, map[string]string{".tool-versions": "ruby system\n", "Gemfile": "ruby '3.3.6'"}, "3.3", "Gemfile"},
		{"ruby JRuby lockfile", "ruby", nil, map[string]string{"Gemfile.lock": "RUBY VERSION\n   ruby 3.3.6p108 (jruby 9.4.9.0)\n"}, "3.3", "Gemfile.lock"},
		{"ruby lockfile without RUBY VERSION falls through", "ruby", nil, map[string]string{
			"Gemfile.lock": "GEM\n  remote: https://rubygems.org/\n  specs:\n    rake (13.2.1)\n", ".ruby-version": "3.3.1"}, "3.3", ".ruby-version"},
		{"ruby .ruby-version beats .tool-versions", "ruby", nil, map[string]string{".ruby-version": "4.0.1", ".tool-versions": "ruby 3.3.6\n"}, "4.0", ".ruby-version"},
		{"ruby .ruby-version beats Gemfile", "ruby", nil, map[string]string{"Gemfile": "source 'x'\nruby '3.3.6'\n", ".ruby-version": "4.0.1"}, "4.0", ".ruby-version"},
		{"ruby .tool-versions", "ruby", nil, map[string]string{".tool-versions": "# pinned\nnodejs 24.1.0\nruby 3.3.6 3.4.7 # fallback\n"}, "3.3", ".tool-versions"},
		{"ruby .tool-versions beats Gemfile", "ruby", nil, map[string]string{".tool-versions": "ruby 3.3.6\n", "Gemfile": "ruby '4.0.0'"}, "3.3", ".tool-versions"},
		{"ruby .tool-versions without ruby falls through", "ruby", nil, map[string]string{".tool-versions": "nodejs 24.1.0\n", "Gemfile": "ruby '4.0.0'"}, "4.0", "Gemfile"},
		{"ruby pessimistic minor", "ruby", nil, map[string]string{"Gemfile": `ruby "~> 3.3.0"`}, "3.3", "Gemfile"},
		{"ruby pessimistic major", "ruby", nil, map[string]string{"Gemfile": `ruby "~> 3.3"`}, "3.4", "Gemfile"},
		{"ruby file", "ruby", nil, map[string]string{".ruby-version": "ruby-4.0.0"}, "4.0", ".ruby-version"},
		{"ruby file: directive falls through", "ruby", nil, map[string]string{"Gemfile": `ruby file: ".ruby-version"`, ".ruby-version": "3.3.1"}, "3.3", ".ruby-version"},
		{"elixir env", "elixir", map[string]string{"ELIXIR_VERSION": "1.17.3"}, nil, "1.17", "ELIXIR_VERSION=1.17.3"},
		{"elixir with matching erlang", "elixir", map[string]string{"ELIXIR_VERSION": "1.19", "ERLANG_VERSION": "28.1"}, nil, "1.19", "ELIXIR_VERSION=1.19, ERLANG_VERSION=28.1"},
		{"go ignores go.mod (Render parity)", "go", nil, map[string]string{"go.mod": "module x\n\ngo 1.26\n"}, "1.27", ""},
		{"rust stays on its one image, naming rustup's toolchain", "rust", map[string]string{"RUSTUP_TOOLCHAIN": "1.80"}, nil, "1", "RUSTUP_TOOLCHAIN=1.80"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveNativeVersion(tc.runtime, tc.env, files(tc.files))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got.Line != tc.line || got.Source != tc.source {
				t.Fatalf("got line %q from %q, want %q from %q", got.Line, got.Source, tc.line, tc.source)
			}
			if line, _ := nativeToolchains[tc.runtime].find(tc.line); got.Image != line.image {
				t.Fatalf("image %q is not the %s line's pin", got.Image, tc.line)
			}
		})
	}
}

// sidekiqLock is render-examples/sidekiq's Gemfile.lock at 73f3ab4 (w9/133):
// its Gemfile pins ruby '3.2.0', which Bundler enforces, and the lockfile
// records that run.
const sidekiqLock = `PLATFORMS
  x86_64-linux

DEPENDENCIES
  puma (~> 6)

RUBY VERSION
   ruby 3.2.0p0

BUNDLED WITH
   2.4.1
`

// TestRubyExactSignalsPinTheirPatch: every Ruby signal that names a fully
// qualified version asks for exactly that release, as Render installs it and
// as Bundler requires when the Gemfile pins it (w9/133). Before, only
// .ruby-version did, so render-examples/sidekiq built on 3.2.11 and Bundler
// refused it.
func TestRubyExactSignalsPinTheirPatch(t *testing.T) {
	reviewed, _ := nativeToolchains[nativeRubyRuntime].find("3.2")
	cases := []struct {
		name         string
		files        map[string]string
		source, want string
	}{
		{"sidekiq lockfile and Gemfile", map[string]string{"Gemfile.lock": sidekiqLock, "Gemfile": "source 'https://rubygems.org'\nruby '3.2.0'\n"}, "Gemfile.lock", "3.2.0"},
		{"lockfile with a patchlevel", map[string]string{"Gemfile.lock": discourseLock}, "Gemfile.lock", "3.4.7"},
		{"gems.locked", map[string]string{"gems.locked": discourseLock}, "gems.locked", "3.4.7"},
		{"Bundler 4 lockfile, CRLF", map[string]string{"Gemfile.lock": "RUBY VERSION\r\n  ruby 3.3.6\r\n"}, "Gemfile.lock", "3.3.6"},
		{"lockfile beats a ranged Gemfile", map[string]string{"Gemfile.lock": sidekiqLock, "Gemfile": "ruby '~> 3.2'"}, "Gemfile.lock", "3.2.0"},
		{"Gemfile alone", map[string]string{"Gemfile": "source 'https://rubygems.org'\nruby '3.2.0'\n"}, "Gemfile", "3.2.0"},
		{"Gemfile with a ruby- prefix", map[string]string{"Gemfile": `ruby "ruby-3.3.1"`}, "Gemfile", "3.3.1"},
		{"stale unsupported lock defers to the Gemfile's patch", map[string]string{"Gemfile.lock": "RUBY VERSION\n   ruby 2.7.8p225\n", "Gemfile": "ruby '3.2.0'"}, "Gemfile", "3.2.0"},
		{".tool-versions after another tool", map[string]string{".tool-versions": "# pinned\nnodejs 24.1.0\nruby 3.3.6 3.4.7 # fallback\n"}, ".tool-versions", "3.3.6"},
		{".ruby-version, as before", map[string]string{".ruby-version": "ruby-3.2.0\n"}, ".ruby-version", "3.2.0"},
		// A requirement Bundler meets with the line's patch keeps it.
		{"pessimistic Gemfile", map[string]string{"Gemfile": `ruby "~> 3.2.0"`}, "Gemfile", ""},
		{"two-sided Gemfile", map[string]string{"Gemfile": `ruby ">= 3.2.0", "< 3.3"`}, "Gemfile", ""},
		{"Gemfile names only the line", map[string]string{"Gemfile": `ruby "3.2"`}, "Gemfile", ""},
		{"lockfile records the reviewed patch", map[string]string{"Gemfile.lock": "RUBY VERSION\n   ruby " + reviewed.version + "p100\n"}, "Gemfile.lock", ""},
		{".tool-versions names only the line", map[string]string{".tool-versions": "ruby 3.2\n"}, ".tool-versions", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveNativeVersion(nativeRubyRuntime, nil, files(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			if got.Source != tc.source || got.Exact != tc.want {
				t.Fatalf("got exact %q from %q, want %q from %q", got.Exact, got.Source, tc.want, tc.source)
			}
		})
	}
}

func TestResolveNativeVersionRefusesUnsupportedByName(t *testing.T) {
	cases := []struct {
		runtime    string
		env, files map[string]string
		want       string
	}{
		{"python", map[string]string{"PYTHON_VERSION": "2.7.18"}, nil, `PYTHON_VERSION=2.7.18 is not a supported Python version; supported Python lines are 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15`},
		{"python", map[string]string{"PYTHON_VERSION": "3.16.0"}, nil, `PYTHON_VERSION=3.16.0 is not a supported Python version; supported Python lines are 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15`},
		{"python", map[string]string{"PYTHON_VERSION": "3.6.15"}, nil, `PYTHON_VERSION=3.6.15 is not a supported Python version`},
		{"python", nil, map[string]string{".python-version": "pypy3.10"}, `.python-version "pypy3.10"`},
		{"node", map[string]string{"NODE_VERSION": "8"}, nil, "supported Node lines are 14, 16, 18, 20, 22, 24, 26"},
		{"node", map[string]string{"NODE_VERSION": "lts/notacodename"}, nil, "NODE_VERSION=lts/notacodename is not a supported Node version; supported Node lines are 14, 16, 18, 20, 22, 24, 26"},
		{"node", nil, map[string]string{"package.json": `{"engines":{"node":"<14"}}`}, `package.json engines.node "<14"`},
		{"ruby", nil, map[string]string{"Gemfile": `ruby "2.7.8"`}, `Gemfile ruby "2.7.8"`},
		{"ruby", nil, map[string]string{".tool-versions": "ruby 2.7.8\n"}, `.tool-versions ruby "2.7.8"`},
		// w8/373: Render builds any compatible pair from Elixir 1.12.0 and
		// Erlang/OTP 24.3.4, and an incompatible one fails.
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.16.3", "ERLANG_VERSION": "28.0"}, nil,
			`ERLANG_VERSION="28.0" is not compatible with Elixir 1.16.3, which supports Erlang/OTP 24 to 26; set ERLANG_VERSION to a compatible release or choose another ELIXIR_VERSION`},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.12.3", "ERLANG_VERSION": "25"}, nil, `ERLANG_VERSION="25" is not compatible with Elixir 1.12.3, which supports Erlang/OTP 22 to 24`},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.13.3", "ERLANG_VERSION": "25.3"}, nil, "Elixir 1.13.3, which supports Erlang/OTP 22 to 24"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.18.2", "ERLANG_VERSION": "28.1"}, nil, "Elixir 1.18.2, which supports Erlang/OTP 25 to 27"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.18", "ERLANG_VERSION": "24.3.4"}, nil, "Elixir 1.18.6, which supports Erlang/OTP 25 to 28"},
		{"elixir", map[string]string{"ERLANG_VERSION": "29"}, nil, `ERLANG_VERSION="29" is not compatible with Elixir 1.18.6 (the default), which supports Erlang/OTP 25 to 28`},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.12.3", "ERLANG_VERSION": "24.2"}, nil,
			`ERLANG_VERSION="24.2" is earlier than Erlang/OTP 24.3.4, the oldest Render supports; set ERLANG_VERSION to 24.3.4 or a later release`},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.12.3", "ERLANG_VERSION": "OTP-24.3.3.9"}, nil, `ERLANG_VERSION="OTP-24.3.3.9" is earlier than Erlang/OTP 24.3.4`},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.12.3", "ERLANG_VERSION": "23"}, nil, "is earlier than Erlang/OTP 24.3.4"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.11.4"}, nil,
			"ELIXIR_VERSION=1.11.4 is not a supported Elixir version; supported Elixir lines are 1.17, 1.18, 1.19, 1.20, and any other Elixir release from 1.12.0 (Render's minimum) to 1.20 installs on a compatible Erlang/OTP"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.21.0"}, nil, "ELIXIR_VERSION=1.21.0 is not a supported Elixir version"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.16.3-rc.0"}, nil, "ELIXIR_VERSION=1.16.3-rc.0 is not a supported Elixir version"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.17.x"}, nil, "ELIXIR_VERSION=1.17.x is not a supported Elixir version"},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.18", "ERLANG_VERSION": "28.x"}, nil, "ERLANG_VERSION=28.x is not an Erlang/OTP release"},
	}
	for _, tc := range cases {
		_, err := ResolveNativeVersion(tc.runtime, tc.env, files(tc.files))
		var unsupported *UnsupportedVersionError
		if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %v %v: err = %v, want an UnsupportedVersionError containing %q", tc.runtime, tc.env, tc.files, err, tc.want)
		}
	}
}

// `lts` names the newest line whose LTS has begun when the build runs, so it
// moves on Node's own date with no edit (w8/332).
func TestNodeLTSLineFollowsTheSchedule(t *testing.T) {
	for _, tc := range []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, time.October, 27, 23, 59, 59, 0, time.UTC), "24"},
		{time.Date(2026, time.October, 28, 0, 0, 0, 0, time.UTC), "26"},
		{time.Date(2026, time.November, 16, 13, 50, 0, 0, time.UTC), "26"},
		{time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC), "22"},
		{time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC), "14"},
	} {
		if got := nodeLTSLineAt(tc.now); got != tc.want {
			t.Errorf("nodeLTSLineAt(%s) = %q, want %q", tc.now.Format(time.DateOnly), got, tc.want)
		}
	}
}

// Every reviewed Node line carries its LTS date and every entry names a
// reviewed line, oldest first, so adding a line without its schedule fails
// here and `lts` never names a line bex cannot build.
func TestNodeLTSCoversTheReviewedLines(t *testing.T) {
	lines := nativeToolchains[nativeNodeRuntime].lines
	if len(nodeLTS) != len(lines) {
		t.Fatalf("nodeLTS has %d entries for %d reviewed Node lines", len(nodeLTS), len(lines))
	}
	for i, r := range nodeLTS {
		if r.line != lines[i].line {
			t.Errorf("nodeLTS[%d] is line %s, want the reviewed line %s", i, r.line, lines[i].line)
		}
		if r.codename != strings.ToLower(r.codename) {
			t.Errorf("line %s codename %q must be lower-case", r.line, r.codename)
		}
		if i > 0 && !nodeLTS[i-1].from.Before(r.from) {
			t.Errorf("line %s's LTS date %s is not after line %s's", r.line, r.from.Format(time.DateOnly), nodeLTS[i-1].line)
		}
	}
}

// Render's docs lines, which its build logs print after each runtime's version
// line (w9/141; docs/render-artifacts/native-version-narration.md quotes them).
const (
	nodeDocs   = "\n==> Docs on specifying a Node.js version: https://render.com/docs/node-version"
	pythonDocs = "\n==> Docs on specifying a Python version: https://render.com/docs/python-version"
	rubyDocs   = "\n==> Docs on specifying a Ruby version: https://render.com/docs/ruby-version"
	elixirDocs = "\n==> Docs on specifying Elixir and Erlang versions: https://render.com/docs/elixir-erlang-versions"
	bunDocs    = "\n==> Docs on specifying a Bun version: https://render.com/docs/bun-version"
	poetryDocs = "\n==> Docs on specifying a Poetry version: https://render.com/docs/poetry-version"
	uvDocs     = "\n==> Docs on specifying a uv version: https://render.com/docs/uv-version"
)

// w9/139: each runtime's line reads as Render's build log has it for every
// signal a Render log shows (docs/render-artifacts/native-version-narration.md):
// `version`, the environment variable by name, a version file by its path in
// the image, `(default)` when nothing names a version. A signal no Render log
// shows keeps bex's line. Before the resolve phase swaps in an exact image the
// line names the line's reviewed patch (w4/668). w9/141: Render's docs line
// follows each runtime's version line whatever the signal (none for Go or
// Rust), and a Node range or alias is echoed first, as the service wrote it.
func TestNativeChoiceNarration(t *testing.T) {
	reviewed := func(runtime, line string) string {
		t.Helper()
		l, ok := nativeToolchains[runtime].find(line)
		if !ok || l.version == "" {
			t.Fatalf("no reviewed %s %s patch", runtime, line)
		}
		return l.version
	}
	primary := func(runtime string, env, fs map[string]string) func() (NativeChoice, error) {
		return func() (NativeChoice, error) { return ResolveNativeVersion(runtime, env, files(fs)) }
	}
	bun := func(env, fs map[string]string) func() (NativeChoice, error) {
		return func() (NativeChoice, error) {
			c, _, err := ResolveNativeBun(time.Time{}, env, files(fs))
			return c, err
		}
	}
	uv := func(env map[string]string) func() (NativeChoice, error) {
		return func() (NativeChoice, error) {
			c, _, err := ResolveNativeUV(time.Time{}, env, files(map[string]string{"uv.lock": ""}))
			return c, err
		}
	}
	poetry := func(env map[string]string) func() (NativeChoice, error) {
		return func() (NativeChoice, error) { return ResolveNativePoetry(env) }
	}
	fixed := func(c NativeChoice) func() (NativeChoice, error) {
		return func() (NativeChoice, error) { return c, nil }
	}
	const src = "/opt/render/project/src/"
	for _, tc := range []struct {
		name    string
		resolve func() (NativeChoice, error)
		want    string
	}{
		{"python env", primary("python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil),
			"==> Using Python version " + reviewed("python", "3.11") + " via environment variable PYTHON_VERSION" + pythonDocs},
		{"python file", primary("python", nil, map[string]string{".python-version": "3.12\n"}),
			"==> Using Python version " + reviewed("python", "3.12") + " via " + src + ".python-version" + pythonDocs},
		{"python default", primary("python", nil, nil),
			"==> Using Python version " + reviewed("python", "3.13") + " (default)" + pythonDocs},
		// A partial or exact version is not echoed: Render echoes no exact
		// one, and its partial one is unmeasured.
		{"node env", primary("node", map[string]string{"NODE_VERSION": "22"}, nil),
			"==> Using Node.js version " + reviewed("node", "22") + " via environment variable NODE_VERSION" + nodeDocs},
		{"node env exact", primary("node", map[string]string{"NODE_VERSION": reviewed("node", "22")}, nil),
			"==> Using Node.js version " + reviewed("node", "22") + " via environment variable NODE_VERSION" + nodeDocs},
		{"node env range", primary("node", map[string]string{"NODE_VERSION": "20.x"}, nil),
			"==> Requesting Node.js version 20.x\n==> Using Node.js version " + reviewed("node", "20") + " via environment variable NODE_VERSION" + nodeDocs},
		{"node .node-version", primary("node", nil, map[string]string{".node-version": "20\n"}),
			"==> Using Node.js version " + reviewed("node", "20") + " via " + src + ".node-version" + nodeDocs},
		{"node .nvmrc", primary("node", nil, map[string]string{".nvmrc": "18\n"}),
			"==> Using Node.js version " + reviewed("node", "18") + " via " + src + ".nvmrc" + nodeDocs},
		{"node .nvmrc alias", primary("node", nil, map[string]string{".nvmrc": "lts/hydrogen\n"}),
			"==> Requesting Node.js version lts/hydrogen\n==> Using Node.js version " + reviewed("node", "18") + " via " + src + ".nvmrc" + nodeDocs},
		{"node engines", primary("node", nil, map[string]string{"package.json": `{"engines":{"node":"^20"}}`}),
			"==> Requesting Node.js version ^20\n==> Using Node.js version " + reviewed("node", "20") + " via " + src + "package.json" + nodeDocs},
		{"node engines as written", primary("node", nil, map[string]string{"package.json": `{"engines":{"node":" >= 20.4.0 <21 "}}`}),
			"==> Requesting Node.js version >= 20.4.0 <21\n==> Using Node.js version " + reviewed("node", "20") + " via " + src + "package.json" + nodeDocs},
		{"node default", primary("node", nil, nil),
			"==> Using Node.js version " + reviewed("node", "24") + " (default)" + nodeDocs},
		{"ruby Gemfile.lock", primary("ruby", nil, map[string]string{"Gemfile.lock": "RUBY VERSION\n   ruby 3.3.0p0\n"}),
			"==> Using Ruby version " + reviewed("ruby", "3.3") + " via " + src + "Gemfile.lock" + rubyDocs},
		{"ruby .ruby-version", primary("ruby", nil, map[string]string{".ruby-version": "3.2\n"}),
			"==> Using Ruby version " + reviewed("ruby", "3.2") + " via " + src + ".ruby-version" + rubyDocs},
		{"ruby Gemfile", primary("ruby", nil, map[string]string{"Gemfile": "ruby \"~> 3.1.0\"\n"}),
			"==> Using Ruby version " + reviewed("ruby", "3.1") + " via " + src + "Gemfile" + rubyDocs},
		{"ruby default", primary("ruby", nil, nil),
			"==> Using Ruby version " + reviewed("ruby", "3.4") + " (default)" + rubyDocs},
		{"elixir default", primary("elixir", nil, nil),
			"==> Using Elixir version " + reviewed("elixir", "1.18") + " (default)" + elixirDocs},
		// Render names the default Erlang on a line of its own, before Elixir,
		// and points to one page for the pair.
		{"elixir on Render's Erlang", fixed(NativeChoice{Label: "Elixir", Line: "1.18.4", otp: "28.0.2"}),
			"==> Using Erlang version 28.0.2 (default)\n==> Using Elixir version 1.18.4 (default)" + elixirDocs},
		{"bun lockfile", bun(nil, map[string]string{"bun.lock": "{}"}),
			"==> Using Bun version " + nativeBun.lines[len(nativeBun.lines)-1].version + " (default)" + bunDocs},
		{"bun binary lockfile", bun(nil, map[string]string{"bun.lockb": "\x00"}),
			"==> Using Bun version " + nativeBun.lines[len(nativeBun.lines)-1].version + " (default)" + bunDocs},
		{"poetry default", poetry(nil), "==> Using Poetry version " + defaultPoetryVersion + " (default)" + poetryDocs},
		{"uv lockfile", uv(nil), "==> Using uv version " + legacyUVVersion + " (default)" + uvDocs},
		// bex's own note about the image comes after Render's lines, where
		// Render's install line would be.
		{"note", fixed(NativeChoice{Label: "Node", Line: "22.1.5", note: "==> No official image node:22.1.5-bookworm; using the reviewed Node 22 image"}),
			"==> Using Node.js version 22.1.5 (default)" + nodeDocs + "\n==> No official image node:22.1.5-bookworm; using the reviewed Node 22 image"},
		{"declined companion", fixed(NativeChoice{Label: "Ruby", declined: "Gemfile asks for 2.7"}), "==> Not adding Ruby: Gemfile asks for 2.7"},

		// No Render log on record for these signals: bex's own line stays,
		// with the runtime's docs line after it.
		{"ruby .tool-versions", primary("ruby", nil, map[string]string{".tool-versions": "ruby 3.0.7\n"}),
			"==> Using Ruby " + reviewed("ruby", "3.0") + " (from .tool-versions)" + rubyDocs},
		{"ruby gems.locked", primary("ruby", nil, map[string]string{"gems.locked": "RUBY VERSION\n   ruby 3.3.0p0\n"}),
			"==> Using Ruby " + reviewed("ruby", "3.3") + " (from gems.locked)" + rubyDocs},
		{"elixir env", primary("elixir", map[string]string{"ELIXIR_VERSION": "1.19"}, nil),
			"==> Using Elixir " + reviewed("elixir", "1.19") + " (from ELIXIR_VERSION=1.19)" + elixirDocs},
		{"elixir ERLANG_VERSION", fixed(NativeChoice{Label: "Elixir", Line: "1.18.4", otp: "28.0.2", Source: "ERLANG_VERSION=28.0", erlang: "28.0"}),
			"==> Using Elixir 1.18.4 on Erlang/OTP 28.0.2 (from ERLANG_VERSION=28.0)" + elixirDocs},
		{"bun env", bun(map[string]string{"BUN_VERSION": "1.3"}, nil), "==> Using Bun 1.3.14 (from BUN_VERSION=1.3)" + bunDocs},
		{"bun file", bun(nil, map[string]string{".bun-version": "1.3\n"}), "==> Using Bun 1.3.14 (from .bun-version)" + bunDocs},
		{"poetry env", poetry(map[string]string{"POETRY_VERSION": "1.8.4"}), "==> Using Poetry 1.8.4 (from POETRY_VERSION=1.8.4)" + poetryDocs},
		{"uv env", uv(map[string]string{"UV_VERSION": "0.9.0"}), "==> Using uv 0.9.0 (from UV_VERSION=0.9.0)" + uvDocs},
		// Render's Go line has no docs line after it; no Rust line is on record.
		{"go", primary("go", nil, nil), "==> Using Go 1.27 (default)"},
		{"rust", primary("rust", nil, nil), "==> Using Rust " + reviewed("rust", "1") + " (default)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := tc.resolve()
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Narration(); got != tc.want {
				t.Errorf("narration = %q, want %q", got, tc.want)
			}
		})
	}

	// Render names a version file under the service's root directory
	// ("…/backend/.python-version"); a variable has no path.
	c, _ := ResolveNativeVersion("node", nil, files(map[string]string{".nvmrc": "18\n"}))
	if got, want := c.NarrationIn(src+"apps/web"), "==> Using Node.js version "+reviewed("node", "18")+" via "+src+"apps/web/.nvmrc"+nodeDocs; got != want {
		t.Errorf("root directory narration = %q, want %q", got, want)
	}
	c, _ = ResolveNativeVersion("node", map[string]string{"NODE_VERSION": "18"}, files(nil))
	if got := c.NarrationIn(src + "apps/web"); !strings.HasSuffix(got, " via environment variable NODE_VERSION"+nodeDocs) {
		t.Errorf("root directory env narration = %q", got)
	}
}

// Every pinned line is a reviewed inventory entry (docs/ADR060 D7).
func TestNativeToolchainLinesAreInventoried(t *testing.T) {
	inv, err := LoadToolchainInventory()
	if err != nil {
		t.Fatal(err)
	}
	committed := map[string]bool{}
	for _, img := range inv.Images {
		committed[img.Committed] = true
	}
	inventoried := map[string]nativeToolchain{"bun": nativeBun}
	maps.Copy(inventoried, nativeToolchains)
	for runtime, toolchain := range inventoried {
		for _, line := range toolchain.lines {
			if !committed[line.image] {
				t.Errorf("%s line %s image %s is not in toolchain-freshness.json", runtime, line.line, line.image)
			}
		}
	}
	if pnpm := "pnpm-" + nativePnpm.version + ".tgz@" + nativePnpm.checksum; !committed[pnpm] {
		t.Errorf("pnpm %s is not in toolchain-freshness.json", pnpm)
	}
	if !committed[nativeFFmpegImage] {
		t.Errorf("ffmpeg %s is not in toolchain-freshness.json", nativeFFmpegImage)
	}
}

// writeResolveFixture lays out what the resolve phase sees: the preparer's
// env bundle and Dockerfile, and a checkout root with the given files.
func writeResolveFixture(t *testing.T, o Options, env, src map[string]string) (bundle, dockerfile, sourceDir string) {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	for k, v := range env {
		b.WriteString(k + "=" + base64.StdEncoding.EncodeToString([]byte(v)) + "\n")
	}
	bundle, dockerfile, sourceDir = filepath.Join(dir, "render-env"), filepath.Join(dir, "Dockerfile"), filepath.Join(dir, "src")
	must(t, os.WriteFile(bundle, []byte(b.String()), 0o600))
	must(t, os.WriteFile(dockerfile, []byte(nativeDockerfile(o)), 0o600))
	must(t, os.MkdirAll(sourceDir, 0o755))
	for name, content := range src {
		must(t, os.MkdirAll(filepath.Dir(filepath.Join(sourceDir, name)), 0o755))
		must(t, os.WriteFile(filepath.Join(sourceDir, name), []byte(content), 0o644))
	}
	return bundle, dockerfile, sourceDir
}

// resolvedDockerfile is o's generated Dockerfile after the resolve phase, for
// env and an empty source.
func resolvedDockerfile(t *testing.T, o Options, env map[string]string) string {
	t.Helper()
	return resolvedDockerfileFrom(t, o, env, nil)
}

// resolvedDockerfileFrom is resolvedDockerfile for a source holding files.
func resolvedDockerfileFrom(t *testing.T, o Options, env, files map[string]string) string {
	t.Helper()
	dockerfile, _ := resolveFixture(t, o, env, files)
	return dockerfile
}

// nativeLineSignal selects line on runtime through Render's own version
// signal: an env var, or Ruby's .ruby-version. Go and Rust offer one line.
func nativeLineSignal(runtime, line string) (env, files map[string]string) {
	env, files = map[string]string{}, map[string]string{}
	switch runtime {
	case nativePythonRuntime:
		env["PYTHON_VERSION"] = line
	case nativeNodeRuntime:
		env["NODE_VERSION"] = line
	case nativeElixirRuntime:
		env["ELIXIR_VERSION"] = line
	case nativeRubyRuntime:
		files[".ruby-version"] = line
	}
	return env, files
}

// resolveFixture runs the resolve phase over o's generated Dockerfile, env and
// files, returning the rewritten Dockerfile and the choices it made.
func resolveFixture(t *testing.T, o Options, env, files map[string]string) (string, []NativeChoice) {
	t.Helper()
	bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
	choices, err := resolveNativeBuild(nativeRuntime(o), bundle, dockerfile, src)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), choices
}

// commonResolutionKeeps reports whether a common-base resolution of the
// generated Dockerfile before changed nothing the generator wrote: the final
// stage starts from the common image, every generated line after the original
// FROM survives byte for byte at the end, and the inserted runtime lines copy
// each named image.
func commonResolutionKeeps(before, after, generatedFrom string, copied ...string) bool {
	_, rest, found := strings.Cut(before, "\nFROM "+generatedFrom+"\n")
	if !found || !strings.Contains(after, "\nFROM "+nativeCommonBase+"\n"+"COPY --link --from=") || !strings.HasSuffix(after, "\n"+rest) {
		return false
	}
	for _, image := range copied {
		if !strings.Contains(after, "\nCOPY --link --from="+image+" ") {
			return false
		}
	}
	return true
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The resolve phase rewrites the generated Dockerfile's FROM — and only it —
// to the requested line; the w8/m51 live repro (PYTHON_VERSION=3.11.9 built
// on 3.13) is the first case.
func TestResolveNativeBuildRewritesTheBase(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": "3.11.9", "OTHER": "x\ny"}, nil)
	before, _ := os.ReadFile(dockerfile)
	// The exact 3.11.9 tag is looked up; here the registry has none, so the
	// line's reviewed image is the base (the exact case: nativeexact_test.go).
	choices, err := ResolveNativeBuildExact(context.Background(), &fakeExact{}, time.Time{}, "python", bundle, dockerfile, src)
	if err != nil || len(choices) != 3 || choices[0].Line != "3.11" || choices[1].Label != "Poetry" || choices[2].Label != "Node" {
		t.Fatalf("resolve = %+v, %v", choices, err)
	}
	after, _ := os.ReadFile(dockerfile)
	py311, _ := nativeToolchains["python"].find("3.11")
	want := strings.Replace(string(before), "FROM "+nativeToolchains["python"].defaultImage()+"\n", "FROM "+expectedNativeBase(py311.image)+"\n"+
		nativeJSToolLines(nativeToolchains["node"].defaultImage(), nativeToolchains["node"].defaultLine)+
		"RUN /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install --no-cache-dir --disable-pip-version-check --no-input poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry\n", 1)
	if nativeCommonBase != "" {
		// The common base copies Python 3.11 and the default Node companion
		// in; Poetry follows them, and the generated lines stay as they were.
		poetry := "RUN /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install --no-cache-dir --disable-pip-version-check --no-input poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry\n"
		if !commonResolutionKeeps(string(before), string(after), nativeToolchains["python"].defaultImage(), py311.image, nativeToolchains["node"].defaultImage()) ||
			!strings.Contains(string(after), `ENV PYTHON_VERSION="`+py311.version+`"`) || !strings.Contains(string(after), poetry) {
			t.Fatalf("Dockerfile =\n%s", after)
		}
	} else if string(after) != want {
		t.Fatalf("Dockerfile =\n%s\nwant\n%s", after, want)
	}

	// Node reads its files from the service's root directory.
	o.Runtime = "node"
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n"})
	if choices, err := resolveNativeBuild("node", bundle, dockerfile, src); err != nil || len(choices) != 1 || choices[0].Source != ".nvmrc" || choices[0].Line != "22" {
		t.Fatalf("node resolve = %+v, %v", choices, err)
	}
}

// w8/m73: choosing a Node line rewrites only the FROM, so every reviewed line
// keeps the generated pnpm layers in front of the authored build, alongside an
// opted-in Bun.
func TestResolveNativeBuildKeepsPnpmOnEveryNodeLine(t *testing.T) {
	for _, l := range nativeToolchains[nativeNodeRuntime].lines {
		env := map[string]string{"NODE_VERSION": l.line}
		if l.line == "26" {
			env["BUN_VERSION"] = "1.3"
		}
		bundle, dockerfile, src := writeResolveFixture(t, nativeOptions(), env, nil)
		before, _ := os.ReadFile(dockerfile)
		if _, err := resolveNativeBuild(nativeNodeRuntime, bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(dockerfile)
		// Node 14/16 carry the standalone pnpm instead of the package (w8/226).
		tools := strings.Replace(nativeNodeTools(nativeOptions()), nativePnpmInstall(), nativePnpmFor(l.line), 1)
		if !strings.Contains(string(after), "FROM "+expectedNativeBase(l.image)+"\n") || !strings.Contains(string(after), tools) ||
			strings.Index(string(after), tools) > strings.Index(string(after), "\nCOPY . .\n") {
			t.Fatalf("node %s Dockerfile lost its pnpm layers:\n%s", l.line, after)
		}
		if l.line == "26" && !strings.Contains(string(after), "/usr/local/bin/bun\n") {
			t.Fatalf("Bun opt-in displaced:\n%s", after)
		}
		wantDefault := strings.Replace(string(before), "FROM "+l.image+"\n", "FROM "+expectedNativeBase(l.image)+"\n", 1)
		unchanged := string(after) == wantDefault
		if nativeCommonBase != "" {
			unchanged = commonResolutionKeeps(string(before), string(after), l.image, l.image)
		}
		if l.line == "24" && !unchanged {
			t.Fatalf("the default line must leave the Dockerfile unchanged:\n%s", after)
		}
	}
}

// A version file that is a symlink is not followed: the refusal quotes what
// it read, and the env bundle (the tenant's secrets) sits beside the source.
func TestResolveNativeBuildIgnoresSymlinkedVersionFiles(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"SECRET": "s3cr3t"}, nil)
	must(t, os.Symlink(bundle, filepath.Join(src, ".python-version")))
	choices, err := resolveNativeBuild("python", bundle, dockerfile, src)
	if err != nil || len(choices) != 3 || choices[0].Source != "" {
		t.Fatalf("symlinked .python-version: %+v, %v; want the default, unread", choices, err)
	}
}

func TestResolveNativeBuildRefusesAForeignDockerfile(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	must(t, os.WriteFile(dockerfile, []byte("FROM evil:latest\nRUN true\n"), 0o600))
	if _, err := resolveNativeBuild("python", bundle, dockerfile, src); err == nil {
		t.Fatal("rewrote a Dockerfile the preparer did not generate")
	}
}

// The resolve phase runs after the preparer and before BuildKit, only for
// native builds with a resolver image, read-only on the checkout.
func TestNativeBuildJobRunsTheResolverAfterThePreparer(t *testing.T) {
	o := nativeOptions()
	o.NativeResolverImage = "ghcr.io/bex-co/bex-operator@sha256:" + strings.Repeat("a", 64)
	o.RootDir = "apps/web"
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	names := make([]string, 0, len(spec.InitContainers))
	var resolver corev1.Container
	for _, c := range spec.InitContainers {
		names = append(names, c.Name)
		if c.Name == nativeResolveContainer {
			resolver = c
		}
	}
	order := strings.Join(names, ",")
	if !strings.Contains(order, "prepare-native-build,"+nativeResolveContainer+",buildkit") {
		t.Fatalf("init order = %s", order)
	}
	if resolver.Image != o.NativeResolverImage || resolver.Command[0] != NativeResolveBinary {
		t.Fatalf("resolver = %+v", resolver)
	}
	for _, m := range resolver.VolumeMounts {
		if m.Name == "source" && !m.ReadOnly {
			t.Error("the resolver must not be able to write the checkout")
		}
	}
	env := map[string]string{}
	for _, e := range resolver.Env {
		env[e.Name] = e.Value
	}
	// The log names a version file where the image has it (w9/139).
	if env["BEX_NATIVE_SOURCE_DIR"] != "/source/apps/web" || env[NativeProjectDirEnv] != NativeWorkdir+"/apps/web" || env["BEX_NATIVE_RUNTIME"] != "node" {
		t.Fatalf("resolver env = %v", env)
	}

	o.NativeResolverImage = ""
	for _, c := range BuildJob(o, o.ImageRef()).Spec.Template.Spec.InitContainers {
		if c.Name == nativeResolveContainer {
			t.Fatal("no resolver image must mean no resolve phase")
		}
	}
}

// Bun is an opt-in companion (w8/m71), resolved with Render's precedence; no
// signal leaves the build without it.
func TestResolveNativeBun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		files   map[string]string
		line    string // "" = not selected
		source  string
		exact   string // the release installed exactly (w8/315); "" = the line's reviewed patch
		wantErr string
	}{
		{"sweep306 BUN_VERSION", map[string]string{"BUN_VERSION": "1.3.4"}, nil, "1.3", "BUN_VERSION=1.3.4", "1.3.4", ""},
		{"Render's elysiajs template (w8/315)", map[string]string{"BUN_VERSION": "1.3.0"}, nil, "1.3", "BUN_VERSION=1.3.0", "1.3.0", ""},
		{"the reviewed patch", map[string]string{"BUN_VERSION": "1.4.4"}, nil, "1.4", "BUN_VERSION=1.4.4", "", ""},
		{"Render's default line (w5/m144)", map[string]string{"BUN_VERSION": "1.4.3"}, nil, "1.4", "BUN_VERSION=1.4.3", "1.4.3", ""},
		{"BUN_VERSION wins over the file", map[string]string{"BUN_VERSION": "1.2.20"}, map[string]string{".bun-version": "1.3.0\n"}, "1.2", "BUN_VERSION=1.2.20", "1.2.20", ""},
		{".bun-version", nil, map[string]string{".bun-version": "# pinned\nv1.2.21\n"}, "1.2", ".bun-version", "1.2.21", ""},
		{"bun-v prefix", map[string]string{"BUN_VERSION": "bun-v1.3.0"}, nil, "1.3", "BUN_VERSION=bun-v1.3.0", "1.3.0", ""},
		{"a line", map[string]string{"BUN_VERSION": "1.3"}, nil, "1.3", "BUN_VERSION=1.3", "", ""},
		{"bare major", map[string]string{"BUN_VERSION": "1"}, nil, "1.4", "BUN_VERSION=1", "", ""},
		{"latest", map[string]string{"BUN_VERSION": "latest"}, nil, "1.4", "BUN_VERSION=latest", "", ""},
		{"range", map[string]string{"BUN_VERSION": "~1.2.0"}, nil, "1.2", "BUN_VERSION=~1.2.0", "", ""},
		{"text lockfile", nil, map[string]string{"bun.lock": "{}"}, "1.4", "bun.lock", "", ""},
		{"binary lockfile", nil, map[string]string{"bun.lockb": "\x00"}, "1.4", "bun.lockb", "", ""},
		{"file wins over a lockfile", nil, map[string]string{".bun-version": "1.2", "bun.lock": "{}"}, "1.2", ".bun-version", "", ""},
		{"canary (Render 2026-10-16) starts on the default line", map[string]string{"BUN_VERSION": "canary"}, nil, "1.4", "BUN_VERSION=canary", "canary", ""},
		{".bun-version canary", nil, map[string]string{".bun-version": "Canary\n"}, "1.4", ".bun-version", "canary", ""},
		{"no opt-in", map[string]string{"NODE_VERSION": "22"}, map[string]string{"package.json": "{}"}, "", "", "", ""},
		// Render installs any exact Bun release (w8/372): one on no reviewed
		// line starts on the default line's image, which the resolve phase
		// swaps for its own or refuses by name.
		{"sweep r23 BUN_VERSION=1.1.38", map[string]string{"BUN_VERSION": "1.1.38"}, nil, "1.4", "BUN_VERSION=1.1.38", "1.1.38", ""},
		{".bun-version 1.0.36", nil, map[string]string{".bun-version": "1.0.36\n"}, "1.4", ".bun-version", "1.0.36", ""},
		{"bun-v prefix on no reviewed line", map[string]string{"BUN_VERSION": "bun-v1.1.0"}, nil, "1.4", "BUN_VERSION=bun-v1.1.0", "1.1.0", ""},
		{"a 0.x release", map[string]string{"BUN_VERSION": "0.8.1"}, nil, "1.4", "BUN_VERSION=0.8.1", "0.8.1", ""},
		{"no such release waits for the registry", map[string]string{"BUN_VERSION": "9.9.9"}, nil, "1.4", "BUN_VERSION=9.9.9", "9.9.9", ""},
		{"a line no review carries", map[string]string{"BUN_VERSION": "1.1"}, nil, "", "", "", "BUN_VERSION=1.1 is not a supported Bun version; supported Bun lines are 1.2, 1.3, 1.4 or canary, and any other Bun release installs when named exactly (such as 1.1.38)"},
		{"a range no review carries", nil, map[string]string{".bun-version": "~1.1.0"}, "", "", "", `.bun-version "~1.1.0" is not a supported Bun version; supported Bun lines are 1.2, 1.3, 1.4 or canary, and any other Bun release installs when named exactly (such as 1.1.38)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readFile := func(name string) ([]byte, bool) {
				v, ok := tc.files[name]
				return []byte(v), ok
			}
			choice, found, err := ResolveNativeBun(time.Time{}, tc.env, readFile)
			if tc.wantErr != "" {
				var unsupported *UnsupportedVersionError
				if !errors.As(err, &unsupported) || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || found != (tc.line != "") || choice.Line != tc.line || choice.Source != tc.source || choice.Exact != tc.exact {
				t.Fatalf("= %+v, %v, %v; want line %q from %q, exact %q", choice, found, err, tc.line, tc.source, tc.exact)
			}
			// Only a release on no reviewed line must install or fail by name.
			if want := tc.exact != "" && tc.exact != nativeBunCanary && !strings.HasPrefix(tc.exact, tc.line+"."); choice.unreviewed != want {
				t.Errorf("unreviewed = %v for %+v, want %v", choice.unreviewed, choice, want)
			}
		})
	}
	// Canary resolves to the reviewed default image plus the tag to pin, so no
	// choice ever carries an unpinned image.
	canary, _, _ := ResolveNativeBun(time.Time{}, map[string]string{"BUN_VERSION": "canary"}, func(string) ([]byte, bool) { return nil, false })
	if reviewed, _ := nativeBun.find(nativeBun.defaultLine); canary.Image != reviewed.image || canary.Exact != "canary" {
		t.Errorf("canary choice = %+v, want the reviewed default image with Exact canary", canary)
	}
	choice, _, _ := ResolveNativeBun(time.Time{}, map[string]string{"BUN_VERSION": "1.3"}, func(string) ([]byte, bool) { return nil, false })
	if got := choice.Narration(); got != "==> Using Bun 1.3.14 (from BUN_VERSION=1.3)"+bunDocs {
		t.Errorf("narration = %q", got)
	}
}

// An opted-in Bun is copied into the toolchain image after its FROM and
// before the source and the authored build; the primary choice still
// rewrites the base, and a refused Bun request leaves the Dockerfile alone.
func TestResolveNativeBuildAddsAnOptedInBun(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "node"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "1.3"}, map[string]string{".nvmrc": "22\n"})
	choices, err := resolveNativeBuild("node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[0].Label != "Node" || choices[0].Line != "22" || choices[1].Label != "Bun" || choices[1].Line != "1.3" {
		t.Fatalf("choices = %+v, %v; want Node 22 then Bun 1.3", choices, err)
	}
	df, _ := os.ReadFile(dockerfile)
	node22, _ := nativeToolchains["node"].find("22")
	bun13, _ := nativeBun.find("1.3")
	from := strings.Index(string(df), "\nFROM "+expectedNativeBase(node22.image)+"\n")
	copyBun := strings.Index(string(df), "\nCOPY --link --from="+bun13.image+" /usr/local/bin/bun /usr/local/bin/bun\nRUN ln -sf bun /usr/local/bin/bunx\n")
	source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
	if from < 0 || copyBun < from || source < copyBun || build < source {
		t.Fatalf("Bun is not copied between FROM and the source/build:\n%s", df)
	}

	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "0.8"}, nil)
	before, _ := os.ReadFile(dockerfile)
	var unsupported *UnsupportedVersionError
	if _, err := resolveNativeBuild("node", bundle, dockerfile, src); !errors.As(err, &unsupported) {
		t.Fatalf("unsupported BUN_VERSION = %v, want a tenant UnsupportedVersionError", err)
	}
	if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
		t.Error("a refused Bun request rewrote the Dockerfile")
	}

	// No opt-in: exactly one choice and only the supplied FROM may change.
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, nil)
	before, _ = os.ReadFile(dockerfile)
	if choices, err := resolveNativeBuild("node", bundle, dockerfile, src); err != nil || len(choices) != 1 {
		t.Fatalf("no opt-in choices = %+v, %v", choices, err)
	}
	defaultImage := nativeToolchains[nativeNodeRuntime].defaultImage()
	want := strings.Replace(string(before), "FROM "+defaultImage+"\n", "FROM "+expectedNativeBase(defaultImage)+"\n", 1)
	after, _ := os.ReadFile(dockerfile)
	unchanged := string(after) == want
	if nativeCommonBase != "" {
		unchanged = commonResolutionKeeps(string(before), string(after), defaultImage, defaultImage) && !strings.Contains(string(after), "/usr/local/bin/bun")
	}
	if !unchanged {
		t.Fatalf("no opt-in changed the Dockerfile:\n%s", after)
	}
}

// BUN_VERSION=canary builds with the canary digest current at build time,
// pinned and narrated; an unreachable registry keeps the reviewed default
// line and says so (w5/178).
func TestResolveNativeBuildPinsBunCanary(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "node"
	exact := &fakeExact{digests: map[string]string{bunRepo + ":canary": exactDigest}}
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "canary"}, map[string]string{".nvmrc": "22\n"})
	choices, err := ResolveNativeBuildExact(t.Context(), exact, time.Time{}, "node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[1].Line != "canary" || choices[1].Image != "oven/bun:canary@"+exactDigest {
		t.Fatalf("choices = %+v, %v; want Bun canary pinned to its digest", choices, err)
	}
	if !strings.Contains(choices[1].Narration(), "==> Bun canary is oven/bun:canary@"+exactDigest) {
		t.Errorf("narration = %q, want the pinned digest", choices[1].Narration())
	}
	if df, _ := os.ReadFile(dockerfile); !strings.Contains(string(df), "COPY --link --from=oven/bun:canary@"+exactDigest+" /usr/local/bin/bun") {
		t.Fatalf("Dockerfile does not copy the pinned canary:\n%s", df)
	}

	offline := &fakeExact{err: errors.New("registry down")}
	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "canary"}, nil)
	choices, err = ResolveNativeBuildExact(t.Context(), offline, time.Time{}, "node", bundle, dockerfile, src)
	reviewed, _ := nativeBun.find(nativeBun.defaultLine)
	if err != nil || len(choices) != 2 || choices[1].Image != reviewed.image {
		t.Fatalf("offline choices = %+v, %v; want the reviewed default Bun", choices, err)
	}
	if !strings.Contains(choices[1].Narration(), "Could not look up oven/bun:canary") {
		t.Errorf("offline narration = %q", choices[1].Narration())
	}

	missing := &fakeExact{}
	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "canary"}, nil)
	choices, err = ResolveNativeBuildExact(t.Context(), missing, time.Time{}, "node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[1].Image != reviewed.image || !strings.Contains(choices[1].Narration(), "No official image oven/bun:canary") {
		t.Fatalf("missing tag choices = %+v, %v", choices, err)
	}
}

// w8/315: an exact BUN_VERSION installs that release's official image, pinned
// by digest, as Render installs the exact version; a missing tag keeps the
// reviewed line and says so.
func TestResolveNativeBuildPinsAnExactBun(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "node"
	exact := &fakeExact{digests: map[string]string{bunRepo + ":1.3.0": exactDigest}}
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "1.3.0"}, map[string]string{".nvmrc": "22\n"})
	choices, err := ResolveNativeBuildExact(t.Context(), exact, time.Time{}, "node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[1].Line != "1.3.0" || choices[1].Image != "oven/bun:1.3.0@"+exactDigest {
		t.Fatalf("choices = %+v, %v; want Bun 1.3.0 pinned to its digest", choices, err)
	}
	if got := choices[1].Narration(); got != "==> Using Bun 1.3.0 (from BUN_VERSION=1.3.0)"+bunDocs {
		t.Errorf("narration = %q", got)
	}
	if df, _ := os.ReadFile(dockerfile); !strings.Contains(string(df), "COPY --link --from=oven/bun:1.3.0@"+exactDigest+" /usr/local/bin/bun") {
		t.Fatalf("Dockerfile does not copy the exact Bun:\n%s", df)
	}

	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "1.3.0"}, nil)
	choices, err = ResolveNativeBuildExact(t.Context(), &fakeExact{}, time.Time{}, "node", bundle, dockerfile, src)
	reviewed, _ := nativeBun.find("1.3")
	if err != nil || len(choices) != 2 || choices[1].Image != reviewed.image || !strings.Contains(choices[1].Narration(), "No official image oven/bun:1.3.0") {
		t.Fatalf("missing tag choices = %+v, %v; want the reviewed 1.3 line, said", choices, err)
	}
}

// w8/372: Render installs any exact Bun release (render.com/docs/bun-version),
// so one no reviewed line carries installs from its own official image, pinned
// by digest, from BUN_VERSION or .bun-version alike. A release with no image
// does not exist and fails the build by name; an unreadable registry fails it
// too, since no reviewed line stands in for another Bun.
func TestResolveNativeBuildInstallsAnyBunRelease(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "node"
	for _, tc := range []struct {
		name    string
		env     map[string]string
		files   map[string]string
		release string
		log     string
	}{
		{"BUN_VERSION", map[string]string{"BUN_VERSION": "1.1.38"}, nil, "1.1.38", "==> Using Bun 1.1.38 (from BUN_VERSION=1.1.38)"},
		{".bun-version", nil, map[string]string{".bun-version": "1.0.36\n", "bun.lock": "{}"}, "1.0.36", "==> Using Bun 1.0.36 (from .bun-version)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exact := &fakeExact{digests: map[string]string{bunRepo + ":" + tc.release: exactDigest}}
			bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, tc.files)
			choices, err := ResolveNativeBuildExact(t.Context(), exact, time.Time{}, "node", bundle, dockerfile, src)
			image := "oven/bun:" + tc.release + "@" + exactDigest
			if err != nil || len(choices) != 2 || choices[1].Line != tc.release || choices[1].Image != image {
				t.Fatalf("choices = %+v, %v; want Bun %s pinned to its digest", choices, err, tc.release)
			}
			if got := choices[1].Narration(); got != tc.log+bunDocs {
				t.Errorf("narration = %q, want %q", got, tc.log+bunDocs)
			}
			if df, _ := os.ReadFile(dockerfile); !strings.Contains(string(df), "COPY --link --from="+image+" /usr/local/bin/bun /usr/local/bin/bun\n") {
				t.Fatalf("Dockerfile does not copy Bun %s:\n%s", tc.release, df)
			}
		})
	}

	for _, tc := range []struct {
		name, request, file, wantErr string
	}{
		{"BUN_VERSION", "9.9.9", "", "BUN_VERSION=9.9.9 is not a Bun release: no official image oven/bun:9.9.9 exists; set a published Bun release, latest, or a supported Bun line (1.2, 1.3, 1.4 or canary)"},
		{".bun-version", "", "v1.1.99\n", `.bun-version "v1.1.99" is not a Bun release: no official image oven/bun:1.1.99 exists; set a published Bun release, latest, or a supported Bun line (1.2, 1.3, 1.4 or canary)`},
		{"a version no tag can name", "1.1." + strings.Repeat("9", 200), "", "BUN_VERSION=1.1." + strings.Repeat("9", 104) + "… is not a Bun release: no official image oven/bun:1.1." + strings.Repeat("9", 116) + "… exists; set a published Bun release, latest, or a supported Bun line (1.2, 1.3, 1.4 or canary)"},
	} {
		t.Run("no such release from "+tc.name, func(t *testing.T) {
			env, files := map[string]string{"BUN_VERSION": tc.request}, map[string]string{}
			if tc.file != "" {
				env, files = nil, map[string]string{".bun-version": tc.file}
			}
			bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
			before, _ := os.ReadFile(dockerfile)
			_, err := ResolveNativeBuildExact(t.Context(), &fakeExact{}, time.Time{}, "node", bundle, dockerfile, src)
			var unsupported *UnsupportedVersionError
			if !errors.As(err, &unsupported) || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want the tenant error %q", err, tc.wantErr)
			}
			if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
				t.Error("a release that does not exist rewrote the Dockerfile")
			}
		})
	}

	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "1.1.38"}, nil)
	_, err := ResolveNativeBuildExact(t.Context(), &fakeExact{err: errors.New("registry down")}, time.Time{}, "node", bundle, dockerfile, src)
	if _, tenant := errors.AsType[*UnsupportedVersionError](err); err == nil || tenant || !strings.Contains(err.Error(), "look up Bun 1.1.38 (oven/bun:1.1.38): registry down") {
		t.Fatalf("offline err = %v, want a platform error naming the lookup", err)
	}
}

// A service opted into Bun by its lockfile alone builds on Render's default
// Bun for its creation date (render.com/docs/bun-version, w8/371): that
// release's line, installed exactly when it is not the reviewed patch. Any
// version signal, or a service of unknown age, is unchanged.
func TestResolveNativeBunDatedDefault(t *testing.T) {
	at := func(date string) time.Time {
		created, err := time.Parse(time.RFC3339, date)
		if err != nil {
			t.Fatal(err)
		}
		return created
	}
	lock := map[string]string{"bun.lock": "{}"}
	for _, tc := range []struct {
		name    string
		created time.Time
		env     map[string]string
		files   map[string]string
		line    string
		exact   string
		log     string
	}{
		{"since 2026-11-20", at("2026-11-20T00:00:00Z"), nil, lock, "1.4", "1.4.5", ""},
		{"the day before", at("2026-11-19T23:59:59Z"), nil, lock, "1.4", "", "==> Using Bun version 1.4.4 (default)"},
		{"binary lockfile", at("2026-11-21T08:59:23Z"), nil, map[string]string{"bun.lockb": "\x00"}, "1.4", "1.4.5", ""},
		{"2026-10-12 to 2026-10-22", at("2026-10-15T00:00:00Z"), nil, lock, "1.4", "1.4.3", ""},
		{"2026-09-09 to 2026-10-12", at("2026-09-09T00:00:00Z"), nil, lock, "1.4", "1.4.2", ""},
		{"2026-07-16 to 2026-09-09: the 1.3 line's patch", at("2026-08-01T00:00:00Z"), nil, lock, "1.3", "", "==> Using Bun version 1.3.14 (default)"},
		{"2025-12-08 to 2026-07-16", at("2026-01-01T00:00:00Z"), nil, lock, "1.3", "1.3.4", ""},
		{"2025-08-18 to 2025-12-08", at("2025-08-18T00:00:00Z"), nil, lock, "1.2", "1.2.20", ""},
		{"before 2025-08-18: 1.1.0 on no reviewed line (w8/372)", at("2025-01-01T00:00:00Z"), nil, lock, "1.4", "1.1.0", ""},
		{"unknown age", time.Time{}, nil, lock, "1.4", "", "==> Using Bun version 1.4.4 (default)"},
		{"an explicit BUN_VERSION=1.4.4 wins", at("2026-11-21T00:00:00Z"), map[string]string{"BUN_VERSION": "1.4.4"}, lock, "1.4", "", "==> Using Bun 1.4.4 (from BUN_VERSION=1.4.4)"},
		{"a .bun-version line wins", at("2026-11-21T00:00:00Z"), nil, map[string]string{".bun-version": "1.3\n", "bun.lock": "{}"}, "1.3", "", ""},
		{"latest", at("2026-11-21T00:00:00Z"), map[string]string{"BUN_VERSION": "latest"}, nil, "1.4", "", ""},
		{"canary", at("2026-11-21T00:00:00Z"), map[string]string{"BUN_VERSION": "canary"}, lock, "1.4", "canary", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readFile := func(name string) ([]byte, bool) {
				v, ok := tc.files[name]
				return []byte(v), ok
			}
			choice, found, err := ResolveNativeBun(tc.created, tc.env, readFile)
			if err != nil || !found || choice.Line != tc.line || choice.Exact != tc.exact {
				t.Fatalf("= %+v, %v, %v; want line %q, exact %q", choice, found, err, tc.line, tc.exact)
			}
			if reviewed, _ := nativeBun.find(tc.line); choice.Image != reviewed.image {
				t.Errorf("image = %q, want the reviewed %s line's %q", choice.Image, tc.line, reviewed.image)
			}
			if tc.log != "" && choice.Narration() != tc.log+bunDocs {
				t.Errorf("narration = %q, want %q", choice.Narration(), tc.log+bunDocs)
			}
		})
	}
}

// w8/371: the resolve phase installs a lockfile-only service's dated default
// Bun exactly, pinned by digest and named in the log; a missing tag keeps the
// reviewed patch and says so.
func TestResolveNativeBuildPinsTheDatedDefaultBun(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "node"
	created := time.Date(2026, time.November, 21, 8, 59, 23, 0, time.UTC)
	exact := &fakeExact{digests: map[string]string{bunRepo + ":1.4.5": exactDigest}}
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n", "bun.lock": "{}"})
	choices, err := ResolveNativeBuildExact(t.Context(), exact, created, "node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[1].Line != "1.4.5" || choices[1].Image != "oven/bun:1.4.5@"+exactDigest {
		t.Fatalf("choices = %+v, %v; want Bun 1.4.5 pinned to its digest", choices, err)
	}
	// The lockfile names no version, so Render's line says its default (w9/139).
	if got := choices[1].Narration(); got != "==> Using Bun version 1.4.5 (default)"+bunDocs {
		t.Errorf("narration = %q", got)
	}
	if df, _ := os.ReadFile(dockerfile); !strings.Contains(string(df), "COPY --link --from=oven/bun:1.4.5@"+exactDigest+" /usr/local/bin/bun") {
		t.Fatalf("Dockerfile does not copy the dated default Bun:\n%s", df)
	}

	bundle, dockerfile, src = writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n", "bun.lock": "{}"})
	choices, err = ResolveNativeBuildExact(t.Context(), &fakeExact{}, created, "node", bundle, dockerfile, src)
	reviewed, _ := nativeBun.find("1.4")
	if err != nil || len(choices) != 2 || choices[1].Image != reviewed.image ||
		choices[1].Narration() != "==> Using Bun version 1.4.4 (default)"+bunDocs+"\n==> No official image oven/bun:1.4.5; using the reviewed Bun 1.4 image" {
		t.Fatalf("missing tag choices = %+v, %v; want the reviewed 1.4.4, said", choices, err)
	}

	// Render's 1.1.0 for a service created before 2025-08-18 is on no reviewed
	// line (w8/372): installed exactly, and as a default it falls back.
	old := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n", "bun.lock": "{}"})
	choices, err = ResolveNativeBuildExact(t.Context(), &fakeExact{digests: map[string]string{bunRepo + ":1.1.0": exactDigest}}, old, "node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[1].Image != "oven/bun:1.1.0@"+exactDigest || choices[1].Narration() != "==> Using Bun version 1.1.0 (default)"+bunDocs {
		t.Fatalf("pre-2025-08-18 choices = %+v, %v; want Bun 1.1.0 pinned to its digest", choices, err)
	}
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n", "bun.lock": "{}"})
	choices, err = ResolveNativeBuildExact(t.Context(), &fakeExact{}, old, "node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[1].Image != reviewed.image ||
		choices[1].Narration() != "==> Using Bun version 1.4.4 (default)"+bunDocs+"\n==> No official image oven/bun:1.1.0; using the reviewed Bun 1.4 image" {
		t.Fatalf("pre-2025-08-18 missing tag choices = %+v, %v; want the reviewed 1.4.4, said", choices, err)
	}
}

// Every native Python build carries Poetry (w8/m72, Render's Poetry version
// contract): the default release, or POETRY_VERSION's exact one; anything
// else is refused by name before the build runs.
func TestResolveNativePoetry(t *testing.T) {
	for _, tc := range []struct {
		name, request, version, source, wantErr string
	}{
		{"default", "", "2.1.3", "", ""},
		{"sweep307 override", "2.1.3", "2.1.3", "POETRY_VERSION=2.1.3", ""},
		{"older 1.x", "1.7.1", "1.7.1", "POETRY_VERSION=1.7.1", ""},
		{"v prefix", " v2.0.1 ", "2.0.1", "POETRY_VERSION=v2.0.1", ""},
		{"range", ">=2.0", "", "", "POETRY_VERSION=>=2.0 is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as 2.1.3"},
		{"major only", "2", "", "", "POETRY_VERSION=2 is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as 2.1.3"},
		{"shell text", "2.1.3; curl x", "", "", "POETRY_VERSION=2.1.3; curl x is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as 2.1.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			choice, err := ResolveNativePoetry(map[string]string{"POETRY_VERSION": tc.request, "PYTHON_VERSION": "3.12"})
			if tc.wantErr != "" {
				var unsupported *UnsupportedVersionError
				if !errors.As(err, &unsupported) || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || choice.Label != "Poetry" || choice.Line != tc.version || choice.Source != tc.source {
				t.Fatalf("= %+v, %v; want %s from %q", choice, err, tc.version, tc.source)
			}
		})
	}
	choice, _ := ResolveNativePoetry(map[string]string{"POETRY_VERSION": "2.1.3"})
	if got := choice.Narration(); got != "==> Using Poetry 2.1.3 (from POETRY_VERSION=2.1.3)"+poetryDocs {
		t.Errorf("narration = %q", got)
	}
}

// The Poetry install lands between the selected Python's FROM and the
// source/build, on every Python line; a refused request leaves the
// Dockerfile alone; other runtimes never gain it.
func TestResolveNativeBuildAddsPoetryToPython(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	for _, line := range nativeToolchains["python"].lines {
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": line.line, "POETRY_VERSION": "2.1.3"}, nil)
		choices, err := resolveNativeBuild("python", bundle, dockerfile, src)
		if err != nil || len(choices) != 3 || choices[0].Line != line.line || choices[1].Line != "2.1.3" {
			t.Fatalf("python %s choices = %+v, %v", line.line, choices, err)
		}
		df, _ := os.ReadFile(dockerfile)
		from := strings.Index(string(df), "\nFROM "+expectedNativeBase(line.image)+"\n")
		poetry := strings.Index(string(df), "\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && ")
		source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
		if from < 0 || poetry < from || source < poetry || build < source || !strings.Contains(string(df), "poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry\n") {
			t.Fatalf("python %s: Poetry is not installed between FROM and the source/build:\n%s", line.line, df)
		}
	}

	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"POETRY_VERSION": "latest"}, nil)
	before, _ := os.ReadFile(dockerfile)
	var unsupported *UnsupportedVersionError
	if _, err := resolveNativeBuild("python", bundle, dockerfile, src); !errors.As(err, &unsupported) {
		t.Fatalf("unsupported POETRY_VERSION = %v, want a tenant UnsupportedVersionError", err)
	}
	if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
		t.Error("a refused Poetry request rewrote the Dockerfile")
	}

	o.Runtime = "node"
	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"POETRY_VERSION": "2.1.3"}, nil)
	if choices, err := resolveNativeBuild("node", bundle, dockerfile, src); err != nil || len(choices) != 1 {
		t.Fatalf("node choices = %+v, %v; Poetry is Python's", choices, err)
	}
	if df, _ := os.ReadFile(dockerfile); strings.Contains(string(df), "poetry") {
		t.Fatalf("node build gained Poetry:\n%s", df)
	}
}

// Every non-Node native build carries Render's JavaScript tools (w8/m77
// Ruby/Python, w4/m213 Go/Rust/Elixir): the selected Node line's node/npm,
// Yarn and pnpm land between the primary FROM and the source/build on every
// primary line, NODE_VERSION picks the Node line, a Node request bex refuses
// takes the default Node (w8/400), and primary Node gains no second copy.
func TestResolveNativeBuildAddsJSToolsToNonNodeRuntimes(t *testing.T) {
	for _, runtime := range []string{"ruby", "python", "go", "rust", "elixir"} {
		o := nativeOptions()
		o.Runtime = runtime
		for _, line := range nativeToolchains[runtime].lines {
			env := map[string]string{}
			files := map[string]string{}
			switch runtime {
			case "python":
				env["PYTHON_VERSION"] = line.line
			case "elixir":
				env["ELIXIR_VERSION"] = line.version
			case "ruby":
				files[".ruby-version"] = line.version + "\n"
			} // Go and Rust each have one reviewed line: no signal selects it
			bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
			choices, err := resolveNativeBuild(runtime, bundle, dockerfile, src)
			if err != nil || choices[0].Line != line.line || choices[len(choices)-1].Label != "Node" || choices[len(choices)-1].Line != "24" {
				t.Fatalf("%s %s choices = %+v, %v", runtime, line.line, choices, err)
			}
			df, _ := os.ReadFile(dockerfile)
			from := strings.Index(string(df), "\nFROM "+expectedNativeBase(line.image)+"\n")
			tools := strings.Index(string(df), "\n"+nativeJSToolLines(nativeToolchains["node"].defaultImage(), nativeToolchains["node"].defaultLine))
			source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
			if from < 0 || tools < from || source < tools || build < source {
				t.Fatalf("%s %s: JS tools are not between FROM and the source/build:\n%s", runtime, line.line, df)
			}
		}

		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": "22"}, nil)
		choices, err := resolveNativeBuild(runtime, bundle, dockerfile, src)
		node22, _ := nativeToolchains["node"].find("22")
		if df, _ := os.ReadFile(dockerfile); err != nil || choices[len(choices)-1].Source != "NODE_VERSION=22" ||
			!strings.Contains(string(df), "COPY --link --from="+node22.image+" /usr/local/bin/node /usr/local/bin/node\n") {
			t.Fatalf("%s NODE_VERSION=22: %+v, %v\n%s", runtime, choices, err, df)
		}

		// A Node request bex refuses builds on the default Node with a line
		// naming it, where it once failed the build (w8/400).
		bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"NODE_VERSION": "12"}, nil)
		choices, err = resolveNativeBuild(runtime, bundle, dockerfile, src)
		if df, _ := os.ReadFile(dockerfile); err != nil || choices[len(choices)-1].Source != "" ||
			!strings.Contains(choices[len(choices)-1].Narration(), "==> NODE_VERSION=12 is not a supported Node version;") ||
			!strings.Contains(string(df), nativeJSToolLines(nativeToolchains["node"].defaultImage(), nativeToolchains["node"].defaultLine)) {
			t.Fatalf("%s NODE_VERSION=12: %+v, %v\n%s", runtime, choices, err, df)
		}
	}
	o := nativeOptions()
	o.Runtime = "node"
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	if _, err := resolveNativeBuild("node", bundle, dockerfile, src); err != nil {
		t.Fatal(err)
	}
	if df, _ := os.ReadFile(dockerfile); strings.Contains(string(df), " /usr/local/bin/node /usr/local/bin/node\n") {
		t.Fatalf("primary Node gained a second node binary:\n%s", df)
	}
}

// w8/174: Render picks a Python service's default by its creation date, so a
// service created after the cutoff defaults to 3.14, an older or unknown one
// stays on 3.13, and any explicit signal still wins.
func TestPythonDefaultFollowsCreationDate(t *testing.T) {
	after := toolchains.PythonCreationCutoff.Add(time.Hour)
	before := toolchains.PythonCreationCutoff.Add(-time.Hour)
	for _, tc := range []struct {
		name    string
		created time.Time
		env     map[string]string
		line    string
	}{
		{"created after the cutoff", after, nil, "3.14"},
		{"created exactly at the cutoff", toolchains.PythonCreationCutoff, nil, "3.14"},
		{"created before the cutoff", before, nil, "3.13"},
		{"unknown creation keeps the legacy default", time.Time{}, nil, "3.13"},
		{"explicit signal wins on a new service", after, map[string]string{"PYTHON_VERSION": "3.12"}, "3.12"},
	} {
		got, err := ResolveNativeVersionAt(tc.created, "python", tc.env, files(nil), packageJSON)
		if err != nil || got.Line != tc.line {
			t.Errorf("%s: line = %q, err = %v, want %q", tc.name, got.Line, err, tc.line)
		}
	}
	// Other runtimes ignore the creation date.
	if got, _ := ResolveNativeVersionAt(after, "node", nil, files(nil), packageJSON); got.Line != "24" {
		t.Errorf("node default = %q", got.Line)
	}
}

func TestNativeResolverCarriesServiceCreationTime(t *testing.T) {
	o := nativeInitOptions()
	o.ServiceCreatedAt = time.Date(2026, 10, 23, 1, 2, 3, 0, time.UTC)
	c := nativeVersionResolver(o)
	for _, e := range c.Env {
		if e.Name == NativeServiceCreatedEnv && e.Value == "2026-10-23T01:02:03Z" {
			return
		}
	}
	t.Errorf("resolver env = %v, want %s", c.Env, NativeServiceCreatedEnv)
}

// w8/373: Render sets Elixir and Erlang/OTP independently and builds any
// compatible pair from Elixir 1.12.0 and Erlang/OTP 24.3.4
// (render.com/docs/elixir-erlang-versions). A release no line carries is
// installed exactly, on the newest OTP major it supports unless ERLANG_VERSION
// names one; a requested OTP the line's own image does not carry is installed
// exactly; one it does carry keeps the line's image.
func TestElixirOTPPairs(t *testing.T) {
	created := renderElixirDefault20250612.Add(time.Hour)
	for _, tc := range []struct {
		env                         map[string]string
		line, exact, erlang, source string
		unreviewed                  bool
	}{
		{map[string]string{"ELIXIR_VERSION": "1.16.3"}, "1.18", "1.16.3", "26", "ELIXIR_VERSION=1.16.3", true},
		{map[string]string{"ELIXIR_VERSION": "1.16"}, "1.18", "1.16.3", "26", "ELIXIR_VERSION=1.16", true},
		{map[string]string{"ELIXIR_VERSION": "v1.12.0"}, "1.18", "1.12.0", "24", "ELIXIR_VERSION=v1.12.0", true},
		{map[string]string{"ELIXIR_VERSION": "1.13.3"}, "1.18", "1.13.3", "24", "ELIXIR_VERSION=1.13.3", true},
		{map[string]string{"ELIXIR_VERSION": "1.13.4"}, "1.18", "1.13.4", "25", "ELIXIR_VERSION=1.13.4", true},
		{map[string]string{"ELIXIR_VERSION": "1.14.4"}, "1.18", "1.14.4", "25", "ELIXIR_VERSION=1.14.4", true},
		{map[string]string{"ELIXIR_VERSION": "1.14"}, "1.18", "1.14.5", "26", "ELIXIR_VERSION=1.14", true},
		{map[string]string{"ELIXIR_VERSION": "1.15.8", "ERLANG_VERSION": "24.3"}, "1.18", "1.15.8", "24.3", "ELIXIR_VERSION=1.15.8, ERLANG_VERSION=24.3", true},
		{map[string]string{"ELIXIR_VERSION": "1.16.3", "ERLANG_VERSION": "26"}, "1.18", "1.16.3", "26", "ELIXIR_VERSION=1.16.3, ERLANG_VERSION=26", true},
		{map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "27.3"}, "1.18", "1.18.4", "27.3", "ELIXIR_VERSION=1.18.4, ERLANG_VERSION=27.3", false},
		{map[string]string{"ELIXIR_VERSION": "1.18", "ERLANG_VERSION": "26"}, "1.18", "", "26", "ELIXIR_VERSION=1.18, ERLANG_VERSION=26", false},
		{map[string]string{"ELIXIR_VERSION": "1.17.3", "ERLANG_VERSION": "25.3"}, "1.17", "", "25.3", "ELIXIR_VERSION=1.17.3, ERLANG_VERSION=25.3", false},
		{map[string]string{"ERLANG_VERSION": "27.3"}, "1.18", "1.18.4", "27.3", "ERLANG_VERSION=27.3", false},
		// The line's own major keeps the line's image; a finer release of it
		// is installed exactly (w8/197).
		{map[string]string{"ELIXIR_VERSION": "1.19", "ERLANG_VERSION": "28"}, "1.19", "", "", "ELIXIR_VERSION=1.19, ERLANG_VERSION=28", false},
		{map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3"}, "1.19", "1.19.5", "28.3", "ELIXIR_VERSION=1.19.5, ERLANG_VERSION=28.3", false},
		{map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "28"}, "1.18", "1.18.4", "", "ELIXIR_VERSION=1.18.4, ERLANG_VERSION=28", false},
	} {
		choice, err := ResolveNativeVersionAt(created, nativeElixirRuntime, tc.env, files(nil), packageJSON)
		if err != nil {
			t.Errorf("%v: %v", tc.env, err)
			continue
		}
		if choice.Line != tc.line || choice.Exact != tc.exact || choice.erlang != tc.erlang || choice.Source != tc.source || choice.unreviewed != tc.unreviewed {
			t.Errorf("%v: line %q exact %q erlang %q source %q unreviewed %v, want %q %q %q %q %v", tc.env,
				choice.Line, choice.Exact, choice.erlang, choice.Source, choice.unreviewed, tc.line, tc.exact, tc.erlang, tc.source, tc.unreviewed)
		}
		if line, _ := nativeToolchains[nativeElixirRuntime].find(tc.line); choice.Image != line.image {
			t.Errorf("%v: starts on %s, want the %s line's image", tc.env, choice.Image, tc.line)
		}
	}
}

// TestElixirOTPTable keeps Elixir's compatibility table consistent with the
// reviewed lines and Render's minimum.
func TestElixirOTPTable(t *testing.T) {
	if first := elixirOTP[0].minor + ".0"; first != elixirMinimum {
		t.Errorf("table starts at %s, Render's minimum is %s", first, elixirMinimum)
	}
	for i, row := range elixirOTP {
		if i > 0 && !semver.MustParse(elixirOTP[i-1].minor+".0").LessThan(semver.MustParse(row.minor+".0")) {
			t.Errorf("rows out of order at %s", row.minor)
		}
		if row.lo > row.hi || row.hi < otpParts(erlangMinimum)[0] {
			t.Errorf("%s supports no Erlang/OTP from Render's minimum: %d to %d", row.minor, row.lo, row.hi)
		}
		_, onLine := nativeToolchains[nativeElixirRuntime].find(row.minor)
		if onLine == (row.final != "") || row.final != "" && !strings.HasPrefix(row.final, row.minor+".") {
			t.Errorf("%s: a final release belongs to exactly the minors no line carries, got %q", row.minor, row.final)
		}
	}
	for _, line := range nativeToolchains[nativeElixirRuntime].lines {
		lo, hi, ok := elixirOTPMajors(line.version)
		if otp, _ := strconv.Atoi(line.otp); !ok || otp < lo || otp > hi {
			t.Errorf("line %s's image pairs Elixir %s with Erlang/OTP %s, outside the table's %d to %d", line.line, line.version, line.otp, lo, hi)
		}
	}
}
