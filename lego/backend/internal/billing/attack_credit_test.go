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
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m709 (ADR023 § Bandwidth served to a DDoS attack is not billed): an egress
// hour a live attack window covers bills nothing, and an hour Stripe already
// rated comes back as a credit.

func billedBy(st *buildBeforeStore) map[string]int64 {
	out := map[string]int64{}
	for _, batch := range st.attempts {
		for _, a := range batch {
			if a.BilledQuantity != nil {
				out[a.Row.ServiceID+"@"+a.Row.WindowStart.Format(time.RFC3339)] = *a.BilledQuantity
			}
		}
	}
	return out
}

// An attack hour is frozen at 0 and never reaches Stripe, and the hour after
// it bills as if the attack's bytes had used none of the allowance (the store's
// UsageBefore leaves them out, which the month list here stands for).
func TestEmitterSendsAttackHoursAsZero(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	attack := row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 40<<30, w)
	attack.AttackHour = true
	after := row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 6<<30, w.Add(time.Hour))
	st := &buildBeforeStore{month: []store.HourlyRow{after}}
	st.queue = [][]store.HourlyRow{{attack, after}}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now)
	e.Plans = planReader{plan: "hobby"}

	e.emitOnce(context.Background(), false)

	billed := billedBy(st)
	if got, ok := billed["srv-1@"+attack.WindowStart.Format(time.RFC3339)]; !ok || got != 0 {
		t.Errorf("attack hour billed %d (frozen %t), want 0 frozen", got, ok)
	}
	if got := billed["srv-1@"+after.WindowStart.Format(time.RFC3339)]; got != 1<<30 {
		t.Errorf("hour after the attack billed %d bytes, want 1 GiB: Hobby's 5 GB untouched by the attack", got)
	}
	for _, ev := range ing.allEvents() {
		if name, value, skip := stripeMeterEvent(ev); !skip && ev.Timestamp.Equal(attack.WindowStart) {
			t.Errorf("the attack hour reached Stripe as %s=%s", name, value)
		}
	}
	if st.stampedCount() != 2 {
		t.Errorf("stamped %d rows, want both: the attack hour leaves the outbox", st.stampedCount())
	}
}

// Without plans the emitter sends egress whole, but an attack hour still owes
// nothing.
func TestEmitterSendsAttackHoursAsZeroWithoutPlans(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	attack := row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 3<<30, now.Add(-72*time.Hour))
	attack.AttackHour = true
	plain := row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 2<<30, now.Add(-71*time.Hour))
	st := &buildBeforeStore{}
	st.queue = [][]store.HourlyRow{{attack, plain}}
	e := newEmitter(st, &fakeIngester{}, now)

	e.emitOnce(context.Background(), false)

	var attackBilled, plainBilled *int64
	for _, batch := range st.attempts {
		for _, a := range batch {
			switch a.Row.WindowStart {
			case attack.WindowStart:
				attackBilled = a.BilledQuantity
			case plain.WindowStart:
				plainBilled = a.BilledQuantity
			}
		}
	}
	if attackBilled == nil || *attackBilled != 0 {
		t.Errorf("attack hour billed %v, want 0 frozen", attackBilled)
	}
	if plainBilled != nil {
		t.Errorf("an ordinary hour froze %d with no plans; it is sent whole", *plainBilled)
	}
}

func monthRow(hour int, qty int64, emitted bool, billed int64, attack bool) store.EgressMonthRow {
	return store.EgressMonthRow{
		WindowStart: time.Date(2026, 11, 1, hour, 0, 0, 0, time.UTC),
		Quantity:    qty, Emitted: emitted, Billed: billed, AttackHour: attack,
	}
}

func TestAttackCreditBytes(t *testing.T) {
	const gib = int64(1 << 30)
	allowance := 5 * gib
	for _, tc := range []struct {
		name string
		rows []store.EgressMonthRow
		want int64
	}{{
		name: "attack declared after its hours shipped: their billed bytes come back",
		rows: []store.EgressMonthRow{
			monthRow(0, 10*gib, true, 5*gib, true), // the attack spent the 5 GiB and billed 5 more
		},
		want: 5 * gib,
	}, {
		name: "the hours after an attack that the attack pushed above the allowance come back too",
		rows: []store.EgressMonthRow{
			monthRow(0, 4*gib, true, 0, true),      // attack inside the allowance
			monthRow(1, 3*gib, true, 2*gib, false), // billed 2 because the attack used 4
		},
		want: 2 * gib,
	}, {
		name: "attack declared in time: shipped as 0, nothing owed",
		rows: []store.EgressMonthRow{
			monthRow(0, 10*gib, true, 0, true),
			monthRow(1, 3*gib, true, 0, false),
		},
		want: 0,
	}, {
		name: "unsent hours are the emitter's to zero, not the credit's",
		rows: []store.EgressMonthRow{
			monthRow(0, 10*gib, false, 0, true),
			monthRow(1, 8*gib, false, 0, false),
		},
		want: 0,
	}, {
		name: "no attack hour: what was billed is what is owed",
		rows: []store.EgressMonthRow{
			monthRow(0, 4*gib, true, 0, false),
			monthRow(1, 3*gib, true, 2*gib, false),
		},
		want: 0,
	}, {
		name: "a withdrawn window's hours shipped as 0 take nothing back",
		rows: []store.EgressMonthRow{
			monthRow(0, 10*gib, true, 0, false),
		},
		want: 0,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := attackCreditBytes(tc.rows, allowance); got != tc.want {
				t.Errorf("attackCreditBytes = %d, want %d", got, tc.want)
			}
		})
	}
	// With no allowance every non-attack byte owes itself.
	rows := []store.EgressMonthRow{monthRow(0, 2*gib, true, 2*gib, true), monthRow(1, gib, true, gib, false)}
	if got := attackCreditBytes(rows, 0); got != 2*gib {
		t.Errorf("attackCreditBytes without an allowance = %d, want the attack's 2 GiB", got)
	}
}

