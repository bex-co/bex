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
	"slices"
	"strings"
	"testing"
)

// w6/m546: Render keeps a Go service's source in its GOPATH, at
// /opt/render/project/go/src/github.com/<owner>/<repo> (render.com/docs/disks),
// and runs its commands from the root directory under it.
func TestNativeGoSourceDirIsRendersGoPath(t *testing.T) {
	const gopath = "/opt/render/project/go/src/"
	for _, tc := range []struct {
		repo, rootDir, want string
	}{
		{"https://github.com/render-examples/go-gin-web-server", "", gopath + "github.com/render-examples/go-gin-web-server"},
		{"https://github.com/Owner/My.Repo.git", "", gopath + "github.com/Owner/My.Repo"},
		{"https://github.com/owner/repo/", "", gopath + "github.com/owner/repo"},
		{"https://x-access-token:secret@GitHub.com/owner/repo.git", "", gopath + "github.com/owner/repo"},
		{"git@github.com:owner/repo.git", "", gopath + "github.com/owner/repo"},
		{"ssh://git@github.com:2222/owner/repo.git", "", gopath + "github.com/owner/repo"},
		{"https://gitlab.com/group/sub/repo.git", "", gopath + "gitlab.com/group/sub/repo"},
		{"https://bitbucket.org/team/repo", "", gopath + "bitbucket.org/team/repo"},
		{"https://github.com/bex-co/block-eden-mono.git", "projects/reverse-proxy-poc", gopath + "github.com/bex-co/block-eden-mono/projects/reverse-proxy-poc"},
		{"https://github.com/owner/repo", "/backend/./api/", gopath + "github.com/owner/repo/backend/api"},
		{"https://github.com/owner/repo", ".", gopath + "github.com/owner/repo"},
	} {
		if got, ok := nativeGoSourceDir(tc.repo, tc.rootDir); !ok || got != tc.want {
			t.Errorf("nativeGoSourceDir(%q, %q) = %q, %v; want %q", tc.repo, tc.rootDir, got, ok, tc.want)
		}
	}
	// A URL with no owner and repository, or a part the WORKDIR line or the
	// link step would have to quote, keeps the service at NativeWorkdir.
	for _, tc := range []struct{ repo, rootDir string }{
		{"", ""},
		{"https://github.com/", ""},
		{"https://github.com/repo", ""},
		{"git@github.com:repo.git", ""},
		{"git@github.com", ""},
		{"file:///srv/owner/repo", ""},
		{"https://github.com/owner//repo", ""},
		{"https://github.com/../repo", ""},
		{"https://github.com/owner/re$po", ""},
		{"https://github.com/owner/repo", "my app"},
		{"https://github.com/owner/repo", "a\\b"},
	} {
		if got, ok := nativeGoSourceDir(tc.repo, tc.rootDir); ok {
			t.Errorf("nativeGoSourceDir(%q, %q) = %q, want no directory", tc.repo, tc.rootDir, got)
		}
	}
}

// w6/m546: a Go service's image works in Render's Go source directory, with
// /opt/render/project/src a link to it so the paths and the mounts of a
// service built before keep resolving; every other image is unchanged.
func TestNativeDockerfileWorkdirIsTheRuntimesSourcePath(t *testing.T) {
	const goDir = "/opt/render/project/go/src/github.com/example/app"
	for runtime := range nativeToolchains {
		o := nativeOptions()
		o.Runtime = runtime
		df := nativeDockerfile(o)
		if runtime == "go" {
			want := "\nWORKDIR " + goDir + "\nRUN ln -s " + goDir + " /opt/render/project/src\nCOPY . .\n"
			if !strings.Contains(df, want) || strings.Count(df, "WORKDIR ") != 1 {
				t.Errorf("go Dockerfile lacks %q:\n%s", want, df)
			}
			continue
		}
		if !strings.Contains(df, "\nWORKDIR /opt/render/project/src\nCOPY . .\n") || strings.Contains(df, "/opt/render/project/go/src") {
			t.Errorf("%s Dockerfile moved off /opt/render/project/src:\n%s", runtime, df)
		}
	}

	// A build source's image runs under each linked service's start command,
	// in the same directory a Go service of its own would.
	carrier := nativeOptions()
	carrier.Runtime, carrier.BuildOnly, carrier.StartCommand = "go", true, ""
	if df := nativeDockerfile(carrier); !strings.Contains(df, "\nWORKDIR "+goDir+"\nRUN ln -s ") {
		t.Errorf("a Go build source's image is not in Render's Go path:\n%s", df)
	}

	// The root directory is under the repository, as Render runs commands
	// from it, and the link points at it: before, it was the root directory.
	monorepo := nativeOptions()
	monorepo.Runtime, monorepo.RootDir = "go", "services/api"
	if df := nativeDockerfile(monorepo); !strings.Contains(df, "\nWORKDIR "+goDir+"/services/api\nRUN ln -s "+goDir+"/services/api /opt/render/project/src\n") {
		t.Errorf("a Go service's root directory is not its working directory:\n%s", df)
	}

	// A static site has no runtime on Render, and a repository the path
	// cannot be built from keeps the old directory, with no link.
	static := nativeOptions()
	static.Runtime, static.StaticSite, static.StaticPublishPath = "go", true, "public"
	unparsable := nativeOptions()
	unparsable.Runtime, unparsable.RootDir = "go", "my app"
	for name, o := range map[string]Options{"static site": static, "unsafe root directory": unparsable} {
		if df := nativeDockerfile(o); !strings.Contains(df, "\nWORKDIR /opt/render/project/src\nCOPY . .\n") || strings.Contains(df, "ln -s /opt") {
			t.Errorf("%s left /opt/render/project/src:\n%s", name, df)
		}
	}
}

