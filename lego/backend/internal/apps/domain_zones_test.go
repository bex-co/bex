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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Lookups do not apply a claim's rule, so a public suffix stored before it can
// still be removed.
func TestAStoredPublicSuffixCanStillBeDeleted(t *testing.T) {
	svc, cl := newService(nil, appWithHosts("web", "github.io"))
	if err := svc.DeleteDomain(context.Background(), "web", "github.io"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; len(hosts) != 0 {
		t.Fatalf("spec.hosts = %v, want the stored suffix removed", hosts)
	}
}

// The CNAME is named below the registrable domain, the zone its owner manages
// and the one the ownership TXT record is named in, whatever the suffix or the
// stored spelling. Stripping two labels named www.example.co.uk's "www.example".
func TestDNSRecordNameIsRelativeToTheRegistrableDomain(t *testing.T) {
	for host, want := range map[string]string{
		"www.example.co.uk":   "www",
		"shop.example.com.au": "shop",
		"WWW.Example.COM.":    "www",
	} {
		got := dnsRecordFor(host, "subdomain", "web.onbex.co")
		if got.Type != "CNAME" || got.Name != want || got.Value != "web.onbex.co" {
			t.Errorf("dnsRecordFor(%q) = %+v, want {CNAME %s web.onbex.co}", host, got, want)
		}
		if canonical := normalizeHostname(host); got.Name+"."+ownershipDomain(canonical) != canonical {
			t.Errorf("%q: CNAME %q is not named in the TXT record's zone", host, got.Name)
		}
	}
}

// A host too long for "<app>-tls-<host>" is verified once the Secret the
// operator issues it into exists. bex-api truncated the name the operator
// hashes, so such a host stayed pending after its certificate was issued.
func TestALongHostIsVerifiedOnceItsSecretExists(t *testing.T) {
	label := strings.Repeat("a", 63)
	host := label + "." + label + "." + label + "." + strings.Repeat("b", 52) + ".com"
	a := appWithHosts("web", host)
	a.Spec.Host = "web.onbex.co"
	secret := tlsSecret("default", appv1alpha1.TLSSecretName("web", false, host))
	if len("web-tls-"+host) <= 253 || strings.Contains(secret.Name, host) {
		t.Fatalf("host %d chars: not long enough to need the hashed name %q", len(host), secret.Name)
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(a, secret), Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", host)
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "verified" {
		t.Fatalf("verificationStatus = %q, want verified: its TLS Secret %s exists", d.VerificationStatus, secret.Name)
	}
}

// BEX_RESERVED_DOMAINS replaces the zones reserved around the platform hosts.
// A self-hoster with the dashboard at bex.acme.com reserves that subtree and
// can claim the rest of acme.com; the platform hosts stay reserved. Unset, the
// dashboard's whole registrable domain stays reserved.
func TestReservedDomainsReplaceThePlatformApex(t *testing.T) {
	ctx := context.Background()
	svc, _ := newBaseDomainService("onacme.app", "bex.acme.com", sampleApp("web"))
	svc.PlatformHosts = []string{"api.acme.com"}
	if _, err := svc.AddDomain(ctx, "web", "www.acme.com"); !isReserved(err) {
		t.Fatalf("by default AddDomain(www.acme.com) = %v, want reserved", err)
	}

	svc.ReservedDomains = []string{"bex.acme.com"}
	svc.PlatformHosts = []string{"API.acme.com"} // a configured URL keeps its case
	for _, host := range []string{"bex.acme.com", "admin.bex.acme.com", "api.acme.com"} {
		if _, err := svc.AddDomain(ctx, "web", host); !isReserved(err) {
			t.Errorf("AddDomain(%q) = %v, want reserved", host, err)
		}
	}
	if _, err := svc.AddDomain(ctx, "web", "www.acme.com"); err != nil {
		t.Fatalf("AddDomain(www.acme.com) with the bex subtree reserved = %v, want it added", err)
	}
}

// The first host the operator serves, a custom primary or the first custom
// host of a service whose platform subdomain is off, has its certificate in
// the legacy "<app>-tls". bex-api looked for "<app>-tls-<host>", and such a
// domain stayed pending.
func TestTheFirstEffectiveHostIsVerifiedFromTheLegacySecret(t *testing.T) {
	ctx := context.Background()
	disabled := appWithHosts("web", "www.example.com")
	disabled.Spec.Type, disabled.Spec.Expose = appv1alpha1.TypeWebService, true
	disabled.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyDisabled
	svc := &Service{Base: &core.Base{Client: fakeClient(disabled, tlsSecret("default", "web-tls")), Namespace: "default"}, BaseDomain: "onbex.co"}
	if d, err := svc.GetDomain(ctx, "web", "www.example.com"); err != nil || d.VerificationStatus != "verified" {
		t.Errorf("subdomain off: GetDomain = %+v, %v, want verified from web-tls", d, err)
	}

	primary := appWithHosts("web", "example.com")
	primary.Spec.Type, primary.Spec.Expose, primary.Spec.Host = appv1alpha1.TypeWebService, true, "www.example.com"
	cl := fakeClient(primary, tlsSecret("default", "web-tls"))
	if ready, err := domainCertificateReady(ctx, cl, primary, "onbex.co", "www.example.com"); err != nil || !ready {
		t.Errorf("custom primary: ready = %v, %v, want ready from web-tls", ready, err)
	}
}

func isReserved(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == "CUSTOM_DOMAIN_RESERVED"
}
