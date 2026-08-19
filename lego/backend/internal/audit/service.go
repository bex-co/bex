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

// Package audit is the audit-log feature (w4/m10): a read verb over the
// audit_events control-plane table (internal/store) plus the retention sweep
// that bounds both ordinary events and SSH-session audit metadata. The WRITE side is core.Base.Audit
// (core.AuditSink, satisfied directly by *store.PGStore) wired in the
// composition root — this Service never writes an event, it only reads and
// prunes what core.Base.Authorize/AuthorizeOn already recorded. Requires the
// control-plane store (BEX_CP_DB_URI): with it unwired, List reports
// core.ErrAuditUnavailable (503) — omitted, not faked, the deploy-history
// precedent.
package audit

import (
	"context"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// AuditStore is the Service's seam to the control-plane store — the read and
// retention slice; the write side is core.AuditSink (*store.PGStore
// satisfies both, but this Service only ever calls these two).
type AuditStore interface {
	ListAuditEvents(ctx context.Context, workspaceID string, filter store.AuditFilter) ([]store.AuditRow, error)
	PurgeAuditEvents(ctx context.Context, before time.Time) (int64, error)
	PurgeSSHSessions(ctx context.Context, before time.Time) (int64, error)
}

// Service is the audit-log feature. Base carries the authz gate every verb
// calls first (List's own AuthorizeOn(RelCanManage, ...) below is itself
// audited — deliberately: viewing the audit trail is manage-tier, and a
// write-relation authorize call is exactly what core.Base records regardless
// of which verb triggers it. This is documented behavior, not a
// read-verb-leaked-into-the-log bug (docs/ADR006-bex-api.md § Audit log)).
type Service struct {
	*core.Base
	Store AuditStore
	// RetentionDays is how long an event survives before the sweep purges it
	// (BEX_AUDIT_RETENTION_DAYS). <1 means DefaultRetentionDays.
	RetentionDays int
}

// Event is the neutral projection of a store.AuditRow the adapters render in
// Render's audit-log shape.
type Event struct {
	ID                string
	Caller            string
	CallerMethod      string
	Verb              string
	Resource          string
	Target            string // the resource acted ON ("service:my-api"); Render's metadata carries it
	TargetName        string // the target's display name (migration 0038); "" on pre-0038 rows
	Outcome           string
	At                time.Time
	MaintenanceModeTo *bool
	// RoleFrom/RoleTo are the team-membership verbs' typed role detail
	// (w1/m33, migration 0040) — nil for every other verb.
	RoleFrom *string
	RoleTo   *string
	// Relation is the RelCan… the decision was made against. Empty on typed
	// system events and pre-0088 rows.
	Relation string
	// OAuthClientID / OAuthAudience / OAuthScopes are verified human-OAuth
	// grant facts. Empty on session, machine, system, and pre-0088 rows.
	OAuthClientID string
	OAuthAudience string
	OAuthScopes   []string
}

func view(r store.AuditRow) Event {
	return Event{
		ID:                r.ID,
		Caller:            r.Caller,
		CallerMethod:      r.CallerMethod,
		Verb:              r.Verb,
		Resource:          r.Resource,
		Target:            r.Target,
		TargetName:        r.TargetName,
		Outcome:           r.Outcome,
		At:                r.At,
		MaintenanceModeTo: r.MaintenanceModeTo,
		RoleFrom:          r.RoleFrom,
		RoleTo:            r.RoleTo,
		Relation:          r.Relation,
		OAuthClientID:     r.OAuthClientID,
		OAuthAudience:     r.OAuthAudience,
		OAuthScopes:       r.OAuthScopes,
	}
}

// Render's direction vocabulary (api-docs.render.com list-owner-audit-logs) —
// the shared enum core.ParseDirection validates for every list surface that
// honors it, aliased here for the adapters' convenience.
const (
	DirectionBackward = core.DirectionBackward
	DirectionForward  = core.DirectionForward
)

// renderEvents maps a bex verb onto Render's audit-log event vocabulary
// (docs/render-artifacts/audit-logs-api.md, w4/m26) where an unambiguous 1:1
// exists — the lifecycle verbs plus the ip-allow-list and service-rename
// writes; every other verb passes through renderEvent as bex's own
// "<feature>.<Verb>" event value — a documented extension, never a guessed
// mapping onto a Render name whose semantics might not match. The SINGLE
// source of this vocabulary, consumed by both the REST and GraphQL fragments
// (the events feed's eventTypes and webhooks' verbEvents are its snake_case
// siblings for their own surfaces); TestRenderEventVerbsExist guards the
// keys against silent service-method renames.
var renderEvents = map[string]string{
	"apps.Create":             "CreateServerEvent",
	"apps.Delete":             "DeleteServerEvent",
	"apps.Suspend":            "SuspendServiceEvent",
	"apps.Resume":             "ResumeServiceEvent",
	"apps.SetDisplayName":     "UpdateServiceNameEvent",
	"apps.SetIPAllowList":     "UpdateIPAllowListEvent",
	"postgres.CreatePostgres": "CreatePostgresEvent",
	"postgres.DeletePostgres": "DeletePostgresEvent",
	"postgres.Suspend":        "SuspendPostgresEvent",
	"postgres.Resume":         "ResumePostgresEvent",
	"postgres.SetIPAllowList": "UpdateIPAllowListEvent",
	"keyvalue.CreateKeyValue": "CreateRedisEvent",
	"keyvalue.DeleteKeyValue": "DeleteRedisEvent",
	"keyvalue.SetIPAllowList": "UpdateIPAllowListEvent",
	"projects.Create":         "CreateProjectEvent",
	"projects.Delete":         "DeleteProjectEvent",
	"workspaces.Create":       "CreateWorkspaceEvent",
	"workspaces.Delete":       "DeleteWorkspaceEvent",
	"environments.Create":     "CreateEnvironmentEvent",
	"environments.Delete":     "DeleteEnvironmentEvent",

	core.AuditVerbMaintenanceModeEnabled:    "MaintenanceModeEnabledEvent",
	core.AuditVerbMaintenanceModeURIUpdated: "MaintenanceModeURIUpdatedEvent",
}

func renderEvent(verb string) string {
	if ev, ok := renderEvents[verb]; ok {
		return ev
	}
	return verb
}

// denied is the one place the package reads the binary outcome vocabulary —
// REST's success|error mapping, its metadata denied marker, and GraphQL's
// richer denied status all consult it, so the three can't drift if the
// outcome vocabulary ever grows.
func denied(e Event) bool { return e.Outcome == string(core.AuditDenied) }

// Filter narrows List — the neutral shape the REST/GraphQL adapters translate
// Render's startTime/endTime/direction/cursor/limit query params into.
type Filter struct {
	Since time.Time
	Until time.Time
	// Direction is Render's ordering enum: "" or DirectionBackward is
	// newest-first, DirectionForward oldest-first; anything else is a named
	// 400 (w3/m8's "nothing accepted is ignored" — w4/013 closed the drift
	// where this param was accepted and ignored). Validated here in the verb
	// so REST and GraphQL cannot diverge.
	Direction string
	Cursor    string
	// Limit is clamped, not rejected, when non-positive or above the audit cap
	// (both fall back to the default page, not to the cap): the
	// store substitutes a real default page (store.DefaultAuditPageSize)
	// rather than reading <=0 as "unbounded", which is the discriminator
	// core.ErrLimitNotPositive documents between the two limit policies. The
	// deploys/jobs lists reject for exactly the opposite reason.
	Limit int
}

// FilterOf builds a Filter from the params in the string form every adapter
// has them in (a query value, a GraphQL argument) — one translator for both
// surfaces, the deploys.FilterOf/jobs.FilterFromStrings precedent, so a REST
// call and a GraphQL query with the same params cannot page differently.
//
// A malformed time bound is a named 400, NOT a silently dropped filter. Both
// adapters used to parse these inline with `if err == nil { assign }`, so
// `?startTime=yesterday` answered with the store's default page of the
// caller's whole history, UNFILTERED by the window they asked for — a wrong
// answer shaped exactly like the right one, with no error to notice. (The read
// was never row-unbounded: Limit stayed 0 and the store substitutes
// store.DefaultAuditPageSize. It is the dropped FILTER that is the defect.)
//
// That is the same rule the Direction field above already carries (w3/m8,
// "nothing accepted is ignored"); it had simply never been applied to the time
// bounds beside it.
func FilterOf(startTime, endTime, direction, cursor string, limit int) (Filter, error) {
	f := Filter{Direction: direction, Cursor: cursor, Limit: limit}
	var err error
	if f.Since, err = core.ParseTime("startTime", startTime); err != nil {
		return Filter{}, err
	}
	if f.Until, err = core.ParseTime("endTime", endTime); err != nil {
		return Filter{}, err
	}
	return f, nil
}

// List returns ownerID's audit trail — newest-first unless
// filter.Direction is DirectionForward (Render's GET
// /owners/{id}/audit-logs) — admin-scoped (RelCanManage), the same bar
// workspace rename/delete uses: only a workspace's own admins may see who did
// what to it. ownerID must be the caller's own workspace — AuthorizeOn checks
// the named object directly, so a caller from another workspace naming this
// one by id gets ErrForbidden, never a leak of its trail through a guessed id.
func (s *Service) List(ctx context.Context, ownerID string, filter Filter) ([]Event, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(ownerID)); err != nil {
		return nil, err
	}
	oldestFirst, err := core.ParseDirection(filter.Direction)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrAuditUnavailable
	}
	rows, err := s.Store.ListAuditEvents(ctx, ownerID, store.AuditFilter{
		Since: filter.Since, Until: filter.Until, Cursor: filter.Cursor, Limit: filter.Limit,
		OldestFirst: oldestFirst,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = view(r)
	}
	return out, nil
}

