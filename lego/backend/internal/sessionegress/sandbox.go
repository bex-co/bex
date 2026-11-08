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

package sessionegress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Render's sandbox `allow-list` network policy (w32/m283, ADR042 § Sandbox
// network policies). allow-all and deny-all are static platform policies
// selected by the EgressLabel a sandbox Pod carries; only allow-list needs a
// per-sandbox object, because its destinations are the tenant's. That object
// is the narrowest shape bex-api writes: filtered DNS for, and TLS 443 with a
// matching SNI to, exactly the listed hostnames — never a CIDR or entity rule
// (the bex-api-sandbox-egress admission policy enforces the same grammar).
const (
	// SandboxIDLabel is the sandbox's public id (`sbx-…`), persisted by
	// OpenSandbox as a Pod label; the allow-list policy selects on it.
	SandboxIDLabel = "bex.co/sandbox-id"
	// EgressLabel names the enforced posture of a Render sandbox: allow-all,
	// deny-all or allow-list. A sandbox Pod without it predates w32/m283 and
	// keeps the legacy platform allowlist.
	EgressLabel = "bex.co/egress"

	// MaxSandboxRules is Render's `rules` maxItems.
	MaxSandboxRules = 100

	// CodeSandboxPolicyInvalid is the named refusal for any invalid sandbox
	// network policy input.
	CodeSandboxPolicyInvalid = "SANDBOX_NETWORK_POLICY_INVALID"

	sandboxManagedByValue = "bex-sandbox-egress"
)

// sandboxDomain is one lowercase DNS hostname, optionally with a single
// leading `*.` wildcard label: Render's "a wildcard is only allowed as the
// leftmost label".
var sandboxDomain = regexp.MustCompile(`^(\*\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$`)

// SandboxAllowList validates and canonicalizes a Render allow-list's domains.
// Matching is exact, as on Render: `example.com` does not cover
// `api.example.com`; `*.example.com` covers one subdomain level and not the
// apex (the TLS-certificate wildcard reading of Render's leftmost-label rule).
// Schemes, ports, paths, uppercase, IP literals, cluster-local names,
// duplicates and more than MaxSandboxRules entries each get a named 400.
func SandboxAllowList(domains []string) ([]string, error) {
	if len(domains) == 0 {
		return nil, invalidSandboxRule("rules_required", "", "an allow-list policy needs at least one rule")
	}
	if len(domains) > MaxSandboxRules {
		return nil, invalidSandboxRule("too_many_rules", "", fmt.Sprintf("at most %d rules are allowed", MaxSandboxRules))
	}
	out := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		bare := strings.TrimPrefix(domain, "*.")
		switch {
		case domain == "":
			return nil, invalidSandboxRule("empty", domain, "rule domain must not be empty")
		case len(domain) > 253:
			return nil, invalidSandboxRule("too_long", domain, "rule domain must be at most 253 characters")
		case strings.ContainsAny(domain, ":/?#@ "):
			return nil, invalidSandboxRule("not_a_hostname", domain, "use a hostname without a scheme, port, or path")
		case domain != strings.ToLower(domain):
			return nil, invalidSandboxRule("not_lowercase", domain, "rule domain must be lowercase")
		case net.ParseIP(bare) != nil:
			return nil, invalidSandboxRule("ip_literal", domain, "IP addresses are not supported; name a hostname")
		case !sandboxDomain.MatchString(domain) || len(validation.IsDNS1123Subdomain(bare)) != 0:
			return nil, invalidSandboxRule("not_a_hostname", domain, "rule domain must be a multi-label hostname; a wildcard is only allowed as the leftmost label (*.example.com)")
		case PrivateOrClusterHost(bare):
			return nil, invalidSandboxRule("private_name", domain, "private and cluster-local names cannot be allowed")
		}
		if _, dup := seen[domain]; dup {
			return nil, invalidSandboxRule("duplicate", domain, "each domain may be listed once")
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	slices.Sort(out)
	return out, nil
}

func invalidSandboxRule(reason, domain, message string) error {
	params := map[string]any{"field": "networkPolicy.rules", "reason": reason}
	if domain != "" {
		params["domain"] = domain
	}
	return core.NewBadRequestError(CodeSandboxPolicyInvalid, message, params)
}

// SandboxManager converges the per-sandbox allow-list policy. A nil Client is
// not a permissive mode: every call fails closed.
type SandboxManager struct {
	Client client.Client
}

// Apply installs the allow-list policy for one sandbox. It runs before the
// sandbox is created, so the Pod is never selected by a looser rule; until the
// object exists the structural sandbox default-deny drops everything.
func (m *SandboxManager) Apply(ctx context.Context, namespace, sandboxID string, domains []string) error {
	if m == nil || m.Client == nil {
		return errNoPolicyClient
	}
	desired, err := RenderSandboxPolicy(namespace, sandboxID, domains)
	if err != nil {
		return err
	}
	// The sandbox id is freshly minted, so a create is the expected path.
	if err := m.Client.Create(ctx, desired); !apierrors.IsAlreadyExists(err) {
		return err
	}
	current, err := m.get(ctx, namespace, sandboxID)
	if err != nil {
		return err
	}
	if current == nil {
		return errors.New("sandbox egress: policy vanished during update; retry")
	}
	desired.SetResourceVersion(current.GetResourceVersion())
	return m.Client.Update(ctx, desired)
}

// Rules reads back the allow-list a sandbox was created with from its
// policy's FQDN selectors, so reads report Render's `networkPolicy.rules`. A
// missing policy reads as no rules; the sandbox then has no egress at all (the
// default-deny still selects it).
func (m *SandboxManager) Rules(ctx context.Context, namespace, sandboxID string) ([]string, error) {
	obj, err := m.get(ctx, namespace, sandboxID)
	if err != nil || obj == nil {
		return nil, err
	}
	egress, _, _ := unstructured.NestedSlice(obj.Object, "spec", "egress")
	var domains []string
	for _, rule := range egress {
		fqdns, _, _ := unstructured.NestedSlice(rule.(map[string]any), "toFQDNs")
		for _, fqdn := range fqdns {
			selector, _ := fqdn.(map[string]any)
			for _, key := range []string{"matchName", "matchPattern"} {
				if name, ok := selector[key].(string); ok {
					domains = append(domains, name)
				}
			}
		}
	}
	slices.Sort(domains)
	return domains, nil
}

// Delete removes the sandbox's allow-list policy, if any. Gone is the goal, so
// a concurrent delete is not an error.
func (m *SandboxManager) Delete(ctx context.Context, namespace, sandboxID string) error {
	obj, err := m.get(ctx, namespace, sandboxID)
	if err != nil || obj == nil {
		return err
	}
	if err := m.Client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete sandbox egress policy: %w", err)
	}
	return nil
}

