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

// Package webhooks is the outbound event webhooks feature (w3/m11): a
// workspace registers a destination URL + an event-type subscription, and bex
// pushes signed, thin-payload notifications when those transitions happen —
// Render's /webhooks direction (Render → you), closing bex's inbound-only
// webhook asymmetry (docs/ADR018-render-parity.md § Platform events &
// integrations).
//
// # One instrumentation pass (w3/m11/t002)
//
// bex does not add a webhook write path. The event SOURCE is the same
// two-table composition the per-service events feed derives from (w3/m7,
// internal/events: deploys + audit_events) — the Worker's dispatcher tails it
// workspace-wide through a durable watermark (store.ListWebhookEvents) instead
// of paging it per-service. The write paths were instrumented exactly once
// (w2/m5's deploys rows, w3/m7's audit target column); a webhook therefore
// fires for a transition no matter which path performed it (an API verb, a
// git-push redeploy, the reconciler closing a deploy), and emission can never
// block or fail a verb — there IS no emission, only rows the verb already
// wrote.
//
// # Vocabulary
//
// Render's webhook event names (verified live, render.com/docs/webhooks
// 2026-07-12), scoped to the transitions bex's two sources actually record:
// deploy_started/deploy_ended from deploys rows; the rest mapped from audited
// verbs. Render types with no bex source (build_started/build_ended,
// server_failed/server_available, autoscaling_started/autoscaling_ended,
// cron_job_run_ended) are omitted, not faked — the events feed's own policy.
//
// # Delivery semantics (w3/m11/t003, matching Render's documented behavior)
//
// Thin payload {type, timestamp, data: {id, serviceId, serviceName, status?}};
// Standard-Webhooks HMAC-SHA256 signing (webhook-id/-timestamp/-signature
// headers); 2xx within 15s or the attempt failed; up to 8 retries on a
// bounded exponential backoff (final ~33h out); a failure notice emailed
// after 3 consecutive failures; the endpoint auto-disabled (until manually
// re-enabled) once retries are exhausted.
//
// # Deliberate divergences from Render
//
//   - No plan-tier gating: bex has no billing system (w6 non-goal), so any
//     workspace may register any number of endpoints (Render: Pro=1,
//     Scale=100).
//   - Endpoint management is a full REST/GraphQL/MCP/UI surface; Render's
//     public docs manage webhooks from the dashboard only.
//
// Requires the control-plane store (BEX_CP_DB_URI): endpoints, deliveries,
// and both event sources live there, so with it unwired every verb reports
// core.ErrWebhooksUnavailable (503) — omitted, not faked.
package webhooks

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Render's webhook event types (the subset bex can emit truthfully), spelled
// exactly as render.com/docs/webhooks spells them. Note the webhook vocabulary
// is Render's own and differs from its list-events enum where the two overlap
// (webhooks say service_suspended; the events feed says suspender_added).
const (
	TypeDeployStarted             = "deploy_started"
	TypeDeployEnded               = "deploy_ended"
	TypeServerRestarted           = "server_restarted"
	TypeServiceSuspended          = "service_suspended"
	TypeServiceResumed            = "service_resumed"
	TypeInstanceCountChanged      = "instance_count_changed"
	TypeAutoscalingConfigChanged  = "autoscaling_config_changed"
	TypeCronJobRunStarted         = "cron_job_run_started"
	TypeCronJobRunEnded           = "cron_job_run_ended"
	TypeMaintenanceModeEnabled    = "maintenance_mode_enabled"
	TypeMaintenanceModeURIUpdated = "maintenance_mode_uri_updated"
	// Managed datastore events (w3/m26): Render's webhook vocabulary extended to
	// Postgres and Key Value lifecycle transitions. Render does not document these
	// publicly — bex defines them following the service_* naming convention so a
	// Render-shaped receiver can extend its handler without a schema break.
	TypePostgresCreated   = "postgres_created"
	TypePostgresDeleted   = "postgres_deleted"
	TypePostgresSuspended = "postgres_suspended"
	TypePostgresResumed   = "postgres_resumed"
	TypeKeyValueCreated   = "key_value_created"
	TypeKeyValueDeleted   = "key_value_deleted"
	TypeKeyValueSuspended = "key_value_suspended"
	TypeKeyValueResumed   = "key_value_resumed"
)

