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

package pricing

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
)

// w4/m690: every row of Render's price tables on render.com/pricing — the
// workspace plans, compute, Postgres, Key Value, cron jobs, Workflows and
// Sandboxes — and every question of its pricing FAQ, captured in testdata/ on
// 2026-11-26, maps to exactly one disposition: a bex tier at Render's price ×
// 0.70 (ADR030 §2) to the cent, a gap with the board item that owns it, a
// non-goal, or the bex rule that answers a question. A row Render adds, or one
// bex forgets, fails here; so does a disposition whose row is gone, and a bex
// price one cent off Render's × 0.70. TestRenderPricingDrift compares Render's
// live pages with every fixture (scripts/render-pricing-drift.sh, weekly).
// ADR030 §9 lists the same dispositions.

// priceFixtures is each captured price table: its file in testdata/ (and in
// the directory scripts/lib/render_pricing_page.py writes), the prefix of its
// rows' keys, its cells per row and the cell that names a row.
var priceFixtures = []struct {
	file, table string
	width, key  int
}{
	{"render-plan-prices.tsv", "plans", 3, 0},
	{"render-compute-prices.tsv", "compute", 4, 3},
	{"render-postgres-prices.tsv", "postgres", 5, 4},
	{"render-keyvalue-prices.tsv", "keyvalue", 6, 5},
	{"render-cron-prices.tsv", "cron", 4, 3},
	{"render-workflow-prices.tsv", "workflows", 4, 3},
	{"render-sandbox-prices.tsv", "sandboxes", 5, 4},
	{"render-pricing-faq.tsv", "faq", 2, 0},
}

// readPriceTable reads a captured price table: each row keyed by its naming
// cell, its values every cell as displayed.
func readPriceTable(t *testing.T, path, table string, width, key int) []planRow {
	t.Helper()
	var rows []planRow
	for _, cells := range readTSV(t, path, width) {
		rows = append(rows, planRow{key: table + ": " + cells[key], values: cells})
	}
	return rows
}

func capturedPriceRows(t *testing.T, dir string) []planRow {
	t.Helper()
	var rows []planRow
	for _, f := range priceFixtures {
		rows = append(rows, readPriceTable(t, filepath.Join(dir, f.file), f.table, f.width, f.key)...)
	}
	return rows
}

// splitPrice splits a price cell into its leading dollar amount and the rest:
// "$1,000/month" is "1,000" and "/month".
func splitPrice(cell string) (amount, rest string, ok bool) {
	amount, ok = strings.CutPrefix(cell, "$")
	end := strings.IndexFunc(amount, func(r rune) bool { return !unicode.IsDigit(r) && r != '.' && r != ',' })
	if end < 0 {
		end = len(amount)
	}
	return amount[:end], amount[end:], ok
}

// renderUSD reads a price cell's leading dollar amount: "$1,000/month" is
// 1000, "$0.00016/minute" 0.00016, "$0.12 / active CPU hour" 0.12.
func renderUSD(cell string) (float64, bool) {
	amount, _, ok := splitPrice(cell)
	if !ok {
		return 0, false
	}
	usd, err := strconv.ParseFloat(strings.ReplaceAll(amount, ",", ""), 64)
	return usd, err == nil
}

// cents is a dollar amount to the nearest cent.
func cents(usd float64) int64 { return int64(math.Round(usd * 100)) }

var ramCell = regexp.MustCompile(`^(\d+(?:\.\d+)?) ?(MB|GB) RAM$`)

// renderRAM reads a RAM cell ("512 MB RAM", "2GB RAM") in bytes; Render's
// MB and GB are the binary sizes the tier catalog allocates.
func renderRAM(cell string) (int64, bool) {
	m := ramCell.FindStringSubmatch(cell)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	unit := float64(1 << 20)
	if m[2] == "GB" {
		unit = 1 << 30
	}
	return int64(n * unit), true
}

// renderConnections reads a connections cell ("3,000 connections").
func renderConnections(cell string) (int32, bool) {
	n, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSuffix(cell, " connections"), ",", ""), 10, 32)
	return int32(n), err == nil
}

