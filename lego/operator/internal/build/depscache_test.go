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
	"maps"
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
		"python": {"/opt/render/project/src/.venv"},
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

// Python's virtualenv is made before the source copy, so the recipe records
// it as made in the same RUN, where the next build's seed can tell it from a
// repository's own. A build that keeps no dependencies records nothing, and
// its recipe is as it was.
func TestPythonRecipeRecordsTheVirtualenvItMakes(t *testing.T) {
	const venvRun = "RUN python3 -m venv /opt/render/project/src/.venv"
	o := depsOpts("python")
	df := nativeDockerfile(o)
	line := venvRun + " && " + nativeDepsRecord(nativePythonVenv) + "\n"
	if at := strings.Index(df, "\n"+line); strings.Count(df, line) != 1 || at < 0 || at > strings.Index(df, "\nCOPY . .\n") {
		t.Errorf("the virtualenv is not recorded once, as it is made, before the source copy:\n%s", df)
	}
	if want := "mkdir -p '/opt/bex/deps-made/opt/render/project/src' && find '/opt/render/project/src/.venv' "; !strings.HasPrefix(nativeDepsRecord(nativePythonVenv), want) ||
		!strings.HasSuffix(nativeDepsRecord(nativePythonVenv), " > '/opt/bex/deps-made/opt/render/project/src/.venv'") {
		t.Errorf("record = %s", nativeDepsRecord(nativePythonVenv))
	}
	off := depsOpts("python")
	off.CacheStore = CacheStore{}
	if df := nativeDockerfile(off); !strings.Contains(df, "\n"+venvRun+"\n") || strings.Contains(df, nativeDepsMadeDir) {
		t.Errorf("a build that keeps no dependencies records its virtualenv:\n%s", df)
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

// depsRig retargets the cache mount at dir/mount and the recipe's records at
// dir/made; the directories a test passes are under dir, so each sits in the
// mount and among the records at its own absolute path.
type depsRig struct {
	t                *testing.T
	dir, mount, made string
	// env is added to the environment the seed runs in.
	env []string
}

func newDepsRig(t *testing.T) *depsRig {
	t.Helper()
	dir := t.TempDir()
	r := &depsRig{t: t, dir: dir, mount: filepath.Join(dir, "mount"), made: filepath.Join(dir, "made")}
	if err := os.MkdirAll(r.mount, 0o755); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *depsRig) path(rel string) string { return filepath.Join(r.dir, rel) }

func (r *depsRig) cached(rel string) string { return r.mount + r.path(rel) }

// retarget points a script's mount and records at the rig's.
func (r *depsRig) retarget(script string) string {
	return strings.NewReplacer(nativeDepsCacheDir, r.mount, nativeDepsMadeDir, r.made).Replace(script)
}

// seed sources the loader's seed in bash, as the build RUN does, and returns
// what it printed.
func (r *depsRig) seed(rels ...string) string {
	r.t.Helper()
	dirs := make([]string, len(rels))
	for i, rel := range rels {
		dirs[i] = r.path(rel)
	}
	cmd := exec.Command("bash", "-c", r.retarget(nativeDepsSeed(dirs))+"echo \"after $?\"\n")
	cmd.Env = append(os.Environ(), r.env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("seed: %v\n%s", err, out)
	}
	return string(out)
}

// recordVenv runs the recipe's record of the virtualenv at src/.venv under
// sh, as its RUN does.
func (r *depsRig) recordVenv() {
	r.t.Helper()
	if out, err := exec.Command("sh", "-c", r.retarget(nativeDepsRecord(r.path("src/.venv")))).CombinedOutput(); err != nil {
		r.t.Fatalf("record: %v\n%s", err, out)
	}
}

// pythonCfg is a virtualenv's pyvenv.cfg as `python3 -m venv` writes it on
// the Python of version.
func pythonCfg(version string) string {
	return "home = /usr/local/bin\ninclude-system-site-packages = false\nversion = " + version +
		"\nexecutable = /usr/local/bin/python3.13\ncommand = /usr/local/bin/python3 -m venv /opt/render/project/src/.venv\n"
}

// makeVenv lays out a virtualenv at dir as `python3 -m venv` leaves one:
// pyvenv.cfg, the interpreter's links, pip in site-packages and lib64.
func makeVenv(t *testing.T, dir, version string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, nativeVenvConfig), pythonCfg(version))
	writeFile(t, filepath.Join(dir, "bin", "pip"), "#!"+dir+"/bin/python\n")
	writeFile(t, filepath.Join(dir, "lib", "python3.13", "site-packages", "pip", "__init__.py"), "__version__ = '25.2'\n")
	for link, target := range map[string]string{"bin/python3": "/usr/local/bin/python3", "bin/python": "python3", "lib64": "lib"} {
		if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}
}

