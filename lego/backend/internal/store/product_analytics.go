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
	"github.com/jackc/pgx/v5"
)

// RecordProductActivity enriches the durable, deduplicated successful effect.
// Subject locking prevents an in-flight recorder restoring identity after deletion.
//
// An unset surface falls back to the column's default rather than being sent as
// an empty string, which its CHECK would reject. This is defensive only: the
// sole caller reaches here through ObserveProductActivity, which clamps the
// value, and logs-and-swallows any error — so no value written here can fail a
// resource creation. It does not make arbitrary input safe; a direct caller
// passing "Dashboard" still violates the CHECK, and should.
func (s *PGStore) RecordProductActivity(ctx context.Context, e core.ProductActivity) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if e.ActorID != "" {
			if err := lockSubjectMembership(ctx, tx, e.ActorID); err != nil {
				return err
			}
			if err := refuseDeletingSubject(ctx, tx, e.ActorID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO product_activity_events
            (source_key,workspace_id,resource_id,parent_id,resource_type,event_type,at,actor_id,actor_type,surface)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,COALESCE(NULLIF($10,''),'unknown'))
            ON CONFLICT(source_key) DO UPDATE SET actor_id=EXCLUDED.actor_id,actor_type=EXCLUDED.actor_type
            WHERE product_activity_events.actor_id='' AND EXCLUDED.actor_id<>''`,
			e.EventType+":"+e.ResourceID, e.WorkspaceID, e.ResourceID, e.ParentID, e.ResourceType, e.EventType, e.At, e.ActorID, e.ActorType, e.Surface)
		return err
	})
}

// RecordProductHosting samples current state at most every five minutes unless
// it changes. was_live is monotonic within the UTC day; live is the latest sample.
func (s *PGStore) RecordProductHosting(ctx context.Context, workspace, resource, kind string, live bool, at time.Time) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `INSERT INTO product_hosting_daily
        (workspace_id,resource_id,resource_type,day,observed_at,live,was_live)
        SELECT $1,$2,$3,($5::timestamptz AT TIME ZONE 'UTC')::date,$5,$4,$4
        WHERE EXISTS (SELECT 1 FROM tenants WHERE id=$1)
        `+productHostingConflictSQL, workspace, resource, kind, live, at)
	return tag.RowsAffected() > 0, err
}

const productHostingConflictSQL = `ON CONFLICT(resource_id,day) DO UPDATE SET observed_at=EXCLUDED.observed_at,
    live=EXCLUDED.live,was_live=product_hosting_daily.was_live OR EXCLUDED.was_live
    WHERE EXCLUDED.observed_at >= product_hosting_daily.observed_at AND
      (EXCLUDED.observed_at >= product_hosting_daily.observed_at + interval '5 minutes'
       OR EXCLUDED.live IS DISTINCT FROM product_hosting_daily.live)`

func (s *PGStore) RecordProductDomainTLS(ctx context.Context, domain string, ready bool, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO product_domain_observations(domain_id,observed_at,tls_ready,first_tls_ready_at)
        SELECT id,$2,$3,CASE WHEN $3 THEN $2::timestamptz END FROM domains WHERE id=$1
        ON CONFLICT(domain_id) DO UPDATE SET observed_at=EXCLUDED.observed_at,tls_ready=EXCLUDED.tls_ready,
          first_tls_ready_at=COALESCE(product_domain_observations.first_tls_ready_at,EXCLUDED.first_tls_ready_at)
        WHERE EXCLUDED.observed_at >= product_domain_observations.observed_at`, domain, at, ready)
	return err
}

// RollbackAppCreation removes only a just-provisioned App and the analytics
// emitted while attempting it, in one transaction. Ordinary deletion preserves
// recent history even when its creation event has aged out of retention.
func (s *PGStore) RollbackAppCreation(ctx context.Context, appID string) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM apps WHERE id=$1`, appID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM product_activity_events WHERE resource_id=$1 OR parent_id=$1`, appID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM product_inventory_lifecycle WHERE resource_id=$1`, appID)
		return err
	})
}

// PurgeProductAnalytics shares the audit retention window, not its row counts.
func (s *PGStore) PurgeProductAnalytics(ctx context.Context, before time.Time) (int64, error) {
	var count int64
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE product_analytics_collection SET events_retained_from=GREATEST(events_retained_from,$1)`, before); err != nil {
			return err
		}
		events, err := tx.Exec(ctx, `DELETE FROM product_activity_events WHERE at<$1`, before)
		if err != nil {
			return err
		}
		count = events.RowsAffected()
		snapshots, err := tx.Exec(ctx, `DELETE FROM product_hosting_daily WHERE day<($1::timestamptz AT TIME ZONE 'UTC')::date`, before)
		count += snapshots.RowsAffected()
		if err != nil {
			return err
		}
		inventory, err := tx.Exec(ctx, `DELETE FROM product_inventory_batches WHERE bucket<$1`, before)
		count += inventory.RowsAffected()
		if err != nil {
			return err
		}
		lifecycle, err := tx.Exec(ctx, `DELETE FROM product_inventory_lifecycle WHERE removed_at<$1`, before)
		count += lifecycle.RowsAffected()
		return err
	})
	return count, err
}
