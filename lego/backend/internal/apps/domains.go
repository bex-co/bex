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
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DomainOwnershipVerifier checks one exact TXT value at a challenge name.
// Implementations must treat resolver errors and absent/mismatched values as a
// refusal. The narrow seam keeps DNS deterministic in tests.
type DomainOwnershipVerifier interface {
	VerifyTXT(ctx context.Context, name, value string) error
}

type systemDomainOwnershipVerifier struct{}

func (systemDomainOwnershipVerifier) VerifyTXT(ctx context.Context, name, value string) error {
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	values, err := net.DefaultResolver.LookupTXT(lookupCtx, name)
	if err != nil {
		return err
	}
	if !slices.Contains(values, value) {
		return errors.New("TXT challenge value not found")
	}
	return nil
}

func domainOwnershipChallenge(app *appv1alpha1.App, host string) (name, value string) {
	root := registrableDomain(host)
	if root == "" {
		root = host
	}
	appID := managedAppID(app)
	if appID == "" {
		appID = app.Labels[core.LabelAppID]
	}
	if appID == "" {
		appID = string(app.UID)
	}
	if appID == "" {
		appID = app.Namespace + "/" + app.Name
	}
	digest := sha256.Sum256([]byte(appID + "\x00" + root))
	return "_bex-challenge." + root, fmt.Sprintf("bex-domain-verification=%x", digest[:16])
}

func (s *Service) requireDomainOwnership(ctx context.Context, app *appv1alpha1.App, host string) error {
	name, value := domainOwnershipChallenge(app, host)
	verifier := s.DomainOwnership
	if verifier == nil {
		verifier = systemDomainOwnershipVerifier{}
	}
	if err := verifier.VerifyTXT(ctx, name, value); err != nil {
		return fmt.Errorf("%w: domain ownership is unverified; create TXT %s with value %s and retry", core.ErrConflict, name, value)
	}
	return nil
}

// DomainView is the neutral bex projection of a custom domain on an App.
type DomainView struct {
	Name               string
	DomainType         string // "apex" or "subdomain" (Render's enum)
	VerificationStatus string // "pending" or "verified" (TLS cert issued?)
	ServerStatus       string // "active" or "pending"
	RedirectForName    string // canonical host for an auto-paired sibling; empty when served directly
	// DNSRecord is the record the tenant must create at their registrar to point
	// this domain at the service (Render's post-add DNS instructions, w5/m10).
	DNSRecord DNSRecordView
}

// DNSRecordView is the single DNS record a tenant creates to point a custom
// domain at its service — the three fields any DNS record needs. Captured from
// Render's add-domain flow (docs/render-artifacts/custom-domain-dns-instructions.md):
//   - subdomain → CNAME <label prefix> -> <app>.<base-domain>
//   - apex      → ALIAS  @             -> <app>.<base-domain>
//
// (bex points apex at the platform host via ALIAS/ANAME/CNAME-flattening rather
// than a bare A-record IP — the edge is Cloudflare-proxied, docs/ADR005-custom-domain.md.)
type DNSRecordView struct {
	Type  string // "CNAME" (subdomain) or "ALIAS" (apex)
	Name  string // the record host to create: the subdomain label(s), or "@" for apex
	Value string // the target the record points to: the platform host <app>.<base-domain>
}

// registrableDomain returns the eTLD+1 (registrable domain) of host — e.g.
// "example.co.uk" for both "example.co.uk" and "www.example.co.uk" — backed by
// the real public-suffix list (golang.org/x/net/publicsuffix) rather than a
// dots-count heuristic, so multi-label public suffixes (co.uk, com.au, …) are
// classified correctly. Empty if host has no registrable domain (a bare public
// suffix like "com", an IP literal, or an unrecognized single-label host).
func registrableDomain(host string) string {
	etldPlus1, err := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(host))
	if err != nil {
		return ""
	}
	return etldPlus1
}

// isApex reports whether host IS its own registrable domain, i.e. it has no
// label below the eTLD+1 — "example.com" and "example.co.uk" are apexes,
// "www.example.com" and "app.example.co.uk" are not.
func isApex(host string) bool {
	reg := registrableDomain(host)
	return reg != "" && reg == strings.ToLower(host)
}

