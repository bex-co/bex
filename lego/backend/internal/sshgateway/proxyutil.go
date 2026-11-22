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

package sshgateway

import (
	"net/http"
	"net/netip"
	"strings"
)

// deniedEgressPrefixes are never reachable through any gateway outbound proxy
// (egressproxy and sandboxbroker): unspecified, RFC 1918 private, CGNAT,
// loopback, link-local (including cloud metadata 169.254.169.254),
// IETF/benchmark/documentation, multicast/reserved, IPv6 loopback/ULA/
// link-local/multicast, and IPv4-embedding tunneling prefixes (NAT64, Teredo,
// 6to4).
var deniedEgressPrefixes = func() []netip.Prefix {
	raw := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001:db8::/32",
		"2001::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
	}
	out := make([]netip.Prefix, 0, len(raw))
	for _, s := range raw {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// BlockedEgressAddr reports whether addr is invalid or falls inside any
// built-in non-public/metadata/private/reserved prefix or deployment-configured
// extra deny prefix. IPv4-mapped IPv6 addresses are unmapped first.
func BlockedEgressAddr(addr netip.Addr, extra []netip.Prefix) bool {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return true
	}
	for _, prefix := range deniedEgressPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	for _, prefix := range extra {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// hopHeaders never cross an HTTP proxy hop (RFC 9110 §7.6.1).
var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// RemoveHopHeaders deletes RFC 9110 hop-by-hop headers (including any header
// names nominated by Connection) from header in place.
func RemoveHopHeaders(header http.Header) {
	for _, field := range header.Values("Connection") {
		for _, name := range strings.Split(field, ",") {
			if name = strings.TrimSpace(name); name != "" {
				header.Del(name)
			}
		}
	}
	for _, name := range hopHeaders {
		header.Del(name)
	}
}
