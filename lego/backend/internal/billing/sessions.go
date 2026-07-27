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
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
)

const (
	checkoutSubscriptionMetadataKey = "bex_subscription"
	checkoutSessionLifetime         = 35 * time.Minute
)

type inputError struct{ message string }

func (e *inputError) Error() string { return e.message }

type stateError struct{ message string }

func (e *stateError) Error() string { return e.message }

func isInputError(err error) bool {
	var target *inputError
	return errors.As(err, &target)
}

func isStateError(err error) bool {
	var target *stateError
	return errors.As(err, &target)
}

// Readiness resolves Stripe objects without creating them. This makes GET a
// true read: estimate-only/excluded workspaces do not gain billing objects just
// because an admin opens the Usage page.
func (c *StripeClient) Readiness(ctx context.Context, workspaceID string) (Readiness, error) {
	out := Readiness{Mode: c.mode(), Tax: c.taxReadiness(ctx, nil)}
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil {
		return out, err
	}
	if !found {
		return out, nil
	}
	out.CustomerReady = true

	customerParams := &stripe.CustomerParams{}
	customerParams.Context = ctx
	customer, err := c.sc.Customers.Get(customerID, customerParams)
	if err != nil {
		return out, fmt.Errorf("stripe: retrieve customer for %s: %w", workspaceID, err)
	}
	subscription, err := c.findSubscriptionObject(ctx, workspaceID, customerID)
	if err != nil {
		return out, err
	}
	if subscription == nil {
		out.PaymentMethodReady = customerDefaultPaymentMethod(customer) != ""
		return out, nil
	}
	out.SubscriptionReady = true
	out.PaymentMethodReady = subscriptionDefaultPaymentMethod(subscription) != "" || customerDefaultPaymentMethod(customer) != ""
	out.Tax = c.taxReadiness(ctx, subscription)
	return out, nil
}