// keyValueConnectionDisagreements are the Key Value plans whose connection
// limit on render.com/pricing differs from render.com/docs/compute-plans
// (both read 2026-11-26): plan ID → the pricing page's figure. bex enforces
// compute-plans' 250 and 1,000 (connlimit.Valkey, w6/m456): it is Render's
// plan reference, listing each plan ID beside its legacy name and limit; its
// figures follow the one connection per megabyte every larger plan has on
// both pages; and the higher limit never refuses a client Render would admit.
// ADR030 §9 records the choice.
var keyValueConnectionDisagreements = map[string]int32{"256mb": 200, "1g": 250}

// instanceTier is a Render plan bex sells as one tier: the tier's monthly
// price is Render's × 0.70 to the cent, it runs Render's RAM (and, for a
// datastore, Render's connection limit), and the tier catalog's aliases
// resolve the plan ID to it.
func instanceTier(resourceKind, tier string) planDisposition {
	family := map[string]string{store.ResourceKindService: "compute", store.ResourceKindPostgres: "postgres", store.ResourceKindKeyValue: "keyvalue"}[resourceKind]
	ref := fmt.Sprintf("pricing.yaml %s %s", family, tier)
	return planDisposition{kind: "tier", ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		planID, price := row.values[len(row.values)-1], row.values[1]
		render, ok := renderUSD(price)
		if !ok {
			t.Errorf("%s: unreadable price %q", row.key, price)
			return
		}
		rate, listed := Default.instanceRate(tier, resourceKind)
		if !listed {
			t.Errorf("%s: %s is not on the price sheet", row.key, ref)
			return
		}
		if got, want := cents(rate*MonthSeconds), cents(render*0.70); got != want {
			t.Errorf("%s: Render %s × 0.70 = $%.2f a month, but %s is $%.2f: move the tier to Render's price less 30%%", row.key, price, float64(want)/100, ref, float64(got)/100)
		}
		memory, connections, resolves := "", int32(-1), false
		switch resourceKind {
		case store.ResourceKindService:
			c, _ := tiers.Compute.ByID(tier)
			memory, resolves = c.Memory, tiers.ComputeSpecID(tier) == planID
		case store.ResourceKindPostgres:
			p, _ := tiers.Postgres.ByID(tier)
			memory, connections, resolves = p.Memory, p.MaxConnections, tiers.PostgresSpecID(tier) == planID
		case store.ResourceKindKeyValue:
			v, _ := tiers.Valkey.ByID(tier)
			memory, connections, resolves = v.Memory, v.ConnectionLimit, tiers.Valkey.CanonicalID(planID) == tier
		}
		if !resolves {
			t.Errorf("%s: the tier catalog does not resolve Render's plan ID %q to %s", row.key, planID, tier)
		}
		// The one RAM bex sells differently: Render's free Key Value has 25 MB,
		// bex's 128 MiB with a disk, the more generous divergence ADR018 § Other
		// datastores and ADR021 record.
		if !(resourceKind == store.ResourceKindKeyValue && tier == "free") {
			checkRAM(t, row, ref, memory)
		}
		if connections >= 0 {
			checkConnections(t, row, ref, connections)
		}
	}}
}

func checkRAM(t reporter, row planRow, ref, memory string) {
	t.Helper()
	want, ok := renderRAM(row.values[2])
	if !ok {
		t.Errorf("%s: unreadable RAM %q", row.key, row.values[2])
		return
	}
	q, err := resource.ParseQuantity(memory)
	if err != nil || q.Value() != want {
		t.Errorf("%s: Render's plan has %s, %s runs %q (tiers.yaml)", row.key, row.values[2], ref, memory)
	}
}

