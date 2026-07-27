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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// SelectUnemittedUsage returns up to limit sealed usage_hourly rows that have
// not yet shipped to Stripe — the billing outbox read
// (docs/ADR040-billing-metronome.md §4). A row qualifies when its window is
// final (window_start < sealBefore, i.e. past the rewrite horizon), not below
// the billing floor (window_start >= floor), still un-emitted, and its
// workspace is not billing_excluded. Oldest first, so batches ship in order and
// the loop makes forward progress. The JOIN filters excluded tenants at the
// source: an excluded workspace's rows are never even considered for export.
func (s *PGStore) SelectUnemittedUsage(ctx context.Context, floor, sealBefore time.Time, limit int) ([]HourlyRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT u.workspace_id, u.service_id, u.kind, u.tier, u.resource_kind, u.window_start, u.quantity
		FROM usage_hourly u
		JOIN tenants t ON t.id = u.workspace_id
		WHERE u.emitted_at IS NULL
		  AND u.window_start >= $1
		  AND u.window_start <  $2
		  AND t.billing_excluded = false
		ORDER BY u.window_start
		LIMIT $3`,
		floor, sealBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HourlyRow
	for rows.Next() {
		var r HourlyRow
		if err := rows.Scan(&r.WorkspaceID, &r.ServiceID, &r.Kind, &r.Tier, &r.ResourceKind, &r.WindowStart, &r.Quantity); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkUsageEmitted stamps emitted_at on exactly the given rows — the outbox
// write after a successful ingest. Keyed by the full primary key
// (resource_kind, service_id, kind, tier, window_start), it only touches rows
// still un-emitted (emitted_at IS NULL), so a re-run after a crash between
// ingest and stamp is a safe no-op. A single unnest-driven statement keeps the
// whole batch atomic.
func (s *PGStore) MarkUsageEmitted(ctx context.Context, rows []HourlyRow, at time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	resourceKinds := make([]string, len(rows))
	serviceIDs := make([]string, len(rows))
	kinds := make([]string, len(rows))
	tiers := make([]string, len(rows))
	windows := make([]time.Time, len(rows))
	for i, r := range rows {
		resourceKinds[i] = NormalizeResourceKind(r.ResourceKind)
		serviceIDs[i] = r.ServiceID
		kinds[i] = r.Kind
		tiers[i] = r.Tier
		windows[i] = r.WindowStart
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE usage_hourly u SET emitted_at = $1
		FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[])
			AS k(resource_kind, service_id, kind, tier, window_start)
		WHERE u.resource_kind = k.resource_kind
		  AND u.service_id = k.service_id
		  AND u.kind = k.kind
		  AND u.tier = k.tier
		  AND u.window_start = k.window_start
		  AND u.emitted_at IS NULL`,
		at, resourceKinds, serviceIDs, kinds, tiers, windows)
	return err
}

// SetTenantBillingExcluded flips a workspace's billing-exclusion flag
// (docs/ADR040-billing-metronome.md §7, Mode A) and, when the value actually
// changes, records an audit_events row (verb billing.SetExclusion) attributing
// it to actor. This flag decides whether money is owed, so its only caller is
// the admin-only control-plane internal API — never a tenant. Returns whether
// the value changed (a no-op toggle writes no audit row); ErrNotFound when the
// workspace does not exist.
func (s *PGStore) SetTenantBillingExcluded(ctx context.Context, tenantID string, excluded bool, actor string, at time.Time) (bool, error) {
	var current bool
	if err := s.Pool.QueryRow(ctx,
		`SELECT billing_excluded FROM tenants WHERE id = $1`, tenantID,
	).Scan(&current); err != nil {
		return false, classify("tenant", err)
	}
	if current == excluded {
		return false, nil // idempotent no-op: no state change, no audit noise
	}
	if _, err := s.Pool.Exec(ctx,
		`UPDATE tenants SET billing_excluded = $2, updated_at = now() WHERE id = $1`,
		tenantID, excluded,
	); err != nil {
		return false, err
	}
	if actor == "" {
		actor = "control-plane"
	}
	if err := s.Record(ctx, core.AuditEvent{
		Caller:            actor,
		CallerMethod:      "control-plane",
		Verb:              core.AuditVerbBillingExclusionChanged,
		Resource:          core.WorkspaceObject(tenantID),
		Outcome:           core.AuditAllowed,
		At:                at,
		BillingExcludedTo: &excluded,
	}); err != nil {
		// The flag is set; a failed audit write must not lose that fact. Surface
		// it so the caller can retry the audit, but the exclusion already holds.
		return true, err
	}
	return true, nil
}
