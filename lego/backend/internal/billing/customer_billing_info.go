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
	"fmt"
	"strings"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// companyNameMetadataKey marks a Customer whose name is a company name a
// member set in Billing info (w6/m628): a workspace rename leaves it.
const companyNameMetadataKey = "bex_company_name"

// Stripe prints a Customer's name as an invoice's Bill-to (an open invoice's
// customer_name follows customer.name until it is finalized). bex names each
// Customer after its workspace, as Render bills a workspace by its name, and
// renames it when the workspace is renamed, while the name is still bex's.

// workspaceName is the name a workspace's Customer bills: its display name,
// else its slug (store.Tenant.Label). A workspace bex cannot find, or a
// client with no label reader (isolated tests), bills its id.
func (c *StripeClient) workspaceName(ctx context.Context, workspaceID string) (string, error) {
	if c.workspaceLabel == nil {
		return workspaceID, nil
	}
	name, err := c.workspaceLabel(ctx, workspaceID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && name == "") {
		return workspaceID, nil
	}
	if err != nil {
		return "", fmt.Errorf("stripe: read the name of workspace %s: %w", workspaceID, err)
	}
	return name, nil
}

// namedByWorkspace reports whether a Customer's name is still bex's own: none,
// the workspace's id (what bex named Customers before w6/m628) or the
// workspace's name. A company name set in Billing info, and a name a member
// typed into the Stripe portal, are the member's.
func namedByWorkspace(customer *stripe.Customer, workspaceID, name string) bool {
	if customer.Metadata[companyNameMetadataKey] != "" {
		return false
	}
	return customer.Name == "" || customer.Name == workspaceID || customer.Name == name
}

// FollowWorkspaceRename renames a workspace's Customer from previous to name
// when the workspace is renamed, so the next invoice bills the new name. A
// Customer whose name is a member's own (namedByWorkspace) keeps it, and a
// workspace with no Customer has nothing to rename.
func (c *StripeClient) FollowWorkspaceRename(ctx context.Context, workspaceID, previous, name string) error {
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil || !found {
		return err
	}
	customer, gone, err := c.liveCustomer(ctx, customerID)
	if err != nil || gone || customer.Name == name || !namedByWorkspace(customer, workspaceID, previous) {
		return err
	}
	params := &stripe.CustomerParams{Name: stripe.String(name)}
	params.Context = ctx
	if _, err := c.sc.Customers.Update(customerID, params); err != nil && !resourceMissing(err) {
		return fmt.Errorf("stripe: rename Customer %s of %s: %w", customerID, workspaceID, err)
	}
	return nil
}

// fillCheckoutCustomer gives a Customer Checkout created what one
// EnsureCustomer or workspace creation creates has: the workspace's name
// (nameCheckoutCustomer) and its billing email in effect (w6/179). Checkout
// named it after what it collected, and gave it the email bex sent or, where
// bex could not name one when the session opened, the one the member typed;
// with no email bex can name now, that one stays. Every delivery of the
// event runs it, and it changes nothing once both are set.
func (c *StripeClient) fillCheckoutCustomer(ctx context.Context, checkout verifiedCheckout) error {
	customer, gone, err := c.liveCustomer(ctx, checkout.customerID)
	if err != nil || gone {
		return err
	}
	params := &stripe.CustomerParams{}
	if err := c.nameCheckoutCustomer(ctx, checkout, customer, params); err != nil {
		return err
	}
	email, err := c.workspaceBillingEmail(ctx, checkout.workspaceID)
	if err != nil {
		return err
	}
	if email != "" && customer.Email != email {
		params.Email = stripe.String(email)
	}
	if params.Name == nil && params.Email == nil && len(params.Metadata) == 0 {
		return nil
	}
	params.Context = ctx
	if _, err := c.sc.Customers.Update(checkout.customerID, params); err != nil {
		return fmt.Errorf("stripe: name and email Checkout Customer %s of %s: %w", checkout.customerID, checkout.workspaceID, err)
	}
	return nil
}

// nameCheckoutCustomer adds to params the name a Checkout Customer bills:
// its workspace's. A business name Checkout collected with a tax ID (only
// once Stripe Tax is configured) is the member's company name, so it stays
// and is marked as one, and a name already marked as one is left alone.
func (c *StripeClient) nameCheckoutCustomer(ctx context.Context, checkout verifiedCheckout, customer *stripe.Customer, params *stripe.CustomerParams) error {
	switch {
	case customer.Metadata[companyNameMetadataKey] != "":
	case checkout.businessName != "" && customer.Name == checkout.businessName:
		params.AddMetadata(companyNameMetadataKey, "true")
	default:
		name, err := c.workspaceName(ctx, checkout.workspaceID)
		if err != nil {
			return err
		}
		if customer.Name != name {
			params.Name = stripe.String(name)
		}
	}
	return nil
}

// CustomerBillingInfo reads the billing info on a workspace's Customer. A
// workspace with no Customer reads CustomerReady false; none is created.
func (c *StripeClient) CustomerBillingInfo(ctx context.Context, workspaceID string) (BillingInfo, error) {
	customer, err := c.billingInfoCustomer(ctx, workspaceID)
	if err != nil || customer == nil {
		return BillingInfo{WorkspaceID: workspaceID}, err
	}
	name, err := c.workspaceName(ctx, workspaceID)
	if err != nil {
		return BillingInfo{}, err
	}
	info := BillingInfo{WorkspaceID: workspaceID, CustomerReady: true, BillTo: customer.Name}
	if !namedByWorkspace(customer, workspaceID, name) {
		info.CompanyName = customer.Name
	}
	if a := customer.Address; a != nil && *a != (stripe.Address{}) {
		info.Address = &BillingAddress{Line1: a.Line1, Line2: a.Line2, City: a.City, State: a.State, PostalCode: a.PostalCode, Country: a.Country}
	}
	if customer.TaxIDs != nil && len(customer.TaxIDs.Data) > 0 {
		// bex keeps one; of several a member added in the portal, the
		// newest (Stripe lists newest first).
		t := customer.TaxIDs.Data[0]
		info.TaxID = &BillingTaxID{Type: string(t.Type), Value: t.Value}
	}
	return info, nil
}