func checkConnections(t reporter, row planRow, ref string, limit int32) {
	t.Helper()
	cell := row.values[len(row.values)-2]
	shown, ok := renderConnections(cell)
	if !ok {
		t.Errorf("%s: unreadable connections %q", row.key, cell)
		return
	}
	planID := row.values[len(row.values)-1]
	if pricingPage, disagrees := keyValueConnectionDisagreements[planID]; disagrees && strings.HasPrefix(row.key, "keyvalue: ") {
		if shown != pricingPage {
			t.Errorf("%s: render.com/pricing now shows %s, where it showed %d against compute-plans' %d: re-read both pages and update keyValueConnectionDisagreements and ADR030 §9", row.key, cell, pricingPage, limit)
		}
		return
	}
	if shown != limit {
		t.Errorf("%s: Render allows %s, %s allows %d (connlimit)", row.key, cell, ref, limit)
	}
}

// cronMinutesPerMonth is the month Render prices a cron job's minute over:
// each row of its cron table is the same plan's monthly compute price over 30
// days (43,200 minutes), shown to five decimals ($7 → $0.00016, $85 →
// $0.00197). bex bills a cron run per second at the plan's compute rate,
// derived over 730 hours (MonthSeconds), so its minute is Render's × 0.70 ×
// 720/730, 1.4% under ADR030's promise: w4/752 owns the difference.
const cronMinutesPerMonth = 30 * 24 * 60

// cronTier is a Render cron plan run on a bex compute tier: Render's cron
// minute is the monthly price the compute check holds the tier to, over
// Render's 30-day month.
func cronTier(tier string) planDisposition {
	ref := fmt.Sprintf("pricing.yaml compute %s (a run billed per second, ADR030 §7; the 720/730 month is w4/752)", tier)
	return planDisposition{kind: "tier", ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		planID, price := row.values[3], row.values[1]
		perMinute, ok := renderUSD(price)
		if !ok || !strings.HasSuffix(price, "/minute") {
			t.Errorf("%s: unreadable price per minute %q", row.key, price)
			return
		}
		rate, _ := Default.instanceRate(tier, store.ResourceKindService)
		renderMonthly := rate * MonthSeconds / 0.70
		if implied := renderMonthly / cronMinutesPerMonth; math.Abs(implied-perMinute) > 0.000005+1e-12 {
			t.Errorf("%s: Render's cron minute is %s, but %s is Render's $%.2f/month × 0.70, which over Render's 30-day month is $%.6f a minute: Render's cron and compute prices moved apart; move the tier or price cron on its own (w4/752)", row.key, price, ref, renderMonthly, implied)
		}
		if tiers.ComputeSpecID(tier) != planID {
			t.Errorf("%s: the tier catalog does not resolve Render's plan ID %q to %s", row.key, planID, tier)
		}
		c, _ := tiers.Compute.ByID(tier)
		checkRAM(t, row, ref, c.Memory)
	}}
}

// planFee is a workspace plan's monthly fee: Render's × 0.70 to the cent.
func planFee(plan string) planDisposition {
	ref := "pricing.yaml workspace " + plan
	return planDisposition{kind: "tier", ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		render, ok := renderUSD(row.values[1])
		if !ok || !strings.HasSuffix(row.values[1], "/month + compute") {
			t.Errorf("%s: unreadable plan price %q", row.key, row.values[1])
			return
		}
		usd, listed := Default.WorkspaceUSDPerMonth(plan)
		if got, want := cents(usd), cents(render*0.70); !listed || got != want {
			t.Errorf("%s: Render %s × 0.70 = $%.2f a month, but %s is $%.2f (listed %v)", row.key, row.values[1], float64(want)/100, ref, usd, listed)
		}
	}}
}

// sandboxRates holds bex's sandbox rates to Render's × 0.70 per hour: the CPU
// a running sandbox uses, and the memory it is allocated per GB, Render's
// memory price over the row's RAM.
func sandboxRates() planDisposition {
	ref := "pricing.yaml sandboxUsage (ADR104 D8)"
	return planDisposition{kind: "tier", ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		cpu, cpuOK := renderUSD(row.values[1])
		memory, memoryOK := renderUSD(row.values[3])
		ram, ramOK := renderRAM(row.values[2])
		if !cpuOK || !memoryOK || !ramOK || !strings.HasSuffix(row.values[1], " / active CPU hour") {
			t.Errorf("%s: unreadable sandbox prices %q", row.key, row.values)
			return
		}
		if got, want := Default.sandboxCPU*3600, cpu*0.70; math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: Render %s × 0.70 = $%.4f a CPU-hour, but pricing.yaml sandboxUsage usdPerCPUSecond × 3,600 = $%.4f", row.key, row.values[1], want, got)
		}
		perGB := memory / (float64(ram) / (1 << 30))
		if got, want := Default.sandboxMemory*3600, perGB*0.70; math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: Render %s for %s is $%.4f a GB-hour, × 0.70 = $%.5f, but pricing.yaml sandboxUsage usdPerMemoryGBSecond × 3,600 = $%.5f", row.key, row.values[3], row.values[2], perGB, want, got)
		}
	}}
}

