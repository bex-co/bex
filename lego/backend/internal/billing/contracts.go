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
	"fmt"
	"net/http"
	"time"

	metronome "github.com/Metronome-Industries/metronome-go/v3"
	"github.com/Metronome-Industries/metronome-go/v3/option"
)

// compCreditAmount is the credit granted to a comped (Mode B) customer: large
// enough to zero out any realistic invoice, so the net always nets to $0
// (ADR040 §7 Mode B). It is in the org's USD credit type unit (cents if the
// credit type is "USD (cents)").
const compCreditAmount = 1_000_000_000 // $10M in cents — effectively unbounded comp

// EnsureContract binds a customer to the rate card so Metronome actually rates
// their exported usage into invoices (ADR040 §2, §7: per-workspace charging is
// a contract). It is a no-op when no rate card is configured (RateCardID unset
// ⇒ m47 shadow-export behavior, byte-identical) and idempotent otherwise: a
// process-local cache and a list-before-create both prevent a second contract.
// billing_excluded customers never reach here — the emitter filters them at the
// source — so they get no contract.
func (c *Client) EnsureContract(ctx context.Context, customerID string) error {
	if c.RateCardID == "" {
		return nil // shadow-export only: usage lands in Metronome, rated by nothing
	}
	if c.cached(c.contracted, customerID) {
		return nil
	}
	list, err := c.mc.V1.Contracts.List(ctx, metronome.V1ContractListParams{CustomerID: customerID}, readOpts)
	if err != nil {
		return fmt.Errorf("metronome: list contracts %s: %w", customerID, err)
	}
	if len(list.Data) > 0 {
		c.mark(c.contracted, customerID) // already contracted
		return nil
	}
	if _, err := c.mc.V1.Contracts.New(ctx, metronome.V1ContractNewParams{
		CustomerID: customerID,
		StartingAt: c.contractStart(),
		RateCardID: metronome.String(c.RateCardID),
	}, option.WithMaxRetries(3)); err != nil {
		return fmt.Errorf("metronome: create contract %s: %w", customerID, err)
	}
	c.mark(c.contracted, customerID)
	return nil
}

// CompCustomer applies ADR040 §7 Mode B: the customer keeps a real contract
// (so Metronome rates their usage into real line items) but a credit ≥ any
// balance nets every invoice to $0. It is the comp *mechanism* — meant to be
// driven by a deliberate admin action, never automatically — and is idempotent
// via the credit grant's uniqueness key, so re-invoking it is safe. Wiring an
// audited control-plane comp verb (mirroring m47's billing-excluded verb) is a
// follow-up; today CompCustomer is invoked out-of-band. Non-collectible marking
// is a Phase-3 (collection) belt-and-suspenders; with no collection layer yet,
// the ≥balance credit alone guarantees $0 due.
func (c *Client) CompCustomer(ctx context.Context, customerID string) error {
	if c.RateCardID == "" {
		return errors.New("billing: comp requires a configured rate card (BEX_METRONOME_RATE_CARD_ID)")
	}
	if c.USDCreditTypeID == "" {
		return errors.New("billing: comp requires a configured USD credit type (BEX_METRONOME_USD_CREDIT_TYPE_ID)")
	}
	if err := c.EnsureContract(ctx, customerID); err != nil {
		return err
	}
	_, err := c.mc.V1.CreditGrants.New(ctx, metronome.V1CreditGrantNewParams{
		CustomerID: customerID,
		Name:       "bex comp " + customerID,
		ExpiresAt:  c.contractStart().AddDate(100, 0, 0),
		Priority:   1,
		GrantAmount: metronome.V1CreditGrantNewParamsGrantAmount{
			Amount:       compCreditAmount,
			CreditTypeID: c.USDCreditTypeID,
		},
		PaidAmount: metronome.V1CreditGrantNewParamsPaidAmount{
			Amount:       0, // comped: the customer paid nothing for the credit
			CreditTypeID: c.USDCreditTypeID,
		},
		Reason:        metronome.String("bex comp (ADR040 §7 Mode B)"),
		UniquenessKey: metronome.String("bex-comp-" + customerID), // idempotent
	}, option.WithMaxRetries(3))
	if err != nil {
		var apiErr *metronome.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			return nil // the uniqueness key already applied this comp
		}
		return fmt.Errorf("metronome: comp credit for %s: %w", customerID, err)
	}
	return nil
}

// contractStart is the contract's starting_at: the billing epoch (usage before
// it is not rated) when configured, else now.
func (c *Client) contractStart() time.Time {
	if !c.ContractStart.IsZero() {
		return c.ContractStart.UTC()
	}
	return time.Now().UTC()
}

func (c *Client) contractCached(customerID string) bool {
	c.ensuredMu.Lock()
	defer c.ensuredMu.Unlock()
	_, ok := c.contracted[customerID]
	return ok
}

func (c *Client) markContract(customerID string) {
	c.ensuredMu.Lock()
	c.contracted[customerID] = struct{}{}
	c.ensuredMu.Unlock()
}
