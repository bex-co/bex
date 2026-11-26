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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nativerepo_test.go is w4/m650. Render: "Docker-based services can't access
// files in your repo outside their root directory." and "Services on a native
// runtime can access files outside their root directory"
// (render.com/docs/monorepo-support, read 2026-11-25). A native build's
// context was its root directory, so a Go module whose go.mod sits above
// cmd/api, or a pnpm workspace's apps/web, built on Render and failed on bex.

// buildContext is the directory BuildKit reads as o's build context, as the
// Job's buildctl arguments name it.
func buildContext(t *testing.T, o Options) string {
	t.Helper()
	args := buildkitContainer(t, BuildJob(o, o.ImageRef())).Args
	for i, arg := range args {
		if value, ok := strings.CutPrefix(arg, "context="); ok && i > 0 && args[i-1] == "--local" {
			return value
		}
	}
	t.Fatalf("no --local context= in %q", args)
	return ""
}

func TestNativeBuildReadsTheWholeRepository(t *testing.T) {
	const goRepo = "/opt/render/project/go/src/github.com/example/app"
	for _, tc := range []struct {
		name, runtime, rootDir string
		static                 bool
		copy                   string // the source copy, up to the env loader
	}{
		{"node in cmd/api", "node", "cmd/api", false,
			"\nWORKDIR /opt/render/project/src\nCOPY . .\nWORKDIR /opt/render/project/src/cmd/api\nCOPY <<"},
		{"go in cmd/api", "go", "cmd/api", false,
			"\nWORKDIR " + goRepo + "\nCOPY . .\nWORKDIR " + goRepo + "/cmd/api\nRUN ln -s " + goRepo + "/cmd/api /opt/render/project/src\nCOPY <<"},
		{"python with an uncleaned root directory", "python", "/backend/./api/", false,
			"\nWORKDIR /opt/render/project/src\nCOPY . .\nWORKDIR /opt/render/project/src/backend/api\nCOPY <<"},
		// Traversal stays inside the checkout, as the clone step reads it.
		{"ruby with a traversal", "ruby", "../../apps/web", false,
			"\nWORKDIR /opt/render/project/src\nCOPY . .\nWORKDIR /opt/render/project/src/apps/web\nCOPY <<"},
		{"a static site", "go", "apps/web", true,
			"\nWORKDIR /opt/render/project/src\nCOPY . .\nWORKDIR /opt/render/project/src/apps/web\nCOPY <<"},
		// No root directory: the lines from before w4/m650.
		{"node without a root directory", "node", "", false, "\nWORKDIR /opt/render/project/src\nCOPY . .\nCOPY <<"},
		{"go without a root directory", "go", ".", false,
			"\nWORKDIR " + goRepo + "\nRUN ln -s " + goRepo + " /opt/render/project/src\nCOPY . .\nCOPY <<"},
	} {
		o := nativeOptions()
		o.Runtime, o.RootDir = tc.runtime, tc.rootDir
		if tc.static {
			o.StaticSite, o.StartCommand, o.StaticPublishPath = true, "", "dist"
		}
		if got := buildContext(t, o); got != sourceMount {
			t.Errorf("%s: build context %q, want the whole checkout %q", tc.name, got, sourceMount)
		}
		df := nativeDockerfile(o)
		if !strings.Contains(df, tc.copy) || strings.Count(df, "\nCOPY . .\n") != 1 {
			t.Errorf("%s: Dockerfile lacks %q:\n%s", tc.name, tc.copy, df)
		}
		// The dependency directories' key is the Dockerfile up to its first
		// `COPY . .` (xdgBuildctl), which must still come before anything a
		// release changes.
		if copyAt, revisionAt := strings.Index(df, "\nCOPY . .\n"), strings.Index(df, ": bex-native-release="); copyAt < 0 || copyAt > revisionAt {
			t.Errorf("%s: the source copy no longer precedes the release revision:\n%s", tc.name, df)
		}
	}

	// A Docker context is a Dockerfile build's setting, never a native one's.
	o := nativeOptions()
	o.RootDir, o.DockerContext = "apps/web", "apps"
	if got := buildContext(t, o); got != sourceMount {
		t.Errorf("a native build with a Docker context reads %q, want %q", got, sourceMount)
	}
}

