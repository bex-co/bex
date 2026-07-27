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
