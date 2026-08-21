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
	"time"
)

// RoleReconciliation is one claimed Postgres -> OpenFGA exact-role repair.
// Subject is the raw identity id; callers add OpenFGA's user: prefix.
type RoleReconciliation struct {
	TenantID  string
	Subject   string
	Role      string
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ClaimRoleReconciliations leases due rows before returning them. SKIP LOCKED
// partitions work safely across bex-api replicas; exact-role writes remain
// idempotent if a worker dies after applying OpenFGA but before acknowledging.
func (s *PGStore) ClaimRoleReconciliations(ctx context.Context, limit int) ([]RoleReconciliation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
			SELECT tenant_id, subject
			FROM membership_role_reconciliations
			WHERE next_attempt_at <= now()
			ORDER BY next_attempt_at, updated_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE membership_role_reconciliations r
		SET next_attempt_at = now() + interval '1 minute', updated_at = now()
		FROM due
		WHERE r.tenant_id = due.tenant_id AND r.subject = due.subject
		RETURNING r.tenant_id, r.subject, r.role, r.attempts, r.created_at, r.updated_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoleReconciliation
	for rows.Next() {
		var row RoleReconciliation
		if err := rows.Scan(&row.TenantID, &row.Subject, &row.Role, &row.Attempts, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// HasPendingRoleReconciliation reports whether subject still has an
// unconverged role intent for tenantID (round-19 #3). Not filtered on
// next_attempt_at: a row backed off after failures is still pending
// convergence, and that is exactly the window the caller needs to know about.
func (s *PGStore) HasPendingRoleReconciliation(ctx context.Context, tenantID, subject string) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM membership_role_reconciliations WHERE tenant_id = $1 AND subject = $2)`,
		tenantID, subject).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CompleteRoleReconciliation acknowledges only the role that was applied. A
// concurrent newer role upsert remains queued instead of being deleted by a
// stale worker.
func (s *PGStore) CompleteRoleReconciliation(ctx context.Context, tenantID, subject, role string) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM membership_role_reconciliations WHERE tenant_id = $1 AND subject = $2 AND role = $3`,
		tenantID, subject, role)
	return err
}

func (s *PGStore) FailRoleReconciliation(ctx context.Context, tenantID, subject, role, message string) error {
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE membership_role_reconciliations
		SET attempts = attempts + 1,
		    next_attempt_at = now() + LEAST(interval '1 hour', interval '5 seconds' * power(2, LEAST(attempts, 10))),
		    last_error = $4,
		    updated_at = now()
		WHERE tenant_id = $1 AND subject = $2 AND role = $3`, tenantID, subject, role, message)
	return err
}
