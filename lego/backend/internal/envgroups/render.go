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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
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
