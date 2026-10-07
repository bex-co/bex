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

package controller

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestKeyValueReasonsAreClassified (w5/m129): every reason the Key Value
// controller writes is a lego/types constant, and is either one it fails a
// store with (appv1alpha1.KeyValueFailedReasons, each of which bex-api
// explains to the tenant) or one that never fails it. A new reason has to be
// placed in one of the two, so it cannot reach a tenant as an unexplained
// failure.
func TestKeyValueReasonsAreClassified(t *testing.T) {
	notFailures := map[string]bool{
		appv1alpha1.ReasonProvisioning:               true,
		appv1alpha1.ReasonPodUnready:                 true,
		appv1alpha1.ReasonPersistenceTransition:      true,
		appv1alpha1.ReasonWaitingForPVC:              true,
		appv1alpha1.ReasonWaitingForPVCBinding:       true,
		appv1alpha1.ReasonPVCResizePending:           true,
		appv1alpha1.ReasonFileSystemResizePending:    true,
		appv1alpha1.ReasonStorageProvisioned:         true,
		appv1alpha1.ReasonConnectionSecretRebuilding: true,
		appv1alpha1.ReasonProvisioned:                true,
		appv1alpha1.ReasonSuspended:                  true,
	}
	failed := map[string]bool{}
	for _, reason := range appv1alpha1.KeyValueFailedReasons {
		failed[reason] = true
	}
	files, err := filepath.Glob("keyvalue*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Key Value controller files: %v", err)
	}
	// A reason written as a literal escapes the classification below.
	literal := regexp.MustCompile(`(?:kvFail\([^)]*, |[Rr]eason(?:, message)? *:?= *|[Rr]eason: *|return )"([A-Z][A-Za-z]+)"`)
	constant := regexp.MustCompile(`appv1alpha1\.Reason(\w+)`)
	seen := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range literal.FindAllSubmatch(src, -1) {
			t.Errorf("%s writes the reason %q as a literal, not a lego/types constant", file, m[1])
		}
		for _, m := range constant.FindAllSubmatch(src, -1) {
			// Every reason constant spells its own value: ReasonFoo = "Foo".
			reason := string(m[1])
			seen++
			if failed[reason] == notFailures[reason] {
				t.Errorf("%s writes %q, which is in neither or both of KeyValueFailedReasons and this test's non-failures", file, reason)
			}
		}
	}
	if seen < 20 {
		t.Fatalf("found only %d reason references; the scan is broken", seen)
	}
}
