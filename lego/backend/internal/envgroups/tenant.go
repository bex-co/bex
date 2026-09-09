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

package envgroups

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// tenant.go moves env-group OpenBao storage off the shared legacy tenant
// ("default") onto workspace-prefixed tenants (w2/m80), reusing the store's
// existing withTenant seam (w7/m70, exported as secrets.WithTenant) rather than
// inventing a second scoping mechanism. A group's meta always keeps a thin
// "locator" pointer at the legacy tenant (writeMetaLocator) so a caller who
// knows only its id — GetEnvGroup, the SSH/Blueprint seams — can still find its
// workspace-prefixed home without a global cross-tenant scan. During the
// dual-read window an unmigrated group (created before this milestone, or not
// yet visited by MigratePaths) still has its full content at the legacy
// tenant; every read here falls back there, and every write lands only on the
// new prefix, so ordinary traffic converges a group toward its migrated
// location without an explicit step (MigratePaths, migrate.go, handles the
// remaining idle groups explicitly).

// groupCtx scopes ctx to workspace's own OpenBao tenant. Empty workspace
// normalizes to the legacy shared tenant (secrets.LegacyTenant) — store-off
// mode and any group never attributed to a real workspace keep their
// byte-identical pre-w2/m80 location.
func groupCtx(ctx context.Context, workspace string) context.Context {
	return secrets.WithTenant(ctx, workspace)
}

// legacyCtx scopes ctx to the pre-w2/m80 shared tenant: the migration source
// and the dual-read fallback for a group not yet migrated. groupCtx(ctx, "")
// and legacyCtx(ctx) address the exact same OpenBao path.
func legacyCtx(ctx context.Context) context.Context {
	return secrets.WithTenant(ctx, secrets.LegacyTenant)
}

// isLocator reports whether a legacy-tenant meta read is a thin pointer to the
// group's real workspace-prefixed home rather than the group's full metadata.
// writeMetaLocator sets "locator"; MigratePaths additionally sets "tombstoned"
// on the legacy meta it replaces once a group's full content has moved — both
// mean "read the workspace tenant instead."
func isLocator(raw map[string]string) bool {
	return raw["locator"] == "1" || raw["tombstoned"] == "1"
}

// getGroupMap reads path with dual-read semantics: the workspace tenant first
// (a migrated group's authoritative home), falling back to the legacy shared
// tenant (an unmigrated group's only home, or the byte-identical location for
// workspace ""). The fallback is cheap once a store is fully migrated — the
// workspace-tenant Get already returned data and no second call happens.
func (s *Service) getGroupMap(ctx context.Context, workspace, path string) (map[string]string, error) {
	if workspace != "" {
		m, err := s.Store.Get(groupCtx(ctx, workspace), path)
		if err != nil {
			return nil, err
		}
		if len(m) > 0 {
			return m, nil
		}
	}
	return s.Store.Get(legacyCtx(ctx), path)
}

// putGroupMap writes path under the workspace tenant only — every write lands
// on the new layout, never the legacy shared tenant, so ordinary traffic
// converges a group toward its migrated location without an explicit step.
func (s *Service) putGroupMap(ctx context.Context, workspace, path string, data map[string]string) error {
	return s.Store.Put(groupCtx(ctx, workspace), path, data)
}

// deleteGroupPath removes path under the workspace tenant only. A legacy-
// tenant leftover (an unmigrated or already-tombstoned group) is cleaned up
// explicitly by deleteGroupArtifacts / MigratePaths, never implicitly here.
func (s *Service) deleteGroupPath(ctx context.Context, workspace, path string) error {
	return s.Store.Delete(groupCtx(ctx, workspace), path)
}

// storeMap writes data to the source of truth under workspace's tenant,
// deleting the path when data is empty.
func (s *Service) storeMap(ctx context.Context, workspace, path string, data map[string]string) error {
	if len(data) == 0 {
		return s.deleteGroupPath(ctx, workspace, path)
	}
	return s.putGroupMap(ctx, workspace, path, data)
}

