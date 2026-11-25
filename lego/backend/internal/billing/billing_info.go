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
	"unicode"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Billing info (w6/m628, docs/render-artifacts/billing-info.md): the company
// name, address and VAT or tax ID a workspace's invoices carry. Render's
// Billing page has a button "to add additional billing info to be displayed
// on all invoices"; its form reads "Provide additional billing info such as
// company name, address, or VAT number". bex keeps it on the workspace's
// Stripe Customer, the same fields the Stripe portal edits, so Stripe prints
// it on every invoice it finalizes afterwards. Without a company name the
// invoices bill the workspace's own name. Showing a tax ID collects no tax:
// Stripe Tax stays off until ADR040 §6's inputs exist.

// BillingAddress is the postal address invoices print. Country is an ISO
// 3166-1 alpha-2 code; every other line may be empty.
type BillingAddress struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// BillingTaxID is a VAT or tax ID typed as one of Stripe's customer tax ID
// types (eu_vat, gb_vat, us_ein, …).
type BillingTaxID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// BillingInfo is a workspace's billing info as REST, GraphQL and MCP serve
// it. BillTo is the name its invoices bill: the company name when one is
// set, else the workspace's name. CustomerReady is false until the workspace
// has a billing account (its Stripe Customer, made when a payment method is
// first added), before which there is nothing to read or set.
type BillingInfo struct {
	WorkspaceID   string          `json:"workspaceId"`
	CustomerReady bool            `json:"customerReady"`
	BillTo        string          `json:"billTo"`
	CompanyName   string          `json:"companyName"`
	Address       *BillingAddress `json:"address"`
	TaxID         *BillingTaxID   `json:"taxId"`
}

// BillingInfoInput replaces a workspace's billing info: an empty company
// name bills the workspace's name again, a nil address or tax ID removes it.
type BillingInfoInput struct {
	CompanyName string          `json:"companyName"`
	Address     *BillingAddress `json:"address"`
	TaxID       *BillingTaxID   `json:"taxId"`
}

// BillingInfoProvider reads and writes the billing info on a workspace's
// Stripe Customer (*StripeClient).
type BillingInfoProvider interface {
	CustomerBillingInfo(ctx context.Context, workspaceID string) (BillingInfo, error)
	SetCustomerBillingInfo(ctx context.Context, workspaceID string, in BillingInfoInput) (BillingInfo, error)
}

// BillingInfoInvalidCode refuses billing info a field of which is invalid;
// params.field names it (companyName, address.country, taxId.value, …) and
// params.reason says why, so a form can show the reason beside it.
const BillingInfoInvalidCode = "BILLING_INFO_INVALID"

// BillingCustomerNotReadyCode refuses a billing info write while the
// workspace has no billing account to hold it: no payment method was ever
// added.
const BillingCustomerNotReadyCode = "BILLING_CUSTOMER_NOT_READY"

// Field lengths bex accepts. Stripe holds a Customer name to 256 characters;
// the address lines and tax ID get bounds no real address or ID comes near.
const (
	maxCompanyNameLength  = 256
	maxAddressFieldLength = 200
	maxTaxIDValueLength   = 64
)

// BillingInfo reads a workspace's billing info. It is billing data, read with
// can_view_billing (Admin, Developer and Billing, Render's "View billing
// details", w9/m620), as Status is.
func (s *Service) BillingInfo(ctx context.Context, workspaceID string) (BillingInfo, error) {
	ctx, tenantID, err := s.authorize(ctx, workspaceID, core.RelCanViewBilling)
	if err != nil {
		return BillingInfo{}, err
	}
	if s.Info == nil {
		return BillingInfo{}, core.ErrBillingUnavailable
	}
	info, err := s.Info.CustomerBillingInfo(ctx, tenantID)
	if err != nil {
		return BillingInfo{}, core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("billing info of %s: %w", tenantID, err))
	}
	info.WorkspaceID = tenantID
	return info, nil
}

