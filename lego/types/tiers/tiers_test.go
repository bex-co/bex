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
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/types/tiers/connlimit"
)

// wantCompute is the ladder this package replaces — the operator's former
// tierResources map (lego/operator/internal/controller/app_controller.go) —
// asserted byte-for-byte so the embedded catalog can never silently drift
// from what shipped before it. disk is the ephemeral-storage bound (round-14
// #2), 1→32Gi scaling with the tier.
var wantCompute = map[string]struct{ cpu, mem, disk string }{
	"free":      {"100m", "512Mi", "1Gi"},
	"starter":   {"500m", "512Mi", "2Gi"},
	"standard":  {"1", "2Gi", "4Gi"},
	"pro":       {"2", "4Gi", "8Gi"},
	"pro-plus":  {"4", "8Gi", "16Gi"},
	"pro-max":   {"4", "16Gi", "16Gi"},
	"pro-ultra": {"8", "32Gi", "32Gi"},
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
	"1c-2g":       {"1", "2Gi", 10, 1},
	"pro-4gb":     {"1", "4Gi", 10, 1},
	"basic-4gb":   {"2", "4Gi", 10, 1},
}

// wantValkey is the managed key-value ladder shipped in tiers.yaml — Render's
// Key Value vocabulary (free/starter/standard, shared with the web-service
// ladder, not the Postgres basic-* names), single-instance tiers differing by
// compute and storage.
var wantValkey = map[string]struct {
	cpu, mem  string
	storageGB int32
	instances int32
}{
	"free":     {"100m", "128Mi", 1, 1},
	"starter":  {"100m", "256Mi", 1, 1},
	"standard": {"500m", "1Gi", 5, 1},
	// w5/m266: Render's larger plans.
	"3g":        {"1", "3Gi", 6, 1},
	"pro":       {"1", "5Gi", 10, 1},
	"pro_plus":  {"2", "10Gi", 20, 1},
	"pro_max":   {"4", "20Gi", 40, 1},
	"pro_ultra": {"4", "40Gi", 80, 1},
	"50g":       {"8", "50Gi", 100, 1},
}

// w5/m266: the stores a standard node cannot hold name the pool that can,
// and the xxl tiers share Pro Ultra's pause (user decision D3, 2026-11-05).
func TestValkeyLargeTiersPlacementAndPause(t *testing.T) {
	for id, want := range map[string]struct {
		pool   string
		paused bool
	}{
		"free": {}, "starter": {}, "standard": {}, "3g": {}, "pro": {},
		"pro_plus": {"xl", false}, "pro_max": {"xl", false},
		"pro_ultra": {"xxl", true}, "50g": {"xxl", true},
	} {
		got, _ := Valkey.ByID(id)
		if got.ServingPool != want.pool || Valkey.Paused(id) != want.paused {
			t.Errorf("%s: pool %q paused %v, want %q %v", id, got.ServingPool, Valkey.Paused(id), want.pool, want.paused)
		}
	}
	if Compute.Paused("pro-ultra") != Valkey.Paused("pro_ultra") {
		t.Error("Key Value pro_ultra must share the compute Pro Ultra pause")
	}
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
		if got.CPU != want.cpu || got.Memory != want.mem || got.EphemeralStorage != want.disk {
			t.Errorf("compute %q: got cpu=%s mem=%s disk=%s, want cpu=%s mem=%s disk=%s", id, got.CPU, got.Memory, got.EphemeralStorage, want.cpu, want.mem, want.disk)
		}
	}
}

// TestComputeEphemeralStorageBoundedEveryTier (round-14 #2): every compute
// rung must carry a positive ephemeral-storage quantity — the writable-layer
// disk bound tenant code runs under — so a future rung added without one
// fails here instead of shipping an unbounded tenant container.
func TestComputeEphemeralStorageBoundedEveryTier(t *testing.T) {
	for _, id := range Compute.IDs() {
		cpu, mem, disk, ok := Compute.Resources(id)
		if !ok {
			t.Fatalf("Resources(%q) = ok=false for a catalog id", id)
		}
		if cpu == "" || mem == "" {
			t.Errorf("Resources(%q) returned an empty cpu/memory", id)
		}
		q, err := resource.ParseQuantity(disk)
		if err != nil {
			t.Errorf("Resources(%q) ephemeral-storage %q unparseable: %v", id, disk, err)
			continue
		}
		if q.Value() <= 0 {
			t.Errorf("Resources(%q) ephemeral-storage = %s, want a positive bound", id, disk)
		}
	}
}

