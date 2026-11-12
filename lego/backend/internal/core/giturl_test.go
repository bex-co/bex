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

package core

import "testing"

func TestCanonicalRepo(t *testing.T) {
	// Every form of the same repo canonicalizes to one key.
	want := "github.com/octo/app"
	for _, u := range []string{
		"https://github.com/octo/app",
		"https://github.com/octo/app.git",
		"HTTPS://GitHub.com/Octo/App.git",
		"git@github.com:octo/app.git",
		"ssh://git@github.com/octo/app",
		"git://github.com/octo/app.git",
		"https://github.com/octo/app/",
	} {
		if got := CanonicalRepo(u); got != want {
			t.Errorf("CanonicalRepo(%q) = %q, want %q", u, got, want)
		}
	}
	// A different repo canonicalizes differently.
	if CanonicalRepo("https://github.com/octo/other") == want {
		t.Error("distinct repos must not collide")
	}
}

func TestValidGitRefName(t *testing.T) {
	// Git accepts non-ASCII names, NBSP and NEL included, and so does the
	// public-Git ref check; the agent branch gate refuses those on top.
	for _, ref := range []string{"main", "feature/fix-login", "release-1.2", "bex-agent/ags-1/fix", "refs/heads/x", "users/a_b", "a\u00a0b", "a\u0085b"} {
		if !ValidGitRefName(ref) {
			t.Errorf("ValidGitRefName(%q) = false, want true", ref)
		}
	}
	for _, ref := range []string{"", "/x", "x/", "a//b", ".x", "a/.b", "a.", "a.lock", "a/b.lock/c", "a..b", "a@{1}",
		"a b", "a\tb", "a~1", "a^", "a:b", "a?", "a*", "a[", `a\b`, "a\x7fb"} {
		if ValidGitRefName(ref) {
			t.Errorf("ValidGitRefName(%q) = true, want false", ref)
		}
	}
}
