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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Repository environment builds (ADR111): one repository's setup steps baked
// into the image its sessions boot from.

// The build statuses (ADR111 D2).
const (
	AgentRepoEnvironmentBuildQueued    = "queued"
	AgentRepoEnvironmentBuildBuilding  = "building"
	AgentRepoEnvironmentBuildSuccess   = "success"
	AgentRepoEnvironmentBuildFailed    = "failed"
	AgentRepoEnvironmentBuildCancelled = "cancelled"
)

// Failure codes the store itself records; the rest come from the builder.
const (
	AgentRepoEnvironmentBuildInternalError = "internal_error"
	AgentRepoEnvironmentBuildTimeout       = "build_timeout"
)

// The statuses as SQL literals. Interpolated, not bound: a bound status
// would keep a generic plan off the partial indexes keyed on it.
const (
	buildQueuedSQL    = "'" + AgentRepoEnvironmentBuildQueued + "'"
	buildBuildingSQL  = "'" + AgentRepoEnvironmentBuildBuilding + "'"
	buildSuccessSQL   = "'" + AgentRepoEnvironmentBuildSuccess + "'"
	buildFailedSQL    = "'" + AgentRepoEnvironmentBuildFailed + "'"
	buildCancelledSQL = "'" + AgentRepoEnvironmentBuildCancelled + "'"
)

var (
	// ErrAgentRepoEnvironmentNothingToBuild is a build of an environment with
	// no setup steps: it has nothing to bake (ADR111 D1).
	ErrAgentRepoEnvironmentNothingToBuild = errors.New("agent repo environment has no setup steps")
	// ErrAgentRepoEnvironmentBuildBusy is an only-if-idle queue for a
	// repository that already has a build queued or running.
	ErrAgentRepoEnvironmentBuildBusy = errors.New("agent repo environment build already queued or running")
)

// AgentRepoEnvironmentBuildRateError is a queue past the repository's build
// starts in the window; NextAt is when the next one may start.
type AgentRepoEnvironmentBuildRateError struct {
	Limit  int
	NextAt time.Time
}

func (e *AgentRepoEnvironmentBuildRateError) Error() string {
	return fmt.Sprintf("agent repo environment build rate limit (%d) reached until %s", e.Limit, e.NextAt.UTC().Format(time.RFC3339))
}

// AgentRepoEnvironmentBuild is one row of agent_repo_environment_builds.
type AgentRepoEnvironmentBuild struct {
	ID          string
	WorkspaceID string
	Repo        string
	Version     string
	Trigger     string
	Status      string
	// FailureCode and FailureReason are set on a failed build, and the reason
	// also on a cancelled one.
	FailureCode   string
	FailureReason string
	SandboxID     string
	BaseImage     string
	// ImageDigest names the build's image; empty for a build without one.
	// ImageDeletedAt marks an image retired from use.
	ImageDeletedAt *time.Time
	ImageDigest    string
	SizeBytes      *int64
	HeadSHA        string
	// Steps, the redacted log, is read by GetAgentRepoEnvironmentBuild only.
	Steps       json.RawMessage
	CreatedAt   time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
	HeartbeatAt *time.Time
}

// HasImage reports whether a session may boot from the build's image.
func (b AgentRepoEnvironmentBuild) HasImage() bool {
	return b.ImageDigest != "" && b.ImageDeletedAt == nil
}

const agentRepoEnvironmentBuildColumns = `id, workspace_id, repo, version, trigger, status,
	coalesce(failure_code, ''), coalesce(failure_reason, ''), coalesce(sandbox_id, ''), coalesce(base_image, ''),
	image_deleted_at, coalesce(image_digest, ''), size_bytes, coalesce(head_sha, ''),
	created_at, started_at, finished_at, heartbeat_at`

func buildFields(b *AgentRepoEnvironmentBuild) []any {
	return []any{&b.ID, &b.WorkspaceID, &b.Repo, &b.Version, &b.Trigger, &b.Status,
		&b.FailureCode, &b.FailureReason, &b.SandboxID, &b.BaseImage,
		&b.ImageDeletedAt, &b.ImageDigest, &b.SizeBytes, &b.HeadSHA,
		&b.CreatedAt, &b.StartedAt, &b.FinishedAt, &b.HeartbeatAt}
}

