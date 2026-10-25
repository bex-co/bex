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

package store

import (
	"context"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// PlatformRebuildRevisions is the set of appID's build revisions (gen-<N>)
// whose release the platform itself opened (TriggerDeployedByRender). Their
// build Jobs are the platform's maintenance, not the tenant's work, so usage
// meters them no build_seconds (w1/m184).
func (s *PGStore) PlatformRebuildRevisions(ctx context.Context, appID string) (map[string]bool, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT DISTINCT generation FROM deploys WHERE app_id = $1 AND trigger = $2 AND generation > 0`,
		appID, TriggerDeployedByRender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var generation int64
		if err := rows.Scan(&generation); err != nil {
			return nil, err
		}
		out[appv1alpha1.BuildRevision(generation)] = true
	}
	return out, rows.Err()
}
