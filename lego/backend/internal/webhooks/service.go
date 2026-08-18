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
// three-source composition the per-service events feed derives from (deploys +
// audit_events + typed service_event_facts) — the Worker's dispatcher tails it
// workspace-wide through a durable watermark (store.ListWebhookEvents) instead
// of paging it per-service. The write paths were instrumented exactly once
// (deploy rows, audit targets, or a typed fact producer); a webhook therefore
// fires for a transition no matter which path performed it (an API verb, a
// git-push redeploy, the reconciler closing a deploy), and emission can never
// block or fail a verb — there IS no emission, only rows the verb already
// wrote.
//
// # Vocabulary
//
// Render's webhook event names (verified live, render.com/docs/webhooks
// 2026-07-18), scoped to transitions bex can record truthfully: deploy rows,
// audited API intent, and typed observed/Git facts. Unsupported provider,
// billing, workflow, preview, disk, and edge-cache events are omitted, not faked.
//
// # Delivery semantics (w3/m11/t003, matching Render's documented behavior)
//
// Thin payload {type, timestamp, data: {id, serviceId, serviceName, status?}};
// Standard-Webhooks HMAC-SHA256 signing (webhook-id/-timestamp/-signature
// headers); 2xx within 15s or the attempt failed; up to 8 total attempts on a
// bounded exponential backoff (final ~33h out); a failure notice emailed
// after 3 consecutive failures; the endpoint auto-disabled (until manually
// re-enabled) once retries are exhausted.
//
// # Deliberate divergences from Render
//
//   - No plan-tier gating: every workspace may register up to 25 endpoints,
//     independent of plan (Render: Pro=1, Scale=100).
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
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/eventvocab"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Render's webhook event types (the subset bex can emit truthfully), spelled
// exactly as render.com/docs/webhooks spells them, except branch_changed, an
// explicit bex extension for the dashboard label Render exposes without an
// equivalent public enum. Intent suspender_* and observed service_* remain
// distinct in the service feed; outbound notifications use observed service_*.
const (
	TypeDeployStarted              = "deploy_started"
	TypeDeployEnded                = "deploy_ended"
	TypeBranchDeleted              = "branch_deleted"
	TypeBuildStarted               = "build_started"
	TypeBuildEnded                 = "build_ended"
	TypePreDeployStarted           = "pre_deploy_started"
	TypePreDeployEnded             = "pre_deploy_ended"
	TypeJobRunEnded                = "job_run_ended"
	TypeAutoDeployEnabled          = "auto_deploy_enabled"
	TypeAutoDeployDisabled         = "auto_deploy_disabled"
	TypeServerRestarted            = "server_restarted"
	TypeServiceSuspended           = "service_suspended"
	TypeServiceResumed             = "service_resumed"
	TypeInstanceCountChanged       = "instance_count_changed"
	TypeAutoscalingConfigChanged   = "autoscaling_config_changed"
	TypeCronJobRunStarted          = "cron_job_run_started"
	TypeCronJobRunEnded            = "cron_job_run_ended"
	TypeMaintenanceModeEnabled     = "maintenance_mode_enabled"
	TypeMaintenanceModeURIUpdated  = "maintenance_mode_uri_updated"
	TypePlanChanged                = eventvocab.TypePlanChanged
	TypePostgresCreated            = eventvocab.TypePostgresCreated
	TypePostgresRestarted          = eventvocab.TypePostgresRestarted
	TypePostgresCredentialsCreated = eventvocab.TypePostgresCredentialsCreated
	TypePostgresCredentialsDeleted = eventvocab.TypePostgresCredentialsDeleted
	TypePostgresBackupStarted      = eventvocab.TypePostgresBackupStarted
	TypeImagePullFailed            = "image_pull_failed"
	TypeServerFailed               = "server_failed"
	TypeServerAvailable            = "server_available"
	TypeBranchChanged              = "branch_changed"
	TypeCommitIgnored              = "commit_ignored"
	TypeAutoscalingStarted         = "autoscaling_started"
	TypeAutoscalingEnded           = "autoscaling_ended"
)

