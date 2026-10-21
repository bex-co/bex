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

package apps

import "testing"

// Render's minio template sets dockerContext: / for the repository root
// (w5/172).
func TestRepoRootRelativeReadsALeadingSlashAsTheRepositoryRoot(t *testing.T) {
	for in, want := range map[string]string{
		"/":            ".",
		"/docker":      "docker",
		"//docker/sub": "docker/sub",
		"docker":       "docker",
		"":             "",
		"/../escape":   "../escape",
	} {
		if got := repoRootRelative(in); got != want {
			t.Errorf("repoRootRelative(%q) = %q, want %q", in, got, want)
		}
	}
	if err := checkDockerContextPath(repoRootRelative("/../escape")); err == nil {
		t.Error("a '..' component must still be refused after the leading slash goes")
	}
}