// domainType classifies a hostname as "apex" (registrable domain itself) or
// "subdomain" (anything below it), via isApex/registrableDomain (the real PSL,
// not a dots-count heuristic — see registrableDomain).
func domainType(hostname string) string {
	if isApex(hostname) {
		return "apex"
	}
	return "subdomain"
}

// normalizeHostname canonicalizes a custom-hostname value: trim surrounding
// whitespace, trim one terminal dot, lowercase. DNS names are
// case-insensitive and a trailing dot is the root-label spelling of the same
// name, so two inputs that differ only by case/dot/whitespace are ONE host —
// persisting them verbatim let case-variant claims slip past the uniqueness
// sweep while downstream consumers (the activator's last-write-wins host map)
// canonicalized and collapsed them.
func normalizeHostname(raw string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
}

// canonicalHostname is normalizeHostname plus DNS-1123 host validation — the
// ONE canonicalization every custom-hostname write boundary (create/update
// spec.hosts, AddDomain) runs before persisting, so stored values are always
// canonical and comparable.
//
// Wildcard hosts ("*.example.com") are REJECTED here (round-5 finding 7). The
// collision check and the store's UNIQUE(host) constraint both compare literal
// hosts, so a wildcard and a concrete host beneath it (foo.example.com) look
// non-colliding and BOTH become live public TLS Ingress rules — one tenant's
// "*.example.com" would then hijack a concrete host routed to another tenant.
// Render offers no tenant wildcard custom domain, so nothing legitimate needs
// one; platform wildcard hosts (*.onbex.co) are provisioned by the operator via
// BEX_BASE_DOMAIN, never through this tenant-facing boundary.
func canonicalHostname(raw string) (string, error) {
	host := normalizeHostname(raw)
	if strings.HasPrefix(host, "*.") || strings.Contains(host, "*") {
		return "", fmt.Errorf("%w: wildcard hostnames are not allowed: %q", core.ErrBadRequest, raw)
	}
	if host == "" || len(validation.IsDNS1123Subdomain(host)) != 0 {
		return "", fmt.Errorf("%w: invalid hostname %q", core.ErrBadRequest, raw)
	}
	return host, nil
}

// wwwSibling returns the www<->apex pairing partner Render auto-adds when a
// tenant registers host (docs/render-artifacts/custom-domain-pairing.md, w6/m23
// t001): an apex's sibling is "www."+apex; "www."+apex's sibling is the apex;
// any other host (a non-www subdomain, or a host with no registrable domain)
// has no sibling and returns "".
func wwwSibling(host string) string {
	reg := registrableDomain(host)
	if reg == "" {
		return ""
	}
	lower := strings.ToLower(host)
	switch lower {
	case reg:
		return "www." + reg
	case "www." + reg:
		return reg
	default:
		return ""
	}
}

// platformHost returns the App's platform hostname `<subdomain>.<base-domain>`
// — the CNAME/ALIAS target a custom domain points at. Prefers BEX_BASE_DOMAIN
// (the same value the operator computes URLs from, so it's correct even
// before the operator has written status), falling back to the
// `<subdomain>.<something>` entry in the App's status URLs (the
// Expose-generated host). Empty if neither is available.
//
// subdomain is spec.subdomain — the control plane's globally-unique slug
// (w4/m19), the SAME value the operator's effectiveHosts prefers — falling
// back to app.Name for an App the control plane hasn't stamped (pre-migration
// or hand-applied). Never app.Name alone: two workspaces' same-named Apps
// would otherwise get identical, colliding DNS instructions.
func (s *Service) platformHost(app *appv1alpha1.App) string {
	subdomain := app.Spec.PlatformSubdomain(app.Name)
	if s.BaseDomain != "" {
		return subdomain + "." + s.BaseDomain
	}
	prefix := subdomain + "."
	for _, u := range append([]string{app.Status.URL}, app.Status.URLs...) {
		h := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://"), "/")
		if strings.HasPrefix(h, prefix) {
			return h
		}
	}
	return ""
}

// dnsRecordFor computes the DNS record the tenant must create for host, given its
// already-classified dtype ("apex"/"subdomain") and the app's platform host as the
// target. A subdomain gets a CNAME whose record name is the label prefix below the
// root zone (www.example.com -> "www"); an apex gets an ALIAS at "@". Target is the
// platform host; empty target still yields a well-typed record (the dashboard
// renders the type/host guidance regardless).
func dnsRecordFor(host, dtype, platformHost string) DNSRecordView {
	if dtype == "apex" {
		return DNSRecordView{Type: "ALIAS", Name: "@", Value: platformHost}
	}
	// Subdomain: strip the trailing two labels (the root zone) to get the record name.
	labels := strings.Split(host, ".")
	name := host
	if len(labels) > 2 {
		name = strings.Join(labels[:len(labels)-2], ".")
	}
	return DNSRecordView{Type: "CNAME", Name: name, Value: platformHost}
}

