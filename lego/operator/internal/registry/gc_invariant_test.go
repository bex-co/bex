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
package registry

import (
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

// TestGCDelayOutlivesTheLongestPossiblePush asserts the ADR060 D4 invariant:
// Zot's garbage collector must not be able to remove a dangling blob while the
// build pushing it could still be running. If gcDelay drops below the build
// deadline, GC can collect blobs from an in-flight push and the failure
// presents as a corrupt image rather than as a configuration mistake.
func TestGCDelayOutlivesTheLongestPossiblePush(t *testing.T) {
	delay, err := time.ParseDuration(zotGCDelay)
	if err != nil {
		t.Fatalf("zotGCDelay %q is not a duration: %v", zotGCDelay, err)
	}
	if delay < build.BuildTimeout {
		t.Errorf("zot gcDelay %s < build deadline %s: GC can collect blobs from an in-flight push",
			delay, build.BuildTimeout)
	}
}
