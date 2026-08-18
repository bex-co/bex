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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appWithHosts builds a sample App with spec.hosts set.
func appWithHosts(name string, hosts ...string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Hosts = hosts
	return a
}

// tlsSecret creates a fake TLS Secret as cert-manager would.
func tlsSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("fake-cert"),
			"tls.key": []byte("fake-key"),
		},
	}
}

// --- Service verbs ---

func TestListDomainsEmpty(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	domains, err := svc.ListDomains(context.Background(), "web")
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 0 {
		t.Errorf("no spec.hosts => empty list, got %v", domains)
	}
}

func TestListDomainsReturnsHosts(t *testing.T) {
	app := appWithHosts("web", "www.example.com", "api.example.com")
	app.Spec.HostRedirects = map[string]string{"www.example.com": "example.com"}
	svc, _ := newService(nil, app)
	domains, err := svc.ListDomains(context.Background(), "web")
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 2 {
		t.Fatalf("want 2 domains, got %d", len(domains))
	}
	if domains[0].Name != "www.example.com" || domains[1].Name != "api.example.com" {
		t.Errorf("domain names wrong: %v", domains)
	}
	if domains[0].RedirectForName != "example.com" || domains[1].RedirectForName != "" {
		t.Errorf("redirect targets wrong: %v", domains)
	}
}

func TestGetDomainFoundAndNotFound(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("web", "www.example.com"))

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain found: %v", err)
	}
	if d.Name != "www.example.com" {
		t.Errorf("name = %q", d.Name)
	}

	if _, err := svc.GetDomain(context.Background(), "web", "nope.example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing domain => ErrNotFound, got %v", err)
	}
}

// app.example.com is a deep subdomain — wwwSibling returns "" for it (t002), so
// these generic add tests stay about a single, unpaired host. Paired-add
// behavior gets its own tests below (TestAddDomainAutoPairsSibling etc).
func TestAddDomainAppendsToHosts(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	d, err := svc.AddDomain(context.Background(), "web", "app.example.com")
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if d.Name != "app.example.com" {
		t.Errorf("returned name = %q", d.Name)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "app.example.com" {
		t.Errorf("spec.hosts = %v, want [app.example.com]", got)
	}
}

type recordingDomainOwnership struct {
	err         error
	name, value string
}

type domainOwnershipFunc func(context.Context, string, string) error

func (f domainOwnershipFunc) VerifyTXT(ctx context.Context, name, value string) error {
	return f(ctx, name, value)
}

// memoryDomainClaimStore adds the new durable-claim methods to recordingStore
// without changing legacy managed-App tests, which intentionally exercise the
// old IntentStore fallback. Production's PGStore satisfies the same optional
// interface.
type memoryDomainClaimStore struct {
	recordingStore
	claims map[string]store.Domain
	next   int
}

func newMemoryDomainClaimStore() *memoryDomainClaimStore {
	return &memoryDomainClaimStore{claims: map[string]store.Domain{}}
}

func (m *memoryDomainClaimStore) AddDomainClaim(_ context.Context, appID, host, redirect string) (store.Domain, bool, error) {
	if existing, ok := m.claims[host]; ok {
		if existing.AppID != appID {
			return store.Domain{}, false, store.ErrConflict
		}
		existing.RedirectForName = redirect
		m.claims[host] = existing
		return existing, false, nil
	}
	m.next++
	claim := store.Domain{
		ID: "cdm-test-" + host, AppID: appID, Host: host,
		RedirectForName: redirect, ClaimState: "pending",
		Challenge:        "bex-domain-verification=test-" + host,
		ChallengeVersion: 1, CreatedAt: time.Unix(int64(m.next), 0),
	}
	m.claims[host] = claim
	return claim, true, nil
}

func (m *memoryDomainClaimStore) GetDomainClaim(_ context.Context, appID, host string) (store.Domain, error) {
	claim, ok := m.claims[host]
	if !ok || claim.AppID != appID {
		return store.Domain{}, store.ErrNotFound
	}
	return claim, nil
}

func (m *memoryDomainClaimStore) ListDomainClaims(_ context.Context, appID string) ([]store.Domain, error) {
	var out []store.Domain
	for _, claim := range m.claims {
		if claim.AppID == appID {
			out = append(out, claim)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Primary != out[j].Primary {
			return out[i].Primary
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *memoryDomainClaimStore) RecordDomainVerificationAttempt(_ context.Context, appID, id string, at time.Time) error {
	for host, claim := range m.claims {
		if claim.AppID == appID && claim.ID == id && claim.ClaimState == "pending" {
			claim.VerificationAttempts++
			claim.LastVerificationAt = &at
			m.claims[host] = claim
		}
	}
	return nil
}

func (m *memoryDomainClaimStore) PromoteDomainClaim(_ context.Context, appID, id, challenge string, at time.Time) (store.Domain, error) {
	for host, claim := range m.claims {
		if claim.AppID != appID || claim.ID != id {
			continue
		}
		if claim.ClaimState == "verified" {
			return claim, nil
		}
		if claim.Challenge != challenge {
			return store.Domain{}, store.ErrConflict
		}
		claim.ClaimState = "verified"
		claim.VerifiedAt = &at
		claim.VerificationAttempts++
		claim.LastVerificationAt = &at
		m.claims[host] = claim
		return claim, nil
	}
	return store.Domain{}, store.ErrConflict
}

func (m *memoryDomainClaimStore) ReplaceDomainClaims(_ context.Context, appID string, declarations []store.DomainDeclaration) ([]store.Domain, error) {
	wanted := map[string]bool{}
	for _, declaration := range declarations {
		wanted[declaration.Host] = true
		claim, _, err := m.AddDomainClaim(context.Background(), appID, declaration.Host, declaration.RedirectForName)
		if err != nil {
			return nil, err
		}
		claim.Primary = declaration.Primary
		m.claims[declaration.Host] = claim
	}
	for host, claim := range m.claims {
		if claim.AppID == appID && !wanted[host] {
			delete(m.claims, host)
		}
	}
	return m.ListDomainClaims(context.Background(), appID)
}

func (m *memoryDomainClaimStore) RemoveDomain(_ context.Context, appID, host string) error {
	delete(m.claims, host)
	for source, claim := range m.claims {
		if claim.AppID == appID && claim.RedirectForName == host {
			claim.RedirectForName = ""
			m.claims[source] = claim
		}
	}
	return nil
}

func TestManagedDomainClaimLifecycleNeverServesPending(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	svc, cl := newService(claims, managedApp("web", "srv-1"))
	resolver := &recordingDomainOwnership{err: errors.New("not propagated")}
	svc.DomainOwnership = resolver

	created, err := svc.AddDomain(context.Background(), "web", "app.example.com")
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if created.OwnershipStatus != "pending" || created.OwnershipDNSRecord == nil || created.OwnershipDNSRecord.Type != "TXT" {
		t.Fatalf("pending view = %+v", created)
	}
	challenge := created.OwnershipDNSRecord.Value
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Fatalf("pending claim reached serving spec: %v", got)
	}
	retried, err := svc.AddDomain(context.Background(), "web", "app.example.com")
	if err != nil || retried.OwnershipDNSRecord == nil || retried.OwnershipDNSRecord.Value != challenge {
		t.Fatalf("same-App retry did not preserve challenge: %+v err=%v", retried, err)
	}

	_, err = svc.VerifyDomain(context.Background(), "web", "app.example.com")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "DOMAIN_OWNERSHIP_PENDING" {
		t.Fatalf("wrong TXT = %v, want DOMAIN_OWNERSHIP_PENDING", err)
	}
	if strings.Contains(err.Error(), challenge) || len(getApp(t, cl, "web").Spec.Hosts) != 0 {
		t.Fatal("failed verification leaked its challenge or served the pending host")
	}

	resolver.err = nil
	verified, err := svc.VerifyDomain(context.Background(), "web", "app.example.com")
	if err != nil {
		t.Fatalf("VerifyDomain: %v", err)
	}
	if verified.OwnershipStatus != "verified" || verified.OwnershipDNSRecord != nil || verified.VerificationStatus != "pending" {
		t.Fatalf("post-promotion view = %+v", verified)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; !slices.Equal(got, []string{"app.example.com"}) {
		t.Fatalf("verified claim not projected: %v", got)
	}
	if again, err := svc.VerifyDomain(context.Background(), "web", "app.example.com"); err != nil || again.OwnershipStatus != "verified" {
		t.Fatalf("verified retry = %+v err=%v", again, err)
	}
}

func TestManagedDomainVerificationCannotPromoteRecreatedClaim(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	svc, cl := newService(claims, managedApp("web", "srv-1"))
	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	svc.DomainOwnership = domainOwnershipFunc(func(_ context.Context, _, _ string) error {
		delete(claims.claims, "app.example.com")
		claims.next++
		claims.claims["app.example.com"] = store.Domain{
			ID: "cdm-recreated", AppID: "srv-1", Host: "app.example.com",
			ClaimState: "pending", Challenge: "bex-domain-verification=recreated",
			ChallengeVersion: 2, CreatedAt: time.Unix(99, 0),
		}
		return nil
	})

	_, err := svc.VerifyDomain(context.Background(), "web", "app.example.com")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "DOMAIN_CLAIM_STALE" {
		t.Fatalf("stale proof = %v, want DOMAIN_CLAIM_STALE", err)
	}
	if claims.claims["app.example.com"].ClaimState != "pending" || len(getApp(t, cl, "web").Spec.Hosts) != 0 {
		t.Fatal("stale proof promoted or served the replacement claim")
	}
}

func TestManagedServiceCreatePersistsPendingWithoutServing(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	svc, cl := newService(claims)
	req := CreateRequest{
		Name: "created", Image: "img:1", Hosts: []string{"created.example.com"},
	}
	spec, err := specFromCreate(req)
	if err != nil {
		t.Fatalf("specFromCreate: %v", err)
	}
	view, err := svc.materializeNewApp(context.Background(), req, &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: "default"},
		Spec:       spec,
	}, "tea-test", core.EnvironmentAssignment{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	app := getApp(t, cl, view.Name)
	if app.Spec.Host != "" || len(app.Spec.Hosts) != 0 {
		t.Fatalf("pending create domain reached serving spec: host=%q hosts=%v", app.Spec.Host, app.Spec.Hosts)
	}
	rows, err := claims.ListDomainClaims(context.Background(), "srv-test")
	if err != nil || len(rows) != 1 || rows[0].Host != "created.example.com" || rows[0].ClaimState != "pending" || !rows[0].Primary {
		t.Fatalf("create claims = %+v err=%v", rows, err)
	}
}

func TestManagedBlueprintDomainSyncPreservesPendingClaimWithoutServing(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	existing := managedApp("web", "srv-1")
	svc, cl := newService(claims, existing)
	final := existing.DeepCopy()
	final.Spec.Host = "blueprint.example.com"
	if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: "web"}, existing, final, stackChanges{
		specChanged: true, domainsChanged: true,
	}); err != nil {
		t.Fatalf("Blueprint domain sync: %v", err)
	}
	if app := getApp(t, cl, "web"); app.Spec.Host != "" || len(app.Spec.Hosts) != 0 {
		t.Fatalf("pending Blueprint claim reached serving spec: %+v", app.Spec)
	}
	claim := claims.claims["blueprint.example.com"]
	if claim.ClaimState != "pending" || !claim.Primary {
		t.Fatalf("Blueprint claim = %+v", claim)
	}
	challenge := claim.Challenge
	current := getApp(t, cl, "web")
	retry := current.DeepCopy()
	retry.Spec.Host = "blueprint.example.com"
	if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: "web"}, current, retry, stackChanges{
		specChanged: true, domainsChanged: true,
	}); err != nil {
		t.Fatalf("Blueprint retry: %v", err)
	}
	if got := claims.claims["blueprint.example.com"].Challenge; got != challenge {
		t.Fatalf("Blueprint retry rotated challenge: %q -> %q", challenge, got)
	}
}