// DefaultRetentionDays is the default retention window (t002): 90 days
// matches Render's documented minimum audit-log retention
// (render.com/docs/audit-logs), a reasonable bex default absent its own
// compliance requirement.
const DefaultRetentionDays = 90

// RetentionInterval is the sweep cadence — daily, the same cadence usage's
// compaction loop (w8/m4) uses for its retention pass.
const RetentionInterval = 24 * time.Hour

// Run starts the retention sweep loop; blocks until ctx is cancelled. Only
// meaningful with Store wired (BEX_CP_DB_URI) — cmd/api/main.go starts this
// in a goroutine alongside the usage retention loop.
func (s *Service) Run(ctx context.Context) {
	s.RunWithInterval(ctx, RetentionInterval)
}

func (s *Service) RunWithInterval(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.purge(ctx) // once at startup so restarts don't defer the first sweep a full interval
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.purge(ctx)
		}
	}
}

func (s *Service) purge(ctx context.Context) {
	days := s.RetentionDays
	if days < 1 {
		days = DefaultRetentionDays
	}
	before := s.Now().UTC().AddDate(0, 0, -days)
	events, err := s.Store.PurgeAuditEvents(ctx, before)
	if err != nil {
		log.Printf("audit: purge events before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	sessions, err := s.Store.PurgeSSHSessions(ctx, before)
	if err != nil {
		log.Printf("audit: purge SSH sessions before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	if events+sessions > 0 {
		log.Printf("audit: purged %d events and %d SSH sessions older than %s", events, sessions, before.Format(time.RFC3339))
	}
}
