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

package apps

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// PlacementReader reads service membership after callers authorize the CRs.
type PlacementReader interface {
	GetAppPlacements(context.Context, []string) (map[string]store.AppPlacement, error)
}

// readViews keeps runtime status on the CR while reading membership from the
// committed source. A list uses one bounded lookup before transport filtering.
func (s *Service) readViews(ctx context.Context, objects []appv1alpha1.App) ([]AppView, error) {
	var placements map[string]store.AppPlacement
	if s.Placements != nil {
		var ids []string
		for i := range objects {
			if objects[i].DeletionTimestamp.IsZero() {
				if id := managedAppID(&objects[i]); id != "" {
					ids = append(ids, id)
				}
			}
		}
		if len(ids) > 0 {
			var err error
			placements, err = s.Placements.GetAppPlacements(ctx, ids)
			if err != nil {
				return nil, err
			}
		}
	}
	views := make([]AppView, 0, len(objects))
	for i := range objects {
		a := &objects[i]
		// Deleting CRs and deleted source rows must not reappear while their
		// finalizers/projector are catching up. Quota reports terminating CRs.
		if !a.DeletionTimestamp.IsZero() {
			continue
		}
		id := managedAppID(a)
		fromStore := s.Placements != nil && id != ""
		var placement store.AppPlacement
		if fromStore {
			var exists bool
			placement, exists = placements[id]
			if !exists {
				continue
			}
			if placement.TenantID != a.Labels[core.LabelTenant] {
				return nil, core.ErrForbidden
			}
		}
		v := s.view(a)
		if fromStore {
			// Empty committed IDs clear old labels after removal/deletion.
			v.ProjectID, v.EnvironmentID = placement.ProjectID, placement.EnvironmentID
		}
		views = append(views, v)
	}
	return views, nil
}