func (v *recordingDomainOwnership) VerifyTXT(_ context.Context, name, value string) error {
	v.name, v.value = name, value
	return v.err
}

func TestAddDomainRequiresCurrentAppBoundTXTBeforeRouting(t *testing.T) {
	a := sampleApp("web")
	a.Labels = map[string]string{core.LabelAppID: "srv-current"}
	svc, cl := newService(nil, a)
	proof := &recordingDomainOwnership{err: errors.New("stale CNAME only")}
	svc.DomainOwnership = proof

	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("unproved domain = %v, want conflict", err)
	}
	if proof.name != "_bex-challenge.example.com" || !strings.HasPrefix(proof.value, "bex-domain-verification=") {
		t.Fatalf("challenge = %q %q", proof.name, proof.value)
	}
	if got := getApp(t, cl, "web"); len(got.Spec.Hosts) != 0 {
		t.Fatalf("unproved domain became routable: %v", got.Spec.Hosts)
	}

	proof.err = nil
	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); err != nil {
		t.Fatalf("proved domain: %v", err)
	}
	if got := getApp(t, cl, "web"); !slices.Contains(got.Spec.Hosts, "app.example.com") {
		t.Fatalf("proved domain not routed: %v", got.Spec.Hosts)
	}
}

func TestAddDomainIdempotent(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Second add of the same hostname must be a no-op (still one entry).
	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); err != nil {
		t.Fatalf("second add: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 {
		t.Errorf("duplicate add must not create a second entry, got %v", got)
	}
}

func TestAddDomainEmptyHostnameIsBadRequest(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty hostname => ErrBadRequest, got %v", err)
	}
}

// round-5 finding 7: a wildcard custom domain and a concrete host beneath it are
// compared literally by both the collision check and the store's UNIQUE(host)
// constraint, so both would go live and the wildcard could hijack a concrete
// host routed to another tenant. Reject wildcards at the write boundary.
func TestAddDomainRejectsWildcard(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	for _, host := range []string{"*.example.com", "*.foo.example.com", "foo.*.example.com"} {
		if _, err := svc.AddDomain(context.Background(), "web", host); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("wildcard %q => ErrBadRequest, got %v", host, err)
		}
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("rejected wildcard must not touch spec.hosts, got %v", got)
	}
}

// --- w7/m6: cross-app collision + reserved-host guards ---

// TestAddDomainRejectsHostOnAnotherApp: a host registered on a different App
// (any tenant) is refused with core.ErrConflict — Render's "already exists on
// another site" — and the claiming App's spec.hosts stays untouched. Covers both
// the other App's spec.hosts[] and its primary spec.host.
func TestAddDomainRejectsHostOnAnotherApp(t *testing.T) {
	owner := appWithHosts("owner", "www.example.com")
	primaryOwner := sampleApp("primary")
	primaryOwner.Spec.Host = "apex.example.com"
	svc, cl := newService(nil, owner, primaryOwner, sampleApp("web"))

	for _, host := range []string{"www.example.com", "apex.example.com"} {
		if _, err := svc.AddDomain(context.Background(), "web", host); !errors.Is(err, core.ErrConflict) {
			t.Errorf("add %q claimed elsewhere => ErrConflict, got %v", host, err)
		}
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("rejected add must not touch spec.hosts, got %v", got)
	}
}