func rule(ref string) planDisposition { return planDisposition{kind: "rule", ref: ref} }

// The board items that own the Render plans bex cannot run yet.
const (
	gapMidSize      = "w6/blocked/m332 (cx53 capacity: 2c-8g, 2c-16g, 8c-16g, 12c-24g)"
	gapLarge        = "w6/blocked/114 (dedicated-vCPU limit: 4c-32g, 8c-64g, 12c-48g, 12c-96g)"
	gapPostgresSize = "w6/192 (Postgres plans above 4 GB)"
	workflows       = "DO_NOT_DO: Workflows & tasks; ADR018 Workflows & tasks"
)

// priceDispositions is bex's answer to every captured price row and FAQ
// question.
var priceDispositions = map[string]planDisposition{
	// render.com/pricing § Pricing Tiers
	"plans: Hobby": planFee(store.PlanHobby),
	"plans: Pro":   planFee(store.PlanPro),
	"plans: Scale": planFee(store.PlanScale),
	"plans: Enterprise": {kind: "commercial", ref: "a contract price: pricing.yaml lists no Enterprise fee", check: func(t reporter, row planRow) {
		t.Helper()
		if _, listed := Default.WorkspaceUSDPerMonth(store.PlanEnterprise); listed || row.values[1] != "Custom pricing" {
			t.Errorf("%s: Render shows %q and bex lists a catalog fee = %v; Enterprise is priced by contract on both", row.key, row.values[1], listed)
		}
	}},

	// § Compute > Services & Workers
	"compute: free":       instanceTier(store.ResourceKindService, "free"),
	"compute: 0.5c-512mb": instanceTier(store.ResourceKindService, "starter"),
	"compute: 1c-2g":      instanceTier(store.ResourceKindService, "standard"),
	"compute: 2c-4g":      instanceTier(store.ResourceKindService, "pro"),
	"compute: 4c-8g":      instanceTier(store.ResourceKindService, "pro-plus"),
	"compute: 4c-16g":     instanceTier(store.ResourceKindService, "pro-max"),
	"compute: 8c-32g":     instanceTier(store.ResourceKindService, "pro-ultra"),
	"compute: 2c-8g":      gap(gapMidSize),
	"compute: 2c-16g":     gap(gapMidSize),
	"compute: 8c-16g":     gap(gapMidSize),
	"compute: 12c-24g":    gap(gapMidSize),
	"compute: 4c-32g":     gap(gapLarge),
	"compute: 8c-64g":     gap(gapLarge),
	"compute: 12c-48g":    gap(gapLarge),
	"compute: 12c-96g":    gap(gapLarge),

	// § Compute > Postgres
	"postgres: free":       instanceTier(store.ResourceKindPostgres, "free"),
	"postgres: 0.1c-256mb": instanceTier(store.ResourceKindPostgres, "basic-256mb"),
	"postgres: 0.5c-1g":    instanceTier(store.ResourceKindPostgres, "basic-1gb"),
	"postgres: 1c-2g":      instanceTier(store.ResourceKindPostgres, "1c-2g"),
	"postgres: 1c-4g":      instanceTier(store.ResourceKindPostgres, "pro-4gb"),
	"postgres: 2c-4g":      instanceTier(store.ResourceKindPostgres, "basic-4gb"),
	"postgres: 2c-8g":      gap(gapPostgresSize),
	"postgres: 2c-16g":     gap(gapPostgresSize),
	"postgres: 4c-16g":     gap(gapPostgresSize),
	"postgres: 4c-32g":     gap(gapPostgresSize),
	"postgres: 8c-32g":     gap(gapPostgresSize),
	"postgres: 8c-64g":     gap(gapPostgresSize),
	"postgres: 16c-64g":    gap(gapPostgresSize),
	"postgres: 16c-128g":   gap(gapPostgresSize),
	"postgres: 32c-128g":   gap(gapPostgresSize),
	"postgres: 32c-256g":   gap(gapPostgresSize),
	"postgres: 48c-192g":   gap(gapPostgresSize),
	"postgres: 48c-384g":   gap(gapPostgresSize),
	"postgres: 64c-256g":   gap(gapPostgresSize),
	"postgres: 64c-512g":   gap(gapPostgresSize),
	"postgres: 96c-384g":   gap(gapPostgresSize),
	"postgres: 96-768g":    gap(gapPostgresSize),
	"postgres: 128c-512g":  gap(gapPostgresSize),
	"postgres: 128c-1024g": gap(gapPostgresSize),

	// § Compute > Key Value
	"keyvalue: free":  instanceTier(store.ResourceKindKeyValue, "free"),
	"keyvalue: 256mb": instanceTier(store.ResourceKindKeyValue, "starter"),
	"keyvalue: 1g":    instanceTier(store.ResourceKindKeyValue, "standard"),
	"keyvalue: 3g":    instanceTier(store.ResourceKindKeyValue, "3g"),
	"keyvalue: 5g":    instanceTier(store.ResourceKindKeyValue, "pro"),
	"keyvalue: 10g":   instanceTier(store.ResourceKindKeyValue, "pro_plus"),
	"keyvalue: 20g":   instanceTier(store.ResourceKindKeyValue, "pro_max"),
	"keyvalue: 40g":   instanceTier(store.ResourceKindKeyValue, "pro_ultra"),
	"keyvalue: 50g":   instanceTier(store.ResourceKindKeyValue, "50g"),

	// § Compute > Cron Jobs
	"cron: 0.5c-512mb": cronTier("starter"),
	"cron: 1c-2g":      cronTier("standard"),
	"cron: 2c-4g":      cronTier("pro"),
	"cron: 4c-8g":      cronTier("pro-plus"),
	"cron: 4c-16g":     cronTier("pro-max"),
	"cron: 8c-32g":     cronTier("pro-ultra"),
	"cron: 2c-8g":      gap(gapMidSize),
	"cron: 2c-16g":     gap(gapMidSize),
	"cron: 8c-16g":     gap(gapMidSize),
	"cron: 4c-32g":     gap(gapLarge),
	"cron: 8c-64g":     gap(gapLarge),

	// § Compute > Workflows
	"workflows: flex":   nonGoal(workflows),
	"workflows: 2c-4g":  nonGoal(workflows),
	"workflows: 2c-8g":  nonGoal(workflows),
	"workflows: 4c-8g":  nonGoal(workflows),
	"workflows: 4c-16g": nonGoal(workflows),
	"workflows: 8c-32g": nonGoal(workflows),

	// § Compute > Sandboxes
	"sandboxes: 1c-2g":  sandboxRates(),
	"sandboxes: 2c-4g":  sandboxRates(),
	"sandboxes: 4c-8g":  sandboxRates(),
	"sandboxes: 8c-16g": sandboxRates(),

	// § Frequently Asked Questions
	"faq: How does billing work?": rule("ADR030 §2: a plan fee (pricing.yaml workspace), metered usage and compute"),

	"faq: How am I billed for compute?": rule("pricing.yaml compute usdPerSecond: instance_seconds billed per second (ADR023), a cron job only while a run executes (ADR030 §7)"),

	"faq: My Pro or Scale workspace did not have any activity or services this month. Will I still be charged for the workspace subscription?": rule("ADR030 §19, w4/blocked/m689: bex charges no plan fee yet, so it has none to waive"),

	"faq: I run a large agency with developers on multiple teams. What does this mean for us?": rule("core.OrganizationPlan: a Scale or Enterprise organization holds any number of workspaces (ADR115)"),

	"faq: What are build pipeline minutes, and how can I track my usage?": rule("ADR030 §13, pricing.yaml build.includedMinutes; the Performance pipeline is a gap, w6/m310"),

	"faq: How do free compute plans work?": rule("tiers.yaml free web service, Postgres and Key Value tiers at $0 (ADR030 §7)"),

	"faq: Which payment methods do you accept?": rule("ADR040: cards through Stripe Checkout"),

	"faq: Why was I charged $1 after adding a credit card?": rule("ADR040 payment onboarding: a setup-mode SetupIntent saves the card, and bex takes no charge for it"),

	"faq: We are migrating infrastructure from another provider. Can Render provide professional services to handle the migration?": commercial("professional services"),

	"faq: I have an infrequently asked question.": commercial("support channel"),
}

