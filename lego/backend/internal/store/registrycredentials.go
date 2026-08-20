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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// RegistryCredential is a row of `registry_credentials` (w2/m14): the metadata
// for a workspace's stored credential to a private external image registry.
// The secret value is never a field here — it lives in OpenBao, resolved
// separately by the registrycreds feature (docs/ADR013-secrets.md, the same
// split the env-vars/secret-files feature uses).
type RegistryCredential struct {
	ID          string
	WorkspaceID string
	// Name is a human display label (Render's registryCredential.name) — the
	// caller may leave it empty at creation, in which case CreateRegistryCredential
	// defaults it to host.
	Name      string
	Host      string
	Username  string
	ExpiresAt *time.Time
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const registryCredentialColumns = `id, workspace_id, name, host, username, expires_at, created_by, created_at, updated_at`

func scanRegistryCredential(row pgx.Row) (RegistryCredential, error) {
	var c RegistryCredential
	err := row.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Host, &c.Username, &c.ExpiresAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// CreateRegistryCredential mints a new credential row (id.New(id.RegistryCredential))
// and inserts it. The caller writes the secret to OpenBao itself — this method
// only ever touches metadata. An empty name defaults to host.
func (s *PGStore) CreateRegistryCredential(ctx context.Context, workspaceID, name, host, username, createdBy string, expiresAt *time.Time) (RegistryCredential, error) {
	if name == "" {
		name = host
	}
	c := RegistryCredential{ID: ids.New(ids.RegistryCredential), WorkspaceID: workspaceID, Name: name, Host: host, Username: username, ExpiresAt: expiresAt, CreatedBy: createdBy}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO registry_credentials (id, workspace_id, name, host, username, expires_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at`,
		c.ID, c.WorkspaceID, c.Name, c.Host, c.Username, c.ExpiresAt, c.CreatedBy,
	).Scan(&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return RegistryCredential{}, classify("registry credential", err)
	}
	return c, nil
}

// ListRegistryCredentials returns a workspace's stored credentials, newest first.
func (s *PGStore) ListRegistryCredentials(ctx context.Context, workspaceID string) ([]RegistryCredential, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+registryCredentialColumns+` FROM registry_credentials WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegistryCredential
	for rows.Next() {
		c, err := scanRegistryCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountRegistryCredentials returns how many stored registry credentials the
// workspace owns. It backs the feature's per-workspace admission quota.
func (s *PGStore) CountRegistryCredentials(ctx context.Context, workspaceID string) (int, error) {
	var count int
	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM registry_credentials WHERE workspace_id = $1`, workspaceID,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetRegistryCredential returns one credential, scoped to workspaceID so a
// caller can never fetch another workspace's row by guessing its id.
func (s *PGStore) GetRegistryCredential(ctx context.Context, workspaceID, id string) (RegistryCredential, error) {
	c, err := scanRegistryCredential(s.Pool.QueryRow(ctx,
		`SELECT `+registryCredentialColumns+` FROM registry_credentials WHERE id = $1 AND workspace_id = $2`, id, workspaceID))
	if err != nil {
		return RegistryCredential{}, classify("registry credential", err)
	}
	return c, nil
}

// GetRegistryCredentialByID returns one credential without pre-scoping it to a
// workspace. It exists for resource-binding resolvers that must distinguish an
// unknown id (404) from an existing credential owned by another workspace
// (403). Public credential reads continue to use GetRegistryCredential so this
// lookup cannot accidentally become an unscoped read surface.
func (s *PGStore) GetRegistryCredentialByID(ctx context.Context, id string) (RegistryCredential, error) {
	c, err := scanRegistryCredential(s.Pool.QueryRow(ctx,
		`SELECT `+registryCredentialColumns+` FROM registry_credentials WHERE id = $1`, id))
	if err != nil {
		return RegistryCredential{}, classify("registry credential", err)
	}
	return c, nil
}

// GetRegistryCredentialsByIDs returns the credentials matching any id in ids.
// It is unscoped to a workspace, so callers must only use this for batch
// display enrichment (name resolution) where per-resource authorization has
// already been satisfied. Unknown ids are silently omitted.
func (s *PGStore) GetRegistryCredentialsByIDs(ctx context.Context, ids []string) ([]RegistryCredential, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT `+registryCredentialColumns+` FROM registry_credentials WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegistryCredential
	for rows.Next() {
		c, err := scanRegistryCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetRegistryCredentialByHost returns the workspace's credential for host, if
// any — the lookup the operator-wiring materialization path (w2/m14/t002)
// uses to decide whether an App's image needs a pull secret. Newest wins when
// a workspace somehow has more than one credential for the same host.
func (s *PGStore) GetRegistryCredentialByHost(ctx context.Context, workspaceID, host string) (RegistryCredential, error) {
	c, err := scanRegistryCredential(s.Pool.QueryRow(ctx,
		`SELECT `+registryCredentialColumns+` FROM registry_credentials WHERE workspace_id = $1 AND host = $2 ORDER BY created_at DESC LIMIT 1`, workspaceID, host))
	if err != nil {
		return RegistryCredential{}, classify("registry credential", err)
	}
	return c, nil
}

// UpdateRegistryCredential changes name/username/expiresAt in place (a
// rotated secret is a separate OpenBao write the caller makes itself — this
// method never touches secret material). Nil expiresAt clears any existing
// expiry.
func (s *PGStore) UpdateRegistryCredential(ctx context.Context, workspaceID, id, name, username string, expiresAt *time.Time) (RegistryCredential, error) {
	c, err := scanRegistryCredential(s.Pool.QueryRow(ctx,
		`UPDATE registry_credentials SET name = $3, username = $4, expires_at = $5, updated_at = now()
		 WHERE id = $1 AND workspace_id = $2
		 RETURNING `+registryCredentialColumns,
		id, workspaceID, name, username, expiresAt,
	))
	if err != nil {
		return RegistryCredential{}, classify("registry credential", err)
	}
	return c, nil
}

// TouchRegistryCredential bumps updated_at with no other column change — used
// when only the OpenBao-held secret was rotated, so the metadata row still
// reflects when the credential last changed.
func (s *PGStore) TouchRegistryCredential(ctx context.Context, workspaceID, id string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE registry_credentials SET updated_at = now() WHERE id = $1 AND workspace_id = $2`, id, workspaceID)
	if err != nil {
		return classify("registry credential", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRegistryCredential removes the row, scoped to workspaceID. Not-found
// (including a cross-workspace id) is ErrNotFound.
func (s *PGStore) DeleteRegistryCredential(ctx context.Context, workspaceID, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM registry_credentials WHERE id = $1 AND workspace_id = $2`, id, workspaceID)
	if err != nil {
		return classify("registry credential", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
