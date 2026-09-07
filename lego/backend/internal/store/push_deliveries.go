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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// ActivePushSubscription is the internal-only delivery projection. Token is
// never selected by caller-facing device reads and can never enter JSON.
type ActivePushSubscription struct {
	TenantID   string
	Subject    string
	Role       string
	DeviceID   string
	Provider   string
	Platform   string
	Token      string          `json:"-"`
	P256dh     string          `json:"-"`
	Auth       string          `json:"-"`
	PushPolicy json.RawMessage `json:"-"`
	CreatedAt  time.Time
}

// PushNotification is one durable logical inbox item. It is intentionally
// thin and token-free; clients fetch details through the authenticated API.
type PushNotification struct {
	TenantID       string
	Subject        string
	SourceEventKey string
	EventID        string
	EventType      string
	Title          string
	Body           string
	Urgency        string
	ResourceKind   string
	ResourceID     string
	DeepLink       string
	OccurredAt     time.Time
	DeliverAt      time.Time
	CreatedAt      time.Time
	ReadAt         *time.Time
}

// defaultPushInboxPage is the mobile inbox's default page — deliberately
// larger than core.DefaultPageLimit because the inbox is normally read whole,
// not paged through. The MAXIMUM stays core.MaxPageLimit, shared with every
// other list.
const defaultPushInboxPage = 50

