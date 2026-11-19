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
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

var (
	// ErrObjectAccessKeyQuota refuses a key past the live-key cap of its
	// workspace (persistent) or creator (ephemeral).
	ErrObjectAccessKeyQuota = fmt.Errorf("object access key limit reached: %w", ErrConflict)
)

// ObjectAccessKey is one row of object_access_keys (ADR112 D3). The secret is
// not a column: it is in OpenBao at object-storage/keys/<ID>.
type ObjectAccessKey struct {
	ID          string
	WorkspaceID string
	// CreatedBy is the person every request signed with this key is authorized
	// as, on exactly WorkspaceID.
	CreatedBy string
	Name      string
	// Permission is "read" or "readwrite": the ceiling of what a request signed
	// with the key may do.
	Permission string
	// CreatedSignIn is how the creator was signed in when they made the key.
	CreatedSignIn core.SignIn
	CreatedAt     time.Time
	// ExpiresAt is nil for a persistent key.
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	// Broken marks a key revoked because its OpenBao secret is missing.
	Broken bool
}

// Ephemeral reports whether the key expires.
func (k ObjectAccessKey) Ephemeral() bool { return k.ExpiresAt != nil }

// Live reports whether the key may sign a request at now.
func (k ObjectAccessKey) Live(now time.Time) bool {
	return k.RevokedAt == nil && (k.ExpiresAt == nil || now.Before(*k.ExpiresAt))
}

const objectAccessKeyColumns = `id, workspace_id, created_by, name, permission, COALESCE(created_sign_in, ''), created_at, expires_at, last_used_at, revoked_at, broken`

func scanObjectAccessKey(row pgx.Row) (ObjectAccessKey, error) {
	var k ObjectAccessKey
	var signIn string
	err := row.Scan(&k.ID, &k.WorkspaceID, &k.CreatedBy, &k.Name, &k.Permission, &signIn, &k.CreatedAt, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.Broken)
	k.CreatedSignIn = core.ParseSignIn(signIn)
	return k, err
}

// CreateObjectAccessKey inserts a key and enforces its cap in the same
// transaction: at most maxPersistent live persistent keys per workspace, or
// maxEphemeral live ephemeral keys per creator in the workspace, as the new key's
// kind says. Two creates race on the workspace's advisory lock, so the cap holds.
func (s *PGStore) CreateObjectAccessKey(ctx context.Context, in ObjectAccessKey, maxPersistent, maxEphemeral int) (ObjectAccessKey, error) {
	var out ObjectAccessKey
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('object_access_keys:' || $1))`, in.WorkspaceID); err != nil {
			return err
		}
		// An ephemeral key counts against its creator's live ephemeral keys, a
		// persistent one against the workspace's live persistent keys.
		countSQL, cap, args := `SELECT count(*) FROM object_access_keys
			WHERE workspace_id = $1 AND expires_at IS NULL AND revoked_at IS NULL`, maxPersistent, []any{in.WorkspaceID}
		if in.Ephemeral() {
			countSQL, cap, args = `SELECT count(*) FROM object_access_keys
				WHERE workspace_id = $1 AND created_by = $2 AND expires_at > now() AND revoked_at IS NULL`, maxEphemeral, []any{in.WorkspaceID, in.CreatedBy}
		}
		var live int
		if err := tx.QueryRow(ctx, countSQL, args...).Scan(&live); err != nil {
			return err
		}
		if live >= cap {
			return ErrObjectAccessKeyQuota
		}
		var err error
		out, err = scanObjectAccessKey(tx.QueryRow(ctx, `
			INSERT INTO object_access_keys (id, workspace_id, created_by, name, permission, created_sign_in, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING `+objectAccessKeyColumns,
			in.ID, in.WorkspaceID, in.CreatedBy, in.Name, in.Permission, nullIfEmpty(string(in.CreatedSignIn)), in.ExpiresAt))
		return err
	})
	if errors.Is(err, ErrObjectAccessKeyQuota) {
		return ObjectAccessKey{}, err
	}
	if err != nil {
		return ObjectAccessKey{}, classify("object access key", err)
	}
	return out, nil
}

// GetObjectAccessKey returns one key in any state, or ErrNotFound. The S3
// surface reads it by the id a request names, so it sees revoked and expired
// rows too and decides what they mean.
func (s *PGStore) GetObjectAccessKey(ctx context.Context, id string) (ObjectAccessKey, error) {
	k, err := scanObjectAccessKey(s.Pool.QueryRow(ctx, `SELECT `+objectAccessKeyColumns+` FROM object_access_keys WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ObjectAccessKey{}, fmt.Errorf("object access key %s: %w", id, ErrNotFound)
	}
	return k, err
}

