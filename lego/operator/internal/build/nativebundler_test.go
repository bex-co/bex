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
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGem stands in for RubyGems: it logs each call, says whether Bundler
// $FAKE_GEM_HAS is installed, and installs with $FAKE_GEM_RC.
const fakeGem = `#!/bin/sh
echo "gem $*" >> "$FAKE_GEM_LOG"
case $1 in
list) [ "$5" = "$FAKE_GEM_HAS" ] && echo true && exit 0; echo false; exit 1 ;;
install) echo "ERROR:  Could not find a valid gem 'bundler' ($4)"; exit "${FAKE_GEM_RC:-0}" ;;
esac
`

// runLockedBundler sources the step as the env loader does, in a service
// root holding files, under a Ruby build's GEM_PATH, then env, and a PATH
// holding only the fake gem: the step must parse with shell builtins alone.
// It returns the output and the fake's calls.
func runLockedBundler(t *testing.T, files map[string]string, env ...string) (out string, calls []string) {
	t.Helper()
	dir := t.TempDir()
	root, bin, log := filepath.Join(dir, "src"), filepath.Join(dir, "bin"), filepath.Join(dir, "calls")
	for name, body := range files {
		writeFile(t, filepath.Join(root, name), body)
	}
	must(t, os.MkdirAll(root, 0o755))
	writeFile(t, filepath.Join(bin, "gem"), fakeGem)
	must(t, os.Chmod(filepath.Join(bin, "gem"), 0o755))
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable")
	}
	// The loader's PATH is the service's; what the step leaves in the shell
	// is printed after it.
	script := nativeLockedBundler + `echo "after PATH=$PATH"; declare -F bex_locked_bundler; declare -p want lock 2>/dev/null; true`
	cmd := exec.Command(bash, "-c", script)
	cmd.Dir = root
	cmd.Env = append([]string{"PATH=" + bin, "FAKE_GEM_LOG=" + log, "GEM_PATH=" + nativeRubyGems}, env...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("step: %v\n%s", err, raw)
	}
	out = string(raw)
	if !strings.HasSuffix(out, "after PATH="+bin+"\n") {
		t.Fatalf("the step changed the shell it ran in:\n%s", out)
	}
	logged, _ := os.ReadFile(log)
	if s := strings.TrimSpace(string(logged)); s != "" {
		calls = strings.Split(s, "\n")
	}
	return strings.TrimSuffix(out, "after PATH="+bin+"\n"), calls
}

// lockedWith is a minimal lockfile written by Bundler version.
func lockedWith(version string) string {
	return "GEM\n  remote: https://rubygems.org/\n  specs:\n    rack (3.2.9)\n\nPLATFORMS\n  ruby\n\n" +
		"DEPENDENCIES\n  rack\n\nBUNDLED WITH\n   " + version + "\n"
}