// verbEvents maps an audited verb ("<package>.<Method>", the same key
// internal/events maps) to the webhook event type it produces — the SINGLE
// source of the audit-side vocabulary: the dispatcher pushes exactly these
// verbs down into the store query, so a verb absent here is not a webhook
// event. Deploy transitions come from deploys rows, not a verb (see
// eventTypeOf).
var verbEvents = func() map[string]string {
	types := eventvocab.DatastoreAuditTypes()
	types["apps.Restart"] = TypeServerRestarted
	types["apps.Scale"] = TypeInstanceCountChanged
	types["apps.SetAutoscaling"] = TypeAutoscalingConfigChanged
	types["apps.DeleteAutoscaling"] = TypeAutoscalingConfigChanged
	types[core.AuditVerbMaintenanceModeEnabled] = TypeMaintenanceModeEnabled
	types[core.AuditVerbMaintenanceModeURIUpdated] = TypeMaintenanceModeURIUpdated
	types[core.AuditVerbSetPlan] = TypePlanChanged
	return types
}()

const autoDeployVerb = core.AuditVerbSetAutoDeploy

var factEvents = map[string]string{
	string(store.EventFactImagePullFailed):    TypeImagePullFailed,
	string(store.EventFactServiceSuspended):   TypeServiceSuspended,
	string(store.EventFactServiceResumed):     TypeServiceResumed,
	string(store.EventFactServerFailed):       TypeServerFailed,
	string(store.EventFactServerAvailable):    TypeServerAvailable,
	string(store.EventFactBranchChanged):      TypeBranchChanged,
	string(store.EventFactCommitIgnored):      TypeCommitIgnored,
	string(store.EventFactAutoscalingStarted): TypeAutoscalingStarted,
	string(store.EventFactAutoscalingEnded):   TypeAutoscalingEnded,
	string(store.EventFactBranchDeleted):      TypeBranchDeleted,
	string(store.EventFactBuildStarted):       TypeBuildStarted,
	string(store.EventFactBuildEnded):         TypeBuildEnded,
	string(store.EventFactPreDeployStarted):   TypePreDeployStarted,
	string(store.EventFactPreDeployEnded):     TypePreDeployEnded,
	string(store.EventFactJobRunEnded):        TypeJobRunEnded,
	string(store.EventFactCronRunStarted):     TypeCronJobRunStarted,
	string(store.EventFactCronRunEnded):       TypeCronJobRunEnded,
}

// auditVerbs is verbEvents' key set — the dispatcher's push-down filter,
// computed once.
var auditVerbs = func() []string {
	verbs := slices.Collect(maps.Keys(verbEvents))
	verbs = append(verbs, autoDeployVerb)
	slices.Sort(verbs)
	return verbs
}()

// EventTypes is the full subscribable vocabulary, sorted — what Create
// validates against and what the dashboard's event-type picker lists.
var EventTypes = func() []string {
	set := map[string]bool{TypeDeployStarted: true, TypeDeployEnded: true}
	set[TypeAutoDeployEnabled] = true
	set[TypeAutoDeployDisabled] = true
	for _, t := range verbEvents {
		set[t] = true
	}
	for _, t := range factEvents {
		set[t] = true
	}
	return slices.Sorted(maps.Keys(set))
}()

// EndpointStore is the Service's seam to the control-plane store —
// *store.PGStore satisfies it; a fake backs the tests. nil => the store is
// off (BEX_CP_DB_URI unset).
type EndpointStore interface {
	CreateWebhookEndpoint(ctx context.Context, tenantID, name, url, secret string, eventTypes []string, enabled bool, createdBy string) (store.WebhookEndpoint, error)
	ListWebhookEndpoints(ctx context.Context, tenantIDs []string, afterAt time.Time, afterKey string, limit int) ([]store.WebhookEndpoint, error)
	GetWebhookEndpoint(ctx context.Context, tenantID, id string) (store.WebhookEndpoint, error)
	SetWebhookEndpointEnabled(ctx context.Context, tenantID, id string, enabled bool, reason string) (store.WebhookEndpoint, error)
	UpdateWebhookEndpoint(ctx context.Context, tenantID, id, name, url string, eventTypes []string, enabled bool) (store.WebhookEndpoint, error)
	DeleteWebhookEndpoint(ctx context.Context, tenantID, id string) error
	ListWebhookDeliveries(ctx context.Context, filter store.WebhookDeliveryFilter) ([]store.WebhookDelivery, error)
}

// Service is the webhook-endpoint management feature: CRUD + delivery
// history, one implementation behind the three surfaces. The delivery Worker
// is a separate type (worker.go) — it acts for the platform, not a caller,
// so it carries no verbs and no authorization.
type Service struct {
	*core.Base
	Store EndpointStore
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
	Cursor         string
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
	ID              string `json:"id"`
	EventID         string `json:"eventId"`
	EventType       string `json:"eventType"`
	ServiceID       string `json:"serviceId"`
	Status          string `json:"status"`
	AttemptCount    int    `json:"attemptCount"`
	LastStatusCode  int    `json:"lastStatusCode"`
	LastError       string `json:"lastError"`
	ResponseBody    string `json:"responseBody"`
	NextAttemptAt   string `json:"nextAttemptAt"` // "" once the row is terminal
	SentAt          string `json:"sentAt"`
	LastAttemptedAt string `json:"lastAttemptedAt"`
	DeliveredAt     string `json:"deliveredAt"`
	CreatedAt       string `json:"createdAt"`
	Cursor          string `json:"cursor"`
}

