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

import "context"

// SetSandboxName records a sandbox's Render name (w5/m287).
func (s *PGStore) SetSandboxName(ctx context.Context, workspaceID, sandboxID, name string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sandbox_names (workspace_id, sandbox_id, name) VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, sandbox_id) DO UPDATE SET name = EXCLUDED.name`, workspaceID, sandboxID, name)
	return classify("sandbox name", err)
}

// DeleteSandboxName drops the name of a sandbox whose create failed.
func (s *PGStore) DeleteSandboxName(ctx context.Context, workspaceID, sandboxID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sandbox_names WHERE workspace_id = $1 AND sandbox_id = $2`, workspaceID, sandboxID)
	return err
}

// SandboxNames returns one workspace's sandbox names by sandbox id.
func (s *PGStore) SandboxNames(ctx context.Context, workspaceID string) (map[string]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT sandbox_id, name FROM sandbox_names WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// PruneSandboxNames deletes the names of one workspace's sandboxes absent
// from a complete OpenSandbox list. A row younger than ten minutes is kept: it
// may belong to a create that has not reached the list yet.
func (s *PGStore) PruneSandboxNames(ctx context.Context, workspaceID string, live []string) error {
	if live == nil {
		live = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM sandbox_names
		WHERE workspace_id = $1 AND NOT (sandbox_id = ANY($2::text[])) AND created_at < now() - interval '10 minutes'`,
		workspaceID, live)
	return err
}
