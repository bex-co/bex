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

package v1alpha1

import "testing"

// TestBuildPlaneSecretNames pins the two on-the-wire conventions. They are not
// free to change: bex-api has already written Secrets under these names into
// every tenant namespace, an operator running a different release must accept
// the names the backend of its own release writes, and the operator's
// self-reference carve-out (w6/m97) recognizes an App's own build-plane Secret
// by deriving exactly these strings from the App's name.
func TestBuildPlaneSecretNames(t *testing.T) {
	for _, tc := range []struct{ name, got, want string }{
		{"clone/short", CloneSecretName("web"), "web-clone"},
		{"clone/tenant CR name", CloneSecretName("tea-d98210cbbpdc73dcrkvg-qa-web"), "tea-d98210cbbpdc73dcrkvg-qa-web-clone"},
		{"pull/short", ExternalRegistryPullSecretName("web"), "web-registry-pull"},
		{"pull/tenant CR name", ExternalRegistryPullSecretName("tea-d98210cbbpdc73dcrkvg-qa-web"), "tea-d98210cbbpdc73dcrkvg-qa-web-registry-pull"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
