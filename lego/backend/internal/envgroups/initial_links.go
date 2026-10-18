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
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WithInitialEnvGroups composes trusted group references before a new App is
// observable. It is an internal Blueprint creation seam, not a public input for
// Kubernetes Secret names. The caller compensates its App/store writes on error.
func (s *Service) WithInitialEnvGroups(ctx context.Context, names []string, a *appv1alpha1.App, create, complete func() error) (err error) {
	if err := s.AuthorizeAppFresh(ctx, core.RelCanCreate, a); err != nil {
		return err
	}
	if err := s.requireGroups(ctx); err != nil {
		return err
	}
	service := core.AppPublicID(a)
	type initialGroup struct {
		gid   string
		added bool
	}
	groups := make([]initialGroup, 0, len(names))
	for _, name := range names {
		g, found, findErr := s.findGroupByName(ctx, name)
		if findErr != nil {
			return findErr
		}
		if !found {
			return fmt.Errorf("%w: env group %q does not exist", core.ErrBadRequest, name)
		}
		// Reuse the normal group's scoped authorization, with a fresh sensitive
		// check before even an already-linked membership can be accepted.
		g, findErr = s.fetchGroup(ctx, core.RelCanCreate, g.ID)
		if findErr != nil {
			return findErr
		}
		if findErr = s.validateInitialGroup(ctx, g, service, a); findErr != nil {
			return findErr
		}
		groups = append(groups, initialGroup{gid: g.ID})
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
			rollbackErr := s.dropLinks(rollbackCtx, group.gid, service)
			if rollbackErr != nil && !errors.Is(rollbackErr, core.ErrNotFound) {
				err = errors.Join(err, fmt.Errorf("rollback initial group membership: %w", rollbackErr))
			}
		}
	}()
	for i := range groups {
		group := &groups[i]
		addedThisAttempt := false
		_, err = s.mutateGroup(ctx, group.gid, func(cur *store.EnvGroup) error {
			if checkErr := s.validateInitialGroup(ctx, *cur, service, a); checkErr != nil {
				return checkErr
			}
			if slices.Contains(cur.ServiceLinks, service) {
				addedThisAttempt = false
				return errMetaUnchanged
			}
			addedThisAttempt = true
			cur.ServiceLinks = addString(cur.ServiceLinks, service)
			cur.UpdatedAt = s.nowTime()
			return nil
		})
		if err != nil {
			return err
		}
		group.added = addedThisAttempt
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
		// existed yet, preserving unrelated concurrent links.
		_, err = s.mutateGroup(ctx, group.gid, func(cur *store.EnvGroup) error {
			if checkErr := s.validateInitialGroup(ctx, *cur, service, a); checkErr != nil {
				return checkErr
			}
			if slices.Contains(cur.ServiceLinks, service) {
				return errMetaUnchanged
			}
			cur.ServiceLinks = addString(cur.ServiceLinks, service)
			return nil
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

func (s *Service) validateInitialGroup(ctx context.Context, g store.EnvGroup, service string, a *appv1alpha1.App) error {
	if a.Labels[core.LabelTenant] != g.TenantID {
		return core.ErrForbidden
	}
	if err := s.authorizeGroupSensitiveFresh(ctx, g); err != nil {
		return err
	}
	return validateGroupServiceEnvironment(g.EnvironmentID, service, a.Labels)
}