// verbEvents maps an audited verb ("<package>.<Method>", the same key
// internal/events maps) to the webhook event type it produces — the SINGLE
// source of the audit-side vocabulary: the dispatcher pushes exactly these
// verbs down into the store query, so a verb absent here is not a webhook
// event. Deploy transitions come from deploys rows, not a verb (see
// eventTypeOf).
var verbEvents = map[string]string{
	"apps.Restart":                          TypeServerRestarted,
	"apps.Suspend":                          TypeServiceSuspended,
	"apps.Resume":                           TypeServiceResumed,
	"apps.Scale":                            TypeInstanceCountChanged,
	"apps.SetAutoscaling":                   TypeAutoscalingConfigChanged,
	"apps.DeleteAutoscaling":                TypeAutoscalingConfigChanged,
	"apps.TriggerCronRun":                   TypeCronJobRunStarted,
	"apps.CancelCronRun":                    TypeCronJobRunEnded,
	"apps.CancelCurrentCronRun":             TypeCronJobRunEnded,
	core.AuditVerbMaintenanceModeEnabled:    TypeMaintenanceModeEnabled,
	core.AuditVerbMaintenanceModeURIUpdated: TypeMaintenanceModeURIUpdated,
	// Managed datastore lifecycle verbs (w3/m26) — picked up by the
	// database/keyvalue UNION ALL arms in store.webhookEventsQuery.
	core.AuditVerbCreatePostgres:  TypePostgresCreated,
	core.AuditVerbDeletePostgres:  TypePostgresDeleted,
	core.AuditVerbSuspendPostgres: TypePostgresSuspended,
	core.AuditVerbResumePostgres:  TypePostgresResumed,
	core.AuditVerbCreateKeyValue:  TypeKeyValueCreated,
	core.AuditVerbDeleteKeyValue:  TypeKeyValueDeleted,
	core.AuditVerbSuspendKeyValue: TypeKeyValueSuspended,
	core.AuditVerbResumeKeyValue:  TypeKeyValueResumed,
}

// auditVerbs is verbEvents' key set — the dispatcher's push-down filter,
// computed once.
var auditVerbs = slices.Sorted(maps.Keys(verbEvents))

// EventTypes is the full subscribable vocabulary, sorted — what Create
// validates against and what the dashboard's event-type picker lists.
var EventTypes = func() []string {
	set := map[string]bool{TypeDeployStarted: true, TypeDeployEnded: true}
	for _, t := range verbEvents {
		set[t] = true
	}
	return slices.Sorted(maps.Keys(set))
}()

// EndpointStore is the Service's seam to the control-plane store —
// *store.PGStore satisfies it; a fake backs the tests. nil => the store is
// off (BEX_CP_DB_URI unset).
type EndpointStore interface {
	CreateWebhookEndpoint(ctx context.Context, tenantID, name, url, secret string, eventTypes []string, createdBy string) (store.WebhookEndpoint, error)
	ListWebhookEndpoints(ctx context.Context, tenantID string) ([]store.WebhookEndpoint, error)
	GetWebhookEndpoint(ctx context.Context, tenantID, id string) (store.WebhookEndpoint, error)
	SetWebhookEndpointEnabled(ctx context.Context, tenantID, id string, enabled bool, reason string) (store.WebhookEndpoint, error)
	UpdateWebhookEndpoint(ctx context.Context, tenantID, id, name, url string, eventTypes []string, enabled bool) (store.WebhookEndpoint, error)
	DeleteWebhookEndpoint(ctx context.Context, tenantID, id string) error
	ListWebhookDeliveries(ctx context.Context, endpointID string, afterAt time.Time, afterKey string, limit int) ([]store.WebhookDelivery, error)
}

