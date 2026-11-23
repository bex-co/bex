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
	"fmt"
	"log"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5"
)

// Render's trigger-build-source-build deploy.serviceScope values (w4/m596 t002).
const (
	BuildDeployScopeAutoDeployOnly = "autodeploy_only"
	BuildDeployScopeNone           = "none"
	BuildDeployScopeCustom         = "custom"
)

// ErrAwaitingBuildDeployGone is a build-pending service deploy that was already
// promoted or canceled before this fan-out run reached it.
var ErrAwaitingBuildDeployGone = fmt.Errorf("%w: the build-pending deploy was already promoted or canceled", ErrConflict)

type buildDeployScopeKey struct{}

// WithBuildDeployScope stamps build_deploy_scope on the carrier deploy row
// created under ctx.
func WithBuildDeployScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, buildDeployScopeKey{}, scope)
}

// BuildDeployScopeFrom returns the scope set by WithBuildDeployScope, or "".
func BuildDeployScopeFrom(ctx context.Context) string {
	s, _ := ctx.Value(buildDeployScopeKey{}).(string)
	return s
}

type awaitingBuildDeployKey struct{}

// WithAwaitingBuildDeploy makes the deploy opened under ctx promote the
// pre-created awaiting-build row id in place rather than inserting a new row.
func WithAwaitingBuildDeploy(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, awaitingBuildDeployKey{}, id)
}

// AwaitingBuildDeployID returns the awaiting-build row id set on ctx, or "".
func AwaitingBuildDeployID(ctx context.Context) string {
	id, _ := ctx.Value(awaitingBuildDeployKey{}).(string)
	return id
}

// CreateAwaitingBuildDeploy opens a linked service's deploy row at build
// trigger time while its build_id is still building. The row is immediately
// readable via GetDeploy/ListDeploys and stays awaiting_build until the build
// succeeds (promoted in place) or fails/cancels (closed terminal).
func (s *PGStore) CreateAwaitingBuildDeploy(ctx context.Context, appID, trigger, buildID string, commit CommitInfo, triggeredBy string) (Deploy, error) {
	d := Deploy{
		ID: ids.New(ids.Deploy), AppID: appID, Trigger: trigger,
		Commit: commit.Hash, CommitMessage: commit.Message, CommitAuthorAt: commit.AuthorAt,
		BuildID: buildID, TriggeredBy: triggeredBy, Status: DeployCreated, AwaitingBuild: true,
	}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO deploys (id, app_id, trigger, image, generation, commit, commit_message, commit_author_at, triggered_by, status, overlap_pending, restart_deferred, awaiting_build, build_id)
		 VALUES ($1, $2, $3, '', 0, $4, $5, $6, $7, $8, false, false, true, $9)
		 RETURNING created_at, updated_at`,
		d.ID, d.AppID, d.Trigger, d.Commit, d.CommitMessage, d.CommitAuthorAt, d.TriggeredBy, d.Status, d.BuildID,
	).Scan(&d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

// ListBuildAwaitingDeploys lists every service deploy still waiting on buildID.
func (s *PGStore) ListBuildAwaitingDeploys(ctx context.Context, buildID string) ([]Deploy, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+deployColumns+` FROM deploys
		 WHERE build_id = $1 AND awaiting_build AND status = $2
		 ORDER BY created_at, id`, buildID, DeployCreated)
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

// CancelAwaitingBuildDeploy closes a single awaiting-build deploy row as
// canceled. Returns false if it was already promoted or closed.
func (s *PGStore) CancelAwaitingBuildDeploy(ctx context.Context, id, reason string) (bool, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE deploys
		 SET status = $2, awaiting_build = false, cancel_reason = $3,
		     finished_at = clock_timestamp(),
		     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
		 WHERE id = $1 AND awaiting_build AND status = $4`,
		id, DeployCanceled, reason, DeployCreated)
	if err != nil {
		return false, classify("deploy", err)
	}
	return tag.RowsAffected() == 1, nil
}

