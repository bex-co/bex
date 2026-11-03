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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Failure alert emails (w2/m252). Two sources feed one queue: the composed
// event feed and failed Blueprint syncs. Each has its own watermark, advanced
// in the same transaction as the events it produced.
const (
	AlertEmailFeedWatermark      = "feed"
	AlertEmailBlueprintWatermark = "blueprint_syncs"

	AlertEmailServerFailed        = "server_failed"
	AlertEmailCronFailed          = "cron_failed"
	AlertEmailImagePullFailed     = "image_pull_failed"
	AlertEmailBlueprintSyncFailed = "blueprint_sync_failed"
	AlertEmailServiceSuspended    = "service_suspended"
	AlertEmailDiskUsageHigh       = "disk_usage_high"
	AlertEmailDiskUsageRecovered  = "disk_usage_recovered"

	// AlertEmailOccurrenceCap bounds the times a stream keeps (and a digest
	// lists); pending_count stays exact beyond it.
	AlertEmailOccurrenceCap = 20
)

// AlertEmailRecipient is one workspace member a failure alert may reach.
// DeployFailed is the member's failure-email preference, which also governs
// these alerts under a service's `default` notification policy.
type AlertEmailRecipient struct {
	TenantID     string
	Subject      string
	DeployFailed bool
}

// AlertEmailItem is one failure for one recipient.
type AlertEmailItem struct {
	TenantID     string
	Subject      string
	SourceKey    string
	ResourceID   string
	Event        string
	ResourceName string
	Detail       string
	OccurredAt   time.Time
}

// AlertEmailStream is a recipient's claimed alert state for one resource and
// event: the failures pending since the last mail.
type AlertEmailStream struct {
	TenantID     string
	Subject      string
	ResourceID   string
	Event        string
	ResourceName string
	Detail       string
	Occurrences  []time.Time
	PendingCount int
	LastMailedAt *time.Time
	Attempts     int
	ClaimedUntil time.Time
}

// FailedBlueprintSync is one Blueprint sync that ended in error.
type FailedBlueprintSync struct {
	SyncID        string
	CompletedAt   time.Time
	BlueprintID   string
	BlueprintName string
	TenantID      string
	ErrorMessage  string
}

func validAlertEmailEvent(event string) bool {
	switch event {
	case AlertEmailServerFailed, AlertEmailCronFailed, AlertEmailImagePullFailed, AlertEmailBlueprintSyncFailed,
		AlertEmailServiceSuspended, AlertEmailDiskUsageHigh, AlertEmailDiskUsageRecovered:
		return true
	}
	return false
}

