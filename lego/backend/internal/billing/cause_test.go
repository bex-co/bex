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
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// leakedCause is a dependency's error text a client must never see: it names
// an internal address and a provider account.
const leakedCause = "dial tcp 10.0.3.7:5432: connection refused (acct_1Restricted)"

type failingLifecycle struct{}

func (failingLifecycle) GetBillingLifecycle(context.Context, string) (store.BillingLifecycle, error) {
	return store.BillingLifecycle{}, errors.New(leakedCause)
}

// TestABillingStatusFailureAnswersWithoutItsCause (w5/123): a failed provider
// readiness, payment-method marker or lifecycle read answers "billing
// integration unavailable" alone. The cause is logged once, on one line.
// api's TestABillingFailureAnswersWithoutItsCause pins the three surfaces.
func TestABillingStatusFailureAnswersWithoutItsCause(t *testing.T) {
	for name, configure := range map[string]func(*Service){
		"provider readiness": func(s *Service) { s.Provider = &billingProviderFake{err: errors.New(leakedCause)} },
		"payment-method marker": func(s *Service) {
			s.Payment = &PaymentGate{Store: &paymentEligibilityStoreFake{err: errors.New(leakedCause)}}
			s.PaymentAllPlans = true
		},
		"lifecycle": func(s *Service) { s.State = failingLifecycle{} },
	} {
		t.Run(name, func(t *testing.T) {
			var logged bytes.Buffer
			prev := log.Writer()
			log.SetOutput(&logged)
			t.Cleanup(func() { log.SetOutput(prev) })

			svc := billingTestService(&billingProviderFake{status: Readiness{Mode: "test"}})
			configure(svc)
			_, err := svc.Status(billingIdentity(context.Background()), "tea-a")
			if !errors.Is(err, core.ErrBillingUnavailable) || err.Error() != core.ErrBillingUnavailable.Error() {
				t.Fatalf("error = %v, want %q alone", err, core.ErrBillingUnavailable)
			}
			if lines := strings.Count(logged.String(), "\n"); lines != 1 || !strings.Contains(logged.String(), leakedCause) {
				t.Fatalf("logged %d lines, want the cause once:\n%s", lines, logged.String())
			}
		})
	}
}

// TestAHostedSessionProviderFailureAnswersWithoutItsCause (w5/123): a
// checkout or portal session the provider fails for an operator reason answers
// "billing integration unavailable", without the provider's text.
func TestAHostedSessionProviderFailureAnswersWithoutItsCause(t *testing.T) {
	svc := billingTestService(&billingProviderFake{err: errors.New(leakedCause)})
	ctx := billingIdentity(context.Background())
	_, checkoutErr := svc.Checkout(ctx, "tea-a", CheckoutRequest{})
	_, portalErr := svc.Portal(ctx, "tea-a", PortalRequest{})
	for verb, err := range map[string]error{"checkout": checkoutErr, "portal": portalErr} {
		if !errors.Is(err, core.ErrBillingUnavailable) || err.Error() != core.ErrBillingUnavailable.Error() {
			t.Errorf("%s error = %v, want %q alone", verb, err, core.ErrBillingUnavailable)
		}
	}
}
