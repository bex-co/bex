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
	// bex's GET /v1/key-value wraps each item as {keyValue:{…}, cursor:"…"},
	// matching the field name the OFFICIAL render CLI's generated client
	// expects (render-oss/cli pkg/client: KeyValueWithCursor.KeyValue,
	// json:"keyValue") — verified by driving the real CLI (v2.21.0) against a
	// live bex-api. testdata/render-openapi.json is a vendored, dated snapshot
	// that still carries the product's pre-rename schema name ("redis"); the
	// CLI's own generated types are the ones actually exercised at runtime, so
	// bex follows them over the stale spec. Revisit if a refreshed OpenAPI
	// snapshot confirms Render renamed the wire field too.
	// ADR018 §Key Value — REST column: "list envelope key is keyValue, not redis (CLI-verified; spec snapshot predates the rename)".
	"list-redis": {
		{
			contains: `missing required field "redis"`,
			adr018:   "ADR018 §Key Value REST: bex list-key-value wraps items as {keyValue:{}, cursor} (CLI-verified); the vendored spec's stale schema name is \"redis\"",
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
