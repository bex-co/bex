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
// responses and the complete pinned Render OpenAPI response schemas.
//
// RULES:
//   - Each entry cites its ADR018 row (docs/ADR018-render-parity.md) so the
//     divergence can be tracked back to its deliberate decision.
//   - Remove an entry once bex achieves conformance for that field.
//   - A new entry requires a real ADR018 citation; adding one without a cited
//     row will be questioned at review.
//   - The `contains` field is a substring of the raw validation error string
//     produced by kin-openapi — keep it precise enough to avoid masking
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
	// Render marks metadata and a larger serviceDetails projection required.
	// bex truthfully omits values that are not configured or known and exposes
	// its currently implemented details as a documented partial REST surface.
	// ADR018 §Services & lifecycle and §Resource metadata contract.
	"list-services": knownConformanceDivergences(
		"ADR018 §Services & lifecycle / §Resource metadata contract: optional truthful metadata and partial serviceDetails projection",
		`serviceDetails: does not match the documented union`,
		`ownerId: property "ownerId" is missing`,
		`dashboardUrl: property "dashboardUrl" is missing`,
		`updatedAt: property "updatedAt" is missing`,
		`rootDir: property "rootDir" is missing`,
	),
	"retrieve-service": knownConformanceDivergences(
		"ADR018 §Services & lifecycle / §Resource metadata contract: optional truthful metadata and partial serviceDetails projection",
		`/serviceDetails: does not match the documented union`,
		`/ownerId: property "ownerId" is missing`,
		`/dashboardUrl: property "dashboardUrl" is missing`,
		`/updatedAt: property "updatedAt" is missing`,
		`/rootDir: property "rootDir" is missing`,
	),

	// The datastore rows document bex's intentionally truthful omission of
	// provider/configuration metadata and the partial advanced-detail surface.
	"list-postgres": knownConformanceDivergences(
		"ADR018 §Managed Postgres / §Resource metadata contract: omitted unknown metadata and unsupported advanced provider fields",
		`ipAllowList: property "ipAllowList" is missing`,
		`updatedAt: property "updatedAt" is missing`,
		`owner: property "owner" is missing`,
		`region: property "region" is missing`,
		`readReplicas: property "readReplicas" is missing`,
		`role: property "role" is missing`,
		`version: property "version" is missing`,
		`suspenders: property "suspenders" is missing`,
		`dashboardUrl: property "dashboardUrl" is missing`,
		`connectionPool: property "connectionPool" is missing`,
	),
	"retrieve-postgres": knownConformanceDivergences(
		"ADR018 §Managed Postgres / §Resource metadata contract: omitted unknown metadata and unsupported advanced provider fields",
		`/ipAllowList: property "ipAllowList" is missing`,
		`/updatedAt: property "updatedAt" is missing`,
		`/dashboardUrl: property "dashboardUrl" is missing`,
		`/owner: property "owner" is missing`,
		`/project: property "project" is missing`,
		`/region: property "region" is missing`,
		`/readReplicas: property "readReplicas" is missing`,
		`/role: property "role" is missing`,
		`/version: property "version" is missing`,
		`/suspenders: property "suspenders" is missing`,
		`/connectionPool: property "connectionPool" is missing`,
	),
	"retrieve-redis": knownConformanceDivergences(
		// `version` was dropped from this list once the Key Value read began
		// reporting the effective provider version; the guard test fails an
		// allowlist entry that no longer matches a real divergence.
		"ADR018 §Managed Key Value / §Resource metadata contract: omitted unknown metadata",
		`/updatedAt: property "updatedAt" is missing`,
		`/region: property "region" is missing`,
		`/owner: property "owner" is missing`,
		`/ipAllowList: property "ipAllowList" is missing`,
	),

	// Secret-file list values are intentionally redacted; custom-domain status
	// reports bex's pending state and DNS instructions; event details are a
	// documented partial activity-feed projection.
	"list-secret-files-for-service": knownConformanceDivergences(
		"ADR018 §Environment variables & secret files: list returns names without secret contents",
		`secretFile/content: property "content" is missing`,
	),
	"list-custom-domains": knownConformanceDivergences(
		"ADR018 §Custom domains: bex exposes pending DNS-instruction state and omits unavailable provider metadata",
		`customDomain/verificationStatus: value is not one of the allowed values`,
		`customDomain/publicSuffix: property "publicSuffix" is missing`,
		`customDomain/createdAt: property "createdAt" is missing`,
		`customDomain/redirectForName: property "redirectForName" is missing`,
	),
	"list-events": knownConformanceDivergences(
		"ADR018 §Services & lifecycle — Service events / activity feed: deliberate partial event-detail dialect",
		`event/details: does not match the documented union`,
	),
	"retrieve-event": knownConformanceDivergences(
		"ADR018 §Services & lifecycle — Service events / activity feed: deliberate partial event-detail dialect",
		`details: does not match the documented union`,
	),

	// bex's GET /v1/key-value wraps each item as {keyValue:{…}, cursor:"…"},
	// matching the field name the OFFICIAL render CLI's generated client
	// expects (render-oss/cli pkg/client: KeyValueWithCursor.KeyValue,
	// json:"keyValue") — verified by driving the real CLI (v2.21.0) against a
	// live bex-api. Render's current public OpenAPI document still carries the
	// product's pre-rename schema name ("redis"); the
	// CLI's own generated types are the ones actually exercised at runtime, so
	// bex follows them over the stale spec. Revisit if a refreshed OpenAPI
	// snapshot confirms Render renamed the wire field too.
	// ADR018 §Key Value — REST column: "list envelope key is keyValue, not redis (CLI-verified; spec snapshot predates the rename)".
	"list-redis": {
		{
			contains: `/0/redis: property "redis" is missing`,
			adr018:   "ADR018 §Key Value REST: bex list-key-value wraps items as {keyValue:{}, cursor} (CLI-verified); the vendored spec's stale schema name is \"redis\"",
		},
	},
}

func knownConformanceDivergences(adr018 string, contains ...string) []conformanceDivergence {
	out := make([]conformanceDivergence, 0, len(contains))
	for _, fragment := range contains {
		out = append(out, conformanceDivergence{contains: fragment, adr018: adr018})
	}
	return out
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
