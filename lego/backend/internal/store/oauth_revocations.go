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

// BumpOAuthRevocation advances the durable invalidation marker shared by every
// bex-api replica. clock_timestamp() ensures two bumps in one transaction are
// ordered by wall time rather than the transaction start time.
func (s *PGStore) BumpOAuthRevocation(ctx context.Context, subject, clientID string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO oauth_revocations (subject, client_id, revoked_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (subject, client_id) DO UPDATE
		SET revoked_at = GREATEST(oauth_revocations.revoked_at + interval '1 microsecond', clock_timestamp())`,
		subject, clientID)
	return err
}

// OAuthRevokedAt returns the latest shared invalidation marker for one OAuth
// consent chain. Missing rows are the normal never-revoked state.
func (s *PGStore) OAuthRevokedAt(ctx context.Context, subject, clientID string) (time.Time, bool, error) {
	var at time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT revoked_at FROM oauth_revocations WHERE subject = $1 AND client_id = $2`,
		subject, clientID,
	).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	return at, err == nil, err
}
