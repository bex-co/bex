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

import "testing"

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
