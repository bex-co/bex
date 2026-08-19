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

package main

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestReconcileRestoresHealthWhenLastInvalidKeyValueIsDeleted is w1/m76
// t006's regression test: router.delete clears the invalid mark, so the
// deletion path must re-publish health like the pg twin does. Before the fix
// it returned early without SetHealthy, leaving bex_kv_proxy_healthy stuck at
// 0 after the last malformed KeyValue was deleted until an unrelated
// reconcile fired.
func TestReconcileRestoresHealthWhenLastInvalidKeyValueIsDeleted(t *testing.T) {
	testScheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatal(err)
	}
	kv := &appv1alpha1.KeyValue{}
	kv.Name = "broken"
	kv.Namespace = "default"
	// The finalizer keeps the object visible with a deletion timestamp after
	// Delete — exactly the watcher branch the fix touches.
	kv.Finalizers = []string{"test.bex.co/hold"}
	kv.Spec.Public = true
	kv.Status.ExternalHost = "broken.kv.bex.co"
	kv.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "not-a-cidr"}}
	cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(kv).Build()

	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "kv_proxy", "key_value")
	meter.SetHealthy(true)
	watcher := &kvWatcher{Client: cl, router: newRouter("kv.bex.co"), meter: meter}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "broken"}}
	ctx := context.Background()

	if _, err := watcher.Reconcile(ctx, req); err == nil {
		t.Fatal("reconciling an invalid allowlist must return the parse error")
	}
	if got := gatheredGauge(t, registry, "bex_kv_proxy_healthy"); got != 0 {
		t.Fatalf("bex_kv_proxy_healthy = %v after an invalid CR, want 0", got)
	}

	if err := cl.Delete(ctx, kv); err != nil {
		t.Fatal(err)
	}
	if _, err := watcher.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := gatheredGauge(t, registry, "bex_kv_proxy_healthy"); got != 1 {
		t.Fatalf("bex_kv_proxy_healthy = %v after deleting the last invalid CR, want 1 (the gauge must recover)", got)
	}
}

func gatheredGauge(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == name && len(family.Metric) == 1 {
			return family.Metric[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("gauge %s not found in registry", name)
	return 0
}
