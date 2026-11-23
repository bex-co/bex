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
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ErrLogSubscriptionLimit is returned when acquiring a log subscription slot
// would exceed the global, per-subject, or per-workspace cap.
var ErrLogSubscriptionLimit = core.ErrLogSubscriptionLimit

const (
	defaultRateLimitBucketIdle = 10 * time.Minute
	rateLimitFloatEpsilon      = 1e-9
)

// Compile-time check that *PGStore satisfies core.RateLimitStore.
var _ core.RateLimitStore = (*PGStore)(nil)

func refillRateLimitTokens(tokens float64, updatedAt, now time.Time, spec core.RateLimitBucketSpec) float64 {
	elapsed := now.Sub(updatedAt).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	refilled := tokens + elapsed*spec.RPS
	maxTokens := float64(spec.Burst)
	if refilled > maxTokens {
		return maxTokens
	}
	return refilled
}

func bucketIdle(spec core.RateLimitBucketSpec) time.Duration {
	if spec.Idle > 0 {
		return spec.Idle
	}
	return defaultRateLimitBucketIdle
}

func durationForDeficit(deficit, rps float64) time.Duration {
	if deficit <= 0 {
		return 0
	}
	if rps <= 0 {
		return time.Second
	}
	d := time.Duration((deficit / rps) * float64(time.Second))
	if d <= 0 {
		return time.Nanosecond
	}
	return d
}

// TakeRateLimitBuckets atomically refills and charges every spec in one
// transaction, locking rows in sorted bucket_key order to prevent deadlocks.
// If any bucket cannot supply min(n, burst) tokens, the transaction rolls back
// so no earlier bucket loses a token, and wait reports how long until the
// tightest failing bucket can serve the request.
func (s *PGStore) TakeRateLimitBuckets(ctx context.Context, now time.Time, n int, specs []core.RateLimitBucketSpec) ([]core.RateLimitBucketState, time.Duration, error) {
	if len(specs) == 0 {
		return nil, 0, nil
	}
	order := sortedSpecIndices(specs)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	type lockedBucket struct {
		key       string
		tokens    float64
		updatedAt time.Time
		idle      time.Duration
	}
	locked := make(map[string]*lockedBucket, len(specs))
	tokensBefore := make([]float64, len(specs))
	tokensAfter := make([]float64, len(specs))

	refused := false
	var maxWait time.Duration

	for _, idx := range order {
		spec := specs[idx]
		lb, seen := locked[spec.Key]
		if !seen {
			storedTokens, storedUpdatedAt, err := lockOrInitRateLimitBucket(ctx, tx, now, spec)
			if err != nil {
				return nil, 0, err
			}
			lb = &lockedBucket{
				key:       spec.Key,
				tokens:    refillRateLimitTokens(storedTokens, storedUpdatedAt, now, spec),
				updatedAt: storedUpdatedAt,
				idle:      bucketIdle(spec),
			}
			locked[spec.Key] = lb
		} else {
			if idle := bucketIdle(spec); idle > lb.idle {
				lb.idle = idle
			}
			if lb.tokens > float64(spec.Burst) {
				lb.tokens = float64(spec.Burst)
			}
		}

		tokensBefore[idx] = lb.tokens
		need := 0
		if n > 0 {
			need = min(n, spec.Burst)
		}
		if lb.tokens+rateLimitFloatEpsilon < float64(need) {
			refused = true
			if w := durationForDeficit(float64(need)-lb.tokens, spec.RPS); w > maxWait {
				maxWait = w
			}
			tokensAfter[idx] = lb.tokens
			continue
		}
		lb.tokens = max(0, lb.tokens-float64(need))
		tokensAfter[idx] = lb.tokens
	}

	states := make([]core.RateLimitBucketState, len(specs))
	if refused {
		for i, spec := range specs {
			states[i] = core.RateLimitBucketState{
				Tokens: tokensBefore[i],
				RPS:    spec.RPS,
				Burst:  spec.Burst,
			}
		}
		return states, maxWait, nil
	}

	// Persist each distinct bucket in sorted key order.
	var prevKey string
	for i, idx := range order {
		key := specs[idx].Key
		if i > 0 && key == prevKey {
			continue
		}
		prevKey = key
		lb := locked[key]
		nextUpdated := now
		if lb.updatedAt.After(nextUpdated) {
			nextUpdated = lb.updatedAt
		}
		if _, err := tx.Exec(ctx, `
			UPDATE rate_limit_buckets
			SET tokens = $2, updated_at = $3, expires_at = $4
			WHERE bucket_key = $1`,
			key, lb.tokens, nextUpdated, nextUpdated.Add(lb.idle)); err != nil {
			return nil, 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}

	for i, spec := range specs {
		states[i] = core.RateLimitBucketState{
			Tokens: tokensAfter[i],
			RPS:    spec.RPS,
			Burst:  spec.Burst,
		}
	}
	return states, 0, nil
}

func lockOrInitRateLimitBucket(ctx context.Context, tx pgx.Tx, now time.Time, spec core.RateLimitBucketSpec) (float64, time.Time, error) {
	idle := bucketIdle(spec)
	for range 2 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rate_limit_buckets (bucket_key, tokens, updated_at, expires_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (bucket_key) DO NOTHING`,
			spec.Key, float64(spec.Burst), now, now.Add(idle)); err != nil {
			return 0, time.Time{}, err
		}
		var tokens float64
		var updatedAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT tokens, updated_at
			FROM rate_limit_buckets
			WHERE bucket_key = $1
			FOR UPDATE`, spec.Key).Scan(&tokens, &updatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			// Concurrent PruneRateLimitBuckets deleted the row between INSERT and SELECT.
			continue
		}
		return tokens, updatedAt, err
	}
	return 0, time.Time{}, pgx.ErrNoRows
}