var boardItem = regexp.MustCompile(`\bw\d+/(?:blocked/)?(?:m\d+|\d{3})\b`)

func TestEveryRenderPriceRowHasADisposition(t *testing.T) {
	rows := capturedPriceRows(t, "testdata")
	if len(rows) < 80 {
		t.Fatalf("captured %d rows; the price tables and the FAQ hold 83", len(rows))
	}
	missing, stale := undispositioned(rows, priceDispositions)
	for _, key := range missing {
		t.Errorf("Render price row %q has no disposition in bex: add a tier at Render's price × 0.70, a gap with its owner, a non-goal or a rule", key)
	}
	for _, key := range stale {
		t.Errorf("disposition %q names a row Render's captured price tables no longer have", key)
	}
	kinds := []string{"tier", "gap", "nonGoal", "rule", "commercial"}
	for _, row := range rows {
		d, ok := priceDispositions[row.key]
		if !ok {
			continue
		}
		if !slices.Contains(kinds, d.kind) || d.ref == "" {
			t.Errorf("%s: disposition %+v needs one of %v and a reference", row.key, d, kinds)
		}
		if d.kind == "gap" && !boardItem.MatchString(d.ref) {
			t.Errorf("%s: a gap names the board item that owns it, not %q", row.key, d.ref)
		}
		if d.check != nil {
			t.Run(row.key, func(t *testing.T) { d.check(t, row) })
		}
	}
}

