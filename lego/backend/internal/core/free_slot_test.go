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

func TestResolveFreeSlotRaceBreaksCreationTiesByName(t *testing.T) {
	now := time.Now()
	a := freeSlot("dpg-a", "a", "free", now, false)
	b := freeSlot("dpg-b", "b", "free", now, false)
	if _, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{b, a}, "dpg-b"); mineKeeps {
		t.Fatalf("tied creation times must break by name: b loses to a")
	}
	if _, mineKeeps := ResolveFreeSlotRace([]FreeSlotInstance{b, a}, "dpg-a"); !mineKeeps {
		t.Fatalf("tied creation times must break by name: a keeps the slot")
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
