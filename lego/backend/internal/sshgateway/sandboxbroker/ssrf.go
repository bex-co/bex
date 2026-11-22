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

package sandboxbroker

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// Resolver resolves DNS hostnames to IP addresses. *net.Resolver satisfies it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

var (
	// ErrSSRFBlocked indicates a private, loopback, link-local, metadata,
	// cluster, or reserved destination blocked by the SSRF floor.
	ErrSSRFBlocked = errors.New("destination blocked by SSRF policy")
	// ErrDNSLookupFailed indicates DNS resolution failed or returned no IPs.
	ErrDNSLookupFailed = errors.New("destination DNS lookup failed")
	// ErrInvalidTarget indicates a malformed host or port.
	ErrInvalidTarget = errors.New("invalid destination target")
)

// SSRFGuard enforces resolve-once DNS lookup and blocks every private,
// loopback, link-local, metadata, cluster, and deployment-denied IP range
// before any dial occurs.
type SSRFGuard struct {
	Resolver     Resolver
	DenyPrefixes []netip.Prefix
}

// BlockedAddr reports whether addr falls in any built-in or configured deny
// prefix (or is invalid). IPv4-mapped IPv6 addresses are unmapped first so
// ::ffff:127.0.0.1 and ::ffff:169.254.169.254 are always caught.
func (g SSRFGuard) BlockedAddr(addr netip.Addr) bool {
	return sshgateway.BlockedEgressAddr(addr, g.DenyPrefixes)
}

// ResolveAndCheck validates host:port, resolves DNS ONCE (for hostnames),
// verifies that EVERY returned IP is public, and returns the exact AddrPort to
// dial directly so DNS rebinding cannot redirect the subsequent dial.
func (g SSRFGuard) ResolveAndCheck(ctx context.Context, rawHost string, port int) (addr netip.AddrPort, canonicalHost string, literalIP bool, err error) {
	if port <= 0 || port > 65535 {
		return netip.AddrPort{}, "", false, ErrInvalidTarget
	}
	host := NormalizeHost(rawHost)
	if host == "" {
		return netip.AddrPort{}, "", false, ErrInvalidTarget
	}

	var candidates []netip.Addr
	if ip, parseErr := netip.ParseAddr(strings.Trim(host, "[]")); parseErr == nil {
		literalIP = true
		candidates = []netip.Addr{ip.Unmap()}
	} else {
		if isBlockedHostName(host) {
			return netip.AddrPort{}, "", false, ErrSSRFBlocked
		}
		res := g.Resolver
		if res == nil {
			res = net.DefaultResolver
		}
		// Trailing dot prevents cluster DNS search-domain expansion.
		addrs, lookupErr := res.LookupNetIP(ctx, "ip", host+".")
		if lookupErr != nil || len(addrs) == 0 {
			return netip.AddrPort{}, "", false, ErrDNSLookupFailed
		}
		candidates = make([]netip.Addr, 0, len(addrs))
		for _, a := range addrs {
			candidates = append(candidates, a.Unmap())
		}
		canonicalHost = host
	}

	// Reject if ANY returned IP is blocked (defends against mixed-answer DNS rebinding).
	for _, a := range candidates {
		if g.BlockedAddr(a) {
			return netip.AddrPort{}, "", literalIP, ErrSSRFBlocked
		}
	}

	chosen := candidates[0]
	for _, a := range candidates {
		if a.Is4() {
			chosen = a
			break
		}
	}
	return netip.AddrPortFrom(chosen, uint16(port)), canonicalHost, literalIP, nil
}

func isBlockedHostName(host string) bool {
	if !strings.Contains(host, ".") {
		return true
	}
	if sessionegress.PrivateOrClusterHost(host) {
		return true
	}
	for _, suffix := range []string{".svc", ".home.arpa", ".arpa", ".localdomain", ".lan", ".corp", ".home"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}
