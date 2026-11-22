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

// RefuseFreeSlotClaims judges a batch of writes (one Blueprint sync) by the
// free slot the whole batch leaves, never one write at a time, so the file's
// order cannot change the verdict. slots are the workspace's instances now;
// vacating reports, by CR name, each instance the batch moves off the free
// plan; claims are, by display name in apply order, the batch's writes that
// leave an instance on the free plan it is not on now (a create, or a move
// onto it). An instance that stays free keeps the slot; otherwise the first
// claim takes it, and every later claim is refused naming the holder. It
// returns each claim's refusal, nil where the claim is admitted.
func RefuseFreeSlotClaims(slots []FreeSlotInstance, vacating func(name string) bool, claims []string, noun string) []error {
	holder, terminating := "", false
	for _, slot := range slots {
		if slot.Plan == FreePlanID && !vacating(slot.Name) {
			holder, terminating = slot.DisplayName, slot.Terminating
			break
		}
	}
	refusals := make([]error, len(claims))
	for i, claim := range claims {
		if holder != "" {
			refusals[i] = FreeInstanceLimitError(noun, holder, terminating)
			continue
		}
		holder = claim
	}
	return refusals
}

// ResolveFreeSlotRace is a free create's fallback where no tenant lock
// serialized it (a resolver that offers none, WithFreeSlotLock): once the CR
// has landed, mine keeps the slot only when every other free instance was
// created strictly after it. Creation times have second precision, and CR
// names minted on different replicas do not order by time, so an equal time
// orders nothing and whoever sees the tie yields. A create that listed before
// the other landed saw only itself and keeps the slot; the other then sees the
// tie and yields. Two creates that each see the other both yield, which leaves
// the slot open rather than held twice. A create missing from the list counts
// as the newest. It reports the instance mine yields to (the earliest other,
// then by name) and whether mine keeps the slot; an uncontended slot is always
// kept.
func ResolveFreeSlotRace(slots []FreeSlotInstance, mine string) (keeper FreeSlotInstance, mineKeeps bool) {
	var others []FreeSlotInstance
	var created time.Time
	listed := false
	for _, slot := range slots {
		switch {
		case slot.Plan != FreePlanID:
		case slot.Name == mine:
			created, listed = slot.CreatedAt, true
		default:
			others = append(others, slot)
		}
	}
	if len(others) == 0 {
		return FreeSlotInstance{}, true
	}
	earliest := slices.MinFunc(others, func(a, b FreeSlotInstance) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	if listed && earliest.CreatedAt.After(created) {
		return FreeSlotInstance{}, true
	}
	return earliest, false
}
