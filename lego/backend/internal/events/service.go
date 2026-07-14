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

// Package events is the per-service activity feed (w3/m7): Render's
// GET /services/{id}/events, composed — not recorded a second time — from two
// tables bex already writes.
//
// # Sources
//
// Every event is DERIVED. bex added no event table and no event write path; it
// added one COLUMN (audit_events.target, w3/m7) so the rows it was already
// writing could say WHICH service they acted on:
//
//	deploys       → deploy_started (created_at) and deploy_ended (finished_at)
//	audit_events  → every other type, one row per authorized write verb, keyed by
//	                target = core.ServiceTarget(app) and mapped through eventTypes
//
// # Vocabulary (Render's names where its OpenAPI defines one, bex-named otherwise)
//
// Render's list-events enum has 39 types; bex emits the subset its two sources
// can support truthfully, under Render's exact names:
//
//	deploy_started              deploys row opened
//	deploy_ended                deploys row closed (details.deployStatus succeeded|failed)
//	suspender_added             apps.Suspend      (details.actor = the caller)
//	suspender_removed           apps.Resume       (details.actor = the caller)
//	server_restarted            apps.Restart      (details.triggeredByUser = the caller)
//	plan_changed                apps.SetPlan
//	instance_count_changed      apps.Scale
//	autoscaling_config_changed  apps.SetAutoscaling / apps.DeleteAutoscaling
//	cron_job_run_started        apps.TriggerCronRun
//
// and these bex-named types, for writes Render's vocabulary has no name for:
//
//	env_vars_changed            secrets.Set/DeleteEnvVar(s)     (KEYS and VALUES both absent — see Redaction)
//	env_group_linked/unlinked   envgroups.Link/UnlinkService
//	auto_deploy_changed         apps.SetAutoDeploy              (Render splits enabled/disabled; see Omissions)
//	idle_timeout_changed        apps.SetIdleTTL                 (a bex-only feature: "sleep = free")
//	root_directory_changed      apps.SetRootDir
//	publish_path_changed        apps.SetPublishPath
//	routes_changed              apps.SetRoutes
//	headers_changed             apps.SetHeaders
//	custom_domain_added/removed apps.Add/DeleteDomain
//
// # Redaction (structural, not filtered)
//
// No value can reach an event because no value is ever stored: an audit row
// carries a verb NAME, a caller subject, and (since w3/m7) a target resource
// NAME — never a verb's arguments (core.AuditEvent). An env-var write is
// therefore visible as "alice changed env vars at 03:12" and cannot be anything
// more, however the feed is queried. That is the guarantee; the missing detail
// below is its price, paid deliberately.
//
// # Omissions (honest, not faked)
//
//   - details.from/to on plan_changed, instance_count_changed,
//     autoscaling_config_changed — Render marks them required; bex omits them.
//     Carrying them would mean a free-form details column on audit_events, which
//     is precisely the hole w4/m10 closed to make the guarantee above structural.
//     The fact, the time, and the actor are truthful; the before/after values are
//     not recoverable from what bex stores.
//   - auto_deploy_enabled / auto_deploy_disabled — the two Render types are
//     discriminated by the verb's ARGUMENT, which is not recorded; bex emits one
//     honest auto_deploy_changed rather than guessing which happened.
//   - Auto-sleep / wake and autoscaler-driven replica changes — the operator
//     drives them and is DB-free by architecture (docs/ADR003-control-plane.md:
//     mechanism never writes the control plane), and it records no Kubernetes
//     Events either, so these transitions have NO durable source. Recording them
//     would mean giving the operator a control-plane write path — a layering
//     inversion, not a light write. Omitted until an operator→API event channel
//     exists; manual scale (apps.Scale → instance_count_changed) IS covered.
//   - Push-triggered redeploys open no deploys row today (w2/m5's scope), so they
//     produce no deploy_started; when they do, this feed shows them with no
//     change here.
//   - build_started/build_ended, server_failed, disk_*, maintenance_* — no bex
//     source at all.
//
// Requires the control-plane store (BEX_CP_DB_URI): both sources live there, so
// with it unwired the verb reports core.ErrEventsUnavailable (503) — omitted, not
// faked, the deploy-history precedent.
package events

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// EventStore is the Service's seam to the control-plane store — the one composed
// read (internal/store/events.go). *store.PGStore satisfies it.
type EventStore interface {
	ListServiceEvents(ctx context.Context, appID, target, ownerWorkspace string, f store.ServiceEventFilter) ([]store.ServiceEventRow, error)
}

