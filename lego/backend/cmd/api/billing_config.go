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

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/billing"
)

// stripeBillingGate resolves the feature gate before parsing any secondary
// Stripe knob. This keeps a deployment with no runtime key completely inert
// even if stale BEX_STRIPE_* values remain in its environment.
func stripeBillingGate(getenv func(string) string, now time.Time) (secret string, epoch time.Time, enabled bool, err error) {
	secret = getenv("BEX_STRIPE_SECRET_KEY")
	if secret == "" {
		return "", time.Time{}, false, nil
	}
	epoch = now.UTC().Add(-billing.BackfillHorizon)
	if value := getenv("BEX_STRIPE_EPOCH"); value != "" {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			return "", time.Time{}, false, fmt.Errorf("bad BEX_STRIPE_EPOCH %q (want RFC3339, e.g. 2026-07-01T00:00:00Z): %w", value, parseErr)
		}
		epoch = parsed.UTC()
	}
	return secret, epoch, true, nil
}

// dunningGate decides whether the Stripe dunning lifecycle runs for the
// resolved key mode. Live enforcement is an operator choice since w4/m81 t002
// (the w7/m52 test-only fence is lifted): the mode is accepted as a parameter
// precisely so this contract is pinned by a test — a reintroduced live fence
// would have to fail here, not in an untestable Fatal inside the wiring.
func dunningGate(getenv func(string) string, expectedLivemode bool) bool {
	_ = expectedLivemode // any mode is supported; the installer's BEX_STRIPE_ALLOW_LIVE is the live gate
	return getenv("BEX_STRIPE_DUNNING_ENABLED") == "1"
}

// paymentMethodMode is the parsed BEX_REQUIRE_PAYMENT_METHOD policy.
type paymentMethodMode int

const (
	// paymentMethodOff — no gate wired; cardless workspaces create freely.
	paymentMethodOff paymentMethodMode = iota
	// paymentMethodPaidIntent — ADR046: only a non-free plan requires the
	// bound-payment-method marker ("1", the pre-ADR075 behavior).
	paymentMethodPaidIntent
	// paymentMethodAllPlans — ADR075 D7 ("all"): every billable create/plan
	// change requires the marker, free tier included.
	paymentMethodAllPlans
)

// paymentMethodGate validates the fail-closed configuration before any server
// wiring. Enforcement needs both hosted Checkout (Stripe key) and the local
// marker store (control-plane DB); enabling only one side would brick every
// paid create with no possible recovery path. An unknown value fails startup
// rather than silently running a weaker policy than the operator asked for.
func paymentMethodGate(getenv func(string) string) (paymentMethodMode, error) {
	value := strings.TrimSpace(getenv("BEX_REQUIRE_PAYMENT_METHOD"))
	var mode paymentMethodMode
	switch value {
	case "", "0":
		return paymentMethodOff, nil
	case "1":
		mode = paymentMethodPaidIntent
	case "all":
		mode = paymentMethodAllPlans
	default:
		return paymentMethodOff, fmt.Errorf("BEX_REQUIRE_PAYMENT_METHOD must be 1 (paid-intent-only) or all (every plan, ADR075 D7) when enabled (got %q)", value)
	}
	if getenv("BEX_STRIPE_SECRET_KEY") == "" {
		return paymentMethodOff, fmt.Errorf("BEX_REQUIRE_PAYMENT_METHOD=%s requires BEX_STRIPE_SECRET_KEY so a refused workspace can add a payment method", value)
	}
	if getenv("BEX_CP_DB_URI") == "" {
		return paymentMethodOff, fmt.Errorf("BEX_REQUIRE_PAYMENT_METHOD=%s requires BEX_CP_DB_URI for the webhook-stamped payment marker", value)
	}
	return mode, nil
}
