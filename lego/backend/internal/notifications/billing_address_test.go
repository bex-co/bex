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
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// billingAddressOwners is billingOwners with the workspace's plan and the
// billing email it stored (w6/m627).
type billingAddressOwners struct {
	billingOwners
	plan, address string
}

func (b billingAddressOwners) WorkspaceBillingEmail(context.Context, string) (string, string, error) {
	return b.plan, b.address, nil
}

// sentTo lists the addresses mailer reached, in order.
func sentTo(mailer *fakeMailer) []string {
	var out []string
	for _, sent := range mailer.sent {
		out = append(out, sent.to)
	}
	return out
}

// w6/m627: a Pro workspace's billing email gets each billing notice once,
// beside its admins, and the retry that finishes the others does not mail it
// again. A Hobby workspace's notices go to its admins alone.
func TestABillingNoticeAlsoReachesTheWorkspaceBillingEmail(t *testing.T) {
	for _, tc := range []struct {
		plan string
		want []string
	}{
		{store.PlanPro, []string{"alice@example.com", "finance@example.com"}},
		{store.PlanEnterprise, []string{"alice@example.com", "finance@example.com"}},
		{store.PlanHobby, []string{"alice@example.com"}},
	} {
		t.Run(tc.plan, func(t *testing.T) {
			owners := billingAddressOwners{billingOwners: newBillingOwners("alice"), plan: tc.plan, address: "finance@example.com"}
			mailer := &fakeMailer{}
			emails := &countingEmails{fakeIdentities: fakeIdentities{"alice": "alice@example.com"}}
			svc := newTestService(owners, nil, mailer, emails)
			notice := store.BillingNotification{WorkspaceID: "tea-a", Status: store.BillingGrace, TransitionVersion: 2}
			if err := svc.notifyBilling(context.Background(), notice, true); err != nil {
				t.Fatal(err)
			}
			if got := sentTo(mailer); !slices.Equal(got, tc.want) {
				t.Fatalf("mailed %v, want %v", got, tc.want)
			}
			if len(emails.batches) != 1 {
				t.Fatalf("address reads = %v, want one batch", emails.batches)
			}
			// The retry sees what the first attempt recorded and mails nobody.
			notice.MailedTo = slices.Clone(*owners.mailed)
			if err := svc.notifyBilling(context.Background(), notice, true); err != nil {
				t.Fatal(err)
			}
			if got := sentTo(mailer); !slices.Equal(got, tc.want) {
				t.Fatalf("after the retry mailed %v, want still %v", got, tc.want)
			}
		})
	}
}

// An admin who already gets the notice at the billing email is not mailed a
// second copy, whatever the address's case.
func TestABillingNoticeMailsAnAddressOnce(t *testing.T) {
	owners := billingAddressOwners{billingOwners: newBillingOwners("alice"), plan: store.PlanPro, address: "alice@example.com"}
	mailer := &fakeMailer{}
	svc := newTestService(owners, nil, mailer, fakeIdentities{"alice": "Alice@Example.com"})
	if err := svc.notifyBilling(context.Background(), store.BillingNotification{WorkspaceID: "tea-a", Status: store.BillingEnforced}, true); err != nil {
		t.Fatal(err)
	}
	if got := sentTo(mailer); !slices.Equal(got, []string{"Alice@Example.com"}) {
		t.Fatalf("mailed %v, want alice once", got)
	}
}

// An admin whose address could not be read may have the billing email's, so
// the billing email waits for the retry, which mails it once the lookup
// answers; the admins already reached are not mailed again.
func TestABillingNoticeHoldsTheBillingEmailWhileAnAdminIsUnread(t *testing.T) {
	owners := billingAddressOwners{billingOwners: newBillingOwners("alice", "carol"), plan: store.PlanPro, address: "finance@example.com"}
	mailer := &fakeMailer{}
	svc := newTestService(owners, nil, mailer, failedLookup{})
	notice := store.BillingNotification{WorkspaceID: "tea-a", Status: store.BillingGrace, TransitionVersion: 1}
	if err := svc.notifyBilling(context.Background(), notice, true); err == nil {
		t.Fatal("notifyBilling succeeded, want a retry for carol")
	}
	if got := sentTo(mailer); !slices.Equal(got, []string{"alice@example.com"}) {
		t.Fatalf("mailed %v, want alice alone", got)
	}
	notice.MailedTo = slices.Clone(*owners.mailed)
	svc.Identities = fakeIdentities{"alice": "alice@example.com", "carol": "carol@example.com"}
	if err := svc.notifyBilling(context.Background(), notice, true); err != nil {
		t.Fatal(err)
	}
	if got := sentTo(mailer); !slices.Equal(got, []string{"alice@example.com", "carol@example.com", "finance@example.com"}) {
		t.Fatalf("mailed %v, want carol and the billing email added once", got)
	}
}

