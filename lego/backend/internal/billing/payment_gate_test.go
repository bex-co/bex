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
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type paymentEligibilityStoreFake struct {
	eligibility store.PaymentEligibility
	err         error
	workspaceID string
}

func (f *paymentEligibilityStoreFake) PaymentEligibility(_ context.Context, workspaceID string) (store.PaymentEligibility, error) {
	f.workspaceID = workspaceID
	return f.eligibility, f.err
}

func TestPaymentGateRequiresBoundMethodUnlessWorkspaceIsExempt(t *testing.T) {
	for _, tc := range []struct {
		name        string
		eligibility store.PaymentEligibility
		wantError   bool
	}{
		{name: "cardless", wantError: true},
		{name: "bound", eligibility: store.PaymentEligibility{Bound: true}},
		{name: "excluded", eligibility: store.PaymentEligibility{Excluded: true}},
		{name: "comped", eligibility: store.PaymentEligibility{Comped: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &paymentEligibilityStoreFake{eligibility: tc.eligibility}
			err := (&PaymentGate{Store: st}).RequirePaymentMethod(context.Background(), "tea-a")
			if tc.wantError {
				if !errors.Is(err, core.ErrPaymentRequired) {
					t.Fatalf("error = %v, want ErrPaymentRequired", err)
				}
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Extensions()["code"] != "PAYMENT_REQUIRED" || err.Error() != core.PaymentRequiredMessage {
					t.Fatalf("payment refusal = %#v, want actionable PAYMENT_REQUIRED", err)
				}
			} else if err != nil {
				t.Fatalf("exempt eligibility rejected: %v", err)
			}
			if st.workspaceID != "tea-a" {
				t.Fatalf("workspace = %q, want tea-a", st.workspaceID)
			}
		})
	}
}

func TestPaymentGateFailsClosedWhenLocalSnapshotCannotBeRead(t *testing.T) {
	st := &paymentEligibilityStoreFake{err: errors.New("database unavailable")}
	err := (&PaymentGate{Store: st}).RequirePaymentMethod(context.Background(), "tea-a")
	if !errors.Is(err, core.ErrBillingUnavailable) {
		t.Fatalf("error = %v, want ErrBillingUnavailable", err)
	}
}
