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
}

// domainType classifies a hostname as "apex" (root domain, two DNS labels) or
// "subdomain". This is a heuristic — dots == 1 means two labels, e.g. example.com.
func domainType(hostname string) string {
	if strings.Count(hostname, ".") == 1 {
		return "apex"
	}
	return "subdomain"
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

// domainView builds a DomainView for one host on the given App. The TLS Secret
// lookup makes the verification status truthful at query time.
func (s *Service) domainView(ctx context.Context, app *appv1alpha1.App, host string) DomainView {
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
	return DomainView{
		Name:               host,
		DomainType:         domainType(host),
		VerificationStatus: vStatus,
		ServerStatus:       sStatus,
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
	out := make([]DomainView, 0, len(app.Spec.Hosts))
	for _, h := range app.Spec.Hosts {
		if h != "" {
			out = append(out, s.domainView(ctx, app, h))
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
	for _, h := range app.Spec.Hosts {
		if h == hostname {
			return s.domainView(ctx, app, h), nil
		}
	}
	return DomainView{}, core.ErrNotFound
}

// AddDomain appends hostname to App.spec.hosts[] if not already present.
// Idempotent — returns the existing view if the hostname is already registered.
// For store-managed Apps the row is written first (same rationale as Suspend).
func (s *Service) AddDomain(ctx context.Context, appName, hostname string) (DomainView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
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
			return s.domainView(ctx, app, hostname), nil // already present
		}
	}
	if s.Store != nil {
		if id := app.Labels[store.LabelAppID]; id != "" {
			if err := s.Store.AddDomain(ctx, id, hostname); err != nil {
				return DomainView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	base := client.MergeFrom(app.DeepCopy())
	app.Spec.Hosts = append(app.Spec.Hosts, hostname)
	if err := s.Client.Patch(ctx, app, base); err != nil {
		return DomainView{}, err
	}
	return s.domainView(ctx, app, hostname), nil
}

// DeleteDomain removes hostname from App.spec.hosts[]. Idempotent — removing a
// hostname not in spec.hosts[] is a no-op. For store-managed Apps the row is
// deleted first (same row-first rationale as the other intent verbs).
func (s *Service) DeleteDomain(ctx context.Context, appName, hostname string) error {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
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
	}
}

func toCustomDomainList(domains []DomainView) []customDomainWithCursor {
	out := make([]customDomainWithCursor, 0, len(domains))
	for _, d := range domains {
		out = append(out, customDomainWithCursor{CustomDomain: toRenderCustomDomain(d), Cursor: d.Name})
	}
	return out
}
