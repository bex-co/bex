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

// OIDCSigningKey is one row of the Managed OIDC key set
// (docs/ADR116-managed-oidc.md D5): published from CreatedAt, signing from
// ActivatesAt. PrivateKey is PKCS#8 DER; internal/oidcissuer parses it.
type OIDCSigningKey struct {
	KID         string
	PrivateKey  []byte
	CreatedAt   time.Time
	ActivatesAt time.Time
}

// ErrOIDCRotationPending refuses a second rotation while a key inserted by the
// first has not started signing yet (ADR116 D6: one rotation at a time).
var ErrOIDCRotationPending = errors.New("an OIDC signing key rotation is already pending")

// lockOIDCSigningKeys serializes every write to the key set, so a bootstrap
// and a rotation each decide on a set no concurrent write moves.
func lockOIDCSigningKeys(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('oidc-signing-keys', 0))`)
	return err
}

// OIDCSigningKeys returns the whole key set, oldest activation first.
func (s *PGStore) OIDCSigningKeys(ctx context.Context) ([]OIDCSigningKey, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT kid, private_key, created_at, activates_at FROM oidc_signing_keys ORDER BY activates_at, kid`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (OIDCSigningKey, error) {
		var k OIDCSigningKey
		err := row.Scan(&k.KID, &k.PrivateKey, &k.CreatedAt, &k.ActivatesAt)
		return k, err
	})
}

// BootstrapOIDCSigningKey stores a key that signs at once, but only when the
// key set is empty. Replicas starting together all call it; the advisory lock
// makes exactly one of them insert. created reports whether this call did.
func (s *PGStore) BootstrapOIDCSigningKey(ctx context.Context, kid string, privateKey []byte) (created bool, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockOIDCSigningKeys(ctx, tx); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`INSERT INTO oidc_signing_keys (kid, private_key, created_at, activates_at)
			 SELECT $1, $2, now(), now()
			 WHERE NOT EXISTS (SELECT 1 FROM oidc_signing_keys)`, kid, privateKey)
		created = tag.RowsAffected() == 1
		return err
	})
	return created, err
}

// RotateOIDCSigningKey stores a key that starts signing lead from now
// (database time). It refuses with ErrOIDCRotationPending while another key
// has yet to start signing. With replace it also deletes every other key, in
// the same transaction: the emergency rotation of ADR116 D6, after which
// nothing signed before verifies.
func (s *PGStore) RotateOIDCSigningKey(ctx context.Context, kid string, privateKey []byte, lead time.Duration, replace bool) (OIDCSigningKey, error) {
	var k OIDCSigningKey
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockOIDCSigningKeys(ctx, tx); err != nil {
			return err
		}
		if replace {
			if _, err := tx.Exec(ctx, `DELETE FROM oidc_signing_keys`); err != nil {
				return err
			}
		} else {
			var pending bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM oidc_signing_keys WHERE activates_at > now())`).Scan(&pending); err != nil {
				return err
			}
			if pending {
				return ErrOIDCRotationPending
			}
		}
		return tx.QueryRow(ctx,
			`INSERT INTO oidc_signing_keys (kid, private_key, created_at, activates_at)
			 VALUES ($1, $2, now(), now() + make_interval(secs => $3))
			 RETURNING kid, private_key, created_at, activates_at`,
			kid, privateKey, lead.Seconds()).Scan(&k.KID, &k.PrivateKey, &k.CreatedAt, &k.ActivatesAt)
	})
	return k, err
}

// DeleteOIDCSigningKeys removes keys that are no longer published (ADR116 D6:
// superseded long enough ago that nothing they signed is still valid). Two
// replicas pruning the same keys is harmless.
func (s *PGStore) DeleteOIDCSigningKeys(ctx context.Context, kids []string) error {
	if len(kids) == 0 {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM oidc_signing_keys WHERE kid = ANY($1)`, kids)
	return err
}
