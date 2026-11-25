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
	"fmt"
	"log"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A workspace's billing email (w6/m627, docs/render-artifacts/billing-email.md):
// where its invoices and billing notices go. Render asks for it when a
// workspace is created and lets a Pro or higher workspace use an address of
// its own: "Billing email address is available on the Pro plan and above".
// A Hobby workspace's billing email is its owner's account email and cannot
// be changed. bex serves it on GraphQL, REST and MCP, a bex extension:
// Render's public API and MCP have no billing read.

// BillingEmailView is a workspace's billing email. Email is the address in
// effect: on Pro and higher the one the workspace set, else its owner's
// account email; on Hobby always the owner's account email. Configurable
// reports whether the plan lets the workspace set its own.
type BillingEmailView struct {
	WorkspaceID  string `json:"workspaceId"`
	Email        string `json:"email"`
	Configurable bool   `json:"configurable"`
}

// BillingEmailStore reads and writes the billing email a workspace stored
// (*store.PGStore). A plan change to Hobby clears it in the same write.
type BillingEmailStore interface {
	WorkspaceBillingEmail(ctx context.Context, workspaceID string) (plan, email string, err error)
	SetWorkspaceBillingEmail(ctx context.Context, workspaceID, email string) error
}

// CustomerEmailSetter sets the email of a workspace's Stripe Customer
// (*billing.StripeClient); a workspace with no Customer has nothing to set.
type CustomerEmailSetter interface {
	SetCustomerEmail(ctx context.Context, workspaceID, email string) error
}

func (s *Service) billingEmailStore() (BillingEmailStore, error) {
	st, ok := s.Store.(BillingEmailStore)
	if !ok {
		return nil, core.ErrWorkspacesUnavailable
	}
	return st, nil
}

// BillingEmail reads a workspace's billing email. It is billing data, so it
// is read with can_view_billing (Admin, Developer and Billing, Render's "View
// billing details", w9/m620), as billing.Status is; setting it stays
// can_manage_billing.
func (s *Service) BillingEmail(ctx context.Context, workspaceID string) (BillingEmailView, error) {
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanViewBilling, workspaceID)
	if err != nil {
		return BillingEmailView{}, err
	}
	return s.billingEmailView(ctx, id)
}

// BillingEmailsInEffect reads BillingEmail's address without an
// authorization check, for the Stripe Customer bex creates for a workspace
// through Checkout or usage (w6/179): the usage emitter creates one with no
// caller to authorize, before the request-serving Service exists. The
// composition root wires it into billing, which cannot import this package.
func BillingEmailsInEffect(st WorkspaceStore, identities IdentityReader) func(ctx context.Context, workspaceID string) (string, error) {
	return (&Service{Store: st, Identities: identities}).billingEmailInEffect
}

// billingEmailInEffect is empty when bex cannot name a valid address: the
// workspace is gone, or it stored none and its owner's account email cannot
// be read.
func (s *Service) billingEmailInEffect(ctx context.Context, workspaceID string) (string, error) {
	view, err := s.billingEmailView(ctx, workspaceID)
	if errors.Is(err, core.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// An address Stripe would refuse is none: Checkout then asks for one.
	if email, err := normalizeBillingEmail(view.Email); err == nil {
		return email, nil
	}
	return "", nil
}

func (s *Service) billingEmailView(ctx context.Context, id string) (BillingEmailView, error) {
	st, err := s.billingEmailStore()
	if err != nil {
		return BillingEmailView{}, err
	}
	plan, stored, err := st.WorkspaceBillingEmail(ctx, id)
	if err != nil {
		return BillingEmailView{}, mapStoreErr(err)
	}
	view := BillingEmailView{WorkspaceID: id, Email: stored, Configurable: core.PlanAllows(plan, core.FeatureBillingEmail)}
	// The owner's account email stands in wherever the workspace has no
	// address of its own: always on Hobby, and on a higher plan until one is
	// set. An owner whose email cannot be read leaves the stored address,
	// which on Hobby is the account email given at creation.
	if !view.Configurable || stored == "" {
		if owner := s.ownerEmail(ctx, id); owner != "" {
			view.Email = owner
		}
	}
	return view, nil
}

// SetBillingEmail sets a Pro or higher workspace's billing email: the Stripe
// Customer's email first, then bex's own record, so a Stripe failure changes
// nothing. Admin and Billing (can_manage_billing), as Render's Billing role
// "Has full access to the workspace's billing settings". A Hobby workspace is
// refused with PLAN_FEATURE_REQUIRED whatever the address.
func (s *Service) SetBillingEmail(ctx context.Context, workspaceID, email string) (BillingEmailView, error) {
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManageBilling, workspaceID)
	if err != nil {
		return BillingEmailView{}, err
	}
	// Where Stripe sends what it bills is a financial setting, so the grant
	// is re-asserted uncached, as Checkout and the Portal re-assert it: a
	// member revoked inside the decision cache's TTL cannot redirect it.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManageBilling, core.WorkspaceObject(id)); err != nil {
		return BillingEmailView{}, err
	}
	st, err := s.billingEmailStore()
	if err != nil {
		return BillingEmailView{}, err
	}
	plan, _, err := st.WorkspaceBillingEmail(ctx, id)
	if err != nil {
		return BillingEmailView{}, mapStoreErr(err)
	}
	if !core.PlanAllows(plan, core.FeatureBillingEmail) {
		return BillingEmailView{}, core.NewPlanFeatureError("", plan, core.FeatureBillingEmail)
	}
	normalized, err := normalizeBillingEmail(email)
	if err != nil {
		return BillingEmailView{}, err
	}
	if s.CustomerEmail != nil {
		if err := s.CustomerEmail.SetCustomerEmail(ctx, id, normalized); err != nil {
			return BillingEmailView{}, core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("billing email of %s: %w", id, err))
		}
	}
	if err := st.SetWorkspaceBillingEmail(ctx, id, normalized); err != nil {
		return BillingEmailView{}, mapStoreErr(err)
	}
	return BillingEmailView{WorkspaceID: id, Email: normalized, Configurable: true}, nil
}

// resetCustomerEmail sets the Stripe Customer's email of a workspace that has
// just moved to Hobby back to its owner's account email; the plan write
// already cleared the stored address (store.updateTenantPlanTx). The plan has
// changed by then, so a failure is logged rather than returned: every read
// already answers the owner's account email.
func (s *Service) resetCustomerEmail(ctx context.Context, workspaceID string) {
	if s.CustomerEmail == nil {
		return
	}
	email, err := normalizeBillingEmail(s.ownerEmail(ctx, workspaceID))
	if err != nil {
		log.Printf("workspaces: %s moved to Hobby, but its owner's account email could not be read; its Stripe Customer keeps its email", workspaceID)
		return
	}
	if err := s.CustomerEmail.SetCustomerEmail(ctx, workspaceID, email); err != nil {
		log.Printf("workspaces: %s moved to Hobby, but its Stripe Customer's email was not reset: %v", workspaceID, err)
	}
}
