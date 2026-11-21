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

// TestTraefikRouterKeyMatchesTheDeployedTraefik pins the keys Traefik v3.7.17
// gave these routes on a kind cluster: each pair named one router, and the
// Ingress read last replaced the other's. The operator compares these keys to
// keep its routes apart, and bex-api derives an App's router names from them.
func TestTraefikRouterKeyMatchesTheDeployedTraefik(t *testing.T) {
	const ns = "tea-d0000000000000000000"
	type route struct{ ingress, host string }
	for _, c := range []struct {
		name string
		a, b route
		want string
	}{
		{"a wildcard beside its wildcard label", route{ns + "-site.wildcard", "*.example.org"}, route{ns + "-site", "wildcard.example.org"},
			ns + "-" + ns + "-site-wildcard-example-org"},
		{"an App name that extends another's", route{ns + "-web", "api.example.com"}, route{ns + "-web-api", "example.com"},
			ns + "-" + ns + "-web-api-example-com"},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, b := TraefikRouterKey(ns, c.a.ingress, c.a.host, "/"), TraefikRouterKey(ns, c.b.ingress, c.b.host, "/")
			if a != c.want || b != c.want {
				t.Errorf("keys %q and %q, want both %q", a, b, c.want)
			}
		})
	}
}