// SetBillingInfo replaces a workspace's billing info. Admin and Billing
// (can_manage_billing), as Render's Billing role "Has full access to the
// workspace's billing settings"; what every later invoice says about who is
// billed is a financial setting, so the grant is re-asserted uncached, as
// Checkout and the portal re-assert it.
func (s *Service) SetBillingInfo(ctx context.Context, workspaceID string, in BillingInfoInput) (BillingInfo, error) {
	ctx, tenantID, err := s.authorize(ctx, workspaceID, core.RelCanManageBilling)
	if err != nil {
		return BillingInfo{}, err
	}
	if err := s.AuthorizeFresh(ctx, core.RelCanManageBilling); err != nil {
		return BillingInfo{}, err
	}
	if s.Info == nil {
		return BillingInfo{}, core.ErrBillingUnavailable
	}
	in, err = normalizeBillingInfo(in)
	if err != nil {
		return BillingInfo{}, classifyBillingInfoError(tenantID, err)
	}
	info, err := s.Info.SetCustomerBillingInfo(ctx, tenantID, in)
	if err != nil {
		return BillingInfo{}, classifyBillingInfoError(tenantID, err)
	}
	info.WorkspaceID = tenantID
	return info, nil
}

func classifyBillingInfoError(workspaceID string, err error) error {
	var field *fieldError
	switch {
	case errors.As(err, &field):
		return field.coded()
	case isStateError(err):
		return core.NewConflictError(BillingCustomerNotReadyCode, "add a payment method first: billing info is kept with the workspace's billing account, which the first payment method creates", nil)
	default:
		return core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("billing info of %s: %w", workspaceID, err))
	}
}

// fieldError is billing info refused for one field, by bex's own checks or
// by Stripe's (a tax ID whose value does not match its type).
type fieldError struct{ field, message string }

func (e *fieldError) Error() string { return e.field + ": " + e.message }

func (e *fieldError) coded() *core.CodedError {
	return core.NewBadRequestError(BillingInfoInvalidCode, e.Error(), map[string]any{"field": e.field, "reason": e.message})
}

// normalizeBillingInfo trims every field and refuses what Stripe would refuse
// or bex will not store, naming the field: a control character, a field over
// its length, an address without a two-letter country, a tax ID whose type is
// not one of Stripe's or that has no value. An address or tax ID with every
// field empty is none.
func normalizeBillingInfo(in BillingInfoInput) (BillingInfoInput, error) {
	var err error
	if in.CompanyName, err = billingInfoField("companyName", in.CompanyName, maxCompanyNameLength); err != nil {
		return BillingInfoInput{}, err
	}
	if in.Address != nil {
		a := *in.Address
		for _, f := range []struct {
			name string
			v    *string
		}{
			{"address.line1", &a.Line1}, {"address.line2", &a.Line2}, {"address.city", &a.City},
			{"address.state", &a.State}, {"address.postalCode", &a.PostalCode}, {"address.country", &a.Country},
		} {
			if *f.v, err = billingInfoField(f.name, *f.v, maxAddressFieldLength); err != nil {
				return BillingInfoInput{}, err
			}
		}
		a.Country = strings.ToUpper(a.Country)
		switch {
		case a == BillingAddress{}:
			in.Address = nil
		case !isCountryCode(a.Country):
			return BillingInfoInput{}, &fieldError{"address.country", "must be a two-letter ISO 3166-1 country code"}
		default:
			in.Address = &a
		}
	}
	if in.TaxID != nil {
		t := *in.TaxID
		t.Type = strings.ToLower(strings.TrimSpace(t.Type))
		if t.Value, err = billingInfoField("taxId.value", t.Value, maxTaxIDValueLength); err != nil {
			return BillingInfoInput{}, err
		}
		switch {
		case t == BillingTaxID{}:
			in.TaxID = nil
		case !stripeTaxIDTypes[t.Type]:
			return BillingInfoInput{}, &fieldError{"taxId.type", fmt.Sprintf("%q is not a Stripe tax ID type", t.Type)}
		case t.Value == "":
			return BillingInfoInput{}, &fieldError{"taxId.value", "is required"}
		default:
			in.TaxID = &t
		}
	}
	return in, nil
}

func billingInfoField(field, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	for _, c := range value {
		if unicode.IsControl(c) || c == '\u2028' || c == '\u2029' {
			return "", &fieldError{field, "must not contain control characters or line separators"}
		}
	}
	if utf8.RuneCountInString(value) > maxLength {
		return "", &fieldError{field, fmt.Sprintf("must be at most %d characters", maxLength)}
	}
	return value, nil
}

func isCountryCode(s string) bool {
	return len(s) == 2 && 'A' <= s[0] && s[0] <= 'Z' && 'A' <= s[1] && s[1] <= 'Z'
}
