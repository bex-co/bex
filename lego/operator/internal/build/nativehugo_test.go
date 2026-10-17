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
	"slices"
	"strings"
	"testing"
)

// Every native static build, on any runtime, carries the pinned Hugo step
// before its source and authored command, also after the resolve phase picks
// its line; a compute build does not (w8/m106).
func TestNativeStaticBuildCarriesHugo(t *testing.T) {
	static := nativeOptions()
	static.StaticSite, static.Runtime, static.StartCommand, static.StaticPublishPath = true, "", "", "public"
	python := static
	python.Runtime = nativePythonRuntime
	for _, o := range []Options{static, python} {
		df := resolvedDockerfileFrom(t, o, map[string]string{"NODE_VERSION": "22"}, nil)
		stages, lines := nativeStaticTools(o)
		base := strings.Index(df, "\nFROM "+nativeToolchains[nativeRuntime(o)].defaultImage())
		if o.Runtime == "" {
			base = strings.Index(df, "\nFROM node:22")
		}
		if !strings.HasPrefix(df, "# syntax=docker/dockerfile:1.7\n"+stages) || base < 0 || strings.Count(df, lines) != 1 ||
			strings.Index(df, lines) < base || strings.Index(df, lines) > strings.Index(df, "COPY . .") {
			t.Fatalf("%s static build lacks the Hugo stages before its base or the copy before its source:\n%s", nativeRuntime(o), df)
		}
		for _, pin := range []string{nativeHugo.amd64, nativeHugo.arm64} {
			if !strings.Contains(stages, "ADD --checksum="+pin+" ") {
				t.Errorf("Hugo stages lack the pinned checksum %s:\n%s", pin, stages)
			}
		}
	}
	if df := nativeDockerfile(nativeOptions()); strings.Contains(df, "hugo") {
		t.Fatalf("a compute build gained Hugo:\n%s", df)
	}
}

// renderHugoQuickstart is Render's Hugo quickstart
// (https://render.com/docs/deploy-hugo) at the commit the w8/m106 sweep used,
// with its pinned Ananke theme submodule, as source files.
func renderHugoQuickstart(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "clone", "--quiet", "https://github.com/render-examples/hugo-quick-start", ".")
	git(t, dir, "checkout", "--quiet", "48eaf95c3fd4a8c438df1c14dd9e2ff983a77b7c")
	git(t, dir, "submodule", "update", "--init", "--quiet")
	return sourceTree(t, dir)
}

// sourceTree reads every file under dir, Git metadata aside, as a source map
// keyed by its slash path relative to dir.
func sourceTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	src := map[string]string{}
	must(t, filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.Name() == ".git" && d.IsDir():
			return filepath.SkipDir
		case d.Name() == ".git" || d.IsDir():
			return nil
		}
		raw, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		src[filepath.ToSlash(rel)] = string(raw)
		return err
	}))
	return src
}

// TestNativeStaticBuildRunsHugo is w8/m106's execution gate: Render's Hugo
// quickstart, unchanged, builds and publishes through the generated static
// image with its documented command, on the pinned Hugo extended release.
// Opt-in: needs Docker and network for GitHub.
func TestNativeStaticBuildRunsHugo(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build the generated image")
	}
	src := renderHugoQuickstart(t)
	// Both reviewed architectures' tarballs: the host's, and the other under
	// emulation.
	for _, platform := range []string{"linux/amd64", "linux/arm64"} {
		t.Run(platform, func(t *testing.T) {
			t.Setenv("DOCKER_DEFAULT_PLATFORM", platform)
			o := nativeOptions()
			o.StaticSite, o.Runtime, o.StartCommand, o.BuildEnv = true, "", "", nil
			o.BuildCommand, o.StaticPublishPath = "hugo --gc --minify", "public"
			o.NativeEnvRevision = fmt.Sprintf("hugo-%d", os.Getpid())
			tag := fmt.Sprintf("bex-native-static-hugo-it-%s-%d", strings.TrimPrefix(platform, "linux/"), os.Getpid())
			buildResolvedImage(t, o, nil, src, tag)
			index, err := exec.Command("docker", "run", "--rm", "--platform", platform, "--entrypoint", "/bin/cat", tag,
				"/opt/render/project/src/public/index.html").CombinedOutput()
			for _, want := range []string{"<title>Deploy Hugo on Render", `content="Hugo ` + nativeHugo.version + `"`, "ananke/css/main.min.css"} {
				if err != nil || !strings.Contains(string(index), want) {
					t.Fatalf("published index = %v %q, want %q", err, index, want)
				}
			}
		})
	}
}

// TestNativeHugoLayerIsShared pins why hugo is copied to /: cold static builds
// on two different bases both carry the exact layer the copy makes on its own,
// so every static image shares one Hugo layer. Opt-in: needs Docker and
// network for GitHub.
func TestNativeHugoLayerIsShared(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build the generated images")
	}
	layers := func(t *testing.T, tag, dir string) []string {
		t.Helper()
		t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
		cmd := exec.Command("docker", "build", "-q", "--no-cache", "-t", tag, dir)
		cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", tag, err, out)
		}
		out, err := exec.Command("docker", "image", "inspect", "--format", "{{range .RootFS.Layers}}{{println .}}{{end}}", tag).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.Fields(string(out))
	}
	static := nativeOptions()
	static.StaticSite, static.Runtime, static.StartCommand = true, "", ""
	stages, lines := nativeStaticTools(static)
	alone := t.TempDir()
	must(t, os.WriteFile(filepath.Join(alone, "Dockerfile"), []byte(stages+"FROM scratch\n"+strings.SplitAfter(lines, "\n")[0]), 0o644))
	want := layers(t, fmt.Sprintf("bex-native-hugo-layer-alone:%d", os.Getpid()), alone)
	if len(want) != 1 {
		t.Fatalf("the copy alone made layers %q, want one", want)
	}
	for _, runtime := range []string{"", nativePythonRuntime} {
		o := static
		o.Runtime = runtime
		df := resolvedDockerfileFrom(t, o, nil, nil)
		// Up to the source: the layers a site's content cannot change.
		dir := t.TempDir()
		must(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(df[:strings.Index(df, "\nCOPY . .\n")+1]), 0o644))
		if got := layers(t, fmt.Sprintf("bex-native-hugo-layer-%s:%d", nativeRuntime(o), os.Getpid()), dir); !slices.Contains(got, want[0]) {
			t.Errorf("%s static image layers %q lack the shared hugo layer %s", nativeRuntime(o), got, want[0])
		}
	}
}
