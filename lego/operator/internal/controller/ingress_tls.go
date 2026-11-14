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
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// platformWildcardCovers reports whether host is a single DNS label directly
// under BEX_BASE_DOMAIN: the names the platform's `*.<base>` certificate in
// Traefik's default TLSStore serves (ADR093). Such a host orders no
// certificate of its own (w4/650): Let's Encrypt allows 50 new certificates
// per registered domain per week, and a per-host order for every new
// service's `<slug>.onbex.co` used that budget up. A deeper name, which the
// wildcard does not match, and every custom domain keep their per-host
// HTTP-01 certificate (ADR005).
func (r *AppReconciler) platformWildcardCovers(host string) bool {
	if r.BaseDomain == "" {
		return false
	}
	label, ok := strings.CutSuffix(host, "."+r.BaseDomain)
	return ok && label != "" && !strings.ContainsAny(label, ".*")
}

// ingressTLSFor is host's TLS entry. A host the platform wildcard covers
// names no Secret, the shape of the platform-host fallback Ingress
// (config/activator/platform-host-fallback-ingress.yaml): Traefik still mints
// the host's HTTPS router and serves its default certificate, and
// cert-manager's ingress-shim skips the entry instead of ordering one.
func (r *AppReconciler) ingressTLSFor(appName, host string, first bool) networkingv1.IngressTLS {
	entry := networkingv1.IngressTLS{Hosts: []string{host}}
	if !r.platformWildcardCovers(host) {
		entry.SecretName = appv1alpha1.TLSSecretName(appName, first, host)
	}
	return entry
}

// settleIngressTLS finishes an App Ingress's TLS once its hosts are
// appended: an Ingress none of whose entries names a Secret drops the
// cert-manager issuer, so ingress-shim leaves it alone (w4/650), and usable
// certificate references go first (prioritizeIngressTLS).
func (r *AppReconciler) settleIngressTLS(ctx context.Context, ing *networkingv1.Ingress) error {
	if len(ingressTLSSecretNames(ing)) == 0 {
		delete(ing.Annotations, certManagerClusterIssuerAnnotation)
	}
	return r.prioritizeIngressTLS(ctx, ing)
}

// ingressTLSSecretNames is the set of Secrets ing's TLS entries name.
func ingressTLSSecretNames(ing *networkingv1.Ingress) map[string]bool {
	names := map[string]bool{}
	for _, entry := range ing.Spec.TLS {
		if entry.SecretName != "" {
			names[entry.SecretName] = true
		}
	}
	return names
}

// writeAppIngress create-or-updates ing through mutate. ingress-shim deletes
// the Certificates of TLS entries an Ingress drops, but only while the
// Ingress carries its issuer; when the written Ingress has none, those
// Certificates are deleted here, once, so a platform host's old Certificate
// stops renewing or retrying a rate-limited order (w4/650). Its Secret stays
// until App deletion sweeps the App's TLS Secrets.
func (r *AppReconciler) writeAppIngress(ctx context.Context, ing *networkingv1.Ingress, mutate func() error) error {
	var dropped []string
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		previous := ingressTLSSecretNames(ing)
		if err := mutate(); err != nil {
			return err
		}
		current := ingressTLSSecretNames(ing)
		for name := range previous {
			if !current[name] {
				dropped = append(dropped, name)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if _, shim := ing.Annotations[certManagerClusterIssuerAnnotation]; shim || r.ClusterIssuer == "" {
		return nil
	}
	for _, name := range dropped {
		if err := r.deleteIngressCertificate(ctx, ing, name); err != nil {
			return err
		}
	}
	return nil
}

// deleteIngressCertificate deletes ingress-shim's Certificate for the Secret
// name when ing controls it, never one another Ingress still serves.
func (r *AppReconciler) deleteIngressCertificate(ctx context.Context, ing *networkingv1.Ingress, name string) error {
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	if err := r.Get(ctx, client.ObjectKey{Namespace: ing.Namespace, Name: name}, cert); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("read Certificate %s/%s of a dropped TLS entry: %w", ing.Namespace, name, err)
	}
	if !metav1.IsControlledBy(cert, ing) {
		return nil
	}
	if err := deleteTolerantly(ctx, r.Client, cert); err != nil {
		return fmt.Errorf("delete Certificate %s/%s of a dropped TLS entry: %w", ing.Namespace, name, err)
	}
	return nil
}

// prioritizeIngressTLS puts usable certificate references before missing or
// incomplete ones. Traefik's Kubernetes provider stops collecting certificates
// at the first broken reference in an Ingress; it retains the certificates it
// already read. Keep every reference so ingress-shim still issues each host's
// certificate, and leave host/rule order and Secret names unchanged. An entry
// naming no Secret references nothing (Traefik skips it), so it counts as
// usable.
func (r *AppReconciler) prioritizeIngressTLS(ctx context.Context, ing *networkingv1.Ingress) error {
	if len(ing.Spec.TLS) < 2 {
		return nil
	}
	var ready, pending []networkingv1.IngressTLS
	for _, entry := range ing.Spec.TLS {
		if entry.SecretName == "" {
			ready = append(ready, entry)
			continue
		}
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
