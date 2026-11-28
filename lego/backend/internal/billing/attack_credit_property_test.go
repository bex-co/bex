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

package billing

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Property: the attack credit makes a workspace whole, exactly, when the plan
// (allowance) has not changed between export and credit (w4/m709).
//
// The emitter freezes each row's billed quantity under the windows live at
// export (attack rows 0, other rows owedAbove in (window_start,
// resource_kind, service_id) order with attack rows using none of the
// allowance). The credit re-runs that arithmetic under the current windows.
// So exported hours' net after the credit must equal what the emitter would
// have billed had the current windows been live at export — and when nothing
// was overbilled (the frozen values already match the current windows) the
// credit is exactly 0. The cross-check below recomputes the emitter side with
// an independent O(n^2) prefix-sum shape (UsageBefore semantics), not the
// credit's incremental loop, so agreement is not the function agreeing with
// itself.
func TestAttackCreditMakesExportedHoursWhole(t *testing.T) {
	const gib = int64(1 << 30)
	const mib = int64(1 << 20)
	rng := rand.New(rand.NewSource(0xa77ac6))
	const trials = 300

	var hitLateWindow, hitInTime, hitNoAttack, hitAllAttack int
	var hitZeroAllowance, hitSubCentDue, hitUnemitted, hitSameHourMultiRow int
	var hitSpillover int

	kinds := []string{"key_value", "postgres", "service"}
	svcs := []string{"srv-1", "srv-2", "srv-3"}

	for i := 0; i < trials; i++ {
		allowances := []int64{0, gib, 3 * gib}
		allowance := allowances[rng.Intn(len(allowances))]
		if allowance == 0 {
			hitZeroAllowance++
		}

		// Build rows in ledger order: hour-major, then (resource_kind,
		// service_id) like ORDER BY u.window_start, u.resource_kind,
		// u.service_id.
		base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		hours := 1 + rng.Intn(6)
		type genRow struct {
			at  time.Time
			qty int64
		}
		var gen []genRow
		target := hours * (1 + rng.Intn(len(svcs)))
		for len(gen) < target {
			h := rng.Intn(hours)
			// Mostly GiB-scale so months clear the allowance (spillover
			// needs usage above it); sometimes MiB-scale for sub-cent dues.
			qty := (1 + int64(rng.Intn(8))) * 512 * mib
			if rng.Intn(100) < 25 {
				qty = (1 + int64(rng.Intn(64))) * mib
			}
			gen = append(gen, genRow{at: base.Add(time.Duration(h) * time.Hour), qty: qty})
		}
		sort.Slice(gen, func(a, b int) bool { return gen[a].at.Before(gen[b].at) })

		// Sprinkle rows across kinds/services within each hour; keep the
		// global (window_start, resource_kind, service_id) order.
		rows := make([]store.EgressMonthRow, 0, len(gen))
		sameHourMulti := false
		seenHour := map[time.Time]int{}
		for _, g := range gen {
			seenHour[g.at]++
			if seenHour[g.at] > 1 {
				sameHourMulti = true
			}
			rows = append(rows, store.EgressMonthRow{
				WindowStart: g.at,
				Quantity:    g.qty,
				Emitted:     true,
				// Billed filled below by the emitter simulation.
			})
		}
		// Order ties within an hour by (resource_kind, service_id).
		sort.SliceStable(rows, func(a, b int) bool {
			if rows[a].WindowStart.Equal(rows[b].WindowStart) {
				if kinds[a%len(kinds)] != kinds[b%len(kinds)] {
					return kinds[a%len(kinds)] < kinds[b%len(kinds)]
				}
				return svcs[a%len(svcs)] < svcs[b%len(svcs)]
			}
			return rows[a].WindowStart.Before(rows[b].WindowStart)
		})
		if sameHourMulti {
			hitSameHourMultiRow++
		}

		// Windows only added since export: every row attacked at export is
		// still attacked at credit time; other rows may have become so.
		emitAttack := make([]bool, len(rows))
		creditAttack := make([]bool, len(rows))
		for r := range rows {
			if rng.Intn(100) < 30 {
				emitAttack[r] = true
			}
			creditAttack[r] = emitAttack[r] || rng.Intn(100) < 30
			// Some rows have not shipped yet: the emitter's to zero, not
			// the credit's.
			if rng.Intn(100) < 20 {
				rows[r].Emitted = false
			}
		}
		added := false
		anyCredit, allCredit := false, true
		for r := range rows {
			if creditAttack[r] && !emitAttack[r] {
				added = true
			}
			if creditAttack[r] {
				anyCredit = true
			} else {
				allCredit = false
			}
		}
		switch {
		case !anyCredit:
			hitNoAttack++
		case allCredit:
			hitAllAttack++
		case added:
			hitLateWindow++
		default:
			hitInTime++
		}
		unemitted := false
		for r := range rows {
			if !rows[r].Emitted {
				unemitted = true
			}
		}
		if unemitted {
			hitUnemitted++
		}

		// Emitter simulation under the export-time windows (independent
		// O(n^2) prefix sums in ledger order, UsageBefore semantics: each
		// row's before sums the quantities of earlier non-attack rows).
		emitBilled := make([]int64, len(rows))
		for r := range rows {
			if emitAttack[r] {
				emitBilled[r] = 0
				continue
			}
			var before int64
			for j := 0; j < r; j++ {
				if !emitAttack[j] {
					before += rows[j].Quantity
				}
			}
			if allowance > 0 {
				emitBilled[r] = owedAbove(before, rows[r].Quantity, allowance)
			} else {
				emitBilled[r] = rows[r].Quantity
			}
		}
		for r := range rows {
			rows[r].Billed = emitBilled[r]
			rows[r].AttackHour = creditAttack[r]
		}

		due := attackCreditBytes(rows, allowance)

		// Independent counterfactual: what the emitter would have billed the
		// exported rows had the credit-time windows been live at export.
		var billedTotal, counterfactual int64
		for r := range rows {
			if !rows[r].Emitted {
				continue
			}
			billedTotal += emitBilled[r]
			if creditAttack[r] {
				continue
			}
			var before int64
			for j := 0; j < r; j++ {
				if !creditAttack[j] {
					before += rows[j].Quantity
				}
			}
			o := rows[r].Quantity
			if allowance > 0 {
				o = owedAbove(before, rows[r].Quantity, allowance)
			}
			counterfactual += o
		}
		if got, want := billedTotal-due, counterfactual; got != want {
			t.Fatalf("trial %d: billed %d - credit %d = %d, want the current-window rating %d (allowance %d)",
				i, billedTotal, due, got, want, allowance)
		}
		if due < 0 {
			t.Fatalf("trial %d: negative credit %d", i, due)
		}

		// When the frozen values already match the current windows, the
		// credit is exactly 0 — no phantom credit.
		if !added {
			// Recompute: emit flags equal credit flags, so every emitted
			// row already bills the current-window value.
			if due != 0 {
				t.Fatalf("trial %d: nothing overbilled yet credit %d bytes", i, due)
			}
		}

		// Spillover coverage: a non-attack row that paid only because attack
		// bytes consumed the allowance at export.
		for r := range rows {
			if !rows[r].Emitted || creditAttack[r] || emitAttack[r] || allowance == 0 {
				continue
			}
			var beforeEmit, beforeCredit int64
			for j := 0; j < r; j++ {
				if !emitAttack[j] {
					beforeEmit += rows[j].Quantity
				}
				if !creditAttack[j] {
					beforeCredit += rows[j].Quantity
				}
			}
			if owedAbove(beforeEmit, rows[r].Quantity, allowance) > owedAbove(beforeCredit, rows[r].Quantity, allowance) {
				hitSpillover++
				break
			}
		}

		if due > 0 && pricing.Default.BandwidthCents(due) == 0 {
			hitSubCentDue++
		}
	}

	if hitLateWindow == 0 || hitInTime == 0 || hitNoAttack == 0 || hitAllAttack == 0 ||
		hitZeroAllowance == 0 || hitSubCentDue == 0 || hitUnemitted == 0 ||
		hitSameHourMultiRow == 0 || hitSpillover == 0 {
		t.Fatalf("property missed branches: late=%d inTime=%d none=%d all=%d zeroAllow=%d subCent=%d unemitted=%d multiRow=%d spillover=%d",
			hitLateWindow, hitInTime, hitNoAttack, hitAllAttack, hitZeroAllowance, hitSubCentDue, hitUnemitted, hitSameHourMultiRow, hitSpillover)
	}
}
