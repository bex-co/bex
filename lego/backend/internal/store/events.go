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
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// events.go is the read view behind the per-service events feed. It composes
// three durable sources:
//
//   - deploys      — one row per rollout; its created_at and finished_at are two
//     transitions, so one row projects into up to TWO events.
//   - audit_events — one row per authorized write verb, since w3/m7 carrying the
//     TARGET it acted on (core.ServiceTarget), which is what makes it
//     attributable to a service at all.
//   - service_event_facts — closed, typed observations and signed-Git decisions
//     that are neither deploy-row timestamps nor authorized API writes.
//
// The merge is done in SQL (one UNION ALL, one ORDER BY, one LIMIT), not in Go:
// paging a multi-source feed from Go would mean over-fetching every side on every
// page and re-merging, and the keyset below is exactly what Postgres is good at.
// The event VOCABULARY (which verb is which event type) stays in Go — this layer
// never interprets a verb, it only filters on the sets the caller passes in.

// Event source discriminators — which table an event row was projected from.
const (
	EventSourceDeploy = "deploy"
	EventSourceAudit  = "audit"
	EventSourceFact   = "fact"
)

// Deploy-event phases: one deploys row yields a started event at created_at and,
// once terminal, an ended event at finished_at.
const (
	EventPhaseStarted = "started"
	EventPhaseEnded   = "ended"
)

// ServiceEventRow is one row of the composed feed — the raw projection, before
// internal/events maps it onto Render's event vocabulary. Deploy rows fill
// DeployID/Trigger/Status; audit rows fill Verb/Caller and the typed per-verb
// detail fields. No column here can carry a free-form value: deploy rows hold
// ids and a status enum, audit rows hold a verb name, a caller subject, and
// typed scalars mirroring audit_events' typed columns.
type ServiceEventRow struct {
	// Key is the row's stable identity within the feed: "<source row id>:<phase>"
	// for a deploy ("dep-abc:started"), "<audit row id>:" for an audit event. It
	// is the sort tiebreak and the cursor's second component — unique by
	// construction, since a source row id is unique and a row projects each phase
	// at most once. internal/events hashes it into the public evt-… id.
	Key string
	At  time.Time
	// Source is EventSourceDeploy, EventSourceAudit, or EventSourceFact.
	Source string
	// Phase is EventPhaseStarted/EventPhaseEnded for a deploy row; empty for audit.
	Phase string

	// Deploy rows only.
	DeployID string
	Trigger  string // "create" (the app's first deploy) | "api"; the started phase only
	Status   string // the deploy's terminal status; the ended phase only
	// PreDeployStatus is the deploy's pre-deploy step outcome (w1/m33): '' |
	// 'running' | 'succeeded' | 'failed'; the ended phase only.
	PreDeployStatus string
	// Deployed image URI; empty for non-deploy rows. (w1/m47)
	Image string
	// Commit ID (git revision); empty for non-deploy rows. (w1/m47)
	CommitID string
	// Commit message; empty for non-deploy rows. (w1/m47)
	CommitMessage string
	// When the deploy started executing; nil for non-deploy or not-yet-started. (w1/m47)
	StartedAt *time.Time
	// When the deploy finished (terminal status reached); nil for non-deploy or ongoing. (w1/m47)
	FinishedAt *time.Time

	// Audit rows only.
	Verb   string // e.g. "apps.Suspend"
	Caller string // core.Identity.Subject
	// Typed per-verb detail fields from audit_events — nil for every other verb.
	PlanFrom           *string
	PlanTo             *string
	InstanceCountFrom  *int32
	InstanceCountTo    *int32
	AutoscalingMinFrom *int32
	AutoscalingMaxFrom *int32
	AutoscalingMinTo   *int32
	AutoscalingMaxTo   *int32
	AutoDeployEnabled  *bool
	// service_moved placement pair (w6/m134): public prj-/env- ids, nil = no
	// placement on that side. Nil for every other verb.
	ProjectFrom     *string
	ProjectTo       *string
	EnvironmentFrom *string
	EnvironmentTo   *string

	// Typed service_event_facts columns. FactType is the closed discriminator;
	// all remaining values are bounded scalars used only by the types that own
	// them. Image and CommitID reuse the deploy columns above.
	FactType   string
	ReasonCode string
	InstanceID string
	FromCount  *int32
	ToCount    *int32
	BranchFrom string
	BranchTo   string
	CommitURL  string
	// FactStatus is a lifecycle-step fact's terminal outcome (build_ended,
	// pre_deploy_ended, job_run_ended): succeeded|failed|canceled, or "" for the
	// started/observed kinds. A distinct column from the deploy-arm Status above,
	// which carries a deploy row's terminal status (w7/m66).
	FactStatus string
}

