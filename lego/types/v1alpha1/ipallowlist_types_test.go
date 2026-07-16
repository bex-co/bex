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

import (
	"encoding/json"
	"reflect"
	"testing"
)

// legacyIPAllowListFixture documents the retired pre-m24 serialization: a
// bare CIDR string per entry. w4/m29 normalized every CR to the {cidr} object
// shape (scripts/ipallowlist-normalize.sh) and retired the UnmarshalJSON
// union decoder that accepted this fixture, so it must now FAIL to decode —
// the structural CRD schema likewise rejects it at admission. Kept as the
// record of what the normalizer rewrites, not as a supported input.
const legacyIPAllowListFixture = `{"ipAllowList": ["10.0.0.0/8"]}`

// TestIPAllowEntryLegacyStringsNoLongerDecode pins the w4/m29 retirement: the
// legacy bare-string shape is a decode error, never a silently empty entry.
func TestIPAllowEntryLegacyStringsNoLongerDecode(t *testing.T) {
	var spec DatabaseSpec
	if err := json.Unmarshal([]byte(legacyIPAllowListFixture), &spec); err == nil {
		t.Fatalf("legacy bare-string entry decoded to %+v, want an error — the union decoder was retired in w4/m29", spec.IPAllowList)
	}
}

// TestIPAllowEntryDecodesObjects pins the one supported shape: structured
// {cidr, description} objects (description optional).
func TestIPAllowEntryDecodesObjects(t *testing.T) {
	var spec DatabaseSpec
	raw := `{"ipAllowList": [{"cidr": "10.0.0.0/8"}, {"cidr": "203.0.113.0/24", "description": "office"}]}`
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal object entries: %v", err)
	}
	want := []IPAllowEntry{
		{CIDR: "10.0.0.0/8"},
		{CIDR: "203.0.113.0/24", Description: "office"},
	}
	if !reflect.DeepEqual(spec.IPAllowList, want) {
		t.Fatalf("decoded = %+v, want %+v", spec.IPAllowList, want)
	}
}

// TestIPAllowEntryRoundTrip proves a structured entry survives
// marshal → unmarshal unchanged (what a CR update writes back).
func TestIPAllowEntryRoundTrip(t *testing.T) {
	in := []IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "office"}, {CIDR: "192.0.2.0/24"}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out []IPAllowEntry
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip changed the list: %+v -> %+v", in, out)
	}
}
