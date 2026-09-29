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

// The operator writes these names and bex-api reads them; both sides call this
// package, so the contract is pinned once, here.
func TestReleaseSnapshotNamesAgreeWithTheirPredicate(t *testing.T) {
	for _, name := range []string{
		ReleaseSnapshotName("api-env", 3),
		ReleaseSnapshotName("evg-shared-files", 120),
		ReleaseRecordName("api", 7),
	} {
		if !IsReleaseSnapshotName(name) {
			t.Errorf("%q is a snapshot name the predicate does not recognize", name)
		}
	}
	if got := ReleaseRecordName("api", 7); got != "api-podtemplate-r7" {
		t.Errorf("ReleaseRecordName = %q, want api-podtemplate-r7 — changing it orphans every recorded release", got)
	}
}

func TestIsReleaseSnapshotNameRejectsLookalikes(t *testing.T) {
	for name, want := range map[string]bool{
		"api-env-r3":        true,
		"evg-x-files-r120":  true,
		"api-env":           false,
		"api-env-r":         false,
		"my-rr-env":         false,
		"api-env-rollback":  false,
		"-r3":               false, // no source before the suffix
		"api-env-r3-backup": false,
	} {
		if got := IsReleaseSnapshotName(name); got != want {
			t.Errorf("IsReleaseSnapshotName(%q) = %v, want %v", name, got, want)
		}
	}
}
