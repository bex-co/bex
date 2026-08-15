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
		if err := e.enforceApp(ctx, a, state.WorkspaceID, marker); err != nil {
			return err
		}
	}
	var databases appv1alpha1.DatabaseList
	// List cluster-wide to handle both tenant-namespace (ADR043) and legacy shared namespace resources
	if err := e.Client.List(ctx, &databases, client.MatchingLabels{core.LabelTenant: state.WorkspaceID}); err != nil {
		return err
	}
	for i := range databases.Items {
		if err := e.enforceDatabase(ctx, &databases.Items[i], state.WorkspaceID, marker); err != nil {
			return err
		}
	}
	var keyvalues appv1alpha1.KeyValueList
	// List cluster-wide to handle both tenant-namespace (ADR043) and legacy shared namespace resources
	if err := e.Client.List(ctx, &keyvalues, client.MatchingLabels{core.LabelTenant: state.WorkspaceID}); err != nil {
		return err
	}
	for i := range keyvalues.Items {
		if err := e.enforceKeyValue(ctx, &keyvalues.Items[i], state.WorkspaceID, marker); err != nil {
			return err
		}
	}
	return nil
}

func (e *KubernetesEnforcer) enforceApp(ctx context.Context, a *appv1alpha1.App, workspaceID, marker string) error {
	owned := a.Annotations[BillingEnforcementAnnotation]
	if a.Spec.Suspended && owned == "" {
		return nil
	} // user/operator-owned
	entry, err := e.Store.EnsureBillingEnforcement(ctx, store.BillingEnforcement{WorkspaceID: workspaceID, ResourceKind: store.ResourceKindService, ResourceName: a.Name, MarkerID: marker})
	if err != nil {
		return err
	}
	if a.Spec.Suspended && owned == entry.MarkerID {
		return nil
	}
	if id := a.Labels[store.LabelAppID]; a.Labels[store.LabelManagedBy] == store.ManagedByValue && id != "" {
		if err := e.Store.SetAppSuspended(ctx, id, true); err != nil {
			return fmt.Errorf("suspend App row %s: %w", a.Name, err)
		}
	}
	base := client.MergeFrom(a.DeepCopy())
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[BillingEnforcementAnnotation] = entry.MarkerID
	a.Spec.Suspended = true
	return e.Client.Patch(ctx, a, base)
}

func (e *KubernetesEnforcer) enforceDatabase(ctx context.Context, d *appv1alpha1.Database, workspaceID, marker string) error {
	owned := d.Annotations[BillingEnforcementAnnotation]
	if d.Spec.Suspended && owned == "" {
		return nil
	}
	entry, err := e.Store.EnsureBillingEnforcement(ctx, store.BillingEnforcement{WorkspaceID: workspaceID, ResourceKind: store.ResourceKindPostgres, ResourceName: d.Name, MarkerID: marker})
	if err != nil {
		return err
	}
	if d.Spec.Suspended && owned == entry.MarkerID {
		return nil
	}
	base := client.MergeFrom(d.DeepCopy())
	if d.Annotations == nil {
		d.Annotations = map[string]string{}
	}
	d.Annotations[BillingEnforcementAnnotation] = entry.MarkerID
	d.Spec.Suspended = true
	return e.Client.Patch(ctx, d, base)
}

func (e *KubernetesEnforcer) enforceKeyValue(ctx context.Context, kv *appv1alpha1.KeyValue, workspaceID, marker string) error {
	owned := kv.Annotations[BillingEnforcementAnnotation]
	if kv.Spec.Suspended && owned == "" {
		return nil
	}
	entry, err := e.Store.EnsureBillingEnforcement(ctx, store.BillingEnforcement{WorkspaceID: workspaceID, ResourceKind: store.ResourceKindKeyValue, ResourceName: kv.Name, MarkerID: marker})
	if err != nil {
		return err
	}
	if kv.Spec.Suspended && owned == entry.MarkerID {
		return nil
	}
	base := client.MergeFrom(kv.DeepCopy())
	if kv.Annotations == nil {
		kv.Annotations = map[string]string{}
	}
	kv.Annotations[BillingEnforcementAnnotation] = entry.MarkerID
	kv.Spec.Suspended = true
	return e.Client.Patch(ctx, kv, base)
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
	switch v := obj.(type) {
	case *appv1alpha1.App:
		if id := v.Labels[store.LabelAppID]; v.Labels[store.LabelManagedBy] == store.ManagedByValue && id != "" {
			if err := e.Store.SetAppSuspended(ctx, id, false); err != nil {
				return err
			}
		}
		v.Spec.Suspended = false
	case *appv1alpha1.Database:
		v.Spec.Suspended = false
	case *appv1alpha1.KeyValue:
		v.Spec.Suspended = false
	}
	if err := e.Client.Patch(ctx, obj, base); err != nil {
		return err
	}
	return markRecovered()
}