// workspaceID is the store key for the caller's endpoints: the caller's
// tenant when resolvable, else the single-workspace default (mirrors
// registrycreds.Service.workspaceID).
// toView projects a stored endpoint for a caller. exactURL is true only for
// the admin-gated verbs (Create/Update/SetEnabled) and for read callers that
// hold can_manage on the endpoint's workspace: the destination URL carries the
// integration capability, so ordinary member reads get the redacted origin
// (round-13 #7).
func toView(e store.WebhookEndpoint, exactURL bool) EndpointView {
	v := EndpointView{
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
		Cursor:         core.EncodeKeysetCursor(e.CreatedAt, e.ID),
	}
	if !exactURL {
		v.URL = RedactedURL(e.URL)
	}
	return v
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
		ResponseBody:   d.ResponseBody,
		CreatedAt:      d.CreatedAt.UTC().Format(time.RFC3339),
	}
	if d.SentAt != nil {
		v.SentAt = d.SentAt.UTC().Format(time.RFC3339)
		v.Cursor = core.EncodeKeysetCursor(*d.SentAt, d.ID)
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

// CreateRequest is Create's input. Name and URL are required. An empty
// EventTypes slice is Render's all-events subscription. OwnerID is the workspace to
// create in — Render's `ownerId` (empty means the caller's own resolved
// default workspace).
type CreateRequest struct {
	OwnerID    string
	Name       string
	URL        string
	EventTypes []string
	Enabled    bool
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
	name, err := normalizeName(req.Name)
	if err != nil {
		return EndpointView{}, err
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
	e, err := s.Store.CreateWebhookEndpoint(ctx, s.WorkspaceOrDefault(ctx), name, dest, secret, types, req.Enabled, createdBy)
	if err != nil {
		return EndpointView{}, mapCreateErr(err)
	}
	return toView(e, true), nil // Create is admin-gated: echo the exact destination
}

// EndpointLimitCode is the machine-readable refusal a caller sees when the
// workspace is already at the endpoint cap (w1/m67 F2). Named so REST, GraphQL,
// and MCP all surface the same code and the dashboard can render a human
// message instead of a raw store error.
const EndpointLimitCode = "WEBHOOK_ENDPOINT_LIMIT"

const (
	WebhookNameInvalidCode  = "WEBHOOK_NAME_INVALID"
	WebhookNameConflictCode = "WEBHOOK_NAME_CONFLICT"
	WebhookURLInvalidCode   = "WEBHOOK_URL_INVALID"
)

// mapCreateErr turns the store's typed quota refusal into a coded API error.
// Every other store error passes through the shared mapping.
func mapCreateErr(err error) error {
	if errors.Is(err, store.ErrWebhookEndpointLimit) {
		return core.NewBadRequestError(EndpointLimitCode,
			fmt.Sprintf("this workspace already has the maximum of %d webhook endpoints; delete one before adding another",
				store.MaxWebhookEndpointsPerWorkspace),
			map[string]any{"limit": store.MaxWebhookEndpointsPerWorkspace})
	}
	if errors.Is(err, store.ErrConflict) {
		return core.NewConflictError(WebhookNameConflictCode,
			"a webhook with this name already exists in the workspace", nil)
	}
	return mapStoreErr(err)
}

// List returns the workspace's endpoints (secrets never included). ownerID
// optionally names the workspace (Render's `ownerId` filter); empty means the
// caller's own resolved default. Member read.
func (s *Service) List(ctx context.Context, ownerID string) ([]EndpointView, error) {
	return s.ListPage(ctx, []string{ownerID}, "", core.MaxPageLimit)
}

// ListPage returns a stable Render page across one or more authorized
// workspaces. ownerIDs are repeated ownerId query values; absent means the
// caller's resolved default workspace.
func (s *Service) ListPage(ctx context.Context, ownerIDs []string, cursor string, limit int) ([]EndpointView, error) {
	if len(ownerIDs) == 0 {
		ownerIDs = []string{""}
	}
	ownerSet := make(map[string]bool, len(ownerIDs))
	normalizedOwners := make([]string, 0, len(ownerIDs))
	for _, ownerID := range ownerIDs {
		ownerID = strings.TrimSpace(ownerID)
		if ownerSet[ownerID] {
			continue
		}
		ownerSet[ownerID] = true
		normalizedOwners = append(normalizedOwners, ownerID)
	}
	if len(normalizedOwners) > core.MaxPageLimit {
		return nil, core.NewBadRequestError("WEBHOOK_OWNER_FILTER_LIMIT",
			fmt.Sprintf("ownerId accepts at most %d distinct values", core.MaxPageLimit),
			map[string]any{"limit": core.MaxPageLimit})
	}
	tenantSet := make(map[string]bool, len(normalizedOwners))
	for _, ownerID := range normalizedOwners {
		scoped := core.WithWorkspace(ctx, ownerID)
		if err := s.Authorize(scoped, core.RelCanView); err != nil {
			return nil, err
		}
		tenantID := ownerID
		if tenantID == "" {
			tenantID = s.WorkspaceOrDefault(scoped)
		}
		tenantSet[tenantID] = true
	}
	if s.Store == nil {
		return nil, core.ErrWebhooksUnavailable
	}
	after, err := core.DecodeKeysetCursor(cursor)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListWebhookEndpoints(ctx, slices.Sorted(maps.Keys(tenantSet)), after.At, after.Key, limit)
	if err != nil {
		return nil, err
	}
	// Resolve the exact-URL privilege per tenant once (a page can span several
	// authorized workspaces): only can_manage callers see the full destination.
	exact := make(map[string]bool, len(tenantSet))
	for tenantID := range tenantSet {
		exact[tenantID] = s.mayManageWorkspace(ctx, tenantID)
	}
	out := make([]EndpointView, 0, len(rows))
	for _, e := range rows {
		out = append(out, toView(e, exact[e.TenantID]))
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
	e, err := s.Store.GetWebhookEndpoint(ctx, s.WorkspaceOrDefault(ctx), id)
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	return toView(e, s.mayManageWorkspace(ctx, e.TenantID)), nil
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
	e, err := s.Store.SetWebhookEndpointEnabled(ctx, s.WorkspaceOrDefault(ctx), id, enabled, "disabled manually")
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	return toView(e, true), nil // admin-gated verb: echo the exact destination
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
	return mapStoreErr(s.Store.DeleteWebhookEndpoint(ctx, s.WorkspaceOrDefault(ctx), id))
}

// ListDeliveries returns one endpoint's delivery history, newest first,
// keyset-paged. The endpoint is fetched (workspace-scoped) first, so a
// cross-workspace endpoint id 404s before any history is read. Member read.
func (s *Service) ListDeliveries(ctx context.Context, ownerID, endpointID, cursor string, limit int) ([]DeliveryView, error) {
	return s.ListDeliveriesFiltered(ctx, ownerID, endpointID, DeliveryFilter{Cursor: cursor, Limit: limit})
}

// DeliveryFilter is Render's webhook-event history filter. SentAfter and
// SentBefore are strict bounds on the immutable first-attempt timestamp.
type DeliveryFilter struct {
	Cursor     string
	Limit      int
	SentAfter  time.Time
	SentBefore time.Time
	Status     string
}

func (s *Service) ListDeliveriesFiltered(ctx context.Context, ownerID, endpointID string, filter DeliveryFilter) ([]DeliveryView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWebhooksUnavailable
	}
	if _, err := s.Store.GetWebhookEndpoint(ctx, s.WorkspaceOrDefault(ctx), endpointID); err != nil {
		return nil, mapStoreErr(err)
	}
	after, err := core.DecodeKeysetCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	if filter.Status != "" && filter.Status != DeliveryDelivered && filter.Status != DeliveryFailed {
		return nil, core.NewBadRequestError("WEBHOOK_DELIVERY_STATUS_INVALID",
			"status must be delivered or failed", map[string]any{"field": "status"})
	}
	// The store clamps limit to the shared page bounds (the ListServiceEvents
	// convention) — no second feature-side clamp.
	rows, err := s.Store.ListWebhookDeliveries(ctx, store.WebhookDeliveryFilter{
		EndpointID: endpointID, SentAfter: filter.SentAfter, SentBefore: filter.SentBefore,
		Status: filter.Status, AfterAt: after.At, AfterKey: after.Key, Limit: filter.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DeliveryView, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDeliveryView(d))
	}
	return out, nil
}

// UpdateRequest is Update's input — Render's sparse PATCH (w3/m27):
// any nil field means "keep the current value". Each adapter maps its
// surface-native subscription field onto EventTypes before calling Update.
type UpdateRequest struct {
	Name       *string
	URL        *string
	EventTypes *[]string
	Enabled    *bool
}

// Update applies a sparse update to an endpoint (Render's PATCH contract,
// w3/m27). Non-nil fields replace the current value; omitted fields keep it.
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
	cur, err := s.Store.GetWebhookEndpoint(ctx, s.WorkspaceOrDefault(ctx), id)
	if err != nil {
		return EndpointView{}, mapStoreErr(err)
	}
	// Merge: keep current values for omitted fields.
	name := cur.Name
	if req.Name != nil {
		if name, err = normalizeName(*req.Name); err != nil {
			return EndpointView{}, err
		}
	}
	dest := cur.URL
	if req.URL != nil {
		if dest, err = parseDestination(*req.URL); err != nil {
			return EndpointView{}, err
		}
	}
	types := cur.EventTypes
	if req.EventTypes != nil {
		if types, err = normalizeEventTypes(*req.EventTypes); err != nil {
			return EndpointView{}, err
		}
	}
	enabled := cur.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	e, err := s.Store.UpdateWebhookEndpoint(ctx, s.WorkspaceOrDefault(ctx), id, name, dest, types, enabled)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return EndpointView{}, core.NewConflictError(WebhookNameConflictCode,
				"a webhook with this name already exists in the workspace", nil)
		}
		return EndpointView{}, mapStoreErr(err)
	}
	return toView(e, true), nil // admin-gated verb: echo the exact destination
}