// SetCustomerBillingInfo replaces the billing info on a workspace's Customer:
// its name (the company name, else the workspace's name), its address, and
// its one tax ID. A new tax ID is added first, so one Stripe refuses changes
// nothing; the ones it replaces are removed last. A workspace with no
// Customer is refused (stateError): its first payment method creates it.
func (c *StripeClient) SetCustomerBillingInfo(ctx context.Context, workspaceID string, in BillingInfoInput) (BillingInfo, error) {
	customer, err := c.billingInfoCustomer(ctx, workspaceID)
	if err != nil {
		return BillingInfo{}, err
	}
	if customer == nil {
		return BillingInfo{}, &stateError{message: "Stripe Customer is not ready"}
	}
	billTo := in.CompanyName
	if billTo == "" {
		if billTo, err = c.workspaceName(ctx, workspaceID); err != nil {
			return BillingInfo{}, err
		}
	}

	var existing []*stripe.TaxID
	if customer.TaxIDs != nil {
		existing = customer.TaxIDs.Data
	}
	keep := ""
	if in.TaxID != nil {
		for _, t := range existing {
			if string(t.Type) == in.TaxID.Type && sameTaxIDValue(t.Value, in.TaxID.Value) {
				keep = t.ID
				in.TaxID.Value = t.Value
				break
			}
		}
		if keep == "" {
			params := &stripe.TaxIDParams{Customer: stripe.String(customer.ID), Type: stripe.String(in.TaxID.Type), Value: stripe.String(in.TaxID.Value)}
			params.Context = ctx
			created, err := c.sc.TaxIDs.New(params)
			if err != nil {
				return BillingInfo{}, billingInfoStripeError(err, true)
			}
			keep, in.TaxID.Value = created.ID, created.Value
		}
	}

	update := &stripe.CustomerParams{Name: stripe.String(billTo)}
	if in.CompanyName != "" {
		update.AddMetadata(companyNameMetadataKey, "true")
	} else {
		update.AddMetadata(companyNameMetadataKey, "") // an empty value deletes the key
	}
	if a := in.Address; a != nil {
		update.Address = &stripe.AddressParams{
			Line1: stripe.String(a.Line1), Line2: stripe.String(a.Line2), City: stripe.String(a.City),
			State: stripe.String(a.State), PostalCode: stripe.String(a.PostalCode), Country: stripe.String(a.Country),
		}
	} else {
		update.AddUnsetField(stripe.CustomerParamsUnsetFieldAddress)
	}
	update.Context = ctx
	if _, err := c.sc.Customers.Update(customer.ID, update); err != nil {
		return BillingInfo{}, billingInfoStripeError(err, false)
	}

	for _, t := range existing {
		if t.ID == keep {
			continue
		}
		params := &stripe.TaxIDParams{Customer: stripe.String(customer.ID)}
		params.Context = ctx
		if _, err := c.sc.TaxIDs.Del(t.ID, params); err != nil && !resourceMissing(err) {
			return BillingInfo{}, fmt.Errorf("stripe: remove tax ID %s of %s: %w", t.ID, workspaceID, err)
		}
	}
	return BillingInfo{
		WorkspaceID: workspaceID, CustomerReady: true, BillTo: billTo,
		CompanyName: in.CompanyName, Address: in.Address, TaxID: in.TaxID,
	}, nil
}

// billingInfoCustomer is the workspace's live Customer with its tax IDs, nil
// when it has none or the one it had was deleted or retired.
func (c *StripeClient) billingInfoCustomer(ctx context.Context, workspaceID string) (*stripe.Customer, error) {
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil || !found {
		return nil, err
	}
	customer, gone, err := c.liveCustomer(ctx, customerID, "tax_ids")
	if err != nil || gone || customer.Metadata[deletedAtMetadataKey] != "" {
		return nil, err
	}
	return customer, nil
}

// sameTaxIDValue compares tax IDs as Stripe stores them, which may differ
// from what a member typed in case and spacing.
func sameTaxIDValue(a, b string) bool {
	norm := func(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), "")) }
	return norm(a) == norm(b)
}

// billingInfoStripeError names the field of billing info Stripe refused: a
// tax ID whose value its type does not accept (creatingTaxID), an address
// Stripe does not take. Anything else is Stripe's own failure.
func billingInfoStripeError(err error, creatingTaxID bool) error {
	var se *stripe.Error
	if !errors.As(err, &se) || se.Type != stripe.ErrorTypeInvalidRequest {
		return err
	}
	msg := se.Msg
	if msg == "" {
		msg = "is not accepted"
	}
	switch {
	case creatingTaxID && se.Param == "type":
		return &fieldError{"taxId.type", msg}
	case creatingTaxID && (se.Param == "value" || se.Code == stripe.ErrorCodeTaxIDInvalid || se.Code == stripe.ErrorCodeTaxIDProhibited):
		return &fieldError{"taxId.value", msg}
	case strings.HasPrefix(se.Param, "address["):
		return &fieldError{"address." + addressFieldOf(se.Param), msg}
	case se.Param == "name":
		return &fieldError{"companyName", msg}
	}
	return err
}

func addressFieldOf(param string) string {
	key := strings.TrimSuffix(strings.TrimPrefix(param, "address["), "]")
	if key == "postal_code" {
		return "postalCode"
	}
	return key
}
