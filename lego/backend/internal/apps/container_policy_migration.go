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
	"slices"

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

// MigrateContainerPolicy moves one existing service (anything but a static
// site) onto render-v1, Render's measured capability set (ADR089 § Privilege
// slice decisions, w6/m149 t008; native runtimes since w32/m462). Upgrades,
// Blueprint syncs and redeploys never migrate a service; this operator-run
// step is the only way, one service id at a time or many at once through
// MigrateContainerPolicies. Without apply it only plans.
// Applied, it persists the row and rolls the service once as a tracked
// settings_change deploy, under the App's optimistic lock; a refusal found
// under the lock restores the row. No ownership step is needed: a policy
// change keeps the pod's UID mapping (t002).
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
	carrier, err := s.linkedCarrier(ctx, a)
	if err != nil {
		return plan, err
	}
	if err := containerPolicyMigratable(a, carrier); err != nil {
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
	err = s.rollouts().PatchLocked(ctx, s.Client, a, store.TriggerSettingsChange, func(a *appv1alpha1.App) error {
		if err := containerPolicyMigratable(a, carrier); err != nil {
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

// ContainerPolicyMigrationLine is one service of a bulk run: its plan or
// outcome, or why it was refused.
type ContainerPolicyMigrationLine struct {
	ContainerPolicyMigration
	Refused string `json:"refused,omitempty"`
}

// ContainerPolicyMigrationSummary counts a bulk run: Planned services would
// move (a dry run), Migrated moved, NoOp already ran render-v1, and Refused
// were left as they were.
type ContainerPolicyMigrationSummary struct {
	Planned  int `json:"planned"`
	Migrated int `json:"migrated"`
	NoOp     int `json:"noOp"`
	Refused  int `json:"refused"`
}

// ContainerPolicyMigrationScope is the services a bulk run moves.
type ContainerPolicyMigrationScope int

const (
	// MigrationScopeNative is every native runtime service (`--all-native`,
	// w32/m462): a linked service is judged by its build source's runtime.
	MigrationScopeNative ContainerPolicyMigrationScope = iota + 1
	// MigrationScopeAll is every service but a static site (`--all`,
	// w32/040): Docker, image and native runtime services, the set the
	// creation default gives render-v1 (runsContainerPolicy).
	MigrationScopeAll
)

// MigrateContainerPolicies runs MigrateContainerPolicy for every service in
// scope, in its own workspace namespace, in service id order, one at a time
// (w32/m462, w32/040); build sources themselves never run. Each service's
// line goes to emit as it settles. A refusal (suspended, a deploy in flight,
// a change pending, no control-plane row) is reported and the run goes on;
// only failing to list the services stops it.
func (s *Service) MigrateContainerPolicies(ctx context.Context, scope ContainerPolicyMigrationScope, apply bool, emit func(ContainerPolicyMigrationLine)) (ContainerPolicyMigrationSummary, error) {
	var summary ContainerPolicyMigrationSummary
	ids, err := s.migrationServiceIDs(ctx, scope)
	if err != nil {
		return summary, err
	}
	for _, id := range ids {
		// By id, not the listed App: each service is re-read just before it
		// moves, so a run over many services judges each one as it is then.
		plan, err := s.MigrateContainerPolicy(ctx, id, apply)
		line := ContainerPolicyMigrationLine{ContainerPolicyMigration: plan}
		line.ServiceID = id
		switch {
		case err != nil:
			line.Refused = err.Error()
			summary.Refused++
		case plan.NoOp:
			summary.NoOp++
		case plan.Applied:
			summary.Migrated++
		default:
			summary.Planned++
		}
		emit(line)
	}
	return summary, nil
}

// migrationServiceIDs is the public id of every service in scope, sorted.
func (s *Service) migrationServiceIDs(ctx context.Context, scope ContainerPolicyMigrationScope) ([]string, error) {
	if scope != MigrationScopeNative && scope != MigrationScopeAll {
		return nil, fmt.Errorf("%w: unknown container policy migration scope %d", core.ErrBadRequest, scope)
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	var ids []string
	services := core.ServiceApps(list.Items)
	for i := range services {
		a := &services[i]
		id := a.Labels[core.LabelAppID]
		// A linked service keeps its own type, so its build source never
		// changes whether it runs a container policy at all.
		if id == "" || !core.AppInOwnWorkspaceNamespace(a) || !runsContainerPolicy(a.Spec) {
			continue
		}
		if scope == MigrationScopeAll {
			ids = append(ids, id)
			continue
		}
		carrier, err := s.linkedCarrier(ctx, a)
		if err != nil {
			// MigrateContainerPolicy reports the missing build source.
			ids = append(ids, id)
			continue
		}
		if buildSourceNative(sourceForImageRules(a.Spec, carrier)) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
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

// linkedCarrier is the build source carrier a linked service deploys from, in
// the service's own namespace; nil for a service that builds itself.
func (s *Service) linkedCarrier(ctx context.Context, a *appv1alpha1.App) (*appv1alpha1.App, error) {
	if a.Spec.BuildSource == nil {
		return nil, nil
	}
	var carrier appv1alpha1.App
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: a.Spec.BuildSource.Name}, &carrier); err != nil {
		return nil, fmt.Errorf("build source of %s: %w", a.Name, err)
	}
	return &carrier, nil
}

// containerPolicyMigratable refuses what the creation default would not have
// given render-v1, and a service whose rollout, pending change or suspension
// would race the policy change. A linked service is judged by its build
// source's runtime (sourceForImageRules).
func containerPolicyMigratable(a, carrier *appv1alpha1.App) error {
	if !runsContainerPolicy(sourceForImageRules(a.Spec, carrier)) {
		return fmt.Errorf("%w: a static site runs no container policy", core.ErrBadRequest)
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
