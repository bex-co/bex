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
	"encoding/base64"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func files(m map[string]string) func(string) ([]byte, bool) {
	return func(name string) ([]byte, bool) {
		v, ok := m[name]
		return []byte(v), ok
	}
}

// TestResolveNativeVersion pins w8/m51's resolution policy per runtime. Every
// non-default row fails on the single-pin code it replaces.
func TestResolveNativeVersion(t *testing.T) {
	cases := []struct {
		name, runtime string
		env, files    map[string]string
		line, source  string
	}{
		{"python default", "python", nil, nil, "3.13", ""},
		{"python env, full patch → minor line", "python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil, "3.11", "PYTHON_VERSION=3.11.9"},
		{"python env beats file", "python", map[string]string{"PYTHON_VERSION": "3.12"}, map[string]string{".python-version": "3.10"}, "3.12", "PYTHON_VERSION=3.12"},
		{"python file", "python", nil, map[string]string{".python-version": "# pinned\n3.10.4\n"}, "3.10", ".python-version"},
		{"node default", "node", nil, nil, "24", ""},
		{"node env major", "node", map[string]string{"NODE_VERSION": "22"}, nil, "22", "NODE_VERSION=22"},
		{"node env full", "node", map[string]string{"NODE_VERSION": "v22.11.0"}, nil, "22", "NODE_VERSION=v22.11.0"},
		{"node .node-version beats .nvmrc", "node", nil, map[string]string{".node-version": "26", ".nvmrc": "22"}, "26", ".node-version"},
		{"node .nvmrc lts", "node", nil, map[string]string{".nvmrc": "lts/*"}, "24", ".nvmrc"},
		{"node .nvmrc codename", "node", nil, map[string]string{".nvmrc": "lts/jod"}, "22", ".nvmrc"},
		{"node .nvmrc beats engines", "node", nil, map[string]string{".nvmrc": "22", "package.json": `{"engines":{"node":">=26"}}`}, "22", ".nvmrc"},
		{"node bounded engines range", "node", nil, map[string]string{"package.json": `{"engines":{"node":">=20 <23"}}`}, "22", "package.json engines.node"},
		{"node caret engines", "node", nil, map[string]string{"package.json": `{"engines":{"node":"^22.5.0"}}`}, "22", "package.json engines.node"},
		{"node open engines → newest", "node", nil, map[string]string{"package.json": `{"engines":{"node":">=18"}}`}, "26", "package.json engines.node"},
		{"node or-range", "node", nil, map[string]string{"package.json": `{"engines":{"node":"20.x || 22.x"}}`}, "22", "package.json engines.node"},
		{"node package.json without engines", "node", nil, map[string]string{"package.json": `{"name":"x"}`}, "24", ""},
		{"ruby Gemfile beats .ruby-version", "ruby", nil, map[string]string{"Gemfile": "source 'x'\nruby '3.3.6'\n", ".ruby-version": "4.0.1"}, "3.3", "Gemfile"},
		{"ruby pessimistic minor", "ruby", nil, map[string]string{"Gemfile": `ruby "~> 3.3.0"`}, "3.3", "Gemfile"},
		{"ruby pessimistic major", "ruby", nil, map[string]string{"Gemfile": `ruby "~> 3.3"`}, "3.4", "Gemfile"},
		{"ruby file", "ruby", nil, map[string]string{".ruby-version": "ruby-4.0.0"}, "4.0", ".ruby-version"},
		{"ruby file: directive falls through", "ruby", nil, map[string]string{"Gemfile": `ruby file: ".ruby-version"`, ".ruby-version": "3.3.1"}, "3.3", ".ruby-version"},
		{"elixir env", "elixir", map[string]string{"ELIXIR_VERSION": "1.17.3"}, nil, "1.17", "ELIXIR_VERSION=1.17.3"},
		{"elixir with matching erlang", "elixir", map[string]string{"ELIXIR_VERSION": "1.19", "ERLANG_VERSION": "28.1"}, nil, "1.19", "ELIXIR_VERSION=1.19, ERLANG_VERSION=28.1"},
		{"go ignores go.mod (Render parity)", "go", nil, map[string]string{"go.mod": "module x\n\ngo 1.26\n"}, "1.24", ""},
		{"rust stays on rustup", "rust", map[string]string{"RUSTUP_TOOLCHAIN": "1.80"}, nil, "1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveNativeVersion(tc.runtime, tc.env, files(tc.files))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got.Line != tc.line || got.Source != tc.source {
				t.Fatalf("got line %q from %q, want %q from %q", got.Line, got.Source, tc.line, tc.source)
			}
			if line, _ := nativeToolchains[tc.runtime].find(tc.line); got.Image != line.image {
				t.Fatalf("image %q is not the %s line's pin", got.Image, tc.line)
			}
		})
	}
}

