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
	"log"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5"
)

// BuildFanoutLedger makes a build source's fan-out durable (ADR097 §5,
// w9/m180 t002). A carrier deploy that went live is fanned out to its linked
// services once: the run that claims the build holds a lease, completes it
// when every linked service has been considered, and the reconciler's resync
// re-runs any live build whose run never completed (a bex-api restart mid
// fan-out) once its lease has expired. The fan-out itself skips a service
// already on the build, so a re-run opens no second deploy.
//
// Optional on Store: a store without it keeps the best-effort fan-out.
type BuildFanoutLedger interface {
	// ClaimBuildFanout takes the fan-out of carrier deploy deployID for lease.
	// False when it is already complete or another run holds a live lease.
	ClaimBuildFanout(ctx context.Context, deployID string, lease time.Duration) (bool, error)
	// CompleteBuildFanout records that deployID's fan-out finished.
	CompleteBuildFanout(ctx context.Context, deployID string) error
	// PendingBuildFanouts lists, per build source carrier, its latest live
	// deploy when that deploy's fan-out is incomplete and unclaimed (or its
	// lease expired). An older live build is superseded by a newer one.
	PendingBuildFanouts(ctx context.Context, lease time.Duration) ([]Deploy, error)
}

func (s *PGStore) ClaimBuildFanout(ctx context.Context, deployID string, lease time.Duration) (bool, error) {
	var id string
	err := s.Pool.QueryRow(ctx,
		`UPDATE deploys SET fanout_claimed_at = now()
		 WHERE id = $1 AND fanned_out_at IS NULL
		   AND (fanout_claimed_at IS NULL OR fanout_claimed_at < now() - make_interval(secs => $2))
		 RETURNING id`, deployID, lease.Seconds()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *PGStore) CompleteBuildFanout(ctx context.Context, deployID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE deploys SET fanned_out_at = now() WHERE id = $1 AND fanned_out_at IS NULL`, deployID)
	return err
}

func (s *PGStore) PendingBuildFanouts(ctx context.Context, lease time.Duration) ([]Deploy, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+deployColumns+` FROM (
		   SELECT DISTINCT ON (d.app_id) d.*
		   FROM deploys d JOIN apps a ON a.id = d.app_id
		   WHERE a.type = $1 AND d.status = $2
		   ORDER BY d.app_id, d.finished_at DESC NULLS LAST, d.generation DESC, d.id DESC
		 ) deploys
		 WHERE fanned_out_at IS NULL
		   AND (fanout_claimed_at IS NULL OR fanout_claimed_at < now() - make_interval(secs => $3))
		 ORDER BY app_id`, TypeBuildSource, DeployLive, lease.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deploy
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

const defaultFanoutLease = 10 * time.Minute

func (r *Reconciler) fanoutLease() time.Duration {
	if r.FanoutLease > 0 {
		return r.FanoutLease
	}
	return defaultFanoutLease
}

// startFanout runs one build's fan-out in the background. With a ledger, the
// run claims the build first (a concurrent run, or another replica, skips it)
// and completes it afterwards; an interrupted run's claim expires and the
// resync takes it over.
func (r *Reconciler) startFanout(ctx context.Context, b BuiltSource) {
	ledger, durable := r.Store.(BuildFanoutLedger)
	r.fanouts.Add(1)
	go func() {
		defer r.fanouts.Done()
		ctx := context.WithoutCancel(ctx)
		if durable {
			claimed, err := ledger.ClaimBuildFanout(ctx, b.DeployID, r.fanoutLease())
			if err != nil {
				log.Printf("controlplane: claim fan-out of build %s: %v", b.DeployID, err)
				return
			}
			if !claimed {
				return
			}
		}
		if err := r.BuildFanout.BuildSucceeded(ctx, b); err != nil {
			// Left open: the resync retries once the lease expires.
			log.Printf("controlplane: fan-out of build %s incomplete: %v", b.DeployID, err)
			return
		}
		if durable {
			if err := ledger.CompleteBuildFanout(ctx, b.DeployID); err != nil {
				log.Printf("controlplane: complete fan-out of build %s: %v", b.DeployID, err)
			}
		}
	}()
}

// resyncBuildFanouts re-runs the fan-out of every carrier's latest live build
// whose fan-out never completed, e.g. because bex-api restarted mid run
// (ADR097 §5, w9/m180 t002). It needs the carrier's CR for its namespace and
// name, so a carrier not yet projected waits for a later pass.
func (r *Reconciler) resyncBuildFanouts(ctx context.Context, desired []DesiredApp, byID map[string]*appv1alpha1.App) {
	ledger, ok := r.Store.(BuildFanoutLedger)
	if !ok || r.BuildFanout == nil {
		return
	}
	pending, err := ledger.PendingBuildFanouts(ctx, r.fanoutLease())
	if err != nil {
		log.Printf("controlplane: list pending build fan-outs: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	tenantOf := make(map[string]string, len(desired))
	for _, d := range desired {
		if d.Type == TypeBuildSource {
			tenantOf[d.ID] = d.TenantID
		}
	}
	for _, dep := range pending {
		tenant, isCarrier := tenantOf[dep.AppID]
		cur := byID[dep.AppID]
		if !isCarrier || cur == nil {
			continue
		}
		image := dep.ResolvedImage
		if image == "" {
			image = cur.Status.Image
		}
		r.startFanout(ctx, BuiltSource{
			TenantID: tenant, CarrierID: dep.AppID, DeployID: dep.ID, Commit: dep.CommitInfo(),
			Namespace: cur.Namespace, Carrier: cur.Name, Image: image,
		})
	}
}
