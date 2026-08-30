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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	AccountDeletionPending  = "pending"
	AccountDeletionCleaning = "cleaning"
	AccountDeletionIdentity = "identity"
	AccountDeletionDone     = "done"

	AccountWorkspaceDelete  = "delete"
	AccountWorkspaceLeave   = "leave"
	AccountWorkspaceBlocked = "blocked"
)

type AccountWorkspaceDisposition struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

type AccountDeletion struct {
	Subject       string
	DeletedMarker string
	State         string
	Workspaces    []AccountWorkspaceDisposition
	Attempts      int
	ClaimedUntil  *time.Time
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AccountDeletionBlockedError struct {
	Workspaces []AccountWorkspaceDisposition
}

func (e *AccountDeletionBlockedError) Error() string {
	return "account deletion would orphan one or more workspaces"
}

func (s *PGStore) AccountDeletionTombstoned(ctx context.Context, subject string) (bool, error) {
	var pending bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM account_deletions WHERE subject = $1)`, subject,
	).Scan(&pending)
	return pending, err
}

func accountDispositionWithoutMachines(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, subject string, machineSubjects []string) ([]AccountWorkspaceDisposition, error) {
	rows, err := q.Query(ctx, `
		SELECT t.id, t.name,
		       (SELECT count(*) FROM tenant_members all_m
		        WHERE all_m.tenant_id = t.id AND NOT (all_m.subject = ANY($2::text[]))),
		       (SELECT count(*) FROM tenant_members admins
		        WHERE admins.tenant_id = t.id AND admins.role = 'admin' AND admins.subject != $1
		          AND NOT (admins.subject = ANY($2::text[])))
		FROM tenants t
		JOIN tenant_members mine ON mine.tenant_id = t.id
		WHERE mine.subject = $1
		ORDER BY t.id`, subject, machineSubjects)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountWorkspaceDisposition
	for rows.Next() {
		var d AccountWorkspaceDisposition
		var members, otherAdmins int
		if err := rows.Scan(&d.ID, &d.Name, &members, &otherAdmins); err != nil {
			return nil, err
		}
		switch {
		case members == 1:
			d.Action = AccountWorkspaceDelete
		case otherAdmins > 0:
			d.Action = AccountWorkspaceLeave
		default:
			d.Action = AccountWorkspaceBlocked
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGStore) PreviewAccountDeletion(ctx context.Context, subject string, machineSubjects []string) ([]AccountWorkspaceDisposition, error) {
	return accountDispositionWithoutMachines(ctx, s.Pool, subject, machineSubjects)
}

// BeginAccountDeletion serializes against membership mutations with the same
// per-workspace advisory locks they use, rechecks disposition, and writes the
// tombstone plus immutable workspace plan in one transaction.
func (s *PGStore) BeginAccountDeletion(ctx context.Context, subject, email string, machineSubjects []string) (AccountDeletion, error) {
	existing, err := scanAccountDeletion(s.Pool.QueryRow(ctx, `
		SELECT subject, deleted_marker, state, workspace_plan, attempts, claimed_until,
		       next_attempt_at, last_error, created_at, updated_at
		FROM account_deletions WHERE subject = $1`, subject))
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AccountDeletion{}, err
	}

	var deletion AccountDeletion
	ownID, err := s.OwnerIDForSubject(ctx, subject)
	if err != nil {
		return AccountDeletion{}, err
	}
	marker := "deleted:" + ownID
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		// The optimistic read above avoids a transaction for ordinary internal
		// retries. Recheck under the subject lock so concurrent first requests
		// converge on the winner without re-running preflight or email cleanup.
		existing, err := scanAccountDeletion(tx.QueryRow(ctx, `
			SELECT subject, deleted_marker, state, workspace_plan, attempts, claimed_until,
			       next_attempt_at, last_error, created_at, updated_at
			FROM account_deletions WHERE subject = $1`, subject))
		if err == nil {
			deletion = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		rows, err := tx.Query(ctx,
			`SELECT tenant_id FROM tenant_members WHERE subject = $1 ORDER BY tenant_id`, subject)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(ids) > 0 {
			if _, err := tx.Exec(ctx, `
				SELECT pg_advisory_xact_lock(hashtext(id))
				FROM (SELECT unnest($1::text[]) AS id ORDER BY id) ordered`, ids); err != nil {
				return err
			}
		}
		// Membership mutations take the same workspace advisory locks before
		// changing rows. Lock the caller's rows only after those locks (the shared
		// order avoids deadlocks), then ensure no workspace joined the set between
		// discovery and locking.
		lockedRows, err := tx.Query(ctx,
			`SELECT tenant_id FROM tenant_members WHERE subject = $1 ORDER BY tenant_id FOR UPDATE`, subject)
		if err != nil {
			return err
		}
		var lockedIDs []string
		for lockedRows.Next() {
			var id string
			if err := lockedRows.Scan(&id); err != nil {
				lockedRows.Close()
				return err
			}
			lockedIDs = append(lockedIDs, id)
		}
		lockedRows.Close()
		if err := lockedRows.Err(); err != nil {
			return err
		}
		if !slices.Equal(ids, lockedIDs) {
			return fmt.Errorf("account memberships changed during deletion preflight: %w", ErrConflict)
		}
		plan, err := accountDispositionWithoutMachines(ctx, tx, subject, machineSubjects)
		if err != nil {
			return err
		}
		var blocked []AccountWorkspaceDisposition
		for _, d := range plan {
			if d.Action == AccountWorkspaceBlocked {
				blocked = append(blocked, d)
			}
		}
		if len(blocked) > 0 {
			return &AccountDeletionBlockedError{Workspaces: blocked}
		}
		encoded, err := json.Marshal(plan)
		if err != nil {
			return err
		}
		deletion, err = scanAccountDeletion(tx.QueryRow(ctx, `
			INSERT INTO account_deletions (subject, deleted_marker, workspace_plan)
			VALUES ($1, $2, $3)
			ON CONFLICT (subject) DO NOTHING
			RETURNING subject, deleted_marker, state, workspace_plan, attempts, claimed_until,
			          next_attempt_at, last_error, created_at, updated_at`, subject, marker, encoded))
		if errors.Is(err, pgx.ErrNoRows) {
			deletion, err = scanAccountDeletion(tx.QueryRow(ctx, `
				SELECT subject, deleted_marker, state, workspace_plan, attempts, claimed_until,
				       next_attempt_at, last_error, created_at, updated_at
				FROM account_deletions WHERE subject = $1`, subject))
			return err
		}
		if err != nil {
			return err
		}
		return cleanupAccountEmail(ctx, tx, email, deletion.DeletedMarker)
	})
	if err != nil {
		return AccountDeletion{}, err
	}
	return deletion, nil
}

func scanAccountDeletion(row pgx.Row) (AccountDeletion, error) {
	var d AccountDeletion
	var encoded []byte
	err := row.Scan(&d.Subject, &d.DeletedMarker, &d.State, &encoded, &d.Attempts, &d.ClaimedUntil,
		&d.NextAttemptAt, &d.LastError, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return AccountDeletion{}, err
	}
	if err := json.Unmarshal(encoded, &d.Workspaces); err != nil {
		return AccountDeletion{}, err
	}
	return d, nil
}

func (s *PGStore) ClaimAccountDeletions(ctx context.Context, limit int) ([]AccountDeletion, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []AccountDeletion
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE account_deletions d SET
				claimed_until = now() + interval '2 minutes', attempts = attempts + 1,
				updated_at = now()
			WHERE subject IN (
				SELECT subject FROM account_deletions
				WHERE state != 'done' AND next_attempt_at <= now()
				  AND (claimed_until IS NULL OR claimed_until < now())
				ORDER BY next_attempt_at, created_at
				FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING subject, deleted_marker, state, workspace_plan, attempts, claimed_until,
			          next_attempt_at, last_error, created_at, updated_at`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanAccountDeletion(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func (s *PGStore) AdvanceAccountDeletion(ctx context.Context, subject, from, to string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE account_deletions SET state = $3,
			claimed_until = CASE WHEN $3 = 'done' THEN NULL ELSE now() + interval '2 minutes' END,
			next_attempt_at = now(), last_error = '', updated_at = now(),
			completed_at = CASE WHEN $3 = 'done' THEN now() ELSE completed_at END
		WHERE subject = $1 AND state = $2`, subject, from, to)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("account deletion state changed: %w", ErrConflict)
	}
	return nil
}

func (s *PGStore) FailAccountDeletion(ctx context.Context, subject, message string) error {
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE account_deletions SET claimed_until = NULL,
			next_attempt_at = now() + LEAST(interval '15 minutes', interval '5 seconds' * power(2, LEAST(attempts, 8))),
			last_error = $2, updated_at = now()
		WHERE subject = $1`, subject, message)
	return err
}

// RemoveAccountMember rechecks that another admin exists and removes the
// deleting subject atomically. It is intentionally narrower than general
// member removal and clears only the onboarding owner binding.
func (s *PGStore) RemoveAccountMember(ctx context.Context, tenantID, subject string) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID); err != nil {
			return err
		}
		var otherAdmins int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'admin' AND subject != $2`, tenantID, subject).Scan(&otherAdmins); err != nil {
			return err
		}
		if otherAdmins == 0 {
			return ErrLastAdmin
		}
		if _, err := tx.Exec(ctx, `DELETE FROM tenant_members WHERE tenant_id = $1 AND subject = $2`, tenantID, subject); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE tenants SET owner_identity_id = NULL, updated_at = now() WHERE id = $1 AND owner_identity_id = $2`, tenantID, subject)
		return err
	})
}