// Render's event types (the subset bex can emit truthfully), spelled exactly as
// its list-events enum spells them — a Render-trained client switches on these.
const (
	TypeDeployStarted            = "deploy_started"
	TypeDeployEnded              = "deploy_ended"
	TypeSuspenderAdded           = "suspender_added"
	TypeSuspenderRemoved         = "suspender_removed"
	TypeServerRestarted          = "server_restarted"
	TypePlanChanged              = "plan_changed"
	TypeInstanceCountChanged     = "instance_count_changed"
	TypeAutoscalingConfigChanged = "autoscaling_config_changed"
	TypeCronJobRunStarted        = "cron_job_run_started"
)

// bex-named types — real writes Render's vocabulary has no name for. Named in
// Render's snake_case house style so they read as one vocabulary.
const (
	TypeEnvVarsChanged       = "env_vars_changed"
	TypeEnvGroupLinked       = "env_group_linked"
	TypeEnvGroupUnlinked     = "env_group_unlinked"
	TypeAutoDeployChanged    = "auto_deploy_changed"
	TypeIdleTimeoutChanged   = "idle_timeout_changed"
	TypeRootDirectoryChanged = "root_directory_changed"
	TypePublishPathChanged   = "publish_path_changed"
	TypeRoutesChanged        = "routes_changed"
	TypeHeadersChanged       = "headers_changed"
	TypeCustomDomainAdded    = "custom_domain_added"
	TypeCustomDomainRemoved  = "custom_domain_removed"
)

// eventTypes maps an audited verb (core.callerVerb's "<package>.<Method>") to the
// event type it produces. It is the SINGLE source of the vocabulary: the store
// filters audit rows to exactly these verbs (auditVerbs), so a verb absent here
// is not an event and a page is never silently short.
//
// Deliberately absent:
//   - apps.Create — its first deploy already appears as deploy_started with
//     trigger.firstBuild, which is how Render itself shows a service's birth.
//   - apps.Delete — the service (and its feed) is gone; the row survives in the
//     workspace audit log, which is where "who deleted it" belongs.
//   - deploys.Trigger — the deploys row it opens IS the deploy_started event;
//     mapping the verb too would show every API deploy twice.
var eventTypes = map[string]string{
	"apps.Suspend":            TypeSuspenderAdded,
	"apps.Resume":             TypeSuspenderRemoved,
	"apps.Restart":            TypeServerRestarted,
	"apps.SetPlan":            TypePlanChanged,
	"apps.Scale":              TypeInstanceCountChanged,
	"apps.SetAutoscaling":     TypeAutoscalingConfigChanged,
	"apps.DeleteAutoscaling":  TypeAutoscalingConfigChanged,
	"apps.TriggerCronRun":     TypeCronJobRunStarted,
	"apps.SetAutoDeploy":      TypeAutoDeployChanged,
	"apps.SetIdleTTL":         TypeIdleTimeoutChanged,
	"apps.SetRootDir":         TypeRootDirectoryChanged,
	"apps.SetPublishPath":     TypePublishPathChanged,
	"apps.SetRoutes":          TypeRoutesChanged,
	"apps.SetHeaders":         TypeHeadersChanged,
	"apps.AddDomain":          TypeCustomDomainAdded,
	"apps.DeleteDomain":       TypeCustomDomainRemoved,
	"secrets.SetEnvVars":      TypeEnvVarsChanged,
	"secrets.SetEnvVar":       TypeEnvVarsChanged,
	"secrets.DeleteEnvVar":    TypeEnvVarsChanged,
	"envgroups.LinkService":   TypeEnvGroupLinked,
	"envgroups.UnlinkService": TypeEnvGroupUnlinked,
}

// allVerbs is eventTypes' key set and allPhases the two deploy transitions —
// the unfiltered query's push-down sets, computed once (they are constants).
var (
	allVerbs  = slices.Sorted(maps.Keys(eventTypes))
	allPhases = []string{store.EventPhaseStarted, store.EventPhaseEnded}
)

