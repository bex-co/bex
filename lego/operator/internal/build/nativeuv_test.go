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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// uvProbeProject is a real uv project and its lockfile (uv 0.12, one pinned
// pure-Python dependency), so the execution gate runs an actual uv sync/run.
const (
	uvProbePyproject = `[project]
name = "bex-uv-probe"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = ["six==1.17.0"]
`
	uvProbeLock = `version = 1
revision = 3
requires-python = ">=3.11"

[[package]]
name = "bex-uv-probe"
version = "0.1.0"
source = { virtual = "." }
dependencies = [
    { name = "six" },
]

[package.metadata]
requires-dist = [{ name = "six", specifier = "==1.17.0" }]

[[package]]
name = "six"
version = "1.17.0"
source = { registry = "https://pypi.org/simple" }
sdist = { url = "https://files.pythonhosted.org/packages/94/e7/b2c673351809dca68a0e064b6af791aa332cf192da575fd474ed7d6f16a2/six-1.17.0.tar.gz", hash = "sha256:ff70335d468e7eb6ec65b95b99d3a2836546063f63acc5171de367e834932a81", size = 34031, upload-time = "2024-12-04T17:35:28.174Z" }
wheels = [
    { url = "https://files.pythonhosted.org/packages/b7/ce/149a00dd41f10bc29e5921b496af8b574d8413afcd5e30dfa0ed46c2cc5e/six-1.17.0-py2.py3-none-any.whl", hash = "sha256:4721f391ed90541fddacab5acf947aa0d3dc7d27b2e1e8eda2be8970586c3274", size = 11050, upload-time = "2024-12-04T17:35:26.475Z" },
]
`
)

// A native Python build adds uv as Render does (w8/m81): a uv.lock in the
// service's root directory opts in, empty or not, and UV_VERSION then picks an
// exact release; without the lockfile nothing changes and UV_VERSION is not
// read, even when it would be refused.
func TestResolveNativeUV(t *testing.T) {
	for _, tc := range []struct {
		name, request    string
		files            map[string]string // nil: no lockfile
		found            bool
		version, source  string
		wantErrSubstring string
	}{
		{name: "no lockfile"},
		{name: "no lockfile ignores UV_VERSION", request: "0.11.3"},
		{name: "no lockfile ignores an unusable UV_VERSION", request: "latest"},
		{name: "lockfile default", files: map[string]string{"uv.lock": uvProbeLock}, found: true, version: defaultUVVersion, source: "uv.lock"},
		{name: "empty lockfile", files: map[string]string{"uv.lock": ""}, found: true, version: defaultUVVersion, source: "uv.lock"},
		{name: "exact override", files: map[string]string{"uv.lock": uvProbeLock}, request: "0.12.10", found: true, version: "0.12.10", source: "UV_VERSION=0.12.10"},
		{name: "v-prefixed override", files: map[string]string{"uv.lock": uvProbeLock}, request: " v0.11.3 ", found: true, version: "0.11.3", source: "UV_VERSION=v0.11.3"},
		{name: "range", files: map[string]string{"uv.lock": uvProbeLock}, request: ">=0.12", wantErrSubstring: "UV_VERSION=>=0.12 is not a supported uv version; set an exact uv release such as " + defaultUVVersion},
		{name: "minor only", files: map[string]string{"uv.lock": uvProbeLock}, request: "0.12", wantErrSubstring: "UV_VERSION=0.12 is not a supported uv version"},
		{name: "latest", files: map[string]string{"uv.lock": uvProbeLock}, request: "latest", wantErrSubstring: "UV_VERSION=latest is not a supported uv version"},
		{name: "shell text", files: map[string]string{"uv.lock": uvProbeLock}, request: "0.12.10; curl x", wantErrSubstring: "UV_VERSION=0.12.10; curl x is not a supported uv version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"PYTHON_VERSION": "3.12"}
			if tc.request != "" {
				env["UV_VERSION"] = tc.request
			}
			choice, found, err := ResolveNativeUV(env, files(tc.files))
			if tc.wantErrSubstring != "" {
				var unsupported *UnsupportedVersionError
				if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tc.wantErrSubstring) || found {
					t.Fatalf("ResolveNativeUV = %+v, %v, %v; want a tenant error containing %q", choice, found, err, tc.wantErrSubstring)
				}
				return
			}
			if err != nil || found != tc.found {
				t.Fatalf("ResolveNativeUV = %+v, found %v, %v; want found %v", choice, found, err, tc.found)
			}
			if found && (choice.Label != "uv" || choice.Line != tc.version || choice.Source != tc.source || choice.Image != "") {
				t.Errorf("choice = %+v, want uv %s from %q", choice, tc.version, tc.source)
			}
		})
	}
	choice, _, _ := ResolveNativeUV(nil, func(string) ([]byte, bool) { return nil, true })
	if got := choice.Narration(); got != "==> Using uv "+defaultUVVersion+" (from uv.lock)" {
		t.Errorf("narration = %q", got)
	}
}