// Several credits for one month round on the month's whole credit, so their
// cents add up to the whole credit's cents.
func TestOwedAttackCreditRoundsOnTheWhole(t *testing.T) {
	const gib = int64(1 << 30)
	ledger := store.EgressAttackLedger{Rows: []store.EgressMonthRow{monthRow(0, 100*gib, true, 95*gib, true)}, NextSeq: 1}
	first, ok := owedAttackCredit(ledger, 5*gib)
	if !ok || first.Bytes != 95*gib || first.AmountCents != pricing.Default.BandwidthCents(95*gib) || first.Seq != 1 {
		t.Fatalf("first credit = %+v, %t; want 95 GiB at the bandwidth rate, seq 1", first, ok)
	}
	if _, ok := owedAttackCredit(store.EgressAttackLedger{Rows: ledger.Rows, CreditedBytes: first.Bytes, CreditedCents: first.AmountCents, NextSeq: 2}, 5*gib); ok {
		t.Error("a month already credited in full was credited again")
	}
	// An extension covers one more already-billed hour.
	ledger.Rows = append(ledger.Rows, monthRow(1, 33*gib, true, 33*gib, true))
	ledger.CreditedBytes, ledger.CreditedCents, ledger.NextSeq = first.Bytes, first.AmountCents, 2
	second, ok := owedAttackCredit(ledger, 5*gib)
	if !ok || second.Bytes != 33*gib || second.Seq != 2 {
		t.Fatalf("second credit = %+v, %t; want the extension's 33 GiB, seq 2", second, ok)
	}
	if got, want := first.AmountCents+second.AmountCents, pricing.Default.BandwidthCents(128*gib); got != want {
		t.Errorf("credits add up to %d cents, want the whole credit's %d", got, want)
	}
	// Less than half a cent waits for more.
	if c, ok := owedAttackCredit(store.EgressAttackLedger{Rows: []store.EgressMonthRow{monthRow(0, 1<<20, true, 1<<20, true)}, NextSeq: 1}, 0); ok {
		t.Errorf("a 1 MiB credit of %d cents was recorded; it rounds to nothing", c.AmountCents)
	}
}

// attackCreditFake records credits the way the store does: a seq already
// taken inserts nothing.
type attackCreditFake struct {
	months  []store.EgressAttackCreditMonth
	ledgers map[string]store.EgressAttackLedger
	credits []store.EgressAttackCredit
	sent    map[int]string
}

func (f *attackCreditFake) EgressAttackCreditMonths(context.Context) ([]store.EgressAttackCreditMonth, error) {
	return f.months, nil
}

func (f *attackCreditFake) EgressAttackLedger(_ context.Context, workspaceID string, _ time.Time) (store.EgressAttackLedger, error) {
	l := f.ledgers[workspaceID]
	l.CreditedBytes, l.CreditedCents, l.NextSeq = 0, 0, 1
	for _, c := range f.credits {
		if c.WorkspaceID == workspaceID {
			l.CreditedBytes += c.Bytes
			l.CreditedCents += c.AmountCents
			l.NextSeq = c.Seq + 1
		}
	}
	return l, nil
}

func (f *attackCreditFake) RecordEgressAttackCredit(_ context.Context, credit store.EgressAttackCredit) (bool, error) {
	for _, c := range f.credits {
		if c.WorkspaceID == credit.WorkspaceID && c.Seq == credit.Seq {
			return false, nil
		}
	}
	f.credits = append(f.credits, credit)
	return true, nil
}

