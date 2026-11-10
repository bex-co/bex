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
		"1c-2g":       "1c-2g",
		"1c-4g":       "pro-4gb",
		"pro_4gb":     "pro-4gb",
		"2c-4g":       "basic-4gb",
		"basic_4gb":   "basic-4gb",
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
		"5g":       "pro",
		"10g":      "pro_plus",
		"20g":      "pro_max",
		"40g":      "pro_ultra",
		"3g":       "3g",
		"pro plus": "pro_plus", // Render's Blueprint schema spelling
		"pro max":  "pro_max",
		"50g":      "50g",
		"100g":     "100g",
	}
	for in, want := range cases {
		if got := Valkey.CanonicalID(in); got != want {
			t.Errorf("Valkey.CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := Valkey.ByID(Valkey.CanonicalID("1g")); !ok {
		t.Fatal("1g must resolve to standard")
	}
	// Every Render size id lands on a rung whose memory is that size (w5/m266).
	for size, mem := range map[string]string{"5g": "5Gi", "10g": "10Gi", "20g": "20Gi", "40g": "40Gi", "3g": "3Gi", "50g": "50Gi"} {
		if tier, ok := Valkey.ByID(Valkey.CanonicalID(size)); !ok || tier.Memory != mem {
			t.Errorf("%s resolves to %+v, want a %s rung", size, tier, mem)
		}
	}
	if _, ok := Valkey.ByID(Valkey.CanonicalID("100g")); ok {
		t.Fatal("a size Render does not sell must not resolve to a catalog tier")
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

// w8/246: a refusal about a plan the caller did not name uses its Render
// spec-based id; a rung without one keeps its own id.
func TestPostgresSpecID(t *testing.T) {
	for id, want := range map[string]string{"basic-256mb": "0.1c-256mb", "basic-1gb": "0.5c-1g", "pro-4gb": "1c-4g", "free": "free"} {
		if got := PostgresSpecID(id); got != want {
			t.Errorf("PostgresSpecID(%q) = %q, want %q", id, got, want)
		}
	}
}
