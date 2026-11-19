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
	"slices"
	"strings"
	"time"
)

// FreePlanID is the plan under the one-free-instance rule (w4/m241): a
// workspace holds at most one free Postgres database and one free Key Value
// store, each kind tracked separately. The Postgres and Key Value services
// each spell this literal in their admission gates; the rule itself matches
// on it here so the two copies cannot drift apart.
const FreePlanID = "free"

// FreeSlotInstance is the one-free-slot rule's view of one datastore CR: the
// fields the rule reads, projected by each service from its own CR type (the
// types differ, so the projection stays per service; the rule below is
// shared).
type FreeSlotInstance struct {
	// Name is the CR name: the exclusion identity and the race identity.
	Name string
	// DisplayName is the user-facing name the refusal reports (Spec.Name,
	// falling back to the CR name).
	DisplayName string
	Plan        string
	CreatedAt   time.Time
	Terminating bool
}

// CollectFreeSlots projects one list page onto the rule, keeping list order.
// view maps each item to its slot view; returning false skips the item (a
// service reports another tenant's items that way).
func CollectFreeSlots[T any](items []T, view func(T) (FreeSlotInstance, bool)) []FreeSlotInstance {
	var out []FreeSlotInstance
	for _, item := range items {
		if slot, ok := view(item); ok {
			out = append(out, slot)
		}
	}
	return out
}

// FreeSlotHolder returns the display name (and mid-deletion state) of the
// instance holding the workspace's free slot: the first free-plan instance in
// list order, excluding excludeID (w4/m241, w6/m129). It reports "", false
// when the slot is open.
func FreeSlotHolder(slots []FreeSlotInstance, excludeID string) (string, bool) {
	for _, slot := range slots {
		if slot.Name == excludeID || slot.Plan != FreePlanID {
			continue
		}
		return slot.DisplayName, slot.Terminating
	}
	return "", false
}

// RefuseSecondFree refuses a free-plan write when the workspace's free slot
// is taken — the shared admission every writer runs.
func RefuseSecondFree(slots []FreeSlotInstance, excludeID, noun string) error {
	if name, terminating := FreeSlotHolder(slots, excludeID); name != "" {
		return FreeInstanceLimitError(noun, name, terminating)
	}
	return nil
}

// ResolveFreeSlotRace closes the create race the pre-check cannot (two free
// creates passing it concurrently): the earliest free instance (creation
// time, then name) keeps the slot and a later one loses. It reports the
// keeper and whether mine (by CR name) keeps it; an uncontended slot is
// always kept.
func ResolveFreeSlotRace(slots []FreeSlotInstance, mine string) (keeper FreeSlotInstance, mineKeeps bool) {
	var free []FreeSlotInstance
	for _, slot := range slots {
		if slot.Plan == FreePlanID {
			free = append(free, slot)
		}
	}
	if len(free) <= 1 {
		return FreeSlotInstance{}, true
	}
	slices.SortFunc(free, func(a, b FreeSlotInstance) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	if free[0].Name == mine {
		return free[0], true
	}
	return free[0], false
}
