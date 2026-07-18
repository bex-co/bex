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

// lifecycle.go is the managed-Postgres lifecycle surface — suspend / resume /
// restart, mirroring Render's POST /postgres/{id}/{suspend,resume,restart} and
// the App/KeyValue lifecycle verbs. Each writes intent to the Database CR spec
// and lets the operator converge (hibernate the CNPG cluster / bump the restart
// annotation); none block on reconciliation, exactly like the compute verbs.
package postgres

import (
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Suspend hibernates the managed Postgres (spec.suspended = true): the operator
// sets CNPG's hibernation annotation, stopping compute while the PVC (data),
// credentials and any external route are preserved — Render's Postgres suspend,
// honoring the sleep-is-free promise.
func (s *Service) Suspend(ctx context.Context, name string) (PostgresView, error) {
	return s.setSuspended(ctx, name, true)
}

// Resume wakes a suspended Postgres (spec.suspended = false); the operator
// un-hibernates the CNPG cluster back to healthy.
func (s *Service) Resume(ctx context.Context, name string) (PostgresView, error) {
	return s.setSuspended(ctx, name, false)
}

// setSuspended flips spec.suspended and lets the operator converge (hibernate /
// wake). A no-op write is skipped so an idempotent suspend/resume doesn't bump
// the generation needlessly.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if suspended {
		if err := s.requireUnprotected(ctx, d, "suspend"); err != nil {
			return PostgresView{}, err
		}
	}
	if d.Spec.Suspended == suspended {
		return pgView(d), nil
	}
	// Reuse the object already in hand — no second fetch.
	return s.patchDatabaseObj(ctx, d, func(d *appv1alpha1.Database) {
		d.Spec.Suspended = suspended
	})
}

// Restart requests a rolling restart of the primary (spec.restartedAt = now);
// the operator stamps CNPG's restart annotation and CNPG bounces the instance.
func (s *Service) Restart(ctx context.Context, name string) (PostgresView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	view, err := s.patchDatabaseObj(ctx, d, func(d *appv1alpha1.Database) {
		d.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
	if err != nil {
		return PostgresView{}, err
	}
	s.RecordDatabaseEffect(ctx, d, core.DatabaseRestarted)
	return view, nil
}

// Failover requests a CNPG planned switchover (spec.failoverAt = now). The
// operator reads instanceNames from CNPG status and patches targetPrimary to
// promote a standby. Only meaningful when highAvailability is enabled; the
// operator is a no-op if there is no ready standby. Mirrors Render's
// POST /v1/postgres/{id}/failover → 202 (fire-and-forget intent write).
func (s *Service) Failover(ctx context.Context, name string) error {
	_, err := s.patchDatabase(ctx, core.RelCanOperate, name, func(d *appv1alpha1.Database) {
		d.Spec.FailoverAt = s.Now().UTC().Format(time.RFC3339)
	})
	return err
}