func TestPostgresCatalogMatchesFormerDBPlans(t *testing.T) {
	if got := Postgres.DiskAutoscalingCapGB(); got != 16*1024 {
		t.Fatalf("Postgres disk-autoscaling cap = %d GB, want catalog contract 16384 GB", got)
	}
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

func TestValkeyCatalogMatchesShippedLadder(t *testing.T) {
	if got := len(Valkey.IDs()); got != len(wantValkey) {
		t.Fatalf("want %d valkey tiers, got %d", len(wantValkey), got)
	}
	for id, want := range wantValkey {
		got, ok := Valkey.ByID(id)
		if !ok {
			t.Fatalf("valkey tier %q missing from embedded catalog", id)
		}
		if got.CPU != want.cpu || got.Memory != want.mem || got.StorageGB != want.storageGB || got.Instances != want.instances {
			t.Errorf("valkey %q: got %+v, want %+v", id, got, want)
		}
	}
}

// TestValkeyConnectionLimitsMatchRender pins every Key Value plan bex accepts,
// by id and by Render's size alias, to Render's connection limit
// (render.com/docs/compute-plans#render-key-value-plans, read 2026-11-17;
// w6/m456). A tier added without a figure fails parse, and one whose figure
// drifts from Render's fails here.
func TestValkeyConnectionLimitsMatchRender(t *testing.T) {
	render := map[string]int32{
		"free": 50, "starter": 250, "standard": 1000, "3g": 3000, "pro": 5000,
		"pro_plus": 10000, "pro_max": 20000, "pro_ultra": 40000, "50g": 50000,
		// Render's plan ids for the legacy names.
		"256mb": 250, "1g": 1000, "5g": 5000, "10g": 10000, "20g": 20000, "40g": 40000,
	}
	if len(Valkey.IDs())+len(valkeyInputAliases) != len(render) {
		t.Fatalf("%d ids + %d aliases, but Render's table pins %d", len(Valkey.IDs()), len(valkeyInputAliases), len(render))
	}
	for plan, want := range render {
		tier, ok := Valkey.ByID(Valkey.CanonicalID(plan))
		if !ok {
			t.Fatalf("plan %q does not resolve to a tier", plan)
		}
		if tier.ConnectionLimit != want {
			t.Errorf("plan %q: connection limit %d, Render's is %d", plan, tier.ConnectionLimit, want)
		}
	}
}

// TestPostgresMaxConnectionsMatchRender pins every Postgres plan bex accepts,
// by id and by Render's alias, to Render's connection limit
// (render.com/docs/compute-plans, read 2026-11-17: "Max 100 connections" free,
// 0.1c-256mb, 0.5c-1g, 1c-2g, 1c-4g and 2c-4g; 200 2c-8g; 400 2c-16g and
// 4c-16g; 500 4c-32g and up). Each tier is checked under Render's spec id, so
// a tier added later (2c-8g, w6/m332) must carry Render's figure; a tier
// added without one fails parse (w6/m457).
func TestPostgresMaxConnectionsMatchRender(t *testing.T) {
	render := map[string]int32{
		"free": 100, "0.1c-256mb": 100, "0.5c-1g": 100, "1c-2g": 100, "1c-4g": 100, "2c-4g": 100,
		"2c-8g": 200, "2c-16g": 400, "4c-16g": 400,
	}
	renderFigure := func(spec string, memory string) (int32, bool) {
		if want, ok := render[spec]; ok {
			return want, true
		}
		// Render's last heading: 500 for every plan from 32 GB.
		if q := resource.MustParse(memory); q.Cmp(resource.MustParse("32Gi")) >= 0 {
			return 500, true
		}
		return 0, false
	}
	for _, id := range Postgres.IDs() {
		tier, _ := Postgres.ByID(id)
		want, ok := renderFigure(PostgresSpecID(id), tier.Memory)
		if !ok {
			t.Errorf("tier %q (Render %q) is not in Render's connection table", id, PostgresSpecID(id))
			continue
		}
		if tier.MaxConnections != want {
			t.Errorf("tier %q: max connections %d, Render's is %d", id, tier.MaxConnections, want)
		}
	}
	for alias, id := range postgresInputAliases {
		if got, want := Postgres.Resolve(Postgres.CanonicalID(alias)), Postgres.Resolve(id); got.MaxConnections != want.MaxConnections || got.ID != id {
			t.Errorf("alias %q resolves to %q (%d connections), want %q", alias, got.ID, got.MaxConnections, id)
		}
	}
	if connlimit.PostgresPoolerMaxClientConn != 30000 {
		t.Errorf("pooled clients %d, Render's PgBouncer max_client_conn is 30000", connlimit.PostgresPoolerMaxClientConn)
	}
}

// TestResolveFallsBackToTheDefaultPlan pins the one resolution the operator
// (server sizing) and the SNI proxies (route admission) share: a known id is
// its tier, an empty or unknown one the free default.
func TestResolveFallsBackToTheDefaultPlan(t *testing.T) {
	if got := Postgres.Resolve("basic-1gb"); got.ID != "basic-1gb" {
		t.Errorf("Postgres.Resolve(basic-1gb) = %q", got.ID)
	}
	if got := Valkey.Resolve("standard"); got.ID != "standard" || got.ConnectionLimit != 1000 {
		t.Errorf("Valkey.Resolve(standard) = %q (%d)", got.ID, got.ConnectionLimit)
	}
	for _, id := range []string{"", "gold"} {
		if got := Postgres.Resolve(id); got.ID != "free" {
			t.Errorf("Postgres.Resolve(%q) = %q, want free", id, got.ID)
		}
		if got := Valkey.Resolve(id); got.ID != "free" {
			t.Errorf("Valkey.Resolve(%q) = %q, want free", id, got.ID)
		}
	}
}

// TestConnectionLimitsAreTheCatalogs keeps connlimit, which the SNI proxies
// size their routes from, in step with the catalog the operator sizes servers
// from (w6/154): each ladder names exactly the catalog's plans, and every plan
// id, an empty one and an unknown one resolve to the same limit on both sides.
// Parse already refuses a tier without a limit and a default connlimit does not
// share; this catches a limit left behind for a plan the catalog dropped.
func TestConnectionLimitsAreTheCatalogs(t *testing.T) {
	if got, want := connlimit.Postgres.Plans(), slices.Sorted(slices.Values(Postgres.IDs())); !slices.Equal(got, want) {
		t.Errorf("connlimit.Postgres names %v, the catalog %v", got, want)
	}
	if got, want := connlimit.Valkey.Plans(), slices.Sorted(slices.Values(Valkey.IDs())); !slices.Equal(got, want) {
		t.Errorf("connlimit.Valkey names %v, the catalog %v", got, want)
	}
	for _, plan := range append(Postgres.IDs(), "", "gold") {
		if got, want := connlimit.Postgres.Resolve(plan), Postgres.Resolve(plan).MaxConnections; got != want {
			t.Errorf("Postgres plan %q: the proxies size %d, the operator %d", plan, got, want)
		}
	}
	for _, plan := range append(Valkey.IDs(), "", "gold") {
		if got, want := connlimit.Valkey.Resolve(plan), Valkey.Resolve(plan).ConnectionLimit; got != want {
			t.Errorf("Key Value plan %q: the proxies size %d, the operator %d", plan, got, want)
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
	if got := Valkey.Default(); got.ID != "free" {
		t.Errorf("Valkey.Default() = %q, want %q (the KeyValue plan fallback)", got.ID, "free")
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
	if _, _, _, ok := Compute.Resources(""); ok {
		t.Error(`Resources("") should report ok=false (best-effort fallback), not a resolved tier`)
	}
	if _, _, _, ok := Compute.Resources("gold"); ok {
		t.Error(`Resources("gold") should report ok=false for an unknown tier`)
	}
	cpu, mem, disk, ok := Compute.Resources("standard")
	if !ok || cpu != "1" || mem != "2Gi" || disk != "4Gi" {
		t.Errorf(`Resources("standard") = (%q, %q, %q, %v), want ("1", "2Gi", "4Gi", true)`, cpu, mem, disk, ok)
	}
}

// TestComputeInstanceCap pins the w6/m118 policy: free is capped at one running
// instance (the $0.00/second rate would otherwise buy N× capacity for $0), and
// every paid tier — plus an empty/unknown id — reports no plan cap, so only the
// platform ceiling applies to them.
func TestComputeInstanceCap(t *testing.T) {
	cap, ok := Compute.InstanceCap("free")
	if !ok || cap != 1 {
		t.Errorf(`InstanceCap("free") = (%d, %v), want (1, true)`, cap, ok)
	}
	for _, id := range Compute.IDs() {
		if id == "free" {
			continue
		}
		if _, ok := Compute.InstanceCap(id); ok {
			t.Errorf("InstanceCap(%q) reported a plan cap; only free should be capped", id)
		}
	}
	if _, ok := Compute.InstanceCap(""); ok {
		t.Error(`InstanceCap("") should report ok=false — an untiered App is not plan-capped`)
	}
	if _, ok := Compute.InstanceCap("gold"); ok {
		t.Error(`InstanceCap("gold") should report ok=false for an unknown tier`)
	}
}

// TestComputeServingPool pins the w2/m250 routing: the three tiers no standard
// bex-tenant-0 node can hold name their dedicated pool — pro-plus and pro-max
// the cx53 xl pool, pro-ultra (32Gi, more than any 32 GB node allocates) the
// ccx43 xxl pool — and every other tier, plus an empty/unknown id, keeps the
// standard serving pool.
func TestComputeServingPool(t *testing.T) {
	want := map[string]string{"pro-plus": "xl", "pro-max": "xl", "pro-ultra": "xxl"}
	for _, id := range append(Compute.IDs(), "", "gold") {
		if pool := Compute.ServingPool(id); pool != want[id] {
			t.Errorf("ServingPool(%q) = %q, want %q", id, pool, want[id])
		}
	}
	if got := Compute.ServingPools(); len(got) != 2 || got[0] != "xl" || got[1] != "xxl" {
		t.Errorf("ServingPools() = %v, want [xl xxl]", got)
	}
}

// TestComputePaused pins the temporary w2/m250 pause (user decision
// 2026-11-02): only pro-ultra is refused for new selections, and an
// empty/unknown id is never paused. Releasing the pause (deleting the field
// from tiers.yaml) is expected to update this test.
func TestComputePaused(t *testing.T) {
	for _, id := range append(Compute.IDs(), "", "gold") {
		if got, want := Compute.Paused(id), id == "pro-ultra"; got != want {
			t.Errorf("Paused(%q) = %v, want %v", id, got, want)
		}
	}
	if Compute.ServingPool("pro-ultra") != "xxl" {
		t.Error("a paused tier keeps its servingPool so existing services still place")
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
      ephemeralStorage: 1Gi
postgres:
  default: free
  tiers:
    - id: free
      cpu: 100m
      memory: 256Mi
      storageGB: 1
      instances: 1
valkey:
  default: free
  tiers:
    - id: free
      cpu: 100m
      memory: 128Mi
      storageGB: 1
      instances: 1
`

// validValkey is a flow-style valkey block appended to every malformed case so
// each only breaks the family it intends (parse rejects an absent family too).
const validValkey = `
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1}
`

func TestParseAcceptsMinimalValidCatalog(t *testing.T) {
	if _, _, _, err := parse([]byte(valid)); err != nil {
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
` + validValkey,
		"paused default tier": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi, paused: true}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"valkey tier on an unknown pool": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
    - {id: big, renderPlan: big, cpu: "4", memory: 16Gi, ephemeralStorage: 1Gi, servingPool: xl}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1}
    - {id: pro_ultra, cpu: "4", memory: 40Gi, storageGB: 40, instances: 1, servingPool: xxl}
`,
		"paused default valkey tier": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1, paused: true}
`,
		"empty postgres family": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers: []
` + validValkey,
		"empty valkey family": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
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
` + validValkey,
		"duplicate compute id": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
    - {id: starter, renderPlan: starter2, cpu: "1", memory: 1Gi, ephemeralStorage: 2Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"duplicate renderPlan": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
    - {id: starter2, renderPlan: starter, cpu: "1", memory: 1Gi, ephemeralStorage: 2Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"empty renderPlan": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: "", cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"unparseable quantity": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: not-a-quantity, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"compute tier missing ephemeralStorage (round-14 #2)": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"compute tier bad ephemeralStorage (round-14 #2)": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: not-a-quantity}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"compute default names undefined tier": `
compute:
  default: nonexistent
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"missing postgres default": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"duplicate postgres id": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"postgres zero storage": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 0, instances: 1}
` + validValkey,
		"postgres zero instances": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 0}
` + validValkey,
		"postgres tier without a connection limit": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
    - {id: gold, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"postgres default other than connlimit's": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: basic-256mb
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
    - {id: basic-256mb, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
		"a connection limit in the catalog": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1, maxConnections: 100}
` + validValkey,
		"valkey zero storage": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 0, instances: 1}
`,
		"valkey zero instances": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 0}
`,
		"valkey tier without a connection limit": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1}
    - {id: gold, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1}
`,
		"valkey default other than connlimit's": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: starter
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1}
    - {id: starter, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
`,
		"a Key Value connection limit in the catalog": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1, connectionLimit: 50}
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
` + validValkey,
		"servingPool not a node label value": `
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi, servingPool: "XL pool"}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
` + validValkey,
	}
	for name, raw := range cases {
		if _, _, _, err := parse([]byte(raw)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

// TestPostgresSupportsHighAvailability pins Render's "at least 1 CPU" HA rule
// (w8/m43, w2/m171): every shipped sub-1-CPU rung refuses, and every ≥1-CPU
// rung accepts.
func TestPostgresSupportsHighAvailability(t *testing.T) {
	for id, want := range map[string]bool{
		"free": false, "basic-256mb": false, "basic-1gb": false,
		"1c-2g": true, "pro-4gb": true, "basic-4gb": true,
	} {
		tier, ok := Postgres.ByID(id)
		if !ok {
			t.Fatalf("plan %q missing from catalog", id)
		}
		if got := tier.SupportsHighAvailability(); got != want {
			t.Errorf("%s (cpu %s): SupportsHighAvailability = %v, want %v", id, tier.CPU, got, want)
		}
	}
	for cpu, want := range map[string]bool{"999m": false, "1": true, "1000m": true, "2": true, "": false} {
		if got := (PostgresTier{CPU: cpu}).SupportsHighAvailability(); got != want {
			t.Errorf("cpu %q: SupportsHighAvailability = %v, want %v", cpu, got, want)
		}
	}
}

func TestPostgresCapacityFeatures(t *testing.T) {
	for _, tc := range []struct {
		plan     string
		paid     bool
		storage  int32
		replicas int
	}{
		{"free", false, 1, 0},
		{"basic-256mb", true, 16384, 0},
		{"basic-1gb", true, 16384, 5},
		{"1c-2g", true, 16384, 5},
		{"pro-4gb", true, 16384, 5},
		{"basic-4gb", true, 16384, 5},
	} {
		tier, ok := Postgres.ByID(tc.plan)
		if !ok {
			t.Fatal(tc.plan)
		}
		if tier.SupportsDiskAutoscaling() != tc.paid || tier.SupportsConnectionPooling() != tc.paid ||
			Postgres.MaxStorageGB(tc.plan) != tc.storage || tier.MaxReadReplicas() != tc.replicas {
			t.Fatalf("capacity features for %s: %+v", tc.plan, tier)
		}
	}
	if Postgres.MaxStorageGB("") != 1 || Postgres.MaxStorageGB("0.5c-1g") != 16384 || PostgresReadReplicaMinStorageGB != 10 {
		t.Fatal("default/alias storage or replica storage minimum drifted")
	}
	for cpu, want := range map[string]int{"": 0, "invalid": 0, "499m": 0, "0.5": 5, "500m": 5, "1": 5} {
		if got := (PostgresTier{CPU: cpu}).MaxReadReplicas(); got != want {
			t.Errorf("CPU %q: replica limit=%d, want %d", cpu, got, want)
		}
	}
}

// w5/m266: a Key Value tier may name a pool the compute ladder names.
func TestValkeyServingPool(t *testing.T) {
	_, _, v, err := parse([]byte(`
compute:
  default: starter
  tiers:
    - {id: starter, renderPlan: starter, cpu: 500m, memory: 512Mi, ephemeralStorage: 1Gi}
    - {id: big, renderPlan: big, cpu: "4", memory: 16Gi, ephemeralStorage: 1Gi, servingPool: xl}
postgres:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 256Mi, storageGB: 1, instances: 1}
valkey:
  default: free
  tiers:
    - {id: free, cpu: 100m, memory: 128Mi, storageGB: 1, instances: 1}
    - {id: pro_max, cpu: "4", memory: 20Gi, storageGB: 20, instances: 1, servingPool: xl}
`))
	if err != nil {
		t.Fatal(err)
	}
	large, _ := v.ByID("pro_max")
	free, _ := v.ByID("free")
	if large.ServingPool != "xl" || free.ServingPool != "" {
		t.Fatalf("pools: pro_max %q free %q", large.ServingPool, free.ServingPool)
	}
}