// Service is the webhook-endpoint management feature: CRUD + delivery
// history, one implementation behind the three surfaces. The delivery Worker
// is a separate type (worker.go) — it acts for the platform, not a caller,
// so it carries no verbs and no authorization.
type Service struct {
	*core.Base
	Store EndpointStore
	// Selections is the shared MCP session-workspace reader (core.SelectedWorkspace).
	Selections core.WorkspaceSelectionReader
}

// EndpointView is the neutral shape every adapter renders. Secret is
// populated only by Create — the mint-once read (the api-key precedent);
// every other verb's view leaves it empty because the store's read queries
// never select the column.
type EndpointView struct {
	ID             string
	Name           string
	URL            string
	OwnerID        string
	EventTypes     []string
	Enabled        bool
	DisabledReason string
	Secret         string
	CreatedBy      string
	CreatedAt      string
	UpdatedAt      string
}

// Delivery statuses a DeliveryView reports — derived from which terminal
// timestamp the row carries, not stored.
const (
	DeliveryPending   = "pending"
	DeliveryDelivered = "delivered"
	DeliveryFailed    = "failed"
)

// DeliveryView is one delivery-history entry. Cursor is the opaque keyset
// position a client echoes back to resume (the events-feed cursor shape —
// rows are updated in place by retries, so paging on a mutable field would
// shift pages under the reader; (createdAt, id) never changes).
type DeliveryView struct {
	ID              string
	EventID         string
	EventType       string
	ServiceID       string
	Status          string
	AttemptCount    int
	LastStatusCode  int
	LastError       string
	NextAttemptAt   string // "" once the row is terminal
	LastAttemptedAt string
	DeliveredAt     string
	CreatedAt       string
	Cursor          string
}

// workspaceID is the store key for the caller's endpoints: the caller's
// tenant when resolvable, else the single-workspace default (mirrors
// registrycreds.Service.workspaceID).
func (s *Service) workspaceID(ctx context.Context) string {
	if tid, ok := s.Tenant(ctx); ok && tid != "" {
		return tid
	}
	return core.DefaultTenant
}