func TestResolveNativeVersionRefusesUnsupportedByName(t *testing.T) {
	cases := []struct {
		runtime    string
		env, files map[string]string
		want       string
	}{
		{"python", map[string]string{"PYTHON_VERSION": "2.7.18"}, nil, `PYTHON_VERSION=2.7.18 is not a supported Python version; supported Python lines are 3.10, 3.11, 3.12, 3.13, 3.14`},
		{"python", nil, map[string]string{".python-version": "pypy3.10"}, `.python-version "pypy3.10"`},
		{"node", map[string]string{"NODE_VERSION": "18"}, nil, "supported Node lines are 22, 24, 26"},
		{"node", nil, map[string]string{"package.json": `{"engines":{"node":"<20"}}`}, `package.json engines.node "<20"`},
		{"ruby", nil, map[string]string{"Gemfile": `ruby "2.7.8"`}, `Gemfile ruby "2.7.8"`},
		{"elixir", map[string]string{"ELIXIR_VERSION": "1.18", "ERLANG_VERSION": "26.2"}, nil, "runs on Erlang/OTP 28"},
	}
	for _, tc := range cases {
		_, err := ResolveNativeVersion(tc.runtime, tc.env, files(tc.files))
		var unsupported *UnsupportedVersionError
		if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %v %v: err = %v, want an UnsupportedVersionError containing %q", tc.runtime, tc.env, tc.files, err, tc.want)
		}
	}
}

func TestNativeChoiceNarration(t *testing.T) {
	c, _ := ResolveNativeVersion("python", map[string]string{"PYTHON_VERSION": "3.11.9"}, files(nil))
	if got := c.Narration(); got != "==> Using Python 3.11 (from PYTHON_VERSION=3.11.9)" {
		t.Errorf("narration = %q", got)
	}
	d, _ := ResolveNativeVersion("node", nil, files(nil))
	if got := d.Narration(); got != "==> Using Node 24 (default)" {
		t.Errorf("default narration = %q", got)
	}
}

// Every pinned line is a reviewed inventory entry (docs/ADR060 D7).
func TestNativeToolchainLinesAreInventoried(t *testing.T) {
	inv, err := LoadToolchainInventory()
	if err != nil {
		t.Fatal(err)
	}
	committed := map[string]bool{}
	for _, img := range inv.Images {
		committed[img.Committed] = true
	}
	toolchains := map[string]nativeToolchain{"bun": nativeBun}
	maps.Copy(toolchains, nativeToolchains)
	for runtime, toolchain := range toolchains {
		for _, line := range toolchain.lines {
			if !committed[line.image] {
				t.Errorf("%s line %s image %s is not in toolchain-freshness.json", runtime, line.line, line.image)
			}
		}
	}
	if pnpm := "pnpm-" + nativePnpm.version + ".tgz@" + nativePnpm.checksum; !committed[pnpm] {
		t.Errorf("pnpm %s is not in toolchain-freshness.json", pnpm)
	}
}

