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
	"strings"
)

// ResourceDisplayName is one retained (kind, id) -> name record for a tenant.
// See migration 0121: the record outlives the resource so a charge line can
// still name what it is billing for.
type ResourceDisplayName struct {
	Kind string // ResourceKind* — "service", "postgres", "key_value", "sandbox"
	ID   string
	Name string
}

// ResourceDisplayNameKey is the map key the usage presenter already uses to
// pair a kind with an id; different kinds may legally share an id.
func ResourceDisplayNameKey(kind, id string) string { return kind + "/" + id }

// RecordResourceDisplayNames upserts a batch of retained names for one tenant.
// It is called while the resources are alive: at service create and rename, and
// from the usage read that is by construction the one place every *metered*
// resource is enumerated. Blank kinds, ids and names are skipped rather than
// stored, so a resource bex never knew the name of leaves no misleading row.
//
// Upsert, not insert: a rename must move the retained name forward, which is
// what makes a live renamed resource bill under its current name.
func (s *PGStore) RecordResourceDisplayNames(ctx context.Context, tenantID string, records []ResourceDisplayName) error {
	if tenantID == "" || len(records) == 0 {
		return nil
	}
	kinds := make([]string, 0, len(records))
	ids := make([]string, 0, len(records))
	names := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, r := range records {
		kind, id, name := strings.TrimSpace(r.Kind), strings.TrimSpace(r.ID), strings.TrimSpace(r.Name)
		if kind == "" || id == "" || name == "" {
			continue
		}
		key := ResourceDisplayNameKey(kind, id)
		if _, dup := seen[key]; dup {
			// ON CONFLICT cannot see a duplicate inside its own statement.
			continue
		}
		seen[key] = struct{}{}
		kinds = append(kinds, kind)
		ids = append(ids, id)
		names = append(names, name)
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO resource_display_names (tenant_id, resource_kind, resource_id, display_name)
		 SELECT $1, k, i, n FROM unnest($2::text[], $3::text[], $4::text[]) AS t(k, i, n)
		 ON CONFLICT (tenant_id, resource_kind, resource_id)
		 DO UPDATE SET display_name = EXCLUDED.display_name, updated_at = now()
		 WHERE resource_display_names.display_name <> EXCLUDED.display_name`,
		tenantID, kinds, ids, names)
	if err != nil {
		return classify("resource display name", err)
	}
	return nil
}

// ResourceDisplayNames resolves retained names for one tenant, keyed by
// ResourceDisplayNameKey. Missing ids are simply absent from the map — a
// resource bex never recorded a name for has none to retain.
func (s *PGStore) ResourceDisplayNames(ctx context.Context, tenantID string, refs []ResourceDisplayName) (map[string]string, error) {
	if tenantID == "" || len(refs) == 0 {
		return nil, nil
	}
	kinds := make([]string, 0, len(refs))
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Kind == "" || r.ID == "" {
			continue
		}
		kinds = append(kinds, r.Kind)
		ids = append(ids, r.ID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT resource_kind, resource_id, display_name
		 FROM resource_display_names
		 WHERE tenant_id = $1
		   AND (resource_kind, resource_id) IN (
		       SELECT k, i FROM unnest($2::text[], $3::text[]) AS t(k, i))`,
		tenantID, kinds, ids)
	if err != nil {
		return nil, classify("resource display name", err)
	}
	defer rows.Close()
	out := make(map[string]string, len(ids))
	for rows.Next() {
		var kind, id, name string
		if err := rows.Scan(&kind, &id, &name); err != nil {
			return nil, classify("resource display name", err)
		}
		out[ResourceDisplayNameKey(kind, id)] = name
	}
	if err := rows.Err(); err != nil {
		return nil, classify("resource display name", err)
	}
	return out, nil
}

// SandboxLabels derives a human label for each sandbox id from the agent
// session that owns (or owned) it: "<repo>" or "<repo> (<branch>)".
//
// A sandbox has no name of its own, and it is reaped long before its charges
// stop mattering — but the session outlives it (sessions are archived, never
// dropped), and a rehydrated session records the sandbox it came from on its
// dispatch. So this resolves labels for sandbox UUIDs that no live `sandboxes`
// query lists any more, which was the whole reason those rows billed as bare
// UUIDs.
func (s *PGStore) SandboxLabels(ctx context.Context, tenantID string, sandboxIDs []string) (map[string]string, error) {
	if tenantID == "" || len(sandboxIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT DISTINCT ON (sandbox) sandbox, label FROM (
		     SELECT s.sandbox_id AS sandbox,
		            CASE WHEN s.branch <> '' THEN s.repo || ' (' || s.branch || ')' ELSE s.repo END AS label,
		            s.updated_at
		     FROM agent_sessions s
		     WHERE s.workspace_id = $1 AND s.repo <> '' AND s.sandbox_id = ANY($2::text[])
		     UNION ALL
		     SELECT d.previous_sandbox_id AS sandbox,
		            CASE WHEN s.branch <> '' THEN s.repo || ' (' || s.branch || ')' ELSE s.repo END AS label,
		            s.updated_at
		     FROM agent_session_dispatches d
		     JOIN agent_sessions s ON s.id = d.session_id
		     WHERE s.workspace_id = $1 AND s.repo <> '' AND d.previous_sandbox_id = ANY($2::text[])
		 ) AS labelled
		 ORDER BY sandbox, updated_at DESC`,
		tenantID, sandboxIDs)
	if err != nil {
		return nil, classify("sandbox label", err)
	}
	defer rows.Close()
	out := make(map[string]string, len(sandboxIDs))
	for rows.Next() {
		var id, label string
		if err := rows.Scan(&id, &label); err != nil {
			return nil, classify("sandbox label", err)
		}
		out[id] = label
	}
	if err := rows.Err(); err != nil {
		return nil, classify("sandbox label", err)
	}
	return out, nil
}