// tree is every entry under dir, dir itself included, by its path: a
// directory, a file's bytes or a link's target. A link is not followed.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		switch {
		case d.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			entries[rel] = "-> " + target
			return err
		case d.IsDir():
			entries[rel] = "dir"
		default:
			body, err := os.ReadFile(path)
			entries[rel] = string(body)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// installed is the packages a virtualenv at dir holds, beside pip.
func installed(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(dir, "lib", "python3.13", "site-packages"))
	var names []string
	for _, e := range entries {
		if e.Name() != "pip" {
			names = append(names, e.Name())
		}
	}
	return names
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

// relayer stands for the toolchain layer's trip through the layer cache to
// the next build: the directory is unpacked anew, with every time changed.
func relayer(t *testing.T, dir string) {
	t.Helper()
	trip := dir + ".trip"
	must(t, os.Mkdir(trip, 0o755))
	if out, err := exec.Command("sh", "-c", `tar -C "$1" -cf - . | tar -C "$2" -xf - --touch`, "sh", dir, trip).CombinedOutput(); err != nil {
		t.Fatalf("relayer: %v\n%s", err, out)
	}
	must(t, os.RemoveAll(dir))
	must(t, os.Rename(trip, dir))
}

// A source-only commit's build finds the recipe's fresh virtualenv where the
// kept one belongs: the seed swaps the whole directory for the kept one.
func TestDependencySeedReplacesTheRecipesVirtualenvWithTheKeptOne(t *testing.T) {
	gnuUserland(t)
	r := newDepsRig(t)
	venv := r.path("src/.venv")
	makeVenv(t, venv, "3.13.18")
	writeFile(t, filepath.Join(venv, "fresh-only"), "the recipe's")
	r.recordVenv()
	relayer(t, venv)
	kept := r.cached("src/.venv")
	makeVenv(t, kept, "3.13.18")
	writeFile(t, filepath.Join(kept, "lib", "python3.13", "site-packages", "flask", "__init__.py"), "flask")
	writeFile(t, filepath.Join(kept, "bin", "flask"), "#!"+venv+"/bin/python\n")

	if out := r.seed("src/.venv"); out != "after 0\n" {
		t.Errorf("the seed printed into the build log or left a status: %q", out)
	}
	if got := installed(t, venv); !slices.Equal(got, []string{"flask"}) {
		t.Errorf("the build's virtualenv holds %v, want the kept flask", got)
	}
	if _, err := os.Lstat(filepath.Join(venv, "fresh-only")); err == nil {
		t.Error("the kept virtualenv was merged into the recipe's instead of replacing it")
	}
	if target, err := os.Readlink(filepath.Join(venv, "bin", "python3")); err != nil || target != "/usr/local/bin/python3" {
		t.Errorf("the interpreter's link = %q %v", target, err)
	}
	for _, staging := range []string{venv + ".bex-seed", venv + ".bex-made"} {
		if _, err := os.Lstat(staging); err == nil {
			t.Errorf("%s was left behind", staging)
		}
	}
	if entries, _ := os.ReadDir(r.mount); len(entries) != 0 {
		t.Errorf("the mount still holds %v", entries)
	}
}

// The recipe's virtualenv gives way only while it is as the recipe made it
// and the kept one names the same interpreter: a repository's own .venv, which
// the source copy laid over the recipe's, wins, and so does the recipe's when
// the kept one was made by another Python.
func TestDependencySeedKeepsAVirtualenvTheBuildBrought(t *testing.T) {
	gnuUserland(t)
	for name, change := range map[string]func(t *testing.T, r *depsRig, venv, kept string){
		"the repository commits packages": func(t *testing.T, _ *depsRig, venv, _ string) {
			writeFile(t, filepath.Join(venv, "lib", "python3.13", "site-packages", "vendored", "__init__.py"), "from the repository")
		},
		"the repository commits a pip of its own": func(t *testing.T, _ *depsRig, venv, _ string) {
			writeFile(t, filepath.Join(venv, "bin", "pip"), "#!/usr/bin/env python3\n")
		},
		"the repository commits its own pyvenv.cfg": func(t *testing.T, _ *depsRig, venv, _ string) {
			writeFile(t, filepath.Join(venv, nativeVenvConfig), strings.Replace(pythonCfg("3.13.18"), "/usr/local/bin\n", "/usr/locai/bin\n", 1))
		},
		"the repository commits another interpreter link": func(t *testing.T, _ *depsRig, venv, _ string) {
			must(t, os.Remove(filepath.Join(venv, "bin", "python3")))
			must(t, os.Symlink("/usr/bin/python3", filepath.Join(venv, "bin", "python3")))
		},
		"the repository commits a .venv link": func(t *testing.T, _ *depsRig, venv, _ string) {
			must(t, os.Rename(venv, venv+".real"))
			must(t, os.Symlink(".venv.real", venv))
		},
		"the kept one was made by another Python": func(t *testing.T, _ *depsRig, _, kept string) {
			writeFile(t, filepath.Join(kept, nativeVenvConfig), pythonCfg("3.12.11"))
		},
		"the recipe recorded nothing": func(t *testing.T, r *depsRig, _, _ string) {
			must(t, os.RemoveAll(r.made))
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := newDepsRig(t)
			venv, kept := r.path("src/.venv"), r.cached("src/.venv")
			makeVenv(t, venv, "3.13.18")
			r.recordVenv()
			makeVenv(t, kept, "3.13.18")
			writeFile(t, filepath.Join(kept, "lib", "python3.13", "site-packages", "flask", "__init__.py"), "flask")
			change(t, r, venv, kept)
			before := tree(t, venv)
			r.seed("src/.venv")
			if after := tree(t, venv); !maps.Equal(after, before) {
				t.Errorf("the build's virtualenv changed:\n%v\nwant\n%v", after, before)
			}
			if entries, _ := os.ReadDir(r.mount); len(entries) != 0 {
				t.Errorf("the mount still holds %v", entries)
			}
		})
	}
}

