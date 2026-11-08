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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestBlueprintProtectedPlacementTruthTable pins which Blueprint placements
// need an Admin (w3/m281): a create in, or a move into or out of, a protected
// environment — never a resource the apply leaves where it is.
func TestBlueprintProtectedPlacementTruthTable(t *testing.T) {
	placements := blueprintPlacements{
		protected: map[string]bool{"env-prod": true, "env-staging": false},
		targets: map[string]groupingTarget{
			"prod":     {environmentID: "env-prod", protected: true},
			"staging":  {environmentID: "env-staging"},
			"new-prod": {protected: true}, // the apply creates it, protected
			"new-dev":  {},
		},
	}
	unknown := placementOf{unknown: true}
	for _, tc := range []struct {
		name      string
		grouping  string
		ungrouped bool
		current   placementOf
		wantEnv   string
		refused   bool
	}{
		{"create in protected", "prod", false, placementOf{}, "env-prod", true},
		{"create in a new protected environment", "new-prod", false, placementOf{}, "", true},
		{"create in unprotected", "staging", false, placementOf{}, "", false},
		{"create ungrouped", "", true, placementOf{}, "", false},
		{"create with no placement", "", false, placementOf{}, "", false},
		{"stays in protected", "prod", false, placedIn(map[string]string{core.LabelEnvironment: "env-prod"}), "", false},
		{"moves into protected", "prod", false, placementOf{exists: true, environmentID: "env-staging"}, "env-prod", true},
		{"moves from unassigned into protected", "prod", false, placementOf{exists: true}, "env-prod", true},
		{"moves out of protected", "staging", false, placementOf{exists: true, environmentID: "env-prod"}, "env-prod", true},
		{"moves out of protected to unassigned", "", true, placementOf{exists: true, environmentID: "env-prod"}, "env-prod", true},
		{"moves out of protected into a new environment", "new-dev", false, placementOf{exists: true, environmentID: "env-prod"}, "env-prod", true},
		{"moves between unprotected", "new-dev", false, placementOf{exists: true, environmentID: "env-staging"}, "", false},
		{"stays unassigned", "", true, placementOf{exists: true}, "", false},
		{"declared without placement stays in protected", "", false, placementOf{exists: true, environmentID: "env-prod"}, "", false},
		{"an unread placement into protected counts as a move", "prod", false, unknown, "env-prod", true},
		{"an unread placement into unprotected is open", "staging", false, unknown, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, refused := placements.check(tc.grouping, tc.ungrouped, tc.current)
			if refused != tc.refused || env != tc.wantEnv {
				t.Fatalf("check(%q, ungrouped=%v, %+v) = %q, %v; want %q, %v", tc.grouping, tc.ungrouped, tc.current, env, refused, tc.wantEnv, tc.refused)
			}
		})
	}
}
