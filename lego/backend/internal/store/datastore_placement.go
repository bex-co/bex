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
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type datastorePlacementStore interface {
	GetProject(context.Context, string) (Project, error)
	GetEnvironment(context.Context, string) (Environment, error)
}

type placementLookup[T any] struct {
	value T
	err   error
}

// A pass caches authoritative reads only for its lifetime. An unavailable or
// foreign grouping is not evidence of deletion and must never remove an ACL.
type datastorePlacementRepair struct {
	reconciler      *Reconciler
	store           datastorePlacementStore
	projects        map[string]placementLookup[Project]
	environments    map[string]placementLookup[Environment]
	workspaceErrors map[string]error
}

func newDatastorePlacementRepair(r *Reconciler) *datastorePlacementRepair {
	st, ok := r.Store.(datastorePlacementStore)
	if !ok {
		return nil
	}
	return &datastorePlacementRepair{reconciler: r, store: st, projects: map[string]placementLookup[Project]{}, environments: map[string]placementLookup[Environment]{}, workspaceErrors: map[string]error{}}
}

func (r *Reconciler) repairDatastorePlacements(ctx context.Context, databases []appv1alpha1.Database, keyValues []appv1alpha1.KeyValue) error {
	repair := newDatastorePlacementRepair(r)
	total := len(databases) + len(keyValues)
	if repair == nil || total == 0 {
		return nil
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var errs []error
	start := r.datastorePlacementCursor % total
	for offset := 0; offset < total; offset++ {
		if err := bounded.Err(); err != nil {
			errs = append(errs, fmt.Errorf("datastore placement repair budget: %w", err))
			break
		}
		index := (start + offset) % total
		r.datastorePlacementCursor = (index + 1) % total
		var obj client.Object
		if index < len(databases) {
			obj = &databases[index]
		} else {
			obj = &keyValues[index-len(databases)]
		}
		if err := repair.repair(bounded, obj); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *datastorePlacementRepair) project(ctx context.Context, id string) (Project, error) {
	if value, ok := p.projects[id]; ok {
		return value.value, value.err
	}
	value, err := p.store.GetProject(ctx, id)
	p.projects[id] = placementLookup[Project]{value: value, err: err}
	return value, err
}

func (p *datastorePlacementRepair) environment(ctx context.Context, id string) (Environment, error) {
	if value, ok := p.environments[id]; ok {
		return value.value, value.err
	}
	value, err := p.store.GetEnvironment(ctx, id)
	p.environments[id] = placementLookup[Environment]{value: value, err: err}
	return value, err
}

func (p *datastorePlacementRepair) ownsWorkspace(ctx context.Context, workspace string) error {
	if err, ok := p.workspaceErrors[workspace]; ok {
		return err
	}
	_, err := p.reconciler.Store.GetTenant(ctx, workspace)
	if err == nil {
		var ns corev1.Namespace
		err = p.reconciler.Client.Get(ctx, client.ObjectKey{Name: WorkspaceNamespace(workspace)}, &ns)
		if err == nil && (ns.Labels[LabelWorkspace] != workspace || !isManaged(&ns) || ns.Labels[RegimeLabel] != RegimeHosting || !ownedBy(ns.Labels, p.reconciler.identity())) {
			err = fmt.Errorf("workspace namespace ownership is not established")
		}
	}
	p.workspaceErrors[workspace] = err
	return err
}

func (p *datastorePlacementRepair) repair(ctx context.Context, obj client.Object) error {
	if p == nil || obj.GetDeletionTimestamp() != nil {
		return nil
	}
	var layer *[]string
	switch resource := obj.(type) {
	case *appv1alpha1.Database:
		layer = &resource.Spec.EnvironmentIPAllowList
	case *appv1alpha1.KeyValue:
		layer = &resource.Spec.EnvironmentIPAllowList
	default:
		return fmt.Errorf("unsupported datastore placement resource %T", obj)
	}
	labels := obj.GetLabels()
	projectID, environmentID := labels[core.LabelProject], labels[core.LabelEnvironment]
	if projectID == "" && environmentID == "" && len(*layer) == 0 {
		return nil
	}
	workspace := labels[LabelTenant]
	if workspace == "" || labels[LabelWorkspace] != workspace || !core.DatastoreInOwnWorkspaceNamespace(obj) {
		return fmt.Errorf("datastore %s/%s placement ownership is ambiguous", obj.GetNamespace(), obj.GetName())
	}
	if owner := labels[ControlPlaneLabel]; owner != "" && owner != p.reconciler.identity() {
		return fmt.Errorf("datastore %s/%s belongs to another control plane", obj.GetNamespace(), obj.GetName())
	}
	if err := p.ownsWorkspace(ctx, workspace); err != nil {
		return fmt.Errorf("verify datastore %s/%s workspace: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	clearProject, clearEnvironment := false, environmentID == "" && len(*layer) > 0
	if projectID != "" {
		project, err := p.project(ctx, projectID)
		switch {
		case errors.Is(err, ErrNotFound):
			clearProject = true
			clearEnvironment = true
		case err != nil:
			return fmt.Errorf("read project %s: %w", projectID, err)
		case project.TenantID != workspace:
			return fmt.Errorf("datastore %s/%s project belongs to another workspace", obj.GetNamespace(), obj.GetName())
		}
	}
	if environmentID != "" {
		environment, err := p.environment(ctx, environmentID)
		switch {
		case errors.Is(err, ErrNotFound):
			clearEnvironment = true
		case err != nil:
			return fmt.Errorf("read environment %s: %w", environmentID, err)
		case environment.TenantID != workspace:
			return fmt.Errorf("datastore %s/%s environment belongs to another workspace", obj.GetNamespace(), obj.GetName())
		default:
			parent, err := p.project(ctx, environment.ProjectID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return fmt.Errorf("read environment parent %s: %w", environment.ProjectID, err)
			}
			if err == nil && parent.TenantID != workspace {
				return fmt.Errorf("environment %s parent belongs to another workspace", environmentID)
			}
			if errors.Is(err, ErrNotFound) || environment.ProjectID != projectID {
				clearEnvironment = true
			}
		}
	}
	if !clearProject && !clearEnvironment {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	before := obj.DeepCopyObject().(client.Object)
	if clearProject {
		delete(labels, core.LabelProject)
	}
	if clearEnvironment {
		delete(labels, core.LabelEnvironment)
		*layer = nil
	}
	// Comparing resourceVersion makes a concurrent valid reassignment win.
	// The next pass re-reads both labels and groupings before trying again.
	if err := p.reconciler.Client.Patch(ctx, obj, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return fmt.Errorf("repair datastore %s/%s placement: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}
