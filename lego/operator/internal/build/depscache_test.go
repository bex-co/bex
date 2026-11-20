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
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// depsOpts is a native build of runtime that keeps its dependency
// directories between builds (w9/m494).
func depsOpts(runtime string) Options {
	o := xdgOpts()
	o.Runtime = runtime
	return o
}

// loaderOf is the env loader the recipe df writes before the build RUN.
func loaderOf(t *testing.T, df string) string {
	t.Helper()
	open := "COPY <<'" + nativeEnvLoaderEOF + "' " + nativeEnvLoaderPath + "\n"
	at := strings.Index(df, open)
	if at < 0 {
		t.Fatalf("no env loader:\n%s", df)
	}
	body := df[at+len(open):]
	return body[:strings.Index(body, "\n"+nativeEnvLoaderEOF+"\n")+1]
}

// buildRunLine is the recipe's build command RUN line.
func buildRunLine(t *testing.T, df string) (line, after string) {
	t.Helper()
	at := strings.Index(df, "\n"+nativeEnvRunMount)
	if at < 0 {
		t.Fatalf("no build RUN:\n%s", df)
	}
	line, after, _ = strings.Cut(df[at+1:], "\n")
	return line, after
}

// --- the recipe ---

func TestNativeRecipeCarriesEachRuntimesDependencyDirectories(t *testing.T) {
	want := map[string][]string{
		"ruby":   {"/opt/render/project/.gems/ruby"},
		"go":     {"/opt/render/project/go/pkg"},
		"rust":   {"/opt/render/project/.cargo/registry", "/opt/render/project/.cargo/git", "/opt/render/project/src/target"},
		"elixir": {"/opt/render/project/src/deps", "/opt/render/project/src/_build"},
	}
	staticRuby := depsOpts("ruby")
	staticRuby.StaticSite, staticRuby.StartCommand, staticRuby.StaticPublishPath = true, "", "_site"
	cases := map[string]Options{"static ruby": staticRuby}
	for runtime := range nativeToolchains {
		cases[runtime] = depsOpts(runtime)
	}
	for name, o := range cases {
		dirs := want[nativeRuntime(o)]
		if got := o.depsCache(); !slices.Equal(got, dirs) {
			t.Errorf("%s: carries %v, want %v", name, got, dirs)
			continue
		}
		df := nativeDockerfile(o)
		run, after := buildRunLine(t, df)
		loader := loaderOf(t, df)
		if len(dirs) == 0 {
			for _, token := range []string{nativeDepsCacheDir, nativeDepsCacheID, nativeDepsSaveEOF} {
				if strings.Contains(df, token) {
					t.Errorf("%s: a runtime with no dependency directories carries %q:\n%s", name, token, df)
				}
			}
			continue
		}
		// The build command's RUN mounts the cache beside $XDG_CACHE_HOME's.
		if !strings.Contains(run, nativeXDGCacheMount+nativeDepsCacheMount+" [") {
			t.Errorf("%s: the build RUN does not mount the dependency cache: %s", name, run)
		}
		// The loader moves them into place first, before any variable of the
		// service is exported.
		seed := nativeDepsSeed(dirs)
		if !strings.HasPrefix(loader, ": bex-native-env-rev=") || !strings.Contains(loader, "\n"+seed) ||
			strings.Index(loader, seed) > strings.Index(loader, "while IFS= read") {
			t.Errorf("%s: the loader does not seed before decoding:\n%s", name, loader)
		}
		for _, dir := range dirs {
			if !strings.Contains(seed, " '"+dir+"'") {
				t.Errorf("%s: the seed leaves %s out:\n%s", name, dir, seed)
			}
		}
		// The save step follows the command's RUN at once, and only there.
		if !strings.HasPrefix(after, nativeDepsSave(dirs)) || strings.Count(df, nativeDepsSaveEOF) != 2 {
			t.Errorf("%s: the save step does not follow the build command:\n%s", name, df)
		}
	}

	// Without the S3 store the mount would start empty and end discarded:
	// the recipe carries nothing of it.
	for label, o := range map[string]Options{"registry cache": func() Options {
		o := depsOpts("ruby")
		o.CacheStore = CacheStore{}
		return o
	}(), "cache off": func() Options {
		o := depsOpts("ruby")
		o.BuildCache = false
		return o
	}(), "no App UID": func() Options {
		o := depsOpts("ruby")
		o.AppUID = ""
		return o
	}()} {
		if o.depsCache() != nil || strings.Contains(nativeDockerfile(o), nativeDepsCacheDir) {
			t.Errorf("%s: carries the dependency cache", label)
		}
	}
}

