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
	"log"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// renderEnvGroup is Render's envGroup, the group REST and MCP answer with. Its
// serviceLinks name each linked service as an envGroupLink object, where the
// view, and GraphQL with it, keeps the stored ids the dashboard reads
// (w5/092).
type renderEnvGroup struct {
	EnvGroupView
	ServiceLinks []serviceLink `json:"serviceLinks"`
}

// serviceLink is Render's envGroupLink.
type serviceLink struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// renderEnvGroups renders views, listing the services of each workspace with a
// linked group once. A link whose service is gone or being deleted, deleted
// outside the API or not created yet, is left out: a Render link names a live
// service.
func (s *Service) renderEnvGroups(ctx context.Context, views []EnvGroupView) ([]renderEnvGroup, error) {
	services := map[string]map[string]*appv1alpha1.App{}
	out := make([]renderEnvGroup, 0, len(views))
	for _, view := range views {
		links := make([]serviceLink, 0, len(view.ServiceLinks))
		if len(view.ServiceLinks) > 0 {
			byID, listed := services[view.OwnerID]
			if !listed {
				apps, err := s.linkCandidates(ctx, view.OwnerID)
				if err != nil {
					return nil, err
				}
				byID = make(map[string]*appv1alpha1.App, len(apps))
				for i := range apps {
					if apps[i].DeletionTimestamp.IsZero() {
						byID[core.AppPublicID(&apps[i])] = &apps[i]
					}
				}
				services[view.OwnerID] = byID
			}
			for _, link := range view.ServiceLinks {
				if a, ok := byID[link]; ok {
					links = append(links, serviceLink{ID: link, Name: core.DisplayedAppName(a), Type: resourcemeta.ServiceTypeShort(a.Spec.Type)})
				}
			}
		}
		out = append(out, renderEnvGroup{EnvGroupView: view, ServiceLinks: links})
	}
	return out, nil
}

// rendered renders the group a read returned, passing its error through, so
// each read stays one call: s.rendered(ctx)(s.GetEnvGroup(ctx, id)).
func (s *Service) rendered(ctx context.Context) func(EnvGroupView, error) (renderEnvGroup, error) {
	return func(view EnvGroupView, err error) (renderEnvGroup, error) {
		if err != nil {
			return renderEnvGroup{}, err
		}
		out, err := s.renderEnvGroups(ctx, []EnvGroupView{view})
		if err != nil {
			return renderEnvGroup{}, err
		}
		return out[0], nil
	}
}

// committed is rendered for a write. The write has committed by then, so a
// failure to name its links answers them empty and is logged, rather than
// failing a call a retry cannot repeat: a create would find its name taken.
func (s *Service) committed(ctx context.Context) func(EnvGroupView, error) (renderEnvGroup, error) {
	return func(view EnvGroupView, err error) (renderEnvGroup, error) {
		if err != nil {
			return renderEnvGroup{}, err
		}
		out, err := s.renderEnvGroups(ctx, []EnvGroupView{view})
		if err != nil {
			log.Printf("envgroups: group %s: name its linked services: %v", view.ID, err)
			return renderEnvGroup{EnvGroupView: view, ServiceLinks: []serviceLink{}}, nil
		}
		return out[0], nil
	}
}

// revealedEnvGroup is the group REST answers with: Render's envGroup requires
// `value` on every envVar and `content` on every secretFile (w8/182), where the
// shared view keeps them off (the dashboard, GraphQL and MCP reveal per key). The
// outer SecretFiles shadows the embedded view's masked one so an empty file
// reads `content:""`.
type revealedEnvGroup struct {
	renderEnvGroup
	SecretFiles []SecretFileContent `json:"secretFiles"`
}

// revealGroups fills each group's values and file contents from the store, for
// the REST read shapes. It is a sensitive read: every group is authorized for
// can_view_sensitive and re-asserted uncached, like the per-key reveals, and a
// caller without it is refused the whole read rather than served a masked body.
func (s *Service) revealGroups(ctx context.Context, groups []renderEnvGroup) ([]revealedEnvGroup, error) {
	out := make([]revealedEnvGroup, 0, len(groups))
	for _, group := range groups {
		g, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, group.ID)
		if err != nil {
			return nil, err
		}
		if err := s.authorizeGroupSensitiveFresh(ctx, g); err != nil {
			return nil, err
		}
		var contents groupContents
		if group.writtenContents != nil {
			// The create itself just wrote these contents (w4/m242): reveal
			// them directly instead of re-reading a store whose read may lag
			// or fail in the window right after the write.
			contents = *group.writtenContents
		} else if err := s.withGroupContent(ctx, group.ID, func(ctx context.Context, g store.EnvGroup) error {
			var err error
			contents, err = s.readGroupContents(ctx, g)
			// A group with nothing saved yet has no maps: it reveals as empty.
			if errors.Is(err, core.ErrNotFound) {
				contents, err = groupContents{}, nil
			}
			return err
		}); err != nil {
			return nil, err
		}
		// A contents map missing a listed key is not an empty group: it is a
		// read that has not caught up with the write. Fail rather than invent
		// blank values for keys the group lists (w4/m242).
		for _, v := range group.EnvVars {
			if _, ok := contents.env[v.Key]; !ok {
				return nil, fmt.Errorf("env group %s contents are not readable yet", group.ID)
			}
		}
		for _, f := range group.SecretFiles {
			if _, ok := contents.files[f.Name]; !ok {
				return nil, fmt.Errorf("env group %s contents are not readable yet", group.ID)
			}
		}
		revealed := revealedEnvGroup{renderEnvGroup: group, SecretFiles: make([]SecretFileContent, 0, len(group.SecretFiles))}
		revealed.EnvVars = slices.Clone(group.EnvVars)
		for i, v := range revealed.EnvVars {
			revealed.EnvVars[i].Value, revealed.EnvVars[i].ValueSet = contents.env[v.Key], true
		}
		for _, f := range group.SecretFiles {
			revealed.SecretFiles = append(revealed.SecretFiles, SecretFileContent{Name: f.Name, Content: contents.files[f.Name]})
		}
		out = append(out, revealed)
	}
	return out, nil
}

// revealedOne is revealGroups for the single-group shapes. A group a write has
// already committed is answered masked, with the failure logged, when its values
// cannot be revealed: a retry cannot repeat the write.
func (s *Service) revealedOne(ctx context.Context, group renderEnvGroup, committed bool) (any, error) {
	out, err := s.revealGroups(ctx, []renderEnvGroup{group})
	if err != nil {
		if committed {
			log.Printf("envgroups: group %s: reveal values for the write response: %v", group.ID, err)
			return group, nil
		}
		return nil, err
	}
	return out[0], nil
}

// committedRevealed is committed for a REST write response: the envGroup with
// its values and file contents, as Render's schema requires (w8/182).
func (s *Service) committedRevealed(ctx context.Context) func(EnvGroupView, error) (any, error) {
	return func(view EnvGroupView, err error) (any, error) {
		group, err := s.committed(ctx)(view, err)
		if err != nil {
			return nil, err
		}
		return s.revealedOne(ctx, group, true)
	}
}
