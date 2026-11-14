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
	RowDryRun         = "Dry-run previews"
)

// Allowlist maps a Render operationId to its recorded divergences. It only
// shrinks: TestAllowlistOnlyShrinks pins its size, and each suite's TestMain
// fails an entry no response in its full run reproduces any more. Remove an
// entry (and lower the pinned size) once bex conforms; fix a new violation
// rather than adding one.
var Allowlist = map[string][]Divergence{
	// Services: a partial serviceDetails, checked field by field against the
	// union member the service's type selects. Owner, region, dashboard link
	// and updatedAt are not entries: the fake API server stamps managedFields
	// (Stamped), and a server lacking a metadata source is excused by its
	// Source list (Wrap, Unallowed), not here.
	"list-services": join(
		details(SuiteAPI, "envSpecificDetails", "url"),
		imageCommands(SuiteAPI),
	),
	"retrieve-service": join(
		details(SuiteAPI, "envSpecificDetails", "url"),
		imageCommands(SuiteAPI),
		details(SuiteApps, "env", "runtime"),
	),
	"create-service": join(
		details(SuiteAPI, "envSpecificDetails", "url"),
		imageCommands(SuiteAPI),
		[]Divergence{{Contains: "status 200: not a documented response", ADR018: RowDryRun, Suite: SuiteApps}},
	),
	"update-service": join(
		details(SuiteAPI, "envSpecificDetails", "url"),
		details(SuiteApps, "env", "runtime"),
		imageCommands(SuiteApps),
	),

	// Postgres: version and project, open questions recorded in their
	// RowMetadata row.
	"list-postgres":     missing(RowMetadata, SuiteAPI, "version"),
	"retrieve-postgres": missing(RowMetadata, SuiteAPI, "version", "project"),
	"create-postgres":   missing(RowMetadata, SuiteAPI, "version", "project"),
	"update-postgres":   missing(RowMetadata, SuiteAPI, "version", "project"),
	"recover-postgres":  missing(RowMetadata, SuiteAPI, "version", "project"),
	"create-key-value":  dryRun(),
	// Render's deprecated /redis spelling of the same Key Value (w3/m274).
	"create-redis": dryRun(),

	// Events: bex-named types keep their enum violation; every other event's
	// details are checked against the member its type names (w3/m302), and
	// these are the Render-required fields bex does not record.
	"list-events": join(
		[]Divergence{{Contains: `event/type: "`, ADR018: RowEvents, Suite: SuiteAPI}},
		eventDetails(SuiteAPI, "deploy_ended", "status"),
		eventDetails(SuiteAPI, "build_ended", "status", "buildId"),
		eventDetails(SuiteAPI, "suspender_added", "suspendedByUser/email"),
		eventDetails(SuiteAPI, "disk_snapshot_restored", "diskId"),
	),
}

// eventDetails is the missing-field entries for one event type's details.
func eventDetails(suite, eventType string, fields ...string) []Divergence {
	out := make([]Divergence, 0, len(fields))
	for _, field := range fields {
		out = append(out, Divergence{Contains: MissingEventDetail(eventType, field), ADR018: RowEvents, Suite: suite})
	}
	return out
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
		out = append(out, Divergence{Contains: missingProperty(field), ADR018: row, Suite: suite})
	}
	return out
}

// missingProperty is the tail of Validate's violation for a required field
// missing at the JSON pointer tail field ("owner/email" is a present owner's
// email).
func missingProperty(field string) string {
	return fmt.Sprintf(`/%s: property %q is missing`, field, field[strings.LastIndex(field, "/")+1:])
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

// Unallowed returns the violations no Allowlist entry excuses, after dropping
// the omissions the server's unconfigured metadata sources explain.
func Unallowed(operationID string, violations []string, unconfigured ...Source) []string {
	var out []string
	for _, v := range withoutUnconfigured(violations, unconfigured) {
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
