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
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	MaxActiveWebPushBrowsersPerSubject   = 10
	MaxActiveWebPushBrowsersPerWorkspace = 1000
)

var (
	ErrWebPushSubjectLimit   = fmt.Errorf("member has the maximum number of active web-push browsers: %w", ErrConflict)
	ErrWebPushWorkspaceLimit = fmt.Errorf("workspace has the maximum number of active web-push browsers: %w", ErrConflict)
)

// WebPushSubscription is one browser PushSubscription. Endpoint, keys, and
// digest are internal delivery capabilities and are omitted from JSON.
type WebPushSubscription struct {
	TenantID         string     `json:"tenantId"`
	Subject          string     `json:"subject"`
	BrowserID        string     `json:"browserId"`
	Endpoint         string     `json:"-"`
	P256dh           string     `json:"-"`
	Auth             string     `json:"-"`
	EndpointDigest   string     `json:"-"`
	PreferenceID     string     `json:"preferenceRef,omitempty"`
	RevokedAt        *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastRegisteredAt time.Time  `json:"lastRegisteredAt"`
}

func (s *PGStore) UpsertWebPushSubscription(ctx context.Context, sub WebPushSubscription) (WebPushSubscription, error) {
	digest := pushTokenDigest("webpush", sub.Endpoint)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "webpush-workspace:"+sub.TenantID); err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, digest); err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE webpush_subscriptions
		SET revoked_at = now(), updated_at = now()
		WHERE endpoint_digest = $1 AND revoked_at IS NULL
		  AND ROW(tenant_id, subject, browser_id) IS DISTINCT FROM ROW($2, $3, $4)`,
		digest, sub.TenantID, sub.Subject, sub.BrowserID,
	); err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}

	var subjectCount, workspaceCount int
	var alreadyActive bool
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE subject = $2), count(*),
		       COALESCE(bool_or(subject = $2 AND browser_id = $3), false)
		FROM webpush_subscriptions
		WHERE tenant_id = $1 AND revoked_at IS NULL`,
		sub.TenantID, sub.Subject, sub.BrowserID,
	).Scan(&subjectCount, &workspaceCount, &alreadyActive); err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}
	if !alreadyActive && subjectCount >= MaxActiveWebPushBrowsersPerSubject {
		return WebPushSubscription{}, ErrWebPushSubjectLimit
	}
	if !alreadyActive && workspaceCount >= MaxActiveWebPushBrowsersPerWorkspace {
		return WebPushSubscription{}, ErrWebPushWorkspaceLimit
	}

	sub.EndpointDigest = digest
	err = tx.QueryRow(ctx, `
		INSERT INTO webpush_subscriptions
			(tenant_id, subject, browser_id, endpoint, p256dh, auth, endpoint_digest, preference_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7,
			(SELECT id FROM notification_settings WHERE tenant_id = $1 AND subject = $2))
		ON CONFLICT (tenant_id, subject, browser_id) DO UPDATE SET
			endpoint = EXCLUDED.endpoint,
			p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth,
			endpoint_digest = EXCLUDED.endpoint_digest,
			preference_id = EXCLUDED.preference_id,
			revoked_at = NULL,
			updated_at = now(),
			last_registered_at = now()
		RETURNING COALESCE(preference_id, ''), revoked_at, created_at, updated_at, last_registered_at`,
		sub.TenantID, sub.Subject, sub.BrowserID, sub.Endpoint, sub.P256dh, sub.Auth, digest,
	).Scan(&sub.PreferenceID, &sub.RevokedAt, &sub.CreatedAt, &sub.UpdatedAt, &sub.LastRegisteredAt)
	if err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return WebPushSubscription{}, classifyWebPushSubscriptionError(err)
	}
	return sub, nil
}

func (s *PGStore) ListOwnWebPushSubscriptions(ctx context.Context, tenantID, subject string) ([]WebPushSubscription, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT tenant_id, subject, browser_id, COALESCE(preference_id, ''),
		       created_at, updated_at, last_registered_at
		FROM webpush_subscriptions
		WHERE tenant_id = $1 AND subject = $2 AND revoked_at IS NULL
		ORDER BY updated_at DESC, browser_id`, tenantID, subject)
	if err != nil {
		return nil, fmt.Errorf("webpush subscriptions: %w", err)
	}
	defer rows.Close()
	var out []WebPushSubscription
	for rows.Next() {
		var sub WebPushSubscription
		if err := rows.Scan(
			&sub.TenantID, &sub.Subject, &sub.BrowserID, &sub.PreferenceID,
			&sub.CreatedAt, &sub.UpdatedAt, &sub.LastRegisteredAt,
		); err != nil {
			return nil, fmt.Errorf("webpush subscription scan: %w", err)
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *PGStore) RevokeWebPushSubscription(ctx context.Context, tenantID, subject, browserID string) (bool, error) {
	result, err := s.Pool.Exec(ctx, `
		UPDATE webpush_subscriptions
		SET revoked_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND subject = $2 AND browser_id = $3 AND revoked_at IS NULL`,
		tenantID, subject, browserID)
	if err != nil {
		return false, classifyWebPushSubscriptionError(err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *PGStore) RevokeAllWebPushSubscriptions(ctx context.Context, tenantID, subject string) (int64, error) {
	result, err := s.Pool.Exec(ctx, `
		UPDATE webpush_subscriptions
		SET revoked_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND subject = $2 AND revoked_at IS NULL`, tenantID, subject)
	if err != nil {
		return 0, classifyWebPushSubscriptionError(err)
	}
	return result.RowsAffected(), nil
}

func classifyWebPushSubscriptionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("webpush subscription: %w", ErrNotFound)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("webpush subscription: %w", ErrConflict)
		case "23503":
			return fmt.Errorf("webpush subscription reference: %w", ErrNotFound)
		case "23514":
			return fmt.Errorf("webpush subscription: %w", ErrInvalid)
		}
	}
	return errors.New("webpush subscription store operation failed")
}
