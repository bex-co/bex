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
	"slices"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ValidateCIDRs rejects any entry that is not a valid CIDR with ErrBadRequest —
// the shared gate for every ipAllowList write path (postgres + keyvalue +
// environments, create and set alike), so a bad entry is a 400 before anything
// lands in a CR spec or store row.
func ValidateCIDRs(cidrs []string) error {
	if len(cidrs) > MaxAllowListEntries {
		return fmt.Errorf("%w: ipAllowList has %d entries; the limit is %d", ErrBadRequest, len(cidrs), MaxAllowListEntries)
	}
	for _, c := range cidrs {
		if _, _, err := net.ParseCIDR(c); err != nil {
			return fmt.Errorf("%w: %q is not a valid CIDR", ErrBadRequest, c)
		}
	}
	return nil
}

// IPAllowListEntry is Render's ipAllowList wire shape (components.schemas
// cidrBlockAndDescription: {cidrBlock, description}), verified against the
// render-oss/cli generated client (`client.CidrBlockAndDescription`) — a bare
// CIDR-string array breaks the CLI's decode of create/get/list on postgres and
// key-value alike. Shared by the postgres, keyvalue, and environments features
// (w4/m24) so the three surfaces speak one entry shape; both fields persist end
// to end (the Database/KeyValue CR's spec.ipAllowList and the environments
// store row carry {cidr, description} entries), and an empty description reads
// back empty — never fabricated.
type IPAllowListEntry struct {
	CIDRBlock   string `json:"cidrBlock"`
	Description string `json:"description"`
}

// AllowListToSpec / AllowListFromSpec convert between the wire entries and the
// CR's product-neutral {cidr, description} spec entries — the persistence seam
// postgres and keyvalue share.
func AllowListToSpec(entries []IPAllowListEntry) []appv1alpha1.IPAllowEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]appv1alpha1.IPAllowEntry, len(entries))
	for i, e := range entries {
		out[i] = appv1alpha1.IPAllowEntry{CIDR: e.CIDRBlock, Description: e.Description}
	}
	return out
}

func AllowListFromSpec(list []appv1alpha1.IPAllowEntry) []IPAllowListEntry {
	if len(list) == 0 {
		return nil
	}
	out := make([]IPAllowListEntry, len(list))
	for i, e := range list {
		out[i] = IPAllowListEntry{CIDRBlock: e.CIDR, Description: e.Description}
	}
	return out
}

// AllowListCIDRs projects entries down to their CIDR strings — the
// product-neutral string-list shape GraphQL/MCP's legacy arguments and the
// bex-native {"cidrs"} REST routes keep speaking.
func AllowListCIDRs(entries []IPAllowListEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.CIDRBlock
	}
	return out
}

// AllowListFromCIDRs lifts bare CIDR strings into entries with empty
// descriptions — the write-side twin of AllowListCIDRs for the string-list
// arguments (a full-replace through them clears any stored descriptions,
// matching every Set verb's replace semantics).
func AllowListFromCIDRs(cidrs []string) []IPAllowListEntry {
	if len(cidrs) == 0 {
		return nil
	}
	out := make([]IPAllowListEntry, len(cidrs))
	for i, c := range cidrs {
		out[i] = IPAllowListEntry{CIDRBlock: c}
	}
	return out
}

// AllowListOrCIDRs is the one home of the GraphQL/MCP argument precedence
// rule: the description-carrying entries argument (w4/m24), when present,
// wins over the legacy string-list one. Both are a full replace.
func AllowListOrCIDRs(entries []IPAllowListEntry, cidrs []string) []IPAllowListEntry {
	if entries != nil {
		return entries
	}
	return AllowListFromCIDRs(cidrs)
}

// ResolveAllowListInputs resolves adapters that expose both the structured
// and legacy flat forms. One provided form is used directly; equivalent dual
// inputs are accepted; conflicting dual inputs are a bad request rather than
// silently dropping descriptions or CIDRs.
func ResolveAllowListInputs(entries []IPAllowListEntry, entriesSet bool, cidrs []string, cidrsSet bool) ([]IPAllowListEntry, error) {
	if !entriesSet {
		return AllowListFromCIDRs(cidrs), nil
	}
	if !cidrsSet {
		return entries, nil
	}
	legacy := AllowListFromCIDRs(cidrs)
	if !slices.Equal(entries, legacy) {
		return nil, fmt.Errorf("%w: ipAllowList and ipAllowListEntries are set to conflicting values", ErrBadRequest)
	}
	return entries, nil
}

// DenyAllCIDR is the unmatchable placeholder the environment fan-out projects
// for an explicitly empty environment rule list (w4/m28, Render's
// empty-means-deny-all): Traefik rejects an ipAllowList middleware with an
// empty sourceRange, so deny-all must be a real range no source matches.
const DenyAllCIDR = "255.255.255.255/32"

// MaxAllowListEntries caps how many CIDR entries one ipAllowList write may carry
// (round-5 finding 15). Every prefix is materialized into the cluster-wide
// pg-sni-proxy / kv-sni-proxy router and scanned on each TLS handshake, so an
// unbounded list lets one contributor amplify cheap handshakes into shared-proxy
// CPU and router-lock contention against other tenants. 1000 is far above any
// legitimate allowlist and fails closed in the shared validators below, so the
// REST/GraphQL/MCP write paths all inherit the bound.
const MaxAllowListEntries = 1000

// DefaultEnvironmentAllowList is the seeded allow-all rule pair a new (or
// migrated pre-m28) environment starts with — Render's dashboard seeds
// 0.0.0.0/0; the ::/0 twin is bex's IPv6 extension so the semantic flip to
// empty-means-deny-all never cut off IPv6 sources that an absent middleware
// previously admitted.
func DefaultEnvironmentAllowList() []IPAllowListEntry {
	return []IPAllowListEntry{
		{CIDRBlock: "0.0.0.0/0", Description: "Allow all (default)"},
		{CIDRBlock: "::/0", Description: "Allow all (default)"},
	}
}

// EnvironmentLayerCIDRs projects an environment's rule entries onto the flat
// CIDR list member CRs carry (spec.environmentIPAllowList, w4/m28): CIDRs
// only — descriptions stay on the environment row — with an explicitly empty
// rule set projected as the DenyAllCIDR placeholder. A nil result never comes
// from here; clearing the layer (leaving the environment) is the caller
// passing nil through, not this projection.
func EnvironmentLayerCIDRs(entries []IPAllowListEntry) []string {
	if len(entries) == 0 {
		return []string{DenyAllCIDR}
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.CIDRBlock
	}
	return out
}

// ValidateAllowList is ValidateCIDRs over entry CIDRs — descriptions are never
// validated (free text) and never influence enforcement.
func ValidateAllowList(entries []IPAllowListEntry) error {
	return ValidateCIDRs(AllowListCIDRs(entries))
}