// TestAddDomainOwnHostNotAConflict: the collision guard must not false-positive
// on the App's own already-registered host — a re-add stays idempotent even with
// other Apps present.
func TestAddDomainOwnHostNotAConflict(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("web", "www.example.com"), sampleApp("other"))
	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Errorf("re-adding own host must be idempotent, got %v", err)
	}
}

// TestAddDomainStoreConflictMapsToConflict: if the CR scan misses a concurrent
// add (the race the DB UNIQUE index closes), the store's ErrConflict is mapped to
// core.ErrConflict, not a 500, and the CR is left untouched.
func TestAddDomainStoreConflictMapsToConflict(t *testing.T) {
	rec := &recordingStore{err: store.ErrConflict}
	svc, cl := newService(rec, managedApp("web", "srv-1"))
	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); !errors.Is(err, core.ErrConflict) {
		t.Errorf("store ErrConflict => core.ErrConflict, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("store-conflict add must not touch spec.hosts, got %v", got)
	}
}

// TestAddDomainReservedPlatformHosts: with BaseDomain set, the apex and any
// foreign `<x>.<base>` platform host are refused (core.ErrBadRequest), while the
// App's own `<app>.<base>` auto host is allowed (Render lets a service keep its
// own platform subdomain).
func TestAddDomainReservedPlatformHosts(t *testing.T) {
	svc, cl := newBaseDomainService("onbex.co", "", sampleApp("web"))
	for _, host := range []string{"onbex.co", "other.onbex.co", "api.onbex.co"} {
		if _, err := svc.AddDomain(context.Background(), "web", host); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("reserved host %q => ErrBadRequest, got %v", host, err)
		}
	}
	// The App's own auto host is not reserved.
	if _, err := svc.AddDomain(context.Background(), "web", "web.onbex.co"); err != nil {
		t.Errorf("own auto host web.onbex.co must be allowed, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "web.onbex.co" {
		t.Errorf("own auto host should append, got %v", got)
	}
}

// TestAddDomainReservedDashboardHost: the configured dashboard host is reserved.
func TestAddDomainReservedDashboardHost(t *testing.T) {
	svc, _ := newBaseDomainService("onbex.co", "dashboard.bex.co", sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "dashboard.bex.co"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("dashboard host => ErrBadRequest, got %v", err)
	}
}

// --- w6/m23: www<->apex sibling pairing ---

// TestAddDomainAutoPairsWwwSibling: adding an apex auto-adds its www sibling,
// per the Render capture (docs/render-artifacts/custom-domain-pairing.md).
func TestAddDomainAutoPairsWwwSibling(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 2 || got[0] != "foo.com" || got[1] != "www.foo.com" {
		t.Errorf("spec.hosts = %v, want [foo.com www.foo.com]", got)
	}
	if got := getApp(t, cl, "web").Spec.HostRedirects; len(got) != 1 || got["www.foo.com"] != "foo.com" {
		t.Errorf("spec.hostRedirects = %v, want www.foo.com -> foo.com", got)
	}
}

// TestAddDomainNormalizesCaseBeforePairing: a mixed-case add is lowercased
// before the sibling is computed and stored, so the pair (and any later
// cross-app collision match against it) can't be split by casing.
func TestAddDomainNormalizesCaseBeforePairing(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "Foo.COM"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 2 || got[0] != "foo.com" || got[1] != "www.foo.com" {
		t.Errorf("spec.hosts = %v, want [foo.com www.foo.com] (lowercased)", got)
	}
}

// TestAddDomainAutoPairsApexSibling: adding www auto-adds the apex, the
// symmetric direction.
func TestAddDomainAutoPairsApexSibling(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "www.foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 2 || got[0] != "www.foo.com" || got[1] != "foo.com" {
		t.Errorf("spec.hosts = %v, want [www.foo.com foo.com]", got)
	}
	if got := getApp(t, cl, "web").Spec.HostRedirects; len(got) != 1 || got["foo.com"] != "www.foo.com" {
		t.Errorf("spec.hostRedirects = %v, want foo.com -> www.foo.com", got)
	}
}

// TestAddDomainReAddingPairedSiblingMakesBothExplicit: explicitly adding the
// auto-created sibling clears its redirect and leaves both hosts served
// directly, without creating duplicates.
func TestAddDomainReAddingPairedSiblingMakesBothExplicit(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "foo.com"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := svc.AddDomain(context.Background(), "web", "www.foo.com"); err != nil {
		t.Errorf("re-adding the auto-added sibling directly must be idempotent, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 2 {
		t.Errorf("still just the pair, got %v", got)
	}
	if got := getApp(t, cl, "web").Spec.HostRedirects; len(got) != 0 {
		t.Errorf("explicit-both must clear the sibling redirect, got %v", got)
	}
}

// TestAddDomainNoAutoPairForNonWwwSubdomain: a deep subdomain (wwwSibling
// returns "", t002) gets no auto-added sibling — only www<->apex pairs.
func TestAddDomainNoAutoPairForNonWwwSubdomain(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "app.foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "app.foo.com" {
		t.Errorf("spec.hosts = %v, want [app.foo.com] (no pairing)", got)
	}
}

// TestAddDomainAutoPairsPublicSuffixApex: the DoD's public-suffix claim,
// exercised through AddDomain itself (not just the wwwSibling unit tests) — a
// multi-label public suffix like .co.uk pairs on its registrable domain, not
// on the last two labels ("co.uk" is not itself an apex).
func TestAddDomainAutoPairsPublicSuffixApex(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "foo.co.uk"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 2 || got[0] != "foo.co.uk" || got[1] != "www.foo.co.uk" {
		t.Errorf("spec.hosts = %v, want [foo.co.uk www.foo.co.uk]", got)
	}
}

// TestAddDomainSiblingClaimedElsewhereIsConflict closes w7/m6's documented
// blind spot (w6/m23 t004): registering www.foo.com on app A now reserves
// foo.com against app B too, with the same 409 the per-host guard uses — and
// the reverse direction (apex reserving www).
func TestAddDomainSiblingClaimedElsewhereIsConflict(t *testing.T) {
	cases := []struct{ existing, add string }{
		{"www.foo.com", "foo.com"},
		{"foo.com", "www.foo.com"},
	}
	for _, tc := range cases {
		t.Run(tc.add, func(t *testing.T) {
			svc, cl := newService(nil, appWithHosts("a", tc.existing), sampleApp("b"))
			if _, err := svc.AddDomain(context.Background(), "b", tc.add); !errors.Is(err, core.ErrConflict) {
				t.Errorf("sibling of another App's %q => ErrConflict, got %v", tc.existing, err)
			}
			if got := getApp(t, cl, "b").Spec.Hosts; len(got) != 0 {
				t.Errorf("rejected add must not touch spec.hosts, got %v", got)
			}
		})
	}
}

