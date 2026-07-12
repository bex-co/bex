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