// pushDown translates a caller's event-TYPE filter into the two sets the store
// query takes: the audit verbs that produce that type, and the deploy phases
// that do. It is what keeps the type filter IN SQL — filtering the result in Go
// after the store's LIMIT would return short (sometimes empty) pages, which a
// cursor client reads as the end of the feed and stops on.
//
// An empty type asks for everything. An unknown type matches nothing (empty
// sets), which is an empty feed — not, as a Go-side filter would give, a page of
// zero items that a client cannot distinguish from the end.
func pushDown(eventType string) (verbs, phases []string) {
	switch eventType {
	case "":
		return allVerbs, allPhases
	case TypeDeployStarted:
		return nil, []string{store.EventPhaseStarted}
	case TypeDeployEnded:
		return nil, []string{store.EventPhaseEnded}
	}
	for _, verb := range allVerbs {
		if eventTypes[verb] == eventType {
			verbs = append(verbs, verb)
		}
	}
	return verbs, nil
}

// DefaultWindow is how far back an events query reaches when the caller names no
// startTime — Render's documented default for this endpoint (now-1h), matched so
// a Render client's unparameterized call means the same thing against bex. Ask
// for more with startTime.
const DefaultWindow = time.Hour

// Trigger is Render's deploy trigger object (deploy_started.details.trigger) —
// which of the mutually-exclusive causes started this rollout. bex fills the two
// its deploys table records ("create" ⇒ FirstBuild, "api" ⇒ Manual) and leaves
// the rest false rather than guessing.
type Trigger struct {
	FirstBuild       bool
	EnvUpdated       bool
	Manual           bool
	DeployedByRender bool
	ClearCache       bool
	Rollback         bool
}

// Details is the per-type payload, a closed struct rather than a free-form map:
// nothing can be attached to an event that this type does not already name, which
// is the same structural reason a value cannot reach one. Fields are populated
// only for the types that define them (see the package doc); everything else
// serializes as an empty object, matching Render's payload-less types.
type Details struct {
	// deploy_started / deploy_ended
	DeployID     string
	DeployStatus string   // "succeeded" | "failed" — Render's deployStatus enum
	Trigger      *Trigger // deploy_started only
	// suspender_added / suspender_removed
	Actor string
	// server_restarted
	TriggeredByUser string
}

// Event is the neutral projection every adapter renders. Cursor is the opaque
// position a client echoes back to resume — it encodes the feed's keyset
// (timestamp + row key), NOT the event id: the id is a hash of the source row,
// and a hash cannot be resumed from. It also outlives its row, so a cursor still
// pages correctly after the audit retention sweep deletes the event it named.
type Event struct {
	ID        string
	Type      string
	At        time.Time
	ServiceID string
	Details   Details
	Cursor    string
}

// Filter narrows List — the neutral shape the REST/GraphQL/MCP adapters translate
// Render's type/startTime/endTime/cursor/limit query params into.
type Filter struct {
	Type   string // one event type; empty ⇒ all
	Since  time.Time
	Until  time.Time
	Cursor string
	Limit  int
}

// FilterOf builds a Filter from the five params Render's endpoint takes, in the
// string form every adapter has them in (a query value, a GraphQL argument, an
// MCP tool field). One translator for all three, so a REST call and a tool call
// with the same params cannot page differently. An unparseable timestamp is left
// zero — the Service then applies Render's default window — rather than 400,
// which is the permissive reading every other bex list fragment takes.
func FilterOf(eventType, startTime, endTime, cursor string, limit int) Filter {
	f := Filter{Type: eventType, Cursor: cursor, Limit: pageLimit(limit)}
	if t, err := time.Parse(time.RFC3339, startTime); err == nil {
		f.Since = t
	}
	if t, err := time.Parse(time.RFC3339, endTime); err == nil {
		f.Until = t
	}
	return f
}

// pageLimit clamps a caller's limit to Render's bounds — the same clamp
// core.PageParams applies to a query string, for the two adapters that don't
// have one.
func pageLimit(limit int) int {
	if limit < 1 {
		return core.DefaultPageLimit
	}
	return min(limit, core.MaxPageLimit)
}

