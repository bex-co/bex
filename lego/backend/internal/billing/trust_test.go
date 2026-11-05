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
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

type fakeCharges map[string]string

func (f fakeCharges) DisputedChargeCustomer(_ context.Context, chargeID string) (string, error) {
	return f[chargeID], nil
}

func trustInvoice(workspaceID, invoiceID string, amountCents int64, reason string, paidAt time.Time) string {
	return fmt.Sprintf(`{"id":%q,"object":"invoice","livemode":false,"customer":"cus_x","amount_paid":%d,"billing_reason":%q,`+
		`"status_transitions":{"paid_at":%d},"parent":{"type":"subscription_details","subscription_details":`+
		`{"subscription":"sub_x","metadata":{"bex_workspace":%q,"bex_billing_contract":"true"}}}}`,
		invoiceID, amountCents, reason, paidAt.Unix(), workspaceID)
}

func trustEvent(id string, eventType stripe.EventType, at time.Time, object string) *stripe.Event {
	e := lifecycleEvent(id, eventType, object)
	e.Created = at.Unix()
	return e
}

// ADR091 D10.1 acceptance (real DB): invoice.paid webhooks move a workspace
// T0 → T1 → T2, redelivery never double counts, a failed payment or a
// dispute drops it to T0, and the lock holds it there for 30 days after the
// settling payment.
func TestTrustWebhooksMoveTiersAgainstPostgres(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "trust-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()
	customer := "cus_trust_" + strings.ReplaceAll(tenant.ID, "-", "")
	if err := pg.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{WorkspaceID: tenant.ID, CustomerID: customer}); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	now := base
	var changes []string
	h := &Trust{Store: pg, Charges: fakeCharges{"ch_disputed": customer}, Clock: func() time.Time { return now },
		OnTierChanged: func(_ context.Context, ws string, tier pricing.Tier) {
			if ws == tenant.ID {
				changes = append(changes, tier.Name)
			}
		}}
	tierAt := func(at time.Time) string {
		t.Helper()
		trust, err := pg.GetBillingTrust(ctx, tenant.ID)
		if err != nil {
			t.Fatal(err)
		}
		return pricing.TrustTier(trust.LifetimePaidCents, trust.PaidCycleInvoices, trust.Lock(), at).Name
	}
	send := func(e *stripe.Event) {
		t.Helper()
		if err := h.HandleStripeEvent(ctx, e); err != nil {
			t.Fatalf("%s %s: %v", e.Type, e.ID, err)
		}
	}
	inv := func(n int) string { return fmt.Sprintf("in_%s_%d", tenant.ID, n) }

	if got := tierAt(now); got != "T0" {
		t.Fatalf("new workspace = %s, want T0", got)
	}
	// A $0 invoice is not payment history.
	now = base.Add(time.Hour)
	send(trustEvent("evt_zero", stripe.EventTypeInvoicePaid, now, trustInvoice(tenant.ID, inv(0), 0, "subscription_cycle", now)))
	// $10 paid → T1; the redelivered event (and its payment_succeeded twin
	// shape) counts once.
	now = base.Add(2 * time.Hour)
	first := trustEvent("evt_paid_1", stripe.EventTypeInvoicePaid, now, trustInvoice(tenant.ID, inv(1), 1000, "subscription_create", now))
	send(first)
	send(first)
	if got := tierAt(now); got != "T1" {
		t.Fatalf("after $10 = %s, want T1", got)
	}
	// $90 more on a cycle invoice → $100 lifetime and 1 paid cycle → T2.
	now = base.Add(24 * time.Hour)
	send(trustEvent("evt_paid_2", stripe.EventTypeInvoicePaid, now, trustInvoice(tenant.ID, inv(2), 9000, "subscription_cycle", now)))
	trust, _ := pg.GetBillingTrust(ctx, tenant.ID)
	if trust.LifetimePaidCents != 10000 || trust.PaidCycleInvoices != 1 || tierAt(now) != "T2" {
		t.Fatalf("after $100 = %+v (%s), want 10000¢, 1 cycle, T2", trust, tierAt(now))
	}

	// A failed payment drops it to T0 and holds it until settled.
	now = base.Add(48 * time.Hour)
	send(trustEvent("evt_failed", stripe.EventTypeInvoicePaymentFailed, now, trustInvoice(tenant.ID, inv(3), 0, "subscription_cycle", now)))
	if got := tierAt(now.AddDate(1, 0, 0)); got != "T0" {
		t.Fatalf("unsettled failure a year later = %s, want T0", got)
	}
	// Settled: the lock runs 30 days from the settling payment.
	settled := base.Add(72 * time.Hour)
	now = settled
	send(trustEvent("evt_settle", stripe.EventTypeInvoicePaid, now, trustInvoice(tenant.ID, inv(3), 2500, "subscription_cycle", now)))
	for at, want := range map[time.Time]string{
		settled.Add(24 * time.Hour):                                    "T0",
		settled.Add(store.TrustLockAfterSettlement - time.Second):      "T0",
		settled.Add(store.TrustLockAfterSettlement):                    "T2",
		settled.Add(store.TrustLockAfterSettlement + 365*24*time.Hour): "T2",
	} {
		if got := tierAt(at); got != want {
			t.Errorf("tier at settlement+%s = %s, want %s", at.Sub(settled), got, want)
		}
	}
	// A late redelivery of the (now settled) failure changes nothing.
	send(trustEvent("evt_failed", stripe.EventTypeInvoicePaymentFailed, base.Add(48*time.Hour), trustInvoice(tenant.ID, inv(3), 0, "subscription_cycle", base)))
	if got := tierAt(settled.Add(store.TrustLockAfterSettlement)); got != "T2" {
		t.Fatalf("a stale failure re-locked the workspace: %s", got)
	}

	// Another invoice's failure locks again; paying a different invoice does
	// not unlock it.
	afterHold := settled.Add(store.TrustLockAfterSettlement)
	now = afterHold.Add(time.Hour)
	send(trustEvent("evt_failed_4", stripe.EventTypeInvoicePaymentFailed, now, trustInvoice(tenant.ID, inv(4), 0, "subscription_cycle", now)))
	now = now.Add(time.Hour)
	send(trustEvent("evt_paid_5", stripe.EventTypeInvoicePaid, now, trustInvoice(tenant.ID, inv(5), 500, "subscription_cycle", now)))
	if got := tierAt(now.AddDate(1, 0, 0)); got != "T0" {
		t.Fatalf("another invoice's payment unlocked: %s", got)
	}
	now = now.Add(time.Hour)
	send(trustEvent("evt_paid_4", stripe.EventTypeInvoicePaid, now, trustInvoice(tenant.ID, inv(4), 500, "subscription_cycle", now)))
	if got := tierAt(now.Add(store.TrustLockAfterSettlement)); got != "T2" {
		t.Fatalf("paying the failed invoice did not settle: %s", got)
	}

	// A dispute, resolved through the charge's customer, holds T0 until it
	// closes as won; a lost dispute keeps the hold.
	dispute := func(id, eventType, status string) {
		t.Helper()
		send(trustEvent("evt_"+id+"_"+status, stripe.EventType(eventType), now,
			fmt.Sprintf(`{"id":%q,"object":"dispute","livemode":false,"charge":"ch_disputed","status":%q}`, id, status)))
	}
	later := now.Add(store.TrustLockAfterSettlement + time.Hour)
	now = later
	dispute("dp_1", "charge.dispute.created", "needs_response")
	if got := tierAt(now.AddDate(0, 6, 0)); got != "T0" {
		t.Fatalf("after a dispute = %s, want T0", got)
	}
	dispute("dp_1", "charge.dispute.closed", "won")
	if got := tierAt(now); got != "T2" {
		t.Fatalf("after a won dispute = %s, want T2", got)
	}
	dispute("dp_2", "charge.dispute.created", "needs_response")
	dispute("dp_2", "charge.dispute.closed", "lost")
	if got := tierAt(now.AddDate(1, 0, 0)); got != "T0" {
		t.Fatalf("after a lost dispute = %s, want T0 until ops clears it", got)
	}
	// An unknown customer's dispute is acknowledged and changes nothing.
	send(trustEvent("evt_dispute_other", stripe.EventTypeChargeDisputeCreated, now,
		`{"id":"dp_3","object":"dispute","livemode":false,"charge":"ch_unknown"}`))

	// The settling payments leave the tier at T0 (held), so they are not
	// changes; a won dispute is.
	if want := []string{"T1", "T2", "T0", "T0", "T0", "T2", "T0"}; !slices.Equal(changes, want) {
		t.Fatalf("tier-changed signals = %v, want %v", changes, want)
	}
}

// The Stripe endpoint must subscribe to every event the receiver acts on:
// the provisioning script's list is the code's list.
func TestWebhookSetupScriptSubscribesToHandledEvents(t *testing.T) {
	raw, err := os.ReadFile("../../../../scripts/stripe-billing-setup.py")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)WEBHOOK_EVENTS = \[(.*?)\]`).FindSubmatch(raw)
	if block == nil {
		t.Fatal("WEBHOOK_EVENTS not found in scripts/stripe-billing-setup.py")
	}
	var events []string
	for _, m := range regexp.MustCompile(`"([a-z_.]+)"`).FindAllSubmatch(block[1], -1) {
		events = append(events, string(m[1]))
	}
	slices.Sort(events)
	if want := HandledStripeEventTypes(); !slices.Equal(events, want) {
		t.Fatalf("stripe-billing-setup.py WEBHOOK_EVENTS = %v, want HandledStripeEventTypes %v", events, want)
	}
	for _, event := range []string{"charge.dispute.created", "charge.dispute.closed"} {
		if !slices.Contains(events, event) {
			t.Fatalf("%s must be subscribed", event)
		}
	}
}