// A root directory the image has no place for keeps the root directory alone
// as its context, in the Dockerfile from before: one the WORKDIR line would
// have to quote, or a Go service whose repository URL names no owner and
// repository, whose working directory never held the root directory's path.
func TestNativeRootDirectoryWithNoPlaceKeepsItsOwnContext(t *testing.T) {
	for _, tc := range []struct {
		name, runtime, repo, rootDir, context string
	}{
		{"a space", "node", "https://github.com/example/app", "my app", sourceMount + "/my app"},
		{"a dollar", "python", "https://github.com/example/app", "a/$HOME", sourceMount + "/a/$$HOME"},
		{"go from an unparsable repository", "go", "file:///srv/app", "cmd/api", sourceMount + "/cmd/api"},
	} {
		o := nativeOptions()
		o.Runtime, o.Repo, o.RootDir = tc.runtime, tc.repo, tc.rootDir
		if got := buildContext(t, o); got != tc.context {
			t.Errorf("%s: build context %q, want %q", tc.name, got, tc.context)
		}
		if df := nativeDockerfile(o); !strings.Contains(df, "\nWORKDIR /opt/render/project/src\nCOPY . .\nCOPY <<") {
			t.Errorf("%s: Dockerfile changed:\n%s", tc.name, df)
		}
	}
}

// Render's Docker-based services "can't access files in your repo outside
// their root directory": a Dockerfile build's context is unchanged, its
// Docker context when set, else its root directory, and its Dockerfile is
// read from the root directory either way.
func TestDockerfileBuildContextIsItsRootDirectory(t *testing.T) {
	for _, tc := range []struct {
		rootDir, dockerContext, context string
	}{
		{"", "", sourceMount},
		{"svc", "", sourceMount + "/svc"},
		{"/svc/./api/", "", sourceMount + "/svc/api"},
		{"", "ctx", sourceMount + "/ctx"},
		{"svc", "ctx", sourceMount + "/ctx"},
		{"svc/api", "svc", sourceMount + "/svc"},
		{"svc", ".", sourceMount},
		{"../../svc", "", sourceMount + "/svc"},
		{"svc", "../../ctx", sourceMount + "/ctx"},
	} {
		o := dockerOpts()
		o.RootDir, o.DockerContext = tc.rootDir, tc.dockerContext
		if got := buildContext(t, o); got != tc.context {
			t.Errorf("rootDir %q dockerContext %q: build context %q, want %q", tc.rootDir, tc.dockerContext, got, tc.context)
		}
		if got := dockerContextDir(o); got != tc.context {
			t.Errorf("rootDir %q dockerContext %q: the secret files go to %q, want %q", tc.rootDir, tc.dockerContext, got, tc.context)
		}
		args := buildkitContainer(t, BuildJob(o, o.ImageRef())).Args
		if want := "dockerfile=" + boundedSourceDir(tc.rootDir); !strings.Contains(strings.Join(args, " "), "--local "+want+" ") {
			t.Errorf("rootDir %q dockerContext %q: args %q lack --local %s", tc.rootDir, tc.dockerContext, args, want)
		}
	}
}

// A native build's generated Dockerfile comes with an ignore file of its own,
// which BuildKit reads in place of a .dockerignore in the context: the
// repository's .dockerignore is written for its Docker services, and a
// pattern such as `**` with `!services/api` would otherwise leave a native
// service's own root directory out of its image. Only git's directories stay
// out.
func TestNativePreparerWritesTheDockerfilesIgnoreFile(t *testing.T) {
	o := nativeOptions()
	o.RuntimeEnvSecret = ""
	native := filepath.Join(t.TempDir(), "native")
	must(t, os.Mkdir(native, 0o755))
	runPreparer(t, o, native, t.TempDir())
	got, err := os.ReadFile(filepath.Join(native, "Dockerfile.dockerignore"))
	must(t, err)
	if string(got) != ".git\n**/.git\n" {
		t.Errorf("Dockerfile.dockerignore = %q, want git's directories only", got)
	}
	// BuildKit reads <filename>.dockerignore beside the Dockerfile it is given.
	args := strings.Join(buildkitContainer(t, BuildJob(o, o.ImageRef())).Args, " ")
	if !strings.Contains(args, "--local dockerfile=/native --opt filename=Dockerfile ") {
		t.Errorf("the Dockerfile and its ignore file are no longer read from /native: %q", args)
	}
}

// runPreparer runs o's native preparer script with its /native directory at
// native and the secret files it copies read from secrets, as the Job's
// preparer container does.
func runPreparer(t *testing.T, o Options, native, secrets string) {
	t.Helper()
	prep := nativeBuildPreparer(o)
	script := strings.NewReplacer("/native-files", secrets, "/native", native,
		"/runtime-env", filepath.Join(native, "absent-runtime-env")).Replace(prep.Args[0])
	cmd := exec.Command("sh", "-eu", "-c", script)
	cmd.Env = os.Environ()
	for _, env := range prep.Env {
		// The kubelet reads $$ in a literal as $ (execution.KubernetesLiteral).
		cmd.Env = append(cmd.Env, env.Name+"="+strings.ReplaceAll(env.Value, "$$", "$"))
	}
	if log, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare: %v: %s", err, log)
	}
}

