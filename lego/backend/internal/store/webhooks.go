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

// MaxWebhookEndpointsPerWorkspace bounds how many endpoints one workspace may
// register (w1/m67 F2). Every event is fanned out across every enabled endpoint
// of its workspace by a worker SHARED with every other tenant, so an unbounded
// endpoint count is an unbounded multiplier on shared memory and database work —
// growable persistently, by an ordinary workspace admin, at no cost to them.
// The value is far above any real integration topology (Render's own docs
// describe a handful of endpoints per workspace); it exists to stop the
// pathological case, not to shape normal use.
const MaxWebhookEndpointsPerWorkspace = 25

// ErrWebhookEndpointLimit is the typed refusal when a workspace is already at
// MaxWebhookEndpointsPerWorkspace. Wraps ErrConflict so existing REST/GraphQL/MCP
// error mapping treats it like the other quota refusals (cf. ErrInvitePlanLimit).
var ErrWebhookEndpointLimit = fmt.Errorf("workspace has the maximum number of webhook endpoints: %w", ErrConflict)

// CreateWebhookEndpoint mints a new endpoint row (id.New(id.Webhook)) and
// inserts it. The secret is minted by the caller (internal/webhooks owns the
// format) and returned on this one response only. An empty name defaults to
// the URL.
//
// The count check and the insert share ONE transaction, and the count takes a
// row-level lock over the workspace's existing endpoints, so two concurrent
// creates at the boundary cannot both observe "one slot left" and both take it.
func (s *PGStore) CreateWebhookEndpoint(ctx context.Context, tenantID, name, url, secret string, eventTypes []string, createdBy string) (WebhookEndpoint, error) {
	if name == "" {
		name = url
	}
	e := WebhookEndpoint{
		ID: ids.New(ids.Webhook), TenantID: tenantID, Name: name, URL: url,
		Secret: secret, EventTypes: eventTypes, Enabled: true, CreatedBy: createdBy,
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
		return tx.QueryRow(ctx,
			`INSERT INTO webhook_endpoints (id, tenant_id, name, url, secret, event_types, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at`,
			e.ID, e.TenantID, e.Name, e.URL, e.Secret, e.EventTypes, e.CreatedBy,
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
		`UPDATE webhook_endpoints SET enabled = $3, disabled_reason = $4, updated_at = now(),
		     notified_at = CASE WHEN $3 THEN NULL ELSE notified_at END
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

// UpdateWebhookEndpoint replaces an endpoint's mutable fields (name, url,
// event_types, enabled) in one SQL round trip. Re-enabling an endpoint clears
// any disabled_reason; disabling leaves it unchanged (use SetWebhookEndpointEnabled
// for an explicit reason). Secret is immutable after creation.
func (s *PGStore) UpdateWebhookEndpoint(ctx context.Context, tenantID, id, name, url string, eventTypes []string, enabled bool) (WebhookEndpoint, error) {
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
// straight into an API call; ServiceName is the human app name.
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
	Status   string
	Verb     string // audit rows: e.g. "apps.Suspend"
	FactType string // fact rows: closed service_event_facts.fact_type
	// AppID is the app's internal control-plane id, as opposed to ServiceID's
	// public composite. Empty on the datastore audit arm, which has no app.
	AppID string
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
//   - Datastore audit rows need no control-plane join: their typed target holds
//     the immutable dpg-/red- id and target_name holds the display name.
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
    SELECT d.created_at                        AS cursor_at,
           d.id || ':` + EventPhaseStarted + `' AS key,
           d.created_at                        AS at,
           a.tenant_id                         AS tenant_id,
           t.name || '-' || a.name             AS service_id,
           a.name                              AS service_name,
           '` + EventSourceDeploy + `'::text   AS source,
           '` + EventPhaseStarted + `'::text   AS phase,
           d.id                                AS deploy_id,
           ''::text                            AS status,
           ''::text                            AS verb,
           ''::text                            AS fact_type,
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
           a.name,
           '` + EventSourceDeploy + `'::text,
           '` + EventPhaseEnded + `'::text,
           d.id,
           d.status,
           ''::text,
           ''::text,
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
           a.name,
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
           e.verb,
           ''::text,
           a.id
    FROM audit_events e
    JOIN apps a ON a.tenant_id = ANY($5)
     AND (e.workspace_id = a.tenant_id OR e.workspace_id = '` + core.DefaultTenant + `')
    JOIN tenants t ON t.id = a.tenant_id
     AND e.target IN ('service:' || t.name || '-' || a.name, 'service:' || a.name)
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
           a.name,
           '` + EventSourceFact + `'::text,
           ''::text,
           f.deploy_id,
           COALESCE(f.status, ''),
           ''::text,
           f.fact_type,
           a.id
    FROM service_event_facts f
    JOIN apps a ON a.id = f.app_id
    JOIN tenants t ON t.id = a.tenant_id
    WHERE a.tenant_id = ANY($5)
)
SELECT cursor_at, key, at, tenant_id, service_id, service_name, source, phase, deploy_id, status, verb, fact_type, app_id
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
		if err := rows.Scan(&r.CursorAt, &r.Key, &r.At, &r.TenantID, &r.ServiceID, &r.ServiceName, &r.Source, &r.Phase, &r.DeployID, &r.Status, &r.Verb, &r.FactType, &r.AppID); err != nil {
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
		// ON CONFLICT (endpoint_id, event_id) DO NOTHING makes dispatch idempotent
		// across replicas and restarts (w1/m58): a second replica reading the same
		// feed window — or this worker re-reading after a crash — inserts no
		// duplicate delivery even though it mints a fresh delivery id each pass.
		b.Queue(
			`INSERT INTO webhook_deliveries (id, endpoint_id, event_id, event_type, service_id, payload, next_attempt_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (endpoint_id, event_id) DO NOTHING`,
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

// ClaimDueWebhookDeliveries atomically leases open, due deliveries for enabled
// endpoints (oldest first) to exactly one caller and returns them with what the
// sender needs. It is the multi-replica-safe replacement for a plain due-read
// (w1/m58): `FOR UPDATE ... SKIP LOCKED` hands each concurrent worker a DISJOINT
// batch (no two replicas claim the same row), and the claim bumps next_attempt_at
// to leaseUntil so a row stays invisible to other workers for the whole POST
// window — a worker that crashes mid-send simply releases the lease, and the row
// becomes due again after leaseUntil (at-least-once; receivers dedupe on
// webhook-id). The lease is not an attempt: attempt_count is untouched, so a
// crashed claim does not consume a retry. Disabling an endpoint still parks its
// queue (rows stay open but are never claimed) until it is re-enabled.
func (s *PGStore) ClaimDueWebhookDeliveries(ctx context.Context, now, leaseUntil time.Time, limit int) ([]DueWebhookDelivery, error) {
	rows, err := s.Pool.Query(ctx,
		`WITH due AS (
		     SELECT d.id
		     FROM webhook_deliveries d
		     JOIN webhook_endpoints e ON e.id = d.endpoint_id
		     WHERE d.delivered_at IS NULL AND d.failed_at IS NULL
		       AND d.next_attempt_at <= $1 AND e.enabled
		     ORDER BY d.next_attempt_at
		     LIMIT $3
		     FOR UPDATE OF d SKIP LOCKED
		 ), claimed AS (
		     UPDATE webhook_deliveries d
		     SET next_attempt_at = $2
		     FROM due
		     WHERE d.id = due.id
		     RETURNING `+prefixColumns("d", webhookDeliveryColumns)+`
		 )
		 SELECT `+prefixColumns("claimed", webhookDeliveryColumns)+`, e.url, e.secret, e.tenant_id, e.name, e.created_by
		 FROM claimed
		 JOIN webhook_endpoints e ON e.id = claimed.endpoint_id
		 ORDER BY claimed.created_at, claimed.id`, now, leaseUntil, limit)
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

// SweepWebhookDeliveries purges reclaimable delivery rows (w1/m67 F3). The table
// is both the durable delivery QUEUE and the product's history surface, so before
// m67 nothing ever reclaimed a finished row: ordinary tenant activity grew shared
// table, index, and backup storage without bound.
//
// Terminal rows (delivered_at or failed_at set) are purged when either:
//
//   - older than `before`, so history has a finite lifetime; or
//   - beyond `keepPerEndpoint` most recent rows for their endpoint, so a burst
//     inside the age window cannot evade the age rule alone.
//
// Abandoned PENDING rows older than `before` are also purged (scan finding #3):
// a disabled endpoint's open deliveries are never claimed (ClaimDue requires
// e.enabled) and never terminalized, so without this they park forever and grow
// the shared table without bound — an attacker's disabled-endpoint backlog would
// survive retention indefinitely. This is safe because a live retryable delivery
// exhausts its ~33h schedule far inside the 90-day retention floor, so a still-open
// row older than `before` is definitively dead, not mid-retry. Park-and-resume for
// a recently disabled endpoint is unaffected (its rows are younger than `before`).
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
		`WITH ranked AS (
		   SELECT id, created_at,
		          row_number() OVER (PARTITION BY endpoint_id ORDER BY created_at DESC, id DESC) AS rn
		     FROM webhook_deliveries
		    WHERE delivered_at IS NOT NULL OR failed_at IS NOT NULL
		 ),
		 eligible AS (
		   SELECT id FROM ranked
		    WHERE rn > $2 OR created_at < $1
		    ORDER BY created_at
		    LIMIT $3
		    FOR UPDATE SKIP LOCKED
		 )
		 DELETE FROM webhook_deliveries d USING eligible e WHERE d.id = e.id`,
		before, keepPerEndpoint, limit)
	if err != nil {
		return 0, classify("webhook deliveries", err)
	}
	abandoned, err := s.Pool.Exec(ctx,
		`WITH eligible AS (
		   SELECT id FROM webhook_deliveries
		    WHERE delivered_at IS NULL AND failed_at IS NULL AND created_at < $1
		    ORDER BY created_at
		    LIMIT $2
		    FOR UPDATE SKIP LOCKED
		 )
		 DELETE FROM webhook_deliveries d USING eligible e WHERE d.id = e.id`,
		before, limit)
	if err != nil {
		return 0, classify("webhook deliveries", err)
	}
	return terminal.RowsAffected() + abandoned.RowsAffected(), nil
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
