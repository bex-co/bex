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
	"net/netip"
	"slices"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ValidateCIDRs rejects any entry that is not a valid CIDR with ErrBadRequest —
// the shared gate for every ipAllowList write path (postgres + keyvalue +
// environments, create and set alike), so a bad entry is a 400 before anything
// lands in a CR spec or store row.
//
// It parses with netip.ParsePrefix, the exact grammar the pg-sni-proxy /
// kv-sni-proxy enforcement path uses (internal/sniproxy/allowlist.go). The two
// parsers must agree: net.ParseCIDR accepts zero-padded prefix lengths like
// "10.0.0.0/024" that netip.ParsePrefix rejects, so validating with the looser
// parser would admit a value that is unparseable at enforcement time — which
// deletes the route and pins the shared proxy health gauge to unhealthy for
// every tenant (round-21 finding 8).
func ValidateCIDRs(cidrs []string) error {
	if len(cidrs) > MaxAllowListEntries {
		return fmt.Errorf("%w: ipAllowList has %d entries; the limit is %d", ErrBadRequest, len(cidrs), MaxAllowListEntries)
	}
	for _, c := range cidrs {
		if _, err := netip.ParsePrefix(c); err != nil {
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

// UnrestrictedAllowList is the rule pair that admits every source of both
// address families. It carries no descriptions: it is how every Postgres
// create (w2/038, w8/157, w4/553) saves an omitted list, and how every read
// spells a public datastore with no saved rules (EffectiveAllowList).
func UnrestrictedAllowList() []IPAllowListEntry {
	return []IPAllowListEntry{{CIDRBlock: "0.0.0.0/0"}, {CIDRBlock: "::/0"}}
}

// ServiceDefaultAllowList is the rule a web service or static site nobody has
// restricted reads: Render's default, one entry, 0.0.0.0/0 "everywhere"
// (w8/284).
func ServiceDefaultAllowList() []IPAllowListEntry {
	return []IPAllowListEntry{{CIDRBlock: "0.0.0.0/0", Description: "everywhere"}}
}

// ServiceAllowListFromCIDRs is a service's rule set written in the flat CIDR
// form, which reads ["0.0.0.0/0"] for a service nobody restricted. That form
// cannot carry descriptions, so writing back what it read is Render's default
// rule, not a bare 0.0.0.0/0 saved as written (w8/284).
func ServiceAllowListFromCIDRs(cidrs []string) []IPAllowListEntry {
	if entries := AllowListFromCIDRs(cidrs); !slices.Equal(AllowListCIDRs(entries), AllowListCIDRs(ServiceDefaultAllowList())) {
		return entries
	}
	return ServiceDefaultAllowList()
}

// EffectiveAllowList is a datastore's external access as every read reports
// it: REST, GraphQL, MCP and the Blueprint export (w4/m315). Render has no
// public toggle, and its clients read an empty list as "external connections
// blocked", so the list alone has to say whether the store is reachable:
//
//   - private: [], whatever inactive rules it keeps;
//   - public with no saved rules: UnrestrictedAllowList;
//   - otherwise its saved rules.
//
// Never nil: Render's schema marks ipAllowList required, and a strict
// generated client treats a missing or null one as a decode failure (w6/m109).
// Writes are unchanged (a nonempty
// list publishes, an empty one unpublishes), so writing back what was read
// keeps access as it is.
func EffectiveAllowList(public bool, list []appv1alpha1.IPAllowEntry) []IPAllowListEntry {
	switch {
	case !public:
		return []IPAllowListEntry{}
	case len(list) == 0:
		return UnrestrictedAllowList()
	}
	return AllowListFromSpec(list)
}

// AllowListCIDRs projects entries down to their CIDR strings — the
// product-neutral string-list shape GraphQL/MCP's legacy arguments and the
// bex-native {"cidrs"} REST routes keep speaking.
//
// Empty in, EMPTY (never nil) out, for EffectiveAllowList's reason one level
// down: this is a read-side projection, and its only REST consumers wrap it in
// {"cidrs": …} — a nil would have made a private datastore answer
// {"cidrs":null} where an array is declared (w4/m116/t006). The nested field is
// past core.WriteJSON's nil-slice guard, so the normalization has to happen at
// the one place both datastores' handlers share: here.
func AllowListCIDRs(entries []IPAllowListEntry) []string {
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

// ProvidedAllowListOrCIDRs is AllowListOrCIDRs for a create that must keep an
// explicitly empty list (no rules) distinct from an omitted one (nil): the
// flat form alone folds an empty argument into nil.
func ProvidedAllowListOrCIDRs(entries []IPAllowListEntry, cidrs []string) []IPAllowListEntry {
	if entries == nil && cidrs == nil {
		return nil
	}
	return ProvidedAllowList(AllowListOrCIDRs(entries, cidrs))
}

// ResolveAllowListInputs resolves adapters that expose both the structured
// and legacy flat forms. One provided form is used directly; equivalent dual
// inputs are accepted; conflicting dual inputs are a bad request rather than
// silently dropping descriptions or CIDRs.
func ResolveAllowListInputs(entries []IPAllowListEntry, entriesSet bool, cidrs []string, cidrsSet bool) ([]IPAllowListEntry, error) {
	switch {
	case !entriesSet && !cidrsSet:
		return nil, nil
	case !entriesSet:
		return ProvidedAllowList(AllowListFromCIDRs(cidrs)), nil
	case !cidrsSet:
		return ProvidedAllowList(entries), nil
	}
	legacy := AllowListFromCIDRs(cidrs)
	if !slices.Equal(entries, legacy) {
		return nil, fmt.Errorf("%w: ipAllowList and ipAllowListEntries are set to conflicting values", ErrBadRequest)
	}
	return ProvidedAllowList(entries), nil
}

// ResolveAllowListPatch is ResolveAllowListInputs in the shape a patch-shaped
// MCP tool needs (w1/m71): both spellings arrive as pointers, where nil means
// "the caller did not mention the allowlist" and a non-nil pointer means
// "replace it with exactly this, empty included". It returns nil when neither
// was given, so the caller can feed it straight into a *[]IPAllowListEntry
// patch field whose nil already means unchanged.
//
// Every folded update_* tool resolves the pair through here rather than
// repeating the unwrap, so the three surfaces cannot drift on what "both forms
// present" means: equivalent is accepted, conflicting is a bad request.
func ResolveAllowListPatch(entries *[]IPAllowListEntry, cidrs *[]string) (*[]IPAllowListEntry, error) {
	if entries == nil && cidrs == nil {
		return nil, nil
	}
	var entryList []IPAllowListEntry
	if entries != nil {
		entryList = *entries
	}
	var cidrList []string
	if cidrs != nil {
		cidrList = *cidrs
	}
	resolved, err := ResolveAllowListInputs(entryList, entries != nil, cidrList, cidrs != nil)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

// DenyAllCIDR is the unmatchable placeholder the environment fan-out projects
// for an explicitly empty environment rule list (w4/m28, Render's
// empty-means-deny-all): Traefik rejects an ipAllowList middleware with an
// empty sourceRange, so deny-all must be a real range no source matches.
const DenyAllCIDR = "255.255.255.255/32"

// ServiceAllowListToSpec is how a web service's or static site's inbound rule
// set is saved (w8/m157). Render: "If you delete all rules, Render blocks all
// inbound traffic to the affected service(s)", and its default is the single
// rule 0.0.0.0/0. A service's spec cannot tell an empty list from an absent
// one, so the two states are saved as different rules:
//
//   - an empty set is saved as the DenyAllCIDR placeholder, the rule the
//     environment layer projects for its own empty set. The operator already
//     turns any stored rule into the service's middleware, so the block holds
//     on a cluster whose operator predates this;
//   - the default a service reads (ServiceDefaultAllowList), or the
//     two-range allow-all rule it read before w8/284, saves no rule, so a
//     client that writes back what it read changes nothing;
//   - anything else is saved as written, descriptions included, so it reads
//     back as written, as on Render (w8/284). A lone 0.0.0.0/0 then admits
//     IPv4 sources only; no service host is reachable over IPv6 today.
//
// Callers pass a set the request carried. An omitted field changes nothing and
// never reaches here.
func ServiceAllowListToSpec(entries []IPAllowListEntry) []appv1alpha1.IPAllowEntry {
	switch {
	case len(entries) == 0:
		return []appv1alpha1.IPAllowEntry{{CIDR: DenyAllCIDR}}
	case slices.Equal(entries, ServiceDefaultAllowList()), slices.Equal(entries, UnrestrictedAllowList()):
		return nil
	}
	return AllowListToSpec(entries)
}

// ServiceAllowList is a service's inbound rule set as every read reports it:
// REST, GraphQL, MCP and the Blueprint export (w8/m157).
//
//   - blocked (the DenyAllCIDR placeholder alone): [];
//   - a publicly routable service with no saved rule: Render's default,
//     ServiceDefaultAllowList (w8/284);
//   - otherwise its saved rules, nil for a type with no public endpoint.
//
// Writing back what was read keeps access as it is.
func ServiceAllowList(routable bool, list []appv1alpha1.IPAllowEntry) []IPAllowListEntry {
	switch {
	case ServiceBlocksAll(list):
		return []IPAllowListEntry{}
	case len(list) == 0 && routable:
		return ServiceDefaultAllowList()
	}
	return AllowListFromSpec(list)
}

// ServiceBlocksAll reports whether a service's saved rule set is the one an
// explicitly empty list is saved as.
func ServiceBlocksAll(list []appv1alpha1.IPAllowEntry) bool {
	return len(list) == 1 && list[0].CIDR == DenyAllCIDR
}

// ProvidedAllowList marks a rule set a request carried: never nil, so an
// explicitly empty list stays distinct from an omitted field (nil) on its way
// to the service layer, where the two mean "block" and "leave as it is".
func ProvidedAllowList(entries []IPAllowListEntry) []IPAllowListEntry {
	if entries == nil {
		return []IPAllowListEntry{}
	}
	return entries
}

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
