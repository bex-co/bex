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

// TestIPAllowEntryDecodesBothShapes pins the backward-compatibility contract:
// a pre-m24 CR serialized its allow list as bare CIDR strings, and that shape
// must keep decoding (with empty descriptions) alongside the structured
// {cidr, description} form — in the same list, since a legacy CR partially
// updated by a new writer can hold both.
func TestIPAllowEntryDecodesBothShapes(t *testing.T) {
	var spec DatabaseSpec
	raw := `{"ipAllowList": ["10.0.0.0/8", {"cidr": "203.0.113.0/24", "description": "office"}]}`
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal mixed shapes: %v", err)
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