// tlsSecretForHost returns the TLS Secret name the operator creates for a host
// in App.spec.hosts[], mirroring the operator's naming convention:
// - first effective host → "<app>-tls"
// - subsequent hosts    → "<app>-tls-<host>"
// A host in spec.hosts[] is at position 0 only when spec.host is unset and
// spec.expose is false (no platform or explicit primary host precedes it).
func tlsSecretForHost(app *appv1alpha1.App, host string) string {
	if app.Spec.Host == "" && !app.Spec.Expose && len(app.Spec.Hosts) > 0 && app.Spec.Hosts[0] == host {
		return app.Name + "-tls"
	}
	name := app.Name + "-tls-" + strings.ReplaceAll(host, "*", "wildcard")
	if len(name) > 253 {
		name = name[:253]
	}
	return name
}

// domainVerified reports whether cert-manager has issued a TLS certificate for
// the host by looking for the corresponding TLS Secret. Any absence or error
// is treated as "pending" — the conservative state during cert issuance.
func (s *Service) domainVerified(ctx context.Context, app *appv1alpha1.App, host string) bool {
	var sec corev1.Secret
	// cert-manager writes the TLS Secret into the App's own namespace (its
	// Ingress lives there), which is the per-tenant `<ws>` namespace under ADR043,
	// not the shared one — read it from the App's namespace.
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: tlsSecretForHost(app, host)}, &sec)
	return err == nil && len(sec.Data["tls.crt"]) > 0
}

// domainView builds a DomainView for one host on the given App. platformHost is
// passed in (host-independent, so callers compute it once per app rather than per
// host). The TLS Secret lookup makes the verification status truthful at query time.
func (s *Service) domainView(ctx context.Context, app *appv1alpha1.App, host, platformHost string) DomainView {
	verified := s.domainVerified(ctx, app, host)
	vStatus := "pending"
	if verified {
		vStatus = "verified"
	}
	// "active" once the cert is issued and the App is not suspended; "pending" otherwise.
	sStatus := "pending"
	if verified && !app.Spec.Suspended {
		sStatus = "active"
	}
	dtype := domainType(host)
	return DomainView{
		Name:               host,
		DomainType:         dtype,
		VerificationStatus: vStatus,
		ServerStatus:       sStatus,
		RedirectForName:    app.Spec.HostRedirects[host],
		DNSRecord:          dnsRecordFor(host, dtype, platformHost),
	}
}

// ListDomains returns the custom domains from App.spec.hosts[].
func (s *Service) ListDomains(ctx context.Context, appName string) ([]DomainView, error) {
	app, err := s.AuthorizeApp(ctx, core.RelCanView, appName)
	if err != nil {
		return nil, err
	}
	platformHost := s.platformHost(app) // host-independent — compute once for all hosts
	out := make([]DomainView, 0, len(app.Spec.Hosts))
	for _, h := range app.Spec.Hosts {
		if h != "" {
			out = append(out, s.domainView(ctx, app, h, platformHost))
		}
	}
	return out, nil
}

// filterDomains applies Render's two custom-domain list filters. Both are
// optional ("" ⇒ unfiltered) and an unrecognized value is a named
// core.ErrBadRequest. REST, GraphQL, and MCP all route through here so the
// accepted vocabulary and its 400 cannot drift between the three surfaces.
func filterDomains(domains []DomainView, verificationStatus, domainType string) ([]DomainView, error) {
	status, err := core.ParseEnum("verificationStatus", verificationStatus, "pending", "verified")
	if err != nil {
		return nil, err
	}
	dtype, err := core.ParseEnum("domainType", domainType, "apex", "subdomain")
	if err != nil {
		return nil, err
	}
	return core.Filter(domains, func(d DomainView) bool {
		return (status == "" || d.VerificationStatus == status) &&
			(dtype == "" || d.DomainType == dtype)
	}), nil
}

