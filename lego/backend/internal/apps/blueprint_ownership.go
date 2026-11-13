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
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// BlueprintTakeoverConfirmation is the server-issued phrase that authorizes
// transferring a resource's blueprint ownership. Computed from the OWNING
// blueprint id so the caller confirms exactly what they are taking over from.
// The phrase names the public exs- id (w2/m269); takeoverExpectedOwner reads
// it, or a phrase naming the durable blp- id, back to the stored id.
func BlueprintTakeoverConfirmation(owningBlueprintID string) string {
	return takeoverPhrasePrefix + ids.BlueprintPublicID(owningBlueprintID)
}

const takeoverPhrasePrefix = "takeover blueprint "

// blueprintOwnershipConflict is one resource owned by another blueprint.
type blueprintOwnershipConflict struct {
	kind  store.BlueprintClaimKind
	name  string
	owner string // owning blueprint id
}

// blueprintClaimResourceKind is a claim kind's manifest kind, as plan actions
// name it: a claim calls a Postgres "database". Any other kind (the CHECK
// admits none) passes through verbatim, so no resolver finds it.
func blueprintClaimResourceKind(kind store.BlueprintClaimKind) BlueprintResourceKind {
	switch kind {
	case store.BlueprintClaimService:
		return BlueprintResourceService
	case store.BlueprintClaimDatabase:
		return BlueprintResourcePostgres
	case store.BlueprintClaimKeyValue:
		return BlueprintResourceKeyValue
	case store.BlueprintClaimBuildSource:
		return BlueprintResourceBuildSource
	}
	return BlueprintResourceKind(kind)
}

// blueprintResourceOwners reads the workspace's durable claims (w8/m40) once
// for a whole manifest (w5/114). A failed read, like an unwired store, returns
// nil and the CR labels decide; the claim each write takes still enforces
// ownership.
func (s *Service) blueprintResourceOwners(ctx context.Context, tenantID string) map[store.BlueprintResourceKey]string {
	if s.Blueprints == nil {
		return nil
	}
	owners, err := s.Blueprints.BlueprintResourceOwners(ctx, tenantID)
	if err != nil {
		return nil
	}
	return owners
}

// blueprintResourceOwner prefers the durable claim and falls back to the CR
// label.
func blueprintResourceOwner(owners map[store.BlueprintResourceKey]string, kind store.BlueprintClaimKind, name, labelOwner string) string {
	if owner := owners[store.BlueprintResourceKey{Kind: kind, Name: name}]; owner != "" {
		return owner
	}
	return labelOwner
}

// blueprintOwnershipConflicts lists the parsed stack's resources that a
// DIFFERENT blueprint currently owns. Prefers the durable claim table when
// wired (w8/m40); falls back to CR labels. selfID "" means "no blueprint
// identity" (a bare validate): every owned resource conflicts. It reads snap's
// workspace.
func (s *Service) blueprintOwnershipConflicts(ctx context.Context, selfID string, st parsedStack, snap *workspaceSnapshot) ([]blueprintOwnershipConflict, error) {
	tenantID := snap.tenantID
	if len(st.services) == 0 && len(st.databases) == 0 && len(st.keyValues) == 0 && len(st.buildSources) == 0 {
		return nil, nil
	}
	owners := s.blueprintResourceOwners(ctx, tenantID)
	var conflicts []blueprintOwnershipConflict
	record := func(kind store.BlueprintClaimKind, name, labelOwner string) {
		if owner := blueprintResourceOwner(owners, kind, name, labelOwner); owner != "" && owner != selfID {
			conflicts = append(conflicts, blueprintOwnershipConflict{kind: kind, name: name, owner: owner})
		}
	}

	if len(st.services) > 0 {
		// The service the apply updates decides the owner, not a stale twin
		// left in the shared namespace (w5/m133).
		services, err := snap.services(ctx)
		if err != nil {
			return nil, err
		}
		for _, svc := range st.services {
			var labelOwner string
			if app := services[svc.req.Name]; app != nil {
				labelOwner = app.Labels[core.LabelBlueprint]
			}
			record(store.BlueprintClaimService, svc.req.Name, labelOwner)
		}
	}
	if len(st.databases) > 0 {
		databases, err := snap.databases(ctx)
		if err != nil {
			return nil, err
		}
		byName := map[string]string{}
		for i := range databases.Items {
			byName[databases.Items[i].Spec.Name] = databases.Items[i].Labels[core.LabelBlueprint]
		}
		for _, db := range st.databases {
			record(store.BlueprintClaimDatabase, db.name, byName[db.name])
		}
	}
	if len(st.keyValues) > 0 {
		keyValues, err := snap.keyValues(ctx)
		if err != nil {
			return nil, err
		}
		byName := map[string]string{}
		for i := range keyValues.Items {
			byName[keyValues.Items[i].Spec.Name] = keyValues.Items[i].Labels[core.LabelBlueprint]
		}
		for _, kv := range st.keyValues {
			record(store.BlueprintClaimKeyValue, kv.name, byName[kv.name])
		}
	}
	if len(st.buildSources) > 0 {
		carriers, err := snap.buildSources(ctx)
		if err != nil {
			return nil, err
		}
		for _, b := range st.buildSources {
			var labelOwner string
			if carrier := carriers[b.name]; carrier != nil {
				labelOwner = carrier.Labels[core.LabelBlueprint]
			}
			record(store.BlueprintClaimBuildSource, b.name, labelOwner)
		}
	}
	return conflicts, nil
}

