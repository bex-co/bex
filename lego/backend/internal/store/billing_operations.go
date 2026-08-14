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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

const billingRepairHorizon = 34 * 24 * time.Hour

type BillingExportRow struct {
	WorkspaceID   string     `json:"workspaceId"`
	ResourceKind  string     `json:"resourceKind"`
	ServiceID     string     `json:"serviceId"`
	Kind          string     `json:"kind"`
	Tier          string     `json:"tier,omitempty"`
	WindowStart   time.Time  `json:"windowStart"`
	Quantity      int64      `json:"quantity"`
	State         string     `json:"state"`
	AttemptedAt   *time.Time `json:"attemptedAt,omitempty"`
	EmittedAt     *time.Time `json:"emittedAt,omitempty"`
	TransactionID string     `json:"transactionId,omitempty"`
	EventName     string     `json:"eventName,omitempty"`
	ErrorCode     string     `json:"errorCode,omitempty"`
}

type BillingExportReport struct {
	WorkspaceID    string             `json:"workspaceId"`
	CustomerID     string             `json:"customerId,omitempty"`
	SubscriptionID string             `json:"subscriptionId,omitempty"`
	Livemode       *bool              `json:"livemode,omitempty"`
	Rows           []BillingExportRow `json:"rows"`
}

type BillingExportIssue struct {
	TransactionID string     `json:"transactionId"`
	WorkspaceID   string     `json:"workspaceId"`
	ResourceKind  string     `json:"resourceKind"`
	ServiceID     string     `json:"serviceId"`
	Kind          string     `json:"kind"`
	Tier          string     `json:"tier,omitempty"`
	WindowStart   time.Time  `json:"windowStart"`
	EventName     string     `json:"eventName"`
	IssueKind     string     `json:"issueKind"`
	ErrorCode     string     `json:"errorCode"`
	ErrorMessage  string     `json:"errorMessage"`
	FirstSeenAt   time.Time  `json:"firstSeenAt"`
	LastSeenAt    time.Time  `json:"lastSeenAt"`
	ResolvedAt    *time.Time `json:"resolvedAt,omitempty"`
	Resolution    string     `json:"resolution,omitempty"`
	Actor         string     `json:"actor,omitempty"`
	Reason        string     `json:"reason,omitempty"`
}

const billingExportIssueColumns = `transaction_id, workspace_id, resource_kind, service_id, kind, tier,
	window_start, event_name, issue_kind, error_code, error_message, first_seen_at, last_seen_at,
	resolved_at, resolution, actor, reason`

func scanBillingExportIssue(row rowScanner) (BillingExportIssue, error) {
	var issue BillingExportIssue
	err := row.Scan(&issue.TransactionID, &issue.WorkspaceID, &issue.ResourceKind, &issue.ServiceID,
		&issue.Kind, &issue.Tier, &issue.WindowStart, &issue.EventName, &issue.IssueKind,
		&issue.ErrorCode, &issue.ErrorMessage, &issue.FirstSeenAt, &issue.LastSeenAt,
		&issue.ResolvedAt, &issue.Resolution, &issue.Actor, &issue.Reason)
	return issue, err
}

func (s *PGStore) BillingExportReport(ctx context.Context, workspaceID string, start, end time.Time) (BillingExportReport, error) {
	report := BillingExportReport{WorkspaceID: workspaceID, Rows: []BillingExportRow{}}
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(m.customer_id,''), COALESCE(m.subscription_id,''), m.livemode
		FROM tenants t LEFT JOIN billing_provider_mappings m ON m.workspace_id=t.id
		WHERE t.id=$1`, workspaceID).Scan(&report.CustomerID, &report.SubscriptionID, &report.Livemode); err != nil {
		return BillingExportReport{}, classify("billing export workspace", err)
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id, resource_kind, service_id, kind, tier, window_start, quantity,
		       billing_export_state, billing_export_attempted_at, emitted_at,
		       COALESCE(billing_export_transaction_id,''), COALESCE(billing_export_event_name,''),
		       billing_export_error_code
		FROM usage_hourly
		WHERE workspace_id=$1 AND window_start >= $2 AND window_start < $3
		ORDER BY window_start, resource_kind, service_id, kind, tier`, workspaceID, start.UTC(), end.UTC())
	if err != nil {
		return BillingExportReport{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var r BillingExportRow
		if err := rows.Scan(&r.WorkspaceID, &r.ResourceKind, &r.ServiceID, &r.Kind, &r.Tier,
			&r.WindowStart, &r.Quantity, &r.State, &r.AttemptedAt, &r.EmittedAt,
			&r.TransactionID, &r.EventName, &r.ErrorCode); err != nil {
			return BillingExportReport{}, err
		}
		report.Rows = append(report.Rows, r)
	}
	return report, rows.Err()
}

