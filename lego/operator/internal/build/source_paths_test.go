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
		{Options{Builder: BuilderDockerfile, RootDir: "examples"}, `the Dockerfile "examples/Dockerfile" does not exist`},
		{Options{Builder: BuilderDockerfile, RootDir: "examples/app", DockerfilePath: "Dockerfile.prod"}, `the Dockerfile "examples/app/Dockerfile.prod" does not exist`},
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
