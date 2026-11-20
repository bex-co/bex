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
	"log"
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
// environment, leaving every other member as it is — including one another
// request placed while this one runs, so each kind applies as a delta on the
// named ids, never as a full list computed from an earlier read. A malformed
// id refuses the whole request before anything changes; the kinds then apply
// in turn, and a failure reverses what this request changed in the kinds
// already applied, newest first, so the request is all-or-nothing. When a
// reversal fails too, the answer is ENVIRONMENT_MEMBERSHIP_PARTIAL naming the
// members that may still be changed.
func (s *Service) ChangeResources(ctx context.Context, id string, resourceIDs []string, add bool) (EnvironmentView, error) {
	// Authorization and existence first: a caller who may not touch the
	// environment learns nothing about its id format rules.
	viewCtx, e, err := s.viewableEnvironment(ctx, id)
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
	kinds := []memberKind{
		{requested: want.services, services: true},
		{requested: want.databases, idx: s.databases()},
		{requested: want.keyValues, idx: s.keyValues()},
		{requested: want.envGroups, idx: s.envGroups()},
	}
	var done []memberKind
	for _, k := range kinds {
		if len(k.requested) == 0 {
			continue
		}
		var failed string
		k.changed, failed, err = s.changeKind(ctx, k, id, k.requested, add)
		done = append(done, k)
		if err != nil {
			// Reverse only the members this request changed: the others may
			// have been placed by a concurrent request since.
			var kept []string
			for _, d := range slices.Backward(done) {
				if len(d.changed) == 0 {
					continue
				}
				if _, _, undoErr := s.changeKind(ctx, d, id, d.changed, !add); undoErr != nil {
					log.Printf("environments: reversing %v in %s: %v", d.changed, e.ID, undoErr)
					kept = append(kept, d.changed...)
				}
			}
			return EnvironmentView{}, membershipInterrupted(e.ID, kept, failed, err)
		}
	}
	return s.toFullView(viewCtx, e)
}

// memberKind is one member kind of a ChangeResources request: the ids it names
// and, once applied, the members it changed.
type memberKind struct {
	requested, changed []string
	// services marks the kind with its own store verb; idx serves the
	// label-backed kinds (nil when that kind is unwired).
	services bool
	idx      resourceIndex
}

// changeKind adds (add) or removes only memberIDs of k's kind, leaving every
// other member as it is. It returns the members it changed — on failure too,
// with the member it stopped at when one member's change failed — which a
// reversal passes back with add flipped.
func (s *Service) changeKind(ctx context.Context, k memberKind, id string, memberIDs []string, add bool) ([]string, string, error) {
	if k.services {
		changed, err := s.changeServices(ctx, id, memberIDs, add)
		return changed, "", err
	}
	mode := removeMembers
	if add {
		mode = addMembers
	}
	_, _, changed, failed, err := s.changeMembers(ctx, k.idx, id, memberIDs, mode)
	return changed, failed, err
}
