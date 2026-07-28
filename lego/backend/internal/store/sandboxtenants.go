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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Per-workspace OpenSandbox tenant keys (w3/m32 t006, ADR042 D4). bex-api mints
// one opaque key per workspace and sends it to the OpenSandbox server as the
// OPEN-SANDBOX-API-KEY header; the server resolves it back through bex-api's
// GET /v1/sandbox-tenants tenant-lookup endpoint (store/api.go) to the
// workspace's `<ws>-sandbox` namespace, then scopes every lifecycle op there.
// The key is a high-entropy secret (not a resource id) — never minted through
// internal/id, whose xid values are unguessable-in-practice but not secret.

// sandboxKeyBytes is the entropy of a per-workspace OpenSandbox tenant key: 32
// random bytes (256 bits), base64url-encoded and `osk-` prefixed for provenance.
const sandboxKeyBytes = 32

func newSandboxKey() (string, error) {
	b := make([]byte, sandboxKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("mint sandbox key: %w", err)
	}
	return "osk-" + base64.RawURLEncoding.EncodeToString(b), nil
}

// SandboxKeyForWorkspace returns the workspace's OpenSandbox tenant key, minting
// one on first use. Idempotent and race-safe: the UNIQUE(workspace_id) constraint
// collapses concurrent first-mints to a single key — ON CONFLICT returns the
// already-stored key rather than a second one. This is the KeyProvider the
// sandbox feature calls to stamp each request's OPEN-SANDBOX-API-KEY.
func (s *PGStore) SandboxKeyForWorkspace(ctx context.Context, workspaceID string) (string, error) {
	if workspaceID == "" {
		return "", fmt.Errorf("%w: empty workspace", ErrInvalid)
	}
	key, err := newSandboxKey()
	if err != nil {
		return "", err
	}
	var out string
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO sandbox_tenant_keys (api_key, workspace_id) VALUES ($1, $2)
		 ON CONFLICT (workspace_id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id
		 RETURNING api_key`,
		key, workspaceID,
	).Scan(&out)
	if err != nil {
		return "", classify("sandbox tenant key", err)
	}
	return out, nil
}

// WorkspaceForSandboxKey resolves an OpenSandbox tenant key to its workspace id,
// returning ErrNotFound for an unknown key — which the tenant-lookup endpoint
// maps to the 401 the OpenSandbox HTTP tenant provider expects (invalid key).
func (s *PGStore) WorkspaceForSandboxKey(ctx context.Context, apiKey string) (string, error) {
	if apiKey == "" {
		return "", ErrNotFound
	}
	var ws string
	err := s.Pool.QueryRow(ctx,
		`SELECT workspace_id FROM sandbox_tenant_keys WHERE api_key = $1`, apiKey,
	).Scan(&ws)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return ws, nil
}