// runSourceBuildctl runs o's exact Job buildctl arguments through the pinned
// BuildKit image with the checkout src mounted where the Job mounts it, so the
// build context is the one the Job names, and the native preparer directory,
// if any, at /native. It returns BuildKit's log and its error.
func runSourceBuildctl(t *testing.T, o Options, src, native, out string) ([]byte, error) {
	t.Helper()
	args := buildkitContainer(t, BuildJob(o, o.ImageRef())).Args
	run := []string{"run", "--rm", "--privileged", "--entrypoint", "buildctl-daemonless.sh",
		"-v", src + ":" + sourceMount + ":ro", "-v", out + ":" + outputMount}
	if native != "" {
		run = append(run, "-v", native+":/native:ro")
	}
	return exec.Command("docker", append(append(run, defaultBuildkitImage), args...)...).CombinedOutput()
}

// buildRepositoryImage builds o's native image from a checkout holding files
// as the Job does: the preparer, the resolve phase over the root directory,
// native-init staged, BuildKit with the Job's own context, then loads it into
// the local daemon and returns its ID.
func buildRepositoryImage(t *testing.T, o Options, files, secretFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	src, native, out, secrets := filepath.Join(dir, "src"), filepath.Join(dir, "native"), filepath.Join(dir, "out"), filepath.Join(dir, "secrets")
	for _, d := range []string{src, native, out, secrets} {
		must(t, os.Mkdir(d, 0o755))
	}
	for name, content := range files {
		must(t, os.MkdirAll(filepath.Dir(filepath.Join(src, name)), 0o755))
		must(t, os.WriteFile(filepath.Join(src, name), []byte(content), 0o644))
	}
	for name, content := range secretFiles {
		must(t, os.WriteFile(filepath.Join(secrets, name), []byte(content), 0o644))
	}
	runPreparer(t, o, native, secrets)
	if _, err := resolveNativeBuild(nativeRuntime(o), filepath.Join(native, "render-env"), filepath.Join(native, "Dockerfile"),
		filepath.Join(src, repoRelative(o.RootDir))); err != nil {
		t.Fatal(err)
	}
	must(t, StageNativeInit(compileNativeInit(t), filepath.Join(native, "init", nativeInitFile)))
	if log, err := runSourceBuildctl(t, o, src, native, out); err != nil {
		t.Fatalf("buildkit: %v\n%s", err, log)
	}
	return loadOCIArchive(t, out, fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), os.Getpid()))
}

