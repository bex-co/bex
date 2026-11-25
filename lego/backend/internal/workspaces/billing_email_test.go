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

package workspaces

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// billingEmailFake is the workspace store with the billing email column: a
// plan change to Hobby clears it, as store.updateTenantPlanTx does.
type billingEmailFake struct {
	*fakeStore
	emails map[string]string
}

func (f *billingEmailFake) WorkspaceBillingEmail(ctx context.Context, id string) (string, string, error) {
	t, err := f.GetTenant(ctx, id)
	return t.Plan, f.emails[id], err
}

func (f *billingEmailFake) SetWorkspaceBillingEmail(_ context.Context, id, email string) error {
	f.emails[id] = email
	return nil
}

func (f *billingEmailFake) UpdateTenantPlan(ctx context.Context, id, plan string) (store.Tenant, error) {
	if plan == store.PlanHobby {
		delete(f.emails, id)
	}
	return f.fakeStore.UpdateTenantPlan(ctx, id, plan)
}

// stripeEmails records the Stripe Customer email writes, failing them while
// err is set.
type stripeEmails struct {
	set []string // "<workspace> <email>"
	err error
}

func (s *stripeEmails) SetCustomerEmail(_ context.Context, workspaceID, email string) error {
	if s.err != nil {
		return s.err
	}
	s.set = append(s.set, workspaceID+" "+email)
	return nil
}

// billingEmailFixture is a workspace of plan owned by owner@example.com.
func billingEmailFixture(t *testing.T, plan string) (*Service, *billingEmailFake, *stripeEmails, string) {
	t.Helper()
	st := &billingEmailFake{fakeStore: newFakeStore(), emails: map[string]string{}}
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	svc.Identities = fakeIdentities{"owner": {Email: "Owner@Example.com"}}
	stripe := &stripeEmails{}
	svc.CustomerEmail = stripe
	w, err := svc.Create(ctxAs("owner"), "acme", plan)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, stripe, w.ID
}

// w6/m627: a Pro workspace's billing manager sets an address of its own; it
// reaches the Stripe Customer, then bex's record, and reads back.
func TestAProWorkspaceSetsItsOwnBillingEmail(t *testing.T) {
	svc, st, stripe, ws := billingEmailFixture(t, "pro")
	before, err := svc.BillingEmail(ctxAs("owner"), ws)
	if err != nil || before.Email != "Owner@Example.com" || !before.Configurable {
		t.Fatalf("before = %+v, %v; want the owner's account email, configurable", before, err)
	}

	got, err := svc.SetBillingEmail(ctxAs("owner"), ws, "  Finance@Example.com ")
	if err != nil {
		t.Fatal(err)
	}
	want := BillingEmailView{WorkspaceID: ws, Email: "finance@example.com", Configurable: true}
	if got != want {
		t.Fatalf("set = %+v, want %+v", got, want)
	}
	if !slices.Equal(stripe.set, []string{ws + " finance@example.com"}) || st.emails[ws] != "finance@example.com" {
		t.Fatalf("stripe %v, stored %q; want both finance@example.com", stripe.set, st.emails[ws])
	}
	if read, err := svc.BillingEmail(ctxAs("owner"), ws); err != nil || read != want {
		t.Fatalf("read back = %+v, %v; want %+v", read, err, want)
	}
}

// A Hobby workspace's billing email is its owner's account email: its write
// is refused whatever the address, and nothing changes in Stripe or bex.
func TestAHobbyWorkspaceCannotSetABillingEmail(t *testing.T) {
	svc, st, stripe, ws := billingEmailFixture(t, "hobby")
	// An address stored at creation does not override the owner's on Hobby.
	st.emails[ws] = "stale@example.com"
	for _, email := range []string{"finance@example.com", "owner@example.com", "not an address"} {
		_, err := svc.SetBillingEmail(ctxAs("owner"), ws, email)
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode ||
			coded.Params["feature"] != "billingEmail" || coded.Params["requiredPlan"] != "pro" || coded.Params["plan"] != "hobby" {
			t.Fatalf("set %q on Hobby = %v, want PLAN_FEATURE_REQUIRED billingEmail/pro", email, err)
		}
	}
	if len(stripe.set) != 0 || st.emails[ws] != "stale@example.com" {
		t.Fatalf("a refused write changed stripe %v, stored %q", stripe.set, st.emails[ws])
	}
	read, err := svc.BillingEmail(ctxAs("owner"), ws)
	if err != nil || read.Email != "Owner@Example.com" || read.Configurable {
		t.Fatalf("Hobby read = %+v, %v; want the owner's account email, not configurable", read, err)
	}
}