func scanAgentRepoEnvironmentBuild(row pgx.Row) (AgentRepoEnvironmentBuild, error) {
	var b AgentRepoEnvironmentBuild
	err := row.Scan(buildFields(&b)...)
	return b, err
}

// lockAgentRepoEnvironmentBuilds serializes the queue of one repository's
// builds, so the rate count and the replaced queued build are read under it.
func lockAgentRepoEnvironmentBuilds(ctx context.Context, tx pgx.Tx, workspaceID, repo string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"agent-repo-environment-builds:"+workspaceID+"/"+repo)
	return err
}

// NewAgentRepoEnvironmentBuild is a build to queue.
type NewAgentRepoEnvironmentBuild struct {
	WorkspaceID string
	Repo        string
	Trigger     string
	// RateLimit builds may start per repository in RateWindow (0: no limit).
	RateLimit  int
	RateWindow time.Duration
	// IfIdle queues nothing (ErrAgentRepoEnvironmentBuildBusy) when the
	// repository already has a build queued or running, rather than replacing
	// the queued one: the daily refresh's mode.
	IfIdle bool
}

// QueueAgentRepoEnvironmentBuild queues a build of the repository's current
// environment. A build already queued for it is cancelled, replaced by this
// one; a running one is left alone (ADR111 D3). The environment must exist
// (ErrNotFound) and have setup steps (ErrAgentRepoEnvironmentNothingToBuild);
// past RateLimit starts in RateWindow it is an
// *AgentRepoEnvironmentBuildRateError.
func (s *PGStore) QueueAgentRepoEnvironmentBuild(ctx context.Context, in NewAgentRepoEnvironmentBuild) (AgentRepoEnvironmentBuild, error) {
	var out AgentRepoEnvironmentBuild
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockAgentRepoEnvironmentBuilds(ctx, tx, in.WorkspaceID, in.Repo); err != nil {
			return err
		}
		var version string
		var steps int
		if err := tx.QueryRow(ctx, `SELECT version, jsonb_array_length(setup) FROM agent_repo_environments
			WHERE workspace_id = $1 AND repo = $2`, in.WorkspaceID, in.Repo).Scan(&version, &steps); err != nil {
			return classify("agent repo environment", err)
		}
		if steps == 0 {
			return ErrAgentRepoEnvironmentNothingToBuild
		}
		if in.IfIdle {
			var busy bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_repo_environment_builds
				WHERE workspace_id = $1 AND repo = $2 AND status IN (`+buildQueuedSQL+`, `+buildBuildingSQL+`))`, in.WorkspaceID, in.Repo).Scan(&busy); err != nil {
				return err
			}
			if busy {
				return ErrAgentRepoEnvironmentBuildBusy
			}
		}
		if in.RateLimit > 0 {
			rows, err := tx.Query(ctx, `SELECT started_at FROM agent_repo_environment_builds
				WHERE workspace_id = $1 AND repo = $2 AND started_at > now() - make_interval(secs => $3)
				ORDER BY started_at`, in.WorkspaceID, in.Repo, in.RateWindow.Seconds())
			if err != nil {
				return err
			}
			starts, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
			if err != nil {
				return err
			}
			if len(starts) >= in.RateLimit {
				return &AgentRepoEnvironmentBuildRateError{Limit: in.RateLimit, NextAt: starts[len(starts)-in.RateLimit].Add(in.RateWindow)}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_repo_environment_builds
			SET status = `+buildCancelledSQL+`, failure_reason = 'a newer build replaced it', finished_at = now()
			WHERE workspace_id = $1 AND repo = $2 AND status = `+buildQueuedSQL, in.WorkspaceID, in.Repo); err != nil {
			return err
		}
		var err error
		out, err = scanAgentRepoEnvironmentBuild(tx.QueryRow(ctx, `INSERT INTO agent_repo_environment_builds
			(id, workspace_id, repo, version, trigger) VALUES ($1, $2, $3, $4, $5)
			RETURNING `+agentRepoEnvironmentBuildColumns,
			ids.New(ids.AgentRepoEnvironmentBuild), in.WorkspaceID, in.Repo, version, in.Trigger))
		return err
	})
	var rate *AgentRepoEnvironmentBuildRateError
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAgentRepoEnvironmentNothingToBuild) ||
		errors.Is(err, ErrAgentRepoEnvironmentBuildBusy) || errors.As(err, &rate) {
		return AgentRepoEnvironmentBuild{}, err
	}
	if err != nil {
		return AgentRepoEnvironmentBuild{}, fmt.Errorf("queue agent repo environment build: %w", err)
	}
	return out, nil
}