// GetDomain returns one custom domain by hostname, or core.ErrNotFound.
func (s *Service) GetDomain(ctx context.Context, appName, hostname string) (DomainView, error) {
	app, err := s.AuthorizeApp(ctx, core.RelCanView, appName)
	if err != nil {
		return DomainView{}, err
	}
	platformHost := s.platformHost(app)
	for _, h := range app.Spec.Hosts {
		if h == hostname {
			return s.domainView(ctx, app, h, platformHost), nil
		}
	}
	return DomainView{}, core.ErrNotFound
}

// VerifyDomain re-checks a custom domain's DNS/cert state now and returns its
// fresh view — bex's analogue of Render's POST …/custom-domains/{id}/verify. bex
// verification is automatic (cert-manager continuously reconciles the per-host TLS
// secret, docs/ADR005-custom-domain.md), so there is no verification job to trigger and
// "verify" is a read at read altitude: it re-evaluates the TLS-secret/serving state
// and reports the current status, giving the dashboard a "Verify / re-check" action
// that refreshes a pending row without a mutation. Delegating to GetDomain keeps it
// identical to a fresh read (same RelCanView authorization, same lookup). Idempotent;
// unknown host → core.ErrNotFound.
func (s *Service) VerifyDomain(ctx context.Context, appName, hostname string) (DomainView, error) {
	return s.GetDomain(ctx, appName, hostname)
}

// ownPlatformHost is the App's own `<slug>.<base>` auto host — the single
// `*.<base>` name exempt from reservedHost. It derives from the App's
// globally-unique platform subdomain (spec.subdomain, w4/m19), NEVER the
// workspace-local CR name: App names collide across tenants (ADR043 per-tenant
// namespaces), so exempting a caller-supplied `<appName>.<base>` let a tenant
// name its service after a victim's slug and claim the victim's platform host
// (codex F5). Empty when BaseDomain is unset (nothing to exempt under it).
func (s *Service) ownPlatformHost(app *appv1alpha1.App) string {
	if s.BaseDomain == "" {
		return ""
	}
	return app.Spec.PlatformSubdomain(app.Name) + "." + s.BaseDomain
}

// reservedHost reports whether host is a platform-owned name no App may claim as
// a custom domain. Reserved: the BEX_BASE_DOMAIN apex, and any `<label…>.<base>`
// platform host other than this App's own `<slug>.<base>` auto host (the whole
// `*.<base>` namespace is platform-controlled — its DNS resolves to the shared
// ingress, so a foreign claim would pass ACME and hijack another App's platform
// subdomain), plus the dashboard host when configured. Render likewise reserves
// its own `*.onrender.com` subdomains; the dashboard/base-apex entries are the
// bex analog for the platform's own control-plane hosts. Inert when BaseDomain is
// unset (nothing to reserve under it). ownPlatformHost is the resolved App's own
// exempt `<slug>.<base>` host (from s.ownPlatformHost) — pass the immutable slug
// host, never the caller-supplied CR name (codex F5).
func (s *Service) reservedHost(ownPlatformHost, host string) bool {
	if s.BaseDomain != "" {
		if host == s.BaseDomain {
			return true // the base-domain apex itself
		}
		// a `<x>.<base>` platform host — reserved unless it's this App's own auto host
		if strings.HasSuffix(host, "."+s.BaseDomain) && host != ownPlatformHost {
			return true
		}
	}
	return s.DashboardHost != "" && host == s.DashboardHost
}

// errDomainInUse is the cross-App collision rejection — Render's "this domain
// already exists on another site" (core.ErrConflict => 409). Built in one place
// so the service-level guard and the store-race backstop word it identically.
func errDomainInUse() error {
	return fmt.Errorf("%w: this domain already exists on another site", core.ErrConflict)
}

// hostClaimedElsewhere reports whether host is already registered (as spec.host
// or in spec.hosts[]) — or is the www<->apex sibling (wwwSibling, t002) of a host
// already registered — on a *different* App in the namespace. The sibling check
// is what closes w7/m6's documented blind spot (w6/m23 t004): registering
// `www.foo.com` on app A now also reserves `foo.com` against app B, and vice
// versa, matching the cross-App, cross-tenant collision Render blocks with
// "this domain already exists on another site." wwwSibling is its own inverse
// for a valid pair, so a single `wwwSibling(h) == host` check (no need to also
// compute wwwSibling(host)) catches both add orders. host must already be
// canonical (canonicalHostname at the write boundaries); stored claims are
// normalized defensively (normalizeHostname) so a legacy verbatim-stored value
// with stray case/dot/whitespace still collides instead of slipping past. The
// owning App's name is deliberately not returned: a caller must not learn
// another tenant's service name from the rejection.
func (s *Service) hostClaimedElsewhere(ctx context.Context, owner *appv1alpha1.App, host string) (bool, error) {
	// A host is unique across the whole platform, and Apps are spread across
	// per-tenant namespaces (ADR043), so the collision sweep must be cluster-wide.
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list); err != nil {
		return false, err
	}
	return hostClaimedInApps(list.Items, owner, host), nil
}

