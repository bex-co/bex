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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSourceChecks executes the clone script's post-checkout path checks (the
// real script text) against a fake checkout rooted at dir.
func runSourceChecks(t *testing.T, o Options, dir string) (string, int) {
	t.Helper()
	c := buildCloneContainer(o, "git")
	script := c.Args[0]
	start := strings.Index(script, "\ncommit=") + 1
	end := strings.Index(script, "rm -rf .git")
	if start < 1 || end < start {
		t.Fatalf("clone script lost its path checks:\n%s", script)
	}
	check := strings.Replace(script[start:end], `"$(git rev-parse HEAD)"`, `"e88c7e33"`, 1)
	cmd := exec.Command("sh", "-eu", "-c", check)
	for _, e := range c.Env {
		if strings.HasPrefix(e.Name, "BEX_") {
			cmd.Env = append(cmd.Env, e.Name+"="+strings.Replace(e.Value, sourceMount, dir, 1))
		}
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	}
	return strings.TrimSpace(string(out)), code
}

// w8/053: a missing root directory, Docker context or Dockerfile fails the
// clone step as tenant input in the repository's own terms, not BuildKit's
// "invalid local: resolve : lstat /source/…".
func TestCloneRefusesMissingSourcePaths(t *testing.T) {
	dir := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(dir, "examples", "app"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "examples", "app", "Dockerfile"), []byte("FROM x\n"), 0o644))

	ok := []Options{
		{Builder: BuilderDockerfile, RootDir: "examples/app"},
		{Builder: BuilderNative, RootDir: "examples/app"},
		{Builder: BuilderDockerfile, RootDir: "examples/app", DockerContext: "examples"},
		{Builder: BuilderNative},
	}
	for _, o := range ok {
		if out, code := runSourceChecks(t, o, dir); code != 0 {
			t.Errorf("%+v refused: %d %s", o, code, out)
		}
	}
	refused := []struct {
		o    Options
		want string
	}{
		{Options{Builder: BuilderNative, RootDir: "examples/no-such-dir"}, `the root directory "examples/no-such-dir" does not exist in the repository at "e88c7e33"`},
		{Options{Builder: BuilderDockerfile, RootDir: "examples/app", DockerContext: "nope"}, `the Docker build context "nope" does not exist`},
		// A Dockerfile path that names a directory keeps bex's sentence: Render's
		// log for it is only quoted with its path elided (w8/404).
		{Options{Builder: BuilderDockerfile, RootDir: "examples", DockerfilePath: "app"}, `the Dockerfile "examples/app" does not exist`},
		// Traversal stays inside the checkout: "../../etc" is the repo's "etc".
		{Options{Builder: BuilderNative, RootDir: "../../etc"}, `the root directory "etc" does not exist`},
	}
	for _, tc := range refused {
		out, code := runSourceChecks(t, tc.o, dir)
		if code != ExitTenantError || !strings.Contains(out, tc.want) || strings.Contains(out, "/source") {
			t.Errorf("%+v: exit %d %q, want ExitTenantError with %q", tc.o, code, out, tc.want)
		}
	}
	// Buildpack builds carry no Dockerfile and are never checked for one.
	for _, e := range sourcePathChecks(Options{Builder: BuilderBuildpack}) {
		if e.Name == "BEX_DOCKERFILE" {
			t.Error("buildpack build checks for a Dockerfile")
		}
	}
}