// Stripe first: a Stripe failure answers billing unavailable and leaves bex's
// record as it was, so the two never disagree.
func TestAStripeFailureLeavesTheBillingEmailUnchanged(t *testing.T) {
	svc, st, stripe, ws := billingEmailFixture(t, "pro")
	st.emails[ws] = "old@example.com"
	stripe.err = errors.New("stripe: 500")
	if _, err := svc.SetBillingEmail(ctxAs("owner"), ws, "new@example.com"); !errors.Is(err, core.ErrBillingUnavailable) {
		t.Fatalf("set with Stripe down = %v, want ErrBillingUnavailable", err)
	}
	if st.emails[ws] != "old@example.com" {
		t.Fatalf("stored = %q, want old@example.com kept", st.emails[ws])
	}
}

// An invalid address is refused before anything is written.
func TestAnInvalidBillingEmailIsRefused(t *testing.T) {
	svc, st, stripe, ws := billingEmailFixture(t, "scale")
	for _, email := range []string{"", "finance", "a@b@c", "Finance <finance@example.com>"} {
		if _, err := svc.SetBillingEmail(ctxAs("owner"), ws, email); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("set %q = %v, want a bad request", email, err)
		}
	}
	if len(stripe.set) != 0 || st.emails[ws] != "" {
		t.Fatalf("an invalid write changed stripe %v, stored %q", stripe.set, st.emails[ws])
	}
}

// A move to Hobby clears the stored address and sets the Stripe Customer's
// email back to the owner's account email; an upgrade then starts from it.
func TestADowngradeToHobbyResetsTheBillingEmail(t *testing.T) {
	svc, st, stripe, ws := billingEmailFixture(t, "pro")
	if _, err := svc.SetBillingEmail(ctxAs("owner"), ws, "finance@example.com"); err != nil {
		t.Fatal(err)
	}
	stripe.set = nil
	if _, err := svc.ChangePlan(ctxAs("owner"), ws, "hobby"); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.emails[ws]; ok || !slices.Equal(stripe.set, []string{ws + " owner@example.com"}) {
		t.Fatalf("after the downgrade stored %q, stripe %v; want cleared and the owner's email", st.emails[ws], stripe.set)
	}
	if _, err := svc.ChangePlan(ctxAs("owner"), ws, "pro"); err != nil {
		t.Fatal(err)
	}
	read, err := svc.BillingEmail(ctxAs("owner"), ws)
	if err != nil || read.Email != "Owner@Example.com" || !read.Configurable {
		t.Fatalf("after the upgrade = %+v, %v; want the owner's account email again", read, err)
	}
	if len(stripe.set) != 1 {
		t.Fatalf("the upgrade wrote the Stripe email again: %v", stripe.set)
	}
}

// The plan change has happened by the time Stripe is told, so a Stripe
// failure then is logged, not returned.
func TestADowngradeSucceedsWhenStripeCannotBeReset(t *testing.T) {
	svc, _, stripe, ws := billingEmailFixture(t, "pro")
	stripe.err = errors.New("stripe: 500")
	got, err := svc.ChangePlan(ctxAs("owner"), ws, "hobby")
	if err != nil || got.Plan != "hobby" {
		t.Fatalf("downgrade = %+v, %v; want hobby", got, err)
	}
}

// A store without the billing email column answers unavailable, not a guess.
func TestBillingEmailNeedsTheStore(t *testing.T) {
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	w, err := svc.Create(ctxAs("owner"), "acme", "pro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BillingEmail(ctxAs("owner"), w.ID); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("read = %v, want ErrWorkspacesUnavailable", err)
	}
	if _, err := svc.SetBillingEmail(ctxAs("owner"), w.ID, "finance@example.com"); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("write = %v, want ErrWorkspacesUnavailable", err)
	}
}