func (f *attackCreditFake) UnsentEgressAttackCredits(context.Context, int) ([]store.EgressAttackCredit, error) {
	var out []store.EgressAttackCredit
	for _, c := range f.credits {
		if _, done := f.sent[c.Seq]; !done {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *attackCreditFake) MarkEgressAttackCreditSent(_ context.Context, c store.EgressAttackCredit, itemID string, _ time.Time) error {
	f.sent[c.Seq] = itemID
	return nil
}

type attackCreditorFake struct {
	calls []store.EgressAttackCredit
	err   error
}

func (f *attackCreditorFake) CreditEgressAttack(_ context.Context, c store.EgressAttackCredit) (string, error) {
	f.calls = append(f.calls, c)
	if f.err != nil {
		return "", f.err
	}
	return "ii_" + string(rune('0'+c.Seq)), nil
}

// However often it runs, a month's credit is recorded and sent once; an
// extension over more billed hours adds a credit for those alone; a failed
// Stripe call is retried on the next pass.
func TestAttackCreditsCreditOnceHoweverOftenTheyRun(t *testing.T) {
	const gib = int64(1 << 30)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	st := &attackCreditFake{
		months:  []store.EgressAttackCreditMonth{{WorkspaceID: "tea-a", Month: month}},
		ledgers: map[string]store.EgressAttackLedger{"tea-a": {Rows: []store.EgressMonthRow{monthRow(0, 20*gib, true, 15*gib, true)}}},
		sent:    map[int]string{},
	}
	stripe := &attackCreditorFake{err: errors.New("stripe down")}
	a := &AttackCredits{Store: st, Stripe: stripe, Plans: planReader{plan: "hobby"}}

	if err := a.RunOnce(context.Background()); err == nil {
		t.Fatal("a failed Stripe call was not reported")
	}
	if len(st.credits) != 1 || len(st.sent) != 0 {
		t.Fatalf("credits %d, sent %d; want one recorded and none sent", len(st.credits), len(st.sent))
	}
	stripe.err = nil
	for range 3 {
		if err := a.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(st.credits) != 1 || st.credits[0].Bytes != 15*gib || st.sent[1] == "" {
		t.Fatalf("credits = %+v, sent = %v; want one 15 GiB credit, sent", st.credits, st.sent)
	}
	if len(stripe.calls) != 2 {
		t.Errorf("Stripe calls = %d, want 2: the failure and its one retry", len(stripe.calls))
	}

	l := st.ledgers["tea-a"]
	l.Rows = append(l.Rows, monthRow(1, 2*gib, true, 2*gib, true))
	st.ledgers["tea-a"] = l
	if err := a.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(st.credits) != 2 || st.credits[1].Bytes != 2*gib || st.credits[1].Seq != 2 || st.sent[2] == "" {
		t.Fatalf("credits after the extension = %+v; want a second, 2 GiB credit, sent", st.credits)
	}
}

// A month already compacted has no hours left to work a credit out from.
func TestAttackCreditsSkipACompactedMonth(t *testing.T) {
	st := &attackCreditFake{
		months:  []store.EgressAttackCreditMonth{{WorkspaceID: "tea-a", Month: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}},
		ledgers: map[string]store.EgressAttackLedger{"tea-a": {Compacted: true, Rows: []store.EgressMonthRow{monthRow(0, 20<<30, true, 15<<30, true)}}},
		sent:    map[int]string{},
	}
	a := &AttackCredits{Store: st, Stripe: &attackCreditorFake{}, Plans: planReader{plan: "hobby"}}
	if err := a.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(st.credits) != 0 {
		t.Errorf("credits = %+v, want none for a compacted month", st.credits)
	}
}

// The credit is a negative pending invoice item on the live Subscription,
// named for the month and keyed by its workspace-month and seq.
func TestStripeCreditEgressAttackAddsANegativeInvoiceItem(t *testing.T) {
	c, stub := newStripeTest(t, func(_ string, path string) (int, string) {
		switch {
		case strings.HasPrefix(path, "/v1/subscriptions"):
			return 200, cronMinimumSubscriptions
		case path == "/v1/invoiceitems":
			return 200, `{"id":"ii_9","object":"invoiceitem"}`
		}
		return 404, `{"error":{"type":"invalid_request_error","message":"unexpected"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	credit := store.EgressAttackCredit{WorkspaceID: "tea-a", Month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Seq: 2, Bytes: 40 << 30, AmountCents: 60}
	itemID, err := c.CreditEgressAttack(context.Background(), credit)
	if err != nil || itemID != "ii_9" {
		t.Fatalf("CreditEgressAttack = %q, %v", itemID, err)
	}
	items := stub.requests("/v1/invoiceitems")
	if len(items) != 1 {
		t.Fatalf("invoice items = %d, want 1", len(items))
	}
	form, err := url.ParseQuery(items[0].body)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"customer": "cus_1", "subscription": "sub_1", "amount": "-60", "currency": "usd",
		"description":                        "Outbound bandwidth from a DDoS attack, not billed (November 2026)",
		"metadata[bex_workspace]":            "tea-a",
		"metadata[bex_egress_attack_credit]": "2026-11/2",
	}
	for key, value := range want {
		if got := form.Get(key); got != value {
			t.Errorf("invoice item %s = %q, want %q", key, got, value)
		}
	}
	if got := items[0].header.Get("Idempotency-Key"); got != "bex-egress-attack-credit-tea-a-2026-11-2" {
		t.Errorf("Idempotency-Key = %q, want one fixed per credit", got)
	}
}
