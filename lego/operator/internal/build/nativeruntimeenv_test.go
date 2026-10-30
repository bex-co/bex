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

// Elixir builds and runs with MIX_ENV=prod (w8/112), Python with Render's
// Gunicorn flags (w8/113) and forwarding trust (w8/116), and Ruby with
// RAILS_ENV=production (w8/114), Render's Bundler settings (w8/117) and the
// rest of Render's Ruby defaults (w5/162): each
// default precedes the build RUN once on
// every reviewed line and for an explicit static build of that runtime, and
// no other runtime gets it.
func TestNativeBuildDefaultsPrecedeTheBuild(t *testing.T) {
	for _, tc := range []struct{ runtime, versionEnv, versionFile, want, name string }{
		{"elixir", "ELIXIR_VERSION", "", "ENV MIX_ENV=prod\n", "MIX_ENV"},
		{nativePythonRuntime, "PYTHON_VERSION", "", `ENV GUNICORN_CMD_ARGS="--preload --access-logfile -"` + "\n", "GUNICORN_CMD_ARGS"},
		{nativePythonRuntime, "PYTHON_VERSION", "", `ENV FORWARDED_ALLOW_IPS="*"` + "\n", "FORWARDED_ALLOW_IPS"},
		{nativeRubyRuntime, "", ".ruby-version", "ENV RAILS_ENV=production RAILS_SERVE_STATIC_FILES=true RAILS_LOG_TO_STDOUT=true\n", "RAILS_SERVE_STATIC_FILES"},
		{nativeRubyRuntime, "", ".ruby-version", "ENV MALLOC_ARENA_MAX=2 PIDFILE=/tmp/puma-server.pid\n", "MALLOC_ARENA_MAX"},
		{nativeRubyRuntime, "", ".ruby-version", "ENV PASSENGER_ENGINE=builtin PASSENGER_ENVIRONMENT=production\n", "PASSENGER_ENGINE"},
		{nativeRubyRuntime, "", ".ruby-version",
			"ENV BUNDLE_DEPLOYMENT=true BUNDLE_PATH=/opt/render/project/.gems BUNDLE_BIN=/opt/render/project/.gems/bin" +
				" BUNDLE_APP_CONFIG=/opt/render/project/.gems\n", "BUNDLE_DEPLOYMENT"},
		{nativeRubyRuntime, "", ".ruby-version", "ENV PATH=/opt/render/project/.gems/bin:$PATH\n", "/opt/render/project/.gems"},
	} {
		precedesBuild := func(df string) bool {
			at := strings.Index(df, "\n"+tc.want)
			return strings.Count(df, tc.want) == 1 && at >= 0 && at < strings.Index(df, nativeBuildRun)
		}
		o := nativeOptions()
		o.Runtime = tc.runtime
		for _, line := range nativeToolchains[tc.runtime].lines {
			env, files := map[string]string{tc.versionEnv: line.line}, map[string]string(nil)
			if tc.versionFile != "" {
				env, files = nil, map[string]string{tc.versionFile: line.version + "\n"}
			}
			df := resolvedDockerfileFrom(t, o, env, files)
			if !strings.Contains(df, "\nFROM "+expectedNativeBase(line.image)+"\n") || !precedesBuild(df) {
				t.Fatalf("%s %s: %s default is not once before the build RUN on that line:\n%s", tc.runtime, line.line, tc.name, df)
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

// pythonGunicornImage builds Render's Flask shape as a plain WSGI app — pip
// install, then a bare `gunicorn app:app` — answering with the Gunicorn flags
// the build and the runtime saw and the URL scheme the request arrived with.
func pythonGunicornImage(t *testing.T) string {
	t.Helper()
	o := nativeOptions()
	o.Runtime, o.BuildEnv = nativePythonRuntime, nil
	o.BuildCommand = `pip install --no-cache-dir --disable-pip-version-check -q gunicorn==23.0.0 && printf '%s' "$GUNICORN_CMD_ARGS" > build.txt`
	o.StartCommand = "gunicorn app:app"
	app := `import os

def app(environ, start_response):
    start_response("200 OK", [("Content-Type", "text/plain")])
    built = open("build.txt").read()
    return [("build=%s runtime=%s scheme=%s" % (built, os.environ.get("GUNICORN_CMD_ARGS"), environ["wsgi.url_scheme"])).encode()]
`
	return buildResolvedImage(t, o, map[string]string{"PYTHON_VERSION": "3.12"}, map[string]string{"app.py": app},
		fmt.Sprintf("bex-native-gunicorn-it:%d", os.Getpid()))
}

// TestNativePythonImageGunicornAccessLog is w8/113's execution gate: the
// generated Python image writes an access line for each request to the
// service's stdout at the platform's port, with the flags present at build
// too, while an explicitly empty value restores Gunicorn's own default (no
// access log). Opt-in: needs Docker and network for PyPI.
func TestNativePythonImageGunicornAccessLog(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	tag := pythonGunicornImage(t)
	for i, tc := range []struct {
		name, env, body string
		accessLine      bool
	}{
		{"default", "", "build=--preload --access-logfile - runtime=--preload --access-logfile - scheme=http", true},
		{"explicit empty", "GUNICORN_CMD_ARGS=", "build=--preload --access-logfile - runtime= scheme=http", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"--network", "none"}
			if tc.env != "" {
				args = append(args, "-e", tc.env)
			}
			body, readLogs := runServing(t, tag, fmt.Sprintf("bex-native-gunicorn-it-%d-%d", i, os.Getpid()),
				"http://127.0.0.1:10000/?probe=m8113", args...)
			if body != tc.body {
				t.Fatalf("body = %q, want %q", body, tc.body)
			}
			// The line follows the response; give it a few seconds to arrive
			// (or, when it must not, to have arrived).
			var logs string
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
				if logs = readLogs(); strings.Contains(logs, `"GET /?probe=m8113 HTTP/1.1" 200`) {
					break
				}
			}
			if got := strings.Contains(logs, `"GET /?probe=m8113 HTTP/1.1" 200`); got != tc.accessLine {
				t.Fatalf("access line present = %v, want %v:\n%s", got, tc.accessLine, logs)
			}
		})
	}
}

// TestNativePythonImageTrustsTheIngressScheme is w8/116's execution gate: a
// request arriving from a non-loopback peer — the ingress hop, here the
// container's own address — with X-Forwarded-Proto: https reaches the app as
// https by default, as Render's FORWARDED_ALLOW_IPS=* has it, so generated
// URLs and redirects stay on HTTPS. A configured empty value, or an explicit
// Gunicorn --forwarded-allow-ips, still wins. Opt-in: needs Docker and
// network for PyPI.
func TestNativePythonImageTrustsTheIngressScheme(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	tag := pythonGunicornImage(t)
	for i, tc := range []struct{ name, env, scheme string }{
		{"default", "", "https"},
		{"configured empty", "FORWARDED_ALLOW_IPS=", "http"},
		{"explicit Gunicorn setting", "GUNICORN_CMD_ARGS=--forwarded-allow-ips=127.0.0.1", "http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("bex-native-gunicorn-scheme-it-%d-%d", i, os.Getpid())
			// Not --network none: the request must come from a non-loopback peer.
			var args []string
			if tc.env != "" {
				args = append(args, "-e", tc.env)
			}
			runServing(t, tag, name, "http://127.0.0.1:10000/", args...)
			out, err := exec.Command("docker", "exec", name, "curl", "-sf", "-H", "X-Forwarded-Proto: https",
				"http://"+containerIP(t, name)+":10000/").Output()
			if err != nil || !strings.HasSuffix(string(out), " scheme="+tc.scheme) {
				t.Fatalf("forwarded request = %q (%v), want scheme=%s", out, err, tc.scheme)
			}
		})
	}
}

