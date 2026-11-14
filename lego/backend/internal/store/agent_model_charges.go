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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// The agent-model threshold charge ledger (ADR091 D10.2a, migration 0247):
// bex charges the card the moment a workspace's unbilled agent-model spend
// reaches its trust tier's charge threshold, and each crossing is one row.

// Agent-model threshold charge statuses.
const (
	AgentModelChargePending = "pending"
	AgentModelChargePaid    = "paid"
	AgentModelChargeFailed  = "failed"
)

// AgentModelCrossing is one spend read that found an agent-model threshold
// charge due (ADR091 D10.2a): the mint makes it and the charger charges it.
type AgentModelCrossing struct {
	WorkspaceID string
	// At is when the spend was read. SeenSeq is the month's last charge in
	// that read: a charge numbered past it already covers this crossing, so
	// the charger joins it.
	At      time.Time
	SeenSeq int
	// UnbilledMicroUSD is what the read left to charge after the billing
	// credits it counted (CreditMicroUSD). When the charger reads a
	// different balance, it charges nothing and the spend is read again.
	UnbilledMicroUSD int64
	CreditMicroUSD   int64
}

// AgentModelChargeKey names one charge: the workspace's seq-th crossing in
// the calendar month (UTC) starting at Month.
type AgentModelChargeKey struct {
	WorkspaceID string
	Month       time.Time
	Seq         int
}

// AgentModelCharge is one threshold charge.
type AgentModelCharge struct {
	AgentModelChargeKey
	// AmountMicroUSD is the unbilled spend the crossing charged; AmountCents
	// what its invoice bills (rounded up, at least Stripe's minimum).
	AmountMicroUSD int64
	AmountCents    int64
	Status         string
	// InvoiceID is the standalone Stripe invoice, set before anything can
	// charge it; HostedInvoiceURL is where the customer pays it.
	InvoiceID        string
	HostedInvoiceURL string
	// Failure is the Stripe decline code of a failed charge.
	Failure string
	// NetItemID is the negative pending invoice item on the Subscription
	// that nets a paid charge out of the next cycle invoice.
	NetItemID string
	// Attempts counts the tries Stripe could not decide.
	Attempts  int
	CreatedAt time.Time
	// SettledAt is when the charge was paid or failed; zero while pending.
	SettledAt time.Time
}

// AgentModelChargeSettlement is the outcome a pending charge settles to.
type AgentModelChargeSettlement struct {
	Status           string // AgentModelChargePaid or AgentModelChargeFailed
	HostedInvoiceURL string
	Failure          string
	At               time.Time
}

const agentModelChargeColumns = `workspace_id, month, seq, amount_microusd, amount_cents, status,
	COALESCE(invoice_id, ''), COALESCE(hosted_invoice_url, ''), COALESCE(failure, ''), COALESCE(net_item_id, ''),
	attempts, created_at, settled_at`

func scanAgentModelCharge(row pgx.Row) (AgentModelCharge, error) {
	var (
		c       AgentModelCharge
		settled pgtype.Timestamptz
	)
	if err := row.Scan(&c.WorkspaceID, &c.Month, &c.Seq, &c.AmountMicroUSD, &c.AmountCents, &c.Status,
		&c.InvoiceID, &c.HostedInvoiceURL, &c.Failure, &c.NetItemID, &c.Attempts, &c.CreatedAt, &settled); err != nil {
		return AgentModelCharge{}, err
	}
	c.Month, c.CreatedAt = c.Month.UTC(), c.CreatedAt.UTC()
	if settled.Valid {
		c.SettledAt = settled.Time.UTC()
	}
	return c, nil
}

func collectAgentModelCharges(rows pgx.Rows, err error) ([]AgentModelCharge, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentModelCharge, error) { return scanAgentModelCharge(row) })
}

