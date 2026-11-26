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
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WithInitialEnvGroups composes trusted group references before a new App is
// observable. It is an internal Blueprint creation seam, not a public input for
// Kubernetes Secret names: a render.yaml names its groups. The caller
// compensates its App/store writes on error.
func (s *Service) WithInitialEnvGroups(ctx context.Context, names []string, a *appv1alpha1.App, create, complete func() error) error {
	return s.withInitialGroups(ctx, names, s.initialGroupByName, a, create, complete)
}

// WithInitialEnvGroupIDs is WithInitialEnvGroups for groups named by id, as
// the dashboard's create form names them (GraphQL createService envGroups,
// w4/731; Render's create form sends its Linked Environment Groups' ids).
func (s *Service) WithInitialEnvGroupIDs(ctx context.Context, ids []string, a *appv1alpha1.App, create, complete func() error) error {
	return s.withInitialGroups(ctx, ids, s.initialGroupByID, a, create, complete)
}

// CheckInitialEnvGroupIDs is WithInitialEnvGroupIDs' read-only half, for a
// create's plan: every refusal the link meets before its first write, so a
// dry-run never answers "would succeed" for groups the create refuses.
func (s *Service) CheckInitialEnvGroupIDs(ctx context.Context, ids []string, a *appv1alpha1.App) error {
	_, err := s.resolveInitialGroups(ctx, ids, s.initialGroupByID, a)
	return err
}

// initialGroupLookup finds one initial group in the acting workspace, or
// answers why it cannot be linked.
type initialGroupLookup func(ctx context.Context, ref string, a *appv1alpha1.App) (store.EnvGroup, error)

// initialGroupByName is a Blueprint's lookup: the name in the acting
// workspace, then the group's own scoped authorization.
func (s *Service) initialGroupByName(ctx context.Context, name string, _ *appv1alpha1.App) (store.EnvGroup, error) {
	g, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return store.EnvGroup{}, err
	}
	if !found {
		return store.EnvGroup{}, fmt.Errorf("%w: env group %q does not exist", core.ErrBadRequest, name)
	}
	return s.fetchGroup(ctx, core.RelCanCreate, g.ID)
}

// initialGroupByID is the create form's lookup. A group of another workspace
// reads as one that does not exist, before any authorization, so the answer
// says nothing about another workspace's ids.
func (s *Service) initialGroupByID(ctx context.Context, gid string, a *appv1alpha1.App) (store.EnvGroup, error) {
	missing := fmt.Errorf("%w: env group %q does not exist", core.ErrBadRequest, gid)
	if !id.WellFormed(gid) {
		return store.EnvGroup{}, missing
	}
	g, err := s.readGroup(ctx, gid)
	if errors.Is(err, core.ErrNotFound) || err == nil && g.TenantID != a.Labels[core.LabelTenant] {
		return store.EnvGroup{}, missing
	}
	if err != nil {
		return store.EnvGroup{}, err
	}
	// fetchGroup's authorization, on the row already read.
	if err := s.AuthorizeLabeled(ctx, core.RelCanCreate, map[string]string{core.LabelTenant: g.TenantID}); err != nil {
		return store.EnvGroup{}, err
	}
	return g, nil
}

// initialGroup is one group a new App links as it is written.
type initialGroup struct {
	gid, name string
	added     bool
}

// resolveInitialGroups authorizes the new App's create and finds, authorizes
// and checks every group it links, writing nothing. A group named twice is
// linked once.
func (s *Service) resolveInitialGroups(ctx context.Context, refs []string, lookup initialGroupLookup, a *appv1alpha1.App) ([]initialGroup, error) {
	if err := s.AuthorizeAppFresh(ctx, core.RelCanCreate, a); err != nil {
		return nil, err
	}
	if err := s.requireGroups(); err != nil {
		return nil, err
	}
	service := core.AppPublicID(a)
	groups := make([]initialGroup, 0, len(refs))
	for _, ref := range refs {
		// Reuse the normal group's scoped authorization, with a fresh sensitive
		// check before even an already-linked membership can be accepted.
		g, err := lookup(ctx, ref, a)
		if err != nil {
			return nil, err
		}
		if err := s.validateInitialGroup(ctx, g, service, a); err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(groups, func(have initialGroup) bool { return have.gid == g.ID }) {
			groups = append(groups, initialGroup{gid: g.ID, name: g.Name})
		}
	}
	return groups, nil
}

func (s *Service) withInitialGroups(ctx context.Context, refs []string, lookup initialGroupLookup, a *appv1alpha1.App, create, complete func() error) (err error) {
	groups, err := s.resolveInitialGroups(ctx, refs, lookup, a)
	if err != nil {
		return err
	}
	service := core.AppPublicID(a)
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
			cur.ServiceLinks = core.AddString(cur.ServiceLinks, service)
			cur.UpdatedAt = s.nowTime()
			return nil
		})
		if err != nil {
			return err
		}
		group.added = addedThisAttempt
		a.Spec.EnvFromSecrets = rollout.InsertGroupRef(a.Spec.EnvFromSecrets, envSecretName(group.gid))
		a.Spec.FilesFromSecrets = rollout.InsertGroupRef(a.Spec.FilesFromSecrets, filesSecretName(group.gid))
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
			cur.ServiceLinks = core.AddString(cur.ServiceLinks, service)
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
			s.RecordEnvGroupLinkChanged(ctx, a, core.AuditVerbLinkService, group.gid, group.name)
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
	serviceEnv, err := s.serviceEnvironment(ctx, a)
	if err != nil {
		return err
	}
	return s.withGroupContent(ctx, g.ID, func(_ context.Context, fresh store.EnvGroup) error {
		return validateGroupServiceEnvironment(fresh.EnvironmentID, service, serviceEnv)
	})
}
