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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// platformHostFallbackIngress answers every platform host no App route claims
// yet. A new service's own route exists only after the operator's first
// reconcile writes it, and until Traefik loads it the host answered Traefik's
// raw "404 page not found" (w4/m235 t008). This route sends those requests to
// the activator, which already knows the App from its spec: it answers the
// first-deploy response for an App that never served and a bex 404 for a host
// no public App owns.
const platformHostFallbackIngress = "bex-platform-host-fallback"

// traefikRouterPriorityAnnotation pins a router's priority. Every App route
// keeps Traefik's default, its rule length, which is always far above 1.
const traefikRouterPriorityAnnotation = "traefik.ingress.kubernetes.io/router.priority"

// EnsurePlatformHostFallback writes the lowest-priority `*.<base domain>`
// route to the activator. It needs both a platform domain and an activator;
// without either there is nothing to answer for. The TLS entry names no
// Secret, so Traefik serves its default wildcard certificate. cl is an
// uncached client: the route lives in the platform namespace, which the
// manager's cache need not watch.
func (r *AppReconciler) EnsurePlatformHostFallback(ctx context.Context, cl client.Client) error {
	if r.Mode != ModeKubernetes || r.BaseDomain == "" || r.ActivatorService == "" {
		return nil
	}
	host := "*." + r.BaseDomain
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: platformHostFallbackIngress, Namespace: r.activatorNamespace()}}
	_, err := controllerutil.CreateOrUpdate(ctx, cl, ing, func() error {
		if ing.Annotations == nil {
			ing.Annotations = map[string]string{}
		}
		ing.Annotations[traefikRouterPriorityAnnotation] = "1"
		class := ingressClassTraefik
		ing.Spec.IngressClassName = &class
		ing.Spec.TLS = []networkingv1.IngressTLS{{Hosts: []string{host}}}
		pathType := networkingv1.PathTypePrefix
		ing.Spec.Rules = []networkingv1.IngressRule{{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path: "/", PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: r.ActivatorService, Port: networkingv1.ServiceBackendPort{Number: int32(r.ActivatorPort)},
					}},
				}},
			}},
		}}
		return nil
	})
	return err
}