// preflightBlueprintOwnership refuses a blueprint apply that would overwrite
// another blueprint's resources — before any write — unless the request
// carries the exact takeover confirmation, which transfers ownership (the
// post-apply stamp rewrites the label). Non-blueprint deploys are exempt.
func (s *Service) preflightBlueprintOwnership(ctx context.Context, req DeployRequest, st parsedStack, snap *workspaceSnapshot) error {
	if req.BlueprintID == "" {
		return nil
	}
	return s.refuseUnconfirmedTakeover(ctx, req.BlueprintID, req.Confirm, st, snap)
}

// refuseUnconfirmedTakeover is preflightBlueprintOwnership's rule for the
// Blueprint selfID, "" for one that does not exist yet (every owner is then
// another Blueprint). A one-step create asks it before admission, so a create
// that would take another Blueprint's resource is refused with nothing
// recorded, not left behind as a Blueprint in error that every push re-runs
// into the same refusal (w8/m153).
func (s *Service) refuseUnconfirmedTakeover(ctx context.Context, selfID, confirm string, st parsedStack, snap *workspaceSnapshot) error {
	if _, ok := s.Tenant(ctx); !ok {
		return nil
	}
	conflicts, err := s.blueprintOwnershipConflicts(ctx, selfID, st, snap)
	if err != nil {
		return fmt.Errorf("checking Blueprint resource ownership: %w", err)
	}
	if len(conflicts) == 0 {
		return nil
	}
	first := conflicts[0]
	if takeoverExpectedOwner(confirm) == first.owner {
		// Explicit takeover of first.owner's resources: proceed — but only
		// for THAT owner. A stack conflicting with two different blueprints
		// resolves one owner per confirmation.
		for _, c := range conflicts {
			if c.owner != first.owner {
				return s.ownershipRefusal(ctx, c)
			}
		}
		return nil
	}
	return s.ownershipRefusal(ctx, first)
}

// plannedApplyKey marks a context as the apply of an approved planned sync.
type plannedApplyKey struct{}

// withPlannedApply marks ctx as applying a planned sync that was approved.
// Render's approve carries no `confirm`, so an ownership refusal met there is
// worded without the phrase (ownershipRefusal).
func withPlannedApply(ctx context.Context) context.Context {
	return context.WithValue(ctx, plannedApplyKey{}, true)
}

// ownershipRefusal refuses a resource another Blueprint manages, worded for
// the surface the apply came from: with the takeover phrase where `confirm`
// is accepted, and without it for an approved planned sync (w8/251).
func (s *Service) ownershipRefusal(ctx context.Context, c blueprintOwnershipConflict) error {
	if planned, _ := ctx.Value(plannedApplyKey{}).(bool); !planned {
		return blueprintOwnershipError(c)
	}
	return core.NewConflictError("BLUEPRINT_RESOURCE_CONFLICT", s.plannedOwnershipNotice(ctx, c, map[string]string{}),
		map[string]any{"resource": c.name, "kind": string(blueprintClaimResourceKind(c.kind)), "owningBlueprintId": ids.BlueprintPublicID(c.owner)})
}

