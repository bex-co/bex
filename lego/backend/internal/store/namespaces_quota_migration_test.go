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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A namespace migrating from the single-quota shape must never be without a
// CPU or pod cap (w2/m247): the scoped tenant-compute-quota has to exist before
// the update that strips those keys from tenant-quota, and if creating it
// fails, tenant-quota must keep them. TestComputeQuotaExemptsDatastores checks
// only the end state, which a swapped order would also reach.
func TestQuotaMigrationNeverLeavesNamespaceUncapped(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	store := newMemStore()
	legacy, _ := store.CreateTenant(ctx, "legacy", PlanHobby)
	ns := WorkspaceNamespace(legacy.ID)

	seed := func(cl client.Client, r *NamespaceReconciler) {
		t.Helper()
		if err := cl.Create(ctx, r.workspaceNamespaceObject(ns, legacy, RegimeHosting)); err != nil {
			t.Fatal(err)
		}
		if err := cl.Create(ctx, &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: core.TenantQuotaName, Namespace: ns, Labels: r.managedLabels(legacy.ID)},
			Spec:       corev1.ResourceQuotaSpec{Hard: quotaForPlan(PlanHobby)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	isTenantQuota := func(obj client.Object) bool {
		return obj.GetNamespace() == ns && obj.GetName() == core.TenantQuotaName
	}

	t.Run("compute quota exists before tenant-quota loses cpu and pods", func(t *testing.T) {
		stripped := false
		var cl client.WithWatch
		cl = fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if q, ok := obj.(*corev1.ResourceQuota); ok && isTenantQuota(q) {
					_, hasCPU := q.Spec.Hard[corev1.ResourceRequestsCPU]
					_, hasPods := q.Spec.Hard[corev1.ResourcePods]
					if !hasCPU || !hasPods {
						stripped = true
						var compute corev1.ResourceQuota
						if err := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: core.TenantComputeQuotaName}, &compute); err != nil {
							t.Errorf("tenant-quota lost cpu/pods before tenant-compute-quota existed: %v", err)
						}
					}
				}
				return c.Update(ctx, obj, opts...)
			},
		}).Build()
		r := NewNamespaceReconciler(cl, store)
		seed(cl, r)
		if err := r.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if !stripped {
			t.Fatal("the migration never moved cpu/pods out of tenant-quota")
		}
	})

	t.Run("a failed compute-quota create keeps tenant-quota's caps", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if obj.GetNamespace() == ns && obj.GetName() == core.TenantComputeQuotaName {
					return errors.New("apiserver unavailable")
				}
				return c.Create(ctx, obj, opts...)
			},
		}).Build()
		r := NewNamespaceReconciler(cl, store)
		seed(cl, r)
		_ = r.ReconcileOnce(ctx) // the failure is reported; what matters is what was left behind
		var rest corev1.ResourceQuota
		if err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: core.TenantQuotaName}, &rest); err != nil {
			t.Fatal(err)
		}
		for _, key := range []corev1.ResourceName{corev1.ResourceRequestsCPU, corev1.ResourceRequestsMemory, corev1.ResourcePods} {
			if _, ok := rest.Spec.Hard[key]; !ok {
				t.Errorf("tenant-quota lost %s although tenant-compute-quota was never created", key)
			}
		}
	})
}
