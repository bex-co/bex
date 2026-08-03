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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintActionPlan returns an authorized current-state plan when the
// complete non-secret state needed to make one is available. Env groups store
// mutable secret values behind a deliberately write-only seam, so their plan
// stays explicitly structural rather than falsely claiming an update/no-op.
func (s *Service) blueprintActionPlan(ctx context.Context, ir BlueprintIR, st parsedStack) (BlueprintPlan, bool, error) {
	if s.Client == nil || len(st.envGroups) > 0 {
		return BlueprintPlan{}, false, nil
	}
	resolver, err := newBlueprintActionResolver(ctx, s, st)
	if err != nil {
		return BlueprintPlan{}, false, err
	}
	plan, err := PlanBlueprintIR(ctx, ir, resolver)
	if err != nil {
		return BlueprintPlan{}, false, err
	}
	return plan, true, nil
}

type blueprintActionResolver struct {
	services  map[string]*appv1alpha1.App
	databases map[string]*appv1alpha1.Database
	keyValues map[string]*appv1alpha1.KeyValue
	parsed    parsedStack
}

func newBlueprintActionResolver(ctx context.Context, s *Service, parsed parsedStack) (*blueprintActionResolver, error) {
	resolver := &blueprintActionResolver{
		services:  map[string]*appv1alpha1.App{},
		databases: map[string]*appv1alpha1.Database{},
		keyValues: map[string]*appv1alpha1.KeyValue{},
		parsed:    parsed,
	}
	tenantID, scoped := s.Tenant(ctx)
	var apps appv1alpha1.AppList
	if err := s.Client.List(ctx, &apps, client.InNamespace(s.AppNamespace(tenantID))); err != nil {
		return nil, err
	}
	for i := range apps.Items {
		app := &apps.Items[i]
		if scoped && app.Labels[core.LabelTenant] != tenantID {
			continue
		}
		resolver.services[app.Name] = app
	}
	var databases appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &databases, client.InNamespace(s.Namespace)); err != nil {
		return nil, err
	}
	for i := range databases.Items {
		database := &databases.Items[i]
		if scoped && database.Labels[core.LabelTenant] != tenantID {
			continue
		}
		if _, duplicate := resolver.databases[database.Spec.Name]; duplicate {
			return nil, fmt.Errorf("%w: database name %q is already used more than once in this workspace", core.ErrConflict, database.Spec.Name)
		}
		resolver.databases[database.Spec.Name] = database
	}
	var keyValues appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &keyValues, client.InNamespace(s.Namespace)); err != nil {
		return nil, err
	}
	for i := range keyValues.Items {
		keyValue := &keyValues.Items[i]
		if scoped && keyValue.Labels[core.LabelTenant] != tenantID {
			continue
		}
		if _, duplicate := resolver.keyValues[keyValue.Spec.Name]; duplicate {
			return nil, fmt.Errorf("%w: key-value name %q is already used more than once in this workspace", core.ErrConflict, keyValue.Spec.Name)
		}
		resolver.keyValues[keyValue.Spec.Name] = keyValue
	}
	return resolver, nil
}

func (r *blueprintActionResolver) ResolveBlueprintResource(_ context.Context, kind BlueprintResourceKind, name string) (BlueprintCurrentResource, bool, error) {
	switch kind {
	case BlueprintResourceService:
		app, ok := r.services[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		id := store.ManagedAppID(app.Labels)
		if id == "" {
			id = app.Name
		}
		return BlueprintCurrentResource{ID: id}, true, nil
	case BlueprintResourcePostgres:
		database, ok := r.databases[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		return BlueprintCurrentResource{ID: database.Name}, true, nil
	case BlueprintResourceKeyValue:
		keyValue, ok := r.keyValues[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		return BlueprintCurrentResource{ID: keyValue.Name}, true, nil
	default:
		return BlueprintCurrentResource{}, false, nil
	}
}

func (r *blueprintActionResolver) PlanBlueprintResource(_ context.Context, resource BlueprintResourceIR, current BlueprintCurrentResource, exists bool) (BlueprintPlanAction, error) {
	action := BlueprintPlanAction{Kind: resource.Kind, Name: resource.Name, SourcePath: resource.SourcePath, ResourceID: current.ID}
	if !exists {
		action.Operation = BlueprintPlanCreate
		return action, nil
	}
	switch resource.Kind {
	case BlueprintResourceService:
		svc, ok := parsedBlueprintService(r.parsed, resource.Name)
		if !ok {
			return BlueprintPlanAction{}, fmt.Errorf("compiled service is missing")
		}
		want, err := specFromCreate(svc.req)
		if err != nil {
			return BlueprintPlanAction{}, err
		}
		probe := r.services[resource.Name].Spec
		if ApplyBlueprintServiceSpec(&probe, want, svc.fields) {
			action.Operation = BlueprintPlanUpdate
			action.ChangedFields = blueprintPlanFieldChanges(resource.Fields)
			return action, nil
		}
	case BlueprintResourcePostgres:
		database, ok := parsedBlueprintDatabase(r.parsed, resource.Name)
		if !ok {
			return BlueprintPlanAction{}, fmt.Errorf("compiled database is missing")
		}
		probe := r.databases[resource.Name].Spec
		changed, err := ApplyBlueprintDatabaseSpec(&probe, database.spec, database.fields)
		if err != nil {
			if conflict, ok := err.(*BlueprintFieldConflictError); ok {
				return BlueprintPlanAction{}, fmt.Errorf("%w: database %q %s %s", core.ErrBadRequest, resource.Name, conflict.Path, conflict.Message)
			}
			return BlueprintPlanAction{}, fmt.Errorf("%w: %v", core.ErrBadRequest, err)
		}
		if changed {
			action.Operation = BlueprintPlanUpdate
			action.ChangedFields = blueprintPlanFieldChanges(resource.Fields)
			return action, nil
		}
	case BlueprintResourceKeyValue:
		keyValue, ok := parsedBlueprintKeyValue(r.parsed, resource.Name)
		if !ok {
			return BlueprintPlanAction{}, fmt.Errorf("compiled key value is missing")
		}
		probe := r.keyValues[resource.Name].Spec
		if ApplyBlueprintKeyValueSpec(&probe, keyValue.spec, keyValue.fields) {
			action.Operation = BlueprintPlanUpdate
			action.ChangedFields = blueprintPlanFieldChanges(resource.Fields)
			return action, nil
		}
	}
	action.Operation = BlueprintPlanNoop
	return action, nil
}

func parsedBlueprintService(st parsedStack, name string) (parsedService, bool) {
	for _, service := range st.services {
		if service.req.Name == name {
			return service, true
		}
	}
	return parsedService{}, false
}

func parsedBlueprintDatabase(st parsedStack, name string) (parsedDatabase, bool) {
	for _, database := range st.databases {
		if database.name == name {
			return database, true
		}
	}
	return parsedDatabase{}, false
}

func parsedBlueprintKeyValue(st parsedStack, name string) (parsedKeyValue, bool) {
	for _, keyValue := range st.keyValues {
		if keyValue.name == name {
			return keyValue, true
		}
	}
	return parsedKeyValue{}, false
}

func blueprintPlanFieldChanges(fields map[string]BlueprintField) []BlueprintFieldChange {
	changes := make([]BlueprintFieldChange, 0, len(fields))
	for name := range fields {
		changes = append(changes, BlueprintFieldChange{Path: name})
	}
	return changes
}
