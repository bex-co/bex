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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// wildcardDNS answers a wildcard domain's two checks: whether a name's CNAME
// reaches the service, and whether its _acme-challenge name delegates.
type wildcardDNS struct {
	pointed, delegated bool
	probes             []string
	challenge, target  string
}

func (d *wildcardDNS) PointsAtService(_ context.Context, host, _ string) bool {
	d.probes = append(d.probes, host)
	return d.pointed
}

func (d *wildcardDNS) CNAMEsToService(_ context.Context, host, _ string) bool {
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

// edge is the address every platform host resolves to.
var edge = []string{"49.12.20.236"}

// workspaceApp is a store-managed App of tenant, served at <name>-x1.onbex.co.
func workspaceApp(name, appID, tenant string) *appv1alpha1.App {
	a := managedApp(name, appID)
	a.Labels[core.LabelTenant] = tenant
	a.Spec.Subdomain = name + "-x1"
	return a
}

// twoWorkspaceDomains is web (srv-1, workspace tea-a) and theirs (srv-2,
// theirsTenant) on one claim store, resolving through resolver with the real
// traffic checker.
func twoWorkspaceDomains(theirsTenant string, resolver fakeDomainResolver) (*Service, client.Client, *memoryDomainClaimStore) {
	claims := newMemoryDomainClaimStore()
	claims.workspaces["srv-1"], claims.workspaces["srv-2"] = "tea-a", theirsTenant
	svc, cl := newService(claims, workspaceApp("web", "srv-1", "tea-a"), workspaceApp("theirs", "srv-2", theirsTenant))
	svc.BaseDomain, svc.ACMEDelegationZone = "onbex.co", "acme.bexdns.example"
	svc.DomainTraffic = systemDomainTrafficChecker{resolver: resolver}
	return svc, cl, claims
}

// staleIndexClient is a replica whose host-claim index predates another
// replica's sweep: its first platform-wide App List, the index load, reads
// the Apps, then runs during, and returns what it read before.
type staleIndexClient struct {
	client.Client
	during func()
}

func (c *staleIndexClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := c.Client.List(ctx, list, opts...)
	if _, ok := list.(*appv1alpha1.AppList); ok && len(opts) == 0 && c.during != nil {
		during := c.during
		c.during = nil
		during()
	}
	return err
}

// TestOverlappingClaimsVerifyOnceAcrossTwoReplicas (w4/m326): two bex-api
// replicas sweep the same due list, holding another workspace's wildcard and
// a name under it, each pointed at its own service. Replica 1's whole tick
// runs after replica 2 has loaded its host-claim index, in both list orders.
// Only the claim replica 1 promoted first verifies: the store refuses the
// other as replica 2 promotes it, though replica 2's index never saw the
// first, and it stays pending with the attempt recorded.
func TestOverlappingClaimsVerifyOnceAcrossTwoReplicas(t *testing.T) {
	cnamed := fakeDomainResolver{cnames: map[string]string{
		"*.example.com":               "theirs-x1.onbex.co",
		"_acme-challenge.example.com": "theirs-x1.acme.bexdns.example",
		"api.example.com":             "web-x1.onbex.co",
	}}
	for _, wildcardFirst := range []bool{true, false} {
		svc1, cl, claims := twoWorkspaceDomains("tea-b", cnamed)
		ctx := context.Background()
		add := func(appID, host string) {
			if _, _, err := claims.AddDomainClaim(ctx, appID, host, ""); err != nil {
				t.Fatal(err)
			}
		}
		if wildcardFirst {
			add("srv-2", "*.example.com")
			add("srv-1", "api.example.com")
		} else {
			add("srv-1", "api.example.com")
			add("srv-2", "*.example.com")
		}
		replica1 := &DomainVerificationWorker{Svc: svc1}
		stale := &staleIndexClient{Client: cl, during: func() {
			if err := replica1.tick(ctx); err != nil {
				t.Fatal(err)
			}
		}}
		replica2 := &DomainVerificationWorker{Svc: &Service{
			Base: &core.Base{Client: stale, Namespace: "default"}, Store: claims, DomainTraffic: svc1.DomainTraffic,
			BaseDomain: svc1.BaseDomain, ACMEDelegationZone: svc1.ACMEDelegationZone,
		}}
		if err := replica2.tick(ctx); err != nil {
			t.Fatal(err)
		}
		if stale.during != nil {
			t.Fatal("replica 2 never loaded its index")
		}

		wildcard, _ := claims.GetDomainClaim(ctx, "srv-2", "*.example.com")
		name, _ := claims.GetDomainClaim(ctx, "srv-1", "api.example.com")
		winner, loser, winnerApp, loserApp := wildcard, name, "theirs", "web"
		if !wildcardFirst {
			winner, loser, winnerApp, loserApp = name, wildcard, "web", "theirs"
		}
		// Replica 1 refused the loser on its index, replica 2 in the store.
		if winner.ClaimState != "verified" || loser.ClaimState != "pending" || loser.VerificationAttempts != 2 {
			t.Errorf("wildcardFirst=%v: %s %s, %s %s after %d attempts; want only %s verified",
				wildcardFirst, winner.Host, winner.ClaimState, loser.Host, loser.ClaimState, loser.VerificationAttempts, winner.Host)
		}
		if hosts := getApp(t, cl, winnerApp).Spec.Hosts; !slices.Equal(hosts, []string{winner.Host}) {
			t.Errorf("wildcardFirst=%v: %s serves %v, want [%s]", wildcardFirst, winnerApp, hosts, winner.Host)
		}
		if hosts := getApp(t, cl, loserApp).Spec.Hosts; len(hosts) != 0 {
			t.Errorf("wildcardFirst=%v: %s serves %v, want nothing", wildcardFirst, loserApp, hosts)
		}
	}
}

// TestWildcardRecordsFollowRender (w4/m326): a wildcard verifies only through
// its `*` CNAME, and a name under another workspace's pending wildcard only
// through a CNAME to its own service. While the wildcard's `*` record answers
// with the edge addresses, neither verifies, whichever is checked first. Once
// the `*` CNAME Render documents is in place, the wildcard verifies and the
// name under it resolves to the wildcard's service, so it stays pending.
func TestWildcardRecordsFollowRender(t *testing.T) {
	addressed := fakeDomainResolver{
		cnames: map[string]string{"_acme-challenge.example.com": "theirs-x1.acme.bexdns.example"},
		addrs:  map[string][]string{"*.example.com": edge, "web-x1.onbex.co": edge, "theirs-x1.onbex.co": edge},
	}
	svc, cl, _ := twoWorkspaceDomains("tea-b", addressed)
	ctx := context.Background()
	if _, err := svc.AddDomain(ctx, "web", "api.example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddDomain(ctx, "theirs", "*.example.com"); err != nil {
		t.Fatal(err)
	}
	verify := func(app, host string) string {
		t.Helper()
		view, err := svc.VerifyDomain(ctx, app, host)
		if err != nil {
			t.Fatalf("VerifyDomain(%s): %v", host, err)
		}
		return view.VerificationStatus
	}
	if got := verify("web", "api.example.com"); got != "unverified" {
		t.Errorf("a name under another workspace's wildcard, on edge addresses: %s", got)
	}
	if got := verify("theirs", "*.example.com"); got != "unverified" {
		t.Errorf("a wildcard whose * record answers with edge addresses: %s", got)
	}

	addressed.cnames["*.example.com"] = "theirs-x1.onbex.co"
	if got := verify("web", "api.example.com"); got != "unverified" {
		t.Errorf("a name resolving through another service's * CNAME: %s", got)
	}
	if got := verify("theirs", "*.example.com"); got != "verified" {
		t.Errorf("a wildcard with its * CNAME: %s", got)
	}
	if hosts := getApp(t, cl, "theirs").Spec.Hosts; !slices.Equal(hosts, []string{"*.example.com"}) {
		t.Errorf("theirs serves %v, want the wildcard", hosts)
	}
	if hosts := getApp(t, cl, "web").Spec.Hosts; len(hosts) != 0 {
		t.Errorf("web serves %v, want nothing", hosts)
	}
}

// Only a non-apex name under another workspace's wildcard needs a CNAME. An
// apex, a name with no wildcard over it, and a name under its own
// workspace's wildcard keep the address proof (w4/m326).
func TestOnlyANameUnderAnotherWorkspacesWildcardNeedsACNAME(t *testing.T) {
	for _, tc := range []struct {
		name, wildcardTenant, host string
		addWildcard                bool
		want                       string
	}{
		{"under another workspace's wildcard", "tea-b", "api.example.com", true, "unverified"},
		{"two labels under it", "tea-b", "login.eu.example.com", true, "unverified"},
		{"no wildcard over it", "tea-b", "api.example.com", false, "verified"},
		{"under its own workspace's wildcard", "tea-a", "api.example.com", true, "verified"},
		{"the apex below another workspace's wildcard", "tea-b", "example.com", true, "verified"},
	} {
		addressed := fakeDomainResolver{addrs: map[string][]string{tc.host: edge, "web-x1.onbex.co": edge}}
		svc, _, claims := twoWorkspaceDomains(tc.wildcardTenant, addressed)
		ctx := context.Background()
		if tc.addWildcard {
			if _, _, err := claims.AddDomainClaim(ctx, "srv-2", "*.example.com", ""); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := claims.AddDomainClaim(ctx, "srv-1", tc.host, ""); err != nil {
			t.Fatal(err)
		}
		view, err := svc.VerifyDomain(ctx, "web", tc.host)
		if err != nil || view.VerificationStatus != tc.want {
			t.Errorf("%s: verify %s on edge addresses = %s, %v; want %s", tc.name, tc.host, view.VerificationStatus, err, tc.want)
		}
	}
}

// The synchronous refusal of a host that verifies only through a CNAME says
// so, and names the CNAME and its target (w4/m326).
func TestCNAMEOnlyRefusalNamesTheTarget(t *testing.T) {
	addressed := fakeDomainResolver{addrs: map[string][]string{"*.example.com": edge, "web-x1.onbex.co": edge}}
	// A pending wildcard of another workspace, held in the claim store.
	foreignWildcard := newMemoryDomainClaimStore()
	foreignWildcard.workspaces["srv-2"] = "tea-b"
	if _, _, err := foreignWildcard.AddDomainClaim(context.Background(), "srv-2", "*.example.com", ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host  string
		store IntentStore
		want  string
	}{
		{"*.example.com", nil, "a wildcard verifies only through CNAME records: create CNAME * with value web-x1.onbex.co and CNAME _acme-challenge with value web-x1.acme.bexdns.example"},
		{"api.example.com", foreignWildcard, "it verifies only through a CNAME: create CNAME api with value web-x1.onbex.co"},
	} {
		// An App the store does not manage is admitted only once its record
		// already points (requireDomainTraffic).
		web := sampleApp("web")
		web.Labels = map[string]string{core.LabelTenant: "tea-a"}
		web.Spec.Subdomain = "web-x1"
		svc, _ := newService(tc.store, web)
		svc.BaseDomain, svc.ACMEDelegationZone = "onbex.co", "acme.bexdns.example"
		svc.DomainTraffic = systemDomainTrafficChecker{resolver: addressed}
		_, err := svc.AddDomain(context.Background(), "web", tc.host)
		if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("AddDomain(%s) = %v, want a conflict naming %q", tc.host, err, tc.want)
		}
	}
}