// The uv install lands between the selected Python's FROM and the
// source/build, on every Python line, beside Poetry and the JavaScript tools;
// the lockfile is read at the service's root directory, not the checkout's; a
// refused request leaves the Dockerfile alone; other runtimes never gain uv.
func TestResolveNativeBuildAddsUVToPythonUVProjects(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	const install = "RUN /usr/local/bin/python -m venv /opt/bex/uv && /opt/bex/uv/bin/pip install --no-cache-dir --disable-pip-version-check --no-input uv==0.12.10" +
		" && ln -s /opt/bex/uv/bin/uv /usr/local/bin/uv && ln -s /opt/bex/uv/bin/uvx /usr/local/bin/uvx\n"
	for _, line := range nativeToolchains["python"].lines {
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": line.line, "UV_VERSION": "0.12.10"}, map[string]string{"uv.lock": uvProbeLock})
		choices, err := ResolveNativeBuild("python", bundle, dockerfile, src)
		if err != nil || len(choices) != 4 || choices[0].Line != line.line || choices[1].Label != "Poetry" || choices[2].Label != "uv" || choices[2].Line != "0.12.10" || choices[3].Label != "Node" {
			t.Fatalf("python %s choices = %+v, %v; want Python, Poetry, uv, Node", line.line, choices, err)
		}
		df, _ := os.ReadFile(dockerfile)
		from := strings.Index(string(df), "\nFROM "+line.image+"\n")
		uv := strings.Index(string(df), "\n"+install)
		source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
		if from < 0 || uv < from || source < uv || build < source || !strings.Contains(string(df), "poetry==") {
			t.Fatalf("python %s: uv is not installed between FROM and the source/build beside Poetry:\n%s", line.line, df)
		}
	}

	// The uv step is the cache input: identical inputs resolve byte for byte,
	// and a version change changes only that step, which follows Poetry's.
	resolved := func(version string) string {
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"UV_VERSION": version}, map[string]string{"uv.lock": uvProbeLock})
		if _, err := ResolveNativeBuild("python", bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		df, _ := os.ReadFile(dockerfile)
		return string(df)
	}
	first, again, other := resolved("0.12.10"), resolved("0.12.10"), resolved("0.11.3")
	if first != again || strings.Replace(first, "uv==0.12.10", "uv==0.11.3", 1) != other {
		t.Fatalf("resolution is not a stable cache input:\n%s\n---\n%s", first, other)
	}
	if strings.Index(first, "/opt/bex/uv/bin/pip") < strings.Index(first, "/opt/bex/poetry/bin/pip") {
		t.Fatalf("uv's step does not follow Poetry's:\n%s", first)
	}

	// Without the lockfile the Dockerfile is exactly what it was before uv.
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"UV_VERSION": "0.12.10"}, map[string]string{"requirements.txt": "six==1.17.0\n"})
	choices, err := ResolveNativeBuild("python", bundle, dockerfile, src)
	if err != nil || len(choices) != 3 {
		t.Fatalf("no-lock choices = %+v, %v; want Python, Poetry, Node", choices, err)
	}
	if df, _ := os.ReadFile(dockerfile); strings.Contains(string(df), "/opt/bex/uv") {
		t.Fatalf("a project without uv.lock gained uv:\n%s", df)
	}

	// The resolver reads the service's root directory: a lockfile elsewhere in
	// the checkout does not opt the service in, and one in its rootDir does.
	for _, tc := range []struct {
		lockAt string
		want   bool
	}{{"apps/web/uv.lock", true}, {"uv.lock", false}} {
		bundle, dockerfile, checkout := writeResolveFixture(t, o, nil, nil)
		root := filepath.Join(checkout, "apps", "web")
		must(t, os.MkdirAll(root, 0o755))
		must(t, os.WriteFile(filepath.Join(checkout, tc.lockAt), []byte(uvProbeLock), 0o644))
		choices, err := ResolveNativeBuild("python", bundle, dockerfile, root)
		if got := err == nil && len(choices) == 4 && choices[2].Label == "uv"; got != tc.want {
			t.Errorf("lockfile at %s: choices = %+v, %v; want uv %v", tc.lockAt, choices, err, tc.want)
		}
	}

	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"UV_VERSION": "latest"}, map[string]string{"uv.lock": uvProbeLock})
	before, _ := os.ReadFile(dockerfile)
	var unsupported *UnsupportedVersionError
	if _, err := ResolveNativeBuild("python", bundle, dockerfile, src); !errors.As(err, &unsupported) {
		t.Fatalf("unsupported UV_VERSION = %v, want a tenant UnsupportedVersionError", err)
	}
	if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
		t.Error("a refused uv request rewrote the Dockerfile")
	}

	for _, runtime := range []string{"node", "ruby"} {
		o.Runtime = runtime
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"UV_VERSION": "0.12.10"}, map[string]string{"uv.lock": uvProbeLock})
		if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
			t.Fatalf("%s: %v", runtime, err)
		}
		if df, _ := os.ReadFile(dockerfile); strings.Contains(string(df), "/opt/bex/uv") {
			t.Fatalf("%s build gained uv:\n%s", runtime, df)
		}
	}
}

