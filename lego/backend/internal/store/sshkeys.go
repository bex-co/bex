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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SSHKey is one identity-owned public key. Private key material never enters
// the API or store; Fingerprint is the canonical OpenSSH SHA256 fingerprint.
type SSHKey struct {
	ID          string    `json:"id"`
	Subject     string    `json:"-"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"publicKey"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"createdAt"`
}

const sshKeyColumns = `id, subject, name, public_key, fingerprint, created_at`

func scanSSHKey(row pgx.Row) (SSHKey, error) {
	var key SSHKey
	err := row.Scan(&key.ID, &key.Subject, &key.Name, &key.PublicKey, &key.Fingerprint, &key.CreatedAt)
	return key, err
}

// CreateSSHKey persists a canonicalized public key. The fingerprint's unique
// constraint is the ambiguity guard: one SSH handshake must map to one subject.
func (s *PGStore) CreateSSHKey(ctx context.Context, key SSHKey) (SSHKey, error) {
	var created SSHKey
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, key.Subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, key.Subject); err != nil {
			return err
		}
		var err error
		created, err = scanSSHKey(tx.QueryRow(ctx, `
			INSERT INTO ssh_keys (id, subject, name, public_key, fingerprint)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING `+sshKeyColumns,
			key.ID, key.Subject, key.Name, key.PublicKey, key.Fingerprint))
		return err
	})
	if err != nil {
		if err == ErrAccountDeletionPending {
			return SSHKey{}, err
		}
		return SSHKey{}, classify("ssh key", err)
	}
	return created, nil
}

// ListSSHKeys returns only one identity's keys in stable creation order.
func (s *PGStore) ListSSHKeys(ctx context.Context, subject string) ([]SSHKey, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+sshKeyColumns+`
		FROM ssh_keys WHERE subject = $1 ORDER BY created_at, id`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]SSHKey, 0)
	for rows.Next() {
		key, err := scanSSHKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// DeleteSSHKey scopes deletion by subject so even a guessed id cannot revoke
// another identity's access. Missing and foreign ids deliberately look alike.
func (s *PGStore) DeleteSSHKey(ctx context.Context, subject, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM ssh_keys WHERE id = $1 AND subject = $2`, id, subject)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ssh key %s: %w", id, ErrNotFound)
	}
	return nil
}

// SSHKeyByFingerprint is the gateway-only lookup that maps a presented public
// key to its owning subject. It returns public metadata only.
func (s *PGStore) SSHKeyByFingerprint(ctx context.Context, fingerprint string) (SSHKey, error) {
	key, err := scanSSHKey(s.Pool.QueryRow(ctx, `SELECT `+sshKeyColumns+`
		FROM ssh_keys WHERE fingerprint = $1`, fingerprint))
	if err != nil {
		return SSHKey{}, classify("ssh key", err)
	}
	return key, nil
}
