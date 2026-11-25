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

package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// The outcomes of one attempt to pay an overdue invoice with a newly bound
// card (w6/m626, migration 0346); a claimed attempt is "pending" until then.
const (
	OverduePaymentPaid     = "paid"
	OverduePaymentDeclined = "declined"
	OverduePaymentError    = "error"
)

// OverduePaymentReclaimAfter is how long an attempt may stay pending before
// it counts as abandoned (the process died between the claim and Stripe's
// answer) and the card may be tried again.
const OverduePaymentReclaimAfter = 10 * time.Minute

// ClaimOverdueInvoicePayment claims the one attempt bex makes to pay invoiceID
// with the card cardKey names. It returns the claim's attempt number, or 0
// when the card already had its attempt: paid, declined, or still pending.
// An attempt that failed for a reason other than the card ("error"), or that
// was abandoned pending for OverduePaymentReclaimAfter, is claimed again under
// the next number, so it gets its own Stripe idempotency key.
func (s *PGStore) ClaimOverdueInvoicePayment(ctx context.Context, workspaceID, invoiceID, cardKey string, at time.Time) (int, error) {
	var attempt int
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO billing_overdue_payments (invoice_id, card_key, workspace_id, outcome, attempted_at)
		VALUES ($1, $2, $3, 'pending', $4)
		ON CONFLICT (invoice_id, card_key) DO UPDATE
		SET outcome = 'pending', reason = '', attempts = billing_overdue_payments.attempts + 1,
		    attempted_at = EXCLUDED.attempted_at
		WHERE billing_overdue_payments.workspace_id = EXCLUDED.workspace_id
		  AND (billing_overdue_payments.outcome = 'error'
		       OR (billing_overdue_payments.outcome = 'pending'
		           AND billing_overdue_payments.attempted_at < $5))
		RETURNING attempts`, invoiceID, cardKey, workspaceID, at, at.Add(-OverduePaymentReclaimAfter)).Scan(&attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return attempt, err
}

// RecordOverdueInvoicePayment records how the claimed attempt ended, with the
// decline code or Stripe error code as its reason.
func (s *PGStore) RecordOverdueInvoicePayment(ctx context.Context, workspaceID, invoiceID, cardKey, outcome, reason string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE billing_overdue_payments SET outcome = $4, reason = $5
		WHERE invoice_id = $1 AND card_key = $2 AND workspace_id = $3`,
		invoiceID, cardKey, workspaceID, outcome, reason)
	return err
}
