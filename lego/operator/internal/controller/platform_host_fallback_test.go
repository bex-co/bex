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
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// The fallback route claims every platform host at the lowest priority and
// sends it to the activator; an App's own route never sets a priority, so
// Traefik's rule-length default always ranks it above (w4/m235 t008).
func TestPlatformHostFallbackRouteIsTheLowestPriorityCatchAll(t *testing.T) {
	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888}
	for range 2 { // idempotent
		if err := r.EnsurePlatformHostFallback(ctx, cl); err != nil {
			t.Fatal(err)
		}
	}
	var ing networkingv1.Ingress
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-system", Name: platformHostFallbackIngress}, &ing); err != nil {
		t.Fatal(err)
	}
	rule := ing.Spec.Rules[0]
	backend := rule.HTTP.Paths[0].Backend.Service
	if ing.Annotations[traefikRouterPriorityAnnotation] != "1" || rule.Host != "*.onbex.co" ||
		backend.Name != "bex-activator" || backend.Port.Number != 8888 ||
		len(ing.Spec.TLS) != 1 || ing.Spec.TLS[0].SecretName != "" {
		t.Fatalf("fallback route = %+v", ing)
	}

	app := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	r.applyIngressRouting(app, nil)
	if _, ok := app.Annotations[traefikRouterPriorityAnnotation]; ok {
		t.Fatal("an App route pins a priority; it must keep Traefik's default above the fallback")
	}

	for _, r := range []*AppReconciler{
		{Client: cl, Mode: ModeKubernetes, ActivatorService: "bex-activator"},
		{Client: cl, Mode: ModeKubernetes, BaseDomain: "onbex.co"},
	} {
		if err := r.EnsurePlatformHostFallback(ctx, fake.NewClientBuilder().WithScheme(deletionScheme(t)).Build()); err != nil {
			t.Fatalf("unconfigured fallback = %v, want a no-op", err)
		}
	}
}