// The dependency directories' archive is keyed by what comes before the
// source copy, so the build command's own RUN text, the service's variables
// and its source never move the key.
func TestDependencyKeyIsWrittenAheadOfTheSource(t *testing.T) {
	a, b := depsOpts("ruby"), depsOpts("ruby")
	b.BuildCommand, b.NativeEnvRevision = "bundle install && rake assets:precompile", "rev-2"
	prefix := func(df string) string { return df[:strings.Index(df, "\nCOPY . .\n")] }
	if dfA, dfB := nativeDockerfile(a), nativeDockerfile(b); prefix(dfA) != prefix(dfB) || dfA == dfB {
		t.Error("the build command or the service's variables reach the toolchain's part of the recipe")
	}
	if prefix(nativeDockerfile(a)) == prefix(nativeDockerfile(depsOpts("go"))) {
		t.Error("two runtimes share a toolchain key")
	}
}

// --- the loader's seed and the save step, run as the build runs them ---

// depsRig retargets the cache mount at dir/mount; the directories a test
// passes are under dir, so each sits in the mount at its own absolute path.
type depsRig struct {
	t          *testing.T
	dir, mount string
}

func newDepsRig(t *testing.T) *depsRig {
	t.Helper()
	dir := t.TempDir()
	r := &depsRig{t: t, dir: dir, mount: filepath.Join(dir, "mount")}
	if err := os.MkdirAll(r.mount, 0o755); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *depsRig) path(rel string) string { return filepath.Join(r.dir, rel) }

func (r *depsRig) cached(rel string) string { return r.mount + r.path(rel) }

// seed sources the loader's seed in bash, as the build RUN does, and returns
// what it printed.
func (r *depsRig) seed(rels ...string) string {
	r.t.Helper()
	dirs := make([]string, len(rels))
	for i, rel := range rels {
		dirs[i] = r.path(rel)
	}
	script := strings.ReplaceAll(nativeDepsSeed(dirs), nativeDepsCacheDir, r.mount) + "echo \"after $?\"\n"
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		r.t.Fatalf("seed: %v\n%s", err, out)
	}
	return string(out)
}

// save runs the save step's heredoc under sh with bound in place of the
// archive's, and returns what it printed.
func (r *depsRig) save(bound int64, rels ...string) string {
	r.t.Helper()
	dirs := make([]string, len(rels))
	for i, rel := range rels {
		dirs[i] = r.path(rel)
	}
	step := nativeDepsSave(dirs)
	body := step[strings.Index(step, "\n")+1 : strings.LastIndex(step, nativeDepsSaveEOF)]
	body = strings.NewReplacer(nativeDepsCacheDir, r.mount, xdgMaxBytes, strconv.FormatInt(bound, 10)).Replace(body)
	out, err := exec.Command("sh", "-c", body).CombinedOutput()
	if err != nil {
		r.t.Fatalf("the save step failed the build: %v\n%s", err, out)
	}
	return string(out)
}