func (s *PGStore) ListBillingExportIssues(ctx context.Context, openOnly bool, limit int) ([]BillingExportIssue, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT ` + billingExportIssueColumns + ` FROM billing_export_issues`
	if openOnly {
		query += ` WHERE resolved_at IS NULL`
	}
	query += ` ORDER BY first_seen_at, transaction_id LIMIT $1`
	rows, err := s.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BillingExportIssue{}
	for rows.Next() {
		issue, err := scanBillingExportIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, issue)
	}
	return out, rows.Err()
}

// ResolveBillingExportIssue performs one explicit, audited operator decision:
// acknowledge keeps the row held, retry requeues only a definite permanent
// reject still inside Stripe's event-time horizon, and mark_repaired stamps the
// row after external reconciliation proves the provider outcome.
func (s *PGStore) ResolveBillingExportIssue(ctx context.Context, transactionID, action, actor, reason string, at time.Time) (BillingExportIssue, error) {
	if actor == "" || reason == "" {
		return BillingExportIssue{}, fmt.Errorf("%w: actor and reason are required", ErrInvalid)
	}
	if action != "acknowledge" && action != "retry" && action != "mark_repaired" {
		return BillingExportIssue{}, fmt.Errorf("%w: action must be acknowledge, retry, or mark_repaired", ErrInvalid)
	}
	var issue BillingExportIssue
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		issue, err = scanBillingExportIssue(tx.QueryRow(ctx, `SELECT `+billingExportIssueColumns+`
		FROM billing_export_issues WHERE transaction_id=$1 FOR UPDATE`, transactionID))
		if err != nil {
			return classify("billing export issue", err)
		}
		if issue.ResolvedAt != nil {
			return fmt.Errorf("%w: billing export issue already resolved", ErrConflict)
		}
		if action == "retry" {
			if issue.IssueKind != "permanent_reject" {
				return fmt.Errorf("%w: ambiguous provider outcome cannot be retried; reconcile then mark_repaired or acknowledge", ErrConflict)
			}
			if issue.WindowStart.Before(at.Add(-billingRepairHorizon)) {
				return fmt.Errorf("%w: event timestamp is outside the safe Stripe backfill horizon", ErrConflict)
			}
			if _, err = tx.Exec(ctx, `UPDATE usage_hourly
			SET billing_export_state='pending', billing_export_attempted_at=NULL,
			    billing_export_error_code='', billing_export_error=''
			WHERE resource_kind=$1 AND service_id=$2 AND kind=$3 AND tier=$4 AND window_start=$5
			  AND billing_export_transaction_id=$6 AND billing_export_state='rejected'`,
				issue.ResourceKind, issue.ServiceID, issue.Kind, issue.Tier, issue.WindowStart, issue.TransactionID); err != nil {
				return err
			}
		} else if action == "mark_repaired" {
			if _, err = tx.Exec(ctx, `UPDATE usage_hourly
			SET billing_export_state='emitted', emitted_at=$1,
			    billing_export_error_code='', billing_export_error=''
			WHERE resource_kind=$2 AND service_id=$3 AND kind=$4 AND tier=$5 AND window_start=$6
			  AND billing_export_transaction_id=$7 AND billing_export_state IN ('rejected','ambiguous')`,
				at.UTC(), issue.ResourceKind, issue.ServiceID, issue.Kind, issue.Tier, issue.WindowStart, issue.TransactionID); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE billing_export_issues
		SET resolved_at=$2, resolution=$3, actor=$4, reason=$5
		WHERE transaction_id=$1`, transactionID, at.UTC(), action, actor, boundedBillingDiagnostic(reason, 240)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id, workspace_id, caller, caller_method, verb, resource, target, outcome, at, billing_override_reason)
		VALUES ($1,$2,$3,'internal','billing.ResolveExportIssue',$4,$5,'allowed',$6,$7)`,
			ids.New(ids.Audit), issue.WorkspaceID, actor, core.WorkspaceObject(issue.WorkspaceID),
			"billing_export:"+transactionID, at.UTC(), boundedBillingDiagnostic(reason, 240))
		return err
	})
	if err != nil {
		return BillingExportIssue{}, err
	}
	issue.Resolution = action
	issue.Actor = actor
	issue.Reason = boundedBillingDiagnostic(reason, 240)
	resolved := at.UTC()
	issue.ResolvedAt = &resolved
	return issue, nil
}
