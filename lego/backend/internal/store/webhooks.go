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

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// webhooks.go is the store side of outbound event webhooks (w3/m11,
// internal/webhooks): the endpoint registry, the durable delivery queue, and
// the dispatcher's watermark cursor into the composed event feed.
//
// The feed the dispatcher reads is the SAME composition the per-service
// events feed reads (events.go — deploys + audit_events), just workspace-wide
// and ascending from a watermark instead of per-service and descending from a
// cursor. That identity is w3/m11/t002's whole point: the write paths were
// instrumented once (w3/m7's audit target column + w2/m5's deploys rows), and
// both consumers derive from them — no second emission pass, and a webhook
// fires for a transition no matter which path performed it (API verb, git
// push, reconciler close).

// WebhookEndpoint is a row of `webhook_endpoints`. Secret is populated only
// by CreateWebhookEndpoint (the mint-once read) and by the sender's
// DueWebhookDeliveries join (it signs with it) — every caller-facing read
// (List/Get) leaves it "", structurally: those queries never select the
// column.
type WebhookEndpoint struct {
	ID             string
	TenantID       string
	Name           string
	URL            string
	Secret         string
	EventTypes     []string
	Enabled        bool
	DisabledReason string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// Latest* is populated only by ListWebhookEndpoints. It is the newest
	// completed immutable attempt for this endpoint and the state of that
	// attempt's logical notification, fetched in the list query rather than by
	// one history query per endpoint.
	LatestAttemptStatus string
	LatestAttemptAt     *time.Time
	LatestParentStatus  string
}

// webhookEndpointColumns deliberately omits `secret` — see WebhookEndpoint.
const webhookEndpointColumns = `id, tenant_id, name, url, event_types, enabled, disabled_reason, created_by, created_at, updated_at`

func webhookEndpointScanTargets(e *WebhookEndpoint) []any {
	return []any{
		&e.ID, &e.TenantID, &e.Name, &e.URL, &e.EventTypes, &e.Enabled,
		&e.DisabledReason, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	}
}

func scanWebhookEndpoint(row pgx.Row) (WebhookEndpoint, error) {
	var e WebhookEndpoint
	err := row.Scan(webhookEndpointScanTargets(&e)...)
	return e, err
}

func scanWebhookEndpointWithLatest(row pgx.Row) (WebhookEndpoint, error) {
	var e WebhookEndpoint
	targets := append(webhookEndpointScanTargets(&e),
		&e.LatestAttemptStatus, &e.LatestAttemptAt, &e.LatestParentStatus,
	)
	err := row.Scan(targets...)
	return e, err
}

// MaxWebhookEndpointsPerWorkspace bounds how many endpoints one workspace may
// register (w1/m67 F2). Every event is fanned out across every enabled endpoint
// of its workspace by a worker SHARED with every other tenant, so an unbounded
// endpoint count is an unbounded multiplier on shared memory and database work —
// growable persistently, by an ordinary workspace admin, at no cost to them.
// The value is far above any real integration topology (Render's own docs
// describe a handful of endpoints per workspace); it exists to stop the
// pathological case, not to shape normal use.
const MaxWebhookEndpointsPerWorkspace = 25

// MaxWebhookURLBytes is the destination length cap (codex round-15 #2). The
// shared worker reloads every enabled endpoint every two seconds, so an
// unbounded HTTPS URL (the global 2 MiB body cap) is a persistent memory and
// query-byte multiplier across tenants. 2048 is far above any real webhook
// destination (Slack/PagerDuty/GitHub are hundreds of bytes) and matches the
// practical URL length most intermediaries accept.
const MaxWebhookURLBytes = 2048

// ErrWebhookEndpointLimit is the typed refusal when a workspace is already at
// MaxWebhookEndpointsPerWorkspace. Wraps ErrConflict so existing REST/GraphQL/MCP
// error mapping treats it like the other quota refusals (cf. ErrInvitePlanLimit).
var ErrWebhookEndpointLimit = fmt.Errorf("workspace has the maximum number of webhook endpoints: %w", ErrConflict)

// ErrWebhookEndpointDisabled refuses a manual resend while its destination is
// disabled. It wraps ErrConflict so every adapter keeps the shared conflict
// classification while exposing a stable feature-specific code.
var ErrWebhookEndpointDisabled = fmt.Errorf("webhook endpoint is disabled: %w", ErrConflict)

// ErrWebhookAttemptPending refuses a second, distinct resend while one attempt
// for the same logical notification is already reserved. It bounds concurrent
// replay fan-out without weakening idempotent repeats of the same request key.
var ErrWebhookAttemptPending = fmt.Errorf("webhook attempt is already pending: %w", ErrConflict)

// ErrWebhookEndpointNotFound distinguishes a missing owner-scoped endpoint
// from a missing source attempt while preserving the shared not-found class.
var ErrWebhookEndpointNotFound = fmt.Errorf("webhook endpoint not found: %w", ErrNotFound)

// CreateWebhookEndpoint mints a new endpoint row (id.New(id.Webhook)) and
// inserts it. The secret is minted by the caller (internal/webhooks owns the
// format) and returned on this one response only. Name is already validated by
// the service and is stored verbatim.
//
// The count check and the insert share ONE transaction, and the count takes a
// row-level lock over the workspace's existing endpoints, so two concurrent
// creates at the boundary cannot both observe "one slot left" and both take it.
func (s *PGStore) CreateWebhookEndpoint(ctx context.Context, tenantID, name, url, secret string, eventTypes []string, enabled bool, createdBy string) (WebhookEndpoint, error) {
	e := WebhookEndpoint{
		ID: ids.New(ids.Webhook), TenantID: tenantID, Name: name, URL: url,
		Secret: secret, EventTypes: eventTypes, Enabled: enabled, CreatedBy: createdBy,
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// A transaction-scoped advisory lock keyed on the workspace serializes
		// concurrent creates for THIS tenant against each other, without blocking
		// any other workspace. FOR UPDATE alone (codex F8) locked only the
		// endpoint rows the count returned — an empty or sub-limit workspace has no
		// row to lock, so two parallel creates both read the same count and both
		// insert, overflowing the cap. The advisory lock has a stable target
		// (hashtext of the tenant id) that exists even with zero rows, so the
		// second create blocks until the first commits and re-reads the true count.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM webhook_endpoints WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
			return err
		}
		if n >= MaxWebhookEndpointsPerWorkspace {
			return ErrWebhookEndpointLimit
		}
		if len(url) > MaxWebhookURLBytes {
			return fmt.Errorf("%w: webhook url exceeds %d bytes", ErrInvalid, MaxWebhookURLBytes)
		}
		return tx.QueryRow(ctx,
			`INSERT INTO webhook_endpoints (id, tenant_id, name, url, secret, event_types, enabled, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING created_at, updated_at`,
			e.ID, e.TenantID, e.Name, e.URL, e.Secret, e.EventTypes, e.Enabled, e.CreatedBy,
		).Scan(&e.CreatedAt, &e.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, ErrWebhookEndpointLimit) {
			return WebhookEndpoint{}, err
		}
		return WebhookEndpoint{}, classify("webhook endpoint", err)
	}
	return e, nil
}

