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

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invoke runs the command over dir with env, as the driver does.
func invoke(t *testing.T, dir string, env map[string]string, args ...string) (selection, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"python"}, append(args, dir)...), func(k string) string { return env[k] }, &stdout, &stderr)
	var sel selection
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &sel); err != nil {
			t.Fatalf("stdout is not one selection: %v\n%s", err, stdout.String())
		}
	}
	return sel, strings.TrimSpace(stderr.String()), code
}

func checkout(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const created = "2026-11-20T09:30:00Z"

// The DoD's cases (w4/m544): each source, the exact patch and the default.
func TestAgentToolchainPython(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   map[string]string
		files map[string]string
		want  selection
	}{
		{
			name: "PYTHON_VERSION pins a patch the image lacks", env: map[string]string{"PYTHON_VERSION": "3.11.9"}, files: map[string]string{".python-version": "3.13\n"},
			want: selection{Line: "3.11", Version: "3.11.9", Baked: "3.11.18", Image: "python:3.11.9-bookworm", Source: "PYTHON_VERSION",
				Narration: "==> Using Python 3.11.9 (from PYTHON_VERSION)", Fallback: "==> Using Python 3.11.18 (from PYTHON_VERSION)"},
		},
		{
			name: ".python-version names a line", files: map[string]string{".python-version": "3.13\n"},
			want: selection{Line: "3.13", Version: "3.13.18", Baked: "3.13.18", Source: ".python-version", Narration: "==> Using Python 3.13.18 (from .python-version)"},
		},
		{
			name: "no signal gets Render's dated default",
			want: selection{Line: "3.14", Version: "3.14.10", Baked: "3.14.10", Source: "default", Narration: "==> Using Python 3.14.10 (default)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{sessionCreatedEnv: created}
			for k, v := range tc.env {
				env[k] = v
			}
			got, stderr, code := invoke(t, checkout(t, tc.files), env)
			if code != 0 || got != tc.want {
				t.Fatalf("exit %d (%s): got %+v, want %+v", code, stderr, got, tc.want)
			}
		})
	}
}

// A session created before bex passed its creation time (or one whose value
// is malformed) keeps the legacy line, as a native build of unknown age does.
func TestAgentToolchainUnknownCreationKeepsTheLegacyLine(t *testing.T) {
	for _, value := range []string{"", "yesterday"} {
		got, _, code := invoke(t, checkout(t, nil), map[string]string{sessionCreatedEnv: value})
		if code != 0 || got.Version != "3.13.18" || got.Source != "default" {
			t.Errorf("created %q: exit %d, %+v; want the legacy 3.13.18", value, code, got)
		}
	}
}

// An unsupported request exits 2 naming it, and so does 3.7, which only
// bullseye builds carry; any other misuse is not a refusal.
func TestAgentToolchainRefusesByName(t *testing.T) {
	for _, tc := range []struct {
		env   map[string]string
		files map[string]string
		want  string
	}{
		{env: map[string]string{"PYTHON_VERSION": "2.7.18"}, want: "PYTHON_VERSION=2.7.18 is not a supported Python version; supported Python lines are 3.7, 3.8"},
		{files: map[string]string{".python-version": "2.7\n"}, want: `.python-version "2.7" is not a supported Python version`},
		{env: map[string]string{"PYTHON_VERSION": "3.7.17"}, want: "PYTHON_VERSION=3.7.17 is not a supported Python version in an agent session: Python 3.7's official build is for Debian bullseye, not the session image's bookworm; supported Python lines are 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15"},
	} {
		_, stderr, code := invoke(t, checkout(t, tc.files), tc.env)
		if code != 2 || !strings.HasPrefix(stderr, tc.want) {
			t.Errorf("env %v files %v: exit %d, stderr %q; want exit 2 with %q", tc.env, tc.files, code, stderr, tc.want)
		}
	}
	var stderr bytes.Buffer
	if code := run([]string{"node"}, func(string) string { return "" }, &bytes.Buffer{}, &stderr); code != 64 {
		t.Errorf("an unknown toolchain exits %d, want the usage code 64", code)
	}
}

// A symlinked .python-version is not read, as a native build does not read
// one: the checkout's own default applies.
func TestAgentToolchainIgnoresASymlinkedVersionFile(t *testing.T) {
	dir := checkout(t, map[string]string{"elsewhere": "3.12\n"})
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(dir, ".python-version")); err != nil {
		t.Fatal(err)
	}
	got, _, code := invoke(t, dir, map[string]string{sessionCreatedEnv: created})
	if code != 0 || got.Source != "default" {
		t.Fatalf("exit %d, %+v; want the default", code, got)
	}
}
