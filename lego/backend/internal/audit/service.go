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
	Outcome           string
	At                time.Time
	MaintenanceModeTo *bool
}

func view(r store.AuditRow) Event {
	return Event{
		ID:                r.ID,
		Caller:            r.Caller,
		CallerMethod:      r.CallerMethod,
		Verb:              r.Verb,
		Resource:          r.Resource,
		Outcome:           r.Outcome,
		At:                r.At,
		MaintenanceModeTo: r.MaintenanceModeTo,
	}
}

// Render's direction vocabulary (api-docs.render.com list-owner-audit-logs) —
// the shared enum core.ParseDirection validates for every list surface that
// honors it, aliased here for the adapters' convenience.
const (
	DirectionBackward = core.DirectionBackward
	DirectionForward  = core.DirectionForward
)

// Filter narrows List — the neutral shape the REST/GraphQL adapters translate
// Render's startTime/endTime/direction/cursor/limit query params into.
type Filter struct {
	Since  time.Time
	Until  time.Time
	// Direction is Render's ordering enum: "" or DirectionBackward is
	// newest-first, DirectionForward oldest-first; anything else is a named
	// 400 (w3/m8's "nothing accepted is ignored" — w4/013 closed the drift
	// where this param was accepted and ignored). Validated here in the verb
	// so REST and GraphQL cannot diverge.
	Direction string
	Cursor    string
	Limit     int
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
