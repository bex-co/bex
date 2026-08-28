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
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedProxies is the parsed form of BEX_TRUSTED_PROXY_CIDRS: the CIDRs of
// the edge proxies (the Traefik pods) whose forwarding headers bex-api may
// believe. In production every public request's TCP peer is a Traefik pod, so
// an IP-keyed rate limiter that reads only RemoteAddr keys every anonymous
// Internet client into ONE shared bucket (.pm/w4/029.md report #10). nil/empty
// means "trust nobody": ClientIP then returns the raw peer IP and ignores
// X-Forwarded-For/X-Real-IP entirely — byte-identical to the behavior before
// trusted-proxy support existed.
type TrustedProxies []netip.Prefix

// ParseTrustedProxies parses a comma-separated CIDR list. Empty input returns
// nil (no trusted proxies). Any malformed member is an error so a typo'd env
// value fails closed at startup instead of silently trusting nobody.
func ParseTrustedProxies(csv string) (TrustedProxies, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, nil
	}
	var out TrustedProxies
	for _, member := range strings.Split(csv, ",") {
		member = strings.TrimSpace(member)
		if member == "" {
			return nil, fmt.Errorf("empty CIDR member in %q", csv)
		}
		prefix, err := netip.ParsePrefix(member)
		if err != nil {
			return nil, fmt.Errorf("bad CIDR %q: %w", member, err)
		}
		out = append(out, prefix)
	}
	return out, nil
}

// trusted reports whether addr is inside any configured CIDR.
func (p TrustedProxies) trusted(addr netip.Addr) bool {
	for _, prefix := range p {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// TrustsPeer reports whether the request's immediate TCP peer (RemoteAddr) is
// one of the configured trusted proxies. It is the gate for believing any
// forwarding header the peer set (e.g. X-Forwarded-Proto): an untrusted peer's
// forged header must never influence a security decision. Empty/nil proxies
// trust nobody, so this is always false — matching ClientIP's peer-only path.
func (p TrustedProxies) TrustsPeer(r *http.Request) bool {
	addr, err := netip.ParseAddr(PeerIP(r.RemoteAddr))
	return err == nil && p.trusted(addr)
}

// ClientIP derives the client IP for rate-limit keying. When the immediate
// peer (RemoteAddr) is NOT a trusted proxy — or no proxies are configured —
// the peer IP is returned and the forwarding headers are ignored, so a spoofed
// X-Forwarded-For from an untrusted peer can never influence the key. When the
// peer IS trusted, the client is the first untrusted X-Forwarded-For entry
// walking rightmost-to-leftmost (the entry the outermost trusted proxy vouches
// for; entries the client itself prepended sit left of it and are skipped); if
// every entry is trusted the leftmost is the best available answer. X-Real-IP
// is the fallback when X-Forwarded-For is absent, and the peer IP is the last
// resort when neither header carries a usable address.
func (p TrustedProxies) ClientIP(r *http.Request) string {
	peer := PeerIP(r.RemoteAddr)
	peerAddr, err := netip.ParseAddr(peer)
	if err != nil || !p.trusted(peerAddr) {
		return peer
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			entry, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err != nil {
				// A malformed entry means the chain can no longer be walked
				// reliably — stop and fall back rather than guess left of it.
				break
			}
			if !p.trusted(entry) || i == 0 {
				return entry.String()
			}
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if xriAddr, err := netip.ParseAddr(xri); err == nil {
			return xriAddr.String()
		}
	}
	return peer
}

// PeerIP extracts the host part of a host:port remote address, returning the
// raw value when it is not one (or the host is empty) — the exact fallback
// semantics the rate limiters had before trusted-proxy support.
func PeerIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil || host == "" {
		return remoteAddr
	}
	return host
}
