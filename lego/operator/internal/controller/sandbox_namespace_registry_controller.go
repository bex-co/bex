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

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/operator/internal/registry"
)

// sandboxRegimeLabel marks the per-workspace `<ws>-sandbox` namespaces the
// control plane provisions (ADR043; stamped by bex-api's NamespaceReconciler).
const sandboxRegimeLabel = "app.bex.co/regime"

// SandboxNamespaceRegistryReconciler provisions per-workspace snapshot
// resume-pull credentials (w3/m42 t002): for every namespace labeled
// app.bex.co/regime=sandbox it mints a read-only Zot user scoped to that
// namespace's snapshot repositories plus the in-namespace
// kubernetes.io/dockerconfigjson Secret the patched OpenSandbox controller's
// --resume-pull-secret flag references, and revokes both when the namespace
// goes away. Mechanism only: the workspace lifecycle itself belongs to
// bex-api; this reconciler reacts to the namespaces it finds.
type SandboxNamespaceRegistryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Registry *registry.Creds
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *SandboxNamespaceRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var ns corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		if apierrors.IsNotFound(err) {
			// The watch is label-filtered, so a vanished namespace here was a
			// sandbox namespace: drop its Zot identity. Idempotent when the
			// credentials never existed.
			if err := r.Registry.RevokeSnapshotCreds(ctx, req.Name); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("revoked snapshot pull credentials", "namespace", req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !ns.DeletionTimestamp.IsZero() {
		// Revoke as soon as teardown starts; the in-namespace Secret is
		// deleted with the namespace itself.
		if err := r.Registry.RevokeSnapshotCreds(ctx, ns.Name); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.Registry.EnsureSnapshotCreds(ctx, ns.Name); err != nil {
		if err == registry.ErrConflictRequeue {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler, filtered to sandbox-regime
// namespaces so unrelated namespace churn never reaches Reconcile.
func (r *SandboxNamespaceRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	sandboxOnly := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels()[sandboxRegimeLabel] == "sandbox"
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}, builder.WithPredicates(sandboxOnly)).
		Named("sandboxnamespaceregistry").
		Complete(r)
}
