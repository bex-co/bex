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

// DefaultAuditPageSize/MaxAuditPageSize bound ListAuditEvents' page —
// Render's audit-specific limit param (default 20, maximum 1000; captured
// from the public OpenAPI's auditLogLimitParam, w4/m26 — note the cap is
// audit-specific, NOT the general core.MaxPageLimit of 100).
const (
	DefaultAuditPageSize = 20
	MaxAuditPageSize     = 1000
)

// AuditRow is one persisted audit_events row.
type AuditRow struct {
	ID           string
	WorkspaceID  string
	Caller       string
	CallerMethod string
	Verb         string
	Resource     string
	// Target is the resource the verb acted ON ("service:my-api",
	// core.ServiceTarget) — empty for a workspace-wide verb. Resource is what the
	// verb was authorized AGAINST (the workspace); Target is what it changed.
	Target string
	// TargetName is the resource's display name when a typed datastore effect
	// needs it for Render's thin webhook payload.
	TargetName string
	Outcome    string
	At         time.Time
	// MaintenanceModeTo is Render's MaintenanceModeEnabledEvent metadata.to.
	// It is nil for every other audit verb.
	MaintenanceModeTo *bool
	// RoleFrom/RoleTo are the team-membership verbs' typed role detail
	// (w1/m33, migration 0040): ChangeRole records old→new, Invite/AcceptInvite
	// record RoleTo alone. Nil for every other verb and for pre-0040 rows.
	RoleFrom *string
	RoleTo   *string
	// Relation is the RelCan… the decision was made against. Empty on typed
	// system events and pre-0088 rows.
	Relation string
	// OAuthClientID / OAuthAudience / OAuthScopes are the verified grant
	// facts for a human OAuth caller. Empty on session, machine, system, and
	// pre-0088 rows.
	OAuthClientID string
	OAuthAudience string
	OAuthScopes   []string
}

// AuditFilter narrows ListAuditEvents: Since/Until bound At inclusively
// (Render's startTime/endTime); Cursor resumes strictly after a previously
// returned row's id (keyset paging, stable under concurrent inserts — unlike
// an OFFSET); Limit caps the page (<1 or >MaxAuditPageSize clamps to
// DefaultAuditPageSize). OldestFirst flips the order (Render's
// direction=forward, w4/013) — note a cursor's meaning follows the direction
// it is paged under: the same row id resumes strictly older rows backward and
// strictly newer rows forward.
type AuditFilter struct {
	Since       time.Time
	Until       time.Time
	Cursor      string
	Limit       int
	OldestFirst bool
}

// workspaceOf extracts the tenant component of an OpenFGA workspace object
// ("workspace:tea-abc" -> "tea-abc"). Every Authorize/AuthorizeOn call passes
// a core.WorkspaceObject, so this always matches; an unshaped resource (should
// never happen) falls back to the raw string rather than failing the write.
func workspaceOf(resource string) string {
	_, id, ok := strings.Cut(resource, ":")
	if !ok {
		return resource
	}
	return id
}

// Record persists one audit event — *PGStore structurally satisfies
// core.AuditSink (Record(ctx, core.AuditEvent) error), so the composition root
// wires it directly onto core.Base.Audit (cmd/api/main.go) with no adapter
// type needed.
func (s *PGStore) Record(ctx context.Context, ev core.AuditEvent) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO audit_events (id, workspace_id, caller, caller_method, verb, resource, target, target_name, outcome, at,
		    maintenance_mode_to, plan_from, plan_to, instance_count_from, instance_count_to,
		    autoscaling_min_from, autoscaling_max_from, autoscaling_min_to, autoscaling_max_to, auto_deploy_enabled,
		    role_from, role_to, billing_excluded_to,
		    relation, oauth_client_id, oauth_audience, oauth_scopes,
		    project_from, project_to, environment_from, environment_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31)`,
		ids.New(ids.Audit), workspaceOf(ev.Resource), ev.Caller, ev.CallerMethod, ev.Verb, ev.Resource, ev.Target, ev.TargetName, string(ev.Outcome), ev.At,
		ev.MaintenanceModeTo,
		ev.PlanFrom, ev.PlanTo,
		ev.InstanceCountFrom, ev.InstanceCountTo,
		ev.AutoscalingMinFrom, ev.AutoscalingMaxFrom, ev.AutoscalingMinTo, ev.AutoscalingMaxTo,
		ev.AutoDeployEnabled,
		ev.RoleFrom, ev.RoleTo, ev.BillingExcludedTo,
		nullIfEmpty(ev.Relation), nullIfEmpty(ev.OAuthClientID), nullIfEmpty(ev.OAuthAudience), nullIfEmptyScopes(ev.OAuthScopes),
		ev.ProjectFrom, ev.ProjectTo, ev.EnvironmentFrom, ev.EnvironmentTo)
	return err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullIfEmptyScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	return scopes
}

const auditColumns = `id, workspace_id, caller, caller_method, verb, resource, target, target_name, outcome, at, maintenance_mode_to, role_from, role_to, relation, oauth_client_id, oauth_audience, oauth_scopes`

func scanAuditRow(row pgx.Row) (AuditRow, error) {
	var r AuditRow
	var relation, oauthClientID, oauthAudience *string
	err := row.Scan(&r.ID, &r.WorkspaceID, &r.Caller, &r.CallerMethod, &r.Verb, &r.Resource, &r.Target, &r.TargetName, &r.Outcome, &r.At, &r.MaintenanceModeTo, &r.RoleFrom, &r.RoleTo, &relation, &oauthClientID, &oauthAudience, &r.OAuthScopes)
	if err != nil {
		return AuditRow{}, err
	}
	if relation != nil {
		r.Relation = *relation
	}
	if oauthClientID != nil {
		r.OAuthClientID = *oauthClientID
	}
	if oauthAudience != nil {
		r.OAuthAudience = *oauthAudience
	}
	return r, nil
}

// ListAuditEvents returns workspaceID's audit trail — newest-first by
// default, oldest-first with filter.OldestFirst — honoring filter's time
// bounds/cursor/limit — the one query the REST and GraphQL read fragments
// (internal/audit) both delegate to, so they can't diverge.
func (s *PGStore) ListAuditEvents(ctx context.Context, workspaceID string, filter AuditFilter) ([]AuditRow, error) {
	limit := filter.Limit
	if limit < 1 || limit > MaxAuditPageSize {
		limit = DefaultAuditPageSize
	}
	query := `SELECT ` + auditColumns + ` FROM audit_events WHERE workspace_id = $1`
	args := []any{workspaceID}
	if !filter.Since.IsZero() {
		args = append(args, filter.Since)
		query += fmt.Sprintf(" AND at >= $%d", len(args))
	}
	if !filter.Until.IsZero() {
		args = append(args, filter.Until)
		query += fmt.Sprintf(" AND at <= $%d", len(args))
	}
	query, args = pageKeyset(query, args, "audit_events", "at", filter.Cursor, limit, filter.OldestFirst)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		r, err := scanAuditRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PurgeAuditEvents deletes rows older than before — the retention sweep
// (internal/audit, BEX_AUDIT_RETENTION_DAYS) calls this, mirroring
// CompactUsage's atomic, idempotent delete.
func (s *PGStore) PurgeAuditEvents(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM audit_events WHERE at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
