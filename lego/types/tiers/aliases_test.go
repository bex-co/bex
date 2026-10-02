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

package tiers

import "testing"

func TestPostgresCanonicalIDAcceptsRenderAliases(t *testing.T) {
	cases := map[string]string{
		"basic-256mb": "basic-256mb",
		"0.1c-256mb":  "basic-256mb",
		"0.5c-1g":     "basic-1gb",
		"basic_256mb": "basic-256mb",
		"basic_1gb":   "basic-1gb",
		"free":        "free",
		"2c-8g":       "2c-8g", // no bex rung — unchanged so ByID rejects
	}
	for in, want := range cases {
		if got := Postgres.CanonicalID(in); got != want {
			t.Errorf("Postgres.CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := Postgres.ByID(Postgres.CanonicalID("0.1c-256mb")); !ok {
		t.Fatal("0.1c-256mb must resolve to a catalog tier")
	}
	if _, ok := Postgres.ByID(Postgres.CanonicalID("2c-8g")); ok {
		t.Fatal("2c-8g must not resolve to a catalog tier")
	}
}

func TestValkeyCanonicalIDAcceptsRenderAliases(t *testing.T) {
	cases := map[string]string{
		"starter":  "starter",
		"256mb":    "starter",
		"1g":       "standard",
		"standard": "standard",
		"5g":       "5g",
	}
	for in, want := range cases {
		if got := Valkey.CanonicalID(in); got != want {
			t.Errorf("Valkey.CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := Valkey.ByID(Valkey.CanonicalID("1g")); !ok {
		t.Fatal("1g must resolve to standard")
	}
	if _, ok := Valkey.ByID(Valkey.CanonicalID("5g")); ok {
		t.Fatal("5g must not resolve to a catalog tier")
	}
}

// TestComputeByRenderPlanAcceptsExactSizeAliases (w1/m170): every Render
// compute plan ID bex aliases must land on a rung whose CPU/RAM equal what the
// ID spells, and an ID with no same-size rung must stay unknown rather than
// round to a neighbour.
func TestComputeByRenderPlanAcceptsExactSizeAliases(t *testing.T) {
	want := map[string]struct{ id, cpu, memory string }{
		"0.5c-512mb": {"starter", "500m", "512Mi"},
		"1c-2g":      {"standard", "1", "2Gi"},
		"2c-4g":      {"pro", "2", "4Gi"},
		"4c-8g":      {"pro-plus", "4", "8Gi"},
		"4c-16g":     {"pro-max", "4", "16Gi"},
		"8c-32g":     {"pro-ultra", "8", "32Gi"},
	}
	if len(computeInputAliases) != len(want) {
		t.Fatalf("computeInputAliases has %d entries, want %d", len(computeInputAliases), len(want))
	}
	for plan, w := range want {
		tier, ok := Compute.ByRenderPlan(plan)
		if !ok || tier.ID != w.id || tier.CPU != w.cpu || tier.Memory != w.memory {
			t.Errorf("Compute.ByRenderPlan(%q) = %+v, %v; want %s (%s CPU, %s)", plan, tier, ok, w.id, w.cpu, w.memory)
		}
	}
	for _, plan := range []string{"2c-8g", "2c-16g", "4c-32g", "8c-16g", "8c-64g", "12c-24g", "12c-48g", "12c-96g", "pro plus"} {
		if tier, ok := Compute.ByRenderPlan(plan); ok {
			t.Errorf("Compute.ByRenderPlan(%q) = %+v, want no tier", plan, tier)
		}
	}
	if tier, ok := Compute.ByRenderPlan("pro_plus"); !ok || tier.ID != "pro-plus" {
		t.Errorf("legacy pro_plus must still resolve, got %+v %v", tier, ok)
	}
}
