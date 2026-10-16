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
	"time"
)

// A primary Node serving image defaults NODE_ENV=production after its build
// RUN (w8/111) on every reviewed line; a static image, other primary runtimes
// (whose Node companion stays as it was) and the build itself get nothing.
func TestNativeNodeServingImageDefaultsProduction(t *testing.T) {
	const want = "ENV NODE_ENV=production\n"
	o := nativeOptions()
	for _, line := range nativeToolchains[nativeNodeRuntime].lines {
		df := resolvedDockerfile(t, o, map[string]string{"NODE_VERSION": line.line})
		at, run, cmd := strings.Index(df, "\n"+want), strings.Index(df, nativeBuildRun), strings.Index(df, "\nCMD ")
		if strings.Count(df, want) != 1 || at < run || at > cmd {
			t.Fatalf("node %s: NODE_ENV default is not once between the build RUN and CMD:\n%s", line.line, df)
		}
	}
	static := nativeOptions()
	static.StaticSite, static.Runtime, static.StartCommand = true, "", ""
	if df := nativeDockerfile(static); strings.Contains(df, "NODE_ENV=production") {
		t.Errorf("static image gained the Node default:\n%s", df)
	}
	// Python and Ruby gain their Node companion in the resolve phase; the
	// default must not ride along.
	for _, runtime := range []string{"python", "ruby"} {
		other := nativeOptions()
		other.Runtime = runtime
		df := resolvedDockerfile(t, other, nil)
		if !strings.Contains(df, "/usr/local/bin/node") || strings.Contains(df, "NODE_ENV=production") {
			t.Errorf("%s: want the Node companion and no Node default:\n%s", runtime, df)
		}
	}
}

