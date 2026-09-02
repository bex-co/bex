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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// gitwebhook.go is the bounded durable replay ledger for the signed Git push
// webhook. HMAC authenticates bytes, not freshness, so claims for the currently
// accepted signing-secret epoch must not expire. Each tenant scope instead has
// a hard cardinality bound. Secret rotation creates a new authenticated epoch;
// replica leases retain the old epoch until no live process accepts it, then the
// maintenance pass safely purges its claims.

const (
	GitWebhookReplayKeyManual = "manual"
	GitWebhookReplayKeyGitHub = "github"

	// MaxGitWebhookReplayClaimsPerScope bounds the durable rows one tenant can
	// allocate across all signing epochs that live API replicas still accept.
	// At roughly one row per push, 100k leaves years of ordinary headroom while
	// imposing a finite per-tenant storage cost. Reaching it fails closed until
	// a retired signing epoch is reclaimed or an operator intervenes.
	MaxGitWebhookReplayClaimsPerScope = 100_000

	// GitWebhookReplayMaintenanceInterval is each replica's epoch heartbeat and
	// retired-epoch sweep cadence.
	GitWebhookReplayMaintenanceInterval = time.Minute
	// GitWebhookReplayLeaseTTL is the failure-detection window before a crashed
	// or disconnected replica's signing epoch can be retired.
	GitWebhookReplayLeaseTTL = 5 * time.Minute
)

// ErrGitWebhookReplayCapacity is returned before insertion when a tenant's
// exact durable replay-row budget is exhausted.
var ErrGitWebhookReplayCapacity = errors.New("git webhook replay capacity reached")

// ErrGitWebhookReplayEpochRetired fails closed when a replica tries to accept a
// signing key whose claims were already safely purged after its lease expired.
var ErrGitWebhookReplayEpochRetired = errors.New("git webhook replay signing epoch retired")

// GitWebhookReplayClaim is one tenant- and signing-epoch-bound body digest.
type GitWebhookReplayClaim struct {
	Scope    string
	KeyClass string
	Epoch    string
	Digest   string
}

// GitWebhookReplayEpoch identifies one key class and exact signing secret that
// an API replica currently accepts.
type GitWebhookReplayEpoch struct {
	KeyClass string
	Epoch    string
}

// GitWebhookSigningEpoch derives a non-secret stable identifier for the exact
// HMAC key a replica accepts. The epoch changes on key rotation, which is the
// only point at which claims made under the old key can safely be discarded.
func GitWebhookSigningEpoch(keyClass, secret string) string {
	if secret == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte("bex/git-webhook-replay-epoch/v1/" + keyClass))
	return hex.EncodeToString(h.Sum(nil))
}

// GitWebhookReplayEpochs derives the active epochs from configured secrets.
func GitWebhookReplayEpochs(manualSecret, githubSecret string) []GitWebhookReplayEpoch {
	epochs := make([]GitWebhookReplayEpoch, 0, 2)
	if epoch := GitWebhookSigningEpoch(GitWebhookReplayKeyManual, manualSecret); epoch != "" {
		epochs = append(epochs, GitWebhookReplayEpoch{KeyClass: GitWebhookReplayKeyManual, Epoch: epoch})
	}
	if epoch := GitWebhookSigningEpoch(GitWebhookReplayKeyGitHub, githubSecret); epoch != "" {
		epochs = append(epochs, GitWebhookReplayEpoch{KeyClass: GitWebhookReplayKeyGitHub, Epoch: epoch})
	}
	return epochs
}

func validGitWebhookReplayClaim(claim GitWebhookReplayClaim) bool {
	return claim.Scope != "" && claim.Epoch != "" && claim.Digest != "" &&
		(claim.KeyClass == GitWebhookReplayKeyManual || claim.KeyClass == GitWebhookReplayKeyGitHub)
}

