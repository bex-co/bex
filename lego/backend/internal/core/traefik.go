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

package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const traefikIngressClass = "traefik"

// traefikTLSEntrypoint is the HTTPS entrypoint name in the deployed Traefik
// (deploy/gitops/base/values/traefik.values.yaml `ports.websecure`) — the
// prefix Traefik v3 puts on the TLS sibling router it mints for every
// TLS-covered Ingress host (w1/m51/t001 ground truth, Traefik v3.7.5; unchanged through v3.7.17, w8/m145).
const traefikTLSEntrypoint = "websecure"

// traefikWebEntrypoint is the plain-HTTP entrypoint (`ports.web`). Since both
// entrypoints carry `http.middlewares` (w8/m123, 2026-10-20), Traefik builds an
// entrypoint model for each and splits every multi-entrypoint router into
// `web-<key>` and `websecure-<key>` (pkg/server/aggregator.go applyModel,
// v3.7.17); the bare `<key>` router stopped existing, so bytes on :80 (tenant
// hosts' HTTPS redirects, w8/m118, and any non-redirected route) went
// uncounted (block-eden-mono w1/069). The bare name stays in the set so
// windows reaching back before that change still sum its series.
const traefikWebEntrypoint = "web"

// TraefikRouterNames resolves the exact metric-label identities Traefik 3.7.5
// emits for an App's operator-owned Ingresses: its serving Ingress, and the
// children it controls beside it — the www/apex redirect hosts' and its
// wildcard custom domains' (v1alpha1.ChildIngressNames, w4/m326 t012). A
// wildcard-only App has no serving Ingress at all. An App without public hosts
// (AppSpec.HasPublicHosts) has no Ingress and no public HTTP egress, and
// returns an empty slice. That includes an exposed service whose subdomain is
// disabled and whose last custom domain was deleted (w5/112). An App with
// public hosts whose Ingress is missing is unresolved only while Running —
// Failed/Building/etc. have no live public surface to meter (prod: Failed
// build-never-Ingress Apps were flooding usage catch-up logs as unavailable).
func (b *Base) TraefikRouterNames(ctx context.Context, app *appv1alpha1.App) ([]string, error) {
	if b.Client == nil {
		return nil, fmt.Errorf("resolve Traefik routers: Kubernetes client is unavailable")
	}
	namespace := app.Namespace
	if namespace == "" {
		namespace = b.Namespace
	}
	// One List serves both the App's own Ingresses and the collision check.
	var ingresses networkingv1.IngressList
	if err := b.Client.List(ctx, &ingresses, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("resolve Traefik routers: list Ingresses in %s: %w", namespace, err)
	}
	children := map[string]bool{}
	for _, name := range appv1alpha1.ChildIngressNames(app.Name) {
		children[name] = true
	}
	owned := map[string]bool{}
	var names []string
	for i := range ingresses.Items {
		ingress := &ingresses.Items[i]
		if !appIngress(ingress, app, children) {
			continue
		}
		routers, err := TraefikRouterNamesForIngress(ingress)
		if err != nil {
			return nil, err
		}
		owned[ingress.Name] = true
		names = append(names, routers...)
	}
	// Two of the App's own Ingresses can normalize to one router key (a
	// "wildcard.<base>" host beside "*.<base>"); count that router once.
	names = slices.Compact(slices.Sorted(slices.Values(names)))
	if len(owned) == 0 {
		if app.Spec.HasPublicHosts() && app.Status.Phase == appv1alpha1.PhaseRunning {
			return nil, fmt.Errorf("resolve Traefik routers: expected Ingress %s/%s is missing", namespace, app.Name)
		}
		return []string{}, nil
	}

	// Traefik concatenates namespace, Ingress name, host, and path before it
	// normalizes punctuation. Different operator-owned Ingresses can therefore
	// collapse to the same metric label across hyphen boundaries (for example,
	// App "web" + host "api-x" versus App "web-api" + host "x"). Traefik's
	// per-Ingress collision hash does not cover that cross-Ingress case. Reject
	// the ambiguity instead of charging both Apps for one router counter.
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	for i := range ingresses.Items {
		other := &ingresses.Items[i]
		if owned[other.Name] {
			continue
		}
		otherNames, err := TraefikRouterNamesForIngress(other)
		if err != nil {
			continue // not an operator-owned App Ingress shape
		}
		for _, name := range otherNames {
			if _, collision := wanted[name]; collision {
				return nil, fmt.Errorf("resolve Traefik routers: metric label %q of App %s/%s is ambiguous with Ingress %s/%s",
					name, namespace, app.Name, namespace, other.Name)
			}
		}
	}
	return names, nil
}