// w9/128: before the build command, the Bundler the lockfile names is
// installed where GEM_PATH reads it, unless RubyGems already has it, and the
// build log says which Bundler the build uses.
func TestLockedBundlerIsInstalledWhereGemPathReadsIt(t *testing.T) {
	gemfile := "source \"https://rubygems.org\"\ngem \"rack\"\n"
	list := func(v string) string { return "gem list -i ^bundler$ -v " + v }
	installInto := func(v, dir string) string {
		return "gem install bundler -v " + v + " --install-dir " + dir + " --no-document"
	}
	install := func(v string) string { return installInto(v, "/opt/render/project/.gems") }
	for _, tc := range []struct {
		name  string
		files map[string]string
		env   []string
		out   string
		calls []string
	}{
		{name: "newer than the Ruby's own", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")},
			env: []string{"FAKE_GEM_HAS=2.5.22"},
			out: "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9"), install("2.6.9")}},
		{name: "already installed", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")},
			env: []string{"FAKE_GEM_HAS=2.6.9"},
			out: "==> Using Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9")}},
		{name: "older than the Ruby's own", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.4.22")},
			env: []string{"FAKE_GEM_HAS=2.5.22"},
			out: "==> Installing Bundler 2.4.22 (from Gemfile.lock)\n", calls: []string{list("2.4.22"), install("2.4.22")}},
		{name: "written on Windows", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": strings.ReplaceAll(lockedWith("2.6.9"), "\n", "\r\n")},
			out: "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9"), install("2.6.9")}},
		{name: "no final newline", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": strings.TrimSuffix(lockedWith("2.6.9"), "\n")},
			out: "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9"), install("2.6.9")}},
		// Bundler's own order: gems.rb before Gemfile, each with its lock.
		{name: "gems.rb", files: map[string]string{"gems.rb": gemfile, "gems.locked": lockedWith("2.6.9"), "Gemfile": gemfile, "Gemfile.lock": lockedWith("2.5.6")},
			out: "==> Installing Bundler 2.6.9 (from gems.locked)\n", calls: []string{list("2.6.9"), install("2.6.9")}},
		{name: "BUNDLE_GEMFILE", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.5.6"), "app/Gemfile": gemfile, "app/Gemfile.lock": lockedWith("2.6.9")},
			env: []string{"BUNDLE_GEMFILE=app/Gemfile"},
			out: "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9"), install("2.6.9")}},
		// RubyGems searches GEM_PATH: its first directory, a service's own
		// included, or RubyGems' own where there is none (a static site's
		// Ruby).
		{name: "a GEM_PATH of its own", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")},
			env: []string{"GEM_PATH=/srv/gems:/opt/render/project/.gems"},
			out: "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9"), installInto("2.6.9", "/srv/gems")}},
		{name: "no GEM_PATH", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")},
			env: []string{"GEM_PATH="},
			out: "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n", calls: []string{list("2.6.9"), "gem install bundler -v 2.6.9 --no-document"}},
		{name: "install fails", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")},
			env:   []string{"FAKE_GEM_RC=2"},
			out:   "==> Installing Bundler 2.6.9 (from Gemfile.lock)\n==> Could not install Bundler 2.6.9:\nERROR:  Could not find a valid gem 'bundler' (2.6.9)\n",
			calls: []string{list("2.6.9"), install("2.6.9")}},
		// Where Bundler itself would not switch to the lockfile's version,
		// or there is none to switch to, nothing is installed or said.
		{name: "BUNDLER_VERSION", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")}, env: []string{"BUNDLER_VERSION=2.5.22"}},
		{name: "BUNDLE_VERSION=system", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9")}, env: []string{"BUNDLE_VERSION=system"}},
		{name: "no lockfile", files: map[string]string{"Gemfile": gemfile}},
		{name: "a lockfile without a Gemfile", files: map[string]string{"Gemfile.lock": lockedWith("2.6.9")}},
		{name: "no BUNDLED WITH", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": strings.Split(lockedWith("2.6.9"), "BUNDLED")[0]}},
		{name: "a development Bundler", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.7.0.dev")}},
		{name: "not a version", files: map[string]string{"Gemfile": gemfile, "Gemfile.lock": lockedWith("2.6.9; touch pwned")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, calls := runLockedBundler(t, tc.files, tc.env...)
			if out != tc.out || strings.Join(calls, "\n") != strings.Join(tc.calls, "\n") {
				t.Errorf("printed %q and ran %q, want %q and %q", out, calls, tc.out, tc.calls)
			}
		})
	}
}

// The step is the env loader's last part on every Ruby build, a static
// Ruby site's included, and on a static site whose dependencies bex installs
// (its Gemfile gets a Ruby): after the service's variables are exported (so a
// BUNDLE_GEMFILE of its own counts), after the dependency seed (so a kept
// Bundler counts) and before the dependency install. No other build carries
// it.
func TestLockedBundlerStepEndsARubyBuildsLoader(t *testing.T) {
	staticRuby := depsOpts(nativeRubyRuntime)
	staticRuby.StaticSite, staticRuby.StartCommand, staticRuby.StaticPublishPath = true, "", "_site"
	staticNode := nativeOptions()
	staticNode.StaticSite, staticNode.Runtime, staticNode.StartCommand, staticNode.StaticPublishPath = true, "", "", "_site"
	staticRoot := staticNode
	staticRoot.StaticPublishPath = "./"
	cases := map[string]Options{"ruby without the dependency cache": func() Options {
		o := nativeOptions()
		o.Runtime = nativeRubyRuntime
		return o
	}(), "static ruby": staticRuby, "static node": staticNode, "static node publishing its root": staticRoot}
	for runtime := range nativeToolchains {
		cases[runtime] = depsOpts(runtime)
	}
	for name, o := range cases {
		df := nativeDockerfile(o)
		loader := loaderOf(t, df)
		if nativeRuntime(o) != nativeRubyRuntime && !nativeStaticInstalls(o) {
			if strings.Contains(loader, "bex_locked_bundler") {
				t.Errorf("%s: carries the Bundler step:\n%s", name, loader)
			}
			continue
		}
		if !strings.HasSuffix(loader, nativeLockedBundler) || strings.Index(loader, nativeLockedBundler) < strings.Index(loader, "done < "+nativeEnvSecretPath) {
			t.Errorf("%s: the Bundler step does not end the loader:\n%s", name, loader)
		}
	}
}

// bundlerFixture is the w9/128 drill's service: bcrypt, puma and rack locked
// by Bundler 2.6.9, newer than the 2.5.22 Ruby 3.3 ships.
var bundlerFixture = map[string]string{
	".ruby-version": "3.3\n",
	"Gemfile":       "source \"https://rubygems.org\"\ngem \"bcrypt\"\ngem \"puma\"\ngem \"rack\"\n",
	"Gemfile.lock": `GEM
  remote: https://rubygems.org/
  specs:
    bcrypt (3.1.22)
    nio4r (2.7.5)
    puma (8.1.2)
      nio4r (~> 2.0)
    rack (3.2.9)

PLATFORMS
  aarch64-linux
  ruby
  x86_64-linux

DEPENDENCIES
  bcrypt
  puma
  rack

BUNDLED WITH
   2.6.9
`,
	"app.rb":    "require \"bcrypt\"\nrequire \"rack\"\nputs \"app bundler #{Bundler::VERSION} rack #{Rack.release}\"\n",
	"config.ru": "run ->(_env) { [200, {\"content-type\" => \"text/plain\"}, [\"bundler #{Bundler::VERSION}\"]] }\n",
}

// TestNativeRubyImageStartsOnTheLockedBundler is w9/128's execution gate: a
// Ruby 3.3 service whose lockfile names Bundler 2.6.9 builds, and its
// `bundle exec ruby app.rb` and `bundle exec puma` start on that Bundler, on
// a cold build and on the rebuild after a source-only commit that the
// dependency cache serves. Opt-in: needs Docker and network for RubyGems.
func TestNativeRubyImageStartsOnTheLockedBundler(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := s3CacheOpts()
	o.Builder, o.Runtime, o.BuildEnv = BuilderNative, nativeRubyRuntime, nil
	o.BuildCommand = "bundle install"
	o.StartCommand = "bundle exec ruby app.rb && exec bundle exec puma -b tcp://0.0.0.0:$PORT"
	// The recipe's cache mount id is the build pod's own; on the test's
	// shared Docker daemon each run gets its own, so the first build is cold.
	cacheID := fmt.Sprintf("bex-deps-cache-it-%d", os.Getpid())
	src := maps.Clone(bundlerFixture)
	for i, want := range []string{"==> Installing Bundler 2.6.9 (from Gemfile.lock)", "==> Using Bundler 2.6.9 (from Gemfile.lock)"} {
		o.Revision = fmt.Sprintf("gen-%d", i+1)
		src["app.rb"] += "# commit " + o.Revision + "\n"
		tag := fmt.Sprintf("bex-native-bundler-locked-it-%d:%d", i, os.Getpid())
		out, err := dockerBuildResolvedWith(t, o, nil, src, tag, func(dockerfile string) string {
			return strings.ReplaceAll(dockerfile, "id="+nativeDepsCacheID+",", "id="+cacheID+",")
		})
		if err != nil {
			t.Fatalf("build %d: %v\n%s", i+1, err, out)
		}
		if !strings.Contains(string(out), want) {
			t.Fatalf("build %d log lacks %q:\n%s", i+1, want, out)
		}
		// The cold build installs every gem, the rebuild none, and neither
		// restarts Bundler.
		if installed := strings.Contains(string(out), "Installing bcrypt"); installed != (i == 0) || strings.Contains(string(out), "restarting using that version") {
			t.Fatalf("build %d: installed gems %v, want %v, or Bundler restarted:\n%s", i+1, installed, i == 0, out)
		}
		name := fmt.Sprintf("bex-native-bundler-locked-it-%d-%d", i, os.Getpid())
		body, logs := runServing(t, tag, name, "http://127.0.0.1:10000/")
		if body != "bundler 2.6.9" || !strings.Contains(logs(), "app bundler 2.6.9 rack 3.2.9") {
			t.Fatalf("build %d served %q:\n%s", i+1, body, logs())
		}
	}
}

// dockerBuildResolvedWith is dockerBuildResolved with the resolved
// Dockerfile passed through edit before the build.
func dockerBuildResolvedWith(t *testing.T, o Options, env, src map[string]string, tag string, edit func(string) string) ([]byte, error) {
	t.Helper()
	bundle, dockerfile, dir := writeResolveFixture(t, o, env, src)
	if _, err := resolveNativeBuild(nativeRuntime(o), bundle, dockerfile, dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(edit(string(raw))), 0o600))
	t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
	cmd := exec.Command("docker", "build", "--progress=plain", "-t", tag, "--secret", "id=render-env,src="+bundle, dir)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	return cmd.CombinedOutput()
}