// ClaimGitWebhookDelivery atomically claims a signed body within its tenant and
// signing-key epoch. A per-scope advisory transaction lock makes the
// count-before-insert cap exact even when distinct deliveries arrive on several
// replicas concurrently. The digest remains globally unique for rolling-upgrade
// compatibility with pre-0104 replicas and because the signed body itself
// embeds the installation/repository identity.
func (s *PGStore) ClaimGitWebhookDelivery(ctx context.Context, claim GitWebhookReplayClaim) (bool, error) {
	return s.claimGitWebhookDelivery(ctx, claim, MaxGitWebhookReplayClaimsPerScope)
}

func (s *PGStore) claimGitWebhookDelivery(ctx context.Context, claim GitWebhookReplayClaim, maxClaims int) (fresh bool, err error) {
	if !validGitWebhookReplayClaim(claim) || maxClaims < 1 {
		return false, fmt.Errorf("%w: invalid git webhook replay claim", ErrInvalid)
	}
	now := time.Now().UTC()
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		// Refresh only an unexpired lease. A replica disconnected from Postgres
		// for longer than the TTL must not resume an epoch whose claims another
		// replica may already have retired and purged.
		tag, err := tx.Exec(ctx, `
			UPDATE git_webhook_replay_epoch_leases
			SET heartbeat_at = $1
			WHERE instance_id = $2 AND key_class = $3 AND epoch = $4
			  AND heartbeat_at >= $5`,
			now, s.gitWebhookReplayInstance, claim.KeyClass, claim.Epoch,
			now.Add(-GitWebhookReplayLeaseTTL))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrGitWebhookReplayEpochRetired
		}
		// The lock key is tenant-scoped deliberately: manual/GitHub key classes
		// and overlapping rotation epochs all consume one shared tenant budget.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, claim.Scope); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM git_webhook_replays
				WHERE digest = $1
			)`, claim.Digest).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		var count int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM git_webhook_replays WHERE scope = $1 AND epoch <> 'legacy'`,
			claim.Scope,
		).Scan(&count); err != nil {
			return err
		}
		if count >= maxClaims {
			return ErrGitWebhookReplayCapacity
		}
		tag, err = tx.Exec(ctx, `
			INSERT INTO git_webhook_replays (scope, key_class, epoch, digest)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (digest) DO NOTHING`, claim.Scope, claim.KeyClass, claim.Epoch, claim.Digest)
		if err == nil {
			fresh = tag.RowsAffected() == 1
		}
		return err
	})
	return fresh, err
}

