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
	"testing"
)

func TestUnallowedExcusesOnlyTheUnconfiguredSources(t *testing.T) {
	// list-custom-domains has no Allowlist entries, so only Sources excuse.
	const op = "list-custom-domains"
	region := `/0/service/serviceDetails/region: property "region" is missing`
	diskID := `/serviceDetails/disk/id: property "id" is missing`
	email := `/owner/email: property "email" is missing`
	other := `/0/customDomain/createdAt: expected string, but got number`
	all := []string{region, diskID, email, other}
	for _, tc := range []struct {
		name         string
		unconfigured []Source
		want         []string
	}{
		{"a production-configured server is held to every field", nil, all},
		{"no region source excuses region alone", []Source{SourceRegion}, []string{diskID, email, other}},
		{"no workspace store excuses the disk id, not the owner's email", []Source{SourceWorkspaces}, []string{region, email, other}},
		{"every source missing still keeps a wrong value", AllSources, []string{other}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Unallowed(op, all, tc.unconfigured...); !slices.Equal(got, tc.want) {
				t.Errorf("Unallowed = %q, want %q", got, tc.want)
			}
		})
	}
}