// The execution gate: a Go module whose go.mod sits above its root directory
// cmd/api, and a pnpm workspace's apps/web that imports packages/ui, build
// from the whole repository and serve what they read above their root
// directory, from their root directory. The repository's .dockerignore,
// which would leave everything out, is not read, and git's directories are
// not in the image. A Dockerfile service with a root directory still cannot
// copy a file above it. Opt-in: needs Docker, pulls the pinned toolchain
// images and pnpm.
func TestNativeImageReadsTheWholeRepository(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated images")
	}
	// The official images run on any machine; which base the toolchain sits
	// on has no part in what the build context holds.
	withoutCommonBase(t)
	t.Run("go module above cmd/api", func(t *testing.T) {
		const repo = "/opt/render/project/go/src/github.com/example/app"
		o := nativeInitOptions()
		o.Runtime, o.RootDir, o.BuildEnv, o.RuntimeEnvSecret = "go", "cmd/api", nil, ""
		o.BuildCommand, o.StartCommand = "go build -o app . && test \"$(cat /etc/secrets/api.env)\" = SECRET=1", "./app"
		o.NativeFilesSecrets, o.NativeFiles = []string{"bld-web-native-files"}, []string{"api.env"}
		image := buildRepositoryImage(t, o, map[string]string{
			"go.mod":                  "module example.com/m650\n\ngo 1.22\n",
			"internal/greet/greet.go": "package greet\n\nconst Text = \"read from internal/greet\"\n",
			"cmd/api/main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n\t\"os\"\n\n\t\"example.com/m650/internal/greet\"\n)\n\n" +
				"func main() {\n\twd, _ := os.Getwd()\n\thttp.HandleFunc(\"/\", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintf(w, \"%s in %s\", greet.Text, wd) })\n" +
				"\t_ = http.ListenAndServe(\":\"+os.Getenv(\"PORT\"), nil)\n}\n",
			".dockerignore":        "**\n",
			".git/HEAD":            "ref: refs/heads/main\n",
			"third_party/sub/.git": "gitdir: ../../.git/modules/sub\n",
		}, map[string]string{"api.env": "SECRET=1"})
		body, _ := runServing(t, image, fmt.Sprintf("bex-m650-go-%d", os.Getpid()), "http://127.0.0.1:10000/")
		if want := "read from internal/greet in " + repo + "/cmd/api"; body != want {
			t.Errorf("served %q, want %q", body, want)
		}
		check := "test -f " + repo + "/go.mod && test -f " + repo + "/.dockerignore && test ! -e " + repo + "/.git && test ! -e " + repo + "/third_party/sub/.git" +
			" && test \"$(readlink /opt/render/project/src)\" = " + repo + "/cmd/api" +
			// The secret file is linked in the root directory, where it was
			// before, and nowhere else in the repository.
			" && test \"$(readlink " + repo + "/cmd/api/api.env)\" = /etc/secrets/api.env && test ! -e " + repo + "/api.env"
		if out, err := exec.Command("docker", "run", "--rm", "--entrypoint", "/bin/sh", image, "-c", check).CombinedOutput(); err != nil {
			t.Errorf("image layout: %v\n%s", err, out)
		}
	})
	t.Run("pnpm workspace apps/web", func(t *testing.T) {
		o := nativeInitOptions()
		o.RootDir, o.BuildEnv, o.RuntimeEnvSecret = "apps/web", nil, ""
		o.BuildCommand, o.StartCommand = "pnpm install --no-frozen-lockfile", "node server.js"
		image := buildRepositoryImage(t, o, map[string]string{
			"package.json":             `{"name":"m650-workspace","private":true}`,
			"pnpm-workspace.yaml":      "packages:\n  - apps/*\n  - packages/*\n",
			"packages/ui/package.json": `{"name":"@m650/ui","version":"1.0.0","main":"index.js"}`,
			"packages/ui/index.js":     `module.exports = "text from packages/ui";` + "\n",
			"apps/web/package.json":    `{"name":"@m650/web","version":"1.0.0","private":true,"dependencies":{"@m650/ui":"workspace:*"}}`,
			"apps/web/server.js": `require("http").createServer((_, res) => res.end(require("@m650/ui") + " in " + process.cwd()))` +
				`.listen(process.env.PORT);` + "\n",
			".dockerignore": "**\n!apps/api\n",
		}, nil)
		body, _ := runServing(t, image, fmt.Sprintf("bex-m650-pnpm-%d", os.Getpid()), "http://127.0.0.1:10000/")
		if want := "text from packages/ui in /opt/render/project/src/apps/web"; body != want {
			t.Errorf("served %q, want %q", body, want)
		}
	})
	t.Run("docker control", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		must(t, os.MkdirAll(filepath.Join(src, "svc"), 0o755))
		must(t, os.MkdirAll(filepath.Join(src, "shared"), 0o755))
		must(t, os.WriteFile(filepath.Join(src, "shared", "file.txt"), []byte("shared\n"), 0o644))
		must(t, os.WriteFile(filepath.Join(src, "svc", "Dockerfile"), []byte("FROM scratch\nCOPY ../shared/file.txt /file.txt\n"), 0o644))
		o := opts()
		o.Builder, o.RootDir = BuilderDockerfile, "svc"
		log, err := runSourceBuildctl(t, o, src, "", t.TempDir())
		if err == nil || !strings.Contains(string(log), `"/shared/file.txt": not found`) {
			t.Errorf("a Dockerfile build copied a file above its root directory: %v\n%s", err, log)
		}
		// The same Dockerfile with the repository as its Docker context builds.
		o.DockerContext = "."
		if log, err := runSourceBuildctl(t, o, src, "", t.TempDir()); err != nil {
			t.Errorf("the control's Dockerfile does not build from the repository root: %v\n%s", err, log)
		}
	})
}

// The working directory a native image is left in is NativeSourceDir, the
// directory the operator maps a disk's mount path against (movedDiskMount,
// internal/controller/disk.go) and the resolve phase names version files
// under, whether the image holds the whole repository or its root directory
// alone: the wider copy moved no disk and no workload's directory.
func TestNativeImageWorkdirIsNativeSourceDir(t *testing.T) {
	for runtime := range nativeToolchains {
		for _, repo := range []string{"https://github.com/example/app", "file:///srv/app"} {
			for _, rootDir := range []string{"", ".", "backend", "apps/web", "/apps/./web/", "my app"} {
				o := nativeOptions()
				o.Runtime, o.Repo, o.RootDir = runtime, repo, rootDir
				df := nativeDockerfile(o)
				workdir := df[strings.LastIndex(df, "\nWORKDIR ")+len("\nWORKDIR "):]
				workdir = workdir[:strings.IndexByte(workdir, '\n')]
				if want := NativeSourceDir(runtime, repo, rootDir); workdir != want {
					t.Errorf("%s %s root %q: the image works in %q, the operator maps disks against %q", runtime, repo, rootDir, workdir, want)
				}
			}
		}
	}
}