// TestNativeRubyImageRailsEnvironment is w8/114's execution gate on a real
// Rails 8 app started the ordinary way (`bin/rails server`, no binding flag):
// the generated Ruby image builds and boots in production by default, so
// Rails binds every interface on PORT and answers on the container's own
// address, not only loopback. A configured development or empty value wins
// at runtime, where Rails falls back to development on localhost.
// Opt-in: needs Docker and network for RubyGems.
func TestNativeRubyImageRailsEnvironment(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	app := map[string]string{
		"Gemfile":        "source \"https://rubygems.org\"\ngem \"railties\", \"8.0.4\"\ngem \"actionpack\", \"8.0.4\"\ngem \"puma\", \"7.1.0\"\n",
		"config.ru":      "require_relative \"config/environment\"\nrun Rails.application\n",
		"config/boot.rb": "ENV[\"BUNDLE_GEMFILE\"] ||= File.expand_path(\"../Gemfile\", __dir__)\nrequire \"bundler/setup\"\n",
		"config/application.rb": `require_relative "boot"
require "rails"
require "action_controller/railtie"
module QA114
  class Application < Rails::Application
    config.load_defaults 8.0
    config.eager_load = false
    config.secret_key_base = "qa114" * 13
    config.hosts.clear
    config.logger = ActiveSupport::Logger.new($stdout)
  end
end
`,
		"config/environment.rb": "require_relative \"application\"\nRails.application.initialize!\n",
		"config/routes.rb":      "Rails.application.routes.draw do\n  root to: proc { [200, {}, [\"built=#{File.read(\"build.txt\")} running=#{Rails.env} static=#{ENV[\"RAILS_SERVE_STATIC_FILES\"]} stdout=#{ENV[\"RAILS_LOG_TO_STDOUT\"]} arena=#{ENV[\"MALLOC_ARENA_MAX\"]} pid=#{ENV[\"PIDFILE\"]}\"]] }\nend\n",
		"bin/rails":             "#!/usr/bin/env ruby\nAPP_PATH = File.expand_path(\"../config/application\", __dir__)\nrequire_relative \"../config/boot\"\nrequire \"rails/commands\"\n",
	}
	o := nativeOptions()
	o.Runtime, o.BuildEnv = nativeRubyRuntime, nil
	// Locked at build time only because this fixture commits no
	// Gemfile.lock; deployment mode (w8/117) installs from it.
	// rake --version guards Ruby's bundled gems staying reachable outside the
	// Gemfile (a GEM_PATH default would hide them, w5/162).
	o.BuildCommand = `rake --version >/dev/null && BUNDLE_DEPLOYMENT=false bundle lock >/dev/null && bundle install --quiet && ` +
		`ruby -e 'print ENV.fetch("RAILS_ENV", "unset")' > build.txt`
	o.StartCommand = "bundle exec ruby bin/rails server"
	tag := buildResolvedImage(t, o, nil, app, fmt.Sprintf("bex-native-rails-env-it:%d", os.Getpid()))
	// Render's other Ruby defaults reach the app; a service value (here an
	// explicit empty one) still wins over the image's (w5/162).
	const defaults = " static=true stdout=true arena=2 pid=/tmp/puma-server.pid"
	for i, tc := range []struct {
		name, env, running string
		public             bool // reachable on the container's own address
		rest               string
	}{
		{"default", "", "production", true, defaults},
		{"configured development", "RAILS_ENV=development", "development", false, defaults},
		{"explicit empty", "RAILS_ENV=", "development", false, defaults},
		{"service clears static serving", "RAILS_SERVE_STATIC_FILES=", "production", true, " static= stdout=true arena=2 pid=/tmp/puma-server.pid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("bex-native-rails-env-it-%d-%d", i, os.Getpid())
			// Not --network none: the check needs the container's own,
			// non-loopback address.
			var args []string
			if tc.env != "" {
				args = append(args, "-e", tc.env)
			}
			body, readLogs := runServing(t, tag, name, "http://127.0.0.1:10000/", args...)
			if want := "built=production running=" + tc.running + tc.rest; body != want {
				t.Fatalf("body = %q, want %q", body, want)
			}
			// curl exits 7 when the connection is refused: nothing listens there.
			err := exec.Command("docker", "exec", name, "curl", "-s", "-o", "/dev/null",
				"http://"+containerIP(t, name)+":10000/").Run()
			var exit *exec.ExitError
			if refused := errors.As(err, &exit) && exit.ExitCode() == 7; tc.public && err != nil || !tc.public && !refused {
				t.Fatalf("on the container address: %v, want public=%v:\n%s", err, tc.public, readLogs())
			}
		})
	}
}