// The worker notices (Free instance hours here) reach the billing email too.
func TestAFreeHoursNoticeAlsoReachesTheWorkspaceBillingEmail(t *testing.T) {
	for _, tc := range []struct {
		plan string
		want []string
	}{
		{store.PlanScale, []string{"alice@example.com", "finance@example.com"}},
		{store.PlanHobby, []string{"alice@example.com"}},
	} {
		owners := billingAddressOwners{billingOwners: newBillingOwners("alice"), plan: tc.plan, address: "finance@example.com"}
		mailer := &fakeMailer{}
		svc := newTestService(owners, nil, mailer, fakeIdentities{"alice": "alice@example.com"})
		if err := (FreeHoursNotifier{Service: svc}).NotifyFreeHours(context.Background(), "tea-a", "warned", 600, 750); err != nil {
			t.Fatal(err)
		}
		if got := sentTo(mailer); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: mailed %v, want %v", tc.plan, got, tc.want)
		}
	}
}

// Store-backed: on Postgres, a Pro workspace's own billing email gets a
// lifecycle notice once beside its admin, recorded in the notice's mailed_to,
// and a Hobby workspace's notice goes to its admin alone.
func TestABillingNoticeReachesTheStoredBillingEmailOnce(t *testing.T) {
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
	// Before every other test's deadlines, so that this workspace's rows are
	// the only ones due when the shared database is asked for work.
	base := time.Date(2001, 11, 14, 0, 0, 0, 0, time.UTC)

	for i, tc := range []struct {
		plan string
		want []string
	}{
		{store.PlanPro, []string{"owner@example.com", "finance@example.com"}},
		{store.PlanHobby, []string{"owner@example.com"}},
	} {
		subject := fmt.Sprintf("m627-owner-%d-%d", i, time.Now().UnixNano())
		tenant, err := pg.CreateTenantWithMember(ctx, subject, tc.plan)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), tenant.ID) })
		if err := pg.SetWorkspaceBillingEmail(ctx, tenant.ID, "finance@example.com"); err != nil {
			t.Fatal(err)
		}
		at := base.Add(time.Duration(i) * 24 * time.Hour)
		if _, _, _, err := pg.RecordStripeBillingEvent(ctx, store.StripeBillingEvent{
			EventID: "evt_m627_" + tenant.ID, EventType: "billing.test", WorkspaceID: tenant.ID,
			CustomerID: "cus_m627_" + tenant.ID, SubscriptionID: "sub_m627_" + tenant.ID, ObjectID: "in_m627_" + tenant.ID,
			Livemode: false, ProviderCreatedAt: at, ReceivedAt: at, Outcome: store.BillingOutcomeFailure, Reason: "m627_test",
		}, time.Hour); err != nil {
			t.Fatal(err)
		}
		claim, ok, err := pg.ClaimDueBillingLifecycle(ctx, at.Add(2*time.Hour), time.Minute)
		if err != nil || !ok || claim.WorkspaceID != tenant.ID {
			t.Fatalf("%s: claim = %+v ok=%v err=%v", tc.plan, claim, ok, err)
		}
		if _, err := pg.CompleteBillingLifecycleWork(ctx, tenant.ID, claim.TransitionVersion, store.BillingEnforced, at.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		notice := func(now time.Time) store.BillingNotification {
			t.Helper()
			claimed, err := pg.ClaimBillingNotifications(ctx, now, time.Minute, 100)
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range claimed {
				if n.WorkspaceID == tenant.ID && n.Status == store.BillingEnforced {
					return n
				}
			}
			t.Fatalf("%s: the enforced notice was not claimed (claimed %d)", tc.plan, len(claimed))
			return store.BillingNotification{}
		}
		mailer := &fakeMailer{}
		svc := newTestService(pg, nil, mailer, fakeIdentities{subject: "owner@example.com"})
		if err := svc.notifyBilling(ctx, notice(at.Add(3*time.Hour)), false); err != nil {
			t.Fatal(err)
		}
		if got := sentTo(mailer); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: mailed %v, want %v", tc.plan, got, tc.want)
		}
		// A redelivery of the same notice, claimed again once its lease ran
		// out, reads who it reached from mailed_to and mails nobody twice.
		again := notice(at.Add(4 * time.Hour))
		if err := svc.notifyBilling(ctx, again, false); err != nil {
			t.Fatal(err)
		}
		if got := sentTo(mailer); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: after the redelivery mailed %v (mailed_to %v), want still %v", tc.plan, got, again.MailedTo, tc.want)
		}
	}
}
