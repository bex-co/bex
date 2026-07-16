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
}

// webhookEndpointColumns deliberately omits `secret` — see WebhookEndpoint.
const webhookEndpointColumns = `id, tenant_id, name, url, event_types, enabled, disabled_reason, created_by, created_at, updated_at`

func scanWebhookEndpoint(row pgx.Row) (WebhookEndpoint, error) {
	var e WebhookEndpoint
	err := row.Scan(&e.ID, &e.TenantID, &e.Name, &e.URL, &e.EventTypes, &e.Enabled, &e.DisabledReason, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// CreateWebhookEndpoint mints a new endpoint row (id.New(id.Webhook)) and
// inserts it. The secret is minted by the caller (internal/webhooks owns the
// format) and returned on this one response only. An empty name defaults to
// the URL.
func (s *PGStore) CreateWebhookEndpoint(ctx context.Context, tenantID, name, url, secret string, eventTypes []string, createdBy string) (WebhookEndpoint, error) {
	if name == "" {
		name = url
	}
	e := WebhookEndpoint{
		ID: ids.New(ids.Webhook), TenantID: tenantID, Name: name, URL: url,
		Secret: secret, EventTypes: eventTypes, Enabled: true, CreatedBy: createdBy,
	}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO webhook_endpoints (id, tenant_id, name, url, secret, event_types, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at`,
		e.ID, e.TenantID, e.Name, e.URL, e.Secret, e.EventTypes, e.CreatedBy,
	).Scan(&e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return WebhookEndpoint{}, classify("webhook endpoint", err)
	}
	return e, nil
}

// ListWebhookEndpoints returns a workspace's endpoints, newest first (secrets
// omitted).
func (s *PGStore) ListWebhookEndpoints(ctx context.Context, tenantID string) ([]WebhookEndpoint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+webhookEndpointColumns+` FROM webhook_endpoints WHERE tenant_id = $1 ORDER BY created_at DESC, id DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookEndpoint
	for rows.Next() {
		e, err := scanWebhookEndpoint(rows)
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
		`UPDATE webhook_endpoints SET enabled = $3, disabled_reason = $4, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2 RETURNING `+webhookEndpointColumns, id, tenantID, enabled, reason))
	if err != nil {
		return WebhookEndpoint{}, classify("webhook endpoint", err)
	}
	return e, nil
}

// DisableWebhookEndpoint is the delivery worker's auto-disable path — not
// tenant-scoped (the worker acts on the endpoint id it is delivering for, not
// on behalf of a caller).
func (s *PGStore) DisableWebhookEndpoint(ctx context.Context, id, reason string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE webhook_endpoints SET enabled = false, disabled_reason = $2, updated_at = now() WHERE id = $1`, id, reason)
	return err
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

// ListEnabledWebhookEndpoints returns every enabled endpoint platform-wide
// (secrets omitted) — the dispatcher's subscription table, refreshed each
// poll pass.
func (s *PGStore) ListEnabledWebhookEndpoints(ctx context.Context) ([]WebhookEndpoint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+webhookEndpointColumns+` FROM webhook_endpoints WHERE enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookEndpoint
	for rows.Next() {
		e, err := scanWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// WebhookDelivery is a row of `webhook_deliveries`: one event × one
// subscribed endpoint, retried in place until delivered, exhausted, or its
// endpoint is disabled.
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
	NextAttemptAt   time.Time
	LastAttemptedAt *time.Time
	DeliveredAt     *time.Time
	FailedAt        *time.Time
	CreatedAt       time.Time
}

const webhookDeliveryColumns = `id, endpoint_id, event_id, event_type, service_id, payload, attempt_count, last_status, last_error, next_attempt_at, last_attempted_at, delivered_at, failed_at, created_at`

func scanWebhookDelivery(row pgx.Row) (WebhookDelivery, error) {
	var d WebhookDelivery
	err := row.Scan(&d.ID, &d.EndpointID, &d.EventID, &d.EventType, &d.ServiceID, &d.Payload,
		&d.AttemptCount, &d.LastStatus, &d.LastError, &d.NextAttemptAt, &d.LastAttemptedAt,
		&d.DeliveredAt, &d.FailedAt, &d.CreatedAt)
	return d, err
}

// ListWebhookDeliveries returns one endpoint's delivery history, newest
// first, keyset-paged on (created_at, id) — the events-feed cursor shape, so
// a page stays stable however rows are updated in place by retries.
func (s *PGStore) ListWebhookDeliveries(ctx context.Context, endpointID string, afterAt time.Time, afterKey string, limit int) ([]WebhookDelivery, error) {
	if limit < 1 || limit > core.MaxPageLimit {
		limit = core.DefaultPageLimit
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT `+webhookDeliveryColumns+` FROM webhook_deliveries
		 WHERE endpoint_id = $1
		   AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $4`,
		endpointID, nullTime(afterAt), afterKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookDelivery
	for rows.Next() {
		d, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
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

// WebhookEventRow is one workspace-attributed row of the dispatcher's
// composed feed — the same projection ServiceEventRow carries, plus the
// tenant and service identity the join contributes (which the per-service
// feed takes as parameters instead). ServiceID is the PUBLIC service id — the
// projected CR name "<tenantName>-<appName>" (core.CRName), what
// GET /v1/services/{id} accepts — so a webhook receiver can turn a payload
// straight into an API call; ServiceName is the human app name.
type WebhookEventRow struct {
	Key         string
	At          time.Time
	TenantID    string
	ServiceID   string
	ServiceName string
	Source      string // EventSourceDeploy | EventSourceAudit
	Phase       string // deploy rows: EventPhaseStarted | EventPhaseEnded
	DeployID    string
	Status      string // the deploy's terminal status; the ended phase only
	Verb        string // audit rows: e.g. "apps.Suspend"
}

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
//   - the target matches EITHER of a service's two addressable spellings
//     (w4/m19, core.appCandidateNames): an audit row records the raw name the
//     caller passed (core.AuthorizeApp → ServiceTarget), which is normally the
//     full service id "<tenantName>-<appName>" (what list/create return) but
//     can be the bare app name when a caller addressed the App by its
//     LabelServiceName fallback. The per-service feed is handed the caller's
//     spelling as a parameter; this query must recognize both itself.
//   - ascending keyset from the watermark, bounded above by $3 (now minus the
//     dispatcher's safety lag, so a row committed slightly out of timestamp
//     order can't be skipped forever by an already-advanced watermark).
//   - $5 restricts every arm to the tenants that currently have an enabled
//     endpoint — the dispatcher knows the subscriber set, so unsubscribed
//     tenants' transitions never leave Postgres at all.
const webhookEventsQuery = `
WITH feed AS (
    SELECT d.id || ':` + EventPhaseStarted + `' AS key,
           d.created_at                        AS at,
           a.tenant_id                         AS tenant_id,
           t.name || '-' || a.name             AS service_id,
           a.name                              AS service_name,
           '` + EventSourceDeploy + `'::text   AS source,
           '` + EventPhaseStarted + `'::text   AS phase,
           d.id                                AS deploy_id,
           ''::text                            AS status,
           ''::text                            AS verb
    FROM deploys d
    JOIN apps a ON a.id = d.app_id
    JOIN tenants t ON t.id = a.tenant_id
    WHERE a.tenant_id = ANY($5)
  UNION ALL
    SELECT d.id || ':` + EventPhaseEnded + `',
           d.finished_at,
           a.tenant_id,
           t.name || '-' || a.name,
           a.name,
           '` + EventSourceDeploy + `'::text,
           '` + EventPhaseEnded + `'::text,
           d.id,
           d.status,
           ''::text
    FROM deploys d
    JOIN apps a ON a.id = d.app_id
    JOIN tenants t ON t.id = a.tenant_id
    WHERE d.finished_at IS NOT NULL AND a.tenant_id = ANY($5)
  UNION ALL
    SELECT e.id || ':',
           e.at,
           a.tenant_id,
           t.name || '-' || a.name,
           a.name,
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
           e.verb
    FROM audit_events e
    JOIN apps a ON a.tenant_id = ANY($5)
     AND (e.workspace_id = a.tenant_id OR e.workspace_id = '` + core.DefaultTenant + `')
    JOIN tenants t ON t.id = a.tenant_id
     AND e.target IN ('service:' || t.name || '-' || a.name, 'service:' || a.name)
    WHERE e.outcome = 'allowed' AND e.verb = ANY($4)
  UNION ALL
    SELECT e.id || ':',
           e.at,
           e.workspace_id,
           split_part(e.target, ':', 2),
           split_part(e.target, ':', 2),
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
           e.verb
    FROM audit_events e
    WHERE e.outcome = 'allowed'
      AND e.verb = ANY($4)
      AND e.workspace_id = ANY($5)
      AND e.target LIKE 'database:%'
  UNION ALL
    SELECT e.id || ':',
           e.at,
           e.workspace_id,
           split_part(e.target, ':', 2),
           split_part(e.target, ':', 2),
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
           e.verb
    FROM audit_events e
    WHERE e.outcome = 'allowed'
      AND e.verb = ANY($4)
      AND e.workspace_id = ANY($5)
      AND e.target LIKE 'keyvalue:%'
)
SELECT key, at, tenant_id, service_id, service_name, source, phase, deploy_id, status, verb
FROM feed
WHERE (at, key) > ($1, $2) AND at <= $3
ORDER BY at, key
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
		if err := rows.Scan(&r.Key, &r.At, &r.TenantID, &r.ServiceID, &r.ServiceName, &r.Source, &r.Phase, &r.DeployID, &r.Status, &r.Verb); err != nil {
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
// subscribes to.
func (s *PGStore) EnqueueWebhookDeliveries(ctx context.Context, deliveries []WebhookDelivery, at time.Time, key string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	// One round trip for the whole batch (a dispatch pass can carry hundreds
	// of event×endpoint rows), not one per insert while the txn sits open.
	b := &pgx.Batch{}
	for _, d := range deliveries {
		b.Queue(
			`INSERT INTO webhook_deliveries (id, endpoint_id, event_id, event_type, service_id, payload, next_attempt_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			d.ID, d.EndpointID, d.EventID, d.EventType, d.ServiceID, d.Payload, d.NextAttemptAt)
	}
	b.Queue(`UPDATE webhook_watermark SET at = $1, key = $2 WHERE id`, at, key)
	if err := tx.SendBatch(ctx, b).Close(); err != nil {
		return classify("webhook delivery", err)
	}
	return tx.Commit(ctx)
}

// DueWebhookDelivery is a pending delivery joined with what the sender needs
// from its endpoint: the destination, the signing secret, and who to notify
// when it keeps failing.
type DueWebhookDelivery struct {
	WebhookDelivery
	URL          string
	Secret       string
	TenantID     string
	EndpointName string
	CreatedBy    string
}

// DueWebhookDeliveries returns open deliveries whose next attempt is due,
// oldest first, for enabled endpoints only — disabling an endpoint parks its
// queue (rows stay open but are never selected) until it is re-enabled.
func (s *PGStore) DueWebhookDeliveries(ctx context.Context, now time.Time, limit int) ([]DueWebhookDelivery, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+prefixColumns("d", webhookDeliveryColumns)+`, e.url, e.secret, e.tenant_id, e.name, e.created_by
		 FROM webhook_deliveries d
		 JOIN webhook_endpoints e ON e.id = d.endpoint_id
		 WHERE d.delivered_at IS NULL AND d.failed_at IS NULL AND d.next_attempt_at <= $1 AND e.enabled
		 ORDER BY d.next_attempt_at
		 LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueWebhookDelivery
	for rows.Next() {
		var d DueWebhookDelivery
		if err := rows.Scan(&d.ID, &d.EndpointID, &d.EventID, &d.EventType, &d.ServiceID, &d.Payload,
			&d.AttemptCount, &d.LastStatus, &d.LastError, &d.NextAttemptAt, &d.LastAttemptedAt,
			&d.DeliveredAt, &d.FailedAt, &d.CreatedAt,
			&d.URL, &d.Secret, &d.TenantID, &d.EndpointName, &d.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RecordWebhookAttempt books one attempt's outcome: increments the count,
// records status/error/time, and either closes the row (delivered/failed) or
// schedules the next try.
func (s *PGStore) RecordWebhookAttempt(ctx context.Context, id string, status int, errMsg string, at, next time.Time, delivered, failed bool) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET attempt_count = attempt_count + 1, last_status = $2, last_error = $3,
		     last_attempted_at = $4, next_attempt_at = $5,
		     delivered_at = CASE WHEN $6 THEN $4 ELSE delivered_at END,
		     failed_at    = CASE WHEN $7 THEN $4 ELSE failed_at END
		 WHERE id = $1`,
		id, status, errMsg, at, next, delivered, failed)
	return err
}

// prefixColumns qualifies a comma-separated column list with a table alias —
// "id, at" -> "d.id, d.at" — so a joined query can reuse a scan's column list.
func prefixColumns(alias, columns string) string {
	parts := strings.Split(columns, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}
