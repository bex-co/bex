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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DomainView is the neutral bex projection of a custom domain on an App.
type DomainView struct {
	Name               string
	DomainType         string // "apex" or "subdomain" (Render's enum)
	VerificationStatus string // "pending" or "verified" (TLS cert issued?)
	ServerStatus       string // "active" or "pending"
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

// domainType classifies a hostname as "apex" (root domain, two DNS labels) or
// "subdomain". This is a heuristic — dots == 1 means two labels, e.g. example.com.
func domainType(hostname string) string {
	if strings.Count(hostname, ".") == 1 {
		return "apex"
	}
	return "subdomain"
}

// platformHost returns the App's platform hostname `<app>.<base-domain>` — the
// CNAME/ALIAS target a custom domain points at. Prefers BEX_BASE_DOMAIN (the same
// value the operator computes URLs from, so it's correct even before the operator
// has written status), falling back to the `<app>.<something>` entry in the App's
// status URLs (the Expose-generated host). Empty if neither is available.
func (s *Service) platformHost(app *appv1alpha1.App) string {
	if s.BaseDomain != "" {
		return app.Name + "." + s.BaseDomain
	}
	prefix := app.Name + "."
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
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: tlsSecretForHost(app, host)}, &sec)
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
		DNSRecord:          dnsRecordFor(host, dtype, platformHost),
	}
}

// ListDomains returns the custom domains from App.spec.hosts[].
func (s *Service) ListDomains(ctx context.Context, appName string) ([]DomainView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	app, err := s.GetApp(ctx, appName)
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

// GetDomain returns one custom domain by hostname, or core.ErrNotFound.
func (s *Service) GetDomain(ctx context.Context, appName, hostname string) (DomainView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return DomainView{}, err
	}
	app, err := s.GetApp(ctx, appName)
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

// reservedHost reports whether host is a platform-owned name no App may claim as
// a custom domain. Reserved: the BEX_BASE_DOMAIN apex, and any `<label…>.<base>`
// platform host other than this App's own `<app>.<base>` auto host (the whole
// `*.<base>` namespace is platform-controlled — its DNS resolves to the shared
// ingress, so a foreign claim would pass ACME and hijack another App's platform
// subdomain), plus the dashboard host when configured. Render likewise reserves
// its own `*.onrender.com` subdomains; the dashboard/base-apex entries are the
// bex analog for the platform's own control-plane hosts. Inert when BaseDomain is
// unset (nothing to reserve under it).
func (s *Service) reservedHost(appName, host string) bool {
	if s.BaseDomain != "" {
		if host == s.BaseDomain {
			return true // the base-domain apex itself
		}
		// a `<x>.<base>` platform host — reserved unless it's this App's own auto host
		if strings.HasSuffix(host, "."+s.BaseDomain) && host != appName+"."+s.BaseDomain {
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
// or in spec.hosts[]) on a *different* App in the namespace — the cross-App,
// cross-tenant collision Render blocks with "this domain already exists on
// another site." The owning App's name is deliberately not returned: a caller
// must not learn another tenant's service name from the rejection.
func (s *Service) hostClaimedElsewhere(ctx context.Context, appName, host string) (bool, error) {
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		return false, err
	}
	for i := range list.Items {
		a := &list.Items[i]
		if a.Name == appName {
			continue
		}
		if a.Spec.Host == host {
			return true, nil
		}
		for _, h := range a.Spec.Hosts {
			if h == host {
				return true, nil
			}
		}
	}
	return false, nil
}

// AddDomain appends hostname to App.spec.hosts[] if not already present.
// Idempotent — returns the existing view if the hostname is already registered
// on this App. Rejects a hostname that is platform-reserved (core.ErrBadRequest)
// or already registered on another App (core.ErrConflict, Render's "already in
// use"). For store-managed Apps the row is written first (same rationale as Suspend).
func (s *Service) AddDomain(ctx context.Context, appName, hostname string) (DomainView, error) {
	if err := s.AuthorizeTarget(ctx, core.RelCanOperate, core.ServiceTarget(appName)); err != nil {
		return DomainView{}, err
	}
	if hostname == "" {
		return DomainView{}, fmt.Errorf("%w: hostname is required", core.ErrBadRequest)
	}
	app, err := s.GetApp(ctx, appName)
	if err != nil {
		return DomainView{}, err
	}
	for _, h := range app.Spec.Hosts {
		if h == hostname {
			return s.domainView(ctx, app, hostname, s.platformHost(app)), nil // already present
		}
	}
	if s.reservedHost(appName, hostname) {
		return DomainView{}, fmt.Errorf("%w: %q is a reserved platform hostname", core.ErrBadRequest, hostname)
	}
	if claimed, err := s.hostClaimedElsewhere(ctx, appName, hostname); err != nil {
		return DomainView{}, err
	} else if claimed {
		return DomainView{}, errDomainInUse()
	}
	if s.Store != nil {
		if id := app.Labels[store.LabelAppID]; id != "" {
			if err := s.Store.AddDomain(ctx, id, hostname); err != nil {
				if errors.Is(err, store.ErrConflict) { // lost a race to another App's add
					return DomainView{}, errDomainInUse()
				}
				return DomainView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	base := client.MergeFrom(app.DeepCopy())
	app.Spec.Hosts = append(app.Spec.Hosts, hostname)
	if err := s.Client.Patch(ctx, app, base); err != nil {
		return DomainView{}, err
	}
	return s.domainView(ctx, app, hostname, s.platformHost(app)), nil
}

// DeleteDomain removes hostname from App.spec.hosts[]. Idempotent — removing a
// hostname not in spec.hosts[] is a no-op. For store-managed Apps the row is
// deleted first (same row-first rationale as the other intent verbs).
func (s *Service) DeleteDomain(ctx context.Context, appName, hostname string) error {
	if err := s.AuthorizeTarget(ctx, core.RelCanOperate, core.ServiceTarget(appName)); err != nil {
		return err
	}
	app, err := s.GetApp(ctx, appName)
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
		if id := app.Labels[store.LabelAppID]; id != "" {
			if err := s.Store.RemoveDomain(ctx, id, hostname); err != nil {
				return fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	base := client.MergeFrom(app.DeepCopy())
	app.Spec.Hosts = updated
	return s.Client.Patch(ctx, app, base)
}

// --- Render wire types ---

// renderCustomDomain mirrors components.schemas.customDomain from Render's
// public OpenAPI spec (fields bex has real equivalents for). publicSuffix and
// redirectForName require a public-suffix list and redirect mapping bex doesn't
// maintain — omitted rather than faked, as a safe superset.
type renderCustomDomain struct {
	ID                 string `json:"id"`   // hostname used as opaque id (Render ids are opaque)
	Name               string `json:"name"` // the FQDN
	DomainType         string `json:"domainType"`
	VerificationStatus string `json:"verificationStatus"`
	ServerStatus       string `json:"serverStatus"`
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