// ListObjectAccessKeys returns a workspace's keys in creation order. A revoked
// or expired key is still listed, with its state, until the sweep deletes it.
func (s *PGStore) ListObjectAccessKeys(ctx context.Context, workspaceID string) ([]ObjectAccessKey, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+objectAccessKeyColumns+`
		FROM object_access_keys WHERE workspace_id = $1 ORDER BY created_at, id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]ObjectAccessKey, 0)
	for rows.Next() {
		k, err := scanObjectAccessKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// RevokeObjectAccessKey marks a key of the workspace revoked and returns it. A
// key that is missing, in another workspace or already revoked is ErrNotFound,
// so a guessed id leaks nothing.
func (s *PGStore) RevokeObjectAccessKey(ctx context.Context, workspaceID, id string) (ObjectAccessKey, error) {
	k, err := scanObjectAccessKey(s.Pool.QueryRow(ctx, `
		UPDATE object_access_keys SET revoked_at = now()
		WHERE id = $1 AND workspace_id = $2 AND revoked_at IS NULL
		RETURNING `+objectAccessKeyColumns, id, workspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ObjectAccessKey{}, fmt.Errorf("object access key %s: %w", id, ErrNotFound)
	}
	return k, err
}

// MarkObjectAccessKeyBroken revokes a key whose OpenBao secret has gone missing
// and marks it broken, so the listing says why it stopped working.
func (s *PGStore) MarkObjectAccessKeyBroken(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE object_access_keys SET revoked_at = COALESCE(revoked_at, now()), broken = true WHERE id = $1`, id)
	return err
}

// TouchObjectAccessKey records that a key signed a request.
func (s *PGStore) TouchObjectAccessKey(ctx context.Context, id string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE object_access_keys SET last_used_at = $2 WHERE id = $1`, id, at)
	return err
}

// DeleteObjectAccessKey removes a key's row: the rollback of a create whose
// secret could not be written is not a revoke, it never existed.
func (s *PGStore) DeleteObjectAccessKey(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM object_access_keys WHERE id = $1`, id)
	return err
}

// DeleteObjectAccessKeysOf deletes the keys a person made, in one workspace or,
// when workspaceID is empty, in all of them, and returns what it deleted so the
// caller removes their OpenBao secrets. A member's removal and an account's
// deletion call it: the rows are deleted, not revoked, so re-adding the creator
// never revives a key (ADR112 D3).
func (s *PGStore) DeleteObjectAccessKeysOf(ctx context.Context, createdBy, workspaceID string) ([]ObjectAccessKey, error) {
	return s.deleteObjectAccessKeys(ctx, `created_by = $1 AND ($2 = '' OR workspace_id = $2)`, createdBy, workspaceID)
}

// DeleteObjectAccessKeysOfWorkspace deletes every key of a workspace, for its
// own deletion.
func (s *PGStore) DeleteObjectAccessKeysOfWorkspace(ctx context.Context, workspaceID string) ([]ObjectAccessKey, error) {
	return s.deleteObjectAccessKeys(ctx, `workspace_id = $1`, workspaceID)
}

// DeleteEndedObjectAccessKeys deletes the keys that were revoked or expired
// before cutoff, and the keys of a workspace whose tenant row is gone (a delete's
// purge that failed), and returns them, for the sweep to remove their secrets.
func (s *PGStore) DeleteEndedObjectAccessKeys(ctx context.Context, cutoff time.Time) ([]ObjectAccessKey, error) {
	return s.deleteObjectAccessKeys(ctx, `revoked_at < $1 OR expires_at < $1 OR NOT EXISTS (SELECT 1 FROM tenants t WHERE t.id = object_access_keys.workspace_id)`, cutoff)
}

func (s *PGStore) deleteObjectAccessKeys(ctx context.Context, where string, args ...any) ([]ObjectAccessKey, error) {
	rows, err := s.Pool.Query(ctx, `DELETE FROM object_access_keys WHERE `+where+` RETURNING `+objectAccessKeyColumns, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectAccessKey
	for rows.Next() {
		k, err := scanObjectAccessKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