// blueprintOwnershipError refuses a resource another blueprint manages, for
// an apply and for the preview's validation entries. Its kind param is the
// manifest kind a client labels it by (w5/098), mapped from the claim kind; the
// message keeps the wording it has always had.
func blueprintOwnershipError(c blueprintOwnershipConflict) error {
	phrase := BlueprintTakeoverConfirmation(c.owner)
	public := ids.BlueprintPublicID(c.owner)
	kind := blueprintClaimResourceKind(c.kind)
	return core.NewConflictError("BLUEPRINT_RESOURCE_CONFLICT",
		fmt.Sprintf("%s %q is managed by blueprint %s; retry with confirm=%q to transfer ownership to this blueprint", blueprintOwnershipWording(kind), c.name, public, phrase),
		map[string]any{"resource": c.name, "kind": string(kind), "owningBlueprintId": public, "confirm": phrase})
}

// takeoverExpectedOwner returns the owning blueprint's durable (blp-) id
// encoded in Confirm, or "" when Confirm is not a takeover phrase. Either
// spelling of the id confirms the same owner.
func takeoverExpectedOwner(confirm string) string {
	if strings.HasPrefix(confirm, takeoverPhrasePrefix) {
		return ids.BlueprintStorageID(strings.TrimPrefix(confirm, takeoverPhrasePrefix))
	}
	return ""
}

// claimBlueprintResourceName takes the durable claim for one resource and
// stamps the CR label. expectedOwner comes from an explicit takeover Confirm;
// empty means adopt-or-keep-mine only.
func (s *Service) claimBlueprintResourceName(ctx context.Context, kind store.BlueprintClaimKind, name string) error {
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
	return s.claimBlueprintResource(ctx, tenantID, kind, name, req.BlueprintID, takeoverExpectedOwner(req.Confirm))
}

