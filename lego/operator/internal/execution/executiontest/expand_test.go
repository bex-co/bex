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

package executiontest

import "testing"

func TestExpandMatchesTheKubeletContract(t *testing.T) {
	env := map[string]string{"PORT": "10000", "CHAIN": "$(PORT) $$", "": "empty"}
	for _, tc := range []struct{ input, want string }{
		{"plain unicode λ", "plain unicode λ"},
		{"$$", "$"},
		{"a$$b", "a$b"},
		{"$(PORT)", "10000"},
		{"$(UNKNOWN)", "$(UNKNOWN)"},
		{"$$(PORT)", "$(PORT)"},
		{"$$$(PORT)", "$10000"},
		{"$$$$(PORT)", "$$(PORT)"},
		{"$PORT ${PORT}", "$PORT ${PORT}"},
		{"$λ", "$λ"},
		{"$(CHAIN)", "$(PORT) $$"},
		{"prefix$", "prefix$"},
		{"$(", "$("},
		{"$(unfinished$$", "$(unfinished$"},
		{"$($(PORT))", "$($(PORT))"},
		{"$()", "empty"},
		{"$(PORT)$(PORT)", "1000010000"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			if got := Expand(tc.input, env); got != tc.want {
				t.Fatalf("Expand(%q)=%q, want%q", tc.input, got, tc.want)
			}
		})
	}
}
