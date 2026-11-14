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
	"fmt"
	"net/netip"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// AllowLayers are the parsed inbound-IP layers of one public managed
// datastore: its own IPAllowList, the environment-layer
// EnvironmentIPAllowList (w4/m28) and the workspace-layer
// WorkspaceIPAllowList (w9/m388). The layers stay separate because a source
// must pass EVERY one (AND semantics, see Database.Spec docs; Render: "an
// inbound IP must be allowed by each level's rules"); a caller that merged
// them would silently widen the rule set.
type AllowLayers struct {
	Own         []netip.Prefix
	Environment []netip.Prefix
	Workspace   []netip.Prefix
}

// Admits reports whether source passes every layer. An empty layer is
// unrestricted (allowedBy), which is what makes the layers composable: a
// resource with no allowlist of its own still inherits its environment's and
// its workspace's, and a workspace that never set rules adds nothing.
func (l AllowLayers) Admits(source netip.Addr) bool {
	return allowedBy(source, l.Own) && allowedBy(source, l.Environment) && allowedBy(source, l.Workspace)
}

// ParseAllowList parses the three inbound-IP layers a public managed-datastore
// spec carries into masked prefixes.
//
// A malformed CIDR anywhere fails the whole parse: the front doors treat that
// as an unroutable resource rather than serving it with a partial allowlist.
//
// Render's "allow any IP" rule is exactly 0.0.0.0/0, and Render's datastore
// endpoints are IPv4-only, so there it admits every client. Ours also publish
// AAAA, and dual-stack clients try IPv6 first, so the literal rule would
// refuse every one of them (w8/140). A rule that masks to 0.0.0.0/0 therefore
// also admits ::/0 here, at enforcement only: the saved rule and every
// readback stay as authored. Narrower IPv4 rules are not widened.
func ParseAllowList(entries []appv1alpha1.IPAllowEntry, environment, workspace []string) (AllowLayers, error) {
	own := make([]string, len(entries))
	for i, entry := range entries {
		own[i] = entry.CIDR
	}
	var layers AllowLayers
	var err error
	if layers.Own, err = parseLayer("allowlist", own); err != nil {
		return AllowLayers{}, err
	}
	if layers.Environment, err = parseLayer("environment allowlist", environment); err != nil {
		return AllowLayers{}, err
	}
	if layers.Workspace, err = parseLayer("workspace allowlist", workspace); err != nil {
		return AllowLayers{}, err
	}
	return layers, nil
}

// parseLayer parses one layer's CIDRs into masked prefixes; name labels a
// malformed CIDR's error.
func parseLayer(name string, cidrs []string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, cidr := range cidrs {
		var err error
		if prefixes, err = appendRule(prefixes, cidr); err != nil {
			return nil, fmt.Errorf("invalid %s CIDR %q: %w", name, cidr, err)
		}
	}
	return prefixes, nil
}

var (
	anyIPv4 = netip.MustParsePrefix("0.0.0.0/0")
	anyIPv6 = netip.MustParsePrefix("::/0")
)

func appendRule(prefixes []netip.Prefix, cidr string) ([]netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return prefixes, err
	}
	prefix = prefix.Masked()
	prefixes = append(prefixes, prefix)
	if prefix == anyIPv4 {
		prefixes = append(prefixes, anyIPv6)
	}
	return prefixes, nil
}

// allowedBy reports whether source falls inside one layer's prefixes. An empty
// layer is unrestricted — that is what makes the layers composable (see
// AllowLayers.Admits).
func allowedBy(source netip.Addr, prefixes []netip.Prefix) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if prefix.Contains(source.Unmap()) {
			return true
		}
	}
	return false
}
