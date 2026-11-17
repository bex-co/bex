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

package notifications

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

const (
	paymentRecoveredText = "Payment recovered and resources changed by billing enforcement were restored. Independently suspended resources remain suspended."
	nothingRestoredText  = "Your workspace billing status is healthy. Billing enforcement did not suspend any resource in this workspace, so there was nothing to restore. If you received a billing notice about a failed payment or a suspension for this workspace, it no longer applies."
	noPaymentDetails     = " · no payment details are included in this message."
)

// sendBillingNotice mails one notice to one owner and returns the message.
func sendBillingNotice(t *testing.T, n store.BillingNotification, livemode bool) (subject, text string) {
	t.Helper()
	mailer := &fakeMailer{}
	svc := newTestService(newBillingOwners("alice"), nil, mailer, fakeIdentities{"alice": "alice@example.com"})
	svc.DashboardBaseURL = "https://dashboard.example.com"
	if err := svc.notifyBilling(context.Background(), n, livemode); err != nil {
		t.Fatalf("notifyBilling: %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("mails sent = %d, want 1", len(mailer.sent))
	}
	return mailer.sent[0].subject, mailer.sent[0].body
}

// w21/104: a payment's recovery keeps its text, an operator's recovery of a
// workspace that restored nothing says so, and neither claims a payment or a
// restore that did not happen.
func TestBillingRecoveredNoticeSaysOnlyWhatHappened(t *testing.T) {
	subject, text := sendBillingNotice(t, store.BillingNotification{WorkspaceID: "tea-a", Status: store.BillingHealthy}, true)
	if subject != "[bex Live Mode] Billing recovered" || !strings.Contains(text, paymentRecoveredText) || !strings.Contains(text, "Stripe Live Mode"+noPaymentDetails) {
		t.Fatalf("payment recovery = %q\n%s\nwant the unchanged \"Billing recovered\" text", subject, text)
	}

	subject, text = sendBillingNotice(t, store.BillingNotification{WorkspaceID: "tea-a", Status: store.BillingHealthy, Reason: store.BillingReasonNothingRestored}, true)
	if subject != "[bex Live Mode] Billing status is healthy" || !strings.Contains(text, nothingRestoredText) || !strings.Contains(text, "Stripe Live Mode"+noPaymentDetails) {
		t.Fatalf("nothing restored = %q\n%s\nwant the truthful wording", subject, text)
	}
	for _, untrue := range []string{"Payment recovered", "Billing recovered", "were restored", "Test Mode"} {
		if strings.Contains(subject+text, untrue) {
			t.Fatalf("the nothing-restored notice says %q, which is not true of a workspace that never paid and was never enforced:\n%s %s", untrue, subject, text)
		}
	}
}

// w21/104: every billing notice is labeled with the platform's configured
// Stripe mode, in the subject and in the footer: Live Mode on a live key,
// Test Mode on a test key, never the other one.
func TestBillingNoticeLabelFollowsTheConfiguredMode(t *testing.T) {
	deadline := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	for _, status := range []string{store.BillingGrace, store.BillingEnforced, store.BillingHealthy, store.BillingExcluded, store.BillingComped} {
		for _, livemode := range []bool{true, false} {
			label, other := "Live Mode", "Test Mode"
			if !livemode {
				label, other = other, label
			}
			subject, text := sendBillingNotice(t, store.BillingNotification{WorkspaceID: "tea-a", Status: status, GraceDeadline: &deadline}, livemode)
			if !strings.HasPrefix(subject, "[bex "+label+"] ") || !strings.Contains(text, "Stripe "+label+noPaymentDetails) || strings.Contains(subject+text, other) {
				t.Errorf("%s notice on livemode=%v: subject %q, body\n%s\nwant the %q label only", status, livemode, subject, text, label)
			}
		}
	}
}

// w21/104, against the real store: the label is the configured mode whether
// the workspace has a provider mapping or not. A workspace with no mapping
// (bex's reclaimer drops it for a cardless one) used to read as test mode, so
// production, whose Stripe account is live, mailed its owners "Stripe Test Mode".
func TestBillingNoticeLabelIsTheConfiguredModeWithAndWithoutAMapping(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	// Long before any other test's grace deadline, so that this workspace's
	// row is the only one due when the shared database is asked for work.
	base := time.Date(2001, 11, 16, 0, 0, 0, 0, time.UTC)

	for _, mapped := range []bool{true, false} {
		name := "label-mapped"
		if !mapped {
			name = "label-unmapped"
		}
		tenant, err := pg.CreateTenant(ctx, name+"-"+time.Now().Format("150405.000000000"), store.PlanHobby)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), tenant.ID) })
		// A live-mode failure, as production's webhook records it.
		if _, _, _, err := pg.RecordStripeBillingEvent(ctx, store.StripeBillingEvent{
			EventID: "evt_label_" + tenant.ID, EventType: "billing.test", WorkspaceID: tenant.ID,
			CustomerID: "cus_label_" + tenant.ID, SubscriptionID: "sub_label_" + tenant.ID, ObjectID: "in_label_" + tenant.ID,
			Livemode: true, ProviderCreatedAt: base, ReceivedAt: base, Outcome: store.BillingOutcomeFailure, Reason: "label_test",
		}, time.Hour); err != nil {
			t.Fatal(err)
		}
		claim, ok, err := pg.ClaimDueBillingLifecycle(ctx, base.Add(2*time.Hour), time.Minute)
		if err != nil || !ok || claim.WorkspaceID != tenant.ID {
			t.Fatalf("claim = %+v ok=%v err=%v", claim, ok, err)
		}
		if !mapped {
			if _, err := pool.Exec(ctx, `DELETE FROM billing_provider_mappings WHERE workspace_id=$1`, tenant.ID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pg.CompleteBillingLifecycleWork(ctx, tenant.ID, claim.TransitionVersion, store.BillingEnforced, base.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		var notice *store.BillingNotification
		claimed, err := pg.ClaimBillingNotifications(ctx, base.Add(3*time.Hour), time.Minute, 100)
		if err != nil {
			t.Fatal(err)
		}
		for i := range claimed {
			if claimed[i].WorkspaceID == tenant.ID && claimed[i].Status == store.BillingEnforced {
				notice = &claimed[i]
			}
		}
		if notice == nil {
			t.Fatalf("mapped=%t: the enforced notice was not claimed (claimed %d)", mapped, len(claimed))
		}
		for _, livemode := range []bool{true, false} {
			label := "Live Mode"
			if !livemode {
				label = "Test Mode"
			}
			subject, text := sendBillingNotice(t, *notice, livemode)
			if !strings.HasPrefix(subject, "[bex "+label+"] ") || !strings.Contains(text, "Stripe "+label+noPaymentDetails) {
				t.Errorf("mapped=%t configured livemode=%v: subject %q, body\n%s\nwant the %q label", mapped, livemode, subject, text, label)
			}
		}
	}
}
