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

// UnsafeOriginIP reports whether ip must not be dialled as an outbound webhook
// or fetch target: loopback, private, link-local unicast/multicast, multicast,
// and unspecified addresses are all off-limits to prevent SSRF against
// cloud-metadata endpoints (169.254.169.254) or internal services.
func UnsafeOriginIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
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
