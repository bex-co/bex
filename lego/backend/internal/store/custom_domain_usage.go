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
	"time"

	"github.com/jackc/pgx/v5"
)

// CustomDomainUsage is one workspace's custom domains that count toward its
// plan's included count (w7/m429, docs/ADR030 §16), with its plan.
type CustomDomainUsage struct {
	WorkspaceID string
	Plan        string
	// VerifiedAt is when each counted domain was verified, oldest first.
	VerifiedAt []time.Time
}

// countedCustomDomain selects the domains rows d that count as one custom
// domain each: verified, since an unverified claim never routes, and not the
// www/apex sibling bex adds itself (redirect_for_name set), which Render adds
// for the domain the customer added. A sibling the customer added explicitly
// has its redirect cleared and counts as its own domain.
const countedCustomDomain = `d.claim_state = 'verified' AND d.verified_at IS NOT NULL AND d.redirect_for_name IS NULL`

// CountedCustomDomains returns, per workspace, the custom domains verified
// before `before` that count toward its plan's included count, oldest first.
// workspaceID "" reads every workspace. A deleted domain's row is gone, so it
// counts for no hour metered after its deletion.
func (s *PGStore) CountedCustomDomains(ctx context.Context, before time.Time, workspaceID string) ([]CustomDomainUsage, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT a.tenant_id, t.plan, d.verified_at
		FROM domains d
		JOIN apps a ON a.id = d.app_id
		JOIN tenants t ON t.id = a.tenant_id
		WHERE `+countedCustomDomain+` AND `+serviceAppsOnly+`
		  AND d.verified_at < $1
		  AND ($2 = '' OR a.tenant_id = $2)
		ORDER BY a.tenant_id, d.verified_at, d.id`, before.UTC(), workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomDomainUsage
	for rows.Next() {
		var workspace, plan string
		var verifiedAt time.Time
		if err := rows.Scan(&workspace, &plan, &verifiedAt); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].WorkspaceID != workspace {
			out = append(out, CustomDomainUsage{WorkspaceID: workspace, Plan: plan})
		}
		last := &out[len(out)-1]
		last.VerifiedAt = append(last.VerifiedAt, verifiedAt.UTC())
	}
	return out, rows.Err()
}

// UsageMeterCursor is the start of the last hour window meter kind recorded
// as metered (usage_meter_cursors); ok is false before its first.
func (s *PGStore) UsageMeterCursor(ctx context.Context, kind string) (time.Time, bool, error) {
	var window time.Time
	err := s.Pool.QueryRow(ctx, `SELECT last_window FROM usage_meter_cursors WHERE kind = $1`, kind).Scan(&window)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return window.UTC(), true, nil
}

// AdvanceUsageMeterCursor records that meter kind has metered window. The
// cursor never moves back, so two replicas metering the same hours agree.
func (s *PGStore) AdvanceUsageMeterCursor(ctx context.Context, kind string, window time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO usage_meter_cursors (kind, last_window) VALUES ($1, $2)
		ON CONFLICT (kind) DO UPDATE
		SET last_window = GREATEST(usage_meter_cursors.last_window, EXCLUDED.last_window)`,
		kind, window.UTC().Truncate(time.Hour))
	return err
}