// ListWebhookEndpoints returns the requested workspaces' endpoints, newest
// first, keyset-paged on immutable (created_at,id); secrets are omitted.
func (s *PGStore) ListWebhookEndpoints(ctx context.Context, tenantIDs []string, afterAt time.Time, afterKey string, limit int) ([]WebhookEndpoint, error) {
	limit = core.PageLimitOrAbsent(limit)
	rows, err := s.Pool.Query(ctx,
		`SELECT e.id, e.tenant_id, e.name, e.url, e.event_types, e.enabled,
		        e.disabled_reason, e.created_by, e.created_at, e.updated_at,
		        COALESCE(latest.status, ''), latest.sent_at,
		        COALESCE(latest.parent_status, '')
		 FROM webhook_endpoints e
		 LEFT JOIN LATERAL (
		   SELECT a.status, a.sent_at,
		          CASE
		            WHEN EXISTS (
		              SELECT 1 FROM webhook_delivery_attempts pending
		              WHERE pending.notification_id = d.id AND pending.status = 'pending'
		            ) THEN 'pending'
		            WHEN d.delivered_at IS NOT NULL THEN 'delivered'
		            WHEN d.failed_at IS NOT NULL THEN 'failed'
		            ELSE 'pending'
		          END AS parent_status
		   FROM webhook_delivery_attempts a
		   JOIN webhook_deliveries d ON d.id = a.notification_id
		   WHERE a.endpoint_id = e.id AND a.sent_at IS NOT NULL
		   ORDER BY a.sent_at DESC, a.id DESC
		   LIMIT 1
		 ) latest ON true
		 WHERE e.tenant_id = ANY($1)
		   AND ($2::timestamptz IS NULL OR (e.created_at, e.id) < ($2, $3))
		 ORDER BY e.created_at DESC, e.id DESC
		 LIMIT $4`, tenantIDs, nullTime(afterAt), afterKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookEndpoint
	for rows.Next() {
		e, err := scanWebhookEndpointWithLatest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetWebhookEndpoint returns one endpoint (secret omitted), scoped to
// tenantID so a caller can never fetch another workspace's row by guessing
// its id.
func (s *PGStore) GetWebhookEndpoint(ctx context.Context, tenantID, id string) (WebhookEndpoint, error) {
	e, err := scanWebhookEndpoint(s.Pool.QueryRow(ctx,
		`SELECT `+webhookEndpointColumns+` FROM webhook_endpoints WHERE id = $1 AND tenant_id = $2`, id, tenantID))
	if err != nil {
		return WebhookEndpoint{}, classify("webhook endpoint", err)
	}
	return e, nil
}

// SetWebhookEndpointEnabled flips an endpoint's enabled flag (the caller's
// manual toggle — also how an auto-disabled endpoint is re-armed). Enabling
// clears any disabled reason; disabling records the caller's.
func (s *PGStore) SetWebhookEndpointEnabled(ctx context.Context, tenantID, id string, enabled bool, reason string) (WebhookEndpoint, error) {
	if enabled {
		reason = ""
	}
	e, err := scanWebhookEndpoint(s.Pool.QueryRow(ctx,
		`UPDATE webhook_endpoints SET enabled = $3, disabled_reason = $4, updated_at = now(),
		     notified_at = CASE WHEN $3 THEN NULL ELSE notified_at END
		 WHERE id = $1 AND tenant_id = $2 RETURNING `+webhookEndpointColumns, id, tenantID, enabled, reason))
	if err != nil {
		return WebhookEndpoint{}, classify("webhook endpoint", err)
	}
	return e, nil
}

// DeleteWebhookEndpoint removes an endpoint (its deliveries cascade).
func (s *PGStore) DeleteWebhookEndpoint(ctx context.Context, tenantID, id string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM webhook_endpoints WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook endpoint: %w", ErrNotFound)
	}
	return nil
}

// UpdateWebhookEndpoint replaces an endpoint's mutable fields (name, url,
// event_types, enabled) in one SQL round trip. Re-enabling an endpoint clears
// any disabled_reason; disabling leaves it unchanged (use SetWebhookEndpointEnabled
// for an explicit reason). Secret is immutable after creation.
func (s *PGStore) UpdateWebhookEndpoint(ctx context.Context, tenantID, id, name, url string, eventTypes []string, enabled bool) (WebhookEndpoint, error) {
	if len(url) > MaxWebhookURLBytes {
		return WebhookEndpoint{}, fmt.Errorf("%w: webhook url exceeds %d bytes", ErrInvalid, MaxWebhookURLBytes)
	}
	e, err := scanWebhookEndpoint(s.Pool.QueryRow(ctx,
		`UPDATE webhook_endpoints
		 SET name = $3, url = $4, event_types = $5, enabled = $6,
		     disabled_reason = CASE WHEN $6 THEN '' ELSE disabled_reason END,
		     notified_at = CASE WHEN $6 THEN NULL ELSE notified_at END,
		     updated_at = now()
		 WHERE id = $1 AND tenant_id = $2
		 RETURNING `+webhookEndpointColumns, id, tenantID, name, url, eventTypes, enabled))
	if err != nil {
		return WebhookEndpoint{}, classify("webhook endpoint", err)
	}
	return e, nil
}

// ListEnabledWebhookEndpoints returns every enabled endpoint platform-wide
// for the dispatcher's subscription table, refreshed each poll pass.
//
// Only the columns dispatch needs are selected (id, tenant, event types,
// created_at). The destination URL is loaded later, per due delivery, so a
// large catalog cannot pin unbounded URL text in the shared API process
// (codex round-15 #2).
func (s *PGStore) ListEnabledWebhookEndpoints(ctx context.Context) ([]WebhookEndpoint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, tenant_id, event_types, created_at FROM webhook_endpoints WHERE enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookEndpoint
	for rows.Next() {
		var e WebhookEndpoint
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventTypes, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// WebhookDelivery is the logical parent row for one event × one subscribed
// endpoint. Immutable send evidence lives in WebhookAttempt child rows.
type WebhookDelivery struct {
	ID              string
	EndpointID      string
	EventID         string
	EventType       string
	ServiceID       string
	Payload         string
	AttemptCount    int
	LastStatus      int
	LastError       string
	ResponseBody    string
	NextAttemptAt   time.Time
	SentAt          *time.Time
	LastAttemptedAt *time.Time
	DeliveredAt     *time.Time
	FailedAt        *time.Time
	CreatedAt       time.Time
}

// WebhookEnqueueResult is aggregate, bounded evidence from one dispatcher
// commit. It deliberately carries no workspace, endpoint, event, URL, or
// payload dimension: callers may log/meter it without turning tenant-controlled
// values into an observability cardinality or confidentiality problem.
type WebhookEnqueueResult struct {
	Admitted     int
	Capped       int
	Deduplicated int
}

const (
	WebhookAttemptPending   = "pending"
	WebhookAttemptDelivered = "delivered"
	WebhookAttemptFailed    = "failed"

	WebhookAttemptAutomatic = "automatic"
	WebhookAttemptManual    = "manual"
)

// WebhookAttempt is one scheduled send and its immutable terminal evidence.
// NotificationID identifies the logical endpoint/event parent. Pending rows
// reserve an ID before a send; the worker fills SentAt and outcome exactly once.
// Payload is joined from the parent so evidence never copies request bytes.
type WebhookAttempt struct {
	ID             string
	NotificationID string
	EndpointID     string
	EventID        string
	EventType      string
	ServiceID      string
	AttemptNumber  int
	Status         string
	StatusCode     int
	TransportError string
	ResponseBody   string
	Payload        string
	SentAt         *time.Time
	Origin         string
	RequestedBy    string
	IdempotencyKey string
	ParentStatus   string
	NextAttemptAt  *time.Time
	// ResumeAutomaticAt is internal queue state: a manual reservation parks an
	// unsent automatic retry here and restores it only if the manual send fails.
	ResumeAutomaticAt *time.Time
	CreatedAt         time.Time
}

// WebhookAttemptFilter is one bounded immutable-attempt history query.
type WebhookAttemptFilter struct {
	EndpointID string
	SentAfter  time.Time
	SentBefore time.Time
	Status     string
	AfterAt    time.Time
	AfterKey   string
	Limit      int
}

const webhookAttemptProjection = `
    a.id, a.notification_id, a.endpoint_id,
    d.event_id, d.event_type, d.service_id,
    a.attempt_number, a.status, a.status_code, a.transport_error,
    a.response_body, d.payload, a.sent_at, a.origin, a.requested_by,
    a.idempotency_key,
    CASE
        WHEN EXISTS (
            SELECT 1 FROM webhook_delivery_attempts pending
            WHERE pending.notification_id = d.id AND pending.status = '` + WebhookAttemptPending + `'
        ) THEN '` + WebhookAttemptPending + `'
        WHEN d.delivered_at IS NOT NULL THEN '` + WebhookAttemptDelivered + `'
        WHEN d.failed_at IS NOT NULL THEN '` + WebhookAttemptFailed + `'
        ELSE '` + WebhookAttemptPending + `'
    END,
    (
        SELECT min(pending.available_at) FROM webhook_delivery_attempts pending
        WHERE pending.notification_id = d.id AND pending.status = '` + WebhookAttemptPending + `'
    ),
    a.resume_automatic_at, a.created_at`

func scanWebhookAttempt(row pgx.Row) (WebhookAttempt, error) {
	var a WebhookAttempt
	err := row.Scan(
		&a.ID, &a.NotificationID, &a.EndpointID,
		&a.EventID, &a.EventType, &a.ServiceID,
		&a.AttemptNumber, &a.Status, &a.StatusCode, &a.TransportError,
		&a.ResponseBody, &a.Payload, &a.SentAt, &a.Origin, &a.RequestedBy,
		&a.IdempotencyKey, &a.ParentStatus, &a.NextAttemptAt,
		&a.ResumeAutomaticAt, &a.CreatedAt,
	)
	return a, err
}

// ListWebhookAttempts returns one endpoint's completed network exchanges,
// newest first, keyset-paged on immutable (sent_at,id). Unsent reservations do
// not pretend to have exchange evidence; QueueWebhookResend returns its pending
// reservation directly and it joins history after the worker terminalizes it.
func (s *PGStore) ListWebhookAttempts(ctx context.Context, filter WebhookAttemptFilter) ([]WebhookAttempt, error) {
	filter.Limit = core.PageLimitOrAbsent(filter.Limit)
	rows, err := s.Pool.Query(ctx,
		`SELECT `+webhookAttemptProjection+`
		 FROM webhook_delivery_attempts a
		 JOIN webhook_deliveries d ON d.id = a.notification_id
		 WHERE a.endpoint_id = $1
		   AND a.sent_at IS NOT NULL
		   AND ($2::timestamptz IS NULL OR a.sent_at > $2)
		   AND ($3::timestamptz IS NULL OR a.sent_at < $3)
		   AND ($4 = '' OR a.status = $4)
		   AND ($5::timestamptz IS NULL OR (a.sent_at, a.id) < ($5, $6))
		 ORDER BY a.sent_at DESC, a.id DESC
		 LIMIT $7`,
		filter.EndpointID, nullTime(filter.SentAfter), nullTime(filter.SentBefore), filter.Status,
		nullTime(filter.AfterAt), filter.AfterKey, filter.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookAttempt
	for rows.Next() {
		a, err := scanWebhookAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func getWebhookAttempt(ctx context.Context, q queryRower, attemptID string) (WebhookAttempt, error) {
	return scanWebhookAttempt(q.QueryRow(ctx,
		`SELECT `+webhookAttemptProjection+`
		 FROM webhook_delivery_attempts a
		 JOIN webhook_deliveries d ON d.id = a.notification_id
		 WHERE a.id = $1`, attemptID))
}

// WebhookResendRequest names the several identities at the store seam so owner,
// endpoint, source-attempt, and caller IDs cannot be accidentally transposed.
type WebhookResendRequest struct {
	TenantID        string
	EndpointID      string
	SourceAttemptID string
	RequestedBy     string
	IdempotencyKey  string
	RequestedAt     time.Time
}

// QueueWebhookResend reserves one immediate manual attempt for a source
// attempt, owner-scoped through its endpoint. The caller key is durable: a
// repeat returns the same pending or completed attempt forever. A scheduled,
// unsent automatic retry is parked on the manual row and restored only when
// that manual exchange fails, so Resend neither consumes nor races the normal
// retry budget.
func (s *PGStore) QueueWebhookResend(ctx context.Context, request WebhookResendRequest) (WebhookAttempt, error) {
	if request.IdempotencyKey == "" || request.RequestedBy == "" {
		return WebhookAttempt{}, fmt.Errorf("webhook resend identity and idempotency key are required: %w", ErrConflict)
	}
	reservedID := ids.New(ids.WebhookDelivery)
	var out WebhookAttempt
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var enabled bool
		if err := tx.QueryRow(ctx,
			`SELECT enabled FROM webhook_endpoints
			 WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, request.EndpointID, request.TenantID).Scan(&enabled); errors.Is(err, pgx.ErrNoRows) {
			return ErrWebhookEndpointNotFound
		} else if err != nil {
			return err
		}

		// The endpoint row lock serializes every resend for this endpoint. Check
		// idempotency before enabled state so an exact repeat can retrieve the
		// already-authorized result even if the endpoint was disabled afterwards.
		var existingID string
		err := tx.QueryRow(ctx,
			`SELECT id FROM webhook_delivery_attempts
			 WHERE endpoint_id = $1 AND origin = $2 AND idempotency_key = $3`,
			request.EndpointID, WebhookAttemptManual, request.IdempotencyKey).Scan(&existingID)
		switch {
		case err == nil:
			var getErr error
			out, getErr = getWebhookAttempt(ctx, tx, existingID)
			return getErr
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		if !enabled {
			return ErrWebhookEndpointDisabled
		}

		var notificationID string
		if err := tx.QueryRow(ctx,
			`SELECT notification_id FROM webhook_delivery_attempts
			 WHERE id = $1 AND endpoint_id = $2 AND sent_at IS NOT NULL`, request.SourceAttemptID, request.EndpointID).Scan(&notificationID); err != nil {
			return err
		}
		// Lock the logical notification so queue/complete operations cannot both
		// manipulate its one pending send slot.
		var lockedID string
		if err := tx.QueryRow(ctx,
			`SELECT id FROM webhook_deliveries
			 WHERE id = $1 AND endpoint_id = $2 FOR UPDATE`, notificationID, request.EndpointID).Scan(&lockedID); err != nil {
			return err
		}

		var manualPending bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (
			    SELECT 1 FROM webhook_delivery_attempts
			    WHERE notification_id = $1 AND status = $2 AND origin = $3
			)`, notificationID, WebhookAttemptPending, WebhookAttemptManual).Scan(&manualPending); err != nil {
			return err
		}
		if manualPending {
			return ErrWebhookAttemptPending
		}

		var resumeAt *time.Time
		if err := tx.QueryRow(ctx,
			`WITH deleted AS (
			    DELETE FROM webhook_delivery_attempts
			    WHERE notification_id = $1 AND status = $2 AND origin = $3
			      AND (lease_until IS NULL OR lease_until <= $4)
			    RETURNING available_at
			 )
			 SELECT min(available_at) FROM deleted`,
			notificationID, WebhookAttemptPending, WebhookAttemptAutomatic, request.RequestedAt).Scan(&resumeAt); err != nil {
			return err
		}
		// The DELETE predicate is rechecked after any concurrent child-row lock is
		// released. If ClaimDue leased the reservation between our inspection and
		// delete, it remains present and the replay must not race a POST whose
		// evidence row the worker already holds.
		var automaticPending bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (
			    SELECT 1 FROM webhook_delivery_attempts
			    WHERE notification_id = $1 AND status = $2 AND origin = $3
			)`, notificationID, WebhookAttemptPending, WebhookAttemptAutomatic).Scan(&automaticPending); err != nil {
			return err
		}
		if automaticPending {
			return ErrWebhookAttemptPending
		}

		var attemptNumber int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(max(attempt_number), 0) + 1
			 FROM webhook_delivery_attempts WHERE notification_id = $1`, notificationID).Scan(&attemptNumber); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO webhook_delivery_attempts (
			     id, notification_id, endpoint_id, attempt_number, status, origin,
			     requested_by, idempotency_key, available_at, resume_automatic_at, created_at
			 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $9)`,
			reservedID, notificationID, request.EndpointID, attemptNumber, WebhookAttemptPending,
			WebhookAttemptManual, request.RequestedBy, request.IdempotencyKey, request.RequestedAt, resumeAt); err != nil {
			return err
		}
		var getErr error
		out, getErr = getWebhookAttempt(ctx, tx, reservedID)
		return getErr
	})
	if err != nil {
		if errors.Is(err, ErrWebhookEndpointNotFound) || errors.Is(err, ErrWebhookEndpointDisabled) || errors.Is(err, ErrWebhookAttemptPending) {
			return WebhookAttempt{}, err
		}
		return WebhookAttempt{}, classify("webhook attempt", err)
	}
	return out, nil
}

// EnsureWebhookWatermark seeds the dispatcher's watermark at `at` if none
// exists yet and returns the current one. Seeding at first start (rather than
// zero) is what keeps the feature from replaying every event from before it
// existed.
func (s *PGStore) EnsureWebhookWatermark(ctx context.Context, at time.Time) (time.Time, string, error) {
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO webhook_watermark (id, at, key) VALUES (true, $1, '') ON CONFLICT (id) DO NOTHING`, at); err != nil {
		return time.Time{}, "", err
	}
	var wmAt time.Time
	var wmKey string
	if err := s.Pool.QueryRow(ctx, `SELECT at, key FROM webhook_watermark WHERE id`).Scan(&wmAt, &wmKey); err != nil {
		return time.Time{}, "", err
	}
	return wmAt, wmKey, nil
}

// FeedCommitLag is how far behind now a tailer of ListWebhookEvents must read.
// The feed's timestamps are assigned before their rows commit (Go's clock for
// audit rows, the statement's now() for deploys), so a row can appear under an
// already-advanced watermark; reading only rows older than this leaves in-flight
// commits time to land. A transaction slower than this can still slip under —
// accepted, documented, and bounded (audit inserts time out at 2s,
// core/audit.go).
//
// It lives with the query rather than with either consumer because it is a
// property of THIS feed's write path, not a per-consumer tunable: every tailer
// of ListWebhookEvents (internal/webhooks, internal/notifications) is exposed to
// the same in-flight commits and must use the same lag. Page size and park
// interval are genuinely per-consumer and stay in their own packages.
const FeedCommitLag = 3 * time.Second

// WebhookEventRow is one workspace-attributed row of the dispatcher's
// composed feed — the same projection ServiceEventRow carries, plus the
// tenant and service identity the join contributes (which the per-service
// feed takes as parameters instead). ServiceID is the PUBLIC service id — the
// projected CR name "<tenantName>-<appName>" (core.CRName), what
// GET /v1/services/{id} accepts — so a webhook receiver can turn a payload
// straight into an API call; ServiceName is the human label that service is
// shown under everywhere else — its displayName once renamed, else the
// immutable name (appDisplayLabel), so a payload never reports a stale
// creation-time name. Datastore rows carry the audit row's own target_name.
type WebhookEventRow struct {
	// CursorAt is when the source row became dispatch-visible. It normally equals
	// At; late-persisted observed facts use service_event_facts.recorded_at so an
	// older occurrence timestamp cannot fall behind the durable watermark.
	CursorAt    time.Time
	Key         string
	At          time.Time
	TenantID    string
	ServiceID   string
	ServiceName string
	Source      string // EventSourceDeploy | EventSourceAudit | EventSourceFact
	Phase       string // deploy rows: EventPhaseStarted | EventPhaseEnded
	DeployID    string
	// Status is the deploy's terminal status on the ended phase, and the fact's
	// own status on fact rows. Empty elsewhere.
	Status string
	Verb   string // audit rows: e.g. "apps.Suspend"
	// AutoDeployEnabled discriminates apps.SetAutoDeploy into Render's enabled
	// and disabled event types. nil on every other row (and on legacy audit rows).
	AutoDeployEnabled *bool
	FactType          string // fact rows: closed service_event_facts.fact_type
	// AppID is the app's internal control-plane id, as opposed to ServiceID's
	// public composite. Empty on the datastore audit arm, which has no app.
	AppID string
}

// appDisplayLabel is the SQL spelling of renderServiceName (internal/apps): the
// feed joins apps at dispatch time and has no CR to read spec.displayName off,
// so it resolves the label from apps.display_name, the projection
// SetDisplayName mirrors there (w6/m101).
const appDisplayLabel = `COALESCE(NULLIF(a.display_name, ''), a.name)`

// webhookEventsQuery is serviceEventsQuery's workspace-wide, ascending twin
// (see events.go for the composition's rationale). Differences, each forced
// by the dispatcher's shape:
//
//   - JOIN apps: the per-service query takes appID/target/ownerWorkspace as
//     parameters; here every service's transitions stream through one cursor,
//     so the join recovers each row's tenant + app name. The audit arm joins
//     on the target name scoped to the app's own tenant (or the default
//     workspace — the same two-workspace set the per-service feed allows, and
//     with the same caveat: a default-workspace caller's write on a name two
//     tenants share attributes to both, exactly as it appears in both their
//     per-service feeds).
//   - Datastore audit rows need no control-plane join: their typed target holds
//     the immutable dpg-/red- id and target_name holds the display name. The
//     observed datastore facts (w3/m82) join nothing for the same reason — the
//     fact row already carries both its workspace and that same typed id.
//   - the target matches every supported service spelling: the current CR name
//     "<tenantID>-<appName>", the legacy "<tenantName>-<appName>", or the bare
//     app-name fallback. The per-service feed is handed the caller's spelling;
//     this workspace-wide query must recognize all of them itself.
//   - ascending keyset from the watermark, bounded above by $3 (now minus the
//     dispatcher's safety lag, so a row committed slightly out of timestamp
//     order can't be skipped forever by an already-advanced watermark).
//   - $5 restricts every arm to the tenants that currently have an enabled
//     endpoint — the dispatcher knows the subscriber set, so unsubscribed
//     tenants' transitions never leave Postgres at all.
const webhookEventsQuery = `
WITH feed AS (
    SELECT d.created_at                        AS cursor_at,
           d.id || ':` + EventPhaseStarted + `' AS key,
           d.created_at                        AS at,
           a.tenant_id                         AS tenant_id,
           t.name || '-' || a.name             AS service_id,
           ` + appDisplayLabel + `             AS service_name,
           '` + EventSourceDeploy + `'::text   AS source,
           '` + EventPhaseStarted + `'::text   AS phase,
           d.id                                AS deploy_id,
           ''::text                            AS status,
           ''::text                            AS verb,
           ''::text                            AS fact_type,
	       NULL::boolean                       AS auto_deploy_enabled,
           a.id                                AS app_id
    FROM deploys d
    JOIN apps a ON a.id = d.app_id
    JOIN tenants t ON t.id = a.tenant_id
    WHERE a.tenant_id = ANY($5)
  UNION ALL
    SELECT d.finished_at,
           d.id || ':` + EventPhaseEnded + `',
           d.finished_at,
           a.tenant_id,
           t.name || '-' || a.name,
           ` + appDisplayLabel + `,
           '` + EventSourceDeploy + `'::text,
           '` + EventPhaseEnded + `'::text,
           d.id,
           d.status,
           ''::text,
           ''::text,
	       NULL::boolean,
           a.id
    FROM deploys d
    JOIN apps a ON a.id = d.app_id
    JOIN tenants t ON t.id = a.tenant_id
    WHERE d.finished_at IS NOT NULL AND a.tenant_id = ANY($5)
  UNION ALL
    SELECT e.at,
           e.id || ':',
           e.at,
           a.tenant_id,
           t.name || '-' || a.name,
           ` + appDisplayLabel + `,
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
           e.verb,
           ''::text,
	       e.auto_deploy_enabled,
           a.id
    FROM audit_events e
    JOIN apps a ON a.tenant_id = ANY($5)
     AND (e.workspace_id = a.tenant_id OR e.workspace_id = '` + core.DefaultTenant + `')
    JOIN tenants t ON t.id = a.tenant_id
     AND e.target IN (
         'service:' || a.tenant_id || '-' || a.name,
         'service:' || t.name || '-' || a.name,
         'service:' || a.name
     )
    WHERE e.outcome = 'allowed' AND e.verb = ANY($4)
  UNION ALL
    SELECT e.at,
           e.id || ':',
           e.at,
           e.workspace_id,
           split_part(e.target, ':', 2),
		   COALESCE(NULLIF(e.target_name, ''), split_part(e.target, ':', 2)),
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
		   e.verb,
		   ''::text,
		   NULL::boolean,
		   ''::text
    FROM audit_events e
	WHERE e.workspace_id = ANY($5)
	  AND e.outcome = 'allowed'
	  AND e.verb = ANY($4)
	  AND (e.target LIKE 'database:%' OR e.target LIKE 'keyvalue:%')
  UNION ALL
    SELECT f.recorded_at,
           'fact:' || f.source_key,
           f.at,
           a.tenant_id,
           t.name || '-' || a.name,
           ` + appDisplayLabel + `,
           '` + EventSourceFact + `'::text,
           ''::text,
           f.deploy_id,
           COALESCE(f.status, ''),
           ''::text,
           f.fact_type,
	       NULL::boolean,
           a.id
    FROM service_event_facts f
    JOIN apps a ON a.id = f.app_id
    JOIN tenants t ON t.id = a.tenant_id
    WHERE a.tenant_id = ANY($5)
  UNION ALL
    SELECT f.recorded_at,
           'fact:' || f.source_key,
           f.at,
           f.workspace_id,
           f.datastore_id,
           f.datastore_id,
           '` + EventSourceFact + `'::text,
           ''::text,
           ''::text,
           ''::text,
           ''::text,
           f.fact_type,
	       NULL::boolean,
           ''::text
    FROM datastore_event_facts f
    WHERE f.workspace_id = ANY($5)
)
SELECT cursor_at, key, at, tenant_id, service_id, service_name, source, phase, deploy_id, status, verb, fact_type, auto_deploy_enabled, app_id
FROM feed
WHERE (cursor_at, key) > ($1, $2) AND cursor_at <= $3
ORDER BY cursor_at, key
LIMIT $6`

// ListWebhookEvents returns the composed feed strictly after the watermark
// (afterAt, afterKey), up to and including `until`, oldest first, restricted
// to `tenants` (the subscriber set). verbs is the audit-verb subset the
// webhook vocabulary maps (internal/webhooks owns the mapping, exactly as
// internal/events owns its own).
func (s *PGStore) ListWebhookEvents(ctx context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]WebhookEventRow, error) {
	rows, err := s.Pool.Query(ctx, webhookEventsQuery, afterAt, afterKey, until, verbs, tenants, limit)
	if err != nil {
		return nil, fmt.Errorf("list webhook events: %w", err)
	}
	defer rows.Close()
	var out []WebhookEventRow
	for rows.Next() {
		var r WebhookEventRow
		if err := rows.Scan(&r.CursorAt, &r.Key, &r.At, &r.TenantID, &r.ServiceID, &r.ServiceName, &r.Source, &r.Phase, &r.DeployID, &r.Status, &r.Verb, &r.FactType, &r.AutoDeployEnabled, &r.AppID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EnqueueWebhookDeliveries inserts a batch of pending deliveries and advances
// the watermark to (at, key) in one transaction — so a crash between the two
// can't drop events (re-reading the same rows re-enqueues them) or skip them.
// Called with an empty batch to advance the watermark past events nobody
// subscribes to. maxPerWorkspace == 0 explicitly disables the ceiling.
//
// Every workspace represented in the batch is locked in sorted order with a
// transaction-scoped advisory lock before its open count is read. Concurrent
// dispatch replicas therefore cannot both observe the same last slot, while
// unrelated workspaces remain independent. Rows rejected by the ceiling are
// intentionally NOT errors: the watermark advances in the same transaction so
// notification pressure can never roll back or replay the source mutation.
func (s *PGStore) EnqueueWebhookDeliveries(ctx context.Context, deliveries []WebhookDelivery, at time.Time, key string, maxPerWorkspace int) (WebhookEnqueueResult, error) {
	if maxPerWorkspace < 0 {
		maxPerWorkspace = 0
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return WebhookEnqueueResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	result := WebhookEnqueueResult{}
	if len(deliveries) > 0 {
		ids := make([]string, 0, len(deliveries))
		endpointIDs := make([]string, 0, len(deliveries))
		eventIDs := make([]string, 0, len(deliveries))
		eventTypes := make([]string, 0, len(deliveries))
		serviceIDs := make([]string, 0, len(deliveries))
		payloads := make([]string, 0, len(deliveries))
		nextAttemptAts := make([]time.Time, 0, len(deliveries))
		for _, d := range deliveries {
			ids = append(ids, d.ID)
			endpointIDs = append(endpointIDs, d.EndpointID)
			eventIDs = append(eventIDs, d.EventID)
			eventTypes = append(eventTypes, d.EventType)
			serviceIDs = append(serviceIDs, d.ServiceID)
			payloads = append(payloads, d.Payload)
			nextAttemptAts = append(nextAttemptAts, d.NextAttemptAt)
		}

		if maxPerWorkspace == 0 {
			var inputCount, insertedCount int
			err = tx.QueryRow(ctx,
				`WITH input AS (
				   SELECT *
				     FROM unnest(
				       $1::text[], $2::text[], $3::text[], $4::text[],
				       $5::text[], $6::text[], $7::timestamptz[]
				     ) WITH ORDINALITY AS item(
				       id, endpoint_id, event_id, event_type, service_id, payload,
				       next_attempt_at, ordinal
				     )
				 ), inserted AS (
				   INSERT INTO webhook_deliveries (
				     id, endpoint_id, event_id, event_type, service_id, payload,
				     next_attempt_at
				   )
				   SELECT id, endpoint_id, event_id, event_type, service_id, payload,
				          next_attempt_at
				     FROM input
				    ORDER BY ordinal
				   ON CONFLICT DO NOTHING
				   RETURNING id
				 )
				 SELECT (SELECT count(*) FROM input),
				        (SELECT count(*) FROM inserted)`,
				ids, endpointIDs, eventIDs, eventTypes, serviceIDs, payloads,
				nextAttemptAts,
			).Scan(&inputCount, &insertedCount)
			if err != nil {
				return WebhookEnqueueResult{}, classify("webhook delivery", err)
			}
			result.Admitted = insertedCount
			result.Deduplicated = inputCount - insertedCount
		} else {
			// The prefix keeps this lock namespace distinct from endpoint-create and
			// other workspace-keyed advisory locks. MATERIALIZED + ORDER BY gives every
			// multi-workspace transaction the same acquisition order, avoiding a
			// cross-tenant batch deadlock.
			if _, err := tx.Exec(ctx,
				`WITH tenants AS MATERIALIZED (
			   SELECT DISTINCT e.tenant_id
			     FROM unnest($1::text[]) AS input(endpoint_id)
			     JOIN webhook_endpoints e ON e.id = input.endpoint_id
			    ORDER BY e.tenant_id
			 )
			 SELECT pg_advisory_xact_lock(
			     hashtextextended('webhook-deliveries:' || tenant_id, 0)
			 )
			 FROM tenants
			 ORDER BY tenant_id`, endpointIDs); err != nil {
				return WebhookEnqueueResult{}, err
			}

			var inputCount, candidateCount, admittedCount, insertedCount int
			err = tx.QueryRow(ctx,
				`WITH input AS (
			   SELECT *
			     FROM unnest(
			       $1::text[], $2::text[], $3::text[], $4::text[],
			       $5::text[], $6::text[], $7::timestamptz[]
			     ) WITH ORDINALITY AS item(
			       id, endpoint_id, event_id, event_type, service_id, payload,
			       next_attempt_at, ordinal
			     )
			 ), unique_input AS (
			   SELECT DISTINCT ON (endpoint_id, event_id) *
			     FROM input
			    ORDER BY endpoint_id, event_id, ordinal
			 ), candidates AS (
			   SELECT input.*, endpoint.tenant_id
			     FROM unique_input input
			     JOIN webhook_endpoints endpoint ON endpoint.id = input.endpoint_id
			    WHERE NOT EXISTS (
			      SELECT 1 FROM webhook_deliveries existing
			       WHERE existing.endpoint_id = input.endpoint_id
			         AND existing.event_id = input.event_id
			    )
			 ), open_counts AS (
			   SELECT endpoint.tenant_id, count(*) AS open_count
			     FROM webhook_deliveries delivery
			     JOIN webhook_endpoints endpoint ON endpoint.id = delivery.endpoint_id
			    WHERE endpoint.tenant_id IN (SELECT DISTINCT tenant_id FROM candidates)
			      AND delivery.delivered_at IS NULL
			      AND delivery.failed_at IS NULL
			    GROUP BY endpoint.tenant_id
			 ), ranked AS (
			   SELECT candidates.*,
			          row_number() OVER (
			            PARTITION BY candidates.tenant_id ORDER BY candidates.ordinal
			          ) AS workspace_rank,
			          coalesce(open_counts.open_count, 0) AS open_count
			     FROM candidates
			     LEFT JOIN open_counts USING (tenant_id)
			 ), admitted AS (
			   SELECT * FROM ranked
			    WHERE workspace_rank <= greatest($8 - open_count, 0)
			 ), inserted AS (
			   INSERT INTO webhook_deliveries (
			     id, endpoint_id, event_id, event_type, service_id, payload,
			     next_attempt_at
			   )
			   SELECT id, endpoint_id, event_id, event_type, service_id, payload,
			          next_attempt_at
			     FROM admitted
			    ORDER BY ordinal
			   ON CONFLICT DO NOTHING
			   RETURNING id
			 )
			 SELECT (SELECT count(*) FROM input),
			        (SELECT count(*) FROM candidates),
			        (SELECT count(*) FROM admitted),
			        (SELECT count(*) FROM inserted)`,
				ids, endpointIDs, eventIDs, eventTypes, serviceIDs, payloads,
				nextAttemptAts, maxPerWorkspace,
			).Scan(&inputCount, &candidateCount, &admittedCount, &insertedCount)
			if err != nil {
				return WebhookEnqueueResult{}, classify("webhook delivery", err)
			}
			result.Admitted = insertedCount
			result.Capped = candidateCount - admittedCount
			result.Deduplicated = inputCount - candidateCount + admittedCount - insertedCount
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE webhook_watermark SET at = $1, key = $2 WHERE id AND (at, key) <> ($1, $2)`,
		at, key); err != nil {
		return WebhookEnqueueResult{}, classify("webhook delivery", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return WebhookEnqueueResult{}, err
	}
	return result, nil
}

// DueWebhookAttempt is one pending attempt joined with its immutable parent
// request and endpoint delivery capability. AutomaticAttemptCount is the
// completed automatic-send count used by the worker's fixed retry budget;
// manual attempts never increment it.
type DueWebhookAttempt struct {
	WebhookAttempt
	URL                   string
	Secret                string
	TenantID              string
	EndpointName          string
	CreatedBy             string
	AutomaticAttemptCount int
}

// ClaimDueWebhookAttempts leases disjoint pending reservations across worker
// replicas. The attempt identity already exists before the claim; a crashed
// worker leaves it pending and reclaimable after leaseUntil rather than
// inventing evidence for a network exchange that might never have happened.
func (s *PGStore) ClaimDueWebhookAttempts(ctx context.Context, now, leaseUntil time.Time, limit int) ([]DueWebhookAttempt, error) {
	rows, err := s.Pool.Query(ctx,
		`WITH eligible AS MATERIALIZED (
		     SELECT a.id, a.available_at, a.created_at, e.tenant_id,
		            row_number() OVER (
		              PARTITION BY e.tenant_id
		              ORDER BY a.available_at, a.created_at, a.id
		            ) AS workspace_rank
		       FROM webhook_delivery_attempts a
		       JOIN webhook_endpoints e ON e.id = a.endpoint_id
		      WHERE a.status = $3 AND a.available_at <= $1
		        AND (a.lease_until IS NULL OR a.lease_until <= $1)
		        AND e.enabled
		 ), due AS (
		     SELECT a.id
		       FROM webhook_delivery_attempts a
		       JOIN webhook_endpoints e ON e.id = a.endpoint_id
		       JOIN eligible fair ON fair.id = a.id
		      WHERE a.status = $3 AND a.available_at <= $1
		        AND (a.lease_until IS NULL OR a.lease_until <= $1)
		        AND e.enabled
		      ORDER BY fair.workspace_rank, fair.available_at, fair.created_at,
		               fair.tenant_id, fair.id
		      LIMIT $4
		      FOR UPDATE OF a SKIP LOCKED
		 ), claimed AS (
		     UPDATE webhook_delivery_attempts a
		     SET lease_until = $2
		     FROM due
		     WHERE a.id = due.id
		     RETURNING a.*
		 )
		 SELECT `+webhookAttemptProjection+`,
		        e.url, e.secret, e.tenant_id, e.name, e.created_by,
		        d.attempt_count
		 FROM claimed a
		 JOIN webhook_deliveries d ON d.id = a.notification_id
		 JOIN webhook_endpoints e ON e.id = a.endpoint_id
		 ORDER BY a.available_at, a.id`, now, leaseUntil, WebhookAttemptPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueWebhookAttempt
	for rows.Next() {
		var a DueWebhookAttempt
		if err := rows.Scan(
			&a.ID, &a.NotificationID, &a.EndpointID,
			&a.EventID, &a.EventType, &a.ServiceID,
			&a.AttemptNumber, &a.Status, &a.StatusCode, &a.TransportError,
			&a.ResponseBody, &a.Payload, &a.SentAt, &a.Origin, &a.RequestedBy,
			&a.IdempotencyKey, &a.ParentStatus, &a.NextAttemptAt,
			&a.ResumeAutomaticAt, &a.CreatedAt,
			&a.URL, &a.Secret, &a.TenantID, &a.EndpointName, &a.CreatedBy,
			&a.AutomaticAttemptCount,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// WebhookAttemptCompletion names terminal evidence and parent-transition
// controls that would otherwise be ambiguous adjacent values and booleans.
type WebhookAttemptCompletion struct {
	AttemptID      string
	NextAttemptID  string
	StatusCode     int
	TransportError string
	ResponseBody   string
	CompletedAt    time.Time
	NextAttemptAt  time.Time
	Delivered      bool
	Exhausted      bool
	DisableReason  string
}

// CompleteWebhookAttempt performs the pending -> terminal transition exactly
// once and updates the logical notification in the same transaction. A failed
// automatic send reserves its next retry atomically. A failed manual send
// restores the automatic reservation it parked, without consuming the parent's
// automatic attempt_count; manual success closes the notification and drops it.
func (s *PGStore) CompleteWebhookAttempt(ctx context.Context, completion WebhookAttemptCompletion) (bool, error) {
	completed := false
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// QueueWebhookResend takes the notification parent before inspecting or
		// deleting a pending child. Complete uses the identical parent -> attempt
		// lock order so a resend racing an in-flight completion cannot deadlock.
		var notificationID, endpointID string
		if err := tx.QueryRow(ctx,
			`SELECT notification_id, endpoint_id FROM webhook_delivery_attempts WHERE id = $1`, completion.AttemptID,
		).Scan(&notificationID, &endpointID); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		// QueueWebhookResend locks endpoint -> notification -> attempt. Every
		// completion uses the same order, and exhaustion can disable the endpoint
		// before commit without leaving an enabled replay race window.
		var lockedEndpoint string
		if err := tx.QueryRow(ctx,
			`SELECT id FROM webhook_endpoints WHERE id = $1 FOR UPDATE`, endpointID,
		).Scan(&lockedEndpoint); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		var lockedID string
		if err := tx.QueryRow(ctx,
			`SELECT id FROM webhook_deliveries WHERE id = $1 FOR UPDATE`, notificationID,
		).Scan(&lockedID); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}

		terminalStatus := WebhookAttemptFailed
		if completion.Delivered {
			terminalStatus = WebhookAttemptDelivered
		}
		var origin string
		var attemptNumber int
		var resumeAt *time.Time
		err := tx.QueryRow(ctx,
			`UPDATE webhook_delivery_attempts
			 SET status = $2, sent_at = $3, status_code = $4,
			     transport_error = $5, response_body = $6, lease_until = NULL
			 WHERE id = $1 AND status = $7
			 RETURNING attempt_number, origin, resume_automatic_at`,
			completion.AttemptID, terminalStatus, completion.CompletedAt, completion.StatusCode, completion.TransportError, completion.ResponseBody,
			WebhookAttemptPending,
		).Scan(&attemptNumber, &origin, &resumeAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		completed = true

		switch origin {
		case WebhookAttemptAutomatic:
			if _, err := tx.Exec(ctx,
				`UPDATE webhook_deliveries
				 SET attempt_count = attempt_count + 1,
				     last_status = $2, last_error = $3, response_body = $4,
				     sent_at = COALESCE(sent_at, $5), last_attempted_at = $5,
				     next_attempt_at = CASE WHEN $6 OR $7 THEN $5 ELSE $8 END,
				     delivered_at = CASE WHEN $6 THEN $5 ELSE NULL END,
				     failed_at = CASE WHEN $7 THEN $5 ELSE NULL END
				 WHERE id = $1`,
				notificationID, completion.StatusCode, completion.TransportError, completion.ResponseBody, completion.CompletedAt,
				completion.Delivered, completion.Exhausted, completion.NextAttemptAt); err != nil {
				return err
			}
			if !completion.Delivered && !completion.Exhausted {
				if completion.NextAttemptID == "" {
					return errors.New("next automatic webhook attempt id is required")
				}
				if _, err := tx.Exec(ctx,
					`INSERT INTO webhook_delivery_attempts (
					     id, notification_id, endpoint_id, attempt_number, status,
					     origin, available_at, created_at
					 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
					completion.NextAttemptID, notificationID, endpointID, attemptNumber+1,
					WebhookAttemptPending, WebhookAttemptAutomatic, completion.NextAttemptAt, completion.CompletedAt); err != nil {
					return err
				}
			}
			if completion.Exhausted {
				if completion.DisableReason == "" {
					return errors.New("webhook endpoint disable reason is required on exhaustion")
				}
				if _, err := tx.Exec(ctx,
					`UPDATE webhook_endpoints
					 SET enabled = false, disabled_reason = $2, updated_at = now()
					 WHERE id = $1`, endpointID, completion.DisableReason); err != nil {
					return err
				}
			}

		case WebhookAttemptManual:
			if _, err := tx.Exec(ctx,
				`UPDATE webhook_deliveries
				 SET last_status = $2, last_error = $3, response_body = $4,
				     sent_at = COALESCE(sent_at, $5), last_attempted_at = $5,
				     next_attempt_at = CASE WHEN $6 THEN $5 ELSE next_attempt_at END,
				     delivered_at = CASE WHEN $6 THEN $5 ELSE delivered_at END,
				     failed_at = CASE WHEN $6 THEN NULL ELSE failed_at END
				 WHERE id = $1`,
				notificationID, completion.StatusCode, completion.TransportError, completion.ResponseBody, completion.CompletedAt, completion.Delivered); err != nil {
				return err
			}
			if !completion.Delivered && resumeAt != nil {
				if completion.NextAttemptID == "" {
					return errors.New("resumed automatic webhook attempt id is required")
				}
				if _, err := tx.Exec(ctx,
					`INSERT INTO webhook_delivery_attempts (
					     id, notification_id, endpoint_id, attempt_number, status,
					     origin, available_at, created_at
					 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
					completion.NextAttemptID, notificationID, endpointID, attemptNumber+1,
					WebhookAttemptPending, WebhookAttemptAutomatic, *resumeAt, completion.CompletedAt); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx,
					`UPDATE webhook_deliveries
					 SET next_attempt_at = $2, delivered_at = NULL, failed_at = NULL
					 WHERE id = $1`, notificationID, *resumeAt); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unknown webhook attempt origin %q", origin)
		}
		return nil
	})
	if err != nil {
		return false, classify("webhook attempt", err)
	}
	return completed, nil
}

// ClaimWebhookFailureNotice compare-and-sets the endpoint's notified_at marker:
// it succeeds (returns true, records `now`) only when no notice has been sent
// since `threshold` (now − suppression window), so across replicas and restarts
// exactly one worker emails per window (w1/m58). NULL notified_at (never
// notified, or cleared on re-enable) always claims.
func (s *PGStore) ClaimWebhookFailureNotice(ctx context.Context, endpointID string, now, threshold time.Time) (bool, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE webhook_endpoints SET notified_at = $2
		 WHERE id = $1 AND (notified_at IS NULL OR notified_at <= $3)`,
		endpointID, now, threshold)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// SweepWebhookDeliveries purges reclaimable delivery rows (w1/m67 F3). The table
// is both the durable delivery QUEUE and the product's history surface, so before
// m67 nothing ever reclaimed a finished row: ordinary tenant activity grew shared
// table, index, and backup storage without bound.
//
// Terminal notifications (delivered_at or failed_at set, with no pending
// child) are purged with all attempts when either:
//
//   - older than `before`, so history has a finite lifetime; or
//   - beyond `keepPerEndpoint` most recent rows for their endpoint, so a burst
//     inside the age window cannot evade the age rule alone.
//
// keepPerEndpoint counts immutable attempts, not parents: a notification with
// eight retries consumes eight evidence slots. Whole-parent deletion prevents
// orphaning payloads or leaving a partial forensic sequence.
//
// Notifications whose newest pending reservation is older than `before` are
// abandoned and purged too. This includes a manual resend parked on a previously
// terminal parent; recent reservations on old source events remain safe.
//
// Deletion is bounded per call (`limit` for each of the two passes) and safe to
// run concurrently on two replicas — rows are claimed with FOR UPDATE SKIP LOCKED,
// so a second sweeper simply takes different rows. Returns the number deleted.
func (s *PGStore) SweepWebhookDeliveries(ctx context.Context, before time.Time, keepPerEndpoint, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	if keepPerEndpoint < 0 {
		keepPerEndpoint = 0
	}
	terminal, err := s.Pool.Exec(ctx,
		`WITH terminal_notifications AS (
		   SELECT d.id, d.endpoint_id, max(a.sent_at) AS latest_at, count(*) AS attempt_total
		     FROM webhook_deliveries d
		     JOIN webhook_delivery_attempts a ON a.notification_id = d.id
		    WHERE (d.delivered_at IS NOT NULL OR d.failed_at IS NOT NULL)
		      AND NOT EXISTS (
		          SELECT 1 FROM webhook_delivery_attempts pending
		          WHERE pending.notification_id = d.id AND pending.status = $4
		      )
		    GROUP BY d.id, d.endpoint_id
		 ), ranked AS (
		   SELECT id, latest_at,
		          sum(attempt_total) OVER (
		              PARTITION BY endpoint_id ORDER BY latest_at DESC, id DESC
		          ) AS retained_attempts
		     FROM terminal_notifications
		 ),
		 eligible AS (
		   SELECT id FROM ranked
		    WHERE retained_attempts > $2 OR latest_at < $1
		    ORDER BY latest_at
		    LIMIT $3
		    FOR UPDATE SKIP LOCKED
		 )
		 DELETE FROM webhook_deliveries d USING eligible e WHERE d.id = e.id`,
		before, keepPerEndpoint, limit, WebhookAttemptPending)
	if err != nil {
		return 0, classify("webhook deliveries", err)
	}
	abandoned, err := s.Pool.Exec(ctx,
		`WITH allow_pending_delete AS (
		   SELECT set_config('bex.webhook_pending_delete', 'on', true)
		 ), pending_age AS (
		   SELECT d.id, max(a.created_at) AS newest_reservation
		     FROM webhook_deliveries d
		     JOIN webhook_delivery_attempts a ON a.notification_id = d.id
		     CROSS JOIN allow_pending_delete
		    WHERE a.status = $3
		    GROUP BY d.id
		 ), eligible AS (
		   SELECT id FROM pending_age
		    WHERE newest_reservation < $1
		    ORDER BY newest_reservation
		    LIMIT $2
		    FOR UPDATE SKIP LOCKED
		 )
		 DELETE FROM webhook_deliveries d USING eligible e WHERE d.id = e.id`,
		before, limit, WebhookAttemptPending)
	if err != nil {
		return 0, classify("webhook deliveries", err)
	}
	return terminal.RowsAffected() + abandoned.RowsAffected(), nil
}
