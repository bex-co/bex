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
	"sort"
	"strings"
	"unicode"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const traefikIngressClass = "traefik"

// traefikTLSEntrypoint is the HTTPS entrypoint name in the deployed Traefik
// (deploy/gitops/base/values/traefik.values.yaml `ports.websecure`) — the
// prefix Traefik v3 puts on the TLS sibling router it mints for every
// TLS-covered Ingress host (w1/m51/t001 ground truth, Traefik v3.7.5).
const traefikTLSEntrypoint = "websecure"

// TraefikRouterNames resolves the exact metric-label identities Traefik 3.7.5
// emits for an App's operator-owned Ingress. A private App with no Ingress has
// no public HTTP egress and returns an empty slice. An App that declares public
// exposure but whose Ingress is missing is unresolved, not a successful zero.
func (b *Base) TraefikRouterNames(ctx context.Context, app *appv1alpha1.App) ([]string, error) {
	if b.Client == nil {
		return nil, fmt.Errorf("resolve Traefik routers: Kubernetes client is unavailable")
	}
	namespace := app.Namespace
	if namespace == "" {
		namespace = b.Namespace
	}
	var ingress networkingv1.Ingress
	err := b.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: app.Name}, &ingress)
	if apierrors.IsNotFound(err) {
		if app.Spec.Host != "" || app.Spec.Expose || len(app.Spec.Hosts) > 0 {
			return nil, fmt.Errorf("resolve Traefik routers: expected Ingress %s/%s is missing", namespace, app.Name)
		}
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve Traefik routers: get Ingress %s/%s: %w", namespace, app.Name, err)
	}
	names, err := TraefikRouterNamesForIngress(&ingress)
	if err != nil {
		return nil, err
	}

	// Traefik concatenates namespace, Ingress name, host, and path before it
	// normalizes punctuation. Different operator-owned Ingresses can therefore
	// collapse to the same metric label across hyphen boundaries (for example,
	// App "web" + host "api-x" versus App "web-api" + host "x"). Traefik's
	// per-Ingress collision hash does not cover that cross-Ingress case. Reject
	// the ambiguity instead of charging both Apps for one router counter.
	var ingresses networkingv1.IngressList
	if err := b.Client.List(ctx, &ingresses, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("resolve Traefik routers: list Ingresses in %s: %w", namespace, err)
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	for i := range ingresses.Items {
		other := &ingresses.Items[i]
		if other.Name == ingress.Name {
			continue
		}
		otherNames, err := TraefikRouterNamesForIngress(other)
		if err != nil {
			continue // not an operator-owned App Ingress shape
		}
		for _, name := range otherNames {
			if _, collision := wanted[name]; collision {
				return nil, fmt.Errorf("resolve Traefik routers: metric label %q is ambiguous between Ingresses %s/%s and %s/%s",
					name, namespace, ingress.Name, namespace, other.Name)
			}
		}
	}
	return names, nil
}

// TraefikRouterNamesForIngress reproduces the Kubernetes Ingress provider's
// router-key algorithm in the Traefik version pinned by production (v3.7.5).
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
	// provider (v3.7.5 on prod, chart 41.0.0) creates the base router
	// `<ns>-<name>-<host>-<path>@kubernetes` on the non-TLS entrypoints and,
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
		variants := []string{key + "@kubernetes"}
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

// traefikNormalize is provider.Normalize from Traefik v3.7.5: runs of every
// non-letter/non-number character collapse to one hyphen.
func traefikNormalize(name string) string {
	return strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), "-")
}
