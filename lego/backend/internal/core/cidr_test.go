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
	"errors"
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestIPAllowListEntryDecodesBothShapes pins the wire/store compatibility
// contract: entries decode from Render's {cidrBlock, description} objects AND
// from bare CIDR strings (legacy environment store rows + old bex clients),
// mixed in one list, with descriptions empty for the string form.
func TestIPAllowListEntryDecodesBothShapes(t *testing.T) {
	var got []IPAllowListEntry
	raw := `["10.0.0.0/8", {"cidrBlock": "203.0.113.0/24", "description": "office"}]`
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal mixed shapes: %v", err)
	}
	want := []IPAllowListEntry{
		{CIDRBlock: "10.0.0.0/8"},
		{CIDRBlock: "203.0.113.0/24", Description: "office"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded = %+v, want %+v", got, want)
	}
}

// TestAllowListSpecConversionsRoundTrip proves wire → CR spec → wire preserves
// both fields, and that the CIDR/lift projections are inverses for the
// string-list surfaces.
func TestAllowListSpecConversionsRoundTrip(t *testing.T) {
	in := []IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}, {CIDRBlock: "192.0.2.0/24"}}
	spec := AllowListToSpec(in)
	if want := []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "office"}, {CIDR: "192.0.2.0/24"}}; !reflect.DeepEqual(spec, want) {
		t.Fatalf("AllowListToSpec = %+v, want %+v", spec, want)
	}
	if out := AllowListFromSpec(spec); !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip changed the list: %+v -> %+v", in, out)
	}
	cidrs := AllowListCIDRs(in)
	if !reflect.DeepEqual(cidrs, []string{"10.0.0.0/8", "192.0.2.0/24"}) {
		t.Fatalf("AllowListCIDRs = %v", cidrs)
	}
	lifted := AllowListFromCIDRs(cidrs)
	if !reflect.DeepEqual(lifted, []IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}, {CIDRBlock: "192.0.2.0/24"}}) {
		t.Fatalf("AllowListFromCIDRs = %+v", lifted)
	}
	// Empty stays nil on every projection — no fabricated empty entries.
	if AllowListToSpec(nil) != nil || AllowListFromSpec(nil) != nil || AllowListCIDRs(nil) != nil || AllowListFromCIDRs(nil) != nil {
		t.Fatal("nil list must project to nil everywhere")
	}
}

// TestValidateAllowListNamesTheBadEntry pins the failure mode: a malformed
// CIDR is ErrBadRequest and the error names the offending value, while the
// free-text description is never validated.
func TestValidateAllowListNamesTheBadEntry(t *testing.T) {
	err := ValidateAllowList([]IPAllowListEntry{
		{CIDRBlock: "10.0.0.0/8", Description: "fine"},
		{CIDRBlock: "not-a-cidr", Description: "still accepted as text"},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("want ErrBadRequest, got %v", err)
	}
	if !strings.Contains(err.Error(), `"not-a-cidr"`) {
		t.Fatalf("error must name the offending entry, got %q", err.Error())
	}
	if err := ValidateAllowList([]IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "any text at all — never validated"}}); err != nil {
		t.Fatalf("valid CIDR with free-text description must pass, got %v", err)
	}
}

func TestResolveAllowListInputs(t *testing.T) {
	entries := []IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}
	got, err := ResolveAllowListInputs(entries, true, nil, false)
	if err != nil || !reflect.DeepEqual(got, entries) {
		t.Fatalf("structured only = %+v, %v", got, err)
	}
	got, err = ResolveAllowListInputs(nil, false, []string{"192.0.2.0/24"}, true)
	if err != nil || !reflect.DeepEqual(got, []IPAllowListEntry{{CIDRBlock: "192.0.2.0/24"}}) {
		t.Fatalf("legacy only = %+v, %v", got, err)
	}
	got, err = ResolveAllowListInputs(
		[]IPAllowListEntry{{CIDRBlock: "192.0.2.0/24"}}, true,
		[]string{"192.0.2.0/24"}, true,
	)
	if err != nil || len(got) != 1 {
		t.Fatalf("equivalent dual inputs = %+v, %v", got, err)
	}
	_, err = ResolveAllowListInputs(entries, true, []string{"203.0.113.0/24"}, true)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("description-dropping dual input must conflict, got %v", err)
	}
	got, err = ResolveAllowListInputs([]IPAllowListEntry{}, true, nil, false)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("explicit structured clear = %#v, %v", got, err)
	}
}