// AgentRepoEnvironmentBuildLimits bounds which queued build a claim may start.
type AgentRepoEnvironmentBuildLimits struct {
	// PerWorkspace builds run at once in one workspace.
	PerWorkspace int
	// LiveSandboxes is the workspace's concurrent agent sandbox limit, which
	// builds share with sessions (ADR111 D3); 0 is none.
	LiveSandboxes int
}

// ClaimedAgentRepoEnvironmentBuild is a build a claim started, with the
// environment it builds, read in the claim's transaction so the steps the
// builder runs are the version the build records.
type ClaimedAgentRepoEnvironmentBuild struct {
	Build       AgentRepoEnvironmentBuild
	Environment AgentRepoEnvironment
}

// maxClaimCandidates bounds the queued builds one claim looks at.
const maxClaimCandidates = 32

// ClaimAgentRepoEnvironmentBuild starts the oldest queued build whose
// repository runs none, whose workspace runs fewer than PerWorkspace and has
// a free agent sandbox slot, and returns it, or ErrNotFound when none may
// start. A queued build whose environment was deleted or lost its setup steps
// is cancelled on the way. Every replica claims: a workspace's counts are read
// under its advisory lock, taken with try so a claim skips a workspace another
// is claiming in rather than waiting, and only the started row is locked.
func (s *PGStore) ClaimAgentRepoEnvironmentBuild(ctx context.Context, limits AgentRepoEnvironmentBuildLimits) (ClaimedAgentRepoEnvironmentBuild, error) {
	var out *ClaimedAgentRepoEnvironmentBuild
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT b.id, b.workspace_id, b.repo FROM agent_repo_environment_builds b
			WHERE b.status = `+buildQueuedSQL+` AND NOT EXISTS (SELECT 1 FROM agent_repo_environment_builds r
				WHERE r.workspace_id = b.workspace_id AND r.repo = b.repo AND r.status = `+buildBuildingSQL+`)
			ORDER BY b.created_at LIMIT $1`, maxClaimCandidates)
		if err != nil {
			return err
		}
		type candidate struct{ id, workspace, repo string }
		candidates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidate, error) {
			var c candidate
			err := row.Scan(&c.id, &c.workspace, &c.repo)
			return c, err
		})
		if err != nil {
			return err
		}
		admitted := map[string]bool{}
		for _, c := range candidates {
			ok, seen := admitted[c.workspace]
			if !seen {
				if ok, err = admitBuild(ctx, tx, c.workspace, limits); err != nil {
					return err
				}
				admitted[c.workspace] = ok
			}
			if !ok {
				continue
			}
			if out, err = startQueuedBuild(ctx, tx, c.id, c.workspace, c.repo); err != nil || out != nil {
				return err
			}
		}
		// Committed even when nothing started: it keeps the cancellations.
		return nil
	})
	if err != nil {
		return ClaimedAgentRepoEnvironmentBuild{}, fmt.Errorf("claim agent repo environment build: %w", err)
	}
	if out == nil {
		return ClaimedAgentRepoEnvironmentBuild{}, ErrNotFound
	}
	return *out, nil
}

// admitBuild reports whether a workspace may start one more build: its lock
// is free, it runs fewer than PerWorkspace, and it has a free agent sandbox
// slot.
func admitBuild(ctx context.Context, tx pgx.Tx, workspaceID string, limits AgentRepoEnvironmentBuildLimits) (bool, error) {
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))`,
		"agent-repo-environment-builds:"+workspaceID).Scan(&locked); err != nil || !locked {
		return false, err
	}
	if limits.PerWorkspace > 0 {
		var building int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_repo_environment_builds
			WHERE workspace_id = $1 AND status = `+buildBuildingSQL, workspaceID).Scan(&building); err != nil || building >= limits.PerWorkspace {
			return false, err
		}
	}
	if limits.LiveSandboxes > 0 {
		live, err := countActiveAgentSessions(ctx, tx, workspaceID, AgentSessionCount{WithSandbox: true})
		if err != nil || live >= limits.LiveSandboxes {
			return false, err
		}
	}
	return true, nil
}

// startQueuedBuild starts one queued build with its environment's current
// version, or returns nil: when another claim took it, or when its
// environment has no setup steps left, which cancels it.
func startQueuedBuild(ctx context.Context, tx pgx.Tx, id, workspaceID, repo string) (*ClaimedAgentRepoEnvironmentBuild, error) {
	env, err := scanAgentRepoEnvironment(tx.QueryRow(ctx, `SELECT `+agentRepoEnvironmentColumns+`
		FROM agent_repo_environments WHERE workspace_id = $1 AND repo = $2 AND jsonb_array_length(setup) > 0`, workspaceID, repo))
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `UPDATE agent_repo_environment_builds
			SET status = `+buildCancelledSQL+`, failure_reason = 'the environment has no setup steps to build', finished_at = now()
			WHERE id = $1 AND status = `+buildQueuedSQL, id)
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	build, err := scanAgentRepoEnvironmentBuild(tx.QueryRow(ctx, `UPDATE agent_repo_environment_builds
		SET status = `+buildBuildingSQL+`, version = $2, started_at = now(), heartbeat_at = now()
		WHERE id = $1 AND status = `+buildQueuedSQL+` RETURNING `+agentRepoEnvironmentBuildColumns, id, env.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ClaimedAgentRepoEnvironmentBuild{Build: build, Environment: env}, nil
}

// SetAgentRepoEnvironmentBuilder records a running build's builder sandbox
// and the agent image it runs. false when the build is no longer running.
func (s *PGStore) SetAgentRepoEnvironmentBuilder(ctx context.Context, id, sandboxID, baseImage string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE agent_repo_environment_builds
		SET sandbox_id = $2, base_image = $3, heartbeat_at = now() WHERE id = $1 AND status = `+buildBuildingSQL, id, sandboxID, baseImage)
	return tag.RowsAffected() == 1, err
}

