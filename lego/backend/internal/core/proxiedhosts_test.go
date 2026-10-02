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
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// countingResolver answers from a table and counts lookups per host.
type countingResolver struct {
	mu      sync.Mutex
	answers map[string][]string
	calls   map[string]int
}

func (r *countingResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.calls[host]++
	raw, ok := r.answers[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	out := make([]netip.Addr, len(raw))
	for i, a := range raw {
		out[i] = netip.MustParseAddr(a)
	}
	return out, nil
}

func TestIsCloudflareAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"104.16.0.1":                true,  // 104.16.0.0/13
		"172.67.200.3":              true,  // 172.64.0.0/13
		"188.114.97.7":              true,  // 188.114.96.0/20
		"2606:4700:3030::ac43:1":    true,  // 2606:4700::/32
		"::ffff:104.21.32.1":        true,  // IPv4-mapped is matched as IPv4
		"49.12.20.236":              false, // the bex Hetzner edge
		"10.10.0.7":                 false,
		"2a01:4f8:c17:1234::1":      false, // Hetzner IPv6
		"104.15.255.255":            false, // just below 104.16.0.0/13
		"2a06:98c0::1":              true,  // the /29 IPv6 range
		"2a06:98c8::1":              false, // just past 2a06:98c0::/29
		"203.0.113.10":              false,
		"::ffff:49.12.20.236":       false,
		"2400:cb00:2048:1::6814:1b": true,
	} {
		if got := IsCloudflareAddr(netip.MustParseAddr(addr)); got != want {
			t.Errorf("IsCloudflareAddr(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestCloudflareProxiedFiltersCachesAndDegrades(t *testing.T) {
	r := &countingResolver{answers: map[string][]string{
		"orange.example.com": {"49.12.20.236", "104.21.32.1"}, // one CF address is enough
		"grey.example.com":   {"49.12.20.236"},
		"v6.example.com":     {"2606:4700::6810:1"},
	}}
	d := &ProxiedHostDetector{Resolver: r}
	hosts := []string{"Orange.Example.com.", "grey.example.com", "missing.example.com", "v6.example.com", "orange.example.com"}

	want := []string{"orange.example.com", "v6.example.com"}
	for range 2 {
		if got := d.CloudflareProxied(context.Background(), hosts); !slices.Equal(got, want) {
			t.Fatalf("CloudflareProxied = %v, want %v (input order, normalized, deduplicated)", got, want)
		}
	}
	// Answers are cached; a failed lookup is retried rather than remembered.
	for host, calls := range map[string]int{"orange.example.com": 1, "grey.example.com": 1, "v6.example.com": 1, "missing.example.com": 2} {
		if r.calls[host] != calls {
			t.Errorf("%s resolved %d times, want %d", host, r.calls[host], calls)
		}
	}

	var nilDetector *ProxiedHostDetector
	if got := nilDetector.CloudflareProxied(context.Background(), hosts); got != nil {
		t.Errorf("a nil detector must detect nothing, got %v", got)
	}
}

func TestAllowListRestricts(t *testing.T) {
	for name, tc := range map[string]struct {
		entries []IPAllowListEntry
		want    bool
	}{
		"empty":                {nil, false},
		"one office range":     {[]IPAllowListEntry{{CIDRBlock: "203.0.113.0/24"}}, true},
		"single host":          {[]IPAllowListEntry{{CIDRBlock: "198.51.100.7/32"}}, true},
		"v4 open":              {[]IPAllowListEntry{{CIDRBlock: "0.0.0.0/0"}}, false},
		"seeded default":       {DefaultEnvironmentAllowList(), false},
		"range plus v6 /0":     {[]IPAllowListEntry{{CIDRBlock: "203.0.113.0/24"}, {CIDRBlock: "::/0"}}, false},
		"deny-all placeholder": {[]IPAllowListEntry{{CIDRBlock: DenyAllCIDR}}, true},
	} {
		if got := AllowListRestricts(tc.entries); got != tc.want {
			t.Errorf("%s: AllowListRestricts = %v, want %v", name, got, tc.want)
		}
	}
}

func TestAppCustomDomains(t *testing.T) {
	a := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{
		Type:   appv1alpha1.TypeWebService,
		Expose: true,
		Host:   "www.example.com",
		Hosts:  []string{"example.com", "WWW.example.com", ""},
	}}
	if got, want := AppCustomDomains(a), []string{"www.example.com", "example.com"}; !slices.Equal(got, want) {
		t.Errorf("AppCustomDomains = %v, want %v", got, want)
	}
	a.Spec.Type = appv1alpha1.TypePrivateService
	if got := AppCustomDomains(a); got != nil {
		t.Errorf("a private service has no public hosts, got %v", got)
	}
}
