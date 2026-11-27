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
	"strings"
	"testing"
)

// A reference create admits and the pre-create check blesses must resolve in
// the deploy's metadata read: bare, :tag, @digest and :tag@digest forms all
// split to the digest the preview verified, never to a repo carrying the tag.
func TestParseExternalImageReferenceAgreesWithCreateAndCheck(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name                            string
		image                           string
		host, repo, repoPath, reference string
	}{
		{"tag", "registry.test/acme/app:1",
			"registry.test", "acme/app", "registry.test/acme/app", "1"},
		{"digest", "registry.test/acme/app@" + digest,
			"registry.test", "acme/app", "registry.test/acme/app", digest},
		{"tag and digest", "registry.test/acme/app:1@" + digest,
			"registry.test", "acme/app", "registry.test/acme/app", digest},
		{"port host tag and digest", "registry.test:5000/acme/app:1@" + digest,
			"registry.test:5000", "acme/app", "registry.test:5000/acme/app", digest},
		{"docker hub tag and digest", "docker.io/library/nginx:1.25@" + digest,
			"registry-1.docker.io", "library/nginx", "docker.io/library/nginx", digest},
		{"docker hub short tag", "nginx:1.25",
			"registry-1.docker.io", "library/nginx", "nginx", "1.25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, repo, repoPath, reference, ok := parseExternalImageReference(tc.image)
			if !ok {
				t.Fatalf("parseExternalImageReference(%q) refused a reference create admits", tc.image)
			}
			if host != tc.host || repo != tc.repo || repoPath != tc.repoPath || reference != tc.reference {
				t.Fatalf("parseExternalImageReference(%q) = %q %q %q %q, want %q %q %q %q",
					tc.image, host, repo, repoPath, reference, tc.host, tc.repo, tc.repoPath, tc.reference)
			}
		})
	}
}
