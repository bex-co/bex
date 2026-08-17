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

// blueprint_ownership.go (w8/m23): a resource created or adopted by a
// Git-connected Blueprint carries core.LabelBlueprint; a second blueprint
// naming the same resource is refused pre-write with
// BLUEPRINT_RESOURCE_CONFLICT unless the explicit takeover confirmation
// transfers ownership. Render documents "max one Blueprint per resource;
// last sync wins" without enforcing it — refusing is a deliberate, documented
// improvement (ADR018 Blueprint row). Disconnect clears the marker; manual
// resources without a marker adopt freely, unchanged.

import (
	"context"
	"fmt"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// BlueprintTakeoverConfirmation is the server-issued phrase that authorizes
// transferring a resource's blueprint ownership. Computed from the OWNING
// blueprint id so the caller confirms exactly what they are taking over from.
func BlueprintTakeoverConfirmation(owningBlueprintID string) string {
	return "takeover blueprint " + owningBlueprintID
}

// blueprintOwnershipConflict is one resource owned by another blueprint.
type blueprintOwnershipConflict struct {
	kind  string // service | database | key_value
	name  string
	owner string // owning blueprint id
}

// blueprintOwnershipConflicts lists the parsed stack's resources that a
// DIFFERENT blueprint currently owns. selfID "" means "no blueprint identity"
// (a bare validate): every owned resource conflicts.
func (s *Service) blueprintOwnershipConflicts(ctx context.Context, tenantID, selfID string, st parsedStack) ([]blueprintOwnershipConflict, error) {
	// Cluster-wide, label-scoped lists (the DatastoreListOptions shape): a
	// workspace's resources may straddle the shared and per-tenant namespaces.
	opts := []client.ListOption{client.MatchingLabels{core.LabelTenant: tenantID}}

	var conflicts []blueprintOwnershipConflict
	record := func(kind, name, owner string) {
		if owner != "" && owner != selfID {
			conflicts = append(conflicts, blueprintOwnershipConflict{kind: kind, name: name, owner: owner})
		}
	}

	if len(st.services) > 0 {
		var apps appv1alpha1.AppList
		if err := s.Client.List(ctx, &apps, opts...); err != nil {
			return nil, err
		}
		byName := map[string]string{}
		for i := range apps.Items {
			byName[appServiceName(&apps.Items[i])] = apps.Items[i].Labels[core.LabelBlueprint]
		}
		for _, svc := range st.services {
			if owner, ok := byName[svc.req.Name]; ok {
				record("service", svc.req.Name, owner)
			}
		}
	}
	if len(st.databases) > 0 {
		databases, err := s.listWorkspaceDatabases(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		byName := map[string]string{}
		for i := range databases.Items {
			byName[databases.Items[i].Spec.Name] = databases.Items[i].Labels[core.LabelBlueprint]
		}
		for _, db := range st.databases {
			if owner, ok := byName[db.name]; ok {
				record("database", db.name, owner)
			}
		}
	}
	if len(st.keyValues) > 0 {
		keyValues, err := s.listWorkspaceKeyValues(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		byName := map[string]string{}
		for i := range keyValues.Items {
			byName[keyValues.Items[i].Spec.Name] = keyValues.Items[i].Labels[core.LabelBlueprint]
		}
		for _, kv := range st.keyValues {
			if owner, ok := byName[kv.name]; ok {
				record("key value", kv.name, owner)
			}
		}
	}
	return conflicts, nil
}

// preflightBlueprintOwnership refuses a blueprint apply that would overwrite
// another blueprint's resources — before any write — unless the request
// carries the exact takeover confirmation, which transfers ownership (the
// post-apply stamp rewrites the label). Non-blueprint deploys are exempt.
func (s *Service) preflightBlueprintOwnership(ctx context.Context, req DeployRequest, st parsedStack) error {
	if req.BlueprintID == "" {
		return nil
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return nil
	}
	conflicts, err := s.blueprintOwnershipConflicts(ctx, tenantID, req.BlueprintID, st)
	if err != nil {
		return fmt.Errorf("checking Blueprint resource ownership: %w", err)
	}
	if len(conflicts) == 0 {
		return nil
	}
	first := conflicts[0]
	phrase := BlueprintTakeoverConfirmation(first.owner)
	if req.Confirm == phrase {
		// Explicit takeover of first.owner's resources: proceed — but only
		// for THAT owner. A stack conflicting with two different blueprints
		// resolves one owner per confirmation.
		for _, c := range conflicts {
			if c.owner != first.owner {
				return blueprintOwnershipError(c)
			}
		}
		return nil
	}
	return blueprintOwnershipError(first)
}

func blueprintOwnershipError(c blueprintOwnershipConflict) error {
	phrase := BlueprintTakeoverConfirmation(c.owner)
	return core.NewConflictError("BLUEPRINT_RESOURCE_CONFLICT",
		fmt.Sprintf("%s %q is managed by blueprint %s; retry with confirm=%q to transfer ownership to this blueprint", c.kind, c.name, c.owner, phrase),
		map[string]any{"resource": c.name, "kind": c.kind, "owningBlueprintId": c.owner, "confirm": phrase})
}

// stampBlueprintOwnership records req.BlueprintID on every resource the apply
// converged — create, adopt, and takeover all land here, and re-stamping is
// how pre-existing managed resources lazily backfill on their next sync.
// Best-effort by design: the apply has already succeeded.
// appServiceName is the manifest-facing service name: the service-name label
// for store-managed Apps (CR names carry the tenant prefix), the bare CR name
// for hand-applied ones.
func appServiceName(a *appv1alpha1.App) string {
	if name := a.Labels[core.LabelServiceName]; name != "" {
		return name
	}
	return a.Name
}

func (s *Service) stampBlueprintOwnership(ctx context.Context, blueprintID string, st parsedStack) {
	if blueprintID == "" {
		return
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return
	}

	stamp := func(obj client.Object) {
		if obj.GetLabels()[core.LabelBlueprint] == blueprintID {
			return
		}
		base := obj.DeepCopyObject().(client.Object)
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[core.LabelBlueprint] = blueprintID
		obj.SetLabels(labels)
		if err := s.Client.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
			log.Printf("blueprint %s: stamping ownership on %s/%s: %v", blueprintID, obj.GetNamespace(), obj.GetName(), err)
		}
	}

	var apps appv1alpha1.AppList
	if err := s.Client.List(ctx, &apps, client.MatchingLabels{core.LabelTenant: tenantID}); err == nil {
		wanted := map[string]bool{}
		for _, svc := range st.services {
			wanted[svc.req.Name] = true
		}
		for i := range apps.Items {
			if wanted[appServiceName(&apps.Items[i])] {
				stamp(&apps.Items[i])
			}
		}
	}
	if databases, err := s.listWorkspaceDatabases(ctx, tenantID); err == nil {
		wanted := map[string]bool{}
		for _, db := range st.databases {
			wanted[db.name] = true
		}
		for i := range databases.Items {
			if wanted[databases.Items[i].Spec.Name] {
				stamp(&databases.Items[i])
			}
		}
	}
	if keyValues, err := s.listWorkspaceKeyValues(ctx, tenantID); err == nil {
		wanted := map[string]bool{}
		for _, kv := range st.keyValues {
			wanted[kv.name] = true
		}
		for i := range keyValues.Items {
			if wanted[keyValues.Items[i].Spec.Name] {
				stamp(&keyValues.Items[i])
			}
		}
	}
}

// clearBlueprintOwnership removes the marker from every resource the
// disconnected blueprint owned — they become unmanaged (Render disconnect
// semantics: resources survive, management ends). Best-effort post-disconnect.
func (s *Service) clearBlueprintOwnership(ctx context.Context, tenantID, blueprintID string) {
	owned := []client.ListOption{client.MatchingLabels{core.LabelTenant: tenantID, core.LabelBlueprint: blueprintID}}

	clear := func(obj client.Object) {
		base := obj.DeepCopyObject().(client.Object)
		labels := obj.GetLabels()
		if labels == nil {
			return
		}
		delete(labels, core.LabelBlueprint)
		obj.SetLabels(labels)
		if err := s.Client.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
			log.Printf("blueprint %s: clearing ownership on %s/%s: %v", blueprintID, obj.GetNamespace(), obj.GetName(), err)
		}
	}

	var apps appv1alpha1.AppList
	if err := s.Client.List(ctx, &apps, owned...); err == nil {
		for i := range apps.Items {
			clear(&apps.Items[i])
		}
	}
	var databases appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &databases, owned...); err == nil {
		for i := range databases.Items {
			clear(&databases.Items[i])
		}
	}
	var keyValues appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &keyValues, owned...); err == nil {
		for i := range keyValues.Items {
			clear(&keyValues.Items[i])
		}
	}
}

// previewOwnershipConflicts reports cross-blueprint conflicts as validation
// entries for the preview surfaces. Self-identity resolves through the
// existing blueprint row for repo+branch, so an existing blueprint's own
// pre-sync preview never conflicts with itself; a not-yet-created blueprint
// conflicts with any owner. Scan failures are swallowed (the apply-path
// preflight is the enforcement point).
func (s *Service) previewOwnershipConflicts(ctx context.Context, repo, branch string, st parsedStack) []BlueprintValidationError {
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return nil
	}
	selfID := ""
	if b, err := s.Blueprints.GetBlueprintByRepo(ctx, tenantID, repo, branch); err == nil {
		selfID = b.ID
	}
	conflicts, err := s.blueprintOwnershipConflicts(ctx, tenantID, selfID, st)
	if err != nil {
		return nil
	}
	entries := make([]BlueprintValidationError, 0, len(conflicts))
	for _, c := range conflicts {
		entries = append(entries, BlueprintValidationError{
			Code:  "BLUEPRINT_RESOURCE_CONFLICT",
			Error: blueprintOwnershipError(c).Error(),
		})
	}
	return entries
}
