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

// two_factor_reminders.go keeps the day of each workspace's last batch of 2FA
// reminders (w1/m647, migration 0347): Render sends at most one batch per
// workspace per day.

// ClaimTwoFactorReminderDay claims day (its UTC date) for workspaceID's batch
// of 2FA reminders: true when no batch has claimed that day or a later one,
// false when one already has. The claim is one conditional upsert, so two
// admins sending at once get one true between them.
func (s *PGStore) ClaimTwoFactorReminderDay(ctx context.Context, workspaceID string, day time.Time) (bool, error) {
	var claimed string
	err := s.Pool.QueryRow(ctx, `INSERT INTO workspace_two_factor_reminders (workspace_id, sent_on)
		VALUES ($1, $2::date)
		ON CONFLICT (workspace_id) DO UPDATE SET sent_on = EXCLUDED.sent_on
		WHERE workspace_two_factor_reminders.sent_on < EXCLUDED.sent_on
		RETURNING workspace_id`, workspaceID, utcDate(day)).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, classify("workspace", err)
	}
	return true, nil
}

// ReleaseTwoFactorReminderDay gives back day's claim when its batch sent
// nothing, so the admin can try again the same day. A later day's claim is
// left alone.
func (s *PGStore) ReleaseTwoFactorReminderDay(ctx context.Context, workspaceID string, day time.Time) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM workspace_two_factor_reminders
		WHERE workspace_id = $1 AND sent_on = $2::date`, workspaceID, utcDate(day))
	return err
}

// utcDate is day's UTC calendar date, as Postgres reads a date literal.
func utcDate(day time.Time) string { return day.UTC().Format(time.DateOnly) }
