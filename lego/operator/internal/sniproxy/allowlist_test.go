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
	layers, err := ParseAllowList(
		[]appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.5/24"}},
		[]string{"10.1.2.3/8"},
		[]string{"192.168.7.9/16"},
	)
	if err != nil {
		t.Fatalf("ParseAllowList: %v", err)
	}
	if got := layers.Own[0].String(); got != "203.0.113.0/24" {
		t.Errorf("resource layer = %s, want 203.0.113.0/24", got)
	}
	if got := layers.Environment[0].String(); got != "10.0.0.0/8" {
		t.Errorf("environment layer = %s, want 10.0.0.0/8", got)
	}
	if got := layers.Workspace[0].String(); got != "192.168.0.0/16" {
		t.Errorf("workspace layer = %s, want 192.168.0.0/16", got)
	}
}

func TestParseAllowListRejectsMalformedCIDR(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []appv1alpha1.IPAllowEntry
		env     []string
		ws      []string
		want    string
	}{
		{"resource layer", []appv1alpha1.IPAllowEntry{{CIDR: "not-a-cidr"}}, nil, nil, "invalid allowlist CIDR"},
		{"environment layer", nil, []string{"10.0.0.0/99"}, nil, "invalid environment allowlist CIDR"},
		{"workspace layer", nil, nil, []string{"10.0.0.0/99"}, "invalid workspace allowlist CIDR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layers, err := ParseAllowList(tc.entries, tc.env, tc.ws)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			// A partial parse must not reach a caller — half an allowlist would
			// silently serve a resource with the wrong rule set.
			if layers.Own != nil || layers.Environment != nil || layers.Workspace != nil {
				t.Errorf("partial result leaked: %+v", layers)
			}
		})
	}
}

// TestWorkspaceLayerIsAThirdAND pins w9/m388: the workspace layer composes
// with the resource and environment layers by intersection, as Render's
// "an inbound IP must be allowed by each level's rules" says. A source the
// service and environment admit is refused when the workspace does not; an
// absent workspace layer changes nothing; the deny-all placeholder an empty
// workspace rule list projects refuses everyone.
func TestWorkspaceLayerIsAThirdAND(t *testing.T) {
	own := []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24"}}
	env := []string{"203.0.113.0/25"}
	inside := netip.MustParseAddr("203.0.113.9")
	for _, tc := range []struct {
		name   string
		ws     []string
		source netip.Addr
		want   bool
	}{
		{"no workspace layer: service and environment decide", nil, inside, true},
		{"workspace admits too", []string{"203.0.113.0/28"}, inside, true},
		{"service and environment admit, workspace refuses", []string{"198.51.100.0/24"}, inside, false},
		{"deny-all placeholder refuses an otherwise admitted source", []string{"255.255.255.255/32"}, inside, false},
		{"deny-all placeholder refuses IPv6 too", []string{"255.255.255.255/32"}, netip.MustParseAddr("2001:db8::1"), false},
		{"workspace admits, environment still refuses", []string{"0.0.0.0/0"}, netip.MustParseAddr("203.0.113.200"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layers, err := ParseAllowList(own, env, tc.ws)
			if err != nil {
				t.Fatalf("ParseAllowList: %v", err)
			}
			if got := layers.Admits(tc.source); got != tc.want {
				t.Errorf("Admits(%s) = %v, want %v (layers %+v)", tc.source, got, tc.want, layers)
			}
		})
	}
	// Workspace layer alone (no service or environment rule): it is the only
	// restriction, and it holds.
	layers, err := ParseAllowList(nil, nil, []string{"198.51.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if layers.Admits(inside) || !layers.Admits(netip.MustParseAddr("198.51.100.7")) {
		t.Errorf("workspace-only layer: admits %s=%v, 198.51.100.7=%v", inside, layers.Admits(inside), layers.Admits(netip.MustParseAddr("198.51.100.7")))
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
			if got := allowedBy(netip.MustParseAddr(tc.source), tc.prefixes); got != tc.want {
				t.Errorf("allowedBy(%s) = %v, want %v", tc.source, got, tc.want)
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
			layers, err := ParseAllowList(tc.entries, tc.env, nil)
			if err != nil {
				t.Fatalf("ParseAllowList: %v", err)
			}
			if got := layers.Admits(tc.source); got != tc.want {
				t.Errorf("admitted %s = %v, want %v (layers %+v)", tc.source, got, tc.want, layers)
			}
			// The workspace layer widens 0.0.0.0/0 the same way.
			ws, err := ParseAllowList(nil, nil, tc.env)
			if err != nil {
				t.Fatalf("ParseAllowList (workspace): %v", err)
			}
			if tc.entries == nil && ws.Admits(tc.source) != tc.want {
				t.Errorf("as a workspace layer, admitted %s = %v, want %v", tc.source, ws.Admits(tc.source), tc.want)
			}
		})
	}
}