// parseDestination validates a destination URL: absolute HTTPS, with a host
// and no userinfo. Returned trimmed — what the store keeps and the sender
// POSTs to. URL userinfo is refused outright (round-13 #7, the repo-URL
// invariant in store/api.go): a credential embedded in the URL would be stored
// verbatim and echoed to every workspace viewer through the read verbs.
func parseDestination(raw string) (string, error) {
	dest := strings.TrimSpace(raw)
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", core.NewBadRequestError(WebhookURLInvalidCode,
			"url must be an absolute HTTPS URL", map[string]any{"field": "url"})
	}
	if u.User != nil {
		return "", core.NewBadRequestError(WebhookURLInvalidCode,
			"url must not embed credentials in userinfo; provider tokens belong in the destination's own auth layer",
			map[string]any{"field": "url"})
	}
	return dest, nil
}

// RedactedURL is the viewer-safe projection of a stored destination (round-13
// #7): the exact path/query of a webhook destination commonly carries the
// reusable capability itself (Slack/T000/B000/xxx, PagerDuty routing keys,
// ?token=…), and list/get are member reads — so anything beyond the
// scheme://host origin is collapsed. Admins (can_manage) still see the exact
// URL they configured.
func RedactedURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw // unparseable stored value: never improved, never widened
	}
	if (u.Path == "" || u.Path == "/") && u.RawQuery == "" && u.Fragment == "" && u.User == nil {
		return u.Scheme + "://" + u.Host
	}
	return u.Scheme + "://" + u.Host + "/…"
}

