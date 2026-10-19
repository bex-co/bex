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

func TestPublicImageRefStripsOnlyClusterInternalHosts(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for in, want := range map[string]string{
		// Cluster-internal build registry: host dropped, tag+digest kept.
		"zot.bex-registry.svc:5000/ws/ws-svc:gen-2@" + digest:  "ws/ws-svc:gen-2@" + digest,
		"zot.bex-registry.svc.cluster.local:5000/ws/app:gen-1": "ws/app:gen-1",
		"registry.bex-system.svc/ws/app:gen-3":                 "ws/app:gen-3",
		"mirror.corp.cluster.local/app:1":                      "app:1",
		"ZOT.BEX-REGISTRY.SVC:5000/ws/app:gen-1":               "ws/app:gen-1",
		"zot.bex-registry.svc.:5000/ws/app:gen-1":              "ws/app:gen-1",
		// User-supplied images pass through byte-identical.
		"docker.io/traefik/whoami:v1.10":    "docker.io/traefik/whoami:v1.10",
		"ghcr.io/acme/web:v1@" + digest:     "ghcr.io/acme/web:v1@" + digest,
		"myreg.example.com:5000/team/app:2": "myreg.example.com:5000/team/app:2",
		"traefik/whoami:v1.10":              "traefik/whoami:v1.10",
		"nginx":                             "nginx",
		"":                                  "",
		// A look-alike public host merely containing the suffix text in a
		// non-final label is not internal.
		"svc.example.com/app:1":           "svc.example.com/app:1",
		"cluster.local.example.com/app:1": "cluster.local.example.com/app:1",
	} {
		if got := PublicImageRef(in); got != want {
			t.Errorf("PublicImageRef(%q) = %q; want %q", in, got, want)
		}
	}
}