// hostClaimedInApps is hostClaimedElsewhere's matching core over an
// already-fetched App list, so ensureHostsClaimable can check a whole host set
// against one cluster-wide sweep instead of Listing per host.
func hostClaimedInApps(items []appv1alpha1.App, owner *appv1alpha1.App, host string) bool {
	ownerID := appClaimIdentity(owner)
	for i := range items {
		a := &items[i]
		if ownerID != "" && appClaimIdentity(a) == ownerID {
			continue
		}
		claimed := append([]string{a.Spec.Host}, a.Spec.Hosts...)
		for _, h := range claimed {
			h = normalizeHostname(h)
			if h != "" && (h == host || wwwSibling(h) == host) {
				return true
			}
		}
	}
	return false
}

// appClaimIdentity returns an immutable identity whenever one exists. Public
// srv- ids are stable across projection/recreation; UID is the fallback for
// hand-applied CRs. Namespace/name is used only before Kubernetes assigns a UID
// and remains collision-free for canonical per-workspace object names.
func appClaimIdentity(app *appv1alpha1.App) string {
	if app == nil {
		return ""
	}
	if id := app.Labels[core.LabelAppID]; id != "" {
		return "id:" + id
	}
	if app.UID != "" {
		return "uid:" + string(app.UID)
	}
	if app.Namespace != "" && app.Name != "" {
		return "name:" + app.Namespace + "/" + app.Name
	}
	return ""
}

// ensureHostsClaimable runs the same platform-reserved + cross-App collision
// gate AddDomain enforces (reservedHost + hostClaimedElsewhere) over a NEW App's
// create-time host set, so the create/blueprint/deploy-manifest paths — which all
// funnel through writeNewApp — cannot bind spec.host/spec.hosts to a reserved
// platform name (api/dashboard/`*.<base>`) or one another tenant already owns and
// have the operator mint an Ingress that hijacks it (w7/m57). Before this, the
// guard lived ONLY in AddDomain, so a create could claim any host unchecked.
// app is the new App; hostClaimedElsewhere skips app.Name, so a blueprint
// re-apply that re-states the App's own hosts is not self-rejected, and
// reservedHost exempts only the App's own immutable `<slug>.<base>` platform
// host (via s.ownPlatformHost) — not `<app.Name>.<base>`, which a tenant could
// otherwise set to a victim's slug and hijack at create time too (codex F5).
func (s *Service) ensureHostsClaimable(ctx context.Context, app *appv1alpha1.App) error {
	ownHost := s.ownPlatformHost(app)
	// One cluster-wide sweep serves every host in the set, fetched on first
	// need so a hostless create still Lists nothing and a reserved first host
	// still fails before any List.
	var apps []appv1alpha1.App
	fetched := false
	for _, h := range append([]string{app.Spec.Host}, app.Spec.Hosts...) {
		if h == "" {
			continue
		}
		if s.reservedHost(ownHost, h) {
			return fmt.Errorf("%w: %q is a reserved platform hostname", core.ErrBadRequest, h)
		}
		if !fetched {
			var list appv1alpha1.AppList
			if err := s.Client.List(ctx, &list); err != nil {
				return err
			}
			apps = list.Items
			fetched = true
		}
		if hostClaimedInApps(apps, app, h) {
			return errDomainInUse()
		}
		if err := s.requireDomainOwnership(ctx, app, h); err != nil {
			return err
		}
	}
	return nil
}

