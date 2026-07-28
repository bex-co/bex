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

package v1alpha1

// History, deliberately detached from the type's godoc so it never reaches
// the generated CRD description: a pre-m24 CR stored the list as bare CIDR
// strings, decoded by a custom UnmarshalJSON union while the CRD field was
// Schemaless. w4/m29 normalized the CR fleet and retired the decoder; the
// structural schema (required cidr) now rejects a bare string at admission.
// The legacy shape survives only as a test fixture
// (ipallowlist_types_test.go). w1/m56 later retired the backend wire/store
// decoder after normalizing that fleet too.

// IPAllowEntry is one ipAllowList item on an App/Database/KeyValue spec: the
// CIDR the operator enforces plus an optional human-facing description that
// rides along untouched (Render's {cidrBlock, description} pairs, stored
// product-neutrally as {cidr, description}). Enforcement reads CIDR only.
type IPAllowEntry struct {
	// CIDR is the source range this entry allows (e.g. "10.0.0.0/8").
	CIDR string `json:"cidr"`

	// Description is the operator-facing label a human gave this entry.
	// Never read by enforcement.
	// +optional
	Description string `json:"description,omitempty"`
}

// EffectiveIPAllowListEntries returns the service allowlist in its structured
// form. New writes use IPAllowListEntries; legacy Apps created before w9/m56
// carry only IPAllowList and are lifted with empty descriptions. The returned
// slice never aliases the spec.
func (s AppSpec) EffectiveIPAllowListEntries() []IPAllowEntry {
	if len(s.IPAllowListEntries) > 0 {
		return append([]IPAllowEntry(nil), s.IPAllowListEntries...)
	}
	if len(s.IPAllowList) == 0 {
		return nil
	}
	out := make([]IPAllowEntry, len(s.IPAllowList))
	for i, cidr := range s.IPAllowList {
		out[i] = IPAllowEntry{CIDR: cidr}
	}
	return out
}

// EffectiveIPAllowListCIDRs projects the service allowlist down to the values
// enforced by Traefik. Descriptions are metadata and never reach middleware.
func (s AppSpec) EffectiveIPAllowListCIDRs() []string {
	entries := s.EffectiveIPAllowListEntries()
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.CIDR
	}
	return out
}

// SetIPAllowListEntries replaces a service allowlist using the structured
// shape. It always clears the legacy flat field, including on an empty write,
// so clearing a migrated App cannot resurrect stale legacy CIDRs.
func (s *AppSpec) SetIPAllowListEntries(entries []IPAllowEntry) {
	s.IPAllowList = nil
	if len(entries) == 0 {
		s.IPAllowListEntries = nil
		return
	}
	s.IPAllowListEntries = append([]IPAllowEntry(nil), entries...)
}
