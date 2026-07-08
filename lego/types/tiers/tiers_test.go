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

import (
	"testing"
)

// wantCompute is the ladder this package replaces — the operator's former
// tierResources map (lego/operator/internal/controller/app_controller.go) —
// asserted byte-for-byte so the embedded catalog can never silently drift
// from what shipped before it.
var wantCompute = map[string]struct{ cpu, mem string }{
	"free":      {"100m", "512Mi"},
	"starter":   {"500m", "512Mi"},
	"standard":  {"1", "2Gi"},
	"pro":       {"2", "4Gi"},
	"pro-plus":  {"4", "8Gi"},
	"pro-max":   {"4", "16Gi"},
	"pro-ultra": {"8", "32Gi"},
}

// wantPostgres is the operator's former dbPlans map
// (lego/operator/internal/controller/database_controller.go), same rationale.
var wantPostgres = map[string]struct {
	cpu, mem  string
	storageGB int32
	instances int32
}{
	"free":        {"100m", "256Mi", 1, 1},
	"basic-256mb": {"100m", "256Mi", 1, 1},
	"basic-1gb":   {"500m", "1Gi", 5, 1},
}

func TestComputeCatalogMatchesFormerOperatorLadder(t *testing.T) {
	if got := len(Compute.IDs()); got != len(wantCompute) {
		t.Fatalf("want %d compute tiers, got %d", len(wantCompute), got)
	}
	for id, want := range wantCompute {
		got, ok := Compute.ByID(id)
		if !ok {
			t.Fatalf("compute tier %q missing from embedded catalog", id)
		}
		if got.CPU != want.cpu || got.Memory != want.mem {
			t.Errorf("compute %q: got cpu=%s mem=%s, want cpu=%s mem=%s", id, got.CPU, got.Memory, want.cpu, want.mem)
		}
	}
}

func TestPostgresCatalogMatchesFormerDBPlans(t *testing.T) {
	if got := len(Postgres.IDs()); got != len(wantPostgres) {
		t.Fatalf("want %d postgres tiers, got %d", len(wantPostgres), got)
	}
	for id, want := range wantPostgres {
		got, ok := Postgres.ByID(id)
		if !ok {
			t.Fatalf("postgres tier %q missing from embedded catalog", id)
		}
		if got.CPU != want.cpu || got.Memory != want.mem || got.StorageGB != want.storageGB || got.Instances != want.instances {
			t.Errorf("postgres %q: got %+v, want %+v", id, got, want)
		}
	}
}

func TestDefaultsAreFree(t *testing.T) {
	if got := Compute.Default(); got.ID != "free" {
		t.Errorf("Compute.Default() = %q, want %q (the store's prior normalizeTier(\"\") behavior)", got.ID, "free")
	}
	if got := Postgres.Default(); got.ID != "free" {
		t.Errorf("Postgres.Default() = %q, want %q (the operator's prior dbPlans fallback)", got.ID, "free")
	}
}

func TestByRenderPlanRoundTrip(t *testing.T) {
	// pro-plus is the one entry whose Render spelling diverges from its id
	// (underscore vs hyphen) — the case the whole RenderPlan column exists for.
	tier, ok := Compute.ByRenderPlan("pro_plus")
	if !ok || tier.ID != "pro-plus" {
		t.Errorf(`ByRenderPlan("pro_plus") = (%+v, %v), want ID "pro-plus"`, tier, ok)
	}
	if _, ok := Compute.ByRenderPlan("nonexistent"); ok {
		t.Error(`ByRenderPlan("nonexistent") should report ok=false`)
	}
}

// TestRenderPlansMatchesIDsPositionally pins the two lists as siblings — same
// length, same order, each pair round-tripping through ByID — the property
// that lets a caller build a bad-plan error message purely from RenderPlans()
// without a second lookup per entry.
func TestRenderPlansMatchesIDsPositionally(t *testing.T) {
	ids, plans := Compute.IDs(), Compute.RenderPlans()
	if len(ids) != len(plans) {
		t.Fatalf("IDs() has %d entries, RenderPlans() has %d", len(ids), len(plans))
	}
	for i, id := range ids {
		want, ok := Compute.ByID(id)
		if !ok || want.RenderPlan != plans[i] {
			t.Errorf("position %d: id %q's RenderPlan is %q, but RenderPlans()[%d] = %q", i, id, want.RenderPlan, i, plans[i])
		}
	}
}

func TestComputeResourcesOkFalseForEmptyOrUnknownTier(t *testing.T) {
	if _, _, ok := Compute.Resources(""); ok {
		t.Error(`Resources("") should report ok=false (best-effort fallback), not a resolved tier`)
	}
	if _, _, ok := Compute.Resources("gold"); ok {
		t.Error(`Resources("gold") should report ok=false for an unknown tier`)
	}
	cpu, mem, ok := Compute.Resources("standard")
	if !ok || cpu != "1" || mem != "2Gi" {
		t.Errorf(`Resources("standard") = (%q, %q, %v), want ("1", "2Gi", true)`, cpu, mem, ok)
	}
}

// --- parse() validation: every malformed-catalog case fails loudly, never silently ---

// valid is a minimal well-formed catalog the malformed cases below mutate from.
const valid = `
compute:
  default: starter
  tiers:
    - id: starter
      renderPlan: starter
      cpu: 500m
      memory: 512Mi
postgres:
  default: free
  tiers:
    - id: free
      cpu: 100m
      memory: 256Mi
      storageGB: 1
      instances: 1
`

func TestParseAcceptsMinimalValidCatalog(t *testing.T) {
	if _, _, err := parse([]byte(valid)); err != nil {
		t.Fatalf("minimal valid catalog rejected: %v", err)
	}
}

func TestParseRejectsMalformedCatalogs(t *testing.T) {
	cases := map[string]string{
		"malformed YAML": "compute: [not: valid: yaml:",
		"empty compute family": `
compute:
  default: starter
  tiers: []
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"empty postgres family": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers: []
`,
		"unknown field (typo)": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memroy: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"duplicate compute id": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
    - {id: starter, renderPlan: starter2, cpu: "1", memory: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"duplicate renderPlan": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
    - {id: starter2, renderPlan: starter, cpu: "1", memory: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"empty renderPlan": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: "", cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"unparseable quantity": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: not-a-quantity, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"compute default names undefined tier": `
compute:
  default: nonexistent
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"missing postgres default": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"duplicate postgres id": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"postgres zero storage": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 0, instances: 1}
`,
		"postgres zero instances": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 0}
`,
		"empty tier id": `
compute:
  default: starter
  tiers:
    - {id: "", renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
	}
	for name, raw := range cases {
		if _, _, err := parse([]byte(raw)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}