// listGroupIDs lists group ids under workspace's own tenant. During the
// dual-read migration window an unmigrated group's id is visible only under
// the legacy tenant, so includeLegacyUnion additionally lists there and
// returns the deduplicated union of ids whose LEGACY entry is still full
// content — never a locator/tombstone. That filter matters for isolation, not
// just cost: every migrated group (in ANY workspace) leaves a permanent
// legacy-tenant locator (writeMetaLocator, so a bare-gid lookup keeps
// working), so an unfiltered union would hand every scoped caller every OTHER
// workspace's migrated group ids too — and resolving each one's real content
// in the caller's later readMeta would require a Get against that OTHER
// workspace's own tenant, a genuine cross-tenant read. Recognizing and
// dropping a locator costs only a legacy-tenant Get (the shared tenant every
// caller may already touch), never the foreign workspace's. workspace == ""
// (store off / unscoped) lists the legacy tenant alone unconditionally, since
// that is its only possible home.
func (s *Service) listGroupIDs(ctx context.Context, workspace string, includeLegacyUnion bool) ([]string, error) {
	if workspace == "" {
		return s.Store.List(legacyCtx(ctx), "env-groups")
	}
	ids, err := s.Store.List(groupCtx(ctx, workspace), "env-groups")
	if err != nil {
		return nil, err
	}
	if !includeLegacyUnion {
		return ids, nil
	}
	legacyIDs, err := s.Store.List(legacyCtx(ctx), "env-groups")
	if err != nil {
		return nil, err
	}
	if len(legacyIDs) == 0 {
		return ids, nil
	}
	seen := make(map[string]struct{}, len(ids))
	for _, gid := range ids {
		seen[gid] = struct{}{}
	}
	out := append([]string{}, ids...)
	for _, gid := range legacyIDs {
		if _, ok := seen[gid]; ok {
			continue
		}
		raw, err := s.Store.Get(legacyCtx(ctx), metaPath(gid))
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 || isLocator(raw) {
			continue // already migrated elsewhere — its own tenant's List covers it
		}
		seen[gid] = struct{}{}
		out = append(out, gid)
	}
	return out, nil
}

// writeMetaLocator publishes a thin pointer at the legacy tenant so a caller
// who knows only a group's id — never its workspace — can still find its
// workspace-prefixed home. Every workspace-prefixed group has exactly one of
// these; a store-off/legacy-tenant group needs none (it already lives at the
// location this would point to).
func (s *Service) writeMetaLocator(ctx context.Context, gid, workspace string) error {
	return s.Store.Put(legacyCtx(ctx), metaPath(gid), map[string]string{
		"workspace": workspace,
		"locator":   "1",
	})
}

// deleteGroupArtifacts removes every store path a group may occupy: its
// workspace-prefixed home (meta/env/files/revision) and any legacy-tenant
// leftover (an unmigrated full copy, or the locator writeMeta left behind) —
// correct regardless of whether the group has been migrated yet.
func (s *Service) deleteGroupArtifacts(ctx context.Context, workspace, gid string) error {
	_ = s.clearOpArtifacts(ctx, workspace, gid, "")
	for _, path := range []string{metaPath(gid), envPath(gid), filesPath(gid), revisionPath(gid), opRecordPath(gid)} {
		if err := s.deleteGroupPath(ctx, workspace, path); err != nil {
			return err
		}
		if err := s.Store.Delete(legacyCtx(ctx), path); err != nil {
			return err
		}
	}
	return nil
}

// getRevisionSnapshot dual-reads the CAS revision lock: the workspace tenant
// first, falling back to the legacy tenant's data (state/generation) for an
// unmigrated group. The returned Version is always the WORKSPACE tenant's own
// (zero when nothing has been written there yet), so a caller's PutCAS lands
// as a create at the new location while inheriting the correct starting
// generation from wherever the group's revision counter previously lived —
// the same "writes never touch legacy" rule putGroupMap follows.
func (s *Service) getRevisionSnapshot(ctx context.Context, versioned core.VersionedSecretKV, workspace, gid string) (core.SecretKVSnapshot, error) {
	path := revisionPath(gid)
	own, err := versioned.GetVersioned(groupCtx(ctx, workspace), path)
	if err != nil {
		return core.SecretKVSnapshot{}, err
	}
	if own.Version != 0 || workspace == "" {
		return own, nil
	}
	legacy, err := versioned.GetVersioned(legacyCtx(ctx), path)
	if err != nil {
		return core.SecretKVSnapshot{}, err
	}
	if legacy.Version == 0 {
		return own, nil
	}
	return core.SecretKVSnapshot{Data: legacy.Data, Version: own.Version}, nil
}