// HeartbeatAgentRepoEnvironmentBuild says a running build's worker is alive.
// false when the build is no longer running (cancelled, or failed by the
// sweeper), which stops the worker.
func (s *PGStore) HeartbeatAgentRepoEnvironmentBuild(ctx context.Context, id string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE agent_repo_environment_builds SET heartbeat_at = now()
		WHERE id = $1 AND status = `+buildBuildingSQL, id)
	return tag.RowsAffected() == 1, err
}

// ReserveAgentRepoEnvironmentBuildImage records a running build's image size
// before the image is copied, under the workspace's storage lock that imports
// take too, refused with *SandboxStorageLimitError when the workspace's
// retained images would pass limitBytes (ADR111 D9). The reservation counts
// while the build runs and stays with its image.
func (s *PGStore) ReserveAgentRepoEnvironmentBuildImage(ctx context.Context, workspaceID, buildID string, sizeBytes, limitBytes int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockSandboxStorage(ctx, tx, workspaceID); err != nil {
			return err
		}
		var used int64
		if err := tx.QueryRow(ctx, sandboxStorageBytesSQL, workspaceID).Scan(&used); err != nil {
			return err
		}
		if used+sizeBytes > limitBytes {
			return &SandboxStorageLimitError{UsedBytes: used}
		}
		tag, err := tx.Exec(ctx, `UPDATE agent_repo_environment_builds SET size_bytes = $3
			WHERE id = $1 AND workspace_id = $2 AND status = `+buildBuildingSQL, buildID, workspaceID, sizeBytes)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return err
	})
}

// AgentRepoEnvironmentBuildImage is a successful build's image and log.
type AgentRepoEnvironmentBuildImage struct {
	Digest    string
	SizeBytes int64
	HeadSHA   string
	Steps     json.RawMessage
}

// SucceedAgentRepoEnvironmentBuild settles a running build with its image.
// false when it is no longer running, and its image is then named by nothing.
func (s *PGStore) SucceedAgentRepoEnvironmentBuild(ctx context.Context, id string, image AgentRepoEnvironmentBuildImage) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE agent_repo_environment_builds
		SET status = `+buildSuccessSQL+`, image_digest = $2, size_bytes = $3, head_sha = NULLIF($4, ''), steps = $5, finished_at = now()
		WHERE id = $1 AND status = `+buildBuildingSQL, id, image.Digest, image.SizeBytes, image.HeadSHA, orEmptyJSON(image.Steps, "[]"))
	return tag.RowsAffected() == 1, err
}