func (s *PGStore) ListOwnPushNotifications(ctx context.Context, tenantID, subject string, limit int) ([]PushNotification, error) {
	// Defense in depth only: the sole caller (notifications.Service) rejects an
	// out-of-range limit with a 400 before reaching here, so neither arm is
	// reachable today. Kept as a normalization rather than deleted — a store
	// method must not depend on one caller's validation — but spelled with the
	// shared maximum and a named default instead of bare 50/100 literals.
	if limit < 1 {
		limit = defaultPushInboxPage
	}
	limit = min(limit, core.MaxPageLimit)
	rows, err := s.Pool.Query(ctx, `SELECT tenant_id,subject,source_event_key,event_id,event_type,title,body,urgency,resource_kind,resource_id,deep_link,occurred_at,deliver_at,created_at,read_at
	 FROM push_notifications WHERE tenant_id=$1 AND subject=$2 ORDER BY occurred_at DESC,event_id DESC LIMIT $3`, tenantID, subject, limit)
	if err != nil {
		return nil, safePushStoreError(ctx, err)
	}
	defer rows.Close()
	var out []PushNotification
	for rows.Next() {
		var n PushNotification
		if err := rows.Scan(&n.TenantID, &n.Subject, &n.SourceEventKey, &n.EventID, &n.EventType, &n.Title, &n.Body, &n.Urgency, &n.ResourceKind, &n.ResourceID, &n.DeepLink, &n.OccurredAt, &n.DeliverAt, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, safePushStoreError(ctx, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *PGStore) MarkOwnPushNotificationRead(ctx context.Context, tenantID, subject, eventID string, at time.Time) (bool, error) {
	if at.IsZero() {
		return false, fmt.Errorf("push notification read: %w", ErrInvalid)
	}
	r, err := s.Pool.Exec(ctx, `UPDATE push_notifications SET read_at=COALESCE(read_at,$4) WHERE tenant_id=$1 AND subject=$2 AND event_id=$3`, tenantID, subject, eventID, at)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return r.RowsAffected() == 1, nil
}

func (s *PGStore) CountUnreadPushNotifications(ctx context.Context, tenantID, subject string) (int64, error) {
	var count int64
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM push_notifications WHERE tenant_id=$1 AND subject=$2 AND read_at IS NULL`, tenantID, subject).Scan(&count)
	if err != nil {
		return 0, safePushStoreError(ctx, err)
	}
	return count, nil
}

// PushNotificationBatchItem fans one logical notification out to these exact
// active device identities in the same transaction as the push watermark.
type PushNotificationBatchItem struct {
	Notification PushNotification
	DeviceIDs    []string
}

// DuePushDelivery is a leased per-device row joined to its current active
// token. The token remains internal and is excluded from JSON by construction.
type DuePushDelivery struct {
	PushNotification
	DeviceID         string
	SessionID        string
	Provider         string
	Platform         string
	Token            string `json:"-"`
	P256dh           string `json:"-"`
	Auth             string `json:"-"`
	TokenDigest      string `json:"-"`
	ClaimedUntil     time.Time
	AttemptCount     int
	AcceptedAt       *time.Time
	ReceiptDueAt     *time.Time
	ProviderTicketID string
}

type PushQueueStats struct {
	Pending        int64
	ReceiptPending int64
	Terminal       int64
}

type PushSweepResult struct {
	RevokedDeliveries     int64
	TerminalNotifications int64
}

// ListActivePushSubscriptions is the worker-only destination read. Membership
// role is read in the same query so later policy integration cannot use stale
// caller-supplied authorization input.
func (s *PGStore) ListActivePushSubscriptions(ctx context.Context) ([]ActivePushSubscription, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT tenant_id, subject, role, device_id, provider, platform, token,
		       push_policy, created_at, p256dh, auth
		FROM (
			SELECT d.tenant_id, d.subject, m.role, d.device_id, d.provider,
			       d.platform, d.token, n.push_policy, d.created_at, '' AS p256dh, '' AS auth
			FROM device_push_subscriptions d
			JOIN tenant_members m ON m.tenant_id = d.tenant_id AND m.subject = d.subject
			LEFT JOIN notification_settings n ON n.tenant_id = d.tenant_id AND n.subject = d.subject
			WHERE d.revoked_at IS NULL
			UNION ALL
			SELECT w.tenant_id, w.subject, m.role, w.browser_id, 'webpush',
			       'web', w.endpoint, n.push_policy, w.created_at, w.p256dh, w.auth
			FROM webpush_subscriptions w
			JOIN tenant_members m ON m.tenant_id = w.tenant_id AND m.subject = w.subject
			LEFT JOIN notification_settings n ON n.tenant_id = w.tenant_id AND n.subject = w.subject
			WHERE w.revoked_at IS NULL
		) destinations
		ORDER BY tenant_id, subject, device_id`)
	if err != nil {
		return nil, safePushStoreError(ctx, err)
	}
	defer rows.Close()
	var out []ActivePushSubscription
	for rows.Next() {
		var destination ActivePushSubscription
		if err := rows.Scan(
			&destination.TenantID, &destination.Subject, &destination.Role,
			&destination.DeviceID, &destination.Provider, &destination.Platform,
			&destination.Token, &destination.PushPolicy, &destination.CreatedAt,
			&destination.P256dh, &destination.Auth,
		); err != nil {
			return nil, safePushStoreError(ctx, err)
		}
		out = append(out, destination)
	}
	if err := rows.Err(); err != nil {
		return nil, safePushStoreError(ctx, err)
	}
	return out, nil
}

// EnsurePushWatermark seeds push's independent feed cursor on first start.
func (s *PGStore) EnsurePushWatermark(ctx context.Context, at time.Time) (time.Time, string, error) {
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO push_watermark (id, at, key) VALUES (true, $1, '') ON CONFLICT (id) DO NOTHING`, at); err != nil {
		return time.Time{}, "", safePushStoreError(ctx, err)
	}
	var watermarkAt time.Time
	var watermarkKey string
	if err := s.Pool.QueryRow(ctx, `SELECT at, key FROM push_watermark WHERE id`).Scan(&watermarkAt, &watermarkKey); err != nil {
		return time.Time{}, "", safePushStoreError(ctx, err)
	}
	return watermarkAt, watermarkKey, nil
}

// EnqueuePushNotifications inserts logical and per-device rows and advances
// the distinct push cursor in one transaction. Composite unique constraints
// make feed replay, restart, and concurrent replicas converge without duplicates.
func (s *PGStore) EnqueuePushNotifications(ctx context.Context, items []PushNotificationBatchItem, at time.Time, key string) error {
	if at.IsZero() {
		return fmt.Errorf("push watermark: %w", ErrInvalid)
	}
	for _, item := range items {
		if err := ValidatePushNotification(item.Notification); err != nil {
			return err
		}
		for _, deviceID := range item.DeviceIDs {
			if strings.TrimSpace(deviceID) == "" || strings.ContainsRune(deviceID, '\x00') {
				return fmt.Errorf("push delivery device: %w", ErrInvalid)
			}
		}
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return safePushStoreError(ctx, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	batch := &pgx.Batch{}
	for _, item := range items {
		n := item.Notification
		batch.Queue(`
			INSERT INTO push_notifications (
				tenant_id, subject, source_event_key, event_id, event_type,
				title, body, urgency, resource_kind, resource_id, deep_link,
				occurred_at, deliver_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (tenant_id, subject, source_event_key) DO NOTHING`,
			n.TenantID, n.Subject, n.SourceEventKey, n.EventID, n.EventType,
			n.Title, n.Body, n.Urgency, n.ResourceKind, n.ResourceID, n.DeepLink,
			n.OccurredAt, n.DeliverAt)
		for _, deviceID := range item.DeviceIDs {
			batch.Queue(`
				INSERT INTO push_deliveries (
					tenant_id, subject, device_id, source_event_key, next_attempt_at
				)
				VALUES ($1,$2,$3,$4,$5)
				ON CONFLICT (tenant_id, subject, device_id, source_event_key) DO NOTHING`,
				n.TenantID, n.Subject, deviceID, n.SourceEventKey, n.DeliverAt)
		}
	}
	// Never let a stale concurrent dispatcher move the durable cursor backward.
	batch.Queue(`UPDATE push_watermark SET at = $1, key = $2 WHERE id AND (at, key) < ($1, $2)`, at, key)
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return safePushStoreError(ctx, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return safePushStoreError(ctx, err)
	}
	return nil
}

// ClaimDuePushDeliveries leases disjoint rows across worker replicas. A lease
// is not an attempt; a crash makes the row visible again after leaseUntil.
func (s *PGStore) ClaimDuePushDeliveries(ctx context.Context, now, leaseUntil time.Time, limit int) ([]DuePushDelivery, error) {
	if limit < 1 || leaseUntil.Before(now) {
		return nil, fmt.Errorf("claim push deliveries: %w", ErrInvalid)
	}
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
			SELECT d.tenant_id, d.subject, d.device_id, d.source_event_key
			FROM push_deliveries d
			JOIN push_notifications n USING (tenant_id, subject, source_event_key)
			LEFT JOIN device_push_subscriptions expo
			  ON expo.tenant_id = d.tenant_id AND expo.subject = d.subject
			 AND expo.device_id = d.device_id AND expo.revoked_at IS NULL
			LEFT JOIN webpush_subscriptions web
			  ON web.tenant_id = d.tenant_id AND web.subject = d.subject
			 AND web.browser_id = d.device_id AND web.revoked_at IS NULL
			WHERE d.accepted_at IS NULL
			  AND d.failed_at IS NULL
			  AND (d.claimed_until IS NULL OR d.claimed_until <= $1)
			  AND d.next_attempt_at <= $1
			  AND n.deliver_at <= $1
			  AND (expo.device_id IS NOT NULL OR web.browser_id IS NOT NULL)
			ORDER BY n.deliver_at, n.occurred_at, n.event_id, d.device_id
			LIMIT $3
			FOR UPDATE OF d SKIP LOCKED
		), claimed AS (
			UPDATE push_deliveries d
			SET claimed_until = $2
			FROM due
			WHERE d.tenant_id = due.tenant_id AND d.subject = due.subject
			  AND d.device_id = due.device_id AND d.source_event_key = due.source_event_key
			RETURNING d.tenant_id, d.subject, d.device_id, d.source_event_key
		)
		SELECT n.tenant_id, n.subject, n.source_event_key, n.event_id,
		       n.event_type, n.title, n.body, n.urgency, n.resource_kind,
		       n.resource_id, n.deep_link, n.occurred_at, n.deliver_at, n.created_at,
		       c.device_id, COALESCE(expo.session_id, ''),
		       COALESCE(expo.provider, 'webpush'), COALESCE(expo.platform, 'web'),
		       COALESCE(expo.token, web.endpoint), COALESCE(web.p256dh, ''), COALESCE(web.auth, ''),
		       $2, d.attempt_count, d.accepted_at, d.receipt_due_at, d.provider_ticket_id
		FROM claimed c
		JOIN push_notifications n USING (tenant_id, subject, source_event_key)
		JOIN push_deliveries d USING (tenant_id, subject, device_id, source_event_key)
		LEFT JOIN device_push_subscriptions expo
		  ON expo.tenant_id = c.tenant_id AND expo.subject = c.subject
		 AND expo.device_id = c.device_id AND expo.revoked_at IS NULL
		LEFT JOIN webpush_subscriptions web
		  ON web.tenant_id = c.tenant_id AND web.subject = c.subject
		 AND web.browser_id = c.device_id AND web.revoked_at IS NULL
		ORDER BY n.occurred_at, n.event_id, c.device_id`, now, leaseUntil, limit)
	if err != nil {
		return nil, safePushStoreError(ctx, err)
	}
	defer rows.Close()
	var out []DuePushDelivery
	for rows.Next() {
		var delivery DuePushDelivery
		if err := rows.Scan(
			&delivery.TenantID, &delivery.Subject, &delivery.SourceEventKey,
			&delivery.EventID, &delivery.EventType, &delivery.Title, &delivery.Body,
			&delivery.Urgency, &delivery.ResourceKind, &delivery.ResourceID,
			&delivery.DeepLink, &delivery.OccurredAt, &delivery.DeliverAt,
			&delivery.CreatedAt, &delivery.DeviceID, &delivery.SessionID, &delivery.Provider,
			&delivery.Platform, &delivery.Token, &delivery.P256dh, &delivery.Auth, &delivery.ClaimedUntil,
			&delivery.AttemptCount, &delivery.AcceptedAt, &delivery.ReceiptDueAt,
			&delivery.ProviderTicketID,
		); err != nil {
			return nil, safePushStoreError(ctx, err)
		}
		out = append(out, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, safePushStoreError(ctx, err)
	}
	return out, nil
}

// AcceptPushDelivery records provider acceptance only (not device receipt).
// expectedClaim prevents a late worker from completing another worker's lease.
func (s *PGStore) AcceptPushDelivery(ctx context.Context, delivery DuePushDelivery, ticketID string, at, receiptDue time.Time) (bool, error) {
	if strings.TrimSpace(ticketID) == "" || len(ticketID) > 256 || strings.ContainsRune(ticketID, '\x00') || at.IsZero() {
		return false, fmt.Errorf("accept push delivery: %w", ErrInvalid)
	}
	result, err := s.Pool.Exec(ctx, `
		UPDATE push_deliveries
		SET provider_ticket_id = $6, accepted_token_digest = $9, accepted_at = $7, receipt_due_at = $8,
		    last_attempted_at = $7, attempt_count = attempt_count + 1, claimed_until = NULL
		WHERE tenant_id = $1 AND subject = $2 AND device_id = $3
		  AND source_event_key = $4 AND claimed_until = $5 AND accepted_at IS NULL`,
		delivery.TenantID, delivery.Subject, delivery.DeviceID,
		delivery.SourceEventKey, delivery.ClaimedUntil, ticketID, at, receiptDue,
		pushTokenDigest(delivery.Provider, delivery.Token))
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return result.RowsAffected() == 1, nil
}

// CompletePushDelivery marks a Web Push send delivered without a receipt poll.
func (s *PGStore) CompletePushDelivery(ctx context.Context, delivery DuePushDelivery, at time.Time) (bool, error) {
	if at.IsZero() {
		return false, fmt.Errorf("complete push delivery: %w", ErrInvalid)
	}
	result, err := s.Pool.Exec(ctx, `
		UPDATE push_deliveries
		SET delivered_at = $6, last_attempted_at = $6, attempt_count = attempt_count + 1,
		    claimed_until = NULL
		WHERE tenant_id = $1 AND subject = $2 AND device_id = $3
		  AND source_event_key = $4 AND claimed_until = $5
		  AND accepted_at IS NULL AND failed_at IS NULL AND delivered_at IS NULL`,
		delivery.TenantID, delivery.Subject, delivery.DeviceID,
		delivery.SourceEventKey, delivery.ClaimedUntil, at)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *PGStore) RecordPushSendFailure(ctx context.Context, delivery DuePushDelivery, code string, at, next time.Time, terminal bool) (bool, error) {
	var failedAt any
	if terminal {
		failedAt = at
	}
	result, err := s.Pool.Exec(ctx, `
		UPDATE push_deliveries SET attempt_count = attempt_count + 1,
		  last_error_code = $6, last_attempted_at = $7, next_attempt_at = $8,
		  failed_at = $9, claimed_until = NULL
		WHERE tenant_id=$1 AND subject=$2 AND device_id=$3 AND source_event_key=$4
		  AND claimed_until=$5 AND accepted_at IS NULL AND failed_at IS NULL`,
		delivery.TenantID, delivery.Subject, delivery.DeviceID, delivery.SourceEventKey,
		delivery.ClaimedUntil, code, at, next, failedAt)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *PGStore) ClaimDuePushReceipts(ctx context.Context, now, leaseUntil time.Time, limit int) ([]DuePushDelivery, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
		 SELECT d.tenant_id,d.subject,d.device_id,d.source_event_key FROM push_deliveries d
		 WHERE d.accepted_at IS NOT NULL AND d.delivered_at IS NULL AND d.failed_at IS NULL AND d.ambiguous_at IS NULL
		   AND d.receipt_due_at <= $1 AND (d.claimed_until IS NULL OR d.claimed_until <= $1)
		 ORDER BY d.receipt_due_at LIMIT $3 FOR UPDATE SKIP LOCKED
		), claimed AS (
		 UPDATE push_deliveries d SET claimed_until=$2 FROM due
		 WHERE d.tenant_id=due.tenant_id AND d.subject=due.subject AND d.device_id=due.device_id AND d.source_event_key=due.source_event_key
		 RETURNING d.tenant_id,d.subject,d.device_id,d.source_event_key
		)
		SELECT n.tenant_id,n.subject,n.source_event_key,n.event_id,n.event_type,n.title,n.body,n.urgency,n.resource_kind,n.resource_id,n.deep_link,n.occurred_at,n.deliver_at,n.created_at,
			 c.device_id,s.provider,s.platform,s.token,d.accepted_token_digest,$2,d.attempt_count,d.accepted_at,d.receipt_due_at,d.provider_ticket_id
		FROM claimed c JOIN push_notifications n USING (tenant_id,subject,source_event_key)
		JOIN push_deliveries d USING (tenant_id,subject,device_id,source_event_key)
		JOIN device_push_subscriptions s USING (tenant_id,subject,device_id)`, now, leaseUntil, limit)
	if err != nil {
		return nil, safePushStoreError(ctx, err)
	}
	defer rows.Close()
	var out []DuePushDelivery
	for rows.Next() {
		var d DuePushDelivery
		if err := rows.Scan(&d.TenantID, &d.Subject, &d.SourceEventKey, &d.EventID, &d.EventType, &d.Title, &d.Body, &d.Urgency, &d.ResourceKind, &d.ResourceID, &d.DeepLink, &d.OccurredAt, &d.DeliverAt, &d.CreatedAt, &d.DeviceID, &d.Provider, &d.Platform, &d.Token, &d.TokenDigest, &d.ClaimedUntil, &d.AttemptCount, &d.AcceptedAt, &d.ReceiptDueAt, &d.ProviderTicketID); err != nil {
			return nil, safePushStoreError(ctx, err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGStore) RecordPushReceipt(ctx context.Context, d DuePushDelivery, code string, at, next time.Time, delivered, failed, ambiguous bool) (bool, error) {
	result, err := s.Pool.Exec(ctx, `UPDATE push_deliveries SET last_error_code=$6,receipt_checked_at=$7,receipt_due_at=$8,
	 delivered_at=CASE WHEN $9 THEN $7 ELSE delivered_at END, failed_at=CASE WHEN $10 THEN $7 ELSE failed_at END,
	 ambiguous_at=CASE WHEN $11 THEN $7 ELSE ambiguous_at END, claimed_until=NULL
	 WHERE tenant_id=$1 AND subject=$2 AND device_id=$3 AND source_event_key=$4 AND claimed_until=$5`,
		d.TenantID, d.Subject, d.DeviceID, d.SourceEventKey, d.ClaimedUntil, code, at, next, delivered, failed, ambiguous)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *PGStore) RevokeExactPushSubscription(ctx context.Context, d DuePushDelivery) (bool, error) {
	digest := d.TokenDigest
	if digest == "" {
		digest = pushTokenDigest(d.Provider, d.Token)
	}
	result, err := s.Pool.Exec(ctx, `UPDATE device_push_subscriptions SET revoked_at=now(),updated_at=now()
	 WHERE tenant_id=$1 AND subject=$2 AND device_id=$3 AND token_digest=$4 AND revoked_at IS NULL`, d.TenantID, d.Subject, d.DeviceID, digest)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	if result.RowsAffected() == 1 {
		return true, nil
	}
	result, err = s.Pool.Exec(ctx, `UPDATE webpush_subscriptions SET revoked_at=now(),updated_at=now()
	 WHERE tenant_id=$1 AND subject=$2 AND browser_id=$3 AND endpoint_digest=$4 AND revoked_at IS NULL`, d.TenantID, d.Subject, d.DeviceID, digest)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *PGStore) PushDeliveryStats(ctx context.Context) (PushQueueStats, error) {
	var v PushQueueStats
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE accepted_at IS NULL AND failed_at IS NULL),count(*) FILTER (WHERE accepted_at IS NOT NULL AND delivered_at IS NULL AND failed_at IS NULL AND ambiguous_at IS NULL),count(*) FILTER (WHERE delivered_at IS NOT NULL OR failed_at IS NOT NULL OR ambiguous_at IS NOT NULL) FROM push_deliveries`).Scan(&v.Pending, &v.ReceiptPending, &v.Terminal)
	if err != nil {
		return v, safePushStoreError(ctx, err)
	}
	return v, nil
}

func (s *PGStore) SweepPushRetention(ctx context.Context, revokedBefore, terminalBefore time.Time) (PushSweepResult, error) {
	var out PushSweepResult
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return out, safePushStoreError(ctx, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := tx.Exec(ctx, `DELETE FROM push_deliveries d USING device_push_subscriptions s WHERE d.tenant_id=s.tenant_id AND d.subject=s.subject AND d.device_id=s.device_id AND s.revoked_at IS NOT NULL AND s.revoked_at < $1 AND (d.accepted_at IS NULL OR d.delivered_at IS NOT NULL OR d.failed_at IS NOT NULL OR d.ambiguous_at IS NOT NULL)`, revokedBefore)
	if err != nil {
		return out, safePushStoreError(ctx, err)
	}
	out.RevokedDeliveries = r.RowsAffected()
	r, err = tx.Exec(ctx, `DELETE FROM push_deliveries d USING webpush_subscriptions w WHERE d.tenant_id=w.tenant_id AND d.subject=w.subject AND d.device_id=w.browser_id AND w.revoked_at IS NOT NULL AND w.revoked_at < $1 AND (d.accepted_at IS NULL OR d.delivered_at IS NOT NULL OR d.failed_at IS NOT NULL OR d.ambiguous_at IS NOT NULL)`, revokedBefore)
	if err != nil {
		return out, safePushStoreError(ctx, err)
	}
	out.RevokedDeliveries += r.RowsAffected()
	r, err = tx.Exec(ctx, `DELETE FROM push_deliveries d WHERE (d.accepted_at IS NULL OR d.delivered_at IS NOT NULL OR d.failed_at IS NOT NULL OR d.ambiguous_at IS NOT NULL)
	  AND NOT EXISTS (SELECT 1 FROM device_push_subscriptions s WHERE s.tenant_id=d.tenant_id AND s.subject=d.subject AND s.device_id=d.device_id)
	  AND NOT EXISTS (SELECT 1 FROM webpush_subscriptions w WHERE w.tenant_id=d.tenant_id AND w.subject=d.subject AND w.browser_id=d.device_id)`)
	if err != nil {
		return out, safePushStoreError(ctx, err)
	}
	out.RevokedDeliveries += r.RowsAffected()
	r, err = tx.Exec(ctx, `DELETE FROM push_notifications n WHERE n.created_at < $1 AND NOT EXISTS (SELECT 1 FROM push_deliveries d WHERE d.tenant_id=n.tenant_id AND d.subject=n.subject AND d.source_event_key=n.source_event_key AND d.delivered_at IS NULL AND d.failed_at IS NULL AND d.ambiguous_at IS NULL)`, terminalBefore)
	if err != nil {
		return out, safePushStoreError(ctx, err)
	}
	out.TerminalNotifications = r.RowsAffected()
	if err := tx.Commit(ctx); err != nil {
		return out, safePushStoreError(ctx, err)
	}
	return out, nil
}

// ReleasePushDelivery leaves a failed provider attempt retryable without
// persisting provider text. expectedClaim protects a newer lease.
func (s *PGStore) ReleasePushDelivery(ctx context.Context, delivery DuePushDelivery) (bool, error) {
	result, err := s.Pool.Exec(ctx, `
		UPDATE push_deliveries SET claimed_until = NULL
		WHERE tenant_id = $1 AND subject = $2 AND device_id = $3
		  AND source_event_key = $4 AND claimed_until = $5 AND accepted_at IS NULL`,
		delivery.TenantID, delivery.Subject, delivery.DeviceID,
		delivery.SourceEventKey, delivery.ClaimedUntil)
	if err != nil {
		return false, safePushStoreError(ctx, err)
	}
	return result.RowsAffected() == 1, nil
}

// pushEventRule is the per-event-type resource contract: which ResourceKind the
// notification must name, which id kind its ResourceID must be, and the deep
// link prefix it must carry. Holding the three together means the producer's
// vocabulary and this validator cannot drift apart field by field.
type pushEventRule struct {
	resourceKind   string
	idKind         ids.Kind
	deepLinkPrefix string
}

var (
	serviceEventRule = pushEventRule{resourceKind: "service", idKind: ids.Service, deepLinkPrefix: "/services/"}
	// Agent-session terminal pushes (w11/m6 t005) are a bex extension keyed on
	// the workspace, not an App — so the resource is the agent_session id and the
	// deep link opens the session, never a service.
	agentEventRule = pushEventRule{resourceKind: "agentSession", idKind: ids.AgentSession, deepLinkPrefix: "/sessions/"}
)

// pushEventRules is the whole DB-enqueueable push vocabulary; its key set must
// stay equal to the push_notifications event_type CHECK (migration 0070). The
// four lifecycle names are deliberately the fact names; deploy_*/cron_failed
// are projection-only names with no fact constant.
var pushEventRules = map[string]pushEventRule{
	"deploy_started":                  serviceEventRule,
	"deploy_succeeded":                serviceEventRule,
	"deploy_failed":                   serviceEventRule,
	string(EventFactServerFailed):     serviceEventRule,
	string(EventFactServerAvailable):  serviceEventRule,
	string(EventFactServiceSuspended): serviceEventRule,
	string(EventFactServiceResumed):   serviceEventRule,
	"cron_failed":                     serviceEventRule,
	"agent_pr_ready":                  agentEventRule,
	"agent_failed":                    agentEventRule,
	"agent_needs_decision":            agentEventRule,
}

var pushUrgencies = map[string]bool{"routine": true, "important": true, "critical": true}

// ValidatePushNotification is the producer-side admission seam. Feed
// projections must be able to drop one malformed logical item without making
// the shared watermark retry that poison event forever.
func ValidatePushNotification(notification PushNotification) error {
	rule, known := pushEventRules[notification.EventType]
	eventKind, eventOK := ids.KindOf(notification.EventID)
	resourceKind, resourceOK := ids.KindOf(notification.ResourceID)
	valid := known &&
		strings.TrimSpace(notification.TenantID) != "" && strings.TrimSpace(notification.Subject) != "" &&
		strings.TrimSpace(notification.SourceEventKey) != "" && len(notification.SourceEventKey) <= 512 &&
		eventOK && eventKind == ids.Event &&
		len(notification.Title) >= 1 && len(notification.Title) <= 120 && !strings.ContainsRune(notification.Title, '\x00') &&
		len(notification.Body) >= 1 && len(notification.Body) <= 1024 && !strings.ContainsRune(notification.Body, '\x00') &&
		pushUrgencies[notification.Urgency] && resourceOK &&
		!notification.OccurredAt.IsZero() && !notification.DeliverAt.IsZero() &&
		notification.ResourceKind == rule.resourceKind && resourceKind == rule.idKind &&
		notification.DeepLink == rule.deepLinkPrefix+notification.ResourceID
	if !valid {
		return fmt.Errorf("push notification: %w", ErrInvalid)
	}
	return nil
}

// safePushStoreError never returns driver detail because due-delivery reads
// contain the provider token capability.
func safePushStoreError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("push delivery: %w", ErrNotFound)
	}
	return errors.New("push delivery store operation failed")
}
