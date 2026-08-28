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

func TestValidAbsoluteHTTPURL(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"https://status.example.com/maintenance", true},
		{"http://example.com", true},
		{"  https://example.com/x  ", true}, // trimmed
		{"", false},
		{"ftp://example.com", false},
		{"example.com", false},         // no scheme
		{"https://", false},            // no host
		{"javascript:alert(1)", false}, // not http(s)
		// codex-security target #7: userinfo must be rejected so a credential
		// can't be stored + replayed, and an allowlist can't be confused by
		// authority ambiguity.
		{"https://user:pass@evil.example.com/", false},
		{"https://trusted-host@evil.example.com/", false},
		{"http://user@internal.svc/", false},
	}
	for _, c := range cases {
		if got := ValidAbsoluteHTTPURL(c.raw); got != c.want {
			t.Errorf("ValidAbsoluteHTTPURL(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}