func toView(e store.WebhookEndpoint) EndpointView {
	return EndpointView{
		ID:             e.ID,
		Name:           e.Name,
		URL:            e.URL,
		OwnerID:        e.TenantID,
		EventTypes:     e.EventTypes,
		Enabled:        e.Enabled,
		DisabledReason: e.DisabledReason,
		Secret:         e.Secret,
		CreatedBy:      e.CreatedBy,
		CreatedAt:      e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toDeliveryView(d store.WebhookDelivery) DeliveryView {
	v := DeliveryView{
		ID:             d.ID,
		EventID:        d.EventID,
		EventType:      d.EventType,
		ServiceID:      d.ServiceID,
		Status:         DeliveryPending,
		AttemptCount:   d.AttemptCount,
		LastStatusCode: d.LastStatus,
		LastError:      d.LastError,
		CreatedAt:      d.CreatedAt.UTC().Format(time.RFC3339),
		Cursor:         core.EncodeKeysetCursor(d.CreatedAt, d.ID),
	}
	switch {
	case d.DeliveredAt != nil:
		v.Status = DeliveryDelivered
		v.DeliveredAt = d.DeliveredAt.UTC().Format(time.RFC3339)
	case d.FailedAt != nil:
		v.Status = DeliveryFailed
	default:
		v.NextAttemptAt = d.NextAttemptAt.UTC().Format(time.RFC3339)
	}
	if d.LastAttemptedAt != nil {
		v.LastAttemptedAt = d.LastAttemptedAt.UTC().Format(time.RFC3339)
	}
	return v
}

// CreateRequest is Create's input. URL and EventTypes are required; Name is a
// human display label defaulting to the URL. OwnerID is the workspace to
// create in — Render's `ownerId` (empty means the caller's own resolved
// default workspace).
type CreateRequest struct {
	OwnerID    string
	Name       string
	URL        string
	EventTypes []string
}

// Create registers a new endpoint and returns it WITH its freshly minted
// signing secret — the only response that ever carries it (store it; it is
// not retrievable). Admin-only (RelCanManage): an endpoint exports the
// workspace's whole activity stream to an external URL, the same bar
// registrycreds/github hold for workspace integrations.
func (s *Service) Create(ctx context.Context, req CreateRequest) (EndpointView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return EndpointView{}, err
	}
	if s.Store == nil {
		return EndpointView{}, core.ErrWebhooksUnavailable
	}
	dest, err := parseDestination(req.URL)
	if err != nil {
		return EndpointView{}, err
	}
	types, err := normalizeEventTypes(req.EventTypes)
	if err != nil {
		return EndpointView{}, err
	}
	secret, err := NewSecret()
	if err != nil {
		return EndpointView{}, fmt.Errorf("mint webhook secret: %w", err)
	}
	createdBy := ""
	if id, ok := core.IdentityFrom(ctx); ok {
		createdBy = id.Subject
	}
	e, err := s.Store.CreateWebhookEndpoint(ctx, s.workspaceID(ctx), strings.TrimSpace(req.Name), dest, secret, types, createdBy)
	if err != nil {
		return EndpointView{}, err
	}
	return toView(e), nil
}

// List returns the workspace's endpoints (secrets never included). ownerID
// optionally names the workspace (Render's `ownerId` filter); empty means the
// caller's own resolved default. Member read.
func (s *Service) List(ctx context.Context, ownerID string) ([]EndpointView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWebhooksUnavailable
	}
	rows, err := s.Store.ListWebhookEndpoints(ctx, s.workspaceID(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]EndpointView, 0, len(rows))
	for _, e := range rows {
		out = append(out, toView(e))
	}
	return out, nil
}

// Get returns one endpoint (secret never included). ownerID optionally names
// the workspace to look in (empty = the caller's resolved default — the
// apikeys convention, so a multi-workspace caller's switcher works); the store
// lookup is scoped to it, so another workspace's id is a 404, never a leak.
// Member read.
func (s *Service) Get(ctx context.Context, ownerID, id string) (EndpointView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return EndpointView{}, err
	}
	if s.Store == nil {
		return EndpointView{}, core.ErrWebhooksUnavailable
	}
	e, err := s.Store.GetWebhookEndpoint(ctx, s.workspaceID(ctx), id)
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	return toView(e), nil
}

// SetEnabled flips an endpoint on or off — also how an auto-disabled endpoint
// is re-armed after its destination is fixed (Render: disabled "until you
// re-enable it"). Admin-only, matching Create.
func (s *Service) SetEnabled(ctx context.Context, ownerID, id string, enabled bool) (EndpointView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return EndpointView{}, err
	}
	if s.Store == nil {
		return EndpointView{}, core.ErrWebhooksUnavailable
	}
	e, err := s.Store.SetWebhookEndpointEnabled(ctx, s.workspaceID(ctx), id, enabled, "disabled manually")
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	return toView(e), nil
}

// Delete removes an endpoint and (by cascade) its delivery history.
// Admin-only, matching Create.
func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrWebhooksUnavailable
	}
	return mapStoreErr(s.Store.DeleteWebhookEndpoint(ctx, s.workspaceID(ctx), id))
}

