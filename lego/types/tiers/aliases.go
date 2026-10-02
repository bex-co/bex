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

package tiers

// Input aliases map Render CLI/help spellings onto bex's catalog IDs (w8/011).
// Only unambiguous overlaps are accepted — no "nearest larger tier" guesses
// that would mis-size or misprice. Spec-based names with no bex rung stay 400.

// postgresInputAliases: Render's modern Nc-Ng / underscore legacy names that
// match bex's three Postgres rungs (free is already identical).
var postgresInputAliases = map[string]string{
	"0.1c-256mb":  "basic-256mb",
	"0.5c-1g":     "basic-1gb",
	"basic_256mb": "basic-256mb",
	"basic_1gb":   "basic-1gb",
}

// valkeyInputAliases: Render Key Value size spellings that match bex's
// starter/standard rungs (free/starter/standard ids already identical).
var valkeyInputAliases = map[string]string{
	"256mb": "starter",
	"1g":    "standard",
}

// computeInputAliases: Render's spec-based compute plan IDs (render.com/docs/
// blueprint-spec `plan`, the default for a new service since 2026-10 is
// 0.5c-512mb) whose CPU/RAM equal a bex compute rung exactly (tiers.yaml). The
// larger and odd-shaped sizes (2c-8g, 12c-96g, ...) have no bex rung and stay
// unknown (w1/m170).
var computeInputAliases = map[string]string{
	"0.5c-512mb": "starter",
	"1c-2g":      "standard",
	"2c-4g":      "pro",
	"4c-8g":      "pro-plus",
	"4c-16g":     "pro-max",
	"8c-32g":     "pro-ultra",
}

// CanonicalID maps a Postgres plan input (bex id or accepted Render alias) to
// the Database CRD's spec.plan spelling. Unknown inputs are returned unchanged
// so ByID can still reject them.
func (c PostgresCatalog) CanonicalID(id string) string {
	if id == "" {
		return id
	}
	if _, ok := c.byID[id]; ok {
		return id
	}
	if canon, ok := postgresInputAliases[id]; ok {
		return canon
	}
	return id
}

// CanonicalID maps a Key Value plan input (bex id or accepted Render alias) to
// the KeyValue CRD's spec.plan spelling.
func (c ValkeyCatalog) CanonicalID(id string) string {
	if id == "" {
		return id
	}
	if _, ok := c.byID[id]; ok {
		return id
	}
	if canon, ok := valkeyInputAliases[id]; ok {
		return canon
	}
	return id
}