// Every tier bex prices is the disposition of a Render row: a tier no
// captured row names would be priced against nothing.
func TestEveryPricedTierAnswersARenderRow(t *testing.T) {
	answered := map[string]bool{}
	for _, d := range priceDispositions {
		answered[d.ref] = true
	}
	for family, rates := range map[string]map[string]float64{"compute": Default.compute, "postgres": Default.postgres, "keyvalue": Default.keyvalue} {
		for tier := range rates {
			if ref := fmt.Sprintf("pricing.yaml %s %s", family, tier); !answered[ref] {
				t.Errorf("%s is priced but answers no row of Render's %s table", ref, family)
			}
		}
	}
}

// bumpPrice moves a price cell's leading amount by delta, keeping its suffix.
func bumpPrice(t *testing.T, cell string, delta float64) string {
	t.Helper()
	usd, ok := renderUSD(cell)
	_, rest, _ := splitPrice(cell)
	if !ok {
		t.Fatalf("unreadable price %q", cell)
	}
	return "$" + strconv.FormatFloat(usd+delta, 'f', -1, 64) + rest
}

// The guard itself: a Render price a cent away from the one each tier is
// held to fails that tier's check, naming the pricing.yaml entry to move;
// the captured price passes.
func TestPriceTableGuardFailsOnACentAndNamesTheTier(t *testing.T) {
	checked := 0
	for _, row := range capturedPriceRows(t, "testdata") {
		d := priceDispositions[row.key]
		if d.kind != "tier" || d.check == nil {
			continue
		}
		var kept failures
		d.check(&kept, row)
		if kept.failed {
			t.Errorf("%s: the captured row must pass: %v", row.key, kept.messages)
		}
		for _, delta := range []float64{0.01, -0.01} {
			if usd, _ := renderUSD(row.values[1]); usd+delta < 0 {
				continue
			}
			moved := planRow{key: row.key, values: slices.Clone(row.values)}
			moved.values[1] = bumpPrice(t, row.values[1], delta)
			var f failures
			d.check(&f, moved)
			if !f.failed || !strings.Contains(strings.Join(f.messages, "\n"), strings.SplitN(d.ref, " (", 2)[0]) {
				t.Errorf("%s at %s: the check must fail and name %q, got %v", row.key, moved.values[1], d.ref, f.messages)
			}
		}
		checked++
	}
	if checked < 30 {
		t.Fatalf("only %d tier rows checked", checked)
	}
	// A cron minute one displayed digit off fails too.
	row := planRow{key: "cron: 2c-4g", values: []string{"2 CPU", "$0.00198/minute", "4 GB RAM", "2c-4g"}}
	var f failures
	priceDispositions[row.key].check(&f, row)
	if !f.failed {
		t.Error("a cron minute one digit off Render's must fail")
	}
}

