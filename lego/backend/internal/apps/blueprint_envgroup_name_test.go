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
	"context"
	"testing"
)

// w8/094: env-group names are trimmed when parsed and applied, but the plan
// keyed the IR on the raw name, so a padded declaration of an existing group
// planned a create without its ID. Every declaration location now plans the
// conservative update of the existing group under its canonical name.
func TestBlueprintPaddedEnvGroupNamePlansTheExistingGroup(t *testing.T) {
	svc, _ := newBlueprintEnvService(newFakeEnvGroups("shared"), nil)
	place := map[string]func(name string) string{
		"root":        func(n string) string { return `{"envVarGroups":[{"name":"` + n + `","envVars":[]}]}` },
		"ungrouped":   func(n string) string { return `{"ungrouped":{"envVarGroups":[{"name":"` + n + `","envVars":[]}]}}` },
		"environment": func(n string) string { return `{"projects":[{"name":"p","environments":[{"name":"e","envVarGroups":[{"name":"` + n + `","envVars":[]}]}]}]}` },
	}
	for where, manifest := range place {
		for _, tc := range []struct{ declared, wantOp, wantName, wantID string }{
			{"shared", "update", "shared", "evg-shared"},
			{"  shared", "update", "shared", "evg-shared"},
			{"shared  ", "update", "shared", "evg-shared"},
			{"\u00a0shared\u2003", "update", "shared", "evg-shared"},
			{"  fresh", "create", "fresh", ""},
			{"sha red", "create", "sha red", ""},
		} {
			result, err := svc.ValidateBlueprint(context.Background(), "", manifest(tc.declared), "")
			if err != nil || !result.Valid || result.Plan == nil {
				t.Fatalf("%s %q: %+v, %v; want valid", where, tc.declared, result, err)
			}
			var found bool
			for _, a := range result.Plan.Actions {
				if a.Kind != BlueprintResourceEnvVarGroup {
					continue
				}
				found = true
				if string(a.Operation) != tc.wantOp || a.Name != tc.wantName || a.ResourceID != tc.wantID {
					t.Errorf("%s %q: action %s %q %q, want %s %q %q", where, tc.declared, a.Operation, a.Name, a.ResourceID, tc.wantOp, tc.wantName, tc.wantID)
				}
			}
			if !found {
				t.Errorf("%s %q: no env group action in %+v", where, tc.declared, result.Plan.Actions)
			}
		}
	}

	// A whitespace-only name keeps its error.
	result, err := svc.ValidateBlueprint(context.Background(), "", place["root"]("   "), "")
	if err != nil || result.Valid {
		t.Fatalf("whitespace-only name = %+v, %v; want invalid", result, err)
	}
}