var errNoPolicyClient = errors.New("sandbox egress: Kubernetes policy client is not configured")

// get returns the sandbox's own policy, nil when there is none, and an error
// when the object at its name belongs to something else (never adopted).
func (m *SandboxManager) get(ctx context.Context, namespace, sandboxID string) (*unstructured.Unstructured, error) {
	if m == nil || m.Client == nil {
		return nil, errNoPolicyClient
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	obj.SetNamespace(namespace)
	obj.SetName(sandboxPolicyName(sandboxID))
	if err := m.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); apierrors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("get sandbox egress policy: %w", err)
	}
	labels := obj.GetLabels()
	if labels[managedByLabel] != sandboxManagedByValue || labels[SandboxIDLabel] != sandboxID {
		return nil, errors.New("sandbox egress: existing policy is not owned by this sandbox")
	}
	return obj, nil
}

// RenderSandboxPolicy returns the exact allow-list CiliumNetworkPolicy for one
// sandbox, without a Kubernetes client (the isolation matrix renders its
// fixture with it, as it does the session policy).
func RenderSandboxPolicy(namespace, sandboxID string, domains []string) (*unstructured.Unstructured, error) {
	if len(validation.IsValidLabelValue(sandboxID)) != 0 || sandboxID == "" {
		return nil, fmt.Errorf("sandbox egress: invalid sandbox id %q", sandboxID)
	}
	domains, err := SandboxAllowList(domains)
	if err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      sandboxPolicyName(sandboxID),
			"namespace": namespace,
			"labels": map[string]any{
				managedByLabel: sandboxManagedByValue,
				SandboxIDLabel: sandboxID,
			},
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{SandboxIDLabel: sandboxID}},
			"egress":           []any{dnsRule(domainSelectors(domains)), fqdnRule(domainSelectors(domains), domains)},
		},
	}}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	return obj, nil
}

// domainSelectors renders a Render domain as Cilium's selector: an exact name
// is a matchName, and `*.example.com` is a matchPattern, which in Cilium
// matches exactly one label at that level and not the apex. Cilium's TLS
// serverNames accepts the same leading `*.` form, so fqdnRule's SNI list is
// the domains themselves.
func domainSelectors(domains []string) []any {
	out := make([]any, 0, len(domains))
	for _, domain := range domains {
		if strings.HasPrefix(domain, "*.") {
			out = append(out, map[string]any{"matchPattern": domain})
		} else {
			out = append(out, map[string]any{"matchName": domain})
		}
	}
	return out
}

func sandboxPolicyName(sandboxID string) string {
	digest := sha256.Sum256([]byte(sandboxID))
	return "sandbox-egress-" + hex.EncodeToString(digest[:8])
}