// ServiceEventLookup is one globally-addressed event plus the resource identity
// materialized beside its source key. ServiceID is the canonical public id:
// apps.id (srv-…) for a service, or the typed dpg-/red- target for a datastore.
type ServiceEventLookup struct {
	Event     ServiceEventRow
	ServiceID string
}

// AutoDeployFilter constrains the auto_deploy_enabled column on the audit arm.
// It is used when the caller filters by an auto-deploy event type so that the
// discrimination (enabled vs disabled vs legacy changed) runs in SQL before the
// LIMIT rather than in Go after it. A Go-side drop after LIMIT would return
// short (or empty) pages, which a cursor client reads as end-of-feed.
type AutoDeployFilter int16

const (
	// AutoDeployFilterNone imposes no constraint — all auto_deploy_enabled values pass.
	AutoDeployFilterNone AutoDeployFilter = 0
	// AutoDeployFilterEnabled selects rows where auto_deploy_enabled = true.
	AutoDeployFilterEnabled AutoDeployFilter = 1
	// AutoDeployFilterDisabled selects rows where auto_deploy_enabled = false.
	AutoDeployFilterDisabled AutoDeployFilter = 2
	// AutoDeployFilterChanged selects rows where auto_deploy_enabled IS NULL (legacy rows).
	AutoDeployFilterChanged AutoDeployFilter = 3
)

// ServiceEventFilter narrows ListServiceEvents.
//
// Verbs and Phases are how the caller's event-TYPE filter is pushed down into
// SQL. They are the only two knobs the vocabulary needs, and passing them
// (rather than filtering the result in Go) is what keeps a page exactly `Limit`
// long: a Go-side filter after the LIMIT returns short — sometimes empty —
// pages, which a cursor client reads as the end of the feed and stops on.
// Empty Verbs excludes every audit row; empty Phases excludes every deploy row.
type ServiceEventFilter struct {
	// Since/Until bound At inclusively (Render's startTime/endTime).
	Since time.Time
	Until time.Time
	// AfterAt/AfterKey resume strictly after a previously returned event —
	// keyset paging on the feed's total order (At DESC, Key DESC), so a row
	// inserted between two pages shifts nothing already returned. Zero AfterAt
	// starts at the head.
	AfterAt  time.Time
	AfterKey string
	// Verbs are the audit verbs that map to an event type the caller asked for
	// (internal/events owns the mapping; the store never interprets a verb).
	Verbs []string
	// Phases are the deploy transitions the caller asked for (EventPhaseStarted
	// and/or EventPhaseEnded).
	Phases []string
	// FactTypes are the closed service_event_facts kinds requested by the caller.
	FactTypes []string
	// LegacyTarget is the old workspace-unique service:<public-name> audit key.
	// It is matched only inside ownerWorkspace, never workspace:default; current
	// writes use the namespace-unique CR-name target passed to ListServiceEvents.
	LegacyTarget string
	// AutoDeploy pushes down the auto-deploy boolean discrimination into SQL when
	// the Verbs set includes apps.SetAutoDeploy. AutoDeployFilterNone (zero value)
	// means no additional constraint on auto_deploy_enabled.
	AutoDeploy AutoDeployFilter
	// Limit caps the page (<1 or >core.MaxPageLimit clamps to core.DefaultPageLimit).
	Limit int
}

