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

package agentsession

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// chargerFake answers every threshold charge with outcome (or err), first
// running onCharge (the charge's effect on the ledger).
type chargerFake struct {
	outcome  ThresholdChargeOutcome
	err      error
	onCharge func()
	calls    []ThresholdCharge
}

func (f *chargerFake) ChargeThreshold(_ context.Context, c ThresholdCharge) (ThresholdChargeOutcome, error) {
	f.calls = append(f.calls, c)
	if f.onCharge != nil {
		f.onCharge()
	}
	return f.outcome, f.err
}

// ADR091 D10.2a: a platform-route mint whose unbilled spend reaches the tier's
// threshold charges the card in that request. Paid, the request is served (the
// status is re-read: the workspace is T1, so T0's $10 ceiling no longer
// binds); declined, it is refused with BILLING_BALANCE_DUE; undecided or
// failed, with the spend cap. Without a charger (billing off), or for a
// workspace bex cannot charge, T0 stops at $10.
func TestMintChargesTheThresholdInTheRequest(t *testing.T) {
	enablePlatformRoute(t, "tea-allow")
	config, _ := json.Marshal(map[string]string{"agent": "claude"})
	session := store.AgentSession{ID: "ags-one", WorkspaceID: "tea-allow", SandboxID: "sbx-1", Phase: "running", AgentConfig: config}
	req := ModelMintRequest{SessionID: "ags-one", Workspace: "tea-allow", PodName: "sbx-1-0", PodUID: "uid-one"}
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	minter := func(ledger *spendLedger, charger ThresholdCharger) *ModelMinter {
		return &ModelMinter{Keys: &recordingKV{memoryKV: memoryKV{}}, Sessions: fakeSessions{session: session}, Spend: ledger,
			Charges: charger, Now: func() time.Time { return now }}
	}
	atThreshold := func() *spendLedger {
		return &spendLedger{plan: store.PlanPro, used: 10_000_000, chargeable: true}
	}

	t.Run("paid", func(t *testing.T) {
		ledger := atThreshold()
		charger := &chargerFake{outcome: ThresholdChargePaid, onCharge: func() {
			ledger.trust, ledger.charged, ledger.chargeSeq = store.BillingTrust{LifetimePaidCents: 1000}, 10_000_000, 1
		}}
		m := minter(ledger, charger)
		got, err := m.Mint(ctx, req)
		if err != nil || got.Credential != testPlatformKey {
			t.Fatalf("mint = %v platformKey=%t, want the request served", err, got.Credential == testPlatformKey)
		}
		want := []ThresholdCharge{{WorkspaceID: "tea-allow", At: now, SeenSeq: 0, UnbilledMicroUSD: 10_000_000}}
		if !slices.Equal(charger.calls, want) {
			t.Fatalf("charges = %+v, want %+v", charger.calls, want)
		}
		// The cached read, its fresh confirmation, and the re-read after
		// the payment.
		if ledger.reads != 3 {
			t.Fatalf("spend reads = %d, want 3", ledger.reads)
		}
		// The next exchange uses the re-read (cached) T1 status: no charge.
		ledger.used = 10_500_000
		if _, err := m.Mint(ctx, req); err != nil || len(charger.calls) != 1 || ledger.reads != 3 {
			t.Fatalf("next mint = %v after %d charges, %d reads", err, len(charger.calls), ledger.reads)
		}
	})

	t.Run("declined", func(t *testing.T) {
		ledger := atThreshold()
		charger := &chargerFake{outcome: ThresholdChargeDeclined, onCharge: func() { ledger.due = true }}
		m := minter(ledger, charger)
		if _, err := m.Mint(ctx, req); !errors.Is(err, ErrBillingBalanceDue) {
			t.Fatalf("mint = %v, want ErrBillingBalanceDue", err)
		}
		// The cache was dropped: the next mint reads the balance due and
		// refuses without charging again.
		if _, err := m.Mint(ctx, req); !errors.Is(err, ErrBillingBalanceDue) || len(charger.calls) != 1 {
			t.Fatalf("next mint = %v after %d charges, want refused with no second charge", err, len(charger.calls))
		}
	})

	for name, charger := range map[string]*chargerFake{
		"undecided": {outcome: ThresholdChargePending},
		"failed":    {err: errors.New("db down")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := minter(atThreshold(), charger).Mint(ctx, req); !errors.Is(err, ErrModelSpendCapReached) || errors.Is(err, ErrBillingBalanceDue) {
				t.Fatalf("mint = %v, want ErrModelSpendCapReached", err)
			}
			if len(charger.calls) != 1 {
				t.Fatalf("charges = %d, want 1", len(charger.calls))
			}
		})
	}

	t.Run("no charger stops at $10", func(t *testing.T) {
		ledger := &spendLedger{plan: store.PlanPro, used: 9_999_999, chargeable: true}
		m := minter(ledger, nil)
		if got, err := m.Mint(ctx, req); err != nil || got.Credential != testPlatformKey {
			t.Fatalf("at $9.99: %v", err)
		}
		m.ForgetSpend("tea-allow")
		ledger.used = 10_000_000
		if _, err := m.Mint(ctx, req); !errors.Is(err, ErrModelSpendCapReached) {
			t.Fatalf("at $10 with no charger: %v, want the spend cap", err)
		}
	})

	t.Run("never charged", func(t *testing.T) {
		for name, ledger := range map[string]*spendLedger{
			"not chargeable": {plan: store.PlanPro, used: 10_000_000},
			"locked":         {plan: store.PlanPro, used: 10_000_000, chargeable: true, trust: store.BillingTrust{LifetimePaidCents: 1000, LockedUntil: store.LockedIndefinitely}},
			"below":          {plan: store.PlanPro, used: 19_999_999, chargeable: true, charged: 10_000_000, chargeSeq: 1, trust: store.BillingTrust{LifetimePaidCents: 1000}},
		} {
			charger := &chargerFake{outcome: ThresholdChargePaid}
			_, err := minter(ledger, charger).Mint(ctx, req)
			if len(charger.calls) != 0 {
				t.Fatalf("%s: charged %+v", name, charger.calls)
			}
			if name == "below" && err != nil {
				t.Fatalf("below the T1 threshold (unbilled $9.99): %v, want served", err)
			}
			if name != "below" && !errors.Is(err, ErrModelSpendCapReached) {
				t.Fatalf("%s: %v, want the T0 cap", name, err)
			}
		}
	})

	t.Run("a stale cached view is confirmed first", func(t *testing.T) {
		ledger := atThreshold()
		charger := &chargerFake{outcome: ThresholdChargePaid}
		m := minter(ledger, charger)
		if status, err := m.platformStatus(ctx, "tea-allow"); err != nil || !status.ChargeDue() {
			t.Fatalf("primed status due=%t, %v", status.ChargeDue(), err)
		}
		// Another replica's charge paid meanwhile.
		ledger.trust, ledger.charged, ledger.chargeSeq = store.BillingTrust{LifetimePaidCents: 1000}, 10_000_000, 1
		if got, err := m.Mint(ctx, req); err != nil || got.Credential != testPlatformKey || len(charger.calls) != 0 {
			t.Fatalf("mint = %v with %d charges, want served without charging twice", err, len(charger.calls))
		}
	})
}

