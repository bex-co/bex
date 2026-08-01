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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// PaymentEligibilityStore is the one local read required on a paid mutation.
// *store.PGStore satisfies it; no Stripe network call can enter this path.
type PaymentEligibilityStore interface {
	PaymentEligibility(context.Context, string) (store.PaymentEligibility, error)
}

// PaymentGate implements core.PaymentGate over the webhook-stamped marker and
// the two operator-owned billing exemptions.
type PaymentGate struct {
	Store PaymentEligibilityStore
}

var _ core.PaymentGate = (*PaymentGate)(nil)

func (g *PaymentGate) RequirePaymentMethod(ctx context.Context, workspaceID string) error {
	if g == nil || g.Store == nil {
		return core.ErrBillingUnavailable
	}
	eligibility, err := g.Store.PaymentEligibility(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("%w: read payment-method marker: %v", core.ErrBillingUnavailable, err)
	}
	if eligibility.AllowsPaidIntent() {
		return nil
	}
	return core.NewPaymentRequiredError()
}
