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
	"encoding/json"
	"fmt"
	"net"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ValidateCIDRs rejects any entry that is not a valid CIDR with ErrBadRequest —
// the shared gate for every ipAllowList write path (postgres + keyvalue +
// environments, create and set alike), so a bad entry is a 400 before anything
// lands in a CR spec or store row.
func ValidateCIDRs(cidrs []string) error {
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

// UnmarshalJSON also accepts a bare CIDR string — the pre-m24 serialization
// still present in legacy environment store rows, and the lenient shape old
// bex clients sent on the wire before RC12/RC14 aligned it with Render's.
// Its CR-side twin (v1alpha1.IPAllowEntry.UnmarshalJSON) was retired in
// w4/m29 after the fleet normalization made the CRD structural again; this
// wire-side leniency is a deliberate surface-contract survivor, not a mirror
// of the CR.
func (e *IPAllowListEntry) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		e.Description = ""
		return json.Unmarshal(data, &e.CIDRBlock)
	}
	// A local alias drops the method set so the object form decodes without
	// recursing into this method.
	type entry IPAllowListEntry
	var v entry
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*e = IPAllowListEntry(v)
	return nil
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

// ValidateAllowList is ValidateCIDRs over entry CIDRs — descriptions are never
// validated (free text) and never influence enforcement.
func ValidateAllowList(entries []IPAllowListEntry) error {
	for _, e := range entries {
		if _, _, err := net.ParseCIDR(e.CIDRBlock); err != nil {
			return fmt.Errorf("%w: %q is not a valid CIDR", ErrBadRequest, e.CIDRBlock)
		}
	}
	return nil
}
