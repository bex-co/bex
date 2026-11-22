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
	"strings"
	"testing"
	"time"
)

func freeSlot(name, display, plan string, created time.Time, terminating bool) FreeSlotInstance {
	return FreeSlotInstance{Name: name, DisplayName: display, Plan: plan, CreatedAt: created, Terminating: terminating}
}

func TestFreeSlotHolderTakesFirstFreeInListOrder(t *testing.T) {
	now := time.Now()
	slots := []FreeSlotInstance{
		freeSlot("dpg-a", "a", "free", now, false),
		freeSlot("dpg-b", "b", "free", now.Add(time.Minute), false),
	}
	name, terminating := FreeSlotHolder(slots, "")
	if name != "a" || terminating {
		t.Fatalf("holder = %q, %v; want a, false (list order, not creation order)", name, terminating)
	}
}

func TestFreeSlotHolderSkipsExcludedPaidAndOthersSlots(t *testing.T) {
	now := time.Now()
	slots := []FreeSlotInstance{
		freeSlot("dpg-mine", "mine", "free", now, false),
		freeSlot("dpg-paid", "paid", "basic-256mb", now, false),
		freeSlot("dpg-yours", "yours", "free", now, true),
	}
	if name, _ := FreeSlotHolder(slots, ""); name != "mine" {
		t.Fatalf("holder = %q; want mine", name)
	}
	if name, terminating := FreeSlotHolder(slots, "dpg-mine"); name != "yours" || !terminating {
		t.Fatalf("holder excluding mine = %q, %v; want yours, true", name, terminating)
	}
	if name, _ := FreeSlotHolder(slots[:1], "dpg-mine"); name != "" {
		t.Fatalf("holder = %q; want open slot", name)
	}
}

func TestRefuseSecondFreeNamesHolderAndSlotState(t *testing.T) {
	now := time.Now()
	err := RefuseSecondFree([]FreeSlotInstance{freeSlot("dpg-a", "a", "free", now, false)}, "", "Postgres database")
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "already has a free Postgres database (a)") {
		t.Fatalf("refusal = %v; want FREE_INSTANCE_LIMIT naming a", err)
	}
	err = RefuseSecondFree([]FreeSlotInstance{freeSlot("dpg-a", "a", "free", now, true)}, "", "Key Value store")
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "still being deleted") {
		t.Fatalf("terminating refusal = %v; want still-being-deleted", err)
	}
	if err := RefuseSecondFree(nil, "", "Postgres database"); err != nil {
		t.Fatalf("open slot => %v, want admitted", err)
	}
}

func TestResolveFreeSlotRaceEarliestKeepsSlot(t *testing.T) {
	now := time.Now()
	early := freeSlot("dpg-early", "early", "free", now, false)
	late := freeSlot("dpg-late", "late", "free", now.Add(time.Hour), false)
	paid := freeSlot("dpg-paid", "paid", "basic-256mb", now.Add(-time.Hour), false)

	keeper, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{late, paid, early}, "dpg-late")
	if mineKeeps || keeper.DisplayName != "early" {
		t.Fatalf("race = keeper %q, mineKeeps %v; want keeper early, late loses", keeper.DisplayName, mineKeeps)
	}
	if _, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{late, paid, early}, "dpg-early"); !mineKeeps {
		t.Fatalf("earliest create must keep the slot")
	}
}

// Creation times have second precision and CR names minted on different
// replicas do not order by time, so a tie orders nothing: whoever sees it
// yields, whatever the names. A create that listed before the other landed saw
// only itself and kept the slot; the other must not keep it too.
func TestResolveFreeSlotRaceYieldsOnACreationTie(t *testing.T) {
	second := time.Now().Truncate(time.Second)
	first := freeSlot("dpg-z-first", "first", "free", second, false)
	later := freeSlot("dpg-a-later", "later", "free", second, false)
	if _, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{first}, "dpg-z-first"); !mineKeeps {
		t.Fatalf("a create that sees only itself must keep the slot")
	}
	keeper, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{later, first}, "dpg-a-later")
	if mineKeeps || keeper.DisplayName != "first" {
		t.Fatalf("tie seen by the name that sorts first = keeper %q, mineKeeps %v; want it to yield to first", keeper.DisplayName, mineKeeps)
	}
	if _, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{later, first}, "dpg-z-first"); mineKeeps {
		t.Fatalf("a create that sees a tie must yield too: two keeps would leave two free instances")
	}
}

func TestResolveFreeSlotRaceUnlistedCreateYields(t *testing.T) {
	other := freeSlot("dpg-other", "other", "free", time.Now(), false)
	if keeper, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{other}, "dpg-mine"); mineKeeps || keeper.DisplayName != "other" {
		t.Fatalf("create missing from its own list = keeper %q, mineKeeps %v; want it to yield to other", keeper.DisplayName, mineKeeps)
	}
}

func TestRefuseFreeSlotClaimsJudgesTheBatchEndState(t *testing.T) {
	now := time.Now()
	holder := freeSlot("dpg-1", "db1", "free", now, false)
	paid := freeSlot("dpg-2", "db2", "basic-256mb", now, false)
	none := func(string) bool { return false }
	refused := func(errs []error) []bool {
		out := make([]bool, len(errs))
		for i, err := range errs {
			out[i] = err != nil
		}
		return out
	}
	for _, tc := range []struct {
		name     string
		slots    []FreeSlotInstance
		vacating func(string) bool
		claims   []string
		want     []bool
		holder   string
	}{
		{"claim beside a free instance", []FreeSlotInstance{holder, paid}, none, []string{"db2"}, []bool{true}, "(db1)"},
		{"first of two claims takes the open slot", []FreeSlotInstance{paid}, none, []string{"db2", "db3"}, []bool{false, true}, "(db2)"},
		{"the holder moving off free opens the slot", []FreeSlotInstance{holder, paid}, func(name string) bool { return name == "dpg-1" }, []string{"db2"}, []bool{false}, ""},
		{"a deleting holder still holds it", []FreeSlotInstance{freeSlot("dpg-1", "db1", "free", now, true)}, none, []string{"db2"}, []bool{true}, "still being deleted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := RefuseFreeSlotClaims(tc.slots, tc.vacating, tc.claims, "Postgres database")
			if got := refused(errs); !slices.Equal(got, tc.want) {
				t.Fatalf("refused = %v, want %v (%v)", got, tc.want, errs)
			}
			for _, err := range errs {
				if err != nil && (!errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), tc.holder)) {
					t.Fatalf("refusal %v, want FREE_INSTANCE_LIMIT naming %s", err, tc.holder)
				}
			}
		})
	}
}

func TestResolveFreeSlotRaceUncontendedSlotIsKept(t *testing.T) {
	now := time.Now()
	only := []FreeSlotInstance{freeSlot("dpg-only", "only", "free", now, false)}
	if _, mineKeeps := ResolveFreeSlotRace(only, "dpg-only"); !mineKeeps {
		t.Fatalf("sole holder must keep the slot")
	}
	if _, mineKeeps := ResolveFreeSlotRace(nil, "dpg-new"); !mineKeeps {
		t.Fatalf("empty list must not delete the landed create")
	}
}
