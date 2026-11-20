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

package connlimit

import "testing"

// TestResolveFallsBackToTheDefaultPlan pins the resolution the SNI proxies size
// a route by: a known plan is its own limit, and an empty or unknown one is the
// default plan's, as tiers resolves the plan the operator sizes the server by.
// tiers' tests pin each figure to Render's and every plan to the catalog's.
func TestResolveFallsBackToTheDefaultPlan(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ladder Ladder
		plan   string
		want   int32
	}{
		{"Valkey starter", Valkey, "starter", 250},
		{"Valkey empty", Valkey, "", 50},
		{"Valkey unknown", Valkey, "gold", 50},
		{"Postgres basic-1gb", Postgres, "basic-1gb", 100},
		{"Postgres empty", Postgres, "", 100},
		{"Postgres unknown", Postgres, "gold", 100},
	} {
		if got := tc.ladder.Resolve(tc.plan); got != tc.want {
			t.Errorf("%s: Resolve(%q) = %d, want %d", tc.name, tc.plan, got, tc.want)
		}
	}
	for _, ladder := range []Ladder{Valkey, Postgres} {
		if _, ok := ladder.Limit(ladder.DefaultPlan()); !ok {
			t.Errorf("default plan %q has no limit", ladder.DefaultPlan())
		}
		if limit, ok := ladder.Limit("gold"); ok {
			t.Errorf("Limit(gold) = %d, true for a plan the ladder lacks", limit)
		}
	}
}
