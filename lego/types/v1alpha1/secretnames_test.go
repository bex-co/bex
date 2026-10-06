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

import (
	"strings"
	"testing"
)

// TestBuildPlaneSecretNames pins the two on-the-wire conventions. They are not
// free to change: bex-api has already written Secrets under these names into
// every tenant namespace, an operator running a different release must accept
// the names the backend of its own release writes, and the operator's
// self-reference carve-out (w6/m97) recognizes an App's own build-plane Secret
// by deriving exactly these strings from the App's name.
func TestBuildPlaneSecretNames(t *testing.T) {
	for _, tc := range []struct{ name, got, want string }{
		{"clone/short", CloneSecretName("web"), "web-clone"},
		{"clone/tenant CR name", CloneSecretName("tea-d98210cbbpdc73dcrkvg-qa-web"), "tea-d98210cbbpdc73dcrkvg-qa-web-clone"},
		{"pull/short", ExternalRegistryPullSecretName("web"), "web-registry-pull"},
		{"pull/tenant CR name", ExternalRegistryPullSecretName("tea-d98210cbbpdc73dcrkvg-qa-web"), "tea-d98210cbbpdc73dcrkvg-qa-web-registry-pull"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// The names are pinned: the operator issues certificates into them and bex-api
// reads them back, so a change here strands every issued certificate.
func TestTLSSecretName(t *testing.T) {
	long := strings.Repeat("a", 240) + ".example.com"
	for _, tc := range []struct {
		app   string
		first bool
		host  string
		want  string
	}{
		{"beancount-cms", true, "beancount-cms-v2.onbex.co", "beancount-cms-tls"},
		{"a", false, "www.example.com", "a-tls-www.example.com"},
		{"a", false, "*.example.com", "a-tls-wildcard.example.com"},
		{"a", false, long, "a-tls-99a9d9674c365e9e"},
	} {
		got := TLSSecretName(tc.app, tc.first, tc.host)
		if got != tc.want || len(got) > 253 {
			t.Errorf("TLSSecretName(%q, %v, %q) = %q, want %q", tc.app, tc.first, tc.host, got, tc.want)
		}
	}
}

// One character past the 253-character Secret name limit is where the name
// turns into a hash; at the limit it stays readable.
func TestTLSSecretNameAtTheLimit(t *testing.T) {
	label := strings.Repeat("a", 63)
	atLimit := label + "." + label + "." + label + "." + strings.Repeat("b", 51) + ".com"
	if got := TLSSecretName("a", false, atLimit); got != "a-tls-"+atLimit || len(got) != 253 {
		t.Fatalf("a 253-character name = %q (%d), want it kept", got, len(got))
	}
	if got := TLSSecretName("a", false, "c"+atLimit); len(got) >= 253 || strings.Contains(got, atLimit) {
		t.Fatalf("a 254-character name = %q, want the hashed form", got)
	}
}

// The first host is the one EffectiveHosts serves first, which keeps the
// legacy name: a custom primary, or the first custom host when the platform
// host is off or not exposed.
func TestTLSSecretNameForFollowsEffectiveHosts(t *testing.T) {
	for name, tc := range map[string]struct {
		spec  AppSpec
		host  string
		first bool
	}{
		"custom primary":       {AppSpec{Type: TypeWebService, Expose: true, Host: "www.example.com", Hosts: []string{"example.com"}}, "www.example.com", true},
		"after the primary":    {AppSpec{Type: TypeWebService, Expose: true, Host: "www.example.com", Hosts: []string{"example.com"}}, "example.com", false},
		"after the platform":   {AppSpec{Type: TypeWebService, Expose: true, Hosts: []string{"example.com"}}, "example.com", false},
		"subdomain disabled":   {AppSpec{Type: TypeWebService, Expose: true, SubdomainPolicy: SubdomainPolicyDisabled, Hosts: []string{"example.com"}}, "example.com", true},
		"not exposed":          {AppSpec{Type: TypeWebService, Hosts: []string{"example.com"}}, "example.com", true},
		"own platform host":    {AppSpec{Type: TypeWebService, Expose: true, Hosts: []string{"web.onbex.co", "example.com"}}, "web.onbex.co", true},
		"second listed domain": {AppSpec{Type: TypeWebService, Hosts: []string{"example.com", "www.example.com"}}, "www.example.com", false},
	} {
		want := TLSSecretName("web", tc.first, tc.host)
		if got := tc.spec.TLSSecretNameFor("web", "onbex.co", tc.host); got != want {
			t.Errorf("%s: TLSSecretNameFor(%q) = %q, want %q", name, tc.host, got, want)
		}
	}
}