// The threshold, unbilled spend and due flag a status carries.
func TestSpendStatusChargeDue(t *testing.T) {
	now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	status := SpendStatusOf(nil, store.AgentSpendInputs{Plan: store.PlanPro, MonthToDateMicroUSD: 34_000_000, ChargedMicroUSD: 10_000_000,
		ChargeSeq: 1, Chargeable: true, Trust: store.BillingTrust{LifetimePaidCents: 1000}}, now)
	if status.ThresholdMicroUSD == nil || *status.ThresholdMicroUSD != 25_000_000 || status.UnbilledMicroUSD() != 24_000_000 || status.ChargeDue() {
		t.Fatalf("T1 at $24 unbilled = %+v, want threshold $25 and not due", status)
	}
	status.MonthToDate = 35_000_000
	if !status.ChargeDue() {
		t.Fatal("T1 at $25 unbilled is not due")
	}
	status.ChargedMicroUSD = 50_000_000
	if status.UnbilledMicroUSD() != 0 {
		t.Fatalf("unbilled = %d, want floored at 0", status.UnbilledMicroUSD())
	}
	t4 := SpendStatusOf(nil, store.AgentSpendInputs{Plan: store.PlanPro, MonthToDateMicroUSD: 1 << 40, Chargeable: true,
		Trust: store.BillingTrust{LifetimePaidCents: 300_000, PaidCycleInvoices: 6}}, now)
	if t4.ThresholdMicroUSD != nil || t4.ChargeDue() {
		t.Fatalf("T4 = %+v, want no threshold", t4)
	}
}
