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
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
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
		{"pull/legacy", LegacyExternalRegistryPullSecretName("web"), "web-registry-pull"},
		{"pull/legacy tenant CR name", LegacyExternalRegistryPullSecretName("tea-d98210cbbpdc73dcrkvg-qa-web"), "tea-d98210cbbpdc73dcrkvg-qa-web-registry-pull"},
		{"pull/per credential", ExternalRegistryPullSecretName("web", "rgc-d98210cbbpdc73dcrkvg"), "web-registry-pull-rgc-d98210cbbpdc73dcrkvg"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// An App's own pull Secrets are the legacy one and one per minted credential
// (w8/m529); the operator exempts exactly those from its protected-Secret
// refusal, so nothing else of the App or of another App may pass.
func TestIsExternalRegistryPullSecretName(t *testing.T) {
	const credential = "rgc-d98210cbbpdc73dcrkvg"
	for _, tc := range []struct {
		app, name string
		want      bool
	}{
		{"web", "web-registry-pull", true},
		{"web", ExternalRegistryPullSecretName("web", credential), true},
		{"web", "web-clone", false},
		{"web", "web-registry-pull-", false},
		{"web", "web-registry-pull-rgc-x", false},
		{"web", ExternalRegistryPullSecretName("web", credential) + "-clone", false},
		{"web", "web-registry-pull-rgc-x-clone", false},
		{"web", ExternalRegistryPullSecretName("other", credential), false},
		{"other", ExternalRegistryPullSecretName("web", credential), false},
		{"web-registry-pull-rgc", "web-registry-pull-rgc-clone", false},
	} {
		if got := IsExternalRegistryPullSecretName(tc.app, tc.name); got != tc.want {
			t.Errorf("IsExternalRegistryPullSecretName(%q, %q) = %v, want %v", tc.app, tc.name, got, tc.want)
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
		{"a", false, "wildcard.example.com", "a-tls-wildcard.example.com"},
		{"a", false, "*.example.com", "a-tls--wildcard.example.com"},
		{"a", true, "*.example.com", "a-tls"},
		{"a", false, long, "a-tls-99a9d9674c365e9e"},
	} {
		got := TLSSecretName(tc.app, tc.first, tc.host)
		if got != tc.want || len(got) > 253 {
			t.Errorf("TLSSecretName(%q, %v, %q) = %q, want %q", tc.app, tc.first, tc.host, got, tc.want)
		}
	}
}

// A wildcard and the concrete host whose first label is "wildcard" are two
// hosts with two certificates. Sharing one Secret, cert-manager issued only
// whichever Ingress it reached first, and bex-api reported both issued. The
// long bases put the wildcard's name, then both names, past the length limit.
func TestTLSSecretNameKeepsAWildcardFromItsWildcardLabel(t *testing.T) {
	label := strings.Repeat("a", 63)
	long := label + "." + label + "." + label + "."
	for _, base := range []string{"example.com", long + strings.Repeat("b", 42) + ".com", long + strings.Repeat("b", 46) + ".com"} {
		if errs := validation.IsDNS1123Subdomain("wildcard." + base); len(errs) != 0 {
			t.Fatalf("wildcard.%s is no host: %v", base, errs)
		}
		wildcard, concrete := TLSSecretName("a", false, "*."+base), TLSSecretName("a", false, "wildcard."+base)
		if wildcard == concrete {
			t.Errorf("*.%s and wildcard.%s share the Secret %q", base, base, wildcard)
		}
	}
}

// No two hosts of one App share a Secret, whatever mix of concrete hosts,
// wildcards and hosts long enough to be hashed it serves, and every name is
// one Kubernetes accepts.
func TestTLSSecretNamesOfOneAppAreDistinct(t *testing.T) {
	labels := []string{"wildcard", "wildcard--example", "www", "tls", "srv-d1abc-tls", "99a9d9674c365e9e", strings.Repeat("x", 63), strings.Repeat("y", 50)}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		var hosts []string
		for range 8 {
			parts := []string{"com"}
			for range 1 + rng.IntN(4) {
				parts = slices.Insert(parts, 0, labels[rng.IntN(len(labels))])
			}
			base := strings.Join(parts, ".")
			if len(validation.IsDNS1123Subdomain(base)) != 0 {
				continue // not a host bex-api or an Ingress accepts
			}
			// A host, the wildcard over it, or that wildcard beside the
			// concrete host its "wildcard" label spells.
			for _, h := range [][]string{{base}, {WildcardHostPrefix + base}, {WildcardHostPrefix + base, "wildcard." + base}}[rng.IntN(3)] {
				if len(validation.IsDNS1123Subdomain(strings.TrimPrefix(h, WildcardHostPrefix))) == 0 && !slices.Contains(hosts, h) {
					hosts = append(hosts, h)
				}
			}
		}
		owner := map[string]string{}
		for i, host := range hosts {
			name := TLSSecretName("srv-d1abc", i == 0, host)
			if errs := validation.IsDNS1123Subdomain(name); len(errs) != 0 {
				t.Fatalf("TLSSecretName(%q) = %q: %v", host, name, errs)
			}
			if other, dup := owner[name]; dup {
				t.Fatalf("%q and %q share the Secret %q", other, host, name)
			}
			owner[name] = host
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