// serviceEventsQuery is the composed view: two projections of `deploys`, one of
// `audit_events`, and one of `service_event_facts`, ordered by the feed's total
// order and paged by keyset.
//
// Two predicates on the audit arm are NOT caller-supplied, because they are what
// make the feed truthful rather than merely filtered:
//
//   - outcome='allowed' — a DENIED authorize is audit-log material (who TRIED
//     what), not something that happened to the service.
//   - workspace_id = ANY(owner, default) — core.Base.AuthorizeTarget records the
//     target BEFORE the App is fetched, so a cross-tenant caller who names
//     someone else's service writes an allowed-looking row for a verb that then
//     403s and never happened. Their row carries THEIR workspace, so scoping to
//     the service's own tenant (plus the no-resolved-tenant default caller — the
//     platform bootstrap, and the authz-off dev mode) is what keeps a stranger
//     from injecting entries into someone else's activity feed.
//
// Both live here, in the only query that reads by target, rather than in the
// feature — a future reader of `target` (a Database feed, an export) inherits
// them instead of having to remember them.
const serviceEventsQuery = `
WITH feed AS (
    SELECT d.id || ':` + EventPhaseStarted + `' AS key,
           d.created_at                        AS at,
           '` + EventSourceDeploy + `'::text   AS source,
           '` + EventPhaseStarted + `'::text   AS phase,
           d.id                                AS deploy_id,
           d.trigger                           AS trigger,
           ''::text                            AS status,
           ''::text                            AS pre_deploy_status,
           ''::text                            AS verb,
           ''::text                            AS caller,
           NULL::text                          AS plan_from,
           NULL::text                          AS plan_to,
           NULL::integer                       AS instance_count_from,
           NULL::integer                       AS instance_count_to,
           NULL::integer                       AS autoscaling_min_from,
           NULL::integer                       AS autoscaling_max_from,
           NULL::integer                       AS autoscaling_min_to,
           NULL::integer                       AS autoscaling_max_to,
           NULL::boolean                       AS auto_deploy_enabled,
           NULL::text                          AS project_from,
           NULL::text                          AS project_to,
           NULL::text                          AS environment_from,
           NULL::text                          AS environment_to,
           d.image                             AS image,
           d.commit                            AS commit_id,
           d.commit_message                    AS commit_message,
           d.started_at                        AS started_at,
           d.finished_at                       AS finished_at,
           ''::text                            AS fact_type,
           ''::text                            AS reason_code,
           ''::text                            AS instance_id,
           NULL::integer                       AS fact_from_count,
           NULL::integer                       AS fact_to_count,
           ''::text                            AS branch_from,
           ''::text                            AS branch_to,
           ''::text                            AS commit_url,
           ''::text                            AS fact_status
    FROM deploys d
    WHERE d.app_id = $1 AND '` + EventPhaseStarted + `' = ANY($5)
  UNION ALL
    SELECT d.id || ':` + EventPhaseEnded + `',
           d.finished_at,
           '` + EventSourceDeploy + `'::text,
           '` + EventPhaseEnded + `'::text,
           d.id,
           ''::text,
           d.status,
           d.pre_deploy_status,
           ''::text,
           ''::text,
           NULL::text,
           NULL::text,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::boolean,
           NULL::text,
           NULL::text,
           NULL::text,
           NULL::text,
           d.image,
           d.commit,
           d.commit_message,
           d.started_at,
           d.finished_at,
           ''::text,
           ''::text,
           ''::text,
           NULL::integer,
           NULL::integer,
           ''::text,
           ''::text,
           ''::text,
           ''::text
    FROM deploys d
    WHERE d.app_id = $1 AND d.finished_at IS NOT NULL AND '` + EventPhaseEnded + `' = ANY($5)
  UNION ALL
    SELECT a.id || ':',
           a.at,
           '` + EventSourceAudit + `'::text,
           ''::text,
           ''::text,
           ''::text,
           ''::text,
           ''::text,
           a.verb,
           a.caller,
           a.plan_from,
           a.plan_to,
           a.instance_count_from,
           a.instance_count_to,
           a.autoscaling_min_from,
           a.autoscaling_max_from,
           a.autoscaling_min_to,
           a.autoscaling_max_to,
           a.auto_deploy_enabled,
           a.project_from,
           a.project_to,
           a.environment_from,
           a.environment_to,
           ''::text,
           ''::text,
           ''::text,
           NULL::timestamptz,
           NULL::timestamptz,
           ''::text,
           ''::text,
           ''::text,
           NULL::integer,
           NULL::integer,
           ''::text,
           ''::text,
           ''::text,
           ''::text
    FROM audit_events a
    WHERE ((a.target = $2 AND a.workspace_id = ANY($3))
           OR ($13::text <> '' AND a.target = $13 AND a.workspace_id = $14))
      AND a.outcome = 'allowed'
      AND a.verb = ANY($4)
      AND ($11::smallint IS NULL
           OR ($11 = 1 AND a.auto_deploy_enabled = true)
           OR ($11 = 2 AND a.auto_deploy_enabled = false)
           OR ($11 = 3 AND a.auto_deploy_enabled IS NULL))
  UNION ALL
    SELECT 'fact:' || f.source_key,
           f.at,
           '` + EventSourceFact + `'::text,
           ''::text,
           f.deploy_id,
           ''::text,
           ''::text,
           ''::text,
           ''::text,
           ''::text,
           NULL::text,
           NULL::text,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::integer,
           NULL::boolean,
           NULL::text,
           NULL::text,
           NULL::text,
           NULL::text,
           f.image,
           f.commit_id,
           ''::text,
           NULL::timestamptz,
           NULL::timestamptz,
           f.fact_type,
           f.reason_code,
           f.instance_id,
           f.from_count,
           f.to_count,
           f.branch_from,
           f.branch_to,
           f.commit_url,
           f.status
    FROM service_event_facts f
    WHERE f.app_id = $1 AND f.fact_type = ANY($12)
)
SELECT key, at, source, phase, deploy_id, trigger, status, pre_deploy_status, verb, caller,
       plan_from, plan_to, instance_count_from, instance_count_to,
       autoscaling_min_from, autoscaling_max_from, autoscaling_min_to, autoscaling_max_to,
       auto_deploy_enabled, project_from, project_to, environment_from, environment_to,
       image, commit_id, commit_message, started_at, finished_at,
       fact_type, reason_code, instance_id, fact_from_count, fact_to_count,
       branch_from, branch_to, commit_url, fact_status
FROM feed
WHERE ($6::timestamptz IS NULL OR at >= $6)
  AND ($7::timestamptz IS NULL OR at <= $7)
  AND ($8::timestamptz IS NULL OR (at, key) < ($8, $9))
ORDER BY at DESC, key DESC
LIMIT $10`

