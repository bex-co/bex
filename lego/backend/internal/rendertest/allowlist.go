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

package rendertest

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Divergence is one known, recorded difference between a bex response and the
// pinned Render schema.
type Divergence struct {
	// Contains is a substring of the Validate violation it excuses. Keep it
	// precise (lead with the JSON pointer's tail) so it cannot mask an
	// unrelated future error in the same operation.
	Contains string
	// ADR018 is the exact first cell of the docs/ADR018-render-parity.md
	// "Allowlisted response-schema divergences" row recording the divergence;
	// TestAllowlistCitesADR018Rows fails a citation with no such row.
	ADR018 string
	// Suite is the test package whose full run reproduces the divergence:
	// SuiteAPI (the composed server) or SuiteApps (the bare service mux).
	Suite string
}

// The suites that report Unexercised entries from their TestMain.
const (
	SuiteAPI  = "internal/api"
	SuiteApps = "internal/apps"
)

// The ADR018 rows the entries cite.
const (
	RowMetadata       = "Resource metadata omitted when unknown"
	RowServiceDetails = "Partial serviceDetails projection"
	RowEvents         = "Service event types and details"
	RowBlueprintIDs   = "Blueprint and Blueprint sync id prefixes"
	RowDryRun         = "Dry-run previews"
)

// Allowlist maps a Render operationId to its recorded divergences. It only
// shrinks: TestAllowlistOnlyShrinks pins its size, and each suite's TestMain
// fails an entry no response in its full run reproduces any more. Remove an
// entry (and lower the pinned size) once bex conforms; fix a new violation
// rather than adding one.
var Allowlist = map[string][]Divergence{
	// Services: bex omits the owner, dashboard link and timestamps it cannot
	// state (no resolved workspace, BEX_DASHBOARD_URL unset) and projects a
	// partial serviceDetails, checked field by field against the union member
	// the service's type selects. openPorts is not among the omissions
	// (emitted since w6/m148, pinned by apps.TestRenderServiceDetailsOpenPorts).
	"list-services": join(
		missing(RowMetadata, SuiteAPI, "ownerId", "dashboardUrl", "updatedAt"),
		details(SuiteAPI, "buildPlan", "envSpecificDetails", "healthCheckPath", "plan", "publishPath", "region", "url"),
		imageCommands(SuiteAPI),
	),
	"retrieve-service": join(
		missing(RowMetadata, SuiteAPI, "ownerId", "dashboardUrl", "updatedAt"),
		details(SuiteAPI, "buildPlan", "envSpecificDetails", "healthCheckPath", "plan", "region", "url"),
		imageCommands(SuiteAPI),
		details(SuiteApps, "env", "publishPath", "runtime"),
	),
	"create-service": join(
		missing(RowMetadata, SuiteAPI, "ownerId", "dashboardUrl"),
		details(SuiteAPI, "buildPlan", "envSpecificDetails", "healthCheckPath", "region", "url"),
		imageCommands(SuiteAPI),
		details(SuiteApps, "disk/id"),
		[]Divergence{{Contains: "status 200: not a documented response", ADR018: RowDryRun, Suite: SuiteApps}},
	),
	"update-service": join(
		missing(RowMetadata, SuiteAPI, "dashboardUrl"),
		missing(RowMetadata, SuiteApps, "ownerId", "updatedAt"),
		details(SuiteAPI, "buildPlan", "envSpecificDetails", "healthCheckPath", "region", "url"),
		details(SuiteApps, "env", "plan", "runtime"),
		imageCommands(SuiteApps),
	),

	// Datastores and projects: owner (and its email), region, version,
	// project and dashboard link are omitted when unknown or unconfigured.
	// ipAllowList and readReplicas left these lists in w6/m109, role and
	// suspenders in w8/129, the Key Value version when it was reported.
	"list-postgres":      missing(RowMetadata, SuiteAPI, "updatedAt", "owner", "owner/email", "region", "version", "dashboardUrl"),
	"retrieve-postgres":  missing(RowMetadata, SuiteAPI, "updatedAt", "owner", "owner/email", "region", "version", "dashboardUrl", "project"),
	"create-postgres":    missing(RowMetadata, SuiteAPI, "owner", "region", "version", "dashboardUrl", "project"),
	"update-postgres":    missing(RowMetadata, SuiteAPI, "owner", "region", "version", "dashboardUrl", "project"),
	"recover-postgres":   missing(RowMetadata, SuiteAPI, "owner", "region", "version", "dashboardUrl", "project"),
	"list-key-value":     missing(RowMetadata, SuiteAPI, "updatedAt", "owner", "owner/email", "region", "dashboardUrl"),
	"retrieve-key-value": missing(RowMetadata, SuiteAPI, "updatedAt", "owner", "owner/email", "region"),
	"create-key-value":   join(missing(RowMetadata, SuiteAPI, "owner", "region"), dryRun()),
	"update-key-value":   missing(RowMetadata, SuiteAPI, "owner", "region"),
	// Render's deprecated /redis spelling of the same Key Value (w3/m274).
	"list-redis":       missing(RowMetadata, SuiteAPI, "updatedAt", "owner", "region", "dashboardUrl"),
	"retrieve-redis":   missing(RowMetadata, SuiteAPI, "updatedAt", "owner", "region"),
	"create-redis":     join(missing(RowMetadata, SuiteAPI, "owner", "region"), dryRun()),
	"update-redis":     missing(RowMetadata, SuiteAPI, "owner", "region"),
	"list-projects":    missing(RowMetadata, SuiteAPI, "owner"),
	"retrieve-project": missing(RowMetadata, SuiteAPI, "owner/email"),
	"create-project":   missing(RowMetadata, SuiteAPI, "owner/email"),

	// Events: bex-named types (configuration changes, placement moves,
	// custom-domain verification) and a partial details dialect.
	"list-events": {
		{Contains: `event/type: "`, ADR018: RowEvents, Suite: SuiteAPI},
		{Contains: `event/details: does not match the documented union`, ADR018: RowEvents, Suite: SuiteAPI},
	},
	"retrieve-event": {{Contains: `/details: does not match the documented union`, ADR018: RowEvents, Suite: SuiteAPI}},

	// Blueprints keep bex's blp-/bsr- ids where Render mints exs-/exe-.
	"list-blueprints":      {{Contains: `/blueprint/id: string doesn't match the regular expression "^exs-`, ADR018: RowBlueprintIDs, Suite: SuiteApps}},
	"list-blueprint-syncs": {{Contains: `/sync/id: string doesn't match the regular expression "^exe-`, ADR018: RowBlueprintIDs, Suite: SuiteApps}},
}

