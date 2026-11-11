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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// dockercontext_test.go is w8/253. Render gives a Dockerfile build its secret
// files twice: as build secrets, and "Render also copies your secret files
// into the root of your Docker build context". bex gave the build secrets
// only, so a Dockerfile that reads an uploaded `.npmrc` with `COPY` built on
// Render and failed on bex.

func initContainerNames(spec corev1.PodSpec) []string {
	names := make([]string, len(spec.InitContainers))
	for i, c := range spec.InitContainers {
		names[i] = c.Name
	}
	return names
}

func TestDockerfileBuildCopiesSecretFilesIntoTheContext(t *testing.T) {
	o := dockerOpts()
	o.RootDir, o.DockerContext = "services/api", "services"
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	// After the clone, before BuildKit.
	names := initContainerNames(spec)
	clone, step, buildkit := slices.Index(names, "clone"), slices.Index(names, contextFilesContainer), slices.Index(names, "buildkit")
	if clone != 0 || step < 0 || step <= clone || step >= buildkit {
		t.Fatalf("init containers = %v, want %s between clone and buildkit", names, contextFilesContainer)
	}
	c := spec.InitContainers[step]

	env := map[string]string{}
	for _, e := range c.Env {
		env[e.Name] = e.Value
	}
	// The context is DockerContext, not RootDir, as buildctl's is.
	if env[dockerBuildContextEnv] != sourceMount+"/services" || env[dockerBuildFilesEnv] != ".env\ncredentials.json" {
		t.Errorf("env = %v, want the Docker context and both file names", env)
	}
	mounts := map[string]corev1.VolumeMount{}
	for _, m := range c.VolumeMounts {
		mounts[m.Name] = m
	}
	if len(mounts) != 2 || mounts["source"].ReadOnly || !mounts[secretFilesVolume].ReadOnly {
		t.Errorf("mounts = %+v, want the source writable, the secret files read-only and nothing else", c.VolumeMounts)
	}
	// Restricted, like every container but BuildKit (the bex-build Job policy).
	sc := c.SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation || sc.Capabilities == nil ||
		!slices.Contains(sc.Capabilities.Drop, corev1.Capability("ALL")) || len(sc.Capabilities.Add) != 0 ||
		sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("security context = %+v, want the restricted one", sc)
	}
	if _, named := buildStepNames[contextFilesContainer]; !named {
		t.Errorf("the step has no tenant-facing name in buildStepNames")
	}

	// BuildKit still reads the source read-only, and the build secrets still
	// come from the private snapshot, not from the context: a name listed in
	// .dockerignore keeps mounting.
	bk := buildkitContainer(t, BuildJob(o, o.ImageRef()))
	for _, m := range bk.VolumeMounts {
		if m.Name == "source" && !m.ReadOnly {
			t.Error("the BuildKit container's source mount became writable")
		}
	}
	if !slices.Contains(bk.Args, "id=_env,src="+dockerBuildInputsMount+"/files/.env") {
		t.Errorf("the .env build secret no longer reads the snapshot: %q", bk.Args)
	}
}

func TestOnlyADockerfileBuildWithSecretFilesGetsTheStep(t *testing.T) {
	without := dockerOpts()
	without.NativeFilesSecret, without.NativeFiles = "", nil
	native := opts()
	native.Builder = BuilderNative
	native.NativeFilesSecret, native.NativeFiles = "bld-hello-native-files", []string{".env"}
	for name, o := range map[string]Options{"a Dockerfile build without secret files": without, "a native build": native} {
		if names := initContainerNames(BuildJob(o, o.ImageRef()).Spec.Template.Spec); slices.Contains(names, contextFilesContainer) {
			t.Errorf("%s runs %s: %v", name, contextFilesContainer, names)
		}
	}
}

// runContextFiles runs the step's script against a checkout at source, with
// the secret files in files and the context at contextDir (inside source
// unless a test says otherwise). It returns the exit code and stderr.
func runContextFiles(t *testing.T, source, files, contextDir string, names ...string) (int, string) {
	t.Helper()
	script := strings.NewReplacer(sourceMount+"/*)", source+"/*)", dockerBuildFilesMount, files).Replace(dockerContextFilesScript())
	cmd := exec.Command("sh", "-eu", "-c", script)
	cmd.Env = append(os.Environ(), dockerBuildContextEnv+"="+contextDir, dockerBuildFilesEnv+"="+strings.Join(names, "\n"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("run the script: %v: %s", err, out)
	}
	return exit.ExitCode(), string(out)
}

// contextCheckout is a scratch checkout for the step's script: the source
// mount, the secret files' mount and a directory outside the repository.
type contextCheckout struct {
	t                      *testing.T
	source, files, outside string
}

func newContextCheckout(t *testing.T) contextCheckout {
	t.Helper()
	// EvalSymlinks: the script compares physical paths, and a temp dir may
	// itself sit behind a link.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := contextCheckout{t: t, source: filepath.Join(root, "source"), files: filepath.Join(root, "files"), outside: filepath.Join(root, "outside")}
	c.write(filepath.Join(c.files, ".npmrc"), "//registry/:_authToken=secret\n")
	c.write(filepath.Join(c.outside, "target"), "untouched\n")
	if err := os.MkdirAll(c.source, 0o755); err != nil {
		t.Fatal(err)
	}
	return c
}

func (c contextCheckout) write(path, content string) {
	c.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		c.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		c.t.Fatal(err)
	}
}

