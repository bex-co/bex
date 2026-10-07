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

package build

import (
	"maps"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestJobNameCrossModuleContract pins the exact Job names this operator
// creates, so a future re-implementation on either side of the CR boundary
// fails here rather than silently orphaning in-flight builds. Changing these
// literals is a deliberate migration, not a test update.
func TestJobNameCrossModuleContract(t *testing.T) {
	cases := []struct{ name, app, revision, want string }{
		{"short name", "web", "gen-3", "bld-web-gen-3"},
		{"uppercase is lowered", "Web", "gen-3", "bld-web-gen-3"},
		{"empty revision defaults", "web", "", "bld-web-latest"},
		{"just under the limit", strings.Repeat("a", 53), "gen-1", "bld-" + strings.Repeat("a", 53) + "-gen-1"},
		{"just over the limit", strings.Repeat("a", 54), "gen-1", "bld-" + strings.Repeat("a", 46) + "-cffb98e74742"},
		{"far over the limit", strings.Repeat("b", 70), "gen-42", "bld-" + strings.Repeat("b", 46) + "-de75591f8608"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := JobName(c.app, c.revision)
			if got != c.want {
				t.Fatalf("JobName(%q, %q) = %q, want %q — bex-api pins this exact literal", c.app, c.revision, got, c.want)
			}
			if len(got) > 63 {
				t.Fatalf("name %q is %d chars, exceeds the 63-char DNS label limit", got, len(got))
			}
		})
	}
}

// TestKpackImageReleaseLabelsCrossModuleContract pins the other half: a kpack
// Image's name hashes the App's UID, so bex-api cancels a buildpack release by
// selecting its Image with appv1alpha1.ReleaseBuildLabels. The Image this
// operator creates for a release matches that selector, and a later release's
// does not. The keys are persisted on in-flight Images, so changing their
// literals is a migration too (w5/136).
func TestKpackImageReleaseLabelsCrossModuleContract(t *testing.T) {
	want := map[string]string{"app.bex.co/build": "web", "app.bex.co/app-uid": "uid-1", "app.bex.co/build-revision": "gen-3"}
	if got := appv1alpha1.ReleaseBuildLabels("web", "uid-1", "gen-3"); !maps.Equal(got, want) {
		t.Fatalf("ReleaseBuildLabels = %v, want %v", got, want)
	}
	release := opts()
	release.Revision = appv1alpha1.BuildRevision(3)
	selector := labels.SelectorFromSet(appv1alpha1.ReleaseBuildLabels(release.Name, release.AppUID, release.Revision))
	if image := KpackImage(release); !selector.Matches(labels.Set(image.GetLabels())) {
		t.Fatalf("release 3's Image %s has labels %v, which its cancel selector %v misses", image.GetName(), image.GetLabels(), selector)
	}
	later := release
	later.Revision = appv1alpha1.BuildRevision(4)
	if selector.Matches(labels.Set(KpackImage(later).GetLabels())) {
		t.Fatal("release 3's cancel selector matches release 4's Image")
	}
}