// runServing starts tag as container name with the docker run flags given,
// waits up to a minute for url to answer from inside it, and returns the
// body and a reader of the container's log.
func runServing(t *testing.T, tag, name, url string, args ...string) (string, func() string) {
	t.Helper()
	if out, err := exec.Command("docker", append(append([]string{"run", "-d", "--name", name}, args...), tag)...).CombinedOutput(); err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	logs := func() string { out, _ := exec.Command("docker", "logs", name).CombinedOutput(); return string(out) }
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(time.Second) {
		if out, err := exec.Command("docker", "exec", name, "curl", "-sf", url).Output(); err == nil {
			return string(out), logs
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never served %s:\n%s", name, url, logs())
		}
	}
}

// containerIP is a running container's address on its Docker network.
func containerIP(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", name).Output()
	ip := strings.TrimSpace(string(out))
	if err != nil || ip == "" {
		t.Fatalf("container address: %v %q", err, out)
	}
	return ip
}

// TestNativeRubyImageBundlerDeployment is w8/117's execution gate on real
// Bundler: by default a lock that matches its Gemfile installs outside the
// source tree and serves the exact locked gem — through `bundle exec` and as
// a bare executable — without being rewritten, while a stale lock, or none,
// fails the build instead of being re-resolved. A service's own Bundler
// settings still win: BUNDLE_DEPLOYMENT=false or BUNDLE_FROZEN=false accept
// a stale lock, and a BUNDLE_PATH of its own is where gems go (binstubs
// still reach them). Opt-in: needs Docker and network for RubyGems.
func TestNativeRubyImageBundlerDeployment(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	lock := func(rake string) string {
		return "GEM\n  remote: https://rubygems.org/\n  specs:\n    rake (" + rake + ")\n\nPLATFORMS\n  ruby\n\n" +
			"DEPENDENCIES\n  rake (= " + rake + ")\n\nBUNDLED WITH\n   2.6.9\n"
	}
	// Ruby 3.4's own rake is 13.2.1: a bare `rake` reporting 13.3.1 is the
	// bundle's binstub, not the image's gem.
	report := `bundle exec ruby -e 'require "rake"; print Rake::VERSION'`
	observe := report + `; printf ' bare %s' "$(rake --version | awk '{print $NF}')"` +
		`; [ -d vendor ] && printf ' vendor'; [ -d ` + nativeRubyGems + `/ruby ] && printf ' gems'; true`
	for i, tc := range []struct {
		name, lock, fails, want string
		buildEnv                map[string]string
		runEnv                  []string // a service's value reaches both phases
	}{
		{name: "matched lock", lock: lock("13.3.1"), want: "13.3.1 bare 13.3.1 gems lock unchanged"},
		{name: "stale lock", lock: lock("13.2.1"), fails: "frozen"},
		{name: "no lock", fails: "requires a lockfile"},
		{name: "stale lock with deployment off", lock: lock("13.2.1"), want: "13.3.1 bare 13.3.1 gems lock rewritten",
			buildEnv: map[string]string{"BUNDLE_DEPLOYMENT": "false"}, runEnv: []string{"-e", "BUNDLE_DEPLOYMENT=false"}},
		{name: "stale lock with frozen off", lock: lock("13.2.1"), want: "13.3.1 bare 13.3.1 gems lock rewritten",
			buildEnv: map[string]string{"BUNDLE_FROZEN": "false"}, runEnv: []string{"-e", "BUNDLE_FROZEN=false"}},
		{name: "own BUNDLE_PATH", lock: lock("13.3.1"), want: "13.3.1 bare 13.3.1 vendor lock unchanged",
			buildEnv: map[string]string{"BUNDLE_PATH": "vendor/bundle"}, runEnv: []string{"-e", "BUNDLE_PATH=vendor/bundle"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime, o.BuildEnv = nativeRubyRuntime, nil
			o.NativeEnvRevision = fmt.Sprintf("bundler-%d-%d", i, os.Getpid())
			o.BuildCommand = `cp Gemfile.lock committed.lock 2>/dev/null || true; bundle install && ` + report + ` > build.txt`
			o.StartCommand = `cmp -s Gemfile.lock committed.lock && same=unchanged || same=rewritten; ` +
				`echo "build $(cat build.txt) runtime $(` + observe + `) lock $same"`
			src := map[string]string{"Gemfile": "source \"https://rubygems.org\"\ngem \"rake\", \"13.3.1\"\n", ".ruby-version": "3.4\n"}
			if tc.lock != "" {
				src["Gemfile.lock"] = tc.lock
			}
			tag := fmt.Sprintf("bex-native-bundler-it-%d:%d", i, os.Getpid())
			out, err := dockerBuildResolved(t, o, tc.buildEnv, src, tag)
			if tc.fails != "" {
				if err == nil || !strings.Contains(string(out), tc.fails) {
					t.Fatalf("build = %v, want a failure mentioning %q:\n%s", err, tc.fails, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			run, err := exec.Command("docker", append(append([]string{"run", "--rm", "--network", "none"}, tc.runEnv...), tag)...).CombinedOutput()
			if want := "build 13.3.1 runtime " + tc.want; err != nil || !strings.Contains(string(run), want) {
				t.Fatalf("run = %v\n%s; want %q", err, run, want)
			}
		})
	}
}
