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

package environments

import (
	"context"
	"fmt"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Render's environment-membership API (w8/188): POST and DELETE
// /environments/{id}/resources add or remove resources by id — a service
// (srv-), Postgres (dpg-), Key Value (red-) or environment group (evg-) — where
// bex's own PUT …-links routes replace one kind's whole list.

// codeEnvironmentNotEmpty names Render's 409 for deleting an environment (or a
// project) that still holds resources.
const codeEnvironmentNotEmpty = "ENVIRONMENT_NOT_EMPTY"

// EnvironmentOccupied reports whether v holds any resource.
func EnvironmentOccupied(v EnvironmentView) bool {
	return len(v.ServiceIDs)+len(v.DatabaseIDs)+len(v.KeyValueIDs)+len(v.EnvGroupIDs) > 0
}

// NotEmptyError is the 409 REST answers when a delete would orphan resources,
// as Render's API does; the dashboard and GraphQL keep their confirmed
// detach-and-delete flow.
func NotEmptyError(kind, id string) error {
	return core.NewConflictError(codeEnvironmentNotEmpty,
		fmt.Sprintf("the %s still contains resources; move or delete them first, then delete it", kind),
		map[string]any{"id": id})
}

// resourceKinds groups requested ids by the member kind their prefix names.
type resourceKinds struct{ services, databases, keyValues, envGroups []string }

func classifyResources(resourceIDs []string) (resourceKinds, error) {
	var out resourceKinds
	for _, id := range resourceIDs {
		kind, ok := ids.KindOf(id)
		switch {
		case !ok:
			return out, fmt.Errorf("%w: %q is not a service, Postgres, Key Value or environment group id", core.ErrBadRequest, id)
		case kind == ids.Service:
			out.services = append(out.services, id)
		case kind == ids.Postgres:
			out.databases = append(out.databases, id)
		case kind == ids.KeyValue:
			out.keyValues = append(out.keyValues, id)
		case kind == ids.EnvGroup:
			out.envGroups = append(out.envGroups, id)
		default:
			return out, fmt.Errorf("%w: %q is not a service, Postgres, Key Value or environment group id", core.ErrBadRequest, id)
		}
	}
	return out, nil
}

// ChangeResources adds (or removes) the listed resources to (from) the
// environment, leaving every other member as it is. A malformed id refuses the
// whole request before anything changes; the kinds then apply in turn, and a
// failure restores the kinds already changed so the request is all-or-nothing
// as far as the individual membership verbs allow.
func (s *Service) ChangeResources(ctx context.Context, id string, resourceIDs []string, add bool) (EnvironmentView, error) {
	// Authorization and existence first: a caller who may not touch the
	// environment learns nothing about its id format rules.
	current, err := s.Get(ctx, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if len(resourceIDs) == 0 {
		return EnvironmentView{}, fmt.Errorf("%w: resourceIds is required", core.ErrBadRequest)
	}
	want, err := classifyResources(resourceIDs)
	if err != nil {
		return EnvironmentView{}, err
	}
	type step struct {
		now, requested []string
		set            func(context.Context, string, []string) (EnvironmentView, error)
	}
	steps := []step{
		{current.ServiceIDs, want.services, s.SetServices},
		{current.DatabaseIDs, want.databases, s.SetDatabases},
		{current.KeyValueIDs, want.keyValues, s.SetKeyValues},
		{current.EnvGroupIDs, want.envGroups, s.SetEnvGroups},
	}
	var applied []step
	view := current
	for _, st := range steps {
		if len(st.requested) == 0 {
			continue
		}
		next := slices.Clone(st.now)
		for _, rid := range st.requested {
			has := slices.Contains(next, rid)
			switch {
			case add && !has:
				next = append(next, rid)
			case !add && has:
				next = slices.DeleteFunc(next, func(m string) bool { return m == rid })
			}
		}
		if slices.Equal(next, st.now) {
			continue
		}
		v, err := st.set(ctx, id, next)
		if err != nil {
			for _, done := range slices.Backward(applied) {
				_, _ = done.set(ctx, id, done.now) // best effort: put the earlier kinds back
			}
			return EnvironmentView{}, err
		}
		view = v
		applied = append(applied, st)
	}
	return view, nil
}
