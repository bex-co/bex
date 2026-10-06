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
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WithInitialEnvGroups composes trusted group references before a new App is
// observable. It is an internal Blueprint creation seam, not a public input for
// Kubernetes Secret names. The caller compensates its App/store writes on error.
func (s *Service) WithInitialEnvGroups(ctx context.Context, names []string, a *appv1alpha1.App, create, complete func() error) (err error) {
	if err := s.AuthorizeAppFresh(ctx, core.RelCanCreate, a); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	if _, ok := s.Store.(core.VersionedSecretKV); !ok {
		return core.ErrSecretsUnavailable
	}
	service := core.AppPublicID(a)
	type initialGroup struct {
		gid   string
		meta  meta
		added bool
	}
	groups := make([]initialGroup, 0, len(names))
	for _, name := range names {
		gid, m, found, findErr := s.findGroupByName(ctx, name)
		if findErr != nil {
			return findErr
		}
		if !found {
			return fmt.Errorf("%w: env group %q does not exist", core.ErrBadRequest, name)
		}
		// Reuse the normal group's scoped authorization, with a fresh sensitive
		// check before even an already-linked membership can be accepted.
		m, findErr = s.fetchGroup(ctx, core.RelCanCreate, gid)
		if findErr != nil {
			return findErr
		}
		if findErr = s.validateInitialGroup(ctx, m, service, a); findErr != nil {
			return findErr
		}
		groups = append(groups, initialGroup{gid: gid, meta: m})
	}
	defer func() {
		if err == nil {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		for i := len(groups) - 1; i >= 0; i-- {
			group := groups[i]
			if !group.added {
				continue
			}
			rollbackErr := s.dropLinks(rollbackCtx, group.gid, group.meta.workspace, service)
			if rollbackErr != nil && !errors.Is(rollbackErr, core.ErrNotFound) {
				err = errors.Join(err, fmt.Errorf("rollback initial group membership: %w", rollbackErr))
			}
		}
	}()
	for i := range groups {
		group := &groups[i]
		addedThisAttempt := false
		var committed meta
		committed, err = s.mutateMetaCAS(ctx, group.gid, group.meta.workspace, func(cur meta) (meta, error) {
			if checkErr := s.validateInitialGroup(ctx, cur, service, a); checkErr != nil {
				return meta{}, checkErr
			}
			addedThisAttempt = !slices.Contains(cur.links, service)
			cur.linksByID = cur.linksByID || len(cur.links) == 0
			cur.links = addString(cur.links, service)
			cur.updatedAt = s.now()
			return cur, nil
		})
		// A conflicting/failed PutCAS owns nothing. A locator failure still
		// returns its committed snapshot and must compensate that addition.
		group.added = addedThisAttempt && slices.Contains(committed.links, service)
		if err != nil {
			return err
		}
		a.Spec.EnvFromSecrets = addString(a.Spec.EnvFromSecrets, envSecretName(group.gid))
		a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, filesSecretName(group.gid))
	}
	if err = create(); err != nil {
		return err
	}
	if err = s.AuthorizeAppFresh(ctx, core.RelCanCreate, a); err != nil {
		return err
	}
	for _, group := range groups {
		// A deletion/scope change while creating must not turn a prepared link
		// into success. Recommit also restores a membership pruned while no App
		// existed yet, preserving unrelated concurrent links through CAS.
		_, err = s.mutateMetaCAS(ctx, group.gid, group.meta.workspace, func(cur meta) (meta, error) {
			if checkErr := s.validateInitialGroup(ctx, cur, service, a); checkErr != nil {
				return meta{}, checkErr
			}
			cur.linksByID = cur.linksByID || len(cur.links) == 0
			cur.links = addString(cur.links, service)
			return cur, nil
		})
		if err != nil {
			return err
		}
	}
	if complete != nil {
		if err = complete(); err != nil {
			return err
		}
	}
	for _, group := range groups {
		if group.added {
			s.RecordAppConfigChanged(ctx, a, core.AuditVerbLinkService)
		}
	}
	return nil
}

func (s *Service) validateInitialGroup(ctx context.Context, m meta, service string, a *appv1alpha1.App) error {
	if a.Labels[core.LabelTenant] != m.workspace {
		return core.ErrForbidden
	}
	if err := s.authorizeGroupSensitiveFresh(ctx, m); err != nil {
		return err
	}
	return validateGroupServiceEnvironment(m.environment, service, a.Labels)
}
