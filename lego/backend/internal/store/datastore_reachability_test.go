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

package store

import (
	"net"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
)

// datastore_reachability_test.go guards leg 2 of the w7/m77 defect (ADR043 D8):
// a tenant namespace is default-deny, and its only in-cluster allow is
// same-namespace, so an App could not reach a managed Postgres/Key Value that
// lived anywhere else — every datastore ClusterIP is RFC1918, which
// allow-internet-egress explicitly excepts.
//
// Unlike legs 1 and 3 (internal/apps/datastore_namespace_test.go) this leg has
// no independent failing unit test: it is a consequence of placement, not a
// separate code path. What it needs instead is a guard that co-location stays
// SUFFICIENT — because the fix rests entirely on that. If a future change
// narrowed allow-same-namespace (say, to an app-label selector), links would
// break again with the same invisible timeout, and only this test would notice.

// egressAllowed evaluates the generated policy set the way Kubernetes does for
// the dimensions these policies actually use: a destination is permitted if ANY
// policy's egress rule matches it (NetworkPolicies are additive), where a rule
// matches on same-namespace pod peers, namespace-name peers, or an ipBlock with
// exceptions. Ports are checked only when a rule constrains them.
//
// It deliberately models the generated output rather than restating the
// intended behavior — a test that asserted "allow-same-namespace exists" would
// pass even if that policy stopped granting egress.
func egressAllowed(policies []*networkingv1.NetworkPolicy, dest destination) bool {
	for _, p := range policies {
		if !hasEgressType(p) {
			continue
		}
		for _, rule := range p.Spec.Egress {
			if !portMatches(rule, dest.port) {
				continue
			}
			for _, peer := range rule.To {
				if peerMatches(peer, dest) {
					return true
				}
			}
		}
	}
	return false
}

// destination describes where a tenant pod is trying to reach: the namespace
// the workload sits in, its ClusterIP, and the port.
type destination struct {
	namespace string
	ip        string
	port      int32
	// sameNamespaceAsSource is true when the destination pod shares the
	// source pod's namespace — what a bare PodSelector peer means.
	sameNamespaceAsSource bool
}

func hasEgressType(p *networkingv1.NetworkPolicy) bool {
	for _, t := range p.Spec.PolicyTypes {
		if t == networkingv1.PolicyTypeEgress {
			return true
		}
	}
	return false
}

func portMatches(rule networkingv1.NetworkPolicyEgressRule, port int32) bool {
	if len(rule.Ports) == 0 {
		return true // unconstrained
	}
	for _, p := range rule.Ports {
		if p.Port != nil && p.Port.IntVal == port {
			return true
		}
	}
	return false
}

func peerMatches(peer networkingv1.NetworkPolicyPeer, dest destination) bool {
	switch {
	case peer.IPBlock != nil:
		return ipBlockMatches(peer.IPBlock, dest.ip)
	case peer.NamespaceSelector != nil:
		// The generated policies select namespaces only by their name label.
		return peer.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] == dest.namespace
	case peer.PodSelector != nil:
		// A bare PodSelector peer means "pods in this policy's own namespace".
		return dest.sameNamespaceAsSource
	}
	return false
}

func ipBlockMatches(block *networkingv1.IPBlock, ip string) bool {
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	_, cidr, err := net.ParseCIDR(block.CIDR)
	if err != nil || !cidr.Contains(addr) {
		return false
	}
	for _, ex := range block.Except {
		if _, exCIDR, err := net.ParseCIDR(ex); err == nil && exCIDR.Contains(addr) {
			return false
		}
	}
	return true
}

// TestHostingAllowSetMakesCoLocatedDatastoresReachable is the load-bearing
// assertion behind ADR043 D8: co-locating a datastore with its App is what
// makes the link work, so the allow set must actually permit that traffic.
func TestHostingAllowSetMakesCoLocatedDatastoresReachable(t *testing.T) {
	const ws = "tea-reach"
	policies := allowNetworkPolicies(WorkspaceNamespace(ws), RegimeHosting)

	for _, tc := range []struct {
		what string
		port int32
	}{
		{"Postgres", 5432},
		{"Valkey", 6379},
		{"Valkey TLS", 6380},
		{"pgbouncer pooler", 6432},
	} {
		dest := destination{
			namespace:             WorkspaceNamespace(ws),
			ip:                    "10.43.0.17", // a cluster ClusterIP: RFC1918
			port:                  tc.port,
			sameNamespaceAsSource: true,
		}
		if !egressAllowed(policies, dest) {
			t.Errorf("a co-located %s (:%d) is NOT reachable from its own workspace's App pods — ADR043 D8's fix rests on this path being open",
				tc.what, tc.port)
		}
	}
}