// writeResolveFixture lays out what the resolve phase sees: the preparer's
// env bundle and Dockerfile, and a checkout root with the given files.
func writeResolveFixture(t *testing.T, o Options, env, src map[string]string) (bundle, dockerfile, sourceDir string) {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	for k, v := range env {
		b.WriteString(k + "=" + base64.StdEncoding.EncodeToString([]byte(v)) + "\n")
	}
	bundle, dockerfile, sourceDir = filepath.Join(dir, "render-env"), filepath.Join(dir, "Dockerfile"), filepath.Join(dir, "src")
	must(t, os.WriteFile(bundle, []byte(b.String()), 0o600))
	must(t, os.WriteFile(dockerfile, []byte(nativeDockerfile(o)), 0o600))
	must(t, os.MkdirAll(sourceDir, 0o755))
	for name, content := range src {
		must(t, os.WriteFile(filepath.Join(sourceDir, name), []byte(content), 0o644))
	}
	return bundle, dockerfile, sourceDir
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The resolve phase rewrites the generated Dockerfile's FROM — and only it —
// to the requested line; the w8/m51 live repro (PYTHON_VERSION=3.11.9 built
// on 3.13) is the first case.
func TestResolveNativeBuildRewritesTheBase(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": "3.11.9", "OTHER": "x\ny"}, nil)
	before, _ := os.ReadFile(dockerfile)
	choices, err := ResolveNativeBuild("python", bundle, dockerfile, src)
	if err != nil || len(choices) != 3 || choices[0].Line != "3.11" || choices[1].Label != "Poetry" || choices[2].Label != "Node" {
		t.Fatalf("resolve = %+v, %v", choices, err)
	}
	after, _ := os.ReadFile(dockerfile)
	py311, _ := nativeToolchains["python"].find("3.11")
	want := strings.Replace(string(before), "FROM "+nativeToolchains["python"].defaultImage()+"\n", "FROM "+py311.image+"\n"+
		nativeJSToolLines(nativeToolchains["node"].defaultImage())+
		"RUN /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install --no-cache-dir --disable-pip-version-check --no-input poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry\n", 1)
	if string(after) != want {
		t.Fatalf("Dockerfile =\n%s\nwant\n%s", after, want)
	}

	// Node reads its files from the service's root directory.
	o.Runtime = "node"
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n"})
	if choices, err := ResolveNativeBuild("node", bundle, dockerfile, src); err != nil || len(choices) != 1 || choices[0].Source != ".nvmrc" || choices[0].Line != "22" {
		t.Fatalf("node resolve = %+v, %v", choices, err)
	}
}

// w8/m73: choosing a Node line rewrites only the FROM, so every reviewed line
// keeps the generated pnpm layers in front of the authored build, alongside an
// opted-in Bun.
func TestResolveNativeBuildKeepsPnpmOnEveryNodeLine(t *testing.T) {
	for _, l := range nativeToolchains[nativeNodeRuntime].lines {
		env := map[string]string{"NODE_VERSION": l.line}
		if l.line == "26" {
			env["BUN_VERSION"] = "1.3"
		}
		bundle, dockerfile, src := writeResolveFixture(t, nativeOptions(), env, nil)
		before, _ := os.ReadFile(dockerfile)
		if _, err := ResolveNativeBuild(nativeNodeRuntime, bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(dockerfile)
		tools := nativeNodeTools(nativeOptions())
		if !strings.Contains(string(after), "FROM "+l.image+"\n") || !strings.Contains(string(after), tools) ||
			strings.Index(string(after), tools) > strings.Index(string(after), "\nCOPY . .\n") {
			t.Fatalf("node %s Dockerfile lost its pnpm layers:\n%s", l.line, after)
		}
		if l.line == "26" && !strings.Contains(string(after), "/usr/local/bin/bun\n") {
			t.Fatalf("Bun opt-in displaced:\n%s", after)
		}
		if l.line == "24" && string(after) != string(before) {
			t.Fatalf("the default line must leave the Dockerfile unchanged:\n%s", after)
		}
	}
}

// A version file that is a symlink is not followed: the refusal quotes what
// it read, and the env bundle (the tenant's secrets) sits beside the source.
func TestResolveNativeBuildIgnoresSymlinkedVersionFiles(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"SECRET": "s3cr3t"}, nil)
	must(t, os.Symlink(bundle, filepath.Join(src, ".python-version")))
	choices, err := ResolveNativeBuild("python", bundle, dockerfile, src)
	if err != nil || len(choices) != 3 || choices[0].Source != "" {
		t.Fatalf("symlinked .python-version: %+v, %v; want the default, unread", choices, err)
	}
}

func TestResolveNativeBuildRefusesAForeignDockerfile(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	must(t, os.WriteFile(dockerfile, []byte("FROM evil:latest\nRUN true\n"), 0o600))
	if _, err := ResolveNativeBuild("python", bundle, dockerfile, src); err == nil {
		t.Fatal("rewrote a Dockerfile the preparer did not generate")
	}
}

