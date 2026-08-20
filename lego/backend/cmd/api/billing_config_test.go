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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/billing"
)

func TestStripeBillingGateDisabledIgnoresSecondaryConfig(t *testing.T) {
	env := map[string]string{"BEX_STRIPE_EPOCH": "not-a-time"}
	secret, epoch, enabled, err := stripeBillingGate(envGetter(env), time.Now())
	if err != nil || enabled || secret != "" || !epoch.IsZero() {
		t.Fatalf("disabled gate = secret %q epoch %v enabled %v err %v", secret, epoch, enabled, err)
	}
}

func TestStripeBillingGateRejectsMalformedEnabledEpoch(t *testing.T) {
	env := map[string]string{"BEX_STRIPE_SECRET_KEY": "rk_test", "BEX_STRIPE_EPOCH": "not-a-time"}
	_, _, _, err := stripeBillingGate(envGetter(env), time.Now())
	if err == nil || !strings.Contains(err.Error(), "bad BEX_STRIPE_EPOCH") {
		t.Fatalf("malformed enabled epoch error = %v", err)
	}
}

func TestStripeBillingGateDefaultsAndParsesEpoch(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	secret, epoch, enabled, err := stripeBillingGate(envGetter(map[string]string{"BEX_STRIPE_SECRET_KEY": "rk_test"}), now)
	if err != nil || !enabled || secret != "rk_test" || !epoch.Equal(now.Add(-billing.BackfillHorizon)) {
		t.Fatalf("default gate = secret %q epoch %v enabled %v err %v", secret, epoch, enabled, err)
	}

	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	_, epoch, enabled, err = stripeBillingGate(envGetter(map[string]string{
		"BEX_STRIPE_SECRET_KEY": "rk_test",
		"BEX_STRIPE_EPOCH":      "2026-07-01T00:00:00Z",
	}), now)
	if err != nil || !enabled || !epoch.Equal(want) {
		t.Fatalf("explicit gate = epoch %v enabled %v err %v", epoch, enabled, err)
	}
}

func TestPaymentMethodGateDisabledIsByteCompatible(t *testing.T) {
	for _, value := range []string{"", "0"} {
		mode, err := paymentMethodGate(envGetter(map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": value}))
		if err != nil || mode != paymentMethodOff {
			t.Fatalf("value %q = mode %v err %v, want off", value, mode, err)
		}
	}
}

func TestPaymentMethodGateRequiresStripeAndControlPlaneStore(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "invalid value", env: map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": "true"}, want: "must be 1"},
		{name: "missing Stripe", env: map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": "1", "BEX_CP_DB_URI": "postgres://db"}, want: "BEX_STRIPE_SECRET_KEY"},
		{name: "missing database", env: map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": "1", "BEX_STRIPE_SECRET_KEY": "rk_test"}, want: "BEX_CP_DB_URI"},
		{name: "all missing Stripe", env: map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": "all", "BEX_CP_DB_URI": "postgres://db"}, want: "BEX_STRIPE_SECRET_KEY"},
		{name: "all missing database", env: map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": "all", "BEX_STRIPE_SECRET_KEY": "rk_test"}, want: "BEX_CP_DB_URI"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := paymentMethodGate(envGetter(tc.env))
			if mode != paymentMethodOff || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("mode=%v err=%v, want error containing %q", mode, err, tc.want)
			}
		})
	}

	for value, want := range map[string]paymentMethodMode{
		"1":   paymentMethodPaidIntent,
		"all": paymentMethodAllPlans,
	} {
		mode, err := paymentMethodGate(envGetter(map[string]string{
			"BEX_REQUIRE_PAYMENT_METHOD": value,
			"BEX_STRIPE_SECRET_KEY":      "rk_test",
			"BEX_CP_DB_URI":              "postgres://db",
		}))
		if err != nil || mode != want {
			t.Fatalf("fully configured %q = mode %v err %v, want %v", value, mode, err, want)
		}
	}
}

// The w7/m52 test-only dunning fence is lifted (w4/m81 t002): live and test
// modes are equally bootable dunning configurations, gated only by the opt-in
// env. A reintroduced live fence would have to fail this contract.
func TestDunningGateAcceptsBothModes(t *testing.T) {
	enabledEnv := envGetter(map[string]string{"BEX_STRIPE_DUNNING_ENABLED": "1"})
	for _, live := range []bool{false, true} {
		if !dunningGate(enabledEnv, live) {
			t.Fatalf("dunning enabled with livemode=%t = false, want true", live)
		}
		if dunningGate(envGetter(nil), live) {
			t.Fatalf("dunning default with livemode=%t = true, want false", live)
		}
	}
}

func envGetter(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