// TestAddDomainSiblingReservedIsSkippedNotFailed: if the auto-paired sibling
// happens to be a platform-reserved host, the primary add still succeeds — the
// sibling pairing is best-effort (domains.go AddDomain doc comment). Uses
// DashboardHost (an exact-match reserved host, unrelated to BaseDomain) set to
// exactly the sibling "www.foo.com", so "foo.com" itself is unreserved.
func TestAddDomainSiblingReservedIsSkippedNotFailed(t *testing.T) {
	svc, cl := newBaseDomainService("", "www.foo.com", sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "foo.com" {
		t.Errorf("spec.hosts = %v, want [foo.com] (reserved sibling www.foo.com skipped)", got)
	}
}

// TestDeleteDomainLeavesSiblingUntouched pins bex's documented delete
// semantics (docs/render-artifacts/custom-domain-pairing.md, w6/m23 t001):
// Render doesn't specify sibling-delete behavior, so bex treats each half as
// an independent spec.hosts[] entry — deleting one never removes the other.
func TestDeleteDomainLeavesSiblingUntouched(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", "foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if err := svc.DeleteDomain(context.Background(), "web", "foo.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "www.foo.com" {
		t.Errorf("spec.hosts = %v, want [www.foo.com] (sibling survives the delete)", got)
	}
	if got := getApp(t, cl, "web").Spec.HostRedirects; len(got) != 0 {
		t.Errorf("surviving sibling must serve directly after its target is deleted, got redirects %v", got)
	}
}

func TestDeleteDomainRemovesFromHosts(t *testing.T) {
	svc, cl := newService(nil, appWithHosts("web", "www.example.com", "api.example.com"))

	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "api.example.com" {
		t.Errorf("spec.hosts = %v, want [api.example.com]", got)
	}
}

func TestDeleteDomainIdempotent(t *testing.T) {
	svc, cl := newService(nil, appWithHosts("web", "www.example.com"))

	// Remove existing host.
	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	// Removing again must be a silent no-op.
	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("second delete (no-op) should not error: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("spec.hosts should be empty, got %v", got)
	}
}

// --- Store write-through for managed Apps ---

func TestAddDomainManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if len(rec.domainAdds) != 1 || rec.domainAdds[0].id != "srv-1" || rec.domainAdds[0].host != "app.example.com" {
		t.Fatalf("want row write [srv-1 app.example.com], got %v", rec.domainAdds)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "app.example.com" {
		t.Errorf("CR spec.hosts = %v, want [app.example.com]", got)
	}
}

// TestAddDomainPairedSiblingWritesStoreRowToo: the auto-added sibling goes
// through the store write-through too — it becomes a first-class domain row,
// not a synthetic display-only record (so it participates in the domains.host
// UNIQUE index the collision guard's store-race backstop leans on, w6/m23 t004).
func TestAddDomainPairedSiblingWritesStoreRowToo(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.AddDomain(context.Background(), "web", "foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if len(rec.domainAdds) != 2 || rec.domainAdds[0].host != "foo.com" || rec.domainAdds[0].redirectForName != "" ||
		rec.domainAdds[1].host != "www.foo.com" || rec.domainAdds[1].redirectForName != "foo.com" {
		t.Fatalf("want row writes [foo.com www.foo.com], got %v", rec.domainAdds)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 2 {
		t.Errorf("CR spec.hosts = %v, want the pair", got)
	}
}

func TestDeleteDomainManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedAppWithHosts("web", "srv-1", "www.example.com"))

	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if len(rec.domainRems) != 1 || rec.domainRems[0].id != "srv-1" || rec.domainRems[0].host != "www.example.com" {
		t.Fatalf("want row remove [srv-1 www.example.com], got %v", rec.domainRems)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("CR spec.hosts should be empty, got %v", got)
	}
}

func TestAddDomainUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	a := sampleApp("hand")
	a.Labels = map[string]string{core.LabelAppID: "srv-direct"}
	svc, cl := newService(rec, a)

	if _, err := svc.AddDomain(context.Background(), "hand", "app.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if len(rec.domainAdds) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.domainAdds)
	}
	if got := getApp(t, cl, "hand").Spec.Hosts; len(got) != 1 || got[0] != "app.example.com" {
		t.Errorf("CR spec.hosts = %v", got)
	}
}

func TestAddDomainRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); err == nil {
		t.Fatal("want error when row write fails")
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("CR spec.hosts must be untouched when row write failed, got %v", got)
	}
}

// --- Verification status (TLS Secret lookup) ---

func TestDomainVerifiedWhenTLSSecretExists(t *testing.T) {
	// spec.host is set, so spec.hosts[] items are secondary → secret = "<app>-tls-<host>"
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	secret := tlsSecret("default", "web-tls-www.example.com")
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "verified" {
		t.Errorf("verificationStatus = %q, want verified (TLS secret present)", d.VerificationStatus)
	}
	if d.ServerStatus != "active" {
		t.Errorf("serverStatus = %q, want active (cert issued and not suspended)", d.ServerStatus)
	}
}

func TestDomainPendingWithoutTLSSecret(t *testing.T) {
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	cl := fakeClient(a) // no TLS secret
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "pending" {
		t.Errorf("verificationStatus = %q, want pending (no TLS secret)", d.VerificationStatus)
	}
	if d.ServerStatus != "pending" {
		t.Errorf("serverStatus = %q, want pending", d.ServerStatus)
	}
}

func TestDomainServerStatusPendingWhenSuspended(t *testing.T) {
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	a.Spec.Suspended = true
	secret := tlsSecret("default", "web-tls-www.example.com")
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	// Cert issued but app is suspended → serverStatus must be pending.
	if d.VerificationStatus != "verified" {
		t.Errorf("verificationStatus = %q, want verified", d.VerificationStatus)
	}
	if d.ServerStatus != "pending" {
		t.Errorf("serverStatus = %q, want pending (app suspended)", d.ServerStatus)
	}
}

// tlsSecretNameForFirstHost: if spec.host is unset and expose is false, spec.hosts[0]
// is the first effective host and gets the "<app>-tls" secret (not "<app>-tls-<host>").
func TestTLSSecretForFirstHostWhenNoPrimaryHost(t *testing.T) {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Hosts: []string{"www.example.com"}},
	}
	secret := tlsSecret("default", "web-tls") // first effective host → "<app>-tls"
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "verified" {
		t.Errorf("verificationStatus = %q, want verified (first host uses <app>-tls)", d.VerificationStatus)
	}
}

// --- domainType / PSL helpers (w6/m23 t002) ---

func TestDomainTypeClassification(t *testing.T) {
	cases := map[string]string{
		"example.com":       "apex",
		"www.example.com":   "subdomain",
		"api.example.com":   "subdomain",
		"a.b.example.com":   "subdomain",
		"example.co.uk":     "apex", // multi-label public suffix — the dots-count heuristic gets this wrong
		"www.example.co.uk": "subdomain",
		"app.example.co.uk": "subdomain",
	}
	for hostname, want := range cases {
		if got := domainType(hostname); got != want {
			t.Errorf("domainType(%q) = %q, want %q", hostname, got, want)
		}
	}
}

