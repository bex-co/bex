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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// staleAllowChecker models the codex round-8 #8 window: the cached path (Check)
// still answers a warm positive while the source of truth (CheckFresh) already
// says the membership is gone — a member revoked on another replica inside
// PositiveTTL.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// codex round-8 #8: a hosted Checkout Session binds a payment method to the
// workspace — durable financial-capability issuance — so a revoked member
// riding a stale positive must be refused before the provider is ever called.
func TestCheckoutFailsClosedOnFreshRevocation(t *testing.T) {
	provider := &billingProviderFake{}
	svc := billingTestService(provider)
	svc.Authz = staleAllowChecker{}

	if _, err := svc.Checkout(billingIdentity(context.Background()), "tea-a", CheckoutRequest{}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Checkout on a stale positive: %v, want ErrForbidden", err)
	}
	if provider.workspace != "" {
		t.Fatalf("denied Checkout reached the provider for workspace %q", provider.workspace)
	}
}

// codex round-8 #8: same class as Checkout — the Portal Session is a durable
// financial surface, refused before the provider on a fresh denial.
func TestPortalFailsClosedOnFreshRevocation(t *testing.T) {
	provider := &billingProviderFake{}
	svc := billingTestService(provider)
	svc.Authz = staleAllowChecker{}

	if _, err := svc.Portal(billingIdentity(context.Background()), "tea-a", PortalRequest{}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Portal on a stale positive: %v, want ErrForbidden", err)
	}
	if provider.workspace != "" {
		t.Fatalf("denied Portal reached the provider for workspace %q", provider.workspace)
	}
}