// The resolve phase runs after the preparer and before BuildKit, only for
// native builds with a resolver image, read-only on the checkout.
func TestNativeBuildJobRunsTheResolverAfterThePreparer(t *testing.T) {
	o := nativeOptions()
	o.NativeResolverImage = "ghcr.io/bex-co/bex-operator@sha256:" + strings.Repeat("a", 64)
	o.RootDir = "apps/web"
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	names := make([]string, 0, len(spec.InitContainers))
	var resolver corev1.Container
	for _, c := range spec.InitContainers {
		names = append(names, c.Name)
		if c.Name == nativeResolveContainer {
			resolver = c
		}
	}
	order := strings.Join(names, ",")
	if !strings.Contains(order, "prepare-native-build,"+nativeResolveContainer+",buildkit") {
		t.Fatalf("init order = %s", order)
	}
	if resolver.Image != o.NativeResolverImage || resolver.Command[0] != NativeResolveBinary {
		t.Fatalf("resolver = %+v", resolver)
	}
	for _, m := range resolver.VolumeMounts {
		if m.Name == "source" && !m.ReadOnly {
			t.Error("the resolver must not be able to write the checkout")
		}
	}
	env := map[string]string{}
	for _, e := range resolver.Env {
		env[e.Name] = e.Value
	}
	if env["BEX_NATIVE_SOURCE_DIR"] != "/source/apps/web" || env["BEX_NATIVE_RUNTIME"] != "node" {
		t.Fatalf("resolver env = %v", env)
	}

	o.NativeResolverImage = ""
	for _, c := range BuildJob(o, o.ImageRef()).Spec.Template.Spec.InitContainers {
		if c.Name == nativeResolveContainer {
			t.Fatal("no resolver image must mean no resolve phase")
		}
	}
}

// Bun is an opt-in companion (w8/m71), resolved with Render's precedence; no
// signal leaves the build without it.
func TestResolveNativeBun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		files   map[string]string
		line    string // "" = not selected
		source  string
		wantErr string
	}{
		{"sweep306 BUN_VERSION", map[string]string{"BUN_VERSION": "1.3.4"}, nil, "1.3", "BUN_VERSION=1.3.4", ""},
		{"BUN_VERSION wins over the file", map[string]string{"BUN_VERSION": "1.2.20"}, map[string]string{".bun-version": "1.3.0\n"}, "1.2", "BUN_VERSION=1.2.20", ""},
		{".bun-version", nil, map[string]string{".bun-version": "# pinned\nv1.2.21\n"}, "1.2", ".bun-version", ""},
		{"bare major", map[string]string{"BUN_VERSION": "1"}, nil, "1.3", "BUN_VERSION=1", ""},
		{"latest", map[string]string{"BUN_VERSION": "latest"}, nil, "1.3", "BUN_VERSION=latest", ""},
		{"range", map[string]string{"BUN_VERSION": "~1.2.0"}, nil, "1.2", "BUN_VERSION=~1.2.0", ""},
		{"text lockfile", nil, map[string]string{"bun.lock": "{}"}, "1.3", "bun.lock", ""},
		{"binary lockfile", nil, map[string]string{"bun.lockb": "\x00"}, "1.3", "bun.lockb", ""},
		{"file wins over a lockfile", nil, map[string]string{".bun-version": "1.2", "bun.lock": "{}"}, "1.2", ".bun-version", ""},
		{"no opt-in", map[string]string{"NODE_VERSION": "22"}, map[string]string{"package.json": "{}"}, "", "", ""},
		{"unsupported", map[string]string{"BUN_VERSION": "0.8.1"}, nil, "", "", "BUN_VERSION=0.8.1 is not a supported Bun version; supported Bun lines are 1.2, 1.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readFile := func(name string) ([]byte, bool) {
				v, ok := tc.files[name]
				return []byte(v), ok
			}
			choice, found, err := ResolveNativeBun(tc.env, readFile)
			if tc.wantErr != "" {
				var unsupported *UnsupportedVersionError
				if !errors.As(err, &unsupported) || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || found != (tc.line != "") || choice.Line != tc.line || choice.Source != tc.source {
				t.Fatalf("= %+v, %v, %v; want line %q from %q", choice, found, err, tc.line, tc.source)
			}
		})
	}
	choice, _, _ := ResolveNativeBun(map[string]string{"BUN_VERSION": "1.3.4"}, func(string) ([]byte, bool) { return nil, false })
	if got := choice.Narration(); got != "==> Using Bun 1.3 (from BUN_VERSION=1.3.4)" {
		t.Errorf("narration = %q", got)
	}
}

