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

// IsAgentSessionDescendant reports whether sessionID is a strict descendant of
// ancestorID in the workspace: its parent, its parent's parent, and so on.
// A session is not its own descendant, and a sibling or a parent is not a
// descendant. It walks up from sessionID (the depth limit bounds the walk), so
// a deleted link in the chain ends the walk and answers false.
func (s *PGStore) IsAgentSessionDescendant(ctx context.Context, workspaceID, ancestorID, sessionID string) (bool, error) {
	if !textBindable(ancestorID) || !textBindable(sessionID) || ancestorID == sessionID {
		return false, nil
	}
	var found bool
	err := s.Pool.QueryRow(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_session_id, 0 AS hops FROM agent_sessions WHERE workspace_id = $1 AND id = $3
			UNION ALL
			SELECT p.id, p.parent_session_id, up.hops + 1 FROM agent_sessions p
			JOIN up ON p.id = up.parent_session_id WHERE p.workspace_id = $1 AND up.hops < 8
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2 AND hops > 0)`, workspaceID, ancestorID, sessionID).Scan(&found)
	return found, err
}
