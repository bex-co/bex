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
	"log"
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

// IsInputError reports a provider response that proves caller-visible setup is
// incomplete or mismatched, rather than a transient Stripe failure.
func IsInputError(err error) bool {
	return isInputError(err)
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
	// Expanded so Readiness can name the card ("visa ···4242") instead of only
	// reporting that one exists; unexpanded, Stripe returns the id alone.
	customerParams.AddExpand("invoice_settings.default_payment_method")
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
		describeCard(&out, customerCard(customer))
		return out, nil
	}
	out.SubscriptionReady = true
	out.PaymentMethodReady = subscriptionDefaultPaymentMethod(subscription) != "" || customerDefaultPaymentMethod(customer) != ""
	// The subscription's own default wins where it has one — that is the card
	// Stripe will actually charge for this workspace.
	if card := subscriptionCard(subscription); card != nil {
		describeCard(&out, card)
	} else {
		describeCard(&out, customerCard(customer))
	}
	out.Tax = c.taxReadiness(ctx, subscription)
	return out, nil
}

// CreateCheckoutSession creates setup-mode Checkout without minting anything
// in Stripe first. A workspace that already owns a Customer reuses
// it; one that does not gets customer_creation=always, so Stripe creates the
// Customer only when the user actually submits the form, and completion adopts
// it and attaches the metered Subscription. Opening the page and walking away
// therefore leaves no Customer and no cardless Subscription behind. It
// deliberately sends no line_items and no payment_method_types, so Stripe's
// dynamic payment-method selection remains active.
func (c *StripeClient) CreateCheckoutSession(ctx context.Context, workspaceID string, req CheckoutRequest) (HostedSession, error) {
	if err := c.validateReturnURL(req.SuccessURL); err != nil {
		return HostedSession{}, fmt.Errorf("successUrl: %w", err)
	}
	if err := c.validateReturnURL(req.CancelURL); err != nil {
		return HostedSession{}, fmt.Errorf("cancelUrl: %w", err)
	}
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil {
		return HostedSession{}, err
	}
	var subscription *stripe.Subscription
	if found {
		if subscription, err = c.findSubscriptionObject(ctx, workspaceID, customerID); err != nil {
			return HostedSession{}, err
		}
	}

	suffix, err := randomLetters(8)
	if err != nil {
		return HostedSession{}, fmt.Errorf("stripe: create Checkout request id: %w", err)
	}
	metadata := map[string]string{workspaceMetadataKey: workspaceID}
	params := &stripe.CheckoutSessionParams{
		Mode:                  stripe.String(string(stripe.CheckoutSessionModeSetup)),
		Currency:              stripe.String(string(stripe.CurrencyUSD)),
		ClientReferenceID:     stripe.String(workspaceID),
		SuccessURL:            stripe.String(req.SuccessURL),
		CancelURL:             stripe.String(req.CancelURL),
		ExpiresAt:             stripe.Int64(time.Now().Add(checkoutSessionLifetime).Unix()),
		IntegrationIdentifier: stripe.String("bex_billing_" + suffix),
		Metadata:              metadata,
		SetupIntentData:       &stripe.CheckoutSessionSetupIntentDataParams{Metadata: metadata},
	}
	if found {
		params.Customer = stripe.String(customerID)
	} else {
		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	}
	// A setup session has no taxable transaction. Once the operator tax gate is
	// valid, Checkout collects and saves the location/tax-id inputs that the
	// Subscription's future automatic-tax invoices need. customer_update is only
	// valid for an existing Customer; a Checkout-created one gets its tax id
	// from the session and its address from the bound card at adoption.
	if tax := c.taxReadiness(ctx, subscription); tax.Configured {
		params.BillingAddressCollection = stripe.String("required")
		if found {
			params.CustomerUpdate = &stripe.CheckoutSessionCustomerUpdateParams{
				Address: stripe.String("auto"),
				Name:    stripe.String("auto"),
			}
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
	// Record the intent separately from the binding. Best-effort on purpose: this
	// is analytics-grade provenance, and failing the checkout the user is standing
	// in front of to preserve it would be the wrong trade.
	if c.state != nil {
		if err := c.state.MarkCheckoutStarted(ctx, workspaceID, time.Now().UTC()); err != nil {
			log.Printf("billing: record checkout start for %s: %v", workspaceID, err)
		}
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
	checkout, err := c.verifiedCheckout(ctx, eventSession)
	if errors.Is(err, errSupersededCheckout) {
		return nil
	}
	if err != nil {
		return err
	}
	paymentMethod, err := c.verifiedSetupPaymentMethod(ctx, checkout)
	if err != nil {
		return err
	}
	if err := c.bindDefaultPaymentMethod(ctx, checkout, paymentMethod); err != nil {
		return err
	}
	// The verified webhook is the sole enforcement-snapshot writer. A failed
	// stamp fails webhook processing so Stripe retries; acknowledging here while
	// the local gate remained false would strand a paid-intent request even
	// though the provider defaults were successfully bound.
	if c.state != nil {
		if err := c.state.SetPaymentMethodBound(ctx, checkout.workspaceID, time.Now().UTC()); err != nil {
			return fmt.Errorf("stripe: persist payment-method binding for %s: %w", checkout.workspaceID, err)
		}
	}
	return nil
}

// verifiedCheckout is a Checkout Session re-read from Stripe and proven to
// belong to the workspace it claims, together with the Customer and
// Subscription that ownership resolved to.
type verifiedCheckout struct {
	sessionID     string
	setupIntentID string
	workspaceID   string
	customerID    string
	subscription  *stripe.Subscription
	// checkoutCreatedCustomer is true when the session itself created the
	// Customer; binding then also seeds its billing address. It derives from
	// the session, not from whether this delivery adopted it, so a retried
	// webhook sends a byte-identical bind under the same idempotency key.
	checkoutCreatedCustomer bool
}

// errSupersededCheckout reports a completed session whose Checkout-created
// Customer lost to another Customer the workspace acquired meanwhile (two
// tabs, both completed). The loser is deleted and the event acknowledged:
// retrying could never succeed, and the workspace keeps the winner's card.
var errSupersededCheckout = errors.New("checkout superseded by another workspace Customer")

// verifiedCheckout re-reads the event's session from Stripe and proves the
// workspace owns the Customer it names; the event's own metadata is never
// trusted on its own. Only bex creates sessions (with its secret key), so the
// re-read session's metadata is authoritative. A Customer the session itself
// created (customer_creation=always) and that no workspace has claimed is
// adopted here; the Subscription is then ensured, so it never exists before a
// card does.
func (c *StripeClient) verifiedCheckout(ctx context.Context, eventSession *stripe.CheckoutSession) (verifiedCheckout, error) {
	if eventSession == nil || eventSession.ID == "" {
		return verifiedCheckout{}, &inputError{message: "checkout session is missing"}
	}
	params := &stripe.CheckoutSessionParams{}
	params.Context = ctx
	session, err := c.sc.CheckoutSessions.Get(eventSession.ID, params)
	if err != nil {
		return verifiedCheckout{}, fmt.Errorf("stripe: retrieve Checkout Session %s: %w", eventSession.ID, err)
	}
	if !c.expectedLivemode(session.Livemode) || session.Mode != stripe.CheckoutSessionModeSetup || session.Status != stripe.CheckoutSessionStatusComplete {
		return verifiedCheckout{}, &inputError{message: "checkout session is not a completed setup session in the billing environment"}
	}
	workspaceID := session.Metadata[workspaceMetadataKey]
	if workspaceID == "" || session.Customer == nil || session.Customer.ID == "" || session.SetupIntent == nil {
		return verifiedCheckout{}, &inputError{message: "checkout session ownership metadata is incomplete"}
	}
	createdBySession := session.CustomerCreation == stripe.CheckoutSessionCustomerCreationAlways
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil {
		return verifiedCheckout{}, err
	}
	switch {
	case found && customerID == session.Customer.ID:
	case !found && createdBySession:
		if err := c.adoptCheckoutCustomer(ctx, workspaceID, session.ID, session.Customer.ID); err != nil {
			return verifiedCheckout{}, err
		}
		customerID = session.Customer.ID
	case found && createdBySession:
		c.discardCheckoutCustomer(ctx, workspaceID, session.Customer.ID)
		return verifiedCheckout{}, errSupersededCheckout
	default:
		return verifiedCheckout{}, &inputError{message: "checkout session Customer does not belong to the workspace"}
	}
	subscription, err := c.findSubscriptionObject(ctx, workspaceID, customerID)
	if err != nil {
		return verifiedCheckout{}, err
	}
	// Sessions opened while Checkout still pre-minted the Subscription name it;
	// such a claim must still match.
	if claimed := session.Metadata[checkoutSubscriptionMetadataKey]; claimed != "" && (subscription == nil || subscription.ID != claimed) {
		return verifiedCheckout{}, &inputError{message: "checkout session Subscription does not belong to the workspace"}
	}
	if subscription == nil {
		if err := c.EnsureContract(ctx, workspaceID); err != nil {
			return verifiedCheckout{}, err
		}
		if subscription, err = c.findSubscriptionObject(ctx, workspaceID, customerID); err != nil {
			return verifiedCheckout{}, err
		}
		if subscription == nil {
			return verifiedCheckout{}, fmt.Errorf("stripe: subscription missing after ensure for workspace %s", workspaceID)
		}
	}
	return verifiedCheckout{
		sessionID:               session.ID,
		setupIntentID:           session.SetupIntent.ID,
		workspaceID:             workspaceID,
		customerID:              customerID,
		subscription:            subscription,
		checkoutCreatedCustomer: createdBySession,
	}, nil
}

// adoptCheckoutCustomer tags a Checkout-created Customer with its workspace and
// records the mapping. A Customer already tagged for another workspace is
// refused; one already tagged for this workspace is a replayed adoption.
func (c *StripeClient) adoptCheckoutCustomer(ctx context.Context, workspaceID, sessionID, customerID string) error {
	customer, gone, err := c.liveCustomer(ctx, customerID)
	if err != nil {
		return err
	}
	if gone || !c.expectedLivemode(customer.Livemode) {
		return &inputError{message: "checkout session Customer is not adoptable"}
	}
	if owner := customer.Metadata[workspaceMetadataKey]; owner != "" && owner != workspaceID {
		return &inputError{message: "checkout session Customer does not belong to the workspace"}
	}
	update := &stripe.CustomerParams{}
	update.Context = ctx
	update.AddMetadata(workspaceMetadataKey, workspaceID)
	update.SetIdempotencyKey("bex-adopt-customer-" + sessionID)
	if _, err := c.sc.Customers.Update(customerID, update); err != nil {
		return fmt.Errorf("stripe: adopt Checkout Customer for %s: %w", workspaceID, err)
	}
	return c.rememberCustomer(ctx, workspaceID, customerID)
}

// discardCheckoutCustomer deletes a Checkout-created Customer that lost the
// race to another workspace Customer. Best-effort: the abandoned-checkout
// reclaimer retries anything left behind.
func (c *StripeClient) discardCheckoutCustomer(ctx context.Context, workspaceID, customerID string) {
	c.metrics.Operation("checkout_superseded_customer", "success")
	if err := c.deleteCustomer(ctx, customerID); err != nil {
		log.Printf("billing: discard superseded Checkout Customer for %s: %v", workspaceID, err)
	}
}

// verifiedSetupPaymentMethod returns the payment method the session's
// SetupIntent succeeded on, once both the intent and the method are re-read and
// proven to belong to the verified Customer.
func (c *StripeClient) verifiedSetupPaymentMethod(ctx context.Context, checkout verifiedCheckout) (*stripe.PaymentMethod, error) {
	setupParams := &stripe.SetupIntentParams{}
	setupParams.Context = ctx
	setup, err := c.sc.SetupIntents.Get(checkout.setupIntentID, setupParams)
	if err != nil {
		return nil, fmt.Errorf("stripe: retrieve SetupIntent %s: %w", checkout.setupIntentID, err)
	}
	claimed := setup.Metadata[checkoutSubscriptionMetadataKey]
	if setup.Status != stripe.SetupIntentStatusSucceeded || setup.PaymentMethod == nil || setup.Customer == nil || setup.Customer.ID != checkout.customerID || setup.Metadata[workspaceMetadataKey] != checkout.workspaceID || (claimed != "" && claimed != checkout.subscription.ID) {
		return nil, &inputError{message: "SetupIntent does not prove the workspace payment setup"}
	}
	paymentMethodID := setup.PaymentMethod.ID
	pmParams := &stripe.PaymentMethodParams{}
	pmParams.Context = ctx
	paymentMethod, err := c.sc.PaymentMethods.Get(paymentMethodID, pmParams)
	if err != nil {
		return nil, fmt.Errorf("stripe: retrieve PaymentMethod %s: %w", paymentMethodID, err)
	}
	if paymentMethod.Customer == nil || paymentMethod.Customer.ID != checkout.customerID {
		return nil, &inputError{message: "payment method is not attached to the workspace Customer"}
	}
	return paymentMethod, nil
}

// bindDefaultPaymentMethod makes the proven method the default on both the
// Customer and the Subscription. Both idempotency keys derive from the session
// id, so a replayed or reordered webhook converges on the same state.
func (c *StripeClient) bindDefaultPaymentMethod(ctx context.Context, checkout verifiedCheckout, paymentMethod *stripe.PaymentMethod) error {
	paymentMethodID := paymentMethod.ID
	customerUpdate := &stripe.CustomerParams{InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{DefaultPaymentMethod: stripe.String(paymentMethodID)}}
	// A Checkout-created Customer carries no address in setup mode (Stripe only
	// writes it back through customer_update, which needs a pre-existing
	// Customer). Seed it from the card's billing details so tax location reads
	// the same as it did for pre-minted Customers.
	if checkout.checkoutCreatedCustomer {
		customerUpdate.Address = addressParams(paymentMethod)
	}
	customerUpdate.Context = ctx
	customerUpdate.SetIdempotencyKey("bex-payment-customer-" + checkout.sessionID)
	if _, err := c.sc.Customers.Update(checkout.customerID, customerUpdate); err != nil {
		return fmt.Errorf("stripe: bind Customer default payment method: %w", err)
	}
	subscriptionUpdate := &stripe.SubscriptionParams{
		DefaultPaymentMethod: stripe.String(paymentMethodID),
		ProrationBehavior:    stripe.String("none"),
	}
	// The card is now bound, so the subscription is no longer merely "checkout was
	// opened". Clearing the marker here keeps the Stripe side agreeing with
	// payment_method_bound_at, which stays the only authority.
	subscriptionUpdate.AddMetadata(pendingSetupMetadataKey, "")
	if tax := c.taxReadiness(ctx, checkout.subscription); tax.Configured {
		subscriptionUpdate.AutomaticTax = &stripe.SubscriptionAutomaticTaxParams{Enabled: stripe.Bool(true)}
	}
	subscriptionUpdate.Context = ctx
	subscriptionUpdate.SetIdempotencyKey("bex-payment-subscription-" + checkout.sessionID)
	if _, err := c.sc.Subscriptions.Update(checkout.subscription.ID, subscriptionUpdate); err != nil {
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
	// See Readiness: expanded so the card can be named, not just counted.
	params.AddExpand("data.default_payment_method")
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
		c.metrics.Operation("duplicate_subscription", "error")
		ids := make([]string, 0, len(found))
		for _, sub := range found {
			ids = append(ids, sub.ID)
		}
		return nil, fmt.Errorf("stripe: workspace %s has %d live bex subscriptions: %v", workspaceID, len(found), ids)
	}
	if len(found) == 0 {
		return nil, nil
	}
	if err := c.rememberSubscription(ctx, workspaceID, customerID, found[0].ID); err != nil {
		return nil, err
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

// customerCard returns the customer's expanded default card, or nil when there
// is none, it is not a card, or the field was not expanded.
func customerCard(customer *stripe.Customer) *stripe.PaymentMethodCard {
	if customer == nil || customer.InvoiceSettings == nil {
		return nil
	}
	return paymentMethodCard(customer.InvoiceSettings.DefaultPaymentMethod)
}

// subscriptionCard returns the subscription's expanded default card, or nil.
func subscriptionCard(subscription *stripe.Subscription) *stripe.PaymentMethodCard {
	if subscription == nil {
		return nil
	}
	return paymentMethodCard(subscription.DefaultPaymentMethod)
}

func paymentMethodCard(pm *stripe.PaymentMethod) *stripe.PaymentMethodCard {
	if pm == nil || pm.Card == nil || pm.Card.Last4 == "" {
		return nil
	}
	return pm.Card
}

func describeCard(out *Readiness, card *stripe.PaymentMethodCard) {
	if card == nil {
		return
	}
	out.PaymentMethodBrand = string(card.Brand)
	out.PaymentMethodLast4 = card.Last4
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

func addressParams(pm *stripe.PaymentMethod) *stripe.AddressParams {
	if pm == nil || pm.BillingDetails == nil || pm.BillingDetails.Address == nil {
		return nil
	}
	a := pm.BillingDetails.Address
	if a.Country == "" {
		return nil
	}
	return &stripe.AddressParams{
		City: stripe.String(a.City), Country: stripe.String(a.Country),
		Line1: stripe.String(a.Line1), Line2: stripe.String(a.Line2),
		PostalCode: stripe.String(a.PostalCode), State: stripe.String(a.State),
	}
}
