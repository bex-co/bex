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

// The invite at-rest contract is sha256(token) (w1/041), not merely "some
// hash the lookups agree with": a pinned known vector fails an identity,
// truncated or weakened hash that would keep every invite path working.
func TestHashInviteTokenIsSHA256(t *testing.T) {
	if got, want := hashInviteToken("tok-late"), "b2e1ba9e7d363901dda8850fc26ae7a9355f633495edc6e5926d50c5d14f8cff"; got != want {
		t.Fatalf("hashInviteToken(%q) = %q, want sha256 hex %q", "tok-late", got, want)
	}
}
