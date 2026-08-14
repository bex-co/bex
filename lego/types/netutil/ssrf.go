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

// Package netutil provides outbound-dial guards shared across the bex modules.
package netutil

import (
	"context"
	"errors"
	"net"
	"time"
)

// mustPrefix parses a CIDR literal at package init, panicking on a typo — a
// silently nil network would make Contains always-false, failing the deny
// list OPEN. net.IPNet (not netip.Prefix) is deliberate: its Contains
// normalizes IPv4-mapped IPv6 answers via To4, netip's does not.
func mustPrefix(cidr string) net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("netutil: invalid special-purpose prefix " + cidr)
	}
	return *network
}

// specialPurposePrefixes are the non-public ranges net.IP's predicates do NOT
// cover — the remainder of the IANA special-purpose registries after
// IsLoopback/IsPrivate/IsLinkLocal*/IsMulticast/IsUnspecified have matched
// their families (codex r7 #14; the CGNAT entry is w1/m65 F15). None is a
// legitimate webhook destination, and the IPv6 transition prefixes (NAT64 /
// Teredo / 6to4) can embed a private IPv4 target inside a "public-looking"
// v6 answer. Kept deliberately separate from egressmeter's nonPublicPrefixes:
// that list decides what traffic is NOT BILLED, this one decides what may
// not be dialled — changing one must never silently change the other.
var specialPurposePrefixes = []net.IPNet{
	mustPrefix("0.0.0.0/8"),       // "this network" (IsUnspecified matches only 0.0.0.0 itself)
	mustPrefix("100.64.0.0/10"),   // RFC 6598 shared CGNAT space
	mustPrefix("192.0.0.0/24"),    // IETF protocol assignments
	mustPrefix("192.0.2.0/24"),    // TEST-NET-1
	mustPrefix("198.18.0.0/15"),   // benchmarking
	mustPrefix("198.51.100.0/24"), // TEST-NET-2
	mustPrefix("203.0.113.0/24"),  // TEST-NET-3
	mustPrefix("240.0.0.0/4"),     // reserved, incl. 255.255.255.255 broadcast
	mustPrefix("64:ff9b::/96"),    // NAT64 well-known prefix (embeds an IPv4 target)
	mustPrefix("100::/64"),        // discard-only
	mustPrefix("2001::/32"),       // Teredo (embeds an IPv4 target)
	mustPrefix("2001:db8::/32"),   // documentation
	mustPrefix("2002::/16"),       // 6to4 (embeds an IPv4 target)
}

// UnsafeOriginIP reports whether ip must not be dialled as an outbound webhook
// or fetch target: loopback, private (RFC1918 + IPv6 ULA), link-local
// unicast/multicast, multicast, unspecified, and every special-purpose prefix
// above are off-limits to prevent SSRF against cloud-metadata endpoints
// (169.254.169.254) or internal services. The policy is an explicit
// not-globally-routable deny, not merely "not IsPrivate" (net.IPNet.Contains
// normalizes IPv4-mapped IPv6 via To4, so a mapped-form answer is caught too).
func UnsafeOriginIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, prefix := range specialPurposePrefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// UnsafeMetadataIP is the guard for an in-cluster client that MUST still reach
// private ClusterIP services (the registry, Prometheus) but must never dial the
// cloud-metadata endpoint (169.254.169.254). Unlike UnsafeOriginIP it permits
// private (RFC1918/CGNAT) ranges, and it also permits loopback — the operator
// pod's own localhost is not a meaningful SSRF target and blocking it would
// break httptest-based clients. It blocks link-local (the metadata endpoint),
// multicast, and unspecified addresses. Use it for the operator's shared
// transport; use UnsafeOriginIP for tenant-facing fetches.
func UnsafeMetadataIP(ip net.IP) bool {
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

// SafeDialContext returns a DialContext function suitable for
// http.Transport.DialContext that resolves the hostname and blocks the
// connection if any resolved address is loopback, private, link-local,
// multicast, or unspecified. timeout is the TCP-level dial deadline; pass 0 to
// rely on the request context alone.
func SafeDialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return SafeDialContextFunc(timeout, UnsafeOriginIP)
}

// SafeDialContextFunc is SafeDialContext with a caller-supplied unsafe-address
// predicate, so an in-cluster client can block the metadata endpoint while still
// reaching private ClusterIP services (pass UnsafeMetadataIP) instead of the
// stricter UnsafeOriginIP. It resolves the host and refuses the connection when
// any resolved address is unsafe (DNS-rebind safe).
func SafeDialContextFunc(timeout time.Duration, unsafe func(net.IP) bool) func(context.Context, string, string) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, errors.New("host did not resolve")
		}
		for _, a := range addrs {
			if unsafe(a.IP) {
				return nil, errors.New("destination resolves to a blocked address")
			}
		}
		return d.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
	}
}
