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
// Schemaless. w4/m29 normalized the fleet (scripts/ipallowlist-normalize.sh,
// verified clean) and retired the decoder; the structural schema (required
// cidr) now rejects a bare string at admission. The legacy shape survives
// only as a test fixture (ipallowlist_types_test.go). The backend's
// core.IPAllowListEntry keeps its own wire-side union decoder — that surface
// contract is unchanged.

// IPAllowEntry is one ipAllowList item on a Database/KeyValue spec: the CIDR
// the operator enforces plus an optional human-facing description that rides
// along untouched (Render's {cidrBlock, description} pairs, stored
// product-neutrally as {cidr, description}). Enforcement reads CIDR only
// (the operator's ipAllowListMiddlewareSpec).
type IPAllowEntry struct {
	// CIDR is the source range this entry allows (e.g. "10.0.0.0/8").
	CIDR string `json:"cidr"`

	// Description is the operator-facing label a human gave this entry.
	// Never read by enforcement.
	// +optional
	Description string `json:"description,omitempty"`
}