// CheckAwaitingBuildDeploy checks that id is still an awaiting-build row for
// appID before openRelease mutates the App CR.
func (s *PGStore) CheckAwaitingBuildDeploy(ctx context.Context, id, appID string) error {
	var waiting bool
	err := s.Pool.QueryRow(ctx, `SELECT awaiting_build AND status = $3 FROM deploys WHERE id = $1 AND app_id = $2`, id, appID, DeployCreated).Scan(&waiting)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !waiting) {
		return ErrAwaitingBuildDeployGone
	}
	return err
}

// CloseBuildAwaitingDeploys closes every service deploy still waiting on
// buildID when the carrier build ends without succeeding.
func (s *PGStore) CloseBuildAwaitingDeploys(ctx context.Context, buildID, status, failureReason, cancelReason string) (int64, error) {
	var n int64
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var err error
		n, err = closeBuildAwaitingDeploysCountTx(ctx, tx, buildID, status, failureReason, cancelReason)
		return err
	})
	return n, err
}

func closeBuildAwaitingDeploysTx(ctx context.Context, tx pgx.Tx, buildID, carrierStatus, failureReason, cancelReason string) error {
	_, err := closeBuildAwaitingDeploysCountTx(ctx, tx, buildID, carrierStatus, failureReason, cancelReason)
	return err
}