func (c contextCheckout) read(path string) string {
	c.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		c.t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestContextFilesScript(t *testing.T) {
	c := newContextCheckout(t)
	source, files, outside, write, read := c.source, c.files, c.outside, c.write, c.read
	write(filepath.Join(files, ".env"), "TOKEN=1\n")
	write(filepath.Join(files, "config"), "from-secret\n")
	write(filepath.Join(files, "linked"), "from-secret\n")
	ctx := filepath.Join(source, "app")
	write(filepath.Join(ctx, "Dockerfile"), "FROM scratch\n")
	write(filepath.Join(ctx, ".env"), "TOKEN=from-repository\n")                                       // a repository file of the name
	write(filepath.Join(ctx, "config", "keep.txt"), "repository\n")                                    // a repository directory of the name
	if err := os.Symlink(filepath.Join(outside, "target"), filepath.Join(ctx, "linked")); err != nil { // a link of the name
		t.Fatal(err)
	}

	if code, out := runContextFiles(t, source, files, ctx, ".npmrc", ".env", "config", "linked"); code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	// In the context root, and nowhere else in the checkout.
	if got := read(filepath.Join(ctx, ".npmrc")); got != "//registry/:_authToken=secret\n" {
		t.Errorf(".npmrc = %q", got)
	}
	if _, err := os.Stat(filepath.Join(source, ".npmrc")); err == nil {
		t.Error(".npmrc was also written to the repository root, outside the context")
	}
	// The secret file replaces a repository file of its name.
	if got := read(filepath.Join(ctx, ".env")); got != "TOKEN=1\n" {
		t.Errorf(".env = %q, want the secret file", got)
	}
	// A checkout's mode, so BuildKit reads it as another user.
	if info, err := os.Stat(filepath.Join(ctx, ".npmrc")); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf(".npmrc mode = %v (err %v), want 0644", info.Mode().Perm(), err)
	}
	// A repository directory of the name is kept, with nothing put in it.
	if got := read(filepath.Join(ctx, "config", "keep.txt")); got != "repository\n" {
		t.Errorf("the repository's config directory changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(ctx, "config", "config")); err == nil {
		t.Error("the secret file was copied into the repository's directory of its name")
	}
	// A link of the name is replaced, never followed.
	if info, err := os.Lstat(filepath.Join(ctx, "linked")); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("linked is still a link (err %v)", err)
	}
	if got := read(filepath.Join(ctx, "linked")); got != "from-secret\n" {
		t.Errorf("linked = %q, want the secret file", got)
	}
	if got := read(filepath.Join(outside, "target")); got != "untouched\n" {
		t.Errorf("the link's target outside the repository was written: %q", got)
	}
}

// Nothing in the repository steers the copy out of it.
func TestContextFilesScriptStaysInTheRepository(t *testing.T) {
	c := newContextCheckout(t)
	source, files, outside, read := c.source, c.files, c.outside, c.read

	// A context that is a link out of the repository is the tenant's error,
	// and nothing is written through it.
	escape := filepath.Join(source, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Fatal(err)
	}
	code, out := runContextFiles(t, source, files, escape, ".npmrc")
	if code != ExitTenantError || !strings.Contains(out, "leaves the repository") {
		t.Errorf("a context outside the repository = exit %d %q, want the tenant error", code, out)
	}
	if _, err := os.Stat(filepath.Join(outside, ".npmrc")); err == nil {
		t.Error("the secret file was written outside the repository")
	}
	// A context that is not in the checkout is the tenant's error too, not an
	// unclassified failure the Job would retry.
	code, out = runContextFiles(t, source, files, filepath.Join(source, "missing"), ".npmrc")
	if code != ExitTenantError || !strings.Contains(out, "not a directory of the repository") {
		t.Errorf("a missing context = exit %d %q, want the tenant error", code, out)
	}
	// A context whose name ends in a newline: command substitution drops
	// the newline, and the path left names its sibling, a link out of the
	// repository. The copy writes relative to the directory it entered, so
	// it cannot follow that.
	newline := filepath.Join(source, "ctx\n")
	if err := os.Mkdir(newline, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "ctx")); err != nil {
		t.Fatal(err)
	}
	if code, out := runContextFiles(t, source, files, newline, ".npmrc"); code != 0 || read(filepath.Join(newline, ".npmrc")) == "" {
		t.Errorf("a newline-named context = exit %d %q, want the file in it", code, out)
	}
	if _, err := os.Stat(filepath.Join(outside, ".npmrc")); err == nil {
		t.Error("the secret file followed the newline-less sibling link out of the repository")
	}
	// The repository root itself is a context.
	if code, out := runContextFiles(t, source, files, source, ".npmrc"); code != 0 || read(filepath.Join(source, ".npmrc")) == "" {
		t.Errorf("the repository root as the context = exit %d %q", code, out)
	}
}
