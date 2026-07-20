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

package webhook

import "testing"

// TestPodAdmitterMatchesRegistryBoundary pins the w1/m53 SSRF fix: the registry
// match must require a "/" boundary so an attacker host that merely prefixes the
// registry string is NOT treated as a tenant image (which would make imagecheck
// fetch — and thus dial — the attacker host).
func TestPodAdmitterMatchesRegistryBoundary(t *testing.T) {
	p := &PodAdmitter{registry: "zot.bex-registry.svc:5000"}
	cases := []struct {
		image string
		want  bool
	}{
		{"zot.bex-registry.svc:5000/myapp:rev-abc", true},
		{"zot.bex-registry.svc:5000/ns/app@sha256:deadbeef", true},
		// The exploit: a host that starts with the registry string but is a
		// different host must NOT match.
		{"zot.bex-registry.svc:5000.attacker.tld/x:tag", false},
		{"zot.bex-registry.svc:5000evil/x:tag", false},
		// The registry string with no repo path is not a tenant image either.
		{"zot.bex-registry.svc:5000", false},
		// Unrelated public images pass through unverified (allowed).
		{"docker.io/library/nginx:1.25", false},
		{"ghcr.io/org/repo:tag", false},
	}
	for _, tc := range cases {
		if got := p.matches(tc.image); got != tc.want {
			t.Errorf("matches(%q) = %v, want %v", tc.image, got, tc.want)
		}
	}
}