// FailAgentRepoEnvironmentBuild settles a running build as failed, releasing
// any storage it reserved. false when it is no longer running.
func (s *PGStore) FailAgentRepoEnvironmentBuild(ctx context.Context, id, code, reason string, steps json.RawMessage) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE agent_repo_environment_builds
		SET status = `+buildFailedSQL+`, failure_code = $2, failure_reason = $3, steps = $4, size_bytes = NULL, finished_at = now()
		WHERE id = $1 AND status = `+buildBuildingSQL, id, code, reason, orEmptyJSON(steps, "[]"))
	return tag.RowsAffected() == 1, err
}

// FailStaleAgentRepoEnvironmentBuilds fails the running builds whose worker
// stopped saying it is alive for staleAfter (internal_error), or that have run
// past maxDuration (build_timeout), and returns them so their builders are
// terminated.
func (s *PGStore) FailStaleAgentRepoEnvironmentBuilds(ctx context.Context, staleAfter, maxDuration time.Duration) ([]AgentRepoEnvironmentBuild, error) {
	rows, err := collectWith(scanAgentRepoEnvironmentBuild)(s.Pool.Query(ctx, `UPDATE agent_repo_environment_builds
		SET status = `+buildFailedSQL+`, size_bytes = NULL, finished_at = now(),
		    failure_code = CASE WHEN started_at < now() - make_interval(secs => $2) THEN $3 ELSE $4 END,
		    failure_reason = CASE WHEN started_at < now() - make_interval(secs => $2)
		        THEN 'the build ran past its time limit' ELSE 'the build was interrupted; it runs again on its next trigger' END
		WHERE status = `+buildBuildingSQL+` AND (heartbeat_at < now() - make_interval(secs => $1) OR started_at < now() - make_interval(secs => $2))
		RETURNING `+agentRepoEnvironmentBuildColumns, staleAfter.Seconds(), maxDuration.Seconds(),
		AgentRepoEnvironmentBuildTimeout, AgentRepoEnvironmentBuildInternalError))
	if err != nil {
		return nil, fmt.Errorf("fail stale agent repo environment builds: %w", err)
	}
	return rows, nil
}

// GetAgentRepoEnvironmentBuild returns one build with its log, or ErrNotFound.
func (s *PGStore) GetAgentRepoEnvironmentBuild(ctx context.Context, id string) (AgentRepoEnvironmentBuild, error) {
	var b AgentRepoEnvironmentBuild
	err := s.Pool.QueryRow(ctx, `SELECT `+agentRepoEnvironmentBuildColumns+`, steps
		FROM agent_repo_environment_builds WHERE id = $1`, id).Scan(append(buildFields(&b), &b.Steps)...)
	if err != nil {
		return AgentRepoEnvironmentBuild{}, classify("agent repo environment build", err)
	}
	return b, nil
}

// AgentRepoEnvironmentBuildBinding is what binds a build principal to its
// builder: its workspace, repository, status and builder sandbox.
type AgentRepoEnvironmentBuildBinding struct {
	ID          string
	WorkspaceID string
	Repo        string
	Status      string
	SandboxID   string
}

// GetAgentRepoEnvironmentBuildBinding returns one build's binding, or
// ErrNotFound.
func (s *PGStore) GetAgentRepoEnvironmentBuildBinding(ctx context.Context, id string) (AgentRepoEnvironmentBuildBinding, error) {
	var b AgentRepoEnvironmentBuildBinding
	err := s.Pool.QueryRow(ctx, `SELECT id, workspace_id, repo, status, coalesce(sandbox_id, '')
		FROM agent_repo_environment_builds WHERE id = $1`, id).Scan(&b.ID, &b.WorkspaceID, &b.Repo, &b.Status, &b.SandboxID)
	if err != nil {
		return AgentRepoEnvironmentBuildBinding{}, classify("agent repo environment build", err)
	}
	return b, nil
}

// AgentRepoEnvironmentBuilds lists a repository's builds, newest first.
func (s *PGStore) AgentRepoEnvironmentBuilds(ctx context.Context, workspaceID, repo string, limit int) ([]AgentRepoEnvironmentBuild, error) {
	rows, err := collectWith(scanAgentRepoEnvironmentBuild)(s.Pool.Query(ctx, `SELECT `+agentRepoEnvironmentBuildColumns+`
		FROM agent_repo_environment_builds WHERE workspace_id = $1 AND repo = $2
		ORDER BY created_at DESC, id DESC LIMIT $3`, workspaceID, repo, limit))
	if err != nil {
		return nil, fmt.Errorf("list agent repo environment builds: %w", err)
	}
	return rows, nil
}

// LatestAgentRepoEnvironmentBuilds returns each repository's newest build in
// the workspace.
func (s *PGStore) LatestAgentRepoEnvironmentBuilds(ctx context.Context, workspaceID string) ([]AgentRepoEnvironmentBuild, error) {
	rows, err := collectWith(scanAgentRepoEnvironmentBuild)(s.Pool.Query(ctx, `SELECT DISTINCT ON (repo) `+agentRepoEnvironmentBuildColumns+`
		FROM agent_repo_environment_builds WHERE workspace_id = $1
		ORDER BY repo, created_at DESC, id DESC`, workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list latest agent repo environment builds: %w", err)
	}
	return rows, nil
}

// AgentRepoEnvironmentBuildRef names a build's image.
type AgentRepoEnvironmentBuildRef struct {
	ID          string
	ImageDigest string
}

// NewestAgentRepoEnvironmentBuildImage returns the repository's newest
// successful build of version, run on baseImage, whose image a session may
// still boot from (ADR111 D4), or ErrNotFound.
func (s *PGStore) NewestAgentRepoEnvironmentBuildImage(ctx context.Context, workspaceID, repo, version, baseImage string) (AgentRepoEnvironmentBuildRef, error) {
	var out AgentRepoEnvironmentBuildRef
	err := s.Pool.QueryRow(ctx, `SELECT id, image_digest FROM agent_repo_environment_builds
		WHERE workspace_id = $1 AND repo = $2 AND version = $3 AND base_image = $4
		  AND status = `+buildSuccessSQL+` AND image_digest IS NOT NULL AND image_deleted_at IS NULL
		ORDER BY finished_at DESC, id DESC LIMIT 1`, workspaceID, repo, version, baseImage).Scan(&out.ID, &out.ImageDigest)
	if err != nil {
		return AgentRepoEnvironmentBuildRef{}, classify("agent repo environment build", err)
	}
	return out, nil
}

// RetainAgentRepoEnvironmentBuilds applies a repository's retention (ADR111
// D8): every image but those of its keepImages newest successful builds is
// retired, and its terminal builds past the keepRecords newest that hold no
// image are removed.
func (s *PGStore) RetainAgentRepoEnvironmentBuilds(ctx context.Context, workspaceID, repo string, keepImages, keepRecords int) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE agent_repo_environment_builds SET image_deleted_at = now()
			WHERE workspace_id = $1 AND repo = $2 AND image_digest IS NOT NULL AND image_deleted_at IS NULL
			  AND id NOT IN (SELECT id FROM agent_repo_environment_builds
			                 WHERE workspace_id = $1 AND repo = $2 AND status = `+buildSuccessSQL+`
			                 ORDER BY finished_at DESC, id DESC LIMIT $3)`, workspaceID, repo, keepImages); err != nil {
			return fmt.Errorf("retain agent repo environment build images: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM agent_repo_environment_builds
			WHERE workspace_id = $1 AND repo = $2 AND image_digest IS NULL
			  AND status IN (`+buildSuccessSQL+`, `+buildFailedSQL+`, `+buildCancelledSQL+`)
			  AND id NOT IN (SELECT id FROM agent_repo_environment_builds
			                 WHERE workspace_id = $1 AND repo = $2
			                 ORDER BY created_at DESC, id DESC LIMIT $3)`, workspaceID, repo, keepRecords); err != nil {
			return fmt.Errorf("retain agent repo environment build records: %w", err)
		}
		return nil
	})
}

// AgentRepoEnvironmentBuildImageIDs lists the workspace's builds whose images
// the registry may still hold, for a workspace delete.
func (s *PGStore) AgentRepoEnvironmentBuildImageIDs(ctx context.Context, workspaceID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM agent_repo_environment_builds
		WHERE workspace_id = $1 AND image_digest IS NOT NULL`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list agent repo environment build images: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// AgentRepoEnvironmentBuildPurge paces the deletion of retired images.
type AgentRepoEnvironmentBuildPurge struct {
	// Grace is how long a retired image stays in the registry.
	Grace time.Duration
	// Backoff is the wait after a claim, doubled per earlier attempt up to
	// MaxBackoff: the next claim of an image whose delete failed.
	Backoff    time.Duration
	MaxBackoff time.Duration
	Limit      int
}

// ClaimAgentRepoEnvironmentBuildPurges claims retired images past their grace
// whose delete is due, and defers each one's next attempt, so one replica
// deletes each and a failure backs off.
func (s *PGStore) ClaimAgentRepoEnvironmentBuildPurges(ctx context.Context, p AgentRepoEnvironmentBuildPurge) ([]AgentRepoEnvironmentBuild, error) {
	rows, err := collectWith(scanAgentRepoEnvironmentBuild)(s.Pool.Query(ctx, `UPDATE agent_repo_environment_builds
		SET purge_attempts = purge_attempts + 1,
		    purge_after = now() + least(make_interval(secs => $2) * power(2, purge_attempts), make_interval(secs => $3))
		WHERE id IN (SELECT id FROM agent_repo_environment_builds
		             WHERE image_digest IS NOT NULL AND image_deleted_at < now() - make_interval(secs => $1)
		               AND (purge_after IS NULL OR purge_after <= now())
		             ORDER BY image_deleted_at LIMIT $4 FOR UPDATE SKIP LOCKED)
		RETURNING `+agentRepoEnvironmentBuildColumns, p.Grace.Seconds(), p.Backoff.Seconds(), p.MaxBackoff.Seconds(), p.Limit))
	if err != nil {
		return nil, fmt.Errorf("claim agent repo environment build purges: %w", err)
	}
	return rows, nil
}

// ClearAgentRepoEnvironmentBuildImage records that the registry no longer
// holds a retired build's image.
func (s *PGStore) ClearAgentRepoEnvironmentBuildImage(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_repo_environment_builds SET image_digest = NULL
		WHERE id = $1 AND image_deleted_at IS NOT NULL`, id)
	return err
}

// AgentRepoEnvironmentRefresh is an environment with setup steps and no
// build queued or running, with its newest build if it has one: a candidate
// for the daily refresh. Newest carries its status, failure code, version,
// agent image and times.
type AgentRepoEnvironmentRefresh struct {
	WorkspaceID string
	Repo        string
	Version     string
	Newest      *AgentRepoEnvironmentBuild
}

// AgentRepoEnvironmentRefreshes lists every refresh candidate.
func (s *PGStore) AgentRepoEnvironmentRefreshes(ctx context.Context) ([]AgentRepoEnvironmentRefresh, error) {
	rows, err := s.Pool.Query(ctx, `SELECT e.workspace_id, e.repo, e.version,
		       b.status, coalesce(b.failure_code, ''), b.version, coalesce(b.base_image, ''), b.created_at, b.finished_at
		FROM agent_repo_environments e
		LEFT JOIN LATERAL (SELECT status, failure_code, version, base_image, created_at, finished_at
		                   FROM agent_repo_environment_builds b
		                   WHERE b.workspace_id = e.workspace_id AND b.repo = e.repo
		                   ORDER BY created_at DESC, id DESC LIMIT 1) b ON true
		WHERE jsonb_array_length(e.setup) > 0 AND coalesce(b.status, '') NOT IN (`+buildQueuedSQL+`, `+buildBuildingSQL+`)
		ORDER BY e.workspace_id, e.repo`)
	if err != nil {
		return nil, fmt.Errorf("list agent repo environment refreshes: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentRepoEnvironmentRefresh, error) {
		var r AgentRepoEnvironmentRefresh
		var status, code, version, base *string
		var created *time.Time
		var finished *time.Time
		if err := row.Scan(&r.WorkspaceID, &r.Repo, &r.Version, &status, &code, &version, &base, &created, &finished); err != nil {
			return r, err
		}
		if status != nil {
			r.Newest = &AgentRepoEnvironmentBuild{WorkspaceID: r.WorkspaceID, Repo: r.Repo, Status: *status, FailureCode: *code,
				Version: *version, BaseImage: *base, CreatedAt: *created, FinishedAt: finished}
		}
		return r, nil
	})
}
