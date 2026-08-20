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
	"errors"
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// migrate.go is the explicit, opt-in one-time move (w2/m80) of every group
// still fully resident at the shared legacy OpenBao tenant onto its own
// workspace-prefixed tenant — the same move ordinary read/write traffic
// converges toward lazily (tenant.go), just performed eagerly and reported so
// an operator can drain the legacy tenant on a schedule rather than waiting
// for every group to be touched. It is deliberately NOT run automatically:
// see docs/runbooks/env-group-path-migration.md for the auth gate, dry-run/
// apply sequence, verification, and rollback.

// MigrationReport summarizes one MigratePaths pass (dry-run or apply): how
// many legacy-tenant entries were examined, how many groups were moved (or,
// in dry-run, would be), how many were already migrated (idempotent no-op —
// a locator/tombstone, or a prior partial run's workspace copy that only
// needed its legacy tombstone finished), and how many had no resolvable
// workspace to move to (a store-off-era group; left alone, exactly as
// readMeta's own lazy DefaultTenant attribution would leave it until read).
type MigrationReport struct {
	Scanned            int      `json:"scanned"`
	Migrated           int      `json:"migrated"`
	AlreadyMigrated    int      `json:"alreadyMigrated"`
	SkippedNoWorkspace int      `json:"skippedNoWorkspace"`
	Failed             []string `json:"failed,omitempty"`
}

// MigratePaths walks every group at the legacy shared OpenBao tenant and moves
// its meta/env/files/revision to its own workspace-prefixed tenant, leaving a
// thin locator at the legacy tenant so a bare-gid lookup (GetEnvGroup, the
// SSH/Blueprint seams) still finds it. dryRun reports the exact work without
// writing. Idempotent: a re-run finds only locators (Scanned excludes them
// entirely — AlreadyMigrated only counts a full legacy meta this pass itself
// resolved) and does no further work for a group already moved.
func MigratePaths(ctx context.Context, store core.SecretKV, dryRun bool) (MigrationReport, error) {
	var report MigrationReport
	if store == nil {
		return report, core.ErrSecretsUnavailable
	}
	versioned, ok := store.(core.VersionedSecretKV)
	if !ok {
		return report, core.ErrSecretsUnavailable
	}
	ids, err := store.List(legacyCtx(ctx), "env-groups")
	if err != nil {
		return report, err
	}
	for _, gid := range ids {
		raw, err := store.Get(legacyCtx(ctx), metaPath(gid))
		if err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %v", gid, err))
			continue
		}
		if len(raw) == 0 {
			continue // create publishes metadata last; ignore an in-flight id
		}
		if isLocator(raw) {
			continue // already migrated by an earlier pass; nothing to scan
		}
		report.Scanned++
		workspace := raw["workspace"]
		if workspace == "" || workspace == secrets.LegacyTenant {
			// No real workspace to move to (a store-off-era group, or one
			// lazily attributed to core.DefaultTenant — which IS the legacy
			// tenant, so it already lives at its only correct home).
			report.SkippedNoWorkspace++
			continue
		}
		migrated, err := migrateOneGroup(ctx, store, versioned, gid, workspace, raw, dryRun)
		if err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %v", gid, err))
			continue
		}
		if migrated {
			report.Migrated++
		} else {
			report.AlreadyMigrated++
		}
	}
	return report, nil
}

// migrateOneGroup moves one group's content to workspace's tenant. It returns
// migrated=false when the workspace tenant already held this group's full
// meta (a previous pass copied it but crashed before tombstoning the legacy
// entry) — that case only finishes the tombstone, never re-copying content,
// so a partially-applied prior run is completed rather than duplicated.
func migrateOneGroup(
	ctx context.Context,
	store core.SecretKV,
	versioned core.VersionedSecretKV,
	gid, workspace string,
	legacyMeta map[string]string,
	dryRun bool,
) (migrated bool, err error) {
	existing, err := store.Get(groupCtx(ctx, workspace), metaPath(gid))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		if dryRun {
			return false, nil
		}
		return false, tombstoneLegacy(ctx, store, gid, workspace)
	}
	if dryRun {
		return true, nil
	}
	env, err := store.Get(legacyCtx(ctx), envPath(gid))
	if err != nil {
		return false, err
	}
	files, err := store.Get(legacyCtx(ctx), filesPath(gid))
	if err != nil {
		return false, err
	}
	revision, err := versioned.GetVersioned(legacyCtx(ctx), revisionPath(gid))
	if err != nil {
		return false, err
	}
	if len(env) > 0 {
		if err := store.Put(groupCtx(ctx, workspace), envPath(gid), env); err != nil {
			return false, err
		}
	}
	if len(files) > 0 {
		if err := store.Put(groupCtx(ctx, workspace), filesPath(gid), files); err != nil {
			return false, err
		}
	}
	if len(revision.Data) > 0 {
		if _, err := versioned.PutCAS(groupCtx(ctx, workspace), revisionPath(gid), revision.Data, 0); err != nil && !errors.Is(err, core.ErrConflict) {
			return false, err
		}
	}
	if err := store.Put(groupCtx(ctx, workspace), metaPath(gid), legacyMeta); err != nil {
		return false, err
	}
	// Verify before touching the legacy copy — never tombstone a source we
	// have not confirmed was actually written.
	verify, err := store.Get(groupCtx(ctx, workspace), metaPath(gid))
	if err != nil {
		return false, err
	}
	if verify["name"] != legacyMeta["name"] || verify["workspace"] != legacyMeta["workspace"] {
		return false, fmt.Errorf("verification mismatch after copying group %s to workspace %s", gid, workspace)
	}
	if err := tombstoneLegacy(ctx, store, gid, workspace); err != nil {
		return false, err
	}
	return true, nil
}

// tombstoneLegacy replaces the legacy tenant's full meta with a thin locator
// (writeMetaLocator's exact shape, plus "tombstoned" recording that this
// specific entry is migration leftover rather than a freshly-created group's
// locator) and deletes the legacy env/files/revision copies now superseded by
// the workspace tenant's.
func tombstoneLegacy(ctx context.Context, store core.SecretKV, gid, workspace string) error {
	if err := store.Put(legacyCtx(ctx), metaPath(gid), map[string]string{
		"workspace": workspace, "locator": "1", "tombstoned": "1",
	}); err != nil {
		return err
	}
	if err := store.Delete(legacyCtx(ctx), envPath(gid)); err != nil {
		return err
	}
	if err := store.Delete(legacyCtx(ctx), filesPath(gid)); err != nil {
		return err
	}
	return store.Delete(legacyCtx(ctx), revisionPath(gid))
}