func readOrEmpty(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func TestDependencySeedFillsOnlyWhatTheBuildDidNotBring(t *testing.T) {
	gnuUserland(t)
	r := newDepsRig(t)
	for _, rel := range []string{"gems", "src/target", "src/deps", "src/_build", "src/link"} {
		writeFile(t, filepath.Join(r.cached(rel), "cached"), "previous build")
	}
	writeFile(t, r.cached("src/target/release/.fingerprint/dep"), "fp")
	// The source brought its own deps, the image an empty _build, and a link
	// stands where another directory would go.
	writeFile(t, r.path("src/deps/vendored"), "from the repository")
	if err := os.MkdirAll(r.path("src/_build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/elsewhere", r.path("src/link")); err != nil {
		t.Fatal(err)
	}
	if out := r.seed("gems", "src/target", "src/deps", "src/_build", "src/link", "absent"); out != "after 0\n" {
		t.Errorf("the seed printed into the build log or left a status: %q", out)
	}
	for rel, want := range map[string]string{
		"gems/cached":                         "previous build",
		"src/target/cached":                   "previous build",
		"src/target/release/.fingerprint/dep": "fp",
		"src/_build/cached":                   "previous build",
		"src/deps/vendored":                   "from the repository",
		"src/deps/cached":                     "",
	} {
		if got := readOrEmpty(r.path(rel)); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if target, err := os.Readlink(r.path("src/link")); err != nil || target != "/elsewhere" {
		t.Errorf("the link was replaced: %q %v", target, err)
	}
	if entries, _ := os.ReadDir(r.mount); len(entries) != 0 {
		t.Errorf("the mount still holds %v: the build would pay its disk twice", entries)
	}
	for _, rel := range []string{"gems", "src/target"} {
		if _, err := os.Stat(r.path(rel) + ".bex-seed"); err == nil {
			t.Errorf("%s's staging copy was left behind", rel)
		}
	}
}

func TestDependencySeedWithAnEmptyCacheChangesNothing(t *testing.T) {
	gnuUserland(t)
	r := newDepsRig(t)
	if out := r.seed("gems"); out != "after 0\n" {
		t.Errorf("%q", out)
	}
	if _, err := os.Stat(r.path("gems")); err == nil {
		t.Error("an empty cache made a directory")
	}
}

func TestDependencySaveCopiesWhatTheBuildLeft(t *testing.T) {
	gnuUserland(t)
	r := newDepsRig(t)
	writeFile(t, r.path("gems/ruby/3.4.0/gems/rack/lib/rack.rb"), "rack")
	if err := os.Symlink("rack.rb", r.path("gems/ruby/3.4.0/gems/rack/lib/alias.rb")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(r.path("gems/ruby/3.4.0/gems/rack/lib"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(r.path("gems/ruby/3.4.0/gems/rack/lib"), 0o755)
		_ = os.Chmod(r.cached("gems/ruby/3.4.0/gems/rack/lib"), 0o755)
	})
	if err := os.Symlink("/etc", r.path("go")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, r.cached("stale/left/by/another/runtime"), "stale")
	if out := r.save(xdgCacheMaxBytes, "gems", "go", "absent"); out != "" {
		t.Errorf("the save step printed %q", out)
	}
	if got := readOrEmpty(r.cached("gems/ruby/3.4.0/gems/rack/lib/rack.rb")); got != "rack" {
		t.Errorf("saved gem = %q", got)
	}
	if target, err := os.Readlink(r.cached("gems/ruby/3.4.0/gems/rack/lib/alias.rb")); err != nil || target != "rack.rb" {
		t.Errorf("a link inside was not kept as a link: %q %v", target, err)
	}
	if fi, err := os.Stat(r.cached("gems/ruby/3.4.0/gems/rack/lib")); err != nil || fi.Mode().Perm() != 0o555 {
		t.Errorf("modes were not kept: %v %v", fi, err)
	}
	if _, err := os.Lstat(r.cached("go")); err == nil {
		t.Error("a directory that is a link was saved")
	}
	if _, err := os.Stat(r.cached("stale")); err == nil {
		t.Error("what another build left in the mount was archived again")
	}
	if got := readOrEmpty(r.path("gems/ruby/3.4.0/gems/rack/lib/rack.rb")); got != "rack" {
		t.Error("the save step changed the image's own copy")
	}
}

func TestDependencySaveKeepsNothingOverTheBound(t *testing.T) {
	gnuUserland(t)
	r := newDepsRig(t)
	writeFile(t, r.path("src/target/big"), strings.Repeat("x", 64<<10))
	writeFile(t, r.cached("old"), "previous")
	if out := r.save(32<<10, "src/target"); out != "" {
		t.Errorf("%q", out)
	}
	if entries, _ := os.ReadDir(r.mount); len(entries) != 0 {
		t.Errorf("a cache over the bound was kept: %v", entries)
	}
}

// --- the whole round trip: build, save, export, seed, next build ---

// depsPipeline is one build pod's cache steps over the same two mounts:
// dir/cache/{xdg,deps} as the seed and export solves mount them, the deps
// mount also where the build's own RUN does.
type depsPipeline struct {
	*depsRig
	xdg string
}

func newDepsPipeline(t *testing.T) *depsPipeline {
	t.Helper()
	r := newDepsRig(t)
	xdg := archiveMounts(t, r.dir)
	if err := os.Remove(r.mount); err != nil {
		t.Fatal(err)
	}
	r.mount = filepath.Join(r.dir, "cache", xdgArchiveDeps)
	return &depsPipeline{depsRig: r, xdg: xdg}
}

// pod starts the next build pod: its mounts and the image's directories are
// new, and the archive the last one exported is what cache-restore fetched.
func (p *depsPipeline) pod(rels ...string) {
	p.t.Helper()
	for _, rel := range slices.Concat(rels, []string{"in", "cache"}) {
		if err := os.RemoveAll(p.path(rel)); err != nil {
			p.t.Fatal(err)
		}
	}
	if _, err := os.Stat(p.path("out")); err == nil {
		if err := os.Rename(p.path("out"), p.path("in")); err != nil {
			p.t.Fatal(err)
		}
	}
	archiveMounts(p.t, p.dir)
}

func (p *depsPipeline) export(key string) {
	p.t.Helper()
	runStep(p.t, xdgStep(p.t, "BEX_XDG_SAVE", p.dir, xdgCacheMaxBytes), "BEX_DEPS_KEY="+key)
}

func (p *depsPipeline) seedSolve(key string) string {
	p.t.Helper()
	return runStep(p.t, xdgStep(p.t, "BEX_XDG_SEED", p.dir, xdgCacheMaxBytes), "BEX_DEPS_KEY="+key)
}

func TestDependencyCacheCrossesBuildsOnlyOnTheSameToolchain(t *testing.T) {
	gnuUserland(t)
	const ruby34, ruby33 = "aaaa-x86_64", "bbbb-x86_64"
	dirs := []string{"gems", "src/target"}
	p := newDepsPipeline(t)

	// Build 1, cold: the command installs, the save step and the export run.
	writeFile(t, p.path("gems/ruby/3.4.0/gems/rack/lib/rack.rb"), "rack 3")
	writeFile(t, p.path("src/target/release/deps/libserde.rlib"), "serde")
	writeFile(t, filepath.Join(p.xdg, "pip", "wheel"), "wheel")
	p.save(xdgCacheMaxBytes, dirs...)
	p.export(ruby34)
	if _, err := os.Stat(p.path("out/" + xdgCacheObject)); err != nil {
		t.Fatalf("build 1 exported no archive: %v", err)
	}

	// Build 2, a source-only commit: the same toolchain gets everything back.
	p.pod(dirs...)
	if out := p.seedSolve(ruby34); !strings.Contains(out, "restored in") {
		t.Fatalf("seed: %s", out)
	}
	p.seed(dirs...)
	for rel, want := range map[string]string{
		"gems/ruby/3.4.0/gems/rack/lib/rack.rb": "rack 3",
		"src/target/release/deps/libserde.rlib": "serde",
	} {
		if got := readOrEmpty(p.path(rel)); got != want {
			t.Errorf("build 2: %s = %q, want %q", rel, got, want)
		}
	}
	if got := readOrEmpty(filepath.Join(p.xdg, "pip", "wheel")); got != "wheel" {
		t.Errorf("build 2: $XDG_CACHE_HOME lost its wheel: %q", got)
	}
	p.save(xdgCacheMaxBytes, dirs...)
	p.export(ruby34)

	// Build 3, Ruby 3.3: the dependencies are dropped, $XDG_CACHE_HOME stays.
	p.pod(dirs...)
	if out := p.seedSolve(ruby33); !strings.Contains(out, "restored in") {
		t.Fatalf("seed: %s", out)
	}
	if entries, _ := os.ReadDir(p.mount); len(entries) != 0 {
		t.Errorf("another toolchain's dependencies were seeded: %v", entries)
	}
	if got := readOrEmpty(filepath.Join(p.xdg, "pip", "wheel")); got != "wheel" {
		t.Errorf("a toolchain change dropped $XDG_CACHE_HOME too: %q", got)
	}
	p.seed(dirs...)
	if _, err := os.Stat(p.path("gems")); err == nil {
		t.Error("build 3 started with build 2's gems")
	}

	// An archive that names no toolchain seeds none, whatever this build's
	// key: nor does a build with no key keep any.
	unkeyed := tarGz(t, []tar.Header{{Name: "deps/kept", Typeflag: tar.TypeReg}}, map[string]string{"deps/kept": "?"})
	for _, key := range []string{"", ruby33} {
		p.pod(dirs...)
		writeFile(t, p.path("in/"+xdgCacheObject), string(unkeyed))
		p.seedSolve(key)
		if entries, _ := os.ReadDir(p.mount); len(entries) != 0 {
			t.Errorf("key %q: an archive naming no toolchain seeded %v", key, entries)
		}
	}

	// A build with no key keeps no dependencies either way.
	writeFile(t, p.path("gems/ruby/3.3.0/gems/rack/lib/rack.rb"), "rack 2")
	writeFile(t, filepath.Join(p.xdg, "pip", "wheel"), "wheel")
	p.save(xdgCacheMaxBytes, dirs...)
	p.export("")
	for _, member := range archiveMembers(t, p.path("out/"+xdgCacheObject)) {
		if strings.HasPrefix(member, xdgArchiveDeps) {
			t.Errorf("an export with no key archived %s", member)
		}
	}
	p.pod(dirs...)
	p.seedSolve(ruby33)
	if entries, _ := os.ReadDir(p.mount); len(entries) != 0 {
		t.Errorf("an unkeyed archive seeded dependencies: %v", entries)
	}
}

// archiveMembers is the names an exported archive holds.
func archiveMembers(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tr := tar.NewReader(gz); ; {
		h, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
}

func TestDependencyCacheKeepsNothingOverTheSharedBound(t *testing.T) {
	gnuUserland(t)
	p := newDepsPipeline(t)
	writeFile(t, filepath.Join(p.xdg, "big"), strings.Repeat("x", 48<<10))
	writeFile(t, filepath.Join(p.mount, "deps-big"), strings.Repeat("y", 48<<10))
	// Either half fits the bound alone; together they are past it.
	runStep(t, xdgStep(t, "BEX_XDG_SAVE", p.dir, 64<<10), "BEX_DEPS_KEY=k")
	if _, err := os.Stat(p.path("out/" + xdgCacheObject)); err == nil {
		t.Error("an archive over the shared bound was kept")
	}
	if readOrEmpty(p.path("out/skipped")) == "" {
		t.Error("no skipped marker")
	}
}

func TestDependencyCacheAloneIsStillArchived(t *testing.T) {
	gnuUserland(t)
	p := newDepsPipeline(t)
	writeFile(t, filepath.Join(p.mount, p.path("gems"), "g"), "gem")
	p.export("k")
	if _, err := os.Stat(p.path("out/" + xdgCacheObject)); err != nil {
		t.Fatalf("a build whose $XDG_CACHE_HOME is empty kept no dependencies: %v", err)
	}
	p.pod()
	p.seedSolve("k")
	if got := readOrEmpty(filepath.Join(p.mount, p.path("gems"), "g")); got != "gem" {
		t.Errorf("seeded %q", got)
	}
}

// --- the wrapper's key ---

func TestXDGBuildctlKeysTheDependenciesToTheToolchain(t *testing.T) {
	head := "# syntax=docker/dockerfile:1.7\nFROM " + wrapperBase + "\nCOPY --link --from=ruby:3.4@sha256:aa /usr/local/ /usr/local/\nWORKDIR /opt/render/project/src\nCOPY . .\n"
	keyOf := func(t *testing.T, dockerfile string) string {
		t.Helper()
		r := newWrapperRig(t, dockerfile)
		writeFile(t, filepath.Join(r.xin, xdgCacheObject), "archive")
		writeFile(t, filepath.Join(r.xin, "transfer"), "0 7 1\n")
		if out, err := r.run(wrapperBuildArgs); err != nil {
			t.Fatalf("wrapper: %v\n%s", err, out)
		}
		var keys []string
		for _, call := range r.calls() {
			if target := callTarget(call); target == "bex-xdg-seed" || target == "bex-xdg-export" {
				_, rest, ok := strings.Cut(call, "--opt build-arg:BEX_DEPS_KEY=")
				if !ok {
					t.Fatalf("%s carries no key: %s", target, call)
				}
				keys = append(keys, strings.Fields(rest)[0])
			}
		}
		if len(keys) != 2 || keys[0] != keys[1] {
			t.Fatalf("seed and export keys %v", keys)
		}
		return keys[0]
	}
	key := keyOf(t, head+"RUN bundle install\n")
	sum := sha256.Sum256([]byte(head))
	machine, err := exec.Command("uname", "-m").Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := hex.EncodeToString(sum[:]) + "-" + strings.TrimSpace(string(machine)); key != want {
		t.Errorf("key = %q, want the toolchain part's sha256 and the machine %q", key, want)
	}
	if other := keyOf(t, head+"COPY <<'X' /opt/bex/load-env\n: rev-2\nX\nRUN rake\n"); other != key {
		t.Error("a change after the source copy moved the key")
	}
	if other := keyOf(t, strings.Replace(head, "ruby:3.4@sha256:aa", "ruby:3.3@sha256:bb", 1)+"RUN bundle install\n"); other == key {
		t.Error("another Ruby kept the key")
	}

	// No hash, no key: the seed then keeps no dependencies and the export
	// archives none, rather than every toolchain sharing "-<machine>".
	r := newWrapperRig(t, head)
	writeFile(t, filepath.Join(r.xin, xdgCacheObject), "archive")
	if err := os.WriteFile(filepath.Join(r.bin, "sha256sum"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := r.run(wrapperBuildArgs); err != nil {
		t.Fatalf("a failed hash failed the build: %v\n%s", err, out)
	}
	for _, call := range r.calls() {
		if target := callTarget(call); (target == "bex-xdg-seed" || target == "bex-xdg-export") && !strings.Contains(call, "--opt build-arg:BEX_DEPS_KEY= ") {
			t.Errorf("%s carries a key without a hash: %s", target, call)
		}
	}
}

// --- the build log ---

func TestNativeBuildLogKeepsTheDependencySaveStepOut(t *testing.T) {
	command := "bundle install"
	save := "#24 [stage-0 12/13] RUN --network=none" + nativeDepsCacheMount + " <<'" + nativeDepsSaveEOF + "' (find /opt/bex/deps-cache -mindepth 1 -delete...)\n"
	for name, step := range map[string]string{"ran": save + "#24 DONE 1.4s\n", "cached": save + "#24 CACHED\n"} {
		transcript := strings.Replace(nativeTranscript(command, ""), "#24 exporting", step+"#25 exporting", 1)
		log, _, exit := runBuildkit(t, nativeLogOptions(command), transcript, 0)
		plain, _, _ := runBuildkit(t, nativeLogOptions(command), nativeTranscript(command, ""), 0)
		if exit != 0 || log != plain {
			t.Errorf("%s: the save step reached the log (exit %d):\n%s", name, exit, log)
		}
	}
	// A failure header naming both cache mounts still names only the command.
	header := `> [stage-0 11/13] ` + nativeEnvRunMount + nativeXDGCacheMount + nativeDepsCacheMount + ` ["/bin/bash","-c",". /opt/bex/load-env\nbundle install"]:`
	if got, _ := nativeCommandFailure(header); got != "> [stage-0 11/13] RUN build command 'bundle install':" {
		t.Errorf("error header = %q", got)
	}
}