// BeginAgentModelCharge starts workspaceID's charge for a crossing it read
// at now, or joins the one that makes it moot (started false):
//   - a charge already pending (in any month) is returned as it is, so
//     concurrent mints on every replica make exactly one charge;
//   - a charge numbered past seenSeq (the month's last charge the caller's
//     read saw) is returned: the caller's spend read predates it, so its
//     amount would charge usage that charge already covered;
//   - otherwise a pending charge of amountMicroUSD (billed as amountCents) is
//     inserted as the month's next seq.
//
// The partial unique index on pending rows is what makes the start single:
// a concurrent insert that loses does nothing, and the re-read returns the
// winner. ErrWorkspaceGone when the workspace was deleted.
func (s *PGStore) BeginAgentModelCharge(ctx context.Context, workspaceID string, now time.Time, seenSeq int, amountMicroUSD, amountCents int64) (charge AgentModelCharge, started bool, err error) {
	if workspaceID == "" || now.IsZero() || seenSeq < 0 || amountMicroUSD <= 0 || amountCents <= 0 {
		return AgentModelCharge{}, false, fmt.Errorf("agent-model charge needs a workspace, a time and a positive amount")
	}
	month := MonthStartUTC(now)
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// Two passes: the second only follows an insert that lost a race,
		// whose winner the re-read then finds.
		for range 2 {
			var err error
			charge, err = scanAgentModelCharge(tx.QueryRow(ctx, `SELECT `+agentModelChargeColumns+`
				FROM agent_model_threshold_charges WHERE workspace_id = $1 AND status = 'pending'`, workspaceID))
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			latest := 0
			charge, err = scanAgentModelCharge(tx.QueryRow(ctx, `SELECT `+agentModelChargeColumns+`
				FROM agent_model_threshold_charges WHERE workspace_id = $1 AND month = $2
				ORDER BY seq DESC LIMIT 1`, workspaceID, month))
			switch {
			case err == nil:
				if charge.Seq > seenSeq {
					return nil
				}
				latest = charge.Seq
			case !errors.Is(err, pgx.ErrNoRows):
				return err
			}
			charge, err = scanAgentModelCharge(tx.QueryRow(ctx, `
				INSERT INTO agent_model_threshold_charges
				  (workspace_id, month, seq, amount_microusd, amount_cents, status, created_at)
				VALUES ($1, $2, $3, $4, $5, 'pending', $6)
				ON CONFLICT DO NOTHING
				RETURNING `+agentModelChargeColumns,
				workspaceID, month, latest+1, amountMicroUSD, amountCents, now.UTC()))
			if err == nil {
				started = true
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return trustWriteError(workspaceID, err)
			}
		}
		return fmt.Errorf("agent-model charge for %s: no charge to start or join", workspaceID)
	})
	if err != nil {
		return AgentModelCharge{}, false, err
	}
	return charge, started, nil
}

// GetAgentModelCharge reads one charge; ErrNotFound when it does not exist.
func (s *PGStore) GetAgentModelCharge(ctx context.Context, key AgentModelChargeKey) (AgentModelCharge, error) {
	charge, err := scanAgentModelCharge(s.Pool.QueryRow(ctx, `SELECT `+agentModelChargeColumns+`
		FROM agent_model_threshold_charges WHERE workspace_id = $1 AND month = $2 AND seq = $3`,
		key.WorkspaceID, MonthStartUTC(key.Month), key.Seq))
	if err != nil {
		return AgentModelCharge{}, classify("agent-model charge", err)
	}
	return charge, nil
}

