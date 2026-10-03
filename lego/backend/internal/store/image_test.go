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

import (
	"strings"
	"testing"
)

func TestImageRepositoryNormalizesDockerHubAndDropsVersion(t *testing.T) {
	for in, want := range map[string]string{
		"nginx":                         "docker.io/library/nginx",
		"nginx:1.27":                    "docker.io/library/nginx",
		"docker.io/library/nginx:1.27":  "docker.io/library/nginx",
		"index.docker.io/library/nginx": "docker.io/library/nginx",
		"traefik/whoami@sha256:" + strings.Repeat("a", 64): "docker.io/traefik/whoami",
		"ghcr.io/acme/web:v1":                              "ghcr.io/acme/web",
		"GHCR.io/acme/web:v1":                              "ghcr.io/acme/web",
		"zot.bex-registry.svc:5000/t/app:3":                "zot.bex-registry.svc:5000/t/app",
	} {
		if got, err := ImageRepository(in); err != nil || got != want {
			t.Errorf("ImageRepository(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ImageRepository("Bad Ref"); err == nil {
		t.Error("malformed reference accepted")
	}
}