// ListServiceEvents returns one service's composed activity feed, newest first.
//
// appID is the app's control-plane row id (deploys are keyed by it); target is
// core.ServiceTarget(appName) (audit rows are keyed by it) — the two sources key
// on different identifiers for the same service, which is why both are passed
// rather than derived here. ownerWorkspace is the tenant that OWNS the service:
// the query scopes audit rows to it (see serviceEventsQuery), so it is a
// parameter of the read, not an option a caller may forget to set.
func (s *PGStore) ListServiceEvents(ctx context.Context, appID, target, ownerWorkspace string, f ServiceEventFilter) ([]ServiceEventRow, error) {
	limit := f.Limit
	if limit < 1 || limit > core.MaxPageLimit {
		limit = core.DefaultPageLimit
	}
	// The default workspace is always allowed: it is where a caller with no
	// resolved tenant lands (the platform bootstrap, and the authz-off dev mode),
	// and core.Base.GetApp lets such a caller act on the service — so its writes
	// belong in the feed of the service it changed.
	workspaces := []string{core.DefaultTenant}
	if ownerWorkspace != "" && ownerWorkspace != core.DefaultTenant {
		workspaces = append(workspaces, ownerWorkspace)
	}
	rows, err := s.Pool.Query(ctx, serviceEventsQuery,
		appID, target, workspaces, f.Verbs, f.Phases,
		nullTime(f.Since), nullTime(f.Until), nullTime(f.AfterAt), f.AfterKey,
		limit, nullAutoDeployFilter(f.AutoDeploy), f.FactTypes, f.LegacyTarget, ownerWorkspace)
	if err != nil {
		return nil, fmt.Errorf("list service events: %w", err)
	}
	defer rows.Close()
	var out []ServiceEventRow
	for rows.Next() {
		r, err := scanServiceEventRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// getServiceEventQuery starts from service_event_index's owner/event primary
// key, then follows the recorded source identity to one row. The index stores
// no details payload: deploy/audit/fact columns remain single-sourced and the
// projection is deliberately identical to ListServiceEvents' row shape.
const getServiceEventQuery = `
WITH hit AS MATERIALIZED (
    SELECT event_key, source, source_row_id, phase, service_id, app_id
    FROM service_event_index
    WHERE workspace_id = $1 AND event_id = $2
)
SELECT h.event_key AS key,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' AND h.phase = '` + EventPhaseStarted + `' THEN d.created_at
           WHEN h.source = '` + EventSourceDeploy + `' THEN d.finished_at
           WHEN h.source = '` + EventSourceAudit + `' THEN a.at
           ELSE f.at
       END AS at,
       h.source,
       h.phase,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' THEN d.id
           WHEN h.source = '` + EventSourceFact + `' THEN f.deploy_id
           ELSE ''
       END AS deploy_id,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' AND h.phase = '` + EventPhaseStarted + `' THEN d.trigger
           ELSE ''
       END AS trigger,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' AND h.phase = '` + EventPhaseEnded + `' THEN d.status
           ELSE ''
       END AS status,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' AND h.phase = '` + EventPhaseEnded + `' THEN d.pre_deploy_status
           ELSE ''
       END AS pre_deploy_status,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.verb ELSE '' END AS verb,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.caller ELSE '' END AS caller,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.plan_from END AS plan_from,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.plan_to END AS plan_to,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.instance_count_from END AS instance_count_from,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.instance_count_to END AS instance_count_to,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.autoscaling_min_from END AS autoscaling_min_from,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.autoscaling_max_from END AS autoscaling_max_from,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.autoscaling_min_to END AS autoscaling_min_to,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.autoscaling_max_to END AS autoscaling_max_to,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.auto_deploy_enabled END AS auto_deploy_enabled,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.project_from END AS project_from,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.project_to END AS project_to,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.environment_from END AS environment_from,
       CASE WHEN h.source = '` + EventSourceAudit + `' THEN a.environment_to END AS environment_to,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' THEN d.image
           WHEN h.source = '` + EventSourceFact + `' THEN f.image
           ELSE ''
       END AS image,
       CASE
           WHEN h.source = '` + EventSourceDeploy + `' THEN d.commit
           WHEN h.source = '` + EventSourceFact + `' THEN f.commit_id
           ELSE ''
       END AS commit_id,
       CASE WHEN h.source = '` + EventSourceDeploy + `' THEN d.commit_message ELSE '' END AS commit_message,
       CASE WHEN h.source = '` + EventSourceDeploy + `' THEN d.started_at END AS started_at,
       CASE WHEN h.source = '` + EventSourceDeploy + `' THEN d.finished_at END AS finished_at,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.fact_type ELSE '' END AS fact_type,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.reason_code ELSE '' END AS reason_code,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.instance_id ELSE '' END AS instance_id,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.from_count END AS fact_from_count,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.to_count END AS fact_to_count,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.branch_from ELSE '' END AS branch_from,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.branch_to ELSE '' END AS branch_to,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.commit_url ELSE '' END AS commit_url,
       CASE WHEN h.source = '` + EventSourceFact + `' THEN f.status ELSE '' END AS fact_status,
       COALESCE(h.app_id, h.service_id)
FROM hit h
LEFT JOIN deploys d
  ON h.source = '` + EventSourceDeploy + `' AND d.id = h.source_row_id
LEFT JOIN audit_events a
  ON h.source = '` + EventSourceAudit + `' AND a.id = h.source_row_id
LEFT JOIN service_event_facts f
  ON h.source = '` + EventSourceFact + `' AND f.source_key = h.source_row_id
WHERE (h.source = '` + EventSourceDeploy + `' AND d.id IS NOT NULL
       AND (h.phase = '` + EventPhaseStarted + `'
            OR (h.phase = '` + EventPhaseEnded + `' AND d.finished_at IS NOT NULL)))
   OR (h.source = '` + EventSourceAudit + `' AND a.id IS NOT NULL AND a.outcome = 'allowed')
   OR (h.source = '` + EventSourceFact + `' AND f.source_key IS NOT NULL)`

// GetServiceEvent returns one event only when the owner workspace and public id
// both match. A foreign id and an absent id therefore share ErrNotFound, and no
// query hashes or scans historical source rows at request time.
func (s *PGStore) GetServiceEvent(ctx context.Context, workspaceID, eventID string) (ServiceEventLookup, error) {
	var out ServiceEventLookup
	r := &out.Event
	err := s.Pool.QueryRow(ctx, getServiceEventQuery, workspaceID, eventID).Scan(
		serviceEventScanDestinations(r, &out.ServiceID)...,
	)
	if err != nil {
		return ServiceEventLookup{}, classify("service event", err)
	}
	return out, nil
}

// nullAutoDeployFilter maps AutoDeployFilterNone to SQL NULL so the $11
// predicate in serviceEventsQuery is a no-op when no discrimination is needed.
func nullAutoDeployFilter(f AutoDeployFilter) *int16 {
	if f == AutoDeployFilterNone {
		return nil
	}
	v := int16(f)
	return &v
}

// nullTime maps a zero time.Time to a SQL NULL, so one query text serves the
// bounded and unbounded cases (the `$n IS NULL OR …` guards above) instead of
// concatenating predicates per call.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func scanServiceEventRow(row pgx.Row) (ServiceEventRow, error) {
	var r ServiceEventRow
	err := row.Scan(serviceEventScanDestinations(&r)...)
	return r, err
}

func serviceEventScanDestinations(r *ServiceEventRow, trailing ...any) []any {
	destinations := []any{
		&r.Key, &r.At, &r.Source, &r.Phase, &r.DeployID, &r.Trigger, &r.Status, &r.PreDeployStatus, &r.Verb, &r.Caller,
		&r.PlanFrom, &r.PlanTo, &r.InstanceCountFrom, &r.InstanceCountTo,
		&r.AutoscalingMinFrom, &r.AutoscalingMaxFrom, &r.AutoscalingMinTo, &r.AutoscalingMaxTo,
		&r.AutoDeployEnabled, &r.ProjectFrom, &r.ProjectTo, &r.EnvironmentFrom, &r.EnvironmentTo,
		&r.Image, &r.CommitID, &r.CommitMessage, &r.StartedAt, &r.FinishedAt,
		&r.FactType, &r.ReasonCode, &r.InstanceID, &r.FromCount, &r.ToCount,
		&r.BranchFrom, &r.BranchTo, &r.CommitURL, &r.FactStatus,
	}
	return append(destinations, trailing...)
}
