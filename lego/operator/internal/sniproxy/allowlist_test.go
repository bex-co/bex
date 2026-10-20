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

package sniproxy

import (
	"net/netip"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestParseAllowListMasksAndSeparatesLayers(t *testing.T) {
	// A host-bit-carrying CIDR must come back masked, or Contains would miss.
	allow, envAllow, err := ParseAllowList(
		[]appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.5/24"}},
		[]string{"10.1.2.3/8"},
	)
	if err != nil {
		t.Fatalf("ParseAllowList: %v", err)
	}
	if got := allow[0].String(); got != "203.0.113.0/24" {
		t.Errorf("resource layer = %s, want 203.0.113.0/24", got)
	}
	if got := envAllow[0].String(); got != "10.0.0.0/8" {
		t.Errorf("environment layer = %s, want 10.0.0.0/8", got)
	}
}

func TestParseAllowListRejectsMalformedCIDR(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []appv1alpha1.IPAllowEntry
		env     []string
		want    string
	}{
		{"resource layer", []appv1alpha1.IPAllowEntry{{CIDR: "not-a-cidr"}}, nil, "invalid allowlist CIDR"},
		{"environment layer", nil, []string{"10.0.0.0/99"}, "invalid environment allowlist CIDR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allow, envAllow, err := ParseAllowList(tc.entries, tc.env)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			// A partial parse must not reach a caller — half an allowlist would
			// silently serve a resource with the wrong rule set.
			if allow != nil || envAllow != nil {
				t.Errorf("partial result leaked: allow=%v envAllow=%v", allow, envAllow)
			}
		})
	}
}

func TestAllowedBy(t *testing.T) {
	prefixes := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	for _, tc := range []struct {
		name     string
		source   string
		prefixes []netip.Prefix
		want     bool
	}{
		{"empty layer is unrestricted", "198.51.100.7", nil, true},
		{"inside the prefix", "203.0.113.9", prefixes, true},
		{"outside the prefix", "198.51.100.7", prefixes, false},
		// A v4-mapped v6 source must compare as v4, or every allowlist would
		// reject clients arriving over a dual-stack listener.
		{"v4-mapped v6 source", "::ffff:203.0.113.9", prefixes, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AllowedBy(netip.MustParseAddr(tc.source), tc.prefixes); got != tc.want {
				t.Errorf("AllowedBy(%s) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}

// TestRenderAnyIPRuleAdmitsIPv6 (w8/140): Render's "allow any IP" rule is
// exactly 0.0.0.0/0. Our datastore hosts also publish AAAA, so in either layer
// that rule must admit IPv6 sources too. Narrower IPv4 rules are not widened,
// and a source must still pass both layers.
func TestRenderAnyIPRuleAdmitsIPv6(t *testing.T) {
	v6 := netip.MustParseAddr("2a01:4f8:c01e:3d1f::1")
	v4 := netip.MustParseAddr("49.12.20.236")
	any4 := []appv1alpha1.IPAllowEntry{{CIDR: "0.0.0.0/0", Description: "everywhere"}}
	narrow := []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24"}}
	for _, tc := range []struct {
		name    string
		entries []appv1alpha1.IPAllowEntry
		env     []string
		source  netip.Addr
		want    bool
	}{
		{"resource layer 0.0.0.0/0 admits IPv6", any4, nil, v6, true},
		{"resource layer 0.0.0.0/0 still admits IPv4", any4, nil, v4, true},
		{"a host-bit spelling of 0.0.0.0/0 admits IPv6", []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.1/0"}}, nil, v6, true},
		{"environment layer 0.0.0.0/0 admits IPv6", nil, []string{"0.0.0.0/0"}, v6, true},
		{"both layers 0.0.0.0/0 admit IPv6", any4, []string{"0.0.0.0/0"}, v6, true},
		{"a narrow IPv4 rule still refuses IPv6", narrow, nil, v6, false},
		{"a narrow environment rule still refuses IPv6", nil, []string{"203.0.113.0/24"}, v6, false},
		{"the layers still AND: open resource, narrow environment", any4, []string{"203.0.113.0/24"}, v6, false},
		{"the layers still AND: narrow resource, open environment", narrow, []string{"0.0.0.0/0"}, v6, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allow, envAllow, err := ParseAllowList(tc.entries, tc.env)
			if err != nil {
				t.Fatalf("ParseAllowList: %v", err)
			}
			if got := AllowedBy(tc.source, allow) && AllowedBy(tc.source, envAllow); got != tc.want {
				t.Errorf("admitted %s = %v, want %v (allow %v, env %v)", tc.source, got, tc.want, allow, envAllow)
			}
		})
	}
}