// RefundRateLimitBuckets atomically returns n tokens (capped at each bucket's
// burst) in sorted key order. Missing rows are already full and skipped.
func (s *PGStore) RefundRateLimitBuckets(ctx context.Context, now time.Time, n int, specs []core.RateLimitBucketSpec) error {
	if n <= 0 || len(specs) == 0 {
		return nil
	}
	order := sortedSpecIndices(specs)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, idx := range order {
		spec := specs[idx]
		var storedTokens float64
		var storedUpdatedAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT tokens, updated_at
			FROM rate_limit_buckets
			WHERE bucket_key = $1
			FOR UPDATE`, spec.Key).Scan(&storedTokens, &storedUpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		refilled := refillRateLimitTokens(storedTokens, storedUpdatedAt, now, spec)
		newTokens := min(float64(spec.Burst), refilled+float64(n))
		nextUpdated := now
		if storedUpdatedAt.After(nextUpdated) {
			nextUpdated = storedUpdatedAt
		}
		if _, err := tx.Exec(ctx, `
			UPDATE rate_limit_buckets
			SET tokens = $2, updated_at = $3, expires_at = GREATEST(expires_at, $4)
			WHERE bucket_key = $1`,
			spec.Key, newTokens, nextUpdated, nextUpdated.Add(bucketIdle(spec))); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PeekRateLimitBucket returns a bucket's refilled state at now without
// deducting tokens or creating a row.
func (s *PGStore) PeekRateLimitBucket(ctx context.Context, now time.Time, spec core.RateLimitBucketSpec) (core.RateLimitBucketState, error) {
	var storedTokens float64
	var storedUpdatedAt time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT tokens, updated_at
		FROM rate_limit_buckets
		WHERE bucket_key = $1`, spec.Key).Scan(&storedTokens, &storedUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.RateLimitBucketState{
			Tokens: float64(spec.Burst),
			RPS:    spec.RPS,
			Burst:  spec.Burst,
		}, nil
	}
	if err != nil {
		return core.RateLimitBucketState{}, err
	}
	return core.RateLimitBucketState{
		Tokens: refillRateLimitTokens(storedTokens, storedUpdatedAt, now, spec),
		RPS:    spec.RPS,
		Burst:  spec.Burst,
	}, nil
}

// PruneRateLimitBuckets deletes idle token-bucket rows whose expires_at has
// passed, returning the number of deleted rows.
func (s *PGStore) PruneRateLimitBuckets(ctx context.Context, now time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM rate_limit_buckets WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func sortedSpecIndices(specs []core.RateLimitBucketSpec) []int {
	order := make([]int, len(specs))
	for i := range specs {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		switch {
		case specs[a].Key < specs[b].Key:
			return -1
		case specs[a].Key > specs[b].Key:
			return 1
		default:
			return 0
		}
	})
	return order
}

// AcquireLogSubscriptionSlot atomically reclaims expired leases, enforces the
// global, per-subject (when subject != ""), and per-workspace caps across all
// bex-api replicas, and records a lease expiring at now + ttl.
func (s *PGStore) AcquireLogSubscriptionSlot(
	ctx context.Context,
	now time.Time,
	slotID, subject, workspace string,
	ttl time.Duration,
	maxGlobal int64,
	maxSubject, maxWorkspace int,
) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "log-subscription-slots"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM log_subscription_slots WHERE expires_at <= $1`, now); err != nil {
		return err
	}
	var globalCount, subjectCount, workspaceCount int64
	if err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE $1 <> '' AND subject = $1),
			COUNT(*) FILTER (WHERE workspace = $2)
		FROM log_subscription_slots`, subject, workspace).Scan(&globalCount, &subjectCount, &workspaceCount); err != nil {
		return err
	}
	if maxGlobal > 0 && globalCount >= maxGlobal {
		return ErrLogSubscriptionLimit
	}
	if maxSubject > 0 && subject != "" && subjectCount >= int64(maxSubject) {
		return ErrLogSubscriptionLimit
	}
	if maxWorkspace > 0 && workspaceCount >= int64(maxWorkspace) {
		return ErrLogSubscriptionLimit
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO log_subscription_slots (slot_id, subject, workspace, expires_at)
		VALUES ($1, $2, $3, $4)`,
		slotID, subject, workspace, now.Add(ttl)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RenewLogSubscriptionSlot extends an active log-subscription lease.
func (s *PGStore) RenewLogSubscriptionSlot(ctx context.Context, slotID string, expiresAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE log_subscription_slots
		SET expires_at = $2
		WHERE slot_id = $1`, slotID, expiresAt)
	return err
}

// ReleaseLogSubscriptionSlot deletes a log-subscription lease immediately when
// the stream closes.
func (s *PGStore) ReleaseLogSubscriptionSlot(ctx context.Context, slotID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM log_subscription_slots WHERE slot_id = $1`, slotID)
	return err
}
