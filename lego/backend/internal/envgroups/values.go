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

	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A group's values live in OpenBao under its workspace's own tenant
// (secrets.WithTenant): env-groups/<id>/env and env-groups/<id>/files, plus,
// while a content save is in flight, its recovery maps under
// env-groups/<id>/op/<opId>/. Nothing else about a group is in OpenBao.

// groupCtx scopes ctx to workspace's own OpenBao tenant.
func groupCtx(ctx context.Context, workspace string) context.Context {
	return secrets.WithTenant(ctx, workspace)
}

func (s *Service) getGroupMap(ctx context.Context, workspace, path string) (map[string]string, error) {
	return s.Store.Get(groupCtx(ctx, workspace), path)
}

func (s *Service) readGroupContents(ctx context.Context, g store.EnvGroup) (groupContents, error) {
	env, err := s.getGroupMap(ctx, g.TenantID, envPath(g.ID))
	if err != nil {
		return groupContents{}, err
	}
	files, err := s.getGroupMap(ctx, g.TenantID, filesPath(g.ID))
	return groupContents{env: env, files: files}, err
}

func (s *Service) putGroupMap(ctx context.Context, workspace, path string, data map[string]string) error {
	return s.Store.Put(groupCtx(ctx, workspace), path, data)
}

func (s *Service) deleteGroupPath(ctx context.Context, workspace, path string) error {
	return s.Store.Delete(groupCtx(ctx, workspace), path)
}

// storeMap writes data, deleting the path when data is empty.
func (s *Service) storeMap(ctx context.Context, workspace, path string, data map[string]string) error {
	if len(data) == 0 {
		return s.deleteGroupPath(ctx, workspace, path)
	}
	return s.putGroupMap(ctx, workspace, path, data)
}

// contentSlot is one of a group's two maps, with the projection Secret
// linked services read it through.
type contentSlot int

const (
	envSlot contentSlot = iota
	filesSlot
)

func (slot contentSlot) path(gid string) string {
	if slot == envSlot {
		return envPath(gid)
	}
	return filesPath(gid)
}

func (slot contentSlot) secret(gid string) string {
	if slot == envSlot {
		return envSecretName(gid)
	}
	return filesSecretName(gid)
}

// writeSlot stores one of a group's maps and re-materializes its projection.
func (s *Service) writeSlot(ctx context.Context, workspace, gid string, slot contentSlot, data map[string]string) error {
	if err := s.storeMap(ctx, workspace, slot.path(gid), data); err != nil {
		return err
	}
	return s.upsertSecret(ctx, workspace, slot.secret(gid), data)
}

// deleteGroupValues removes every OpenBao path a group occupies: its two maps
// and the recovery maps of any operation, stranded ones included.
func (s *Service) deleteGroupValues(ctx context.Context, workspace, gid string) error {
	errs := []error{
		s.deleteGroupPath(ctx, workspace, envPath(gid)),
		s.deleteGroupPath(ctx, workspace, filesPath(gid)),
	}
	ops, err := s.Store.List(groupCtx(ctx, workspace), opDir(gid))
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, opID := range ops {
		errs = append(errs, s.clearOpMaps(ctx, workspace, gid, opID))
	}
	return errors.Join(errs...)
}
