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

package apps

import (
	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintDatabasePlanAfter is the plan the apply leaves db on: a create's
// declared (or default) plan, an existing database's declared plan, or the
// plan it has when its entry omits plan (ApplyBlueprintDatabaseSpec).
func blueprintDatabasePlanAfter(db parsedDatabase, live *appv1alpha1.Database) string {
	if live == nil {
		return db.spec.Plan
	}
	if _, declared := db.fields["plan"]; declared {
		return db.spec.Plan
	}
	return live.Spec.Plan
}

// blueprintVacatesFreeSlot reports whether the apply moves the existing
// database live off the free plan.
func blueprintVacatesFreeSlot(db parsedDatabase, live *appv1alpha1.Database) bool {
	return live != nil && live.Spec.Plan == core.FreePlanID && blueprintDatabasePlanAfter(db, live) != core.FreePlanID
}

// blueprintFreeDatabaseRefusals judges the one-free-Postgres rule (w4/m241)
// on the workspace a Blueprint sync leaves, as Render's "only one Free Render
// Postgres database can be active for any given workspace" reads: every
// database the file declares on the plan the apply leaves it on, every other
// one as it is. A database the file creates on free or moves onto it claims
// the slot. A free database that stays free holds it, so a claim beside it
// is refused; otherwise the first claim in file order takes it and every
// later one is refused. A database the file moves off free gives the slot up,
// so a file that hands it from one database to another is admitted in either
// order. live finds the existing database a declared name applies to; slots
// are the workspace's databases, deleting ones included. It returns each
// refused claim's error by declared name.
func blueprintFreeDatabaseRefusals(databases []parsedDatabase, live func(name string) *appv1alpha1.Database, slots []core.FreeSlotInstance) map[string]error {
	vacating := map[string]bool{}
	var claims []string
	for _, db := range databases {
		existing := live(db.name)
		switch {
		case blueprintVacatesFreeSlot(db, existing):
			vacating[existing.Name] = true
		case blueprintDatabasePlanAfter(db, existing) == core.FreePlanID && (existing == nil || existing.Spec.Plan != core.FreePlanID):
			claims = append(claims, db.name)
		}
	}
	if len(claims) == 0 {
		return nil
	}
	refusals := map[string]error{}
	for i, err := range core.RefuseFreeSlotClaims(slots, func(name string) bool { return vacating[name] }, claims, "Postgres database") {
		if err != nil {
			refusals[claims[i]] = err
		}
	}
	return refusals
}

// blueprintKeyValuePlanAfter is the plan the apply leaves kv on: a create's
// declared (or default) plan, an existing store's declared plan, or the plan
// it has when its entry omits plan (ApplyBlueprintKeyValueSpec).
func blueprintKeyValuePlanAfter(kv parsedKeyValue, live *appv1alpha1.KeyValue) string {
	if live == nil {
		return kv.spec.Plan
	}
	if _, declared := kv.fields["plan"]; declared {
		return kv.spec.Plan
	}
	return live.Spec.Plan
}

// blueprintKeyValueVacatesFreeSlot reports whether the apply moves the
// existing store live off the free plan.
func blueprintKeyValueVacatesFreeSlot(kv parsedKeyValue, live *appv1alpha1.KeyValue) bool {
	return live != nil && live.Spec.Plan == core.FreePlanID && blueprintKeyValuePlanAfter(kv, live) != core.FreePlanID
}

// blueprintFreeKeyValueRefusals judges the one-free-Key-Value rule (w4/m241)
// on the workspace a Blueprint sync leaves, as Render's "only one Free Key
// Value can be active for any given workspace" reads: every store the file
// declares on the plan the apply leaves it on, every other one as it is. A
// store the file creates on free or moves onto it claims the slot. A free
// store that stays free holds it, so a claim beside it is refused; otherwise
// the first claim in file order takes it and every later one is refused. A
// store the file moves off free gives the slot up, so a file that hands it
// from one store to another is admitted in either order. live finds the
// existing store a declared name applies to; slots are the workspace's
// stores, deleting ones included. It returns each refused claim's error by
// declared name.
func blueprintFreeKeyValueRefusals(keyValues []parsedKeyValue, live func(name string) *appv1alpha1.KeyValue, slots []core.FreeSlotInstance) map[string]error {
	vacating := map[string]bool{}
	var claims []string
	for _, kv := range keyValues {
		existing := live(kv.name)
		switch {
		case blueprintKeyValueVacatesFreeSlot(kv, existing):
			vacating[existing.Name] = true
		case blueprintKeyValuePlanAfter(kv, existing) == core.FreePlanID && (existing == nil || existing.Spec.Plan != core.FreePlanID):
			claims = append(claims, kv.name)
		}
	}
	if len(claims) == 0 {
		return nil
	}
	refusals := map[string]error{}
	for i, err := range core.RefuseFreeSlotClaims(slots, func(name string) bool { return vacating[name] }, claims, "Key Value store") {
		if err != nil {
			refusals[claims[i]] = err
		}
	}
	return refusals
}

// blueprintKeyValueApplyOrder is the order the apply writes the file's
// stores in, by index: each one the sync moves off the free plan first, then
// the rest in file order. A sync that hands the free slot from one store to
// another then frees it before it claims it, whatever the file order, so the
// claim's own check under the tenant lock admits it, and the workspace never
// holds two free stores mid-sync.
func blueprintKeyValueApplyOrder(keyValues []parsedKeyValue, live func(name string) *appv1alpha1.KeyValue) []int {
	order := make([]int, 0, len(keyValues))
	var rest []int
	for i, kv := range keyValues {
		if blueprintKeyValueVacatesFreeSlot(kv, live(kv.name)) {
			order = append(order, i)
		} else {
			rest = append(rest, i)
		}
	}
	return append(order, rest...)
}

// blueprintDatabaseApplyOrder is the order the apply writes the file's
// databases in, by index: each one the sync moves off the free plan first,
// then the rest in file order. A sync that hands the free slot from one
// database to another then frees it before it claims it, whatever the file
// order, so the claim's own check under the tenant lock admits it, and the
// workspace never holds two free databases mid-sync.
func blueprintDatabaseApplyOrder(databases []parsedDatabase, live func(name string) *appv1alpha1.Database) []int {
	order := make([]int, 0, len(databases))
	var rest []int
	for i, db := range databases {
		if blueprintVacatesFreeSlot(db, live(db.name)) {
			order = append(order, i)
		} else {
			rest = append(rest, i)
		}
	}
	return append(order, rest...)
}
