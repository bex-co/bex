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
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A group's link set names each linked service by its public srv- id
// (core.AppPublicID). Ids are never reused, so a link cannot pass to another
// service the way a name passes to the next service created with it. A
// hand-applied App has no id and is keyed by its object name.

// isServiceID reports whether a link entry is a service id.
func isServiceID(entry string) bool {
	kind, ok := id.KindOf(entry)
	return ok && kind == id.Service
}

// mountsGroup reports whether a's spec references group gid's Secrets: the
// refs a link adds and an unlink removes. Only a service that mounts a group
// is acted on as linked to it.
func mountsGroup(a *appv1alpha1.App, gid string) bool {
	env, files := rollout.EffectiveGroupSecrets(a)
	return slices.Contains(env, envSecretName(gid)) || slices.Contains(files, filesSecretName(gid))
}

// mountedGroups returns the ids of the groups a's spec references.
func mountedGroups(a *appv1alpha1.App) []string {
	var gids []string
	env, files := rollout.EffectiveGroupSecrets(a)
	for _, ref := range slices.Concat(env, files) {
		for _, suffix := range []string{envSecretName(""), filesSecretName("")} {
			gid, ok := strings.CutSuffix(ref, suffix)
			if kind, group := id.KindOf(gid); ok && group && kind == id.EnvGroup {
				gids = addString(gids, gid)
			}
		}
	}
	return gids
}

// dropLinks removes entries from group gid's link set, writing nothing when
// none of them is there.
func (s *Service) dropLinks(ctx context.Context, gid string, entries ...string) error {
	_, err := s.mutateGroup(ctx, gid, func(cur *store.EnvGroup) error {
		links := removeString(cur.ServiceLinks, entries...)
		if len(links) == len(cur.ServiceLinks) {
			return errMetaUnchanged
		}
		cur.ServiceLinks = links
		cur.UpdatedAt = s.nowTime()
		return nil
	})
	return err
}

// linkCandidates lists the Apps a group of workspace may link: the
// workspace's, by label wherever the namespace cutover (ADR043) left them.
func (s *Service) linkCandidates(ctx context.Context, workspace string) ([]appv1alpha1.App, error) {
	var apps appv1alpha1.AppList
	err := s.ListByTenant(ctx, &apps, workspace)
	return apps.Items, err
}