// TestNativeImageRunsUV is w8/m81's execution gate: for a Python 3.12
// root-lock project, the authored build runs uv and a real `uv sync --locked`,
// and the started image runs `uv run` against the synced environment, both on
// the selected Python with the requested (or default) uv; uvx is on PATH, the
// application's own interpreter cannot import uv, and Poetry, Node and an
// opted-in Bun are still there.
// Opt-in: needs Docker and network for PyPI.
func TestNativeImageRunsUV(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	node24, _ := nativeToolchains[nativeNodeRuntime].find("24")
	for _, tc := range []struct {
		name, request, want string
		bun                 bool
	}{
		{"override-with-bun", "0.11.3", "0.11.3", true},
		{"default", "", defaultUVVersion, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = "python"
			probe := `uv run python -c 'import importlib.util,platform,six; print("app-python=" + platform.python_version() + " six=" + six.__version__ + " app-imports-uv=" + str(importlib.util.find_spec("uv") is not None))'`
			o.BuildCommand = `uv --version > built.txt && uvx --version >> built.txt && uv sync --locked && ` + probe + ` >> built.txt`
			o.StartCommand = `echo "runtime $(uv --version)"; ` + probe + `; echo "poetry $(poetry --version)"; echo "node $(node --version)";` +
				` echo "bun $(bun --version 2>/dev/null || echo absent)"; echo "built $(cat built.txt | tr '\n' ' ')"`
			env := map[string]string{"PYTHON_VERSION": "3.12"}
			if tc.request != "" {
				env["UV_VERSION"] = tc.request
			}
			wantBun := "bun absent"
			if tc.bun {
				env["BUN_VERSION"] = "1.3"
				bun, _ := nativeBun.find("1.3")
				wantBun = "bun " + bun.version
			}
			tag := buildResolvedImage(t, o, env, map[string]string{"pyproject.toml": uvProbePyproject, "uv.lock": uvProbeLock},
				fmt.Sprintf("bex-native-uv-it-%s:%d", tc.name, os.Getpid()))
			out, err := exec.Command("docker", "run", "--rm", "--network", "none", tag).CombinedOutput()
			py312, _ := nativeToolchains["python"].find("3.12")
			app := "app-python=" + py312.version + " six=1.17.0 app-imports-uv=False"
			for _, want := range []string{
				"runtime uv " + tc.want,
				app,
				"poetry Poetry (version " + defaultPoetryVersion + ")",
				"node v" + node24.version,
				wantBun,
				"built uv " + tc.want,
				"uvx " + tc.want,
			} {
				if err != nil || !strings.Contains(string(out), want) {
					t.Fatalf("run = %v\n%s; want %q", err, out, want)
				}
			}
			if strings.Count(string(out), app) != 2 {
				t.Fatalf("run =\n%s; want the synced app environment at build and at runtime", out)
			}
		})
	}
}