func TestRegistrableDomain(t *testing.T) {
	cases := map[string]string{
		"example.com":              "example.com",
		"www.example.com":          "example.com",
		"a.b.example.com":          "example.com",
		"example.co.uk":            "example.co.uk",
		"www.example.co.uk":        "example.co.uk",
		"xn--80akhbyknj4f.com":     "xn--80akhbyknj4f.com", // IDN punycode passthrough
		"www.xn--80akhbyknj4f.com": "xn--80akhbyknj4f.com",
		"com":                      "", // bare public suffix — no registrable domain
	}
	for host, want := range cases {
		if got := registrableDomain(host); got != want {
			t.Errorf("registrableDomain(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestIsApex(t *testing.T) {
	cases := map[string]bool{
		"example.com":       true,
		"www.example.com":   false,
		"example.co.uk":     true,
		"www.example.co.uk": false,
		"app.example.co.uk": false,
	}
	for host, want := range cases {
		if got := isApex(host); got != want {
			t.Errorf("isApex(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestWwwSibling(t *testing.T) {
	cases := map[string]string{
		"example.com":             "www.example.com",
		"www.example.com":         "example.com",
		"example.co.uk":           "www.example.co.uk",
		"www.example.co.uk":       "example.co.uk",
		"app.example.com":         "", // deep subdomain — no pairing
		"api.staging.example.com": "",
		"www.app.example.com":     "", // "www" not immediately below the registrable domain
	}
	for host, want := range cases {
		if got := wwwSibling(host); got != want {
			t.Errorf("wwwSibling(%q) = %q, want %q", host, got, want)
		}
	}
}

// --- DNS record (post-add instructions) ---

func TestDNSRecordFromBaseDomain(t *testing.T) {
	// BaseDomain set => target is <app>.<base>, computed without needing status.
	svc, _ := newService(nil, appWithHosts("web", "www.example.com", "example.com", "api.staging.example.com"))
	svc.BaseDomain = "onbex.co"

	domains, err := svc.ListDomains(context.Background(), "web")
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	byName := map[string]DomainView{}
	for _, d := range domains {
		byName[d.Name] = d
	}

	// Subdomain: CNAME <label prefix> -> platform host.
	if r := byName["www.example.com"].DNSRecord; r.Type != "CNAME" || r.Name != "www" || r.Value != "web.onbex.co" {
		t.Errorf("www.example.com dnsRecord = %+v, want {CNAME www web.onbex.co}", r)
	}
	// Deeper subdomain: record name keeps the labels below the root zone.
	if r := byName["api.staging.example.com"].DNSRecord; r.Type != "CNAME" || r.Name != "api.staging" || r.Value != "web.onbex.co" {
		t.Errorf("api.staging.example.com dnsRecord = %+v, want {CNAME api.staging web.onbex.co}", r)
	}
	// Apex: ALIAS @ -> platform host.
	if r := byName["example.com"].DNSRecord; r.Type != "ALIAS" || r.Name != "@" || r.Value != "web.onbex.co" {
		t.Errorf("example.com dnsRecord = %+v, want {ALIAS @ web.onbex.co}", r)
	}
}

func TestDNSRecordFallsBackToStatusURL(t *testing.T) {
	// BaseDomain unset => derive the platform host from status URLs. sampleApp sets
	// Status.URL = https://web.onbex.co, so the target must still resolve.
	svc, _ := newService(nil, appWithHosts("web", "www.example.com"))
	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.DNSRecord.Value != "web.onbex.co" {
		t.Errorf("dnsRecord.Value = %q, want web.onbex.co (from status URL)", d.DNSRecord.Value)
	}
}

// --- Verify verb (re-check) ---

func TestVerifyDomainReturnsFreshStatus(t *testing.T) {
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	secret := tlsSecret("default", "web-tls-www.example.com")
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, BaseDomain: "onbex.co"}

	d, err := svc.VerifyDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("VerifyDomain: %v", err)
	}
	if d.VerificationStatus != "verified" || d.ServerStatus != "active" {
		t.Errorf("verify status = %q/%q, want verified/active", d.VerificationStatus, d.ServerStatus)
	}
	if d.DNSRecord.Value != "web.onbex.co" {
		t.Errorf("verify dnsRecord.Value = %q, want web.onbex.co", d.DNSRecord.Value)
	}
}

func TestVerifyDomainUnknownHostNotFound(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("web", "www.example.com"))
	if _, err := svc.VerifyDomain(context.Background(), "web", "nope.example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown host => ErrNotFound, got %v", err)
	}
}

// --- REST fragment ---

func TestRESTCustomDomains(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// POST adds and returns 201. app.example.com is a deep subdomain — no
	// sibling pairing (t002) — so this stays a single-domain lifecycle test;
	// the paired case gets its own test (TestRESTCustomDomainsPairedAdd).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains",
		strings.NewReader(`{"name":"app.example.com"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST => 201, got %d: %s", rec.Code, rec.Body)
	}
	var created renderCustomDomain
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Name != "app.example.com" {
		t.Errorf("name = %q", created.Name)
	}
	if created.DomainType != "subdomain" {
		t.Errorf("domainType = %q, want subdomain", created.DomainType)
	}
	if created.DNSRecord.Type != "CNAME" || created.DNSRecord.Name != "app" {
		t.Errorf("dnsRecord = %+v, want CNAME/app", created.DNSRecord)
	}

	// POST …/verify re-checks and returns 200 with the fresh domain.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains/app.example.com/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST verify => 200, got %d: %s", rec.Code, rec.Body)
	}
	var verified renderCustomDomain
	if err := json.Unmarshal(rec.Body.Bytes(), &verified); err != nil {
		t.Fatalf("decode verify: %v", err)
	}
	if verified.Name != "app.example.com" || verified.DNSRecord.Type != "CNAME" {
		t.Errorf("verify result wrong: %+v", verified)
	}

	// GET list returns [{customDomain:{...}, cursor:"..."}].
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET list => 200, got %d", rec.Code)
	}
	var list []customDomainWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if list[0].CustomDomain.Name != "app.example.com" || list[0].Cursor == "" {
		t.Errorf("list item wrong: %+v", list[0])
	}

	// GET single returns the domain.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains/app.example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET single => 200, got %d", rec.Code)
	}

	// DELETE returns 204 No Content.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/services/web/custom-domains/app.example.com", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE => 204, got %d", rec.Code)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("spec.hosts should be empty after delete, got %v", got)
	}

	// GET single after delete => 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains/app.example.com", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleted domain => 404, got %d", rec.Code)
	}
}

// TestRESTCustomDomainsPairedAdd: adding an apex over REST returns the primary
// domain (201, as before) and the list surfaces both paired hosts, each with
// its own DNS instructions — w6/m23 t005.
func TestRESTCustomDomainsPairedAdd(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	svc.BaseDomain = "onbex.co"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains",
		strings.NewReader(`{"name":"foo.com"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST => 201, got %d: %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains", nil))
	var list []customDomainWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("list: %v len=%d, want 2 (the pair)", err, len(list))
	}
	byName := map[string]renderCustomDomain{}
	for _, item := range list {
		byName[item.CustomDomain.Name] = item.CustomDomain
	}
	if r := byName["foo.com"].DNSRecord; r.Type != "ALIAS" || r.Name != "@" {
		t.Errorf("foo.com dnsRecord = %+v, want ALIAS/@", r)
	}
	if r := byName["www.foo.com"].DNSRecord; r.Type != "CNAME" || r.Name != "www" {
		t.Errorf("www.foo.com dnsRecord = %+v, want CNAME/www", r)
	}
	if byName["foo.com"].RedirectForName != "" || byName["www.foo.com"].RedirectForName != "foo.com" {
		t.Errorf("REST redirectForName mismatch: %+v", byName)
	}
}

// TestRESTCustomDomainsSiblingConflict: registering www.foo.com on one service
// blocks foo.com on another over REST too, with the same 409 the per-host
// guard uses (w6/m23 t004/t009 — "on every surface").
func TestRESTCustomDomainsSiblingConflict(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("owner", "www.foo.com"), sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains",
		strings.NewReader(`{"name":"foo.com"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST sibling of another service's host => 409, got %d: %s", rec.Code, rec.Body)
	}
}

func TestRESTCustomDomainsBadRequest(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Missing name field => 400.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains",
		strings.NewReader(`{"name":""}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty name => 400, got %d", rec.Code)
	}
}

func TestRESTCustomDomainsUnknownApp(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/nope/custom-domains", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown app => 404, got %d", rec.Code)
	}
}

func TestManagedPendingDomainSurfaceParity(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	svc, _ := newService(claims, managedApp("web", "srv-1"))
	if _, err := svc.AddDomain(context.Background(), "web", "app.example.com"); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains/app.example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST get = %d: %s", rec.Code, rec.Body)
	}
	var rest renderCustomDomain
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	if rest.OwnershipStatus != "pending" || rest.OwnershipDNSRecord == nil || rest.OwnershipDNSRecord.Type != "TXT" {
		t.Fatalf("REST pending shape = %+v", rest)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{
		customDomain(id: "web", name: "app.example.com") {
			ownershipStatus ownershipDnsRecord { type name value }
		}
	}`})
	if len(gql.Errors) != 0 {
		t.Fatalf("GraphQL get: %v", gql.Errors)
	}
	gqlDomain := gql.Data.(map[string]any)["customDomain"].(map[string]any)
	if gqlDomain["ownershipStatus"] != "pending" || gqlDomain["ownershipDnsRecord"].(map[string]any)["value"] != rest.OwnershipDNSRecord.Value {
		t.Fatalf("GraphQL pending shape = %v", gqlDomain)
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpDomain := call("get_custom_domain", map[string]any{"serviceId": "web", "name": "app.example.com"})
	proof, _ := mcpDomain["ownershipDnsRecord"].(map[string]any)
	if mcpDomain["ownershipStatus"] != "pending" || proof["value"] != rest.OwnershipDNSRecord.Value {
		t.Fatalf("MCP pending shape = %v", mcpDomain)
	}
}

// --- GraphQL fragment ---

func TestGraphQLCustomDomains(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	// addCustomDomain mutation. app.example.com is a deep subdomain — no
	// sibling pairing (t002) — so this stays a single-domain lifecycle test;
	// the paired case gets its own test (TestGraphQLCustomDomainsPairedAdd).
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { addCustomDomain(id: "web", name: "app.example.com") { name domainType verificationStatus } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("addCustomDomain: %v", res.Errors)
	}
	added := res.Data.(map[string]any)["addCustomDomain"].(map[string]any)
	if added["name"] != "app.example.com" || added["domainType"] != "subdomain" {
		t.Errorf("addCustomDomain result wrong: %v", added)
	}

	// customDomains query.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ customDomains(id: "web") { name verificationStatus } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("customDomains: %v", res.Errors)
	}
	domains := res.Data.(map[string]any)["customDomains"].([]any)
	if len(domains) != 1 {
		t.Fatalf("want 1 domain, got %d", len(domains))
	}

	// customDomain query (single) — including the nested dnsRecord.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ customDomain(id: "web", name: "app.example.com") { name dnsRecord { type name value } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("customDomain: %v", res.Errors)
	}
	single := res.Data.(map[string]any)["customDomain"].(map[string]any)
	rec := single["dnsRecord"].(map[string]any)
	if rec["type"] != "CNAME" || rec["name"] != "app" {
		t.Errorf("dnsRecord = %v, want CNAME/app", rec)
	}

	// verifyCustomDomain mutation — re-check returns the fresh domain.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { verifyCustomDomain(id: "web", name: "app.example.com") { name verificationStatus dnsRecord { type } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("verifyCustomDomain: %v", res.Errors)
	}
	if v := res.Data.(map[string]any)["verifyCustomDomain"].(map[string]any); v["name"] != "app.example.com" {
		t.Errorf("verifyCustomDomain result wrong: %v", v)
	}

	// deleteCustomDomain mutation.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { deleteCustomDomain(id: "web", name: "app.example.com") }`})
	if len(res.Errors) > 0 {
		t.Fatalf("deleteCustomDomain: %v", res.Errors)
	}
	if deleted := res.Data.(map[string]any)["deleteCustomDomain"]; deleted != true {
		t.Errorf("deleteCustomDomain should return true on success, got %v", deleted)
	}
}

