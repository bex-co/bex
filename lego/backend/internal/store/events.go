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

// events.go is the read view behind the per-service events feed (w3/m7,
// internal/events). It composes two tables bex already writes and adds no third:
//
//   - deploys      — one row per rollout; its created_at and finished_at are two
//     transitions, so one row projects into up to TWO events.
//   - audit_events — one row per authorized write verb, since w3/m7 carrying the
//     TARGET it acted on (core.ServiceTarget), which is what makes it
//     attributable to a service at all.
//
// The merge is done in SQL (one UNION ALL, one ORDER BY, one LIMIT), not in Go:
// paging a two-source feed from Go would mean over-fetching both sides on every
// page and re-merging, and the keyset below is exactly what Postgres is good at.
// The event VOCABULARY (which verb is which event type) stays in Go — this layer
// never interprets a verb, it only filters on the sets the caller passes in.

// Event source discriminators — which table an event row was projected from.
const (
	EventSourceDeploy = "deploy"
	EventSourceAudit  = "audit"
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
	// Source is EventSourceDeploy or EventSourceAudit.
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
	// AutoDeploy pushes down the auto-deploy boolean discrimination into SQL when
	// the Verbs set includes apps.SetAutoDeploy. AutoDeployFilterNone (zero value)
	// means no additional constraint on auto_deploy_enabled.
	AutoDeploy AutoDeployFilter
	// Limit caps the page (<1 or >core.MaxPageLimit clamps to core.DefaultPageLimit).
	Limit int
}

// serviceEventsQuery is the composed view: two projections of `deploys` and one
// of `audit_events`, ordered by the feed's total order and paged by keyset.
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
           d.image                             AS image,
           d.commit                            AS commit_id,
           d.commit_message                    AS commit_message,
           d.started_at                        AS started_at,
           d.finished_at                       AS finished_at
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
           d.image,
           d.commit,
           d.commit_message,
           d.started_at,
           d.finished_at
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
           ''::text,
           ''::text,
           ''::text,
           NULL::timestamptz,
           NULL::timestamptz
    FROM audit_events a
    WHERE a.target = $2
      AND a.outcome = 'allowed'
      AND a.workspace_id = ANY($3)
      AND a.verb = ANY($4)
      AND ($11::smallint IS NULL
           OR ($11 = 1 AND a.auto_deploy_enabled = true)
           OR ($11 = 2 AND a.auto_deploy_enabled = false)
           OR ($11 = 3 AND a.auto_deploy_enabled IS NULL))
)
SELECT key, at, source, phase, deploy_id, trigger, status, pre_deploy_status, verb, caller,
       plan_from, plan_to, instance_count_from, instance_count_to,
       autoscaling_min_from, autoscaling_max_from, autoscaling_min_to, autoscaling_max_to,
       auto_deploy_enabled, image, commit_id, commit_message, started_at, finished_at
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
		limit, nullAutoDeployFilter(f.AutoDeploy))
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
	err := row.Scan(&r.Key, &r.At, &r.Source, &r.Phase, &r.DeployID, &r.Trigger, &r.Status, &r.PreDeployStatus, &r.Verb, &r.Caller,
		&r.PlanFrom, &r.PlanTo, &r.InstanceCountFrom, &r.InstanceCountTo,
		&r.AutoscalingMinFrom, &r.AutoscalingMaxFrom, &r.AutoscalingMinTo, &r.AutoscalingMaxTo,
		&r.AutoDeployEnabled, &r.Image, &r.CommitID, &r.CommitMessage, &r.StartedAt, &r.FinishedAt)
	return r, err
}