// A swap that cannot finish leaves the recipe's virtualenv where it was: the
// build then installs into it from nothing, as a first build does.
func TestDependencySeedPutsTheRecipesVirtualenvBackWhenTheSwapFails(t *testing.T) {
	gnuUserland(t)
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	r := newDepsRig(t)
	bin := r.path("bin")
	// mv refuses only the last rename, the staged copy into place.
	writeFile(t, filepath.Join(bin, "mv"), "#!/bin/sh\ncase $1 in *.bex-seed) case $2 in *.bex-seed) ;; *) exit 1 ;; esac ;; esac\nexec "+realMv+" \"$@\"\n")
	must(t, os.Chmod(filepath.Join(bin, "mv"), 0o755))
	r.env = []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	venv := r.path("src/.venv")
	makeVenv(t, venv, "3.13.18")
	writeFile(t, filepath.Join(venv, "fresh-only"), "the recipe's")
	r.recordVenv()
	makeVenv(t, r.cached("src/.venv"), "3.13.18")
	writeFile(t, filepath.Join(r.cached("src/.venv"), "lib", "python3.13", "site-packages", "flask", "__init__.py"), "flask")

	if out := r.seed("src/.venv"); out != "after 0\n" {
		t.Errorf("a failed swap failed the build: %q", out)
	}
	if got := readOrEmpty(filepath.Join(venv, "fresh-only")); got != "the recipe's" {
		t.Errorf("the recipe's virtualenv was not put back: %q", got)
	}
	if got := installed(t, venv); len(got) != 0 {
		t.Errorf("half a swap: %v", got)
	}
	for _, staging := range []string{venv + ".bex-seed", venv + ".bex-made"} {
		if _, err := os.Lstat(staging); err == nil {
			t.Errorf("%s was left behind", staging)
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

// The round trip for Python: a cold build installs into the recipe's
// virtualenv and keeps it, a source-only commit's build gets it back in the
// recipe's place, and a build on another Python starts from the recipe's.
func TestDependencyCacheCarriesThePythonVirtualenvAcrossBuilds(t *testing.T) {
	gnuUserland(t)
	const python313, python312 = "cccc-aarch64", "dddd-aarch64"
	p := newDepsPipeline(t)
	venv := p.path("src/.venv")
	// recipe is the toolchain layer's fresh virtualenv, as the next build
	// imports it.
	recipe := func(version string) {
		makeVenv(t, venv, version)
		p.recordVenv()
		relayer(t, venv)
	}
	pipInstall := func(packages ...string) {
		for _, pkg := range packages {
			writeFile(t, filepath.Join(venv, "lib", "python3.13", "site-packages", pkg, "__init__.py"), pkg)
		}
	}

	// Build 1, cold: pip installs into the recipe's virtualenv.
	recipe("3.13.18")
	p.seed("src/.venv")
	pipInstall("flask", "gunicorn")
	p.save(xdgCacheMaxBytes, "src/.venv")
	p.export(python313)

	// Build 2, a source-only commit: every package is there already.
	p.pod("src/.venv", "made")
	recipe("3.13.18")
	if out := p.seedSolve(python313); !strings.Contains(out, "restored in") {
		t.Fatalf("seed: %s", out)
	}
	p.seed("src/.venv")
	if got := installed(t, venv); !slices.Equal(got, []string{"flask", "gunicorn"}) {
		t.Errorf("build 2 found %v, want flask and gunicorn installed", got)
	}
	p.save(xdgCacheMaxBytes, "src/.venv")
	p.export(python313)

	// Build 3, Python 3.12: another image, so another key.
	p.pod("src/.venv", "made")
	recipe("3.12.11")
	p.seedSolve(python312)
	p.seed("src/.venv")
	if got := installed(t, venv); len(got) != 0 {
		t.Errorf("build 3 on another Python started with %v", got)
	}
	if got := readOrEmpty(filepath.Join(venv, nativeVenvConfig)); got != pythonCfg("3.12.11") {
		t.Errorf("build 3's virtualenv is not its recipe's: %q", got)
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
