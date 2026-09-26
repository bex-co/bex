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

// blueprint_ownership.go (w8/m23 + w8/m40): a resource created or adopted by a
// Git-connected Blueprint carries a durable workspace claim
// (blueprint_resource_claims) mirrored by core.LabelBlueprint; a second
// blueprint naming the same resource is refused with BLUEPRINT_RESOURCE_CONFLICT
// unless the explicit takeover confirmation transfers ownership. Claims couple
// to each successful resource write so partial apply cannot leave mutated
// resources unowned, and replicas coordinate through the UNIQUE claim rather
// than a process-local check. Render documents "max one Blueprint per resource;
// last sync wins" without enforcing it — refusing is a deliberate, documented
// improvement (ADR018 Blueprint row). Disconnect clears the claim and marker;
// manual resources without a marker adopt freely, unchanged.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
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
// DIFFERENT blueprint currently owns. Prefers the durable claim table when
// wired (w8/m40); falls back to CR labels. selfID "" means "no blueprint
// identity" (a bare validate): every owned resource conflicts.
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
	ownerOf := func(kind, name, labelOwner string) string {
		if s.Blueprints != nil {
			if claimOwner, err := s.Blueprints.GetBlueprintResourceOwner(ctx, tenantID, kind, name); err == nil && claimOwner != "" {
				return claimOwner
			}
		}
		return labelOwner
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
			labelOwner := byName[svc.req.Name]
			record("service", svc.req.Name, ownerOf("service", svc.req.Name, labelOwner))
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
			labelOwner := byName[db.name]
			record("database", db.name, ownerOf("database", db.name, labelOwner))
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
			labelOwner := byName[kv.name]
			record("key value", kv.name, ownerOf("key_value", kv.name, labelOwner))
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
// converged — create, adopt, and takeover all land here as a fail-loud
// backfill for no-op short circuits that already claimed at write time
// (w8/m40). A failed claim or label patch fails the sync: ownership
// persistence failure cannot look like success.
// appServiceName is the manifest-facing service name: the service-name label
// for store-managed Apps (CR names carry the tenant prefix), the bare CR name
// for hand-applied ones.
func appServiceName(a *appv1alpha1.App) string {
	if name := a.Labels[core.LabelServiceName]; name != "" {
		return name
	}
	return a.Name
}

// takeoverExpectedOwner returns the owning blueprint id encoded in Confirm, or
// "" when Confirm is not a takeover phrase.
func takeoverExpectedOwner(confirm string) string {
	const prefix = "takeover blueprint "
	if strings.HasPrefix(confirm, prefix) {
		return strings.TrimPrefix(confirm, prefix)
	}
	return ""
}

// claimBlueprintResourceName takes the durable claim for one resource and
// stamps the CR label. expectedOwner comes from an explicit takeover Confirm;
// empty means adopt-or-keep-mine only.
func (s *Service) claimBlueprintResourceName(ctx context.Context, kind, name string) error {
	req, ok := ctx.Value(deployAuthorityKey{}).(DeployRequest)
	if !ok || req.BlueprintID == "" || s.Blueprints == nil {
		return nil
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		tenantID = s.resolveTenantID(ctx)
	}
	if tenantID == "" {
		return nil
	}
	expected := takeoverExpectedOwner(req.Confirm)
	if err := s.Blueprints.ClaimBlueprintResource(ctx, tenantID, kind, name, req.BlueprintID, expected); err != nil {
		if errors.Is(err, store.ErrBlueprintResourceConflict) {
			owner, _ := s.Blueprints.GetBlueprintResourceOwner(ctx, tenantID, kind, name)
			if owner == "" {
				owner = "another blueprint"
			}
			displayKind := kind
			if kind == "key_value" {
				displayKind = "key value"
			}
			return blueprintOwnershipError(blueprintOwnershipConflict{kind: displayKind, name: name, owner: owner})
		}
		return fmt.Errorf("claiming Blueprint ownership of %s %q: %w", kind, name, err)
	}
	return nil
}

// labelBlueprintOwnership sets bex.co/blueprint-id on obj when it differs.
// Failures are returned — never logged-and-ignored (w8/m40 t004).
func (s *Service) labelBlueprintOwnership(ctx context.Context, blueprintID string, obj client.Object) error {
	if obj.GetLabels()[core.LabelBlueprint] == blueprintID {
		return nil
	}
	base := obj.DeepCopyObject().(client.Object)
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[core.LabelBlueprint] = blueprintID
	obj.SetLabels(labels)
	if err := s.Client.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("stamping ownership on %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

func (s *Service) stampBlueprintOwnership(ctx context.Context, blueprintID string, generation int64, runID string, st parsedStack) error {
	if blueprintID == "" {
		return nil
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return nil
	}
	// A fenced run must not restamp ownership after abandon, disconnect, or a
	// newer admission took authority (w8/m37 t003 + w8/m39): the stamp lands
	// only while the admitted (generation, runID) claim still owns the row.
	if generation != 0 && runID != "" && s.Blueprints != nil {
		if err := s.Blueprints.AssertBlueprintExecution(ctx, blueprintID, tenantID, generation, runID); err != nil {
			return errBlueprintExecutionLost()
		}
	} else if generation != 0 && s.Blueprints != nil {
		current, err := s.Blueprints.GetBlueprint(ctx, blueprintID, tenantID)
		if err != nil {
			return errBlueprintExecutionLost()
		}
		if current.ExecutionGeneration != generation {
			return errBlueprintExecutionLost()
		}
	}

	req, _ := ctx.Value(deployAuthorityKey{}).(DeployRequest)
	expected := takeoverExpectedOwner(req.Confirm)

	claimAndLabel := func(kind, name string, obj client.Object) error {
		if s.Blueprints != nil {
			if err := s.Blueprints.ClaimBlueprintResource(ctx, tenantID, kind, name, blueprintID, expected); err != nil {
				if errors.Is(err, store.ErrBlueprintResourceConflict) {
					owner, _ := s.Blueprints.GetBlueprintResourceOwner(ctx, tenantID, kind, name)
					if owner == "" {
						owner = "another blueprint"
					}
					return blueprintOwnershipError(blueprintOwnershipConflict{kind: kind, name: name, owner: owner})
				}
				return fmt.Errorf("claiming Blueprint ownership of %s %q: %w", kind, name, err)
			}
		}
		return s.labelBlueprintOwnership(ctx, blueprintID, obj)
	}

	var apps appv1alpha1.AppList
	if err := s.Client.List(ctx, &apps, client.MatchingLabels{core.LabelTenant: tenantID}); err != nil {
		return fmt.Errorf("listing apps for ownership stamp: %w", err)
	}
	wantedSvc := map[string]bool{}
	for _, svc := range st.services {
		wantedSvc[svc.req.Name] = true
	}
	for i := range apps.Items {
		if wantedSvc[appServiceName(&apps.Items[i])] {
			if err := claimAndLabel("service", appServiceName(&apps.Items[i]), &apps.Items[i]); err != nil {
				return err
			}
		}
	}
	if databases, err := s.listWorkspaceDatabases(ctx, tenantID); err != nil {
		return fmt.Errorf("listing databases for ownership stamp: %w", err)
	} else {
		wanted := map[string]bool{}
		for _, db := range st.databases {
			wanted[db.name] = true
		}
		for i := range databases.Items {
			if wanted[databases.Items[i].Spec.Name] {
				if err := claimAndLabel("database", databases.Items[i].Spec.Name, &databases.Items[i]); err != nil {
					return err
				}
			}
		}
	}
	if keyValues, err := s.listWorkspaceKeyValues(ctx, tenantID); err != nil {
		return fmt.Errorf("listing key values for ownership stamp: %w", err)
	} else {
		wanted := map[string]bool{}
		for _, kv := range st.keyValues {
			wanted[kv.name] = true
		}
		for i := range keyValues.Items {
			if wanted[keyValues.Items[i].Spec.Name] {
				if err := claimAndLabel("key_value", keyValues.Items[i].Spec.Name, &keyValues.Items[i]); err != nil {
					return err
				}
			}
		}
	}
	return s.releaseUndeclaredClaims(ctx, tenantID, blueprintID, st)
}

// releaseUndeclaredClaims drops this blueprint's claim on every resource its
// manifest no longer declares (w4/m125).
//
// w8/m23 promised that a managed resource "records A as its managing blueprint,
// visible on the blueprint's resources[]". Half of that held: the claim was
// durable, but resources[] is derived from the CURRENT manifest
// (resolveBlueprintResourcesFromIR walks the IR), so anything a sync stopped
// declaring vanished from the list while keeping the claim. Live, one blueprint
// id said both "I do not manage static-site" and "static-site is managed by me"
// at the same moment, and the only way to free it was to disconnect the
// surviving blueprint — a workflow nobody would find.
//
// The release frees the claim and the marker, nothing else: the resource keeps
// running, which is deliberate (w4/119 — an orphan is not deleted). It runs
// inside stampBlueprintOwnership's fenced window, after the same
// AssertBlueprintExecution the stamps use, so a retired worker cannot release
// claims a successor just took.
func (s *Service) releaseUndeclaredClaims(ctx context.Context, tenantID, blueprintID string, st parsedStack) error {
	if s.Blueprints == nil || tenantID == "" || blueprintID == "" {
		return nil
	}
	claims, err := s.Blueprints.ListBlueprintResourceClaims(ctx, tenantID, blueprintID)
	if err != nil {
		return fmt.Errorf("listing Blueprint resource claims: %w", err)
	}
	declared := blueprintDeclaredClaims(st)
	released := map[string]bool{}
	for _, c := range claims {
		if declared[c.Kind+"/"+c.Name] {
			continue
		}
		if err := s.Blueprints.ReleaseBlueprintResourceClaim(ctx, tenantID, c.Kind, c.Name, blueprintID); err != nil {
			return fmt.Errorf("releasing Blueprint claim on %s %q: %w", c.Kind, c.Name, err)
		}
		released[c.Kind+"/"+c.Name] = true
	}
	if len(released) == 0 {
		return nil
	}
	return s.clearBlueprintMarkers(ctx, tenantID, blueprintID, released)
}

// clearBlueprintMarkers removes core.LabelBlueprint from the resources named in
// released — the CR-label mirror of the claims releaseUndeclaredClaims just
// dropped. The claim is the writer of truth (replicas coordinate through it),
// so a marker that outlives its claim is stale rather than authoritative; the
// first failure is returned so a sync reports the incomplete cleanup instead of
// inferring success, the same posture clearBlueprintOwnership takes.
func (s *Service) clearBlueprintMarkers(ctx context.Context, tenantID, blueprintID string, released map[string]bool) error {
	owned := []client.ListOption{client.MatchingLabels{core.LabelTenant: tenantID, core.LabelBlueprint: blueprintID}}
	var firstErr error
	clear := func(key string, obj client.Object) {
		if !released[key] {
			return
		}
		base := obj.DeepCopyObject().(client.Object)
		labels := obj.GetLabels()
		if labels == nil {
			return
		}
		delete(labels, core.LabelBlueprint)
		obj.SetLabels(labels)
		if err := s.Client.Patch(ctx, obj, client.MergeFrom(base)); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("clearing Blueprint marker on %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	var apps appv1alpha1.AppList
	if err := s.Client.List(ctx, &apps, owned...); err != nil {
		return fmt.Errorf("listing owned apps: %w", err)
	}
	for i := range apps.Items {
		clear("service/"+appServiceName(&apps.Items[i]), &apps.Items[i])
	}
	var databases appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &databases, owned...); err != nil {
		return fmt.Errorf("listing owned databases: %w", err)
	}
	for i := range databases.Items {
		clear("database/"+databases.Items[i].Spec.Name, &databases.Items[i])
	}
	var keyValues appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &keyValues, owned...); err != nil {
		return fmt.Errorf("listing owned key values: %w", err)
	}
	for i := range keyValues.Items {
		clear("key_value/"+keyValues.Items[i].Spec.Name, &keyValues.Items[i])
	}
	return firstErr
}

// clearBlueprintOwnership removes the marker from every resource the
// disconnected blueprint owned — they become unmanaged (Render disconnect
// semantics: resources survive, management ends). It continues past individual
// patch failures and returns the first one: disconnect reports cleanup failure
// instead of inferring success from a discarded error (w8/m37 t003). A failed
// clear leaves the row disconnected — the markers are inert without an owning
// row, and an explicit re-creation adopts or takes them over through the
// normal ownership flow.
func (s *Service) clearBlueprintOwnership(ctx context.Context, tenantID, blueprintID string) error {
	owned := []client.ListOption{client.MatchingLabels{core.LabelTenant: tenantID, core.LabelBlueprint: blueprintID}}
	var firstErr error
	fail := func(format string, args ...any) {
		err := fmt.Errorf(format, args...)
		log.Printf("blueprint %s: %v", blueprintID, err)
		if firstErr == nil {
			firstErr = err
		}
	}

	clear := func(obj client.Object) {
		base := obj.DeepCopyObject().(client.Object)
		labels := obj.GetLabels()
		if labels == nil {
			return
		}
		delete(labels, core.LabelBlueprint)
		obj.SetLabels(labels)
		if err := s.Client.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
			fail("clearing ownership on %s/%s: %v", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	var apps appv1alpha1.AppList
	if err := s.Client.List(ctx, &apps, owned...); err != nil {
		fail("listing owned apps: %v", err)
	} else {
		for i := range apps.Items {
			clear(&apps.Items[i])
		}
	}
	var databases appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &databases, owned...); err != nil {
		fail("listing owned databases: %v", err)
	} else {
		for i := range databases.Items {
			clear(&databases.Items[i])
		}
	}
	var keyValues appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &keyValues, owned...); err != nil {
		fail("listing owned key values: %v", err)
	} else {
		for i := range keyValues.Items {
			clear(&keyValues.Items[i])
		}
	}
	return firstErr
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

// previewConnectionConflict reports, as a validation entry, that a live
// blueprint already tracks this repo+branch — the row-level twin of
// previewOwnershipConflicts (w4/m125). forBlueprintID is the previewing
// blueprint's own id; the pre-sync dialog passes it and never conflicts with
// itself, while `/blueprints/new` passes "" and sees the conflict before it
// presses Deploy. A store failure is swallowed the same way the resource
// preview swallows a scan failure: CreateBlueprint's own check is the
// enforcement point and fails closed there.
func (s *Service) previewConnectionConflict(ctx context.Context, tenantID, repo, branch, path, forBlueprintID string) *BlueprintValidationError {
	if s.Blueprints == nil || tenantID == "" {
		return nil
	}
	existing, err := s.blueprintConnectionConflict(ctx, tenantID, repo, branch, path)
	if err != nil || existing.ID == "" || existing.ID == forBlueprintID {
		return nil
	}
	return &BlueprintValidationError{
		Code:  "BLUEPRINT_CONNECTION_CONFLICT",
		Error: errBlueprintAlreadyConnected(existing).Error(),
	}
}

// validateWorkspaceReferences runs the workspace-resolvable reference checks at
// VALIDATE time (w4/118) — fromDatabase, and fromService pointing at a Key
// Value. Both are resolved by name against the caller's workspace, so unlike
// the same-file-only fromService→service check they cannot be answered from the
// manifest alone; they were therefore checked only at apply, and a manifest
// referencing a database that exists nowhere validated clean.
//
// This is the same resolution the apply path runs, reusing its errors verbatim
// — including the ambiguous-name case — so validate and apply cannot disagree
// about a reference. It reads CR identities only, never a credential value,
// which is why it needs no role beyond the one validate already holds.
//
// A caller with no resolved workspace (the store-less dev path) is skipped: it
// has no workspace for a name to resolve against, and failing there would be an
// answer about the environment rather than the manifest.
func (s *Service) validateWorkspaceReferences(ctx context.Context, source *BlueprintSource, ir BlueprintIR, st parsedStack) []BlueprintValidationError {
	if s.Client == nil {
		return nil
	}
	if _, scoped := s.Tenant(ctx); !scoped {
		return nil
	}
	if _, _, err := s.resolveExistingBlueprintReferences(ctx, st, nil, nil); err != nil {
		if !errors.Is(err, core.ErrBadRequest) && !errors.Is(err, core.ErrConflict) {
			// A cluster read failure is not a manifest problem; the apply path
			// is still the enforcement point.
			return nil
		}
		msg := err.Error()
		for _, prefix := range []string{"bad request: ", "conflict: "} {
			if after, ok := strings.CutPrefix(msg, prefix); ok {
				msg = after
				break
			}
		}
		if pointer := blueprintReferencePointer(ir, msg); pointer != "" {
			return []BlueprintValidationError{blueprintLocatedError(source, msg, pointer)}
		}
		return []BlueprintValidationError{blueprintValidationError(ir, msg)}
	}
	return nil
}

// blueprintReferencePointer finds the first service envVars entry whose
// fromDatabase/fromService names the resource the message quotes — the
// resolver's error names the target, not the service that referenced it.
func blueprintReferencePointer(ir BlueprintIR, msg string) string {
	for _, resource := range ir.Resources {
		if resource.Kind != BlueprintResourceService {
			continue
		}
		envVars, _ := resource.Fields["envVars"].Value.([]any)
		for i, raw := range envVars {
			env, _ := raw.(map[string]any)
			for _, key := range []string{"fromDatabase", "fromService"} {
				ref, _ := env[key].(map[string]any)
				if name, _ := ref["name"].(string); name != "" && strings.Contains(msg, fmt.Sprintf("%q", name)) {
					return fmt.Sprintf("%s/envVars/%d", resource.SourcePath, i)
				}
			}
		}
	}
	return ""
}

func blueprintDeclaredClaims(st parsedStack) map[string]bool {
	declared := map[string]bool{}
	for _, svc := range st.services {
		declared["service/"+svc.req.Name] = true
	}
	for _, db := range st.databases {
		declared["database/"+db.name] = true
	}
	for _, kv := range st.keyValues {
		declared["key_value/"+kv.name] = true
	}
	return declared
}