// claimBlueprintResource takes the durable claim on one resource for
// blueprintID, refusing with the owning blueprint when another holds it.
func (s *Service) claimBlueprintResource(ctx context.Context, tenantID string, kind store.BlueprintClaimKind, name, blueprintID, expectedOwner string) error {
	err := s.Blueprints.ClaimBlueprintResource(ctx, tenantID, kind, name, blueprintID, expectedOwner)
	if errors.Is(err, store.ErrBlueprintResourceConflict) {
		owner, _ := s.Blueprints.GetBlueprintResourceOwner(ctx, tenantID, kind, name)
		if owner == "" {
			owner = "another blueprint"
		}
		return s.ownershipRefusal(ctx, blueprintOwnershipConflict{kind: kind, name: name, owner: owner})
	}
	if err != nil {
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

// stampBlueprintOwnership records req.BlueprintID on every resource the apply
// converged — create, adopt, and takeover all land here as a fail-loud
// backfill for no-op short circuits that already claimed at write time
// (w8/m40). A failed claim or label patch fails the sync: ownership
// persistence failure cannot look like success.
//
// It is the apply's last write, so it reads the workspace afresh through snap
// and stamps the copy of each service the apply resolved. Its label patches
// land on the objects it read, so a reader of snap after a successful stamp
// sees the declared resources as the apply left them; a failed patch fails the
// apply (w5/141).
func (s *Service) stampBlueprintOwnership(ctx context.Context, blueprintID string, generation int64, runID string, st parsedStack, snap *workspaceSnapshot) error {
	snap.forget()
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

	claimAndLabel := func(kind store.BlueprintClaimKind, name string, obj client.Object) error {
		if s.Blueprints != nil {
			if err := s.claimBlueprintResource(ctx, tenantID, kind, name, blueprintID, expected); err != nil {
				return err
			}
		}
		return s.labelBlueprintOwnership(ctx, blueprintID, obj)
	}

	// A kind the manifest does not declare has nothing to stamp, so it is not
	// listed. Each declared service is the one the apply resolved and wrote.
	if len(st.services) > 0 {
		services, err := snap.services(ctx)
		if err != nil {
			return fmt.Errorf("resolving services for ownership stamp: %w", err)
		}
		for _, svc := range st.services {
			if app := services[svc.req.Name]; app != nil {
				if err := claimAndLabel(store.BlueprintClaimService, svc.req.Name, app); err != nil {
					return err
				}
			}
		}
	}
	if len(st.databases) > 0 {
		databases, err := snap.databases(ctx)
		if err != nil {
			return fmt.Errorf("listing databases for ownership stamp: %w", err)
		}
		wanted := map[string]bool{}
		for _, db := range st.databases {
			wanted[db.name] = true
		}
		for i := range databases.Items {
			if wanted[databases.Items[i].Spec.Name] {
				if err := claimAndLabel(store.BlueprintClaimDatabase, databases.Items[i].Spec.Name, &databases.Items[i]); err != nil {
					return err
				}
			}
		}
	}
	if len(st.keyValues) > 0 {
		keyValues, err := snap.keyValues(ctx)
		if err != nil {
			return fmt.Errorf("listing key values for ownership stamp: %w", err)
		}
		wanted := map[string]bool{}
		for _, kv := range st.keyValues {
			wanted[kv.name] = true
		}
		for i := range keyValues.Items {
			if wanted[keyValues.Items[i].Spec.Name] {
				if err := claimAndLabel(store.BlueprintClaimKeyValue, keyValues.Items[i].Spec.Name, &keyValues.Items[i]); err != nil {
					return err
				}
			}
		}
	}
	if len(st.buildSources) > 0 {
		carriers, err := snap.buildSources(ctx)
		if err != nil {
			return fmt.Errorf("listing build sources for ownership stamp: %w", err)
		}
		for _, b := range st.buildSources {
			if carrier := carriers[b.name]; carrier != nil {
				if err := claimAndLabel(store.BlueprintClaimBuildSource, b.name, carrier); err != nil {
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
	released := map[store.BlueprintResourceKey]bool{}
	for _, c := range claims {
		if declared[c.Key()] {
			continue
		}
		if err := s.Blueprints.ReleaseBlueprintResourceClaim(ctx, tenantID, c.Kind, c.Name, blueprintID); err != nil {
			return fmt.Errorf("releasing Blueprint claim on %s %q: %w", c.Kind, c.Name, err)
		}
		released[c.Key()] = true
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
func (s *Service) clearBlueprintMarkers(ctx context.Context, tenantID, blueprintID string, released map[store.BlueprintResourceKey]bool) error {
	owned := []client.ListOption{client.MatchingLabels{core.LabelTenant: tenantID, core.LabelBlueprint: blueprintID}}
	var firstErr error
	clear := func(key store.BlueprintResourceKey, obj client.Object) {
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
		if core.IsBuildSource(&apps.Items[i]) {
			clear(store.BlueprintResourceKey{Kind: store.BlueprintClaimBuildSource, Name: buildSourceName(&apps.Items[i])}, &apps.Items[i])
		} else {
			clear(store.BlueprintResourceKey{Kind: store.BlueprintClaimService, Name: core.AppPublicName(&apps.Items[i])}, &apps.Items[i])
		}
	}
	var databases appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &databases, owned...); err != nil {
		return fmt.Errorf("listing owned databases: %w", err)
	}
	for i := range databases.Items {
		clear(store.BlueprintResourceKey{Kind: store.BlueprintClaimDatabase, Name: databases.Items[i].Spec.Name}, &databases.Items[i])
	}
	var keyValues appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &keyValues, owned...); err != nil {
		return fmt.Errorf("listing owned key values: %w", err)
	}
	for i := range keyValues.Items {
		clear(store.BlueprintResourceKey{Kind: store.BlueprintClaimKeyValue, Name: keyValues.Items[i].Spec.Name}, &keyValues.Items[i])
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
// entries for the preview surfaces. selfID is the Blueprint the preview is
// made for, whose own resources never conflict with it; "" is a Blueprint not
// yet created, which conflicts with any owner. The caller names it: several
// Blueprints may track one repo and branch. Scan failures are swallowed (the
// apply-path preflight is the enforcement point).
func (s *Service) previewOwnershipConflicts(ctx context.Context, selfID string, st parsedStack, snap *workspaceSnapshot) []BlueprintValidationError {
	if _, ok := s.Tenant(ctx); !ok {
		return nil
	}
	conflicts, err := s.blueprintOwnershipConflicts(ctx, selfID, st, snap)
	if err != nil {
		return nil
	}
	entries := make([]BlueprintValidationError, 0, len(conflicts))
	for _, c := range conflicts {
		entries = append(entries, BlueprintValidationError{
			Code:     "BLUEPRINT_RESOURCE_CONFLICT",
			Error:    blueprintOwnershipError(c).Error(),
			conflict: &c,
		})
	}
	return entries
}

// plannedOwnershipNotice words an ownership conflict for a planned sync.
// Render's create, plan and approve carry no `confirm`, and the pinned CLI has
// no flag for one, so the phrase blueprintOwnershipError offers cannot be
// sent from the surface that prints this (w8/251). Render: "Do not manage a
// particular resource with more than one Blueprint". It says what can be done
// instead: a copy in a new Blueprint, or a release by the Blueprint that
// manages the resource. owners remembers each managing Blueprint's name, so
// several conflicts with one Blueprint read it once.
func (s *Service) plannedOwnershipNotice(ctx context.Context, c blueprintOwnershipConflict, owners map[string]string) string {
	owner, known := owners[c.owner]
	if !known {
		owner = ids.BlueprintPublicID(c.owner)
		if b, err := s.Blueprints.GetBlueprint(ctx, c.owner, s.resolveTenantID(ctx)); err == nil && b.Name != "" {
			owner = fmt.Sprintf("%s (%q)", owner, b.Name)
		}
		owners[c.owner] = owner
	}
	wording := blueprintOwnershipWording(blueprintClaimResourceKind(c.kind))
	return fmt.Sprintf("%s %q is managed by blueprint %s, and a resource has one managing Blueprint. For a separate copy, create a new Blueprint with existingResources \"create_new\". To manage this %s here, remove it from that Blueprint's file and sync that Blueprint first, which releases it.",
		wording, c.name, owner, wording)
}

// blueprintOwnershipWording is the word a conflict's message uses for a
// resource of kind.
func blueprintOwnershipWording(kind BlueprintResourceKind) string {
	switch kind {
	case BlueprintResourcePostgres:
		return "database"
	case BlueprintResourceKeyValue:
		return "key value"
	case BlueprintResourceBuildSource:
		return "build source"
	default:
		return string(kind)
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
func (s *Service) validateWorkspaceReferences(ctx context.Context, source *BlueprintSource, ir BlueprintIR, st parsedStack, snap *workspaceSnapshot) []BlueprintValidationError {
	if s.Client == nil {
		return nil
	}
	if _, scoped := s.Tenant(ctx); !scoped {
		return nil
	}
	var out []BlueprintValidationError
	if _, _, err := s.resolveExistingBlueprintReferences(ctx, st, snap); err != nil {
		var refused blueprintResourceErrors
		if !errors.As(err, &refused) {
			// A cluster read failure is not a manifest problem; the apply path
			// is still the enforcement point.
			return nil
		}
		// One located entry per referencing envVars entry (w8/051): two
		// services naming the same missing database each get their own.
		for _, problem := range refused {
			msg := blueprintReferenceMessage(problem.err)
			var pointers []string
			switch problem.kind {
			case BlueprintResourcePostgres:
				pointers = blueprintReferencePointers(ir, "fromDatabase", problem.name)
			case BlueprintResourceKeyValue:
				pointers = blueprintReferencePointers(ir, "fromService", problem.name)
			default:
				if pointer := blueprintReferencePointer(ir, msg); pointer != "" {
					pointers = []string{pointer}
				}
			}
			if len(pointers) == 0 {
				out = append(out, blueprintValidationError(ir, msg))
			}
			for _, pointer := range pointers {
				out = append(out, blueprintLocatedError(source, msg, pointer))
			}
		}
	}
	// A read failure leaves the check to the apply, as above.
	services, err := snap.services(ctx)
	if err != nil {
		return out
	}
	return append(out, blueprintResourceValidationErrors(source, ir, hostReferenceProblems(st, services))...)
}

// hostReferenceProblems refuses, in document order, each fromService
// `property: host` reference to a target the manifest does not declare that
// is no existing workspace service (resolved by manifest name, as the apply
// resolves them, w5/m133), or an existing App the supplied type does not name
// or that has no network address (a static site's host is its public
// hostname, w8/303) — classifyRef's declared-sibling rules
// applied to the App's actual kind. Each refusal is at its authored envVars
// entry. Validate reports them all (w8/026); the apply refuses the first
// before any write (w4/206).
func hostReferenceProblems(st parsedStack, existing map[string]*appv1alpha1.App) blueprintResourceErrors {
	declared := make(map[string]bool, len(st.services))
	for _, svc := range st.services {
		declared[svc.req.Name] = true
	}
	var refused blueprintResourceErrors
	for _, svc := range st.services {
		for _, ref := range svc.hostRefs {
			if declared[ref.target] {
				continue // classifyRef checked the declared sibling
			}
			if err := existingHostTargetError(svc.req.Name, ref, existing[ref.target]); err != nil {
				refused = append(refused, blueprintResourceError{kind: BlueprintResourceService, name: svc.req.Name, err: atEnvEntry(ref.index, err)})
			}
		}
	}
	return refused
}

// existingHostTargetError is the refusal of a host reference to target, an
// existing workspace App (nil when none exists), or nil when it resolves.
func existingHostTargetError(service string, ref hostRef, target *appv1alpha1.App) error {
	if target == nil {
		return fmt.Errorf("%w: service %q: fromService references unknown service %q (declare it under services: or create it in this workspace first)", core.ErrBadRequest, service, ref.target)
	}
	if err := serviceRefTypeError(service, ref.key, ref.refType, ref.target, effectiveType(target.Spec.Type)); err != nil {
		return err
	}
	if takesStaticHost(target.Spec.Type, ref.property) {
		return nil // its public hostname, not a private address
	}
	if !target.Spec.InternallyAddressable() {
		return serviceRefUnaddressable(service, ref.key, serviceRefPropertyHost, ref.target)
	}
	if refersToPrivateAddress(ref.property) && !target.Spec.ReceivesPrivateTraffic() {
		return serviceRefFreePlan(service, ref.key, ref.property, ref.target)
	}
	return nil
}

// blueprintReferenceMessage is a resolver refusal without its transport
// sentinel prefix, as validation reports it.
func blueprintReferenceMessage(err error) string {
	msg := err.Error()
	for _, prefix := range []string{"bad request: ", "conflict: "} {
		if after, ok := strings.CutPrefix(msg, prefix); ok {
			return after
		}
	}
	return msg
}

// blueprintEnvRefPointers is every service envVars entry whose reference
// under key (fromDatabase or fromService) matches, in document order.
func blueprintEnvRefPointers(ir BlueprintIR, key string, match func(name, refType string) bool) []string {
	var pointers []string
	for _, resource := range ir.Resources {
		if resource.Kind != BlueprintResourceService {
			continue
		}
		envVars, _ := resource.Fields["envVars"].Value.([]any)
		for i, raw := range envVars {
			env, _ := raw.(map[string]any)
			ref, _ := env[key].(map[string]any)
			name, _ := ref["name"].(string)
			refType, _ := ref["type"].(string)
			if name != "" && match(name, refType) {
				pointers = append(pointers, fmt.Sprintf("%s/envVars/%d", resource.SourcePath, i))
			}
		}
	}
	return pointers
}

// blueprintReferencePointers is every service envVars entry whose key
// (fromDatabase, or a Key Value fromService) names target, in document order.
func blueprintReferencePointers(ir BlueprintIR, key, target string) []string {
	return blueprintEnvRefPointers(ir, key, func(name, refType string) bool {
		// A service reference that happens to share the name is not one.
		return name == target && (key != "fromService" || isKeyValueType(refType))
	})
}

// blueprintReferencePointer finds the first service fromService entry naming
// the service the message quotes; the resolver's error names the target, not
// the service that referenced it. Database and Key Value entries never match,
// so a resource sharing the service's name never takes the pointer (w1/126).
func blueprintReferencePointer(ir BlueprintIR, msg string) string {
	pointers := blueprintEnvRefPointers(ir, "fromService", func(name, refType string) bool {
		return !isKeyValueType(refType) && strings.Contains(msg, fmt.Sprintf("%q", name))
	})
	if len(pointers) == 0 {
		return ""
	}
	return pointers[0]
}

func blueprintDeclaredClaims(st parsedStack) map[store.BlueprintResourceKey]bool {
	declared := map[store.BlueprintResourceKey]bool{}
	for _, svc := range st.services {
		declared[store.BlueprintResourceKey{Kind: store.BlueprintClaimService, Name: svc.req.Name}] = true
	}
	for _, db := range st.databases {
		declared[store.BlueprintResourceKey{Kind: store.BlueprintClaimDatabase, Name: db.name}] = true
	}
	for _, kv := range st.keyValues {
		declared[store.BlueprintResourceKey{Kind: store.BlueprintClaimKeyValue, Name: kv.name}] = true
	}
	for _, b := range st.buildSources {
		declared[store.BlueprintResourceKey{Kind: store.BlueprintClaimBuildSource, Name: b.name}] = true
	}
	return declared
}