// w6/m558: every native runtime but Go runs from its root directory under
// /opt/render/project/src, as on Render (a Node service with root directory
// backend runs /opt/render/project/src/backend/server.js), and so does a
// static site's build. /opt/render/project/src is its parent, never a link.
func TestNativeRootDirectoryIsTheWorkdir(t *testing.T) {
	const dir = "/opt/render/project/src/apps/api"
	for runtime := range nativeToolchains {
		service := nativeOptions()
		service.Runtime, service.RootDir = runtime, "apps/api"
		carrier := service
		carrier.BuildOnly, carrier.StartCommand = true, ""
		static := service
		static.StaticSite, static.StartCommand, static.StaticPublishPath = true, "", "dist"
		for kind, o := range map[string]Options{"service": service, "build source": carrier, "static site": static} {
			df := nativeDockerfile(o)
			if runtime == "go" && !o.StaticSite {
				// m546's directory, which holds the root directory itself.
				if !strings.Contains(df, "\nWORKDIR /opt/render/project/go/src/github.com/example/app/apps/api\nRUN ln -s ") {
					t.Errorf("go %s left Render's Go path:\n%s", kind, df)
				}
				continue
			}
			if !strings.Contains(df, "\nWORKDIR "+dir+"\nCOPY . .\n") || strings.Contains(df, "ln -s /opt") {
				t.Errorf("%s %s does not work in its root directory:\n%s", runtime, kind, df)
			}
		}
	}

	// The root directory is cleaned as the build context is, and one the
	// WORKDIR line would have to quote keeps the old directory.
	for rootDir, want := range map[string]string{
		"": NativeWorkdir, ".": NativeWorkdir, "/": NativeWorkdir, "/backend/./api/": NativeWorkdir + "/backend/api",
		"../backend": NativeWorkdir + "/backend", "my app": NativeWorkdir, "a\\b": NativeWorkdir, "a/$HOME": NativeWorkdir,
	} {
		if got := NativeSourceDir("node", "https://github.com/example/app", rootDir); got != want {
			t.Errorf("NativeSourceDir(node, %q) = %q, want %q", rootDir, got, want)
		}
	}
	if got := NativeSourceDir("go", "https://github.com/example/app", "apps/api"); got != "/opt/render/project/go/src/github.com/example/app/apps/api" {
		t.Errorf("NativeSourceDir(go) = %q, want m546's Go path", got)
	}
}

// w6/m558: what a build keeps in the directory it runs in follows it there;
// the virtualenv stays at Render's VENV_ROOT, outside the root directory, as a
// Render Python service with root directory backend ran from
// /opt/render/project/src/.venv.
func TestNativeRootDirectoryKeepsItsDependencyDirectories(t *testing.T) {
	for runtime, want := range map[string][]string{
		"rust":   {"/opt/render/project/.cargo/registry", "/opt/render/project/.cargo/git", "/opt/render/project/src/backend/target"},
		"elixir": {"/opt/render/project/src/backend/deps", "/opt/render/project/src/backend/_build"},
		"python": {"/opt/render/project/src/.venv"},
	} {
		o := depsOpts(runtime)
		o.RootDir = "backend"
		if got := o.depsCache(); !slices.Equal(got, want) {
			t.Errorf("%s: carries %v, want %v", runtime, got, want)
		}
	}
	python := nativeOptions()
	python.Runtime, python.RootDir = "python", "backend"
	df := nativeDockerfile(python)
	for _, want := range []string{"RUN python3 -m venv /opt/render/project/src/.venv\n",
		"ENV VIRTUAL_ENV=/opt/render/project/src/.venv VENV_ROOT=/opt/render/project/src/.venv\n",
		"\nWORKDIR /opt/render/project/src/backend\n"} {
		if !strings.Contains(df, want) {
			t.Errorf("python with a root directory lacks %q:\n%s", want, df)
		}
	}

	// A static site's publish directory is checked from its root directory,
	// where the publish Job's extract, running in the image's WORKDIR, reads it.
	static := publishCheckOptions("dist")
	static.RootDir = "apps/web"
	if step := nativePublishCheckStep(static); !strings.Contains(step, "(cd /opt/render/project/src/apps/web && [ -d ") {
		t.Errorf("publish check does not look from the root directory: %q", step)
	}
}
