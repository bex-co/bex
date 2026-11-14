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
	"encoding/json"
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A workspace's inbound IP rules (w9/m388, migration 0258): Render's Scale and
// Enterprise "entire workspace" rules. No row = the workspace never set rules
// (no layer); a saved empty list is deny-all. The rows hold Render's
// {cidrBlock, description} entries, the shape environments.ip_allow_list
// holds.

// WorkspaceIPAllowList reads workspaceID's stored rules. set is false when the
// workspace has no row. It satisfies core.WorkspaceIPRulesReader, the seam
// every create path reads the layer through.
func (s *PGStore) WorkspaceIPAllowList(ctx context.Context, workspaceID string) ([]core.IPAllowListEntry, bool, error) {
	lists, err := s.WorkspaceIPAllowLists(ctx, []string{workspaceID})
	if err != nil {
		return nil, false, err
	}
	entries, set := lists[workspaceID]
	return entries, set, nil
}

// WorkspaceIPAllowLists reads the stored rules of every workspace in
// workspaceIDs in one query; a workspace with no row is absent from the map.
// A present workspace's list is never nil, so a saved empty list stays
// distinct from no row.
func (s *PGStore) WorkspaceIPAllowLists(ctx context.Context, workspaceIDs []string) (map[string][]core.IPAllowListEntry, error) {
	out := map[string][]core.IPAllowListEntry{}
	if len(workspaceIDs) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT tenant_id, ip_allow_list FROM workspace_ip_allow_lists WHERE tenant_id = ANY($1)`, workspaceIDs)
	if err != nil {
		return nil, classify("workspace ip allow list", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, classify("workspace ip allow list", err)
		}
		entries, err := decodeWorkspaceIPAllowList(raw)
		if err != nil {
			return nil, err
		}
		out[id] = entries
	}
	if err := rows.Err(); err != nil {
		return nil, classify("workspace ip allow list", err)
	}
	return out, nil
}

// SetWorkspaceIPAllowList replaces workspaceID's rules, creating its row on
// the first save. nil is saved as the empty (deny-all) list, never as "no
// row". An unknown workspace is ErrNotFound.
func (s *PGStore) SetWorkspaceIPAllowList(ctx context.Context, workspaceID string, entries []core.IPAllowListEntry) error {
	if entries == nil {
		entries = []core.IPAllowListEntry{}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("workspace ip allow list: encoding: %w", err)
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO workspace_ip_allow_lists (tenant_id, ip_allow_list) VALUES ($1, $2)
		ON CONFLICT (tenant_id) DO UPDATE SET ip_allow_list = EXCLUDED.ip_allow_list, updated_at = now()`,
		workspaceID, raw)
	return classify("workspace ip allow list", err)
}

func decodeWorkspaceIPAllowList(raw []byte) ([]core.IPAllowListEntry, error) {
	entries := []core.IPAllowListEntry{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("workspace ip allow list: decoding: %w", err)
		}
	}
	if entries == nil {
		entries = []core.IPAllowListEntry{}
	}
	return entries, nil
}
