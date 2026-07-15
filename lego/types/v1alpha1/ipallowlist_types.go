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

import "encoding/json"

// IPAllowEntry is one ipAllowList item on a Database/KeyValue spec: the CIDR
// the operator enforces plus an optional human-facing description that rides
// along untouched (Render's {cidrBlock, description} pairs, stored
// product-neutrally as {cidr, description}). Enforcement reads CIDR only
// (the operator's ipAllowListMiddlewareSpec).
//
// A pre-m24 CR stored the list as bare CIDR strings; UnmarshalJSON still
// accepts that shape (description empty), so legacy CRs decode without any
// migration. The field carrying this type is marked Schemaless in the CRD
// (both serializations must validate), so writers are responsible for
// validating the CIDR before it lands in a spec (bex-api's
// core.ValidateCIDRs gate).
type IPAllowEntry struct {
	// CIDR is the source range this entry allows (e.g. "10.0.0.0/8").
	CIDR string `json:"cidr"`

	// Description is the operator-facing label a human gave this entry.
	// Never read by enforcement.
	// +optional
	Description string `json:"description,omitempty"`
}

// UnmarshalJSON accepts both the structured {cidr, description} object and the
// legacy bare-string serialization ("10.0.0.0/8") a pre-m24 CR carries.
// The backend's core.IPAllowListEntry.UnmarshalJSON is this decoder's
// wire-side twin (same union semantics over the {cidrBlock} key) — keep the
// two behaviorally identical.
func (e *IPAllowEntry) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		e.Description = ""
		return json.Unmarshal(data, &e.CIDR)
	}
	// A local alias drops the method set so the object form decodes without
	// recursing into this method.
	type entry IPAllowEntry
	var v entry
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*e = IPAllowEntry(v)
	return nil
}
