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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	// A member normally has one phone and perhaps a tablet/test install. Ten
	// leaves ample headroom while bounding the persistent dimension one viewer
	// can project into every push event.
	MaxActivePushDevicesPerSubject = 10
	// The workspace cap is a second, race-safe ceiling across many subjects. It
	// protects the global worker even if a workspace legitimately has a large
	// membership or several compromised members.
	MaxActivePushDevicesPerWorkspace = 1000
)

var (
	ErrPushDeviceSubjectLimit   = fmt.Errorf("member has the maximum number of active push devices: %w", ErrConflict)
	ErrPushDeviceWorkspaceLimit = fmt.Errorf("workspace has the maximum number of active push devices: %w", ErrConflict)
)

// DevicePushSubscription is one opaque provider destination. Token is an
// internal delivery capability: it is deliberately excluded from JSON and
// never projected by the caller-facing notifications service.
type DevicePushSubscription struct {
	TenantID         string     `json:"tenantId"`
	Subject          string     `json:"subject"`
	DeviceID         string     `json:"deviceId"`
	Provider         string     `json:"provider"`
	Platform         string     `json:"platform"`
	Token            string     `json:"-"`
	TokenDigest      string     `json:"-"`
	PreferenceID     string     `json:"preferenceRef,omitempty"`
	RevokedAt        *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastRegisteredAt time.Time  `json:"lastRegisteredAt"`
}

// UpsertDevicePushSubscription atomically registers or replaces one app
// installation. The workspace lock makes both cumulative quotas race-safe; the
// token digest lock serializes account-switch races for the same provider
// capability. A token moved to another member/device revokes its old row before
// activation.
func (s *PGStore) UpsertDevicePushSubscription(ctx context.Context, sub DevicePushSubscription) (DevicePushSubscription, error) {
	digest := pushTokenDigest(sub.Provider, sub.Token)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Always take workspace then token locks in this order. The first serializes
	// quota checks across every member of one workspace; both lock inputs are
	// non-secret (the second is a one-way digest), so no bearer token reaches SQL
	// diagnostics.
	//
	// The namespace separator is ":" and not a NUL: this string is bound as a
	// Postgres `text` parameter, which cannot carry 0x00 (SQLSTATE 22021), and a
	// tenant id is `tea-<xid>` so ":" cannot occur inside one. It also cannot
	// collide with the digest key below, which is always 64 hex characters.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "push-workspace:"+sub.TenantID); err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, digest); err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE device_push_subscriptions
		SET revoked_at = now(), updated_at = now()
		WHERE provider = $1 AND token_digest = $2 AND revoked_at IS NULL
		  AND ROW(tenant_id, subject, device_id) IS DISTINCT FROM ROW($3, $4, $5)`,
		sub.Provider, digest, sub.TenantID, sub.Subject, sub.DeviceID,
	); err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}

	var subjectCount, workspaceCount int
	var alreadyActive bool
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE subject = $2), count(*),
		       COALESCE(bool_or(subject = $2 AND device_id = $3), false)
		FROM device_push_subscriptions
		WHERE tenant_id = $1 AND revoked_at IS NULL`,
		sub.TenantID, sub.Subject, sub.DeviceID,
	).Scan(&subjectCount, &workspaceCount, &alreadyActive); err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}
	if !alreadyActive && subjectCount >= MaxActivePushDevicesPerSubject {
		return DevicePushSubscription{}, ErrPushDeviceSubjectLimit
	}
	if !alreadyActive && workspaceCount >= MaxActivePushDevicesPerWorkspace {
		return DevicePushSubscription{}, ErrPushDeviceWorkspaceLimit
	}

	sub.TokenDigest = digest
	err = tx.QueryRow(ctx, `
		INSERT INTO device_push_subscriptions
			(tenant_id, subject, device_id, provider, platform, token, token_digest, preference_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7,
			(SELECT id FROM notification_settings WHERE tenant_id = $1 AND subject = $2))
		ON CONFLICT (tenant_id, subject, device_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			platform = EXCLUDED.platform,
			token = EXCLUDED.token,
			token_digest = EXCLUDED.token_digest,
			preference_id = EXCLUDED.preference_id,
			revoked_at = NULL,
			updated_at = now(),
			last_registered_at = now()
		RETURNING COALESCE(preference_id, ''), revoked_at, created_at, updated_at, last_registered_at`,
		sub.TenantID, sub.Subject, sub.DeviceID, sub.Provider, sub.Platform, sub.Token, digest,
	).Scan(&sub.PreferenceID, &sub.RevokedAt, &sub.CreatedAt, &sub.UpdatedAt, &sub.LastRegisteredAt)
	if err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DevicePushSubscription{}, classifyPushSubscriptionError(err)
	}
	return sub, nil
}

