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

func envGetter(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