// TestGraphQLCustomDomainsPairedAdd: addCustomDomain on an apex auto-pairs the
// www sibling, and customDomains surfaces both with their own dnsRecord —
// w6/m23 t005.
func TestGraphQLCustomDomainsPairedAdd(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	svc.BaseDomain = "onbex.co"
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { addCustomDomain(id: "web", name: "foo.com") { name } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("addCustomDomain: %v", res.Errors)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ customDomains(id: "web") { name redirectForName dnsRecord { type name } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("customDomains: %v", res.Errors)
	}
	domains := res.Data.(map[string]any)["customDomains"].([]any)
	if len(domains) != 2 {
		t.Fatalf("want 2 domains (the pair), got %d", len(domains))
	}
	byName := map[string]map[string]any{}
	for _, d := range domains {
		m := d.(map[string]any)
		byName[m["name"].(string)] = m
	}
	if rec := byName["foo.com"]["dnsRecord"].(map[string]any); rec["type"] != "ALIAS" || rec["name"] != "@" {
		t.Errorf("foo.com dnsRecord = %v, want ALIAS/@", rec)
	}
	if rec := byName["www.foo.com"]["dnsRecord"].(map[string]any); rec["type"] != "CNAME" || rec["name"] != "www" {
		t.Errorf("www.foo.com dnsRecord = %v, want CNAME/www", rec)
	}
	if byName["foo.com"]["redirectForName"] != nil || byName["www.foo.com"]["redirectForName"] != "foo.com" {
		t.Errorf("GraphQL redirectForName mismatch: %v", byName)
	}
}

// TestGraphQLCustomDomainsSiblingConflict: the sibling collision guard surfaces
// as a GraphQL error too (w6/m23 t004/t009 — "on every surface").
func TestGraphQLCustomDomainsSiblingConflict(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("owner", "www.foo.com"), sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { addCustomDomain(id: "web", name: "foo.com") { name } }`})
	if len(res.Errors) == 0 {
		t.Fatal("addCustomDomain sibling of another service's host: want a GraphQL error, got none")
	}
}

// --- MCP ---

// TestMCPCustomDomainsPairedAdd: add_custom_domain over MCP auto-pairs the
// sibling and list_custom_domains surfaces both with their own DNS record
// (w6/m23 t005 — "on every surface").
func TestMCPCustomDomainsPairedAdd(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	svc.BaseDomain = "onbex.co"
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	call("add_custom_domain", map[string]any{"serviceId": "web", "name": "foo.com"})

	list := call("list_custom_domains", map[string]any{"serviceId": "web"})
	domains, _ := list["customDomains"].([]any)
	if len(domains) != 2 {
		t.Fatalf("list_custom_domains: want 2 domains (the pair), got %v", list)
	}
	byName := map[string]map[string]any{}
	for _, d := range domains {
		m := d.(map[string]any)
		byName[m["name"].(string)] = m
	}
	if rec, _ := byName["foo.com"]["dnsRecord"].(map[string]any); rec["type"] != "ALIAS" || rec["name"] != "@" {
		t.Errorf("foo.com dnsRecord = %v, want ALIAS/@", rec)
	}
	if rec, _ := byName["www.foo.com"]["dnsRecord"].(map[string]any); rec["type"] != "CNAME" || rec["name"] != "www" {
		t.Errorf("www.foo.com dnsRecord = %v, want CNAME/www", rec)
	}
	if _, present := byName["foo.com"]["redirectForName"]; present {
		t.Errorf("direct canonical must omit redirectForName: %v", byName["foo.com"])
	}
	if byName["www.foo.com"]["redirectForName"] != "foo.com" {
		t.Errorf("MCP sibling redirectForName = %v, want foo.com", byName["www.foo.com"]["redirectForName"])
	}
}

// TestMCPCustomDomainsSiblingConflict: the sibling collision guard rejects a
// second service's add as an MCP tool error, not a transport error (w6/m23
// t004/t009 — "on every surface").
func TestMCPCustomDomainsSiblingConflict(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("owner", "www.foo.com"), sampleApp("web"))
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add_custom_domain",
		Arguments: map[string]any{"serviceId": "web", "name": "foo.com"},
	})
	if err != nil {
		t.Fatalf("add_custom_domain sibling conflict: transport err=%v", err)
	}
	if !res.IsError {
		t.Errorf("add_custom_domain sibling of another service's host: want a tool error, got %+v", res)
	}
}

// --- w7/m38: REST list pagination + filter ---

// TestRESTListDomainsPagination: paging through more domains than a single page
// terminates, returns all items exactly once, in cursor-sorted order.
func TestRESTListDomainsPagination(t *testing.T) {
	// Build an app with 5 subdomains so we can page with limit=2.
	app := appWithHosts("web",
		"app.example.com",
		"api.example.com",
		"www.example.com",
		"cdn.example.com",
		"mail.example.com",
	)
	app.Spec.Host = "web.onbex.co" // forces all spec.hosts[] to use "<app>-tls-<host>" names
	svc, _ := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Walk all pages with limit=2, collecting names.
	var (
		cursor   string
		allNames []string
	)
	for {
		url := "/v1/services/web/custom-domains?limit=2"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET page => %d: %s", rec.Code, rec.Body)
		}
		var page []customDomainWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, item := range page {
			allNames = append(allNames, item.CustomDomain.Name)
			cursor = item.Cursor
		}
	}
	if len(allNames) != 5 {
		t.Fatalf("pagination walk: got %d names, want 5: %v", len(allNames), allNames)
	}
	// Verify no duplicates (StablePage guarantees this).
	seen := map[string]bool{}
	for _, n := range allNames {
		if seen[n] {
			t.Errorf("duplicate domain in paginated walk: %q", n)
		}
		seen[n] = true
	}
}

// TestRESTListDomainsFilterVerificationStatus: verificationStatus=verified returns
// only domains with a TLS secret; verificationStatus=pending the rest.
func TestRESTListDomainsFilterVerificationStatus(t *testing.T) {
	app := appWithHosts("web", "www.example.com", "api.example.com")
	app.Spec.Host = "web.onbex.co"
	// Only www.example.com gets a TLS secret → verified.
	secret := tlsSecret("default", "web-tls-www.example.com")
	cl := fakeClient(app, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	getList := func(vs string) []customDomainWithCursor {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET",
			"/v1/services/web/custom-domains?verificationStatus="+vs, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET verificationStatus=%s => %d: %s", vs, rec.Code, rec.Body)
		}
		var list []customDomainWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return list
	}

	verified := getList("verified")
	if len(verified) != 1 || verified[0].CustomDomain.Name != "www.example.com" {
		t.Errorf("verificationStatus=verified: want [www.example.com], got %v", verified)
	}
	pending := getList("pending")
	if len(pending) != 1 || pending[0].CustomDomain.Name != "api.example.com" {
		t.Errorf("verificationStatus=pending: want [api.example.com], got %v", pending)
	}
}

// TestRESTListDomainsFilterDomainType: domainType=apex returns only apex
// domains; domainType=subdomain returns subdomains.
func TestRESTListDomainsFilterDomainType(t *testing.T) {
	// foo.com is apex; www.foo.com and api.foo.com are subdomains.
	app := appWithHosts("web", "foo.com", "www.foo.com", "api.foo.com")
	app.Spec.Host = "web.onbex.co"
	svc, _ := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	getList := func(dt string) []customDomainWithCursor {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET",
			"/v1/services/web/custom-domains?domainType="+dt, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET domainType=%s => %d: %s", dt, rec.Code, rec.Body)
		}
		var list []customDomainWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return list
	}

	apexes := getList("apex")
	if len(apexes) != 1 || apexes[0].CustomDomain.Name != "foo.com" {
		t.Errorf("domainType=apex: want [foo.com], got %v", apexes)
	}
	subs := getList("subdomain")
	if len(subs) != 2 {
		t.Fatalf("domainType=subdomain: want 2 items, got %v", subs)
	}
	subNames := map[string]bool{}
	for _, s := range subs {
		subNames[s.CustomDomain.Name] = true
	}
	if !subNames["www.foo.com"] || !subNames["api.foo.com"] {
		t.Errorf("domainType=subdomain: missing expected host, got %v", subs)
	}
}

// TestRESTListDomainsInvalidEnumBadRequest: unknown verificationStatus or
// domainType values return 400 Bad Request.
func TestRESTListDomainsInvalidEnumBadRequest(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, url := range []string{
		"/v1/services/web/custom-domains?verificationStatus=unknown",
		"/v1/services/web/custom-domains?verificationStatus=VERIFIED",
		"/v1/services/web/custom-domains?domainType=unknown",
		"/v1/services/web/custom-domains?domainType=APEX",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s => %d, want 400", url, rec.Code)
		}
	}
}

// --- w7/m40: GraphQL + MCP list filters & pagination ---

// TestGQLCustomDomainsFilters: verificationStatus / domainType / cursor / limit
// args on customDomains(id:…) match the REST semantics w7/m38 shipped.
func TestGQLCustomDomainsFilters(t *testing.T) {
	// foo.com is apex (pending — no TLS secret); www.foo.com is subdomain+redirect.
	app := appWithHosts("web", "foo.com", "api.example.com")
	app.Spec.Host = "web.onbex.co"
	// api.example.com gets a TLS secret → verified.
	secret := tlsSecret("default", "web-tls-api.example.com")
	cl := fakeClient(app, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, BaseDomain: "onbex.co"}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	run := func(args string) []any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			RequestString: `{ customDomains(` + args + `) { name verificationStatus domainType } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("customDomains(%s): %v", args, res.Errors)
		}
		return res.Data.(map[string]any)["customDomains"].([]any)
	}

	// verificationStatus filter.
	verified := run(`id: "web", verificationStatus: "verified"`)
	if len(verified) != 1 || verified[0].(map[string]any)["name"] != "api.example.com" {
		t.Errorf("verificationStatus=verified: want [api.example.com], got %v", verified)
	}
	pending := run(`id: "web", verificationStatus: "pending"`)
	if len(pending) != 1 || pending[0].(map[string]any)["name"] != "foo.com" {
		t.Errorf("verificationStatus=pending: want [foo.com], got %v", pending)
	}

	// domainType filter.
	apexes := run(`id: "web", domainType: "apex"`)
	if len(apexes) != 1 || apexes[0].(map[string]any)["name"] != "foo.com" {
		t.Errorf("domainType=apex: want [foo.com], got %v", apexes)
	}
	subs := run(`id: "web", domainType: "subdomain"`)
	if len(subs) != 1 || subs[0].(map[string]any)["name"] != "api.example.com" {
		t.Errorf("domainType=subdomain: want [api.example.com], got %v", subs)
	}

	// Unknown enum → error.
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ customDomains(id: "web", verificationStatus: "VERIFIED") { name } }`})
	if len(res.Errors) == 0 {
		t.Error("unknown verificationStatus should return an error")
	}
}

// TestGQLCustomDomainsPagination: cursor/limit args page the list the same way
// REST does, returning all items exactly once in stable order.
func TestGQLCustomDomainsPagination(t *testing.T) {
	app := appWithHosts("web",
		"a.example.com", "b.example.com", "c.example.com", "d.example.com", "e.example.com",
	)
	app.Spec.Host = "web.onbex.co"
	svc, _ := newService(nil, app)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	var (
		cursor   string
		allNames []string
	)
	for {
		args := `id: "web", limit: 2`
		if cursor != "" {
			args += `, cursor: "` + cursor + `"`
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			RequestString: `{ customDomains(` + args + `) { name } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("customDomains page: %v", res.Errors)
		}
		page := res.Data.(map[string]any)["customDomains"].([]any)
		if len(page) == 0 {
			break
		}
		for _, item := range page {
			name := item.(map[string]any)["name"].(string)
			allNames = append(allNames, name)
			cursor = name
		}
	}
	if len(allNames) != 5 {
		t.Fatalf("pagination walk: got %d names, want 5: %v", len(allNames), allNames)
	}
	seen := map[string]bool{}
	for _, n := range allNames {
		if seen[n] {
			t.Errorf("duplicate domain in GraphQL paginated walk: %q", n)
		}
		seen[n] = true
	}
}

