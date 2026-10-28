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
	"os"
	"path/filepath"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// traefikRouterPriorityAnnotation pins a Traefik router's priority.
const traefikRouterPriorityAnnotation = "traefik.ingress.kubernetes.io/router.priority"

// The platform-host fallback route is a platform manifest (the operator may
// not write Ingresses in bex-system): every platform host at the lowest
// priority, to the activator. An App's own route never pins a priority, so
// Traefik's rule-length default always ranks it above (w4/m235 t008).
func TestPlatformHostFallbackRouteIsTheLowestPriorityCatchAll(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "activator", "platform-host-fallback-ingress.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var ing networkingv1.Ingress
	if err := yaml.Unmarshal(raw, &ing); err != nil {
		t.Fatal(err)
	}
	rule := ing.Spec.Rules[0]
	backend := rule.HTTP.Paths[0].Backend.Service
	if ing.Annotations[traefikRouterPriorityAnnotation] != "1" || rule.Host != "*.onbex.co" ||
		backend.Name != "activator" || backend.Port.Number != 8888 ||
		len(ing.Spec.TLS) != 1 || ing.Spec.TLS[0].SecretName != "" || *ing.Spec.IngressClassName != ingressClassTraefik {
		t.Fatalf("fallback route = %+v", ing)
	}

	app := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	(&AppReconciler{}).applyIngressRouting(app, nil)
	if _, ok := app.Annotations[traefikRouterPriorityAnnotation]; ok {
		t.Fatal("an App route pins a priority; it must keep Traefik's default above the fallback")
	}
}
