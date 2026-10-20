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

// ParseAllowList parses the two inbound-IP layers a public managed-datastore
// spec carries — the resource's own IPAllowList and the environment-layer
// EnvironmentIPAllowList — into masked prefixes. The layers stay separate
// because a source must pass BOTH (AND semantics, see Database.Spec docs); a
// caller that merged them would silently widen the rule set.
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
func ParseAllowList(entries []appv1alpha1.IPAllowEntry, environment []string) (allow, envAllow []netip.Prefix, err error) {
	for _, entry := range entries {
		if allow, err = appendRule(allow, entry.CIDR); err != nil {
			return nil, nil, fmt.Errorf("invalid allowlist CIDR %q: %w", entry.CIDR, err)
		}
	}
	for _, cidr := range environment {
		if envAllow, err = appendRule(envAllow, cidr); err != nil {
			return nil, nil, fmt.Errorf("invalid environment allowlist CIDR %q: %w", cidr, err)
		}
	}
	return allow, envAllow, nil
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

// AllowedBy reports whether source falls inside one layer's prefixes. An empty
// layer is unrestricted — that is what makes the two layers composable: a
// resource with no allowlist of its own still inherits its environment's.
func AllowedBy(source netip.Addr, prefixes []netip.Prefix) bool {
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
