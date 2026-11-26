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
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// avatars.go keeps account and workspace avatars (w1/m648, migration 0348):
// one uploaded image per owner, or none, and the single-use upload slots
// avatarPresignedUpload mints.

// MaxAvatarBytes is the largest avatar image bex keeps. The dashboard uploads
// a 128x128 JPEG of a few kilobytes; the cap only bounds a direct API caller.
const MaxAvatarBytes = 1 << 20

// AvatarOwner names whose avatar a row or slot is: an own- id for an ACCOUNT,
// a tea- id for a WORKSPACE.
type AvatarOwner = core.AvatarOwner

// avatarWorkspaceID is the workspace column: the owner for a WORKSPACE, NULL
// for an ACCOUNT.
func avatarWorkspaceID(o AvatarOwner) *string {
	if o.Type == core.AvatarTypeWorkspace {
		return &o.ID
	}
	return nil
}

// Avatar is one stored image.
type Avatar struct {
	Owner       AvatarOwner
	DownloadKey string
	ContentType string
	Content     []byte
}

// AvatarUpload is an open upload slot. TokenHash is the SHA-256 of the token
// the upload URL carries; the token itself is never stored.
type AvatarUpload struct {
	TokenHash []byte
	Owner     AvatarOwner
	SizeBytes int
	Subject   string
	ExpiresAt time.Time
}

// CreateAvatarUpload records an upload slot. Slots that have expired are
// swept on the way, so the table holds only the last few minutes' slots.
func (s *PGStore) CreateAvatarUpload(ctx context.Context, u AvatarUpload) error {
	return classify("avatar upload", pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM avatar_uploads WHERE expires_at < now()`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO avatar_uploads
			(token_hash, owner_id, avatar_type, workspace_id, size_bytes, subject, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			u.TokenHash, u.Owner.ID, u.Owner.Type, avatarWorkspaceID(u.Owner), u.SizeBytes, u.Subject, u.ExpiresAt)
		return err
	}))
}

// RedeemAvatarUpload spends the slot whose token hashes to tokenHash: it is
// gone afterwards whatever the caller does with it. A slot that does not
// exist, was spent, or expired before now is ErrNotFound.
func (s *PGStore) RedeemAvatarUpload(ctx context.Context, tokenHash []byte, now time.Time) (AvatarUpload, error) {
	u := AvatarUpload{TokenHash: tokenHash}
	err := s.Pool.QueryRow(ctx, `DELETE FROM avatar_uploads WHERE token_hash = $1
		RETURNING owner_id, avatar_type, size_bytes, subject, expires_at`, tokenHash,
	).Scan(&u.Owner.ID, &u.Owner.Type, &u.SizeBytes, &u.Subject, &u.ExpiresAt)
	if err != nil {
		return AvatarUpload{}, classify("avatar upload", err)
	}
	if !now.Before(u.ExpiresAt) {
		return AvatarUpload{}, classify("avatar upload", pgx.ErrNoRows)
	}
	return u, nil
}

// PutAvatar stores a.Owner's image under a.DownloadKey, replacing the one it
// had. An account avatar is refused for an account whose deletion is pending,
// so an upload cannot recreate the row its cleanup removes; a workspace avatar
// for a deleted workspace is ErrNotFound.
func (s *PGStore) PutAvatar(ctx context.Context, a Avatar, subject string) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if a.Owner.Type == core.AvatarTypeAccount {
			if err := lockSubjectMembership(ctx, tx, subject); err != nil {
				return err
			}
			if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO avatars
			(owner_id, avatar_type, workspace_id, download_key, content_type, content)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (owner_id) DO UPDATE SET download_key = EXCLUDED.download_key,
				content_type = EXCLUDED.content_type, content = EXCLUDED.content, updated_at = now()`,
			a.Owner.ID, a.Owner.Type, avatarWorkspaceID(a.Owner), a.DownloadKey, a.ContentType, a.Content)
		return err
	})
	return classify("avatar", err)
}

// DeleteAvatar removes owner's image; true when it had one.
func (s *PGStore) DeleteAvatar(ctx context.Context, owner AvatarOwner) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM avatars WHERE owner_id = $1 AND avatar_type = $2`, owner.ID, owner.Type)
	if err != nil {
		return false, classify("avatar", err)
	}
	return tag.RowsAffected() > 0, nil
}

// AvatarByDownloadKey reads the image a download URL names.
func (s *PGStore) AvatarByDownloadKey(ctx context.Context, key string) (Avatar, error) {
	if !textBindable(key) {
		return Avatar{}, classify("avatar", pgx.ErrNoRows)
	}
	a := Avatar{DownloadKey: key}
	err := s.Pool.QueryRow(ctx, `SELECT owner_id, avatar_type, content_type, content
		FROM avatars WHERE download_key = $1`, key,
	).Scan(&a.Owner.ID, &a.Owner.Type, &a.ContentType, &a.Content)
	if err != nil {
		return Avatar{}, classify("avatar", err)
	}
	return a, nil
}

// AvatarDownloadKeys answers the download key of each owner in owners that
// has an avatar, in one read. An owner without one is absent from the map.
func (s *PGStore) AvatarDownloadKeys(ctx context.Context, owners []AvatarOwner) (map[AvatarOwner]string, error) {
	out := make(map[AvatarOwner]string, len(owners))
	if len(owners) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(owners))
	for _, o := range owners {
		if textBindable(o.ID) {
			ids = append(ids, o.ID)
		}
	}
	rows, err := s.Pool.Query(ctx, `SELECT owner_id, avatar_type, download_key FROM avatars WHERE owner_id = ANY($1)`, ids)
	if err != nil {
		return nil, classify("avatar", err)
	}
	defer rows.Close()
	for rows.Next() {
		var o AvatarOwner
		var key string
		if err := rows.Scan(&o.ID, &o.Type, &key); err != nil {
			return nil, err
		}
		out[o] = key
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