// TestMCPListCustomDomainsFilters: verificationStatus / domainType / cursor /
// limit args on list_custom_domains match the REST semantics.
func TestMCPListCustomDomainsFilters(t *testing.T) {
	app := appWithHosts("web", "foo.com", "api.example.com")
	app.Spec.Host = "web.onbex.co"
	secret := tlsSecret("default", "web-tls-api.example.com")
	cl := fakeClient(app, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, BaseDomain: "onbex.co"}
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	domainNames := func(res map[string]any) []string {
		t.Helper()
		domains, _ := res["customDomains"].([]any)
		names := make([]string, 0, len(domains))
		for _, d := range domains {
			names = append(names, d.(map[string]any)["name"].(string))
		}
		return names
	}

	// verificationStatus filter.
	got := domainNames(call("list_custom_domains", map[string]any{"serviceId": "web", "verificationStatus": "verified"}))
	if len(got) != 1 || got[0] != "api.example.com" {
		t.Errorf("verificationStatus=verified: want [api.example.com], got %v", got)
	}
	got = domainNames(call("list_custom_domains", map[string]any{"serviceId": "web", "verificationStatus": "pending"}))
	if len(got) != 1 || got[0] != "foo.com" {
		t.Errorf("verificationStatus=pending: want [foo.com], got %v", got)
	}

	// domainType filter.
	got = domainNames(call("list_custom_domains", map[string]any{"serviceId": "web", "domainType": "apex"}))
	if len(got) != 1 || got[0] != "foo.com" {
		t.Errorf("domainType=apex: want [foo.com], got %v", got)
	}
	got = domainNames(call("list_custom_domains", map[string]any{"serviceId": "web", "domainType": "subdomain"}))
	if len(got) != 1 || got[0] != "api.example.com" {
		t.Errorf("domainType=subdomain: want [api.example.com], got %v", got)
	}

	// cursor is present and non-empty in each page's result.
	res := call("list_custom_domains", map[string]any{"serviceId": "web", "limit": float64(1)})
	if _, ok := res["cursor"]; !ok || res["cursor"] == "" {
		t.Errorf("list_custom_domains page: expected non-empty cursor, got %v", res)
	}
}

