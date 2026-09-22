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

// AppPlacement is authoritative membership; runtime state remains on the App CR.
type AppPlacement struct {
	TenantID      string
	ProjectID     string
	EnvironmentID string
}

// GetAppPlacements reads only the requested immutable IDs. Missing rows are
// absent from the result; callers authorize the resources before requesting them.
func (s *PGStore) GetAppPlacements(ctx context.Context, appIDs []string) (map[string]AppPlacement, error) {
	out := make(map[string]AppPlacement, len(appIDs))
	if len(appIDs) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, tenant_id, COALESCE(project_id, ''), COALESCE(environment_id, '')
		FROM apps WHERE id = ANY($1)`, appIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var placement AppPlacement
		if err := rows.Scan(&id, &placement.TenantID, &placement.ProjectID, &placement.EnvironmentID); err != nil {
			return nil, err
		}
		out[id] = placement
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