// AddDomain appends hostname (canonicalized by canonicalHostname — trimmed,
// terminal dot dropped, lowercased, DNS-1123 validated — so casing or a
// trailing dot can't split one logical host into two spec.hosts[] entries) to
// App.spec.hosts[] if not already
// present, then auto-pairs its www<->apex sibling (wwwSibling, t002) the way
// Render's capture documents (docs/render-artifacts/custom-domain-pairing.md,
// w6/m23 t001): adding `foo.com` also adds `www.foo.com`, and vice versa — one
// hop only, so the sibling add never re-pairs. Pairing is attempted only when
// the primary host was newly added (addOne's `added` result), so re-adding a
// directly served host never resurrects a sibling the tenant deliberately
// deleted. Re-adding an auto-created sibling explicitly clears its redirect and
// makes both halves directly served without creating a duplicate. Rejects a
// hostname that is platform-reserved
// (core.ErrBadRequest) or already registered — directly or via its sibling —
// on another App (core.ErrConflict, Render's "already in use"). The sibling add
// is best-effort: a reserved or (defensively, given the symmetric guard in
// hostClaimedElsewhere) elsewhere-claimed sibling is skipped silently rather
// than failing the caller's own successful add.
func (s *Service) AddDomain(ctx context.Context, appName, hostname string) (DomainView, error) {
	// The raw input goes straight to addOne, which authorizes FIRST (the
	// authz-before-validation verb contract) and then canonicalizes; the
	// canonical value comes back as view.Name for the sibling pairing below.
	view, added, err := s.addOne(ctx, appName, hostname, "")
	if err != nil || !added {
		return view, err
	}
	if sib := wwwSibling(view.Name); sib != "" {
		_, _, _ = s.addOne(ctx, appName, sib, view.Name)
	}
	return view, nil
}

