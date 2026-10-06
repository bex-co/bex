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

package apps

import (
	"context"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func domainsManifest(host string) string {
	return "services:\n  - type: web\n    name: qa-bp15\n    runtime: image\n    image:\n      url: docker.io/traefik/whoami:v1.10\n    plan: free\n    domains:\n      - " + host + "\n"
}

// w8/055: validate refuses the hosts apply refuses — reserved platform names
// and a host another site serves — and locates every hostname refusal at the
// domains entry, not the service's name line.
func TestBlueprintValidateRefusesUnclaimableDomains(t *testing.T) {
	other := &appv1alpha1.App{}
	other.Name, other.Namespace = "elsewhere", "default"
	other.Spec.Hosts = []string{"taken.example.com"}
	svc, _ := newBaseDomainService("onbex.co", "dashboard.bex.co", other)

	refused := map[string]string{
		"dashboard.bex.co":  "reserved platform hostname",
		"foo.onbex.co":      "reserved platform hostname",
		"onbex.co":          "reserved platform hostname",
		"10.0.0.1":          `invalid hostname "10.0.0.1"`,
		"localhost":         `invalid hostname "localhost"`,
		"co.uk":             "it is a public suffix",
		"github.io":         "it is a public suffix",
		"'*.example.com'":   "wildcard hostnames are not allowed",
		"taken.example.com": "this domain already exists on another site",
	}
	for host, want := range refused {
		v, err := svc.ValidateBlueprint(context.Background(), "", domainsManifest(host), "")
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if v.Valid || len(v.Errors) != 1 {
			t.Errorf("%s: %+v, want one refusal", host, v)
			continue
		}
		e := v.Errors[0]
		if !strings.Contains(e.Error, want) || e.Path == nil || *e.Path != "services[0].domains[0]" || e.Line == nil || *e.Line != 9 {
			t.Errorf("%s: %q at %v line %v, want %q at services[0].domains[0] line 9", host, e.Error, ptrValue(e.Path), ptrValue(e.Line), want)
		}
		if strings.Contains(e.Error, "elsewhere") {
			t.Errorf("%s: refusal names the other site: %q", host, e.Error)
		}
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", domainsManifest("example.com"), ""); err != nil || !v.Valid {
		t.Fatalf("example.com control: %+v, %v", v, err)
	}
}

// A re-sync re-stating an existing service's own hosts — its own platform
// host included — is not self-refused.
func TestBlueprintValidateKeepsAnExistingServicesOwnHosts(t *testing.T) {
	current := &appv1alpha1.App{}
	current.Name, current.Namespace = "qa-bp15", "default"
	current.Spec.Subdomain = "qa-bp15-x7k2"
	current.Spec.Hosts = []string{"shop.example.com"}
	svc, _ := newBaseDomainService("onbex.co", "dashboard.bex.co", current)
	for _, host := range []string{"shop.example.com", "qa-bp15-x7k2.onbex.co"} {
		if v, err := svc.ValidateBlueprint(context.Background(), "", domainsManifest(host), ""); err != nil || !v.Valid {
			t.Errorf("%s: %+v, %v", host, v, err)
		}
	}
}
