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

package billing

import (
	"context"
	"fmt"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const BillingEnforcementAnnotation = "billing.bex.co/enforcement"

type EnforcementStore interface {
	EnsureBillingEnforcement(context.Context, store.BillingEnforcement) (store.BillingEnforcement, error)
	ListActiveBillingEnforcements(context.Context, string) ([]store.BillingEnforcement, error)
	MarkBillingEnforcementRecovered(context.Context, string, string, string, time.Time) error
	SetAppSuspended(context.Context, string, bool) error
}

type ResourceEnforcer interface {
	Enforce(context.Context, store.BillingLifecycle) error
	Recover(context.Context, store.BillingLifecycle) error
}

// KubernetesEnforcer applies ordinary suspended intent to tenant CRs. The
// operator stays billing-free; it only reconciles the same spec fields used by
// the public lifecycle verbs. Marker-before-mutation closes crash gaps.
type KubernetesEnforcer struct {
	Client    client.Client
	Store     EnforcementStore
	Namespace string
	Clock     func() time.Time
	// OpsWorkspaceID pins the platform ops workspace (BEX_OPS_WORKSPACE,
	// docs/ADR087-platform-observability-ui.md §4): dunning enforcement refuses
	// to suspend its resources — workspace suspension is exactly what would take
	// every operator's Grafana access down at once — so its lifecycle row fails
	// loudly (core.CodeOpsWorkspaceProtected, retried on the worker's backoff)
	// instead of converging to enforced. Recover stays allowed: un-suspending
	// the ops workspace is always safe. Empty (unset) => no exemption.
	OpsWorkspaceID string
}

// appNamespace is where a workspace's App CRs live: its own `<ws>` namespace
// under per-tenant isolation (ADR043). Databases/KeyValues also moved to
// tenant namespaces under ADR043, so enforcement must be cluster-wide.
func (e *KubernetesEnforcer) appNamespace(workspaceID string) string {
	if workspaceID != "" {
		return workspaceID
	}
	return e.Namespace
}

func (e *KubernetesEnforcer) now() time.Time {
	if e.Clock != nil {
		return e.Clock().UTC()
	}
	return time.Now().UTC()
}

func (e *KubernetesEnforcer) marker(state store.BillingLifecycle) string {
	return state.WorkspaceID + ":" + strconv.FormatInt(state.TransitionVersion, 10)
}

func (e *KubernetesEnforcer) Enforce(ctx context.Context, state store.BillingLifecycle) error {
	if e == nil || e.Client == nil || e.Store == nil {
		return fmt.Errorf("billing resource enforcer unavailable")
	}
	if e.OpsWorkspaceID != "" && state.WorkspaceID == e.OpsWorkspaceID {
		return core.NewOpsWorkspaceProtectedError()
	}
	marker := e.marker(state)
	var apps appv1alpha1.AppList
	if err := e.Client.List(ctx, &apps, client.InNamespace(e.appNamespace(state.WorkspaceID)), client.MatchingLabels{core.LabelTenant: state.WorkspaceID}); err != nil {
		return err
	}
	for i := range apps.Items {
		a := &apps.Items[i]
		if a.Spec.Type == appv1alpha1.TypeStaticSite {
			continue
		}
		if err := e.enforceOne(ctx, a, store.ResourceKindService, state.WorkspaceID, marker); err != nil {
			return err
		}
	}
	var databases appv1alpha1.DatabaseList
	// List cluster-wide to handle both tenant-namespace (ADR043) and legacy shared namespace resources
	if err := e.Client.List(ctx, &databases, client.MatchingLabels{core.LabelTenant: state.WorkspaceID}); err != nil {
		return err
	}
	for i := range databases.Items {
		if err := e.enforceOne(ctx, &databases.Items[i], store.ResourceKindPostgres, state.WorkspaceID, marker); err != nil {
			return err
		}
	}
	var keyvalues appv1alpha1.KeyValueList
	// List cluster-wide to handle both tenant-namespace (ADR043) and legacy shared namespace resources
	if err := e.Client.List(ctx, &keyvalues, client.MatchingLabels{core.LabelTenant: state.WorkspaceID}); err != nil {
		return err
	}
	for i := range keyvalues.Items {
		if err := e.enforceOne(ctx, &keyvalues.Items[i], store.ResourceKindKeyValue, state.WorkspaceID, marker); err != nil {
			return err
		}
	}
	return nil
}

// suspendedIntent and setSuspended are the one place the enforceable-kind type
// switch lives; enforce and recover both read and write Spec.Suspended through
// them, so a fourth CR kind is added in exactly two places.
func suspendedIntent(obj client.Object) bool {
	switch v := obj.(type) {
	case *appv1alpha1.App:
		return v.Spec.Suspended
	case *appv1alpha1.Database:
		return v.Spec.Suspended
	case *appv1alpha1.KeyValue:
		return v.Spec.Suspended
	}
	return false
}

func setSuspended(obj client.Object, suspended bool) {
	switch v := obj.(type) {
	case *appv1alpha1.App:
		v.Spec.Suspended = suspended
	case *appv1alpha1.Database:
		v.Spec.Suspended = suspended
	case *appv1alpha1.KeyValue:
		v.Spec.Suspended = suspended
	}
}

// enforceOne applies suspended intent to one CR under the marker protocol: a
// resource the tenant or operator suspended themselves carries no marker and is
// left alone, one already enforced under the current marker is a no-op, and the
// marker is stamped in the SAME patch as the intent so a crash between them
// cannot orphan either. recoverOne is its exact inverse.
func (e *KubernetesEnforcer) enforceOne(ctx context.Context, obj client.Object, kind, workspaceID, marker string) error {
	suspended := suspendedIntent(obj)
	owned := obj.GetAnnotations()[BillingEnforcementAnnotation]
	if suspended && owned == "" {
		return nil // user/operator-owned
	}
	entry, err := e.Store.EnsureBillingEnforcement(ctx, store.BillingEnforcement{
		WorkspaceID: workspaceID, ResourceKind: kind, ResourceName: obj.GetName(), MarkerID: marker,
	})
	if err != nil {
		return err
	}
	if suspended && owned == entry.MarkerID {
		return nil
	}
	// An App additionally carries its suspended state as a control-plane row.
	if a, ok := obj.(*appv1alpha1.App); ok {
		if id := a.Labels[store.LabelAppID]; a.Labels[store.LabelManagedBy] == store.ManagedByValue && id != "" {
			if err := e.Store.SetAppSuspended(ctx, id, true); err != nil {
				return fmt.Errorf("suspend App row %s: %w", a.Name, err)
			}
		}
	}
	base := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[BillingEnforcementAnnotation] = entry.MarkerID
	obj.SetAnnotations(annotations)
	setSuspended(obj, true)
	return e.Client.Patch(ctx, obj, base)
}

func (e *KubernetesEnforcer) Recover(ctx context.Context, state store.BillingLifecycle) error {
	if e == nil || e.Client == nil || e.Store == nil {
		return fmt.Errorf("billing resource enforcer unavailable")
	}
	entries, err := e.Store.ListActiveBillingEnforcements(ctx, state.WorkspaceID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := e.recoverOne(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// firstNamed returns the named item from a typed CR list as a client.Object, or
// a nil interface when absent — the shared tail of the datastore kinds, which
// are looked up by a labelled cluster-wide List rather than by namespaced key.
func firstNamed[T any, PT interface {
	*T
	client.Object
}](items []T, name string) client.Object {
	for i := range items {
		if item := PT(&items[i]); item.GetName() == name {
			return item
		}
	}
	return nil
}

func (e *KubernetesEnforcer) recoverOne(ctx context.Context, entry store.BillingEnforcement) error {
	markRecovered := func() error {
		return e.Store.MarkBillingEnforcementRecovered(ctx, entry.WorkspaceID, entry.ResourceKind, entry.ResourceName, e.now())
	}
	var obj client.Object
	switch entry.ResourceKind {
	case store.ResourceKindService:
		// Apps use per-tenant namespaces under ADR043, so a namespaced key resolves them.
		app := &appv1alpha1.App{}
		key := client.ObjectKey{Namespace: e.appNamespace(entry.WorkspaceID), Name: entry.ResourceName}
		if err := e.Client.Get(ctx, key, app); err != nil {
			if apierrors.IsNotFound(err) {
				return markRecovered()
			}
			return err
		}
		obj = app
	case store.ResourceKindPostgres:
		// Databases moved to per-tenant namespaces under ADR043; search
		// cluster-wide to handle both tenant-namespace and legacy resources.
		var dbs appv1alpha1.DatabaseList
		if err := e.Client.List(ctx, &dbs, client.MatchingLabels{core.LabelTenant: entry.WorkspaceID}); err != nil {
			return err
		}
		obj = firstNamed(dbs.Items, entry.ResourceName)
	case store.ResourceKindKeyValue:
		// KeyValues moved to per-tenant namespaces under ADR043; same cluster-wide search.
		var kvs appv1alpha1.KeyValueList
		if err := e.Client.List(ctx, &kvs, client.MatchingLabels{core.LabelTenant: entry.WorkspaceID}); err != nil {
			return err
		}
		obj = firstNamed(kvs.Items, entry.ResourceName)
	default:
		return fmt.Errorf("unknown billing resource kind %q", entry.ResourceKind)
	}
	if obj == nil {
		return markRecovered()
	}
	// Losing/replacing the exact annotation is an explicit independent operator
	// action. Release the marker without overriding current intent.
	if obj.GetAnnotations()[BillingEnforcementAnnotation] != entry.MarkerID {
		return markRecovered()
	}
	base := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	annotations := obj.GetAnnotations()
	delete(annotations, BillingEnforcementAnnotation)
	obj.SetAnnotations(annotations)
	// An App additionally carries its suspended state as a control-plane row.
	if a, ok := obj.(*appv1alpha1.App); ok {
		if id := a.Labels[store.LabelAppID]; a.Labels[store.LabelManagedBy] == store.ManagedByValue && id != "" {
			if err := e.Store.SetAppSuspended(ctx, id, false); err != nil {
				return err
			}
		}
	}
	setSuspended(obj, false)
	if err := e.Client.Patch(ctx, obj, base); err != nil {
		return err
	}
	return markRecovered()
}
