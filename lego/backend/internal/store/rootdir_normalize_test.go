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

package store

import (
	"strings"
	"testing"
)

// One directory, one stored spelling (w8/165).
func TestNormalizeRootDir(t *testing.T) {
	for in, want := range map[string]string{
		"": "", ".": "", "./": "", "server": "server", "server/": "server", "./server": "server",
		"./server/": "server", "a//b": "a/b", "a/./b/": "a/b", "a/b/c": "a/b/c",
		// Left for ValidRootDir to refuse.
		"/server": "/server", "../x": "../x", "a/../b": "a/../b", `a\b`: `a\b`, "a\nb": "a\nb",
	} {
		if got := NormalizeRootDir(in); got != want {
			t.Errorf("NormalizeRootDir(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"/server", "../x", "a/../b", `a\b`} {
		if ValidRootDir(NormalizeRootDir(bad)) {
			t.Errorf("%q accepted after normalization", bad)
		}
	}
}

// w21/045: the dashboard's Settings rows mark a build path in the row, with the
// same rules and the same vectors (dashboard/src/features/services/lib/
// __tests__/build-settings-validation.test.ts). Each refusal names the rule that
// failed, so a path that is merely long is no longer blamed on "..".
func TestRootDirProblemVectors(t *testing.T) {
	// The Root Directory is judged after normalization.
	for _, ok := range []string{"", "./", "./server", "server/", "a//b", "trail/", "a b", "a/./b", strings.Repeat("a", 512), strings.Repeat("中", 170)} {
		if got := RootDirProblem(NormalizeRootDir(ok)); got != "" {
			t.Errorf("root dir %q refused: %s", ok, got)
		}
	}
	for in, want := range map[string]string{
		"/abs":                   "relative to the repository",
		"../x":                   "'..'",
		"a/../../b":              "'..'",
		"a/../b":                 "'..'",
		`src\app`:                "backslash",
		"a\tb":                   "control character",
		"a\nb":                   "control character",
		strings.Repeat("a", 513): "512 bytes",
		strings.Repeat("中", 171): "512 bytes",
	} {
		if got := RootDirProblem(NormalizeRootDir(in)); !strings.Contains(got, want) {
			t.Errorf("root dir %q => %q, want it to name %q", in, got, want)
		}
	}
	// The Dockerfile Path and the Docker Build Context are judged as typed: an
	// empty segment is refused where the Root Directory drops it.
	for _, ok := range []string{"", ".", "./Dockerfile", "docker/Dockerfile", "a b", "ctx"} {
		if got := RootDirProblem(ok); got != "" {
			t.Errorf("path %q refused: %s", ok, got)
		}
	}
	for in, want := range map[string]string{
		"../Dockerfile":          "'..'",
		"../ctx":                 "'..'",
		"/Dockerfile":            "relative to the repository",
		`docker\Dockerfile`:      "backslash",
		"a//b":                   "empty path segment",
		"docker/":                "empty path segment",
		"./":                     "empty path segment",
		"a\x00b":                 "control character",
		strings.Repeat("a", 513): "512 bytes",
	} {
		if got := RootDirProblem(in); !strings.Contains(got, want) {
			t.Errorf("path %q => %q, want it to name %q", in, got, want)
		}
	}
	// ValidRootDir is the same rule.
	if ValidRootDir("a//b") || !ValidRootDir("./a") {
		t.Error("ValidRootDir disagrees with RootDirProblem")
	}
}
