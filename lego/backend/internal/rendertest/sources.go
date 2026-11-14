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
	"slices"
	"strings"
)

// Source is a resource-metadata source a test server can run without. A
// production bex-api always has each one (BEX_REGION, BEX_DASHBOARD_URL, the
// workspace store and the identity reader), so a field missing because a test
// server lacks one is a fixture gap, not a divergence from Render. Wrap excuses
// that omission only for a server that really lacks the source; a server
// configured like production is held to the full schema, so a read that drops
// the field fails the test that served it.
type Source string

const (
	// SourceRegion is BEX_REGION: `region`, and a service's
	// `serviceDetails.region`.
	SourceRegion Source = "region"
	// SourceDashboardURL is BEX_DASHBOARD_URL: `dashboardUrl`.
	SourceDashboardURL Source = "dashboardUrl"
	// SourceWorkspaces is the workspace store: `owner` and `ownerId` (with no
	// owner there is no owner email to miss), and a disk's `id`, read from the
	// control-plane row a create writes only for a workspace's service.
	SourceWorkspaces Source = "workspaces"
	// SourceIdentities is the identity reader: a present owner's `email`.
	SourceIdentities Source = "identities"
)

// sourceFields are the JSON pointer tails each source fills.
var sourceFields = map[Source][]string{
	SourceRegion:       {"region"},
	SourceDashboardURL: {"dashboardUrl"},
	SourceWorkspaces:   {"owner", "ownerId", "serviceDetails/disk/id"},
	SourceIdentities:   {"owner/email"},
}

// AllSources is every source, for a bare feature mux that runs without the
// platform's metadata wiring.
var AllSources = []Source{SourceRegion, SourceDashboardURL, SourceWorkspaces, SourceIdentities}

// withoutUnconfigured drops the missing-property violations the unconfigured
// sources explain, and keeps every other violation.
func withoutUnconfigured(violations []string, unconfigured []Source) []string {
	var excused []string
	for _, source := range unconfigured {
		for _, field := range sourceFields[source] {
			excused = append(excused, missingProperty(field))
		}
	}
	if len(excused) == 0 {
		return violations
	}
	return slices.DeleteFunc(slices.Clone(violations), func(v string) bool {
		return slices.ContainsFunc(excused, func(e string) bool { return strings.Contains(v, e) })
	})
}
