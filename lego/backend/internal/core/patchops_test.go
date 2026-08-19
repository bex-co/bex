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
	"errors"
	"slices"
	"testing"
)

// The three w1/m71 update_* tools share this spine, so its contract is asserted
// once here rather than three times through MCP transports.

func TestPatchOpsRunsOnlyPresentOpsInOrder(t *testing.T) {
	var ran []string
	var ops PatchOps[string]
	ops.Add(true, func() (string, error) { ran = append(ran, "first"); return "first", nil })
	ops.Add(false, func() (string, error) { ran = append(ran, "skipped"); return "skipped", nil })
	ops.Add(true, func() (string, error) { ran = append(ran, "last"); return "last", nil })

	got, err := ops.Run(func() (string, error) { return "current", nil })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !slices.Equal(ran, []string{"first", "last"}) {
		t.Errorf("ran = %v, want [first last] — an absent argument must not write", ran)
	}
	if got != "last" {
		t.Errorf("Run = %q, want the last op's result", got)
	}
}

func TestPatchOpsWithNoOpsReflectsCurrentState(t *testing.T) {
	var ops PatchOps[string]
	ops.Add(false, func() (string, error) { t.Fatal("an absent argument must not run its op"); return "", nil })

	got, err := ops.Run(func() (string, error) { return "current", nil })
	if err != nil || got != "current" {
		t.Fatalf("Run = %q, %v; want the read-only no-op to reflect current state", got, err)
	}
}

func TestPatchOpsStopsAtTheFirstFailure(t *testing.T) {
	boom := errors.New("rejected")
	var reached bool
	var ops PatchOps[string]
	ops.Add(true, func() (string, error) { return "", boom })
	ops.Add(true, func() (string, error) { reached = true; return "later", nil })

	got, err := ops.Run(func() (string, error) { return "current", nil })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the failing op's error", err)
	}
	if got != "" {
		t.Errorf("Run = %q, want the zero value on failure", got)
	}
	if reached {
		t.Error("a later op ran after one failed — a rejected patch must not half-apply")
	}
}

func TestIDListTreatsNullAndEmptyAlike(t *testing.T) {
	if got := IDList(nil); got == nil || len(got) != 0 {
		t.Errorf("IDList(nil) = %#v, want a non-nil empty slice", got)
	}
	var null []string
	if got := IDList(&null); got == nil || len(got) != 0 {
		t.Errorf("IDList(&nil) = %#v, want a non-nil empty slice", got)
	}
	ids := []string{"srv-a", "srv-b"}
	if got := IDList(&ids); !slices.Equal(got, ids) {
		t.Errorf("IDList(&ids) = %v, want %v", got, ids)
	}
}

func TestResolveAllowListPatchDistinguishesAbsentFromEmpty(t *testing.T) {
	got, err := ResolveAllowListPatch(nil, nil)
	if err != nil || got != nil {
		t.Fatalf("both absent = %v, %v; want nil (leave the allowlist unchanged)", got, err)
	}

	empty := []IPAllowListEntry{}
	got, err = ResolveAllowListPatch(&empty, nil)
	if err != nil || got == nil || len(*got) != 0 {
		t.Fatalf("explicit [] = %v, %v; want a non-nil empty list (clear it)", got, err)
	}

	cidrs := []string{"203.0.113.0/24"}
	got, err = ResolveAllowListPatch(nil, &cidrs)
	if err != nil || got == nil || len(*got) != 1 || (*got)[0].CIDRBlock != "203.0.113.0/24" {
		t.Fatalf("cidrs form = %v, %v", got, err)
	}

	entries := []IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}
	if _, err := ResolveAllowListPatch(&entries, &cidrs); err == nil {
		t.Error("conflicting forms must be rejected rather than silently dropping one")
	}

	// Equivalent dual input is accepted — the same rule ResolveAllowListInputs
	// applies for the adapters that expose both spellings.
	same := []IPAllowListEntry{{CIDRBlock: "203.0.113.0/24"}}
	if _, err := ResolveAllowListPatch(&same, &cidrs); err != nil {
		t.Errorf("equivalent dual input should be accepted: %v", err)
	}
}