// mayManageWorkspace reports whether the caller holds can_manage on the named
// tenant, with an authoritative (uncached) decision when the checker supports
// it — this decides whether the EXACT credential-bearing destination URL is
// revealed, so a just-demoted admin must not ride a cached positive. Raw
// checker, not Authorize: the read verbs' gate stays can_view (audited there),
// and a per-viewer denial audit row on every list would be noise, not signal
// (the sandbox isWorkspaceAdmin precedent).
func (s *Service) mayManageWorkspace(ctx context.Context, tenantID string) bool {
	if s.Authz == nil {
		return false // authz off: nobody is distinguished — redact for everyone
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return false
	}
	check := s.Authz.Check
	if fresh, ok := s.Authz.(core.FreshChecker); ok {
		check = fresh.CheckFresh
	}
	allowed, err := check(ctx, "user:"+id.Subject, core.RelCanManage, core.WorkspaceObject(tenantID))
	return err == nil && allowed
}

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", core.NewBadRequestError(WebhookNameInvalidCode,
			"name is required", map[string]any{"field": "name"})
	}
	return name, nil
}

// normalizeEventTypes validates and de-duplicates a subscription against the
// vocabulary, preserving EventTypes' canonical order. Empty is stored as the
// compact Render representation of an all-current-and-future-events subscription.
func normalizeEventTypes(types []string) ([]string, error) {
	if len(types) == 0 {
		return []string{}, nil
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