// CleanupAccountSubject removes active personal state and anonymizes retained
// workspace/audit provenance. It runs only after workspace disposition.
func (s *PGStore) CleanupAccountSubject(ctx context.Context, subject, marker string) error {
	if marker == "" {
		return fmt.Errorf("account deletion marker missing: %w", ErrInvalid)
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		deletes := []string{
			`DELETE FROM notification_settings WHERE subject = $1`,
			`DELETE FROM ssh_keys WHERE subject = $1`,
			`DELETE FROM github_connect_transactions WHERE subject = $1`,
		}
		for _, statement := range deletes {
			if _, err := tx.Exec(ctx, statement, subject); err != nil {
				return err
			}
		}
		updates := []string{
			`UPDATE ssh_sessions SET subject = $2 WHERE subject = $1`,
			`UPDATE oauth_revocations SET subject = $2 WHERE subject = $1`,
			`UPDATE tenant_invites SET invited_by = $2 WHERE invited_by = $1`,
			`UPDATE registry_credentials SET created_by = $2 WHERE created_by = $1`,
			`UPDATE webhook_endpoints SET created_by = $2 WHERE created_by = $1`,
			`UPDATE audit_events SET caller = $2 WHERE caller = $1`,
			`UPDATE owner_ids SET subject = $2 WHERE subject = $1`,
		}
		for _, statement := range updates {
			if _, err := tx.Exec(ctx, statement, subject, marker); err != nil {
				return err
			}
		}
		return nil
	})
}

// cleanupAccountEmail runs in the same transaction that first records intent,
// after the tombstone insert and before commit. This removes email-bearing
// state without ever persisting the address in retryable worker state.
func cleanupAccountEmail(ctx context.Context, tx pgx.Tx, email, marker string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM tenant_invites WHERE lower(email) = lower($1) AND accepted_at IS NULL`, email); err != nil {
		return err
	}
	anonymousEmail := strings.ReplaceAll(marker, ":", "-") + "@invalid"
	if _, err := tx.Exec(ctx,
		`UPDATE tenant_invites SET email = $2 WHERE lower(email) = lower($1)`, email, anonymousEmail); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`UPDATE audit_events SET target_name = $2 WHERE lower(target_name) = lower($1)`, email, marker)
	return err
}