// dryRun excuses a create's dry-run preview answering 200 (the bex dryRun
// extension) where Render documents only the create's 201.
func dryRun() []Divergence {
	return []Divergence{{Contains: "status 200: not a documented response", ADR018: RowDryRun, Suite: SuiteAPI}}
}

func join(groups ...[]Divergence) []Divergence { return slices.Concat(groups...) }

// missing is the omission entries for the named fields, each a JSON pointer
// tail ("owner/email" is the email of a present owner).
func missing(row, suite string, fields ...string) []Divergence {
	out := make([]Divergence, 0, len(fields))
	for _, field := range fields {
		name := field[strings.LastIndex(field, "/")+1:]
		out = append(out, Divergence{Contains: fmt.Sprintf(`/%s: property %q is missing`, field, name), ADR018: row, Suite: suite})
	}
	return out
}

// details is missing for serviceDetails members.
func details(suite string, fields ...string) []Divergence {
	prefixed := make([]string, len(fields))
	for i, field := range fields {
		prefixed[i] = "serviceDetails/" + field
	}
	return missing(RowServiceDetails, suite, prefixed...)
}

// imageCommands excuses an image service's envSpecificDetails, which carries
// only the commands the official CLI clones from (w1/m167, w2/042) and so
// matches neither the docker nor the native member.
func imageCommands(suite string) []Divergence {
	return []Divergence{{Contains: `/serviceDetails/envSpecificDetails: does not match the documented union`, ADR018: RowServiceDetails, Suite: suite}}
}

var (
	hitsMu sync.Mutex
	hits   = map[string]bool{} // operationId + "\x00" + Contains
)

// Allowed reports whether violation is a recorded divergence of operationID,
// and records the entry as exercised.
func Allowed(operationID, violation string) bool {
	for _, d := range Allowlist[operationID] {
		if strings.Contains(violation, d.Contains) {
			hitsMu.Lock()
			hits[operationID+"\x00"+d.Contains] = true
			hitsMu.Unlock()
			return true
		}
	}
	return false
}

// Unallowed returns the violations no Allowlist entry excuses.
func Unallowed(operationID string, violations []string) []string {
	var out []string
	for _, v := range violations {
		if !Allowed(operationID, v) {
			out = append(out, v)
		}
	}
	return out
}

// Unexercised lists suite's Allowlist entries no violation has matched so far
// in this test binary, as "operationId: contains", sorted. After the suite's
// full run each is a divergence bex no longer has, and its entry should go.
func Unexercised(suite string) []string {
	hitsMu.Lock()
	defer hitsMu.Unlock()
	var out []string
	for op, entries := range Allowlist {
		for _, d := range entries {
			if d.Suite == suite && !hits[op+"\x00"+d.Contains] {
				out = append(out, op+": "+d.Contains)
			}
		}
	}
	slices.Sort(out)
	return out
}