// addOne adds a single host, with no sibling pairing of its own — AddDomain
// calls it once for the primary host and, on a fresh add, once more for the
// sibling. added reports whether this call actually wrote a new host (false
// for the idempotent already-present path), so AddDomain knows whether pairing
// applies. Authorization runs BEFORE hostname canonicalization/validation — a
// denied caller must get ErrForbidden, never input-validation feedback. For
// store-managed Apps the row is written first (same rationale as Suspend).
func (s *Service) addOne(ctx context.Context, appName, hostname, redirectForName string) (view DomainView, added bool, err error) {
	app, err := s.AuthorizeApp(ctx, core.RelCanOperate, appName)
	if err != nil {
		return DomainView{}, false, err
	}
	hostname, err = canonicalHostname(hostname)
	if err != nil {
		return DomainView{}, false, err
	}
	redirectForName = normalizeHostname(redirectForName)
	present := slices.Contains(app.Spec.Hosts, hostname)
	// Already served with the redirect the caller asked for: nothing to write.
	if present && app.Spec.HostRedirects[hostname] == redirectForName {
		return s.domainView(ctx, app, hostname, s.platformHost(app)), false, nil
	}
	// Claim checks apply only to a host this App doesn't already serve; a host
	// it does serve is only having its redirect rewritten.
	if !present {
		if s.reservedHost(s.ownPlatformHost(app), hostname) {
			return DomainView{}, false, fmt.Errorf("%w: %q is a reserved platform hostname", core.ErrBadRequest, hostname)
		}
		if claimed, err := s.hostClaimedElsewhere(ctx, app, hostname); err != nil {
			return DomainView{}, false, err
		} else if claimed {
			return DomainView{}, false, errDomainInUse()
		}
		if err := s.requireDomainOwnership(ctx, app, hostname); err != nil {
			return DomainView{}, false, err
		}
	}
	if s.Store != nil {
		if id := managedAppID(app); id != "" {
			if err := s.Store.AddDomain(ctx, id, hostname, redirectForName); err != nil {
				if !present && errors.Is(err, store.ErrConflict) { // lost a race to another App's add
					return DomainView{}, false, errDomainInUse()
				}
				return DomainView{}, false, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	base := client.MergeFrom(app.DeepCopy())
	if !present {
		app.Spec.Hosts = append(app.Spec.Hosts, hostname)
	}
	setHostRedirect(app, hostname, redirectForName)
	resourcemeta.Touch(app, s.Now())
	if err := s.Client.Patch(ctx, app, base); err != nil {
		return DomainView{}, false, err
	}
	return s.domainView(ctx, app, hostname, s.platformHost(app)), !present, nil
}

// setHostRedirect updates one source->canonical mapping and keeps the empty
// representation nil. Returning to nil avoids leaving an empty object in the
// CR after an auto-paired sibling is explicitly claimed or deleted.
func setHostRedirect(app *appv1alpha1.App, host, target string) {
	if target == "" {
		delete(app.Spec.HostRedirects, host)
		if len(app.Spec.HostRedirects) == 0 {
			app.Spec.HostRedirects = nil
		}
		return
	}
	if app.Spec.HostRedirects == nil {
		app.Spec.HostRedirects = map[string]string{}
	}
	app.Spec.HostRedirects[host] = target
}

// DeleteDomain removes hostname from App.spec.hosts[]. Idempotent — removing a
// hostname not in spec.hosts[] is a no-op. For store-managed Apps the row is
// deleted first (same row-first rationale as the other intent verbs).
//
// Deleting one half of an auto-paired www<->apex sibling leaves the other half
// in spec.hosts[]. If the deleted host was the canonical redirect target, the
// surviving sibling's redirect is cleared so it serves directly rather than
// dangling. Render does not document pair-delete semantics; this is bex's
// explicit per-host rule.
func (s *Service) DeleteDomain(ctx context.Context, appName, hostname string) error {
	app, err := s.AuthorizeApp(ctx, core.RelCanOperate, appName)
	if err != nil {
		return err
	}
	var updated []string
	for _, h := range app.Spec.Hosts {
		if h != hostname {
			updated = append(updated, h)
		}
	}
	if len(updated) == len(app.Spec.Hosts) {
		return nil // not present — idempotent
	}
	if s.Store != nil {
		if id := managedAppID(app); id != "" {
			for source, target := range app.Spec.HostRedirects {
				if target == hostname {
					if err := s.Store.AddDomain(ctx, id, source, ""); err != nil {
						return fmt.Errorf("clear dependent redirect in source of truth: %w", err)
					}
				}
			}
			if err := s.Store.RemoveDomain(ctx, id, hostname); err != nil {
				return fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	base := client.MergeFrom(app.DeepCopy())
	app.Spec.Hosts = updated
	setHostRedirect(app, hostname, "")
	for source, target := range app.Spec.HostRedirects {
		if target == hostname {
			setHostRedirect(app, source, "")
		}
	}
	resourcemeta.Touch(app, s.Now())
	return s.Client.Patch(ctx, app, base)
}

// --- Render wire types ---

// renderCustomDomain mirrors components.schemas.customDomain from Render's
// public OpenAPI spec (fields bex has real equivalents for). redirectForName is
// Render's spelling for an auto-paired sibling's canonical target; omitempty
// keeps directly served hosts byte-compatible. publicSuffix remains omitted:
// bex computes it internally but has no tenant-facing need to duplicate it.
type renderCustomDomain struct {
	ID                 string `json:"id"`   // hostname used as opaque id (Render ids are opaque)
	Name               string `json:"name"` // the FQDN
	DomainType         string `json:"domainType"`
	VerificationStatus string `json:"verificationStatus"`
	ServerStatus       string `json:"serverStatus"`
	RedirectForName    string `json:"redirectForName,omitempty"`
	// DNSRecord is a bex extension (no Render REST equivalent — Render surfaces the
	// record in the dashboard, not the API): the record the tenant must create to
	// point this domain at the service. A safe superset, w5/m10.
	DNSRecord renderDNSRecord `json:"dnsRecord"`
}

// renderDNSRecord is the wire shape of a DNSRecordView (the DNS record to create).
type renderDNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// customDomainWithCursor is the list-item envelope (matches Render's list shape).
type customDomainWithCursor struct {
	CustomDomain renderCustomDomain `json:"customDomain"`
	Cursor       string             `json:"cursor"`
}

func toRenderCustomDomain(d DomainView) renderCustomDomain {
	return renderCustomDomain{
		ID:                 d.Name,
		Name:               d.Name,
		DomainType:         d.DomainType,
		VerificationStatus: d.VerificationStatus,
		ServerStatus:       d.ServerStatus,
		RedirectForName:    d.RedirectForName,
		DNSRecord: renderDNSRecord{
			Type:  d.DNSRecord.Type,
			Name:  d.DNSRecord.Name,
			Value: d.DNSRecord.Value,
		},
	}
}

func toCustomDomainList(domains []DomainView) []customDomainWithCursor {
	out := make([]customDomainWithCursor, 0, len(domains))
	for _, d := range domains {
		out = append(out, customDomainWithCursor{CustomDomain: toRenderCustomDomain(d), Cursor: d.Name})
	}
	return out
}
