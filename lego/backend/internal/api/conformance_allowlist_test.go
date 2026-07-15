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

package api

import "strings"

// conformance_allowlist_test.go — deliberate divergences between bex's REST
// responses and Render's OpenAPI response schemas (testdata/render-openapi.json).
//
// RULES:
//   - Each entry cites its ADR018 row (docs/ADR018-render-parity.md) so the
//     divergence can be tracked back to its deliberate decision.
//   - Remove an entry once bex achieves conformance for that field.
//   - A new entry requires a real ADR018 citation; adding one without a cited
//     row will be questioned at review.
//   - The `contains` field is a substring of the raw validation error string
//     produced by renderSpec.walk — keep it precise enough to avoid masking
//     unrelated future errors in the same operation.
//
// To see which divergences are currently active, run:
//
//	cd lego/backend && go test ./internal/api/... -run TestRenderConformance -v

// conformanceDivergence is one known divergence entry.
type conformanceDivergence struct {
	contains string // substring match against the validation error
	adr018   string // docs/ADR018-render-parity.md row that documents this
}

// conformanceAllowlist maps Render operationId to the list of known divergences
// whose validation errors are suppressed during TestRenderConformance.
var conformanceAllowlist = map[string][]conformanceDivergence{
	// bex's GET /v1/postgres returns a flat []PostgresView (each item is a
	// Postgres object directly). Render's list-postgres-databases returns
	// [{postgres:{…}, cursor:"…"}] — a cursor-envelope array. The structural
	// mismatch means each item is missing the "postgres" wrapper and the
	// per-item "cursor" field.
	// ADR018 §Postgres — REST column: "list returns flat array (no cursor envelope)".
	"list-postgres-databases": {
		{
			contains: `missing required field "postgres"`,
			adr018:   "ADR018 §Postgres REST: bex list-postgres returns []PostgresView (flat); Render returns [{postgres:{}, cursor}] (cursor-envelope)",
		},
		{
			contains: `missing required field "cursor"`,
			adr018:   "ADR018 §Postgres REST: flat array has no per-item cursor field",
		},
	},

	// bex's GET /v1/key-value returns a flat []KeyValueView. Render's list-redis
	// returns [{redis:{…}, cursor:"…"}] — a cursor-envelope array. Same rationale
	// as postgres above.
	// ADR018 §Key Value — REST column: "list returns flat array (no cursor envelope)".
	"list-redis": {
		{
			contains: `missing required field "redis"`,
			adr018:   "ADR018 §Key Value REST: bex list-key-value returns []KeyValueView (flat); Render returns [{redis:{}, cursor}] (cursor-envelope)",
		},
		{
			contains: `missing required field "cursor"`,
			adr018:   "ADR018 §Key Value REST: flat array has no per-item cursor field",
		},
	},
}

// isAllowed returns true if errStr is covered by an allowlist entry for operationID.
func isAllowed(operationID, errStr string) bool {
	for _, d := range conformanceAllowlist[operationID] {
		if strings.Contains(errStr, d.contains) {
			return true
		}
	}
	return false
}

// filterAllowed returns the subset of errs that are NOT covered by the allowlist —
// the errors that should actually fail the build.
func filterAllowed(operationID string, errs []string) []string {
	var out []string
	for _, e := range errs {
		if !isAllowed(operationID, e) {
			out = append(out, e)
		}
	}
	return out
}