// An opted-in Bun is copied into the toolchain image after its FROM and
// before the source and the authored build; the primary choice still
// rewrites the base, and a refused Bun request leaves the Dockerfile alone.
func TestResolveNativeBuildAddsAnOptedInBun(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "node"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "1.3.4"}, map[string]string{".nvmrc": "22\n"})
	choices, err := ResolveNativeBuild("node", bundle, dockerfile, src)
	if err != nil || len(choices) != 2 || choices[0].Label != "Node" || choices[0].Line != "22" || choices[1].Label != "Bun" || choices[1].Line != "1.3" {
		t.Fatalf("choices = %+v, %v; want Node 22 then Bun 1.3", choices, err)
	}
	df, _ := os.ReadFile(dockerfile)
	node22, _ := nativeToolchains["node"].find("22")
	bun13, _ := nativeBun.find("1.3")
	from := strings.Index(string(df), "\nFROM "+node22.image+"\n")
	copyBun := strings.Index(string(df), "\nCOPY --link --from="+bun13.image+" /usr/local/bin/bun /usr/local/bin/bun\nRUN ln -sf bun /usr/local/bin/bunx\n")
	source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
	if from < 0 || copyBun < from || source < copyBun || build < source {
		t.Fatalf("Bun is not copied between FROM and the source/build:\n%s", df)
	}

	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"BUN_VERSION": "0.8"}, nil)
	before, _ := os.ReadFile(dockerfile)
	var unsupported *UnsupportedVersionError
	if _, err := ResolveNativeBuild("node", bundle, dockerfile, src); !errors.As(err, &unsupported) {
		t.Fatalf("unsupported BUN_VERSION = %v, want a tenant UnsupportedVersionError", err)
	}
	if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
		t.Error("a refused Bun request rewrote the Dockerfile")
	}

	// No opt-in: exactly one choice and the Dockerfile byte-identical.
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, nil)
	before, _ = os.ReadFile(dockerfile)
	if choices, err := ResolveNativeBuild("node", bundle, dockerfile, src); err != nil || len(choices) != 1 {
		t.Fatalf("no opt-in choices = %+v, %v", choices, err)
	}
	if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
		t.Fatalf("no opt-in changed the Dockerfile:\n%s", after)
	}
}

// Every native Python build carries Poetry (w8/m72, Render's Poetry version
// contract): the default release, or POETRY_VERSION's exact one; anything
// else is refused by name before the build runs.
func TestResolveNativePoetry(t *testing.T) {
	for _, tc := range []struct {
		name, request, version, source, wantErr string
	}{
		{"default", "", "2.1.3", "", ""},
		{"sweep307 override", "2.1.3", "2.1.3", "POETRY_VERSION=2.1.3", ""},
		{"older 1.x", "1.7.1", "1.7.1", "POETRY_VERSION=1.7.1", ""},
		{"v prefix", " v2.0.1 ", "2.0.1", "POETRY_VERSION=v2.0.1", ""},
		{"range", ">=2.0", "", "", "POETRY_VERSION=>=2.0 is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as 2.1.3"},
		{"major only", "2", "", "", "POETRY_VERSION=2 is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as 2.1.3"},
		{"shell text", "2.1.3; curl x", "", "", "POETRY_VERSION=2.1.3; curl x is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as 2.1.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			choice, err := ResolveNativePoetry(map[string]string{"POETRY_VERSION": tc.request, "PYTHON_VERSION": "3.12"})
			if tc.wantErr != "" {
				var unsupported *UnsupportedVersionError
				if !errors.As(err, &unsupported) || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || choice.Label != "Poetry" || choice.Line != tc.version || choice.Source != tc.source {
				t.Fatalf("= %+v, %v; want %s from %q", choice, err, tc.version, tc.source)
			}
		})
	}
	choice, _ := ResolveNativePoetry(map[string]string{"POETRY_VERSION": "2.1.3"})
	if got := choice.Narration(); got != "==> Using Poetry 2.1.3 (from POETRY_VERSION=2.1.3)" {
		t.Errorf("narration = %q", got)
	}
}

