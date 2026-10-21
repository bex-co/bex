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
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ContainerPolicyMigration is the plan, or the outcome, of moving one existing
// service onto render-v1.
type ContainerPolicyMigration struct {
	ServiceID    string   `json:"serviceId"`
	Workspace    string   `json:"workspace"`
	From         string   `json:"from"`
	To           string   `json:"to"`
	Capabilities []string `json:"capabilities"`
	NoOp         bool     `json:"noOp"`
	Applied      bool     `json:"applied"`
}

// MigrateContainerPolicy moves one existing Docker/image service onto
// render-v1, Render's measured capability set (ADR089 § Privilege slice
// decisions, w6/m149 t008). Upgrades, Blueprint syncs and redeploys never
// migrate a service; this operator-run step is the only way, one service id
// at a time. Without apply it only plans. Applied, it persists the row and
// rolls the service once as a tracked config_change deploy, under the App's
// optimistic lock; a refusal found under the lock restores the row. No
// ownership step is needed: a policy change keeps the pod's UID mapping
// (t002).
func (s *Service) MigrateContainerPolicy(ctx context.Context, serviceID string, apply bool) (ContainerPolicyMigration, error) {
	a, err := s.appByServiceID(ctx, serviceID)
	if err != nil {
		return ContainerPolicyMigration{}, err
	}
	plan := ContainerPolicyMigration{
		ServiceID: serviceID, Workspace: a.Labels[core.LabelTenant],
		From: a.Spec.ContainerPolicy, To: appv1alpha1.ContainerPolicyRenderV1,
		Capabilities: appv1alpha1.ContainerPolicyCapabilities(appv1alpha1.ContainerPolicyRenderV1),
	}
	if a.Spec.ContainerPolicy == appv1alpha1.ContainerPolicyRenderV1 {
		plan.NoOp = true
		return plan, nil
	}
	if err := containerPolicyMigratable(a); err != nil {
		return plan, err
	}
	if !apply {
		return plan, nil
	}
	id := managedAppID(a)
	if s.Store == nil || id == "" {
		return plan, fmt.Errorf("%w: service %s has no control-plane row to migrate", core.ErrConflict, serviceID)
	}
	if err := s.Store.SetAppContainerPolicy(ctx, id, appv1alpha1.ContainerPolicyRenderV1); err != nil {
		return plan, err
	}
	err = s.rollouts().PatchLocked(ctx, s.Client, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		if err := containerPolicyMigratable(a); err != nil {
			return err
		}
		a.Spec.ContainerPolicy = appv1alpha1.ContainerPolicyRenderV1
		resourcemeta.Touch(a, s.Now())
		return nil
	})
	if err != nil {
		if restore := s.Store.SetAppContainerPolicy(ctx, id, plan.From); restore != nil {
			return plan, errors.Join(err, fmt.Errorf("restore %s policy %q: %w", id, plan.From, restore))
		}
		return plan, err
	}
	plan.Applied = true
	return plan, nil
}

// appByServiceID resolves the operator's argument without a caller identity:
// the App with this public id in its own workspace namespace, ignoring a stray
// same-id copy elsewhere (core.AppInOwnWorkspaceNamespace).
func (s *Service) appByServiceID(ctx context.Context, serviceID string) (*appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.MatchingLabels{core.LabelAppID: serviceID}); err != nil {
		return nil, err
	}
	for i := range list.Items {
		if core.AppInOwnWorkspaceNamespace(&list.Items[i]) {
			return &list.Items[i], nil
		}
	}
	return nil, fmt.Errorf("%w: service %s not found", core.ErrNotFound, serviceID)
}

// containerPolicyMigratable refuses what the creation default would not have
// given render-v1, and a service whose rollout, pending change or suspension
// would race the policy change.
func containerPolicyMigratable(a *appv1alpha1.App) error {
	if !runsContainerPolicy(a.Spec) {
		return fmt.Errorf("%w: only Docker and image services run a container policy", core.ErrBadRequest)
	}
	if a.Spec.Suspended {
		return fmt.Errorf("%w: resume the service before migrating it", core.ErrConflict)
	}
	switch a.Status.Phase {
	case appv1alpha1.PhasePending, appv1alpha1.PhaseBuilding, appv1alpha1.PhaseDeploying:
		return fmt.Errorf("%w: a deploy is in progress; migrate once it settles", core.ErrConflict)
	}
	if a.Generation != a.Status.ObservedGeneration {
		return fmt.Errorf("%w: a change is still being applied; migrate once it settles", core.ErrConflict)
	}
	return nil
}
