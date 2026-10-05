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
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			if !committed[line.image] {
				t.Errorf("%s line %s image %s is not in toolchain-freshness.json", runtime, line.line, line.image)
			}
		}
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
	choice, err := ResolveNativeBuild("python", bundle, dockerfile, src)
	if err != nil || choice.Line != "3.11" {
		t.Fatalf("resolve = %+v, %v", choice, err)
	}
	after, _ := os.ReadFile(dockerfile)
	py311, _ := nativeToolchains["python"].find("3.11")
	want := strings.Replace(string(before), "FROM "+nativeToolchains["python"].defaultImage(), "FROM "+py311.image, 1)
	if string(after) != want {
		t.Fatalf("Dockerfile =\n%s\nwant\n%s", after, want)
	}

	// Node reads its files from the service's root directory.
	o.Runtime = "node"
	bundle, dockerfile, src = writeResolveFixture(t, o, nil, map[string]string{".nvmrc": "22\n"})
	if choice, err := ResolveNativeBuild("node", bundle, dockerfile, src); err != nil || choice.Source != ".nvmrc" || choice.Line != "22" {
		t.Fatalf("node resolve = %+v, %v", choice, err)
	}
}

// A version file that is a symlink is not followed: the refusal quotes what
// it read, and the env bundle (the tenant's secrets) sits beside the source.
func TestResolveNativeBuildIgnoresSymlinkedVersionFiles(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"SECRET": "s3cr3t"}, nil)
	must(t, os.Symlink(bundle, filepath.Join(src, ".python-version")))
	choice, err := ResolveNativeBuild("python", bundle, dockerfile, src)
	if err != nil || choice.Source != "" {
		t.Fatalf("symlinked .python-version: %+v, %v; want the default, unread", choice, err)
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