// The Poetry install lands between the selected Python's FROM and the
// source/build, on every Python line; a refused request leaves the
// Dockerfile alone; other runtimes never gain it.
func TestResolveNativeBuildAddsPoetryToPython(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	for _, line := range nativeToolchains["python"].lines {
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": line.line, "POETRY_VERSION": "2.1.3"}, nil)
		choices, err := ResolveNativeBuild("python", bundle, dockerfile, src)
		if err != nil || len(choices) != 3 || choices[0].Line != line.line || choices[1].Line != "2.1.3" {
			t.Fatalf("python %s choices = %+v, %v", line.line, choices, err)
		}
		df, _ := os.ReadFile(dockerfile)
		from := strings.Index(string(df), "\nFROM "+line.image+"\n")
		poetry := strings.Index(string(df), "\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && ")
		source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
		if from < 0 || poetry < from || source < poetry || build < source || !strings.Contains(string(df), "poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry\n") {
			t.Fatalf("python %s: Poetry is not installed between FROM and the source/build:\n%s", line.line, df)
		}
	}

	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"POETRY_VERSION": "latest"}, nil)
	before, _ := os.ReadFile(dockerfile)
	var unsupported *UnsupportedVersionError
	if _, err := ResolveNativeBuild("python", bundle, dockerfile, src); !errors.As(err, &unsupported) {
		t.Fatalf("unsupported POETRY_VERSION = %v, want a tenant UnsupportedVersionError", err)
	}
	if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
		t.Error("a refused Poetry request rewrote the Dockerfile")
	}

	o.Runtime = "node"
	bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"POETRY_VERSION": "2.1.3"}, nil)
	if choices, err := ResolveNativeBuild("node", bundle, dockerfile, src); err != nil || len(choices) != 1 {
		t.Fatalf("node choices = %+v, %v; Poetry is Python's", choices, err)
	}
	if df, _ := os.ReadFile(dockerfile); strings.Contains(string(df), "poetry") {
		t.Fatalf("node build gained Poetry:\n%s", df)
	}
}

// Ruby and Python builds carry Render's JavaScript tools (w8/m77): the
// selected Node line's node/npm, Yarn and pnpm land between the primary FROM
// and the source/build on every primary line, NODE_VERSION picks the Node
// line, a refused Node request leaves the Dockerfile alone, and runtimes
// outside the set gain nothing.
func TestResolveNativeBuildAddsJSToolsToRubyAndPython(t *testing.T) {
	for _, runtime := range []string{"ruby", "python"} {
		o := nativeOptions()
		o.Runtime = runtime
		for _, line := range nativeToolchains[runtime].lines {
			versionEnv := map[string]string{"python": "PYTHON_VERSION"}[runtime]
			env := map[string]string{}
			files := map[string]string{}
			if versionEnv != "" {
				env[versionEnv] = line.line
			} else {
				files[".ruby-version"] = line.version + "\n"
			}
			bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
			choices, err := ResolveNativeBuild(runtime, bundle, dockerfile, src)
			if err != nil || choices[0].Line != line.line || choices[len(choices)-1].Label != "Node" || choices[len(choices)-1].Line != "24" {
				t.Fatalf("%s %s choices = %+v, %v", runtime, line.line, choices, err)
			}
			df, _ := os.ReadFile(dockerfile)
			from := strings.Index(string(df), "\nFROM "+line.image+"\n")
			tools := strings.Index(string(df), "\n"+nativeJSToolLines(nativeToolchains["node"].defaultImage()))
			source, build := strings.Index(string(df), "\nCOPY . .\n"), strings.Index(string(df), "\nRUN --mount=type=secret")
			if from < 0 || tools < from || source < tools || build < source {
				t.Fatalf("%s %s: JS tools are not between FROM and the source/build:\n%s", runtime, line.line, df)
			}
		}

		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": "22"}, nil)
		choices, err := ResolveNativeBuild(runtime, bundle, dockerfile, src)
		node22, _ := nativeToolchains["node"].find("22")
		if df, _ := os.ReadFile(dockerfile); err != nil || choices[len(choices)-1].Source != "NODE_VERSION=22" ||
			!strings.Contains(string(df), "COPY --link --from="+node22.image+" /usr/local/bin/node /usr/local/bin/node\n") {
			t.Fatalf("%s NODE_VERSION=22: %+v, %v\n%s", runtime, choices, err, df)
		}

		bundle, dockerfile, src = writeResolveFixture(t, o, map[string]string{"NODE_VERSION": "12"}, nil)
		before, _ := os.ReadFile(dockerfile)
		var unsupported *UnsupportedVersionError
		if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); !errors.As(err, &unsupported) {
			t.Fatalf("%s unsupported NODE_VERSION = %v", runtime, err)
		}
		if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
			t.Errorf("%s: a refused Node request rewrote the Dockerfile", runtime)
		}
	}
	for _, runtime := range []string{"go", "rust", "elixir"} {
		o := nativeOptions()
		o.Runtime = runtime
		bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
		if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		if df, _ := os.ReadFile(dockerfile); strings.Contains(string(df), "/usr/local/bin/node") {
			t.Fatalf("%s gained the JS companion:\n%s", runtime, df)
		}
	}
}