// CreateCheckoutSession creates setup-mode Checkout for the existing Customer
// and complete metered Subscription. It deliberately sends no line_items and
// no payment_method_types, so no duplicate Subscription can be created and
// Stripe's dynamic payment-method selection remains active.
func (c *StripeClient) CreateCheckoutSession(ctx context.Context, workspaceID string, req CheckoutRequest) (HostedSession, error) {
	if err := c.validateReturnURL(req.SuccessURL); err != nil {
		return HostedSession{}, fmt.Errorf("successUrl: %w", err)
	}
	if err := c.validateReturnURL(req.CancelURL); err != nil {
		return HostedSession{}, fmt.Errorf("cancelUrl: %w", err)
	}
	if err := c.EnsureContract(ctx, workspaceID); err != nil {
		return HostedSession{}, err
	}
	customerID, ok := c.lookupCustomer(workspaceID)
	if !ok {
		return HostedSession{}, &stateError{message: "Stripe Customer is not ready"}
	}
	subscription, err := c.findSubscriptionObject(ctx, workspaceID, customerID)
	if err != nil {
		return HostedSession{}, err
	}
	if subscription == nil {
		return HostedSession{}, &stateError{message: "Stripe Subscription is not ready"}
	}

	suffix, err := randomLetters(8)
	if err != nil {
		return HostedSession{}, fmt.Errorf("stripe: create Checkout request id: %w", err)
	}
	metadata := map[string]string{
		workspaceMetadataKey:            workspaceID,
		checkoutSubscriptionMetadataKey: subscription.ID,
	}
	params := &stripe.CheckoutSessionParams{
		Mode:                  stripe.String(string(stripe.CheckoutSessionModeSetup)),
		Currency:              stripe.String(string(stripe.CurrencyUSD)),
		Customer:              stripe.String(customerID),
		ClientReferenceID:     stripe.String(workspaceID),
		SuccessURL:            stripe.String(req.SuccessURL),
		CancelURL:             stripe.String(req.CancelURL),
		ExpiresAt:             stripe.Int64(time.Now().Add(checkoutSessionLifetime).Unix()),
		IntegrationIdentifier: stripe.String("bex_billing_" + suffix),
		Metadata:              metadata,
		SetupIntentData:       &stripe.CheckoutSessionSetupIntentDataParams{Metadata: metadata},
	}
	// A setup session has no taxable transaction. Once the operator tax gate is
	// valid, Checkout collects and saves the location/tax-id inputs that the
	// existing Subscription's future automatic-tax invoices need.
	if tax := c.taxReadiness(ctx, subscription); tax.Configured {
		params.BillingAddressCollection = stripe.String("required")
		params.CustomerUpdate = &stripe.CheckoutSessionCustomerUpdateParams{
			Address: stripe.String("auto"),
			Name:    stripe.String("auto"),
		}
		params.TaxIDCollection = &stripe.CheckoutSessionTaxIDCollectionParams{Enabled: stripe.Bool(true)}
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-checkout-" + workspaceID + "-" + suffix)
	session, err := c.sc.CheckoutSessions.New(params)
	if err != nil {
		return HostedSession{}, fmt.Errorf("stripe: create Checkout Session for %s: %w", workspaceID, err)
	}
	if session.URL == "" || session.ID == "" || !c.expectedLivemode(session.Livemode) {
		return HostedSession{}, fmt.Errorf("stripe: Checkout Session returned an invalid mode or empty id/url")
	}
	return HostedSession{URL: session.URL, ExpiresAt: time.Unix(session.ExpiresAt, 0).UTC().Format(time.RFC3339)}, nil
}

func (c *StripeClient) CreatePortalSession(ctx context.Context, workspaceID string, req PortalRequest) (HostedSession, error) {
	if err := c.validateReturnURL(req.ReturnURL); err != nil {
		return HostedSession{}, fmt.Errorf("returnUrl: %w", err)
	}
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil {
		return HostedSession{}, err
	}
	if !found {
		return HostedSession{}, &stateError{message: "Stripe Customer is not ready"}
	}
	if subscription, err := c.findSubscriptionObject(ctx, workspaceID, customerID); err != nil {
		return HostedSession{}, err
	} else if subscription == nil {
		return HostedSession{}, &stateError{message: "Stripe Subscription is not ready"}
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(req.ReturnURL),
	}
	if c.portalConfigurationID != "" {
		params.Configuration = stripe.String(c.portalConfigurationID)
	}
	params.Context = ctx
	session, err := c.sc.BillingPortalSessions.New(params)
	if err != nil {
		return HostedSession{}, fmt.Errorf("stripe: create Portal Session for %s: %w", workspaceID, err)
	}
	if session.URL == "" || !c.expectedLivemode(session.Livemode) {
		return HostedSession{}, fmt.Errorf("stripe: Portal Session returned an invalid mode or empty URL")
	}
	return HostedSession{URL: session.URL}, nil
}

// CompleteCheckoutSession consumes an authenticated checkout.session.completed
// event. Every relationship is re-read from Stripe before defaults are bound;
// metadata alone is never trusted. Deterministic idempotency keys make replayed
// or reordered webhooks converge on the same Customer and Subscription state.
func (c *StripeClient) CompleteCheckoutSession(ctx context.Context, eventSession *stripe.CheckoutSession) error {
	if eventSession == nil || eventSession.ID == "" {
		return &inputError{message: "checkout session is missing"}
	}
	params := &stripe.CheckoutSessionParams{}
	params.Context = ctx
	session, err := c.sc.CheckoutSessions.Get(eventSession.ID, params)
	if err != nil {
		return fmt.Errorf("stripe: retrieve Checkout Session %s: %w", eventSession.ID, err)
	}
	if !c.expectedLivemode(session.Livemode) || session.Mode != stripe.CheckoutSessionModeSetup {
		return &inputError{message: "checkout session mode does not match the billing environment"}
	}
	workspaceID := session.Metadata[workspaceMetadataKey]
	claimedSubscriptionID := session.Metadata[checkoutSubscriptionMetadataKey]
	if workspaceID == "" || claimedSubscriptionID == "" || session.Customer == nil || session.SetupIntent == nil {
		return &inputError{message: "checkout session ownership metadata is incomplete"}
	}
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !found || customerID != session.Customer.ID {
		return &inputError{message: "checkout session Customer does not belong to the workspace"}
	}
	subscription, err := c.findSubscriptionObject(ctx, workspaceID, customerID)
	if err != nil {
		return err
	}
	if subscription == nil || subscription.ID != claimedSubscriptionID {
		return &inputError{message: "checkout session Subscription does not belong to the workspace"}
	}

	setupParams := &stripe.SetupIntentParams{}
	setupParams.Context = ctx
	setup, err := c.sc.SetupIntents.Get(session.SetupIntent.ID, setupParams)
	if err != nil {
		return fmt.Errorf("stripe: retrieve SetupIntent %s: %w", session.SetupIntent.ID, err)
	}
	if setup.Status != stripe.SetupIntentStatusSucceeded || setup.PaymentMethod == nil || setup.Customer == nil || setup.Customer.ID != customerID || setup.Metadata[workspaceMetadataKey] != workspaceID || setup.Metadata[checkoutSubscriptionMetadataKey] != subscription.ID {
		return &inputError{message: "SetupIntent does not prove the workspace payment setup"}
	}
	paymentMethodID := setup.PaymentMethod.ID
	pmParams := &stripe.PaymentMethodParams{}
	pmParams.Context = ctx
	paymentMethod, err := c.sc.PaymentMethods.Get(paymentMethodID, pmParams)
	if err != nil {
		return fmt.Errorf("stripe: retrieve PaymentMethod %s: %w", paymentMethodID, err)
	}
	if paymentMethod.Customer == nil || paymentMethod.Customer.ID != customerID {
		return &inputError{message: "payment method is not attached to the workspace Customer"}
	}

	customerUpdate := &stripe.CustomerParams{InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{DefaultPaymentMethod: stripe.String(paymentMethodID)}}
	customerUpdate.Context = ctx
	customerUpdate.SetIdempotencyKey("bex-payment-customer-" + session.ID)
	if _, err := c.sc.Customers.Update(customerID, customerUpdate); err != nil {
		return fmt.Errorf("stripe: bind Customer default payment method: %w", err)
	}
	subscriptionUpdate := &stripe.SubscriptionParams{
		DefaultPaymentMethod: stripe.String(paymentMethodID),
		ProrationBehavior:    stripe.String("none"),
	}
	if tax := c.taxReadiness(ctx, subscription); tax.Configured {
		subscriptionUpdate.AutomaticTax = &stripe.SubscriptionAutomaticTaxParams{Enabled: stripe.Bool(true)}
	}
	subscriptionUpdate.Context = ctx
	subscriptionUpdate.SetIdempotencyKey("bex-payment-subscription-" + session.ID)
	if _, err := c.sc.Subscriptions.Update(subscription.ID, subscriptionUpdate); err != nil {
		return fmt.Errorf("stripe: bind Subscription default payment method: %w", err)
	}
	return nil
}

func (c *StripeClient) findSubscriptionObject(ctx context.Context, workspaceID, customerID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionListParams{
		ListParams: stripe.ListParams{Limit: stripe.Int64(100)},
		Customer:   stripe.String(customerID),
		Status:     stripe.String("all"),
	}
	params.Context = ctx
	iter := c.sc.Subscriptions.List(params)
	var found []*stripe.Subscription
	for iter.Next() {
		sub := iter.Subscription()
		if sub.Metadata[workspaceMetadataKey] != workspaceID || sub.Metadata[subscriptionMetadataKey] != "true" || sub.Status == stripe.SubscriptionStatusCanceled || sub.Status == stripe.SubscriptionStatusIncompleteExpired {
			continue
		}
		found = append(found, sub)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe: list subscriptions for %s: %w", workspaceID, err)
	}
	if len(found) > 1 {
		ids := make([]string, 0, len(found))
		for _, sub := range found {
			ids = append(ids, sub.ID)
		}
		return nil, fmt.Errorf("stripe: workspace %s has %d live bex subscriptions: %v", workspaceID, len(found), ids)
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

func (c *StripeClient) taxReadiness(ctx context.Context, subscription *stripe.Subscription) TaxReadiness {
	if c.taxCode == "" && c.taxBehavior == "" {
		return c.unconfiguredTax("product_tax_not_configured")
	}
	if !strings.HasPrefix(c.taxCode, "txcd_") || (c.taxBehavior != string(stripe.PriceTaxBehaviorExclusive) && c.taxBehavior != string(stripe.PriceTaxBehaviorInclusive)) {
		return c.unconfiguredTax("product_tax_configuration_invalid")
	}
	if _, err := c.resolvePriceIDs(ctx); err != nil {
		return c.unconfiguredTax("catalog_not_tax_ready")
	}
	params := &stripe.TaxRegistrationListParams{Status: stripe.String(string(stripe.TaxRegistrationStatusActive))}
	params.Context = ctx
	iter := c.sc.TaxRegistrations.List(params)
	registrations := 0
	for iter.Next() {
		registration := iter.TaxRegistration()
		if c.expectedLivemode(registration.Livemode) {
			registrations++
		}
	}
	if iter.Err() != nil {
		return c.unconfiguredTax("registration_verification_failed")
	}
	if registrations == 0 {
		return c.unconfiguredTax("active_registration_missing")
	}
	enabled := subscription != nil && subscription.AutomaticTax != nil && subscription.AutomaticTax.Enabled
	reason := ""
	if !enabled {
		reason = "payment_setup_required"
	}
	return TaxReadiness{Configured: true, Enabled: enabled, Reason: reason, ProductTaxCode: c.taxCode, TaxBehavior: c.taxBehavior, RegistrationCount: registrations}
}

func (c *StripeClient) unconfiguredTax(reason string) TaxReadiness {
	return TaxReadiness{Reason: reason, ProductTaxCode: c.taxCode, TaxBehavior: c.taxBehavior}
}

func (c *StripeClient) validateReturnURL(raw string) error {
	if raw == "" {
		return &inputError{message: "URL is required"}
	}
	trusted, err := url.Parse(c.dashboardURL)
	if err != nil || trusted.Scheme == "" || trusted.Host == "" {
		return &stateError{message: "trusted dashboard origin is not configured"}
	}
	candidate, err := url.Parse(raw)
	if err != nil || candidate.Scheme == "" || candidate.Host == "" || candidate.User != nil {
		return &inputError{message: "URL must be absolute"}
	}
	if !strings.EqualFold(candidate.Scheme, trusted.Scheme) || !strings.EqualFold(candidate.Host, trusted.Host) {
		return &inputError{message: "URL origin is not trusted"}
	}
	if candidate.Scheme != "https" && candidate.Hostname() != "localhost" && candidate.Hostname() != "127.0.0.1" {
		return &inputError{message: "URL must use HTTPS"}
	}
	return nil
}

func (c *StripeClient) mode() string {
	if c.testMode {
		return "test"
	}
	return "live"
}

func (c *StripeClient) expectedLivemode(livemode bool) bool {
	return livemode == !c.testMode
}

func customerDefaultPaymentMethod(customer *stripe.Customer) string {
	if customer != nil && customer.InvoiceSettings != nil && customer.InvoiceSettings.DefaultPaymentMethod != nil {
		return customer.InvoiceSettings.DefaultPaymentMethod.ID
	}
	return ""
}

func subscriptionDefaultPaymentMethod(subscription *stripe.Subscription) string {
	if subscription != nil && subscription.DefaultPaymentMethod != nil {
		return subscription.DefaultPaymentMethod.ID
	}
	return ""
}

func randomLetters(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}