// ListAlertEmailRecipients returns every user member of every workspace with
// their failure-email preference (a missing settings row means failures on).
func (s *PGStore) ListAlertEmailRecipients(ctx context.Context) ([]AlertEmailRecipient, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT m.tenant_id, m.subject, COALESCE(n.deploy_failed, true)
		FROM tenant_members m
		LEFT JOIN notification_settings n ON n.tenant_id = m.tenant_id AND n.subject = m.subject
		WHERE m.kind = 'user'
		ORDER BY m.tenant_id, m.subject`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertEmailRecipient
	for rows.Next() {
		var r AlertEmailRecipient
		if err := rows.Scan(&r.TenantID, &r.Subject, &r.DeployFailed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListAlertEmailRecipientsFor is ListAlertEmailRecipients for one workspace.
func (s *PGStore) ListAlertEmailRecipientsFor(ctx context.Context, tenantID string) ([]AlertEmailRecipient, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT m.tenant_id, m.subject, COALESCE(n.deploy_failed, true)
		FROM tenant_members m
		LEFT JOIN notification_settings n ON n.tenant_id = m.tenant_id AND n.subject = m.subject
		WHERE m.kind = 'user' AND m.tenant_id = $1
		ORDER BY m.subject`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertEmailRecipient
	for rows.Next() {
		var r AlertEmailRecipient
		if err := rows.Scan(&r.TenantID, &r.Subject, &r.DeployFailed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EnsureAlertEmailWatermark seeds one source's cursor at `at` on first start,
// so enabling the feature does not mail the history, and returns the current one.
func (s *PGStore) EnsureAlertEmailWatermark(ctx context.Context, name string, at time.Time) (time.Time, string, error) {
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO alert_email_watermarks (name, at, key) VALUES ($1, $2, '') ON CONFLICT (name) DO NOTHING`, name, at); err != nil {
		return time.Time{}, "", err
	}
	var watermarkAt time.Time
	var key string
	if err := s.Pool.QueryRow(ctx, `SELECT at, key FROM alert_email_watermarks WHERE name = $1`, name).Scan(&watermarkAt, &key); err != nil {
		return time.Time{}, "", err
	}
	return watermarkAt, key, nil
}

// EnqueueAlertEmails records each failure once per recipient, folds it into
// that recipient's stream, and advances the named watermark — in one
// transaction. An empty watermark enqueues without moving any cursor (a
// worker-raised alert, not a tailed source). A first failure (or one after a quiet digest window) is due at
// once; a failure inside the window after the last mail is due when it closes.
func (s *PGStore) EnqueueAlertEmails(ctx context.Context, watermark string, items []AlertEmailItem, window time.Duration, at time.Time, key string) error {
	if (watermark != "" && at.IsZero()) || window <= 0 {
		return fmt.Errorf("alert email watermark: %w", ErrInvalid)
	}
	for _, item := range items {
		if item.TenantID == "" || item.Subject == "" || item.SourceKey == "" || item.ResourceID == "" ||
			item.OccurredAt.IsZero() || !validAlertEmailEvent(item.Event) {
			return fmt.Errorf("alert email item: %w", ErrInvalid)
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(`
			WITH recorded AS (
				INSERT INTO alert_email_events (tenant_id, subject, source_key)
				VALUES ($1, $2, $3)
				ON CONFLICT DO NOTHING
				RETURNING 1
			)
			INSERT INTO alert_email_streams AS st (
				tenant_id, subject, resource_id, event, resource_name, detail,
				occurrences, pending_count, due_at
			)
			SELECT $1, $2, $4, $5, $6, $7, ARRAY[$8::timestamptz], 1, $8::timestamptz FROM recorded
			ON CONFLICT (tenant_id, subject, resource_id, event) DO UPDATE SET
				resource_name = EXCLUDED.resource_name,
				detail = EXCLUDED.detail,
				occurrences = (st.occurrences || EXCLUDED.occurrences)[
					GREATEST(1, cardinality(st.occurrences) + 2 - $9::int):],
				pending_count = st.pending_count + 1,
				due_at = CASE
					WHEN st.pending_count > 0 THEN st.due_at
					WHEN st.last_mailed_at IS NULL THEN EXCLUDED.due_at
					ELSE GREATEST(EXCLUDED.due_at, st.last_mailed_at + $10::interval)
				END`,
			item.TenantID, item.Subject, item.SourceKey, item.ResourceID, item.Event,
			item.ResourceName, item.Detail, item.OccurredAt, AlertEmailOccurrenceCap, window)
	}
	if watermark != "" {
		// Never let a stale concurrent dispatcher move the durable cursor backward.
		batch.Queue(`UPDATE alert_email_watermarks SET at = $2, key = $3 WHERE name = $1 AND (at, key) < ($2, $3)`,
			watermark, at, key)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClaimDueAlertEmailStreams leases disjoint due streams across replicas. A
// crash makes a stream visible again once the lease lapses.
func (s *PGStore) ClaimDueAlertEmailStreams(ctx context.Context, now, leaseUntil time.Time, limit int) ([]AlertEmailStream, error) {
	if limit < 1 || !leaseUntil.After(now) {
		return nil, fmt.Errorf("claim alert email streams: %w", ErrInvalid)
	}
	rows, err := s.Pool.Query(ctx, `
		UPDATE alert_email_streams st SET claimed_until = $2
		WHERE (tenant_id, subject, resource_id, event) IN (
			SELECT tenant_id, subject, resource_id, event FROM alert_email_streams
			WHERE pending_count > 0 AND due_at <= $1
			  AND (claimed_until IS NULL OR claimed_until <= $1)
			ORDER BY due_at
			LIMIT $3
			FOR UPDATE SKIP LOCKED)
		RETURNING tenant_id, subject, resource_id, event, resource_name, detail,
		          occurrences, pending_count, last_mailed_at, attempts, claimed_until`,
		now, leaseUntil, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertEmailStream
	for rows.Next() {
		var st AlertEmailStream
		if err := rows.Scan(&st.TenantID, &st.Subject, &st.ResourceID, &st.Event, &st.ResourceName, &st.Detail,
			&st.Occurrences, &st.PendingCount, &st.LastMailedAt, &st.Attempts, &st.ClaimedUntil); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// CompleteAlertEmailStream records that the claimed stream's pending failures
// were mailed (or deliberately not, e.g. no address). Failures that arrived
// during the send stay pending and fall due when the next window closes. It
// reports false when the lease was lost to another replica.
func (s *PGStore) CompleteAlertEmailStream(ctx context.Context, st AlertEmailStream, mailedAt time.Time, window time.Duration) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE alert_email_streams SET
			last_mailed_at = $6::timestamptz,
			pending_count = pending_count - $7::int,
			occurrences = ARRAY(SELECT o FROM unnest(occurrences) o WHERE o <> ALL($8::timestamptz[]) ORDER BY o),
			due_at = CASE WHEN pending_count - $7::int > 0 THEN $6::timestamptz + $9::interval END,
			claimed_until = NULL,
			attempts = 0
		WHERE tenant_id = $1 AND subject = $2 AND resource_id = $3 AND event = $4 AND claimed_until = $5`,
		st.TenantID, st.Subject, st.ResourceID, st.Event, st.ClaimedUntil, mailedAt,
		st.PendingCount, st.Occurrences, window)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// DropAlertEmailStream discards a claimed stream's pending failures without
// recording a mail, so the next failure is still mailed at once.
func (s *PGStore) DropAlertEmailStream(ctx context.Context, st AlertEmailStream) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE alert_email_streams SET
			pending_count = pending_count - $6::int,
			occurrences = ARRAY(SELECT o FROM unnest(occurrences) o WHERE o <> ALL($7::timestamptz[]) ORDER BY o),
			due_at = CASE WHEN pending_count - $6::int > 0 THEN due_at END,
			claimed_until = NULL,
			attempts = 0
		WHERE tenant_id = $1 AND subject = $2 AND resource_id = $3 AND event = $4 AND claimed_until = $5`,
		st.TenantID, st.Subject, st.ResourceID, st.Event, st.ClaimedUntil, st.PendingCount, st.Occurrences)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// RetryAlertEmailStream releases a claimed stream to be tried again at next.
func (s *PGStore) RetryAlertEmailStream(ctx context.Context, st AlertEmailStream, next time.Time) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE alert_email_streams SET claimed_until = NULL, attempts = attempts + 1, due_at = $6
		WHERE tenant_id = $1 AND subject = $2 AND resource_id = $3 AND event = $4 AND claimed_until = $5`,
		st.TenantID, st.Subject, st.ResourceID, st.Event, st.ClaimedUntil, next)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ListFailedBlueprintSyncs pages Blueprint syncs that ended in error after the
// (afterAt, afterID) cursor and no later than until.
func (s *PGStore) ListFailedBlueprintSyncs(ctx context.Context, afterAt time.Time, afterID string, until time.Time, limit int) ([]FailedBlueprintSync, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.id, r.completed_at, b.id, b.name, b.tenant_id, COALESCE(r.error_message, '')
		FROM blueprint_syncs r
		JOIN blueprints b ON b.id = r.blueprint_id
		WHERE r.state = 'error' AND r.completed_at IS NOT NULL
		  AND (r.completed_at, r.id) > ($1, $2) AND r.completed_at < $3
		ORDER BY r.completed_at, r.id
		LIMIT $4`, afterAt, afterID, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FailedBlueprintSync
	for rows.Next() {
		var f FailedBlueprintSync
		if err := rows.Scan(&f.SyncID, &f.CompletedAt, &f.BlueprintID, &f.BlueprintName, &f.TenantID, &f.ErrorMessage); err != nil {
			return nil, err
		}
		f.ErrorMessage = strings.TrimSpace(f.ErrorMessage)
		out = append(out, f)
	}
	return out, rows.Err()
}

// SweepAlertEmails drops dedupe records older than cutoff and idle streams
// whose last mail is older than cutoff.
func (s *PGStore) SweepAlertEmails(ctx context.Context, cutoff time.Time) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM alert_email_events WHERE recorded_at < $1`, cutoff); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM alert_email_streams
		WHERE pending_count = 0 AND claimed_until IS NULL AND (last_mailed_at IS NULL OR last_mailed_at < $1)`, cutoff)
	return err
}
