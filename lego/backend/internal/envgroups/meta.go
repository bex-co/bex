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
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// metaCASRetries bounds optimistic metadata retries so a continuously contested
// group surfaces ENV_GROUP_METADATA_CONFLICT instead of spinning.
const metaCASRetries = 3

func envGroupMetadataConflict() error {
	return core.NewConflictError(
		"ENV_GROUP_METADATA_CONFLICT",
		"the environment group changed; refresh it before retrying",
		nil,
	)
}

func encodeMeta(m meta) map[string]string {
	return map[string]string{
		"name":        m.name,
		"links":       strings.Join(m.links, ","),
		"workspace":   m.workspace,
		"environment": m.environment,
		"createdAt":   m.createdAt,
		"updatedAt":   m.updatedAt,
	}
}

// isEditableMeta reports whether raw is a full group metadata map (not a
// locator/tombstone pointer and not an empty post-delete fence).
func isEditableMeta(raw map[string]string) bool {
	return len(raw) > 0 && !isLocator(raw) && raw["name"] != ""
}

// getMetaSnapshot dual-reads group metadata the same way getRevisionSnapshot
// does for the content lock: the workspace tenant is authoritative; an
// unmigrated group's full map may still live only at the legacy tenant, in
// which case Version stays the workspace version (0) so the first PutCAS
// creates the workspace copy. A deleted or never-created group returns an
// empty snapshot (Version 0, no editable data).
func (s *Service) getMetaSnapshot(ctx context.Context, versioned core.VersionedSecretKV, workspace, gid string) (core.SecretKVSnapshot, error) {
	path := metaPath(gid)
	own, err := versioned.GetVersioned(groupCtx(ctx, workspace), path)
	if err != nil {
		return core.SecretKVSnapshot{}, err
	}
	if isEditableMeta(own.Data) {
		return own, nil
	}
	if workspace == "" {
		return own, nil
	}
	legacy, err := versioned.GetVersioned(legacyCtx(ctx), path)
	if err != nil {
		return core.SecretKVSnapshot{}, err
	}
	if isEditableMeta(legacy.Data) {
		return core.SecretKVSnapshot{Data: legacy.Data, Version: own.Version}, nil
	}
	return own, nil
}

// mutateMetaCAS reads the group's current metadata, applies mutate, and writes
// back with PutCAS. Unrelated concurrent field changes survive a mergeable
// retry; mutate may return a conflict when the operation cannot safely merge.
// Absent or deleted groups return ErrNotFound — this never creates metadata.
// Workspace attribution is preserved from the stored map.
func (s *Service) mutateMetaCAS(ctx context.Context, gid, workspace string, mutate func(current meta) (meta, error)) (meta, error) {
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return meta{}, core.ErrSecretsUnavailable
	}
	for attempt := 0; attempt <= metaCASRetries; attempt++ {
		snapshot, err := s.getMetaSnapshot(ctx, versioned, workspace, gid)
		if err != nil {
			return meta{}, err
		}
		if !isEditableMeta(snapshot.Data) {
			return meta{}, core.ErrNotFound
		}
		current := decodeMeta(snapshot.Data)
		// A legacy dual-read may omit workspace; pin the caller's known home so
		// PutCAS lands under the same tenant on every attempt.
		if current.workspace == "" {
			current.workspace = workspace
		}
		if workspace != "" && current.workspace != workspace {
			return meta{}, core.ErrNotFound
		}
		next, err := mutate(current)
		if err != nil {
			return meta{}, err
		}
		next.workspace = current.workspace
		if _, err := versioned.PutCAS(groupCtx(ctx, current.workspace), metaPath(gid), encodeMeta(next), snapshot.Version); err != nil {
			if errors.Is(err, core.ErrConflict) && attempt < metaCASRetries {
				continue
			}
			if errors.Is(err, core.ErrConflict) {
				return meta{}, envGroupMetadataConflict()
			}
			return meta{}, err
		}
		if current.workspace != "" && current.workspace != secrets.LegacyTenant {
			if err := s.writeMetaLocator(ctx, gid, current.workspace); err != nil {
				return meta{}, err
			}
		}
		return next, nil
	}
	return meta{}, envGroupMetadataConflict()
}

// clearMetaCAS fences further metadata mutations by replacing the editable map
// with an empty versioned entry (still Version > 0 after a prior create). A
// delayed mutateMetaCAS then sees !isEditableMeta and returns ErrNotFound
// instead of resurrecting the group. Returns the links captured from the
// cleared snapshot so delete can detach them.
func (s *Service) clearMetaCAS(ctx context.Context, gid, workspace string) (links []string, err error) {
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return nil, core.ErrSecretsUnavailable
	}
	for attempt := 0; attempt <= metaCASRetries; attempt++ {
		snapshot, err := s.getMetaSnapshot(ctx, versioned, workspace, gid)
		if err != nil {
			return nil, err
		}
		if !isEditableMeta(snapshot.Data) {
			return nil, core.ErrNotFound
		}
		current := decodeMeta(snapshot.Data)
		if current.workspace == "" {
			current.workspace = workspace
		}
		if _, err := versioned.PutCAS(groupCtx(ctx, current.workspace), metaPath(gid), map[string]string{}, snapshot.Version); err != nil {
			if errors.Is(err, core.ErrConflict) && attempt < metaCASRetries {
				continue
			}
			if errors.Is(err, core.ErrConflict) {
				return nil, envGroupMetadataConflict()
			}
			return nil, err
		}
		return current.links, nil
	}
	return nil, envGroupMetadataConflict()
}

// writeMetaCreate publishes group metadata with PutCAS expected version 0 on
// the workspace tenant. It refuses to overwrite an already-editable workspace
// copy or a cleared-after-delete fence (Version > 0, empty data). A Version-0
// workspace slot may still dual-read legacy content; writing here is the
// create and lazy-attribution migrate-on-write path.
func (s *Service) writeMetaCreate(ctx context.Context, gid string, m meta) error {
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return s.writeMetaUnversioned(ctx, gid, m)
	}
	own, err := versioned.GetVersioned(groupCtx(ctx, m.workspace), metaPath(gid))
	if err != nil {
		return err
	}
	if isEditableMeta(own.Data) {
		return envGroupMetadataConflict()
	}
	if own.Version != 0 {
		// Cleared-after-delete or tombstone fence: Version > 0 with
		// empty/non-editable data.
		return fmt.Errorf("%w: environment group was deleted", core.ErrNotFound)
	}
	if _, err := versioned.PutCAS(groupCtx(ctx, m.workspace), metaPath(gid), encodeMeta(m), 0); err != nil {
		if errors.Is(err, core.ErrConflict) {
			return envGroupMetadataConflict()
		}
		return err
	}
	if m.workspace != "" && m.workspace != secrets.LegacyTenant {
		return s.writeMetaLocator(ctx, gid, m.workspace)
	}
	return nil
}

func (s *Service) writeMetaUnversioned(ctx context.Context, gid string, m meta) error {
	if err := s.Store.Put(groupCtx(ctx, m.workspace), metaPath(gid), encodeMeta(m)); err != nil {
		return err
	}
	if m.workspace != "" && m.workspace != secrets.LegacyTenant {
		return s.writeMetaLocator(ctx, gid, m.workspace)
	}
	return nil
}
