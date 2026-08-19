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
	"regexp"
	"strings"
	"testing"
)

// keyvalue_image_pin_test.go guards the CLASS, not a sample of it (w1/m73).
//
// The digest-pinning inventory was re-reported by seven consecutive security
// rounds partly because each pass pinned the images it happened to touch and
// asserted those. A test that names the images it knows about cannot fail when
// somebody adds an eighth — so this one derives the expectation from the CRD
// instead: every Valkey major a tenant is PERMITTED to ask for must have a
// pinned digest recorded for it.

// permittedVersionsRE reads the kubebuilder enum marker off KeyValueSpec.Version
// — the same line the generated CRD schema is built from, so the test cannot
// drift from what the API actually accepts.
var permittedVersionsRE = regexp.MustCompile(`\+kubebuilder:validation:Enum=([^\n]+)`)

func permittedValkeyVersions(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("../../../types/v1alpha1/keyvalue_types.go")
	if err != nil {
		t.Fatalf("read keyvalue_types.go: %v", err)
	}
	// The Version field's marker is the one immediately preceding `Version string`.
	idx := strings.Index(string(src), "Version string `json:\"version,omitempty\"`")
	if idx < 0 {
		t.Fatal("KeyValueSpec.Version not found — did the field move?")
	}
	m := permittedVersionsRE.FindAllStringSubmatch(string(src)[:idx], -1)
	if len(m) == 0 {
		t.Fatal("no kubebuilder enum marker found before KeyValueSpec.Version")
	}
	var out []string
	for raw := range strings.SplitSeq(m[len(m)-1][1], ";") {
		if v := strings.Trim(strings.TrimSpace(raw), `"`); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func TestValkeyImagesArePinnedForEveryPermittedVersion(t *testing.T) {
	versions := permittedValkeyVersions(t)
	if len(versions) == 0 {
		t.Fatal("parsed an empty permitted-version set — the guard would be vacuous")
	}
	t.Logf("permitted spec.version values: %v", versions)

	for _, v := range versions {
		image, ok := kvVersionImages[v]
		if !ok {
			t.Errorf("spec.version=%q is permitted by the CRD enum but kvVersionImages has no entry — a tenant asking for it would run an unpinned tag.\n"+
				"Resolve the digest (docker buildx imagetools inspect valkey/valkey:%s-alpine --format '{{.Manifest.Digest}}') and add it.", v, v)
			continue
		}
		if !strings.Contains(image, "@sha256:") {
			t.Errorf("kvVersionImages[%q] = %q is not digest-pinned", v, image)
		}
		// And the resolver must actually return it.
		if got := valkeyImage(v); got != image {
			t.Errorf("valkeyImage(%q) = %q, want %q", v, got, image)
		}
	}
}

// TestValkeyImageNeverComposesAMutableTag: the resolver has no path that
// concatenates a version into a tag. An unrecognized version — which the CRD
// enum should make unreachable — falls back to the pinned default rather than
// to whatever `valkey:<version>-alpine` resolves to today.
func TestValkeyImageNeverComposesAMutableTag(t *testing.T) {
	for _, v := range []string{"", "8", "7", "9", "unknown", "latest", "8-alpine"} {
		got := valkeyImage(v)
		if !strings.Contains(got, "@sha256:") {
			t.Errorf("valkeyImage(%q) = %q — not digest-pinned", v, got)
		}
	}
	if got := valkeyImage("9"); got != kvDefaultImage {
		t.Errorf("valkeyImage(unrecognized) = %q, want the pinned default", got)
	}
}

// TestKeyValueSidecarImageIsPinned covers the other half of the tenant pod: the
// exporter runs beside the datastore and is handed its password, so it is code
// with the same reach as the datastore itself.
func TestKeyValueSidecarImageIsPinned(t *testing.T) {
	if !strings.Contains(kvExporterImage, "@sha256:") {
		t.Errorf("kvExporterImage = %q is not digest-pinned", kvExporterImage)
	}
	if !strings.Contains(kvDefaultImage, "@sha256:") {
		t.Errorf("kvDefaultImage = %q is not digest-pinned", kvDefaultImage)
	}
}