// ReleaseGitWebhookDelivery frees a claim whose delivery failed before
// completing, allowing the git host's retry to process it.
func (s *PGStore) ReleaseGitWebhookDelivery(ctx context.Context, claim GitWebhookReplayClaim) error {
	if !validGitWebhookReplayClaim(claim) {
		return fmt.Errorf("%w: invalid git webhook replay claim", ErrInvalid)
	}
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM git_webhook_replays
		WHERE scope = $1 AND key_class = $2 AND epoch = $3 AND digest = $4`,
		claim.Scope, claim.KeyClass, claim.Epoch, claim.Digest)
	return err
}

// MaintainGitWebhookReplayEpochs heartbeats this process's accepted epochs,
// expires dead replica leases, and purges claims only for epochs no live lease
// still accepts. The finite pre-migration legacy partition is never purged: it
// has no authenticated epoch to prove retired.
func (s *PGStore) MaintainGitWebhookReplayEpochs(ctx context.Context, epochs []GitWebhookReplayEpoch, now time.Time) (int64, error) {
	now = now.UTC()
	ordered := append([]GitWebhookReplayEpoch(nil), epochs...)
	sort.Slice(ordered, func(i, j int) bool { return replayEpochLockKey(ordered[i]) < replayEpochLockKey(ordered[j]) })
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		for _, epoch := range ordered {
			if epoch.Epoch == "" || (epoch.KeyClass != GitWebhookReplayKeyManual && epoch.KeyClass != GitWebhookReplayKeyGitHub) {
				return fmt.Errorf("%w: invalid git webhook replay epoch", ErrInvalid)
			}
			// Registration and retirement of one epoch serialize even when a new
			// replica starts at the exact moment another replica sweeps.
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, replayEpochLockKey(epoch)); err != nil {
				return err
			}
			var retired bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM git_webhook_replay_retired_epochs
					WHERE key_class = $1 AND epoch = $2
				)`, epoch.KeyClass, epoch.Epoch).Scan(&retired); err != nil {
				return err
			}
			if retired {
				return ErrGitWebhookReplayEpochRetired
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO git_webhook_replay_epoch_leases (instance_id, key_class, epoch, heartbeat_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (instance_id, key_class) DO UPDATE
				SET epoch = EXCLUDED.epoch, heartbeat_at = EXCLUDED.heartbeat_at`,
				s.gitWebhookReplayInstance, epoch.KeyClass, epoch.Epoch, now); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}

	// Sweep in a separate transaction so registration never holds one epoch's
	// lock while waiting for another replica's epoch lock. Candidate ordering is
	// deterministic as a second defense against cross-sweeper deadlocks.
	var purged int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM git_webhook_replay_epoch_leases WHERE heartbeat_at < $1`,
			now.Add(-GitWebhookReplayLeaseTTL)); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT r.key_class, r.epoch
			FROM git_webhook_replays r
			WHERE r.epoch <> 'legacy'
			  AND NOT EXISTS (
				SELECT 1 FROM git_webhook_replay_epoch_leases l
				WHERE l.key_class = r.key_class AND l.epoch = r.epoch
			  )
			ORDER BY r.key_class, r.epoch`)
		if err != nil {
			return err
		}
		var candidates []GitWebhookReplayEpoch
		for rows.Next() {
			var epoch GitWebhookReplayEpoch
			if err := rows.Scan(&epoch.KeyClass, &epoch.Epoch); err != nil {
				rows.Close()
				return err
			}
			candidates = append(candidates, epoch)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, epoch := range candidates {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, replayEpochLockKey(epoch)); err != nil {
				return err
			}
			var active bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM git_webhook_replay_epoch_leases
					WHERE key_class = $1 AND epoch = $2
				)`, epoch.KeyClass, epoch.Epoch).Scan(&active); err != nil {
				return err
			}
			if active {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO git_webhook_replay_retired_epochs (key_class, epoch, retired_at)
				VALUES ($1, $2, $3)
				ON CONFLICT (key_class, epoch) DO NOTHING`, epoch.KeyClass, epoch.Epoch, now); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `
				DELETE FROM git_webhook_replays
				WHERE key_class = $1 AND epoch = $2`, epoch.KeyClass, epoch.Epoch)
			if err != nil {
				return err
			}
			purged += tag.RowsAffected()
		}
		return nil
	})
	return purged, err
}

func replayEpochLockKey(epoch GitWebhookReplayEpoch) string {
	return "git-webhook-replay-epoch\n" + epoch.KeyClass + "\n" + epoch.Epoch
}

func (s *PGStore) releaseGitWebhookReplayEpochLeases(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM git_webhook_replay_epoch_leases WHERE instance_id = $1`,
		s.gitWebhookReplayInstance)
	return err
}

// RunGitWebhookReplayMaintenance keeps epoch liveness authoritative across API
// replicas. A graceful stop removes its leases without waiting for crash expiry.
func (s *PGStore) RunGitWebhookReplayMaintenance(ctx context.Context, epochs []GitWebhookReplayEpoch) {
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = s.releaseGitWebhookReplayEpochLeases(releaseCtx)
	}()
	core.Poll(ctx, "git-webhook-replay-epochs", GitWebhookReplayMaintenanceInterval, func(ctx context.Context) error {
		purged, err := s.MaintainGitWebhookReplayEpochs(ctx, epochs, time.Now())
		if err == nil && purged > 0 {
			log.Printf("git-webhook-replay-epochs: purged %d claims for retired signing epochs", purged)
		}
		return err
	})
}
