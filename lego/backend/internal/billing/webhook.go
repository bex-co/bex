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

// StripeWebhook verifies Stripe's signature and pinned API version before
// dispatch. Checkout completion performs its authoritative object re-reads;
// lifecycle events are reduced and committed by OnLifecycle before the 204.
type StripeWebhook struct {
	Secret              string
	ExpectedLivemode    bool
	OnLifecycle         func(context.Context, *stripe.Event) error
	OnCheckoutCompleted func(context.Context, *stripe.CheckoutSession) error
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
	if event.APIVersion != stripe.APIVersion {
		http.Error(w, "incompatible Stripe API version", http.StatusBadRequest)
		return
	}
	if event.Livemode != h.ExpectedLivemode {
		http.Error(w, "Stripe event mode mismatch", http.StatusBadRequest)
		return
	}
	if isLifecycleEvent(event.Type) && h.OnLifecycle != nil {
		if err := h.OnLifecycle(r.Context(), &event); err != nil {
			log.Printf("billing: Stripe lifecycle handler event=%s type=%s: %v", event.ID, event.Type, err)
			http.Error(w, "billing lifecycle unavailable", http.StatusServiceUnavailable)
			return
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