// A removed row leaves its disposition stale, and an added one has none.
func TestPriceTableGuardReportsAddedAndRemovedRows(t *testing.T) {
	rows := capturedPriceRows(t, "testdata")
	added := append(slices.Clone(rows), planRow{key: "compute: 16c-128g", values: []string{"16 CPU", "$2,000/month", "128 GB RAM", "16c-128g"}})
	if missing, _ := undispositioned(added, priceDispositions); !slices.Equal(missing, []string{"compute: 16c-128g"}) {
		t.Fatalf("an added plan must be reported, got %v", missing)
	}
	removed := slices.DeleteFunc(slices.Clone(rows), func(r planRow) bool { return r.key == "compute: 0.5c-512mb" })
	if _, stale := undispositioned(removed, priceDispositions); !slices.Equal(stale, []string{"compute: 0.5c-512mb"}) {
		t.Fatalf("a removed plan must be reported, got %v", stale)
	}
}

// The Key Value connection disagreement holds both ways: Render's pricing
// page moving to compute-plans' figure fails, as does any other limit
// leaving connlimit's.
func TestKeyValueConnectionDisagreementIsChecked(t *testing.T) {
	for _, tt := range []struct {
		planID, cell string
		fail         bool
	}{
		{"256mb", "200 connections", false},
		{"256mb", "250 connections", true},
		{"1g", "1,000 connections", true},
		{"3g", "3,000 connections", false},
		{"3g", "3,500 connections", true},
	} {
		row := planRow{key: "keyvalue: " + tt.planID, values: []string{"Standard CPU", "$0/month", "0 MB RAM", "-", tt.cell, tt.planID}}
		tier := tiers.Valkey.CanonicalID(tt.planID)
		v, _ := tiers.Valkey.ByID(tier)
		var f failures
		checkConnections(&f, row, "pricing.yaml keyvalue "+tier, v.ConnectionLimit)
		if f.failed != tt.fail {
			t.Errorf("%s with %s: failed = %v, want %v (%v)", tt.planID, tt.cell, f.failed, tt.fail, f.messages)
		}
	}
}

// renderPricingRows is every captured row of Render's pricing pages under dir.
func renderPricingRows(t *testing.T, dir string) []planRow {
	t.Helper()
	return append(capturedPlanRows(t, dir), capturedPriceRows(t, dir)...)
}

// dispositionRef names what bex keeps for a captured row.
func dispositionRef(key string) string {
	for _, dispositions := range []map[string]planDisposition{planDispositions, priceDispositions} {
		if d, ok := dispositions[key]; ok {
			return d.kind + ": " + d.ref
		}
	}
	return "no disposition"
}

