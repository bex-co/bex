/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"os"
	"strings"
	"testing"
)

// permittedPostgresVersions derives the CRD enum for DatabaseSpec.Version —
// the same source as the Valkey pin guard — so adding a major without a digest
// fails CI rather than waiting for another security scan.
func permittedPostgresVersions(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("../../../types/v1alpha1/database_types.go")
	if err != nil {
		t.Fatalf("read database_types.go: %v", err)
	}
	idx := strings.Index(string(src), "Version string `json:\"version,omitempty\"`")
	if idx < 0 {
		t.Fatal("DatabaseSpec.Version not found — did the field move?")
	}
	m := permittedVersionsRE.FindAllStringSubmatch(string(src)[:idx], -1)
	if len(m) == 0 {
		t.Fatal("no kubebuilder enum marker found before DatabaseSpec.Version")
	}
	var out []string
	for raw := range strings.SplitSeq(m[len(m)-1][1], ";") {
		if v := strings.Trim(strings.TrimSpace(raw), `"`); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func TestCNPGExportImagesArePinnedForEveryPermittedVersion(t *testing.T) {
	versions := permittedPostgresVersions(t)
	if len(versions) == 0 {
		t.Fatal("parsed an empty permitted-version set — the guard would be vacuous")
	}
	t.Logf("permitted Database.spec.version values: %v", versions)

	for _, v := range versions {
		image, ok := cnpgExportImages[v]
		if !ok {
			t.Errorf("spec.version=%q is permitted by the CRD enum but cnpgExportImages has no entry.\n"+
				"Resolve the digest (docker buildx imagetools inspect ghcr.io/cloudnative-pg/postgresql:%s) and add it.", v, v)
			continue
		}
		if !strings.Contains(image, "@sha256:") {
			t.Errorf("cnpgExportImages[%q] = %q is not digest-pinned", v, image)
		}
		if got := cnpgExportImage(v); got != image {
			t.Errorf("cnpgExportImage(%q) = %q, want %q", v, got, image)
		}
	}
}

func TestCNPGExportImageNeverComposesAMutableTag(t *testing.T) {
	for _, v := range []string{"", "16", "18", "19", "unknown", "latest"} {
		got := cnpgExportImage(v)
		if !strings.Contains(got, "@sha256:") {
			t.Errorf("cnpgExportImage(%q) = %q — not digest-pinned", v, got)
		}
	}
	if got := cnpgExportImage("19"); got != cnpgExportImages[logicalExportClientVersion] {
		t.Errorf("cnpgExportImage(unrecognized) = %q, want the pinned default client", got)
	}
}