// TraefikRouterNamesForIngress reproduces the Kubernetes Ingress provider's
// router-key algorithm in the Traefik version pinned by production (v3.7.17;
// identical to v3.7.5, which it was written against — w8/m145).
// It deliberately accepts only the shape reconcileIngress owns: one Prefix
// path at "/" per host. If that contract drifts, metering fails closed rather
// than guessing at an identity and charging another App's bytes.
func TraefikRouterNamesForIngress(ingress *networkingv1.Ingress) ([]string, error) {
	if ingress == nil {
		return nil, fmt.Errorf("resolve Traefik routers: nil Ingress")
	}
	if ingress.Spec.IngressClassName == nil || *ingress.Spec.IngressClassName != traefikIngressClass {
		return nil, fmt.Errorf("resolve Traefik routers: Ingress %s/%s is not class %q", ingress.Namespace, ingress.Name, traefikIngressClass)
	}

	// TLS-covered hosts get a second router: Traefik v3's Kubernetes Ingress
	// provider (v3.7.17 on prod, chart 41.0.0) creates the base router
	// `<ns>-<name>-<host>-<path>@kubernetes` on the non-TLS entrypoints (now
	// `web-<same>`, see traefikWebEntrypoint) and,
	// for every host in spec.tls, a TLS sibling named `websecure-<same>` on
	// the HTTPS entrypoint — and that sibling is where essentially all real
	// bytes flow (w1/m51; prod router-label inventory 2026-07-18 shows exactly
	// these two shapes, no others). Operator-owned Ingresses always carry TLS,
	// so omitting the sibling undercounted every bandwidth/egress consumer to
	// the 80→443 redirect traffic (w1/035).
	tlsHosts := map[string]bool{}
	for i := range ingress.Spec.TLS {
		for _, host := range ingress.Spec.TLS[i].Hosts {
			tlsHosts[host] = true
		}
	}

	type candidate struct {
		key  string
		rule string
		tls  bool
	}
	candidates := make([]candidate, 0, len(ingress.Spec.Rules))
	counts := map[string]int{}
	for i := range ingress.Spec.Rules {
		ingressRule := &ingress.Spec.Rules[i]
		if ingressRule.Host == "" || ingressRule.HTTP == nil || len(ingressRule.HTTP.Paths) != 1 {
			return nil, fmt.Errorf("resolve Traefik routers: Ingress %s/%s rule %d is not one host with one HTTP path", ingress.Namespace, ingress.Name, i)
		}
		path := &ingressRule.HTTP.Paths[0]
		if path.Path != "/" || path.PathType == nil || *path.PathType != networkingv1.PathTypePrefix || path.Backend.Service == nil {
			return nil, fmt.Errorf("resolve Traefik routers: Ingress %s/%s rule %d is not the operator-owned Prefix / service route", ingress.Namespace, ingress.Name, i)
		}

		key := strings.TrimPrefix(traefikNormalize(ingress.Namespace+"-"+ingress.Name+"-"+ingressRule.Host+path.Path), "-")
		rule := fmt.Sprintf(`Host(%q) && PathPrefix("/")`, ingressRule.Host)
		candidates = append(candidates, candidate{key: key, rule: rule, tls: tlsHosts[ingressRule.Host]})
		counts[key]++
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("resolve Traefik routers: Ingress %s/%s has no operator-owned routes", ingress.Namespace, ingress.Name)
	}

	seen := map[string]struct{}{}
	names := make([]string, 0, 2*len(candidates))
	for _, candidate := range candidates {
		key := candidate.key
		if counts[key] > 1 {
			hash := sha256.Sum256([]byte(candidate.rule))
			key = fmt.Sprintf("%s-%.10x", key, hash[:])
		}
		// The sibling derives from the FINAL key (post collision-hash): Traefik
		// prefixes the router it built, hash and all.
		variants := []string{key + "@kubernetes", traefikWebEntrypoint + "-" + key + "@kubernetes"}
		if candidate.tls {
			variants = append(variants, traefikTLSEntrypoint+"-"+key+"@kubernetes")
		}
		for _, name := range variants {
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// TraefikRouterMatcher returns an explicitly anchored Prometheus label regex
// that matches only names returned by TraefikRouterNames. Empty means there is
// no public router and callers should return a successful zero without querying.
func TraefikRouterMatcher(names []string) string {
	if len(names) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		quoted = append(quoted, regexp.QuoteMeta(name))
	}
	sort.Strings(quoted)
	if len(quoted) == 1 {
		return "^" + quoted[0] + "$"
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

// traefikNormalize is provider.Normalize from Traefik v3.7.5–v3.7.17: runs of every
// non-letter/non-number character collapse to one hyphen.
func traefikNormalize(name string) string {
	return strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), "-")
}

// appIngress reports whether ingress is one of app's own: the serving Ingress
// named after it, unless another App controls that name, or a child Ingress
// beside it (v1alpha1.ChildIngressNames) that app controls. A child name can
// also be another App's own name (an App "web-host-redirects" beside "web"),
// so a name alone never makes an Ingress an App's, and a router is never
// metered for two Apps.
func appIngress(ingress *networkingv1.Ingress, app *appv1alpha1.App, children map[string]bool) bool {
	switch {
	case ingress.Name == app.Name:
		controller := metav1.GetControllerOf(ingress)
		return controller == nil || controller.UID == app.UID
	case children[ingress.Name]:
		return metav1.IsControlledBy(ingress, app)
	}
	return false
}
