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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// prioritizeIngressTLS puts usable certificate references before missing or
// incomplete ones. Traefik's Kubernetes provider stops collecting certificates
// at the first broken reference in an Ingress; it retains the certificates it
// already read. Keep every reference so ingress-shim still issues each host's
// certificate, and leave host/rule order and Secret names unchanged.
func (r *AppReconciler) prioritizeIngressTLS(ctx context.Context, ing *networkingv1.Ingress) error {
	if len(ing.Spec.TLS) < 2 {
		return nil
	}
	var ready, pending []networkingv1.IngressTLS
	for _, entry := range ing.Spec.TLS {
		var secret corev1.Secret
		err := r.buildPlaneClient().Get(ctx, client.ObjectKey{Namespace: ing.Namespace, Name: entry.SecretName}, &secret)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("read Ingress TLS Secret %s/%s: %w", ing.Namespace, entry.SecretName, err)
		}
		if err == nil && len(secret.Data[corev1.TLSCertKey]) > 0 && len(secret.Data[corev1.TLSPrivateKeyKey]) > 0 {
			ready = append(ready, entry)
		} else {
			pending = append(pending, entry)
		}
	}
	ing.Spec.TLS = append(ready, pending...)
	return nil
}
