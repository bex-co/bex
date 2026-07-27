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
	"encoding/json"
	"io"
	"log"
	"net/http"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

const maxStripeWebhookBytes = 1 << 20

// StripeWebhook verifies Stripe's signature before accepting events. Checkout
// completion binds the payment method after authoritative Stripe re-reads;
// payment-failure intake remains deliberately non-enforcing until the dunning
// ladder lands. A nil failure callback records the verified event for operators
// without mutating tenant state.
type StripeWebhook struct {
	Secret                 string
	OnInvoicePaymentFailed func(*stripe.Invoice) error
	OnCheckoutCompleted    func(context.Context, *stripe.CheckoutSession) error
}

func (h *StripeWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Secret == "" {
		http.Error(w, "Stripe billing webhook unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxStripeWebhookBytes+1))
	if err != nil {
		http.Error(w, "read webhook", http.StatusBadRequest)
		return
	}
	if len(body) > maxStripeWebhookBytes {
		http.Error(w, "webhook body too large", http.StatusRequestEntityTooLarge)
		return
	}
	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.Secret)
	if err != nil {
		http.Error(w, "invalid Stripe signature", http.StatusBadRequest)
		return
	}
	if event.Type == stripe.EventTypeInvoicePaymentFailed {
		var invoice stripe.Invoice
		if event.Data == nil || json.Unmarshal(event.Data.Raw, &invoice) != nil {
			http.Error(w, "invalid invoice event", http.StatusBadRequest)
			return
		}
		if h.OnInvoicePaymentFailed != nil {
			if err := h.OnInvoicePaymentFailed(&invoice); err != nil {
				log.Printf("billing: Stripe invoice.payment_failed handler for %s: %v", invoice.ID, err)
				http.Error(w, "payment-failure handler unavailable", http.StatusServiceUnavailable)
				return
			}
		} else {
			customerID := ""
			if invoice.Customer != nil {
				customerID = invoice.Customer.ID
			}
			log.Printf("billing: verified Stripe invoice.payment_failed invoice=%s customer=%s (enforcement deferred)", invoice.ID, customerID)
		}
	}
	if event.Type == stripe.EventTypeCheckoutSessionCompleted {
		var session stripe.CheckoutSession
		if event.Data == nil || json.Unmarshal(event.Data.Raw, &session) != nil {
			http.Error(w, "invalid Checkout Session event", http.StatusBadRequest)
			return
		}
		if h.OnCheckoutCompleted != nil {
			if err := h.OnCheckoutCompleted(r.Context(), &session); err != nil {
				log.Printf("billing: Stripe checkout.session.completed handler for %s: %v", session.ID, err)
				http.Error(w, "payment-setup handler unavailable", http.StatusServiceUnavailable)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
