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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

const (
	WorkspaceCreationPrepared       = "prepared"
	WorkspaceCreationSetupPending   = "setup_pending"
	WorkspaceCreationSetupSucceeded = "setup_succeeded"
	WorkspaceCreationFinalized      = "finalized"
	WorkspaceCreationCleanupPending = "cleanup_pending"
	WorkspaceCreationExpired        = "expired"
)

// WorkspaceCreationAttempt is the non-sensitive, durable state of one
// pre-tenant create flow. Provider ids never cross the public workspace API;
// the service returns only the client secret needed by the owning browser.
type WorkspaceCreationAttempt struct {
	ID                      string
	WorkspaceID             string
	OwnerSubject            string
	Name                    string
	Plan                    string
	BillingEmail            string
	PaymentRequired         bool
	State                   string
	ProviderCustomerID      string
	ProviderSetupIntentID   string
	ProviderPaymentMethodID string
	ProviderSubscriptionID  string
	ProviderLivemode        *bool
	ExpiresAt               time.Time
	CleanupClaimedUntil     *time.Time
	FinalizedAt             *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

const workspaceCreationColumns = `id, workspace_id, owner_subject, name, plan,
	billing_email, payment_required, state, provider_customer_id,
	provider_setup_intent_id, provider_payment_method_id,
	provider_subscription_id, provider_livemode, expires_at, cleanup_claimed_until, finalized_at,
	created_at, updated_at`

const workspaceCreationColumnsQualified = `a.id, a.workspace_id, a.owner_subject, a.name, a.plan,
	a.billing_email, a.payment_required, a.state, a.provider_customer_id,
	a.provider_setup_intent_id, a.provider_payment_method_id,
	a.provider_subscription_id, a.provider_livemode, a.expires_at, a.cleanup_claimed_until, a.finalized_at,
	a.created_at, a.updated_at`

func scanWorkspaceCreationAttempt(row rowScanner) (WorkspaceCreationAttempt, error) {
	var a WorkspaceCreationAttempt
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.OwnerSubject, &a.Name, &a.Plan,
		&a.BillingEmail, &a.PaymentRequired, &a.State, &a.ProviderCustomerID,
		&a.ProviderSetupIntentID, &a.ProviderPaymentMethodID,
		&a.ProviderSubscriptionID, &a.ProviderLivemode, &a.ExpiresAt, &a.CleanupClaimedUntil,
		&a.FinalizedAt, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// CreateWorkspaceCreationAttempt reserves ids but creates no tenant. The
// attempt is deliberately subject-bound and short-lived.
func (s *PGStore) CreateWorkspaceCreationAttempt(ctx context.Context, subject, name, plan, billingEmail string, paymentRequired bool, expiresAt time.Time) (WorkspaceCreationAttempt, error) {
	a := WorkspaceCreationAttempt{
		ID:              ids.New(ids.WorkspaceCreationAttempt),
		WorkspaceID:     ids.New(ids.Workspace),
		OwnerSubject:    subject,
		Name:            name,
		Plan:            plan,
		BillingEmail:    billingEmail,
		PaymentRequired: paymentRequired,
		State:           WorkspaceCreationPrepared,
		ExpiresAt:       expiresAt.UTC(),
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO workspace_creation_attempts
			(id, workspace_id, owner_subject, name, plan, billing_email, payment_required, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at, updated_at`, a.ID, a.WorkspaceID, subject, name, plan,
		billingEmail, paymentRequired, a.ExpiresAt).Scan(&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return WorkspaceCreationAttempt{}, classify("workspace creation attempt", err)
	}
	return a, nil
}

// GetWorkspaceCreationAttempt returns not-found for an unknown OR foreign id,
// keeping attempt existence and provider state undiscoverable across subjects.
func (s *PGStore) GetWorkspaceCreationAttempt(ctx context.Context, id, subject string) (WorkspaceCreationAttempt, error) {
	a, err := scanWorkspaceCreationAttempt(s.Pool.QueryRow(ctx, `
		SELECT `+workspaceCreationColumns+`
		FROM workspace_creation_attempts WHERE id=$1 AND owner_subject=$2`, id, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceCreationAttempt{}, ErrNotFound
	}
	return a, err
}

// SetWorkspaceCreationSetup records only safe provider correlations. A retry
// may repeat the same values; changing an already-recorded object is refused.
func (s *PGStore) SetWorkspaceCreationSetup(ctx context.Context, id, subject, customerID, setupIntentID string, livemode bool) (WorkspaceCreationAttempt, error) {
	a, err := scanWorkspaceCreationAttempt(s.Pool.QueryRow(ctx, `
		UPDATE workspace_creation_attempts SET
			provider_customer_id=$3, provider_setup_intent_id=$4,
			provider_livemode=$5, state='setup_pending', updated_at=now()
		WHERE id=$1 AND owner_subject=$2
		  AND state IN ('prepared','setup_pending')
		  AND (provider_customer_id='' OR provider_customer_id=$3)
		  AND (provider_setup_intent_id='' OR provider_setup_intent_id=$4)
		RETURNING `+workspaceCreationColumns, id, subject, customerID, setupIntentID, livemode))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceCreationAttempt{}, ErrConflict
	}
	return a, err
}

func (s *PGStore) MarkWorkspaceCreationSetupSucceeded(ctx context.Context, id, subject, paymentMethodID string) (WorkspaceCreationAttempt, error) {
	a, err := scanWorkspaceCreationAttempt(s.Pool.QueryRow(ctx, `
		UPDATE workspace_creation_attempts SET
			provider_payment_method_id=$3, state='setup_succeeded', updated_at=now()
		WHERE id=$1 AND owner_subject=$2 AND state IN ('setup_pending','setup_succeeded')
		  AND expires_at > now()
		  AND (provider_payment_method_id='' OR provider_payment_method_id=$3)
		RETURNING `+workspaceCreationColumns, id, subject, paymentMethodID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceCreationAttempt{}, ErrConflict
	}
	return a, err
}

func (s *PGStore) SetWorkspaceCreationSubscription(ctx context.Context, id, subject, subscriptionID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE workspace_creation_attempts SET provider_subscription_id=$3, updated_at=now()
		WHERE id=$1 AND owner_subject=$2 AND state='setup_succeeded'
		  AND (provider_subscription_id='' OR provider_subscription_id=$3)`, id, subject, subscriptionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// FinalizeWorkspaceCreation serializes finalizers on the attempt and inserts
// the tenant, owner membership, provider mapping, payment marker, and healthy
// lifecycle in one transaction. Replays return the already-created tenant.
func (s *PGStore) FinalizeWorkspaceCreation(ctx context.Context, id, subject string, boundAt time.Time) (Tenant, error) {
	var t Tenant
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		a, err := scanWorkspaceCreationAttempt(tx.QueryRow(ctx, `
			SELECT `+workspaceCreationColumns+` FROM workspace_creation_attempts
			WHERE id=$1 AND owner_subject=$2 FOR UPDATE`, id, subject))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if a.State == WorkspaceCreationFinalized {
			t, err = scanTenant(tx.QueryRow(ctx,
				`SELECT id,name,plan,created_at FROM tenants WHERE id=$1`, a.WorkspaceID))
			return err
		}
		if time.Now().After(a.ExpiresAt) {
			_, _ = tx.Exec(ctx, `UPDATE workspace_creation_attempts SET state='expired',updated_at=now() WHERE id=$1`, id)
			return ErrConflict
		}
		if a.PaymentRequired && a.State != WorkspaceCreationSetupSucceeded {
			return ErrConflict
		}
		if a.State != WorkspaceCreationPrepared && a.State != WorkspaceCreationSetupSucceeded {
			return ErrConflict
		}
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		t = Tenant{ID: a.WorkspaceID, Name: a.Name, Plan: a.Plan}
		if err := tx.QueryRow(ctx, `
			INSERT INTO tenants (id,name,plan,billing_email) VALUES ($1,$2,$3,$4)
			RETURNING created_at`, t.ID, t.Name, t.Plan, a.BillingEmail).Scan(&t.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_members (tenant_id,subject,role) VALUES ($1,$2,'admin')`, t.ID, subject); err != nil {
			return err
		}
		if a.ProviderCustomerID != "" {
			if a.ProviderLivemode == nil {
				return fmt.Errorf("workspace creation provider mode missing")
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO billing_provider_mappings
					(workspace_id,customer_id,subscription_id,livemode,payment_method_bound_at)
				VALUES ($1,$2,NULLIF($3,''),$4,$5)`, t.ID, a.ProviderCustomerID,
				a.ProviderSubscriptionID, *a.ProviderLivemode, boundAt.UTC()); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO billing_lifecycles (workspace_id,status) VALUES ($1,'healthy')`, t.ID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE workspace_creation_attempts SET state='finalized', finalized_at=now(), updated_at=now() WHERE id=$1`, id)
		return err
	})
	if err != nil {
		return Tenant{}, classify("workspace creation", err)
	}
	return t, nil
}

func (s *PGStore) CancelWorkspaceCreationAttempt(ctx context.Context, id, subject string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE workspace_creation_attempts
		SET state='cleanup_pending', expires_at=now(), cleanup_claimed_until=NULL, updated_at=now()
		WHERE id=$1 AND owner_subject=$2 AND state NOT IN ('finalized','cleanup_pending','expired')`, id, subject)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ExpireWorkspaceCreationAttempts claims a bounded cleanup batch and returns
// provider correlations to the trusted cleanup loop. Finalized work is never
// selected and can therefore never have its Customer removed.
func (s *PGStore) ExpireWorkspaceCreationAttempts(ctx context.Context, limit int) ([]WorkspaceCreationAttempt, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	// Terminal attempts are useful for idempotent browser retries, but not
	// forever. Remove only a bounded old batch on each cleanup pass.
	if _, err := s.Pool.Exec(ctx, `
		DELETE FROM workspace_creation_attempts WHERE id IN (
			SELECT id FROM workspace_creation_attempts
			WHERE state IN ('finalized','expired') AND updated_at < now()-interval '30 days'
			ORDER BY updated_at,id LIMIT 100
		)`); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM workspace_creation_attempts
			WHERE expires_at <= now() AND state NOT IN ('finalized','expired')
			  AND (cleanup_claimed_until IS NULL OR cleanup_claimed_until <= now())
			ORDER BY expires_at,id FOR UPDATE SKIP LOCKED LIMIT $1
		)
		UPDATE workspace_creation_attempts a SET state='cleanup_pending', cleanup_claimed_until=now()+interval '15 minutes', updated_at=now()
		FROM due WHERE a.id=due.id RETURNING `+workspaceCreationColumnsQualified, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceCreationAttempt
	for rows.Next() {
		a, err := scanWorkspaceCreationAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// FinishWorkspaceCreationCleanup acknowledges provider cleanup. A failure only
// leaves a retry delay, so a persistent provider error cannot monopolize each
// ordered batch; success is terminal.
func (s *PGStore) FinishWorkspaceCreationCleanup(ctx context.Context, id string, success bool) error {
	state := WorkspaceCreationCleanupPending
	if success {
		state = WorkspaceCreationExpired
	}
	_, err := s.Pool.Exec(ctx, `UPDATE workspace_creation_attempts
		SET state=$2,
		    cleanup_claimed_until=CASE WHEN $3 THEN NULL ELSE now()+interval '15 minutes' END,
		    updated_at=now()
		WHERE id=$1 AND state='cleanup_pending'`, id, state, success)
	return err
}