// TestNativeNodeImageRuntimeEnvironment is w8/111's execution gate: the
// generated Node image builds with NODE_ENV unset and starts in production by
// default, while a value the service configures — staging, or explicitly
// empty, as a literal or from an env file the way a linked Secret arrives —
// reaches the process exactly. Opt-in: needs Docker.
func TestNativeNodeImageRuntimeEnvironment(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := nativeOptions()
	report := `node -e 'console.log(JSON.stringify({present: "NODE_ENV" in process.env, value: process.env.NODE_ENV ?? null}))'`
	o.BuildCommand = report + ` > build.json`
	o.StartCommand = `echo "build $(cat build.json) runtime $(` + report + `)"`
	o.BuildEnv = nil
	tag := buildResolvedImage(t, o, nil, nil, fmt.Sprintf("bex-native-node-env-it:%d", os.Getpid()))
	envFile := filepath.Join(t.TempDir(), "secret.env")
	must(t, os.WriteFile(envFile, []byte("NODE_ENV=\n"), 0o600))
	for _, tc := range []struct {
		name    string
		args    []string
		runtime string
	}{
		{"default", nil, `{"present":true,"value":"production"}`},
		{"configured literal", []string{"-e", "NODE_ENV=staging"}, `{"present":true,"value":"staging"}`},
		{"explicit empty literal", []string{"-e", "NODE_ENV="}, `{"present":true,"value":""}`},
		{"empty from a Secret env file", []string{"--env-file", envFile}, `{"present":true,"value":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{"run", "--rm", "--network", "none"}, tc.args...), tag)
			out, err := exec.Command("docker", args...).CombinedOutput()
			want := `build {"present":false,"value":null} runtime ` + tc.runtime
			if err != nil || !strings.Contains(string(out), want) {
				t.Fatalf("run = %v\n%s; want %q", err, out, want)
			}
		})
	}
}

// nativeBuildRun marks the generated build RUN in a Dockerfile.
const nativeBuildRun = "\nRUN --mount=type=secret,id=render-env"

// Elixir builds and runs with MIX_ENV=prod (w8/112) and Python with Render's
// Gunicorn flags (w8/113): each default precedes the build RUN once on every
// reviewed line and for an explicit static build of that runtime, and no
// other runtime gets it.
func TestNativeBuildDefaultsPrecedeTheBuild(t *testing.T) {
	for _, tc := range []struct{ runtime, versionEnv, want, name string }{
		{"elixir", "ELIXIR_VERSION", "ENV MIX_ENV=prod\n", "MIX_ENV"},
		{nativePythonRuntime, "PYTHON_VERSION", `ENV GUNICORN_CMD_ARGS="--preload --access-logfile -"` + "\n", "GUNICORN_CMD_ARGS"},
	} {
		precedesBuild := func(df string) bool {
			at := strings.Index(df, "\n"+tc.want)
			return strings.Count(df, tc.want) == 1 && at >= 0 && at < strings.Index(df, nativeBuildRun)
		}
		o := nativeOptions()
		o.Runtime = tc.runtime
		for _, line := range nativeToolchains[tc.runtime].lines {
			if df := resolvedDockerfile(t, o, map[string]string{tc.versionEnv: line.line}); !precedesBuild(df) {
				t.Fatalf("%s %s: %s default is not once before the build RUN:\n%s", tc.runtime, line.line, tc.name, df)
			}
		}
		static := o
		static.StaticSite, static.StartCommand = true, ""
		if df := nativeDockerfile(static); !precedesBuild(df) {
			t.Errorf("explicit %s static build lacks the build default:\n%s", tc.runtime, df)
		}
		for runtime := range nativeToolchains {
			if runtime == tc.runtime {
				continue
			}
			other := nativeOptions()
			other.Runtime = runtime
			if df := nativeDockerfile(other); strings.Contains(df, tc.name) {
				t.Errorf("%s gained %s:\n%s", runtime, tc.name, df)
			}
		}
	}
}

// TestNativeElixirImageMixEnvironment is w8/112's execution gate, on Mix
// itself: the generated Elixir image builds with Mix.env() prod and starts
// with it by default, while the service's own value wins in each phase —
// dev or an explicit empty value through the build's env bundle, and a
// configured literal or env-file value at runtime. Opt-in: needs Docker.
func TestNativeElixirImageMixEnvironment(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	probe := `(cd /tmp && mix run --no-mix-exs -e 'IO.puts(inspect({System.get_env("MIX_ENV"), Mix.env()}))')`
	envFile := filepath.Join(t.TempDir(), "secret.env")
	must(t, os.WriteFile(envFile, []byte("MIX_ENV=\n"), 0o600))
	for i, tc := range []struct {
		name     string
		buildEnv map[string]string
		build    string
		runs     map[string][]string // runtime result → docker run args
	}{
		{name: "defaults", build: `{"prod", :prod}`, runs: map[string][]string{
			`runtime {"prod", :prod}`: nil,
			`runtime {"dev", :dev}`:   {"-e", "MIX_ENV=dev"},
			`runtime {"", `:           {"--env-file", envFile},
		}},
		{name: "service dev at build", buildEnv: map[string]string{"MIX_ENV": "dev"}, build: `{"dev", :dev}`},
		{name: "service empty at build", buildEnv: map[string]string{"MIX_ENV": ""}, build: `{"", `},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime, o.BuildEnv = "elixir", nil
			o.NativeEnvRevision = fmt.Sprintf("mix-%d-%d", i, os.Getpid())
			o.BuildCommand = probe + ` > build.txt`
			o.StartCommand = `echo "build $(cat build.txt) runtime $( ` + probe + `)"`
			tag := buildResolvedImage(t, o, tc.buildEnv, nil, fmt.Sprintf("bex-native-mix-env-it-%d:%d", i, os.Getpid()))
			runs := tc.runs
			if runs == nil {
				runs = map[string][]string{"": nil}
			}
			for want, args := range runs {
				out, err := exec.Command("docker", append(append([]string{"run", "--rm", "--network", "none"}, args...), tag)...).CombinedOutput()
				if err != nil || !strings.Contains(string(out), "build "+tc.build) || !strings.Contains(string(out), want) {
					t.Fatalf("run %v = %v\n%s; want build %q and %q", args, err, out, tc.build, want)
				}
			}
		})
	}
}

// TestNativePythonImageGunicornAccessLog is w8/113's execution gate: Render's
// Flask shape — pip install, then a bare `gunicorn app:app` — writes an access
// line for each request to the service's stdout at the platform's port, with
// the flags present at build too, while an explicitly empty value restores
// Gunicorn's own default (no access log). Opt-in: needs Docker and network
// for PyPI.
func TestNativePythonImageGunicornAccessLog(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := nativeOptions()
	o.Runtime, o.BuildEnv = nativePythonRuntime, nil
	o.BuildCommand = `pip install --no-cache-dir --disable-pip-version-check -q gunicorn==23.0.0 && printf '%s' "$GUNICORN_CMD_ARGS" > build.txt`
	o.StartCommand = "gunicorn app:app"
	app := `import os

def app(environ, start_response):
    start_response("200 OK", [("Content-Type", "text/plain")])
    built = open("build.txt").read()
    return [("build=%s runtime=%s" % (built, os.environ.get("GUNICORN_CMD_ARGS"))).encode()]
`
	tag := buildResolvedImage(t, o, map[string]string{"PYTHON_VERSION": "3.12"}, map[string]string{"app.py": app},
		fmt.Sprintf("bex-native-gunicorn-it:%d", os.Getpid()))
	for i, tc := range []struct {
		name, env, body string
		accessLine      bool
	}{
		{"default", "", "build=--preload --access-logfile - runtime=--preload --access-logfile -", true},
		{"explicit empty", "GUNICORN_CMD_ARGS=", "build=--preload --access-logfile - runtime=", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("bex-native-gunicorn-it-%d-%d", i, os.Getpid())
			args := []string{"run", "-d", "--name", name, "--network", "none"}
			if tc.env != "" {
				args = append(args, "-e", tc.env)
			}
			if out, err := exec.Command("docker", append(args, tag)...).CombinedOutput(); err != nil {
				t.Fatalf("run: %v\n%s", err, out)
			}
			t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
			var body []byte
			for deadline := time.Now().Add(time.Minute); ; time.Sleep(time.Second) {
				out, err := exec.Command("docker", "exec", name, "curl", "-sf", "http://127.0.0.1:10000/?probe=m8113").Output()
				if body = out; err == nil {
					break
				}
				if time.Now().After(deadline) {
					logs, _ := exec.Command("docker", "logs", name).CombinedOutput()
					t.Fatalf("Gunicorn never served:\n%s", logs)
				}
			}
			if string(body) != tc.body {
				t.Fatalf("body = %q, want %q", body, tc.body)
			}
			// The line follows the response; give it a few seconds to arrive
			// (or, when it must not, to have arrived).
			var logs []byte
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
				logs, _ = exec.Command("docker", "logs", name).CombinedOutput()
				if strings.Contains(string(logs), `"GET /?probe=m8113 HTTP/1.1" 200`) {
					break
				}
			}
			if got := strings.Contains(string(logs), `"GET /?probe=m8113 HTTP/1.1" 200`); got != tc.accessLine {
				t.Fatalf("access line present = %v, want %v:\n%s", got, tc.accessLine, logs)
			}
		})
	}
}
