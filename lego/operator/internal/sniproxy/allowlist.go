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
func ParseAllowList(entries []appv1alpha1.IPAllowEntry, environment []string) (allow, envAllow []netip.Prefix, err error) {
	for _, entry := range entries {
		prefix, err := netip.ParsePrefix(entry.CIDR)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid allowlist CIDR %q: %w", entry.CIDR, err)
		}
		allow = append(allow, prefix.Masked())
	}
	for _, cidr := range environment {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid environment allowlist CIDR %q: %w", cidr, err)
		}
		envAllow = append(envAllow, prefix.Masked())
	}
	return allow, envAllow, nil
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
