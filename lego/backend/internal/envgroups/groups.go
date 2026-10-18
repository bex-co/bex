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
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// GroupStore is the control-plane persistence of environment groups
// (w8/m114): every non-secret fact about a group, and the coordination state of
// its content saves. store.PGStore implements it. The env and files maps stay
// in Service.Store (OpenBao).
type GroupStore interface {
	WithEnvGroupContentLock(ctx context.Context, id string, fn func(context.Context) error) error
	InsertEnvGroup(ctx context.Context, g store.EnvGroup) (store.EnvGroup, error)
	GetEnvGroup(ctx context.Context, id string) (store.EnvGroup, error)
	GetEnvGroupByName(ctx context.Context, tenantID, name string) (store.EnvGroup, error)
	ListEnvGroups(ctx context.Context, tenantID string) ([]store.EnvGroup, error)
	CountEnvGroups(ctx context.Context, tenantID string) (int, error)
	EnvGroupNameTaken(ctx context.Context, tenantID, name string) (bool, error)
	UpdateEnvGroupMeta(ctx context.Context, g store.EnvGroup) (store.EnvGroup, error)
	DeleteEnvGroup(ctx context.Context, id string) (store.EnvGroup, error)
	ClaimEnvGroupContent(ctx context.Context, id string, expectedVersion int64, op store.EnvGroupOp) (int64, error)
	AdvanceEnvGroupOp(ctx context.Context, id, opID, phase string, leaseUntil time.Time) error
	ReleaseEnvGroupContent(ctx context.Context, id string, expectedVersion int64, rel store.EnvGroupRelease) error
}

// metaCASRetries bounds how often a metadata write re-reads after losing a
// compare-and-set to a concurrent writer.
const metaCASRetries = 3

// errMetaUnchanged lets a mutateGroup callback say the current row already
// satisfies the request, so nothing is written.
var errMetaUnchanged = errors.New("env group metadata unchanged")

func envGroupMetadataConflict() error {
	return core.NewConflictError(
		"ENV_GROUP_METADATA_CONFLICT",
		"the environment group changed; refresh it before retrying",
		nil,
	)
}

func envGroupNameExists(name, workspace string) error {
	return core.NewConflictError(
		"ENV_GROUP_NAME_EXISTS",
		"Environment group name already exists.",
		map[string]any{"name": name, "workspaceId": workspace},
	)
}

// requireGroups fails closed while either store is unwired.
func (s *Service) requireGroups() error {
	if s.Groups == nil || s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	return nil
}

// groupErr maps a metadata store error onto the API's sentinels.
func groupErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return core.ErrNotFound
	case errors.Is(err, store.ErrEnvGroupChanged):
		return envGroupMetadataConflict()
	}
	return err
}

// readGroup returns one group's row, or core.ErrNotFound.
func (s *Service) readGroup(ctx context.Context, gid string) (store.EnvGroup, error) {
	if err := s.requireGroups(); err != nil {
		return store.EnvGroup{}, err
	}
	g, err := s.Groups.GetEnvGroup(ctx, gid)
	return g, groupErr(err)
}

// mutateGroup re-reads the group, applies mutate to its metadata, and writes
// it back only if no other writer changed the row in between, re-reading after
// a lost race. mutate may return errMetaUnchanged (nothing to write) or a
// refusal. A missing group is core.ErrNotFound; a taken name is
// ENV_GROUP_NAME_EXISTS.
func (s *Service) mutateGroup(ctx context.Context, gid string, mutate func(cur *store.EnvGroup) error) (store.EnvGroup, error) {
	if err := s.requireGroups(); err != nil {
		return store.EnvGroup{}, err
	}
	for range metaCASRetries + 1 {
		cur, err := s.Groups.GetEnvGroup(ctx, gid)
		if err != nil {
			return store.EnvGroup{}, groupErr(err)
		}
		next := cur
		next.ServiceLinks = slices.Clone(cur.ServiceLinks)
		if err := mutate(&next); errors.Is(err, errMetaUnchanged) {
			return cur, nil
		} else if err != nil {
			return store.EnvGroup{}, err
		}
		updated, err := s.Groups.UpdateEnvGroupMeta(ctx, next)
		switch {
		case errors.Is(err, store.ErrEnvGroupChanged):
			continue
		case errors.Is(err, store.ErrEnvGroupNameTaken):
			return store.EnvGroup{}, envGroupNameExists(next.Name, next.TenantID)
		case err != nil:
			return store.EnvGroup{}, groupErr(err)
		}
		return updated, nil
	}
	return store.EnvGroup{}, envGroupMetadataConflict()
}

// timestamp renders a stored time the way every surface has shown group
// timestamps.
func timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// nowTime is the clock reading a metadata write records.
func (s *Service) nowTime() time.Time { return s.Now().UTC() }
