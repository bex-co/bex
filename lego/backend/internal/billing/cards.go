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
	"log"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Card checks at binding (ADR091 D10.5–D10.6). A payment method is screened
// where it becomes the workspace's card: Checkout completion and workspace
// creation. A prepaid card, or a card whose Stripe fingerprint carries an
// unpaid agent-model write-off in any workspace (a deleted one included), is
// detached from the Customer and never stamps payment_method_bound_at; the
// reason is stored for the payment gate and the dashboard. Stripe Radar rules
// are configured in the Stripe Dashboard (docs/runbooks/
// stripe-billing-setup.md), not here. Card data and fingerprints are never
// logged.

// CodePaymentMethodRejected is the typed refusal of a card at binding.
const CodePaymentMethodRejected = "PAYMENT_METHOD_REJECTED"

// IsPaymentMethodRejected reports whether err is the typed refusal of a card at
// binding.
func IsPaymentMethodRejected(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == CodePaymentMethodRejected
}

// CardScreenStore persists rejections and answers the fingerprint check;
// *store.PGStore satisfies it.
type CardScreenStore interface {
	RecordPaymentMethodRejection(ctx context.Context, workspaceID string, reason store.PaymentMethodRejection) error
	UnpaidWriteOffForCardFingerprint(ctx context.Context, fingerprint string) (bool, error)
}

// cardRejection is why pm may not be bound, or "" when it may.
func (c *StripeClient) cardRejection(ctx context.Context, pm *stripe.PaymentMethod) (store.PaymentMethodRejection, error) {
	if pm == nil || pm.Card == nil {
		return "", nil
	}
	if pm.Card.Funding == stripe.CardFundingPrepaid {
		return store.PaymentMethodRejectedPrepaid, nil
	}
	if c.cards == nil || pm.Card.Fingerprint == "" {
		return "", nil
	}
	flagged, err := c.cards.UnpaidWriteOffForCardFingerprint(ctx, pm.Card.Fingerprint)
	if err != nil {
		return "", fmt.Errorf("check card against unpaid write-offs: %w", err)
	}
	if flagged {
		return store.PaymentMethodRejectedUnpaidBalance, nil
	}
	return "", nil
}

// refuseCard detaches a refused method from its Customer and records the
// reason. Both steps are idempotent, so a redelivered webhook converges.
func (c *StripeClient) refuseCard(ctx context.Context, workspaceID string, pm *stripe.PaymentMethod, reason store.PaymentMethodRejection) error {
	if err := c.detachPaymentMethod(ctx, pm.ID); err != nil {
		return fmt.Errorf("stripe: detach refused payment method for %s: %w", workspaceID, err)
	}
	c.metrics.Operation("card_refused", string(reason))
	log.Printf("billing: payment method refused at binding for %s: %s", workspaceID, reason)
	if c.cards == nil {
		return nil
	}
	if err := c.cards.RecordPaymentMethodRejection(ctx, workspaceID, reason); err != nil {
		return fmt.Errorf("record payment-method rejection for %s: %w", workspaceID, err)
	}
	return nil
}

// detachPaymentMethod detaches a method from its Customer. Idempotent: a
// method that is missing, or already detached (a replay after an earlier
// detach succeeded), is not an error. The error carries only Stripe's code.
func (c *StripeClient) detachPaymentMethod(ctx context.Context, id string) error {
	detach := &stripe.PaymentMethodDetachParams{}
	detach.Context = ctx
	_, err := c.sc.PaymentMethods.Detach(id, detach)
	if err == nil || resourceMissing(err) {
		return nil
	}
	get := &stripe.PaymentMethodParams{}
	get.Context = ctx
	if pm, gerr := c.sc.PaymentMethods.Get(id, get); gerr == nil && pm.Customer == nil {
		return nil
	}
	code, _ := safeStripeError(err)
	return errors.New(code)
}

// NewPaymentMethodRejectedError is the typed refusal workspace creation
// returns for a refused card.
func NewPaymentMethodRejectedError(reason store.PaymentMethodRejection) *core.CodedError {
	msg := "Prepaid cards aren't accepted; add a credit or debit card"
	if reason == store.PaymentMethodRejectedUnpaidBalance {
		msg = "This card is linked to an unpaid balance; add a different card"
	}
	return core.NewBadRequestError(CodePaymentMethodRejected, msg, map[string]any{"reason": string(reason)})
}

// WrittenOffCardFingerprint is the fingerprint of the card that owes a
// written-off invoice (ADR091 D10.6): the card whose charge of the invoice
// failed, read from the invoice's payments, else the Customer's default card.
// "" when neither is a card. Only the fingerprint leaves this function.
func (c *StripeClient) WrittenOffCardFingerprint(ctx context.Context, invoiceID, customerID string) (string, error) {
	params := &stripe.InvoicePaymentListParams{Invoice: stripe.String(invoiceID)}
	params.Context = ctx
	params.Limit = stripe.Int64(10)
	params.AddExpand("data.payment.payment_intent.latest_charge")
	params.AddExpand("data.payment.charge")
	iter := c.sc.InvoicePayments.List(params)
	for iter.Next() {
		if fingerprint := failedCardFingerprint(iter.InvoicePayment()); fingerprint != "" {
			return fingerprint, nil
		}
	}
	if err := iter.Err(); err != nil && !resourceMissing(err) {
		code, _ := safeStripeError(err)
		return "", fmt.Errorf("stripe: list payments of written-off invoice: %s", code)
	}
	return c.defaultCardFingerprint(ctx, customerID)
}

// failedCardFingerprint is the card fingerprint of an invoice payment's last
// charge attempt: the PaymentIntent's latest charge, else the method its last
// error names, else a bare charge.
func failedCardFingerprint(payment *stripe.InvoicePayment) string {
	if payment == nil || payment.Payment == nil {
		return ""
	}
	if pi := payment.Payment.PaymentIntent; pi != nil {
		if fp := chargeCardFingerprint(pi.LatestCharge); fp != "" {
			return fp
		}
		if pi.LastPaymentError != nil && pi.LastPaymentError.PaymentMethod != nil && pi.LastPaymentError.PaymentMethod.Card != nil {
			return pi.LastPaymentError.PaymentMethod.Card.Fingerprint
		}
	}
	return chargeCardFingerprint(payment.Payment.Charge)
}

func chargeCardFingerprint(charge *stripe.Charge) string {
	if charge == nil || charge.PaymentMethodDetails == nil || charge.PaymentMethodDetails.Card == nil {
		return ""
	}
	return charge.PaymentMethodDetails.Card.Fingerprint
}

// defaultCardFingerprint is the fingerprint of the Customer's default card, or
// "" when it has none or it is not a card.
func (c *StripeClient) defaultCardFingerprint(ctx context.Context, customerID string) (string, error) {
	params := &stripe.CustomerParams{}
	params.Context = ctx
	params.AddExpand("invoice_settings.default_payment_method")
	customer, err := c.sc.Customers.Get(customerID, params)
	if resourceMissing(err) {
		return "", nil
	}
	if err != nil {
		code, _ := safeStripeError(err)
		return "", fmt.Errorf("stripe: retrieve written-off customer: %s", code)
	}
	if customer.InvoiceSettings == nil || customer.InvoiceSettings.DefaultPaymentMethod == nil || customer.InvoiceSettings.DefaultPaymentMethod.Card == nil {
		return "", nil
	}
	return customer.InvoiceSettings.DefaultPaymentMethod.Card.Fingerprint, nil
}
