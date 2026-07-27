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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v82"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestStripeSandboxEndToEnd is an opt-in smoke test against Stripe test mode
// and a disposable Postgres database. It proves the real outbox boundary:
// sealed usage provisions one metadata-keyed Customer and one complete metered
// Subscription, emits the Stripe meter event, stamps emitted_at, reads the
// invoice preview, and applies the Mode-B comp coupon. The Customer is deleted
// at cleanup, which also cancels its test Subscription.
//
// It is deliberately skipped in ordinary unit/CI runs. Invoke it only with a
// test-mode server key and a throwaway database. A dedicated rk_test_ key is
// preferred; sk_test_ is accepted only so older Stripe CLI login credentials
// can drive a one-shot local smoke without ever being persisted by bex:
//
//	BEX_TEST_STRIPE_KEY=<test-only server key> BEX_TEST_DB_URI=postgres://... \
//	  go test ./internal/billing -run TestStripeSandboxEndToEnd -v
func TestStripeSandboxEndToEnd(t *testing.T) {
	stripeKey := os.Getenv("BEX_TEST_STRIPE_KEY")
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	if stripeKey == "" || dbURI == "" {
		t.Skip("set BEX_TEST_STRIPE_KEY and BEX_TEST_DB_URI for the real Stripe sandbox smoke test")
	}
	isRestrictedTestKey := len(stripeKey) >= len("rk_test_") && stripeKey[:len("rk_test_")] == "rk_test_"
	isSecretTestKey := len(stripeKey) >= len("sk_test_") && stripeKey[:len("sk_test_")] == "sk_test_"
	if !isRestrictedTestKey && !isSecretTestKey {
		t.Fatal("BEX_TEST_STRIPE_KEY must be a Stripe test-mode server key (rk_test_ or sk_test_ prefix); live keys are refused")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate disposable database: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatalf("open disposable database: %v", err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	run := time.Now().UTC().Format("20060102-150405.000000000")
	tenant, err := st.CreateWorkspace(ctx, "stripe-sandbox-"+run, store.PlanHobby, "stripe-sandbox-owner-"+run)
	if err != nil {
		t.Fatalf("create sandbox workspace: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Hour)
	window := now.Add(-3 * time.Hour)
	row := store.HourlyRow{
		WorkspaceID:  tenant.ID,
		ServiceID:    "srv-stripe-sandbox-" + run,
		Kind:         store.UsageKindBuildSeconds,
		ResourceKind: store.ResourceKindService,
		WindowStart:  window,
		Quantity:     60,
	}
	if err := st.UpsertUsageHourly(ctx, row); err != nil {
		t.Fatalf("insert sealed sandbox usage: %v", err)
	}

	client := NewStripe(StripeConfig{SecretKey: stripeKey})
	if client == nil {
		t.Fatal("Stripe client unexpectedly disabled")
	}
	emitter := NewEmitter(st, client)
	emitter.Epoch = window.Add(-time.Hour)
	emitter.SealHours = 2 * time.Hour
	emitter.now = func() time.Time { return now }
	emitter.emitOnce(ctx)

	customerID, ok := client.lookupCustomer(tenant.ID)
	if !ok {
		t.Fatal("sandbox workspace did not provision a Stripe Customer")
	}
	defer func() {
		params := &stripe.CustomerParams{}
		params.Context = context.Background()
		if _, err := client.sc.Customers.Del(customerID, params); err != nil {
			t.Errorf("delete sandbox Stripe Customer %s: %v", customerID, err)
		}
	}()

	subscriptionID, found, err := client.findSubscription(ctx, tenant.ID, customerID)
	if err != nil {
		t.Fatalf("resolve sandbox Subscription: %v", err)
	}
	if !found || subscriptionID == "" {
		t.Fatal("sandbox workspace did not provision a Stripe Subscription")
	}

	remaining, err := st.SelectUnemittedUsage(ctx, emitter.floor(now), now.Add(-emitter.SealHours), 10)
	if err != nil {
		t.Fatalf("read sandbox outbox after emit: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("sandbox usage was not stamped emitted: %d row(s) remain", len(remaining))
	}

	bill, err := client.BillingFor(ctx, tenant.ID, window, now)
	if err != nil {
		t.Fatalf("read sandbox invoice preview: %v", err)
	}
	if bill == nil || bill.CurrentCost == nil || bill.CurrentCost.Currency != "USD" {
		t.Fatalf("sandbox invoice preview missing or malformed: %#v", bill)
	}
	if err := client.CompCustomer(ctx, tenant.ID); err != nil {
		t.Fatalf("apply sandbox Mode-B comp: %v", err)
	}

	t.Logf("Stripe sandbox verified: workspace=%s customer=%s subscription=%s preview=%s %s", tenant.ID, customerID, subscriptionID, bill.CurrentCost.AmountUSD, bill.CurrentCost.Currency)
}
