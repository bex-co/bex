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
	"log"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
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
	return slices.Contains(a.Spec.EnvFromSecrets, envSecretName(gid)) ||
		slices.Contains(a.Spec.FilesFromSecrets, filesSecretName(gid))
}

// mountedGroups returns the ids of the groups a's spec references.
func mountedGroups(a *appv1alpha1.App) []string {
	var gids []string
	for _, ref := range slices.Concat(a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets) {
		for _, suffix := range []string{envSecretName(""), filesSecretName("")} {
			gid, ok := strings.CutSuffix(ref, suffix)
			if kind, group := id.KindOf(gid); ok && group && kind == id.EnvGroup {
				gids = addString(gids, gid)
			}
		}
	}
	return gids
}

// linkAliases are the entries a link to a may be stored as: its id or, from
// before links were ids, any name it answered to.
func linkAliases(a *appv1alpha1.App) []string {
	aliases := []string{core.AppPublicID(a)}
	for _, name := range []string{a.Labels[core.LabelServiceName], a.Name, a.Spec.DisplayName} {
		if name != "" {
			aliases = addString(aliases, name)
		}
	}
	return aliases
}

// dropLinks removes entries from group gid's link set, writing nothing when
// none of them is there.
func (s *Service) dropLinks(ctx context.Context, gid, workspace string, entries ...string) error {
	_, err := s.mutateMetaCAS(ctx, gid, workspace, func(cur meta) (meta, error) {
		links := removeString(cur.links, entries...)
		if len(links) == len(cur.links) {
			return meta{}, errMetaUnchanged
		}
		cur.links = links
		cur.updatedAt = s.now()
		return cur, nil
	})
	return err
}

// needsLinkMigration reports whether m's links may still hold names stored
// before links were service ids.
func needsLinkMigration(m meta) bool {
	return !m.linksByID && len(m.links) > 0
}

// linkCandidates lists the Apps a group of workspace may link: the
// workspace's, by label wherever the namespace cutover (ADR043) left them, or
// every App with the store off.
func (s *Service) linkCandidates(ctx context.Context, workspace string) ([]appv1alpha1.App, error) {
	var apps appv1alpha1.AppList
	var err error
	if workspace != "" {
		err = s.ListByTenant(ctx, &apps, workspace)
	} else {
		err = s.Client.List(ctx, &apps)
	}
	return apps.Items, err
}

// migrateGroupLinks is migrateLinks for one group, listing its candidates.
func (s *Service) migrateGroupLinks(ctx context.Context, gid string, m meta) (meta, error) {
	if !needsLinkMigration(m) {
		return m, nil
	}
	apps, err := s.linkCandidates(ctx, m.workspace)
	if err != nil {
		return meta{}, err
	}
	return s.migrateLinks(ctx, gid, m, apps)
}

// migrateLinks rewrites, once, a link set that may hold names stored before
// links were service ids, given the Apps of the group's workspace. The linked
// services are the ones mounting the group's Secrets. A name one of them
// answers to becomes its id, and one the names no longer reach, such as a
// service linked by a display name it has since changed, is added. A name no
// mounting service answers to was a deleted service's, perhaps since replaced
// by an unrelated one of the same name: it is dropped and logged. Ids stay,
// since none is reused and one may reserve an App not yet created. The group
// is marked once every name in it was resolved.
func (s *Service) migrateLinks(ctx context.Context, gid string, m meta, apps []appv1alpha1.App) (meta, error) {
	if !needsLinkMigration(m) {
		return m, nil
	}
	var mounting []*appv1alpha1.App
	for i := range apps {
		if mountsGroup(&apps[i], gid) {
			mounting = append(mounting, &apps[i])
		}
	}
	resolved := map[string]string{} // name -> id, or "" when no mounting App answers to it
	for _, entry := range m.links {
		if isServiceID(entry) {
			continue
		}
		resolved[entry] = ""
		for _, a := range mounting {
			if slices.Contains(linkAliases(a), entry) {
				resolved[entry] = core.AppPublicID(a)
				break
			}
		}
	}
	var dropped []string
	migrated, err := s.mutateMetaCAS(ctx, gid, m.workspace, func(cur meta) (meta, error) {
		if cur.linksByID {
			return meta{}, errMetaUnchanged
		}
		dropped = dropped[:0]
		complete := true
		var links []string
		for _, entry := range cur.links {
			if key, named := resolved[entry]; named {
				if key == "" {
					dropped = append(dropped, entry)
					continue
				}
				entry = key
			} else if !isServiceID(entry) {
				complete = false // stored after the Apps were listed
			}
			links = addString(links, entry)
		}
		for _, a := range mounting {
			links = addString(links, core.AppPublicID(a))
		}
		cur.links, cur.linksByID = links, complete
		return cur, nil
	})
	if err != nil {
		return meta{}, err
	}
	for _, entry := range dropped {
		log.Printf("envgroups: group %s dropped its link to %q: no service mounting the group answers to it", gid, entry)
	}
	return migrated, nil
}