// Service is the events feature. Store nil (BEX_CP_DB_URI unset) ⇒ every read
// reports core.ErrEventsUnavailable: the feed's BOTH sources are control-plane
// tables, so there is nothing to degrade to.
type Service struct {
	*core.Base
	Store EventStore
	// MaxQueryHours, when positive, caps the startTime–endTime window a caller may
	// ask for (BEX_MAX_QUERY_HOURS) — the same bound logs and metrics enforce.
	// Enforced HERE, not in the REST fragment, so all three surfaces reject the
	// same window (logs/metrics check it adapter-side and their GraphQL surfaces
	// are correspondingly unbounded — a wart this feature does not copy).
	MaxQueryHours int
}

// List returns a service's activity feed, newest first (Render's
// GET /services/{id}/events). A hand-applied App has no control-plane row, hence
// no deploys and no audit target: an empty feed, not an error — the deploy-history
// precedent.
func (s *Service) List(ctx context.Context, service string, filter Filter) ([]Event, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, service)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrEventsUnavailable
	}
	since, until := filter.Since, filter.Until
	if since.IsZero() {
		since = s.Now().Add(-DefaultWindow)
	}
	if err := s.checkWindow(since, until); err != nil {
		return nil, err
	}
	after, err := decodeCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	appID := a.Labels[store.LabelAppID]
	if appID == "" {
		return []Event{}, nil
	}
	verbs, phases := pushDown(filter.Type)
	rows, err := s.Store.ListServiceEvents(ctx, appID, core.ServiceTarget(service), a.Labels[core.LabelTenant], store.ServiceEventFilter{
		Since:    since,
		Until:    until,
		AfterAt:  after.At,
		AfterKey: after.Key,
		Verbs:    verbs,
		Phases:   phases,
		Limit:    filter.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, view(r, service))
	}
	return out, nil
}

// checkWindow rejects a range wider than MaxQueryHours (BEX_MAX_QUERY_HOURS) —
// the same guard logs and metrics apply, mapped to core.ErrBadRequest so all
// three surfaces answer 400 identically.
func (s *Service) checkWindow(since, until time.Time) error {
	if s.MaxQueryHours <= 0 {
		return nil
	}
	if until.IsZero() {
		until = s.Now()
	}
	if until.Sub(since) > time.Duration(s.MaxQueryHours)*time.Hour {
		return fmt.Errorf("%w: query range exceeds %d hours", core.ErrBadRequest, s.MaxQueryHours)
	}
	return nil
}

// view projects one composed store row onto the Render event vocabulary. The id
// is DERIVED from the source row (ids.Derive), never minted: a client pages,
// re-fetches and dedupes on it, so the same source row must yield the same evt-…
// id on every read, forever.
func view(r store.ServiceEventRow, service string) Event {
	ev := Event{
		ID:        ids.Derive(ids.Event, r.Key),
		At:        r.At,
		ServiceID: service,
		Cursor:    encodeCursor(r.At, r.Key),
	}
	switch r.Source {
	case store.EventSourceDeploy:
		ev.Details.DeployID = r.DeployID
		if r.Phase == store.EventPhaseStarted {
			ev.Type = TypeDeployStarted
			ev.Details.Trigger = &Trigger{
				FirstBuild: r.Trigger == store.TriggerCreate,
				Manual:     r.Trigger == store.TriggerAPI,
				Rollback:   r.Trigger == store.TriggerRollback,
			}
		} else {
			ev.Type = TypeDeployEnded
			ev.Details.DeployStatus = deployStatus(r.Status)
		}
	case store.EventSourceAudit:
		ev.Type = eventTypes[r.Verb]
		switch ev.Type {
		case TypeSuspenderAdded, TypeSuspenderRemoved:
			ev.Details.Actor = r.Caller
		case TypeServerRestarted:
			ev.Details.TriggeredByUser = r.Caller
		}
	}
	return ev
}

// deployStatus maps bex's deploy status onto Render's deployStatus enum
// (succeeded|failed|canceled) — all three are live states since w2/m10 added
// Cancel, and a canceled deploy must not be reported as a failed one.
func deployStatus(status string) string {
	switch status {
	case store.DeployLive:
		return "succeeded"
	case store.DeployCanceled:
		return "canceled"
	default:
		return "failed"
	}
}