// TestFilterDomainsComposesBothFilters: the two filters AND together, and each
// one alone leaves the other dimension untouched. REST, GraphQL, and MCP all
// route through filterDomains, so this pins the semantics once for all three.
func TestFilterDomainsComposesBothFilters(t *testing.T) {
	domains := []DomainView{
		{Name: "apex-pending.com", DomainType: "apex", VerificationStatus: "pending"},
		{Name: "apex-verified.com", DomainType: "apex", VerificationStatus: "verified"},
		{Name: "www.sub-pending.com", DomainType: "subdomain", VerificationStatus: "pending"},
		{Name: "www.sub-verified.com", DomainType: "subdomain", VerificationStatus: "verified"},
	}

	cases := []struct {
		status, dtype string
		want          []string
	}{
		{"", "", []string{"apex-pending.com", "apex-verified.com", "www.sub-pending.com", "www.sub-verified.com"}},
		{"verified", "", []string{"apex-verified.com", "www.sub-verified.com"}},
		{"", "apex", []string{"apex-pending.com", "apex-verified.com"}},
		{"verified", "apex", []string{"apex-verified.com"}},
		{"pending", "subdomain", []string{"www.sub-pending.com"}},
		{"unverified", "subdomain", []string{"www.sub-pending.com"}},
	}
	for _, c := range cases {
		got, err := filterDomains(domains, c.status, c.dtype)
		if err != nil {
			t.Fatalf("filterDomains(%q, %q): %v", c.status, c.dtype, err)
		}
		names := make([]string, 0, len(got))
		for _, d := range got {
			names = append(names, d.Name)
		}
		if !slices.Equal(names, c.want) {
			t.Errorf("filterDomains(%q, %q) = %v, want %v", c.status, c.dtype, names, c.want)
		}
	}

	// Every unrecognized value is a bad request, including a correctly-spelled
	// one in the wrong case — Render's enums are lowercase.
	for _, bad := range []struct{ status, dtype string }{
		{"unknown", ""}, {"VERIFIED", ""}, {"", "unknown"}, {"", "APEX"},
	} {
		if _, err := filterDomains(domains, bad.status, bad.dtype); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("filterDomains(%q, %q) = %v, want ErrBadRequest", bad.status, bad.dtype, err)
		}
	}
}

// TestMCPListCustomDomainsInvalidEnumToolError: an unrecognized filter value
// surfaces as an MCP tool error, matching REST's 400 and GraphQL's error —
// the rejection path is wired on all three surfaces, not just the two with
// existing coverage.
func TestMCPListCustomDomainsInvalidEnumToolError(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("web", "foo.com"))
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	for _, args := range []map[string]any{
		{"serviceId": "web", "verificationStatus": "unknown"},
		{"serviceId": "web", "verificationStatus": "VERIFIED"},
		{"serviceId": "web", "domainType": "unknown"},
		{"serviceId": "web", "domainType": "APEX"},
	} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_custom_domains", Arguments: args})
		if err != nil {
			t.Fatalf("list_custom_domains %v: transport err=%v", args, err)
		}
		if !res.IsError {
			t.Errorf("list_custom_domains %v: want a tool error, got %+v", args, res)
		}
	}
}

// --- helpers ---

func managedAppWithHosts(name, appID string, hosts ...string) *appv1alpha1.App {
	a := managedApp(name, appID)
	a.Spec.Hosts = hosts
	return a
}