// SetAgentModelChargeInvoice records the Stripe invoice of a pending charge,
// before anything charges it, so a retry pays that invoice instead of making
// another. ErrNotFound when the charge is no longer pending.
func (s *PGStore) SetAgentModelChargeInvoice(ctx context.Context, key AgentModelChargeKey, invoiceID string) error {
	if invoiceID == "" {
		return fmt.Errorf("agent-model charge invoice id is empty")
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_model_threshold_charges SET invoice_id = $4
		WHERE workspace_id = $1 AND month = $2 AND seq = $3 AND status = 'pending'`,
		key.WorkspaceID, MonthStartUTC(key.Month), key.Seq, invoiceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent-model charge %s/%s/%d is not pending: %w", key.WorkspaceID, MonthStartUTC(key.Month).Format("2006-01"), key.Seq, ErrNotFound)
	}
	return nil
}

// CountAgentModelChargeAttempt records a try Stripe could not decide; the
// charge stays pending for the sweep.
func (s *PGStore) CountAgentModelChargeAttempt(ctx context.Context, key AgentModelChargeKey) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_model_threshold_charges SET attempts = attempts + 1
		WHERE workspace_id = $1 AND month = $2 AND seq = $3 AND status = 'pending'`,
		key.WorkspaceID, MonthStartUTC(key.Month), key.Seq)
	return err
}

// agentModelChargeSettles is the transition a settlement may make: a pending
// charge settles either way, and a failed one becomes paid when its invoice
// is paid later. Paid is final. $n is the settlement's status.
func agentModelChargeSettles(n string) string {
	return `(status = 'pending' OR (status = 'failed' AND ` + n + ` = 'paid'))`
}