// ListDeliveries returns one endpoint's delivery history, newest first,
// keyset-paged. The endpoint is fetched (workspace-scoped) first, so a
// cross-workspace endpoint id 404s before any history is read. Member read.
func (s *Service) ListDeliveries(ctx context.Context, ownerID, endpointID, cursor string, limit int) ([]DeliveryView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWebhooksUnavailable
	}
	if _, err := s.Store.GetWebhookEndpoint(ctx, s.workspaceID(ctx), endpointID); err != nil {
		return nil, mapStoreErr(err)
	}
	after, err := core.DecodeKeysetCursor(cursor)
	if err != nil {
		return nil, err
	}
	// The store clamps limit to the shared page bounds (the ListServiceEvents
	// convention) — no second feature-side clamp.
	rows, err := s.Store.ListWebhookDeliveries(ctx, endpointID, after.At, after.Key, limit)
	if err != nil {
		return nil, err
	}
	out := make([]DeliveryView, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDeliveryView(d))
	}
	return out, nil
}

// UpdateRequest is Update's input — Render's full-body PATCH (w3/m27):
// any zero-value field means "keep the current value". EventTypes may carry
// the `eventFilter` alias from Render-shaped clients (the caller normalizes
// before calling Update).
type UpdateRequest struct {
	Name       string
	URL        string
	EventTypes []string
	Enabled    *bool
}

// Update applies a full-body update to an endpoint (Render's PATCH contract,
// w3/m27). Non-zero fields replace the current value; zero fields keep it.
// URL changes are re-validated; EventTypes changes are re-normalised.
// Admin-only (RelCanManage), matching Create and SetEnabled.
func (s *Service) Update(ctx context.Context, ownerID, id string, req UpdateRequest) (EndpointView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return EndpointView{}, err
	}
	if s.Store == nil {
		return EndpointView{}, core.ErrWebhooksUnavailable
	}
	// Fetch the current endpoint (workspace-scoped, so cross-workspace ids 404).
	cur, err := s.Store.GetWebhookEndpoint(ctx, s.workspaceID(ctx), id)
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	// Merge: keep current values for omitted fields.
	name := cur.Name
	if req.Name != "" {
		name = strings.TrimSpace(req.Name)
	}
	dest := cur.URL
	if req.URL != "" {
		if dest, err = parseDestination(req.URL); err != nil {
			return EndpointView{}, err
		}
	}
	types := cur.EventTypes
	if len(req.EventTypes) > 0 {
		if types, err = normalizeEventTypes(req.EventTypes); err != nil {
			return EndpointView{}, err
		}
	}
	enabled := cur.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	e, err := s.Store.UpdateWebhookEndpoint(ctx, s.workspaceID(ctx), id, name, dest, types, enabled)
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	return toView(e), nil
}

// parseDestination validates a destination URL: absolute, http(s), with a
// host. Returned trimmed — what the store keeps and the sender POSTs to.
func parseDestination(raw string) (string, error) {
	dest := strings.TrimSpace(raw)
	if !core.ValidAbsoluteHTTPURL(dest) {
		return "", core.ErrNotAbsoluteHTTPURL("url")
	}
	return dest, nil
}

// normalizeEventTypes validates and de-duplicates a subscription against the
// vocabulary, preserving EventTypes' canonical order. Empty means "nothing
// would ever fire" and is rejected rather than stored inert.
func normalizeEventTypes(types []string) ([]string, error) {
	if len(types) == 0 {
		return nil, fmt.Errorf("%w: at least one event type is required", core.ErrBadRequest)
	}
	asked := make(map[string]bool, len(types))
	for _, t := range types {
		if !slices.Contains(EventTypes, t) {
			return nil, fmt.Errorf("%w: unknown event type %q (valid: %s)", core.ErrBadRequest, t, strings.Join(EventTypes, ", "))
		}
		asked[t] = true
	}
	out := make([]string, 0, len(asked))
	for _, t := range EventTypes {
		if asked[t] {
			out = append(out, t)
		}
	}
	return out, nil
}

// mapStoreErr maps the store's ErrNotFound onto core.ErrNotFound so the
// adapters' shared error mapping applies (the registrycreds pattern).
func mapStoreErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return core.ErrNotFound
	}
	return err
}
