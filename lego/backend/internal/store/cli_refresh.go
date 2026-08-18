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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const cliRefreshSweepLimit = 1000

// cliRefreshResponse is the exact successful OAuth response cached for one
// inbound refresh-token digest. Body deliberately remains opaque: Hydra owns
// the token schema, and the official CLI must receive byte-identical JSON on a
// duplicate request.
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

// getCLIRefresh returns a live cached response for tokenHash. Expired rows are
// logical misses even before the bounded opportunistic sweep physically
// removes them.
func getCLIRefresh(ctx context.Context, q cliRefreshQuerier, tokenHash [sha256.Size]byte) (cliRefreshResponse, bool, error) {
	var out cliRefreshResponse
	err := q.QueryRow(ctx, `
		SELECT response_body, response_status
		FROM cli_refresh_idempotency
		WHERE token_hash = $1 AND expires_at > clock_timestamp()`, tokenHash[:],
	).Scan(&out.Body, &out.Status)
	if err == nil {
		return out, true, nil
	}
	if err == pgx.ErrNoRows {
		return cliRefreshResponse{}, false, nil
	}
	return cliRefreshResponse{}, false, err
}

func putCLIRefresh(ctx context.Context, q cliRefreshExecer, tokenHash [sha256.Size]byte, response cliRefreshResponse, ttl time.Duration) error {
	_, err := q.Exec(ctx, `
		INSERT INTO cli_refresh_idempotency
			(token_hash, response_body, response_status, expires_at)
		VALUES ($1, $2, $3, clock_timestamp() + ($4::bigint * interval '1 microsecond'))
		ON CONFLICT (token_hash) DO UPDATE SET
			response_body = EXCLUDED.response_body,
			response_status = EXCLUDED.response_status,
			expires_at = EXCLUDED.expires_at,
			created_at = clock_timestamp()`,
		tokenHash[:], response.Body, response.Status, ttl.Microseconds())
	return err
}

// IdempotentCLIRefresh serializes one refresh-token digest across every API
// replica. The transaction-scoped advisory lock is acquired before the cache
// lookup and held until a fresh Hydra response has been persisted, so only one
// caller can mint from a rotating refresh token. Non-2xx OAuth responses and
// transport failures are returned but never cached.
func (s *PGStore) IdempotentCLIRefresh(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	ttl time.Duration,
	mint func(context.Context) ([]byte, int, error),
) ([]byte, int, error) {
	if ttl <= 0 {
		return nil, 0, fmt.Errorf("CLI refresh cache TTL must be positive")
	}

	var body []byte
	var status int
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// PostgreSQL advisory locks take signed int64 keys. Reinterpreting the
		// digest prefix preserves all 64 bits and is stable across processes.
		lockKey := int64(binary.BigEndian.Uint64(tokenHash[:8]))
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
			return err
		}

		cached, ok, err := getCLIRefresh(ctx, tx, tokenHash)
		if err != nil {
			return err
		}
		if ok {
			body, status = cached.Body, cached.Status
			return nil
		}

		if err := sweepExpiredCLIRefreshes(ctx, tx); err != nil {
			return err
		}
		body, status, err = mint(ctx)
		if err != nil || status < 200 || status >= 300 {
			return err
		}

		return putCLIRefresh(ctx, tx, tokenHash, cliRefreshResponse{
			Body: body, Status: status,
		}, ttl)
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