// SettleAgentModelCharge settles a charge as paid or failed and returns it.
// A charge already settled the same way (a webhook raced the mint), or paid,
// is returned as it is.
func (s *PGStore) SettleAgentModelCharge(ctx context.Context, key AgentModelChargeKey, settlement AgentModelChargeSettlement) (AgentModelCharge, error) {
	if err := validAgentModelChargeSettlement(settlement); err != nil {
		return AgentModelCharge{}, err
	}
	charge, err := scanAgentModelCharge(s.Pool.QueryRow(ctx, `
		UPDATE agent_model_threshold_charges
		SET status = $4, hosted_invoice_url = COALESCE(NULLIF($5, ''), hosted_invoice_url),
		    failure = CASE WHEN $4 = 'paid' THEN NULL ELSE NULLIF($6, '') END, settled_at = $7
		WHERE workspace_id = $1 AND month = $2 AND seq = $3 AND `+agentModelChargeSettles("$4")+`
		RETURNING `+agentModelChargeColumns,
		key.WorkspaceID, MonthStartUTC(key.Month), key.Seq, settlement.Status,
		settlement.HostedInvoiceURL, settlement.Failure, settlement.At.UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		return s.GetAgentModelCharge(ctx, key)
	}
	return charge, err
}

// SettleAgentModelChargeInvoice is SettleAgentModelCharge for the charge
// whose Stripe invoice is invoiceID (a webhook names only the invoice).
// found is false for an invoice no charge recorded.
func (s *PGStore) SettleAgentModelChargeInvoice(ctx context.Context, invoiceID string, settlement AgentModelChargeSettlement) (charge AgentModelCharge, found bool, err error) {
	if invoiceID == "" {
		return AgentModelCharge{}, false, fmt.Errorf("agent-model charge invoice id is empty")
	}
	if err := validAgentModelChargeSettlement(settlement); err != nil {
		return AgentModelCharge{}, false, err
	}
	charge, err = scanAgentModelCharge(s.Pool.QueryRow(ctx, `
		UPDATE agent_model_threshold_charges
		SET status = $2, hosted_invoice_url = COALESCE(NULLIF($3, ''), hosted_invoice_url),
		    failure = CASE WHEN $2 = 'paid' THEN NULL ELSE NULLIF($4, '') END, settled_at = $5
		WHERE invoice_id = $1 AND `+agentModelChargeSettles("$2")+`
		RETURNING `+agentModelChargeColumns,
		invoiceID, settlement.Status, settlement.HostedInvoiceURL, settlement.Failure, settlement.At.UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		charge, err = scanAgentModelCharge(s.Pool.QueryRow(ctx, `SELECT `+agentModelChargeColumns+`
			FROM agent_model_threshold_charges WHERE invoice_id = $1`, invoiceID))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentModelCharge{}, false, nil
	}
	if err != nil {
		return AgentModelCharge{}, false, err
	}
	return charge, true, nil
}

func validAgentModelChargeSettlement(s AgentModelChargeSettlement) error {
	if (s.Status != AgentModelChargePaid && s.Status != AgentModelChargeFailed) || s.At.IsZero() {
		return fmt.Errorf("agent-model charge settles as paid or failed, at a time")
	}
	return nil
}

// SetAgentModelChargeNetItem records the negative invoice item that nets a
// paid charge out of the next cycle invoice.
func (s *PGStore) SetAgentModelChargeNetItem(ctx context.Context, key AgentModelChargeKey, itemID string) error {
	if itemID == "" {
		return fmt.Errorf("agent-model charge net item id is empty")
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_model_threshold_charges SET net_item_id = $4
		WHERE workspace_id = $1 AND month = $2 AND seq = $3 AND status = 'paid'`,
		key.WorkspaceID, MonthStartUTC(key.Month), key.Seq, itemID)
	return err
}

// AgentModelChargesToRetry lists up to limit charges the sweep finishes:
// pending ones created before pendingOlderThan (a mint still making a newer
// one is left alone), and paid ones with no net item yet. Oldest first.
func (s *PGStore) AgentModelChargesToRetry(ctx context.Context, pendingOlderThan time.Time, limit int) ([]AgentModelCharge, error) {
	return collectAgentModelCharges(s.Pool.Query(ctx, `SELECT `+agentModelChargeColumns+`
		FROM agent_model_threshold_charges
		WHERE (status = 'pending' AND created_at < $1) OR (status = 'paid' AND net_item_id IS NULL)
		ORDER BY created_at, workspace_id, seq
		LIMIT $2`, pendingOlderThan.UTC(), limit))
}

// ListAgentModelCharges lists workspaceID's charges created at or after
// since, oldest first.
func (s *PGStore) ListAgentModelCharges(ctx context.Context, workspaceID string, since time.Time) ([]AgentModelCharge, error) {
	return collectAgentModelCharges(s.Pool.Query(ctx, `SELECT `+agentModelChargeColumns+`
		FROM agent_model_threshold_charges
		WHERE workspace_id = $1 AND created_at >= $2
		ORDER BY created_at, seq`, workspaceID, since.UTC()))
}

// RecordAgentModelBilledUntil moves workspaceID's charge window boundary
// (migration 0249) forward to until: a bex subscription invoice billed the
// meter through it. It never moves back, and a workspace with no mapping is
// a no-op. advanced reports that the boundary moved, so a spend read made
// before it is stale. Moving it clears the recorded credits (migration
// 0253): that invoice consumed credits the record still counts.
func (s *PGStore) RecordAgentModelBilledUntil(ctx context.Context, workspaceID string, until time.Time) (advanced bool, err error) {
	if workspaceID == "" || until.IsZero() {
		return false, fmt.Errorf("agent-model billed-until needs a workspace and a time")
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE billing_provider_mappings
		SET agent_model_billed_until = $2, agent_model_credit_microusd = NULL, agent_model_credit_read_at = NULL
		WHERE workspace_id = $1 AND (agent_model_billed_until IS NULL OR agent_model_billed_until < $2)`,
		workspaceID, until.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// RecordAgentModelCredits records the billing credits the threshold charger
// read for workspaceID at at (migration 0253): the charge window's unbilled
// spend is reduced by them while the read is inside the window. A workspace
// with no mapping is a no-op.
func (s *PGStore) RecordAgentModelCredits(ctx context.Context, workspaceID string, microUSD int64, at time.Time) error {
	if workspaceID == "" || at.IsZero() || microUSD < 0 {
		return fmt.Errorf("agent-model credits need a workspace, a time and a non-negative amount")
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE billing_provider_mappings SET agent_model_credit_microusd = $2, agent_model_credit_read_at = $3
		WHERE workspace_id = $1`,
		workspaceID, microUSD, at.UTC())
	return err
}