// ListOwnDevicePushSubscriptions returns active devices for one exact member.
// The SQL projection intentionally omits token and digest so even an accidental
// direct JSON encoding of the result cannot disclose the bearer capability.
func (s *PGStore) ListOwnDevicePushSubscriptions(ctx context.Context, tenantID, subject string) ([]DevicePushSubscription, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT tenant_id, subject, device_id, provider, platform,
		       COALESCE(preference_id, ''), created_at, updated_at, last_registered_at
		FROM device_push_subscriptions
		WHERE tenant_id = $1 AND subject = $2 AND revoked_at IS NULL
		ORDER BY updated_at DESC, device_id`, tenantID, subject)
	if err != nil {
		return nil, fmt.Errorf("device push subscriptions: %w", err)
	}
	defer rows.Close()
	var out []DevicePushSubscription
	for rows.Next() {
		var sub DevicePushSubscription
		if err := rows.Scan(
			&sub.TenantID, &sub.Subject, &sub.DeviceID, &sub.Provider, &sub.Platform,
			&sub.PreferenceID, &sub.CreatedAt, &sub.UpdatedAt, &sub.LastRegisteredAt,
		); err != nil {
			return nil, fmt.Errorf("device push subscription scan: %w", err)
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// RevokeDevicePushSubscription is an idempotent, member-scoped logout/delete.
// A guessed device id belonging to another member looks exactly like no row.
func (s *PGStore) RevokeDevicePushSubscription(ctx context.Context, tenantID, subject, deviceID string) (bool, error) {
	result, err := s.Pool.Exec(ctx, `
		UPDATE device_push_subscriptions
		SET revoked_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND subject = $2 AND device_id = $3 AND revoked_at IS NULL`,
		tenantID, subject, deviceID)
	if err != nil {
		return false, classifyPushSubscriptionError(err)
	}
	return result.RowsAffected() == 1, nil
}

// RevokeAllDevicePushSubscriptions removes every active destination for the
// caller in this workspace, used by explicit all-device logout/revocation.
func (s *PGStore) RevokeAllDevicePushSubscriptions(ctx context.Context, tenantID, subject string) (int64, error) {
	result, err := s.Pool.Exec(ctx, `
		UPDATE device_push_subscriptions
		SET revoked_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND subject = $2 AND revoked_at IS NULL`, tenantID, subject)
	if err != nil {
		return 0, classifyPushSubscriptionError(err)
	}
	return result.RowsAffected(), nil
}

func pushTokenDigest(provider, token string) string {
	sum := sha256.Sum256([]byte(provider + "\x00" + token))
	return hex.EncodeToString(sum[:])
}

// classifyPushSubscriptionError intentionally never returns the database
// driver's message: a constraint error's Detail may contain the failing row,
// including the bearer token. Keep only the shared taxonomy and safe constants.
func classifyPushSubscriptionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("device push subscription: %w", ErrNotFound)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("device push subscription: %w", ErrConflict)
		case "23503":
			return fmt.Errorf("device push subscription reference: %w", ErrNotFound)
		case "23514":
			return fmt.Errorf("device push subscription: %w", ErrInvalid)
		}
	}
	return errors.New("device push subscription store operation failed")
}
