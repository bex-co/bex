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
	"strings"
	"testing"

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

// --- helpers ---

func managedAppWithHosts(name, appID string, hosts ...string) *appv1alpha1.App {
	a := managedApp(name, appID)
	a.Spec.Hosts = hosts
	return a
}
