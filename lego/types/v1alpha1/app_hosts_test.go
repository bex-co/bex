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

// TestHasPublicHosts (w5/112): an App has public hosts exactly when the
// operator keeps an Ingress for it. A service whose platform subdomain is
// disabled and whose last custom domain was deleted has none, though it is
// still exposed.
func TestHasPublicHosts(t *testing.T) {
	for name, tc := range map[string]struct {
		spec AppSpec
		want bool
	}{
		"exposed":                             {AppSpec{Expose: true}, true},
		"exposed, subdomain disabled":         {AppSpec{Expose: true, SubdomainPolicy: SubdomainPolicyDisabled}, false},
		"subdomain disabled, a custom domain": {AppSpec{Expose: true, SubdomainPolicy: SubdomainPolicyDisabled, Hosts: []string{"shop.example.com"}}, true},
		"a legacy host":                       {AppSpec{Host: "web.example.com"}, true},
		"a static site, subdomain disabled":   {AppSpec{Type: TypeStaticSite, Expose: true, SubdomainPolicy: SubdomainPolicyDisabled}, false},
		"a private service with a host":       {AppSpec{Type: TypePrivateService, Expose: true, Host: "api.example.com"}, false},
		"not exposed":                         {AppSpec{}, false},
	} {
		if got := tc.spec.HasPublicHosts(); got != tc.want {
			t.Errorf("%s: HasPublicHosts() = %v, want %v", name, got, tc.want)
		}
	}
}