// pricingDrift is every row that differs between the fixtures and the rows
// read from Render's live pages: changed, added, removed or doubled, each
// naming the disposition it moves.
func pricingDrift(fixture, live []planRow) []string {
	captured := map[string][]string{}
	for _, row := range fixture {
		captured[row.key] = row.values
	}
	var drift []string
	seen := map[string]bool{}
	for _, row := range live {
		was, ok := captured[row.key]
		switch {
		case seen[row.key]:
			drift = append(drift, fmt.Sprintf("doubled: Render now shows %q twice", row.key))
		case !ok:
			drift = append(drift, fmt.Sprintf("added: %s %q (%s)", row.key, row.values, dispositionRef(row.key)))
		case !slices.Equal(was, row.values):
			drift = append(drift, fmt.Sprintf("changed: %s was %q, Render now shows %q (%s)", row.key, was, row.values, dispositionRef(row.key)))
		}
		seen[row.key] = true
	}
	for _, row := range fixture {
		if !seen[row.key] {
			drift = append(drift, fmt.Sprintf("removed: %s %q (%s)", row.key, row.values, dispositionRef(row.key)))
		}
	}
	return drift
}

func TestPricingDriftNamesEachChangedAddedAndRemovedRow(t *testing.T) {
	fixture := renderPricingRows(t, "testdata")
	if drift := pricingDrift(fixture, fixture); len(drift) != 0 {
		t.Fatalf("the fixtures against themselves must not drift: %v", drift)
	}
	live := slices.Clone(fixture)
	for i, row := range live {
		switch row.key {
		case "compute: 0.5c-512mb":
			live[i] = planRow{key: row.key, values: []string{"Less than 1 CPU", "$8/month", "512 MB RAM", "0.5c-512mb"}}
		case "pricing: Instant rollbacks":
			live[i] = planRow{key: row.key, values: []string{"5 builds retained", "20 builds retained", "30 builds retained", "30 builds retained"}}
		}
	}
	live = slices.DeleteFunc(live, func(r planRow) bool { return r.key == "keyvalue: 50g" })
	live = append(live, planRow{key: "postgres: 256c-2048g", values: []string{"256 CPU", "$20,000/month", "2048GB RAM", "500 connections", "256c-2048g"}},
		planRow{key: "cron: 1c-2g", values: []string{"1 CPU", "$0.00058/minute", "2 GB RAM", "1c-2g"}})
	want := []string{
		`changed: compute: 0.5c-512mb was ["Less than 1 CPU" "$7/month" "512 MB RAM" "0.5c-512mb"], Render now shows ["Less than 1 CPU" "$8/month" "512 MB RAM" "0.5c-512mb"] (tier: pricing.yaml compute starter)`,
		`changed: pricing: Instant rollbacks was ["5 builds retained" "15 builds retained" "30 builds retained" "30 builds retained"], Render now shows ["5 builds retained" "20 builds retained" "30 builds retained" "30 builds retained"] (limit: ADR060, core.KeptBuilds)`,
		`added: postgres: 256c-2048g ["256 CPU" "$20,000/month" "2048GB RAM" "500 connections" "256c-2048g"] (no disposition)`,
		`doubled: Render now shows "cron: 1c-2g" twice`,
		`removed: keyvalue: 50g ["High CPU" "$750/month" "50 GB RAM" "50 GB storage" "50,000 connections" "50g"] (tier: pricing.yaml keyvalue 50g)`,
	}
	got := pricingDrift(fixture, live)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("drift =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRenderPricingDrift compares Render's live pricing pages, as
// scripts/render-pricing-drift.sh reads them into BEX_RENDER_PRICING_DIR,
// with the fixtures: any changed, added or removed row fails, naming the bex
// disposition it moves, and each live row is held to its disposition's check,
// which names the price a moved tier needs.
func TestRenderPricingDrift(t *testing.T) {
	dir := os.Getenv("BEX_RENDER_PRICING_DIR")
	if dir == "" {
		t.Skip("BEX_RENDER_PRICING_DIR is unset: scripts/render-pricing-drift.sh sets it to the rows it read from render.com")
	}
	live := renderPricingRows(t, dir)
	for _, line := range pricingDrift(renderPricingRows(t, "testdata"), live) {
		t.Error("Render pricing " + line)
	}
	for _, row := range live {
		d, ok := planDispositions[row.key]
		if !ok {
			d = priceDispositions[row.key]
		}
		if d.check != nil {
			t.Run(row.key, func(t *testing.T) { d.check(t, row) })
		}
	}
}
