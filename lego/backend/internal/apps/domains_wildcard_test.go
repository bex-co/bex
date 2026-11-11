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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// wildcardDNS answers a wildcard domain's two checks: whether a name points
// at the service, and whether its _acme-challenge name delegates.
type wildcardDNS struct {
	pointed, delegated bool
	probes             []string
	challenge, target  string
}

func (d *wildcardDNS) PointsAtService(_ context.Context, host, _ string) bool {
	d.probes = append(d.probes, host)
	return d.pointed
}

func (d *wildcardDNS) DelegatesChallenge(_ context.Context, name, target string) bool {
	d.challenge, d.target = name, target
	return d.delegated
}

// TestWildcardDomainLifecycle (w4/m326): a wildcard is added unverified with
// Render's two records, a CNAME at * and the _acme-challenge delegation; it
// verifies only once a name below it points at the service and the
// delegation is in place, and then serves; it deletes like any domain.
func TestWildcardDomainLifecycle(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	app := managedApp("web", "srv-1")
	app.Spec.Subdomain = "web-x1"
	svc, cl := newService(claims, app)
	svc.BaseDomain, svc.ACMEDelegationZone = "onbex.co", "acme.bexdns.example"
	dns := &wildcardDNS{pointed: true}
	svc.DomainTraffic = dns
	ctx := context.Background()

	created, err := svc.AddDomain(ctx, "web", "*.Tenants.Example.com.")
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if created.Name != "*.tenants.example.com" || created.VerificationStatus != "unverified" || created.DomainType != "subdomain" {
		t.Fatalf("added = %+v", created)
	}
	if want := (DNSRecordView{Type: "CNAME", Name: "*.tenants", Value: "web-x1.onbex.co"}); created.DNSRecord != want {
		t.Errorf("DNSRecord = %+v, want %+v", created.DNSRecord, want)
	}
	if want := (DNSRecordView{Type: "CNAME", Name: "_acme-challenge.tenants", Value: "web-x1.acme.bexdns.example"}); created.ChallengeRecord == nil || *created.ChallengeRecord != want {
		t.Errorf("ChallengeRecord = %+v, want %+v", created.ChallengeRecord, want)
	}
	if rest := toRenderCustomDomain(created); rest.ChallengeRecord == nil || rest.ChallengeRecord.Name != "_acme-challenge.tenants" {
		t.Errorf("REST challengeRecord = %+v", rest.ChallengeRecord)
	}

	// The traffic record alone does not verify a wildcard.
	if view, err := svc.VerifyDomain(ctx, "web", "*.tenants.example.com"); err != nil || view.VerificationStatus != "unverified" {
		t.Fatalf("verify without the delegation = %+v, %v", view, err)
	}
	probe := dns.probes[len(dns.probes)-1]
	if label, ok := strings.CutSuffix(probe, ".tenants.example.com"); !ok || label == "" || strings.ContainsAny(label, "*.") {
		t.Errorf("probed %q, want a concrete name one label below the base", probe)
	}
	if dns.challenge != "_acme-challenge.tenants.example.com" || dns.target != "web-x1.acme.bexdns.example" {
		t.Errorf("delegation checked %q -> %q", dns.challenge, dns.target)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; len(hosts) != 0 {
		t.Fatalf("an unverified wildcard serves: %v", hosts)
	}

	dns.delegated = true
	if view, err := svc.VerifyDomain(ctx, "web", "*.tenants.example.com"); err != nil || view.VerificationStatus != "verified" {
		t.Fatalf("verify = %+v, %v", view, err)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; !slices.Equal(hosts, []string{"*.tenants.example.com"}) {
		t.Fatalf("spec.hosts = %v, want the wildcard", hosts)
	}
	if err := svc.DeleteDomain(ctx, "web", "*.tenants.example.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; len(hosts) != 0 {
		t.Fatalf("spec.hosts after delete = %v", hosts)
	}
}

// A wildcard over a platform name stays reserved: the base domain and every
// name under it, the service's own platform host included.
func TestWildcardDomainUnderThePlatformIsReserved(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Subdomain = "web"
	svc, _ := newBaseDomainService("onbex.co", "dashboard.bex.co", app)
	svc.ACMEDelegationZone = "acme.bexdns.example"
	for _, host := range []string{"*.onbex.co", "*.web.onbex.co", "*.other.onbex.co", "*.bex.co"} {
		var coded *core.CodedError
		if _, err := svc.AddDomain(context.Background(), "web", host); !errors.As(err, &coded) || coded.Code != "CUSTOM_DOMAIN_RESERVED" {
			t.Errorf("AddDomain(%q) = %v, want the reserved refusal", host, err)
		}
	}
}

// foreignApp is an App in another workspace's namespace (ADR043).
func foreignApp(name string, hosts ...string) *appv1alpha1.App {
	a := appWithHosts(name, hosts...)
	a.Namespace = "tea-other"
	return a
}

// sharedTenantApp is an App of tenant in the shared namespace, as a workspace not
// yet moved to its own namespace keeps them (ADR043 D8).
func sharedTenantApp(name, tenant string, hosts ...string) *appv1alpha1.App {
	a := appWithHosts(name, hosts...)
	a.Labels = map[string]string{core.LabelTenant: tenant}
	return a
}

// TestWildcardOverlapAcrossWorkspacesIsRefused (w4/m326): a wildcard over
// another workspace's concrete host, or a concrete host under another
// workspace's wildcard, is refused as in use. Within one workspace both
// stand, which routes a name under the wildcard to another service.
func TestWildcardOverlapAcrossWorkspacesIsRefused(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		other  *appv1alpha1.App
		add    string
		refuse bool
	}{
		{"wildcard over a foreign concrete host", foreignApp("theirs", "api.example.com"), "*.example.com", true},
		{"concrete host under a foreign wildcard", foreignApp("theirs", "*.example.com"), "api.example.com", true},
		{"a foreign wildcard twice", foreignApp("theirs", "*.example.com"), "*.example.com", true},
		{"wildcard over an own-workspace host", sharedTenantApp("mine", "tea-a", "api.example.com"), "*.example.com", false},
		{"concrete host under an own-workspace wildcard", sharedTenantApp("mine", "tea-a", "*.example.com"), "api.example.com", false},
		// A DNS wildcard answers at every depth, so overlap does too.
		{"a foreign host two labels down", foreignApp("theirs", "a.b.example.com"), "*.example.com", true},
		{"a name two labels under a foreign wildcard", foreignApp("theirs", "*.example.com"), "login.eu.example.com", true},
		{"a foreign wildcard one level down", foreignApp("theirs", "*.eu.example.com"), "*.example.com", true},
		{"a wildcard under a foreign wildcard", foreignApp("theirs", "*.example.com"), "*.eu.example.com", true},
		{"a foreign apex", foreignApp("theirs", "example.com"), "*.example.com", false},
		{"a foreign wildcard on a sibling domain", foreignApp("theirs", "*.example.org"), "api.example.com", false},
		// A workspace is its tenant, wherever its Apps live (ADR043 D8).
		{"another tenant in the shared namespace", sharedTenantApp("theirs", "tea-b", "api.example.com"), "*.example.com", true},
	} {
		web := sampleApp("web")
		web.Labels = map[string]string{core.LabelTenant: "tea-a"}
		svc, _ := newService(nil, web, tc.other)
		svc.BaseDomain, svc.ACMEDelegationZone = "onbex.co", "acme.bexdns.example"
		_, err := svc.AddDomain(ctx, "web", tc.add)
		if refused := errors.Is(err, core.ErrConflict); refused != tc.refuse || (!tc.refuse && err != nil) {
			t.Errorf("%s: AddDomain(%q) = %v, want refused %v", tc.name, tc.add, err, tc.refuse)
		}
	}
}

// A claim that verifies after another workspace's overlapping claim was
// added never serves: whichever verifies second stays unverified (w4/m326).
func TestWildcardOverlapIsRefusedAtVerification(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	svc, cl := newService(claims, managedApp("web", "srv-1"))
	svc.BaseDomain, svc.ACMEDelegationZone = "onbex.co", "acme.bexdns.example"
	ctx := context.Background()
	if _, err := svc.AddDomain(ctx, "web", "api.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	// Another workspace's wildcard over it went live meanwhile.
	if err := cl.Create(ctx, foreignApp("theirs", "*.example.com")); err != nil {
		t.Fatal(err)
	}
	view, err := svc.VerifyDomain(ctx, "web", "api.example.com")
	if err != nil || view.VerificationStatus != "unverified" {
		t.Fatalf("verify under a foreign wildcard = %+v, %v", view, err)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; len(hosts) != 0 {
		t.Fatalf("spec.hosts = %v, want nothing served", hosts)
	}
}

// A platform with no ACME delegation zone could never issue a wildcard's
// certificate, so it refuses one by name on every add path (w4/m326).
func TestWildcardDomainNeedsADelegationZone(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	svc.BaseDomain = "onbex.co"
	_, err := svc.AddDomain(context.Background(), "web", "*.example.com")
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != "CUSTOM_DOMAIN_WILDCARD_UNAVAILABLE" {
		t.Fatalf("AddDomain without a delegation zone = %v, want CUSTOM_DOMAIN_WILDCARD_UNAVAILABLE", err)
	}
	app := sampleApp("new")
	app.Spec.Hosts = []string{"*.example.com"}
	if err := svc.ensureHostsClaimable(context.Background(), app); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a create declaring a wildcard = %v, want refused", err)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; len(hosts) != 0 {
		t.Fatalf("spec.hosts = %v", hosts)
	}
}