// TestHostingAllowSetDeniesDatastoresInAnotherNamespace records WHY the fix is
// co-location rather than a cross-namespace allow: nothing in the hosting set
// reaches an in-cluster workload outside the namespace, because every
// ClusterIP is RFC1918 and allow-internet-egress excepts exactly those ranges.
//
// This is the shape of the production failure (`Connection timed out` against
// a datastore in the shared namespace), pinned so a future reader does not
// "fix" it by punching a cross-namespace hole — that would re-open tenant
// east-west reachability, which ADR043 D3 exists to close.
func TestHostingAllowSetDeniesDatastoresInAnotherNamespace(t *testing.T) {
	policies := allowNetworkPolicies(WorkspaceNamespace("tea-reach"), RegimeHosting)

	dest := destination{
		namespace:             "default", // where datastores wrongly lived
		ip:                    "10.43.0.17",
		port:                  5432,
		sameNamespaceAsSource: false,
	}
	if egressAllowed(policies, dest) {
		t.Error("egress to an out-of-namespace datastore is allowed; the tenant east-west boundary (ADR043 D3) has been widened")
	}
}

// ingressAllowed is egressAllowed's inbound twin: it evaluates whether the
// generated policy set admits a connection FROM the given peer. Same additive
// semantics, same deliberate modeling of the generated output.
func ingressAllowed(policies []*networkingv1.NetworkPolicy, src destination) bool {
	for _, p := range policies {
		if !hasIngressType(p) {
			continue
		}
		for _, rule := range p.Spec.Ingress {
			for _, peer := range rule.From {
				if peerMatches(peer, src) {
					return true
				}
			}
		}
	}
	return false
}

func hasIngressType(p *networkingv1.NetworkPolicy) bool {
	for _, t := range p.Spec.PolicyTypes {
		if t == networkingv1.PolicyTypeIngress {
			return true
		}
	}
	return false
}

// TestHostingAdmitsDatastoreControlPlanes pins ADR043 D8.3. A datastore is not
// just another application pod: it is driven by the CNPG operator, fronted by
// the SNI proxies, queried directly by bex-api, and scraped by Prometheus.
// Before D8 those peers reached it in the shared namespace, which has no
// default-deny at all; in a tenant namespace each one has to be admitted.
//
// Omitting cnpg-system in particular reproduces w7/m33 — Postgres stuck in
// bootstrap because a policy blocked its control traffic — once per tenant
// namespace rather than once, so it gets its own named case.
func TestHostingAdmitsDatastoreControlPlanes(t *testing.T) {
	policies := allowNetworkPolicies(WorkspaceNamespace("tea-control"), RegimeHosting)

	for _, tc := range []struct {
		peer string
		why  string
	}{
		{"cnpg-system", "the CNPG operator reconciling its Cluster — blocking it is w7/m33's stalled bootstrap"},
		{"bex-system", "the pg-/kv-sni-proxies serving public access, and bex-api's direct query path"},
		{"monitoring", "Prometheus scraping datastore metrics for the API and the disk autoscaler"},
	} {
		if !ingressAllowed(policies, destination{namespace: tc.peer}) {
			t.Errorf("a hosting namespace does not admit %s: %s", tc.peer, tc.why)
		}
	}
}

// TestHostingDoesNotAdmitArbitraryNamespaces is the counterweight: D8.3 widens
// ingress to three named platform namespaces, and must not have widened it to
// everything. Another tenant's namespace is the case that matters — that is the
// boundary ADR043 D3 exists to hold.
func TestHostingDoesNotAdmitArbitraryNamespaces(t *testing.T) {
	policies := allowNetworkPolicies(WorkspaceNamespace("tea-control"), RegimeHosting)

	for _, peer := range []string{WorkspaceNamespace("tea-other"), "tea-other-sandbox", "default", "bex-build", "kube-system"} {
		if ingressAllowed(policies, destination{namespace: peer}) {
			t.Errorf("a hosting namespace admits ingress from %q; D8.3 must admit only the named platform control planes", peer)
		}
	}
}

// TestSandboxRegimeGainsNoDatastoreAllow guards the regime split (ADR043 D2):
// the sandbox namespace is sealed, and a policy added for hosting must not leak
// into it. bex-api creating any allow there is rejected by admission anyway
// (D6), so a leak would surface as a failing reconcile, not a quiet hole — but
// failing here is far cheaper than failing in the cluster.
func TestSandboxRegimeGainsNoDatastoreAllow(t *testing.T) {
	if got := allowNetworkPolicies(SandboxNamespace("tea-control"), RegimeSandbox); len(got) != 0 {
		t.Errorf("sandbox regime generated %d allow policies, want 0", len(got))
	}
}

// TestInternetEgressStillExceptsPrivateRanges pins the mechanism the previous
// test depends on. Without the RFC1918 exceptions, allow-internet-egress would
// silently grant every in-cluster destination and the boundary would be gone
// while both tests above still passed.
func TestInternetEgressStillExceptsPrivateRanges(t *testing.T) {
	policies := allowNetworkPolicies(WorkspaceNamespace("tea-reach"), RegimeHosting)

	for _, ip := range []string{"10.43.0.17", "172.16.5.4", "192.168.1.9", "100.64.0.3", "169.254.169.254"} {
		dest := destination{namespace: "elsewhere", ip: ip, port: 443}
		if egressAllowed(policies, dest) {
			t.Errorf("private/link-local address %s is reachable via the internet-egress allow", ip)
		}
	}
	// The complement: a genuinely public address must still be reachable, or
	// this policy would be a blanket deny and the exceptions untested.
	if !egressAllowed(policies, destination{namespace: "internet", ip: "93.184.216.34", port: 443}) {
		t.Error("public internet egress is denied; the allow no longer functions")
	}
}
