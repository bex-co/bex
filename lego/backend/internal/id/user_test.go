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

package id

import "testing"

func TestUserPublicIDRoundTrips(t *testing.T) {
	own := New(Owner)
	public := UserPublicID(own)
	if public != "usr-"+own[len("own-"):] {
		t.Fatalf("UserPublicID(%q) = %q", own, public)
	}
	if got := UserStorageID(public); got != own {
		t.Fatalf("UserStorageID(%q) = %q, want %q", public, got, own)
	}
	for _, v := range []string{"", "usr-short", "user-a", New(Service), "3b1f8a2e-subject"} {
		if UserPublicID(v) != v || UserStorageID(v) != v {
			t.Errorf("%q must pass through unchanged", v)
		}
	}
	if UserStorageID(own) != own || UserPublicID(public) != public {
		t.Error("each helper leaves the other spelling alone")
	}
}
