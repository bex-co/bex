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
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// An inbound IP allowlist matches the TCP source Traefik sees (w1/m150). On a
// custom domain whose DNS record is proxied by Cloudflare (the "orange cloud")
// that source is a Cloudflare edge, never the end user, so the allowlist
// cannot do what the tenant set it for there. m150 Decision 2 chose to warn
// rather than refuse — refusing would break every tenant who already saved
// such an allowlist — so the save is accepted and every surface names the
// hosts the warning is about (w1/m171).
//
// Detection is a resolver check against Cloudflare's published ranges. Nothing
// else in bex knows whether a host is proxied: domainCertificateReason (w3/037)
// relays cert-manager's free-text reason and is empty once a certificate is
// issued, which is the steady state of most proxied hosts. A stale range list
// can only make the warning miss a host; it never admits or refuses traffic,
// which is why keeping it here is acceptable where trusting CF-Connecting-IP
// from the same list (Decision 2 option b) was not.

// cloudflareRanges is Cloudflare's published edge address space, fetched
// 2026-10-02 from https://www.cloudflare.com/ips-v4 and /ips-v6.
var cloudflareRanges = mustParsePrefixes(
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
)

func mustParsePrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}

// HostResolver is the one DNS call proxy detection makes; *net.Resolver
// satisfies it, and tests inject a deterministic map.
type HostResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

const (
	proxiedHostTTL           = 5 * time.Minute
	proxiedHostLookupTimeout = 2 * time.Second
)

// ProxiedHostDetector reports which hosts resolve into Cloudflare's edge.
// Answers are cached per host for proxiedHostTTL so a Settings page load does
// not re-resolve every custom domain; a failed lookup is not cached and reads
// as "not proxied" — the result is a hint and must never fail the read it
// decorates. A nil *ProxiedHostDetector detects nothing, which is what tests
// and storeless compositions get unless they wire one.
type ProxiedHostDetector struct {
	// Resolver is the DNS seam; nil uses net.DefaultResolver.
	Resolver HostResolver

	once  sync.Once
	cache *TTLCache[bool]
}

// NewProxiedHostDetector returns a detector backed by the system resolver —
// the composition root's production wiring.
func NewProxiedHostDetector() *ProxiedHostDetector {
	return &ProxiedHostDetector{Resolver: net.DefaultResolver}
}

// CloudflareProxied returns the subset of hosts (in input order, deduplicated)
// that resolve to at least one Cloudflare edge address. Lookups run
// concurrently, each bounded by proxiedHostLookupTimeout.
func (d *ProxiedHostDetector) CloudflareProxied(ctx context.Context, hosts []string) []string {
	if d == nil || len(hosts) == 0 {
		return nil
	}
	d.once.Do(func() { d.cache = NewTTLCache[bool]() })
	hosts = uniqueHosts(hosts)
	proxied := make([]bool, len(hosts))
	var wg sync.WaitGroup
	for i, host := range hosts {
		if v, ok := d.cache.Get(host); ok {
			proxied[i] = v
			continue
		}
		wg.Go(func() {
			v, ok := d.lookup(ctx, host)
			if ok {
				d.cache.Put(host, v, time.Now().Add(proxiedHostTTL))
			}
			proxied[i] = v
		})
	}
	wg.Wait()
	var out []string
	for i, host := range hosts {
		if proxied[i] {
			out = append(out, host)
		}
	}
	return out
}

func (d *ProxiedHostDetector) lookup(ctx context.Context, host string) (proxied, ok bool) {
	resolver := d.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	lookupCtx, cancel := context.WithTimeout(ctx, proxiedHostLookupTimeout)
	defer cancel()
	addrs, err := resolver.LookupNetIP(lookupCtx, "ip", host)
	if err != nil {
		return false, false
	}
	return slices.ContainsFunc(addrs, IsCloudflareAddr), true
}

// IsCloudflareAddr reports whether addr is inside Cloudflare's published
// edge ranges (an IPv4-mapped IPv6 address is matched as IPv4).
func IsCloudflareAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	return slices.ContainsFunc(cloudflareRanges, func(p netip.Prefix) bool { return p.Contains(addr) })
}

func uniqueHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
		if h != "" && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}

// AllowListRestricts reports whether entries actually narrow who may connect:
// non-empty and without a match-everything /0 prefix. An open list (Render's
// seeded 0.0.0.0/0 environment default) admits a Cloudflare edge exactly as it
// admits anyone, so there is nothing to warn about. An empty list is not
// restricting either: on a service it means open, and on an environment it is
// deny-all, which refuses Cloudflare and clients alike.
func AllowListRestricts(entries []IPAllowListEntry) bool {
	if len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if p, err := netip.ParsePrefix(strings.TrimSpace(e.CIDRBlock)); err == nil && p.Bits() == 0 {
			return false
		}
	}
	return true
}

// AppCustomDomains is the tenant-chosen public hosts an App is served at:
// spec.host and spec.hosts, for the publicly routable types only. The
// platform host `<slug>.<BEX_BASE_DOMAIN>` is not among them — it is DNS-only
// by construction (w1/m150 live probes), so it can never be proxied.
func AppCustomDomains(a *appv1alpha1.App) []string {
	if a == nil || !a.Spec.PubliclyRoutable() {
		return nil
	}
	return uniqueHosts(append([]string{a.Spec.Host}, a.Spec.Hosts...))
}

// ProxiedAllowListWarning is the one sentence MCP prints next to a result
// whose allowlist field names proxied hosts; empty when there are none. The
// save it accompanies has been applied — this warns, it does not refuse.
func ProxiedAllowListWarning(hosts []string) string {
	if len(hosts) == 0 {
		return ""
	}
	return "Warning: these custom domains are proxied by Cloudflare, so on them the inbound IP allowlist " +
		"matches Cloudflare's edge address instead of the client's: " +
		strings.Join(hosts, ", ") +
		". Set their DNS records to DNS-only for the allowlist to see real client addresses."
}
