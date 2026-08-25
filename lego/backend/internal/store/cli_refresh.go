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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const cliRefreshSweepLimit = 1000

// cliRefreshTTLCache is the process-local half of the CLI refresh idempotency
// boundary (codex-security 2026-08 F2). The durable row in
// cli_refresh_idempotency now records ONLY that a mint happened (the inbound
// token digest + expiry): Hydra's verbatim response — which for the CLI's
// offline_access grant contains the newly issued access token AND the rotated
// refresh token — must never sit in Postgres or its backups as plaintext. The
// response bytes live in this replica-local map for the idempotency TTL. The
// advisory lock serializes concurrent refreshes; a duplicate arriving on THIS
// replica within the TTL is served the cached bytes. A duplicate arriving on a
// DIFFERENT replica re-mints from Hydra — one extra upstream round trip, which
// the 60s rotation grace period (hydra.values.yaml) explicitly tolerates for
// exactly this near-simultaneous-refresh shape.
type cliRefreshTTLCache struct {
	mu      sync.Mutex
	entries map[[sha256.Size]byte]cliRefreshResponse
}

func (c *cliRefreshTTLCache) put(key [sha256.Size]byte, resp cliRefreshResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = resp
}

func (c *cliRefreshTTLCache) get(key [sha256.Size]byte) (cliRefreshResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.entries[key]
	return r, ok
}

// cliRefreshResponse is the exact successful OAuth response cached for one
// inbound refresh-token digest. Body deliberately remains opaque: Hydra owns
// the token schema, and same-replica duplicates must receive byte-identical
// JSON.
type cliRefreshResponse struct {
	Body   []byte
	Status int
}

type cliRefreshQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type cliRefreshExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// getCLIRefresh reports whether a live mint marker exists for tokenHash.
// Expired rows are logical misses even before the bounded opportunistic sweep
// physically removes them. It returns no response bytes — none are stored
// (codex-security 2026-08 F2).
func getCLIRefresh(ctx context.Context, q cliRefreshQuerier, tokenHash [sha256.Size]byte) (bool, error) {
	var one int
	err := q.QueryRow(ctx, `
		SELECT 1
		FROM cli_refresh_idempotency
		WHERE token_hash = $1 AND expires_at > clock_timestamp()`, tokenHash[:],
	).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return false, err
}

// putCLIRefresh records that a successful mint happened for tokenHash. The row
// is a MARKER, not a cache of the response: it carries no token material (F2).
func putCLIRefresh(ctx context.Context, q cliRefreshExecer, tokenHash [sha256.Size]byte, ttl time.Duration) error {
	_, err := q.Exec(ctx, `
		INSERT INTO cli_refresh_idempotency
			(token_hash, response_body, response_status, expires_at)
		VALUES ($1, ''::bytea, 200, clock_timestamp() + ($2::bigint * interval '1 microsecond'))
		ON CONFLICT (token_hash) DO UPDATE SET
			response_body = EXCLUDED.response_body,
			response_status = EXCLUDED.response_status,
			expires_at = EXCLUDED.expires_at,
			created_at = clock_timestamp()`,
		tokenHash[:], ttl.Microseconds())
	return err
}

// IdempotentCLIRefresh serializes one refresh-token digest across every API
// replica. The transaction-scoped advisory lock is acquired before the cache
// lookup and held until the mint marker has been persisted, so concurrent
// callers never mint simultaneously. Non-2xx OAuth responses and transport
// failures are returned but never cached.
//
// Split storage (codex-security 2026-08 F2): the DURABLE row records only that
// a successful mint happened for this digest (plus its expiry — the marker is
// what makes a duplicate re-mint safe to allow, because the 60s rotation grace
// window is still open); the RESPONSE BYTES stay in this process only. Before
// this, Hydra's verbatim token response — access token + rotated refresh
// token, i.e. live credentials carrying bex.read/write/sensitive — was
// persisted as plaintext bytea, readable by anyone with database or backup
// access. A duplicate on another replica re-mints (one extra Hydra round trip
// inside the grace window); a duplicate on this replica within the TTL gets
// the exact cached bytes, preserving the byte-identical-response contract for
// the CLI.
func (s *PGStore) IdempotentCLIRefresh(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	ttl time.Duration,
	mint func(context.Context) ([]byte, int, error),
) ([]byte, int, error) {
	if ttl <= 0 {
		return nil, 0, fmt.Errorf("CLI refresh cache TTL must be positive")
	}

	s.cliRefreshOnce.Do(func() {
		s.cliRefreshLocal = &cliRefreshTTLCache{entries: map[[sha256.Size]byte]cliRefreshResponse{}}
	})

	var body []byte
	var status int
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// PostgreSQL advisory locks take signed int64 keys. Reinterpreting the
		// digest prefix preserves all 64 bits and is stable across processes.
		lockKey := int64(binary.BigEndian.Uint64(tokenHash[:8]))
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
			return err
		}

		// Replica-local hit: serve the exact bytes the first caller saw.
		if cached, ok := s.cliRefreshLocal.get(tokenHash); ok {
			body, status = cached.Body, cached.Status
			return nil
		}

		// Durable hit (minted on another replica, or a prior local process):
		// re-mint rather than return stored bytes — nothing sensitive is
		// persisted to re-serve. The rotation grace window makes this safe.
		durableHit, err := getCLIRefresh(ctx, tx, tokenHash)
		if err != nil {
			return err
		}
		if durableHit {
			body, status, err = mint(ctx)
			if err != nil || status < 200 || status >= 300 {
				return err
			}
			s.cliRefreshLocal.put(tokenHash, cliRefreshResponse{Body: body, Status: status})
			return nil
		}

		if err := sweepExpiredCLIRefreshes(ctx, tx); err != nil {
			return err
		}
		body, status, err = mint(ctx)
		if err != nil || status < 200 || status >= 300 {
			return err
		}
		s.cliRefreshLocal.put(tokenHash, cliRefreshResponse{Body: body, Status: status})

		return putCLIRefresh(ctx, tx, tokenHash, ttl)
	})
	return body, status, err
}

func sweepExpiredCLIRefreshes(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		WITH expired AS (
			SELECT token_hash
			FROM cli_refresh_idempotency
			WHERE expires_at <= clock_timestamp()
			ORDER BY expires_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM cli_refresh_idempotency AS cache
		USING expired
		WHERE cache.token_hash = expired.token_hash`, cliRefreshSweepLimit)
	return err
}