// w8/404: a missing Dockerfile fails the clone step as tenant input with the
// lines Render's BuildKit logs for it (frayel/devagent#6, 2026-09-26), naming
// the path as configured, so `./Dockerfile` reads `Dockerfile`
// (Numbu-bit/TroSky#13). Render's log for a nested path is not observed; bex
// names it as configured too.
func TestCloneFailsAMissingDockerfileInRendersLines(t *testing.T) {
	dir := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(dir, "examples", "app"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "examples", "app", "Dockerfile"), []byte("FROM x\n"), 0o644))
	renders := func(name string) string {
		return "#1 [internal] load build definition from " + name + "\n" +
			"#1 transferring dockerfile: 2B done\n" +
			"#1 DONE 0.0s\n" +
			"error: failed to solve: failed to read dockerfile: open " + name + ": no such file or directory"
	}
	for _, tc := range []struct {
		o    Options
		name string
	}{
		{Options{Builder: BuilderDockerfile, RootDir: "examples"}, "Dockerfile"},
		{Options{Builder: BuilderDockerfile, RootDir: "examples", DockerfilePath: "./Dockerfile"}, "Dockerfile"},
		{Options{Builder: BuilderDockerfile, RootDir: "examples/app", DockerfilePath: "Dockerfile.prod"}, "Dockerfile.prod"},
		{Options{Builder: BuilderDockerfile, RootDir: "examples/app", DockerfilePath: "./nope/Dockerfile"}, "nope/Dockerfile"},
		{Options{Builder: BuilderDockerfile, DockerfilePath: "../../Nope"}, "Nope"},
	} {
		out, code := runSourceChecks(t, tc.o, dir)
		if code != ExitTenantError || out != renders(tc.name) {
			t.Errorf("rootDir %q dockerfilePath %q: exit %d\n%s\nwant ExitTenantError with\n%s", tc.o.RootDir, tc.o.DockerfilePath, code, out, renders(tc.name))
		}
		// The failure reason carries the same lines, BuildKit's verdict last.
		if tail := failureTail(buildkitFailureSummary(out)); tail != renders(tc.name) {
			t.Errorf("rootDir %q dockerfilePath %q: failure tail %q", tc.o.RootDir, tc.o.DockerfilePath, tail)
		}
	}
	for _, o := range []Options{
		{Builder: BuilderDockerfile, RootDir: "examples/app"},
		{Builder: BuilderDockerfile, RootDir: "examples", DockerfilePath: "./app/Dockerfile"},
	} {
		if out, code := runSourceChecks(t, o, dir); code != 0 {
			t.Errorf("rootDir %q dockerfilePath %q refused a Dockerfile that exists: %d %s", o.RootDir, o.DockerfilePath, code, out)
		}
	}
}

// A root directory, build context or Dockerfile directory that is a link out of
// the checkout would hand the build container's own filesystem to the build (a
// Dockerfile's `COPY . /app`, a native build's working directory). The clone
// step runs for every build, Dockerfile or native and with or without secret
// files, so it is where the repository's links are refused as the tenant's own
// error; a link that stays inside the checkout is the repository's business.
func TestCloneRefusesALinkThatLeavesTheRepository(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "checkout")
	outside := filepath.Join(parent, "outside")
	must(t, os.MkdirAll(filepath.Join(dir, "app"), 0o755))
	must(t, os.MkdirAll(outside, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "app", "Dockerfile"), []byte("FROM x\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(outside, "Dockerfile"), []byte("FROM x\n"), 0o644))
	must(t, os.Symlink("../outside", filepath.Join(dir, "relative-out")))
	must(t, os.Symlink(outside, filepath.Join(dir, "absolute-out")))
	must(t, os.Symlink("app", filepath.Join(dir, "inside")))
	must(t, os.MkdirAll(filepath.Join(dir, "nest"), 0o755))
	must(t, os.Symlink("../../outside", filepath.Join(dir, "nest", "deeper")))

	for _, o := range []Options{
		{Builder: BuilderDockerfile, RootDir: "inside"},
		{Builder: BuilderNative, RootDir: "inside"},
		{Builder: BuilderDockerfile, RootDir: "app", DockerContext: "inside"},
	} {
		if out, code := runSourceChecks(t, o, dir); code != 0 {
			t.Errorf("%+v refused a link inside the repository: %d %s", o, code, out)
		}
	}
	for _, tc := range []struct {
		o    Options
		want string
	}{
		{Options{Builder: BuilderDockerfile, RootDir: "relative-out"}, `the root directory "relative-out" is a link that leaves the repository`},
		{Options{Builder: BuilderNative, RootDir: "absolute-out"}, `the root directory "absolute-out" is a link that leaves the repository`},
		{Options{Builder: BuilderDockerfile, RootDir: "app", DockerContext: "relative-out"}, `the Docker build context "relative-out" is a link that leaves the repository`},
		{Options{Builder: BuilderDockerfile, RootDir: "nest/deeper"}, `the root directory "nest/deeper" is a link that leaves the repository`},
	} {
		out, code := runSourceChecks(t, tc.o, dir)
		if code != ExitTenantError || !strings.Contains(out, tc.want) || strings.Contains(out, dir) {
			t.Errorf("%+v: exit %d %q, want ExitTenantError with %q", tc.o, code, out, tc.want)
		}
	}
}

// w4/467: the clone fetch is quiet, so git's "From …" and "* branch … ->
// FETCH_HEAD" lines never lead a build failure's reason; errors still reach
// stderr and the classifier.
func TestCloneFetchIsQuiet(t *testing.T) {
	c := buildCloneContainer(Options{Repo: "https://example.invalid/r.git", Ref: "main"}, "git-image")
	script := strings.Join(c.Args, "\n")
	if !strings.Contains(script, `fetch -q --depth 1 origin -- "$REF"`) {
		t.Fatalf("clone fetch is not quiet:\n%s", script)
	}
}