func closeBuildAwaitingDeploysCountTx(ctx context.Context, tx pgx.Tx, buildID, carrierStatus, failureReason, cancelReason string) (int64, error) {
	targetStatus := DeployBuildFailed
	targetFailure, targetCancel := failureReason, ""
	if carrierStatus == DeployCanceled {
		targetStatus = DeployCanceled
		targetFailure = ""
		targetCancel = cancelReason
		if targetCancel == "" {
			targetCancel = fmt.Sprintf("build %s was canceled", buildID)
		}
	} else if targetFailure == "" {
		targetFailure = fmt.Sprintf("build %s failed", buildID)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE deploys
		 SET status = $2, awaiting_build = false, failure_reason = $3, cancel_reason = $4,
		     finished_at = clock_timestamp(),
		     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
		 WHERE build_id = $1 AND awaiting_build AND status = $5`,
		buildID, targetStatus, targetFailure, targetCancel, DeployCreated)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeferAwaitingBuildDeploy transitions an awaiting-build row in place into a
// restart_deferred queued row when its service has an unadopted deploy in
// flight, keeping its deploy id. Returns deferred=false when nothing is in
// flight or the workspace policy is Override.
func (s *PGStore) DeferAwaitingBuildDeploy(ctx context.Context, id, appID string, req DeferredRequest) (_ Deploy, deferred bool, err error) {
	if err := s.awaitDeployAheadRecorded(ctx, appID); err != nil {
		return Deploy{}, false, classify("deploy", err)
	}
	var out Deploy
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockApp(ctx, tx, appID); err != nil {
			return err
		}
		var waiting bool
		if err := tx.QueryRow(ctx, `SELECT awaiting_build AND status = $3 FROM deploys WHERE id = $1 AND app_id = $2 FOR UPDATE`, id, appID, DeployCreated).Scan(&waiting); errors.Is(err, pgx.ErrNoRows) || (err == nil && !waiting) {
			return ErrAwaitingBuildDeployGone
		} else if err != nil {
			return err
		}
		if err := proceedToWaitingDeploy(ctx, tx, appID); err != nil {
			return err
		}
		if deferred, err = appWaitsForActiveDeploy(ctx, tx, appID); err != nil || !deferred {
			return err
		}
		if err := cancelPendingDeploys(ctx, tx, appID, supersededByReason(id)); err != nil {
			return err
		}
		out, err = scanDeploy(tx.QueryRow(ctx,
			`UPDATE deploys
			 SET status = $2, overlap_pending = true, restart_deferred = true, awaiting_build = false,
			     deferred_request = $3,
			     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
			 WHERE id = $1
			 RETURNING `+deployColumns,
			id, DeployQueued, req))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrAwaitingBuildDeployGone) {
			return Deploy{}, false, err
		}
		return Deploy{}, false, classify("deploy", err)
	}
	if !deferred {
		return Deploy{}, false, nil
	}
	return out, true, nil
}

// promoteAwaitingBuildDeploy promotes an awaiting-build deploy row in place
// into the release just opened for its service, preserving its deploy id.
func (s *PGStore) promoteAwaitingBuildDeploy(ctx context.Context, id, appID, image string, generation int64, commit CommitInfo, replaced func(string) string) (Deploy, error) {
	if err := s.awaitDeployAheadRecorded(ctx, appID); err != nil {
		return Deploy{}, classify("deploy", err)
	}
	var d Deploy
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockApp(ctx, tx, appID); err != nil {
			return err
		}
		var waiting bool
		if err := tx.QueryRow(ctx, `SELECT awaiting_build AND status = $3 FROM deploys WHERE id = $1 AND app_id = $2 FOR UPDATE`, id, appID, DeployCreated).Scan(&waiting); errors.Is(err, pgx.ErrNoRows) || (err == nil && !waiting) {
			return ErrAwaitingBuildDeployGone
		} else if err != nil {
			return err
		}
		status, cancelReason, err := prepareDeployCreate(ctx, tx, appID, generation, replaced(id))
		if err != nil {
			return err
		}
		overlapPending := status == DeployQueued
		d, err = scanDeploy(tx.QueryRow(ctx,
			`UPDATE deploys
			 SET image = $2, generation = $3, commit = $4, commit_message = $5, commit_author_at = $6,
			     build_id = NULLIF($7, ''), status = $8, overlap_pending = $9, awaiting_build = false,
			     cancel_reason = $10,
			     finished_at = CASE WHEN $8 = $11 THEN clock_timestamp() END,
			     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
			 WHERE id = $1
			 RETURNING `+deployColumns,
			id, image, generation, commit.Hash, commit.Message, commit.AuthorAt, commit.BuildID, status, overlapPending, cancelReason, DeployCanceled))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrAwaitingBuildDeployGone) {
			return Deploy{}, err
		}
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

// ReapTerminalBuildAwaitingDeploys closes any awaiting-build deploy whose
// carrier build already reached a terminal non-live status.
func (s *PGStore) ReapTerminalBuildAwaitingDeploys(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE deploys AS d
		 SET status = CASE WHEN c.status = $1 THEN $1 ELSE $2 END,
		     awaiting_build = false,
		     failure_reason = CASE WHEN c.status = $1 THEN '' ELSE COALESCE(NULLIF(c.failure_reason, ''), 'build ' || d.build_id || ' failed') END,
		     cancel_reason = CASE WHEN c.status = $1 THEN COALESCE(NULLIF(c.cancel_reason, ''), 'build ' || d.build_id || ' was canceled') ELSE '' END,
		     finished_at = clock_timestamp(),
		     updated_at = GREATEST(d.updated_at + interval '1 microsecond', clock_timestamp())
		 FROM deploys AS c
		 WHERE d.awaiting_build AND d.status = $3
		   AND c.id = 'dep-' || substr(d.build_id, 5)
		   AND c.status = ANY($4)`,
		DeployCanceled, DeployBuildFailed, DeployCreated,
		[]string{DeployBuildFailed, DeployUpdateFailed, DeployPreDeployFailed, DeployCanceled})
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

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
			TenantID: tenant, CarrierID: dep.AppID, DeployID: dep.ID, BuildDeployScope: dep.BuildDeployScope, Commit: dep.CommitInfo(),
			Namespace: cur.Namespace, Carrier: cur.Name, Image: image,
		})
	}
}
